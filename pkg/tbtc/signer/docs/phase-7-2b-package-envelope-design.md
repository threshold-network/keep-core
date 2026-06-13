# Phase 7.2b: Signed Package Envelopes, Bound Blame, and Vectors

Date: 2026-06-13
Status: Design note (scopes the 7.2b implementation PRs); open questions
RESOLVED post-review — see §10 and the open-questions discussion doc. Key
correction: the engine does pure FROST math and NEVER verifies envelopes;
all envelope verification + blame adjudication is Go-side.
Owner: Threshold Labs
Scope: the signed-body signing-package envelope (frozen spec section 6),
the envelope-bound attributable blame deferred from 7.2a, the FFI
structured-culprit payload, the InteractiveAggregate completion marker,
and the cross-language vectors. Builds on merged 7.1 (#4051) + 7.2a
(#4052, mirror).

## 1. Why this phase exists

7.2a ships InteractiveAggregate but **fails closed without naming
culprits** on an invalid share. That was deliberate: the engine cannot
yet bind the aggregate's public inputs (signing package, taproot root)
to what each member actually signed at Round2, so a coordinator
aggregating against a different package/root would make honest shares
fail verification and frame their members (Codex P1 on #4052). The
frozen spec ties attributable blame to share verification (section 5,
item 4) AND ties the binding that makes it trustworthy to signed
package envelopes (section 6) — with the engine doing pure share-math and
the binding/adjudication living in the Go host (see §4 and the
open-questions doc Q1). 7.2b builds the foundation (the envelopes,
Go-side) and the feature (the bound blame) together, so blame is never
emitted without the evidence that backs it.

This is the recurring lesson made concrete: do not ship the security
feature (blame) ahead of its foundation (the binding).

## 2. The signed package envelope

Mirror the proven #4040 signed-body pattern
(`pkg/frost/roast/gen/pb/evidence.proto`): the bytes are signed once,
embedded verbatim, and re-verified against exactly what was received -
no canonical-form dependence.

New wire types (Go, `pkg/frost/roast/gen/pb/`):

```
// The coordinator-distributed signing package for one attempt.
message SigningPackageBody {
  bytes  attempt_context_hash = 1;  // binds package to the attempt
  uint32 coordinator_id       = 2;
  bytes  signing_package       = 3;  // the frost SigningPackage bytes
  bytes  taproot_merkle_root   = 4;  // empty = key-path
}

// On-wire: exact signed body + the coordinator's operator signature.
message SignedSigningPackage {
  bytes body                 = 1;
  bytes coordinator_signature = 2;
}
```

The coordinator signs `SigningPackageBody` with its operator key and
distributes `SignedSigningPackage` to the chosen subset. Each member:
1. verifies the coordinator signature and the `attempt_context_hash`
   against its live attempt;
2. **retains the exact received envelope bytes** alongside its share;
3. produces its Round2 share over the `signing_package` in that body.

## 3. Equivocation detection (extends #4044)

A coordinator that distributes *different* package bodies to different
members is the framing vector 7.2a could not defend against. With
envelopes it is self-incriminating: each member retains the exact
`SignedSigningPackage` it received and signed over. Two members holding
envelopes with the same `attempt_context_hash` but different bodies, each
carrying the coordinator's signature, are a proof of coordinator
equivocation - the same shape as #4044's `EquivocationEvidence`, now
over package envelopes rather than evidence snapshots. The cross-member
comparison is the RFC-21 Go layer's job (scaffold), not the engine's.

## 4. Bound attributable blame (reintroduces what 7.2a removed)

InteractiveAggregate's per-share verification becomes attributable ONLY
when bound to what the member signed:

- The engine may keep an optional engine-local record of the
  `attempt_context_hash` + the hash of the *signing package* it signed
  over at Round2 (the engine sees the FROST signing package, never the
  coordinator-signed `SignedSigningPackage` envelope — that is a Go wire
  type). This is bookkeeping for idempotency / the completion marker
  (section 6), NOT the network-attributable binding; the authoritative
  binding is the member's retained envelope, held in the Go layer.
- At aggregation the **engine does pure FROST math**: it verifies each
  share against the member's verifying share (public, from the DKG key
  package it already holds) and returns the mathematically failing
  members as *candidate* culprits —
  `EngineError::InvalidSignatureShare { culprits }`. The engine does NOT
  see envelopes or operator signatures (it has no operator-key registry;
  open-questions doc Q1). Authoritative blame is adjudicated in the **Go
  host at the f+1 accuser quorum**, which re-checks each accused share
  against *the package envelope that member signed over* (its retained
  received bytes), never a coordinator-submitted or reconstructed
  package. The coordinator's operator signature on the envelope is the
  artifact that convicts an *equivocating coordinator* (two divergent
  signed bodies for one attempt-context), not what attributes member
  fault.
- If a member's retained envelope diverges from the aggregated package
  (equivocation) or a member signed a different body, the quorum re-check
  yields NOT member blame but coordinator/input fault — a distinct
  non-attributive outcome. This is the precise fix for the 7.2a
  forgeable-blame finding: an honest member whose share fails only
  because the coordinator aggregated a package it never signed is
  exonerated by its own retained bytes.
- Refinement carried from the #4052 review: `culprits == the entire
  subset` is treated as suspect (a coordinator/config error is far more
  likely than every member simultaneously cheating) and is reported as a
  coordinator-side error, not universal member blame.

## 5. FFI structured-culprit payload (Codex P2 on #4052)

`ffi::error_result` currently serializes only `code`, `message`,
`recovery_class`, so a culprit vector would be lost as structured data
and the Go coordinator would have to parse the Display string. 7.2b adds
a typed optional field — `culprits: Option<Vec<u16>>` (the Go
member-identifier form — open-questions doc Q3) — to the FFI error
response so the engine's *candidate* culprits are machine-readable. They
are candidates: the Go host adjudicates final blame at the f+1 quorum
(section 4). A typed field, not a generic `details` map, keeps the FFI
contract strong (YAGNI); it must be reflected in the C header and the Go
bridge decoder.

## 6. InteractiveAggregate completion marker

Deferred from 7.2a: a persisted per-attempt "aggregated" marker
(`aggregated_interactive_attempt_markers: HashSet<String>` on
`SessionState`, mirrored in `PersistedSessionState`, bounded like the
consumed markers). Re-aggregating a completed attempt returns a clear
"already aggregated" error rather than recomputing. Not security-load-
bearing (aggregate is deterministic over public data), but the spec
calls for marking the session complete. (The blame binding itself is
Go-side — section 4 — so any engine-local Round2 bookkeeping here is just
that: local bookkeeping, not the network-attributable blame artifact.)

## 7. Go vs Rust split

- **Go (scaffold branch)**: the `SigningPackageBody`/`SignedSigningPackage`
  protos + gen (mind the Dockerfile gen-COPY allowlist - the #4040 CI
  gotcha), coordinator-side package signing + distribution, member-side
  verify + retain, the operator-signature verification, cross-member
  equivocation comparison (extends #4044), and the **authoritative blame
  adjudication** — re-checking the engine's candidate culprits against
  each member's retained envelope bytes at the f+1 accuser quorum. The Go
  bridge decoder for the new structured FFI error.
- **Rust (mirror branch)**: the FFI candidate-`culprits` payload (pure
  FROST math, no envelopes) + C header, the completion marker, any
  engine-local Round2 bookkeeping, and the engine-side vectors. The
  engine never verifies envelopes or operator signatures.

## 8. Cross-language vectors (frozen spec item 9)

Pin the new wire structs across languages: `SigningPackageBody`
canonicalization, `SignedSigningPackage` byte-preservation/verbatim-
embedding, and an equivocation-detection vector. Regenerate via the
established discipline and byte-copy to
`pkg/tbtc/signer/testdata/`; treat regeneration as a protocol-change
event.

## 9. Suggested sub-PR sequence

1. **7.2b-1 (mirror)**: completion marker + (optional) engine-local
   Round2 signing-package-hash bookkeeping (persistence plumbing only; no
   blame, no envelopes). Self-contained, sets up the state the rest needs.
2. **7.2b-2 (scaffold)**: `SignedSigningPackage` protos + gen +
   coordinator signing/distribution + member verify/retain. Wire +
   `wire_test.go` byte-preservation, no engine change yet.
3. **7.2b-3 (mirror)**: FFI structured-error payload (`culprits: []u16`)
   + C header + the pure-FROST candidate-`InvalidSignatureShare` in
   InteractiveAggregate (no envelope handling in the engine).
4. **7.2b-4 (scaffold)**: cross-member equivocation comparison
   (extends #4044) + the **authoritative blame adjudication** (quorum
   re-check of the engine's candidate culprits against retained
   envelopes) + the Go bridge decoder for the structured error.
5. **7.2b-5**: cross-language vectors, byte-copied both sides.

Each is independently reviewable; the engine's candidate culprits
(7.2b-3) become *authoritative* blame only via the Go adjudication in
7.2b-4 (quorum re-check against retained envelopes from 7.2b-2).

## 10. Open questions

1. **~~Share carries its envelope, or coordinator collects them?~~
   RESOLVED (open-questions doc Q1, Gemini+Codex P1):** moot — the
   *engine* never adjudicates envelopes at all. The engine returns
   candidate culprits from pure FROST math; the Go host verifies operator
   signatures and re-checks blame against each member's retained envelope
   at the f+1 quorum. The earlier "engine binds against the body hash"
   proposal is **withdrawn** — it was unsound (the engine has no
   operator-key registry, and a coordinator signature is the wrong
   authentication direction for member blame).
2. **~~Where does cross-member comparison run?~~ RESOLVED
   (open-questions doc Q2, both reviewers concur):** retention now;
   comparison at the f+1 accuser-quorum exclusion step (Option B).
   Opportunistic gossip deferred.
3. **~~FFI structured-error shape?~~ RESOLVED (open-questions doc Q3,
   both reviewers concur):** a typed optional `culprits: Option<Vec<u16>>`
   field on `ErrorResponse` (not a generic `details` map), Go member-id
   u16 form.

## 11. Acceptance

7.2b is done when: a coordinator equivocating package bodies cannot
produce member blame (Go quorum test, re-checking retained envelopes); a
genuine bad share against an agreed-and-signed package yields
machine-readable *candidate* culprits over the FFI (engine test) that the
Go quorum confirms as attributable `InvalidSignatureShare`; re-aggregation
of a completed attempt is rejected (test); and the cross-language vectors
are pinned both sides.
