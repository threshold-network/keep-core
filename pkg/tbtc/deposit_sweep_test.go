package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/keep-network/keep-core/pkg/tecdsa"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
)

// TODO: Think about covering unhappy paths for specific steps of the deposit sweep action.
func TestDepositSweepAction_Execute(t *testing.T) {
	scenarios, err := test.LoadDepositSweepTestScenarios()
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

			// Record the transactions that will serve as sweep transaction's
			// input in the Bitcoin local chain.
			for _, transaction := range scenario.InputTransactions {
				err := bitcoinChain.BroadcastTransaction(transaction)
				if err != nil {
					t.Fatal(err)
				}
			}

			// depositsKeys will be needed to build the proposal instance.
			depositsKeys := make([]DepositKey, len(scenario.Deposits))

			// depositsExtraInfo will be needed to perform on-chain proposal
			// validation.
			depositsExtraInfo := make([]struct {
				*Deposit
				FundingTx *bitcoin.Transaction
			}, len(scenario.Deposits))

			depositsRevealBlocks := make([]*big.Int, len(scenario.Deposits))

			// Record all necessary deposits' data on the local host chain.
			for i, deposit := range scenario.Deposits {
				fundingTxHash := deposit.Utxo.Outpoint.TransactionHash
				fundingOutputIndex := deposit.Utxo.Outpoint.OutputIndex

				fundingTx, err := bitcoinChain.GetTransaction(fundingTxHash)
				if err != nil {
					t.Fatal(err)
				}

				depositsKeys[i] = DepositKey{
					FundingTxHash:      fundingTxHash,
					FundingOutputIndex: fundingOutputIndex,
				}

				depositsExtraInfo[i] = struct {
					*Deposit
					FundingTx *bitcoin.Transaction
				}{
					Deposit:   (*Deposit)(deposit),
					FundingTx: fundingTx,
				}

				// Build the deposit reveal block based on the deposit index.
				// This field can be an arbitrary value, but it is good to keep
				// it consistent.
				depositRevealBlock := uint64(100 * i)
				depositsRevealBlocks[i] = big.NewInt(int64(depositRevealBlock))

				// The deposit sweep action will look for past deposit
				// revealed events using a specific filter. We need to make
				// sure the local host chain will return the expected result
				// for that filter. We need to build the startBlock and endBlock
				// filter's parameters using the depositRevealBlock value
				// as done within the depositSweepAction.execute function.
				// We also need to use the correct wallet PKH.
				err = hostChain.setPastDepositRevealedEvents(
					&DepositRevealedEventFilter{
						StartBlock:          depositRevealBlock,
						EndBlock:            &depositRevealBlock,
						WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
					},
					[]*DepositRevealedEvent{
						{
							FundingTxHash:       fundingTxHash,
							FundingOutputIndex:  fundingOutputIndex,
							Depositor:           deposit.Depositor,
							Amount:              uint64(deposit.Utxo.Value),
							BlindingFactor:      deposit.BlindingFactor,
							WalletPublicKeyHash: deposit.WalletPublicKeyHash,
							RefundPublicKeyHash: deposit.RefundPublicKeyHash,
							RefundLocktime:      deposit.RefundLocktime,
							Vault:               deposit.Vault,
							BlockNumber:         depositRevealBlock,
						},
					},
				)
				if err != nil {
					t.Fatal(err)
				}

				hostChain.setDepositRequest(
					fundingTxHash,
					fundingOutputIndex,
					&DepositChainRequest{
						// Set only relevant fields.
						Depositor: deposit.Depositor,
						Amount:    uint64(deposit.Utxo.Value),
						Vault:     deposit.Vault,
						ExtraData: deposit.ExtraData,
					},
				)
			}

			// Build the sweep proposal based on the scenario data.
			proposal := &DepositSweepProposal{
				DepositsKeys:         depositsKeys,
				SweepTxFee:           big.NewInt(scenario.Fee),
				DepositsRevealBlocks: depositsRevealBlocks,
			}

			// Choose an arbitrary start block and expiration time.
			proposalProcessingStartBlock := uint64(100)
			proposalExpiryBlock := proposalProcessingStartBlock +
				depositSweepProposalValidityBlocks

			// Simulate the on-chain proposal validation passes with success.
			err = hostChain.setDepositSweepProposalValidationResult(
				walletPublicKeyHash,
				proposal,
				depositsExtraInfo,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			// Record the wallet main UTXO hash in the local host chain so
			// the deposit action can detect it.
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

			// The signatures within the scenario fixture are in the format
			// suitable for applying them directly to a Bitcoin transaction.
			// However, the signing executor operates on raw tECDSA signatures
			// so, we need to unpack them first.
			rawSignatures := make([]*tecdsa.Signature, len(scenario.Signatures))
			for i, signature := range scenario.Signatures {
				rawSignatures[i] = &tecdsa.Signature{
					R: signature.R,
					S: signature.S,
				}
			}

			// Set up the signing executor mock to return the signatures from
			// the test fixture when called with the expected parameters.
			// Note that the start block is set based on the proposal
			// processing start block as done within the action.
			signingExecutor.setSignatures(
				scenario.ExpectedSigHashes,
				proposalProcessingStartBlock,
				rawSignatures,
			)

			action := newDepositSweepAction(
				logger.With(),
				hostChain,
				bitcoinChain,
				// Mainnet has no reservationsActivationBlocks entry, so
				// ReservationsActivationBlock returns math.MaxUint64 and
				// reservationsActive is false for this scenario suite,
				// which never configures reservation state - matching
				// this test's pre-launch, reservations-inactive intent.
				ethereum.Mainnet,
				wallet,
				signingExecutor,
				proposal,
				proposalProcessingStartBlock,
				proposalExpiryBlock,
				func(ctx context.Context, blockHeight uint64) error {
					return nil
				},
				nil,
				0,
			)

			// Modify the default parameters of the action to make
			// it possible to execute in the current test environment.
			action.requiredFundingTxConfirmations = 1
			action.broadcastCheckDelay = 1 * time.Second

			err := action.execute()
			if err != nil {
				t.Fatal(err)
			}

			// Action execution that completes without an error is a sign of
			// success. However, just in case, make an additional check that
			// the expected sweep transaction was actually broadcasted on the
			// local Bitcoin chain.
			broadcastedSweepTransaction, err := bitcoinChain.GetTransaction(
				scenario.ExpectedSweepTransactionHash,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedSweepTransaction.Serialize(),
				broadcastedSweepTransaction.Serialize(),
			)
		})
	}
}

