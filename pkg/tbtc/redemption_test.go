package tbtc

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tecdsa"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
)

// redemptionFeeTestScripts returns the given number of redeemer output
// scripts, alternating between non-witness P2PKH and witness P2WPKH so the
// size estimate covers both output shapes.
func redemptionFeeTestScripts(t *testing.T, count int) []bitcoin.Script {
	t.Helper()

	scripts := make([]bitcoin.Script, count)
	for i := range scripts {
		publicKeyHash := [20]byte{0x20, byte(i)}

		var err error
		if i%2 == 0 {
			scripts[i], err = bitcoin.PayToPublicKeyHash(publicKeyHash)
		} else {
			scripts[i], err = bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	return scripts
}

// redemptionVsize returns the virtual size ValidateRedemptionProposal
// estimates for a redemption transaction paying to the given scripts.
func redemptionVsize(t *testing.T, scripts []bitcoin.Script) int64 {
	t.Helper()

	estimator := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddPublicKeyHashOutputs(1, true)
	for _, script := range scripts {
		switch bitcoin.GetScriptType(script) {
		case bitcoin.P2PKHScript:
			estimator.AddPublicKeyHashOutputs(1, false)
		case bitcoin.P2WPKHScript:
			estimator.AddPublicKeyHashOutputs(1, true)
		default:
			t.Fatalf("unexpected script type for script [%x]", script)
		}
	}

	vsize, err := estimator.VirtualSize()
	if err != nil {
		t.Fatal(err)
	}

	return vsize
}

// validateRedemptionFee runs ValidateRedemptionProposal for a proposal
// redeeming the given scripts with the given fee against a local chain on
// which the on-chain validation passes and every request is pending,
// isolating the follower-side fee gate. It returns the warnings that were
// logged and the validation error.
func validateRedemptionFee(
	t *testing.T,
	scripts []bitcoin.Script,
	fee *big.Int,
) ([]string, error) {
	t.Helper()

	hostChain := Connect()

	walletPublicKeyHash := [20]byte{0x11}

	for _, script := range scripts {
		hostChain.setPendingRedemptionRequest(
			walletPublicKeyHash,
			&RedemptionRequest{
				RedeemerOutputScript: script,
				RequestedAmount:      100_000,
				TreasuryFee:          1000,
				TxMaxFee:             5000,
			},
		)
	}

	proposal := &RedemptionProposal{
		RedeemersOutputScripts: scripts,
		RedemptionTxFee:        fee,
	}

	err := hostChain.setRedemptionProposalValidationResult(
		walletPublicKeyHash,
		proposal,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	logger := &capturingLogger{}

	_, err = ValidateRedemptionProposal(
		logger,
		walletPublicKeyHash,
		proposal,
		hostChain,
	)

	return logger.warnings, err
}

// TestRedemptionValidator_RejectsBelowProtocolMinimum verifies that the
// follower refuses a redemption proposal whose fee is below
// ProtocolMinWalletTxSatPerVByteFee * (vsize - 1). On the base code the
// check only warned, so the proposal validated without error.
func TestRedemptionValidator_RejectsBelowProtocolMinimum(t *testing.T) {
	for _, requestsCount := range []int{1, 4} {
		scripts := redemptionFeeTestScripts(t, requestsCount)
		vsize := redemptionVsize(t, scripts)
		threshold := ProtocolMinWalletTxSatPerVByteFee * (vsize - 1)

		scenarios := map[string]*big.Int{
			"one satoshi below the threshold": big.NewInt(threshold - 1),
			"relay floor fee":                 big.NewInt(vsize),
		}

		for name, fee := range scenarios {
			t.Run(
				fmt.Sprintf("%d requests/%s", requestsCount, name),
				func(t *testing.T) {
					_, err := validateRedemptionFee(t, scripts, fee)
					if err == nil {
						t.Fatalf(
							"expected fee [%v] to be rejected (threshold [%d])",
							fee,
							threshold,
						)
					}
					if !strings.Contains(err.Error(), "protocol minimum") {
						t.Errorf(
							"expected a protocol minimum error, got: [%v]",
							err,
						)
					}
				},
			)
		}
	}
}

// TestRedemptionValidator_AcceptsAtProtocolMinimum verifies that fees at the
// reject threshold and at the full protocol minimum both validate.
func TestRedemptionValidator_AcceptsAtProtocolMinimum(t *testing.T) {
	for _, requestsCount := range []int{1, 4} {
		scripts := redemptionFeeTestScripts(t, requestsCount)
		vsize := redemptionVsize(t, scripts)

		scenarios := map[string]*big.Int{
			"exactly at the threshold": big.NewInt(
				ProtocolMinWalletTxSatPerVByteFee * (vsize - 1),
			),
			"exactly at the full protocol minimum": big.NewInt(
				ProtocolMinWalletTxSatPerVByteFee * vsize,
			),
		}

		for name, fee := range scenarios {
			t.Run(
				fmt.Sprintf("%d requests/%s", requestsCount, name),
				func(t *testing.T) {
					_, err := validateRedemptionFee(t, scripts, fee)
					if err != nil {
						t.Errorf(
							"expected fee [%v] to be accepted, got: [%v]",
							fee,
							err,
						)
					}
				},
			)
		}
	}
}

// TestRedemptionValidator_WarnsBelowBareFloor verifies that the soft warning
// fires below the operator's bare floor and stays quiet at it, for a fee that
// already clears the protocol minimum threshold.
func TestRedemptionValidator_WarnsBelowBareFloor(t *testing.T) {
	scripts := redemptionFeeTestScripts(t, 1)
	bareFloorFee := MinWalletTxSatPerVByteFee * redemptionVsize(t, scripts)

	warnings, err := validateRedemptionFee(t, scripts, big.NewInt(bareFloorFee-1))
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for a fee below the bare floor")
	}

	warnings, err = validateRedemptionFee(t, scripts, big.NewInt(bareFloorFee))
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if len(warnings) != 0 {
		t.Errorf(
			"expected no warning for a fee at the bare floor, got: %v",
			warnings,
		)
	}
}

// TODO: Think about covering unhappy paths for specific steps of the redemption action.
func TestRedemptionAction_Execute(t *testing.T) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			hostChain := Connect()
			bitcoinChain := newLocalBitcoinChain()

			wallet := wallet{
				// Set only relevant fields.
				publicKey: scenario.WalletPublicKey,
			}
			walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

			// Record the transaction that will serve as redemption transaction's
			// input in the Bitcoin local chain.
			err := bitcoinChain.BroadcastTransaction(scenario.InputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			// redeemersOutputScripts will be needed to build the proposal instance.
			redeemersOutputScripts := make(
				[]bitcoin.Script,
				len(scenario.RedemptionRequests),
			)

			// Record all necessary requests' data on the local host chain.
			for i, request := range scenario.RedemptionRequests {
				hostChain.setPendingRedemptionRequest(
					walletPublicKeyHash,
					&RedemptionRequest{
						Redeemer:             request.Redeemer,
						RedeemerOutputScript: request.RedeemerOutputScript,
						RequestedAmount:      request.RequestedAmount,
						TreasuryFee:          request.TreasuryFee,
						TxMaxFee:             request.TxMaxFee,
						RequestedAt:          request.RequestedAt,
					},
				)

				redeemersOutputScripts[i] = request.RedeemerOutputScript
			}

			totalFee := int64(0)
			for _, feeShare := range scenario.FeeShares {
				totalFee += feeShare
			}

			// Build the redemption proposal based on the scenario data.
			proposal := &RedemptionProposal{
				RedeemersOutputScripts: redeemersOutputScripts,
				RedemptionTxFee:        big.NewInt(totalFee),
			}

			// Choose an arbitrary start block and expiration time.
			proposalProcessingStartBlock := uint64(100)
			proposalExpiryBlock := proposalProcessingStartBlock +
				redemptionProposalValidityBlocks

			// Simulate the on-chain proposal validation passes with success.
			err = hostChain.setRedemptionProposalValidationResult(
				walletPublicKeyHash,
				proposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			// Record the wallet main UTXO hash in the local host chain so
			// the redemption action can detect it.
			var walletMainUtxoHash [32]byte
			if scenario.WalletMainUtxo != nil {
				walletMainUtxoHash = hostChain.ComputeMainUtxoHash(
					scenario.WalletMainUtxo,
				)
			}
			hostChain.setWallet(walletPublicKeyHash, &WalletChainData{
				MainUtxoHash: walletMainUtxoHash,
			})

			// Create a signing executor mock instance.
			signingExecutor := newMockWalletSigningExecutor()

			// The signature within the scenario fixture is in the format
			// suitable for applying them directly to a Bitcoin transaction.
			// However, the signing executor operates on raw tECDSA signatures
			// so, we need to unpack it first.
			rawSignature := &tecdsa.Signature{
				R: scenario.Signature.R,
				S: scenario.Signature.S,
			}

			// Set up the signing executor mock to return the signature from
			// the test fixture when called with the expected parameters.
			// Note that the start block is set based on the proposal
			// processing start block as done within the action.
			signingExecutor.setSignatures(
				[]*big.Int{scenario.ExpectedSigHash},
				proposalProcessingStartBlock,
				[]*tecdsa.Signature{rawSignature},
			)

			action := newRedemptionAction(
				logger.With(),
				hostChain,
				bitcoinChain,
				wallet,
				signingExecutor,
				proposal,
				proposalProcessingStartBlock,
				proposalExpiryBlock,
				func(ctx context.Context, blockHeight uint64) error {
					return nil
				},
				nil,
			)

			// Modify the default parameters of the action to make
			// it possible to execute in the current test environment.
			action.broadcastCheckDelay = 1 * time.Second

			// Test scenarios use a different fee distribution than the
			// default one used by the redemption action. Here we override
			// the default distribution by using the one appropriate for
			// test scenarios.
			action.feeDistribution = func(requests []*RedemptionRequest) []int64 {
				return scenario.FeeShares
			}

			// Test scenarios use the RedemptionChangeLast shape which is
			// different from the default RedemptionChangeFirst shape used
			// by the redemption action. We need to override it.
			action.transactionShape = RedemptionChangeLast

			err = action.execute()
			if err != nil {
				t.Fatal(err)
			}

			// Action execution that completes without an error is a sign of
			// success. However, just in case, make an additional check that
			// the expected redemption transaction was actually broadcasted
			// on the local Bitcoin chain.
			broadcastedRedemptionTransaction, err := bitcoinChain.GetTransaction(
				scenario.ExpectedRedemptionTransactionHash,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedRedemptionTransaction.Serialize(),
				broadcastedRedemptionTransaction.Serialize(),
			)
		})
	}
}

