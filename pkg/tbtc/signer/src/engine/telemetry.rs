// Hardening telemetry: latency trackers and metrics reporting.

use super::*;

pub(crate) static HARDENING_TELEMETRY: OnceLock<Mutex<HardeningTelemetryState>> = OnceLock::new();

pub(crate) const HARDENING_LATENCY_SAMPLE_WINDOW: usize = 256;

fn canary_sample_is_fresh(
    observed_at: Instant,
    evaluated_at: Instant,
    max_age_seconds: u64,
) -> bool {
    evaluated_at
        .checked_duration_since(observed_at)
        .is_some_and(|age| age <= Duration::from_secs(max_age_seconds))
}

#[derive(Clone, Copy, Debug)]
pub(crate) struct HardeningLatencySample {
    pub(crate) duration_ms: u64,
    pub(crate) observed_at: Instant,
}

#[derive(Default)]
pub(crate) struct HardeningLatencyTracker {
    pub(crate) samples: VecDeque<HardeningLatencySample>,
}

impl HardeningLatencyTracker {
    pub(crate) fn record(&mut self, duration_ms: u64) {
        self.record_at(duration_ms, Instant::now());
    }

    pub(crate) fn record_at(&mut self, duration_ms: u64, observed_at: Instant) {
        if self.samples.len() >= HARDENING_LATENCY_SAMPLE_WINDOW {
            self.samples.pop_front();
        }
        self.samples.push_back(HardeningLatencySample {
            duration_ms,
            observed_at,
        });
    }

    pub(crate) fn p95_ms(&self) -> u64 {
        Self::p95(self.samples.iter().map(|sample| sample.duration_ms))
    }

    pub(crate) fn fresh_p95_ms(&self, evaluated_at: Instant, max_age_seconds: u64) -> u64 {
        Self::p95(
            self.samples
                .iter()
                .filter(|sample| {
                    canary_sample_is_fresh(sample.observed_at, evaluated_at, max_age_seconds)
                })
                .map(|sample| sample.duration_ms),
        )
    }

    fn p95(samples: impl Iterator<Item = u64>) -> u64 {
        let mut sorted_samples = samples.collect::<Vec<_>>();
        if sorted_samples.is_empty() {
            return 0;
        }
        sorted_samples.sort_unstable();
        let p95_index = (sorted_samples.len() * 95).div_ceil(100).saturating_sub(1);
        sorted_samples[p95_index]
    }

    pub(crate) fn sample_count(&self) -> u64 {
        self.samples.len() as u64
    }

    pub(crate) fn fresh_sample_count(&self, evaluated_at: Instant, max_age_seconds: u64) -> u64 {
        self.samples
            .iter()
            .filter(|sample| {
                canary_sample_is_fresh(sample.observed_at, evaluated_at, max_age_seconds)
            })
            .count() as u64
    }

    pub(crate) fn clear(&mut self) {
        self.samples.clear();
    }
}

#[derive(Clone, Copy, Debug)]
pub(crate) struct HardeningPolicyOutcomeSample {
    pub(crate) rejected: bool,
    pub(crate) observed_at: Instant,
}

#[derive(Default)]
pub(crate) struct HardeningPolicyOutcomeTracker {
    pub(crate) samples: VecDeque<HardeningPolicyOutcomeSample>,
}

impl HardeningPolicyOutcomeTracker {
    pub(crate) fn record(&mut self, rejected: bool) {
        self.record_at(rejected, Instant::now());
    }

    pub(crate) fn record_at(&mut self, rejected: bool, observed_at: Instant) {
        if self.samples.len() >= HARDENING_LATENCY_SAMPLE_WINDOW {
            self.samples.pop_front();
        }
        self.samples.push_back(HardeningPolicyOutcomeSample {
            rejected,
            observed_at,
        });
    }

    pub(crate) fn fresh_snapshot(&self, evaluated_at: Instant, max_age_seconds: u64) -> (u64, u64) {
        let mut sample_count = 0u64;
        let mut rejected_count = 0u64;
        for sample in self.samples.iter().filter(|sample| {
            canary_sample_is_fresh(sample.observed_at, evaluated_at, max_age_seconds)
        }) {
            sample_count = sample_count.saturating_add(1);
            if sample.rejected {
                rejected_count = rejected_count.saturating_add(1);
            }
        }
        (sample_count, rejected_count)
    }

