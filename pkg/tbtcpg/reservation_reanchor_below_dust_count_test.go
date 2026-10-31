package tbtcpg_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// partiallyReadableCustodyChain models the chain adapter reporting an empty
// reservation key set for a wallet that still custodies reservations.
// WalletReservations derives the key set from event logs filtered through
// per-reservation lookups and skips a lookup that fails, so a transient RPC
// failure on the wallet's only reservation produces exactly this shape: an
// empty key list beside a non-zero count. It also models a failing count
// read, the other way the authoritative custody figure can be unavailable.
type partiallyReadableCustodyChain struct {
	*reservationReanchorLocalChain

	// emptyCustodyRead, when true, makes WalletReservations report no keys
	// regardless of the reservations the wallet custodies on this chain.
	emptyCustodyRead bool
	// countErr, when non-nil, makes WalletReservationsCount fail.
	countErr error
}

func (pcrc *partiallyReadableCustodyChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	if pcrc.emptyCustodyRead {
		return nil, nil
	}
	return pcrc.reservationReanchorLocalChain.WalletReservations(
		walletPublicKeyHash,
	)
}

func (pcrc *partiallyReadableCustodyChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	if pcrc.countErr != nil {
		return 0, pcrc.countErr
	}
	return pcrc.reservationReanchorLocalChain.WalletReservationsCount(
		walletPublicKeyHash,
	)
}

// newBelowDustCountFixture wires a reservation-touched MovingFunds wallet
// whose main UTXO is below the moving funds dust threshold, holding one
// reservation under the given key, on a chain whose custody reads are then
// degraded by the caller. Everything except the reservation reads is
// satisfied, so a below-dust notification can only be suppressed by the
// reservation-count gate.
func newBelowDustCountFixture(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	reservationKey *big.Int,
) (*partiallyReadableCustodyChain, *tbtcpg.LocalBitcoinChain) {
	t.Helper()

	tbtcChain := newReservationReanchorLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	const mainUtxoValue = int64(500000)

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
		1000000, // dust threshold, above the wallet's main UTXO
		1000000, 0, 0, nil, 0, 0, 0, 0, nil, 0,
	)
	tbtcChain.SetWalletReservations(
		walletPublicKeyHash,
		[]*big.Int{reservationKey},
	)
	tbtcChain.markReservationTouched(walletPublicKeyHash)

	return &partiallyReadableCustodyChain{
		reservationReanchorLocalChain: tbtcChain,
	}, btcChain
}

// TestReservationReanchorTask_EmptyCustodyReadRequiresZeroCount pins the
// authoritative-count gate in front of the below-dust notification: an
// empty reservation key set is not proof the wallet is drained, because the
// custody read skips a reservation whose lookup failed instead of failing
// the read. Notifying on a partial read starts the wallet's closing clock
// while it still holds anchors, before the reservations were handed off.
//
// The notification must fire only when the on-chain reservation count -
// the contract's own custody figure - is zero, and an unavailable count must
// defer the notification to a later pass rather than assume emptiness.
func TestReservationReanchorTask_EmptyCustodyReadRequiresZeroCount(t *testing.T) {
	walletPublicKeyHash := hexToByte20("ffb3f7538bfa98a511495dd96027cfbd57baf2fa")
	reservationKey := big.NewInt(9001)

	t.Run("empty custody read beside a non-zero count: no notification", func(t *testing.T) {
		tbtcChain, btcChain := newBelowDustCountFixture(
			t,
			walletPublicKeyHash,
			reservationKey,
		)
		tbtcChain.emptyCustodyRead = true

		// The count the degraded read must agree with: the wallet still
		// custodies its reservation even though the key set came back
		// empty.
		if count, err := tbtcChain.WalletReservationsCount(walletPublicKeyHash); err != nil || count != 1 {
			t.Fatalf(
				"fixture precondition failed: count=[%d] error=[%v], want 1",
				count,
				err,
			)
		}

		task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)
		proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || proposal != nil {
			t.Fatalf(
				"expected no proposal, got ok=%v, proposal=%v",
				ok,
				proposal,
			)
		}

		if notifications := tbtcChain.GetBelowDustNotifications(); len(notifications) != 0 {
			t.Fatalf(
				"expected no below-dust notification while the wallet still "+
					"custodies a reservation, got %d",
				len(notifications),
			)
		}
	})

	t.Run("unavailable count: no notification", func(t *testing.T) {
		tbtcChain, btcChain := newBelowDustCountFixture(
			t,
			walletPublicKeyHash,
			reservationKey,
		)
		tbtcChain.countErr = errors.New("simulated transient RPC failure")
		// The wallet appears drained to the custody read, which is exactly
		// the situation in which the count is consulted.
		tbtcChain.emptyCustodyRead = true

		task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)
		proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || proposal != nil {
			t.Fatalf(
				"expected no proposal, got ok=%v, proposal=%v",
				ok,
				proposal,
			)
		}

		if notifications := tbtcChain.GetBelowDustNotifications(); len(notifications) != 0 {
			t.Fatalf(
				"expected no below-dust notification while the custody count "+
					"is unavailable, got %d",
				len(notifications),
			)
		}
	})
}
