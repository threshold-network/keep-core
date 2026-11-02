package spv

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestReservationKeeperObserver_RetiringWalletRegisteredBeforeActivation(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	observer.activationBlock = 900
	spvChain.setWalletParameters(tbtc.WalletParameters{CreationPeriod: 3600})
	source := [20]byte{1}
	spvChain.setWallet(source, &tbtc.WalletChainData{
		State:     tbtc.StateMovingFunds,
		CreatedAt: head.Add(-2 * time.Hour),
	})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		WalletPublicKeyHash: source,
		BlockNumber:         800,
	})
	key := big.NewInt(41)
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: %v", err)
	}
	if got := recorder.Gauge(clientinfo.MetricReservationReanchorNoEligibleTargetCount); got != 1 {
		t.Fatalf("retiring wallet registered before activation must raise the no-target alert: got %v, want 1", got)
	}
}

func TestReservationKeeperObserver_EligibleTargetRegisteredBeforeActivation(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	observer.activationBlock = 900
	spvChain.setWalletParameters(tbtc.WalletParameters{CreationPeriod: 3600})

	source := [20]byte{1}
	spvChain.setWallet(source, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		WalletPublicKeyHash: source,
		BlockNumber:         910,
	})
	target := [20]byte{2}
	spvChain.setWallet(target, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: head.Add(-2 * time.Hour),
	})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		WalletPublicKeyHash: target,
		BlockNumber:         800,
	})
	key := big.NewInt(41)
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: %v", err)
	}
	if got := recorder.Gauge(clientinfo.MetricReservationReanchorNoEligibleTargetCount); got != 0 {
		t.Fatalf("an eligible target registered before activation must prevent the no-target alert: got %v, want 0", got)
	}
}
