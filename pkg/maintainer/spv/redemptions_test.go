package spv

import (
	"encoding/hex"
	"fmt"
	"github.com/go-test/deep"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"testing"
)

func TestSubmitRedemptionProof(t *testing.T) {
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

	// Take an arbitrary redemption transaction:
	// https://live.blockcypher.com/btc-testnet/tx/15c9b4dd136f1c102cd45a92f6d6f41accc610a68566029de9af3524d53f1d82/
	redemptionTransaction := txFromHex("0100000000010189a128bbd1fd4626f752aa9036a118b2f4b2363ef409f5b527c69d048214d3130000000000ffffffff039ef9e92e0000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e6eed10000000000017a91486884e6be1525dab5ae0b451bd2c72cee67dcf4187791411000000000017a914538e4cc700d6510c8cae5e8b688d65276771e6088702483045022100b2e7fc655e0ddadbfef49201fb5f7046a40b36848c08f17ef2e4483bffb7a29e022024616909a96f8c901572d6a9e19d29d6aee6a835b409d4383a463fe1b338a2940121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f700000000")
	// Take the transaction that is the redemption transaction input. It is
	// necessary as the tested function logic fetches its data to determine
	// the wallet public key hash.
	// https://live.blockcypher.com/btc-testnet/tx/13d31482049dc627b5f509f43e36b2f4b218a13690aa52f72646fdd1bb28a189
	redemptionInputTransaction := txFromHex("01000000000101db7aad9f51cffa7cebf5a3b41dc3552e1151d2550d8919a8e13d6bb00e046d5b0000000000ffffffff0333fc0b2f0000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e182612000000000017a914538e4cc700d6510c8cae5e8b688d65276771e60887aa9f10000000000017a91486884e6be1525dab5ae0b451bd2c72cee67dcf418702483045022100dded6eeacf49830de6f6b590a56f9b8ba3c2fda0b24e7f51884226a5ee78b5c2022024b1fbf3406716c9f9c5bfe241cfc0766af8209ecf8eb5f3318b407fd41c59ec0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f700000000")
	// Then, record both transactions on the local BTC chain.
	err := btcChain.BroadcastTransaction(redemptionTransaction)
	if err != nil {
		t.Fatal(err)
	}
	err = btcChain.BroadcastTransaction(redemptionInputTransaction)
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
		if hash == redemptionTransaction.Hash() && confirmations == requiredConfirmations {
			return redemptionTransaction, proof, nil
		}

		return nil, nil, fmt.Errorf("error while assembling spv proof")
	}

	err = submitRedemptionProof(
		redemptionTransaction.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	)
	if err != nil {
		t.Fatal(err)
	}

	submittedProofs := spvChain.getSubmittedRedemptionProofs()

	testutils.AssertIntsEqual(t, "proofs count", 1, len(submittedProofs))

	submittedProof := submittedProofs[0]

	expectedTransactionHash := redemptionTransaction.Hash()
	actualTransactionHash := submittedProof.transaction.Hash()
	testutils.AssertBytesEqual(t, expectedTransactionHash[:], actualTransactionHash[:])

	if diff := deep.Equal(proof, submittedProof.proof); diff != nil {
		t.Errorf("invalid proof: %v", diff)
	}

	expectedMainUtxo := bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: redemptionInputTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 789314611,
	}
	if diff := deep.Equal(expectedMainUtxo, submittedProof.mainUTXO); diff != nil {
		t.Errorf("invalid main UTXO: %v", diff)
	}

	testutils.AssertBytesEqual(t, bytesFromHex("03b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e"), submittedProof.walletPublicKeyHash[:])
}

