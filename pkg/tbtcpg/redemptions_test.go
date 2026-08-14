package tbtcpg_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	"github.com/keep-network/keep-core/pkg/tbtcpg/internal/test"
)

// Test based on example testnet redemption transaction:
// https://live.blockcypher.com/btc-testnet/tx/2724545276df61f43f1e92c4b9f1dd3c9109595c022dbd9dc003efbad8ded38b
func TestEstimateRedemptionFee(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	redeemersOutputScripts := []bitcoin.Script{
		fromHex("76a9142cd680318747b720d67bf4246eb7403b476adb3488ac"),                   // P2PKH
		fromHex("0014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0"),                         // P2WPKH
		fromHex("a914011beb6fb8499e075a57027fb0a58384f2d3f78487"),                       // P2SH
		fromHex("0020ef0b4d985752aa5ef6243e4c6f6bebc2a007e7d671ef27d4b1d0db8dcc93bc1c"), // P2WSH
	}

	// The fixture above yields a 250 vByte redemption transaction.
	const vsize = 250

	tests := map[string]struct {
		estimateSatPerVByte int64
		txMaxTotalFee       uint64
		expectedFee         int
		expectErrorContains string
	}{
		"estimate above the floor is buffered by 25%": {
			estimateSatPerVByte: 16,
			txMaxTotalFee:       100000,
			expectedFee:         5000, // ceil(16*1.25)=20 sat/vByte * 250 vByte
		},
		"low estimate is raised to the minimum floor": {
			estimateSatPerVByte: 1,
			txMaxTotalFee:       100000,
			expectedFee:         1250, // max(5, ceil(1*1.25)=2)=5 sat/vByte * 250 vByte
		},
		"minimum floor above the cap returns an error": {
			estimateSatPerVByte: 1,
			txMaxTotalFee:       uint64(3 * vsize), // below the 5 sat/vByte floor
			expectErrorContains: "minimum safe transaction fee",
		},
		"raw estimate above the cap returns an error": {
			estimateSatPerVByte: 16,   // raw 16*250 = 4000
			txMaxTotalFee:       3000, // below the raw estimate
			expectErrorContains: "estimated fee exceeds the maximum fee",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, tc.estimateSatPerVByte)

			actualFee, err := tbtcpg.EstimateRedemptionFee(
				btcChain,
				redeemersOutputScripts,
				tc.txMaxTotalFee,
			)

			if tc.expectErrorContains != "" {
				if err == nil {
					t.Fatalf("expected an error, got fee [%d]", actualFee)
				}
				if !strings.Contains(err.Error(), tc.expectErrorContains) {
					t.Fatalf(
						"expected error containing [%s]; got [%v]",
						tc.expectErrorContains, err,
					)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			testutils.AssertIntsEqual(t, "fee", tc.expectedFee, int(actualFee))
		})
	}
}

// TestEstimateRedemptionFeeForWalletScript covers the wallet-script-aware
// variant. A P2TR main UTXO input and change output make the transaction one
// virtual byte larger than the legacy P2WPKH shape, so estimating a Taproot
// wallet with the legacy shape would underpay.
func TestEstimateRedemptionFeeForWalletScript(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	redeemersOutputScripts := []bitcoin.Script{
		fromHex("76a9142cd680318747b720d67bf4246eb7403b476adb3488ac"),                   // P2PKH
		fromHex("0014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0"),                         // P2WPKH
		fromHex("a914011beb6fb8499e075a57027fb0a58384f2d3f78487"),                       // P2SH
		fromHex("0020ef0b4d985752aa5ef6243e4c6f6bebc2a007e7d671ef27d4b1d0db8dcc93bc1c"), // P2WSH
	}

	legacyWalletOutputScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}

	taprootWalletOutputScript, err := bitcoin.PayToTaproot([32]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		walletOutputScript bitcoin.Script
		// The legacy fixture yields a 250 vByte transaction; the Taproot shape
		// is one virtual byte larger. Both are below the 5 sat/vByte floor at
		// an estimate of 1 sat/vByte, so each is clamped to floor * vsize.
		expectedFee int
	}{
		"legacy P2WPKH wallet": {
			walletOutputScript: legacyWalletOutputScript,
			expectedFee:        1250, // 5 sat/vByte * 250 vByte
		},
		"Taproot wallet": {
			walletOutputScript: taprootWalletOutputScript,
			expectedFee:        1255, // 5 sat/vByte * 251 vByte
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, 1)

			actualFee, err := tbtcpg.EstimateRedemptionFeeForWalletScript(
				btcChain,
				tc.walletOutputScript,
				redeemersOutputScripts,
				100000,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(t, "fee", tc.expectedFee, int(actualFee))
		})
	}
}

