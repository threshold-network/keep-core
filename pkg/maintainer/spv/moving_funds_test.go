package spv

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestSubmitMovingFundsProof(t *testing.T) {
	bytesFromHex := func(str string) []byte {
		value, err := hex.DecodeString(str)
		if err != nil {
			t.Fatal(err)
		}

		return value
	}

	txFromHex := func(str string) *bitcoin.Transaction {
		transaction := new(bitcoin.Transaction)
		err := transaction.Deserialize(bytesFromHex(str))
		if err != nil {
			t.Fatal(err)
		}

		return transaction
	}

	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	// Take an arbitrary moving funds transaction:
	// https://live.blockcypher.com/btc-testnet/tx/e6218018ed1874e73b78e16a8cf4f5016cbc666a3f9179557a84083e3e66ff7c/
	movingFundsTransaction := txFromHex("0100000000010180653f6e07dabddae14cf08d45475388343763100e4548914d811f373465a42e0100000000ffffffff031c160900000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488ac1d160900000000001600148900de8fc6e4cd1db4c7ab0759d28503b4cb0ab11c160900000000001976a914af7a841e055fc19bf31acf4cbed5ef548a2cc45388ac0247304402202d615c196548b6cb4f1cd1f44b559cd348ce2cb8bd90356be9883a7460d7c8aa0220675e7b67e4d96a6180f7adb5ecb9ab962275d39742009911980e19e734523ff4012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000")
	// Take the transaction that is the moving funds transaction input. It is
	// necessary as the tested function logic fetches its data to determine
	// the wallet public key hash.
	// https://live.blockcypher.com/btc-testnet/tx/2ea46534371f814d9148450e10633734885347458df04ce1dabdda076e3f6580/
	movingFundsInputTransaction := txFromHex("02000000000101f064a0d2775bda695f1b5e476c1860aa541ef065c5d9a4eb5d49fc8595c6c1dd0100000000feffffff0252ba92190200000016001420e46ff6ba650c7898839617ce40ef54ca0e33377d651b00000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d2726024730440220755156b6d9759f213a5fe189e222e9b781b6386ed83fbf070113b082ff2ddcd60220355820a2a87990271dcdedb57a80402b18e1aa6fcbaf7fa15eb06ba06790cfc101210399d30f01b702d4b8607c429af6bb7d0611cf6333a23154b31ba2aaefdd88d5b6b2782100")

	err := btcChain.BroadcastTransaction(movingFundsInputTransaction)
	if err != nil {
		t.Fatal(err)
	}

	// Just a mock proof.
	proof := &bitcoin.SpvProof{
		MerkleProof:    []byte{0x01},
		TxIndexInBlock: 2,
		BitcoinHeaders: []byte{0x03},
	}

	mockSpvProofAssembler := func(
		hash bitcoin.Hash,
		confirmations uint,
		btcChain bitcoin.Chain,
	) (*bitcoin.Transaction, *bitcoin.SpvProof, error) {
		if hash == movingFundsTransaction.Hash() && confirmations == requiredConfirmations {
			return movingFundsTransaction, proof, nil
		}

		return nil, nil, fmt.Errorf("error while assembling spv proof")
	}

	err = submitMovingFundsProof(
		movingFundsTransaction.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	)
	if err != nil {
		t.Fatal(err)
	}

	submittedProofs := spvChain.getSubmittedMovingFundsProofs()

	testutils.AssertIntsEqual(t, "proofs count", 1, len(submittedProofs))

	submittedProof := submittedProofs[0]

	expectedTransactionHash := movingFundsTransaction.Hash()
	actualTransactionHash := submittedProof.transaction.Hash()
	testutils.AssertBytesEqual(t, expectedTransactionHash[:], actualTransactionHash[:])

	if diff := deep.Equal(proof, submittedProof.proof); diff != nil {
		t.Errorf("invalid proof: %v", diff)
	}

	expectedMainUtxo := bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: movingFundsInputTransaction.Hash(),
			OutputIndex:     1,
		},
		Value: 1795453,
	}
	if diff := deep.Equal(expectedMainUtxo, submittedProof.mainUTXO); diff != nil {
		t.Errorf("invalid main UTXO: %v", diff)
	}

	testutils.AssertBytesEqual(t, bytesFromHex("7ac2d9378a1c47e589dfb8095ca95ed2140d2726"), submittedProof.walletPublicKeyHash[:])
}

