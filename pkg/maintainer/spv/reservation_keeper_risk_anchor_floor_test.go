package spv

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_AnchorFloorDoesNotWrap pins the anchor
// floor arithmetic of the re-anchor-no-target gauge: the floor is
// minAmount + txMaxFee, and both are governance-set uint64 values with no
// shared upper bound (the contract requires only minAmount > txMaxFee > 0).
// A wrapping addition turns an unattainable floor into a tiny one and makes
// ordinary anchors look re-anchorable, which under-reports reservations that
// in fact have no reachable target. The floor must saturate instead, so
// every anchor below it still counts.
//
// Each case has an eligible aged target and a satisfiable signing window, so
// the no-target count is driven purely by the anchor-floor decision.
func TestReservationKeeperObserver_AnchorFloorDoesNotWrap(t *testing.T) {
	cases := []struct {
		name      string
		minAmount uint64
		txMaxFee  uint64
		// want is the no-target count for a source wallet holding one
		// Active reservation whose anchor is observerAnchorFloorTestAnchor.
		want int
	}{
		{
			// The reporter's pair: the wrap makes the floor 1, so an
			// ordinary anchor clears it although the real floor is
			// unattainable.
			name:      "floor wraps past the uint64 maximum",
			minAmount: math.MaxUint64,
			txMaxFee:  2,
			want:      1,
		},
		{
			// The sum is exactly MaxUint64: no wrap, but still far above
			// any anchor value, so the clamp into int64 applies.
			name:      "floor reaches the uint64 maximum exactly",
			minAmount: math.MaxUint64 - 2,
			txMaxFee:  2,
			want:      1,
		},
		{
			// A zero fee keeps the sum equal to the min amount, the case
			// a wrap-detection guard must not mistake for a wrap.
			name:      "zero fee keeps the plain sum",
			minAmount: 0,
			txMaxFee:  0,
			want:      0,
		},
		{
			name:      "ordinary governance pair",
			minAmount: 100,
			txMaxFee:  20,
			want:      0,
		},
	}

	const anchorValue = int64(1000)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spvChain := newLocalChain()
			head := time.Unix(1_000_000, 0)
			spvChain.setLatestBlockTimestamp(head)

			observer := newObserverForTest(spvChain, 0, newObserverRecorder())

			source := [20]byte{1}
			observer.cachedWallets[source] = &cachedWalletState{
				data: &tbtc.WalletChainData{State: tbtc.StateClosing},
			}
			observer.cachedWallets[[20]byte{2}] = &cachedWalletState{
				data: &tbtc.WalletChainData{
					State:     tbtc.StateLive,
					CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
				},
			}

			key := big.NewInt(1)
			spvChain.setWalletReservations(source, []*big.Int{key})
			spvChain.setReservation(key, &tbtc.Reservation{
				State:      tbtc.ReservationStateActive,
				AnchorUtxo: observerAnchorUtxo(anchorValue),
			})

			params := &tbtc.ReservationParameters{
				ReservationVault:         observerVault,
				ReservationActionTimeout: 24 * 3600,
				ReservationMinAmount:     tc.minAmount,
				ReservationTxMaxFee:      tc.txMaxFee,
			}

			got, err := observer.countReanchorsWithoutTarget(
				params,
				tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
				head,
			)
			if err != nil {
				t.Fatalf("no-target count failed: [%v]", err)
			}
			if got != tc.want {
				t.Errorf(
					"min amount %d, tx max fee %d: no-target count = %d, want %d",
					tc.minAmount,
					tc.txMaxFee,
					got,
					tc.want,
				)
			}
		})
	}
}
