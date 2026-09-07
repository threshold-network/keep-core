package tbtc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// countingMetricsRecorder is a PerformanceMetricsRecorder that counts counter
// increments so tests can assert the stuck-transaction metric fired.
type countingMetricsRecorder struct {
	counters map[string]float64
}

func newCountingMetricsRecorder() *countingMetricsRecorder {
	return &countingMetricsRecorder{counters: make(map[string]float64)}
}

func (c *countingMetricsRecorder) IncrementCounter(name string, value float64) {
	c.counters[name] += value
}
func (c *countingMetricsRecorder) RecordDuration(string, time.Duration) {}
func (c *countingMetricsRecorder) SetGauge(string, float64)             {}
func (c *countingMetricsRecorder) GetCounterValue(name string) float64 {
	return c.counters[name]
}
func (c *countingMetricsRecorder) GetGaugeValue(string) float64 { return 0 }

// ageTransaction backdates the broadcast time of a tracked transaction to
// simulate the passage of time.
func ageTransaction(
	tm *transactionMonitor,
	txHash bitcoin.Hash,
	by time.Duration,
) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if t, ok := tm.tracked[txHash]; ok {
		t.broadcastAt = t.broadcastAt.Add(-by)
	}
}

func isTracked(tm *transactionMonitor, txHash bitcoin.Hash) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	_, ok := tm.tracked[txHash]
	return ok
}

func trackedCount(tm *transactionMonitor) int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.tracked)
}

type blockingTransactionConfirmationsChain struct {
	*localBitcoinChain

	blockedHash   bitcoin.Hash
	lookupMutex   sync.Mutex
	lookupCount   int
	startedOnce   sync.Once
	doneOnce      sync.Once
	lookupStarted chan struct{}
	lookupRelease chan struct{}
	lookupDone    chan struct{}
}

func newBlockingTransactionConfirmationsChain(
	blockedHash bitcoin.Hash,
) *blockingTransactionConfirmationsChain {
	return &blockingTransactionConfirmationsChain{
		localBitcoinChain: newLocalBitcoinChain(),
		blockedHash:       blockedHash,
		lookupStarted:     make(chan struct{}),
		lookupRelease:     make(chan struct{}),
		lookupDone:        make(chan struct{}),
	}
}

