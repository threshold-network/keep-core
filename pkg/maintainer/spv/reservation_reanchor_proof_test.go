package spv

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestSubmitReservationReanchorProof verifies that submitReservationReanchorProof
// correctly parses a 1-input-1-output re-anchor transaction, looks up the
// matching reservation action generation, and submits the SPV proof to the
// chain. It also covers the failure paths for missing action and mismatched
// action type.
func TestSubmitReservationReanchorProof(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	// Anchor transaction that the re-anchor spends.
	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}
	anchorTxHash := anchorTx.Hash()

	// Re-anchor transaction: 1 input spending anchorTx output 0, 1 output
	// paying to the target wallet.
	targetWalletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	targetScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	reanchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchorTx); err != nil {
		t.Fatal(err)
	}

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
		if hash == reanchorTx.Hash() && confirmations == requiredConfirmations {
			return reanchorTx, proof, nil
		}
		return nil, nil, fmt.Errorf("unexpected proof assembly request")
	}

	reservationKey := big.NewInt(42)
	requestNonce := uint64(7)

	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: targetWalletPKH,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: reanchorTx.Inputs[0].Outpoint,
			Value:    600000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: requestNonce,
	})
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStatePending,
	})

	// Override SubmitReservationProof on the localChain to capture the call.
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		txProof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		rk *big.Int,
		rn uint64,
	) error {
		if proofType != ProofTypeReservationReanchor {
			t.Errorf("unexpected proof type: got %d, want %d", proofType, ProofTypeReservationReanchor)
		}
		if rk == nil || rk.Cmp(reservationKey) != 0 {
			t.Errorf("unexpected reservation key: got %v, want %v", rk, reservationKey)
		}
		if rn != requestNonce {
			t.Errorf("unexpected request nonce: got %d, want %d", rn, requestNonce)
		}
		if mainUtxo == nil {
			t.Fatal("mainUtxo must not be nil")
		}
		if mainUtxo.TxOutputValue != 600000 {
			t.Errorf("unexpected UTXO value: got %d, want %d", mainUtxo.TxOutputValue, 600000)
		}
		if txInfo == nil {
			t.Fatal("txInfo must not be nil")
		}
		if !bytes.Equal(txProof.MerkleProof, proof.MerkleProof) {
			t.Errorf("unexpected merkle proof")
		}
		return nil
	}

	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	); err != nil {
		t.Fatal(err)
	}

	// Negative path: action generation is not Pending.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStateSettled,
	})
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	); err == nil {
		t.Fatal("expected error for settled action generation")
	}

	// Negative path: action generation is the wrong type.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeAcceptance,
		State:      tbtc.ReservationActionStatePending,
	})
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	); err == nil {
		t.Fatal("expected error for wrong action type")
	}

	// Negative path: zero requiredConfirmations.
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		0,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	); err == nil {
		t.Fatal("expected error for zero required confirmations")
	}
}

