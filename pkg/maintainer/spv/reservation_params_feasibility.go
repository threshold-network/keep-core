package spv

import (
	"fmt"
	"math"
	"strings"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationDepositRefundSafetyMarginSeconds is the 24h refund safety
// margin the Bridge adds on top of the reservation term: a reserved
// deposit's refund deadline is the reservation term plus this margin, so
// a deposit reveal-ahead period longer than that window leaves no
// overlap in which a reserved deposit could be revealed and still be
// refunded in time. It mirrors the deploy-time feasibility assertion in
// the reservation bootstrap tooling.
const reservationDepositRefundSafetyMarginSeconds = 24 * 60 * 60

// reservationParamsInfeasibleReasons returns the infeasibility rules the
// given on-chain reservation and deposit parameters violate, i.e. the
// rules under which reserved deposits are impossible. Every reserved
// deposit reveal carries at least the deposit dust threshold, and its
// refund deadline is the reservation term plus the safety margin:
//
//   - a non-zero deposit reveal-ahead period longer than the term plus
//     the margin means the reveal window and the refund window can never
//     overlap, so no reserved deposit could ever be revealed;
//
//   - a non-zero amount cap (per-wallet, single-amount, or max-total)
//     below the dust threshold is below the smallest amount a reveal can
//     ever carry, so no reservation could fit under it.
//
// Zero values are skipped: a zero reveal-ahead period and a zero cap are
// "disabled", not infeasible.
func reservationParamsInfeasibleReasons(
	depositParameters tbtc.DepositParameters,
	termSeconds uint32,
	maxTotalAmount uint64,
	perWalletCap uint64,
	singleAmountCap uint64,
) []string {
	var reasons []string

	revealAhead := uint64(depositParameters.RevealAheadPeriod)
	term := uint64(termSeconds)
	if revealAhead > 0 &&
		revealAhead > term+uint64(reservationDepositRefundSafetyMarginSeconds) {
		reasons = append(reasons,
			fmt.Sprintf(
				"depositRevealAheadPeriod [%ds] exceeds the reservation "+
					"term [%ds] plus the %ds refund safety margin, so no "+
					"reserved deposit could ever be revealed",
				revealAhead,
				term,
				reservationDepositRefundSafetyMarginSeconds,
			),
		)
	}

	dustThreshold := depositParameters.DustThreshold
	for _, capEntry := range []struct {
		name string
		cap_ uint64
	}{
		{"per-wallet amount cap", perWalletCap},
		{"single-amount cap", singleAmountCap},
		{"max-total amount cap", maxTotalAmount},
	} {
		if capEntry.cap_ > 0 && capEntry.cap_ < dustThreshold {
			reasons = append(reasons,
				fmt.Sprintf(
					"%s [%d sats] is below the deposit dust threshold "+
						"[%d sats], so no reservation could fit under it",
					capEntry.name,
					capEntry.cap_,
					dustThreshold,
				),
			)
		}
	}

	return reasons
}

// checkReservationParamsFeasibility reads the Bridge's deposit and
// reservation parameters, evaluates the infeasibility rules above, and
// publishes the result: an ERROR log - at most one per distinct
// infeasible state, naming the numbers and the violated rule - and
// clientinfo.MetricReservationParamsInfeasible set to 1 while any rule
// is violated and back to 0 once the parameters are feasible again.
//
// It is alarm-only: a read failure or an infeasible state is never
// returned as an error, never aborts the caller's poll pass, and never
// stops the node. A read failure leaves the last known gauge value in
// place: unknown parameters are not infeasible parameters.
//
// It is driven by the action-timeout watcher's existing poll pass
// (pollPendingActions), which runs once at wiring and then on the
// watcher's periodic ticker, so no dedicated goroutine is needed.
func (ratw *ReservationActionTimeoutWatcher) checkReservationParamsFeasibility() {
	// math.MaxUint64 is the activation-block sentinel for a network
	// without a reservation activation entry: reservations are inactive
	// there, so there are no reservation parameters to check.
	if ratw.activationBlock == math.MaxUint64 {
		return
	}

	depositParameters, err := ratw.spvChain.GetDepositParameters()
	if err != nil {
		ratw.logReservationParamsReadFailure(
			"failed to read the Bridge deposit parameters for the "+
				"reservation feasibility check: [%v]",
			err,
		)
		return
	}

	reservationParameters, err := ratw.spvChain.ReservationParameters()
	if err != nil {
		ratw.logReservationParamsReadFailure(
			"failed to read the Bridge reservation parameters for the "+
				"reservation feasibility check: [%v]",
			err,
		)
		return
	}

	perWalletCap, singleAmountCap, err := ratw.spvChain.ReservationCaps()
	if err != nil {
		ratw.logReservationParamsReadFailure(
			"failed to read the Bridge reservation caps for the "+
				"reservation feasibility check: [%v]",
			err,
		)
		return
	}

	// All three reads succeeded: clear the failure-stretch flag so the
	// next failure stretch logs again (see
	// logReservationParamsReadFailure).
	ratw.reservationParamsReadFailing = false

	reasons := reservationParamsInfeasibleReasons(
		depositParameters,
		reservationParameters.ReservationTermSeconds,
		reservationParameters.ReservationMaxTotalAmount,
		perWalletCap,
		singleAmountCap,
	)

	stateKey := reservationParamsStateKey(
		len(reasons) > 0,
		depositParameters.RevealAheadPeriod,
		reservationParameters.ReservationTermSeconds,
		depositParameters.DustThreshold,
		reservationParameters.ReservationMaxTotalAmount,
		perWalletCap,
		singleAmountCap,
	)

	if len(reasons) > 0 {
		// Report a distinct infeasible state once, and re-report when
		// the infeasible values themselves change; a state that stays
		// the same across ticks is not re-logged.
		if stateKey != ratw.reservationParamsState {
			ratw.reservationParamsState = stateKey
			logger.Errorf(
				"the on-chain parameters make reserved deposits "+
					"impossible: %s; reserved-deposit reveals will "+
					"revert on the Bridge until the parameters are "+
					"fixed (lower the deposit reveal-ahead period to "+
					"at most the reservation term plus the %ds refund "+
					"safety margin, or raise the amount caps to at "+
					"least the deposit dust threshold)",
				strings.Join(reasons, "; "),
				reservationDepositRefundSafetyMarginSeconds,
			)
		}
		setReservationParamsInfeasibleGauge(ratw.metricsRecorder, 1)
		return
	}

	if ratw.reservationParamsState != "" {
		// The infeasible state was fixed: clear it and note the
		// recovery.
		ratw.reservationParamsState = ""
		logger.Infof(
			"the on-chain reservation parameters are feasible again: "+
				"depositRevealAheadPeriod [%ds] fits the reservation "+
				"term plus the refund safety margin, and every "+
				"non-zero amount cap is at least the deposit dust "+
				"threshold [%d sats]",
			depositParameters.RevealAheadPeriod,
			depositParameters.DustThreshold,
		)
	}
	setReservationParamsInfeasibleGauge(ratw.metricsRecorder, 0)
}

// reservationParamsStateKey derives a stable, comparable key from the
// values checkReservationParamsFeasibility checks. The key is empty
// while the parameters are feasible; any infeasible parameter change
// produces a different non-empty key, which is what lets the ERROR log
// fire once per DISTINCT infeasible state instead of once per tick.
func reservationParamsStateKey(
	infeasible bool,
	revealAheadPeriod uint32,
	termSeconds uint32,
	dustThreshold uint64,
	maxTotalAmount uint64,
	perWalletCap uint64,
	singleAmountCap uint64,
) string {
	if !infeasible {
		return ""
	}
	return fmt.Sprintf(
		"revealAhead=%d,term=%d,dust=%d,maxTotal=%d,perWallet=%d,single=%d",
		revealAheadPeriod,
		termSeconds,
		dustThreshold,
		maxTotalAmount,
		perWalletCap,
		singleAmountCap,
	)
}

// setReservationParamsInfeasibleGauge sets the feasibility gauge through
// the recorder, treating a nil recorder - wiring without a metrics
// pipeline - as "metrics off". It also guards against a typed-nil
// *clientinfo.PerformanceMetrics boxed into the interface: the guard
// mirrors incrementReservationCounter (see its doc), where a deref of
// a typed-nil pointer would panic instead of no-op.
func setReservationParamsInfeasibleGauge(
	recorder MetricsRecorder,
	value float64,
) {
	if recorder == nil {
		return
	}
	if pm, ok := recorder.(*clientinfo.PerformanceMetrics); ok && pm == nil {
		return
	}
	recorder.SetGauge(clientinfo.MetricReservationParamsInfeasible, value)
}

// logReservationParamsReadFailure logs a parameter-read failure once
// per failure stretch: a transient RPC hiccup that persists across
// ticks is not re-logged every tick, but the next failure stretch after
// the reads succeed again is. The last known gauge value and state are
// left in place: an unknown-parameter tick neither clears nor raises
// the alarm.
func (ratw *ReservationActionTimeoutWatcher) logReservationParamsReadFailure(
	format string,
	args ...interface{},
) {
	if !ratw.reservationParamsReadFailing {
		ratw.reservationParamsReadFailing = true
		logger.Warnf(format, args...)
	}
}
