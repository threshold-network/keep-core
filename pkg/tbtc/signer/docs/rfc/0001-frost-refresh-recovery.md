# RFC 0001: FROST Share Refresh / Recovery Protocol

Date: 2026-08-18
Status: Draft for human review
Owner: Protocol + Security (Threshold Labs)
Issue: #4250 (Decision 3, deferred pending this RFC)
Scope: design proposal only — **no implementation in this PR**. The chosen
option is implemented as a separate, follow-up workstream.

## 1. Problem statement

### 1.1 Current behavior: fail-closed

`frost_tbtc_refresh_shares` is wired in the public ABI but is intentionally
non-functional. The FFI entrypoint is `frost_tbtc_refresh_shares` at
`pkg/tbtc/signer/src/lib.rs:379`, which delegates to
`engine::refresh_shares` at `pkg/tbtc/signer/src/engine/lifecycle.rs:601`.
That function always fails:

- It increments `refresh_shares_calls_total` telemetry
  (`pkg/tbtc/signer/src/engine/lifecycle.rs:602-605`).
- It passes the provenance gate and session-id validation
  (`pkg/tbtc/signer/src/engine/lifecycle.rs:607-608`).
- It emits a `lifecycle_policy` `reject` audit record with reason
  `cryptographic_refresh_not_supported`
  (`pkg/tbtc/signer/src/engine/lifecycle.rs:610-615`).
- It returns the terminal error
  `EngineError::CryptographicRefreshNotSupported { session_id }`
  (`pkg/tbtc/signer/src/engine/lifecycle.rs:616-618`).

The error is **terminal**, not recoverable: `EngineError::recovery_class()`
in `pkg/tbtc/signer/src/errors.rs:130` returns `"terminal"` for
`CryptographicRefreshNotSupported`. The FFI error code is
`cryptographic_refresh_not_supported`
(`pkg/tbtc/signer/src/errors.rs:107`). The ABI comment at
`pkg/tbtc/signer/src/lib.rs:40-45` states the intent verbatim:

> Major 4: RefreshShares no longer returns synthetic replacement material for a
> valid request. It fails closed with a terminal
> cryptographic_refresh_not_supported error until a real multi-round protocol
> exists.

The wire shapes `RefreshSharesRequest` (`session_id`, `current_shares`) and
`RefreshSharesResult` (`session_id`, `refresh_epoch`, `new_shares`) are
already declared in `pkg/tbtc/signer/src/api.rs:781-797` (with the
reserved-response doc comment at `api.rs:787-790`) as the reserved

### 1.2 Why this is a liveness risk

A FROST t-of-n signer set has no in-protocol way to issue new shares for
the same group key. The only way to rotate shares today is to re-run DKG
from scratch, which produces a **new group key**, breaks wallet identity,
and forces an on-chain migration (new deposit address, new redemption
flow). For tBTC that is operationally prohibitive.

`trigger_emergency_rekey` is **not** a real rekey path. It is a flag-only
kill switch:

- `engine::trigger_emergency_rekey` at
  `pkg/tbtc/signer/src/engine/lifecycle.rs:217-323` only persists an
  `EmergencyRekeyEvent { reason, triggered_at_unix }` on the session
  (`lifecycle.rs:291-296`).
- It returns a `recommended_new_session_id` of the form
  `{session_id}-rekey-{triggered_at_unix}` (`lifecycle.rs:302`) — a *new*
  session, i.e. a new DKG.
- It does **not** derive new shares, does not contact any peer, and does
  not produce a new key package. It exists so other operations can see
  the kill switch and stop signing against the (still-valid) compromised
  key.

The `RefreshCadenceStatusResult` (`pkg/tbtc/signer/src/api.rs:806-828`)
exposes `refresh_count` and `last_refresh_epoch` fields that are
**always zero** while the protocol is reserved (per the field comments
at `api.rs:809-814`), and `continuity_preserved: false` for any state
written by the retired synthetic stub (`api.rs:820-822`).

### 1.3 The operational scenario

The scenario that triggers the liveness risk is **operator cohort
degrades below the live-DKG threshold** but the underlying key material
is still on reachable-but-misbehaving or compromised operators. The two
shapes that surface today:

1. **Compromise with no on-chain rotation path.** A subset of operators
   is suspected or known compromised (e.g. CVE in an HSM, insider
   collusion below the FROST signing threshold t but above the
   DKG-disruption threshold). The key material they hold is still live;
   the group key is still the correct wallet address. There is no way
   to "re-share to honest operators only" without re-running DKG and
   migrating the on-chain identity.
2. **Stale-share rotation.** Operators join and leave over time. New
   operators can be onboarded into a new DKG, but their shares are
   shares of a **new** key. Old operators' shares can only be retired
   by an entire re-DKG.

Both shapes today force a wallet migration: new deposit address, new
redemption script, manual coordination with the bridge to rotate. That
is the liveness risk the user is asking #4250 to resolve.

## 2. Option A: Share-extension reshare

### 2.1 What the construction is

A "share-extension reshare" is a protocol that takes an existing
FROST t-of-n key package (one group public key, t-of-n Shamir shares
of the secret key) and produces a **new** key package with the **same
group public key** but a **new polynomial** and **new shares**, without
ever exposing the old shares to anyone (including the new shareholders)
beyond their existing holders, and without ever reconstructing the group
secret in one place.