// TestGetUnprovenReservationReanchorTransactions verifies that discovery
// finds exactly the transaction matching a pending re-anchor request, and
// correctly excludes: (a) requests whose action generation already
// settled, (b) unrelated transactions to the same target wallet that do
// not have the re-anchor shape, and (c) re-anchor-shaped transactions
// whose spent input is not registered as the reservation's anchor.
func TestGetUnprovenReservationReanchorTransactions(t *testing.T) {
	historyDepth := uint64(5)
	transactionLimit := 10
	currentBlock := uint64(1000)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	targetWalletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	targetScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	// Anchor transaction that the re-anchor spends.
	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x01},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}

	// The real re-anchor transaction: spends the anchor, pays the target
	// wallet, one input, one output.
	reanchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchorTx); err != nil {
		t.Fatal(err)
	}

	// An unrelated transaction paying the same target wallet with a second
	// output - does not have the 1-input-1-output re-anchor shape and must
	// be skipped without error.
	wrongShapeTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x02},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000, PublicKeyScript: targetScript},
			{Value: 20000, PublicKeyScript: []byte{}},
		},
	}
	if err := btcChain.BroadcastTransaction(wrongShapeTx); err != nil {
		t.Fatal(err)
	}

	// A same-shape (1-in-1-out) transaction paying the target wallet whose
	// spent input is never registered as any reservation's anchor - must
	// be excluded by the ReservationByAnchorUtxo mismatch, not by shape.
	unrelatedSourceTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x03},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           1000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(unrelatedSourceTx); err != nil {
		t.Fatal(err)
	}
	unregisteredAnchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: unrelatedSourceTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           900,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(unregisteredAnchorTx); err != nil {
		t.Fatal(err)
	}

	reservationKey := big.NewInt(42)
	requestNonce := uint64(7)
	sourceWalletPKH := [20]byte{0xaa}

	spvChain.setReservationByAnchorUtxo(anchorTx.Hash(), 0, reservationKey)
	spvChain.setReservationReanchorRequestedEvents([]*tbtc.ReservationReanchorRequestedEvent{
		{
			ReservationKey:            reservationKey,
			RequestNonce:              requestNonce,
			SourceWalletPublicKeyHash: sourceWalletPKH,
			TargetWalletPublicKeyHash: targetWalletPKH,
		},
	})
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStatePending,
	})

	transactions, err := getUnprovenReservationReanchorTransactions(
		historyDepth,
		transactionLimit,
		btcChain,
		spvChain,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 1 {
		t.Fatalf("expected 1 unproven transaction, got %d", len(transactions))
	}
	if transactions[0].Hash() != reanchorTx.Hash() {
		t.Errorf(
			"unexpected transaction: got %s, want %s",
			transactions[0].Hash(),
			reanchorTx.Hash(),
		)
	}

	// Once the action generation settles, the event must be skipped
	// entirely and discovery must return no transactions.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStateSettled,
	})

	transactions, err = getUnprovenReservationReanchorTransactions(
		historyDepth,
		transactionLimit,
		btcChain,
		spvChain,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 0 {
		t.Fatalf(
			"expected no unproven transactions once settled, got %d",
			len(transactions),
		)
	}
}

// TestSubmitDiscoveredReservationReanchorProof verifies that the discovered-
// transaction submitter re-derives (reservationKey, requestNonce) from the
// transaction's spent anchor outpoint and submits the proof, and that it
// fails cleanly when the outpoint is not registered to any reservation.
func TestSubmitDiscoveredReservationReanchorProof(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}
	anchorTxHash := anchorTx.Hash()

	targetWalletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	targetScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	reanchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchorTx); err != nil {
		t.Fatal(err)
	}

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
		if hash == reanchorTx.Hash() && confirmations == requiredConfirmations {
			return reanchorTx, proof, nil
		}
		return nil, nil, fmt.Errorf("unexpected proof assembly request")
	}

	reservationKey := big.NewInt(42)
	requestNonce := uint64(7)

	spvChain.setReservationByAnchorUtxo(anchorTxHash, 0, reservationKey)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: targetWalletPKH,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: reanchorTx.Inputs[0].Outpoint,
			Value:    600000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: requestNonce,
	})
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPKH,
	})

	var capturedReservationKey *big.Int
	var capturedRequestNonce uint64
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		txProof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		rk *big.Int,
		rn uint64,
	) error {
		capturedReservationKey = rk
		capturedRequestNonce = rn
		return nil
	}

	if err := submitDiscoveredReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	); err != nil {
		t.Fatal(err)
	}

	if capturedReservationKey == nil || capturedReservationKey.Cmp(reservationKey) != 0 {
		t.Errorf(
			"unexpected derived reservation key: got %v, want %v",
			capturedReservationKey,
			reservationKey,
		)
	}
	if capturedRequestNonce != requestNonce {
		t.Errorf(
			"unexpected derived request nonce: got %d, want %d",
			capturedRequestNonce,
			requestNonce,
		)
	}

	// Negative path: the spent outpoint is not registered to any
	// reservation.
	sourceTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x0a},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           2000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(sourceTx); err != nil {
		t.Fatal(err)
	}
	unanchoredTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: sourceTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           1000,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(unanchoredTx); err != nil {
		t.Fatal(err)
	}

	mockSpvProofAssembler2 := func(
		hash bitcoin.Hash,
		confirmations uint,
		btcChain bitcoin.Chain,
	) (*bitcoin.Transaction, *bitcoin.SpvProof, error) {
		return unanchoredTx, proof, nil
	}

	if err := submitDiscoveredReservationReanchorProof(
		unanchoredTx.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler2,
	); err == nil {
		t.Fatal("expected error for unanchored outpoint")
	}
}

