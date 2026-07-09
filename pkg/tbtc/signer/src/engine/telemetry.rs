// Hardening telemetry: latency trackers and metrics reporting.

use super::*;

pub(crate) static HARDENING_TELEMETRY: OnceLock<Mutex<HardeningTelemetryState>> = OnceLock::new();

pub(crate) const HARDENING_LATENCY_SAMPLE_WINDOW: usize = 256;

#[derive(Default)]
pub(crate) struct HardeningLatencyTracker {
    pub(crate) samples_ms: VecDeque<u64>,
}

impl HardeningLatencyTracker {
    pub(crate) fn record(&mut self, duration_ms: u64) {
        if self.samples_ms.len() >= HARDENING_LATENCY_SAMPLE_WINDOW {
            self.samples_ms.pop_front();
        }
        self.samples_ms.push_back(duration_ms);
    }

    pub(crate) fn p95_ms(&self) -> u64 {
        if self.samples_ms.is_empty() {
            return 0;
        }

        let mut sorted_samples = self.samples_ms.iter().copied().collect::<Vec<_>>();
        sorted_samples.sort_unstable();
        let p95_index = (sorted_samples.len() * 95).div_ceil(100).saturating_sub(1);
        sorted_samples[p95_index]
    }

    pub(crate) fn sample_count(&self) -> u64 {
        self.samples_ms.len() as u64
    }
}

#[derive(Default)]
pub(crate) struct HardeningTelemetryState {
    pub(crate) run_dkg_calls_total: u64,
    pub(crate) run_dkg_success_total: u64,
    pub(crate) run_dkg_admission_reject_total: u64,
    pub(crate) start_sign_round_calls_total: u64,
    pub(crate) start_sign_round_success_total: u64,
    pub(crate) build_taproot_tx_calls_total: u64,
    pub(crate) build_taproot_tx_success_total: u64,
    pub(crate) build_taproot_tx_policy_reject_total: u64,
    pub(crate) finalize_sign_round_calls_total: u64,
    pub(crate) finalize_sign_round_success_total: u64,
    pub(crate) refresh_shares_calls_total: u64,
    pub(crate) refresh_shares_success_total: u64,
    pub(crate) roast_transcript_audit_calls_total: u64,
    pub(crate) roast_transcript_audit_success_total: u64,
    pub(crate) verify_blame_proof_calls_total: u64,
    pub(crate) verify_blame_proof_success_total: u64,
    pub(crate) attempt_transition_total: u64,
    pub(crate) coordinator_failover_total: u64,
    pub(crate) auto_quarantine_fault_events_total: u64,
    pub(crate) auto_quarantine_enforcements_total: u64,
    pub(crate) differential_fuzz_runs_total: u64,
    pub(crate) differential_fuzz_critical_divergence_total: u64,
    pub(crate) canary_promotions_total: u64,
    pub(crate) canary_rollbacks_total: u64,
    pub(crate) interactive_session_open_calls_total: u64,
    pub(crate) interactive_session_open_success_total: u64,
    pub(crate) interactive_round1_calls_total: u64,
    pub(crate) interactive_round1_success_total: u64,
    pub(crate) interactive_round2_calls_total: u64,
    pub(crate) interactive_round2_success_total: u64,
    pub(crate) interactive_session_abort_calls_total: u64,
    pub(crate) interactive_session_abort_success_total: u64,
    pub(crate) interactive_aggregate_calls_total: u64,
    pub(crate) interactive_aggregate_success_total: u64,
    pub(crate) run_dkg_latency: HardeningLatencyTracker,
    pub(crate) start_sign_round_latency: HardeningLatencyTracker,
    pub(crate) build_taproot_tx_latency: HardeningLatencyTracker,
    pub(crate) finalize_sign_round_latency: HardeningLatencyTracker,
    pub(crate) refresh_shares_latency: HardeningLatencyTracker,
    pub(crate) interactive_round1_latency: HardeningLatencyTracker,
    pub(crate) interactive_round2_latency: HardeningLatencyTracker,
    pub(crate) interactive_aggregate_latency: HardeningLatencyTracker,
    pub(crate) last_updated_unix: u64,
}

#[derive(Clone, Copy)]
pub(crate) enum HardeningOperation {
    BuildTaprootTx,
    RefreshShares,
    // Interactive Open/Abort are O(1) registry mutations and record
    // call/success counters only; the cryptographic rounds and the
    // aggregation get latency tracking.
    InteractiveRound1,
    InteractiveRound2,
    InteractiveAggregate,
}

