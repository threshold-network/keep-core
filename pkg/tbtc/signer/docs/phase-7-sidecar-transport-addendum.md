# Phase 7.0 Addendum: Sidecar Transport Mapping

Date: 2026-06-12
Status: Proposed (same review process as the Phase 7 spec freeze)
Owner: Threshold Labs
Scope: maps the frozen interactive-session API
(`phase-7-interactive-session-spec-freeze.md`, section 5) onto the
sidecar process boundary chosen in Decision Log entry 2, and scopes
what that boundary means for #4007 (the decision-gated TEE checker
stack). This document changes no contract: the sidecar is a
transport swap by construction, and anything here that would alter
the frozen spec is a defect in this document.

## 1. What the sidecar is

A separate OS process that owns the signer engine and every secret
it holds: key-share state, the state-encryption key path, and (after
Phase 7.1) the in-memory interactive nonces. The keep-client host
process — Go runtime, libp2p, Ethereum client, every transitive
dependency — talks to it over local IPC and holds no signing
secrets at any time.

The isolation claim, stated precisely: today a memory-disclosure
bug anywhere in the host address space can read whatever the
in-process engine holds, because the dlopen FFI is an API boundary,
not a security boundary. The sidecar makes the boundary an OS
process boundary. It is also the deliberate stepping stone to the
TEE deployment: a sidecar process becomes an enclave process with
the same wire protocol, which is precisely why decision 2 told
isolation-sensitive work to assume this shape.

## 2. Why the frozen API maps cleanly

Two prior decisions did the work in advance:

* The engine API is already coarse JSON request/response over a C
  ABI — chosen over round-level FFI compatibility partly FOR
  "cleaner future sidecar extraction"
  (`signer-api-contract-decision-brief.md`).
* The frozen section-5 calls are idempotent-or-fail-closed,
  self-contained request/response with no callbacks and no shared
  memory.

One tension to resolve explicitly: the old decision brief argued
against round-level APIs because they kept "nonce/round details
crossing the FFI boundary" and made the transport swap harder. The
Phase 7 API *is* round-level (`Round1`/`Round2`) — interactivity is
forced by true two-round FROST with a network exchange between
rounds — but the brief's actual objection is dissolved by the
frozen spec's section 4: rounds cross the boundary, **nonces do
not**. What transits is public commitments, signing packages, and
shares. The chattiness objection is inherent to interactive FROST
and is bounded (two round trips per attempt against a ~41-block
attempt budget; the Annex B arithmetic gives ~175x headroom).

## 3. Transport mapping

Same JSON envelopes, different carrier:

| Engine call (frozen spec §5 / existing API) | dlopen transitional | Sidecar |
|---|---|---|
| `InstallNativeTBTCSignerConfig` (init) | `frost_tbtc_init_signer_config` symbol | First request after connect (handshake step 2) |
| `InteractiveSessionOpen/Round1/Round2/Aggregate/Abort` | per-call symbols (Phase 7.1/7.2) | One method each, identical JSON bodies |
| Coarse transitional calls (until deleted per spec §7) | existing symbols | Same mapping rule |

Carrier (proposed defaults, section 8): a UNIX domain socket with
length-prefixed JSON frames, a small connection pool, and exactly
one in-flight request per connection. No request multiplexing in
v1: the engine's concurrency model and registries are unchanged,
and the pool bounds parallelism exactly as the host's call sites do
today. Errors keep the structured `ErrorResponse` contract
(`consumed_attempt_replay` etc.) — the codes are the cross-version
interface and MUST NOT fork between transports.

Transport conformance: the contract tests that pin the FFI behavior
become transport-parameterized — the same request/response suites
run against the dlopen bridge and the sidecar, and divergence is a
release blocker. This is the mechanism that keeps "transport swap,
not API rework" true over time.

## 4. Process model and lifecycle

* **Spawn/supervision (proposed default)**: keep-client spawns the
  sidecar as a child process and supervises it (restart with
  backoff). The alternative — independent systemd unit — is open
  question (a); the child model keeps the operator surface to one
  service and lets the existing init-config demand semantics apply
  without a coordination protocol.