func TestAssembleRedemptionTransaction(t *testing.T) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()

			err := bitcoinChain.BroadcastTransaction(scenario.InputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			requests := make([]*RedemptionRequest, len(scenario.RedemptionRequests))
			for i, r := range scenario.RedemptionRequests {
				requests[i] = &RedemptionRequest{
					Redeemer:             r.Redeemer,
					RedeemerOutputScript: r.RedeemerOutputScript,
					RequestedAmount:      r.RequestedAmount,
					TreasuryFee:          r.TreasuryFee,
					TxMaxFee:             r.TxMaxFee,
					RequestedAt:          r.RequestedAt,
				}
			}

			feeDistribution := func(requests []*RedemptionRequest) []int64 {
				return scenario.FeeShares
			}

			builder, err := assembleRedemptionTransaction(
				bitcoinChain,
				scenario.WalletPublicKey,
				scenario.WalletMainUtxo,
				requests,
				feeDistribution,
				RedemptionChangeLast,
			)
			if err != nil {
				t.Fatal(err)
			}

			sigHashes, err := builder.ComputeSignatureHashes()
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"sighash count",
				1,
				len(sigHashes),
			)

			testutils.AssertBigIntsEqual(
				t,
				"sighash",
				scenario.ExpectedSigHash,
				sigHashes[0],
			)

			transaction, err := builder.AddSignatures(
				[]*bitcoin.SignatureContainer{scenario.Signature},
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedRedemptionTransaction.Serialize(),
				transaction.Serialize(),
			)
			testutils.AssertStringsEqual(
				t,
				"redemption transaction hash",
				scenario.ExpectedRedemptionTransactionHash.Hex(bitcoin.InternalByteOrder),
				transaction.Hash().Hex(bitcoin.InternalByteOrder),
			)
			testutils.AssertStringsEqual(
				t,
				"redemption transaction witness hash",
				scenario.ExpectedRedemptionTransactionWitnessHash.Hex(bitcoin.InternalByteOrder),
				transaction.WitnessHash().Hex(bitcoin.InternalByteOrder),
			)
		})
	}
}

