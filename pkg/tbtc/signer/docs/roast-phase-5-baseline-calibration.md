# ROAST Phase 5 Baseline Calibration Worksheet

Date: 2026-03-01
Status: Pending environment readiness
Owner: Threshold Labs
Scope: baseline metric capture and final threshold calibration for Phase 5

## 1. Purpose

Capture baseline operational metrics and finalize Phase 5 hold/rollback
thresholds before production ROAST canary progression.

This worksheet is consumed by:

- `docs/frost-migration/roast-phase-5-security-rollout-gates.md`

## 2. Baseline Window Metadata

| Field | Value |
| --- | --- |
| Baseline window start (UTC) | `TBD` |
| Baseline window end (UTC) | `TBD` |
| Window duration | `TBD` |
| Signer fleet scope | `TBD` |
| Wallet cohort scope | `TBD` |
| Data source dashboards/queries | `TBD` |
| Environment notes | `TBD` |

## 3. Baseline Metrics Capture

| Metric | Baseline Value | Source | Notes |
| --- | --- | --- | --- |
| Attempt success rate | `TBD` | `TBD` | |
| Coordinator rotations/request (mean) | `TBD` | `TBD` | |
| Signing latency p95 | `TBD` | `TBD` | |
| Signing latency p99 | `TBD` | `TBD` | |
| Terminal failure ratio | `TBD` | `TBD` | |

## 4. Final Threshold Calibration

Use baseline values above to confirm/tune thresholds used in rollout gates.

| Trigger | Provisional Threshold | Final Threshold | Rationale |
| --- | --- | --- | --- |
| Hold: success rate | `< 99.0%` over 6h | `TBD` | |
| Rollback: success rate | `< 97.0%` over 1h | `TBD` | |
| Hold: coordinator rotations/request | `> 0.35` over 6h | `TBD` | |
| Rollback: coordinator rotations/request | `> 0.60` over 1h | `TBD` | |
| Hold: p95 latency delta | `> +25%` for 1h | `TBD` | |
| Rollback: p99 latency delta | `> +40%` for 30m | `TBD` | |
| Hold: terminal failures | `> 0.5%` for 1h | `TBD` | |
| Rollback: terminal failures | `> 1.0%` for 30m | `TBD` | |

## 5. Approval Inputs

Record completion artifacts for release or governance approval linkage:

1. baseline dashboard snapshot references (`TBD`)
2. query outputs/raw exports checksum references (`TBD`)
3. threshold update commit/PR reference (`TBD`)
4. reviewer acknowledgment references (`TBD`)
5. formal methods summary packet:
   `docs/frost-migration/formal-verification/formal-methods-summary-packet.md`

## 6. Blocker Tracking

| Blocker | Status | Owner | Notes |
| --- | --- | --- | --- |
| Testnet baseline window unavailable | `OPEN` | `UNASSIGNED` | Populate when environment is restored |
