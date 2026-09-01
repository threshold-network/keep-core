package spv

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationProofScanStartBlock covers the bounded look-back arithmetic:
// the very first scan (current block below the look-back window) starts at
// block 0, while a later scan is bounded to exactly
// reservationProofLookBackBlocks behind the current block.
func TestReservationProofScanStartBlock(t *testing.T) {
	tests := map[string]struct {
		currentBlock  uint64
		expectedStart uint64
	}{
		"current block below the look-back window": {
			currentBlock:  1000,
			expectedStart: 0,
		},
		"current block at the look-back window boundary": {
			currentBlock:  reservationProofLookBackBlocks,
			expectedStart: 0,
		},
		"current block beyond the look-back window": {
			currentBlock:  reservationProofLookBackBlocks + 500,
			expectedStart: 500,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			spvChain := newLocalChain()
			blockCounter := newMockBlockCounter()
			blockCounter.SetCurrentBlock(test.currentBlock)
			spvChain.setBlockCounter(blockCounter)

			startBlock, err := reservationProofScanStartBlock(spvChain)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if startBlock != test.expectedStart {
				t.Errorf(
					"unexpected start block\nexpected: %v\nactual:   %v",
					test.expectedStart,
					startBlock,
				)
			}
		})
	}
}

// TestFindReservationAcceptanceTransaction verifies the acceptance
// transaction matcher: it must find the 1-input-1-output transaction whose
// sole input spends the deposit UTXO identified by event.ReservationKey (via
// BuildDepositKey), skip transactions with the wrong shape, and return nil
// when nothing matches.
func TestFindReservationAcceptanceTransaction(t *testing.T) {
	spvChain := newLocalChain()

	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)

	matchingTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 100}},
	}

	// Wrong shape: two outputs, must be skipped even though it otherwise
	// spends the right outpoint.
	wrongShapeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 100}, {Value: 200}},
	}

	// Non-matching: correct shape, different outpoint.
	otherTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	nonMatchingTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: otherTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 100}},
	}

	event := &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey: reservationKey,
	}

	t.Run("finds the matching transaction among candidates", func(t *testing.T) {
		found, err := findReservationAcceptanceTransaction(
			spvChain,
			event,
			[]*bitcoin.Transaction{wrongShapeTx, nonMatchingTx, matchingTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != matchingTx {
			t.Errorf("expected to find the matching transaction, got %v", found)
		}
	})

	t.Run("returns nil when nothing matches", func(t *testing.T) {
		found, err := findReservationAcceptanceTransaction(
			spvChain,
			event,
			[]*bitcoin.Transaction{wrongShapeTx, nonMatchingTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})

	t.Run("returns nil for an empty candidate list", func(t *testing.T) {
		found, err := findReservationAcceptanceTransaction(
			spvChain,
			event,
			nil,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})
}

// TestFindReservationReanchorTransaction verifies the re-anchor transaction
// matcher: it must find the 1-input-1-output transaction whose sole input
// spends the reservation's current anchor UTXO outpoint exactly, skip
// wrong-shape transactions, and return nil when nothing matches.
func TestFindReservationReanchorTransaction(t *testing.T) {
	anchorTxHash, err := bitcoin.NewHashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     1,
		},
		Value: 600000,
	}

	matchingTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 590000}},
	}

	// Same transaction hash, wrong output index: must not match.
	wrongIndexTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 590000}},
	}

	wrongShapeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 300000}, {Value: 290000}},
	}

	event := &tbtc.ReservationReanchorRequestedEvent{}

	t.Run("finds the matching transaction among candidates", func(t *testing.T) {
		found, err := findReservationReanchorTransaction(
			event,
			anchorUtxo,
			[]*bitcoin.Transaction{wrongShapeTx, wrongIndexTx, matchingTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != matchingTx {
			t.Errorf("expected to find the matching transaction, got %v", found)
		}
	})

	t.Run("returns nil when nothing matches", func(t *testing.T) {
		found, err := findReservationReanchorTransaction(
			event,
			anchorUtxo,
			[]*bitcoin.Transaction{wrongShapeTx, wrongIndexTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})
}

// TestProveReservationTransaction covers the submit-vs-skip decision: a
// transaction with enough confirmations and a proof within relay range must
// invoke submit exactly once; a transaction with too few confirmations must
// not invoke submit at all.
func TestProveReservationTransaction(t *testing.T) {
	// Fixture mirrors TestGetProofInfo's "proof entirely within current
	// epoch" case in spv_test.go: factor 6, 20 confirmations, headers
	// spanning the proof window all at the current epoch's difficulty.
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	transaction := &bitcoin.Transaction{}
	transactionHash := transaction.Hash()

	newFixture := func(confirmations uint) (*localChain, *localBitcoinChain) {
		spvChain := newLocalChain()
		btcChain := newLocalBitcoinChain()

		if err := populateBlockHeaders(
			btcChain,
			proofStart,
			proofStart+19,
			func(uint) *big.Int { return diff(32) },
		); err != nil {
			t.Fatal(err)
		}
		btcChain.addTransactionConfirmations(transactionHash, confirmations)

		spvChain.setTxProofDifficultyFactor(big.NewInt(6))
		spvChain.setCurrentEpoch(392)
		spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

		return spvChain, btcChain
	}

	t.Run("submits when confirmations and relay range are sufficient", func(t *testing.T) {
		spvChain, btcChain := newFixture(20)

		submitted := false
		err := proveReservationTransaction(
			transaction,
			btcChain,
			spvChain,
			spvChain,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				submitted = true
				if hash != transaction.Hash() {
					t.Errorf("unexpected submitted hash")
				}
				if requiredConfirmations != 6 {
					t.Errorf(
						"unexpected required confirmations: got %d, want 6",
						requiredConfirmations,
					)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !submitted {
			t.Error("expected submit to be called")
		}
	})

	t.Run("skips without submitting when confirmations are insufficient", func(t *testing.T) {
		spvChain, btcChain := newFixture(2)

		submitted := false
		err := proveReservationTransaction(
			transaction,
			btcChain,
			spvChain,
			spvChain,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				submitted = true
				return nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if submitted {
			t.Error("expected submit not to be called for insufficient confirmations")
		}
	})

	t.Run("propagates submit errors", func(t *testing.T) {
		spvChain, btcChain := newFixture(20)

		err := proveReservationTransaction(
			transaction,
			btcChain,
			spvChain,
			spvChain,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				return fmt.Errorf("submission failed")
			},
		)
		if err == nil {
			t.Fatal("expected submit error to propagate")
		}
	})
}