func TestAssembleRedemptionTransaction_ValidationErrors(t *testing.T) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	scenario := scenarios[0]

	var tests = map[string]struct {
		requestedAmount uint64
		treasuryFee     uint64
		expectedError   error
	}{
		"treasury fee exceeds requested amount": {
			requestedAmount: 10000,
			treasuryFee:     10001,
			expectedError:   fmt.Errorf("treasury fee exceeds requested amount"),
		},
		"redeemable amount exceeds int64 range": {
			requestedAmount: math.MaxUint64,
			treasuryFee:     0,
			expectedError: fmt.Errorf(
				"redeemable amount exceeds int64 range: [%v]",
				uint64(math.MaxUint64),
			),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()

			err := bitcoinChain.BroadcastTransaction(scenario.InputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			requests := []*RedemptionRequest{
				{
					Redeemer:             scenario.RedemptionRequests[0].Redeemer,
					RedeemerOutputScript: scenario.RedemptionRequests[0].RedeemerOutputScript,
					RequestedAmount:      test.requestedAmount,
					TreasuryFee:          test.treasuryFee,
					TxMaxFee:             scenario.RedemptionRequests[0].TxMaxFee,
					RequestedAt:          scenario.RedemptionRequests[0].RequestedAt,
				},
			}

			feeDistribution := func(requests []*RedemptionRequest) []int64 {
				return []int64{0}
			}

			_, err = assembleRedemptionTransaction(
				bitcoinChain,
				scenario.WalletPublicKey,
				scenario.WalletMainUtxo,
				requests,
				feeDistribution,
				RedemptionChangeLast,
			)

			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					err,
				)
			}
		})
	}
}

func TestWithRedemptionTotalFee(t *testing.T) {
	var tests = map[string]struct {
		totalFee          int64
		requestsCount     int
		expectedFeeShares []int64
	}{
		"total fee divisible by the requests count": {
			totalFee:          10000,
			requestsCount:     5,
			expectedFeeShares: []int64{2000, 2000, 2000, 2000, 2000},
		},
		"total fee indivisible by the requests count": {
			totalFee:          10000,
			requestsCount:     6,
			expectedFeeShares: []int64{1666, 1666, 1666, 1666, 1666, 1670},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			requests := make([]*RedemptionRequest, test.requestsCount)

			feeShares := withRedemptionTotalFee(test.totalFee)(requests)

			if diff := deep.Equal(test.expectedFeeShares, feeShares); diff != nil {
				t.Errorf(
					"unexpected fee shares\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.expectedFeeShares,
					feeShares,
				)
			}
		})
	}
}
