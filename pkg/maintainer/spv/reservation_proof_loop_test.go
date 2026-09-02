package spv

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationProofNextScanRange covers the incremental scan-range
// arithmetic: the very first pass (lastScannedBlock == 0) is bounded to
// reservationProofLookBackBlocks behind the current block (or 0 if the
// chain is younger than that window); every later pass starts exactly one
// block after the previous pass's cursor, so a steady-state loop never
// rescans the full look-back window again.
func TestReservationProofNextScanRange(t *testing.T) {
	tests := map[string]struct {
		currentBlock     uint64
		lastScannedBlock uint64
		expectedStart    uint64
	}{
		"first pass, current block below the look-back window": {
			currentBlock:     1000,
			lastScannedBlock: 0,
			expectedStart:    0,
		},
		"first pass, current block at the look-back window boundary": {
			currentBlock:     reservationProofLookBackBlocks,
			lastScannedBlock: 0,
			expectedStart:    0,
		},
		"first pass, current block beyond the look-back window": {
			currentBlock:     reservationProofLookBackBlocks + 500,
			lastScannedBlock: 0,
			expectedStart:    500,
		},
		"later pass starts one block after the cursor, ignoring the look-back window": {
			currentBlock:     reservationProofLookBackBlocks * 3,
			lastScannedBlock: reservationProofLookBackBlocks * 2,
			expectedStart:    reservationProofLookBackBlocks*2 + 1,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			spvChain := newLocalChain()
			blockCounter := newMockBlockCounter()
			blockCounter.SetCurrentBlock(test.currentBlock)
			spvChain.setBlockCounter(blockCounter)

			startBlock, currentBlock, err := reservationProofNextScanRange(
				spvChain,
				test.lastScannedBlock,
			)
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
			if currentBlock != test.currentBlock {
				t.Errorf(
					"unexpected current block\nexpected: %v\nactual:   %v",
					test.currentBlock,
					currentBlock,
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

// TestProveReservationAcceptanceActions is an end-to-end test of the
// top-level orchestration function wired into production via
// runReservationProofLoop: it seeds a requested event, a matching pending
// action, and a matching wallet transaction, then asserts the submit hook
// fires with the correct (reservationKey, requestNonce) pair. A
// regression that swapped the acceptance and re-anchor submitters (or
// mixed up their arguments) would show up here, not just in the
// lower-level helper unit tests above.
func TestProveReservationAcceptanceActions(t *testing.T) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

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
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{Value: 150000}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}
	fundingTxHash := fundingTx.Hash()
	reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)
	const requestNonce = 1

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(
		transaction.Hash(),
		20,
	); err != nil {
		t.Fatal(err)
	}
	btcChain.setCoinbaseTxHash(transaction.Hash())

	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: walletPublicKeyHash,
		BlockNumber:         500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
		},
	)

	var submittedReservationKey *big.Int
	var submittedRequestNonce uint64
	submissions := 0
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		submissions++
		submittedReservationKey = reservationKey
		submittedRequestNonce = requestNonce
		return nil
	}

	config := Config{TransactionLimit: 100}

	if err := proveReservationAcceptanceActions(
		newReservationProofScanState(),
		config,
		spvChain,
		spvChain,
		btcChain,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if submissions != 1 {
		t.Fatalf("expected exactly one proof submission, got %d", submissions)
	}
	if submittedReservationKey.Cmp(reservationKey) != 0 {
		t.Errorf(
			"unexpected submitted reservation key\nexpected: %v\nactual:   %v",
			reservationKey,
			submittedReservationKey,
		)
	}
	if submittedRequestNonce != requestNonce {
		t.Errorf(
			"unexpected submitted request nonce\nexpected: %d\nactual:   %d",
			requestNonce,
			submittedRequestNonce,
		)
	}

	// Regression test: when the reservation action for a discovered transaction
	// is no longer Pending at submission time, zero submissions occur.
	t.Run("skip when action no longer pending", func(t *testing.T) {
		const proofStart = 790270
		diff := func(d int64) *big.Int { return big.NewInt(d) }

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
		spvChain.setTxProofDifficultyFactor(big.NewInt(6))
		spvChain.setCurrentEpoch(392)
		spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(1000)
		spvChain.setBlockCounter(blockCounter)

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{Value: 150000}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()
		reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)
		const requestNonce = 1

		walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
		walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}

		transaction := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: fundingTxHash,
					OutputIndex:     0,
				},
			}},
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           100000,
				PublicKeyScript: walletScript,
			}},
		}
		if err := btcChain.BroadcastTransaction(transaction); err != nil {
			t.Fatal(err)
		}
		if err := btcChain.addTransactionConfirmations(
			transaction.Hash(),
			20,
		); err != nil {
			t.Fatal(err)
		}
		btcChain.setCoinbaseTxHash(transaction.Hash())

		// Set up a timed-out action (not pending)
		spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      reservationKey,
			RequestNonce:        requestNonce,
			WalletPublicKeyHash: walletPublicKeyHash,
			BlockNumber:         500,
		})
		spvChain.setReservationAction(
			reservationKey,
			requestNonce,
			&tbtc.ReservationAction{
				State:                     tbtc.ReservationActionStateTimedOut, // Not pending!
				ActionType:                tbtc.ReservationActionTypeAcceptance,
				TargetWalletPublicKeyHash: walletPublicKeyHash,
			},
		)

		submissions := 0
		spvChain.submitReservationProofHook = func(
			proofType uint8,
			txInfo *tbtc.BitcoinTxInfo,
			proof *tbtc.BitcoinTxProof,
			mainUtxo *tbtc.BitcoinTxUTXO,
			reservationKey *big.Int,
			requestNonce uint64,
		) error {
			submissions++
			return nil
		}

		config := Config{TransactionLimit: 100}

		if err := proveReservationAcceptanceActions(
			newReservationProofScanState(),
			config,
			spvChain,
			spvChain,
			btcChain,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have zero submissions because action is not pending
		if submissions != 0 {
			t.Fatalf("expected zero proofs submissions when action is not pending, got %d", submissions)
		}
	})
}