pub(crate) struct HardeningOperationLatencyGuard {
    pub(crate) operation: HardeningOperation,
    pub(crate) started_at: Instant,
}

impl HardeningOperationLatencyGuard {
    pub(crate) fn new(operation: HardeningOperation) -> Self {
        Self {
            operation,
            started_at: Instant::now(),
        }
    }
}

impl Drop for HardeningOperationLatencyGuard {
    fn drop(&mut self) {
        // Record latency with millisecond precision and ceil semantics so
        // sub-millisecond calls still contribute non-zero samples.
        let elapsed_micros = self.started_at.elapsed().as_micros();
        let elapsed_ms = elapsed_micros.div_ceil(1000).clamp(1, u64::MAX as u128) as u64;
        record_hardening_operation_latency(self.operation, elapsed_ms);
    }
}

pub(crate) fn hardening_telemetry_state() -> &'static Mutex<HardeningTelemetryState> {
    HARDENING_TELEMETRY.get_or_init(|| Mutex::new(HardeningTelemetryState::default()))
}

pub(crate) fn record_hardening_telemetry<F>(update: F)
where
    F: FnOnce(&mut HardeningTelemetryState),
{
    match hardening_telemetry_state().lock() {
        Ok(mut telemetry) => {
            update(&mut telemetry);
            telemetry.last_updated_unix = now_unix();
        }
        Err(error) => {
            eprintln!("warning: hardening telemetry mutex poisoned: {error}");
        }
    }
}

pub(crate) fn record_hardening_operation_latency(operation: HardeningOperation, duration_ms: u64) {
    record_hardening_telemetry(|telemetry| match operation {
        HardeningOperation::BuildTaprootTx => {
            telemetry.build_taproot_tx_latency.record(duration_ms)
        }
        HardeningOperation::RefreshShares => telemetry.refresh_shares_latency.record(duration_ms),
        HardeningOperation::InteractiveRound1 => {
            telemetry.interactive_round1_latency.record(duration_ms)
        }
        HardeningOperation::InteractiveRound2 => {
            telemetry.interactive_round2_latency.record(duration_ms)
        }
        HardeningOperation::InteractiveAggregate => {
            telemetry.interactive_aggregate_latency.record(duration_ms)
        }
    });
}

