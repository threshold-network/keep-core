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
| `router.md` | `ReservationRouter.sol`, `Bridge.sol`, `BridgeState.sol` | The independent count of 24 entry points, and which of the four delegatecall invariants are genuinely test-asserted rather than natspec-only. Its own m1-minimal derivation ("24 - 5 + 1 = 20") was one entry point short of the design-time-corrected 21 (9 state-changing + 12 views) recorded in `m1-b-implementation.md` (2026-09-08, commit `8d8400106`) - the arithmetic omitted `notifyReservationAcceptanceTimedOut`. Both design-time estimates are superseded: the M1 code has **22** entry points (11 state-changing + 11 views) - see `router.md`'s own Status vs M1 code banner |
| `vault.md` | `ReservationVault.sol` | Its "M1 vault surface" section: the 7 entry points + constructor M1 actually ships (`reservations-upgrade`@`9f8f5ef1`), with access/classification and the in-kind fee economy as implemented. Everything below that section is the pre-Option-B, pre-minimisation full-vault design (662 lines), kept as the reference for M2's new vault deployment, not a description of M1 |
| `touchpoints.md` | Non-reservation Bridge files | The integration seams a rewrite loses most easily, because they do not live in reservation files |
| `pr-map.md` | Measured git state of the eight PRs | Per-PR diffstat, the three branches behind their bases, and the `#1102` fold's actual reach |
| `keep-core.md` | keep-core PR #4238 (superseded; see #4282 / `reservations-epic`) | What PR #4238's own branch has in isolation. For the executor #4238 lacked, see the fragment's Status vs M1 code banner: it was built on `reservations-epic` (`pkg/tbtcpg/reservation_{acceptance,reanchor}.go`, `pkg/chain/ethereum/tbtc.go`, `pkg/maintainer/spv/reservation_*.go`), not on #4238's own branch |

## Provenance and caveats

Solidity fragments were verified against `feat/utxo-reservation-guards` (the
`#1094` tip) unless a row says otherwise. **That tip predates both the `#1102`
fold and the Option-B minimal-vault rewrite** (commit `4d549e64`), so line
numbers in files either touched are pre-fix - see `pr-map.md` section 3 and
`../milestone-inventory.md` C-3. These fragments are evidence for the
**original PR stack** (`#1088`, `#1090`-`#1096` - the M2 target), not for the
current M1 codebase (`reservations-upgrade`@`9f8f5ef1` tbtc-v2,
`reservations-epic`@`f66f11240` keep-core). Where that gap matters to a
reader, the affected fragment carries its own dated "Status vs M1 code
(2026-09-28)" banner (`data-model.md`, `keep-core.md`, `proofs.md`,
`router.md`, `touchpoints.md`, `vault.md`) - `touchpoints.md` also carries
inline "M1 code:" notes at each open question. `pr-map.md` carries a
top-of-file bold status paragraph in the same place a `##` banner would sit,
plus an inline note at its stale keep-core row (`pr-map.md` section 6).
`pr-map.md` itself was measured with git against every fetched branch ref,
and its §2/§3 figures were independently re-measured 2026-09-28 and still
hold; only its keep-core row is stale (see its own inline note).

`proofs.md` labels its own decisions `PD-N` because this set's canonical
decision register (`../milestone-inventory.md` section 7) renumbered them during
synthesis; nine of its twelve changed number. That fragment carries the
concordance. A bare `D-N` always means the register.

Rows that could not be verified carry an explicit `UNVERIFIED` marker rather
than an assertion. Those are collected in `../milestone-inventory.md`.

*Generated 2026-08-21 during the milestone-split inventory pass.*
