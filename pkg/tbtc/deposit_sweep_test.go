package tbtc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"

	"github.com/keep-network/keep-core/pkg/tecdsa"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
)

// depositSweepVsize returns the virtual size ValidateDepositSweepProposal
// estimates for a sweep proposal carrying the given number of deposits.
func depositSweepVsize(t *testing.T, depositsCount int) int64 {
	t.Helper()

	vsize, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(depositsCount, DepositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}

	return vsize
}

// validateSweepFee runs ValidateDepositSweepProposal for a proposal with no
// deposits and the given fee against a stub chain that accepts every
// proposal on-chain, isolating the follower-side fee gate. It returns the
// warnings that were logged and the validation error.
func validateSweepFee(fee *big.Int) ([]string, error) {
	logger := &capturingLogger{}

	_, err := ValidateDepositSweepProposal(
		logger,
		[20]byte{},
		&DepositSweepProposal{SweepTxFee: fee},
		0,
		false,
		depositSweepFeeCheckChain{},
		newLocalBitcoinChain(),
	)

	return logger.warnings, err
}

// TestDepositSweepValidator_RejectsBelowProtocolMinimum verifies that the
// follower refuses a sweep proposal whose fee is below
// ProtocolMinWalletTxSatPerVByteFee * (vsize - 1), i.e. the fixed protocol
// minimum less the one-vbyte size-estimate allowance. On the base code the
// check only warned, so the proposal validated without error.
func TestDepositSweepValidator_RejectsBelowProtocolMinimum(t *testing.T) {
	vsize := depositSweepVsize(t, 0)
	threshold := ProtocolMinWalletTxSatPerVByteFee * (vsize - 1)

	scenarios := map[string]*big.Int{
		"one satoshi below the threshold": big.NewInt(threshold - 1),
		"one sat/vbyte below the minimum": big.NewInt(
			(ProtocolMinWalletTxSatPerVByteFee - 1) * vsize,
		),
		"relay floor fee": big.NewInt(vsize),
		"zero fee":        big.NewInt(0),
		"nil fee":         nil,
	}

	for name, fee := range scenarios {
		t.Run(name, func(t *testing.T) {
			_, err := validateSweepFee(fee)
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
		})
	}
}

// TestDepositSweepValidator_AcceptsAtProtocolMinimum verifies that fees at the
// reject threshold and at the full protocol minimum for the estimated size
// both validate, so an honest leader's proposal is signed.
func TestDepositSweepValidator_AcceptsAtProtocolMinimum(t *testing.T) {
	vsize := depositSweepVsize(t, 0)

	scenarios := map[string]*big.Int{
		"exactly at the threshold": big.NewInt(
			ProtocolMinWalletTxSatPerVByteFee * (vsize - 1),
		),
		"exactly at the full protocol minimum": big.NewInt(
			ProtocolMinWalletTxSatPerVByteFee * vsize,
		),
	}

	for name, fee := range scenarios {
		t.Run(name, func(t *testing.T) {
			_, err := validateSweepFee(fee)
			if err != nil {
				t.Errorf("expected fee [%v] to be accepted, got: [%v]", fee, err)
			}
		})
	}
}

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
// unconditionally reports the proposal as valid, its ReservationsVaultTrusted
// reports no trusted vault (reservations not live on-chain, so the sweep
// gate allows the sweep), and its other methods are never invoked for a
// proposal with no deposits. This isolates the follower-side sweep-fee soft
// check (deposit_sweep.go, below the "calling chain for proposal validation"
// log line) from on-chain proposal validation and deposit-lookup concerns
// that the soft check does not depend on.
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

func (depositSweepFeeCheckChain) ReservationsVaultTrusted() (bool, error) {
	return false, nil
}