* **Handshake**: (1) version exchange — the host refuses to operate
  a sidecar outside its supported range, fail closed; (2) init-
  config install — the host reads `TBTC_SIGNER_INIT_CONFIG_PATH`
  and posts the install request as the first message, exactly the
  #4037/#4041 flow. **Decision 7 carries over unchanged**: with the
  path set, a sidecar that cannot be spawned, cannot complete the
  handshake, or rejects the config is process-fatal for the host,
  in every profile. The enforcement point
  (`enforceNativeInitConfigDemand`) gains "sidecar unreachable" as
  one more member of the same failure family.
* **Crash semantics**: a sidecar crash loses in-flight nonces — by
  the frozen spec's section 4 and ratified question 4
  (markers-only), this is exactly the restart story: live attempts
  fail safe, durable consumption markers prevent any replay, the
  supervisor restarts the sidecar, re-init runs (idempotent by
  config fingerprint), and the attestation TTL applies at re-init
  (runbook prerequisite 6). No new failure mode is introduced; the
  sidecar converts "host process restart" into the strictly smaller
  "signer process restart."
* **Shutdown**: host-initiated graceful stop sends `SessionAbort`
  for live sessions (zeroize), then terminates. SIGKILL is
  equivalent to a crash and is safe by the same argument.

## 5. Security boundary

* Socket: filesystem-permission-guarded UDS (owner-only directory),
  peer-credential check (`SO_PEERCRED`/`LOCAL_PEERCRED`) pinning
  the host UID. Never a network listener — a TCP mode is explicitly
  out of scope and should be rejected in review if proposed.
* Authentication beyond UID pinning is deliberately deferred: the
  v1 trust model is same-host, same-operator. The TEE phase
  replaces this with an attestation-bound channel; designing that
  channel is part of #4007's scope, not this addendum's.
* Secrets: the state-encryption key provider (env/command) runs in
  the **sidecar's** process environment, not the host's. The config
  file may carry `state_key_command` (its 0600 guidance stands);
  the command executes sidecar-side. Host environment variables
  stop being a secret channel entirely.

## 6. What does not change

JSON schema ownership (Rust), the error-code contract, idempotency
and fail-closed semantics, registries and persistence
(sidecar-local files, same formats), provenance gating, the frozen
section-5 verification rules, and the section-7 deletion trigger.
The dlopen bridge remains the shipping transport until the sidecar
lands; Phases 7.1-7.5 build and validate on dlopen without waiting.

## 7. #4007 (TEE checker stack) scoping

#4007 gates *whether a signer may register* on TEE attestation
evidence and stays decision-gated on the DAO's TEE policy — this
addendum does not undraft it. What the sidecar decision gives it is
a concrete subject: the artifact whose identity gets attested is
the sidecar binary (later, the enclave image), not the composite
keep-client process. #4007's open scoping questions become: which
measurement (binary hash / enclave MRENCLAVE-equivalent), who
verifies (the DAO-whitelist checker), and how the attestation binds
to the UDS channel. Those land in #4007's own design doc; the
interface contract it must respect is sections 3-5 here.

## 8. Open questions (proposed defaults; decide at this addendum's
sign-off)

* (a) **Spawn model**: keep-client child process (default) vs.
  independent systemd unit.
* (b) **Wire framing**: length-prefixed JSON frames (default) vs.
  newline-delimited JSON.
* (c) **Connection model**: small pool, one in-flight request per
  connection (default) vs. request-id multiplexing.
* (d) **Packaging**: sidecar binary ships in the same release
  artifact as keep-client (default) vs. separate artifact with its
  own version line.

## 9. Sequencing

The sidecar is not on the 7.1-7.5 critical path: those phases build
on the dlopen transport, and the frozen API guarantees the swap is
transport-only. The sidecar track runs in parallel and must
converge **before the ECDSA-retirement phases** (decision 1's
timing: take the isolation step before mainnet TVL migrates).
Suggested shape: 7.S1 sidecar process + handshake + conformance
suite; 7.S2 operational hardening (supervision, packaging,
runbook); 7.S3 cutover of the production default with dlopen kept
as the rollback transport for one release.
