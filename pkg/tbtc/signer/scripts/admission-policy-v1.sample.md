# admission-policy-v1.sample.json — field documentation

This document accompanies `admission-policy-v1.sample.json`. The JSON file itself
is the canonical input to the `admission_checker` binary and is consumed by the
`AdmissionPolicyV1` struct in `pkg/tbtc/signer/src/bin/admission_checker.rs`,
which is decorated with `#[serde(deny_unknown_fields)]`. Because the consumer
rejects unknown fields, no `_comment` field can be embedded in the JSON itself;
this sibling `.md` carries the documentation instead.

## Fields

| Field | Type | Required | Default behavior |
| --- | --- | --- | --- |
| `max_operators_per_provider` | unsigned integer | optional | When absent, the corresponding provider-diversity check is a no-op (unbounded). |
| `max_operators_per_region` | unsigned integer | optional | When absent, the corresponding region-diversity check is a no-op (unbounded). |
| `allowed_custody_classes` | array of strings | required | Candidate must declare one of these classes. |
| `required_attestation_status` | string | required | Candidate's `attestation_status` must equal this value. |
| `min_patch_sla_days_remaining` | unsigned integer | required | Minimum days until `patch_sla_expires_at_unix` for the candidate. |
| `require_incident_response_contact` | boolean | required | If true, candidate must provide a non-empty `incident_response_contact`. |
| `dao_override_trust_root_pubkey_hex` | hex string (32-byte x-only secp256k1) | optional | Required only when DAO override artifacts may be presented. |
| `dao_override_max_ttl_seconds` | unsigned integer | optional | When absent, the `apply_dao_override` function uses `#[serde(default)]` and defaults the TTL to **7 days (604800 seconds)** at runtime. |

## Setting these to enforce diversity caps

Set `max_operators_per_provider` and `max_operators_per_region` to the maximum
number of operators you permit per provider/region. If either is `null` or
missing, that diversity check is a no-op and the operator population is
unbounded along that axis.

## DAO override TTL

When `dao_override_max_ttl_seconds` is omitted from the policy, the
`apply_dao_override` function in `pkg/tbtc/signer/src/bin/admission_checker.rs`
treats the field as `None` via `#[serde(default)]` and substitutes a
`DEFAULT_DAO_OVERRIDE_MAX_TTL_SECONDS = 7 * 86_400` (7 days, 604800 seconds)
at runtime. The TTL of a presented override is
`expires_at_unix - approved_at_unix` and must not exceed this value.
