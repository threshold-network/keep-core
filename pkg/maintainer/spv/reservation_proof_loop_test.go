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
// BuildDepositKey), whose sole output is P2WPKH to the custody wallet,
// and whose value is depositAmount - fee (with fee <= TxMaxFee), skip
// transactions with wrong shape, wrong script, or invalid value, and return nil
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

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	otherWalletPKH := [20]byte{99, 99, 99}
	otherWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(otherWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	spvChain.setDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})

	matchingTx := &bitcoin.Transaction{
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

	// Wrong script: pays to a different wallet.
	wrongScriptTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: otherWalletScript,
		}},
	}

	// Wrong value: output value >= depositAmount (zero or negative fee).
	wrongValueTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           160000,
			PublicKeyScript: walletScript,
		}},
	}

	// Excess fee: fee 100000 > TxMaxFee 60000.
	excessFeeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           50000,
			PublicKeyScript: walletScript,
		}},
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
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 50000, PublicKeyScript: walletScript},
			{Value: 50000, PublicKeyScript: walletScript},
		},
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
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: walletScript,
		}},
	}

	event := &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		WalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:            60000,
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

	t.Run("skips transaction with wrong output script", func(t *testing.T) {
		found, err := findReservationAcceptanceTransaction(
			spvChain,
			event,
			[]*bitcoin.Transaction{wrongScriptTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for wrong script transaction, got %v", found)
		}
	})

	t.Run("skips transaction with wrong output value", func(t *testing.T) {
		found, err := findReservationAcceptanceTransaction(
			spvChain,
			event,
			[]*bitcoin.Transaction{wrongValueTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for wrong value transaction, got %v", found)
		}
	})

	t.Run("skips transaction with excess fee", func(t *testing.T) {
		found, err := findReservationAcceptanceTransaction(
			spvChain,
			event,
			[]*bitcoin.Transaction{excessFeeTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for excess fee transaction, got %v", found)
		}
	})
}

// TestFindReservationReanchorTransaction verifies the re-anchor transaction
// matcher: it must find the 1-input-1-output transaction whose sole input
// spends the reservation's current anchor UTXO outpoint exactly, whose sole output
// is P2WPKH to the target wallet, and whose value is anchorUtxo.Value - fee
// (with fee <= TxMaxFee), skip wrong-shape, wrong-script, or invalid-value
// transactions, and return nil when nothing matches.
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

	targetWalletPKH := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	targetWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	otherPKH := [20]byte{99, 99, 99}
	otherScript, err := bitcoin.PayToWitnessPublicKeyHash(otherPKH)
	if err != nil {
		t.Fatal(err)
	}

	matchingTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	wrongScriptTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: otherScript,
		}},
	}

	wrongValueTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	excessFeeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           500000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	// Same transaction hash, wrong output index: must not match.
	wrongIndexTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	wrongShapeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 300000, PublicKeyScript: targetWalletScript},
			{Value: 290000, PublicKeyScript: targetWalletScript},
		},
	}

	event := &tbtc.ReservationReanchorRequestedEvent{
		TargetWalletPublicKeyHash: targetWalletPKH,
		TxMaxFee:                  20000,
	}

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

	t.Run("skips transaction with wrong output script", func(t *testing.T) {
		found, err := findReservationReanchorTransaction(
			event,
			anchorUtxo,
			[]*bitcoin.Transaction{wrongScriptTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for wrong script transaction, got %v", found)
		}
	})

	t.Run("skips transaction with wrong output value", func(t *testing.T) {
		found, err := findReservationReanchorTransaction(
			event,
			anchorUtxo,
			[]*bitcoin.Transaction{wrongValueTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for wrong value transaction, got %v", found)
		}
	})

	t.Run("skips transaction with excess fee", func(t *testing.T) {
		found, err := findReservationReanchorTransaction(
			event,
			anchorUtxo,
			[]*bitcoin.Transaction{excessFeeTx},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for excess fee transaction, got %v", found)
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
// fires with the correct (reservationKey, requestNonce) pair on the first pass.
// On a second pass with the on-chain action state mutated to Settled, it
// asserts no second submission occurs and the event is evicted from the pending map.
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

	spvChain.setDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})

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
	scanState := newReservationProofScanState()

	if err := proveReservationAcceptanceActions(
		scanState,
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

	// Second pass: action transitions to Settled. Verify it is not resubmitted
	// and is evicted from the pending map.
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStateSettled,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
		},
	)

	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
	); err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}

	if submissions != 1 {
		t.Errorf("expected submissions to remain 1 on second pass, got %d", submissions)
	}
	key := reservationEventKey(reservationKey, requestNonce)
	if _, exists := scanState.pendingAcceptanceEvents[key]; exists {
		t.Errorf("expected settled event to be evicted from pendingAcceptanceEvents")
	}
}