func TestSubmitMovingFundsProof_TaprootWalletInput(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	walletPublicKeyHash := bytes20FromHex(
		t,
		"c7302d75072d78be94eb8d36c4b77583c7abb06e",
	)
	walletID := [32]byte{
		0x23, 0x36, 0xf6, 0x50, 0x04, 0xd8, 0xf1, 0x22,
		0xf1, 0xfe, 0x94, 0x7e, 0xbd, 0x00, 0x9a, 0x8b,
		0x4a, 0xdd, 0x3a, 0x0d, 0x93, 0x73, 0x56, 0xd5,
		0x68, 0xe3, 0x0f, 0x7f, 0xcc, 0x2e, 0x40, 0x08,
	}
	spvChain.setWallet(walletPublicKeyHash, &tbtc.WalletChainData{
		WalletID: walletID,
	})

	walletOutputScript, err := bitcoin.PayToTaproot(walletID)
	if err != nil {
		t.Fatal(err)
	}

	var previousTxHash bitcoin.Hash
	previousTxHash[0] = 0x01
	inputTx := &bitcoin.Transaction{
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
	if err := btcChain.BroadcastTransaction(inputTx); err != nil {
		t.Fatal(err)
	}

	targetOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(
		bytes20FromHex(t, "3091d288521caec06ea912eacfd733edc5a36d6e"),
	)
	if err != nil {
		t.Fatal(err)
	}

	movingFundsTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: inputTx.Hash(),
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           99000,
				PublicKeyScript: targetOutputScript,
			},
		},
	}

	proof := &bitcoin.SpvProof{
		MerkleProof:    []byte{0x01},
		TxIndexInBlock: 2,
		BitcoinHeaders: []byte{0x03},
	}
	mockSpvProofAssembler := func(
		hash bitcoin.Hash,
		confirmations uint,
		btcChain bitcoin.Chain,
	) (*bitcoin.Transaction, *bitcoin.SpvProof, error) {
		if hash == movingFundsTx.Hash() && confirmations == requiredConfirmations {
			return movingFundsTx, proof, nil
		}

		return nil, nil, fmt.Errorf("error while assembling spv proof")
	}

	err = submitMovingFundsProof(
		movingFundsTx.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	)
	if err != nil {
		t.Fatal(err)
	}

	submittedProofs := spvChain.getSubmittedMovingFundsProofs()
	testutils.AssertIntsEqual(t, "proofs count", 1, len(submittedProofs))
	testutils.AssertBytesEqual(
		t,
		walletPublicKeyHash[:],
		submittedProofs[0].walletPublicKeyHash[:],
	)
}

