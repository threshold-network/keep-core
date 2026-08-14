package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
)

// TODO: Think about covering unhappy paths for specific steps of the moved funds sweep action.
func TestMovedFundsSweepAction_Execute(t *testing.T) {
	scenarios, err := test.LoadMovedFundsSweepTestScenarios()
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

			// Record the transactions that will serve as moved funds sweep
			// transaction's inputs in the Bitcoin local chain.
			for _, transaction := range scenario.InputTransactions {
				err := bitcoinChain.BroadcastTransaction(transaction)
				if err != nil {
					t.Fatal(err)
				}
			}

			// Build the moved funds sweep proposal based on the scenario data.
			proposal := &MovedFundsSweepProposal{
				SweepTxFee:               big.NewInt(scenario.Fee),
				MovingFundsTxHash:        scenario.MovedFundsUtxo.Outpoint.TransactionHash,
				MovingFundsTxOutputIndex: scenario.MovedFundsUtxo.Outpoint.OutputIndex,
			}

			// Choose an arbitrary start block and expiration time.
			proposalProcessingStartBlock := uint64(100)
			proposalExpiryBlock := proposalProcessingStartBlock +
				movedFundsSweepProposalValidityBlocks

			// Simulate the on-chain proposal validation passes with success.
			err = hostChain.setMovedFundsSweepProposalValidationResult(
				walletPublicKeyHash,
				proposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			// Record the wallet main UTXO hash in the local host chain so the
			// moved funds sweep action can detect it.
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

			// The signatures within the scenario fixture are represented as
			// big integer components and need conversion to runtime signature
			// containers used by signing executor.
			rawSignatures := make([]*frost.Signature, len(scenario.Signatures))
			for i, signature := range scenario.Signatures {
				rawSignatures[i] = mustFrostSignatureFromBigInts(
					signature.R,
					signature.S,
				)
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

			action := newMovedFundsSweepAction(
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
			)

			// Modify the default parameters of the action to make
			// it possible to execute in the current test environment.
			action.broadcastCheckDelay = 1 * time.Second

			err = action.execute()
			if err != nil {
				t.Fatal(err)
			}

			// Action execution that completes without an error is a sign of
			// success. However, just in case, make an additional check that
			// the expected moved funds sweep transaction was actually
			// broadcasted on the local Bitcoin chain.
			broadcastedMovedFundsSweepTransaction, err := bitcoinChain.GetTransaction(
				scenario.ExpectedMovedFundsSweepTransactionHash,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedMovedFundsSweepTransaction.Serialize(),
				broadcastedMovedFundsSweepTransaction.Serialize(),
			)
		})
	}
}

func TestAssembleMovedFundsSweepTransaction(t *testing.T) {
	scenarios, err := test.LoadMovedFundsSweepTestScenarios()
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

			walletOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(
				bitcoin.PublicKeyHash(scenario.WalletPublicKey),
			)
			if err != nil {
				t.Fatal(err)
			}

			builder, err := assembleMovedFundsSweepTransaction(
				bitcoinChain,
				scenario.MovedFundsUtxo,
				scenario.WalletMainUtxo,
				walletOutputScript,
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
				scenario.ExpectedMovedFundsSweepTransaction.Serialize(),
				transaction.Serialize(),
			)
			testutils.AssertStringsEqual(
				t,
				"moved funds sweep transaction hash",
				scenario.ExpectedMovedFundsSweepTransactionHash.Hex(bitcoin.InternalByteOrder),
				transaction.Hash().Hex(bitcoin.InternalByteOrder),
			)
			testutils.AssertStringsEqual(
				t,
				"moved funds sweep transaction witness hash",
				scenario.ExpectedMovedFundsSweepTransactionWitnessHash.Hex(bitcoin.InternalByteOrder),
				transaction.WitnessHash().Hex(bitcoin.InternalByteOrder),
			)
		})
	}
}

func TestAssembleMovedFundsSweepTransaction_SupportsTaprootUtxos(
	t *testing.T,
) {
	bitcoinChain := newLocalBitcoinChain()
	walletPublicKey := testWalletPublicKeyFromXOnly(
		t,
		"2336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008",
	)
	walletMainUtxo := testTaprootWalletMainUtxo(
		t,
		bitcoinChain,
		walletPublicKey,
	)

	walletXOnlyPublicKey, err := walletXOnlyPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	walletOutputScript, err := bitcoin.PayToTaproot(walletXOnlyPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	var previousTxHash bitcoin.Hash
	previousTxHash[0] = 0x02
	movingFundsTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: previousTxHash,
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: walletOutputScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(movingFundsTx); err != nil {
		t.Fatal(err)
	}

	builder, err := assembleMovedFundsSweepTransaction(
		bitcoinChain,
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: movingFundsTx.Hash(),
				OutputIndex:     0,
			},
			Value: 100000,
		},
		walletMainUtxo,
		walletOutputScript,
		1000,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !builder.HasOnlyTaprootKeyPathInputs() {
		t.Fatal("expected moved funds sweep builder to use Taproot key-path inputs")
	}
}

func TestAssembleMovedFundsSweepUtxo_RejectsOutOfRangeOutputIndex(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()

	var previousTxHash bitcoin.Hash
	previousTxHash[0] = 0x03

	outputScript, err := bitcoin.PayToWitnessPublicKeyHash(
		hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
	)
	if err != nil {
		t.Fatal(err)
	}

	movingFundsTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: previousTxHash,
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: outputScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(movingFundsTx); err != nil {
		t.Fatal(err)
	}

	_, err = assembleMovedFundsSweepUtxo(
		bitcoinChain,
		movingFundsTx.Hash(),
		1,
	)
	if err == nil {
		t.Fatal("expected out-of-range output index error")
	}
	if !strings.Contains(err.Error(), "output index [1] out of range") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}
