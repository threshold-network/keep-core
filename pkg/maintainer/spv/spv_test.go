package spv

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestGetProofInfo(t *testing.T) {
	// The proof start block in all test cases. Derived from the latest block
	// height and the number of transaction confirmations:
	// proofStartBlock = latestBlockHeight - transactionConfirmations + 1.
	const proofStart = 790270

	// Difficulties are powers of two so they round-trip exactly through the
	// Bitcoin compact (Bits) encoding used by blockHeaderWithDifficulty.
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	tests := map[string]struct {
		transactionConfirmations         uint
		currentEpochDifficulty           *big.Int
		previousEpochDifficulty          *big.Int
		headerDifficultyAt               func(uint) *big.Int
		headersFrom, headersTo           uint
		expectedIsProofWithinRelayRange  bool
		expectedAccumulatedConfirmations uint
		expectedRequiredConfirmations    uint
	}{
		// All proof headers carry the current epoch difficulty. With factor 6,
		// six headers of difficulty 32 reach 6*32.
		"proof entirely within current epoch": {
			transactionConfirmations: 20,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt:       func(uint) *big.Int { return diff(32) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 19,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 20,
			expectedRequiredConfirmations:    6,
		},
		// All proof headers carry the previous epoch difficulty.
		"proof entirely within previous epoch": {
			transactionConfirmations: 20,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt:       func(uint) *big.Int { return diff(16) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 19,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 20,
			expectedRequiredConfirmations:    6,
		},
		// Proof starts in the previous epoch (difficulty 32) two blocks before
		// the epoch boundary and continues in the current epoch (difficulty
		// 16). Required total is 6*32=192; 2*32 + 8*16 = 192 -> 10 headers.
		"proof spans previous and current epochs and difficulty drops": {
			transactionConfirmations: 31,
			currentEpochDifficulty:   diff(16),
			previousEpochDifficulty:  diff(32),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < 790272 {
					return diff(32)
				}
				return diff(16)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + 30,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 31,
			expectedRequiredConfirmations:    10,
		},
		// Required total is 6*16=96; 2*16 + 2*32 = 96 -> 4 headers.
		"proof spans previous and current epochs and difficulty raises": {
			transactionConfirmations: 31,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < 790272 {
					return diff(16)
				}
				return diff(32)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + 30,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 31,
			expectedRequiredConfirmations:    4,
		},
		// Transaction mined in minimum-difficulty (DIFF1) blocks (testnet4
		// BIP94). Leading DIFF1 headers are skipped when binding to the relay
		// difficulty but still contribute their work. Required total is
		// 6*32=192; 1+1+6*32=194 >= 192 -> 8 headers.
		"leading minimum difficulty headers are skipped": {
			transactionConfirmations: 31,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < 790272 {
					return diff(1)
				}
				return diff(32)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + 30,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 31,
			expectedRequiredConfirmations:    8,
		},
		// When the relay epoch difficulty is minimum (test/dev setups),
		// minimum-difficulty headers are not skipped and match directly.
		"epoch difficulty is minimum": {
			transactionConfirmations: 20,
			currentEpochDifficulty:   diff(1),
			previousEpochDifficulty:  diff(1),
			headerDifficultyAt:       func(uint) *big.Int { return diff(1) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 19,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 20,
			expectedRequiredConfirmations:    6,
		},
		// The decisive header difficulty matches neither the current nor the
		// previous relay epoch difficulty. The Bridge would revert, so the
		// transaction is reported as outside the relay range.
		"decisive header matches no epoch difficulty": {
			transactionConfirmations: 20,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt:       func(uint) *big.Int { return diff(8) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 19,

			expectedIsProofWithinRelayRange:  false,
			expectedAccumulatedConfirmations: 0,
			expectedRequiredConfirmations:    0,
		},
		// Long testnet4 runs of minimum-difficulty headers must not prevent a
		// proof from reaching a later decisive header. The Bridge consumes the
		// full header chain, so the maintainer must do the same. After 144 DIFF1
		// headers, the previous-epoch difficulty 16 header binds the requested
		// difficulty and brings the observed total above 6*16=96.
		"decisive header follows long minimum difficulty run": {
			transactionConfirmations: 145,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < proofStart+144 {
					return diff(1)
				}
				return diff(16)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + 144,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 145,
			expectedRequiredConfirmations:    145,
		},
		// The chain tip is reached before enough difficulty is accumulated.
		// The reported requirement is one header more than currently exists,
		// so the caller waits for more confirmations.
		"not enough mined blocks yet": {
			transactionConfirmations: 3,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt:       func(uint) *big.Int { return diff(32) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 2,

			expectedIsProofWithinRelayRange:  true,
			expectedAccumulatedConfirmations: 3,
			expectedRequiredConfirmations:    4,
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
			if err := populateBlockHeaders(
				btcChain,
				test.headersFrom,
				test.headersTo,
				test.headerDifficultyAt,
			); err != nil {
				t.Fatal(err)
			}
			btcChain.addTransactionConfirmations(
				transactionHash,
				test.transactionConfirmations,
			)

			localChain.setTxProofDifficultyFactor(big.NewInt(6))
			localChain.setCurrentEpoch(392)
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

func TestIsInputCurrentWalletsMainUTXO_OutOfRangeFundingOutput(t *testing.T) {
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
	if err := btcChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	_, err := isInputCurrentWalletsMainUTXO(
		fundingTransaction.Hash(),
		2,
		[20]byte{},
		btcChain,
		newLocalChain(),
	)
	if err == nil {
		t.Fatal("expected out-of-range funding output error")
	}
	if !strings.Contains(err.Error(), "funding output index [2] out of range") {
		t.Fatalf("unexpected error: [%v]", err)
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