func TestAssembleDepositSweepTransaction(t *testing.T) {
	scenarios, err := test.LoadDepositSweepTestScenarios()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()

			for _, transaction := range scenario.InputTransactions {
				err := bitcoinChain.BroadcastTransaction(transaction)
				if err != nil {
					t.Fatal(err)
				}
			}

			deposits := make([]*Deposit, len(scenario.Deposits))
			for i, d := range scenario.Deposits {
				deposits[i] = &Deposit{
					Utxo:                d.Utxo,
					Depositor:           d.Depositor,
					BlindingFactor:      d.BlindingFactor,
					WalletPublicKeyHash: d.WalletPublicKeyHash,
					RefundPublicKeyHash: d.RefundPublicKeyHash,
					RefundLocktime:      d.RefundLocktime,
					Vault:               d.Vault,
					ExtraData:           d.ExtraData,
				}
			}

			builder, err := assembleDepositSweepTransaction(
				bitcoinChain,
				scenario.WalletPublicKey,
				scenario.WalletMainUtxo,
				deposits,
				scenario.Fee,
			)
			if err != nil {
				t.Fatal(err)
			}

			sigHashes, err := builder.ComputeSignatureHashes()
			if err != nil {
				t.Fatal(err)
			}

			for i, sigHash := range sigHashes {
				testutils.AssertBigIntsEqual(
					t,
					fmt.Sprintf("sighash for input [%v]", i),
					scenario.ExpectedSigHashes[i],
					sigHash,
				)
			}

			transaction, err := builder.AddSignatures(scenario.Signatures)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedSweepTransaction.Serialize(),
				transaction.Serialize(),
			)
			testutils.AssertStringsEqual(
				t,
				"sweep transaction hash",
				scenario.ExpectedSweepTransactionHash.Hex(bitcoin.InternalByteOrder),
				transaction.Hash().Hex(bitcoin.InternalByteOrder),
			)
			testutils.AssertStringsEqual(
				t,
				"sweep transaction witness hash",
				scenario.ExpectedSweepTransactionWitnessHash.Hex(bitcoin.InternalByteOrder),
				transaction.WitnessHash().Hex(bitcoin.InternalByteOrder),
			)
		})
	}
}

