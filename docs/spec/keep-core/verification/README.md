# Keep Core - security invariant registry (KC-)

This is the keep-core entry of the invariant registry. It registers
the properties that have or need checkers: the WalletRegistry DKG result
lifecycle, slashing and seize, the RandomBeacon relay entry and timeouts,
threshold ECDSA signing, the wallet lifecycle, and the Halmos-checked
properties of `solidity/ecdsa` and `solidity/random-beacon`.

## Publication status

The Go and TypeScript checkers named in this set are in this repository.
The Halmos harnesses (`solidity/ecdsa/test/halmos`, `solidity/random-beacon/test/halmos`),
their `heavy.json` and `halmos.toml` settings, the `.github/scripts/halmos-proofs.py` runner
and the `proofs-pr`, `heavy-proofs` and `security-tools` workflows are not in this
repository yet. Every checker that names one of them carries `harness not yet
published`, and no coverage is claimed for it. The harnesses are published with the
proof work; no entry changes status before that.

## Field layout

Each entry in [Invariants](02-invariants.md) carries the Engineering
Verification Standard section 6 fields `Statement`, `Enforcing code` (scope),
`Checkers` (kind, location, status), `Critical`, `Exposure` and `Fails on`,
plus the local fields `Why` and `Status`. The section 6 fields `formal`,
`sources`, `tier` and `bounds` are not recorded per entry: the tier is stated
once for the set and the bounds are in one closing section. `Critical` is yes
when the invariant guards mint, burn or key use, or when its exposure is USD
1M or more; the `Exposure` line gives the reason and the bound. All `Fails on`
fields are empty, so under SPEC-3 no check counts as coverage yet; the open
work is listed at the top of the invariants chapter and tracked in #586.

## Section map

| Section | Stable IDs | Review purpose |
|---|---|---|
| [Invariants](02-invariants.md) | KC- | DKG result validity, slashing and seize, relay entry and timeouts, threshold ECDSA signing, wallet lifecycle |

The M1 UTXO Reservations registry keeps the INV/SM/PAR/US/EC/FM/DR/TM/XR/AR
ID space in `docs/spec/reservations/verification`. This set adds the `KC-`
space. The two ID spaces never collide; an ID is owned by exactly one entry
in one set.

## Checker

This set is checked by the same checker as the reservations set. From the
repository root:

```sh
python3 docs/spec/reservations/verification/tools/traceability.py docs/spec/keep-core/verification --check
```

`--check` exits 1 on a structural problem: an ID with several owners, an owner
without a status, an ID that no chapter owns, a set with no owned IDs, a
`KC-` entry without a `Critical` field of yes or no, or a critical entry with
no checker (an empty `Checkers` field, or one that starts with none, n/a,
missing or TBD). This set has no derived `11-traceability.md` matrix, so the
matrix comparison is skipped when the file is absent. Go checks named in an
entry's `Checkers` or `Fails on` field are resolved against the git checkout
that holds this repository and listed under `check_references`; unlike the
reservations set, this set has no pinned citations and no smoke check, so a
Go check that is missing on this branch shows only in the `check_references`
output and does not change the exit code. Solidity and TypeScript checkers are
not resolved.

The checker runs on pull requests to `main`, `dev`, `release/**` and
`security-release/**` in `.github/workflows/reservations-spec.yml` (job
`spec-checks`, shown as "Verification set checks (keep-core citations only)",
step "Keep-core registry check (KC- set)").

## Maintenance

- Update the owning entry when a guard, a checker or a contract function
  named in it changes. Line numbers in a `Checkers` field are the numbers in
  this repository at `f03cc61dc` (the keep-core pin the reservations set cites
  as the published code); re-check them with `git grep -n` before citing a
  different revision.
- Before each commit, run the check above from the repository root, and
  `python3 -m unittest test_smoke test_traceability` from
  `docs/spec/reservations/verification/tools` with `markdown-it-py` installed
  (see `tools/requirements.txt`). Both must exit 0.
- A check counts as coverage only if it carries its `KC-` ID in its name or in
  a structured annotation (SPEC-2), and only after it fails on a planted
  mutant or a replay of a past bug, recorded in `Fails on` (SPEC-3). No
  existing check carries a `KC-` ID yet.
