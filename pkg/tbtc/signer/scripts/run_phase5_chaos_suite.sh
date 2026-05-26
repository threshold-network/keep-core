#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST_PATH="$(cd "${SCRIPT_DIR}/.." && pwd)/Cargo.toml"

SCENARIOS=(
  "engine::tests::start_sign_round_rejects_stale_attempt_after_authorized_transition_across_reload|stale_payload_replay_or_duplication|stale attempt payloads remain fail-closed after authorized advancement and reload"
  "engine::tests::start_sign_round_allows_next_attempt_with_valid_transition_evidence_after_reload|restart_recovery_authorized_transition|authorized transition succeeds after restart/reload with deterministic attempt context"
  "engine::tests::start_sign_round_attempt_replay_guard_survives_process_restart_with_sign_cache_loss|process_crash_active_attempt|consumed-attempt replay guard survives simulated crash and cache loss"
  "engine::tests::persist_fault_after_temp_sync_before_rename_preserves_previous_state_on_restart|persist_fault_pre_rename|previous durable state remains intact after injected pre-rename persist fault"
  "engine::tests::persist_fault_after_rename_before_directory_sync_keeps_state_loadable_after_restart|persist_fault_post_rename|renamed durable state remains loadable after injected post-rename persist fault"
)

echo "Phase 5 chaos/failure-injection suite (tbtc-signer)"
echo "Manifest: ${MANIFEST_PATH}"
echo

for scenario in "${SCENARIOS[@]}"; do
  IFS="|" read -r test_name scenario_id pass_criteria <<<"${scenario}"
  echo "[RUN] ${scenario_id}"
  echo "      test: ${test_name}"
  echo "      pass: ${pass_criteria}"
  cargo test --manifest-path "${MANIFEST_PATH}" "${test_name}" -- --exact
  echo
done

echo "PASS: all Phase 5 chaos/failure-injection scenarios satisfied their pass criteria."