// capturingLogger wraps testutils.MockLogger and records Warnf calls for
// assertions.
type capturingLogger struct {
	testutils.MockLogger
	warnings []string
}

func (cl *capturingLogger) Warnf(format string, args ...interface{}) {
	cl.warnings = append(cl.warnings, fmt.Sprintf(format, args...))
}

// depositSweepFeeCheckChain is a minimal stub satisfying the chain interface
// ValidateDepositSweepProposal requires. Its ValidateDepositSweepProposal
// unconditionally reports the proposal as valid, and its other two methods
// are never invoked for a proposal with no deposits. This isolates the
// follower-side sweep-fee soft check (deposit_sweep.go, below the
// "calling chain for proposal validation" log line) from on-chain proposal
// validation and deposit-lookup concerns that the soft check does not
// depend on.
type depositSweepFeeCheckChain struct{}

func (depositSweepFeeCheckChain) PastDepositRevealedEvents(
	*DepositRevealedEventFilter,
) ([]*DepositRevealedEvent, error) {
	return nil, nil
}

func (depositSweepFeeCheckChain) ValidateDepositSweepProposal(
	[20]byte,
	*DepositSweepProposal,
	[]struct {
		*Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	return nil
}

func (depositSweepFeeCheckChain) GetDepositRequest(
	bitcoin.Hash,
	uint32,
) (*DepositChainRequest, bool, error) {
	return nil, false, nil
}

func (depositSweepFeeCheckChain) BuildDepositKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) *big.Int {
	return new(big.Int)
}

func (depositSweepFeeCheckChain) IsReservedDeposit(*big.Int) (bool, error) {
	return false, nil
}

func (depositSweepFeeCheckChain) ReservationParameters() (*ReservationParameters, error) {
	return nil, fmt.Errorf("reservation parameters not configured in this stub")
}

// TestValidateDepositSweepProposal_SweepFeeSoftCheck exercises the
// follower-side soft check on the leader-proposed sweep fee. The check is
// log-only by design (see threshold-network/keep-core#4171): it must warn
// about an unsafe fee but must never fail proposal validation because of it.
func TestValidateDepositSweepProposal_SweepFeeSoftCheck(t *testing.T) {
	var walletPublicKeyHash [20]byte
	stubChain := depositSweepFeeCheckChain{}
	btcChain := newLocalBitcoinChain()

	// Compute the exact safe-minimum fee for a proposal with no deposits
	// using the same estimator call the soft check itself performs
	// (deposit_sweep.go), so the boundary between "below" and "at/above" the
	// floor is derived rather than hardcoded. The floor is the buffered
	// minimum that warnIfProposedWalletTxFeeBelowBufferedFloor
	// (proposal_fee_check.go) recomputes from the bare sweep floor using the
	// WalletTxFeeBufferPercent mirror (the
	// 25% safety buffer that tbtcpg.applyWalletTxFeeFloor also reapplies on
	// the leader side). Keeping the formula here in sync with the helper is
	// exactly the property this test exercises.
	sweepTxSize, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(0, DepositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}
	bufferedRate := (MinWalletTxSatPerVByteFee*(100+WalletTxFeeBufferPercent) +
		99) / 100
	minBufferedSweepTxFee := big.NewInt(int64(bufferedRate) * sweepTxSize)

	scenarios := map[string]struct {
		fee        *big.Int
		expectWarn bool
	}{
		"fee below the safe buffered minimum": {
			fee:        new(big.Int).Sub(minBufferedSweepTxFee, big.NewInt(1)),
			expectWarn: true,
		},
		"fee at the safe buffered minimum": {
			fee:        minBufferedSweepTxFee,
			expectWarn: false,
		},
		"fee above the safe buffered minimum": {
			fee:        new(big.Int).Add(minBufferedSweepTxFee, big.NewInt(1000)),
			expectWarn: false,
		},
		// A nil SweepTxFee cannot occur on the real production path (see the
		// comment on the nil case in deposit_sweep.go): the on-chain
		// WalletProposalValidator call a few lines above the soft check
		// already ABI-packs the fee and panics first, and wire
		// deserialization always constructs a non-nil value. This scenario
		// exists to lock in the defense-in-depth behavior for callers, like
		// this test's stub chain, that can hand the soft check a nil fee
		// directly.
		"nil fee from a test/mock caller": {
			fee:        nil,
			expectWarn: true,
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			proposal := &DepositSweepProposal{
				SweepTxFee: scenario.fee,
			}

			logger := &capturingLogger{}

			// This test only exercises the fee soft check; reservations
			// are irrelevant here, so keep them inactive to avoid the
			// stub chain's always-erroring ReservationParameters.
			_, err := ValidateDepositSweepProposal(
				logger,
				walletPublicKeyHash,
				proposal,
				0,
				false,
				stubChain,
				btcChain,
			)
			if err != nil {
				t.Fatalf(
					"expected the log-only soft check to never fail "+
						"validation; got error: [%v]",
					err,
				)
			}

			gotWarn := len(logger.warnings) > 0
			if gotWarn != scenario.expectWarn {
				t.Errorf(
					"unexpected warning presence for fee [%v]\n"+
						"expected warning: %v\nactual warning:   %v\n"+
						"captured warnings: %v",
					scenario.fee,
					scenario.expectWarn,
					gotWarn,
					logger.warnings,
				)
			}
		})
	}
}

