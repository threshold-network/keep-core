package spv

import (
	"encoding/hex"
	"fmt"
	"sort"
	"testing"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestSubmitMovedFundsSweepProof(t *testing.T) {
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

	txHashFromHex := func(str string) bitcoin.Hash {
		hash, err := bitcoin.NewHashFromString(str, bitcoin.ReversedByteOrder)
		if err != nil {
			t.Fatal(err)
		}

		return hash
	}

	requiredConfirmations := uint(6)

	var tests = map[string]struct {
		movedFundsSweepTx *bitcoin.Transaction
		inputTxs          []*bitcoin.Transaction
		expectedMainUtxo  *bitcoin.UnspentTransactionOutput
	}{
		"wallet has no main UTXO": {
			// Transaction for a wallet no main UTXO: https://live.blockcypher.com/btc-testnet/tx/a586427d66f8ccca1ed8f7e40a2c82aae99a1f85dfce62ffe2f3657350b6fd84/
			movedFundsSweepTx: txFromHex("010000000001019b1b33bdd3c44404544991889d63afe6caa875983b705106f1d988251d1459200000000000ffffffff011c700000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6024730440220242dbac95ab8e632cd2791e99d3048b96e6e042bcd902f30fbae7e942a24ea3e02201b7416e6d7d36ea142521eb80e0bc29d118f62ab6b8a64a062cc5812cfdb8c89012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d900000000"),
			// No need for additional transaction data.
			inputTxs: []*bitcoin.Transaction{},
			// Zero-filled main UTXO.
			expectedMainUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{},
					OutputIndex:     0,
				},
				Value: 0,
			},
		},
		"wallet has main UTXO": {
			// Transaction for a wallet with main UTXO: https://live.blockcypher.com/btc-testnet/tx/fc78f52ab4094b5c0bf8a782750c24f31b5db2667425fbddccc29d64f89baf9b/
			movedFundsSweepTx: txFromHex("0100000000010218201d563e43a926f5f9fd4498af5c513a3ea284373308aeda39b0a0d57585780000000000ffffffff84fdb6507365f3e2ff62cedf851f9ae9aa822c0ae4f7d81ecaccf8667d4286a50000000000ffffffff0104a60000000000001600148db50eb52063ea9d98b3eac91489a90f738986f60248304502210089dfa958867b2265d0fc08d996af82a9a731bd972f20e0530d37937f38d9ec1002200cbc820a696b99747aed39aeed5d848367773cf6d4e24aaa12fe2ad714a1ff99012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d902473044022063dc201589b1f7810247eaa569baf5e3dda8717a10e77a2ad95661fef643bdc602203bee4dd0c4a24291523bb7542394df4e6008c0c7ddfdd30c41d55036b32a8999012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d900000000"),
			// Transaction that created the main UTXO: https://live.blockcypher.com/btc-testnet/tx/a586427d66f8ccca1ed8f7e40a2c82aae99a1f85dfce62ffe2f3657350b6fd84/
			inputTxs: []*bitcoin.Transaction{
				txFromHex("010000000001019b1b33bdd3c44404544991889d63afe6caa875983b705106f1d988251d1459200000000000ffffffff011c700000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6024730440220242dbac95ab8e632cd2791e99d3048b96e6e042bcd902f30fbae7e942a24ea3e02201b7416e6d7d36ea142521eb80e0bc29d118f62ab6b8a64a062cc5812cfdb8c89012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d900000000"),
			},
			// Wallet's main UTXO.
			expectedMainUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: txHashFromHex("a586427d66f8ccca1ed8f7e40a2c82aae99a1f85dfce62ffe2f3657350b6fd84"),
					OutputIndex:     0,
				},
				Value: 28700,
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			btcChain := newLocalBitcoinChain()
			spvChain := newLocalChain()

			for _, inputTransaction := range test.inputTxs {
				err := btcChain.BroadcastTransaction(inputTransaction)
				if err != nil {
					t.Fatal(err)
				}
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
				if hash == test.movedFundsSweepTx.Hash() && confirmations == requiredConfirmations {
					return test.movedFundsSweepTx, proof, nil
				}

				return nil, nil, fmt.Errorf("error while assembling spv proof")
			}

			err := submitMovedFundsSweepProof(
				test.movedFundsSweepTx.Hash(),
				requiredConfirmations,
				btcChain,
				spvChain,
				mockSpvProofAssembler,
			)
			if err != nil {
				t.Fatal(err)
			}

			submittedProofs := spvChain.getSubmittedMovedFundsSweepProofs()

			testutils.AssertIntsEqual(t, "proofs count", 1, len(submittedProofs))

			submittedProof := submittedProofs[0]

			expectedTransactionHash := test.movedFundsSweepTx.Hash()
			actualTransactionHash := submittedProof.transaction.Hash()
			testutils.AssertBytesEqual(t, expectedTransactionHash[:], actualTransactionHash[:])

			if diff := deep.Equal(proof, submittedProof.proof); diff != nil {
				t.Errorf("invalid proof: %v", diff)
			}

			if diff := deep.Equal(*test.expectedMainUtxo, submittedProof.mainUTXO); diff != nil {
				t.Errorf("invalid main UTXO: %v", diff)
			}
		})
	}
}

