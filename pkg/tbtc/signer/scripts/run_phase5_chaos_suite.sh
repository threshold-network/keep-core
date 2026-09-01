#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST_PATH="$(cd "${SCRIPT_DIR}/.." && pwd)/Cargo.toml"

SCENARIOS=(
  "engine::tests::interactive_open_advances_only_the_opening_member_attempt|stale_interactive_attempt_replay|a newer member attempt replaces only its own live state and a stale reopen fails closed"
  "engine::tests::interactive_round2_state_key_failure_does_not_burn_attempt|round2_state_key_outage_recovery|a state-key failure releases no share, does not burn the attempt, and permits retry"
  "engine::tests::interactive_consumption_marker_survives_restart|process_restart_consumed_attempt|a consumed interactive attempt marker rejects replay across a simulated restart"
  "engine::tests::interactive_round2_persist_fault_leaves_nonces_live|round2_persist_fault_pre_rename|a pre-rename persist fault releases no share, rolls back the marker, and preserves retry"
  "engine::tests::interactive_round2_post_rename_persist_failure_consumes_attempt_and_retry_flushes|round2_persist_fault_post_rename|an after-rename persist fault releases no share, consumes the attempt, destroys live nonces, and survives restart before any successful repair"
  "engine::tests::interactive_aggregate_post_rename_persist_failure_finalizes_attempt_and_retry_flushes|aggregate_persist_fault_post_rename|an after-rename aggregate fault retains completion, destroys sibling nonces, and survives restart before any successful repair"
)

echo "Phase 5 chaos/failure-injection suite (tbtc-signer)"
echo "Manifest: ${MANIFEST_PATH}"
echo

for scenario in "${SCENARIOS[@]}"; do
  IFS="|" read -r test_name scenario_id pass_criteria <<<"${scenario}"
  echo "[RUN] ${scenario_id}"
  echo "      test: ${test_name}"
  echo "      pass: ${pass_criteria}"
  if ! test_output="$(
    cargo test --color never --manifest-path "${MANIFEST_PATH}" --lib "${test_name}" -- --exact 2>&1
  )"; then
    printf '%s\n' "${test_output}"
    exit 1
  fi
  printf '%s\n' "${test_output}"
  if [[ "${test_output}" != *"test result: ok. 1 passed; 0 failed; 0 ignored;"* ]]; then
    echo "FAIL: ${scenario_id} expected exactly one passing test for filter [${test_name}]." >&2
    exit 1
  fi
  echo
done

echo "PASS: all Phase 5 chaos/failure-injection scenarios satisfied their pass criteria."
