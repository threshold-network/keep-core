# Phase 7.2b Open Questions — Options, Tradeoffs, Recommendations

Date: 2026-06-13
Status: SIGNED OFF 2026-06-13 — recorded as gates-doc Decision Log entry
9. Reviewed (Gemini + Codex concurred; recommendations resolved — see the
Decision Log at the end); Q1 was corrected back to the frozen spec on a
converging reviewer P1.
Companion to: `phase-7-2b-package-envelope-design.md`

This doc works the three open questions that gate the 7.2b
implementation into explicit options with tradeoffs and a recommendation
each. They are now **resolved** (Gemini + Codex reviewed; see the
Decision Log at the end) — each question's "RESOLVED" block records the
decision and its reasoning. This is a decision record, not an open
solicitation.

A framing distinction that runs through all three, stated once:

> The coordinator is a node and may itself be the adversary. So
> "detect a bad input" splits into two jobs with different trust
> assumptions: (a) **protect honest members from false blame** when the
> coordinator's inputs don't match what members signed; and (b) **prove
> a malicious coordinator equivocated** so it can be excluded. Neither
> can be entrusted to the coordinator's engine: as Q1 resolves, the
> engine emits only mathematical *candidate* culprits, and BOTH jobs are
> enforced at the members / the f+1 accuser quorum — job (a) by
> re-checking an accused share against the accused member's own retained
> envelope, job (b) by comparing members' retained envelopes. Several
> options below are really about which of these two jobs they serve.

---

## Q1 — How is a failing share bound to what the member signed?

**Problem.** InteractiveAggregate (7.2a) fails closed without blame
because a coordinator aggregating against a different package/root than
members signed makes honest shares fail and would frame them. To name a
culprit soundly, the engine must distinguish "member produced a bad
share for the agreed package" from "coordinator fed a different
package." A FROST share cryptographically binds to the package it was
produced over, so a failure alone cannot tell these apart — the engine
needs evidence of what each member was told to sign.

### RESOLVED (post-review) — the engine never touches envelopes; blame adjudication is Go-side

Both reviewers returned a converging **P1** that my pre-review lean
(Option A: *the engine* verifies member envelopes at aggregate) was
wrong, on two independent grounds:

- **Gemini (architecture):** the Rust engine holds only FROST/Schnorr
  threshold key material — it has no operator-key registry, so it
  *cannot* verify a coordinator's operator signature on an envelope.
  Byte-comparing bodies without verifying the signature lets a malicious
  *member* forge a body to dodge blame. Pushing operator keys into the
  engine to fix that would blow the signing-secret boundary the sidecar
  exists to hold (gates-doc decision 2). Verification belongs in the Go
  host, which owns the registry, the network identities, and the exact
  bytes it distributed.
- **Codex (authentication direction):** even if the engine *could*
  verify it, `SignedSigningPackage` is signed by the **coordinator, not
  the member**. A malicious coordinator distributes P′ to member A,
  collects A's share over P′, then aggregates against P with its own
  validly-signed P-envelope: A's share fails against P, the envelope body
  equals P, and the equality check frames honest A. Sound *member* blame
  needs a *member*-authenticated binding of share↔body, never a
  coordinator signature.

The two compose into the design the **frozen spec already specifies**
(§5 item 4 and §6) — my 7.2b design note had drifted from it, so the
reviewers' P1 is really "return to the freeze":

1. **Engine = pure FROST math.** `InteractiveAggregate` verifies each
   share against the member's verifying share (public, from the DKG key
   package the engine already holds) and returns the mathematically
   failing members as *candidate* `culprits`. No envelopes, no operator
   signatures, no network identity in the engine. This makes the 7.2b-3
   engine change *smaller* than the note implied.