pub fn hardening_metrics() -> SignerHardeningMetricsResult {
    let mut result = SignerHardeningMetricsResult {
        runtime_version: TBTC_SIGNER_RUNTIME_VERSION.to_string(),
        provenance_enforced: provenance_gate_enforced(),
        admission_policy_enforced: admission_policy_enforced(),
        signing_policy_firewall_enforced: signing_policy_firewall_enforced(),
        run_dkg_calls_total: 0,
        run_dkg_success_total: 0,
        run_dkg_admission_reject_total: 0,
        start_sign_round_calls_total: 0,
        start_sign_round_success_total: 0,
        build_taproot_tx_calls_total: 0,
        build_taproot_tx_success_total: 0,
        build_taproot_tx_policy_reject_total: 0,
        finalize_sign_round_calls_total: 0,
        finalize_sign_round_success_total: 0,
        refresh_shares_calls_total: 0,
        refresh_shares_success_total: 0,
        roast_transcript_audit_calls_total: 0,
        roast_transcript_audit_success_total: 0,
        verify_blame_proof_calls_total: 0,
        verify_blame_proof_success_total: 0,
        attempt_transition_total: 0,
        coordinator_failover_total: 0,
        auto_quarantine_fault_events_total: 0,
        auto_quarantine_enforcements_total: 0,
        quarantined_operator_count: 0,
        refresh_cadence_overdue_sessions: 0,
        emergency_rekey_sessions_total: 0,
        differential_fuzz_runs_total: 0,
        differential_fuzz_critical_divergence_total: 0,
        canary_promotions_total: 0,
        canary_rollbacks_total: 0,
        run_dkg_latency_p95_ms: 0,
        run_dkg_latency_samples: 0,
        start_sign_round_latency_p95_ms: 0,
        start_sign_round_latency_samples: 0,
        build_taproot_tx_latency_p95_ms: 0,
        build_taproot_tx_latency_samples: 0,
        finalize_sign_round_latency_p95_ms: 0,
        finalize_sign_round_latency_samples: 0,
        refresh_shares_latency_p95_ms: 0,
        refresh_shares_latency_samples: 0,
        interactive_session_open_calls_total: 0,
        interactive_session_open_success_total: 0,
        interactive_round1_calls_total: 0,
        interactive_round1_success_total: 0,
        interactive_round2_calls_total: 0,
        interactive_round2_success_total: 0,
        interactive_session_abort_calls_total: 0,
        interactive_session_abort_success_total: 0,
        interactive_aggregate_calls_total: 0,
        interactive_aggregate_success_total: 0,
        interactive_round1_latency_p95_ms: 0,
        interactive_round1_latency_samples: 0,
        interactive_round2_latency_p95_ms: 0,
        interactive_round2_latency_samples: 0,
        interactive_aggregate_latency_p95_ms: 0,
        interactive_aggregate_latency_samples: 0,
        last_updated_unix: 0,
    };

    match hardening_telemetry_state().lock() {
        Ok(telemetry) => {
            result.run_dkg_calls_total = telemetry.run_dkg_calls_total;
            result.run_dkg_success_total = telemetry.run_dkg_success_total;
            result.run_dkg_admission_reject_total = telemetry.run_dkg_admission_reject_total;
            result.start_sign_round_calls_total = telemetry.start_sign_round_calls_total;
            result.start_sign_round_success_total = telemetry.start_sign_round_success_total;
            result.build_taproot_tx_calls_total = telemetry.build_taproot_tx_calls_total;
            result.build_taproot_tx_success_total = telemetry.build_taproot_tx_success_total;
            result.build_taproot_tx_policy_reject_total =
                telemetry.build_taproot_tx_policy_reject_total;
            result.finalize_sign_round_calls_total = telemetry.finalize_sign_round_calls_total;
            result.finalize_sign_round_success_total = telemetry.finalize_sign_round_success_total;
            result.refresh_shares_calls_total = telemetry.refresh_shares_calls_total;
            result.refresh_shares_success_total = telemetry.refresh_shares_success_total;
            result.roast_transcript_audit_calls_total =
                telemetry.roast_transcript_audit_calls_total;
            result.roast_transcript_audit_success_total =
                telemetry.roast_transcript_audit_success_total;
            result.verify_blame_proof_calls_total = telemetry.verify_blame_proof_calls_total;
            result.verify_blame_proof_success_total = telemetry.verify_blame_proof_success_total;
            result.attempt_transition_total = telemetry.attempt_transition_total;
            result.coordinator_failover_total = telemetry.coordinator_failover_total;
            result.auto_quarantine_fault_events_total =
                telemetry.auto_quarantine_fault_events_total;
            result.auto_quarantine_enforcements_total =
                telemetry.auto_quarantine_enforcements_total;
            result.differential_fuzz_runs_total = telemetry.differential_fuzz_runs_total;
            result.differential_fuzz_critical_divergence_total =
                telemetry.differential_fuzz_critical_divergence_total;
            result.canary_promotions_total = telemetry.canary_promotions_total;
            result.canary_rollbacks_total = telemetry.canary_rollbacks_total;
            result.run_dkg_latency_p95_ms = telemetry.run_dkg_latency.p95_ms();
            result.run_dkg_latency_samples = telemetry.run_dkg_latency.sample_count();
            result.start_sign_round_latency_p95_ms = telemetry.start_sign_round_latency.p95_ms();
            result.start_sign_round_latency_samples =
                telemetry.start_sign_round_latency.sample_count();
            result.build_taproot_tx_latency_p95_ms = telemetry.build_taproot_tx_latency.p95_ms();
            result.build_taproot_tx_latency_samples =
                telemetry.build_taproot_tx_latency.sample_count();
            result.finalize_sign_round_latency_p95_ms =
                telemetry.finalize_sign_round_latency.p95_ms();
            result.finalize_sign_round_latency_samples =
                telemetry.finalize_sign_round_latency.sample_count();
            result.refresh_shares_latency_p95_ms = telemetry.refresh_shares_latency.p95_ms();
            result.refresh_shares_latency_samples = telemetry.refresh_shares_latency.sample_count();
            result.interactive_session_open_calls_total =
                telemetry.interactive_session_open_calls_total;
            result.interactive_session_open_success_total =
                telemetry.interactive_session_open_success_total;
            result.interactive_round1_calls_total = telemetry.interactive_round1_calls_total;
            result.interactive_round1_success_total = telemetry.interactive_round1_success_total;
            result.interactive_round2_calls_total = telemetry.interactive_round2_calls_total;
            result.interactive_round2_success_total = telemetry.interactive_round2_success_total;
            result.interactive_session_abort_calls_total =
                telemetry.interactive_session_abort_calls_total;
            result.interactive_session_abort_success_total =
                telemetry.interactive_session_abort_success_total;
            result.interactive_aggregate_calls_total = telemetry.interactive_aggregate_calls_total;
            result.interactive_aggregate_success_total =
                telemetry.interactive_aggregate_success_total;
            result.interactive_round1_latency_p95_ms =
                telemetry.interactive_round1_latency.p95_ms();
            result.interactive_round1_latency_samples =
                telemetry.interactive_round1_latency.sample_count();
            result.interactive_round2_latency_p95_ms =
                telemetry.interactive_round2_latency.p95_ms();
            result.interactive_round2_latency_samples =
                telemetry.interactive_round2_latency.sample_count();
            result.interactive_aggregate_latency_p95_ms =
                telemetry.interactive_aggregate_latency.p95_ms();
            result.interactive_aggregate_latency_samples =
                telemetry.interactive_aggregate_latency.sample_count();
            result.last_updated_unix = telemetry.last_updated_unix;
        }
        Err(error) => {
            eprintln!("warning: hardening telemetry mutex poisoned: {error}");
        }
    }

    if let Ok(state) = state() {
        if let Ok(engine_state) = state.lock() {
            result.quarantined_operator_count =
                engine_state.quarantined_operator_identifiers.len() as u64;
            result.emergency_rekey_sessions_total = engine_state
                .sessions
                .values()
                .filter(|session| session.emergency_rekey_event.is_some())
                .count() as u64;
            result.refresh_cadence_overdue_sessions = engine_state
                .sessions
                .values()
                .filter(|session| {
                    session.refresh_history.last().is_some_and(|last_refresh| {
                        now_unix()
                            > last_refresh
                                .refreshed_at_unix
                                .saturating_add(refresh_cadence_seconds())
                    })
                })
                .count() as u64;
        }
    }

    result
}

