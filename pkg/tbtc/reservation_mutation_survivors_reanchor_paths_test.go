// Behavior tests for the reservation re-anchor execute-path error returns
// that the 2026-10-17 mutation run left unguarded. Each test drives the real
// code path and fails under the specific mutated build that dropped the
// pinned error branch; all pass on the original code.
package tbtc

import (
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"go.uber.org/zap"
)

// reanchorReadErrorChain is a localChain whose GetReservation and
// GetReservationAction reads fail on demand.
type reanchorReadErrorChain struct {
	*localChain
	reservationErr error
	actionErr      error
}

func (c *reanchorReadErrorChain) GetReservation(
	reservationKey *big.Int,
) (*Reservation, error) {
	if c.reservationErr != nil {
		return nil, c.reservationErr
	}
	return c.localChain.GetReservation(reservationKey)
}

func (c *reanchorReadErrorChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*ReservationAction, error) {
	if c.actionErr != nil {
		return nil, c.actionErr
	}
	return c.localChain.GetReservationAction(reservationKey, requestNonce)
}

// newReanchorActionOver builds a re-anchor action over the given chain
// and Bitcoin chain, wired for sign-step observation like
// newReanchorSignFixture.
func newReanchorActionOver(
	t *testing.T,
	chain Chain,
	btcChain *localBitcoinChain,
	custody wallet,
	targetPKH [20]byte,
) *reservationReanchorAction {
	t.Helper()

	reanchorFee := big.NewInt(
		int64(ProtocolMinWalletTxSatPerVByteFee) * reanchorVsize(t),
	)
	signExecutor := &custodySigningExecutor{custody: custody}
	recorder := newBlockRecorder()

	action := newReservationReanchorAction(
		zap.NewNop().Sugar(),
		chain,
		btcChain,
		custody,
		signExecutor,
		&ReservationReanchorProposal{
			ReservationKey:            big.NewInt(777),
			RequestNonce:              1,
			TargetWalletPublicKeyHash: targetPKH,
			ReanchorTxFee:             reanchorFee,
		},
		300000,
		400000,
		recorder.waitForBlock,
		nil,
	)

	recordingChain := &broadcastRecordingChain{localBitcoinChain: btcChain}
	action.transactionExecutor.btcChain = recordingChain

	return action
}

// TestReservationReanchorAction_RejectsReservationReadError pins the
// GetReservation error return in the re-anchor execute path: a failed
// read must fail the action with the wrap text, not hand a nil
// reservation into the assembly step.
func TestReservationReanchorAction_RejectsReservationReadError(t *testing.T) {
	custody, _ := newCustodyWallet(t)
	targetPKH := [20]byte{0x02}

	chain, btcChain := setupReanchorOnChain(t, custody, targetPKH)
	action := newReanchorActionOver(
		t,
		&reanchorReadErrorChain{
			localChain:     chain,
			reservationErr: errors.New("reservation read failed"),
		},
		btcChain,
		custody,
		targetPKH,
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the reservation read error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot get reservation") {
		t.Fatalf("expected the reservation wrap text, got: [%v]", err)
	}
}

// TestReservationReanchorAction_RejectsActionReadError pins the
// GetReservationAction error return in the re-anchor execute path: a
// failed read must fail the action with the wrap text, not hand a nil
// action into the type check.
func TestReservationReanchorAction_RejectsActionReadError(t *testing.T) {
	custody, _ := newCustodyWallet(t)
	targetPKH := [20]byte{0x02}

	chain, btcChain := setupReanchorOnChain(t, custody, targetPKH)
	action := newReanchorActionOver(
		t,
		&reanchorReadErrorChain{
			localChain: chain,
			actionErr:  errors.New("reservation action read failed"),
		},
		btcChain,
		custody,
		targetPKH,
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the reservation action read error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot get reservation action") {
		t.Fatalf("expected the reservation action wrap text, got: [%v]", err)
	}
}

// TestReservationReanchorAction_RejectsUnspendableAnchorOutput pins the
// AssembleReservationReanchorTransaction error return in the re-anchor
// execute path: an anchor UTXO locked with a script that is not
// P2PKH/P2WPKH must fail the assembly step with the wrap text, not hand
// a nil unsigned transaction to signing.
func TestReservationReanchorAction_RejectsUnspendableAnchorOutput(t *testing.T) {
	custody, _ := newCustodyWallet(t)
	targetPKH := [20]byte{0x02}

	// Rebuild the re-anchor environment with an anchor output that
	// AddPublicKeyHashInput rejects.
	walletPKH := bitcoin.PublicKeyHash(custody.publicKey)
	badAnchorScript, err := bitcoin.PayToWitnessScriptHash([32]byte{0x11})
	if err != nil {
		t.Fatalf("cannot build the bad anchor script: [%v]", err)
	}
	btcChain := newLocalBitcoinChain()
	priorAnchorTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000},
			{Value: 100000, PublicKeyScript: badAnchorScript},
		},
	}
	if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
		t.Fatalf("cannot broadcast the prior anchor transaction: [%v]", err)
	}

	headTime := time.Unix(1_700_000_000, 0)
	chain := Connect()
	chain.setReservation(&Reservation{
		WalletPublicKeyHash: walletPKH,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: priorAnchorTx.Hash(),
				OutputIndex:     1,
			},
			Value: 100000,
		},
	})
	chain.setReservationAction(pendingReanchorActionState(targetPKH))
	chain.setWallet(targetPKH, &WalletChainData{
		CreatedAt: headTime.Add(
			-2 * time.Duration(reanchorTestCreationPeriod) * time.Second,
		),
	})
	chain.SetWalletParameters(WalletParameters{
		CreationPeriod: reanchorTestCreationPeriod,
	})
	chain.SetLatestBlockTimestamp(headTime)

	action := newReanchorActionOver(t, chain, btcChain, custody, targetPKH)

	err = action.execute()
	if err == nil {
		t.Fatal("expected the assembly error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot assemble reservation reanchor transaction") {
		t.Fatalf("expected the assembly wrap text, got: [%v]", err)
	}
}
