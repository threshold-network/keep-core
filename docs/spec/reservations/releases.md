# Release scope: what M1 and M2 each must cover

This file assigns capabilities, goals, requirements and stories to the two product releases of the
UTXO reservation feature. **M1** is the first release: reserved reveal, acceptance, segregated
custody, re-anchor, timeout cleanup, stranding, release to pool before any acceptance request, and
the consumer SDK preflight path, with no owner redemption, renewal or dissolution of an accepted
position. **M2** is the second release: owner redemption (whole and partial), renewal and
dissolution, delivered through a new `ReservationVault` deployment plus a depositor migration
ceremony rather than through flags on the M1 vault. Those are product milestones. The epic tracker
in `../../plans/reservations-epic-tracker.md` uses `M0` to `M7` for the delivery phases of M1 itself
and its own `M2` means "tests and fork dry-runs"; nothing in this file uses that sense, so read
`M1` and `M2` here as releases, never as tracker phases.

Intent sources for the two definitions: `verification/README.md:19-23` (M1 capability set),
`requirements.md:144-165` (the capability split), `README.md:21-25` (the decision log, including
Option B at `:22` and the client activation ordering decision at `:25`),
`m1-b-implementation.md:128-208` (the minimal-vault decision), `roadmap.md:590-659` (what M2
restores and inherits), and `../../plans/reservations-epic-tracker.md:38-116` for the phase names
this file does not use.

Citation rules. A citation of the form `path:line` is relative to this directory at this commit. A
citation of the form `tbtc-v2@<revision>:path:line` names a contract line at that revision. This
file never restates a requirement: it names the owning document and the IDs, so the cited text is
authoritative and there is no second copy to drift. Where an item has no numbered requirement yet,
it is listed as an open question or an open decision, not as a requirement.

## M1

### Goals

M1's goals are the seven system goals of the requirement set, `G-1` to `G-7`
(`requirements.md:40-78`). This table gives the release assignment and the owning line; the goal
wording lives there.

| ID | Goal, compressed | Source |
|---|---|---|
| G-1 | A reserved deposit is segregated and individually traceable | `requirements.md:42` |
| G-2 | Minted tBTC equals the anchor, one to one and minus the fee | `requirements.md:46` |
| G-3 | A position survives routine wallet rotation | `requirements.md:51` |
| G-4 | Caps and the hand-off reserve bound the blast radius of an unfinished feature | `requirements.md:58` |
| G-5 | Neither the reserved deposit nor its anchor is swept or silently spent | `requirements.md:64` |
| G-6 | Storage stays layout-complete for the M2 field set | `requirements.md:67` |
| G-7 | The surface ships without pushing `Bridge` over EIP-170 and without standalone authority | `requirements.md:75` |

The per-actor trust model those goals imply is owned by `requirements.md:167-178` (actors) and
`verification/01-system-model.md` (canonical system model). M1 adds no per-actor goal IDs.

### Requirements

All 34 functional requirements are M1, at `requirements.md:508-550`. Grouped by the surface each
one constrains, with the owning line so a reader can follow every ID:

| Surface | FR IDs | Source |
|---|---|---|
| Admission and segregation | FR-1, FR-2, FR-19, FR-20, FR-30 | `requirements.md:516`, `:517`, `:534`, `:535`, `:545` |
| Acceptance and re-anchor settlement | FR-3, FR-4, FR-6, FR-7, FR-8 | `requirements.md:518`, `:519`, `:521`, `:522`, `:523` |
| Re-anchor reachability and privilege | FR-5, FR-32, FR-33 | `requirements.md:520`, `:548`, `:549` |
| Stranding, timeout and stale release | FR-9, FR-10, FR-11, FR-25, FR-26, FR-27 | `requirements.md:524`, `:525`, `:526`, `:540`, `:541`, `:542` |
| Release to pool before any acceptance request | FR-31 | `requirements.md:547` |
| Vault fee surface | FR-15, FR-16, FR-17 | `requirements.md:530`, `:531`, `:532` |
| Governance surface and router wiring | FR-12, FR-13, FR-14, FR-18 | `requirements.md:527`, `:528`, `:529`, `:533` |
| Client and SDK | FR-21, FR-22, FR-23, FR-24, FR-28, FR-34 | `requirements.md:536`, `:537`, `:538`, `:539`, `:543`, `:550` |
| Public views | FR-29 | `requirements.md:544` |

