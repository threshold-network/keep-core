# Phase 7.2b Open Questions — Options, Tradeoffs, Recommendations

Date: 2026-06-13
Status: Discussion — soliciting reviewer (Gemini, Claude) input before
the 7.2b-1 implementation PR
Companion to: `phase-7-2b-package-envelope-design.md`

This doc works the three open questions that gate the 7.2b
implementation into explicit options with tradeoffs and a
recommendation each. Reviewers: please confirm or dissent on each
recommendation; the "Reviewer ask" line states the specific call we
want a second opinion on.

A framing distinction that runs through all three, stated once:

> The coordinator is a node and may itself be the adversary. So
> "detect a bad input" splits into two jobs with different trust
> assumptions: (a) **protect honest members from false blame** when the
> coordinator's inputs don't match what members signed — this can run
> in the (possibly malicious) coordinator's own engine because its only
> power is to *refuse to emit blame*, which a malicious coordinator
> gains nothing by skipping; and (b) **prove a malicious coordinator
> equivocated** so it can be excluded — this CANNOT rely on the
> coordinator and must run at the members. Several options below are
> really about which of these two jobs they serve.

---

## Q1 — How does the engine bind a failing share to what the member signed?

**Problem.** InteractiveAggregate (7.2a) fails closed without blame
because a coordinator aggregating against a different package/root than
members signed makes honest shares fail and would frame them. To name a
culprit soundly, the engine must distinguish "member produced a bad
share for the agreed package" from "coordinator fed a different
package." A FROST share cryptographically binds to the package it was
produced over, so a failure alone cannot tell these apart — the engine
needs evidence of what each member was told to sign.

### Correction to the design note's lean

The design note (open question 1) floated "the coordinator binds against
the body hash each member's Round2 recorded, so the engine does not need
every member's envelope." **On reflection that is wrong** and I'm
flagging it rather than carrying it forward: the coordinator's engine
holds only its *own* node's Round2 record (engine state is per-node), so
it has nothing to check other members' shares against; and even with the
hashes, distinguishing member-fault from coordinator-equivocation
requires the coordinator's *signature* over the divergent body — the
equivocation proof — which only a member's retained envelope carries.

### Options

| Option | Mechanism | Sound attribution? | Detects coordinator equivocation? | Cost |
|---|---|---|---|---|
| **A. Members transmit their `SignedSigningPackage`** alongside their share; the engine verifies each at aggregate | engine has every member's signed "what I was told to sign" | Yes | Yes (bodies diverge) | +1 envelope per share on the wire; N signature verifies |
| **B. Coordinator binds only against its own distributed package** (no member envelopes) | engine sees only the package it aggregates | No — same as 7.2a; can't separate member-fault from coordinator-fault | No | none, but doesn't solve the problem |
| **C. Members transmit a signed body-hash** (not full package bytes) | engine compares member-attested hashes to the aggregated package | Yes for agreement; weaker proof artifact | Partial — proves divergence but the excludable artifact is a bare hash+sig, not the package | smaller wire; still N verifies |

### Recommendation — **Option A**

