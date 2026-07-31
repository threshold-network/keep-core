package spv

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestGetProofInfo(t *testing.T) {
	tests := map[string]struct {
		latestBlockHeight                uint
		transactionConfirmations         uint
		currentEpoch                     uint64
		currentEpochDifficulty           *big.Int
		previousEpochDifficulty          *big.Int
		expectedIsProofWithinRelayRange  bool
		expectedAccumulatedConfirmations uint
		expectedRequiredConfirmations    uint
	}{
		"proof entirely within current epoch": {
			latestBlockHeight:                790277,
			transactionConfirmations:         3,
			currentEpoch:                     392,
			currentEpochDifficulty:           nil, // not needed
			previousEpochDifficulty:          nil, // not needed
			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 3,
			expectedRequiredConfirmations:    6,
		},
		"proof entirely within previous epoch": {
			latestBlockHeight:                790300,
			transactionConfirmations:         2041,
			currentEpoch:                     392,
			currentEpochDifficulty:           nil, // not needed
			previousEpochDifficulty:          nil, // not needed
			expectedAccumulatedConfirmations: 2041,
			expectedIsProofWithinRelayRange:  true,
			expectedRequiredConfirmations:    6,
		},
		"proof spans previous and current epochs and difficulty drops": {
			latestBlockHeight:                790300,
			transactionConfirmations:         31,
			currentEpoch:                     392,
			currentEpochDifficulty:           big.NewInt(50000000000000),
			previousEpochDifficulty:          big.NewInt(30000000000000),
			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 31,
			expectedRequiredConfirmations:    9,
		},
		"proof spans previous and current epochs and difficulty raises": {
			latestBlockHeight:                790300,
			transactionConfirmations:         31,
			currentEpoch:                     392,
			currentEpochDifficulty:           big.NewInt(30000000000000),
			previousEpochDifficulty:          big.NewInt(60000000000000),
			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 31,
			expectedRequiredConfirmations:    4,
		},
		"proof begins outside previous epoch": {
			latestBlockHeight:                790300,
			transactionConfirmations:         2048,
			currentEpoch:                     392,
			currentEpochDifficulty:           nil, // not needed
			previousEpochDifficulty:          nil, // not needed
			expectedIsProofWithinRelayRange:  false,
			expectedAccumulatedConfirmations: 0,
			expectedRequiredConfirmations:    0,
		},
		"proof ends outside current epoch": {
			latestBlockHeight:                792285,
			transactionConfirmations:         3,
			currentEpoch:                     392,
			currentEpochDifficulty:           nil, // not needed
			previousEpochDifficulty:          nil, // not needed
			expectedIsProofWithinRelayRange:  false,
			expectedAccumulatedConfirmations: 0,
			expectedRequiredConfirmations:    0,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			transactionHash, err := bitcoin.NewHashFromString(
				"44c568bc0eac07a2a9c2b46829be5b5d46e7d00e17bfb613f506a75ccf86a473",
				bitcoin.InternalByteOrder,
			)
			if err != nil {
				t.Fatal(err)
			}

			localChain := newLocalChain()

			btcChain := newLocalBitcoinChain()
			btcChain.addBlockHeader(
				test.latestBlockHeight,
				&bitcoin.BlockHeader{},
			)
			btcChain.addTransactionConfirmations(
				transactionHash,
				test.transactionConfirmations,
			)

			localChain.setTxProofDifficultyFactor(big.NewInt(6))
			localChain.setCurrentEpoch(test.currentEpoch)
			localChain.setCurrentAndPrevEpochDifficulty(
				test.currentEpochDifficulty,
				test.previousEpochDifficulty,
			)

			isProofWithinRelayRange,
				accumulatedConfirmations,
				requiredConfirmations,
				err :=
				getProofInfo(
					transactionHash,
					btcChain,
					localChain,
					localChain,
				)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBoolsEqual(
				t,
				"is proof within range",
				test.expectedIsProofWithinRelayRange,
				isProofWithinRelayRange,
			)

			testutils.AssertUintsEqual(
				t,
				"accumulated confirmations",
				uint64(test.expectedAccumulatedConfirmations),
				uint64(accumulatedConfirmations),
			)

			testutils.AssertUintsEqual(
				t,
				"required confirmations",
				uint64(test.expectedRequiredConfirmations),
				uint64(requiredConfirmations),
			)
		})
	}
}

