# Coordinator-shuffle seed derivation (RFC-21 Annex A mirror)

## Status: HKDF+CSPRNG migration reverted (Decision 2)

The HKDF-SHA256 + CSPRNG coordinator-shuffle migration proposed in the
PR #4005 review (originally decided as the new normative derivation)
was attempted and reverted during implementation. The reverted attempt
is documented here for the audit trail; the rest of this file describes
the **current** derivation, which is unchanged from the prior
unification-PR state.

**What happened.** The migration's HKDF pull-in surfaced a
`digest` crate version conflict between `sha2` (pinned at `0.10` in
`Cargo.toml`) and the `hkdf` crate that the migration would have
required, with the resolver landing on a configuration that broke
downstream `frost-secp256k1-tr` compilation. Decision 2 was reverted
rather than repinned, because the cost of reshuffling the ciphersuite
dependency chain for a derivation change that was not security-binding
on `frost-core` 3.x was disproportionate to the benefit.

**What the current derivation actually is.** The current derivation
remains the Go-`math/rand` port: `GoMathRandShuffle` in
`src/go_math_rand.rs`, the 607-element `RNG_COOKED` table, and the
four-line `roast_attempt_shuffle_seed` derivation in `src/engine/roast.rs`
described in the "Derivation" and "Conformance vectors" sections
below. No behavior changed.

**What was kept from the partially-applied migration.** A single
`COORDINATOR_SHUFFLE_VERSION: u8 = 1` byte (the prior unified state)
is fed into the attempt-context hash as a version-pin. The version
byte is intentionally inert at version `1` — it does not alter the
computed seed or the selected coordinator — but it establishes the
versioning slot so a future real migration can bump to `2` (or
higher) without confusing the conformance corpus. Bumping the version
byte deliberately drops compatibility with the existing test vectors
and forces a documented re-pinning, which is the whole point of the
pin.

**Open follow-up.** The HKDF-SHA256 + CSPRNG migration itself is
tracked as a follow-up to PR #4005. Until that follow-up lands and
bumps `COORDINATOR_SHUFFLE_VERSION`, the normative derivation is the
Go-port described below; the "Status" note at the top of this file
is the load-bearing caveat for any reader cross-referencing the
PR #4005 review's original Decision 2.

This status note is appended to the top of the file (rather than
replacing the body) so that pre-revert readers and the
`coordinator_seed_derivation_matches_cross_language_vectors`
conformance pinning remain diff-stable.

The normative definition of the ROAST coordinator-shuffle seed lives in
keep-core's RFC-21, *Annex A (normative): coordinator-shuffle seed
derivation*
(`docs/rfc/rfc-21-roast-coordinator-retry-and-transition-evidence.adoc`
on the `feat/frost-schnorr-migration-scaffold` branch). This file
mirrors the derivation for signer-side readers; if the two ever
disagree, the RFC annex wins.

## Derivation

```text
AttemptSeed32   = SHA256(KeyGroupBytes || SessionID || MessageDigest)
ShuffleSeed_i64 = int64_from_be_bytes(AttemptSeed32[0..8])
SourceSeed_i64  = ShuffleSeed_i64 + int64(AttemptNumber)      # two's-complement wrap
Coordinator     = GoMathRandShuffle(sort_ascending(IncludedSet), SourceSeed_i64)[0]
```

- `KeyGroupBytes`: UTF-8 bytes of the canonical key-group handle. For
  this engine that is the lowercase hex encoding of the serialized
  group verifying key (the `key_group` string in `DkgResult`), treated
  as an opaque string — never decoded to point bytes before hashing.
- `SessionID`: raw UTF-8 bytes.
- `MessageDigest`: the **raw signing message itself**, big-endian
  left-padded with zeros to exactly 32 bytes (leading zero bytes are
  insignificant; more than 32 significant bytes is rejected). This
  mirrors keep-core's `messageDigestFromBigInt`: in BIP-340 production
  the message the engine receives *is* the 32-byte sighash. It is
  **not** the engine's internal transcript digest
  (`SHA256(message_bytes)`), which continues to feed the
  `round_id`/`attempt_id` derivations only. Implemented by
  `rfc21_message_digest` in `src/engine/roast.rs`; feeding the transcript
  digest here instead was the cross-language coordinator divergence
  caught in review of the unification PR.
- `AttemptNumber`: the RFC-21 **0-based** attempt number. The FFI
  `AttemptContext.attempt_number` carries the **1-based** wire encoding
  (`wire = AttemptNumber + 1`, zero rejected); the engine subtracts one
  before composing the shuffle source (`validate_attempt_context`).
- `GoMathRandShuffle`: the bit-exact port of Go's legacy `math/rand`
  shuffle in `src/go_math_rand.rs`, pinned by keep-core PRs #4026 and
  #4027.

Implemented by `roast_attempt_shuffle_seed` in `src/engine/roast.rs`; the
end-to-end acceptance of a Go-derived context through strict
`StartSignRound` is pinned by
`start_sign_round_accepts_go_derived_attempt_context_in_strict_mode`.

## Conformance vectors

`testdata/coordinator_seed_vectors.json` is a byte-identical copy of
the canonical vector file generated from the Go implementation
(`pkg/frost/roast/testdata/coordinator_seed_vectors.json`, regenerated
there with `ROAST_SEED_VECTORS_REGEN=1 go test ./pkg/frost/roast -run
TestRegenerateCoordinatorSeedVectors`). The unit test
`coordinator_seed_derivation_matches_cross_language_vectors` pins the
seed, the selected coordinator, the 0-/1-based wire mapping, and full
`validate_attempt_context` acceptance for every vector. When the Go
side regenerates the file, re-copy it here verbatim.

## History

Before this unification the engine derived the seed from the first 8
bytes of the raw message digest with the 1-based wire attempt number —
the legacy `signingAttemptSeed` convention of the pre-ROAST keep-core
signing loop. The divergence from the RFC-21 layer was flagged in
keep-core PR #4026 and resolved by adopting the Go derivation as
normative.
