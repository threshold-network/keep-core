package spv

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_RunStopsOnCancel runs the background
// loop with a short interval: the initial pass publishes the gauges
// before the ticker loop starts, and a cancelled context stops the loop
// with a nil return.
func TestReservationKeeperObserver_RunStopsOnCancel(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- observer.Run(ctx, 50*time.Millisecond)
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Run returned before publishing a pass: [%v]", err)
		case <-deadline.C:
			cancel()
			t.Fatal("Run did not publish a pass within the deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Past the initial pass and at least one ticker tick.
	time.Sleep(120 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error on cancel: [%v]", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when the context was cancelled")
	}
}

// TestReservationKeeperObserver_RunRejectsZeroInterval checks that Run
// rejects a non-positive interval before starting the ticker loop: a
// zero interval would create a spinning ticker. The context auto-cancels
// after 200 ms so a mutant that accepts the interval still returns
// within a bounded time.
func TestReservationKeeperObserver_RunRejectsZeroInterval(t *testing.T) {
	spvChain, observer, _, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	if err := observer.Run(ctx, 0); err == nil {
		t.Error("Run accepted a zero interval")
	}
}

// TestReservationKeeperObserver_RunTinyInterval checks that a positive
// (if tiny) interval is accepted: the loop must not reject it, must
// publish a pass, and must stop on cancel.
func TestReservationKeeperObserver_RunTinyInterval(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- observer.Run(ctx, 1*time.Nanosecond)
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Run rejected a positive interval: [%v]", err)
		case <-deadline.C:
			cancel()
			t.Fatal("Run did not publish a pass within the deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}

	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error on cancel: [%v]", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when the context was cancelled")
	}
}

// acceptanceFetchCountingChain wraps the fixture chain, counts the
// acceptance-event fetches it issues, and can inject an error into one.
type acceptanceFetchCountingChain struct {
	*localChain
	fetchCount int
	fetchErr   error
}

func (c *acceptanceFetchCountingChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	c.fetchCount++
	if c.fetchErr != nil {
		return nil, c.fetchErr
	}
	return c.localChain.PastReservationAcceptanceRequestedEvents(filter)
}

// TestReservationKeeperObserver_NoScanPathFetchFailureKeepsPublishing
// checks that the no-activation-entry acceptance path is a real
// no-scan: it issues no acceptance-event fetch (the cursor moves
// straight to the confirmed tip), so an acceptance RPC failure cannot
// make the pass fail or withhold the observation timestamp.
func TestReservationKeeperObserver_NoScanPathFetchFailureKeepsPublishing(
	t *testing.T,
) {
	spvChain, _, recorder, head, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	blockCounter.SetCurrentBlock(20000)
	tip := confirmedTip(20000)

	// One acceptance event below the activation-absent network's scan
	// range: a real no-scan must never pick it up.
	key := big.NewInt(1)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, 0),
	)

	// A no-activation-entry observer (activationBlock is MaxUint64),
	// so the first acceptance scan is the no-scan path.
	observer := newObserverForTest(spvChain, math.MaxUint64, recorder)
	wrap := &acceptanceFetchCountingChain{localChain: spvChain}
	wrap.fetchErr = errors.New("acceptance RPC failed")
	observer.spvChain = wrap

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf(
			"no-scan acceptance pass must not fetch, so a fetch error "+
				"cannot fail it; pass failed: [%v]",
			err,
		)
	}
	if got := wrap.fetchCount; got != 0 {
		t.Fatalf(
			"acceptance-event fetches on the no-scan path = %d, want 0",
			got,
		)
	}
	if got := observer.acceptanceScanCursor; got != tip {
		t.Errorf("acceptance cursor = %d, want %d", got, tip)
	}
	if got := len(observer.trackedAcceptance); got != 0 {
		t.Errorf(
			"no-scan path tracked acceptance events: %d, want 0",
			got,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != float64(head.Unix()) {
		t.Errorf(
			"observation timestamp after the no-scan pass = %v, want %v",
			got,
			head.Unix(),
		)
	}
}
