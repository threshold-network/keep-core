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

	depositOne := &Deposit{
		Depositor:            chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
		WalletXOnlyPublicKey: &walletXOnlyPublicKey,
		RefundXOnlyPublicKey: &refundXOnlyPublicKey,
	}
	copy(depositOne.BlindingFactor[:], hexToSlice("f9f0c90d00039523"))
	copy(depositOne.WalletPublicKeyHash[:], hexToSlice("c92a772f11bc97d8938a16a9db435401f4e6a7bc"))
	copy(depositOne.RefundPublicKeyHash[:], hexToSlice("c2a27a88d8d03e271e8edc556923e9398619f17c"))
	copy(depositOne.RefundLocktime[:], hexToSlice("60bcea61"))

	merkleRootOne, err := depositOne.TaprootMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputScriptOne, err := bitcoin.PayToTaprootWithScriptTree(
		walletXOnlyPublicKey,
		merkleRootOne,
	)
	if err != nil {
		t.Fatal(err)
	}

	depositTwo := &Deposit{
		Depositor:            chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
		WalletXOnlyPublicKey: &walletXOnlyPublicKey,
		RefundXOnlyPublicKey: &refundXOnlyPublicKey,
	}
	copy(depositTwo.BlindingFactor[:], hexToSlice("f9f0c90d00039523"))
	copy(depositTwo.WalletPublicKeyHash[:], hexToSlice("c92a772f11bc97d8938a16a9db435401f4e6a7bc"))
	copy(depositTwo.RefundPublicKeyHash[:], hexToSlice("c2a27a88d8d03e271e8edc556923e9398619f17c"))
	copy(depositTwo.RefundLocktime[:], hexToSlice("60bcea61"))
	var extraData [32]byte
	copy(
		extraData[:],
		hexToSlice(
			"a9b38ea6435c8941d6eda6a46b68e3e2117196995bd154ab55196396b03d9bda",
		),
	)
	depositTwo.ExtraData = &extraData

	merkleRootTwo, err := depositTwo.TaprootMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputScriptTwo, err := bitcoin.PayToTaprootWithScriptTree(
		walletXOnlyPublicKey,
		merkleRootTwo,
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
				PublicKeyScript: fundingOutputScriptOne,
			},
			{
				Value:           110000,
				PublicKeyScript: fundingOutputScriptTwo,
			},
		},
	}

	bitcoinChain := newLocalBitcoinChain()
	if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}

	depositOne.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}
	depositTwo.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     1,
		},
		Value: 110000,
	}

	builder, err := assembleDepositSweepTransaction(
		bitcoinChain,
		walletPublicKey,
		nil,
		[]*Deposit{depositOne, depositTwo},
		1000,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !builder.HasOnlyTaprootKeyPathInputs() {
		t.Fatal("expected only Taproot key-path inputs")
	}

	merkleRoots := builder.TaprootKeyPathInputMerkleRoots()
	if len(merkleRoots) != 2 || merkleRoots[0] == nil || merkleRoots[1] == nil {
		t.Fatalf("expected two Taproot merkle roots")
	}
	testutils.AssertBytesEqual(t, merkleRootOne[:], merkleRoots[0][:])
	testutils.AssertBytesEqual(t, merkleRootTwo[:], merkleRoots[1][:])

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
		209000,
		int(unsignedTx.Outputs[0].Value),
	)
}

