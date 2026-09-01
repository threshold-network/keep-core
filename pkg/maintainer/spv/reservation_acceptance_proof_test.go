package spv

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestSubmitReservationAcceptanceProof verifies that
// submitReservationAcceptanceProof correctly parses a 1-input-1-output
// reservation acceptance (anchor) transaction, looks up the matching
// reservation action generation, and submits the SPV proof to the chain.
// It also covers the failure paths for missing action, mismatched action
// type, and wrong target wallet.
func TestSubmitReservationAcceptanceProof(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	// Funding transaction that the anchor transaction spends (the reserved
	// deposit's own UTXO).
	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}
	fundingTxHash := fundingTx.Hash()

	// Anchor transaction: 1 input spending the deposit's funding UTXO, 1
	// output paying the accepting wallet.
	walletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPKH)
	if err != nil {
		t.Fatal(err)
	}

	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
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
		if hash == anchorTx.Hash() && confirmations == requiredConfirmations {
			return anchorTx, proof, nil
		}
		return nil, nil, fmt.Errorf("unexpected proof assembly request")
	}

	reservationKey := big.NewInt(43)
	requestNonce := uint64(1)

	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPKH,
	})

	spvChain.submitReservationProofHook = func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		txProof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		rk *big.Int,
		rn uint64,
	) error {
		if proofType != ProofTypeReservationAcceptance {
			t.Errorf("unexpected proof type: got %d, want %d", proofType, ProofTypeReservationAcceptance)
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

	if err := submitReservationAcceptanceProof(
		anchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		nil,
	); err != nil {
		t.Fatal(err)
	}

	// Negative path: action generation is not Pending.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStateSettled,
		TargetWalletPublicKeyHash: walletPKH,
	})
	if err := submitReservationAcceptanceProof(
		anchorTx.Hash(),
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
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPKH,
	})
	if err := submitReservationAcceptanceProof(
		anchorTx.Hash(),
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

	// Negative path: target wallet public key hash mismatch - the anchor
	// output pays a different wallet than the action authorized.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: [20]byte{0xff},
	})
	if err := submitReservationAcceptanceProof(
		anchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		nil,
	); err == nil {
		t.Fatal("expected error for target wallet public key hash mismatch")
	}

	// Negative path: zero requiredConfirmations.
	if err := submitReservationAcceptanceProof(
		anchorTx.Hash(),
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
