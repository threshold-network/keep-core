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
1. verifies the coordinator signature, the `attempt_context_hash`
   against its live attempt, AND that `taproot_merkle_root` equals the
   live session/signing root (empty for key-path) — rejecting any
   envelope whose root diverges (coordinator equivocation, §3);
2. **retains the exact received envelope bytes** alongside its share;
3. produces its Round2 share over the `signing_package` in that body,
   under the (now root-verified) session root.

The root check (step 1) is load-bearing (Codex P1): the attempt context
does NOT carry the root, but Round2 signs under the root in the engine
session. Without it a coordinator could hand a member an envelope with
the right package/attempt but a divergent root — the engine would sign
under the real session root, the retained envelope would no longer
describe what the member actually signed, and the later quorum re-check
would use the wrong root and misattribute blame.

## 3. Equivocation detection (extends #4044)

A coordinator that distributes *different* package bodies to different
members is the framing vector 7.2a could not defend against. With
envelopes it is self-incriminating: each member retains the exact
`SignedSigningPackage` it received and signed over. Two members holding
envelopes with the same `attempt_context_hash` but different bodies, each
carrying the coordinator's signature, are a proof of coordinator
equivocation - the same shape as #4044's `EquivocationEvidence`, now
over package envelopes rather than evidence snapshots. Because
`taproot_merkle_root` is a field of the signed body, distributing
different roots is the same equivocation, caught by the same machinery
once members reject a root that does not match their session (§2). The
cross-member comparison is the RFC-21 Go layer's job (scaffold), not the
engine's.

## 4. Bound attributable blame (reintroduces what 7.2a removed)

InteractiveAggregate's per-share verification becomes attributable ONLY
when bound to what the member signed:

- The authoritative binding is the member's **retained envelope**, held
  in the Go layer (it carries the package AND the root — §2). The engine
  itself does NOT need to record what it signed for the blame flow:
  nothing in the corrected design consumes such a record (the quorum
  re-checks Go-retained bytes; the completion marker is attempt-keyed).
  7.2b-1 should add an engine-local Round2 record ONLY if a concrete
  consumer is identified; absent one, the engine-side state is just the
  completion marker (section 6).
- At aggregation the **engine does pure FROST math**: it verifies each
  share against the member's verifying share (public, from the DKG key
  package it already holds) and returns the mathematically failing
  members as *candidate* culprits —
  `EngineError::InvalidSignatureShare { culprits }`. The engine does NOT
  see envelopes or operator signatures (it has no operator-key registry;
  open-questions doc Q1). Authoritative blame is adjudicated in the **Go
  host at the f+1 accuser quorum**, which re-checks each accused share
  against the `signing_package` AND `taproot_merkle_root` inside *the
  envelope that member signed over* (its retained received bytes — both
  fields are what the member signed), never a coordinator-submitted or
  reconstructed package. A candidate culprit becomes authoritative blame
  ONLY for a share **provably submitted by the accused member**
  (member-authenticated submission — see §9/§11, a hard prerequisite): a
  share the coordinator could have fabricated names no one. The
  coordinator's operator signature on the envelope is the artifact that
  convicts an *equivocating coordinator* (two divergent signed bodies for
  one attempt-context), not what attributes member fault.
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
  coordinator-side error, not universal member blame. **For this rule to
  be reachable, 7.2b-3 MUST request all-cheater detection** (Codex P2):
  the plain `frost::aggregate` / `aggregate_with_tweak` hard-code
  `CheaterDetection::FirstCheater` (verified in frost-core 3.0.0), so
  they report only the first invalid share and the full-subset condition
  could never trigger (multi-share failures would also be under-reported,
  forcing one-cheater-per-attempt exclusion grinds). No reimplementation
  is needed: frost-core 3.0.0 already collects every culprit under
  `CheaterDetection::AllCheaters` (its `detect_cheater` extends an
  `all_culprits` vec over all shares). Taproot wrinkle to pin in 7.2b-3:
  `frost_secp256k1_tr::aggregate_with_tweak` does NOT expose the mode (it
  delegates to first-cheater), so the engine applies the tweak itself
  (`public_key_package.tweak(merkle_root)`, as the wrapper does
  internally) and calls `frost_core::aggregate_custom(…,
  CheaterDetection::AllCheaters)` on the tweaked package — confirm
  `.tweak()` is callable from the engine, else reproduce the tweak or
  request an upstream `aggregate_with_tweak_custom`. Soundness does not
  hinge on this engine heuristic: the authoritative
  all-honest-vs-coordinator distinction remains the Go quorum re-check
  against retained envelopes (above).

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
calls for marking the session complete. (The blame binding is Go-side —
section 4 — so the only engine-side state 7.2b adds is this completion
marker.)

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
  FROST math via `CheaterDetection::AllCheaters`, no envelopes) + C
  header, the completion marker, and the engine-side vectors. The engine
  never verifies envelopes or operator signatures.

## 8. Cross-language vectors (frozen spec item 9)

Pin the new wire structs across languages: `SigningPackageBody`
canonicalization, `SignedSigningPackage` byte-preservation/verbatim-
embedding, and an equivocation-detection vector. Regenerate via the
established discipline and byte-copy to
`pkg/tbtc/signer/testdata/`; treat regeneration as a protocol-change
event.

## 9. Suggested sub-PR sequence

1. **7.2b-1 (mirror)**: the InteractiveAggregate completion marker
   (persistence plumbing only; no blame, no envelopes). Self-contained.
   (No engine-local Round2 package-hash record unless §4 identifies a
   consumer — the corrected design has none.)
2. **7.2b-2 (scaffold)**: `SignedSigningPackage` protos + gen +
   coordinator signing/distribution + member verify (incl. the
   `taproot_merkle_root` check, §2) / retain + **member-authenticated
   Round2 share submission** (reuse the #4040 sign-what-you-transmit
   envelope so a share is provably from its claimed member — a hard
   prerequisite for blame, Codex P1). Wire + `wire_test.go`
   byte-preservation, no engine change yet.
3. **7.2b-3 (mirror)**: FFI structured-error payload (`culprits: []u16`)
   + C header + the pure-FROST candidate-`InvalidSignatureShare` in
   InteractiveAggregate (no envelope handling in the engine), aggregating
   with **`CheaterDetection::AllCheaters`** (tweak-aware — §4) so the full
   culprit set is reported, not just the first.
4. **7.2b-4 (scaffold)**: cross-member equivocation comparison
   (extends #4044) + the **authoritative blame adjudication** (quorum
   re-check of the engine's candidate culprits against retained
   envelopes) + the Go bridge decoder for the structured error.
   **Gated on 7.2b-2's member-authenticated share submission** — blame
   must not be enabled until a share is provably attributable to its
   member.
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

7.2b is done when: a member rejects an envelope whose `taproot_merkle_root`
diverges from its session root (test, Codex P1); a coordinator
equivocating package bodies (incl. the root) cannot produce member blame
(Go quorum test, re-checking retained envelopes); authoritative blame is
gated on member-authenticated share submission — a share not provably
from member A cannot make A a culprit (test, Codex P1); a genuine bad
share against an agreed-and-signed package yields machine-readable
*candidate* culprits over the FFI (engine test) that the Go quorum
confirms as attributable `InvalidSignatureShare`; re-aggregation of a
completed attempt is rejected (test); and the cross-language vectors are
pinned both sides.