// TestValidateDepositSweepProposal_FailsClosedOnReservationParametersError
// exercises the fail-closed posture required by reservationsActive's doc
// comment: when reservationsActive is true, a ReservationParameters
// failure must hard-fail proposal validation, not silently degrade to a
// reservations-inactive posture. This is the mirror image of
// TestValidateDepositSweepProposal_SweepFeeSoftCheck above, which
// deliberately passes reservationsActive=false to dodge this same stub
// chain's always-erroring ReservationParameters.
func TestValidateDepositSweepProposal_FailsClosedOnReservationParametersError(t *testing.T) {
	var walletPublicKeyHash [20]byte
	stubChain := depositSweepFeeCheckChain{}
	btcChain := newLocalBitcoinChain()

	proposal := &DepositSweepProposal{
		SweepTxFee: big.NewInt(1),
	}

	logger := &capturingLogger{}

	_, err := ValidateDepositSweepProposal(
		logger,
		walletPublicKeyHash,
		proposal,
		0,
		true,
		stubChain,
		btcChain,
	)
	if err == nil {
		t.Fatal(
			"expected validation to fail closed when reservationsActive " +
				"is true and ReservationParameters errors",
		)
	}
	if !strings.Contains(err.Error(), "reservation parameters") {
		t.Errorf(
			"expected error to mention reservation parameters, got: [%v]",
			err,
		)
	}
}