func (c *blockingTransactionConfirmationsChain) GetTransactionConfirmations(
	ctx context.Context,
	transactionHash bitcoin.Hash,
) (uint, error) {
	if transactionHash == c.blockedHash {
		c.lookupMutex.Lock()
		c.lookupCount++
		c.lookupMutex.Unlock()

		c.startedOnce.Do(func() { close(c.lookupStarted) })
		// Block until released, but honor the context so a check pass that hits
		// its budget can cancel the lookup instead of stalling on the backend.
		select {
		case <-c.lookupRelease:
			c.doneOnce.Do(func() { close(c.lookupDone) })
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	return c.localBitcoinChain.GetTransactionConfirmations(ctx, transactionHash)
}

func (c *blockingTransactionConfirmationsChain) getLookupCount() int {
	c.lookupMutex.Lock()
	defer c.lookupMutex.Unlock()
	return c.lookupCount
}

func TestTransactionMonitor(t *testing.T) {
	chain := newLocalBitcoinChain()
	recorder := newCountingMetricsRecorder()

	monitor, err := newTransactionMonitor(chain, TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	monitor.setMetricsRecorder(recorder)

	tx := &bitcoin.Transaction{}
	txHash := tx.Hash()
	monitor.track(txHash, [20]byte{1, 2, 3})

	stuckCount := func() float64 {
		return recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal)
	}

	// Fresh: not yet stuck.
	monitor.check(context.Background())
	if got := stuckCount(); got != 0 {
		t.Fatalf("expected no alert for a fresh transaction; got counter [%v]", got)
	}

	// Just below the threshold: still not stuck. The alert condition is strictly
	// greater than the threshold. Testing exactly at the threshold is omitted
	// deliberately: with a real clock the check-time drift always nudges the
	// elapsed time a hair past any exact backdated value, so it cannot be pinned
	// deterministically without an injectable clock.
	ageTransaction(monitor, txHash, DefaultTransactionMonitorStuckThreshold-time.Minute)
	monitor.check(context.Background())
	if got := stuckCount(); got != 0 {
		t.Fatalf("expected no alert at the threshold boundary; got counter [%v]", got)
	}

	// Past the threshold: flagged as stuck exactly once across repeated checks.
	ageTransaction(monitor, txHash, 2*time.Minute) // now threshold + 1 minute total
	monitor.check(context.Background())
	monitor.check(context.Background())
	if got := stuckCount(); got != 1 {
		t.Fatalf("expected exactly one alert; got counter [%v]", got)
	}

	// Once the transaction confirms, it stops being tracked.
	if err := chain.BroadcastTransaction(tx); err != nil {
		t.Fatalf("unexpected error confirming transaction: [%v]", err)
	}
	monitor.check(context.Background())
	if isTracked(monitor, txHash) {
		t.Fatal("expected confirmed transaction to be untracked")
	}
}

// TestTransactionMonitor_GivesUpOnNeverConfirming verifies that a transaction
// that never confirms is eventually evicted so it cannot fill the tracking
// table.
func TestTransactionMonitor_GivesUpOnNeverConfirming(t *testing.T) {
	recorder := newCountingMetricsRecorder()
	monitor, err := newTransactionMonitor(newLocalBitcoinChain(), TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	monitor.setMetricsRecorder(recorder)

	tx := &bitcoin.Transaction{}
	txHash := tx.Hash()
	monitor.track(txHash, [20]byte{})

	// Age it beyond the maximum tracking age; on its first check it is past both
	// the stuck threshold and the give-up age. It must still fire exactly one
	// stuck alert (the alert runs before eviction) and then be evicted rather
	// than tracked forever.
	ageTransaction(monitor, txHash, DefaultTransactionMonitorMaxTrackingAge+time.Minute)
	monitor.check(context.Background())

	if got := recorder.GetCounterValue(
		clientinfo.MetricStuckWalletTransactionsTotal,
	); got != 1 {
		t.Fatalf("expected one stuck alert before eviction; got counter [%v]", got)
	}
	if isTracked(monitor, txHash) {
		t.Fatal("expected a never-confirming transaction to be evicted")
	}
}

// TestTransactionMonitor_CheckBudgetBoundsLookup verifies that a chain lookup
// cannot keep the monitor's sole check goroutine blocked beyond the check
// budget.
func TestTransactionMonitor_CheckBudgetBoundsLookup(t *testing.T) {
	blockedTxHash := bitcoin.Hash{1}
	chain := newBlockingTransactionConfirmationsChain(blockedTxHash)
	monitor, err := newTransactionMonitor(chain, TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}

	monitor.track(blockedTxHash, [20]byte{})

	// Ensure the intentionally blocked backend is released even if the test
	// fails, so the mock's lookup goroutine is never left parked.
	lookupReleased := false
	defer func() {
		if !lookupReleased {
			close(chain.lookupRelease)
			lookupReleased = true
		}
	}()

	checkDone := make(chan struct{})
	go func() {
		monitor.checkWithBudget(context.Background(), 500*time.Millisecond)
		close(checkDone)
	}()

	select {
	case <-chain.lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("confirmation lookup did not start")
	}

	// The backend never unblocks, so only the check budget can release the pass.
	// This is the crux: the monitor threads the budgeted context into the chain
	// call, so budget expiry cancels the lookup and the pass returns. If it did
	// not, this would block until the timeout below and fail.
	select {
	case <-checkDone:
	case <-time.After(2 * time.Second):
		t.Fatal("transaction check remained blocked after its budget expired")
	}

	if got := chain.getLookupCount(); got != 1 {
		t.Fatalf("expected exactly one confirmation lookup; got [%d]", got)
	}

	// The cancelled lookup is treated as unconfirmed, so the transaction stays
	// tracked and is retried on a later pass.
	if !isTracked(monitor, blockedTxHash) {
		t.Fatal("expected transaction with a cancelled lookup to remain tracked")
	}
}

// TestTransactionMonitor_BudgetExpiryStillAlertsOldest verifies that when a
// check pass hits its time budget while looking up the oldest tracked
// transaction, that transaction still fires its stuck alert (the alert needs no
// network data) instead of being starved pass after pass. The remaining
// transactions are deferred to the next pass rather than alerted on without a
// lookup, so they cannot false-alert on age alone. This is the multi-transaction
// dual of CheckBudgetBoundsLookup: it guards against head-of-line starvation
// when a hung backend consistently eats the whole budget on the oldest entry.
func TestTransactionMonitor_BudgetExpiryStillAlertsOldest(t *testing.T) {
	blockedTxHash := bitcoin.Hash{1} // oldest; its lookup hangs until the budget expires
	chain := newBlockingTransactionConfirmationsChain(blockedTxHash)
	recorder := newCountingMetricsRecorder()
	monitor, err := newTransactionMonitor(chain, TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	monitor.setMetricsRecorder(recorder)

	newerTxHash := bitcoin.Hash{2}
	monitor.track(blockedTxHash, [20]byte{})
	monitor.track(newerTxHash, [20]byte{})

	// Both transactions are past the stuck threshold; the blocked one is older so
	// oldest-first ordering checks it first and its lookup consumes the budget.
	ageTransaction(monitor, blockedTxHash, DefaultTransactionMonitorStuckThreshold+2*time.Hour)
	ageTransaction(monitor, newerTxHash, DefaultTransactionMonitorStuckThreshold+time.Hour)

	// Release the intentionally blocked backend on exit so its lookup goroutine
	// is never left parked.
	lookupReleased := false
	defer func() {
		if !lookupReleased {
			close(chain.lookupRelease)
			lookupReleased = true
		}
	}()

	checkDone := make(chan struct{})
	go func() {
		monitor.checkWithBudget(context.Background(), 200*time.Millisecond)
		close(checkDone)
	}()

	select {
	case <-checkDone:
	case <-time.After(2 * time.Second):
		t.Fatal("check pass did not return after its budget expired")
	}

	// The oldest transaction's lookup was cancelled by the budget, but its
	// age-based alert must still fire - exactly once, for the oldest only. Before
	// the fix the pass returned on budget expiry before alerting, leaving this at
	// 0; if the never-looked-up newer transaction were also alerted it would be 2.
	if got := recorder.GetCounterValue(
		clientinfo.MetricStuckWalletTransactionsTotal,
	); got != 1 {
		t.Fatalf("expected exactly one stuck alert (oldest only); got counter [%v]", got)
	}

	// A cancelled lookup is not a confirmation, so the oldest stays tracked; the
	// newer transaction was deferred, not confirmed, so it stays tracked too.
	if !isTracked(monitor, blockedTxHash) {
		t.Fatal("expected the oldest transaction to remain tracked after a cancelled lookup")
	}
	if !isTracked(monitor, newerTxHash) {
		t.Fatal("expected the deferred newer transaction to remain tracked")
	}

	// Only the oldest transaction's lookup was attempted; the pass returned
	// before reaching the newer one.
	if got := chain.getLookupCount(); got != 1 {
		t.Fatalf("expected exactly one lookup (only the oldest was attempted); got [%d]", got)
	}
}

// TestTransactionMonitor_CapacityBound verifies the tracking table does not grow
// past its bound.
func TestTransactionMonitor_CapacityBound(t *testing.T) {
	recorder := newCountingMetricsRecorder()
	monitor, err := newTransactionMonitor(newLocalBitcoinChain(), TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	monitor.setMetricsRecorder(recorder)

	const excess = 10
	for i := 0; i < DefaultTransactionMonitorMaxTracked+excess; i++ {
		var h bitcoin.Hash
		h[0] = byte(i)
		h[1] = byte(i >> 8)
		monitor.track(h, [20]byte{})
	}

	if got := trackedCount(monitor); got != DefaultTransactionMonitorMaxTracked {
		t.Fatalf(
			"expected tracking table bounded to [%d]; got [%d]",
			DefaultTransactionMonitorMaxTracked,
			got,
		)
	}

	// The transactions that could not be tracked once the table filled must be
	// surfaced via the unmonitored-transactions metric.
	if got := recorder.GetCounterValue(
		clientinfo.MetricUnmonitoredWalletTransactionsTotal,
	); got != excess {
		t.Fatalf(
			"expected [%d] unmonitored-transaction increments; got [%v]",
			excess,
			got,
		)
	}
}

// TestTransactionMonitor_SnapshotByAge verifies the check pass iterates tracked
// transactions oldest-first, so an old transaction near the stuck threshold is
// never starved when a pass hits its time budget (Go map order is randomized).
func TestTransactionMonitor_SnapshotByAge(t *testing.T) {
	monitor, err := newTransactionMonitor(newLocalBitcoinChain(), TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}

	var h1, h2, h3 bitcoin.Hash
	h1[0], h2[0], h3[0] = 1, 2, 3
	monitor.track(h1, [20]byte{})
	monitor.track(h2, [20]byte{})
	monitor.track(h3, [20]byte{})

	// Backdate broadcast times so the age order is h2 (oldest), h3, h1 (newest).
	// The hour-scale gaps dwarf the sub-second differences in track() times.
	ageTransaction(monitor, h1, 1*time.Hour)
	ageTransaction(monitor, h2, 3*time.Hour)
	ageTransaction(monitor, h3, 2*time.Hour)

	ordered := monitor.snapshotByAge()

	want := []bitcoin.Hash{h2, h3, h1}
	if len(ordered) != len(want) {
		t.Fatalf("expected [%d] entries; got [%d]", len(want), len(ordered))
	}
	for i, w := range want {
		if ordered[i].hash != w {
			t.Fatalf(
				"snapshotByAge not oldest-first at index [%d]\nexpected: %v\ngot:      %v",
				i, want, []bitcoin.Hash{ordered[0].hash, ordered[1].hash, ordered[2].hash},
			)
		}
	}
	// Broadcast times must be non-decreasing across the ordered snapshot.
	for i := 1; i < len(ordered); i++ {
		if ordered[i].broadcastAt.Before(ordered[i-1].broadcastAt) {
			t.Fatalf("entry [%d] is older than entry [%d]; not sorted oldest-first", i, i-1)
		}
	}
}
