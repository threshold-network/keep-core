# Phase 7: Interactive Signing Session — Spec Freeze

Date: 2026-06-12
Status: Proposed (freezes on signer + keep-core owner sign-off)
Owner: Threshold Labs
Scope: the hardened interactive two-round FROST signing session — the
production signing path — with t-of-included finalize native from the
start, and the deletion plan for the transitional deterministic-nonce
flow.

## 1. Objective

Make the interactive two-round FROST exchange the production signing
path, carrying the full session hardening that today exists only in
the coarse transitional path, and finalize with the first `t`
responsive members of the included set (t-of-included) so no single
included member can veto an attempt. Redemption signings adopt the
path first (slashing-backed deadlines; gates-doc decision 5).

Non-goals of this spec: bounded `n-t+1` concurrent attempts
(fast-follow — section 8 reserves the room it needs), DKG redesign
(the interactive DKG primitives ship as-is for now), and the wallet
recovery-leaf question (explicitly open; nothing here may bake in a
key-path-only assumption — the session layer takes the Taproot
merkle root as an input, as today).

## 2. Inherited decisions (settled; cite, do not relitigate)

From the Phase 5 gates-doc Decision Log (2026-06-12) and the merged
review stack:

1. **t-of-included finalize is Phase 7's first engineering item**
   (decision 5). The transitional flow cannot be retrofitted: it
   derives every participant's commitments against the full included
   set at `StartSignRound`, and `finalize_sign_round` rejects any
   contribution outside the declared signing-participant set — so
   first-t-responsive requires the interactive exchange.
2. **The interactive flow is designed t-of-included-NATIVE**
   (decision 6). No deterministic-coexistence constraint: the
   transitional deterministic-nonce path is FROZEN (marker at
   `src/engine/nonce.rs` header) and committed for deletion when the
   trigger in section 7 fires.
3. **Sidecar process boundary is the target architecture**
   (decision 2). The session API in section 5 is therefore
   transport-shaped: idempotent request/response, no shared-memory
   assumptions, every call replayable against a restarted signer
   with an identical fail-closed outcome. The dlopen bridge remains
   the transitional transport; moving to the sidecar must be a
   transport swap, not an API rework.