// TestValidateDepositSweepProposal_RejectsReservedDeposit exercises the
// follower-side defense-in-depth check: a deposit revealed against the
// reservation vault must be rejected here even if the leader (running a
// pre-reservations binary, or simply buggy) proposed sweeping it anyway.
func TestValidateDepositSweepProposal_RejectsReservedDeposit(t *testing.T) {
	scenarios, err := test.LoadDepositSweepTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no deposit sweep test scenarios available")
	}
	scenario := scenarios[0]
	if len(scenario.Deposits) == 0 {
		t.Fatal("scenario has no deposits to reserve")
	}

	hostChain := Connect()
	bitcoinChain := newLocalBitcoinChain()

	wallet := wallet{publicKey: scenario.WalletPublicKey}
	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

	for _, transaction := range scenario.InputTransactions {
		if err := bitcoinChain.BroadcastTransaction(transaction); err != nil {
			t.Fatal(err)
		}
	}

	depositsKeys := make([]DepositKey, len(scenario.Deposits))
	depositsExtraInfo := make([]struct {
		*Deposit
		FundingTx *bitcoin.Transaction
	}, len(scenario.Deposits))
	depositsRevealBlocks := make([]*big.Int, len(scenario.Deposits))

	reservationVault := chain.Address("0xReservationVaultForFollowerTest0000001")

	for i, deposit := range scenario.Deposits {
		fundingTxHash := deposit.Utxo.Outpoint.TransactionHash
		fundingOutputIndex := deposit.Utxo.Outpoint.OutputIndex

		fundingTx, err := bitcoinChain.GetTransaction(fundingTxHash)
		if err != nil {
			t.Fatal(err)
		}

		depositsKeys[i] = DepositKey{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
		}
		depositsExtraInfo[i] = struct {
			*Deposit
			FundingTx *bitcoin.Transaction
		}{
			Deposit:   (*Deposit)(deposit),
			FundingTx: fundingTx,
		}

		depositRevealBlock := uint64(100 * i)
		depositsRevealBlocks[i] = big.NewInt(int64(depositRevealBlock))

		eventVault := deposit.Vault
		if i == 0 {
			// The first deposit's vault is overridden to match the
			// reservation vault configured below, so the pre-filter
			// lets it reach the IsReservedDeposit check this test
			// exercises. Other deposits keep their scenario-fixture
			// vault, which does not match, proving they sweep
			// unaffected.
			eventVault = &reservationVault
		}

		err = hostChain.setPastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				StartBlock:          depositRevealBlock,
				EndBlock:            &depositRevealBlock,
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
			},
			[]*DepositRevealedEvent{
				{
					FundingTxHash:       fundingTxHash,
					FundingOutputIndex:  fundingOutputIndex,
					Depositor:           deposit.Depositor,
					Amount:              uint64(deposit.Utxo.Value),
					BlindingFactor:      deposit.BlindingFactor,
					WalletPublicKeyHash: deposit.WalletPublicKeyHash,
					RefundPublicKeyHash: deposit.RefundPublicKeyHash,
					RefundLocktime:      deposit.RefundLocktime,
					Vault:               eventVault,
					BlockNumber:         depositRevealBlock,
				},
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		hostChain.setDepositRequest(
			fundingTxHash,
			fundingOutputIndex,
			&DepositChainRequest{
				Depositor: deposit.Depositor,
				Amount:    uint64(deposit.Utxo.Value),
				Vault:     deposit.Vault,
				ExtraData: deposit.ExtraData,
			},
		)
	}

	proposal := &DepositSweepProposal{
		DepositsKeys:         depositsKeys,
		SweepTxFee:           big.NewInt(scenario.Fee),
		DepositsRevealBlocks: depositsRevealBlocks,
	}

	if err := hostChain.setDepositSweepProposalValidationResult(
		walletPublicKeyHash,
		proposal,
		depositsExtraInfo,
		true,
	); err != nil {
		t.Fatal(err)
	}

	// Mark the first deposit's key as reserved. Everything else about the
	// proposal remains identical to a scenario the existing
	// TestDepositSweepAction_Execute suite already proves validates and
	// sweeps cleanly when nothing is reserved.
	reservedKey := hostChain.BuildDepositKey(
		depositsKeys[0].FundingTxHash,
		depositsKeys[0].FundingOutputIndex,
	)
	hostChain.setReservedDeposit(reservedKey, true)
	hostChain.setReservationParameters(&ReservationParameters{
		ReservationVault: reservationVault,
	})

	_, err = ValidateDepositSweepProposal(
		&capturingLogger{},
		walletPublicKeyHash,
		proposal,
		0,
		true,
		hostChain,
		bitcoinChain,
	)
	if err == nil {
		t.Fatal("expected validation to reject a proposal containing a reserved deposit")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("expected error to mention the reserved deposit, got: [%v]", err)
	}
}

