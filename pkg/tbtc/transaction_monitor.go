package tbtc

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// transactionMonitorMinConfirmations is the number of confirmations at which
// a tracked transaction is considered mined and stops being tracked.
const transactionMonitorMinConfirmations = uint(1)

// trackedTransaction holds the monitoring state of a single broadcast wallet
// transaction.
type trackedTransaction struct {
	walletPublicKeyHash [20]byte
	broadcastAt         time.Time
	alerted             bool
}

// trackedTransactionSnapshot pairs a copy of a tracked transaction with its hash
// so the check loop can iterate an ordered snapshot outside the lock.
type trackedTransactionSnapshot struct {
	hash bitcoin.Hash
	trackedTransaction
}

// snapshotByAge returns a copy of the tracked transactions ordered by broadcast
// time, oldest first. Checking oldest first ensures the transactions closest to
// the stuck threshold are never starved when a check pass hits its time budget:
// Go map iteration order is randomized, so without an explicit ordering an
// unlucky old transaction could be skipped pass after pass and miss its
// threshold; with it, only the newest transactions - furthest from alerting -
// are ever deferred. The copy is taken under the lock; the sort is not.
func (tm *transactionMonitor) snapshotByAge() []trackedTransactionSnapshot {
	tm.mu.Lock()
	ordered := make([]trackedTransactionSnapshot, 0, len(tm.tracked))
	for hash, t := range tm.tracked {
		ordered = append(ordered, trackedTransactionSnapshot{hash, *t})
	}
	tm.mu.Unlock()

	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].broadcastAt.Before(ordered[j].broadcastAt)
	})

	return ordered
}

// transactionMonitor watches broadcast wallet transactions (deposit sweeps,
// redemptions, moving funds) and raises an alert if one stays unconfirmed for
// longer than a configurable duration. A stuck wallet transaction locks the
// wallet's main UTXO and blocks all subsequent wallet transactions until it
// confirms, so surfacing it lets operators intervene (e.g. mempool acceleration
// or CPFP) promptly.
//
// The monitor only emits a metric and a warn-level log identifying the wallet
// and transaction; automated recovery (fee-bumping / RBF) is intentionally out
// of scope (see threshold-network/keep-core#4171). Alerting is left to the
// operator's monitoring stack. Every wallet operator that broadcasts a given
// transaction tracks it independently, so the metric and log are emitted per
// operator and should be de-duplicated by transaction hash downstream.
//
// The tracked set is in-memory only. A transaction that is already stuck when
// the node restarts is not re-tracked (it was broadcast by the previous
// process), so cross-restart stuck transactions are not detected here; they
// remain covered by the coarser wallet-level liveness metrics.
type transactionMonitor struct {
	btcChain bitcoin.Chain

	mu      sync.Mutex
	tracked map[bitcoin.Hash]*trackedTransaction

	config TransactionMonitorConfig

	metricsRecorder clientinfo.PerformanceMetricsRecorder
}

func newTransactionMonitor(
	btcChain bitcoin.Chain,
	config TransactionMonitorConfig,
) (*transactionMonitor, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction monitor configuration: %w", err)
	}
	return &transactionMonitor{
		btcChain: btcChain,
		tracked:  make(map[bitcoin.Hash]*trackedTransaction),
		config:   config.withDefaults(),
	}, nil
}

// setMetricsRecorder wires the performance metrics recorder used to expose the
// stuck-transaction metric. It is safe to call after construction.
func (tm *transactionMonitor) setMetricsRecorder(
	recorder clientinfo.PerformanceMetricsRecorder,
) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.metricsRecorder = recorder
}

// track registers a freshly broadcast wallet transaction for confirmation
// monitoring. It is a no-op if the transaction is already tracked or the
// tracking table is full. It performs no network calls, so it never blocks the
// broadcast path or silently drops a transaction due to a chain lookup failure.
func (tm *transactionMonitor) track(
	txHash bitcoin.Hash,
	walletPublicKeyHash [20]byte,
) {
	tm.mu.Lock()

	if _, ok := tm.tracked[txHash]; ok {
		tm.mu.Unlock()
		return
	}

	if len(tm.tracked) >= tm.config.MaxTracked {
		// A full table means a real broadcast transaction goes unmonitored;
		// surface it as a metric (emitted outside the lock) as well as a log.
		recorder := tm.metricsRecorder
		tm.mu.Unlock()

		logger.Warnf(
			"transaction monitor tracking table is full ([%d]); transaction "+
				"[%s] will not be monitored",
			tm.config.MaxTracked,
			txHash.Hex(bitcoin.ReversedByteOrder),
		)
		if recorder != nil {
			recorder.IncrementCounter(
				clientinfo.MetricUnmonitoredWalletTransactionsTotal, 1,
			)
		}
		return
	}

	tm.tracked[txHash] = &trackedTransaction{
		walletPublicKeyHash: walletPublicKeyHash,
		broadcastAt:         time.Now(),
	}
	tm.mu.Unlock()
}

