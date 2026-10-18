package spv

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_SigningWindowBoundaries checks the
// re-anchor signing-window floor against the live action timeout: the
// window is open only when the timeout strictly exceeds the floor. Each
// case carries an eligible target, so the no-target count is driven
// purely by the signing-window decision.
func TestReservationKeeperObserver_SigningWindowBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		timeout uint32
		want    int
	}{
		{name: "below floor", timeout: 2 * 3600, want: 1},
		{name: "at floor", timeout: 2*3600 + 1, want: 1},
		{name: "above floor", timeout: 2*3600 + 2, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spvChain := newLocalChain()
			head := time.Unix(1_000_000, 0)
			spvChain.setLatestBlockTimestamp(head)

			observer := newObserverForTest(
				spvChain,
				0,
				newObserverRecorder(),
			)

			source := [20]byte{1}
			observer.cachedWallets[source] = &cachedWalletState{
				data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
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
				AnchorUtxo: observerAnchorUtxo(1000),
			})

			params := &tbtc.ReservationParameters{
				ReservationVault:         observerVault,
				ReservationActionTimeout: tc.timeout,
				ReservationMinAmount:     100,
				ReservationTxMaxFee:      20,
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
					"action timeout %d s: no-target count = %d, want %d",
					tc.timeout,
					got,
					tc.want,
				)
			}
		})
	}
}

// TestReservationKeeperObserver_AnchorFloorBoundary checks the anchor
// amount floor boundary: an anchor exactly at min amount + max fee does
// not satisfy the strict above-floor check and still counts as
// no-target; one satoshi above it clears the count.
func TestReservationKeeperObserver_AnchorFloorBoundary(t *testing.T) {
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
	params := &tbtc.ReservationParameters{
		ReservationVault:         observerVault,
		ReservationActionTimeout: 24 * 3600,
		ReservationMinAmount:     100,
		ReservationTxMaxFee:      20,
	}
	walletParams := tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600}

	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(120), // exactly at the floor
	})
	got, err := observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 1 {
		t.Errorf("anchor at the floor: no-target count = %d, want 1", got)
	}

	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(121), // one above the floor
	})
	got, err = observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 0 {
		t.Errorf("anchor one above the floor: no-target count = %d, want 0", got)
	}
}

// TestReservationKeeperObserver_NilAnchorUtxoCounts checks that an
// Active reservation without an anchor UTXO does not satisfy the anchor
// check and counts toward the no-target gauge (with every other check
// satisfied, so the count is driven purely by the missing anchor), and
// that the missing anchor is not a dereference hazard.
func TestReservationKeeperObserver_NilAnchorUtxoCounts(t *testing.T) {
	spvChain := newLocalChain()
	head := time.Unix(1_000_000, 0)
	spvChain.setLatestBlockTimestamp(head)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
	}
	observer.cachedWallets[[20]byte{2}] = &cachedWalletState{
		data: &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
		},
	}

	key := big.NewInt(1)
	spvChain.setWalletReservations(source, []*big.Int{key})
	params := &tbtc.ReservationParameters{
		ReservationVault:         observerVault,
		ReservationActionTimeout: 24 * 3600,
		ReservationMinAmount:     100,
		ReservationTxMaxFee:      20,
	}
	walletParams := tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600}

	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})
	got, err := observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 0 {
		t.Errorf("anchored reservation: no-target count = %d, want 0", got)
	}

	// The source's only reservation carries no anchor: it must count,
	// and the missing anchor must not panic the anchor check.
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	got, err = observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 1 {
		t.Errorf("nil anchor: no-target count = %d, want 1", got)
	}
}

// TestReservationKeeperObserver_NonActiveReservationSkipped checks that
// a non-Active reservation on a retiring source wallet is skipped by
// the no-target count: one Closed reservation plus one Active one counts
// exactly one.
func TestReservationKeeperObserver_NonActiveReservationSkipped(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateClosing},
	}
	closedKey := big.NewInt(1)
	activeKey := big.NewInt(2)
	spvChain.setWalletReservations(
		source,
		[]*big.Int{closedKey, activeKey},
	)
	spvChain.setReservation(closedKey, &tbtc.Reservation{
		State:      tbtc.ReservationStateClosed,
		AnchorUtxo: observerAnchorUtxo(1000),
	})
	spvChain.setReservation(activeKey, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, rerr := observer.countReanchorsWithoutTarget(
		params,
		tbtc.WalletParameters{},
		head,
	)
	if rerr != nil {
		t.Fatalf("no-target count failed: [%v]", rerr)
	}
	if got != 1 {
		t.Errorf("no-target count = %d, want 1 (the Active one only)", got)
	}
}

// TestReservationKeeperObserver_NilDataWalletDoesNotHideTarget checks
// that cached wallets with nil on-chain data (never fetched) do not
// stop the target search: with a retiring source that has an Active
// reservation, the no-target count stays 0 across many runs even when
// only one of many cached wallets is the eligible aged target, so a
// target-loop variant that terminates at the first nil-data entry
// fails. The map iteration order is random per run, which is why the
// assertion repeats: the original finds the target in every order, so
// every run must count 0.
func TestReservationKeeperObserver_NilDataWalletDoesNotHideTarget(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	// One retiring source with an Active reservation that needs a
	// target, one aged Live target, and many never-fetched (nil-data)
	// wallets: the original skips every nil entry and always reaches
	// the target, whatever the map order.
	source := [20]byte{5}
	target := [20]byte{6}
	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	for i := range 128 {
		pkh := [20]byte{}
		pkh[0] = byte(i)
		observer.cachedWallets[pkh] = &cachedWalletState{}
	}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
	}
	observer.cachedWallets[target] = &cachedWalletState{
		data: &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: head.Add(-31 * 24 * time.Hour),
		},
	}

	key := big.NewInt(2)
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	const runs = 50
	for i := range runs {
		got, rerr := observer.countReanchorsWithoutTarget(
			params,
			tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
			head,
		)
		if rerr != nil {
			t.Fatalf("no-target count on run %d failed: [%v]", i, rerr)
		}
		if got != 0 {
			t.Fatalf(
				"run %d: no-target count = %d, want 0 (a nil-data "+
					"wallet hid the eligible target)",
				i,
				got,
			)
		}
	}
}
