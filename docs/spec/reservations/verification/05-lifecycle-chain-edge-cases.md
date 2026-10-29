# Lifecycle, chain and concurrency boundaries

Evidence pins and vocabulary: [edge cases index](05-edge-cases.md).

### EC-11 Wallet state change mid-action

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

Expected: a wallet that leaves Live while an action is in flight never
leaves a newly settled anchor unmanaged.

Actual: after every acceptance or re-anchor settlement, the code strands
the position when the custodying or target wallet is Closed or Terminated
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:234-253`,
called at `:608` and `:772`); an on-time proof races a wallet leaving Live
and strands the same way. A Closing target is deliberately **not** stranded:
the wallet still has its signers, the owner keeps the re-anchor right that
`requestReservationReanchor` grants from a Closing source, and stranding at
settlement would have removed that right roughly a reservation term before
`notifyReservationStranded` allows it
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:214-233`).
Permissionless stranding covers the remaining states: Terminated and Closed
always, a Live wallet past `dissolutionEligibleAt`, and a Closing wallet past
both `dissolutionEligibleAt` and the closing period
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Reservation.sol:1311-1322`).
Moving funds into a wallet still custodying a proven anchor routes it to
MovingFunds instead of Closing
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Wallets.sol:629-631`), and the
wallet cannot finalize closing while its reserved reservation count is
non-zero (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Wallets.sol:392-395`).

Test: `tbtc-v2@4c98e6b82:solidity/test/bridge/ReservationProofs.test.ts:1308`
(a Closed wallet strands on an on-time proof) and
`tbtc-v2@4c98e6b82:solidity/test/bridge/ReservationProofs.test.ts:1434`
("should not strand on late settlement if wallet is Closing (3)");
`tbtc-v2@4c98e6b82:solidity/test/bridge/Reservation.test.ts:2007` (a proven
anchor keeps the Live wallet in MovingFunds).

Status: `holds` at `tbtc-v2@4c98e6b82` for the corrected rule; the Closing
target and the Live/pending-only routing recorded before VF-10 and VF-11 do
not match the published code.

Status: holds.

### EC-12 Ethereum reorg

Expected: shallow unprocessed reorgs should not produce premature event processing.
Recovery from a deeper processed-history reorg needs a separate cursor/state policy.

Actual: the shared scan range stays 12 blocks behind the head
(`keep-core:pkg/maintainer/spv/reservation_event_scan.go:21`).
The proof loop retains a scan cursor and retries chunks.
These facts do not prove rewind after a reorg deeper than that lag.
If Ethereum unmines a settlement transaction, its state writes and logs are undone.
A replay rejects as already settled only when the canonical chain still contains a settlement.
Generation and spent-outpoint guards protect canonical-state replay
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:192-210`, `:292-294`).
No deep-reorg scenario was executed for this specification.

Test: `keep-core` `TestReservationScanRange`
(`pkg/maintainer/spv/reservation_event_scan_test.go:306`);
`TestRunReservationProofLoop_ResumesScanAfterRestart`
(`pkg/maintainer/spv/reservation_proof_loop_test.go:2421`).

Status: holds-untested for processed-history reorg recovery.

### EC-13 Bitcoin reorg

Expected: proof submission should use a currently confirmed Bitcoin candidate.
A later deep Bitcoin reorg is an explicit security assumption, not a guaranteed recovery path.

Actual: the proof loop checks the candidate's confirmation threshold
(`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:1374`).
It can prefer a confirmed RBF replacement over an unconfirmed candidate (:1316).
The contract verifies header linkage, difficulty and accumulated work.
`txProofDifficultyFactor` is a work requirement, not simply a fixed header count.
A proof accepted on Ethereum does not automatically reverse when Bitcoin later reorganizes.

Test: `keep-core`
`TestProveReservationTransaction_SelectsConfirmedRBFCandidate`
(`pkg/maintainer/spv/reservation_proof_loop_test.go:1272`).

Status: holds-untested for deep post-settlement reorg recovery; the named test covers candidate selection.

### EC-14 Proofs for superseded generations

Expected: only `Pending` or `TimedOut` generations settle; a
generation whose anchor was consumed by a late settlement of an older
generation is terminally `Superseded` and its proof reverts.