// run starts the monitor's polling loop. It blocks until the context is done.
func (tm *transactionMonitor) run(ctx context.Context) {
	ticker := time.NewTicker(tm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tm.check(ctx)
		}
	}
}

// check polls the confirmation status of every tracked transaction once. It
// drops confirmed transactions, gives up on ones that have been unconfirmed for
// too long, and alerts (once) on those that have remained unconfirmed for longer
// than the stuck threshold.
//
// check is only ever called from the single run loop goroutine. track may run
// concurrently but only inserts new entries, so the per-entry alerted flag is
// only ever mutated here, under the mutex.
func (tm *transactionMonitor) check(ctx context.Context) {
	tm.checkWithBudget(ctx, tm.config.CheckBudget)
}

func (tm *transactionMonitor) checkWithBudget(
	ctx context.Context,
	checkBudget time.Duration,
) {
	checkCtx, cancelCheck := context.WithTimeout(
		ctx,
		checkBudget,
	)
	defer cancelCheck()

	now := time.Now()

	// Iterate the tracked set oldest-first (see snapshotByAge) so a pass that
	// hits its time budget never starves the transactions closest to the stuck
	// threshold; only the newest, furthest-from-alerting ones are deferred to the
	// next pass. Chain calls are made on the copy, outside the lock.
	for _, t := range tm.snapshotByAge() {
		txHash := t.hash
		// The chain call is bounded by checkCtx, so a slow or hung backend cannot
		// keep this run loop blocked past the check budget: when the budget
		// expires the call is cancelled and returns.
		confirmations, err :=
			tm.btcChain.GetTransactionConfirmations(checkCtx, txHash)

		// Stop immediately if the monitor itself is shutting down.
		if ctx.Err() != nil {
			return
		}

		// The budget may have expired during the lookup above. The age-based
		// alert and eviction below need no network data, so still run them for
		// the current transaction - whose lookup was attempted and cancelled -
		// before deferring the rest. Otherwise a backend that hangs on the oldest
		// tracked transaction every pass would keep it (the one closest to the
		// stuck threshold) from ever alerting or being evicted. The remaining
		// transactions are deferred, not alerted: alerting on a transaction we
		// never looked up this pass could raise a false alert for one that has
		// in fact confirmed.
		budgetExpired := checkCtx.Err() != nil

		if !budgetExpired && err == nil &&
			confirmations >= transactionMonitorMinConfirmations {
			// The transaction is mined; stop tracking it.
			tm.remove(txHash)
			continue
		}

		// Still unconfirmed (in the mempool), not found, or the lookup returned a
		// transient error. A lookup error is treated the same as unconfirmed: it
		// does not indicate the transaction confirmed and may be transient.
		outstanding := now.Sub(t.broadcastAt)

		// Alert (once) if the transaction has been unconfirmed past the stuck
		// threshold. This runs before the give-up eviction below so that a
		// transaction first observed after the maximum tracking age still fires
		// exactly one alert instead of being silently evicted.
		if outstanding > tm.config.StuckThreshold && !t.alerted {
			logger.Warnf(
				"wallet transaction [%s] for wallet [0x%x] has been unconfirmed "+
					"for [%s] (threshold [%s]); it may be stuck in the mempool "+
					"and blocking subsequent wallet transactions - consider "+
					"fee-bumping or accelerating it",
				txHash.Hex(bitcoin.ReversedByteOrder),
				t.walletPublicKeyHash,
				outstanding.Round(time.Minute),
				tm.config.StuckThreshold,
			)

			// Mark as alerted under the lock, but emit the metric outside the
			// lock to avoid holding it during an external call.
			tm.mu.Lock()
			if tracked, ok := tm.tracked[txHash]; ok {
				tracked.alerted = true
			}
			recorder := tm.metricsRecorder
			tm.mu.Unlock()

			if recorder != nil {
				recorder.IncrementCounter(
					clientinfo.MetricStuckWalletTransactionsTotal, 1,
				)
			}
		}

		// Give up on transactions that have been unconfirmed for too long (e.g.
		// dropped from the mempool) so they cannot fill the tracking table.
		if outstanding > tm.config.MaxTrackingAge {
			logger.Warnf(
				"giving up monitoring wallet transaction [%s] for wallet "+
					"[0x%x]; it has been unconfirmed for [%s]",
				txHash.Hex(bitcoin.ReversedByteOrder),
				t.walletPublicKeyHash,
				outstanding.Round(time.Minute),
			)
			tm.remove(txHash)
		}

		if budgetExpired {
			// The check budget expired during this transaction's lookup; its
			// age-based alert/eviction ran above, and the remaining transactions
			// are deferred to the next pass. The monitor is not shutting down here
			// (that returned earlier), so the warning is unconditional.
			logger.Warnf(
				"transaction monitor check pass exceeded its time budget [%s]; "+
					"deferring the remaining transactions to the next pass",
				checkBudget,
			)
			return
		}
	}
}

// remove stops tracking the transaction with the given hash.
func (tm *transactionMonitor) remove(txHash bitcoin.Hash) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tracked, txHash)
}