The general shape is from proactive secret sharing (PSS), adapted to
FROST: each of the n existing shareholders contributes a **zero-share**
refresh sub-protocol in which they treat their current share as a
"secret" and Verifiably Secret Share (VSS) a random degree-(t-1)
polynomial whose constant term is 0, sending each new shareholder a
share of that polynomial. The new shareholders sum the sub-shares they
receive, add their old share, and obtain a new share of the **same**
secret key under a new (t, n) Shamir sharing.

The t-of-n trust model is unchanged: up to t-1 participants can be
malicious without compromising the secret, and any attempt to extract
the secret requires t colluders, same as the original DKG. The
refresh does not relax or tighten this bound.

The construction has been formalized multiple times. The closest
references a reviewer should consult:

- **Proactive secret sharing** (Herzberg, Jarecki, Krawczyk, Yung, 1995)
  is the canonical general framework; it requires a t-of-n "server"
  model, which we already have.
- **CHURP** (Maram et al., 2019) is the modern t-of-n reshare-with-CHURN
  construction that explicitly addresses *adding and removing* members
  during a refresh, which matches our scenario. It is the most natural
  academic fit for the operator-onboarding/offboarding use case.
- **FROST-specific reshare** is described in the FROST RFC
  draft (Chuengsatiansup, Crites, Stebila, 2023+) as an
  appendix; the same shape works.

> **Cryptographer sign-off required (see §7 Open Questions).** I am not
> certain which specific construction is the *exact* right fit without
> a cryptographer confirming the FROST-specific reshare appendix in
> the FROST RFC draft versus CHURP. Both have the same operator-side
> shape (zero-share VSS, sum-into-new-share), but the verification
> equations and the role of the dealer differ in ways that matter for
> the FFI surface. The implementer should pick **one** of these (most
> likely the FROST-RFC appendix) and a cryptographer should confirm
> the exact constant-term-zero VSS equations before any code is
> written.

### 2.2 Protocol flow (3 rounds over the existing transport)

Assumes the existing authenticated operator envelope transport
(see `pkg/tbtc/signer/docs/phase-7-sidecar-transport-addendum.md` and
`pkg/tbtc/signer/docs/roast-coordinator-seed-derivation.md`); no new
transport is needed.

**Round 0 — coordinator bootstrap.** A designated coordinator (one of
the existing operators, picked by the same leader-election / failover
mechanism as the current signing coordinators) collects the request to
refresh, validates the cadence gate (the cohort is within its refresh
window or governance has approved an emergency refresh), and assembles
the participant list `P = {p1, ..., pn}` and the same group public key
`Y` from the current key package. Round 0 produces a `RefreshSessionId`
distinct from any signing session id, plus a `RefreshTranscript`
header signed by the coordinator (so the round structure is
non-replayable).

**Round 1 — commitments.** Each participant `pi` samples a fresh random
polynomial `f_i(x)` of degree t-1 with `f_i(0) = 0`, computes
commitments `C_{i,k} = f_i(k)·B` for `k in {1..n}`, and broadcasts
`{C_{i,1}, ..., C_{i,n}}` to all other participants, all under the
envelope transport. Each commitment set is signed by `pi` so
equivocation (sending different commitment sets to different peers) is
attributable, exactly the same envelope-blame pattern already used in
the signing path.

**Round 2 — encrypted sub-shares.** Each participant `pi`, for each
other participant `pj`, computes the sub-share
`s_{i→j} = f_i(j)`, encrypts it under `pj`'s long-term refresh key
(separate from the signing key — see §7), and sends it directly to
`pj` via the envelope transport. Each sub-share envelope is signed by
`pi` and carries a binding to the Round 1 commitment set
`H(C_{i,1}, ..., C_{i,n})` so it cannot be replayed against a
different refresh attempt.

**Round 3 — verification + finalization.** Each participant `pj`
collects (n-1) sub-shares `s_{i→j}` for `i != j`, decrypts them, and
verifies each one against the sender's Round 1 commitment set using
Feldman / Pedersen VSS verification. Any sub-share that fails
verification produces a blame proof (sender signed the sub-share; the
commitment set is bound to the sub-share; so equivocation or a
malformed polynomial is attributable to `i`). Once all sub-shares are
verified, `pj` computes
`share'_j = share_j + Σ_{i != j} s_{i→j}` and forms a new key package
`(share'_j, Y)` with the **same** group public key `Y` (the constant
terms of all `f_i` are zero, so the group secret is unchanged; the
new share is a valid Shamir share of the same secret under a new
random polynomial). Each participant persists the new key package,
the old key package is zeroized, and the cohort announces a new
`refresh_epoch` value that flows into `RefreshCadenceStatusResult`.

The refresh is complete in 3 rounds, identical round structure to the
current DKG (`DkgPart1`/`DkgPart2`/`DkgPart3`). The transport is the
existing envelope transport. The blame-proof machinery is the same
as ROAST Round 1/2.

### 2.3 New FFI surface (rough)

Rough request/response types only; final field names and bounded-hex
deserializers should follow the existing `pkg/tbtc/signer/src/api.rs`
pattern.

```
RefreshSessionOpenRequest  { session_id, group_key_id, requested_by,
                             cadence_reason: "scheduled" | "operator_quorum" | "emergency" }
RefreshSessionOpenResult   { refresh_session_id, refresh_epoch, round1_deadline_unix,
                             participant_identifiers, group_public_key_hex }

RefreshRound1Request       { refresh_session_id, commitment_set: Vec<CommitmentEntry>,
                             commitment_set_signature_hex }
RefreshRound1Result        { refresh_session_id, accepted_commitment_sets: HashMap<u16, …>,
                             round2_deadline_unix, missing: Vec<u16> }

RefreshRound2Request       { refresh_session_id, sub_share_envelopes: Vec<SubShareEnvelope> }
RefreshRound2Result        { refresh_session_id, blame_proofs: Vec<BlameProof>,
                             round3_deadline_unix, verified_count: u16 }

RefreshFinalizeRequest     { refresh_session_id, new_share_material, new_public_key_hex,
                             new_share_signature_hex }
RefreshFinalizeResult      { refresh_session_id, refresh_epoch, continuity_preserved: true,
                             group_public_key_hex: <unchanged> }
```

