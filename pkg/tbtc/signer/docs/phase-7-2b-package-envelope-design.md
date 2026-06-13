# Phase 7.2b: Signed Package Envelopes, Bound Blame, and Vectors

Date: 2026-06-13
Status: Design note (scopes the 7.2b implementation PRs)
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
package envelopes (section 6). 7.2b builds the foundation (the
envelopes) and the feature (the bound blame) together, so blame is
never emitted without the evidence that backs it.

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

- A signer's Round2 records the `attempt_context_hash` + a hash of the
  `SignedSigningPackage` body it signed over (a new per-attempt field,
  persisted with the consumption marker - see section 6).
- At aggregation the coordinator submits, per share, the member's
  retained `SignedSigningPackage`. The engine verifies the coordinator
  signature on each, confirms every member signed a body with the SAME
  bytes, and only then treats a frost share-verification failure as
  attributable: `EngineError::InvalidSignatureShare { culprits }` naming
  the members whose shares failed against the package they themselves
  signed.
- If the submitted envelopes disagree (equivocation) or a member signed
  a different body than the aggregated package, the failure is NOT
  member blame - it is coordinator/input fault, returned as a distinct
  non-attributive error. This is the precise fix for the 7.2a forgeable-
  blame finding.
- Refinement carried from the #4052 review: `culprits == the entire
  subset` is treated as suspect (a coordinator/config error is far more
  likely than every member simultaneously cheating) and is reported as a
  coordinator-side error, not universal member blame.

## 5. FFI structured-culprit payload (Codex P2 on #4052)

`ffi::error_result` currently serializes only `code`, `message`,
`recovery_class`, so a culprit vector would be lost as structured data
and the Go coordinator would have to parse the Display string. 7.2b
extends the FFI error response with an optional structured detail
(e.g. `culprits: []string`) so blame is machine-readable. This is a
general FFI-contract change (other structured errors can use it later)
and must be reflected in the C header and the Go bridge decoder.

## 6. InteractiveAggregate completion marker

Deferred from 7.2a: a persisted per-attempt "aggregated" marker
(`aggregated_interactive_attempt_markers: HashSet<String>` on
`SessionState`, mirrored in `PersistedSessionState`, bounded like the
consumed markers). Re-aggregating a completed attempt returns a clear
"already aggregated" error rather than recomputing. Not security-load-
bearing (aggregate is deterministic over public data), but the spec
calls for marking the session complete, and the same Round2 record that
binds blame (section 4) lives in this state.

## 7. Go vs Rust split

- **Go (scaffold branch)**: the `SigningPackageBody`/`SignedSigningPackage`
  protos + gen (mind the Dockerfile gen-COPY allowlist - the #4040 CI
  gotcha), coordinator-side package signing + distribution, member-side
  verify + retain, and cross-member equivocation comparison (extends
  #4044). The Go bridge decoder for the new structured FFI error.
- **Rust (mirror branch)**: the engine's Round2 binding record, the
  envelope-bound blame in InteractiveAggregate, the FFI error payload
  extension + C header, the completion marker, and the engine-side
  vectors.

## 8. Cross-language vectors (frozen spec item 9)

Pin the new wire structs across languages: `SigningPackageBody`
canonicalization, `SignedSigningPackage` byte-preservation/verbatim-
embedding, and an equivocation-detection vector. Regenerate via the
established discipline and byte-copy to
`pkg/tbtc/signer/testdata/`; treat regeneration as a protocol-change
event.

## 9. Suggested sub-PR sequence

1. **7.2b-1 (mirror)**: Round2 binding record + completion marker
   (persistence plumbing only; no blame yet). Self-contained, sets up
   the state the rest needs.
2. **7.2b-2 (scaffold)**: `SignedSigningPackage` protos + gen +
   coordinator signing/distribution + member verify/retain. Wire +
   `wire_test.go` byte-preservation, no engine change yet.
3. **7.2b-3 (mirror)**: FFI structured-error payload + C header + the
   envelope-bound `InvalidSignatureShare` blame in InteractiveAggregate,
   consuming the 7.2b-1 record.
4. **7.2b-4 (scaffold)**: cross-member equivocation comparison
   (extends #4044) + the Go bridge decoder for the structured error.
5. **7.2b-5**: cross-language vectors, byte-copied both sides.

Each is independently reviewable; blame (7.2b-3) only lands once its
binding (7.2b-1) and the envelope (7.2b-2) exist.

## 10. Open questions

1. **Share carries its envelope, or coordinator collects them?** Does
   each member transmit its `SignedSigningPackage` alongside its share,
   or does the coordinator already hold them from distribution? Proposed:
   the coordinator holds the envelope it distributed, and the engine
   binds blame against the body hash each member's Round2 recorded - so
   the engine does not need every member's envelope at aggregate, only
   the body hash agreement. Revisit against the equivocation-detection
   needs of section 3.
2. **Where does cross-member comparison run** - per-node opportunistic
   (a member compares its envelope against peers' on a gossip topic) or
   only at the f+1 accuser-quorum exclusion step? Ties to the
   proof-carrying-blame roadmap; defer the comparison transport,
   implement the retention now.
3. **FFI structured-error shape**: a typed `details` map vs a
   blame-specific field. Lean: a small typed optional field set, since
   culprits are the only structured detail in flight, kept extensible.

## 11. Acceptance

7.2b is done when: a coordinator equivocating package bodies cannot
produce member blame (test); a genuine bad share against an
agreed-and-signed package produces attributable `InvalidSignatureShare`
with machine-readable culprits over the FFI (test); re-aggregation of a
completed attempt is rejected (test); and the cross-language vectors are
pinned both sides.