Actual: `loadSettleableAction` accepts only those two states
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:193`); a
late re-anchor settlement unwinds the newer pending generation into
`Superseded`, releasing its capacity and third-party hold
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:832`,
`:849`); `Vetoed` and `Superseded` proofs revert
(`test/bridge/ReservationProofs.test.ts:334`, `:353`). A timed-out
generation whose anchor outpoint a later settled generation consumed
reverts on the spent-anchor latch
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:292`).

Test: `test/bridge/ReservationProofs.test.ts:1072` ("should unwind
pending reanchor action"); `test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:412`
("rejects a timed-out re-anchor replay against a later anchor").

Status: holds.

### EC-15 Same UTXO used twice

Expected: no outpoint is credited or consumed twice.

Actual: acceptance consumes the revealed deposit exactly once
(`deposit.sweptAt` set at
`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:422`, guard
at `:419`); a second proof of the same deposit reverts on the swept
guard. A re-anchor marks the consumed anchor in `spentMainUTXOs` and
deletes the reverse index entry
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:922`), and
a late settlement of an already-stranded position refuses to restore
an anchor another generation already consumed
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:292`). The
spent-anchor mark also makes the spend recognized by the fraud
challenge defeat path
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:918`).

Test: `test/bridge/ReservationProofs.test.ts:805` ("should revert on
double-consume (deposit already swept)");
`test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:412`.

Status: holds.

### EC-16 P2PKH vs P2WPKH anchor outputs

Expected: the on-chain validator accepts both lock types paying the
authorized wallet; the wallet protocol is P2WPKH.

Actual: `extractPubKeyHash` accepts a 26-byte P2PKH or a 23-byte P2WPKH
script and compares against the expected script for the authorized
pubkeyhash (`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:354`);
both settlement paths use it
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:449`,
`:677`). A P2SH, bare, or other script reverts
("Output must be P2PKH or P2WPKH",
`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:360`). The client
assembles P2WPKH outputs only
(`keep-core:pkg/tbtc/reservation.go:439`, `:493`). So a wallet that
broadcasts a P2PKH anchor for its own key is accepted on-chain even
though no operator tooling produces one.

Test: `test/bridge/ReservationProofs.test.ts:659` ("should accept
P2WPKH anchor output vector format");
`keep-core` `TestAssembleReservationTransactions_HappyPathShape`
(`pkg/tbtc/reservation_test.go:523`).

Status: holds.

### EC-17 Activation block equal to the current block

Expected: the client gate permits a work block equal to B.
Actual execution still follows the coordination cadence and operator readiness.

Actual: the activation comparison is inclusive:
`coordinationBlock >= ReservationsActivationBlock(network)`
(`keep-core:pkg/tbtc/deposit_sweep.go:233`).
A coordination window at B qualifies; if B is between windows, work starts at a later window.
Leader and followers agree only when their builds contain the same network entry.
A rolling upgrade with mismatched activation tables does not ensure agreement.
Scan ranges begin at the configured B and stay behind the tip
(`keep-core:pkg/maintainer/spv/reservation_event_scan.go:43`).
An unconfigured network instead returns `math.MaxUint64`
(`keep-core:pkg/tbtc/coordination.go:119`).

Test: `keep-core` `TestReservationScanRange`
(`pkg/maintainer/spv/reservation_event_scan_test.go:306`); the
inclusive comparison itself has no dedicated test.

Status: holds-untested (inclusive comparison at exactly B not pinned by
a test).

### EC-18 Last slot race on `maxActiveReservations`

Expected: the last slot can be taken once; a lost race reverts; a late
settlement may exceed the cap because caps are request-time throttles.

Actual: the check `activeReservationsCount < maxActiveReservations`
runs before the increment
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1005`), so two
requests are evaluated in transaction order. With one slot remaining,
the first takes it and the second fails. At the cap, new requests revert with
"Active reservations cap exceeded"
(`test/bridge/Reservation.test.ts:709`). A timed-out generation
released its slot, and a late acceptance settlement re-takes the slot
without re-checking the cap
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:533`), so
the counter can sit at the cap plus late settlements; the unwound
generation's release restores balance
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:832`). The
occupancy change is observable from logs:
`ReservationOccupancyChanged`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1009`).

At `tbtc-v2@806b1106f` the acceptance request also passes the hand-off
reserve gate after the occupancy check
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1057-1061`, then
`:1068-1077`; revert "Hand-off reserve exceeded" on
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`): the new count must satisfy
`P + 1 <= max(0, L - R) * N`. Both checks run at the request only; a late
settlement still re-takes the slot with no check
(`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationProofs.sol:533`). Test:
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` (reverts past
`(L - R) * N` with max-active room), `:346`. Sizing: PAR-X4.

