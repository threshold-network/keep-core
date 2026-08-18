package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tecdsa"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
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
	// floor is derived rather than hardcoded.
	sweepTxSize, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(0, DepositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}
	minSweepTxFee := big.NewInt(int64(MinSweepTxSatPerVByteFee) * sweepTxSize)

	scenarios := map[string]struct {
		fee        *big.Int
		expectWarn bool
	}{
		"fee below the safe minimum": {
			fee:        new(big.Int).Sub(minSweepTxFee, big.NewInt(1)),
			expectWarn: true,
		},
		"fee at the safe minimum": {
			fee:        minSweepTxFee,
			expectWarn: false,
		},
		"fee above the safe minimum": {
			fee:        new(big.Int).Add(minSweepTxFee, big.NewInt(1000)),
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

			_, err := ValidateDepositSweepProposal(
				logger,
				walletPublicKeyHash,
				proposal,
				0,
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