func TestGetUnprovenMovingFundsTransactions(t *testing.T) {
	bytesFromHex := func(str string) []byte {
		value, err := hex.DecodeString(str)
		if err != nil {
			t.Fatal(err)
		}

		return value
	}

	bytes20FromHex := func(str string) [20]byte {
		var value [20]byte
		copy(value[:], bytesFromHex(str))
		return value
	}

	txFromHex := func(str string) *bitcoin.Transaction {
		transaction := new(bitcoin.Transaction)
		err := transaction.Deserialize(bytesFromHex(str))
		if err != nil {
			t.Fatal(err)
		}

		return transaction
	}

	// Set an arbitrary history depth and transaction limit.
	historyDepth := uint64(5)
	transactionLimit := 10

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	// Set a predictable current block.
	currentBlock := uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	mainUtxoHash := func(hashHex string, outputIndex uint32, value int64) [32]byte {
		hash, err := bitcoin.NewHashFromString(hashHex, bitcoin.ReversedByteOrder)
		if err != nil {
			t.Fatal(err)
		}

		return spvChain.ComputeMainUtxoHash(
			&bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: hash,
					OutputIndex:     outputIndex,
				},
				Value: value,
			},
		)
	}

	// Define wallets being actors of this scenario:
	wallets := []struct {
		walletPublicKeyHash [20]byte
		data                *tbtc.WalletChainData
		transactions        []*bitcoin.Transaction
	}{
		{
			// Wallet 1: https://live.blockcypher.com/btc-testnet/address/tb1q3k6sadfqv04fmx9naty3fzdfpaecnphkfm3cf3/
			walletPublicKeyHash: bytes20FromHex("8db50eb52063ea9d98b3eac91489a90f738986f6"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO point to Transaction 1.
				MainUtxoHash: mainUtxoHash(
					"11d27b4af5598bd0e5cea22c40f0ed8623278ad18f7fb2afc139ce99087aae44",
					0,
					40000,
				),
				State: tbtc.StateMovingFunds,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Creation of wallets main UTXO https://live.blockcypher.com/btc-testnet/tx/11d27b4af5598bd0e5cea22c40f0ed8623278ad18f7fb2afc139ce99087aae44/
				txFromHex("02000000000101cf063c32da06ff5044f38220c61ee57ac5f3c31368a665bddb109d225d8b4d1a0100000000fdffffff02409c0000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6df66000000000000160014be8347707d02375d1bd0c8f21a59f44c62990d8f024730440220166fbabc5d144d639e350826a04f1d2d148702312ce532da962076eb3859b81c02203d640f23f84a3792451539616432ffb07dc8ddac72047da56a22ecb842165bb4012103399de99c1d409735b7d20f46f616cd485f6b0442db13dfe4cd1b3772ae675fe024572700"),

				// Transaction 2: MovingFunds: https://live.blockcypher.com/btc-testnet/tx/b6a67aeaf684b64b3506759c907a2d96e0891df32b4a7be0e9f8e05b5cd457b5/
				txFromHex("0100000000010144ae7a0899ce39c1afb27f8fd18a272386edf0402ca2cee5d08b59f54a7bd2110000000000ffffffff037a310000000000001600143091d288521caec06ea912eacfd733edc5a36d6e7a3100000000000016001492a6ec889a8fa34f731e639edede4c75e184307c7c31000000000000160014c7302d75072d78be94eb8d36c4b77583c7abb06e0247304402200be4c3706674d848b09bee2c72c23a71832b5da98d8c4fb8ba7febcc4464e5c102207a8fbaea6f253727add9baa5adc0d53ca7eafc7493d937180959c82d9b3d69c9012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d900000000"),

				// Transaction 3: Some unrelated transaction that funded transaction 4: https://live.blockcypher.com/btc-testnet/tx/0ff42f0da4fa4afff6b1cda0ec08d45f3fc50a19eb487e853fb0c8abc9f1bb4b/
				txFromHex("0200000000010198baf342676258d1fe06b44a9d421eac8568135f338f7b6e23d5ed8332affd190100000000fdffffff02b8880000000000001600148db50eb52063ea9d98b3eac91489a90f738986f69e94000000000000160014503af7f416e3ff1cdd5c43dd524d3cca2759d6b302473044022040ed8d5bc709fc8f14117c7090e66e7db945a860ec5011026252b61b102fce5e0220047d05c757c5092b9c43065e7b23b4f7d1054245bb31327f8cdcc02a4f163c620121038e17bbc2bf0bbdf61c25681fd6a003d40f3927c035a2ebd2944b0090afacebc8bd562700"),

				// Transaction 4: Some unrelated transaction that pays to the first target wallet: https://live.blockcypher.com/btc-testnet/tx/bc124a83c29ad2f3ad09e9f61e96f94551dc618df311168139e8b8eb84416f76/
				txFromHex("020000000001014bbbf1c9abc8b03f857e48eb190ac53f5fd408eca0cdb1f6ff4afaa40d2ff40f0100000000fdffffff020f270000000000001600143091d288521caec06ea912eacfd733edc5a36d6e026d000000000000160014edefac5962a46a02969879a7d232f37dbf0ac9f60247304402206941f2aaeb5561c6b58928ca4b0285e03ef0f8e36018c9b45c709e001ed0681a022052e50c1bb2f987dfaee251b70fe1eb6a1287c0723ff043880f46c82c09baacbe012103a9936dedbc8d585219d8daf1471e009253376cccb08f17561bad2ea09c2028bd04592700"),
			},
		},

		{
			// Wallet 2: https://live.blockcypher.com/btc-testnet/address/tb1q0tpdjdu2r3r7tzwlhqy4e2276g2q6fexsz4j0m/
			walletPublicKeyHash: bytes20FromHex("7ac2d9378a1c47e589dfb8095ca95ed2140d2726"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO point to Transaction 1.
				MainUtxoHash: mainUtxoHash(
					"89c1e51322878df5417652643a2cbc4bcc3b2ecaff371c3e03b7b9b285d5e3f8",
					1,
					1473114,
				),
				State: tbtc.StateMovingFunds,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Creation of wallet's main UTXO https://live.blockcypher.com/btc-testnet/tx/89c1e51322878df5417652643a2cbc4bcc3b2ecaff371c3e03b7b9b285d5e3f8/
				txFromHex("02000000014ff17f9f98c5f9c516a94b1c08eabd0ad04fd04e1e3b9485493592e7fc76a7ab000000006a47304402202d3356f81c1d488ec7a9c2917bdb9c6060b9307eca2dcf5e876792e2ba6db85c022016dbb59e1104ef311c20235c1c1894be94ef0c317c4f3b0d13d31c67c592001301210291936829fd41e5217272a8141313ceb754d65787dc07ccb9a9e9a384ef243645feffffff025108ee16020000001600141ea512aa81a96d5ffa2f0d3e37803d9912c89e7e5a7a1600000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d2726e2792100"),

				// Transaction 2: MovingFunds: https://live.blockcypher.com/btc-testnet/tx/d078c00d7e78509062fccdecaf85580efe6e2826d8db77341fbc1097ca2955e5/
				txFromHex("01000000000101f8e3d585b2b9b7033e1c37ffca2e3bcc4bbc2c3a64527641f58d872213e5c1890100000000ffffffff0132571600000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488ac024830450221008b6b3fa3eaf4b46268c3bfb718cf8391afc7879ec1949e465c04fa206235a3f202205f9d00ebba7cdb29414b0dad752cf489710ab60dc06c7b20a99c0d9d3fce8c3c012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000"),
			},
		},
	}

	targetWallets := [][20]byte{
		bytes20FromHex("3091d288521caec06ea912eacfd733edc5a36d6e"),
		bytes20FromHex("92a6ec889a8fa34f731e639edede4c75e184307c"),
		bytes20FromHex("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
		bytes20FromHex("2cd680318747b720d67bf4246eb7403b476adb34"),
	}

	// Record wallet data on both chains.
	for _, wallet := range wallets {
		spvChain.setWallet(wallet.walletPublicKeyHash, wallet.data)

		for _, transaction := range wallet.transactions {
			err := btcChain.BroadcastTransaction(transaction)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, targetWallet := range targetWallets {
		spvChain.setWallet(targetWallet, &tbtc.WalletChainData{
			WalletID: tbtc.DeriveLegacyWalletID(targetWallet),
		})
	}

	// Add moving funds commitment submitted events for the wallets.
	// The block number field is just to make them distinguishable while reading.
	events := []*tbtc.MovingFundsCommitmentSubmittedEvent{
		{
			WalletPublicKeyHash: wallets[0].walletPublicKeyHash,
			TargetWallets:       targetWallets[:3],
			BlockNumber:         100,
		},
		{
			WalletPublicKeyHash: wallets[1].walletPublicKeyHash,
			TargetWallets:       targetWallets[3:],
			BlockNumber:         200,
		},
	}

	for _, event := range events {
		err := spvChain.addPastMovingFundsCommitmentSubmittedEvent(
			&tbtc.MovingFundsCommitmentSubmittedEventFilter{
				StartBlock: currentBlock - historyDepth,
			},
			event,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	transactions, err := getUnprovenMovingFundsTransactions(
		historyDepth,
		transactionLimit,
		btcChain,
		spvChain,
	)
	if err != nil {
		t.Fatal(err)
	}

	transactionsHashes := make([]bitcoin.Hash, len(transactions))
	for i, transaction := range transactions {
		transactionsHashes[i] = transaction.Hash()
	}

	expectedTransactionsHashes := []bitcoin.Hash{
		wallets[0].transactions[1].Hash(), // Wallet 1 - Transaction 2
		wallets[1].transactions[1].Hash(), // Wallet 2 - Transaction 2
	}

	if diff := deep.Equal(expectedTransactionsHashes, transactionsHashes); diff != nil {
		t.Errorf("invalid unproven transaction hashes: %v", diff)
	}
}

func TestGetUnprovenMovingFundsTransactions_FindsTaprootTargetOutput(
	t *testing.T,
) {
	historyDepth := uint64(5)
	transactionLimit := 10

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	currentBlock := uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	sourceWalletPublicKeyHash := bytes20FromHex(
		t,
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	sourceWalletID := [32]byte{
		0x23, 0x36, 0xf6, 0x50, 0x04, 0xd8, 0xf1, 0x22,
		0xf1, 0xfe, 0x94, 0x7e, 0xbd, 0x00, 0x9a, 0x8b,
		0x4a, 0xdd, 0x3a, 0x0d, 0x93, 0x73, 0x56, 0xd5,
		0x68, 0xe3, 0x0f, 0x7f, 0xcc, 0x2e, 0x40, 0x08,
	}
	targetWalletPublicKeyHash := bytes20FromHex(
		t,
		"c7302d75072d78be94eb8d36c4b77583c7abb06e",
	)
	targetWalletID := [32]byte{
		0x90, 0xe7, 0xce, 0x2b, 0x70, 0xe4, 0x23, 0x3a,
		0xbe, 0x6f, 0xd4, 0x9d, 0x45, 0x72, 0x45, 0xe3,
		0x5a, 0x3b, 0xa1, 0x4f, 0x12, 0x1d, 0x16, 0x0c,
		0xeb, 0x61, 0x26, 0xed, 0x1e, 0xdf, 0x14, 0x5a,
	}

	sourceScript, err := bitcoin.PayToTaproot(sourceWalletID)
	if err != nil {
		t.Fatal(err)
	}
	targetScript, err := bitcoin.PayToTaproot(targetWalletID)
	if err != nil {
		t.Fatal(err)
	}

	var previousTxHash bitcoin.Hash
	previousTxHash[0] = 0x02
	inputTx := &bitcoin.Transaction{
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
				PublicKeyScript: sourceScript,
			},
		},
	}
	if err := btcChain.BroadcastTransaction(inputTx); err != nil {
		t.Fatal(err)
	}

	movingFundsTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: inputTx.Hash(),
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           99000,
				PublicKeyScript: targetScript,
			},
		},
	}
	if err := btcChain.BroadcastTransaction(movingFundsTx); err != nil {
		t.Fatal(err)
	}

	spvChain.setWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{
		WalletID: sourceWalletID,
		MainUtxoHash: spvChain.ComputeMainUtxoHash(
			&bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: inputTx.Hash(),
					OutputIndex:     0,
				},
				Value: 100000,
			},
		),
		State: tbtc.StateMovingFunds,
	})
	spvChain.setWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{
		WalletID: targetWalletID,
		State:    tbtc.StateLive,
	})

	err = spvChain.addPastMovingFundsCommitmentSubmittedEvent(
		&tbtc.MovingFundsCommitmentSubmittedEventFilter{
			StartBlock: currentBlock - historyDepth,
		},
		&tbtc.MovingFundsCommitmentSubmittedEvent{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
			TargetWallets: [][20]byte{
				targetWalletPublicKeyHash,
			},
			BlockNumber: currentBlock - 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	unprovenTransactions, err := getUnprovenMovingFundsTransactions(
		historyDepth,
		transactionLimit,
		btcChain,
		spvChain,
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(t, "transactions count", 1, len(unprovenTransactions))
	expectedHash := movingFundsTx.Hash()
	actualHash := unprovenTransactions[0].Hash()
	testutils.AssertBytesEqual(
		t,
		expectedHash[:],
		actualHash[:],
	)
}

func TestGetUnprovenMovingFundsTransactions_TargetWalletLookupFailure(
	t *testing.T,
) {
	historyDepth := uint64(5)
	transactionLimit := 10

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	currentBlock := uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	sourceWalletPublicKeyHash := bytes20FromHex(
		t,
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	targetWalletPublicKeyHash := bytes20FromHex(
		t,
		"c7302d75072d78be94eb8d36c4b77583c7abb06e",
	)

	spvChain.setWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{
		State: tbtc.StateMovingFunds,
	})

	err := spvChain.addPastMovingFundsCommitmentSubmittedEvent(
		&tbtc.MovingFundsCommitmentSubmittedEventFilter{
			StartBlock: currentBlock - historyDepth,
		},
		&tbtc.MovingFundsCommitmentSubmittedEvent{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
			TargetWallets: [][20]byte{
				targetWalletPublicKeyHash,
			},
			BlockNumber: currentBlock - 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = getUnprovenMovingFundsTransactions(
		historyDepth,
		transactionLimit,
		btcChain,
		spvChain,
	)
	if err == nil {
		t.Fatal("expected target wallet lookup failure")
	}

	expectedError := fmt.Sprintf(
		"failed to get target wallet [%x]: [no wallet for given PKH]",
		targetWalletPublicKeyHash,
	)
	testutils.AssertStringsEqual(t, "error", expectedError, err.Error())
}
