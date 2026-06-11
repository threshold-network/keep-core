# Coordinator-shuffle seed derivation (RFC-21 Annex A mirror)

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
- `MessageDigest`: exactly 32 bytes.
- `AttemptNumber`: the RFC-21 **0-based** attempt number. The FFI
  `AttemptContext.attempt_number` carries the **1-based** wire encoding
  (`wire = AttemptNumber + 1`, zero rejected); the engine subtracts one
  before composing the shuffle source (`validate_attempt_context`).
- `GoMathRandShuffle`: the bit-exact port of Go's legacy `math/rand`
  shuffle in `src/go_math_rand.rs`, pinned by keep-core PRs #4026 and
  #4027.

Implemented by `roast_attempt_shuffle_seed` in `src/engine.rs`.

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