// TestValidateDepositSweepProposal_SweepFeeSoftCheck exercises the
// follower-side soft check on the leader-proposed sweep fee. Fees at or above
// the protocol minimum threshold validate; the warning fires only below the
// operator's bare floor (MinWalletTxSatPerVByteFee * vsize), not below the
// buffered floor, so an honest quiet-mempool proposal that lands exactly on
// the bare floor does not raise a false alarm.
func TestValidateDepositSweepProposal_SweepFeeSoftCheck(t *testing.T) {
	sweepTxSize := depositSweepVsize(t, 0)
	bareFloorFee := MinWalletTxSatPerVByteFee * sweepTxSize
	bufferedRate := (MinWalletTxSatPerVByteFee*(100+WalletTxFeeBufferPercent) +
		99) / 100
	bufferedFloorFee := bufferedRate * sweepTxSize

	scenarios := map[string]struct {
		fee        *big.Int
		expectWarn bool
	}{
		"fee one satoshi below the bare floor": {
			fee:        big.NewInt(bareFloorFee - 1),
			expectWarn: true,
		},
		"fee at the bare floor (honest quiet-mempool leader)": {
			fee:        big.NewInt(bareFloorFee),
			expectWarn: false,
		},
		"fee between the bare and the buffered floor": {
			fee:        big.NewInt(bufferedFloorFee - 1),
			expectWarn: false,
		},
		"fee at the buffered floor": {
			fee:        big.NewInt(bufferedFloorFee),
			expectWarn: false,
		},
		"fee above the buffered floor": {
			fee:        big.NewInt(bufferedFloorFee + 1000),
			expectWarn: false,
		},
	}

	// The bare floor must sit above the protocol minimum threshold, otherwise
	// the below-bare-floor scenario would be rejected instead of warned.
	if bareFloorFee-1 < ProtocolMinWalletTxSatPerVByteFee*(sweepTxSize-1) {
		t.Fatalf(
			"test assumes the default floor [%d] leaves the below-bare-floor "+
				"fee above the protocol minimum threshold",
			MinWalletTxSatPerVByteFee,
		)
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			warnings, err := validateSweepFee(scenario.fee)
			if err != nil {
				t.Fatalf(
					"expected the soft check to never fail validation for "+
						"fee [%v]; got error: [%v]",
					scenario.fee,
					err,
				)
			}

			gotWarn := len(warnings) > 0
			if gotWarn != scenario.expectWarn {
				t.Errorf(
					"unexpected warning presence for fee [%v]\n"+
						"expected warning: %v\nactual warning:   %v\n"+
						"captured warnings: %v",
					scenario.fee,
					scenario.expectWarn,
					gotWarn,
					warnings,
				)
			}
		})
	}
}

// TestValidateDepositSweepProposal_ClosedWindowDoesNotOverRefuse is the
// positive control for the reservations sweep gate: with the client's
// activation
// window closed and the Bridge reporting no trusted reservation vault, a
// sweep validates normally. The gate refuses only on positive evidence that
// reservations are live on-chain.
func TestValidateDepositSweepProposal_ClosedWindowDoesNotOverRefuse(
	t *testing.T,
) {
	fixture := newSweepProposalFixture(t)

	if _, err := ValidateDepositSweepProposal(
		&capturingLogger{},
		fixture.walletPublicKeyHash,
		fixture.proposal,
		0,
		false,
		fixture.chain,
		fixture.bitcoinChain,
	); err != nil {
		t.Fatalf(
			"expected a sweep to validate while reservations are not live "+
				"on-chain, got error: [%v]",
			err,
		)
	}
}

// sweepProposalFixture is a deposit sweep proposal that validates cleanly
// against chain, together with the chains the follower-side tests drive
// ValidateDepositSweepProposal with.
type sweepProposalFixture struct {
	chain               *localChain
	bitcoinChain        *localBitcoinChain
	proposal            *DepositSweepProposal
	walletPublicKeyHash [20]byte
}

// newSweepProposalFixture builds the fixture from the first loaded
// deposit-sweep scenario. Every deposit keeps the vault its scenario
// DepositRevealed event carries while the chain's configured reservation
// vault is a different address: exactly the state a still-reserved deposit
// is left in by a reservation vault rotation.
func newSweepProposalFixture(t *testing.T) *sweepProposalFixture {
	t.Helper()

	scenarios, err := test.LoadDepositSweepTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no deposit sweep test scenarios available")
	}
	scenario := scenarios[0]
	if len(scenario.Deposits) == 0 {
		t.Fatal("scenario has no deposits")
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

	// The chain's reservation vault differs from every deposit's revealed
	// vault, so a vault-address comparison cannot be what decides whether
	// a deposit is reserved.
	hostChain.setReservationParameters(&ReservationParameters{
		ReservationVault: reservationVault,
	})

	return &sweepProposalFixture{
		chain:               hostChain,
		bitcoinChain:        bitcoinChain,
		proposal:            proposal,
		walletPublicKeyHash: walletPublicKeyHash,
	}
}

// TestValidateDepositSweepProposal_RejectsReservedDeposit exercises the
// follower-side defense-in-depth check: a deposit the chain reports as
// reserved must be rejected here even if the leader (running a
// pre-reservations binary, or simply buggy) proposed sweeping it anyway.
//
// The deposit's DepositRevealed event carries a vault that is not the
// chain's configured reservation vault, i.e. the state a still-reserved
// deposit is left in by a reservation vault rotation.
func TestValidateDepositSweepProposal_RejectsReservedDeposit(t *testing.T) {
	fixture := newSweepProposalFixture(t)

	fixture.chain.setReservedDeposit(
		fixture.chain.BuildDepositKey(
			fixture.proposal.DepositsKeys[0].FundingTxHash,
			fixture.proposal.DepositsKeys[0].FundingOutputIndex,
		),
		true,
	)

	_, err := ValidateDepositSweepProposal(
		&capturingLogger{},
		fixture.walletPublicKeyHash,
		fixture.proposal,
		0,
		true,
		fixture.chain,
		fixture.bitcoinChain,
	)
	if err == nil {
		t.Fatal("expected validation to reject a proposal containing a reserved deposit")
	}
	if !strings.Contains(err.Error(), "is a reserved deposit") {
		t.Errorf("expected error to mention the reserved deposit, got: [%v]", err)
	}
}