// TestDepositSweepAction_Execute_ReservationsActiveFromCoordinationBlock
// exercises depositSweepAction.execute()'s own reservationsActive
// computation end-to-end - never a hardcoded bool downstream of it, unlike
// TestValidateDepositSweepProposal_RejectsReservedDeposit above, which
// stubs reservationsActive as a literal true and therefore could not have
// caught a bug in how execute() derives that value.
//
// Both subtests below sweep a deposit revealed against a reservation
// vault and rely on execute() to reject it whenever - and only whenever -
// reservations are active at dsa.coordinationBlock, the same block height
// coordinationExecutor.executeLeaderRoutine (coordination.go) uses to
// populate CoordinationProposalRequest.ReservationsActive for the
// leader's own proposal:
//
//   - "mismatch window, follower agrees with leader" configures an
//     activation block strictly between coordinationBlock and
//     proposalProcessingStartBlock (the coordination window's end block,
//     always coordinationDurationBlocks after coordinationBlock - see
//     coordinationWindow.endBlock). Before this fix, execute() derived
//     reservationsActive from proposalProcessingStartBlock directly and
//     would have wrongly computed it as active here, diverging from the
//     leader's coordinationBlock-based "not yet active" determination for
//     the exact same round, and rejecting a reservation the leader itself
//     considered safe to sweep.
//   - "developer network, already active" uses ethereum.Developer, whose
//     ReservationsActivationBlock is always 0, so reservations are active
//     at both coordinationBlock and proposalProcessingStartBlock. This
//     proves the reserved-deposit filter actually engages through
//     execute()'s real wiring, not merely when reservationsActive is
//     asserted directly against ValidateDepositSweepProposal.
func TestDepositSweepAction_Execute_ReservationsActiveFromCoordinationBlock(t *testing.T) {
	scenarios, err := test.LoadDepositSweepTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no deposit sweep test scenarios available")
	}
	scenario := scenarios[0]
	if len(scenario.Deposits) == 0 {
		t.Fatal("scenario has no deposits to reserve")
	}

	tests := map[string]struct {
		network                      ethereum.Network
		proposalProcessingStartBlock uint64
		// activationBlock is ignored for ethereum.Developer, which always
		// activates at block 0 regardless of reservationsActivationBlocks.
		activationBlock uint64
		expectRejected  bool
	}{
		"mismatch window, follower agrees with leader": {
			network: ethereum.Sepolia,
			// coordinationBlock = 1100 - coordinationDurationBlocks = 1000.
			proposalProcessingStartBlock: 1100,
			activationBlock:              1050,
			expectRejected:               false,
		},
		"developer network, already active": {
			network: ethereum.Developer,
			// coordinationBlock = 500 - coordinationDurationBlocks = 400.
			proposalProcessingStartBlock: 500,
			activationBlock:              0,
			expectRejected:               true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			coordinationBlock := tc.proposalProcessingStartBlock - coordinationDurationBlocks

			if tc.network != ethereum.Developer {
				reservationsActivationBlocks[tc.network] = tc.activationBlock
				t.Cleanup(func() { delete(reservationsActivationBlocks, tc.network) })
			}

			hostChain := Connect()
			bitcoinChain := newLocalBitcoinChain()

			wallet := wallet{publicKey: scenario.WalletPublicKey}
			walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

			for _, transaction := range scenario.InputTransactions {
				if err := bitcoinChain.BroadcastTransaction(transaction); err != nil {
					t.Fatal(err)
				}
			}

			depositsKeys := make([]DepositKey, len(scenario.Deposits))
			depositsExtraInfo := make([]struct {
				*Deposit
				FundingTx *bitcoin.Transaction
			}, len(scenario.Deposits))
			depositsRevealBlocks := make([]*big.Int, len(scenario.Deposits))

			reservationVault := chain.Address("0xReservationVaultForExecuteTest00000001")

			for i, deposit := range scenario.Deposits {
				fundingTxHash := deposit.Utxo.Outpoint.TransactionHash
				fundingOutputIndex := deposit.Utxo.Outpoint.OutputIndex

				fundingTx, err := bitcoinChain.GetTransaction(fundingTxHash)
				if err != nil {
					t.Fatal(err)
				}

				depositsKeys[i] = DepositKey{
					FundingTxHash:      fundingTxHash,
					FundingOutputIndex: fundingOutputIndex,
				}
				depositsExtraInfo[i] = struct {
					*Deposit
					FundingTx *bitcoin.Transaction
				}{
					Deposit:   (*Deposit)(deposit),
					FundingTx: fundingTx,
				}

				depositRevealBlock := uint64(100 * i)
				depositsRevealBlocks[i] = big.NewInt(int64(depositRevealBlock))

				eventVault := deposit.Vault
				if i == 0 {
					// The first deposit's vault is overridden to match the
					// reservation vault configured below, so the pre-filter
					// lets it reach the IsReservedDeposit check this test
					// exercises.
					eventVault = &reservationVault
				}

				err = hostChain.setPastDepositRevealedEvents(
					&DepositRevealedEventFilter{
						StartBlock:          depositRevealBlock,
						EndBlock:            &depositRevealBlock,
						WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
					},
					[]*DepositRevealedEvent{
						{
							FundingTxHash:       fundingTxHash,
							FundingOutputIndex:  fundingOutputIndex,
							Depositor:           deposit.Depositor,
							Amount:              uint64(deposit.Utxo.Value),
							BlindingFactor:      deposit.BlindingFactor,
							WalletPublicKeyHash: deposit.WalletPublicKeyHash,
							RefundPublicKeyHash: deposit.RefundPublicKeyHash,
							RefundLocktime:      deposit.RefundLocktime,
							Vault:               eventVault,
							BlockNumber:         depositRevealBlock,
						},
					},
				)
				if err != nil {
					t.Fatal(err)
				}

				hostChain.setDepositRequest(
					fundingTxHash,
					fundingOutputIndex,
					&DepositChainRequest{
						Depositor: deposit.Depositor,
						Amount:    uint64(deposit.Utxo.Value),
						Vault:     deposit.Vault,
						ExtraData: deposit.ExtraData,
					},
				)
			}

			proposal := &DepositSweepProposal{
				DepositsKeys:         depositsKeys,
				SweepTxFee:           big.NewInt(scenario.Fee),
				DepositsRevealBlocks: depositsRevealBlocks,
			}

			if err := hostChain.setDepositSweepProposalValidationResult(
				walletPublicKeyHash,
				proposal,
				depositsExtraInfo,
				true,
			); err != nil {
				t.Fatal(err)
			}

			reservedKey := hostChain.BuildDepositKey(
				depositsKeys[0].FundingTxHash,
				depositsKeys[0].FundingOutputIndex,
			)
			hostChain.setReservedDeposit(reservedKey, true)
			hostChain.setReservationParameters(&ReservationParameters{
				ReservationVault: reservationVault,
			})

			var walletMainUtxoHash [32]byte
			if scenario.WalletMainUtxo != nil {
				walletMainUtxoHash = hostChain.ComputeMainUtxoHash(scenario.WalletMainUtxo)
			}
			hostChain.setWallet(walletPublicKeyHash, &WalletChainData{
				MainUtxoHash: walletMainUtxoHash,
			})

			signingExecutor := newMockWalletSigningExecutor()
			rawSignatures := make([]*tecdsa.Signature, len(scenario.Signatures))
			for i, signature := range scenario.Signatures {
				rawSignatures[i] = &tecdsa.Signature{
					R: signature.R,
					S: signature.S,
				}
			}
			signingExecutor.setSignatures(
				scenario.ExpectedSigHashes,
				tc.proposalProcessingStartBlock,
				rawSignatures,
			)

			action := newDepositSweepAction(
				logger.With(),
				hostChain,
				bitcoinChain,
				tc.network,
				wallet,
				signingExecutor,
				proposal,
				tc.proposalProcessingStartBlock,
				tc.proposalProcessingStartBlock+depositSweepProposalValidityBlocks,
				func(ctx context.Context, blockHeight uint64) error {
					return nil
				},
				nil,
				coordinationBlock,
			)
			action.requiredFundingTxConfirmations = 1
			action.broadcastCheckDelay = 1 * time.Second

			err = action.execute()

			if tc.expectRejected {
				if err == nil {
					t.Fatal("expected execute to reject the reserved deposit")
				}
				if !strings.Contains(err.Error(), "reserved") {
					t.Errorf("expected error to mention the reserved deposit, got: [%v]", err)
				}
				return
			}

			if err != nil {
				t.Fatalf(
					"expected execute to succeed (reservationsActive false "+
						"at coordinationBlock [%v], matching the leader's own "+
						"determination), got: [%v]",
					coordinationBlock,
					err,
				)
			}

			broadcastedSweepTransaction, err := bitcoinChain.GetTransaction(
				scenario.ExpectedSweepTransactionHash,
			)
			if err != nil {
				t.Fatal(err)
			}
			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedSweepTransaction.Serialize(),
				broadcastedSweepTransaction.Serialize(),
			)
		})
	}
}