4. **Production signing is interactive-FROST-only with OS
   randomness** (production profile, since the #4028 hardening).
5. **Coordinator-seed derivation is normative** (RFC-21 Annex A);
   attempt contexts and their hashes are the cross-layer binding
   (RFC-21); evidence rides signed-body envelopes,
   sign-what-you-transmit (#4040).
6. **The external audit is a hard gate for ECDSA retirement**
   (decision 1) and this session layer is in its scope: the spec
   and its vectors are audit inputs.

## 3. Current state (verified in code, 2026-06-12)

* Stateless interactive primitives exist in
  `src/engine/frost_ops.rs`: `dkg_part1/2/3`,
  `generate_nonces_and_commitments`, `new_signing_package`,
  `sign_share`, `aggregate`. They enforce the provenance gate but
  **bypass every other layer of session hardening**: no replay
  registries, no attempt-context validation, no consumed tracking,
  no policy gates, no persistence.
* **Secret nonce custody is the host's** in the stateless flow:
  `generate_nonces_and_commitments` returns serialized
  `SigningNonces` to the caller and `sign_share` accepts them back
  as a request field (`nonces_hex`). Between rounds the secret
  nonces live in Go memory and cross the FFI twice; single-use is
  enforced by caller discipline only — calling `sign_share` twice
  with the same nonces and different messages is the canonical
  FROST key-extraction failure and nothing in the engine prevents
  it today.
* The coarse transitional path holds the hardening inventory to
  carry over: consumed sign/finalize round registries with
  fail-closed capacity behavior, strict-mode attempt-context
  validation (Annex A derivation, Go-parity pinned by test),
  policy gates, durable state with restart safety (persist-fault
  chaos coverage), audit/telemetry events.
* Go side: the RFC-21 machinery is implemented and dormant behind
  `frost_roast_retry` (coordinator state machine, evidence
  recorder, Phase 7.1 bundle production, Phase 7.2 bundle-consuming
  participant selector). `pkg/tbtc/signing_loop.go` still runs
  serial attempts with the legacy `signingAttemptSeed`; its
  migration to Annex A is part of this phase.

## 4. The load-bearing change: nonce custody moves inside the engine

The session layer's defining property: **secret nonces never cross
the FFI and never persist.**

* `InteractiveRound1` generates nonces via OS randomness inside the
  engine, stores them in session-scoped memory keyed by
  `(session_id, attempt_id, key_package)`, zeroizes on consumption,
  and returns only the public commitments plus an opaque
  `nonce_handle`.
* `InteractiveRound2` takes the signing package and the
  `nonce_handle`; the engine atomically (a) marks the handle
  consumed, (b) produces the signature share, (c) zeroizes the
  nonces. A second call with the same handle fails closed with a
  structured `consumed_nonce_replay` error. Consumption-before-
  release ordering: the consumed marker is durable (or the nonce
  irrecoverable) before the share leaves the engine.
* Nonces are **never written to durable state**. Restart loses
  in-flight nonces by construction: the attempt fails and the next
  attempt generates fresh ones. The persisted artifacts are only
  consumption markers and session metadata. This makes the cloned-
  state attack class from the transitional threat model structurally
  irrelevant: two clones produce *different* fresh nonces; neither
  can be induced to sign twice under one nonce pair, because the
  pair exists only inside one process's memory and is consumed
  atomically.

This is also the audit story for the FFI boundary: after Phase 7,
no secret signing material (key shares already env/command-only;
now nonces too) transits the Go/Rust interface in either direction.

## 5. Session model and API contract

An interactive session is identified by `(session_id,
attempt_context)` where the attempt context is the RFC-21 structure
(message digest per Annex A, attempt number, included set,
coordinator) and its hash binds every message, as in the coarse
path's strict mode.

Engine API (names final at freeze; all requests carry the attempt
context, all calls are idempotent-or-fail-closed, all responses are
self-contained):

1. `InteractiveSessionOpen` — validates attempt context (strict
   mode is the only mode here: no legacy-shape fallback), checks
   policy gates and provenance, registers the session. Idempotent
   by full-request fingerprint; conflicting reopen fails closed.
2. `InteractiveRound1` — fresh nonces + commitments as in section
   4. Per (session, attempt, member) at most one live handle;
   repeat calls return the same commitments (idempotent) until
   consumed, then fail closed.
3. `InteractiveRound2` — input: the coordinator's signing package
   (the chosen responsive subset's commitment list). The engine
   verifies (a) own membership in the subset, (b) the subset is a
   subset of the attempt's included set, (c) `|subset| == t`,
   (d) every commitment is well-formed, (e) attempt-context binding
   — then consumes the nonce handle and returns the share.
4. `InteractiveAggregate` — coordinator-side: collects shares
   against the signing package, verifies each share against the
   member's verifying share before aggregation (share verification
   is what converts "invalid contribution" into attributable blame
   evidence), produces the BIP-340 signature, marks the session
   complete in the consumed registry.
5. `InteractiveSessionAbort` — explicit teardown; consumes any
   live nonce handles; idempotent.

Registry semantics: the consumed-registries pattern from the coarse
path applies per call family, with the same fail-closed capacity
behavior; keys carry `(session_id, attempt_id)` so bounded
concurrency (section 8) extends them without weakening replay
protection.

## 6. t-of-included semantics and evidence

* Members submit round-1 commitments to the attempt's coordinator
  (Annex A selection). The coordinator forms the signing package
  from the **first `t` responsive included members** — arrival
  order, no waiting window in v1 (open question 3 proposes the
  default).
* **Safety does not depend on the coordinator's honesty.** FROST
  binds each share to the exact commitment list in the signing
  package; a coordinator equivocating different subsets to
  different members yields shares that cannot aggregate — a
  liveness failure, not a soundness failure. The engine-side checks
  in `InteractiveRound2` (membership, subset-of-included, size
  `t`) bound what a malicious coordinator can request at all.
* **Liveness failures must be attributable.** The signing package
  the coordinator distributes is a signed-body envelope (#4040
  pattern, operator key): members retain the received bytes, so a
  coordinator that equivocates packages within one attempt has
  produced self-incriminating evidence — the same
  `EquivocationEvidence` retention path added by #4044 extends to
  package envelopes. A coordinator that stalls is rotated by the
  existing RFC-21 transition machinery; a member whose share fails
  verification in `InteractiveAggregate` becomes re-checkable blame
  evidence (the proof-carrying-blame roadmap consumes this; the
  f+1 accuser quorum remains the exclusion gate until then).
* Under these semantics a silent included member costs zero
  attempts — it is simply not among the first `t` responders — and
  Annex B's sampling table stops binding liveness. The
  `performance_signing_attempt_*` gauges stay in place and should
  show the regime change on testnet.

## 7. Transitional-path deletion trigger (decision 6, made precise)

"Interactive production path validated end to end" means all of:

1. The interactive session layer passes the Phase-5-equivalent
   suites: replay, restart-safety (incl. consumed-nonce-marker
   ordering under injected persist faults), and the chaos matrix
   extended with coordinator-equivocation and first-t-subset cases.
2. Go orchestration drives interactive signing on a real testnet
   deployment through the full retry/rotation machinery, including
   at least one attempt that finalizes with a strict subset of the
   included set (a real t-of-included finalize, not n-of-n).
3. The cross-language vectors for the new wire structs (signing
   package envelope, round-1/round-2 messages) are pinned on both
   sides, regen-disciplined like the existing corpora.

When all three hold: delete the transitional
`StartSignRound`/`FinalizeSignRound` deterministic flow and the
`RoundNonceBinding` machinery (`src/engine/nonce.rs`), migrate the
tests that pin them, and update the gates doc. The freeze marker in
`nonce.rs` names this document as its trigger definition.

## 8. Bounded concurrency (reserved, not built)

Up to `n-t+1` concurrent attempts per session is the fast-follow.
This spec reserves: attempt-scoped registry keys (already the
shape), attempt-scoped nonce handles (section 5), and the rule that
concurrent attempts never share nonce material. What it does NOT
prescribe: concurrent-attempt scheduling policy or cross-attempt
share reuse (forbidden by construction — one handle, one attempt).

## 9. Phasing (PR-sized, in order)

* **7.0** — this spec freeze (+ #4007 sidecar scoping addendum:
  transport mapping of the section-5 API; separate doc, same
  review).
* **7.1** — engine session layer: session registry, nonce custody
  (section 4), `InteractiveSessionOpen/Round1/Round2/Abort`,
  consumed registries, persistence of markers, unit + restart
  tests. (mirror)
* **7.2** — `InteractiveAggregate` + share verification + package
  envelope evidence; FFI surface + cross-language vectors. (mirror,
  vectors copied per regen discipline)
* **7.3** — Go interactive executor: signing_loop migration to the
  session API, Annex A attempt-seed adoption (retiring the legacy
  `signingAttemptSeed`), wiring into the RFC-21 retry/selector
  machinery; redemptions first behind the existing readiness
  gating. (scaffold)
* **7.4** — t-of-included evidence integration: package-envelope
  retention, equivocation observer extension, blame-evidence
  surfacing. (scaffold + mirror)
* **7.5** — e2e/chaos extension + testnet validation run →
  section 7 trigger fires → transitional-path deletion PR +
  readiness-manifest flip with attached evidence.
* **7.6** — bounded concurrency (fast-follow, own mini-spec).

## 10. Open questions this freeze forces (proposed defaults)

1. **Signing-package distribution channel**: dedicated topic signed
   with the operator key (consistent with RFC-21's resolved
   coordinator-proposed-aggregation decision) — proposed default —
   vs. piggybacking the existing session channel.
2. **Round-1 commitment transport**: members → coordinator only
   (paper-ROAST shape; proposed default) vs. broadcast to all
   (more evidence, more traffic; revisit with bounded concurrency).
3. **Responsive-subset policy**: strict first-t arrival order
   (proposed default: simplest, no fairness window) vs. a short
   gather window with deterministic tie-break. Fairness across
   operators is an economics question; default to first-t and
   revisit on testnet telemetry.
4. **Session-state durability**: markers-only (proposed default,
   per section 4) vs. resumable round-1 state. Resumability
   contradicts never-persist-nonces; default is markers-only and
   a crashed member simply misses that attempt.

## 11. Freeze acceptance criteria

* Signer and keep-core owners sign off on sections 4-7 with no
  unresolved ambiguity on nonce lifecycle, subset-choice
  verification, or the deletion trigger.
* Open questions in section 10 carry decisions (default or
  overridden), recorded in the gates-doc Decision Log.
* The audit scope statement references this document and names the
  section-5 API as in-scope.