Test: `test/bridge/Bridge.ReservationOccupancy.test.ts:30` (request
increments), `:101` (timeout decrements);
`test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:604`
("releases capacity, allows re-authorization, and still settles the
late anchor").

Status: holds.

### EC-19 Concurrent acceptance and re-anchor on the same wallet

Expected: two requesters racing the target wallet's capacity are
serialized by the chain; the loser reverts and the client recovers by
trying the next target.

Actual: every request reserves the target's count and amount in the
same transaction as the capacity check
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:881`, `:902`;
acceptance at `:1019`), so a second request in the same or a later
block sees the raised counters and reverts with "cap exceeded".
Because the check and the reserve are one atomic write, no lost update
exists. The client detects exactly these reverts by the "cap
exceeded" suffix and excludes the target instead of failing the pass
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:614`, loop recovery at
`:559`).

At `tbtc-v2@806b1106f` this recovery applies to third-party requests and to
governance rotation of a Live source. A privileged re-anchor from a
non-Live source skips the target's count and amount cap checks while still
adding to the counters
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`, `:928-935`, `:942-950`),
so it does not revert with "cap exceeded" and the target can end above its
cap. The client falls back to the least-loaded eligible Live wallet only for
privileged member requests
(`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1153-1241`). Test:
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`
(owner, MovingFunds, full target succeeds), `:466` (third party still
reverts), `:485` (governance from a Live source still reverts);
`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1479`.

Test: `keep-core` `TestReservationReanchorTask_CapRevertLeadsToNextCandidate`
(`pkg/tbtcpg/reservation_reanchor_test.go:996`);
`TestReservationReanchorTask_AmountCapFillLeadsToNextCandidate`
(`pkg/tbtcpg/reservation_reanchor_test.go:1137`).

Status: holds.

### EC-20 Vault deactivated mid-flight

Expected: a settled generation credits where the deposit was routed,
even if governance revoked the vault between the request and the
proof.

Actual: settlement credits the deposit's immutable `vault` only while
`isVaultTrusted[vault]` is true; otherwise it credits the depositor's
Bank balance directly
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:620`). All
position state is finalized before that external call, so a hostile or
buggy vault cannot observe a half-settled generation
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:613`).
The re-anchor path finances the in-kind miner fee through
`IReservationFeeFinancer(vault).financeInKindFee` unconditionally at
the end of settlement, with the checks-effects-interactions order
guaranteed by the comment
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:771`).
Governance cannot re-point the vault while reservations or pending
deposits exist
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409`).
The fee-bearing settlement makes an uncaught
`IReservationFeeFinancer(self.deposits[reservationKey].vault).financeInKindFee`
call on the deposit's stored vault
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:785-787`),
so a non-conforming deposit vault makes settlement revert. This is scoped to
governance miswiring: it arises only if governance wires a vault that does not
implement `financeInKindFee`.

Test: `test/bridge/ReservationProofs.test.ts:1757` ("should credit the
deposit's immutable vault, not a reservationVault repointed
mid-flight").

Status: holds.

### EC-21 `liveWalletsCount` falling to zero

Expected: at the original pins reservations imposed no gate on the live wallet count; the
pre-existing moving-funds machinery degrades gracefully. At `806b1106f` only the acceptance request reads it.

Actual at `9d2086965`: no reservation check reads `liveWalletsCount`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol`, no occurrence);
its only readers are the moving-funds target count
(`tbtc-v2:solidity/contracts/bridge/MovingFunds.sol:214`), which
reverts the commitment with "No target wallets available" at zero
(`tbtc-v2:solidity/contracts/bridge/MovingFunds.sol:222`), and the
timeout reset, gated on exactly zero
(`tbtc-v2:solidity/contracts/bridge/MovingFunds.sol:295`). A wallet
with anchors cannot close while custodied
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:384`), so the
re-anchor target space is also empty: no Live target exists for
`requestReservationReanchor`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:815`), and the
client re-anchor task waits on an aged target, publishing the gauge
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:597`).

At `tbtc-v2@806b1106f` the acceptance reserve gate reads the count as L
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1068`) and requires
`P + 1 <= max(0, L - R) * N`
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`), so `L <= R`,
including `L = 0`, rejects every new acceptance with "Hand-off reserve exceeded"
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`; test
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379`). The
re-anchor request, proofs and settlement still do not read it
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1068` is the only
reader in the library), and the moving-funds readers are unchanged.