All 24 non-functional requirements are M1, in six families at `requirements.md:552-689`:

| Family | IDs | Source |
|---|---|---|
| Security | NFR-SEC-1 to NFR-SEC-7 | `requirements.md:552-590` |
| Liveness | NFR-LIV-1 to NFR-LIV-4 | `requirements.md:591-605` |
| Upgradeability | NFR-UPG-1 to NFR-UPG-3 | `requirements.md:606-619` |
| Gas and EIP-170 | NFR-GAS-1, NFR-GAS-2 | `requirements.md:620-643` |
| Observability | NFR-OBS-1 to NFR-OBS-3 | `requirements.md:644-666` |
| Operational duties | NFR-OPS-1 to NFR-OPS-5 | `requirements.md:667-689` |

The parameters, caps and failure-mode tables those requirements are exercised against are
`requirements.md:690-746`.

### User stories

The canonical story namespace is the zero-padded one in the verification set: `US-01` upward, 74
stories across five files, each with a status from the vocabulary in `verification/README.md`. M1
owns all of them. Two details matter when citing:

1. **The canonical form is zero padded.** `US-01` in the verification set and `US-1` in the
   requirement set are different stories with different text. Always cite the ID with its file.
2. **The older namespace is `US-1` to `US-18`** (`requirements.md:180-507`). It is kept because it
   is what the US to FR matrix maps (`requirements.md:846-868`), and it is not the story set M1 is
   verified against. Where this file says `US-01`, the verification set owns it.

| Story file | Story IDs | Source |
|---|---|---|
| `verification/04-deposit-stories.md` | US-01 to US-15, US-101 | `verification/04-deposit-stories.md:7-161` |
| `verification/04-wallet-spv-stories.md` | US-20 to US-34, US-40 to US-51 | `verification/04-wallet-spv-stories.md:7-184` |
| `verification/04-governance-vault-stories.md` | US-60 to US-76, US-80 to US-83 | `verification/04-governance-vault-stories.md:7-138` |
| `verification/04-sdk-stories.md` | US-93 to US-99 | `verification/04-sdk-stories.md:20-146` |
| `verification/04-user-stories.md` | US-90, US-92, US-100 | `verification/04-user-stories.md:38-59` |

The story index and its status vocabulary are `verification/04-user-stories.md` and
`verification/README.md`; the traceability matrix is `verification/11-traceability.md`.

### Out of scope

Each cut is a genuine absence in the M1 code, not a paused surface. Sources are the non-goal list
at `requirements.md:80-142` and the capability column at `requirements.md:144-165`.

- **Owner redemption, whole or partial.** No redemption entry point in the M1 vault
  (`requirements.md:94-100`; `m1-b-implementation.md:128-208`).
- **Renewal.** No `extendReservation` on the router and no `extendCustody` in the vault
  (`requirements.md:100-108`).
- **Dissolution.** Absent from the M1 Bridge; the enum value is kept for layout only
  (`requirements.md:104`).
- **Watchtower veto.** Absent, and vacuous in M1 because no redemption exists to veto
  (`requirements.md:130`; `verification/01-system-model.md:170`).
- **Emergency exit and the standing-committee mechanism.** Design reference only, not scoped or
  funded (`requirements.md:112-115`; `exit/README.md:9-30`).
- **Vault upgradeability, and a full vault behind flags.** The vault is plain `Ownable` with
  immutable constructor arguments; replacing it needs a new deployment (`requirements.md:116-130`;
  `README.md:22`; `roadmap.md:478-515`).
- **A reservation-eligible wallet allowlist.** Decided, not built (`requirements.md:134-138`;
  `roadmap.md:1225-1233`). Open decision, see below.
- **Retry credit.** Declared field only (`requirements.md:161`).
- **A structural bound on cumulative re-anchor fee loss.** Only the per-hop dust floor exists
  (`requirements.md:139-142`; `verification/10-accepted-risks.md:56`).
- **Tier 1 compensation for a stranded position.** No requirement, no storage field, no code
  (`verification/10-accepted-risks.md:47`). Open decision, see below.