The new types sit alongside the existing `RefreshSharesRequest` /
`RefreshSharesResult` (`api.rs:781-797`). Once a real protocol lands,
the one-shot `frost_tbtc_refresh_shares` entrypoint at
`pkg/tbtc/signer/src/lib.rs:379` is **replaced** with
`frost_tbtc_refresh_open` / `…_round1` / `…_round2` / `…_finalize` (or
folded into a single multi-round envelope — see §7), and the failure
mode in `engine::refresh_shares` (`lifecycle.rs:601-619`) is deleted.

### 2.4 State and persistence changes

The current `SessionState` already carries the reserved fields needed
for a refresh — see `pkg/tbtc/signer/src/engine/state.rs:130-132` and
`pkg/tbtc/signer/src/engine/persistence.rs:50-53`:

- `refresh_request_fingerprint: Option<String>` — present.
- `refresh_result: Option<RefreshSharesResult>` — present, used by
  legacy synthetic tests only.
- `refresh_history: Vec<RefreshHistoryRecord>` — present, empty for
  current sessions.

A real implementation would **add** (not modify) the following:

- `refresh_sessions: HashMap<RefreshSessionId, RefreshSessionState>`
  where `RefreshSessionState` holds `{round, round1_commitments,
  round2_sub_share_seen_from, blame_proofs, opened_at_unix,
  deadlines_per_round, participant_identifiers,
  group_public_key_hex}`. This is the new persisted state.
- A `versioned_refresh_material: VersionedRefreshMaterial` slot
  recording the **new** key package per participant, with
  `refresh_epoch` monotonic counter and `supersedes_epoch` for clean
  rollback. The existing `refresh_history` field gets a clear semantic:
  one entry per successful finalize, not per request.
- A **policy-snapshot version** (this is the same field added in
  Decision 1 — a versioned fingerprint of the policy set that was
  active when the refresh was opened). On `interactive_session_open`
  this is checked against the active policy set; mismatch rejects the
  attempt. Same pattern as the in-flight Decision 1 follow-up.

`RefreshCadenceStatusResult.refresh_count` and `last_refresh_epoch`
(`api.rs:809-814`) move from "always zero" to actually-populated
counters. `continuity_preserved` (`api.rs:820-822`) flips to `true`
for any session where a cryptographically valid refresh has been
finalized.

### 2.5 Complexity / effort estimate

**Large.** This is a brand-new multi-round cryptographic protocol on
the hot path of the wallet. Realistic breakdown:

| Sub-effort | Estimate | Why |
| --- | --- | --- |
| Cryptographer design review + construction pick | 1-2 weeks | Must confirm FROST-RFC reshare appendix vs CHURP, the exact zero-constant VSS equations, and that the new polynomial composition preserves the FROST verification equations. |
| `frost_ops` (or equivalent) implementation of the new protocol | 3-5 weeks | Three new FROST primitive functions, test vectors, golden-test corpus. |
| FFI surface (4 new request/response types, 4 new entrypoints) | 1-2 weeks | Follow existing `pkg/tbtc/signer/src/api.rs` patterns; bounded hex deserializers; serde round-trip tests. |
| SessionState + persistence changes | 2-3 weeks | New `refresh_sessions` map, new `versioned_refresh_material`, history record schema, durable state compat, replay tests. |
| Coordinator side (off-signer transport orchestration) | 2-3 weeks | Reuse the existing envelope transport; new coordinator state machine; round-deadline propagation. |
| Test coverage (unit, property, integration, fault injection) | 3-4 weeks | Equivocation attribution tests, malicious-helper blame, full-cohort refresh, partial-cohort refresh, replay-rejection, state-recovery-after-restart. |
| External security audit | 4-8 weeks | A new cryptographic protocol on the Bitcoin-custody hot path cannot ship without a fresh external audit by a firm that has reviewed the original FROST implementation. Audit cost is six figures USD. |
| DAO/operator runbook + migration guide | 1 week | Document the new operator workflow, the cadence change, the on-chain implications (which are **none** for the group key — the whole point of the protocol — but operators will want that stated explicitly). |
| **Total** | **~4-6 months** with one protocol engineer + one cryptographer + one security reviewer; audit in parallel after the impl stabilizes. |

### 2.6 Residual risks

- **Trust assumption on operators during the refresh itself.** This is
  the unavoidable cost of any t-of-n construction. During a refresh,
  up to t-1 operators can be malicious and the protocol still
  succeeds. Up to t can collude to extract the secret. **This is
  identical to the DKG trust model**; the refresh does not add or
  remove trust. The risk is that operators who already passed the
  DKG bar can re-corrupt the new polynomial, so a successful refresh
  followed by another refresh is needed to "rotate out" an
  operator-set compromise. Document this clearly to operators.
- **A malicious participant can disrupt a refresh** (send invalid
  commitments or sub-shares) and force a blame-and-abort. The blame
  proof machinery is the same as the signing path, so the existing
  f+1 accuser quorum pattern (see
  `pkg/tbtc/signer/docs/phase-7-interactive-session-spec-freeze.md:238`)
  applies. A refresh can always be re-attempted, so the liveness hit
  is bounded.