// TestSubmitDiscoveredReservationReanchorProof_StaleActionGeneration verifies
// that submission is skipped, not misattributed, when the reservation's
// current action generation has moved past the one that produced the
// discovered transaction (e.g. the original re-anchor action timed out and a
// new, unrelated action generation is now current). This must return a nil
// error (not an error) since an error here would abort the entire
// proveTransactions round for every other in-flight transaction across
// every proof type this tick (spv.go:292-293).
func TestSubmitDiscoveredReservationReanchorProof_StaleActionGeneration(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}

	targetWalletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	targetScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	reanchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchorTx); err != nil {
		t.Fatal(err)
	}

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
		return reanchorTx, proof, nil
	}

	reservationKey := big.NewInt(99)
	staleNonce := uint64(7)
	currentNonce := uint64(8)

	spvChain.setReservationByAnchorUtxo(anchorTx.Hash(), 0, reservationKey)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: targetWalletPKH,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: reanchorTx.Inputs[0].Outpoint,
			Value:    600000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: currentNonce,
	})
	// The action generation that actually produced reanchorTx (staleNonce)
	// timed out; a new, unrelated action generation (currentNonce) is now
	// pending. The reservation's RequestNonce always points at the latest
	// generation, so the discovered transaction must not be paired with it.
	spvChain.setReservationAction(reservationKey, staleNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStateTimedOut,
		TargetWalletPublicKeyHash: targetWalletPKH,
	})
	spvChain.setReservationAction(reservationKey, currentNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeDissolution,
		State:      tbtc.ReservationActionStatePending,
	})

	hookCalled := false
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		txProof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		rk *big.Int,
		rn uint64,
	) error {
		hookCalled = true
		return nil
	}

	err = submitDiscoveredReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error for a stale action generation (the "+
				"caller must not abort the whole proving round for a "+
				"skip), got: %v",
			err,
		)
	}
	if hookCalled {
		t.Fatal("proof must not be submitted for a stale action generation")
	}
}

// TestSubmitDiscoveredReservationReanchorProof_MismatchedTargetWallet
// verifies that submission is skipped when the reservation's current
// pending re-anchor action generation targets a different wallet than the
// one the discovered transaction actually pays - evidence the transaction
// belongs to a superseded generation even though the current generation is
// also, coincidentally, a pending re-anchor. This must return a nil error
// (not an error) since an error here would abort the entire
// proveTransactions round for every other in-flight transaction across
// every proof type this tick (spv.go:292-293).
func TestSubmitDiscoveredReservationReanchorProof_MismatchedTargetWallet(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}

	oldTargetWalletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	oldTargetScript, err := bitcoin.PayToWitnessPublicKeyHash(oldTargetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}
	newTargetWalletPKH := [20]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0x00, 0x12, 0x34, 0x56, 0x78}

	reanchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: oldTargetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchorTx); err != nil {
		t.Fatal(err)
	}

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
		return reanchorTx, proof, nil
	}

	reservationKey := big.NewInt(100)
	requestNonce := uint64(3)

	spvChain.setReservationByAnchorUtxo(anchorTx.Hash(), 0, reservationKey)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: oldTargetWalletPKH,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: reanchorTx.Inputs[0].Outpoint,
			Value:    600000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: requestNonce,
	})
	// A new re-anchor request superseded the one that produced reanchorTx,
	// this time targeting a different wallet, before reanchorTx's proof was
	// submitted.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: newTargetWalletPKH,
	})

	hookCalled := false
	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		txProof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		rk *big.Int,
		rn uint64,
	) error {
		hookCalled = true
		return nil
	}

	err = submitDiscoveredReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error for a mismatched target wallet (the "+
				"caller must not abort the whole proving round for a "+
				"skip), got: %v",
			err,
		)
	}
	if hookCalled {
		t.Fatal("proof must not be submitted for a mismatched action generation")
	}
}