- **The M2 migration ceremony.** A named obligation with no design (`m1-b-implementation.md:440-473`;
  `verification/10-accepted-risks.md:38`).

### Accepted risks

The register is `verification/10-accepted-risks.md`, scoped to M1 by its header (`:3`). Fourteen
rows describe behaviour M1 ships; AR-04 and AR-05 are `design-only` and carry M2 or process
obligations rather than M1 behaviour.

| ID | Risk, compressed | Status | Source |
|---|---|---|---|
| AR-01 | A third-party re-anchor hold consumes the target's ordinary slot | holds | `verification/10-accepted-risks.md:11` |
| AR-02 | No M1 exit for the owner: no redemption, renewal or dissolution | holds | `verification/10-accepted-risks.md:20` |
| AR-03 | The vault is not upgradeable and re-pointing needs an empty system | holds | `verification/10-accepted-risks.md:29` |
| AR-04 | M2 needs a new vault plus a depositor migration ceremony | design-only | `verification/10-accepted-risks.md:38` |
| AR-05 | Compensation for a stranded position is off-chain only | design-only | `verification/10-accepted-risks.md:47` |
| AR-06 | Re-anchor is unbounded in time, so the in-kind claim can grind down | holds | `verification/10-accepted-risks.md:56` |
| AR-07 | `closeReservation` is unreachable in M1 | holds | `verification/10-accepted-risks.md:65` |
| AR-08 | Permissionless third-party re-anchor griefing beyond the slot reuse | holds | `verification/10-accepted-risks.md:74` |
| AR-09 | The `Closing` wallet lifecycle has no production trigger in the client | holds-untested | `verification/10-accepted-risks.md:83` |
| AR-10 | The late-acceptance window can be missed if the SPV maintainer is down | holds | `verification/10-accepted-risks.md:96` |
| AR-11 | Expiry does not automatically release an accepted position | holds | `verification/10-accepted-risks.md:113` |
| AR-12 | Fee debt can grow while the reserve target is zero | holds | `verification/10-accepted-risks.md:143` |
| AR-13 | The stranding startup scan only looks back 30 days | holds-untested | `verification/10-accepted-risks.md:152` |
| AR-14 | A re-anchor timeout starts a cooldown that blocks everyone but governance | holds | `verification/10-accepted-risks.md:161` |
| AR-15 | `activeReservationsCount` under-reports slot pressure during pending re-anchors | holds-untested | `verification/10-accepted-risks.md:170` |
| AR-16 | The `receiveBalanceIncrease` ordering gap is enforced by the deploy script | holds-untested | `verification/10-accepted-risks.md:179` |

### Release gates

This file names the gates and points at the documents that define them. It does not restate them,
and there is no release-policy file at this commit.

1. **Build gates.** Four M1 gates, each marked Met or Not met: `m1-b-implementation.md:209-374`.
2. **Spec acceptance criterion.** Every FR traces to enforcing code, with the residual test gaps
   named: `requirements.md:747-761`.
3. **Audit program.** Pre-audit and during-audit tooling and external sign-off: `testing-plan.md`.
4. **Testnet round.** The live-network phase: `timeline-estimate.md`.
5. **Activation sign-off.** Deploy order, launch values, the client activation ordering gate and the
   irreversible vault activation warning: `README.md:25` and the deployment runbooks in tbtc-v2,
   `solidity/docs/RESERVATION_CAPS_DEPLOYMENT.md` and
   `solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md`.
6. **Client release.** The client build carrying the activation block ships before the vault is
   trusted: `README.md:25`.

### Two rules where the requirement text and the code had to be reconciled

Both rules were stale in `requirements.md` against the published code. M1 releases with the code,
because the code is the current state; `requirements.md` now carries each rule in the code's form,
labelled where the underlying product decision is open, and this section records what was
reconciled and where the open question lives.