- **Replay across refresh sessions.** A participant who is removed
  from the cohort between two refreshes retains their old share and
  can try to replay old sub-share envelopes. The per-session binding
  (`H(commitment_set, refresh_session_id)`) prevents this, but only
  if the implementation enforces it — see §7.
- **Re-derivation of the group key from new shares must not happen.**
  The whole point of the protocol is the **same** group key. A bug
  that derives `Y' != Y` and persists it is a wallet-identity-loss
  bug. Needs explicit invariant test: post-refresh, the group public
  key is byte-identical to pre-refresh, and the existing
  `wallet_address` derived from `Y` is unchanged.
- **External audit gap.** Without an audit, this protocol is a
  high-stakes custom construction on a Bitcoin-custody hot path. The
  audit dependency is hard-blocking and should not be deferred.

## 3. Option B: Dealer-mediated refresh with governance

### 3.1 What the construction is

A single **dealer** (one of the existing operators, selected by a
governance-signed approval flow) constructs new shares for all
participants — but the **dealer secret** itself is split via XOR-Shamir
across the **online** operator cohort, so no single operator (or
small set) ever holds the new shares in one place.

Conceptually:

- Governance (the DAO or a multisig authorized by the DAO) emits a
  signed approval: `refresh_approval` = `{session_id, new_participants,
  new_threshold_t, new_threshold_n, refresh_reason, expires_at_unix}`,
  signed by an f+1 quorum of the operator cohort's governance keys
  (the same keys that already sign the existing per-round envelopes;
  see `pkg/tbtc/signer/docs/phase-7-sidecar-transport-addendum.md`).
- An operator elected as the **dealer** for this refresh collects the
  approval, samples a fresh degree-(t-1) polynomial `f(x)` over the
  **same secret key** (the dealer reconstructs the secret from the
  current shares via Lagrange interpolation — this is the
  *centralized-trust-moment* of the protocol, see §3.6 below), computes
  the new shares `s_i = f(i)`, and is ready to distribute.
- Instead of distributing the raw shares, the dealer XOR-Shamir splits
  each `s_i` into n_online sub-shares (where n_online is the number of
  *currently-online* operators, including the dealer), one per online
  operator, such that all n_online sub-shares XOR-reconstruct to
  `s_i`. Each operator receives a bundle of sub-shares, one per
  recipient.
- Each operator sums (XOR) the sub-shares addressed to them across
  all sender-bundles and obtains their new share `s_i`. The dealer
  zeroizes `f` and all `s_i` after distribution.
- Each operator persists the new share in their key package,
  zeroizes the old share, and the cohort announces a new
  `refresh_epoch`.

The critical property: the dealer *does* hold the secret transiently
during reconstruction, so the protocol is **not a "no single trusted
party" construction** during the refresh window. The governance
f+1-approval gate plus the dealer-rotation policy is what
de-trust-izes the moment.

### 3.2 Protocol flow (2 rounds + governance round)

**Round -1 — governance approval (off-signer).** The DAO / multisig
emits a signed `refresh_approval` envelope. This is not a signer
FFI call at all; it is an external artifact the signer must be
fed in the request. Without it, no refresh is possible.

**Round 0 — refresh open.** An operator (any of them) calls
`frost_tbtc_refresh_open` with `{session_id, refresh_approval,
  dealer_identifier, online_participants}`. The signer validates the
approval (correct signers, not expired, not replayed, governance
reason-code in the allow-list), elects / confirms the dealer, and
opens a `RefreshSessionState`. Returns the round-1 deadline.

**Round 1 — distribution.** The dealer, acting through the existing
envelope transport, sends to each online operator a bundle of
XOR-Shamir sub-shares, one per recipient. The bundle is signed by
the dealer and carries a binding to the refresh session id and
the governance approval hash, so a dealer who equivocates bundles
between recipients produces blame evidence (same envelope-blame
pattern as elsewhere). The receiving operators store the bundles
and acknowledge receipt.

**Round 2 — verification + finalization.** Each operator reconstructs
their new share by XOR-ing the sub-shares addressed to them across
all received bundles, verifies the reconstructed share against the
group public key (Feldman commitment check), and finalizes. The
dealer broadcasts a signed `refresh_complete` notification, and the
cohort moves to the new `refresh_epoch`.

The round count is lower than Option A (2 vs 3), but the
governance round is a hard external dependency that Option A does
not have.

### 3.3 New FFI surface (rough)

```
RefreshSessionOpenRequest  { session_id, refresh_approval_envelope,
                             refresh_approval_signatures: Vec<OperatorSignedApproval>,
                             dealer_identifier: u16,
                             online_participants: Vec<u16> }
RefreshSessionOpenResult   { refresh_session_id, refresh_epoch,
                             round1_deadline_unix, dealer_identifier }

RefreshRound1Request       { refresh_session_id, dealer_bundle_envelopes: Vec<DealerBundle>,
                             dealer_bundle_signature_hex }
RefreshRound1Result        { refresh_session_id, received_from: HashMap<u16, …>,
                             missing: Vec<u16>, round2_deadline_unix,
                             blame_proofs: Vec<BlameProof> }

RefreshFinalizeRequest     { refresh_session_id, new_share_material,
                             new_public_key_hex, new_share_signature_hex,
                             dealer_completion_attestation_hex }
RefreshFinalizeResult      { refresh_session_id, refresh_epoch,
                             continuity_preserved: true,
                             group_public_key_hex: <unchanged> }
```