func TestGetUnprovenRedemptionTransactions(t *testing.T) {
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
			// Wallet 1: https://live.blockcypher.com/btc-testnet/address/tb1qqwm566yn44rdlhgph8sw8vecta8uutg79afuja
			walletPublicKeyHash: bytes20FromHex("03b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO pointing to Transaction 2.
				MainUtxoHash: mainUtxoHash(
					"13d31482049dc627b5f509f43e36b2f4b218a13690aa52f72646fdd1bb28a189",
					0,
					789314611,
				),
				State: tbtc.StateLive,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Deposit sweep - https://live.blockcypher.com/btc-testnet/tx/5b6d040eb06b3de1a819890d55d251112e55c31db4a3f5eb7cfacf519fad7adb
				txFromHex("010000000001025f421805a688c6044723bf2604fc3b6c48775794a145ae3a9a6e90309c5196ef0000000000ffffffffb70774c563bf11342a904531c9b17e036077da9c00693b9fda59cd0acf9fc82a0100000000ffffffff0115102f2f0000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e024730440220019e6db083462fff19fc2d8b55420f23801895045e70392add0b6342b361cb26022007e0e6bc1e066b4dac6f72061486be463936938c0d2cb7bf139b1c9afa1438af0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f703483045022100f018f213b0b89d9c85a87d355f5d14cc106b20eb014daaaf5f2570ac661c29b402206c19ee0c4c2bf33f8a11af5a2ac3c4c4736e026b008df5b91b564671e855a0af0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c1427343e0410acd8cf711d079c57811fe8c0666df27508b075608a63c879397576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a914be94fbd152b1c9f396a5e2dce4f536de0cddac1e880472bc3b65b175ac6800000000"),
				// Transaction 2: Redemption - https://live.blockcypher.com/btc-testnet/tx/13d31482049dc627b5f509f43e36b2f4b218a13690aa52f72646fdd1bb28a189
				txFromHex("01000000000101db7aad9f51cffa7cebf5a3b41dc3552e1151d2550d8919a8e13d6bb00e046d5b0000000000ffffffff0333fc0b2f0000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e182612000000000017a914538e4cc700d6510c8cae5e8b688d65276771e60887aa9f10000000000017a91486884e6be1525dab5ae0b451bd2c72cee67dcf418702483045022100dded6eeacf49830de6f6b590a56f9b8ba3c2fda0b24e7f51884226a5ee78b5c2022024b1fbf3406716c9f9c5bfe241cfc0766af8209ecf8eb5f3318b407fd41c59ec0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f700000000"),
				// Transaction 3: Redemption - https://live.blockcypher.com/btc-testnet/tx/15c9b4dd136f1c102cd45a92f6d6f41accc610a68566029de9af3524d53f1d82/
				txFromHex("0100000000010189a128bbd1fd4626f752aa9036a118b2f4b2363ef409f5b527c69d048214d3130000000000ffffffff039ef9e92e0000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e6eed10000000000017a91486884e6be1525dab5ae0b451bd2c72cee67dcf4187791411000000000017a914538e4cc700d6510c8cae5e8b688d65276771e6088702483045022100b2e7fc655e0ddadbfef49201fb5f7046a40b36848c08f17ef2e4483bffb7a29e022024616909a96f8c901572d6a9e19d29d6aee6a835b409d4383a463fe1b338a2940121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f700000000"),
			},
		},
		{
			// Wallet 2: https://live.blockcypher.com/btc-testnet/address/tb1q0tpdjdu2r3r7tzwlhqy4e2276g2q6fexsz4j0m
			walletPublicKeyHash: bytes20FromHex("7ac2d9378a1c47e589dfb8095ca95ed2140d2726"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO pointing to Transaction 1.
				MainUtxoHash: mainUtxoHash(
					"fe67cf8fcf227739c375a2caa5623663c008aee366cb0090b357daaa2bed7e27",
					0,
					907458022,
				),
				State: tbtc.StateMovingFunds,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1 Redemption - https://live.blockcypher.com/btc-testnet/tx/fe67cf8fcf227739c375a2caa5623663c008aee366cb0090b357daaa2bed7e27
				txFromHex("020000000001081afaff1906c692569c00296893ab7def72d9a63a1d02b982373afc4e1319914e0000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffffab0911c68ba830dc347ce9102fb9adb966efb174661b5b5967fa77864c38ea9b0000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffff4f26c5850329b901b65fdcb7c9cd635f2466121adc3e5b24cffe2a6dcf0f3e6c0000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdfffffff96397dd8a89e6f2cd2ca8860facfefbe41bc0381db805acddbafa71c884e2800000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffff8de3b217186c524e557068cde4809b133e22ab394217f2f0f7648b3e478e14640000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffff39917ca0672de1e00ce164c9765f7db4f682ce9389a7fb28c96eb1144faddf560000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffffa19170d7c9e44256b0a4a8f4f4037af1373509411b1a22f95003d29880ce7bb40000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffffa164ef963e5c34bf2e78ba4611d43dec976aa35a8c9fb96a6421046e3eaff5eb0000000023220020cdcbf54e6a21f8543b86be6e159c97c88fea24c00afb8be090f90731ece77e1dfdffffff02e6b51636000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d2726f28c7e080000000017a9143a788574273dae7687a9f004901c6f6eec95ea2f8704004730440220449cc8faebc36e642862b8f719464963757120a88a890b68e3a4393defa6db7302200547f493d0b8901d4ade8813270792d625f1432288dc4c9fbbc148107cf1eee401473044022025d20f3b9e1d48575a407bef7af50670779305a09a22f2bbd687faf45d8aaf7f0220546f360980027a9d903e5cd747bb37888c22a5730777485d1791b3330a7294c60147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae04004730440220538bdccdd6f914d2c8041756cc291efcf04c2adc3bfc4ee4c8e1baa416e7909802204f54c20ab99872d14a562ba5932561ca61ffbc0f82cf5ced5cd08edf3d0048560147304402200bf4ef72723838b90025d651f4a49e359df6b6563f4c86fc8ffc14177418ebf20220183a87f7303fc1825d2c5a560746016bbf8203ce2c1052ccdfae986cc182fb500147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae040047304402203f7d797f3a573642cdec69ea64589b9a4fdf90d1c10bc1549594cc15a4c5526e02201ebe61915333794c3cd655ea30b14c2f4808e4c97e7dbad1fb5f72e17652b7d10147304402203f94ec246a9e69560d75e0fd9b7100f494fad302c6f876f7fd005701aa5da10002202b78cf9abfe056a9aaf2bd9ae3b431d35b1a63e0280e546fe91979850c7411240147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae0400473044022022778a00059ef615df7671b78480c377218b022e602a0c7e23195871c4e35826022027c10b6e0ea9e974643cefb336ef9e837991a6fc11fff3a44320b9751c01cdb701473044022020f98851956e16c4522832a98feff2ff16af56ba71984f1fa2e35175cb9b10df0220718bf892eec64bf20712abfd7b1ec26698126c0363ae5a5ed20edc1e5be807840147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae040047304402206dc19efc4ac56d2c8add4cebe7fdd2f21ae1e8c9031406e77218e76b9c66ad1c02205f902b8baa47a07a4ff22afc7593f8f4f70d00688efd166f5070f8ba6a1314a301473044022003ad49229c3b0ecd78c9bf06f89acc11f3713e66c3aa0858e01100b7597b175d02202ee3691398ee4bf3ff991105fffcf67806a8a3ca7279f1186ef0cb23707957bf0147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae040047304402204abdb93f587c5e2910b15c5397c466a015ebf36f6dc409985e00cd05df7ef65c02207e555a2ee60c5720cf37ab0ef2c916c11c149ee45fe7ea8e3ee91a22581880150147304402206e046d3b973a3c796667c91daed1c38dcbab554483fcd40ed8bc3b8649ab7fdc0220634779793c81c638b351f0e9def674b2725017d5e8416af04e77e05400ee629d0147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae0400473044022077d2998402ffb5ed5215ec0bf5af695ea6d96596315eacf71323c0d300cdc3790220189504c92de9ee2c55a0edb2ecf36ec8bcaa7025952ea1e1251e5a3829ec06cf0147304402200839a0d265fac5bd5d2d04502c44e8f4ae988fd681667f21cf69a4953452849202204a733bb0224f3ab44e52559fc7b58a7c12a8fdada43ef6a97e4deee884757a480147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae040047304402200d7a61a4b283638038405da13b0ef2c62864abc1f3cbe73eadeb13df107575fc02206d97d68a9beb70010350571242c198f18b63b6e01997a683e965bdc750b13dcb0147304402202d464ceb7d7307aac44f110ad8e452c5f7e0364e70833f154005abb0eac460ad02207aa37e36fc1d7daafda6b2289e3642173d6325c21249800f56231bd323b22f750147522103eda20693e28c993d25d58e714d18650ea985f194da3c45fe6d993b78c4e7cb7a2103cfb89adf7c80aed9b79153cd2923d9d27431f3e886b7f0d0646bea0c3996ffbd52ae1d1a2500"),
				// Transaction 2: Redemption - https://live.blockcypher.com/btc-testnet/tx/183850b32c685fa8c824e1a16a6d45eed84605021a9829dbafc33adaf493e5bc
				txFromHex("01000000000101277eed2baada57b39000cb66e3ae08c0633662a5caa275c3397722cf8fcf67fe0000000000ffffffff0280841e000000000022002051ee6c8b014a19861f73d8c744299b4d650bdc5974c3944588f648b955293db1762bf835000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d272602483045022100cf6f508186fedc2c67012da5f6f363c1047a173282a5cac68f21bfd43f865f7902203162c84338d0067ec2f70ea1e6f8051fe21f1efadf2767cdc4e6d8633e8fd248012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000"),
			},
		},
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

	// According to MainUtxoHash values set in the wallets mapping, the
	// last unproven transactions for specific wallets are:
	// - Transaction 3 for Wallet 1
	// - Transaction 2 for Wallet 2
	// We need to simulate that there are existing pending requests corresponding
	// to their outputs in order to make this test scenario happen.
	for _, output := range wallets[0].transactions[2].Outputs {
		spvChain.setPendingRedemptionRequest(
			wallets[0].walletPublicKeyHash,
			&tbtc.RedemptionRequest{
				// Only redeemer output script is relevant.
				RedeemerOutputScript: output.PublicKeyScript,
			},
		)
	}
	for _, output := range wallets[1].transactions[1].Outputs {
		spvChain.setPendingRedemptionRequest(
			wallets[1].walletPublicKeyHash,
			&tbtc.RedemptionRequest{
				// Only redeemer output script is relevant.
				RedeemerOutputScript: output.PublicKeyScript,
			},
		)
	}

	// Add redemption events for the wallets. Only wallet public key hash field
	// is relevant as those events are just used to get a list of distinct
	// wallets who likely performed redemptions recently. The block number field
	// is just to make them distinguishable while reading.
	events := []*tbtc.RedemptionRequestedEvent{
		{
			WalletPublicKeyHash: wallets[0].walletPublicKeyHash,
			BlockNumber:         100,
		},
		{
			WalletPublicKeyHash: wallets[0].walletPublicKeyHash,
			BlockNumber:         200,
		},
		{
			WalletPublicKeyHash: wallets[1].walletPublicKeyHash,
			BlockNumber:         300,
		},
		{
			WalletPublicKeyHash: wallets[1].walletPublicKeyHash,
			BlockNumber:         400,
		},
	}

	for _, event := range events {
		err := spvChain.addPastRedemptionRequestedEvent(
			&tbtc.RedemptionRequestedEventFilter{
				StartBlock: currentBlock - historyDepth,
			},
			event,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	transactions, err := getUnprovenRedemptionTransactions(
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
		wallets[0].transactions[2].Hash(), // Wallet 1 - Transaction 3
		wallets[1].transactions[1].Hash(), // Wallet 2 - Transaction 2
	}

	if diff := deep.Equal(expectedTransactionsHashes, transactionsHashes); diff != nil {
		t.Errorf("invalid unproven transaction hashes: %v", diff)
	}
}