// TestProveReservationReanchorActions is an end-to-end test of the
// top-level orchestration function wired into production via
// runReservationProofLoop: it seeds a requested event, a matching
// reservation with an anchor UTXO, a matching pending action, and a
// matching wallet transaction, then asserts the submit hook fires with the
// correct (reservationKey, requestNonce) pair. A regression that swapped
// the acceptance and re-anchor submitters (or mixed up their arguments)
// would show up here, not just in the lower-level helper unit tests above.
func TestProveReservationReanchorActions(t *testing.T) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

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
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(424242)
	const requestNonce = 2

	priorAnchorTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000},
			{Value: 600000},
		},
	}
	if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
		t.Fatal(err)
	}
	anchorTxHash := priorAnchorTx.Hash()
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     1,
		},
		Value: 600000,
	}

	sourceWalletPublicKeyHash := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(
		transaction.Hash(),
		20,
	); err != nil {
		t.Fatal(err)
	}
	btcChain.setCoinbaseTxHash(transaction.Hash())

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
		TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
		BlockNumber:               500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
		},
	)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		AnchorUtxo: anchorUtxo,
	})

	var submittedReservationKey *big.Int
	var submittedRequestNonce uint64
	submissions := 0
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		submissions++
		submittedReservationKey = reservationKey
		submittedRequestNonce = requestNonce
		return nil
	}

	config := Config{TransactionLimit: 100}

	if err := proveReservationReanchorActions(
		newReservationProofScanState(),
		config,
		spvChain,
		spvChain,
		btcChain,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if submissions != 1 {
		t.Fatalf("expected exactly one proof submission, got %d", submissions)
	}
	if submittedReservationKey.Cmp(reservationKey) != 0 {
		t.Errorf(
			"unexpected submitted reservation key\nexpected: %v\nactual:   %v",
			reservationKey,
			submittedReservationKey,
		)
	}
	if submittedRequestNonce != requestNonce {
		t.Errorf(
			"unexpected submitted request nonce\nexpected: %d\nactual:   %d",
			requestNonce,
			submittedRequestNonce,
		)
	}

	// Regression test: when the reservation action for a discovered transaction
	// is no longer Pending at submission time, zero submissions occur.
	t.Run("skip when action no longer pending", func(t *testing.T) {
		const proofStart = 790270
		diff := func(d int64) *big.Int { return big.NewInt(d) }

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
		spvChain.setTxProofDifficultyFactor(big.NewInt(6))
		spvChain.setCurrentEpoch(392)
		spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(1000)
		spvChain.setBlockCounter(blockCounter)

		reservationKey := big.NewInt(424242)
		const requestNonce = 2

		priorAnchorTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{
				{Value: 10000},
				{Value: 600000},
			},
		}
		if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
			t.Fatal(err)
		}
		anchorTxHash := priorAnchorTx.Hash()
		anchorUtxo := &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			Value: 600000,
		}

		sourceWalletPublicKeyHash := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
		walletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}

		transaction := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: anchorTxHash,
					OutputIndex:     1,
				},
			}},
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           590000,
				PublicKeyScript: walletScript,
			}},
		}
		if err := btcChain.BroadcastTransaction(transaction); err != nil {
			t.Fatal(err)
		}
		if err := btcChain.addTransactionConfirmations(
			transaction.Hash(),
			20,
		); err != nil {
			t.Fatal(err)
		}
		btcChain.setCoinbaseTxHash(transaction.Hash())

		// Set up a timed-out action (not pending)
		spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            reservationKey,
			RequestNonce:              requestNonce,
			SourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
			TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
			BlockNumber:               500,
		})
		spvChain.setReservationAction(
			reservationKey,
			requestNonce,
			&tbtc.ReservationAction{
				State:                     tbtc.ReservationActionStateTimedOut, // Not pending!
				ActionType:                tbtc.ReservationActionTypeReanchor,
				TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
			},
		)
		spvChain.setReservation(reservationKey, &tbtc.Reservation{
			AnchorUtxo: anchorUtxo,
		})

		submissions := 0
		spvChain.submitReservationProofHook = func(
			proofType uint8,
			txInfo *tbtc.BitcoinTxInfo,
			proof *tbtc.BitcoinTxProof,
			mainUtxo *tbtc.BitcoinTxUTXO,
			reservationKey *big.Int,
			requestNonce uint64,
		) error {
			submissions++
			return nil
		}

		config := Config{TransactionLimit: 100}

		if err := proveReservationReanchorActions(
			newReservationProofScanState(),
			config,
			spvChain,
			spvChain,
			btcChain,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have zero submissions because action is not pending
		if submissions != 0 {
			t.Fatalf("expected zero proofs submissions when action is not pending, got %d", submissions)
		}
	})
}