    pub(crate) fn clear(&mut self) {
        self.samples.clear();
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
    pub(crate) heartbeat_signing_policy_reject_total: u64,
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
    // Promotion evidence is intentionally separate from the ABI-3 latency
    // metrics above. The public metrics retain the full rolling window of all
    // calls, while these trackers contain only successful operations from the
    // current rollout stage and may be cleared between stages.
    pub(crate) canary_interactive_round1_latency: HardeningLatencyTracker,
    pub(crate) canary_interactive_round2_latency: HardeningLatencyTracker,
    pub(crate) canary_interactive_aggregate_latency: HardeningLatencyTracker,
    pub(crate) canary_policy_outcomes: HardeningPolicyOutcomeTracker,
    // Incremented whenever rollout-stage evidence is reset. A successful
    // interactive operation that straddles that boundary must not be credited
    // to the new stage.
    pub(crate) canary_evidence_epoch: u64,
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
    pub(crate) record_canary_on_drop: bool,
    pub(crate) canary_evidence_epoch: Option<u64>,
}

impl HardeningOperationLatencyGuard {
    pub(crate) fn new(operation: HardeningOperation) -> Self {
        Self {
            operation,
            started_at: Instant::now(),
            record_canary_on_drop: false,
            canary_evidence_epoch: None,
        }
    }

    pub(crate) fn success_only(operation: HardeningOperation) -> Self {
        Self {
            operation,
            started_at: Instant::now(),
            record_canary_on_drop: false,
            canary_evidence_epoch: current_canary_evidence_epoch(),
        }
    }

