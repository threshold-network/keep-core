# Approved design changes outside the implementation snapshots

This chapter separates approved target design from the inspected implementation.
It is not a claim that these changes shipped.
The source is the audit records `handoff-capacity-plan.md:1-48` and
`testnet-readiness-plan.md:264-297`, which are published separately from this review set.
The human approved D31 and D28 on 2026-10-12.
Their implementation is outside the `7aabfc502` / `4b95f0a06` snapshots used by this review set.
D31 is now implemented at later pins: contracts, deploy scripts and SDK at tbtc-v2 `806b1106f`, client at keep-core `d40f6aadc`.
The chapters cite those pins explicitly for D31 behavior. Nothing is deployed on Sepolia or mainnet.
D28 is implemented at keep-core `9c4e1b221` and tbtc-v2 `8eb26c2e2`.
Its local acceptance and remaining live actions are recorded at
`keeper-alert-acceptance.md:1-141`.

## D31: privileged handoff plus acceptance reserve

| Part | Approved rule | Difference from inspected implementation | Evidence status |
|---|---|---|---|
| C: retiring-source handoff | Governance, owner, authorized SPV maintainer or proven source member can request a handoff from MovingFunds/Closing without target per-wallet count/amount cap checks | At the original snapshots the request checked those caps for exempt and third-party callers alike | implemented: `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922` |
| C: bookkeeping | Still add target count and amount; release source accounting only on settlement | Exemption changes admission, not conservation equations | implemented: counts and amounts are added on every path (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:949-950`); mutation equations remain review obligations |
| C: ordinary Live-source rotation | Keep target caps for governance rotation from a Live source | Live-source rotation remains governance-only | unchanged requirement; enforced at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:824-828` |
| C: third-party requests | Keep both target caps and the third-party hold cap | Third-party authorization remains permissionless only from retiring sources | unchanged requirement; hold cap at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:896-906` |
| A: acceptance reserve | New `handoffReserveWallets` R must be at least 1 | No such value or acceptance gate exists at the original pins | implemented: storage `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeState.sol:509`; zero rejected at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1699-1702` |
| A: admission equation | Require `L > R` and `A + 1 <= (L - R) * N` | L is Live-wallet count, A is global occupied position count, N is per-wallet count cap | implemented as `A + 1 <= max(0, L - R) * N` at the acceptance request (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1063-1077`); `L <= R` rejects every acceptance |
| Signing validator | Remove current target count/amount cap rejection for already-authorized re-anchors | The validator at the original snapshots re-checked them | implemented: `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`; replaces VF-01's signing policy |
| Client target search | Prefer targets with headroom; permit a fallback Live target for eligible member-proof handoff | The client at the original snapshots rejected a saturated target | implemented: `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1153-1241` |

The conservation equations must still include Pending acceptance and target holds.
The product and additions must have defined checked arithmetic and width.
A caller must not obtain a handoff exemption through an invalid member proof.
A target must remain Live and different from the source.
The existing cooldown and signing/proof guards remain unless an explicit design change replaces them.

### What the reserve proves, and what it does not

At a successful request, A positions fit in the aggregate count capacity of L minus R wallets.
This is a count-admission bound, not a permanent liveness guarantee.
Wallet state and pending holds can change after that request.
Privileged exemption removes the target-cap obstacle; it does not create a Live wallet or an online signing quorum.
It also does not eliminate fee limits, cooldown, chain availability, proof deadlines or integer limits.
The reserve equation alone does not prove per-wallet amount headroom.

If L falls to R or lower, new acceptance fails the reserve gate.
Existing accepted positions remain real; governance must not erase them to restore an admission inequality.
If no Live target exists, privileged cap exemption cannot perform a handoff.

### Required contract/client/SDK surfaces

| Surface | Approved change | Required verification |
|---|---|---|
| Storage | Add packed uint32 `handoffReserveWallets` without shifting existing storage | Storage-layout and router/Bridge parity checks |
| Caps setter | `updateReservationCaps` gains a fifth argument | Regenerate selector, governance calldata, interfaces and bindings |
| Caps update event | Add the reserve field | Event declaration order/indexing and decoder agreement |
| Views | Add `reservationHandoffReserveWallets()`; retain the existing four-field `reservationCaps()` tuple | Contract/client/SDK ABI agreement |
| Governance | Stage and finalize all five cap arguments | Delay, overwrite and exact-value behavior |
| Request | Gate acceptance by the reserve equation; apply the retiring-source exemption only to eligible callers | Boundary and authorization behavior tests |
| Validator | Accept an authorized above-cap handoff while retaining its other guards | Real validator harness, not a mock echo |
| Client | Eligible fallback target selection | An actual request when every target is at its ordinary cap |
| SDK | Include the reserve view/gate in chosen-wallet and automatic-wallet preflight | Failure before funding when the reserve gate blocks acceptance |
| Deploy/rehearsal scripts | Use the new arity and preserve safe staging order | Run the same actual script after integration |
| Sepolia | R = 2 with four Live wallets and N = 1 | Two admissions fit the gate; the next fails without raising the approved max-active cap |
| Drill D4 | An eligible privileged handoff can use a target occupied by another position's third-party hold | Distinct position keys; source state and cooldown admissible; no claim of overriding ActionPending on the same key |

The earlier D19 risk direction is no longer undecided.
D31 selects C plus A. Its integration is verified on the reservations branches (evidence below); it is not deployed.
The original snapshots still carry the D19 limit. Chapters that describe D31 behavior cite `806b1106f` and `d40f6aadc` explicitly, and their old-pin claims stay identifiable.
Do not describe these pins as a deployment.

### Acceptance evidence required for D31

- A privileged retiring-source request to a full target succeeds.
- The same full target rejects an ordinary third-party request.
- Governance rotation from a Live source still observes ordinary target caps.
- Invalid member proof cannot claim the exemption.
- The reserve rejects at `(L - R) * N` while the global cap still has room.
- A falling Live-wallet count changes new-request admission immediately.
- Zero reserve is rejected.
- The real signing validator accepts the authorized above-cap target.
- Client fallback, SDK preflight and five-argument governance surfaces agree.
- Fork and Sepolia rehearsals record any seeded wallet state explicitly.

| Required evidence | Evidence at `806b1106f` / `d40f6aadc` |
|---|---|
| Privileged retiring-source request to a full target succeeds | owner MovingFunds `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`; owner Closing `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:435`; SPV maintainer `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:443`; proven member `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:452`; above the amount cap `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:494` |
| The same full target rejects a third-party request | count `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`; amount `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:502` |
| Governance rotation from a Live source keeps the caps | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485` |
| Invalid member proof cannot claim the exemption | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235` |
| Reserve rejects at `(L - R) * N` while the global cap has room | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` |
| A falling Live-wallet count tightens admission at once | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346`; `L <= R` blocks every acceptance `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379` |
| Zero reserve is rejected | setter `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163`; calldata script `tbtc-v2@806b1106f:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts:486` |
| The real signing validator accepts the authorized above-cap target | at the cap `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1193`; count above `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1205`; amount above `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1219` |
| Client fallback, SDK preflight and five-argument governance agree | client `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1479`, `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1611`; SDK `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414`; governance `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705`, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4789`; selectors `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36`; drift check 8/8 at `d40f6aadc` |
| Fork and Sepolia rehearsals record seeded state | mainnet fork a local mainnet-fork run (no pin) (one seed: the relay difficulty); Sepolia script run at tbtc-v2 `4306c2345` (fork mode and a local Sepolia fork only) |

### Integration evidence (2026-10-12)

D31 is integrated on the reservations branches. Nothing in this section means a Sepolia or mainnet deployment.

| Evidence | Revision | Result |
|---|---|---|
| Contracts, deploy scripts, SDK | tbtc-v2 `reservations-upgrade` @ `806b1106f` (includes `d4397f208` and `dc5c2ed71`) | 710 contract and deploy tests pass. The SDK passes 927 tests and builds |
| Contract red checks | base `9d2086965` with the new tests | 9 behavioral failures (gate x3, privileged hand-off x5, validator x1); 3 controls pass |
| Deploy script 98 | `7c8b2e62d` | The full-run test failed with the reserve missing from the caps calldata, and passes with it |
| SDK red check | old `reservations-service.ts` with the new tests | 5 behavioral failures |
| Testnet client | keep-core `reservations-epic` @ `1bed10dee`, merged into the testnet build `reservations-epic` @ `d40f6aadc` | Bindings regenerated from the compiled contracts. The full router ABI drift check passes 8/8 with no exclusions; it was re-run at `d40f6aadc` against contracts equal to `806b1106f`. The real validator harness rejects an above-cap target on the old validator and accepts it on the new one. The member-proof fallback fails on the old task code and passes on the new; the request carries a member proof. The full Go suite ran on the merge `ad9905350`; its one failure (a watcher-death test that predates the merge) is fixed in `d40f6aadc`, and that package passes |
| Mainnet fork | tbtc-v2 a local mainnet-fork run (no pin), block 26154824 | 34/34: gate rejection, privileged above-cap hand-off, third-party limits, validator `eth_call`, TBTC deltas. One seed: the relay difficulty |
| Sepolia script | tbtc-v2 `4306c2345` | Fork mode 24 pass / 3 off-chain skips. Live mode on a local Sepolia fork: prepare, resume, refused early activation (no transaction sent), activation 7/7. Reserve 2 read back. The branch head `ad0b4c5a3` adds SDK initiation and the keeper monitor; its contracts are unchanged |

The keep-core review line (an earlier keep-core review line) has no member-proof re-anchor binding. On that line the fallback would send a third-party request, which keeps the caps. Its `reservations-upgrade` branch is superseded by `reservations-epic` and must not be merged.

`client.yml` now pins `TBTC_V2_REF` to the head of the tbtc-v2 reservations branch, and the vendored
fallbacks, the Go bindings and the harness validator were checked against the artifacts compiled at
that commit. A later change to the branch requires the pin and the bindings to move together (XR-G-2).

## D28: existing maintainer role plus watchdog alerts

The keeper is an independent instance of keep-core's existing `maintainer` role.
It adds no signing stake or authorization bypass.
Wallet operator watchers remain in place.
An independent proof submitter still needs SPV-maintainer authorization.
Permissionless cleanup transactions need a funded caller, not a wallet signing quorum.

At keep-core `9c4e1b221`, enabling `Maintainer.Spv.ReservationProofsEnabled` starts the existing proof loops and watcher wiring
(`keep-core@9c4e1b221:pkg/maintainer/spv/spv.go:86-135`).
That wiring starts the keeper observer at the one-minute polling interval
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_wiring.go:58-66`,
`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_wiring.go:646-671`).
The observer reads chain state and submits no transactions
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:38-46`).

At tbtc-v2 `8eb26c2e2`, the existing monitor registers `ReservationKeeperMonitor` only when `RESERVATION_KEEPER_METRICS_URL` is set
(`tbtc-v2@8eb26c2e2:monitoring/src/index.ts:70-85`).
An approved `SENTRY_DSN` enables the existing Sentry receiver
(`tbtc-v2@8eb26c2e2:monitoring/src/index.ts:96-100`).
It sends Warning events as `warning` and Critical events as `fatal`
(`tbtc-v2@8eb26c2e2:monitoring/src/sentry-receiver.ts:26-30`,
`tbtc-v2@8eb26c2e2:monitoring/src/sentry-receiver.ts:60-69`).

### Signals and alert rules

| Watchdog | Gauge rule at `9c4e1b221` | Monitor action at `8eb26c2e2` |
|---|---|---|
| Late acceptance proof risk | Count tracked unproven acceptance generations whose snapshotted `timeoutAt + termSeconds` is at most seven days ahead of chain time. TimedOut generations leave the set after that edge passes (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:85-106`, `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:323-335`). | Positive count produces Critical (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-164`). |
| Retiring source without an eligible target | Count Active positions on MovingFunds or Closing sources without an aged Live target, sufficient anchor value or a satisfiable signing window. Privileged hand-off ignores ordinary target count/amount caps (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:109-208`). | Positive count produces Critical (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:165-172`). |
| Vault fee debt | Read `inKindFeeDebtSat` through the vault named in Bridge parameters (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:170-187`). | Any positive debt produces Warning, not only increasing debt (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:173-180`). |
| Deposit request deadline | Count still-reserved deposits with no acceptance request when `refundDeadline - 24h - actionTimeout` is within one day of chain time. An elapsed or underflowed request window also counts; requested, released and cleared deposits leave the set (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:220-274`). | Positive count produces Warning (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:181-190`). |
| Keeper observation health | Publish the chain-head timestamp only after every required read succeeds. A failed pass keeps the last published gauges and timestamp (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:93-190`). | Unavailable endpoint, invalid samples, missing timestamp or excessive timestamp deviation produces Critical (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:106-155`, `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:192-225`). |

The five exported sample names and their producer/consumer agreement are XR-M-8 through XR-M-12.
The late-proof count does not establish Bitcoin confirmation.
A zero no-target count does not establish caller authorization, cooldown expiry, signing availability or proof delivery.
Positive risk samples can accompany a stale-observation alert; health checks do not suppress those risk events.

### Historical discovery and timing

Wallet registration discovery starts independently of reservation activation, in bounded chunks
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:48-82`).
This includes retiring sources and usable targets registered before activation.
Acceptance and deposit-reveal scans retain their activation boundary
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:198-203`,
`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:244-251`).
Wallet state refreshes every ten minutes
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:33-38`,
`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:282-296`).
A fresh observation is not an instantaneous wallet-state update.

`RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` defaults to 1800 and must be a positive safe integer
(`tbtc-v2@8eb26c2e2:monitoring/src/context.ts:72-86`).
The monitor compares the observation with the latest chain-head time, not an old catch-up window
(`tbtc-v2@8eb26c2e2:monitoring/src/index.ts:75-80`,
`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:98-103`).
It permits exactly the configured deviation in either direction; larger deviation alerts.
EC-23 and EC-24 record discovery and freshness boundaries.
INV-115 records failed-pass publication; US-100 records delivery and its operator prerequisites.

### Recorded acceptance (2026-10-13)

| Check | Pinned evidence | Scope and result |
|---|---|---|
| Four risk alerts and healthy/repayment controls | `keeper-alert-acceptance.md:6-37`, `keeper-alert-acceptance.md:40-60` | Real keeper and normal monitoring CLI on a local mainnet fork. A real re-anchor created 1000 sat debt; repayment cleared it. All four alerts reached a local Sentry envelope sink. |
| Historical wallet discovery regression | `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_wallet_discovery_test.go:12-73`; run record `keeper-alert-acceptance.md:67-90` | Two tests failed before the fix. The corrected binary delivered the previously missed no-target alert against the same fork state. |
| Keeper health incident | `keeper-alert-acceptance.md:92-102` | Pausing the owned keeper caused a real metrics timeout and one fatal health event. Manager success, checkpoint advancement and empty outbox were checked. |
| Rehearsal monitor and Testnet4 setup | `keeper-alert-acceptance.md:110-141` | Monitor `8eb26c2e2` passed 51 tests on the host, repeated the age stage and completed a Testnet4/read-only Sepolia manager run. Other acceptance stages used monitor `ad0b4c5a3`; the Docker image was not rebuilt in that pass. |

These runs do not prove a live Sepolia keeper, an external incident route, signing-quorum availability or Bitcoin proof validity.
The fork used a labelled difficulty-0 relay and a throwaway activation overlay.
Neither fixture belongs in a release build.
No mainnet or Sepolia transaction was sent.
DR-10 remains unresolved for an outage and proof recovery across the late-settlement cutoff.

### Remaining live actions

Follow the pinned operator procedure
(`keeper-drill-setup.md:7-22`,
`keeper-drill-setup.md:110-196`).
The human prepares Sepolia, publishes its package, confirms client rollout and activates at block B.
The agent first generates the real deployment bindings and the activation-block client release.
Operators run one independent keeper and one monitoring writer, with private metrics access and persistent state.
They must confirm fresh observations, manager success, checkpoint progress and delivery to the approved Sentry route.
Local acceptance does not waive these live checks.

Alert rules belong to the existing monitoring service.
Stranding a Closing position early and repaying debt with TBTC remain human-approved incident actions.
A watchdog warns; it does not make a lost quorum, expired proof or spent refund input recoverable.

## Integration rule

When D31 or D28 lands, update the affected SM, INV, PAR, US, EC, FM, DR, TM, XR and AR items.
Record a new immutable implementation pin and the exercised behavior evidence.
Remove stale design-only claims only after that evidence exists.
Keep the old snapshot claims identifiable rather than silently relabeling their line numbers.