pub(crate) fn canary_policy_reject_rate_bps(metrics: &SignerHardeningMetricsResult) -> u64 {
    if metrics.build_taproot_tx_calls_total == 0 {
        return 0;
    }

    metrics
        .build_taproot_tx_policy_reject_total
        .saturating_mul(TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS)
        .saturating_div(metrics.build_taproot_tx_calls_total)
}

pub(crate) fn canary_promotion_gate_failures(
    metrics: &SignerHardeningMetricsResult,
) -> Vec<String> {
    let mut failures = Vec::new();

    let max_start_sign_round_p95_ms = canary_max_start_sign_round_p95_ms();
    if metrics.start_sign_round_latency_samples > 0
        && metrics.start_sign_round_latency_p95_ms > max_start_sign_round_p95_ms
    {
        failures.push(format!(
            "start_sign_round p95 latency [{}ms] exceeds canary gate [{}ms]",
            metrics.start_sign_round_latency_p95_ms, max_start_sign_round_p95_ms
        ));
    }

    let max_finalize_sign_round_p95_ms = canary_max_finalize_sign_round_p95_ms();
    if metrics.finalize_sign_round_latency_samples > 0
        && metrics.finalize_sign_round_latency_p95_ms > max_finalize_sign_round_p95_ms
    {
        failures.push(format!(
            "finalize_sign_round p95 latency [{}ms] exceeds canary gate [{}ms]",
            metrics.finalize_sign_round_latency_p95_ms, max_finalize_sign_round_p95_ms
        ));
    }

    let max_policy_reject_rate_bps = canary_max_policy_reject_rate_bps();
    let policy_reject_rate_bps = canary_policy_reject_rate_bps(metrics);
    if policy_reject_rate_bps > max_policy_reject_rate_bps {
        failures.push(format!(
            "build_taproot_tx policy reject rate [{}bps] exceeds canary gate [{}bps]",
            policy_reject_rate_bps, max_policy_reject_rate_bps
        ));
    }

    failures
}
