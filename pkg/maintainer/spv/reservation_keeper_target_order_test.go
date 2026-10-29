package spv

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// An aged target must prevent false risk alerts under every wallet-cache order.
func TestReservationKeeperObserver_AgedTargetFoundPastIneligibleLiveWallets(t *testing.T) {
	spvChain, observer, recorder, head, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(
		tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
	)

	source := [20]byte{1}
	spvChain.setWallet(source, &tbtc.WalletChainData{
		State: tbtc.StateMovingFunds,
	})
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: source,
			BlockNumber:         800,
		},
	)

	const ineligibleLiveWallets = 256
	for i := range ineligibleLiveWallets {
		pkh := [20]byte{}
		pkh[0] = byte(i)
		pkh[1] = 0x11
		pkh[2] = byte(i >> 8)
		spvChain.setWallet(pkh, &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: head.Add(-time.Duration(29*24) * time.Hour),
		})
		spvChain.addNewWalletRegisteredEvent(
			&tbtc.NewWalletRegisteredEvent{
				WalletPublicKeyHash: pkh,
				BlockNumber:         810,
			},
		)
	}

	agedTarget := [20]byte{0xfe, 0x22}
	spvChain.setWallet(agedTarget, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
	})
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: agedTarget,
			BlockNumber:         820,
		},
	)

	key := big.NewInt(71)
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	blockCounter.SetCurrentBlock(1050)

	const runs = 64
	for run := range runs {
		if err := observer.ObservationPass(); err != nil {
			t.Fatalf("observation pass %d failed: [%v]", run, err)
		}
		got := recorder.Gauge(
			clientinfo.MetricReservationReanchorNoEligibleTargetCount,
		)
		if got != 0 {
			t.Fatalf(
				"pass %d: no-target count = %v, want 0 (the aged target "+
					"exists behind %d ineligible Live wallets)",
				run,
				got,
				ineligibleLiveWallets,
			)
		}
	}
}
