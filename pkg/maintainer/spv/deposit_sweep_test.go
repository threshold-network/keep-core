package spv

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestSubmitDepositSweepProof(t *testing.T) {
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

	// Take an arbitrary deposit sweep transaction:
	// https://live.blockcypher.com/btc-testnet/tx/435d4aff6d4bc34134877bd3213c17970142fdd04d4113d534120033b9eecb2e/
	depositSweepTransaction := txFromHex("010000000001036896f9abcac13ce6bd2b80d125bedf997ff6330e999f2f605ea15ea542f2eaf80000000000ffffffffed0ae94da996c6f3b89dfe967675d4808251db93e81022ae9e038d06f92efed400000000c948304502210092327ddff69a2b8c7ae787c5d590a2f14586089e6339e942d56e82aa42052cd902204c0d1700ba1ac617da27fee032a57937c9607f0187199ed3c46954df845643d7012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68ffffffffe37f552fc23fa0032bfd00c8eef5f5c22bf85fe4c6e735857719ff8a4ff66eb80000000000ffffffff0180ed0000000000001600148db50eb52063ea9d98b3eac91489a90f738986f602483045022100baf754252d0d6a49aceba7eb0ec40b4cc568e8c659e168b96598a11cf56dc078022051117466ee998a3fc72221006817e8cfe9c2e71ad622ff811a0bf100d888d49c012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d90003473044022014a535eb334656665ac69a678dbf7c019c4f13262e9ea4d195c61a00cd5f698d022023c0062913c4614bdff07f94475ceb4c585df53f71611776c3521ed8f8785913012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d95c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac6800000000")
	// Set the transactions that created the inputs of the deposit sweep
	// transaction. It is necessary as the tested function logic fetches its
	// data to determine the wallet's main UTXO and the deposit vault.
	inputTransactions := []*bitcoin.Transaction{
		// main UTXO input: https://live.blockcypher.com/btc-testnet/tx/f8eaf242a55ea15e602f9f990e33f67f99dfbe25d1802bbde63cc1caabf99668/
		txFromHex("01000000000102bc187be612bc3db8cfcdec56b75e9bc0262ab6eacfe27cc1a699bacd53e3d07400000000c948304502210089a89aaf3fec97ac9ffa91cdff59829f0cb3ef852a468153e2c0e2b473466d2e022072902bb923ef016ac52e941ced78f816bf27991c2b73211e227db27ec200bc0a012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68ffffffffdc557e737b6688c5712649b86f7757a722dc3d42786f23b2fa826394dfec545c0000000000ffffffff01488a0000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6000347304402203747f5ee31334b11ebac6a2a156b1584605de8d91a654cd703f9c8438634997402202059d680211776f93c25636266b02e059ed9fcc6209f7d3d9926c49a0d8750ed012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d95c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac6800000000"),
		// deposit input: https://live.blockcypher.com/btc-testnet/tx/d4fe2ef9068d039eae2210e893db518280d4757696fe9db8f3c696a94de90aed/
		txFromHex("01000000000101e37f552fc23fa0032bfd00c8eef5f5c22bf85fe4c6e735857719ff8a4ff66eb80100000000ffffffff02684200000000000017a9143ec459d0f3c29286ae5df5fcc421e2786024277e8742b7100000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed0248304502210084eb60347b9aa48d9a53c6ab0fc2c2357a0df430d193507facfb2238e46f034502202a29d11e128dba3ff3a8ad9a1e820a3b58e89e37fa90d1cc2b3f05207599fef00121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000"),
		// deposit input: https://live.blockcypher.com/btc-testnet/tx/b86ef64f8aff19778535e7c6e45ff82bc2f5f5eec800fd2b03a03fc22f557fe3/
		txFromHex("01000000000101dc557e737b6688c5712649b86f7757a722dc3d42786f23b2fa826394dfec545c0100000000ffffffff02102700000000000022002086a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca962cff100000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed02473044022050759dde2c84bccf3c1502b0e33a6acb570117fd27a982c0c2991c9f9737508e02201fcba5d6f6c0ab780042138a9110418b3f589d8d09a900f20ee28cfcdb14d2970121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000"),
	}
	for _, transaction := range inputTransactions {
		err := btcChain.BroadcastTransaction(transaction)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set deposit request information for the transaction's deposit inputs.
	for i := 1; i < len(depositSweepTransaction.Inputs); i++ {
		spvChain.setDepositRequest(
			depositSweepTransaction.Inputs[i].Outpoint.TransactionHash,
			depositSweepTransaction.Inputs[i].Outpoint.OutputIndex,
			&tbtc.DepositChainRequest{
				RevealedAt: time.Unix(1000, 0),
				SweptAt:    time.Unix(0, 0),
			},
		)
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
		if hash == depositSweepTransaction.Hash() && confirmations == requiredConfirmations {
			return depositSweepTransaction, proof, nil
		}

		return nil, nil, fmt.Errorf("error while assembling spv proof")
	}

	err := submitDepositSweepProof(
		depositSweepTransaction.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	)
	if err != nil {
		t.Fatal(err)
	}

	submittedProofs := spvChain.getSubmittedDepositSweepProofs()
	testutils.AssertIntsEqual(t, "proofs count", 1, len(submittedProofs))

	submittedProof := submittedProofs[0]

	expectedTransactionHash := depositSweepTransaction.Hash()
	actualTransactionHash := submittedProof.transaction.Hash()
	testutils.AssertBytesEqual(t, expectedTransactionHash[:], actualTransactionHash[:])

	if diff := deep.Equal(proof, submittedProof.proof); diff != nil {
		t.Errorf("invalid proof: %v", diff)
	}

	expectedMainUtxo := bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: inputTransactions[0].Hash(),
			OutputIndex:     0,
		},
		Value: 35400,
	}
	if diff := deep.Equal(expectedMainUtxo, submittedProof.mainUTXO); diff != nil {
		t.Errorf("invalid main UTXO: %v", diff)
	}

	expectedVault := [20]byte{}
	testutils.AssertBytesEqual(
		t,
		expectedVault[:],
		submittedProof.vault[:],
	)
}

