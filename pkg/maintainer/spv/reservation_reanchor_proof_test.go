package spv

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

type mockMetricsRecorder struct {
	counts map[string]float64
}

func (m *mockMetricsRecorder) IncrementCounter(name string, value float64) {
	m.counts[name] += value
}

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
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPKH,
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

	metricsRecorder := &mockMetricsRecorder{counts: make(map[string]float64)}
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		metricsRecorder,
	); err != nil {
		t.Fatal(err)
	}
	// Check metrics.
	if count := metricsRecorder.counts["reservation_reanchor_proof_submissions_total"]; count != 1 {
		t.Errorf("unexpected metrics count: got %f, want 1", count)
	}

	// Negative path: nil reservationKey.
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		nil,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		nil,
	); err == nil {
		t.Fatal("expected error for nil reservation key")
	}

	// Negative path: zero requestNonce.
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		0,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		nil,
	); err == nil {
		t.Fatal("expected error for zero request nonce")
	}

	// Negative path: action generation is not Pending.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStateSettled,
		TargetWalletPublicKeyHash: targetWalletPKH,
	})
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		nil,
	); err == nil {
		t.Fatal("expected error for settled action generation")
	}

	// Negative path: action generation is the wrong type.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPKH,
	})
	if err := submitReservationReanchorProof(
		reanchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		nil,
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
		nil,
	); err == nil {
		t.Fatal("expected error for zero required confirmations")
	}
}