func TestValidateDepositSweepProposal_PrefersTaprootRevealOverCompatibilityReveal(t *testing.T) {
	hexToSlice := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatalf("error while converting [%v]: [%v]", hexString, err)
		}
		return bytes
	}

	var fundingTxHash bitcoin.Hash
	copy(fundingTxHash[:], hexToSlice("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"))

	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: fundingTxHash,
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value: 100000,
				PublicKeyScript: bitcoin.Script{
					0x51, 0x20,
					0x23, 0x36, 0xf6, 0x50, 0x04, 0xd8, 0xf1, 0x22,
					0xf1, 0xfe, 0x94, 0x7e, 0xbd, 0x00, 0x9a, 0x8b,
					0x4a, 0xdd, 0x3a, 0x0d, 0x93, 0x73, 0x56, 0xd5,
					0x68, 0xe3, 0x0f, 0x7f, 0xcc, 0x2e, 0x40, 0x08,
				},
			},
		},
	}

	bitcoinChain := newLocalBitcoinChain()
	if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}

	fundingOutputIndex := uint32(0)
	revealBlock := uint64(123)
	var blindingFactor [8]byte
	copy(blindingFactor[:], hexToSlice("f9f0c90d00039523"))
	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], hexToSlice("c92a772f11bc97d8938a16a9db435401f4e6a7bc"))
	var walletXOnlyPublicKey [32]byte
	copy(
		walletXOnlyPublicKey[:],
		hexToSlice("2336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008"),
	)
	var refundPublicKeyHash [20]byte
	copy(refundPublicKeyHash[:], hexToSlice("c2a27a88d8d03e271e8edc556923e9398619f17c"))
	var refundXOnlyPublicKey [32]byte
	copy(
		refundXOnlyPublicKey[:],
		hexToSlice("11223344556677889900aabbccddeeff00112233445566778899aabbccddeeff"),
	)
	var refundLocktime [4]byte
	copy(refundLocktime[:], hexToSlice("60bcea61"))
	depositor := chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637")

	proposal := &DepositSweepProposal{
		DepositsKeys: []struct {
			FundingTxHash      bitcoin.Hash
			FundingOutputIndex uint32
		}{
			{
				FundingTxHash:      fundingTx.Hash(),
				FundingOutputIndex: fundingOutputIndex,
			},
		},
		SweepTxFee: big.NewInt(1000),
		DepositsRevealBlocks: []*big.Int{
			big.NewInt(int64(revealBlock)),
		},
	}

	validationChain := &depositSweepValidationChainStub{
		legacyEvents: []*DepositRevealedEvent{
			{
				FundingTxHash:       fundingTx.Hash(),
				FundingOutputIndex:  fundingOutputIndex,
				Depositor:           depositor,
				Amount:              100000,
				BlindingFactor:      blindingFactor,
				WalletPublicKeyHash: walletPublicKeyHash,
				RefundPublicKeyHash: refundPublicKeyHash,
				RefundLocktime:      refundLocktime,
				BlockNumber:         revealBlock,
			},
		},
		taprootEvents: []*TaprootDepositRevealedEvent{
			{
				FundingTxHash:        fundingTx.Hash(),
				FundingOutputIndex:   fundingOutputIndex,
				Depositor:            depositor,
				Amount:               100000,
				BlindingFactor:       blindingFactor,
				WalletPublicKeyHash:  walletPublicKeyHash,
				WalletXOnlyPublicKey: walletXOnlyPublicKey,
				RefundPublicKeyHash:  refundPublicKeyHash,
				RefundXOnlyPublicKey: refundXOnlyPublicKey,
				RefundLocktime:       refundLocktime,
				BlockNumber:          revealBlock,
			},
		},
		depositRequest: &DepositChainRequest{
			Depositor: depositor,
			Amount:    100000,
		},
	}

	deposits, err := ValidateDepositSweepProposal(
		logger.With(),
		walletPublicKeyHash,
		proposal,
		1,
		validationChain,
		bitcoinChain,
	)
	if err != nil {
		t.Fatal(err)
	}

	if validationChain.legacyValidationCalled {
		t.Fatal("legacy validation should not be called when a Taproot event matches")
	}
	if !validationChain.taprootValidationCalled {
		t.Fatal("Taproot validation was not called")
	}
	if len(deposits) != 1 {
		t.Fatalf("unexpected deposits count: [%v]", len(deposits))
	}
	if !deposits[0].IsTaproot() {
		t.Fatal("expected validated deposit to be Taproot-native")
	}
}

type depositSweepValidationChainStub struct {
	legacyEvents   []*DepositRevealedEvent
	taprootEvents  []*TaprootDepositRevealedEvent
	depositRequest *DepositChainRequest

	legacyValidationCalled  bool
	taprootValidationCalled bool
}

func (dsvcs *depositSweepValidationChainStub) PastDepositRevealedEvents(
	filter *DepositRevealedEventFilter,
) ([]*DepositRevealedEvent, error) {
	return dsvcs.legacyEvents, nil
}

func (dsvcs *depositSweepValidationChainStub) PastTaprootDepositRevealedEvents(
	filter *DepositRevealedEventFilter,
) ([]*TaprootDepositRevealedEvent, error) {
	return dsvcs.taprootEvents, nil
}

func (dsvcs *depositSweepValidationChainStub) ValidateDepositSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *DepositSweepProposal,
	depositsExtraInfo []struct {
		*Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	dsvcs.legacyValidationCalled = true
	return fmt.Errorf("legacy validation should not be called")
}

func (dsvcs *depositSweepValidationChainStub) ValidateTaprootDepositSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *DepositSweepProposal,
	depositsExtraInfo []struct {
		*Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	dsvcs.taprootValidationCalled = true

	if len(depositsExtraInfo) != 1 {
		return fmt.Errorf("unexpected deposits extra info count: [%v]", len(depositsExtraInfo))
	}
	if !depositsExtraInfo[0].Deposit.IsTaproot() {
		return fmt.Errorf("expected Taproot deposit extra info")
	}

	return nil
}

func (dsvcs *depositSweepValidationChainStub) GetDepositRequest(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) (*DepositChainRequest, bool, error) {
	return dsvcs.depositRequest, true, nil
}
