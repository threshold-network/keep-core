# admission-existing.sample.json — documentation

This document accompanies `admission-existing.sample.json`. The JSON file is a
top-level array of already-admitted operator records, which is the format the
`admission_checker` binary parses into `Vec<ExistingOperator>` (see
`pkg/tbtc/signer/src/bin/admission_checker.rs`). Because the file is a JSON
array at the top level, it cannot carry a sibling `_comment` field; this
`.md` sibling carries the documentation.

## Role

The "existing operators" list represents the **already-admitted operator
lookup** that the admission checker uses when evaluating provider/region
diversity caps. For a candidate to be admitted, the candidate's
`(provider, region)` pair must not push the count of operators in either
bucket above the policy's `max_operators_per_provider` /
`max_operators_per_region` (whichever is configured; missing caps are
treated as unbounded).

## Per-entry schema

Each entry in the array has exactly three fields:

| Field | Type | Description |
| --- | --- | --- |
| `operator_id` | string | Stable operator identifier (matches the candidate's `operator_id`). |
| `provider` | string | Cloud/hosting provider (e.g. `aws`, `gcp`, `azure`). |
| `region` | string | Geographic region identifier (e.g. `us-east-1`, `europe-west1`). |

The `ExistingOperator` struct does not declare `#[serde(deny_unknown_fields)]`,
so unrecognized fields are silently ignored on deserialize.

## Producing this file

The list is typically regenerated from the on-chain signer registry or the
DAO-administered operator manifest before each admission run. The
`admission_checker --existing PATH` flag accepts this file; if omitted, the
checker treats the existing set as empty.

## Difference from `admission-candidate.sample.json`

- `admission-candidate.sample.json` (object) = the **incoming operator entry**
  being evaluated against the active policy.
- `admission-existing.sample.json` (array) = the **already-admitted operator
  lookup** used for diversity-cap checks.