func TestRedemptionAction_FindPendingRedemptions(t *testing.T) {
	scenarios, err := test.LoadFindPendingRedemptionsTestScenario()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()

			// Set the average block time enforced by the scenario.
			tbtcChain.SetAverageBlockTime(scenario.ChainParameters.AverageBlockTime)

			// Set the scenario's current block using a mock block counter.
			// This is needed to build a proper filter for the
			// `PastRedemptionRequestedEvents` call.
			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(scenario.ChainParameters.CurrentBlock)
			tbtcChain.SetBlockCounter(blockCounter)

			// Set relevant governable parameters based on values provided by
			// the scenario.
			tbtcChain.SetRedemptionParameters(
				0,
				0,
				0,
				0,
				scenario.ChainParameters.RequestTimeout,
				nil,
				0,
			)
			tbtcChain.SetRedemptionRequestMinAge(scenario.ChainParameters.RequestMinAge)

			requestTimeoutBlocks := uint64(scenario.ChainParameters.RequestTimeout) /
				uint64(scenario.ChainParameters.AverageBlockTime.Seconds())

			// Record scenario pending redemptions to the local chain.
			for _, pendingRedemption := range scenario.PendingRedemptions {
				// Record the corresponding event. Set only relevant fields.
				err = tbtcChain.AddPastRedemptionRequestedEvent(
					&tbtc.RedemptionRequestedEventFilter{
						// Remember about including the constant factor
						// of 1000 blocks.
						StartBlock:          scenario.ChainParameters.CurrentBlock - requestTimeoutBlocks - 1000,
						WalletPublicKeyHash: [][20]byte{pendingRedemption.WalletPublicKeyHash},
					},
					&tbtc.RedemptionRequestedEvent{
						WalletPublicKeyHash:  pendingRedemption.WalletPublicKeyHash,
						RedeemerOutputScript: pendingRedemption.RedeemerOutputScript,
						RequestedAmount:      pendingRedemption.RequestedAmount,
						BlockNumber:          pendingRedemption.RequestBlock,
					},
				)

				// Record the corresponding request object. Set only relevant fields.
				tbtcChain.SetPendingRedemptionRequest(
					pendingRedemption.WalletPublicKeyHash,
					&tbtc.RedemptionRequest{
						RedeemerOutputScript: pendingRedemption.RedeemerOutputScript,
						RequestedAmount:      pendingRedemption.RequestedAmount,
						RequestedAt:          pendingRedemption.RequestedAt,
					},
				)

				// Record the redemption processing delay.
				tbtcChain.SetRedemptionDelay(
					pendingRedemption.WalletPublicKeyHash,
					pendingRedemption.RedeemerOutputScript,
					pendingRedemption.Delay,
				)
			}

			task := tbtcpg.NewRedemptionTask(tbtcChain, nil)

			redeemersOutputScripts, err := task.FindPendingRedemptions(
				&testutils.MockLogger{},
				scenario.WalletPublicKeyHash,
				scenario.MaxNumberOfRequests,
			)
			if err != nil {
				t.Fatal(err)
			}

			if diff := deep.Equal(
				scenario.ExpectedRedeemersOutputScripts,
				redeemersOutputScripts,
			); diff != nil {
				t.Errorf("invalid wallets pending redemptions: %v", diff)
			}
		})
	}
}

