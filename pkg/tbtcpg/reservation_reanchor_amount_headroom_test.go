package tbtcpg

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReanchorTargetSearch_PickSkipsCandidateWithoutAmountHeadroom is the
// missing regression test for the amount-headroom half.
//
// The re-anchor target search's own amount-headroom pre-check
// (reanchorTargetSearch.pick) must skip a candidate whose reserved amount
// plus the anchor value exceeds maxReservationsAmountPerWallet, without any
// request being sent.
//
// The fake chain's own cap enforcement cannot stand in for the client's
// pre-check: the fake rejects a request whose target's reserved amount plus
// the anchor value exceeds the cap, but that is a rejection after the
// request was sent, so a mutation of the client's pre-check leaves the
// package green. This test drives the pre-check directly, with the fake's
// own cap left unset so only the client-side check can skip the candidate.
func TestReanchorTargetSearch_PickSkipsCandidateWithoutAmountHeadroom(t *testing.T) {
	sourceWalletPublicKeyHash := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	fullTargetWalletPublicKeyHash := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	roomyTargetWalletPublicKeyHash := [20]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	lc.SetBlockCounter(blockCounter)

	lc.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	lc.SetWallet(fullTargetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetWallet(roomyTargetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetLiveWalletsCount(2)

	// The fake's own cap is deliberately left unset: the only thing that
	// can skip the full target here is the client's amount-headroom
	// pre-check.
	fullTargetKey := big.NewInt(7001)
	lc.SetReservation(fullTargetKey, &tbtc.Reservation{
		WalletPublicKeyHash: fullTargetWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{},
			Value:    250000,
		},
		State: tbtc.ReservationStateActive,
	})
	lc.SetWalletReservations(fullTargetWalletPublicKeyHash, []*big.Int{fullTargetKey})

	task := NewReservationReanchorTask(lc, btcChain)

	// The candidates are examined in this order, so the full target is
	// examined first and must be skipped on its amount headroom.
	search := task.newReanchorTargetSearch(
		logger,
		sourceWalletPublicKeyHash,
		5,
		300000,
		time.Unix(1000, 0),
		0,
		false,
	)
	search.candidates = [][20]byte{
		fullTargetWalletPublicKeyHash,
		roomyTargetWalletPublicKeyHash,
	}
	search.createdAt[fullTargetWalletPublicKeyHash] = time.Unix(0, 0)
	search.createdAt[roomyTargetWalletPublicKeyHash] = time.Unix(0, 0)

	// The full target custodies 250000 satoshi and the anchor is 100000,
	// so its reserved amount plus the anchor value (350000) exceeds the
	// cap (300000).
	picked, ok, err := search.pick(100000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected the roomy target to be picked")
	}
	if picked != roomyTargetWalletPublicKeyHash {
		t.Errorf(
			"expected the roomy target [%x] to be picked, got [%x]",
			roomyTargetWalletPublicKeyHash,
			picked,
		)
	}

	// The full target must have been skipped on its amount headroom. With
	// the pre-check mutated away it would be picked first instead, so the
	// roomy target being the picked one is the assertion that fails under
	// that mutation.
	//
	// The skip is not an exclusion: a candidate skipped for amount
	// headroom stays eligible for a later pass, so this asserts only the
	// pick.
	if picked == fullTargetWalletPublicKeyHash {
		t.Error(
			"expected the candidate without amount headroom to be skipped",
		)
	}
}

// TestReanchorTargetSearch_PickKeepsCandidateWithAmountHeadroom pins that
// the amount-headroom pre-check skips on the amount alone: a candidate
// whose reserved amount plus the anchor value stays within the cap is
// picked.
func TestReanchorTargetSearch_PickKeepsCandidateWithAmountHeadroom(t *testing.T) {
	sourceWalletPublicKeyHash := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	targetWalletPublicKeyHash := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}

	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	lc.SetBlockCounter(blockCounter)

	lc.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	lc.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})

	targetKey := big.NewInt(7002)
	lc.SetReservation(targetKey, &tbtc.Reservation{
		WalletPublicKeyHash: targetWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{},
			Value:    250000,
		},
		State: tbtc.ReservationStateActive,
	})
	lc.SetWalletReservations(targetWalletPublicKeyHash, []*big.Int{targetKey})

	task := NewReservationReanchorTask(lc, btcChain)

	search := task.newReanchorTargetSearch(
		logger,
		sourceWalletPublicKeyHash,
		5,
		300000,
		time.Unix(1000, 0),
		0,
		false,
	)
	search.candidates = [][20]byte{targetWalletPublicKeyHash}
	search.createdAt[targetWalletPublicKeyHash] = time.Unix(0, 0)

	// The candidate's reserved amount (250000) plus the anchor value
	// (10000) stays within the cap (300000), so it must be picked.
	picked, ok, err := search.pick(10000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected the candidate to be picked when it has headroom")
	}
	if picked != targetWalletPublicKeyHash {
		t.Errorf(
			"expected the candidate [%x] to be picked, got [%x]",
			targetWalletPublicKeyHash,
			picked,
		)
	}
}
