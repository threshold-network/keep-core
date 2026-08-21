# Inventory fragments (evidence tier)

These seven documents are the source-verified working notes behind
`../milestone-inventory.md`. That ledger is the synthesis and the entry point;
these hold the evidence, one per area of the codebase, following this set's
progressive-disclosure convention.

They are kept rather than discarded because every row carries a verified
`File.sol:LINE` citation. When the ledger says "m1 must keep writing
`dissolutionEligibleAt`", the fragment is where the reader finds which functions
read it and which of those m1 deletes.

| Fragment | Area | Most useful for |
|---|---|---|
| `data-model.md` | `Reservation.sol` structs, enums, storage, governance parameters, events | The field-by-field reader analysis that sharpened the storage-completeness rule, and the finding that two cap setters validate nothing at all |
| `proofs.md` | `ReservationProofs.sol` action lifecycles | Its section 4: the complete position-closing site list with per-site m1 reachability. The single most important result in the set |
| `router.md` | `ReservationRouter.sol`, `Bridge.sol`, `BridgeState.sol` | The independent count of 24 entry points, and which of the four delegatecall invariants are genuinely test-asserted rather than natspec-only |
| `vault.md` | `ReservationVault.sol` | The initiation / settlement / accounting path classification that decides what a pause flag may cover, and the line-cited upgradeability verdict |
| `touchpoints.md` | Non-reservation Bridge files | The integration seams a rewrite loses most easily, because they do not live in reservation files |
| `pr-map.md` | Measured git state of the eight PRs | Per-PR diffstat, the three branches behind their bases, and the `#1102` fold's actual reach |
| `keep-core.md` | keep-core PR #4238 | What the Go client has, and the executor it does not contain |

## Provenance and caveats

Solidity fragments were verified against `feat/utxo-reservation-guards` (the
`#1094` tip) unless a row says otherwise. **That tip predates the `#1102`
fold**, so line numbers in files `#1102` touched are pre-fix - see
`pr-map.md` section 3 and `../milestone-inventory.md` C-3. `pr-map.md` itself
was measured with git against every fetched branch ref, so its numbers are
current.

`proofs.md` labels its own decisions `PD-N` because this set's canonical
decision register (`../milestone-inventory.md` section 7) renumbered them during
synthesis; nine of its twelve changed number. That fragment carries the
concordance. A bare `D-N` always means the register.

Rows that could not be verified carry an explicit `UNVERIFIED` marker rather
than an assertion. Those are collected in `../milestone-inventory.md`.

*Generated 2026-08-21 during the milestone-split inventory pass.*
