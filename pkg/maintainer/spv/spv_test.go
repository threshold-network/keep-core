package spv

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
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
		expectedSkipReason               proofSkipReason
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

			expectedSkipReason:               proofSkipNone,
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

			expectedSkipReason:               proofSkipNone,
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

			expectedSkipReason:               proofSkipNone,
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

			expectedSkipReason:               proofSkipNone,
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

			expectedSkipReason:               proofSkipNone,
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

			expectedSkipReason:               proofSkipNone,
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

			expectedSkipReason:               proofSkipOutsideRelayRange,
			expectedAccumulatedConfirmations: 0,
			expectedRequiredConfirmations:    0,
		},
		// A run of minimum-difficulty headers longer than DefaultMaxProofHeaders
		// never reaches a decisive header.

		"minimum difficulty run exceeds header bound": {
			transactionConfirmations: 150,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt:       func(uint) *big.Int { return diff(1) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 149,

			expectedSkipReason:               proofSkipExceededMaxHeaders,
			expectedAccumulatedConfirmations: 0,
			expectedRequiredConfirmations:    0,
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

			expectedSkipReason:               proofSkipNone,
			expectedAccumulatedConfirmations: 3,
			expectedRequiredConfirmations:    4,
		},
		// The decisive header matches the current (not previous) epoch on an
		// epoch-spanning proof. Complements the "difficulty drops/raises" cases
		// (which bind to the previous epoch) by exercising the current-epoch
		// binding branch on asymmetric difficulties. Proof starts in the current
		// epoch (32) for two blocks, then drops to the previous epoch's value
		// (16). Required total is 6*32=192; 2*32 + 8*16 = 192 -> 10 headers.
		"decisive header binds current epoch": {
			transactionConfirmations: 31,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < proofStart+2 {
					return diff(32)
				}
				return diff(16)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + 30,

			expectedSkipReason:               proofSkipNone,
			expectedAccumulatedConfirmations: 31,
			expectedRequiredConfirmations:    10,
		},
		// A minimum-difficulty (DIFF1) header appearing after the decisive
		// header is accumulated like any other header and does not re-enter the
		// skip/binding logic (that runs only until the decisive header is
		// found). Decisive header 32 binds requestedDiff; the interior DIFF1
		// contributes its work to the observed difficulty. Required total is
		// 6*32=192; 32 + 1 + 5*32 = 193 >= 192 -> 7 headers.
		"minimum difficulty header after decisive header is counted": {
			transactionConfirmations: 20,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt: func(h uint) *big.Int {
				if h == proofStart+1 {
					return diff(1)
				}
				return diff(32)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + 19,

			expectedSkipReason:               proofSkipNone,
			expectedAccumulatedConfirmations: 20,
			expectedRequiredConfirmations:    7,
		},
		// The decisive header sits exactly at the header bound: 143 leading
		// DIFF1 headers (skipped for binding but contributing 1 each) followed
		// by the decisive header at position DefaultMaxProofHeaders. Required
		// total is 6*16=96; 143*1 + 16 = 159 >= 96 -> exactly 144 headers, at
		// the bound.
		"decisive header exactly at header bound is proven": {
			transactionConfirmations: DefaultMaxProofHeaders,
			currentEpochDifficulty:   diff(16),
			previousEpochDifficulty:  diff(32),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < proofStart+DefaultMaxProofHeaders-1 {
					return diff(1)
				}
				return diff(16)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + DefaultMaxProofHeaders - 1,

			expectedSkipReason:               proofSkipNone,
			expectedAccumulatedConfirmations: DefaultMaxProofHeaders,
			expectedRequiredConfirmations:    DefaultMaxProofHeaders,
		},
		// The decisive header sits one past the header bound: DefaultMaxProofHeaders
		// leading DIFF1 headers exhaust the walk before the decisive header at
		// position DefaultMaxProofHeaders+1 is ever examined. This is the off-by-one
		// companion to the case above and must be signalled as exceeded.
		"decisive header just past header bound is skipped": {
			transactionConfirmations: DefaultMaxProofHeaders + 1,
			currentEpochDifficulty:   diff(16),
			previousEpochDifficulty:  diff(32),
			headerDifficultyAt: func(h uint) *big.Int {
				if h < proofStart+DefaultMaxProofHeaders {
					return diff(1)
				}
				return diff(16)
			},
			headersFrom: proofStart,
			headersTo:   proofStart + DefaultMaxProofHeaders,

			expectedSkipReason:               proofSkipExceededMaxHeaders,
			expectedAccumulatedConfirmations: 0,
			expectedRequiredConfirmations:    0,
		},
		// The chain tip is reached while still skipping leading DIFF1 headers,
		// before any decisive header is bound (requestedDiff is still nil). The
		// proof is within range and the caller is told to wait for one more
		// header than currently exists.
		"chain tip reached before decisive header": {
			transactionConfirmations: 3,
			currentEpochDifficulty:   diff(32),
			previousEpochDifficulty:  diff(16),
			headerDifficultyAt:       func(uint) *big.Int { return diff(1) },
			headersFrom:              proofStart,
			headersTo:                proofStart + 2,

			expectedSkipReason:               proofSkipNone,
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
			// Note the setter's parameter order is (previous, current).
			localChain.setCurrentAndPrevEpochDifficulty(
				test.previousEpochDifficulty,
				test.currentEpochDifficulty,
			)

			accumulatedConfirmations,
				requiredConfirmations,
				skipReason,
				err :=
				getProofInfo(
					transactionHash,
					btcChain,
					localChain,
					localChain,
					DefaultMaxProofHeaders,
				)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"skip reason",
				int(test.expectedSkipReason),
				int(skipReason),
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

// TestGetProofInfo_MinDifficultyDetectedByExactTarget pins the DIFF1 skip
// predicate to exact target equality. The Bridge skips headers whose target
// equals MIN_DIFFICULTY_TARGET, not headers whose computed difficulty rounds
// to 1. These differ: any target in (maxTarget/2, maxTarget] yields
// Difficulty()==1, but only the exact maxTarget is the canonical
// minimum-difficulty target. A header with Difficulty()==1 yet a target below
// maxTarget must NOT be skipped - it is a decisive header.
//
// Here that decisive header matches neither relay epoch, so the Bridge would
// revert and getProofInfo must report proofSkipOutsideRelayRange. If the
// predicate regressed to Difficulty()==1, the header would be skipped as DIFF1
// and the following current-epoch headers would prove the transaction
// (proofSkipNone) - so this case fails loudly on that regression.
func TestGetProofInfo_MinDifficultyDetectedByExactTarget(t *testing.T) {
	const proofStart = 790270

	// A target of 3/4 * maxTarget: Difficulty() floors to 1, but the target is
	// strictly below the minimum-difficulty target. BigToCompact truncates
	// toward zero, so the encoded target can never round up to maxTarget.
	nonMinTarget := new(big.Int).Mul(btcdiff.LightRelayMinDifficultyTarget, big.NewInt(3))
	nonMinTarget.Div(nonMinTarget, big.NewInt(4))
	decisiveHeader := &bitcoin.BlockHeader{
		Bits: blockchain.BigToCompact(nonMinTarget),
	}

	// Guard the construction; without both properties the test proves nothing.
	if decisiveHeader.Difficulty().Cmp(big.NewInt(1)) != 0 {
		t.Fatalf(
			"test header must have difficulty 1, got [%v]",
			decisiveHeader.Difficulty(),
		)
	}
	if decisiveHeader.Target().Cmp(btcdiff.LightRelayMinDifficultyTarget) == 0 {
		t.Fatal(
			"test header target must differ from the minimum-difficulty target",
		)
	}

	transactionHash, err := bitcoin.NewHashFromString(
		"44c568bc0eac07a2a9c2b46829be5b5d46e7d00e17bfb613f506a75ccf86a473",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	btcChain := newLocalBitcoinChain()
	// The first (decisive) header carries Difficulty()==1 with a non-minimum
	// target; the remaining headers carry the current epoch difficulty.
	if err := btcChain.addBlockHeader(proofStart, decisiveHeader); err != nil {
		t.Fatal(err)
	}
	if err := populateBlockHeaders(
		btcChain,
		proofStart+1,
		proofStart+19,
		func(uint) *big.Int { return big.NewInt(32) },
	); err != nil {
		t.Fatal(err)
	}
	btcChain.addTransactionConfirmations(transactionHash, 20)

	localChain := newLocalChain()
	localChain.setTxProofDifficultyFactor(big.NewInt(6))
	localChain.setCurrentEpoch(392)
	// Note the setter's parameter order is (previous, current).
	localChain.setCurrentAndPrevEpochDifficulty(big.NewInt(16), big.NewInt(32))

	_, _, skipReason, err := getProofInfo(
		transactionHash,
		btcChain,
		localChain,
		localChain,
		DefaultMaxProofHeaders,
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"skip reason",
		int(proofSkipOutsideRelayRange),
		int(skipReason),
	)
}

// recordingMetricsRecorder captures IncrementCounter calls for assertions.
// proveTransactions invokes it synchronously, so no locking is needed.
type recordingMetricsRecorder struct {
	counters map[string]float64
}

func (r *recordingMetricsRecorder) IncrementCounter(name string, value float64) {
	r.counters[name] += value
}

// TestProveTransactions covers the caller-side handling of each proofSkipReason
// in proveTransactions. The safety property under test is that a skip reason
// never results in a proof submission, and that an assemblable proof is
// submitted; the per-reason metric counter is asserted as a secondary check.
func TestProveTransactions(t *testing.T) {
	const proofStart = 790270

	// A concrete transaction so proveTransactions can derive a real hash.
	rawTransaction, err := hex.DecodeString(
		"0100000000010110a15e879b7e8b07df62772579a64bf2b409409bbcc8bc2c7f6e39" +
			"31dc615e920100000000ffffffff02042900000000000017a9143ec459d0f3c29286" +
			"ae5df5fcc421e2786024277e87b4121600000000001600148db50eb52063ea9d98b3" +
			"eac91489a90f738986f6024830450221009740ad12d2e74c00ccb4741d533d2ecd69" +
			"02289144c4626508afb61eed790c97022006e67179e8e2a63dc4f1ab758867d8bbfe" +
			"0a2b67682be6dadfa8e07d3b7ba04d012103989d253b17a6a0f41838b84ff0d20e88" +
			"98f9d7b1a98f2564da4cc29dcf8581d900000000",
	)
	if err != nil {
		t.Fatal(err)
	}
	transaction := new(bitcoin.Transaction)
	if err := transaction.Deserialize(rawTransaction); err != nil {
		t.Fatal(err)
	}
	transactionHash := transaction.Hash()

	tests := map[string]struct {
		headerDifficultyAt       func(uint) *big.Int
		headersTo                uint
		transactionConfirmations uint
		expectSubmitted          bool
		expectedCounter          string
	}{
		// Decisive header (difficulty 8) matches neither epoch -> skipped.
		"outside relay range is skipped and metered": {
			headerDifficultyAt:       func(uint) *big.Int { return big.NewInt(8) },
			headersTo:                proofStart + 19,
			transactionConfirmations: 20,
			expectSubmitted:          false,
			expectedCounter:          "spv_proof_skipped_outside_relay_range_total",
		},
		// A run of DIFF1 headers longer than the bound never binds -> skipped.
		"exceeded max headers is skipped and metered": {
			headerDifficultyAt:       func(uint) *big.Int { return big.NewInt(1) },
			headersTo:                proofStart + 149,
			transactionConfirmations: 150,
			expectSubmitted:          false,
			expectedCounter:          "spv_proof_skipped_exceeded_max_headers_total",
		},
		// All headers at the current epoch difficulty -> proof is submitted.
		"assemblable proof is submitted": {
			headerDifficultyAt:       func(uint) *big.Int { return big.NewInt(32) },
			headersTo:                proofStart + 19,
			transactionConfirmations: 20,
			expectSubmitted:          true,
			expectedCounter:          "",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			btcChain := newLocalBitcoinChain()
			if err := populateBlockHeaders(
				btcChain,
				proofStart,
				test.headersTo,
				test.headerDifficultyAt,
			); err != nil {
				t.Fatal(err)
			}
			btcChain.addTransactionConfirmations(
				transactionHash,
				test.transactionConfirmations,
			)

			localChain := newLocalChain()
			localChain.setTxProofDifficultyFactor(big.NewInt(6))
			localChain.setCurrentEpoch(392)
			// Note the setter's parameter order is (previous, current).
			localChain.setCurrentAndPrevEpochDifficulty(
				big.NewInt(16),
				big.NewInt(32),
			)

			recorder := &recordingMetricsRecorder{
				counters: make(map[string]float64),
			}

			sm := &spvMaintainer{
				metricsRecorder: recorder,
				config:          Config{MaxProofHeaders: DefaultMaxProofHeaders},
				spvChain:        localChain,
				btcDiffChain:    localChain,
				btcChain:        btcChain,
			}

			var submitted []bitcoin.Hash
			getter := func(
				uint64,
				int,
				bitcoin.Chain,
				Chain,
			) ([]*bitcoin.Transaction, error) {
				return []*bitcoin.Transaction{transaction}, nil
			}
			submitter := func(
				hash bitcoin.Hash,
				_ uint,
				_ bitcoin.Chain,
				_ Chain,
				metrics MetricsRecorder,
			) error {
				if metrics != recorder {
					t.Fatal("proof submitter did not receive the maintainer recorder")
				}
				submitted = append(submitted, hash)
				return nil
			}

			if err := sm.proveTransactions(getter, submitter); err != nil {
				t.Fatal(err)
			}

			if test.expectSubmitted {
				if len(submitted) != 1 || submitted[0] != transactionHash {
					t.Errorf(
						"expected the transaction to be submitted, "+
							"got submissions [%v]",
						submitted,
					)
				}
			} else if len(submitted) != 0 {
				t.Errorf(
					"expected no submission on skip, got [%d]",
					len(submitted),
				)
			}

			if test.expectedCounter != "" {
				if got := recorder.counters[test.expectedCounter]; got != 1 {
					t.Errorf(
						"expected counter [%s] to be 1, got [%v]",
						test.expectedCounter,
						got,
					)
				}
			}
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
