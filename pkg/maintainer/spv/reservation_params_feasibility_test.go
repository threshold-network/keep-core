package spv

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"

	log2 "github.com/ipfs/go-log/v2"
)

// The feasibility check runs on the action-timeout watcher's poll pass
// (pollPendingActions), so these tests drive it through that entry point
// and assert on the ERROR log and
// clientinfo.MetricReservationParamsInfeasible.

// gaugeRecordingRecorder is a MetricsRecorder that records the last
// SetGauge value per metric name, so a test can assert the gauge moved
// to 1 and back to 0.
type gaugeRecordingRecorder struct {
	mutex  sync.Mutex
	gauges map[string]float64
}

func newGaugeRecordingRecorder() *gaugeRecordingRecorder {
	return &gaugeRecordingRecorder{gauges: make(map[string]float64)}
}

func (r *gaugeRecordingRecorder) IncrementCounter(name string, value float64) {
}

func (r *gaugeRecordingRecorder) SetGauge(name string, value float64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.gauges[name] = value
}

// Gauge returns the last recorded value of the named gauge (0 when the
// gauge was never set).
func (r *gaugeRecordingRecorder) Gauge(name string) float64 {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.gauges[name]
}

// newFeasibilityCheckWatcher returns an action-timeout watcher whose poll
// pass drives the reservation-parameter feasibility check, against a
// fresh local chain with a usable block counter. The recorder receives
// the feasibility gauge updates.
func newFeasibilityCheckWatcher(
	t *testing.T,
) (*localChain, *ReservationActionTimeoutWatcher, *gaugeRecordingRecorder) {
	t.Helper()

	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	recorder := newGaugeRecordingRecorder()
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		time.Minute,
		common.Address{},
		nil,
		0,
		recorder,
	)

	return spvChain, ratw, recorder
}

// setFeasibleReservationParams installs a feasible parameter set on the
// chain: a zero reveal-ahead period, a zero term, and a zero (disabled)
// cap set, all skipped by the infeasibility rules.
func setFeasibleReservationParams(spvChain *localChain) {
	spvChain.setDepositParameters(tbtc.DepositParameters{})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: 3600,
	})
}

// infeasibleErrorLogEntry mirrors one captured JSON log line.
type infeasibleErrorLogEntry struct {
	Level   string `json:"level"`
	Logger  string `json:"logger"`
	Message string `json:"msg"`
}

// captureSpvLogs runs fn while capturing JSON log entries emitted by the
// spv package logger, returning them in emission order.
func captureSpvLogs(
	t *testing.T,
	fn func(),
) []infeasibleErrorLogEntry {
	t.Helper()

	if err := log2.SetLogLevel("keep-maintainer-spv", "debug"); err != nil {
		t.Fatal(err)
	}

	pipe := log2.NewPipeReader()

	var mutex sync.Mutex
	var entries []infeasibleErrorLogEntry

	done := make(chan struct{})
	go func() {
		defer close(done)
		decoder := json.NewDecoder(pipe)
		for {
			var entry infeasibleErrorLogEntry
			if err := decoder.Decode(&entry); err != nil {
				return
			}
			if entry.Logger != "keep-maintainer-spv" {
				continue
			}
			mutex.Lock()
			entries = append(entries, entry)
			mutex.Unlock()
		}
	}()

	fn()

	if err := pipe.Close(); err != nil {
		t.Logf("could not close log pipe reader: %v", err)
	}
	<-done

	mutex.Lock()
	defer mutex.Unlock()
	return entries
}

func errorEntryCount(
	entries []infeasibleErrorLogEntry,
	messageFragment string,
) int {
	count := 0
	for _, entry := range entries {
		if entry.Level == "error" &&
			strings.Contains(entry.Message, messageFragment) {
			count++
		}
	}
	return count
}

