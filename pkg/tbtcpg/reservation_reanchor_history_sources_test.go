package tbtcpg_test

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// historySourceChain serves a wallet's reservation history from seeded
// acceptance and re-anchor request events, so each of
// walletHasReservationHistory's three event reads can be driven on its
// own.
type historySourceChain struct {
	*reservationReanchorLocalChain

	reanchorSourceEvents []*tbtc.ReservationReanchorRequestedEvent
	reanchorTargetEvents []*tbtc.ReservationReanchorRequestedEvent
	reanchorEventsErr    error
}

func (h *historySourceChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	if h.reanchorEventsErr != nil {
		return nil, h.reanchorEventsErr
	}
	if filter == nil {
		return nil, errors.New("unfiltered re-anchor requested events query")
	}
	if len(filter.SourceWalletPublicKeyHash) > 0 {
		return h.reanchorSourceEvents, nil
	}
	return h.reanchorTargetEvents, nil
}

// newHistorySourceFixture wires a MovingFunds wallet with a main UTXO
// below the moving funds dust threshold, no reservations left, and no
// acceptance-request events, so its reservation history -- and with it the
// below-dust notification -- is decided by the re-anchor request events the
// test seeds.
func newHistorySourceFixture(
	t *testing.T,
	walletPublicKeyHash [20]byte,
) (*historySourceChain, *tbtcpg.LocalBitcoinChain) {
	t.Helper()

	chain := &historySourceChain{
		reservationReanchorLocalChain: newReservationReanchorLocalChain(),
	}
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	mainUtxoTx := &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           500000,
			PublicKeyScript: walletScript,
		}},
	}
	mainUtxoTxHash := mainUtxoTx.Hash()
	btcChain.SetTransaction(mainUtxoTxHash, mainUtxoTx)
	btcChain.SetTxHashesForPublicKeyHash(
		walletPublicKeyHash,
		[]bitcoin.Hash{mainUtxoTxHash},
	)

	mainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: mainUtxoTxHash,
			OutputIndex:     0,
		},
		Value: 500000,
	}

	chain.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{
		State:        tbtc.StateMovingFunds,
		MainUtxoHash: chain.ComputeMainUtxoHash(mainUtxo),
	})
	chain.SetMovingFundsParameters(
		1000000, 1000000, 0, 0, nil, 0, 0, 0, 0, nil, 0,
	)
	chain.SetWalletReservations(walletPublicKeyHash, nil)

	return chain, btcChain
}

// TestReservationReanchorTask_HistoryFromAcceptanceRequest pins that a
// wallet whose only reservation history is an acceptance request counts as
// touched, so a below-dust MovingFunds wallet that once accepted a
// reservation is notified. The success path of the acceptance-request read
// must return its result rather than an error: turning a successful read
// into a failure leaves every such wallet unnotified.
func TestReservationReanchorTask_HistoryFromAcceptanceRequest(t *testing.T) {
	walletPublicKeyHash := hexToByte20("51b6c3de2b4e0c6b3f0d4e59c8a2f7f2e0b45c11")

	chain, btcChain := newHistorySourceFixture(t, walletPublicKeyHash)
	chain.markReservationTouched(walletPublicKeyHash)

	task := tbtcpg.NewReservationReanchorTask(chain, btcChain)
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifications := chain.GetBelowDustNotifications(); len(notifications) != 1 {
		t.Fatalf(
			"expected 1 below-dust notification for a wallet with an "+
				"acceptance request in its history, got %d",
			len(notifications),
		)
	}
}

// TestReservationReanchorTask_HistoryFromReanchorSource pins the re-anchor
// half of the same decision: a wallet that only ever appeared as the
// source of a re-anchor request also counts as touched. The read must
// report a non-empty result as history; turning a successful read into a
// failure leaves a below-dust wallet unnotified.
func TestReservationReanchorTask_HistoryFromReanchorSource(t *testing.T) {
	walletPublicKeyHash := hexToByte20("51b6c3de2b4e0c6b3f0d4e59c8a2f7f2e0b45c12")

	chain, btcChain := newHistorySourceFixture(t, walletPublicKeyHash)
	chain.reanchorSourceEvents = []*tbtc.ReservationReanchorRequestedEvent{{
		SourceWalletPublicKeyHash: walletPublicKeyHash,
	}}

	task := tbtcpg.NewReservationReanchorTask(chain, btcChain)
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifications := chain.GetBelowDustNotifications(); len(notifications) != 1 {
		t.Fatalf(
			"expected 1 below-dust notification for a wallet that is the "+
				"source of a re-anchor request, got %d",
			len(notifications),
		)
	}
}

// TestReservationReanchorTask_HistoryFromReanchorTarget pins the target half
// of the same decision: a wallet that only ever appeared as the target of a
// re-anchor request also counts as touched. The read must report a
// non-empty result as history; turning a successful read into a failure
// leaves a below-dust wallet unnotified.
func TestReservationReanchorTask_HistoryFromReanchorTarget(t *testing.T) {
	walletPublicKeyHash := hexToByte20("51b6c3de2b4e0c6b3f0d4e59c8a2f7f2e0b45c14")

	chain, btcChain := newHistorySourceFixture(t, walletPublicKeyHash)
	chain.reanchorTargetEvents = []*tbtc.ReservationReanchorRequestedEvent{{
		TargetWalletPublicKeyHash: walletPublicKeyHash,
	}}

	task := tbtcpg.NewReservationReanchorTask(chain, btcChain)
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifications := chain.GetBelowDustNotifications(); len(notifications) != 1 {
		t.Fatalf(
			"expected 1 below-dust notification for a wallet that is the "+
				"target of a re-anchor request, got %d",
			len(notifications),
		)
	}
}

// TestReservationReanchorTask_HistoryReadFailureSkipsNotification pins the
// failure half of the same reads: an event read that fails leaves the
// wallet's history unknown, so the notification is suppressed and retried
// on a later pass rather than decided on partial information.
func TestReservationReanchorTask_HistoryReadFailureSkipsNotification(t *testing.T) {
	walletPublicKeyHash := hexToByte20("51b6c3de2b4e0c6b3f0d4e59c8a2f7f2e0b45c13")

	chain, btcChain := newHistorySourceFixture(t, walletPublicKeyHash)
	chain.reanchorEventsErr = errors.New("event log unavailable")

	task := tbtcpg.NewReservationReanchorTask(chain, btcChain)
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifications := chain.GetBelowDustNotifications(); len(notifications) != 0 {
		t.Fatalf(
			"expected no below-dust notification while the history read "+
				"fails, got %d",
			len(notifications),
		)
	}
}