Mirror the proven #4040 shape exactly: members transmit their
`SignedSigningPackage` envelopes, the engine embeds/verifies them
verbatim, and a share-verification failure becomes attributable blame
only when that member's envelope body equals the aggregated package
(otherwise it's coordinator/input fault, not member blame). This is the
direct, sound fix to the #4052 P1 and reuses a pattern already in
production. The wire cost (one envelope per share) is bounded and the
Annex-B latency budget has ample headroom.

**Reviewer ask:** is Option A's per-share envelope worth it over Option
C's lighter body-hash, given C's exclusion artifact is a bare hash and
A's is the full coordinator-signed package? We lean A for evidence
quality; push back if C's wire savings matter at n=100.

---

## Q2 — Where does cross-member equivocation comparison run?

**Problem.** Per the framing above, *proving* a malicious coordinator
equivocated (and excluding it) cannot run in the coordinator's engine.
Members each hold a coordinator-signed envelope; a malicious coordinator
that signed two different bodies is caught only when members pool and
compare their envelopes. The question is the transport for that pooling.

### Options

| Option | Mechanism | Detection latency | Complexity | Handles malicious coordinator? |
|---|---|---|---|---|
| **A. Opportunistic gossip** — members broadcast received envelopes on a topic and compare continuously | early, before the attempt times out | high — new topic, async-consistency caveats (RFC-21 warned against assuming synchronous gossip) | yes |
| **B. At the f+1 accuser-quorum step** — retain now; compare only when an exclusion is being decided | late (at exclusion time) | low — reuses the existing #4029 quorum machinery | yes |
| **C. Coordinator-side at aggregate only** | n/a for malicious coordinator | n/a | low | **no** — a malicious coordinator won't self-incriminate |

### Recommendation — **implement retention now; comparison via Option B**

7.2b lands the *retention* (each member persists the exact
`SignedSigningPackage` it received) regardless. For the comparison
transport, do Option B: feed retained envelopes into the existing f+1
accuser-quorum exclusion path (#4029), which is where an exclusion
decision is actually made and which RFC-21 already chose over
synchronous-gossip assumptions. Option C (coordinator-side at aggregate)
still happens for free under Q1=A but only as the *refuse-to-blame*
protection (job (a)), not as malicious-coordinator detection (job (b)) —
the two are complementary, not alternatives. Opportunistic gossip
(Option A) is a later latency optimization, not a 7.2b requirement.

**Reviewer ask:** confirm that deferring the comparison transport to the
quorum step (retention-only in 7.2b) is acceptable, i.e. that we don't
need early/opportunistic equivocation detection before the
ECDSA-retirement phases. If early detection is a gate, Option A moves
into scope.

---

## Q3 — Shape of the structured FFI error carrying culprits

**Problem.** `ffi::error_result` serializes only `code`, `message`,
`recovery_class`, so the reintroduced `InvalidSignatureShare { culprits }`
would lose the culprit vector as structured data (#4052 P2). The Go
coordinator needs the culprits machine-readable to exclude the right
members.

### Options

| Option | Shape | Typing | Extensible | Churn |
|---|---|---|---|---|
| **A. Typed optional field** — add `culprits: Option<Vec<String>>` (or a small typed `blame` sub-struct) to `ErrorResponse` | strong | per-kind: a new structured error adds a new field | small, localized |
| **B. Generic `details: map<string, JSON>`** | weak — callers parse untyped JSON | any structured error reuses it | medium; defines a general contract |
| **C. Encode culprits in the `message` string** with a parseable convention | none (stringly-typed) | no | tiny code, but it's the anti-pattern #4052 P2 named |

### Recommendation — **Option A**

Culprits are the only structured detail in flight; a typed optional
field (a small `blame { culprits: []u16 }` sub-object on the error
response) is safer and clearer than an untyped map, and a generic
`details` map can be introduced later if a *second* structured error
ever needs one (YAGNI). The change touches the Rust `ErrorResponse`, the
C header, and the Go bridge decoder together. Use the Go member
identifier (u16) form for culprits, consistent with the rest of the wire
surface, not the frost-identifier hex.

**Reviewer ask:** typed-field (A) vs generic-details-map (B) — we lean A
on YAGNI/typing grounds; dissent if you expect several structured error
kinds soon enough that B's one-time contract pays off.

---

## Summary of recommendations

| Q | Recommendation |
|---|---|
| Q1 binding | **A** — members transmit `SignedSigningPackage`; engine verifies + binds (corrects the note's body-hash lean) |
| Q2 equivocation | **retention now; compare at the f+1 quorum step (B)**; opportunistic gossip deferred |
| Q3 FFI error | **A** — typed optional `culprits` (u16) field; generic details map deferred |

None of these changes the frozen Phase 7 spec; they refine 7.2b's
implementation shape. Once settled (recorded in the gates-doc Decision
Log), 7.2b-1 (the Round2 binding record + completion marker) can start.