The governance approval envelope is **not** a new FFI type — it is a
fixed-format signed payload the signer parses and verifies. The
approval signatures are the existing operator governance keys
(reused from the envelope transport).

### 3.4 State and persistence changes

Similar in shape to Option A. The differences:

- The `RefreshSessionState` adds `dealer_identifier`,
  `refresh_approval_hash`, and `online_participants` (set at open
  time, immutable for the session). Round 1 stores the dealer bundles
  received from each operator; round 2 stores the reconstructed
  share and the dealer's completion attestation.
- `versioned_refresh_material` (the new key package slot from §2.4)
  is identical.
- No new `round1_commitments` or `round2_sub_share_seen_from` map;
  those are Option-A-specific.

### 3.5 Governance / external dependencies

This is the **central operational cost** of Option B. The signer
cannot refresh on its own; it requires an external governance
artifact. Concretely, the operator team must have, or build:

1. A **DAO governance proposal** process that ratifies
   refresh-approval issuance. For tBTC this would be a
   `RefreshOperatorCohort` proposal type on the Threshold DAO
   (TokenThreshold / TimeLock pattern), or a multisig-controlled
   approval registry for low-value refreshes.
2. A **signing path** for the f+1 governance approvals, using
   keys held by the operator cohort — *not* the FROST signing keys,
   which must not be used for governance to avoid key reuse. This
   is a new key ceremony / key registry, with its own rotation
   schedule and its own security model.
3. A **replay / revocation registry**: each `refresh_approval`
   must be one-shot (the signer tracks consumed approval hashes) and
   revocable before round 1 closes (in case the approved refresh
   turns out to be unnecessary, e.g. the suspected compromise was
   a false alarm).
4. An **operator-runbook** for who the current `dealer_identifier`
   is, how the dealer is rotated, and what to do if the dealer
   drops mid-refresh.

Without (1)-(4), the signer can never refresh. This is a real
operational dependency that Option A does not have.

### 3.6 Complexity / effort estimate

**Medium-large**, but with the same external-audit dependency as
Option A. The cryptographic surface is smaller (no multi-party
zero-share VSS), but the governance and approval-orchestration
work is new and falls outside the signer.

| Sub-effort | Estimate | Why |
| --- | --- | --- |
| Cryptographer review of dealer + XOR-Shamir split | 1-2 weeks | The "dealer holds the secret transiently" moment is the trust load. Must confirm: dealer-rotation policy is sufficient, XOR-Shamir-with-online-cohort is sound under the t-of-n assumption, the post-distribution dealer-zeroize is enforced. |
| `frost_ops` dealer primitive + verify | 2-3 weeks | One new FROST primitive (sample new polynomial at known secret), test vectors, golden tests. |
| FFI surface (3 new request/response types, 3 new entrypoints) | 1-2 weeks | Same pattern as Option A. |
| SessionState + persistence changes | 1-2 weeks | Smaller than Option A (no round-1 commitment map, no round-2 sub-share map). |
| Governance approval workflow (DAO proposal type, multisig option, key registry, replay/revocation registry) | 4-8 weeks | **Off-signer work** — separate repo, separate audit, separate governance ratification timeline. |
| Operator runbook | 1 week | New operator workflow for refresh-approval and dealer selection. |
| Test coverage | 2-3 weeks | Equivocation, malicious-dealer blame, partial-cohort refresh, replay-rejection, governance-revocation mid-refresh. |
| External security audit (signer) | 3-6 weeks | Smaller than Option A's audit (less novel crypto), but still hard-required. |
| External security audit (governance workflow) | 2-4 weeks | The approval / revocation registry is its own audit surface. |
| **Total** | **~3-5 months** if the governance workflow is already built, **+3-6 months** if the workflow has to be designed and ratified. |

### 3.7 Residual risks

- **The dealer holds the secret transiently.** This is the load-bearing
  trust assumption. The f+1 governance approval mitigates it (the
  dealer is selected by governance, not self-nominated), but does not
  eliminate it. A compromised dealer during a refresh window extracts
  the secret. Compared to Option A, where no single party ever holds
  the secret during the refresh, this is a strict downgrade.
- **Liveness dependency on governance.** If the DAO is
  governance-stuck (low voter turnout, hostile proposal, multisig
  holders unreachable), **no refresh is possible**. This converts a
  cryptographic liveness problem into a governance liveness problem,
  which may be worse. Need to be honest about this: the original
  problem was "we cannot recover from operator degradation without
  re-DKG"; the new problem is "we cannot recover from operator
  degradation without governance being able to ratify a refresh
  within the cadence window."
- **f+1 governance approvals requires the operators themselves to be
  reachable.** If the operators are *offline* (the very scenario the
  refresh is meant to recover from), the governance f+1 quorum
  cannot form, and the refresh is impossible. **Option B cannot
  recover from an offline operator set.** Option A *can* (it only
  needs t online operators to form the new polynomial — same as
  signing).
- **Replay / revocation drift.** A revoked `refresh_approval` must
  propagate to every signer before round 1 closes, or a stale
  approval gets consumed by a malicious operator. This is a
  distributed-consistency problem the governance registry must
  solve.
- **Key reuse between governance and signing.** If the same key
  signs refresh-approvals and signs FROST signing shares, a
  compromise of one is a compromise of both. The governance keys
  must be distinct, which is a key-ceremony cost.

## 4. Comparison