1. **FR-9's wallet states.** FR-9 (`requirements.md:524`) now records all four admissible states:
   `Terminated`, `Closed`, `Live` with `block.timestamp >= dissolutionEligibleAt`, and `Closing`
   past both `dissolutionEligibleAt` and the closing period
   (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Reservation.sol:1301-1326`). The `Live` branch is
   labelled as describing the published code, in the same terms as VF-10
   (`verification/12-finding-adjudication.md:159`), because whether a position on a healthy wallet
   should become strandable once the eligibility time passes is an open product decision (see
   below). Until it is taken, the release behaviour is the code's: once the eligibility time
   passes, anyone can strand the position. The verification set also carries the story, US-101
   (`verification/04-deposit-stories.md:161`).
2. **`Stranded` was called terminal.** The non-goal list (`requirements.md:112-115`), the scope
   table (`requirements.md:154`) and the glossary (`requirements.md:887`) now say that the state
   is a fallback and that it is recoverable, not terminal. The late-recovery rule is the code's
   behaviour: a stranded reservation is admitted to settlement whenever its generation settles
   late (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:267-280`, with the
   `(stranded && late)` precondition at
   `tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:278`), and both settlement
   paths then write the position back to `Active`
   (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:576` for acceptance,
   `tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:843` for re-anchor). A
   late-arriving anchor therefore restores a stranded position rather than leaving it written off,
   and the recovery path is to be kept. The sibling claim that `dissolutionEligibleAt` has one M1
   reader was stale for the same reason: FR-9 now names both readers.

The client side is recorded, not repaired: the shipped stranding watcher dispatches `Closed`,
`Terminated` and `Closing` states and never `Live`, as the VF-10 entry states.

### Open product items

Each item below is undecided. Nothing in M1 depends on this file settling it, and each is marked
**open, decision pending**.

- **Reservation-eligible wallet allowlist.** What is decided: `roadmap.md:1225-1233` records a
  decision to add a governance-set allowlist. What exists: nothing, so any `Live` wallet remains
  selectable at reveal (`requirements.md:134-138`). Consequence while it is open: a
  decided-but-unbuilt capability has no named risk acceptor, and launch admission is bounded only
  by the caps and the hand-off reserve. Open, decision pending.
- **Tier 0 and Tier 1 compensation.** Tier 0 exists as the stranding event; Tier 1, a refund of fees
  paid on a stranded position, has no requirement, no storage field and no code
  (`verification/10-accepted-risks.md:47`; `stranding-compensation-proposal.md:227-238`, the Tier 1
  row at `:234`). The
  milestone inventory assigns both tiers to both releases (`milestone-inventory.md:804-805`), which
  the code does not match. Open, decision pending.
- **Release on time alone.** Whether a position on a healthy wallet must become releasable on time
  alone is the VF-10 product decision (`verification/12-finding-adjudication.md:159`). The code
  already admits the branch and AR-11 describes it (`verification/10-accepted-risks.md:113`), while
  FR-9 still omits it (`requirements.md:524`). The product answer is recorded nowhere. Until it is
  decided, the release story told to a holder is wrong in one direction or the other. Open,
  decision pending.
- **A full vault in M1.** Option B (a minimal M1 vault) is the recorded decision (`README.md:22`;
  `m1-b-implementation.md:128-208`). Reopening it would add an exit-control surface that the M1
  vault does not have, plus a rework of the vault-binding rule that M2 must change in any case
  (`verification/10-accepted-risks.md:29`, `:38`). Open, decision pending.

## M2

### Goals

M2 has no goals in the requirement set. The seven below are new IDs allocated for this file, and
they are free at this commit (see the ID section at the end). Each is a product goal with a design
description in the sources and no numbered requirement yet, except where the row says otherwise.

| ID | Goal | Intent source | State |
|---|---|---|---|
| G-8 | Let the owner redeem the anchor in kind, whole or in part, and close or shrink the position on settlement | `feature-spec.md:510-545`; `feature-spec.md:637-698` (claim equals anchor across the burn); `verification/10-accepted-risks.md:20` | Design described, no requirement |
| G-9 | Let the owner renew custody for one further term inside a bounded window, intent-bound so a stale transaction cannot buy a different duration | `feature-spec.md:579-621`; `verification/10-accepted-risks.md:20` | Design described, no requirement |
| G-10 | Restore dissolution: after the snapshotted eligibility time, release the anchor to the pool permissionlessly, with one action in flight per wallet | `feature-spec.md:566-577`; `roadmap.md:601-618` | Design described, no requirement |
| G-11 | Decide what happens to a position whose term passes while M2 is live: M1 does nothing at expiry, and the only routes out are wallet-state stranding and the `Live`-wallet rule | `requirements.md:100-108`; `verification/04-deposit-stories.md:161`; `requirements.md:885` | **Open, decision pending** |
| G-12 | Charge and account the M2 fees (renewal, redemption) and keep the in-kind fee reserve solvent across M1 positions and the new vault | `feature-spec.md:637-698`; `verification/10-accepted-risks.md:20`; `requirements.md:530-532` | Fee mechanics described, no requirement |
| G-13 | Migrate M1 positions to the new vault by depositor opt-in, without liquidating them and without changing the dates already snapshotted for them | `README.md:22`; `requirements.md:829-845`; `roadmap.md:556-570`; `verification/10-accepted-risks.md:38` | Requirement stated in outline only |
| G-14 | Pay Tier 1 compensation (fee restitution) to a stranded position's owner, if it moves from M1 to M2, where the inventory assigns it | `verification/10-accepted-risks.md:47`; `milestone-inventory.md:804-805`; `stranding-compensation-proposal.md:227-238` | **Open, decision pending** |

Two notes that keep the table honest. G-11 is not G-10: dissolution is a transaction anyone can
send once eligible (`feature-spec.md:566-577`), while lapse is the absence of any transaction at
expiry, which is what M1 has today (`requirements.md:100-108`). G-14's release slot is claimed by
both milestones in the inventory (`milestone-inventory.md:804-805`) and shipped by neither; Tier 1
needs a per-position fees-paid record (`stranding-compensation-proposal.md:234`), so where it
lands is a storage decision as well as a release decision, and it must not be assumed into M2.

### Requirements

M2 has no numbered requirements. The ranges that are free for them, and the existing requirements
M2 has to amend, are both listed here; nothing in this section is an M2 acceptance criterion.

- **Existing requirements M2 amends rather than adds.** FR-5 (the re-anchor eligibility gate,
  `requirements.md:520`), FR-12 (the validator, `requirements.md:527`), FR-15 and FR-17 (the vault
  fee surface, `requirements.md:530-532`), FR-19 and FR-20 (the caps, `requirements.md:534-535`),
  FR-26 (the stranding watcher, `requirements.md:541`), and G-6 (storage completeness,
  `requirements.md:67`).
- **Verification entries that bind M2.** INV-603 (`verification/02-generation-time-invariants.md:91`),
  INV-702 (`verification/02-generation-time-invariants.md:177`),
  PAR-X6 (`verification/03-parameters-governance.md:225`),
  PAR-X7 (`verification/03-parameters-governance.md:233`), SM-X7
  (`verification/01-system-model.md:170`), and the stories US-68, US-69 and US-101
  (`verification/04-governance-vault-stories.md:67`, `:75`;
  `verification/04-deposit-stories.md:161`).
- **Accepted risks that are M2 obligations.** AR-02 (`verification/10-accepted-risks.md:20`), AR-03
  (`:29`), AR-04 (`:38`), AR-05 (`:47`). They are M1-scope register rows marked `design-only` where
  they describe M2 work; no M2-only row exists, so AR-17 and above are free.
- **Design descriptions that an M2 requirement would be written from.** `feature-spec.md`, which
  the index calls the M2 target (`README.md:49`), plus the entries it covers: redemption
  (`feature-spec.md:510-545`), re-anchor (`feature-spec.md:546-565`), dissolution
  (`feature-spec.md:566-577`), renewal (`feature-spec.md:579-621`), the watchtower veto
  (`feature-spec.md:623-636`), claim and fee mechanics (`feature-spec.md:637-698`), the governance
  parameter surface (`feature-spec.md:798-884`), and deployment sequencing
  (`feature-spec.md:885-955`). These are descriptions of code that was never landed on M1, so they
  settle mechanics and questions, not decisions.
- **Held M2 code is cost evidence, not scope.** tbtc-v2 `#1129` re-lands reserved redemption with
  two escrow fixes and states that it is milestone-2 work; the earlier attempt (`#1121`) was merged
  and then reverted as a scope violation, which is recorded in `requirements.md:86-89` and
  `../../plans/reservations-epic-tracker.md:36`. Measure it against NFR-GAS-1
  (`requirements.md:622`) before treating it as a plan, and see the late-redemption question below
  for the one rule it settles differently from the design description.

### User stories

M2 has no user stories. Every story that exists belongs to M1: the 74 canonical stories above and
the older `US-1` to `US-18` set. The verification set is M1-only by its own scope statement, so it
cannot host M2 stories without contradicting itself. What M2 inherits is the set it conflicts with:

| Story | Conflict M2 must resolve |
|---|---|
| US-5 to US-8 (`requirements.md:261-332`) | Re-anchor is unbounded in position age by design; restoring dissolution reopens the question |
| US-10 (`requirements.md:343-367`) | Stranding is M1's only terminal path and its remedy is the loss of the in-kind option; with redemption live the same event means something else |
| US-14 (`requirements.md:422-435`) | Governance updates stop at the M1 parameter set; M2 adds fee parameters and the vault-binding change to the same surface |
| US-15 (`requirements.md:436-455`) | Fee economics is one initiation fee plus a reserve; M2 has three fees and two vaults |
| US-101 (`verification/04-deposit-stories.md:161`) | The only M2-shaped canonical story: stranding once the promise clock passes, written as a stranding rule rather than a lapse or dissolution rule |

### Out of scope

- **The standing-committee emergency exit.** Deferred, not funded, and its reopen trigger is
  evidence from Tier 0 rather than a plan (`exit/README.md:26-32`;
  `milestone-inventory.md:778-796`).
- **A flag-gated full vault instead of the migration.** That is the reopening question below, not a
  scope item; Option B is the recorded decision (`README.md:22`).
- **The FROST signer migration.** A separate program with its own calendar; the reservation
  obligation is the shared storage layout plus the settlement-side patch when FROST activates
  (`frost-reservations-interaction.md`; `milestone-inventory.md:820`).
- **Tiers 2 and above of the loss story.** Not accepted in either milestone
  (`milestone-inventory.md:804-805`).

### Release gates

No M2 gate, acceptance criterion or exit condition is written down anywhere. Pointers to what
exists, and does not substitute for one:

- The M1 gate list, which is the shape an M2 list has to match in rigour:
  `m1-b-implementation.md:209-374` (build gates) and `requirements.md:747-761` (acceptance).
- The client activation ordering decision, which M2 needs an equivalent of for a second vault:
  `README.md:25`.
- The Bridge implementation upgrade procedure, which is the only written procedure for the upgrade
  M2 needs: `roadmap.md:478-515` and the runbook in tbtc-v2,
  `solidity/docs/RESERVATION_CAPS_DEPLOYMENT.md`.
- The held M2 draft body and the tracker, which record intent to keep M2 out of M1 rather than a
  gate: `../../plans/reservations-epic-tracker.md:36`.

### Open questions

These close the gap the release-scope question names. They are questions with a source, not
requirements, and none of them is settled here.

1. **Migration ceremony.** One paragraph of intent exists: a new vault plus a depositor opt-in
   (`requirements.md:829-845`; `verification/10-accepted-risks.md:38`). Open: who opts in and
   whether doing nothing is a valid answer; which state moves (owner, anchor, dates, fee debt) and
   which is re-derived; the outcome for a position whose owner never opts in; whether fees are
   charged once or twice across two vaults; whether migration restarts the term; whether it is
   one-way.
2. **The promise clock.** Two candidate clocks in the documents, `dissolutionEligibleAt`
   (`roadmap.md:635-639`) and `expiresAt` (`feature-spec.md:543-545`), and M1 reads neither on a
   release path. Open: which field is the promise the product sells, what the dissolution delay is,
   and what happens to a position whose clock passes before M2 ships.
3. **M2 size and dates.** The 1.13:1 milestone ratio predates Option B and has not been re-derived
   (`roadmap.md:590-600`), and the code-mass table claims M2's Solidity is already written
   (`roadmap.md:895-905`) while the Option B column in the same section says a new vault is needed.
   Open: which size figure holds once the new vault, the migration and the validator redeploy are
   counted, and who owns the date.
4. **M2 launch gates.** The four items below are the candidates; none has an owner or a criterion.
   A dissolution executor: which client task signs and proves a dissolution, and who is on call if
   it never runs (`requirements.md:541`; `m1-b-implementation.md:209-374` for the M1 pattern). The
   redeemer script fix: which script types are in scope and what a wrong script does. New-vault
   activation ordering: the equivalent of the C-4 gate (`README.md:25`). Migration rehearsal: on
   which network, and what it must prove before any state moves.
5. **Inherited decisions.** Three questions with no owner: whether to restore the re-anchor
   eligibility gate (`roadmap.md:1234-1239`), whether M1-era positions get M2 semantics
   (`roadmap.md:626-634`), and the dated in-kind deadline implied by the term
   (`roadmap.md:635-639`). Open: who decides each, and where the decision is recorded.
6. **No M2 requirement set.** This file is a scope frame and says so. Open: who writes the M2
   requirement set, against which evidence pin, and which acceptance criteria gate the release.
7. **Bridge bytecode headroom.** NFR-GAS-1 holds at the M1 tip (`requirements.md:622`) and the
   tracker restates 1,009 bytes of headroom (`../../plans/reservations-epic-tracker.md:114`), but
   nothing sizes the M2 surface against it. Open: whether M2 fits, or needs the router's second
   extension or a library split. It is measurable before a design freeze.
8. **Reopening Option B.** The decision, its confirmation and its stated cost are recorded
   (`README.md:22`; `roadmap.md:556-570`). Open: whether a flag-gated full vault is still off the
   table now that the migration, the vault-binding change and the opt-in ceremony are all unsized.
   If the answer is no, that is a re-scope, not a review note.
9. **Storage parity with FROST.** Both features share the same `__gap`; there is no combined parity
   test (`frost-reservations-interaction.md`; `requirements.md:616`). Open: which feature allocates
   which slots, and where the combined test that fails on a clash lives.
10. **The exit family's reopen trigger.** The trigger is written in unmeasurable terms
    (`exit/README.md:26-32`). Open: whether it is rewritten in measurable terms (stranding
    frequency, anchor value, carrying cost) or the family is dropped.
11. **The wallet allowlist's release slot.** Decided and unbuilt (`requirements.md:134-138`;
    `roadmap.md:1225-1233`), called a candidate for an M1 backport or for M2
    (`requirements.md:163`). Open, decision pending; it changes the admission rules a migration has
    to respect.