// reservedCheckErrorChain wraps the package's local chain to make
// IsReservedDeposit fail, modeling a reserved-deposit read the client cannot
// answer while its activation window is open.
type reservedCheckErrorChain struct {
	*localChain

	isReservedErr error
}

func (c *reservedCheckErrorChain) IsReservedDeposit(
	*big.Int,
) (bool, error) {
	return false, c.isReservedErr
}

// TestValidateDepositSweepProposal_FailsClosedOnIsReservedDepositError pins
// the fail-closed posture of the reserved-deposit check: while the client's
// activation window is open, an IsReservedDeposit failure must hard-fail
// proposal validation rather than degrade to "not reserved". Treating an
// unreadable reserved status as unreserved is the hole this check exists to
// close.
func TestValidateDepositSweepProposal_FailsClosedOnIsReservedDepositError(
	t *testing.T,
) {
	fixture := newSweepProposalFixture(t)

	errChain := &reservedCheckErrorChain{
		localChain:    fixture.chain,
		isReservedErr: errors.New("rpc timeout"),
	}

	_, err := ValidateDepositSweepProposal(
		&capturingLogger{},
		fixture.walletPublicKeyHash,
		fixture.proposal,
		0,
		true,
		errChain,
		fixture.bitcoinChain,
	)
	if err == nil {
		t.Fatal(
			"expected validation to fail closed when IsReservedDeposit " +
				"errors",
		)
	}
	if !strings.Contains(err.Error(), "reservation status") {
		t.Errorf(
			"expected error to mention the reservation status read, got: [%v]",
			err,
		)
	}
}

// TestValidateDepositSweepProposal_RefusesWhenReservationsLiveOnChainButInactive
// exercises the reservations sweep gate end to end: a client whose compiled
// activation window for the network is still closed must not validate a
// sweep proposal while the Bridge reports the reservation vault as trusted,
// because the reserved-deposit checks are only consulted inside the window.
func TestValidateDepositSweepProposal_RefusesWhenReservationsLiveOnChainButInactive(
	t *testing.T,
) {
	fixture := newSweepProposalFixture(t)
	fixture.chain.setReservationsVaultTrusted(true)

	_, err := ValidateDepositSweepProposal(
		&capturingLogger{},
		fixture.walletPublicKeyHash,
		fixture.proposal,
		0,
		false,
		fixture.chain,
		fixture.bitcoinChain,
	)
	if err == nil {
		t.Fatal(
			"expected validation to refuse a sweep while reservations are " +
				"live on-chain and inactive in this client",
		)
	}
	if !strings.Contains(err.Error(), "reservations") {
		t.Errorf("expected error to mention reservations, got: [%v]", err)
	}
}

// TestValidateDepositSweepProposal_PreM1BridgeStillSweeps covers the
// ordering hazard the M1 release plan carries: a client cut over before the
// Bridge upgrade must keep sweeping. Such a Bridge carries neither
// ReservationsVaultTrusted nor IsReservedDeposit, so both calls fail while
// the client's activation window is still closed; the sweep must proceed
// rather than halt protocol-wide.
func TestValidateDepositSweepProposal_PreM1BridgeStillSweeps(t *testing.T) {
	fixture := newSweepProposalFixture(t)

	preM1Err := errors.New("execution reverted: no such method")
	fixture.chain.setReservationsVaultTrustedErr(preM1Err)

	preM1Chain := &reservedCheckErrorChain{
		localChain:    fixture.chain,
		isReservedErr: preM1Err,
	}

	validated, err := ValidateDepositSweepProposal(
		&capturingLogger{},
		fixture.walletPublicKeyHash,
		fixture.proposal,
		0,
		false,
		preM1Chain,
		fixture.bitcoinChain,
	)
	if err != nil {
		t.Fatalf(
			"expected the sweep to proceed against a pre-reservations "+
				"Bridge, got error: [%v]",
			err,
		)
	}
	if len(validated) != len(fixture.proposal.DepositsKeys) {
		t.Errorf(
			"expected all %d deposits to validate, got %d",
			len(fixture.proposal.DepositsKeys),
			len(validated),
		)
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