Test: `test/bridge/Bridge.MovingFunds.test.ts:455` ("No target wallets
available"), `:942` ("Live wallets count must be zero");
`test/bridge/Reservation.test.ts:1831`.

Status: holds.

### EC-22 Deployment progress files and chain identity

Expected: a resumed live prepare/activate run must use the intended network,
Bridge and deployments. Local progress data is not chain authority.

Actual at the rehearsal snapshot: `loadPrepareArtifact` parses JSON and casts it
to `PrepareArtifact`, then selects by filename and modification time
(`tbtc-v2@52f4f3005:solidity/scripts/reservations-sepolia-rehearsal.ts:362-385`).
The script's on-chain comparisons remain important; the cast is not runtime schema validation.
This task did not demonstrate a bypass of those comparisons.

The Hardhat Sepolia configuration already states chain ID 11155111;
the explicit rehearsal configuration states 31337
(`tbtc-v2@52f4f3005:solidity/hardhat.config.ts:178-183`, `:208-214`).
Do not claim that the original configuration omitted chain identity.
Approved hardening: the M1 release decision walk (item #2, 2026-10-12) approved
validating the prepare JSON and binding it to the provider chain and Bridge.
The guard is merged at the published pin `tbtc-v2@4c98e6b82`.
`chainPreflight` refuses every transaction unless the provider chain matches the
execution mode (`tbtc-v2@4c98e6b82:solidity/scripts/prepare-artifact-guard.ts:397-446`);
the rehearsal script calls it before any signer is used
(`tbtc-v2@4c98e6b82:solidity/scripts/reservations-sepolia-rehearsal.ts:1290`).
`selectPrepareArtifact` rejects a malformed artifact instead of skipping it
(`tbtc-v2@4c98e6b82:solidity/scripts/prepare-artifact-guard.ts:310-317`, returning
`kind: "rejected"`), and a live artifact must record a chain ID and Bridge that
match the run (`bindingMismatch`, `tbtc-v2@4c98e6b82:solidity/scripts/prepare-artifact-guard.ts:353-378`).
History: the earlier pin `52f4f3005` had no guard, and `34160a556` carried the
first version on a branch that then had no pin in this set.

Test: `tbtc-v2@4c98e6b82:solidity/test/prepare-artifact-guard.test.ts` covers the
malformed-artifact rejection, the chain preflight and the binding check; this
documentation task did not run it. No real-key wrong-chain run was performed, and
no test pins a tampered on-chain deployment reference.

Status: `holds` at `tbtc-v2@4c98e6b82` for the inspected guard behaviour (the
malformed-progress-JSON rejection, the provider-chain preflight and the
chain-ID/Bridge binding check). The earlier revision of this entry recorded no
verdict, because the guard had not merged at the pins this chapter otherwise
cites.

### EC-23 D28 wallets registered before reservation activation

Expected: historical registrations must contribute both retiring sources and usable re-anchor targets.
Reservation activation must not hide wallets that already exist.

At `9c4e1b221`, registration discovery starts at block 0, uses its own cursor and scans in bounded chunks
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:48-82`).
Acceptance requests and deposit reveals still start at reservation activation.
An old retiring source without a target raises the no-target count.
An old eligible target prevents a false no-target count.

Test: `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_wallet_discovery_test.go:12-73`.
Both tests failed on the earlier client; the real fork reproduced the missed alert and confirmed its repair
(`keeper-alert-acceptance.md:67-90`).

Status: holds at `9c4e1b221`; the earlier `d8fce4707` observer missed historical wallets.

### EC-24 D28 observation freshness and catch-up windows

Expected: a catch-up scan must compare keeper time with the latest chain head, not its older scanned window.
Exactly the maximum timestamp deviation is fresh in either direction.
One second beyond it produces a Critical health event.

Implementation: the entry point supplies latest chain time
(`tbtc-v2@8eb26c2e2:monitoring/src/index.ts:75-80`).
The monitor uses that time and checks positive/negative deviation
(`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:98-103`,
`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:192-225`).
Missing timestamp, invalid risk samples and unavailable endpoint also produce a health event.
Positive risk events may accompany that event; stale readings are not a healthy control.

Test: equality, one-second excess, catch-up and RPC skew
(`tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:259-328`);
missing/malformed samples and endpoint failure
(`tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:243-256`,
`tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:330-351`).
The paused-keeper run delivered the health event through the actual monitor CLI
(`keeper-alert-acceptance.md:92-102`).

Status: holds at `8eb26c2e2` for these boundaries, not for live operator availability.