// TestReservationParamsInfeasibleRevealWindow raises the alarm: when the
// on-chain deposit reveal-ahead period exceeds the reservation term plus
// the refund safety margin, pollPendingActions logs an ERROR naming the
// rule and numbers, and raises
// clientinfo.MetricReservationParamsInfeasible to 1. A distinct bad
// state logs once; an unchanged state does not re-log; a fix clears the
// gauge back to 0.
func TestReservationParamsInfeasibleRevealWindow(t *testing.T) {
	spvChain, ratw, recorder := newFeasibilityCheckWatcher(t)
	// 12h term; the margin is 24h, so the boundary is 36h: a 40h or 50h
	// reveal-ahead is infeasible, while a 35h reveal-ahead fits.
	setRevealAheadForTerm := func(revealAhead uint32) {
		spvChain.setReservationParameters(
			&tbtc.ReservationParameters{
				ReservationTermSeconds: 12 * 60 * 60,
			},
		)
		spvChain.setDepositParameters(tbtc.DepositParameters{
			RevealAheadPeriod: revealAhead,
		})
	}

	t.Run("infeasible reveal window logs error and raises gauge", func(t *testing.T) {
		setRevealAheadForTerm(40 * 60 * 60)
		entries := captureSpvLogs(t, func() {
			if err := ratw.pollPendingActions(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if got := recorder.Gauge(
			clientinfo.MetricReservationParamsInfeasible,
		); got != 1 {
			t.Errorf(
				"expected the infeasible gauge to be 1, got %v",
				got,
			)
		}

		errors := errorEntryCount(
			entries,
			"make reserved deposits impossible",
		)
		if errors != 1 {
			t.Errorf(
				"expected one error log line naming the infeasible "+
					"state, got %d",
				errors,
			)
		}
	})

	t.Run("an unchanged infeasible state is not re-logged", func(t *testing.T) {
		// The previous subtest already logged this state; a second poll
		// with the same values must stay quiet but keep the gauge
		// raised.
		entries := captureSpvLogs(t, func() {
			if err := ratw.pollPendingActions(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if got := recorder.Gauge(
			clientinfo.MetricReservationParamsInfeasible,
		); got != 1 {
			t.Errorf(
				"expected the infeasible gauge to stay 1, got %v",
				got,
			)
		}

		errors := errorEntryCount(
			entries,
			"make reserved deposits impossible",
		)
		if errors != 0 {
			t.Errorf(
				"expected no re-log for an unchanged infeasible "+
					"state, got %d error lines",
				errors,
			)
		}
	})

	t.Run("a distinct infeasible state logs again", func(t *testing.T) {
		// Same rule, different numbers: 50h has its own state key.
		setRevealAheadForTerm(50 * 60 * 60)
		entries := captureSpvLogs(t, func() {
			if err := ratw.pollPendingActions(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if got := recorder.Gauge(
			clientinfo.MetricReservationParamsInfeasible,
		); got != 1 {
			t.Errorf(
				"expected the infeasible gauge to stay 1, got %v",
				got,
			)
		}

		errors := errorEntryCount(
			entries,
			"make reserved deposits impossible",
		)
		if errors != 1 {
			t.Errorf(
				"expected one error log line for the distinct "+
					"infeasible state, got %d",
				errors,
			)
		}
	})

	t.Run("a fix clears the gauge and logs recovery", func(t *testing.T) {
		// 35h fits the 12h term plus the 24h margin.
		setRevealAheadForTerm(35 * 60 * 60)
		entries := captureSpvLogs(t, func() {
			if err := ratw.pollPendingActions(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if got := recorder.Gauge(
			clientinfo.MetricReservationParamsInfeasible,
		); got != 0 {
			t.Errorf(
				"expected the gauge back to 0 after the fix, got %v",
				got,
			)
		}

		infos := 0
		for _, entry := range entries {
			if entry.Level == "info" &&
				strings.Contains(entry.Message, "feasible again") {
				infos++
			}
		}
		if infos != 1 {
			t.Errorf("expected one recovery log line, got %d", infos)
		}
	})
}

// TestReservationParamsCapBelowDust raises the alarm for a non-zero
// amount cap below the deposit dust threshold.
func TestReservationParamsCapBelowDust(t *testing.T) {
	spvChain, ratw, recorder := newFeasibilityCheckWatcher(t)
	setFeasibleReservationParams(spvChain)
	spvChain.setDepositParameters(tbtc.DepositParameters{DustThreshold: 1000000})
	spvChain.setReservationCaps(500000, 10000000)

	entries := captureSpvLogs(t, func() {
		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if got := recorder.Gauge(
		clientinfo.MetricReservationParamsInfeasible,
	); got != 1 {
		t.Errorf("expected the infeasible gauge to be 1, got %v", got)
	}

	errors := errorEntryCount(entries, "make reserved deposits impossible")
	if errors != 1 {
		t.Errorf(
			"expected one error log line naming the cap-vs-dust rule, "+
				"got %d",
			errors,
		)
	}

	// A cap at the dust threshold is feasible: raise both caps to clear
	// the rule.
	spvChain.setReservationCaps(1000000, 10000000)
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationParamsInfeasible,
	); got != 0 {
		t.Errorf(
			"expected the gauge back to 0 once the caps reached the "+
				"dust threshold, got %v",
			got,
		)
	}
}

// TestReservationParamsFeasibleKeepsAlarmCleared confirms a feasible
// parameter set leaves the gauge at 0 and logs nothing.
func TestReservationParamsFeasibleKeepsAlarmCleared(t *testing.T) {
	spvChain, ratw, recorder := newFeasibilityCheckWatcher(t)
	setFeasibleReservationParams(spvChain)

	entries := captureSpvLogs(t, func() {
		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if got := recorder.Gauge(
		clientinfo.MetricReservationParamsInfeasible,
	); got != 0 {
		t.Errorf("expected the infeasible gauge to be 0, got %v", got)
	}

	errors := errorEntryCount(entries, "make reserved deposits impossible")
	if errors != 0 {
		t.Errorf(
			"expected no error log for feasible parameters, got %d",
			errors,
		)
	}
}

// TestReservationParamsInfeasibleThenFixedReturnsGaugeToZero is a
// dedicated infeasible -> fixed round-trip covering the cap rule: the
// gauge must read 1 while the cap is below dust and return to 0 once
// governance raises it.
func TestReservationParamsInfeasibleThenFixedReturnsGaugeToZero(t *testing.T) {
	spvChain, ratw, recorder := newFeasibilityCheckWatcher(t)
	setFeasibleReservationParams(spvChain)
	spvChain.setDepositParameters(tbtc.DepositParameters{DustThreshold: 1000000})
	spvChain.setReservationCaps(1000000, 900000)

	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationParamsInfeasible,
	); got != 1 {
		t.Fatalf(
			"expected the gauge to be 1 while the cap is below dust, "+
				"got %v",
			got,
		)
	}

	// Governance raises the single-amount cap to the dust threshold.
	spvChain.setReservationCaps(1000000, 1000000)
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationParamsInfeasible,
	); got != 0 {
		t.Errorf(
			"expected the gauge back to 0 after the cap was raised, "+
				"got %v",
			got,
		)
	}
}

// TestReservationParamsFeasibilityCheckSkippedOnInactiveNetwork confirms
// the check (and its chain reads) is skipped entirely on a network
// without a reservation activation entry, where reservations are
// inactive.
func TestReservationParamsFeasibilityCheckSkippedOnInactiveNetwork(t *testing.T) {
	spvChain, ratw, recorder := newFeasibilityCheckWatcher(t)
	ratw.activationBlock = math.MaxUint64

	// Infeasible params on an inactive network must not raise the gauge.
	spvChain.setReservationParameters(&tbtc.ReservationParameters{})
	spvChain.setDepositParameters(tbtc.DepositParameters{
		RevealAheadPeriod: 40 * 60 * 60,
	})
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := recorder.Gauge(
		clientinfo.MetricReservationParamsInfeasible,
	); got != 0 {
		t.Errorf(
			"expected no gauge update on an inactive network, got %v",
			got,
		)
	}
}