2. **Go host = envelope verification + retention.** Members verify the
   coordinator's operator signature and retain the exact received
   `SignedSigningPackage` bytes (§6, #4040 pattern). The coordinator
   signature is the right artifact *here* — for proving *coordinator*
   equivocation (job (b)), where coordinator-authentication is exactly
   what you want — not for attributing member fault.
3. **Authoritative blame = Go, at the f+1 accuser quorum, against the
   member's retained bytes.** The engine's candidate culprits are only a
   trigger. Final exclusion re-checks each accused share against *the
   package envelope that member signed over* (its retained received
   bytes), never a coordinator-submitted or reconstructed package (§6,
   verbatim). Under Codex's attack, A's share verifies against its
   retained P′-envelope, so A is exonerated and the coordinator's two
   divergent signed bodies for one attempt-context convict *it*.

Soundness comes from **Round2 check (f)** — already shipped in 7.1: a
member signs only over a package carrying its own true commitment — plus
the quorum re-check above. A single malicious coordinator can neither
manufacture an f+1 quorum nor make an honest member's retained-envelope
re-check fail.

### Corrected options (the real axis is *where adjudication lives*, not *what crosses the FFI*)

| Option | Engine role | Blame adjudication | Sound vs malicious coordinator? | Verdict |
|---|---|---|---|---|
| **Engine verifies envelopes** (my pre-review A *and* C) | verify operator sigs / body-hashes at aggregate | in-engine | **No** — engine has no registry (member-forgeable) and a coordinator-sig is the wrong auth direction (frames honest member) | **Rejected** (both reviewers, P1) |
| **Engine pure-math + Go quorum adjudication** (frozen §5.4/§6) | verify share vs verifying share → candidate culprits | Go host, at f+1 quorum, vs member's retained bytes | **Yes** — Round2 (f) + quorum re-check; engine stays crypto-only | **Adopted** |

### Hard prerequisite (Codex P1): member-authenticated share submission

For the quorum to attribute "this failing share is A's," A's share
submission to the coordinator MUST be member(operator)-authenticated.
This is not a nice-to-have to confirm — it is **load-bearing for blame
soundness**: without it a malicious coordinator submits random bytes
labeled "A's share," the engine returns A as a *candidate* culprit (the
bytes fail share-verification), and the quorum re-checks bytes A never
sent, framing A. Therefore authoritative blame (7.2b-4) MUST NOT be
enabled until member→coordinator share submissions are authenticated
(reuse the existing #4040 sign-what-you-transmit envelope). **The signed
body must cover the attempt context AND the signing-package/envelope hash,
not just the share bytes** (Codex follow-up P1): otherwise a coordinator
replays an old A-signed share into a different attempt/package and the
quorum re-checks bytes A never submitted for that context — framing A. It
is a Go-layer property (not an engine one) and is part of the
implementation sequence (7.2b-2) and acceptance criteria (neither an
unattributable share nor a replayed A-signed share can make A a culprit).

A second hard prerequisite rides with it (Codex P1, companion design
note §2): members MUST verify `taproot_merkle_root` against their live
session root before signing. The attempt context does not carry the
root, so the retained envelope only faithfully describes "what the member
signed" — the thing the quorum re-checks against — if the member rejected
any envelope whose root diverged.

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

### RESOLVED (post-review) — retention now; comparison via Option B (both reviewers concur)

7.2b lands the *retention* (each member persists the exact
`SignedSigningPackage` it received) regardless. The comparison transport
is **Option B**: feed retained envelopes into the existing f+1
accuser-quorum exclusion path (#4029) — where an exclusion is actually
decided, and which RFC-21 already chose over synchronous-gossip
assumptions. This is also exactly the adjudication site Q1's resolution
relies on.

Gemini's confirming argument closes the "what if the coordinator
equivocates but stays silent" gap: a malicious coordinator that
equivocates package bodies but emits no blame merely causes the attempt
to time out — a *liveness* failure indistinguishable from the
coordinator dropping messages, which the existing RFC-21 rotation
machinery already handles by moving to the next attempt. Proof is needed
only when the coordinator emits blame against an honest member, and there
the accused member defends itself by revealing the equivocation at the
exclusion step. So early/opportunistic gossip (Option A) is an
unnecessary optimization on the critical path, not a 7.2b requirement;
Option C (coordinator-side) cannot catch a malicious coordinator at all
and is excluded as a detection mechanism.

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
| **A. Typed optional field** — add `culprits: Option<Vec<u16>>` (a small typed `blame` sub-struct) to `ErrorResponse` | strong | per-kind: a new structured error adds a new field | small, localized |
| **B. Generic `details: map<string, JSON>`** | weak — callers parse untyped JSON | any structured error reuses it | medium; defines a general contract |
| **C. Encode culprits in the `message` string** with a parseable convention | none (stringly-typed) | no | tiny code, but it's the anti-pattern #4052 P2 named |

### RESOLVED (post-review) — Option A (both reviewers concur)

A typed optional field — `culprits: Option<Vec<u16>>` (a small typed
`blame` sub-object) on `ErrorResponse` — is safer and clearer than an
untyped JSON map, which would force the Go bridge into dynamic type
assertions and weaken the FFI contract. Culprits are the only structured
detail in flight (YAGNI: a generic `details` map can come later if a
*second* structured error ever needs one). The change touches the Rust
`ErrorResponse`, the C header, and the Go bridge decoder together. Use
the Go member-identifier **u16** form, consistent with the rest of the
wire surface — and note this pins the companion design note, which still
said `[]string` (Codex P2); that note is corrected to `[]u16`.

---

## Decision Log (post-review)

Gemini and Codex both reviewed this doc and the companion design note.
All three questions are resolved with reviewer concurrence; the only
substantive change from my pre-review lean is Q1, where both reviewers
returned a P1 that corrected it back to the frozen spec.

| Q | Resolution | Reviewer status |
|---|---|---|
| Q1 binding | Engine does **pure FROST math** → candidate `culprits`; **all** envelope verification, retention, and authoritative blame re-check live in the **Go host at the f+1 accuser quorum**, against the member's retained received bytes (frozen §5.4/§6). Engine never sees envelopes or operator keys. | Gemini P1 + Codex P1 — adopted |
| Q2 equivocation | **Retention now; compare at the f+1 quorum step (B).** Gossip (A) deferred; coordinator-side (C) can't catch a malicious coordinator. | Both concur |
| Q3 FFI error | Typed optional `culprits: Option<Vec<u16>>` on `ErrorResponse` (A); generic details map deferred (YAGNI); design note pinned to `u16`. | Both concur |

Re-review (multiple passes) folded in Codex **P1**s and **P2**s plus a
self-review item on the implementation shape — none changes the Q1/Q2/Q3
decisions above:
- **Taproot root binding** (P1) — members MUST verify `taproot_merkle_root`
  against their live session root before signing (the root is not in the
  attempt context); otherwise the retained envelope misdescribes what was
  signed and the quorum re-check misattributes blame. (design note
  §2/§3/§11)
- **Member-authenticated share submission** (P1) is a hard prerequisite,
  not a confirmation: authoritative blame must not be enabled until a
  share is provably attributable to its member. (this doc, Q1
  prerequisite; design note §9/§11)
- **All-cheater detection** (P2) — 7.2b-3 must aggregate with
  `CheaterDetection::AllCheaters`, not the default first-cheater path, or
  the `culprits == entire subset` rule is unreachable and multi-share
  failures are under-reported. Verified remedy: frost-core 3.0.0 already
  collects all culprits in that mode (no reimplementation); the taproot
  `aggregate_with_tweak` wrapper does not expose the mode, so apply the
  tweak + `aggregate_custom(AllCheaters)`. (design note §4/§9)
- **Elected-coordinator check** (P2) — members must verify `coordinator_id`
  is the *elected* coordinator and the signature is under that key; the
  attempt hash is public, so any operator could otherwise inject packages
  and pollute the equivocation evidence. (design note §2/§3/§9/§11)
- **Retain-on-reject** (P2) — a member that refuses a root-divergent
  envelope must retain it first, or §3 loses the bytes proving the
  coordinator equivocated. (design note §2/§3/§11)
- **Tweak-aware, engine-delegated quorum re-check** (self-review) — the
  quorum's per-share crypto re-verify is FROST math; it must be
  tweak-consistent and is delegated to a stateless engine verify-share (Go
  owns only the quorum policy), not reimplemented in Go. (design note
  §4/§7/§9/§11)
- **Context-bound share authentication** (P1, follow-up) — the
  member-authenticated share submission must bind the share to the attempt
  context + package/envelope hash, not just the share bytes, or a
  coordinator replays an old A-signed share into a different
  attempt/package and frames A. (design note §4/§9/§11; this doc, Q1
  prerequisite)
- **Group key + selector in verify-share inputs** (P2, two passes) — the
  verify-share FFI needs the **group verifying key** (tweaked for taproot),
  not just the per-member verifying share, to compute the challenge/binding
  factors, AND a `session_id`/wallet **selector** to resolve the *right*
  key in a multi-session engine. That key must come from
  **durably-retained wallet DKG material** outliving the signing-session
  TTL sweep, since the quorum re-check can run after the session is gone
  (or, equivalently, the Go quorum passes the public key package explicitly
  — it is public, sound under the f+1-independent-accuser model). (design
  note §4/§7/§9/§11)

These refine 7.2b's implementation shape without changing the frozen
Phase 7 spec — Q1's resolution in fact *realigns* the design docs to it.
Companion design-note corrections applied alongside this doc: §4 (engine
emits candidate culprits, Go adjudicates — not "engine verifies the
coordinator signature"), §5 (`[]u16`), §6 (Round2 record is local
bookkeeping, not the blame-binding artifact), §7 (blame adjudication
moves to the Go column), §10 (superseded body-hash proposal removed).

**Pending owner (MacLane) sign-off** to record as a gates-doc Decision
Log entry; on sign-off, 7.2b-1 (completion marker + any engine-local
Round2 bookkeeping; **no** envelope plumbing in the engine) can start.