// TestProveReservationReanchorActions is an end-to-end test of the
// top-level orchestration function wired into production via
// runReservationProofLoop: it seeds a requested event, a matching
// reservation with an anchor UTXO, a matching pending action, and a
// matching wallet transaction, then asserts the submit hook fires with the
// correct (reservationKey, requestNonce) pair on the first pass.
// On a second pass with the on-chain action state mutated to Settled, it
// asserts no second submission occurs and the event is evicted from the pending map.
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
	scanState := newReservationProofScanState()

	if err := proveReservationReanchorActions(
		scanState,
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

	// Second pass: action transitions to Settled. Verify it is not resubmitted
	// and is evicted from the pending map.
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStateSettled,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
		},
	)

	if err := proveReservationReanchorActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
	); err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}

	if submissions != 1 {
		t.Errorf("expected submissions to remain 1 on second pass, got %d", submissions)
	}
	key := reservationEventKey(reservationKey, requestNonce)
	if _, exists := scanState.pendingReanchorEvents[key]; exists {
		t.Errorf("expected settled event to be evicted from pendingReanchorEvents")
	}
}

func TestProveReservationAcceptanceActions_EvictsOnExceededRetries(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newLocalBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(12345)
	const requestNonce = 1

	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: [20]byte{1},
		BlockNumber:         500,
	})
	// Intentionally do NOT set the reservation action on spvChain, so GetReservationAction fails.

	scanState := newReservationProofScanState()
	config := Config{TransactionLimit: 100}
	key := reservationEventKey(reservationKey, requestNonce)

	for i := uint(1); i < maxReservationActionLoadRetries; i++ {
		if err := proveReservationAcceptanceActions(
			scanState,
			config,
			spvChain,
			spvChain,
			btcChain,
		); err != nil {
			t.Fatalf("unexpected error on pass %d: %v", i, err)
		}

		if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
			t.Fatalf("expected event to remain pending on pass %d (retries %d)", i, i)
		}
		if scanState.acceptanceRetries[key] != i {
			t.Errorf("expected retries to be %d, got %d", i, scanState.acceptanceRetries[key])
		}
	}

	// Final pass: should exceed max retries and be evicted.
	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
	); err != nil {
		t.Fatalf("unexpected error on final pass: %v", err)
	}

	if _, exists := scanState.pendingAcceptanceEvents[key]; exists {
		t.Errorf("expected event to be evicted after exceeding max retries")
	}
	if _, exists := scanState.acceptanceRetries[key]; exists {
		t.Errorf("expected retry entry to be cleaned up after eviction")
	}
}

func TestProveReservationReanchorActions_EvictsOnExceededRetries(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newLocalBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(67890)
	const requestNonce = 1

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: [20]byte{2},
		TargetWalletPublicKeyHash: [20]byte{2},
		BlockNumber:               500,
	})
	// Intentionally do NOT set the reservation action on spvChain, so GetReservationAction fails.

	scanState := newReservationProofScanState()
	config := Config{TransactionLimit: 100}
	key := reservationEventKey(reservationKey, requestNonce)

	for i := uint(1); i < maxReservationActionLoadRetries; i++ {
		if err := proveReservationReanchorActions(
			scanState,
			config,
			spvChain,
			spvChain,
			btcChain,
		); err != nil {
			t.Fatalf("unexpected error on pass %d: %v", i, err)
		}

		if _, exists := scanState.pendingReanchorEvents[key]; !exists {
			t.Fatalf("expected event to remain pending on pass %d (retries %d)", i, i)
		}
		if scanState.reanchorRetries[key] != i {
			t.Errorf("expected retries to be %d, got %d", i, scanState.reanchorRetries[key])
		}
	}

	// Final pass: should exceed max retries and be evicted.
	if err := proveReservationReanchorActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
	); err != nil {
		t.Fatalf("unexpected error on final pass: %v", err)
	}

	if _, exists := scanState.pendingReanchorEvents[key]; exists {
		t.Errorf("expected event to be evicted after exceeding max retries")
	}
	if _, exists := scanState.reanchorRetries[key]; exists {
		t.Errorf("expected retry entry to be cleaned up after eviction")
	}
}