func TestUniqueWalletPublicKeyHashes(t *testing.T) {
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

	events := []*tbtc.DepositRevealedEvent{
		&tbtc.DepositRevealedEvent{
			WalletPublicKeyHash: bytes20FromHex(
				"4cc32253cc0bcd0cf9cfc79ed7b21d10df207f0d",
			),
		},
		&tbtc.DepositRevealedEvent{
			WalletPublicKeyHash: bytes20FromHex(
				"ddbd706d13dbd06038519c7621ac5de167bd3fd6",
			),
		},
		&tbtc.DepositRevealedEvent{
			WalletPublicKeyHash: bytes20FromHex(
				"4cc32253cc0bcd0cf9cfc79ed7b21d10df207f0d",
			),
		},
		&tbtc.DepositRevealedEvent{
			WalletPublicKeyHash: bytes20FromHex(
				"1016a8ff380e8907c82a88158019917e65c16ac4",
			),
		},
		&tbtc.DepositRevealedEvent{
			WalletPublicKeyHash: bytes20FromHex(
				"1016a8ff380e8907c82a88158019917e65c16ac4",
			),
		},
		&tbtc.DepositRevealedEvent{
			WalletPublicKeyHash: bytes20FromHex(
				"2c35ed9921fa35482c3cb3ae1190d87ede65dfd8",
			),
		},
	}
	walletKeyHashes := uniqueWalletPublicKeyHashes(events)

	expectedWalletKeyHashes := [][20]byte{
		bytes20FromHex("4cc32253cc0bcd0cf9cfc79ed7b21d10df207f0d"),
		bytes20FromHex("ddbd706d13dbd06038519c7621ac5de167bd3fd6"),
		bytes20FromHex("1016a8ff380e8907c82a88158019917e65c16ac4"),
		bytes20FromHex("2c35ed9921fa35482c3cb3ae1190d87ede65dfd8"),
	}

	if !reflect.DeepEqual(expectedWalletKeyHashes, walletKeyHashes) {
		t.Errorf(
			"unexpected wallet public key hashes\nexpected: %v\nactual:   %v\n",
			expectedWalletKeyHashes,
			walletKeyHashes,
		)
	}
}