    pub(crate) fn mark_success(&mut self) {
        self.record_canary_on_drop = true;
    }
}

impl Drop for HardeningOperationLatencyGuard {
    fn drop(&mut self) {
        // Record latency with millisecond precision and ceil semantics so
        // sub-millisecond calls still contribute non-zero samples.
        let elapsed_micros = self.started_at.elapsed().as_micros();
        let elapsed_ms = elapsed_micros.div_ceil(1000).clamp(1, u64::MAX as u128) as u64;
        record_hardening_operation_latency_for_epoch(
            self.operation,
            elapsed_ms,
            self.record_canary_on_drop,
            self.canary_evidence_epoch,
        );
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

fn current_canary_evidence_epoch() -> Option<u64> {
    match hardening_telemetry_state().lock() {
        Ok(telemetry) => Some(telemetry.canary_evidence_epoch),
        Err(error) => {
            eprintln!("warning: hardening telemetry mutex poisoned: {error}");
            None
        }
    }
}

#[cfg(test)]
pub(crate) fn record_hardening_operation_latency(operation: HardeningOperation, duration_ms: u64) {
    record_hardening_operation_latency_for_epoch(operation, duration_ms, true, None);
}

fn record_hardening_operation_latency_for_epoch(
    operation: HardeningOperation,
    duration_ms: u64,
    record_canary_success: bool,
    expected_canary_evidence_epoch: Option<u64>,
) {
    record_hardening_telemetry(|telemetry| {
        match operation {
            HardeningOperation::BuildTaprootTx => {
                telemetry.build_taproot_tx_latency.record(duration_ms)
            }
            HardeningOperation::RefreshShares => {
                telemetry.refresh_shares_latency.record(duration_ms)
            }
            HardeningOperation::InteractiveRound1 => {
                telemetry.interactive_round1_latency.record(duration_ms)
            }
            HardeningOperation::InteractiveRound2 => {
                telemetry.interactive_round2_latency.record(duration_ms)
            }
            HardeningOperation::InteractiveAggregate => {
                telemetry.interactive_aggregate_latency.record(duration_ms)
            }
        }

        if !record_canary_success
            || expected_canary_evidence_epoch
                .is_some_and(|expected| expected != telemetry.canary_evidence_epoch)
        {
            return;
        }

        match operation {
            HardeningOperation::InteractiveRound1 => telemetry
                .canary_interactive_round1_latency
                .record(duration_ms),
            HardeningOperation::InteractiveRound2 => telemetry
                .canary_interactive_round2_latency
                .record(duration_ms),
            HardeningOperation::InteractiveAggregate => telemetry
                .canary_interactive_aggregate_latency
                .record(duration_ms),
            HardeningOperation::BuildTaprootTx | HardeningOperation::RefreshShares => {}
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
        heartbeat_signing_policy_reject_total: 0,
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
            result.heartbeat_signing_policy_reject_total =
                telemetry.heartbeat_signing_policy_reject_total;
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

pub(crate) fn record_canary_policy_outcome(rejected: bool) {
    record_hardening_telemetry(|telemetry| {
        telemetry.canary_policy_outcomes.record(rejected);
    });
}

pub(crate) fn reset_canary_promotion_evidence() {
    record_hardening_telemetry(|telemetry| {
        telemetry.canary_interactive_round1_latency.clear();
        telemetry.canary_interactive_round2_latency.clear();
        telemetry.canary_interactive_aggregate_latency.clear();
        telemetry.canary_policy_outcomes.clear();
        telemetry.canary_evidence_epoch = telemetry.canary_evidence_epoch.saturating_add(1);
    });
}

#[cfg(test)]
pub(crate) fn seed_canary_promotion_evidence_for_tests(
    round1_latency_ms: u64,
    round2_latency_ms: u64,
    aggregate_latency_ms: u64,
    policy_rejects: u64,
) {
    let interactive_sample_count = canary_min_samples();
    let policy_sample_count = canary_min_policy_samples();
    record_hardening_telemetry(|telemetry| {
        for _ in 0..interactive_sample_count {
            telemetry
                .interactive_round1_latency
                .record(round1_latency_ms);
            telemetry
                .canary_interactive_round1_latency
                .record(round1_latency_ms);
            telemetry
                .interactive_round2_latency
                .record(round2_latency_ms);
            telemetry
                .canary_interactive_round2_latency
                .record(round2_latency_ms);
            telemetry
                .interactive_aggregate_latency
                .record(aggregate_latency_ms);
            telemetry
                .canary_interactive_aggregate_latency
                .record(aggregate_latency_ms);
        }
        for index in 0..policy_sample_count {
            telemetry
                .canary_policy_outcomes
                .record(index < policy_rejects);
        }
    });
}

pub(crate) fn canary_promotion_gate_failures() -> Vec<String> {
    let mut failures = Vec::new();

    // Each rollout stage needs its own non-vacuous window of recent successful
    // production operations. Telemetry is process-local by design, so a restart
    // clears the window and blocks promotion until fresh evidence accumulates.
    let minimum_samples = canary_min_samples();
    let minimum_policy_samples = canary_min_policy_samples();
    let now = Instant::now();
    let max_age = canary_max_sample_age_seconds();
    let (
        round1_sample_count,
        round1_p95_ms,
        round2_sample_count,
        round2_p95_ms,
        aggregate_sample_count,
        aggregate_p95_ms,
        policy_sample_count,
        policy_reject_count,
    ) = hardening_telemetry_state()
        .lock()
        .map(|telemetry| {
            let (policy_sample_count, policy_reject_count) = telemetry
                .canary_policy_outcomes
                .fresh_snapshot(now, max_age);
            (
                telemetry
                    .canary_interactive_round1_latency
                    .fresh_sample_count(now, max_age),
                telemetry
                    .canary_interactive_round1_latency
                    .fresh_p95_ms(now, max_age),
                telemetry
                    .canary_interactive_round2_latency
                    .fresh_sample_count(now, max_age),
                telemetry
                    .canary_interactive_round2_latency
                    .fresh_p95_ms(now, max_age),
                telemetry
                    .canary_interactive_aggregate_latency
                    .fresh_sample_count(now, max_age),
                telemetry
                    .canary_interactive_aggregate_latency
                    .fresh_p95_ms(now, max_age),
                policy_sample_count,
                policy_reject_count,
            )
        })
        .unwrap_or_else(|error| {
            eprintln!("warning: hardening telemetry mutex poisoned: {error}");
            (0, 0, 0, 0, 0, 0, 0, 0)
        });
    for (operation, sample_count, p95_ms, max_p95_ms) in [
        (
            "interactive_round1",
            round1_sample_count,
            round1_p95_ms,
            canary_max_interactive_round1_p95_ms(),
        ),
        (
            "interactive_round2",
            round2_sample_count,
            round2_p95_ms,
            canary_max_interactive_round2_p95_ms(),
        ),
        (
            "interactive_aggregate",
            aggregate_sample_count,
            aggregate_p95_ms,
            canary_max_interactive_aggregate_p95_ms(),
        ),
    ] {
        if sample_count < minimum_samples {
            failures.push(format!(
                "{operation} fresh successful samples [{sample_count}] below canary minimum [{minimum_samples}]"
            ));
        } else if p95_ms > max_p95_ms {
            failures.push(format!(
                "{operation} p95 latency [{p95_ms}ms] exceeds canary gate [{max_p95_ms}ms]"
            ));
        }
    }

    let max_policy_reject_rate_bps = canary_max_policy_reject_rate_bps();
    if policy_sample_count < minimum_policy_samples {
        failures.push(format!(
            "build_taproot_tx fresh policy samples [{policy_sample_count}] below canary policy minimum [{minimum_policy_samples}]"
        ));
    } else {
        let policy_reject_rate_bps = policy_reject_count
            .saturating_mul(TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS)
            .saturating_div(policy_sample_count);
        if policy_reject_rate_bps > max_policy_reject_rate_bps {
            failures.push(format!(
                "build_taproot_tx policy reject rate [{}bps] exceeds canary gate [{}bps]",
                policy_reject_rate_bps, max_policy_reject_rate_bps
            ));
        }
    }

    failures
}

#[cfg(test)]
pub(crate) fn canary_missing_evidence_gate_failures() -> Vec<String> {
    let minimum_samples = canary_min_samples();
    let minimum_policy_samples = canary_min_policy_samples();
    vec![
        format!(
            "interactive_round1 fresh successful samples [0] below canary minimum [{minimum_samples}]"
        ),
        format!(
            "interactive_round2 fresh successful samples [0] below canary minimum [{minimum_samples}]"
        ),
        format!(
            "interactive_aggregate fresh successful samples [0] below canary minimum [{minimum_samples}]"
        ),
        format!(
            "build_taproot_tx fresh policy samples [0] below canary policy minimum [{minimum_policy_samples}]"
        ),
    ]
}