func TestRedemptionAction_FindPendingRedemptions_UsesChainTimestamp(t *testing.T) {
	currentBlock := uint64(100000)
	averageBlockTime := 10 * time.Second
	requestTimeout := uint32(86400)
	requestMinAge := uint32(600)
	chainNow := time.Now().Add(3 * time.Hour)

	walletPublicKeyHash := hexToByte20(
		"7670343fc00ccc2d0cd65360e6ad400697ea0fed",
	)
	redeemerOutputScript := bitcoin.Script{
		0x00, 0x14, 0xe6, 0xf9, 0xd7, 0x47, 0x26, 0xb1, 0x9b, 0x75,
		0xf1, 0x6f, 0xe1, 0xe9, 0xfe, 0xae, 0xc0, 0x48, 0xaa, 0x4f,
		0xa1, 0xd0,
	}

	tbtcChain := tbtcpg.NewLocalChain()
	tbtcChain.SetAverageBlockTime(averageBlockTime)
	tbtcChain.SetCurrentBlockTimestamp(chainNow)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	tbtcChain.SetBlockCounter(blockCounter)

	tbtcChain.SetRedemptionParameters(
		0,
		0,
		0,
		0,
		requestTimeout,
		nil,
		0,
	)
	tbtcChain.SetRedemptionRequestMinAge(requestMinAge)

	requestTimeoutBlocks := uint64(requestTimeout) /
		uint64(averageBlockTime.Seconds())

	err := tbtcChain.AddPastRedemptionRequestedEvent(
		&tbtc.RedemptionRequestedEventFilter{
			StartBlock:          currentBlock - requestTimeoutBlocks - 1000,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.RedemptionRequestedEvent{
			WalletPublicKeyHash:  walletPublicKeyHash,
			RedeemerOutputScript: redeemerOutputScript,
			RequestedAmount:      100000,
			BlockNumber:          90000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetPendingRedemptionRequest(
		walletPublicKeyHash,
		&tbtc.RedemptionRequest{
			RedeemerOutputScript: redeemerOutputScript,
			RequestedAmount:      100000,
			// This timestamp is deliberately in the future compared to the
			// host clock, but mature compared to the chain clock.
			RequestedAt: chainNow.Add(-(time.Duration(requestMinAge) + 1) * time.Second),
		},
	)
	tbtcChain.SetRedemptionDelay(
		walletPublicKeyHash,
		redeemerOutputScript,
		0,
	)

	task := tbtcpg.NewRedemptionTask(tbtcChain, nil)

	redeemersOutputScripts, err := task.FindPendingRedemptions(
		&testutils.MockLogger{},
		walletPublicKeyHash,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(
		[]bitcoin.Script{redeemerOutputScript},
		redeemersOutputScripts,
	); diff != nil {
		t.Errorf("invalid wallets pending redemptions: %v", diff)
	}
}

func TestRedemptionAction_ProposeRedemption(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], fromHex(""))

	redeemersOutputScripts := []bitcoin.Script{
		fromHex("00140000000000000000000000000000000000000001"),
		fromHex("00140000000000000000000000000000000000000002"),
	}

	var tests = map[string]struct {
		fee              int64
		walletID         [32]byte
		expectedProposal *tbtc.RedemptionProposal
	}{
		"fee provided": {
			fee: 10000,
			expectedProposal: &tbtc.RedemptionProposal{
				RedeemersOutputScripts: redeemersOutputScripts,
				RedemptionTxFee:        big.NewInt(10000),
			},
		},
		"fee estimated": {
			fee: 0, // trigger fee estimation
			expectedProposal: &tbtc.RedemptionProposal{
				RedeemersOutputScripts: redeemersOutputScripts,
				// raw 4300 (172 vByte * 25 sat/vByte), buffered to
				// ceil(25*1.25)=32 sat/vByte * 172 = 5504, below the cap.
				RedemptionTxFee: big.NewInt(5504),
			},
		},
		"fee estimated for FROST wallet": {
			fee:      0, // trigger fee estimation
			walletID: [32]byte{0x01},
			expectedProposal: &tbtc.RedemptionProposal{
				RedeemersOutputScripts: redeemersOutputScripts,
				// The P2TR main UTXO and change output make this one virtual
				// byte larger than the legacy shape: raw 4325 (173 vByte *
				// 25 sat/vByte), buffered to ceil(25*1.25)=32 sat/vByte * 173
				// = 5536, below the cap.
				RedemptionTxFee: big.NewInt(5536),
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			btcChain.SetEstimateSatPerVByteFee(1, 25)
			tbtcChain.SetWallet(
				walletPublicKeyHash,
				&tbtc.WalletChainData{WalletID: test.walletID},
			)

			// Fee estimation bounds the safe-minimum floor by the redemption
			// tx max total fee; set a cap comfortably above the buffered fee.
			tbtcChain.SetRedemptionParameters(0, 0, 0, 6000, 0, nil, 0)

			for _, script := range redeemersOutputScripts {
				tbtcChain.SetPendingRedemptionRequest(
					walletPublicKeyHash,
					&tbtc.RedemptionRequest{
						RedeemerOutputScript: script,
					},
				)
			}

			err := tbtcChain.SetRedemptionProposalValidationResult(
				walletPublicKeyHash,
				test.expectedProposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			task := tbtcpg.NewRedemptionTask(tbtcChain, btcChain)

			proposal, err := task.ProposeRedemption(
				&testutils.MockLogger{},
				walletPublicKeyHash,
				redeemersOutputScripts,
				test.fee,
			)
			if err != nil {
				t.Fatal(err)
			}

			if diff := deep.Equal(proposal, test.expectedProposal); diff != nil {
				t.Errorf("invalid redemption proposal: %v", diff)
			}
		})
	}
}

// warnCapturingLogger records Warnf messages so tests can assert on the
// per-request fee warning emitted during redemption fee estimation. All other
// log methods are inherited as no-ops from testutils.MockLogger.
type warnCapturingLogger struct {
	testutils.MockLogger
	warnings []string
}

func (l *warnCapturingLogger) Warnf(format string, args ...interface{}) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

// TestRedemptionAction_ProposeRedemption_PerRequestFeeWarning verifies that the
// diagnostic warning about a floored fee share exceeding the per-request maximum
// fee (TxMaxFee) is emitted for the worst-case (largest) share. The on-chain fee
// distribution assigns the division remainder to the last request, so that
// request pays floor(total/count)+total%count. The warning must reflect that
// last-request share, not the smaller even share.
func TestRedemptionAction_ProposeRedemption_PerRequestFeeWarning(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], fromHex(""))

	// Three redeemer output scripts make the estimated fee (6496 at 25 sat/vByte
	// buffered to 32) indivisible by the request count: the even share is
	// 6496/3 = 2165 and the last request pays 2165 + 6496%3 = 2166.
	redeemersOutputScripts := []bitcoin.Script{
		fromHex("00140000000000000000000000000000000000000001"),
		fromHex("00140000000000000000000000000000000000000002"),
		fromHex("00140000000000000000000000000000000000000003"),
	}

	var tests = map[string]struct {
		txMaxFee      uint64
		expectWarning bool
	}{
		"worst-case share within the per-request cap": {
			txMaxFee:      3000, // 2166 <= 3000
			expectWarning: false,
		},
		"even share at the cap but last-request share exceeds it": {
			// The even share 2165 equals the cap (a floor-division check would
			// not warn), but the last request pays 2166 and would be rejected.
			txMaxFee:      2165,
			expectWarning: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			btcChain.SetEstimateSatPerVByteFee(1, 25)

			// A legacy (zero wallet ID) wallet, so fee estimation resolves the
			// P2WPKH output script shape the expectations below assume.
			tbtcChain.SetWallet(
				walletPublicKeyHash,
				&tbtc.WalletChainData{},
			)

			// txMaxFee at index 2; txMaxTotalFee at index 3, set comfortably
			// above the estimated total (6496) so it does not bound the fee.
			tbtcChain.SetRedemptionParameters(0, 0, test.txMaxFee, 8000, 0, nil, 0)

			for _, script := range redeemersOutputScripts {
				tbtcChain.SetPendingRedemptionRequest(
					walletPublicKeyHash,
					&tbtc.RedemptionRequest{
						RedeemerOutputScript: script,
					},
				)
			}

			expectedProposal := &tbtc.RedemptionProposal{
				RedeemersOutputScripts: redeemersOutputScripts,
				RedemptionTxFee:        big.NewInt(6496),
			}

			err := tbtcChain.SetRedemptionProposalValidationResult(
				walletPublicKeyHash,
				expectedProposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			task := tbtcpg.NewRedemptionTask(tbtcChain, btcChain)

			logger := &warnCapturingLogger{}

			_, err = task.ProposeRedemption(
				logger,
				walletPublicKeyHash,
				redeemersOutputScripts,
				0, // trigger fee estimation
			)
			if err != nil {
				t.Fatal(err)
			}

			warned := false
			for _, w := range logger.warnings {
				if strings.Contains(w, "exceeds the per-request maximum fee") {
					warned = true
					break
				}
			}

			if warned != test.expectWarning {
				t.Errorf(
					"per-request fee warning emitted = %v, want %v\nwarnings: %v",
					warned, test.expectWarning, logger.warnings,
				)
			}
		})
	}
}
