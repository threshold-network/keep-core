package spv

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"
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

// stubTransactionChain overrides GetTransactionsForPublicKeyHash on the local
// Bitcoin chain so that collectUnprovenWalletTransactions can be exercised with
// a controlled set of transactions and error, independently of how the local
// chain filters by public key hash.
type stubTransactionChain struct {
	*localBitcoinChain
	transactions []*bitcoin.Transaction
	err          error
}

func (s *stubTransactionChain) GetTransactionsForPublicKeyHash(
	_ [20]byte,
	_ int,
) ([]*bitcoin.Transaction, error) {
	return s.transactions, s.err
}

func TestCollectUnprovenWalletTransactions(t *testing.T) {
	// Distinct transactions identified by pointer. Empty Transaction values
	// compare equal under reflect.DeepEqual, so assertions below rely on
	// pointer identity, not value equality.
	tx1 := &bitcoin.Transaction{}
	tx2 := &bitcoin.Transaction{}
	tx3 := &bitcoin.Transaction{}

	// matches returns a predicate reporting a transaction as unproven when it
	// is one of the given transactions (compared by pointer).
	matches := func(unproven ...*bitcoin.Transaction) func(*bitcoin.Transaction) (bool, error) {
		return func(transaction *bitcoin.Transaction) (bool, error) {
			for _, u := range unproven {
				if transaction == u {
					return true, nil
				}
			}
			return false, nil
		}
	}

	predicateErr := fmt.Errorf("predicate failure")
	chainErr := fmt.Errorf("chain failure")

	tests := map[string]struct {
		transactions     []*bitcoin.Transaction
		isUnproven       func(*bitcoin.Transaction) (bool, error)
		stopAtFirstMatch bool
		chainErr         error
		expectedResult   []*bitcoin.Transaction
		expectedErr      string
	}{
		"returns all matches when not stopping at first match": {
			transactions:     []*bitcoin.Transaction{tx1, tx2, tx3},
			isUnproven:       matches(tx1, tx3),
			stopAtFirstMatch: false,
			expectedResult:   []*bitcoin.Transaction{tx1, tx3},
		},
		"returns only the first match when stopping at first match": {
			transactions:     []*bitcoin.Transaction{tx1, tx2, tx3},
			isUnproven:       matches(tx2, tx3),
			stopAtFirstMatch: true,
			expectedResult:   []*bitcoin.Transaction{tx2},
		},
		"returns nothing when no transaction matches": {
			transactions:     []*bitcoin.Transaction{tx1, tx2, tx3},
			isUnproven:       matches(),
			stopAtFirstMatch: false,
			expectedResult:   nil,
		},
		"propagates the chain error": {
			transactions: []*bitcoin.Transaction{tx1},
			isUnproven:   matches(tx1),
			chainErr:     chainErr,
			expectedErr:  "failed to get transactions for wallet",
		},
		"propagates the predicate error": {
			transactions: []*bitcoin.Transaction{tx1},
			isUnproven: func(*bitcoin.Transaction) (bool, error) {
				return false, predicateErr
			},
			expectedErr: "failed to check if transaction is unproven",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			btcChain := &stubTransactionChain{
				localBitcoinChain: newLocalBitcoinChain(),
				transactions:      test.transactions,
				err:               test.chainErr,
			}

			result, err := collectUnprovenWalletTransactions(
				[20]byte{},
				len(test.transactions),
				btcChain,
				test.isUnproven,
				test.stopAtFirstMatch,
			)

			if test.expectedErr != "" {
				if result != nil {
					t.Errorf(
						"expected nil result on error, got [%v]",
						result,
					)
				}
				if err == nil {
					t.Fatalf(
						"expected error containing [%s], got nil",
						test.expectedErr,
					)
				}
				if !strings.Contains(err.Error(), test.expectedErr) {
					t.Errorf(
						"unexpected error\nexpected to contain: [%s]\nactual:              [%v]",
						test.expectedErr,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			testutils.AssertIntsEqual(
				t,
				"number of unproven transactions",
				len(test.expectedResult),
				len(result),
			)

			for i, expected := range test.expectedResult {
				if result[i] != expected {
					t.Errorf(
						"unexpected transaction at index [%d]; "+
							"pointer identity mismatch",
						i,
					)
				}
			}
		})
	}
}