| Dimension | Option A: share-extension reshare | Option B: dealer-mediated + governance |
| --- | --- | --- |
| **Cryptographic novelty / audit burden** | New multi-round FROST sub-protocol (zero-share VSS, new polynomial composition). High novelty. Needs external audit by a FROST-experienced firm. **~4-8 week audit.** | Single new primitive (sample polynomial at known secret) + XOR-Shamir split. Lower novelty. Needs external audit of signer changes **and** the governance approval registry. **~3-6 week signer audit + 2-4 week governance audit.** |
| **Implementation effort** | Large: new FROST primitive trio, 4 FFI entrypoints, new persisted state map, blame-proof integration. **~4-6 months** with protocol engineer + cryptographer + security reviewer. | Medium-large: 1 new FROST primitive, 3 FFI entrypoints, smaller persisted state. **~3-5 months** if governance workflow already exists; **+3-6 months** to build the governance workflow. |
| **New external dependencies** | None new — uses the existing envelope transport, existing operator governance keys, existing session model. | DAO governance proposal type, governance signing key registry, replay/revocation registry, operator runbook for dealer selection. **All of these must be built and audited.** |
| **Operator / governance burden** | Low. Operators run the protocol like a DKG; no governance action required. Governance only gates the *cadence* (scheduled vs emergency) via the existing `cadence_reason` field. | High. Every refresh requires f+1 operator governance signatures, a DAO-or-multisig approval, a designated dealer, and a revocation-window monitor. Operator runbook is meaningfully larger. |
| **Recovery from fully-compromised-but-still-online set** | **Yes.** As long as t honest operators participate in the refresh, the new shares are sound and the compromised operators' old shares are zeroized post-refresh. The compromised operators' *new* shares (they get new shares too) are also valid, but they no longer have the *old* share which was the compromise vector. | **Partially.** Governance must be able to approve the refresh against a compromised set, which requires the governance keys to be on a path *separate* from the compromised operators. If the compromise extends to operator governance keys, Option B cannot refresh. |
| **Recovery from offline set** | **Yes, partial.** Needs t *online* operators to form the new polynomial — same as signing. Operators that are offline simply don't get new shares and are removed from the cohort. Re-onboarding them later requires a new refresh (or DKG, if cohort shrinks below n). | **No.** f+1 operator governance signatures cannot form if the operator set is offline. This is the central liveness gap. |
| **Residual trust assumptions** | The t-of-n trust model is unchanged. The protocol is secure against up to t-1 malicious participants; t colluders still extract the secret. The refresh itself does not add trust. | The dealer holds the secret transiently during each refresh. Governance holds the f+1 approval power. The protocol is secure iff the dealer is honest during the refresh window AND the governance quorum is uncompromised. **Two new trust loci** vs zero for Option A. |
| **Wallet-identity continuity** | Same group key, byte-identical. Existing on-chain identity preserved. | Same group key, byte-identical. Existing on-chain identity preserved. |
| **Re-key cadence after compromise** | Refresh once with the compromised operators included (they get new shares and the old ones are zeroized); refresh again later to remove them if needed. Each refresh is fully cryptographic; no governance gate. | One refresh per governance approval. To remove a compromised operator, governance must approve a refresh that excludes them — two governance actions (current refresh + re-share after exclusion). |
| **Long-term maintenance** | The protocol is self-contained inside the signer; upgrades track FROST RFC evolution. | The signer is coupled to the governance registry's wire format; every governance-registry change is a signer upgrade. |

## 5. Recommendation

**Option A: share-extension reshare.**

### 5.1 Why

1. **It actually solves the original problem.** The scenario in §1.3
   is "operator cohort degrades below the live-DKG threshold, key
   material is on reachable-but-misbehaving or compromised operators."
   Option A can refresh with t online operators and re-establish
   clean shares. Option B *cannot* refresh if the operator set is
   offline — it converts a cryptographic liveness problem into a
   governance liveness problem, which is the *opposite* of the
   direction the user wanted to go.
2. **It adds no new trust loci.** Option A's trust model is
   *identical* to the DKG trust model: t honest operators, t-1
   tolerable bad ones. Option B introduces a dealer (transient secret
   holder) and a governance f+1 quorum (new signing key set), both of
   which are new attack surfaces the original DKG does not have.
3. **It adds no new external dependencies.** The signer stays
   self-contained. The governance workflow for Option B is a
   3-6 month parallel workstream that does not exist today and has
   its own audit surface.
4. **The effort difference is smaller than it looks.** Option A is
   ~4-6 months, Option B is ~3-5 months **plus** the governance
   workstream that doesn't exist. Total elapsed time is comparable;
   the cryptographic novelty is comparable; Option A has the better
   operational story.
5. **It is the academic-canonical solution.** FROST's own RFC draft
   describes a reshare appendix. CHURP exists for the add/remove
   case. The shape is well-understood. Option B's
   dealer-mediated-with-XOR-Shamir construction is **not** a
   standard construction — it is an ad-hoc composition that needs
   its own cryptographer sign-off *and* its own audit, with no
   prior art to lean on.

### 5.2 What would change this recommendation

- If a **f+1 governance approval + dealer-mediated refresh is
  already built and audited** in the governance stack (it is not
  today; I checked `pkg/tbtc/signer/docs/`), then Option B's
  incremental cost drops dramatically and the cost-benefit shifts.
  Worth re-running this RFC when that is true.
- If the **FROST-RFC reshare appendix turns out to be insufficient**
  for some property we need (e.g. it cannot add/remove members,
  only re-share to the *same* set), and CHURP turns out to require
  a fundamentally different transport than the existing envelope,
  then the engineering surface of Option A grows. The
  cryptographer review in §7 should confirm this before commitment.