// TestSubmitReservationReanchorActionProof_UsesTargetWallet verifies that
// submitReservationReanchorActionProof re-checks the action generation
// against event.TargetWalletPublicKeyHash, not
// event.SourceWalletPublicKeyHash. TestProveReservationReanchorActions
// cannot catch a regression that swapped the two fields at the call site:
// this package's local Bitcoin-history test double can only discover a
// transaction via the source wallet's own outputs
// (localBitcoinChain.GetTransactionsForPublicKeyHash matches on output
// script), which forces source and target to coincide by construction in
// any test that goes through discovery. Calling
// submitReservationReanchorActionProof directly with a known transaction
// hash bypasses discovery, so source and target can differ here: the
// installed action authorizes only the target wallet, so passing Source
// instead of Target would make the guard wrongly skip the submission.
func TestSubmitReservationReanchorActionProof_UsesTargetWallet(t *testing.T) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

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
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(555555)
	const requestNonce = 9

	priorAnchorTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000},
			{Value: 600000},
		},
	}
	if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
		t.Fatal(err)
	}
	anchorTxHash := priorAnchorTx.Hash()

	sourceWalletPublicKeyHash := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	targetWalletPublicKeyHash := [20]byte{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(
		transaction.Hash(),
		20,
	); err != nil {
		t.Fatal(err)
	}
	btcChain.setCoinbaseTxHash(transaction.Hash())

	// The on-chain action authorizes only the target wallet - genuinely
	// distinct from the source wallet here, unlike the discovery-bound E2E
	// test above.
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		},
	)

	event := &tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
	}

	submissions := 0
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		submissions++
		return nil
	}

	_, _, requiredConfirmations, err := getProofInfo(transaction.Hash(), btcChain, spvChain, spvChain)
	if err != nil {
		t.Fatalf("failed to get proof info: %v", err)
	}

	if err := submitReservationReanchorActionProof(
		spvChain,
		btcChain,
		event,
		transaction.Hash(),
		requiredConfirmations,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if submissions != 1 {
		t.Fatalf(
			"expected exactly one proof submission using the target wallet, got %d",
			submissions,
		)
	}
}