func TestGetUnprovenMovedFundsSweepTransactions(t *testing.T) {
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

	setMovedFundsSweepRequests := func(requests []struct {
		hash  string
		index uint32
		state tbtc.MovedFundsSweepRequestState
	}) {
		for _, request := range requests {
			spvChain.setMovedFundsSweepRequest(
				hashFromString(request.hash),
				request.index,
				&tbtc.MovedFundsSweepRequest{
					State: request.state,
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
			// Wallet 1: Random wallet that was listed as a target wallet, but
			//           hasn't performed any moved funds sweep transactions.
			walletPublicKeyHash: bytes20FromHex("3091d288521caec06ea912eacfd733edc5a36d6e"),
			data: &tbtc.WalletChainData{
				State: tbtc.StateLive,
			},
		},

		{
			// Wallet 2: https://live.blockcypher.com/btc-testnet/address/tb1q3k6sadfqv04fmx9naty3fzdfpaecnphkfm3cf3/
			walletPublicKeyHash: bytes20FromHex("8db50eb52063ea9d98b3eac91489a90f738986f6"),
			data: &tbtc.WalletChainData{
				// Simulate the wallet has no main UTXO.
				MainUtxoHash:                        [32]byte{},
				State:                               tbtc.StateLive,
				PendingMovedFundsSweepRequestsCount: 1,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Moved funds sweep transaction: https://live.blockcypher.com/btc-testnet/tx/a586427d66f8ccca1ed8f7e40a2c82aae99a1f85dfce62ffe2f3657350b6fd84/
				txFromHex("010000000001019b1b33bdd3c44404544991889d63afe6caa875983b705106f1d988251d1459200000000000ffffffff011c700000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6024730440220242dbac95ab8e632cd2791e99d3048b96e6e042bcd902f30fbae7e942a24ea3e02201b7416e6d7d36ea142521eb80e0bc29d118f62ab6b8a64a062cc5812cfdb8c89012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d900000000"),
			},
		},
		{
			// Wallet 3: https://live.blockcypher.com/btc-testnet/address/tb1q0tpdjdu2r3r7tzwlhqy4e2276g2q6fexsz4j0m/
			walletPublicKeyHash: bytes20FromHex("7ac2d9378a1c47e589dfb8095ca95ed2140d2726"),
			data: &tbtc.WalletChainData{
				// Make the main UTXO point to Transaction 1.
				MainUtxoHash: mainUtxoHash(
					"28f5aad58758acc861893a24edf3a339f8257fcde502a4b8add605e74a7d5f7d",
					0,
					873510,
				),
				State:                               tbtc.StateMovingFunds,
				PendingMovedFundsSweepRequestsCount: 1,
			},
			transactions: []*bitcoin.Transaction{
				// Transaction 1: Moved funds sweep transaction: https://live.blockcypher.com/btc-testnet/tx/f97ed3704f59bf5ed828d90f04598ea6c1c65a7957befa1f1c175a142c17fff9/
				txFromHex("01000000027d5f7d4ae705d6adb8a402e5cd7f25f839a3f3ed243a8961c8ac5887d5aaf528010000006b483045022100ff95e465ae7f632026e30dfe6c53df8f445066d735f60e3ec411fc1f753aa8860220740aa810b18d4ae90653db147b35c83827b942177d74a418aa6d48d387550725012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04effffffff7d5f7d4ae705d6adb8a402e5cd7f25f839a3f3ed243a8961c8ac5887d5aaf528000000006a473044022058901f5a01c214c3d8ddb2246876a6f96646826a87a9669eacd0d36bac73225202206c19cc3fc2e899b36d2e8f2e6e6bdaa135e051a14b98990184b9cbcd5a4a1ab8012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04effffffff0132dd2700000000001976a9147ac2d9378a1c47e589dfb8095ca95ed2140d272688ac00000000"),

				// Transaction 2: Transaction that created the main UTXO: https://live.blockcypher.com/btc-testnet/tx/28f5aad58758acc861893a24edf3a339f8257fcde502a4b8add605e74a7d5f7d/
				txFromHex("01000000000101d914a2171f2fb236e85abca59da852e06747df559b7c66e3dbff842642e55c4b0100000000ffffffff0226540d00000000001976a9147ac2d9378a1c47e589dfb8095ca95ed2140d272688ac4ca81a00000000001976a9147ac2d9378a1c47e589dfb8095ca95ed2140d272688ac024730440220529e25602583815c9ec4d0567d30f917323e26dbcc53a60d1883235a357d56d602204fd018078511e6de40b906874cf4531ee4edc652a6948667ba228b6034d9541a012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000"),
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

	setMovedFundsSweepRequests(
		[]struct {
			hash  string
			index uint32
			state tbtc.MovedFundsSweepRequestState
		}{
			// Moved funds sweep request from Wallet 2.
			{
				"2059141d2588d9f10651703b9875a8cae6af639d889149540444c4d3bd331b9b",
				0,
				tbtc.MovedFundsStatePending,
			},
			// Wallet main UTXO transaction from Wallet 3.
			{
				"28f5aad58758acc861893a24edf3a339f8257fcde502a4b8add605e74a7d5f7d",
				0,
				tbtc.MovedFundsStateUnknown,
			},
			// Moved funds sweep request from Wallet 3.
			{
				"28f5aad58758acc861893a24edf3a339f8257fcde502a4b8add605e74a7d5f7d",
				1,
				tbtc.MovedFundsStatePending,
			},
		},
	)

	// Add moving funds commitment submitted events for the wallets.
	// The block number field is just to make them distinguishable while reading.
	events := []*tbtc.MovingFundsCommitmentSubmittedEvent{
		{
			WalletPublicKeyHash: bytes20FromHex("92a6ec889a8fa34f731e639edede4c75e184307c"),
			TargetWallets: [][20]byte{
				wallets[0].walletPublicKeyHash,
				wallets[1].walletPublicKeyHash,
			},
			BlockNumber: 100,
		},
		{
			WalletPublicKeyHash: bytes20FromHex("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
			TargetWallets: [][20]byte{
				wallets[0].walletPublicKeyHash,
				wallets[1].walletPublicKeyHash,
				wallets[2].walletPublicKeyHash,
			},
			BlockNumber: 200,
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

	transactions, err := getUnprovenMovedFundsSweepTransactions(
		historyDepth,
		transactionLimit,
		DefaultTransactionScanLimit,
		newWalletTransactionScanner(),
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
		wallets[1].transactions[0].Hash(), // Wallet 2 - Transaction 1
		wallets[2].transactions[0].Hash(), // Wallet 3 - Transaction 1
	}

	// The order of returned transaction hashes is random. Sort them before
	// comparing.
	sort.Slice(transactionsHashes, func(i, j int) bool {
		return transactionsHashes[i].String() < transactionsHashes[j].String()
	})

	if diff := deep.Equal(expectedTransactionsHashes, transactionsHashes); diff != nil {
		t.Errorf("invalid unproven transaction hashes: %v", diff)
	}
}