- If the **external audit cost** for Option A is materially
  larger than for Option B (it likely is not, since both have
  novel crypto surfaces, but worth pricing), then the
  cost-benefit narrows. Worth a procurement conversation with
  the audit firm before final commitment.

**Confidence: medium-high.** The reasoning is solid, but it leans
on §7's open questions being resolved in Option A's favor
(specifically: the FROST-RFC reshare appendix is sound for our
threshold and member-set semantics, and the existing envelope
transport carries the new sub-share envelopes without an upgrade).
If §7 turns up a showstopper in any of those, fall back to Option B
*with the governance workstream explicitly scoped and scheduled*,
not as an "incremental later."

## 6. Acceptance criteria (from issue #4250, restated for the implementer)

The eventual implementer must satisfy all of the following before the
reserved `frost_tbtc_refresh_shares` FFI can be re-enabled and the
fail-closed error in `engine::refresh_shares`
(`pkg/tbtc/signer/src/engine/lifecycle.rs:601-619`) can be removed:

1. **Refresh produces new shares against the same group key.** After
   a successful refresh, the group public key is byte-identical to
   pre-refresh (no on-chain migration, no wallet-address change). The
   new key package is persisted in the new
   `versioned_refresh_material` slot, and the old key package is
   zeroized.

2. **No full DKG re-run is required for refresh.** A refresh does not
   run the DKG state machine. The DKG fields on `SessionState`
   remain unchanged. A new `RefreshSessionId` is allocated per
   refresh attempt, distinct from any `session_id` (signing) and
   from any prior `RefreshSessionId`.

3. **Recovery from a degraded operator set down to t-of-n (Option A)
   or to a f+1-reachable governance quorum (Option B).** A cohort
   that has lost operators down to t online members can still
   complete a refresh in Option A. A cohort that has lost operators
   down to f+1 governance-reachable members can still complete a
   refresh in Option B, provided those f+1 members can ratify the
   governance approval and one of them is willing to act as dealer.

4. **`refresh_shares` (or its multi-round replacement) returns
   `RecoveryClass::Recoverable` instead of `Terminal`.** The error
   code `cryptographic_refresh_not_supported` is removed from
   `EngineError` (`pkg/tbtc/signer/src/errors.rs:36, 107, 130`).
   The new FFI surface returns success on a complete refresh and
   intermediate recoverable errors on partial failures
   (e.g. `refresh_round_timeout` for a round-1 / round-2 deadline
   breach, `refresh_blame_proof` for a malicious participant).

5. **External audit coverage.** The chosen option's cryptographic
   implementation is reviewed and signed off by an external auditor
   with FROST / threshold-cryptography experience. The audit
   report is published (or summarized) alongside the implementation
   PR. If Option B is chosen, the governance approval registry is
   also audited.

6. **Operator migration guide.** A new doc
   (`pkg/tbtc/signer/docs/<phase>-frost-refresh-operator-guide.md` or
   similar) covering: the new `frost_tbtc_refresh_open` /
   `…_round1` / `…_round2` / `…_finalize` FFI surface, the
   `RefreshCadenceStatusResult` field semantics, the operator
   runbook for triggering a scheduled vs emergency refresh, the
   on-chain implications (none — group key preserved), and the
   rollback story if a refresh fails mid-protocol.

7. **Continuity invariants preserved.** The `last_refresh_epoch` and
   `refresh_count` fields in `RefreshCadenceStatusResult`
   (`pkg/tbtc/signer/src/api.rs:809-814`) move from "always zero"
   to actually-populated. `continuity_preserved: true` is set after
   any cryptographically valid refresh finalize. The
   `RefreshCadenceStatus` cadence calculation uses the new
   `refresh_history` entries.

8. **Equivocation / blame on the refresh path is attributable.** A
   participant who sends inconsistent commitment sets or sub-share
   envelopes produces a blame proof that is recorded, surfaced via
   the FFI, and consumed by the same f+1 accuser quorum pattern that
   governs signing-path blame
   (`pkg/tbtc/signer/docs/phase-7-interactive-session-spec-freeze.md:238`).

9. **Replay rejection across refresh sessions.** A sub-share or
   commitment set from a prior refresh cannot be replayed into a
   new refresh session. The session-binding (commitment-set hash +
   refresh session id) is verified on every sub-share envelope.

10. **Cadence integration.** The existing `RefreshCadenceStatus` FFI
    is updated to report the new epoch / count / next-due
    information sourced from the cryptographically valid
    `refresh_history`, not the legacy synthetic stub.

## 7. Open questions for the human reviewer

These are the points where I had to guess, where the construction
depends on a specific academic result, or where a cryptographer
sign-off is hard-required before implementation. **Do not start
implementation until these are resolved.**

