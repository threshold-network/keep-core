package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost"
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
			depositsKeys := make([]struct {
				FundingTxHash      bitcoin.Hash
				FundingOutputIndex uint32
			}, len(scenario.Deposits))

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

				depositsKeys[i] = struct {
					FundingTxHash      bitcoin.Hash
					FundingOutputIndex uint32
				}{
					FundingTxHash:      fundingTxHash,
					FundingOutputIndex: fundingOutputIndex,
				}

				depositsExtraInfo[i] = struct {
					*Deposit
					FundingTx *bitcoin.Transaction
				}{
					Deposit: &Deposit{
						Utxo:                deposit.Utxo,
						Depositor:           deposit.Depositor,
						BlindingFactor:      deposit.BlindingFactor,
						WalletPublicKeyHash: deposit.WalletPublicKeyHash,
						RefundPublicKeyHash: deposit.RefundPublicKeyHash,
						RefundLocktime:      deposit.RefundLocktime,
						Vault:               deposit.Vault,
						ExtraData:           deposit.ExtraData,
					},
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

func TestAssembleDepositSweepTransaction_TaprootDeposit(t *testing.T) {
	hexToSlice := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatalf("error while converting [%v]: [%v]", hexString, err)
		}
		return bytes
	}

	var walletXOnlyPublicKey [32]byte
	copy(
		walletXOnlyPublicKey[:],
		hexToSlice("2336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008"),
	)

	compressedWalletPublicKey := append([]byte{0x02}, walletXOnlyPublicKey[:]...)
	parsedWalletPublicKey, err := btcec.ParsePubKey(
		compressedWalletPublicKey,
		btcec.S256(),
	)
	if err != nil {
		t.Fatal(err)
	}
	walletPublicKey := &ecdsa.PublicKey{
		Curve: btcec.S256(),
		X:     parsedWalletPublicKey.X,
		Y:     parsedWalletPublicKey.Y,
	}

	var refundXOnlyPublicKey [32]byte
	copy(
		refundXOnlyPublicKey[:],
		hexToSlice("11223344556677889900aabbccddeeff00112233445566778899aabbccddeeff"),
	)

	deposit := &Deposit{
		Depositor:            chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
		WalletXOnlyPublicKey: &walletXOnlyPublicKey,
		RefundXOnlyPublicKey: &refundXOnlyPublicKey,
	}
	copy(deposit.BlindingFactor[:], hexToSlice("f9f0c90d00039523"))
	copy(deposit.WalletPublicKeyHash[:], hexToSlice("c92a772f11bc97d8938a16a9db435401f4e6a7bc"))
	copy(deposit.RefundPublicKeyHash[:], hexToSlice("c2a27a88d8d03e271e8edc556923e9398619f17c"))
	copy(deposit.RefundLocktime[:], hexToSlice("60bcea61"))

	merkleRoot, err := deposit.TaprootMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputScript, err := bitcoin.PayToTaprootWithScriptTree(
		walletXOnlyPublicKey,
		merkleRoot,
	)
	if err != nil {
		t.Fatal(err)
	}

	var previousTxHash bitcoin.Hash
	copy(previousTxHash[:], hexToSlice("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"))
	fundingTx := &bitcoin.Transaction{
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
				PublicKeyScript: fundingOutputScript,
			},
		},
	}

	bitcoinChain := newLocalBitcoinChain()
	if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}

	deposit.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	builder, err := assembleDepositSweepTransaction(
		bitcoinChain,
		walletPublicKey,
		nil,
		[]*Deposit{deposit},
		1000,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !builder.HasOnlyTaprootKeyPathInputs() {
		t.Fatal("expected only Taproot key-path inputs")
	}

	merkleRoots := builder.TaprootKeyPathInputMerkleRoots()
	if len(merkleRoots) != 1 || merkleRoots[0] == nil {
		t.Fatalf("expected one Taproot merkle root")
	}
	testutils.AssertBytesEqual(t, merkleRoot[:], merkleRoots[0][:])

	unsignedTx := builder.UnsignedTransaction()
	if len(unsignedTx.Outputs) != 1 {
		t.Fatalf("unexpected outputs count: [%v]", len(unsignedTx.Outputs))
	}

	expectedWalletOutputScript, err := bitcoin.PayToTaproot(walletXOnlyPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBytesEqual(
		t,
		expectedWalletOutputScript,
		unsignedTx.Outputs[0].PublicKeyScript,
	)
	testutils.AssertIntsEqual(
		t,
		"output value",
		99000,
		int(unsignedTx.Outputs[0].Value),
	)
}