func TestIsInputCurrentWalletsMainUTXO(t *testing.T) {
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

	bytes32FromHex := func(str string) [32]byte {
		var value [32]byte
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

	tests := map[string]struct {
		walletsCurrentMainUtxoHash [32]byte
		expectedIsCurrentMainUtxo  bool
	}{
		"input is the current main UTXO": {
			walletsCurrentMainUtxoHash: bytes32FromHex(
				"9d84b2a9c1860c3f387d5944c9a8e0de55fea4435d19472df99f142b4f38da75",
			),
			expectedIsCurrentMainUtxo: true,
		},
		"input is not the current main UTXO": {
			walletsCurrentMainUtxoHash: bytes32FromHex(
				"01234567890abcdef01234567890abcdef01234567890abcdef01234567890ab",
			),
			expectedIsCurrentMainUtxo: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			fundingTxHash, err := bitcoin.NewHashFromString(
				"ef25c9c8f4df673def035c0c1880278c90030b3c94a56668109001a591c2c521",
				bitcoin.ReversedByteOrder,
			)
			if err != nil {
				t.Fatal(err)
			}

			fundingTxIndex := uint32(1)
			walletPublicKeyHash := bytes20FromHex(
				"ddbd706d13dbd06038519c7621ac5de167bd3fd6",
			)

			localChain := newLocalChain()
			btcChain := newLocalBitcoinChain()

			fundingTransaction := txFromHex(
				"0100000000010110a15e879b7e8b07df62772579a64bf2b409409bbcc8bc2c7f6e39" +
					"31dc615e920100000000ffffffff02042900000000000017a9143ec459d0f3c29286" +
					"ae5df5fcc421e2786024277e87b4121600000000001600148db50eb52063ea9d98b3" +
					"eac91489a90f738986f6024830450221009740ad12d2e74c00ccb4741d533d2ecd69" +
					"02289144c4626508afb61eed790c97022006e67179e8e2a63dc4f1ab758867d8bbfe" +
					"0a2b67682be6dadfa8e07d3b7ba04d012103989d253b17a6a0f41838b84ff0d20e88" +
					"98f9d7b1a98f2564da4cc29dcf8581d900000000",
			)

			err = btcChain.BroadcastTransaction(fundingTransaction)
			if err != nil {
				t.Fatal(err)
			}

			localChain.setWallet(walletPublicKeyHash, &tbtc.WalletChainData{
				MainUtxoHash: test.walletsCurrentMainUtxoHash,
			})

			isCurrentMainUtxo, err := isInputCurrentWalletsMainUTXO(
				fundingTxHash,
				fundingTxIndex,
				walletPublicKeyHash,
				btcChain,
				localChain,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBoolsEqual(
				t,
				"is current main UTXO",
				test.expectedIsCurrentMainUtxo,
				isCurrentMainUtxo,
			)
		})
	}
}

// TestGetUnprovenWalletTransactions_FindsProtocolTransactionBehindSpam verifies
// that a real protocol transaction is still discovered even when more than
// transactionScanLimit unrelated (spam) transactions are paid to the same
// wallet address after it - it just takes as many scanner rounds as it takes
// to walk past the spam, rather than being permanently evicted. This is the
// shared discovery path used by all SPV scanners, so a bounded tail lookup
// would let the spam evict the protocol transaction forever.
func TestGetUnprovenWalletTransactions_FindsProtocolTransactionBehindSpam(t *testing.T) {
	btcChain := newLocalBitcoinChain()

	walletPublicKeyHash := [20]byte{
		0x8d, 0xb5, 0x0e, 0xb5, 0x20, 0x63, 0xea, 0x9d, 0x98, 0xb3,
		0xea, 0xc9, 0x14, 0x89, 0xa9, 0x0f, 0x73, 0x89, 0x86, 0xf6,
	}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	// buildWalletTx builds a distinct confirmed transaction paying to the wallet
	// address. The output count distinguishes the protocol transaction (a single
	// output, as the deposit sweep scanner requires) from spam, and the input
	// outpoint index keeps each transaction unique.
	buildWalletTx := func(index uint32, outputCount int) *bitcoin.Transaction {
		outputs := make([]*bitcoin.TransactionOutput, outputCount)
		for i := range outputs {
			outputs[i] = &bitcoin.TransactionOutput{
				Value:           546,
				PublicKeyScript: walletScript,
			}
		}
		return &bitcoin.Transaction{
			Version: 1,
			Inputs: []*bitcoin.TransactionInput{
				{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: bitcoin.Hash{},
						OutputIndex:     index,
					},
					Sequence: 0xffffffff,
				},
			},
			Outputs:  outputs,
			Locktime: 0,
		}
	}

	// The protocol transaction is the oldest transaction paying to the wallet.
	protocolTransaction := buildWalletTx(0, 1)
	if err := btcChain.BroadcastTransaction(protocolTransaction); err != nil {
		t.Fatal(err)
	}

	// Add many unrelated transactions after the protocol one, well beyond the
	// per-round scan limit, each paying to the same wallet address.
	transactionLimit := 5
	transactionScanLimit := 4
	spamCount := transactionScanLimit * 6
	for i := 0; i < spamCount; i++ {
		spamTransaction := buildWalletTx(uint32(i+1), 2)
		if err := btcChain.BroadcastTransaction(spamTransaction); err != nil {
			t.Fatal(err)
		}
	}

	protocolTransactionHash := protocolTransaction.Hash()

	scanner := newWalletTransactionScanner()
	key := walletTransactionScanKey{lookupWalletPKH: walletPublicKeyHash}
	predicate := func(transaction *bitcoin.Transaction) (bool, error) {
		return transaction.Hash() == protocolTransactionHash, nil
	}

	// A single round with a scan limit far smaller than the spam count must
	// not find the protocol transaction yet - it is still buried beyond this
	// round's examination budget.
	scanner.beginRound()
	found, err := scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, transactionLimit, transactionScanLimit,
		"test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf(
			"expected the protocol transaction to still be undiscovered "+
				"after one bounded round, got %d match(es)",
			len(found),
		)
	}

	// Repeated rounds (simulating repeated maintainer ticks) must eventually
	// walk the cursor past all the spam and discover the protocol
	// transaction - it must never be permanently evicted.
	maxRounds := (spamCount / transactionScanLimit) + 2
	for round := 0; round < maxRounds && len(found) == 0; round++ {
		scanner.beginRound()
		found, err = scanner.getUnprovenWalletTransactions(
			key, walletPublicKeyHash, transactionLimit, transactionScanLimit,
			"test", btcChain, predicate,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(found) != 1 {
		t.Fatalf(
			"expected exactly one unproven transaction within %d rounds, got %d",
			maxRounds, len(found),
		)
	}
	if found[0].Hash() != protocolTransactionHash {
		t.Fatal(
			"expected the protocol transaction to be discovered behind the spam",
		)
	}

	// Once discovered, the transaction must stay reported on subsequent
	// rounds instead of needing to be rediscovered.
	scanner.beginRound()
	foundAgain, err := scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, transactionLimit, transactionScanLimit,
		"test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundAgain) != 1 || foundAgain[0].Hash() != protocolTransactionHash {
		t.Fatal("expected the protocol transaction to remain a candidate")
	}
}