func TestGetUnprovenDepositSweepTransactions(t *testing.T) {
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

	hashFromString := func(str string) bitcoin.Hash {
		hash, err := bitcoin.NewHashFromString(
			str,
			bitcoin.ReversedByteOrder,
		)
		if err != nil {
			t.Fatal(err)
		}

		return hash
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

	setDepositRequests := func(deposits []struct {
		hash  string
		index uint32
	}) {
		for _, deposit := range deposits {
			spvChain.setDepositRequest(
				hashFromString(deposit.hash),
				deposit.index,
				&tbtc.DepositChainRequest{
					// Mark the deposit as revealed and unswept.
					RevealedAt: time.Unix(100000, 0),
					SweptAt:    time.Unix(0, 0),
				},
			)
		}
	}

	// Define wallets being actors of this scenario:
	wallets := []struct {
		walletPublicKeyHash [20]byte
		data                *tbtc.WalletChainData
		transactions        []*bitcoin.Transaction
	}{
		{
			// Wallet 1: https://live.blockcypher.com/btc-testnet/address/tb1q3k6sadfqv04fmx9naty3fzdfpaecnphkfm3cf3
			walletPublicKeyHash: bytes20FromHex("8db50eb52063ea9d98b3eac91489a90f738986f6"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO filled with zero values as the Transasaction 1
				// was the wallet's first transaction.
				MainUtxoHash: mainUtxoHash(
					"0000000000000000000000000000000000000000000000000000000000000000",
					0,
					0,
				),
				State: tbtc.StateLive,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Deposit: https://live.blockcypher.com/btc-testnet/tx/c580e0e352570d90e303d912a506055ceeb0ee06f97dce6988c69941374f5479
				txFromHex("01000000011d9b71144a3ddbb56dd099ee94e6dd8646d7d1eb37fe1195367e6fa844a388e7010000006a47304402206f8553c07bcdc0c3b906311888103d623ca9096ca0b28b7d04650a029a01fcf9022064cda02e39e65ace712029845cfcf58d1b59617d753c3fd3556f3551b609bbb00121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfaffffffff02204e00000000000017a9143ec459d0f3c29286ae5df5fcc421e2786024277e87a6c2140000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed00000000"),
				// Transaction 2: Deposit sweep: https://live.blockcypher.com/btc-testnet/tx/f5b9ad4e8cd5317925319ebc64dc923092bef3b56429c6b1bc2261bbdc73f351
				txFromHex("010000000179544f374199c68869ce7df906eeb0ee5c0506a512d903e3900d5752e3e080c500000000c847304402205eff3ae003a5903eb33f32737e3442b6516685a1addb19339c2d02d400cf67ce0220707435fc2a0577373c63c99d242c30bea5959ec180169978d43ece50618fe0ff012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68ffffffff0144480000000000001600148db50eb52063ea9d98b3eac91489a90f738986f600000000"),
			},
		},
		{
			// Wallet 2: https://live.blockcypher.com/btc-testnet/address/tb1qqwm566yn44rdlhgph8sw8vecta8uutg79afuja
			walletPublicKeyHash: bytes20FromHex("03b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO pointing to Transaction 2.
				MainUtxoHash: mainUtxoHash(
					"2f1532ab22b8005cfd18361a562545de3daf45af1c6dff8288a563c30d5354aa",
					0,
					4490000,
				),
				State: tbtc.StateMovingFunds,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Deposit - https://live.blockcypher.com/btc-testnet/tx/4da7eb22f550fa86040676d6173936aa2cc3c826369088fcb3c850c670b27d43
				txFromHex("020000000001036af25a46e05249189ec79430ccf13acb47f9d0a89a9fcbacdde41f8b0195f9c800000000232200201ceb5ed9aebfe9f9c3d2954ae8dacdfd68138c135249566a53127ca64d4d9db7fdffffffef6611d6f0715eb783bc1f0b2a2c9654fc97fa84fc9c8e090674931b0030f8cf010000002322002053ee1f96b67fc80dee871942c5a793469749d9cd207966203aa8e066b173ec2bfdffffffe4167d4447b9fcf17a9cac242d1128ee7fcbd88b6ea473203667070d5a46787e01000000232200202897cfe01913f798e3fd0d0bb4bc5a7bd30a6d15732f40f1493b2d627fcd6c95fdffffff0260e3160000000000220020901f47553748a623736b7eb8fbef4760f2d512ee6e6080f7b6f56dfcd892d8fc49e815000000000017a914ef5311088575bceac84ad1dc7375f89f4dd89ea18703473044022040173e3bc5866bdb471661bd22e8e4e3f678afa0c10f015e2fe94ffaf86665db022002a8a7f7bc6107f8ad47712d9be81af3a1b65a562feb63c13b3d9794172024fa0147304402203c8846a3bdb3419218c22d6f524e1c5ac26ec0c7c2dd41f8c6b39506c0f6670102200248316928705447770da8fb77eb325ef88704a3ec7418d8fae83afc148d75a5014e2103e676934d53084891ab3d129e1cb18bb6d3ad8968ec07b0578ba10f4f425b5253ad2102adf20988a38447a419c9f49e50c162d0ea425282131ee26e7ccd092a7dac2b91ac73640380ca00b26803463043022005b29d35370ef7df0ed786e321229e87873b4f7c08ecee6bba518b21ee2e02f4021f461385fedc7dbac79f471a5bc1639591ab01d554220247d99800e289c59ad00147304402200c4eb17dcd8595d40af7b0ec8c45f2df70a05a5fa92d5b052a2b86468d216fb30220039eb7163de31edf143430b562bf9edbabf26c0a24a59c13dc799c6a07276fdc014e21035a8f725c288806079760b2dcaff62c05b9013b18dfb3bfa8354f1ffd96dee0bfad21020347f884d906878b861dfd0b3fd123509ecdf795115018e330051bafa44b76eaac73640380ca00b2680347304402205705c22592e3ef4a8ef08e9e625080cd37fcab2e2ba2af31779fdd83a3946750022072c63e14ba076d6f7c56939edcadccb133c89de891a07fcab08df41afceff613014730440220597c2d9a25c1c6ed74e1a43ea52680b8d2dadb34d3c0b79c4a15ca4281e73d3f022064ba977f20c229002c83c936b05baf2c3a30ea383058e209313adb1c330d2566014e21021daa486b5fcee3ab1feb2486966e81d8b5b869c9aac86b245ccc24e9809a6296ad21039d66a87bae3eab61cfe6e9b7916cc89d526d8517804170629953dff2877c5f14ac73640380ca00b268f6e32400"),
				// Transaction 2: Deposit sweep - https://live.blockcypher.com/btc-testnet/tx/2f1532ab22b8005cfd18361a562545de3daf45af1c6dff8288a563c30d5354aa
				txFromHex("01000000000103437db270c650c8b3fc88903626c8c32caa363917d676060486fa50f522eba74d0000000000ffffffff965a8e2b6611e4cd702dbb6054f322527e4948a12076ce35732fafcde9bccd530100000000ffffffff7aedae166ab68c8da5e83c0c3a2fff584d74e61d4595560f0cb7bf0518fd62380100000000ffffffff01108344000000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e03473044022051115f3cd013e3e28abb8080bf6889c3e740228a92daab9fd6aa023b3855d8d5022026ea788a181213bb75c9c5f3d05e7fb085f00a9d5dbd603850da3af4034c68600121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c1468ad60cc5e8f3b7cc53beab321cf0e6036962dbc75080c6c48d8993396997576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a9147ac2d9378a1c47e589dfb8095ca95ed2140d27268804a7f63365b175ac68034830450221009d4b0b604ccb472934170dabba7453294b68319689a69345dfae23eb9503c89202204e2e01ae15745152b09ff8ad013335a091a2af789de6c2b562535914bcdc7e0c0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c1458c6a45acfcc1fd0e5a103cab2cae00b0b188ec5750837454759e1b5a4337576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a91415834d7aedb4bcbb9fbe6972cfa0ff37ca1e7b668804bd523465b175ac6803473044022008dccf94caf6a21af5b2ae3cf0cfbe20c21f9d57e180026013c70528840dac7d02205b5d811159f3fbb4b3269b78fa8f33c5f9f49be1c5bc37483d28a716d64111990121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c1458c6a45acfcc1fd0e5a103cab2cae00b0b188ec57508f3550e8c19b00c3e7576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a91415834d7aedb4bcbb9fbe6972cfa0ff37ca1e7b6688048c2a3665b175ac6800000000"),
				// Transaction 3: Deposit sweep - https://live.blockcypher.com/btc-testnet/tx/d8e23c68b6971b6c0475be27830a56a4942dcdafbbf95836692754a8c12f9cd8
				txFromHex("01000000000106aa54530dc363a58882ff6d1caf45af3dde4525561a3618fd5c00b822ab32152f0000000000ffffffff6cb09112438796560a92454b9e3074db39905677f65c235591ce95c20d68473a0000000000ffffffffc66592d3108b49c5886434fed322e00a3715b894eb0d983b8e5b7af6552f3fd90100000000ffffffff3361afc47f7c43b4947397b726d3a6ffec2e5169a2197cbbe4aa4535442e43450000000000ffffffff781474e16e166487abaf762b800ad668173aa61ba8aa63b94ce829f1ddfaad370000000000ffffffff6420e0ded70ae5265ab0a89a7ea5fc4e79522b48fbb6d1f8c4f1a8b10cde61040000000000ffffffff01f0bd2a040000000016001403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e0247304402202826048b75632760b304ad0625c5eeecb38ea586d023710e8a167e488ec61a8902204ccc0fc62c7ed48d22adda42d677a6a614225868f44f13f57940c169ad0ef3b10121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f7034830450221008a6f64cdd3ea84ed155f31656afaf0b78b645ad0a5de6bc4148519e33712b7ac02206ee2f3bdfabeba1a0799b2f9c8eb9584c7d17d858148fb3af35efc154759f5680121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c14ab45507d1db315e8618ea26d78f1c852100777927508aa57dc7df5eec8ae7576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a914d0163def6271c9aaa09ef16f3f8d0bccb707f1b28804dd733665b175ac680347304402200846626a31fe9877795607abd711543a76c6fb9acda8c5c5a89a9f87a5dcd91c022002632052e17ce6fd37eea7e1da83cf075705dd24de4fd16188de68804b6e04e80121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c14dafd72e76a04a4d618277a2ff280e3c4a8ac342d75080e20b455c44b75d17576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a91462ae7b18817be4d82b51719b43c690ddd687217c8804819f3665b175ac6803483045022100d479005feeb11bd635f8f990028f6d5f79a115b844542c9ba56c6b4ff95c52ef02204842d0b1876afef3e59d8be4d1176cd68f40e5f6c10889493707fba292e710ad0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c14f119557ac33585405467135ec9a343dcdb04751775082f5668fd4a77d7227576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a9147ac2d9378a1c47e589dfb8095ca95ed2140d27268804c9933665b175ac68034730440220367be02570e07f2d61b1dc24fa7bb8ff5aa5b1561aa9b41380232ec597c1cfb60220326d156c82d0d9601be7cac15f6dca0544425f5ae80b7c0e190ac7876bed7f1d0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c143ff855895ef4ac833c32ab6a0d6c7fbfa137e26e750849bbf447435a4ef37576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a91428eb5ea70f51e5a27820e0c645b4d1b9e2d8e3c68804169d3665b175ac6803473044022018382aa45f5271d0d0c8a1c8703a18dc9f7db16c1e905c8a8d269e13058eac98022017862fe5bc208cfd0d55dc79d39fdfe512039ea638baa3f2bff1a2f433cee92f0121028ed84936be6a9f594a2dcc636d4bebf132713da3ce4dac5c61afbf8bbb47d6f75c141fb2d377340f4b776ed2516b2293bd65fd2058587508745db273eff7244b7576a91403b74d6893ad46dfdd01b9e0e3b3385f4fce2d1e8763ac6776a9143ed40732712fd1d359375438aae4b0bd372fcb5388040c9e3665b175ac6800000000"),
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

	setDepositRequests(
		[]struct {
			hash  string
			index uint32
		}{
			// Deposit from Wallet 1
			{
				"c580e0e352570d90e303d912a506055ceeb0ee06f97dce6988c69941374f5479",
				0,
			},
			// Deposits from Wallet 2
			{
				"3a47680dc295ce9155235cf677569039db74309e4b45920a569687431291b06c",
				0,
			},
			{
				"d93f2f55f67a5b8e3b980deb94b815370ae022d3fe346488c5498b10d39265c6",
				1,
			},
			{
				"45432e443545aae4bb7c19a269512eecffa6d326b7977394b4437c7fc4af6133",
				0,
			},
			{
				"37adfaddf129e84cb963aaa81ba63a1768d60a802b76afab8764166ee1741478",
				0,
			},
			{
				"0461de0cb1a8f1c4f8d1b6fb482b52794efca57e9aa8b05a26e50ad7dee02064",
				0,
			},
		},
	)

	// Add deposit events for the wallets. Only wallet public key hash field
	// is relevant as those events are just used to get a list of distinct
	// wallets who likely performed deposit sweeps recently. The block number
	// field is just to make them distinguishable while reading.
	events := []*tbtc.DepositRevealedEvent{
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
		err := spvChain.addPastDepositRevealedEvent(
			&tbtc.DepositRevealedEventFilter{
				StartBlock: currentBlock - historyDepth,
			},
			event,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	transactions, err := getUnprovenDepositSweepTransactions(
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
		wallets[1].transactions[2].Hash(), // Wallet 2 - Transaction 3
	}

	if diff := deep.Equal(expectedTransactionsHashes, transactionsHashes); diff != nil {
		t.Errorf("invalid unproven transaction hashes: %v", diff)
	}
}
