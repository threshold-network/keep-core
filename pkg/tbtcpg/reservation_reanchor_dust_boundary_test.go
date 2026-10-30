package tbtcpg_test

import (
	"math"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// newDustBoundaryFixture wires a MovingFunds wallet that the reservation
// subsystem has touched, holds no reservations, and whose main UTXO is the
// given value, against a moving funds dust threshold of the given value.
func newDustBoundaryFixture(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	mainUtxoValue int64,
	dustThreshold uint64,
) (*reservationReanchorLocalChain, *tbtcpg.LocalBitcoinChain) {
	t.Helper()

	tbtcChain := newReservationReanchorLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	mainUtxoTx := &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           mainUtxoValue,
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
		Value: mainUtxoValue,
	}

	tbtcChain.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{
		State:        tbtc.StateMovingFunds,
		MainUtxoHash: tbtcChain.ComputeMainUtxoHash(mainUtxo),
	})
	tbtcChain.SetMovingFundsParameters(
		1000000, dustThreshold, 0, 0, nil, 0, 0, 0, 0, nil, 0,
	)
	tbtcChain.SetWalletReservations(walletPublicKeyHash, nil)
	tbtcChain.markReservationTouched(walletPublicKeyHash)

	return tbtcChain, btcChain
}

// TestReservationReanchorTask_DustThresholdComparisonIsStrict pins the
// below-dust decision at its boundary: a wallet main UTXO whose value
// equals the moving funds dust threshold is not below it. Notifying at
// equality sends NotifyMovingFundsBelowDust for a wallet the Bridge does
// not consider below dust, which reverts the notification and leaves the
// wallet's dust state reported incorrectly.
func TestReservationReanchorTask_DustThresholdComparisonIsStrict(t *testing.T) {
	walletPublicKeyHash := hexToByte20("0fd4c1d1a5b4a37dd2f3b6ad6a2f0f4f0d2e9b71")

	const dustThreshold = uint64(1000000)
	tbtcChain, btcChain := newDustBoundaryFixture(
		t,
		walletPublicKeyHash,
		int64(dustThreshold),
		dustThreshold,
	)
	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

	prop, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || prop != nil {
		t.Fatalf("expected no proposal, got ok=%v prop=%v", ok, prop)
	}

	if notifications := tbtcChain.GetBelowDustNotifications(); len(notifications) != 0 {
		t.Fatalf(
			"expected no below-dust notification for a main UTXO exactly at "+
				"the dust threshold [%d], got %d",
			dustThreshold,
			len(notifications),
		)
	}
}

// TestReservationReanchorTask_DustThresholdAtInt64Max pins the inclusivity
// of the int64-fit guard on the dust threshold: a threshold of
// math.MaxInt64 still fits in int64, so the eligibility check must run and
// report the wallet as below dust. Rejecting it drops the notification for
// every wallet the threshold covers, which for that threshold is all of
// them.
func TestReservationReanchorTask_DustThresholdAtInt64Max(t *testing.T) {
	walletPublicKeyHash := hexToByte20("0fd4c1d1a5b4a37dd2f3b6ad6a2f0f4f0d2e9b72")

	tbtcChain, btcChain := newDustBoundaryFixture(
		t,
		walletPublicKeyHash,
		500000,
		uint64(math.MaxInt64),
	)
	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifications := tbtcChain.GetBelowDustNotifications()
	if len(notifications) != 1 {
		t.Fatalf(
			"expected exactly 1 below-dust notification under a "+
				"max-int64 dust threshold, got %d",
			len(notifications),
		)
	}
	if notifications[0].MainUtxo == nil || notifications[0].MainUtxo.Value != 500000 {
		t.Errorf("unexpected notified main UTXO: %+v", notifications[0].MainUtxo)
	}
}