// TestVerifyReservationActionStillProvable tests the guard that confirms a reservation action
// is still the expected pending generation at submission time.
func TestVerifyReservationActionStillProvable(t *testing.T) {
	tests := map[string]struct {
		setupFunc                         func(*localChain, *big.Int, uint64)
		reservationKey                    *big.Int
		requestNonce                      uint64
		targetWalletPKH                   [20]byte
		expectedActionType                tbtc.ReservationActionType
		expectedTargetWalletPublicKeyHash [20]byte
		expectedStillProvable             bool
		expectedWantErr                   bool
		description                       string
	}{
		"happy path": {
			setupFunc: func(lc *localChain, reservationKey *big.Int, requestNonce uint64) {
				lc.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
					ActionType:                tbtc.ReservationActionTypeReanchor,
					State:                     tbtc.ReservationActionStatePending,
					TargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03},
				})
			},
			reservationKey:                    big.NewInt(1),
			requestNonce:                      uint64(5),
			targetWalletPKH:                   [20]byte{0x01, 0x02, 0x03},
			expectedActionType:                tbtc.ReservationActionTypeReanchor,
			expectedTargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03},
			expectedStillProvable:             true,
			expectedWantErr:                   false,
			description:                       "action generation is still pending, still the expected type, and still targets the expected wallet",
		},
		"stale action generation": {
			setupFunc: func(lc *localChain, reservationKey *big.Int, requestNonce uint64) {
				lc.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
					ActionType:                tbtc.ReservationActionTypeReanchor,
					State:                     tbtc.ReservationActionStateTimedOut,
					TargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03},
				})
			},
			reservationKey:                    big.NewInt(2),
			requestNonce:                      uint64(7),
			targetWalletPKH:                   [20]byte{0x01, 0x02, 0x03},
			expectedActionType:                tbtc.ReservationActionTypeReanchor,
			expectedTargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03},
			expectedStillProvable:             false,
			expectedWantErr:                   false,
			description:                       "action generation is no longer pending (timed out)",
		},
		"wrong action type": {
			setupFunc: func(lc *localChain, reservationKey *big.Int, requestNonce uint64) {
				lc.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
					ActionType:                tbtc.ReservationActionTypeDissolution,
					State:                     tbtc.ReservationActionStatePending,
					TargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03}, // must match expected to isolate ActionType check
				})
			},
			reservationKey:                    big.NewInt(3),
			requestNonce:                      uint64(8),
			targetWalletPKH:                   [20]byte{0x01, 0x02, 0x03},
			expectedActionType:                tbtc.ReservationActionTypeReanchor,
			expectedTargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03},
			expectedStillProvable:             false,
			expectedWantErr:                   false,
			description:                       "action generation is Pending but for a different action type than expected",
		},
		"mismatched target wallet": {
			setupFunc: func(lc *localChain, reservationKey *big.Int, requestNonce uint64) {
				lc.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
					ActionType:                tbtc.ReservationActionTypeReanchor,
					State:                     tbtc.ReservationActionStatePending,
					TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x33, 0x44},
				})
			},
			reservationKey:                    big.NewInt(4),
			requestNonce:                      uint64(3),
			targetWalletPKH:                   [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e},
			expectedActionType:                tbtc.ReservationActionTypeReanchor,
			expectedTargetWalletPublicKeyHash: [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e},
			expectedStillProvable:             false,
			expectedWantErr:                   false,
			description:                       "action generation targets a different wallet than expected",
		},
		"genuine chain error": {
			setupFunc: func(lc *localChain, reservationKey *big.Int, requestNonce uint64) {
				lc.getReservationActionErr = fmt.Errorf("simulated chain read failure")
			},
			reservationKey:                    big.NewInt(5),
			requestNonce:                      uint64(1),
			targetWalletPKH:                   [20]byte{}, // unused when error expected
			expectedActionType:                tbtc.ReservationActionTypeReanchor,
			expectedTargetWalletPublicKeyHash: [20]byte{}, // unused when error expected
			expectedStillProvable:             false,
			expectedWantErr:                   true,
			description:                       "chain-level error re-fetching the action generation",
		},
		"absent/zero-value action": {
			setupFunc: func(lc *localChain, reservationKey *big.Int, requestNonce uint64) {
				// Install zero value action: ActionType==None, State==Unknown
				lc.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{})
			},
			reservationKey:                    big.NewInt(6),
			requestNonce:                      uint64(2),
			targetWalletPKH:                   [20]byte{0x01, 0x02, 0x03},
			expectedActionType:                tbtc.ReservationActionTypeReanchor, // expecting Reanchor but got None
			expectedTargetWalletPublicKeyHash: [20]byte{0x01, 0x02, 0x03},
			expectedStillProvable:             false,
			expectedWantErr:                   false,
			description:                       "zero-value action models missing on-chain entry (treated as skip)",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			spvChain := newLocalChain()

			if test.setupFunc != nil {
				test.setupFunc(spvChain, test.reservationKey, test.requestNonce)
			}

			stillProvable, err := verifyReservationActionStillProvable(
				spvChain,
				test.reservationKey,
				test.requestNonce,
				test.expectedActionType,
				test.expectedTargetWalletPublicKeyHash,
			)

			if test.expectedWantErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}
				if test.expectedStillProvable {
					t.Fatal("expected error to report unprovable")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stillProvable != test.expectedStillProvable {
				t.Fatalf("unexpected stillProvable value\nexpected: %v\nactual:   %v", test.expectedStillProvable, stillProvable)
			}
		})
	}
}
