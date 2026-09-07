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
				nil,
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

			builder, err := assembleMovedFundsSweepTransaction(
				bitcoinChain,
				scenario.WalletPublicKey,
				scenario.MovedFundsUtxo,
				scenario.WalletMainUtxo,
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
