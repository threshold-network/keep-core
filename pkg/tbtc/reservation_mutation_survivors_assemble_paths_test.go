// Behavior tests for the reservation transaction assembly error returns that
// the 2026-10-17 mutation run left unguarded. Each test drives the real code
// path and fails under the specific mutated build that dropped the pinned
// error branch; all pass on the original code.
package tbtc

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// TestAssembleReservationAnchorTransaction_RejectsMalformedDepositor
// pins the deposit.Script() error branch of
// AssembleReservationAnchorTransaction: a depositor that does not decode
// to 20 bytes must fail the assembly with the wrap text, not build an
// input with a nil redeem script.
func TestAssembleReservationAnchorTransaction_RejectsMalformedDepositor(t *testing.T) {
	walletPKH := [20]byte{0x07}

	btcChain := newLocalBitcoinChain()
	// The funding output is a spendable P2WSH locking script, so the
	// only failure available to the builder is the deposit script.
	depositForScript := &Deposit{
		Depositor:           reservationTestDepositor,
		WalletPublicKeyHash: walletPKH,
	}
	depositScript, err := depositForScript.Script()
	if err != nil {
		t.Fatalf("cannot build the deposit script: [%v]", err)
	}
	fundingScript, err := bitcoin.PayToWitnessScriptHash(
		sha256.Sum256(depositScript),
	)
	if err != nil {
		t.Fatalf("cannot build the funding script: [%v]", err)
	}
	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: fundingScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatalf("cannot broadcast the funding transaction: [%v]", err)
	}
	fundingTxHash := fundingTx.Hash()

	badDeposit := &Deposit{
		Depositor:           malformedDepositor,
		WalletPublicKeyHash: walletPKH,
		Utxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
			Value: 100000,
		},
	}

	_, err = AssembleReservationAnchorTransaction(
		btcChain,
		badDeposit,
		walletPKH,
		&ReservationAction{
			ActionType: ReservationActionTypeAcceptance,
			State:      ReservationActionStatePending,
			TxMaxFee:   2000,
		},
		1500,
	)
	if err == nil {
		t.Fatal("expected the deposit script error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot get deposit script") {
		t.Fatalf("expected the deposit script wrap text, got: [%v]", err)
	}
}

// TestAssembleReservationAnchorTransaction_RejectsNonScriptHashOutput
// pins the AddScriptHashInput error branch of
// AssembleReservationAnchorTransaction: a funding output locked with a
// script that is not P2SH/P2WSH must fail the assembly with the wrap
// text, not return an input-less builder.
func TestAssembleReservationAnchorTransaction_RejectsNonScriptHashOutput(t *testing.T) {
	walletPKH := [20]byte{0x07}

	btcChain := newLocalBitcoinChain()
	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: nonStandardScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatalf("cannot broadcast the funding transaction: [%v]", err)
	}
	fundingTxHash := fundingTx.Hash()

	deposit := &Deposit{
		Depositor:           reservationTestDepositor,
		WalletPublicKeyHash: walletPKH,
		Utxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
			Value: 100000,
		},
	}

	_, err := AssembleReservationAnchorTransaction(
		btcChain,
		deposit,
		walletPKH,
		&ReservationAction{
			ActionType: ReservationActionTypeAcceptance,
			State:      ReservationActionStatePending,
			TxMaxFee:   2000,
		},
		1500,
	)
	if err == nil {
		t.Fatal("expected the input error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot add input pointing to deposit UTXO") {
		t.Fatalf("expected the input wrap text, got: [%v]", err)
	}
}

// TestAssembleReservationReanchorTransaction_RejectsNonPublicKeyHashAnchor
// pins the AddPublicKeyHashInput error branch of
// AssembleReservationReanchorTransaction: an anchor UTXO locked with a
// script that is not P2PKH/P2WPKH must fail the assembly with the wrap
// text, not return an input-less builder.
func TestAssembleReservationReanchorTransaction_RejectsNonPublicKeyHashAnchor(t *testing.T) {
	targetPKH := [20]byte{0x08}

	btcChain := newLocalBitcoinChain()
	badAnchorScript, err := bitcoin.PayToWitnessScriptHash([32]byte{0x22})
	if err != nil {
		t.Fatalf("cannot build the bad anchor script: [%v]", err)
	}
	anchorTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: badAnchorScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatalf("cannot broadcast the anchor transaction: [%v]", err)
	}
	anchorTxHash := anchorTx.Hash()

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     0,
		},
		Value: 100000,
	}

	_, err = AssembleReservationReanchorTransaction(
		btcChain,
		anchorUtxo,
		targetPKH,
		&ReservationAction{
			ActionType: ReservationActionTypeReanchor,
			State:      ReservationActionStatePending,
			TxMaxFee:   2000,
		},
		1500,
	)
	if err == nil {
		t.Fatal("expected the input error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot add input pointing to anchor UTXO") {
		t.Fatalf("expected the input wrap text, got: [%v]", err)
	}
}