### Contradictions an M2 requirement must resolve first

1. **Late partial redemption.** Open, no choice made here. Side A, the design description: a late
   partial proof against a matching newer generation must settle, preserving the burn
   (`feature-spec.md:537-541`). Side B, the only held code: a timed-out redemption can never be
   proven again, because the timeout already refunded the escrowed claim and a late proof would
   double-claim, which is one of the two escrow fixes in tbtc-v2 `#1129`. Nothing in the tree
   decides between them, so the M2 requirement has to, and until it does the design description and
   the held code disagree on a settlement path.
2. **The promise clock.** See question 2.
3. **M2 size.** See questions 3 and 9.
4. **Owner exit per story, gate per decision.** AR-02 says redemption, renewal and dissolution
   "ship together" (`verification/10-accepted-risks.md:20`), while `roadmap.md:601-618` argues the
   two halves have different risk profiles and need not ship together. The requirement set has to
   pick one.

## ID allocation

- **G-8 to G-14 are new here.** `G-1` to `G-7` are the M1 goals (`requirements.md:40-78`). Nothing
  under `docs/spec` at this commit uses `G-8` or above, checked by grep before this file was
  written; a later M2 requirement set takes them over from here.
- **No new FR, NFR, US or AR IDs are allocated.** When an M2 requirement is written, the free
  ranges are FR-35 and above (M1 ends at FR-34, `requirements.md:550`), NFR-SEC-8, NFR-LIV-5,
  NFR-UPG-4, NFR-GAS-3, NFR-OBS-4 and NFR-OPS-6 and above, US-102 and above (the verification set
  ends at US-101, `verification/04-deposit-stories.md:161`), and AR-17 and above
  (`verification/10-accepted-risks.md:179`).
- **The US-19 to US-101 band is not free in practice.** The verification set numbers stories with
  two or three digits, so a new unpadded `US-25` would be the same string as the existing
  zero-padded `US-25`.

## Purpose note

The normative entry point for M1 evidence is the verification index
(`verification/README.md`). The decision log is `README.md:11-25`. The capability split that this
file assigns to releases is `requirements.md:144-165`, which is the M1 view of the same assignment.