1. **Which exact construction: FROST-RFC reshare appendix or
   CHURP?** §2.1 outlines both. The implementer should pick **one**,
   and a cryptographer should confirm the chosen construction's
   equations are sound for our (t, n) parameter range, member
   add/remove semantics, and FROST verification-equation
   compatibility. (The implementer should *not* invent a new
   construction; the audit cost of "novel custom construction" is
   materially higher than "standard construction adapted to our
   transport.")
2. **Zero-constant VSS verification under Feldman vs Pedersen.**
   Pedersen commitments hide the secret but require a discrete-log
   setup; Feldman commitments are transparent but leak the
   commitment-to-share relationship. The choice has FFI
   implications (the commitment type travels over the wire) and
   audit implications (Pedersen requires trusted setup for
   perfect hiding). Which do we use?
3. **Sub-share encryption key.** §2.2 assumes each participant has a
   long-term refresh encryption key, separate from the signing key.
   Where does that key come from? Is it derived from the operator
   governance key, or is it a new key ceremony? The existing
   envelope transport uses operator signing keys for
   authentication; we need a separate key pair for confidentiality
   of the sub-shares. New key-ceremony cost.
4. **Dealer-rotation policy for Option B.** If the dealer drops
   mid-refresh, what happens? Re-open the session with a new
   dealer (requires a new governance approval)? Elect a
   next-in-line from the cohort? This is an open protocol-design
   question that the implementer should answer before
   implementation, and the answer affects the FFI surface (do we
   need a `refresh_reopen` entrypoint?).
5. **Governance key separation (Option B).** §3.7 notes that
   governance keys must be distinct from signing keys. The
   current `pkg/tbtc/signer/docs/` does not document whether such
   a key set exists. If it does not, Option B's incremental cost
   includes a key-ceremony workstream that is not captured in
   §3.6.
6. **Refresh cadence after a refresh.** §2.4 / §3.4 add a
   `refresh_epoch` counter, but how does the cadence calculation
   in `RefreshCadenceStatus` change? Specifically: does a
   successful refresh reset the next-due window, or does it
   preserve the existing deadline? The current comment in
   `pkg/tbtc/signer/src/engine/lifecycle.rs:159-162` (around the
   synthetic-stub anchor) hints at the policy direction but does
   not pin it down for a real protocol.
7. **What does "t online" mean in practice for Option A?** A
   refresh requires t online participants. If only t-1 operators
   are reachable, the refresh cannot complete. Is that an
   acceptable failure mode? (The current `frost_tbtc_refresh_shares`
   failure is also terminal, so the status quo is *worse* — but
   the implementer should confirm the "t online" semantics is
   acceptable, not silently assume it.)
8. **Concurrent refresh attempts.** The current `refresh_shares`
   request uses `session_id` and rejects conflicts; the new
   multi-round protocol must do the same. The implementer should
   confirm the conflict-detection logic is preserved across the
   round boundaries (one refresh session open at a time per
   group key, or per operator, or both).
9. **Interaction with Decision 1 (policy-snapshot version).** §2.4
   references the same `versioned_refresh_material` slot. The
   exact interaction — does a policy-snapshot change during a
   refresh abort the refresh, or does the policy version bind to
   the refresh open and stay fixed through finalize? — is an
   open design point that should be resolved alongside Decision 1
   in the same PR or a tightly-coupled follow-up.
10. **Interaction with the existing envelope-blame / f+1 accuser
    quorum.** §2.6 and §3.7 both assume the existing f+1 accuser
    quorum (the same one that gates signing-path blame) is the
    blame gate for refresh-path blame. The implementer should
    confirm this is acceptable, or propose a separate refresh
    quorum (which would be more cost).
11. **External audit procurement.** §2.5 and §3.6 both call out
    the external audit as a hard dependency. The procurement
    timeline (4-8 weeks for Option A, 3-6 weeks for Option B
    signer-side + 2-4 weeks for Option B governance-side) is on
    the critical path. The implementer should engage the audit
    firm *before* implementation starts, not after, to avoid a
    post-impl audit queue.

## 8. References

- Issue #4250 (GitHub) — Decision 3, deferred pending this RFC.
- `pkg/tbtc/signer/src/lib.rs:40-45` — ABI 4 fail-closed comment.
- `pkg/tbtc/signer/src/lib.rs:379-388` — `frost_tbtc_refresh_shares`
  FFI entrypoint.
- `pkg/tbtc/signer/src/engine/lifecycle.rs:601-619` —
  `engine::refresh_shares` fail-closed implementation.
- `pkg/tbtc/signer/src/engine/lifecycle.rs:217-323` —
  `engine::trigger_emergency_rekey` flag-only stub.
- `pkg/tbtc/signer/src/api.rs:781-797` — `RefreshSharesRequest` /
  `RefreshSharesResult` reserved wire shapes.
- `pkg/tbtc/signer/src/api.rs:806-828` — `RefreshCadenceStatusResult`
  and the always-zero comments at lines 809-814.
- `pkg/tbtc/signer/src/errors.rs:33-36, 107, 130` — terminal
  classification of `CryptographicRefreshNotSupported`.
- `pkg/tbtc/signer/src/engine/state.rs:130-132` and
  `pkg/tbtc/signer/src/engine/persistence.rs:50-53` — reserved
  `refresh_request_fingerprint` / `refresh_result` /
  `refresh_history` fields.
- `pkg/tbtc/signer/docs/roast-phase-4-liveness-policy-recovery.md` —
  existing recoverable-vs-terminal semantics.
- `pkg/tbtc/signer/docs/phase-7-interactive-session-spec-freeze.md:238`
  — f+1 accuser quorum convention.
- `pkg/tbtc/signer/docs/phase-7-sidecar-transport-addendum.md` —
  envelope transport (assumed for the new sub-share envelopes).
- `pkg/tbtc/signer/docs/permissioned-signer-hardening-rfc.md:55` —
  P2-M1 refresh-reshare policy placeholder; this RFC is the
  proposal that milestone points to.
- Herzberg, Jarecki, Krawczyk, Yung (1995) — Proactive Secret
  Sharing (canonical PSS framework).
- Maram et al. (2019) — CHURP (dynamic-group t-of-n reshare).
- Chuengsatiansup, Crites, Stebila — FROST RFC draft, reshare
  appendix (FROST-specific construction).
