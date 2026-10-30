// Tests for the reservation code paths that the M1 coverage measurement
// left uncovered. Each test drives the real code path, so it fails on a
// build that drops the guard it pins.
package spv

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// feasibilityParamReadErrorChain is a localChain whose parameter reads
// fail on demand. Each field names the read that fails; a nil field
// leaves that read working.
type feasibilityParamReadErrorChain struct {
	*localChain

	depositParametersErr   error
	reservationParametersErr error
	reservationCapsErr     error
}

func (c *feasibilityParamReadErrorChain) GetDepositParameters() (
	tbtc.DepositParameters,
	error,
) {
	if c.depositParametersErr != nil {
		return tbtc.DepositParameters{}, c.depositParametersErr
	}
	return c.localChain.GetDepositParameters()
}

func (c *feasibilityParamReadErrorChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	if c.reservationParametersErr != nil {
		return nil, c.reservationParametersErr
	}
	return c.localChain.ReservationParameters()
}

func (c *feasibilityParamReadErrorChain) ReservationCaps() (
	uint64,
	uint64,
	error,
) {
	if c.reservationCapsErr != nil {
		return 0, 0, c.reservationCapsErr
	}
	return c.localChain.ReservationCaps()
}

// newFeasibilityReadFailureWatcher returns an action-timeout watcher over
// the given chain, ready to drive the feasibility check through its poll
// pass.
func newFeasibilityReadFailureWatcher(
	t *testing.T,
	spvChain Chain,
) (*ReservationActionTimeoutWatcher, *gaugeRecordingRecorder) {
	t.Helper()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	if setter, ok := spvChain.(interface {
		setBlockCounter(chain.BlockCounter)
	}); ok {
		setter.setBlockCounter(blockCounter)
	}

	recorder := newGaugeRecordingRecorder()
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		time.Minute,
		common.Address{},
		nil,
		0,
		recorder,
	)

	return ratw, recorder
}

// warnEntryCount counts the captured log entries at warn level whose
// message contains the given fragment. A parameter-read failure is a warn,
// not an error: it is an unknown-parameter tick, not an infeasible
// parameter state.
func warnEntryCount(
	entries []infeasibleErrorLogEntry,
	messageFragment string,
) int {
	count := 0
	for _, entry := range entries {
		if entry.Level == "warn" &&
			strings.Contains(entry.Message, messageFragment) {
			count++
		}
	}
	return count
}

// TestReservationParamsFeasibilityReadFailuresAreLogged covers the three
// parameter-read guards of the feasibility check.
//
// The check is alarm-only: a failed read must be logged and must leave
// the last known gauge value in place, never raise the infeasible alarm
// and never stop the node. Each of the three reads (deposit parameters,
// reservation parameters, reservation caps) has its own guard and its own
// message, so a read failure on any of them must be visible in the log.
//
// This is the negative control for the alarm behaviour: each case fails
// if the read failure is swallowed silently.
func TestReservationParamsFeasibilityReadFailuresAreLogged(t *testing.T) {
	tests := map[string]struct {
		chain            func(base *localChain) Chain
		expectedFragment string
	}{
		"deposit parameters": {
			chain: func(base *localChain) Chain {
				return &feasibilityParamReadErrorChain{
					localChain:            base,
					depositParametersErr: errors.New("deposit parameter read failed"),
				}
			},
			expectedFragment: "failed to read the Bridge deposit parameters",
		},
		"reservation parameters": {
			chain: func(base *localChain) Chain {
				return &feasibilityParamReadErrorChain{
					localChain:              base,
					reservationParametersErr: errors.New("reservation parameter read failed"),
				}
			},
			expectedFragment: "failed to read the Bridge reservation parameters",
		},
		"reservation caps": {
			chain: func(base *localChain) Chain {
				return &feasibilityParamReadErrorChain{
					localChain:        base,
					reservationCapsErr: errors.New("reservation caps read failed"),
				}
			},
			expectedFragment: "failed to read the Bridge reservation caps",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			base := newLocalChain()
			base.setDepositParameters(tbtc.DepositParameters{})
			base.setReservationParameters(&tbtc.ReservationParameters{
				ReservationActionTimeout: 3600,
			})

			chain := test.chain(base)
			ratw, recorder := newFeasibilityReadFailureWatcher(t, chain)

			entries := captureSpvLogs(t, func() {
				ratw.checkReservationParamsFeasibility()
			})

			found := false
			for _, entry := range entries {
				if strings.Contains(entry.Message, test.expectedFragment) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf(
					"expected a log entry containing [%v], got [%v]",
					test.expectedFragment,
					entries,
				)
			}

			// A read failure is not an infeasible parameter state: the
			// alarm must stay down.
			if gauge := recorder.Gauge(
				clientinfo.MetricReservationParamsInfeasible,
			); gauge != 0 {
				t.Errorf(
					"expected the infeasible gauge to stay at 0 after a read failure, got [%v]",
					gauge,
				)
			}
		})
	}
}

// TestReservationParamsFeasibilityReadFailureIsLoggedOncePerStretch
// covers the failure-stretch guard of logReservationParamsReadFailure.
//
// A transient RPC hiccup that persists across poll ticks must be logged
// once, not re-logged every tick: a log that fires on every tick would
// bury the operator. The next failure stretch after the reads succeed
// again must log again.
func TestReservationParamsFeasibilityReadFailureIsLoggedOncePerStretch(t *testing.T) {
	base := newLocalChain()
	base.setDepositParameters(tbtc.DepositParameters{})
	base.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: 3600,
	})

	failing := &feasibilityParamReadErrorChain{
		localChain:          base,
		depositParametersErr: errors.New("deposit parameter read failed"),
	}
	ratw, _ := newFeasibilityReadFailureWatcher(t, failing)

	entries := captureSpvLogs(t, func() {
		// Three ticks of a persistent failure: the log fires once.
		ratw.checkReservationParamsFeasibility()
		ratw.checkReservationParamsFeasibility()
		ratw.checkReservationParamsFeasibility()

		// The reads succeed again, which clears the failure-stretch flag.
		ratw.spvChain = base
		ratw.checkReservationParamsFeasibility()

		// The next failure stretch logs again.
		ratw.spvChain = failing
		ratw.checkReservationParamsFeasibility()
	})

	count := warnEntryCount(
		entries,
		"failed to read the Bridge deposit parameters",
	)
	if count != 2 {
		t.Fatalf(
			"expected the read failure to be logged twice (once per failure stretch), got [%d]",
			count,
		)
	}
}
