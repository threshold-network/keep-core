# Keep Core - security invariants (KC-)

This set registers the keep-core security properties that have or need checkers:
the WalletRegistry DKG result lifecycle, slashing and seize, the RandomBeacon
relay entry and timeouts, threshold ECDSA signing, the wallet lifecycle, and
the Halmos-checked properties of the two Solidity packages. It follows the
Engineering Verification Standard section 6 layout only in part (SPEC-1); the
gaps are listed under "Open work" below. The existing M1 UTXO Reservations
registry (`docs/spec/reservations/verification`) keeps its
INV/SM/PAR/US/EC/FM/DR/TM/XR/AR ID space; this set adds the `KC-` space. A
check that does not carry its `KC-` ID in its name or in a structured
annotation does not count for coverage (SPEC-2); no existing check carries one
yet. The two-digit `KC-` labels in the Halmos READMEs and harness comments are
earlier candidate labels, not registry IDs.

Fields per entry: `Statement`, `Why`, `Enforcing code` (scope), `Checkers`
(kind, location, status), `Critical`, `Exposure`, `Fails on`, `Status`. `Why`,
`Enforcing code` and `Status` are local fields; the section 6 fields `formal`,
`sources` and a per-entry `tier` and `bounds` are not recorded (the bounds are
in the closing "Halmos bounds" section).

**Status legend** (same as the reservations set): `holds`, `holds-untested`,
`violated`, `design-only`, `unknown`. Here `holds` means the code was read
against the statement and no violation is known; it does not mean that a
checker counts as coverage (see `Fails on`).

**Checker status values**: `passing` (ran green in CI of its own source
closure; a Halmos proof is marked `harness not yet published` until its harness is in
this repository), `deferred` (listed in `solidity/ecdsa/test/halmos/heavy.json`: the PR
slice skips it and counts it as a pass; it runs in the scheduled heavy run,
weekly on Wednesday at 06:30 UTC through `.github/workflows/security-tools.yml`
and `heavy-proofs.yml`, where an unknown is a failure), `unknown` (never
completed within its budgets), `missing` (no checker exists).

**Tier and Critical rule.** This repository is T0 (`AGENTS.md`). `Critical` is
yes when the invariant guards mint, burn or key use, or when its exposure is
USD 1M or more (MON-1). Local reading, for the owner to confirm: key creation
(DKG) counts as key use, and slashing or seizure of stake counts as a guarded
fund flow. The `Exposure` line gives the reason and the bound. No entry gives
a USD estimate: the funds held by one wallet are limited by Bridge wallet
parameters that live outside this repository.

**Open work (issue #586).**

- `Fails on` is empty in all 13 entries. Under SPEC-3 no checker counts as
  coverage until the MUT-2 run records the mutants or past bugs it caught.
- No critical entry has a MON-1 production monitor yet.
- SPEC-1 coverage is partial. Not registered yet: the WalletRegistry
  governance and parameter setters, `notifyOperatorInactivity`,
  `notifySeedTimeout`, `notifyDkgTimeout`, `withdrawRewards`; in
  `solidity/random-beacon` the beacon DKG, `reportRelayEntryTimeout`,
  `reportUnauthorizedSigning`, `notifyOperatorInactivity`, governance, groups
  and `ReimbursementPool`; in Go the BLS relay-entry signing (`pkg/beacon`),
  `pkg/tecdsa/dkg` and the `pkg/tbtc` signing, coordination and heartbeat
  loops. No entry is derived from a known defect yet.
- `traceability.py --check` is a presence gate: a critical entry must list
  real `Checkers` text. It does not check that Solidity or TypeScript checks
  exist, that a deferred proof is in `heavy.json`, or that a monitor or a
  `Fails on` record exists.

---

## 1. DKG result validity (WalletRegistry submit, challenge, approve)

### KC-101 Only the committed DKG result can be approved; a changed or replayed result cannot approve or replace the commitment

- **Statement.** After `submitDkgResult` the only result that can be approved
  or challenged is the exact result whose hash was committed; a changed result
  cannot be approved, a replayed submission with a different members hash
  cannot replace the commitment, and a failed public DKG entry point changes
  nothing. The validity of the committed result is decided by challenge
  (KC-102), not at submission.
- **Why.** Approving a substituted DKG result would register a wallet whose
  signing key the submitter does not control, or that a hostile result claims;
  the wallet then signs on chain for funds the submitter can spend.
- **Enforcing code.** `WalletRegistry.submitDkgResult` stores the result hash
  and moves the DKG to CHALLENGE (`solidity/ecdsa/contracts/WalletRegistry.sol:862-864`);
  `approveDkgResult` and `challengeDkgResult` compare against the stored
  commitment (`EcdsaDkg.approveResult` :327, `EcdsaDkg.challengeResult` :388 in
  `solidity/ecdsa/contracts/libraries/EcdsaDkg.sol`, "Result under approval is
  different than the submitted one"); submission in CHALLENGE reverts.
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepApproval.t.sol` `check_publicApprovalCommitmentOrRollback` (:175), status: deferred; harness not yet published.
  - proof, same file `check_changedResultCannotApprove` (:202), status: deferred; harness not yet published.
  - proof, same file `check_submissionReplayCannotReplaceCommitment` (:218), status: passing in the PR slice. Harness not yet published.
  - property, `solidity/ecdsa/test/DKGValidator.test.ts` (validate, validateFields, validateSignatures), status: passing in `contracts-build-and-test`.
  - property, `solidity/ecdsa/test/WalletRegistry.WalletCreation.test.ts` (requestNewWallet, submitDkgResult paths), status: passing.
- **Critical.** yes - it guards key creation: approving a substituted result
  creates a wallet whose key the submitter does not control (MON-1).
- **Exposure.** Not estimated in USD. The funds that pass through a mis-created
  wallet depend on the Bridge wallet parameters, which live outside this
  repository; the binding reason is the key-creation guard (MON-1), not the
  amount.
- **Fails on.**
- **Status.** holds.

### KC-102 A challenge of an invalid DKG result has the exact cleanup effects or rolls back completely

- **Statement.** A public challenge that the real validator rejects (returns
  false or reverts) either completes the documented cleanup - the submitted
  result cleared, the DKG back in submission, the submitter seized for the
  configured amount (the seize may fail silently) - or leaves the whole
  observed state unchanged. A challenge of a changed result, or after the
  challenge period, reverts.
- **Why.** A challenge that partially mutates state can strand the DKG round,
  slash the wrong member, or let a malicious submitter get away unpenalized.
- **Enforcing code.** `WalletRegistry.challengeDkgResult`
  (`solidity/ecdsa/contracts/WalletRegistry.sol:956-1037`): `dkg.challengeResult`
  returns the malicious hash and submitter; the staking seize is attempted in a
  try/catch so the challenge completes even when slashing fails silently. The
  comment at `WalletRegistry.sol:973-990` records that after TIP-100
  `TokenStaking.seize()` only emits an event and economic enforcement is by
  governance. The challenge does not ban members from rewards; that happens in
  `approveDkgResult`.
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepChallenge.t.sol` `check_publicChallengeFalseEffectsOrRollback` (:232), status: deferred; harness not yet published.
  - proof, same file `check_publicChallengeValidatorRevertEffectsOrRollback` (:239), status: deferred; harness not yet published.
  - proof, same file `check_changedChallengeCommitmentAndDeadline` (:264), status: deferred (changed result and late challenge revert). Harness not yet published.
- **Critical.** yes - it guards key creation: a result that is not valid must
  be removable and its submitter penalized before a wallet is registered
  (MON-1, key creation read as key use).
- **Exposure.** The slash default is 400 T (`WalletRegistry.sol:376-382`
  initializer comment); the binding reason is the key-creation guard, not the
  amount.
- **Fails on.**
- **Status.** holds.

### KC-103 DKG seed authority: only the configured beacon can deliver the seed, once; the request locks the pool and the parameters freeze while a round is in progress

- **Statement.** `requestNewWallet` is callable only by the wallet owner,
  locks the pool before the beacon call and rolls back on a beacon revert;
  only the configured RandomBeacon callback can deliver the seed, and a second
  delivery reverts with `Current state is not AWAITING_SEED`; once a result is
  submitted, a later seed cannot replace it and the owner cannot restart the
  round; while the pool is locked, DKG parameters are frozen and a join or a
  status update of an operator already in the pool is rejected. The proofs
  exclude `address(0)` and the proxy admin as callers.
- **Why.** Seed authority is the trust boundary between the beacon relay and
  the DKG: anyone who can deliver or replace a seed controls the key the new
  wallet generates, and an unlocked pool during a round lets membership move
  under the ceremony.
- **Enforcing code.** `WalletRegistry.requestNewWallet`
  (`solidity/ecdsa/contracts/WalletRegistry.sol:818-821`, locks the DKG state
  and calls `randomBeacon.requestRelayEntry`); the `__beaconCallback` entry
  point (`WalletRegistry.sol:834-842`, reverts for every caller that is not
  the configured beacon); `EcdsaDkg.start` and the DKG state machine
  (`solidity/ecdsa/contracts/libraries/EcdsaDkg.sol:227`); the `EcdsaDkg`
  parameter setters (`EcdsaDkg.sol:467-533`).
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepDkgSeed.t.sol` `check_nonOwnerCannotRequest` (:171), passing; harness not yet published.
  - proof, same file `check_requestLocksBeforeBeaconAndRollsBack` (:180), passing; harness not yet published.
  - proof, same file `check_authenticatedCallbackIsSingleUse` (:201), passing; harness not yet published.
  - proof, same file `check_callbackCannotReplaceSubmittedSeed` (:216), passing; harness not yet published.
  - proof, same file `check_walletRequestCannotRestartRound` (:225), passing; harness not yet published.
  - proof, same file `check_lockedPoolRejectsChanges` (:233), status: deferred; harness not yet published.
  - proof, same file `check_dkgParametersFrozen` (:248), passing; harness not yet published.
  - proof, same file `check_deadlinePredicates` (:256), passing; harness not yet published.
  - proof, same file `check_lateAuthenticatedSeedStarts` (:267), passing (characterization: a late authenticated seed still starts DKG if no timeout was reported). Harness not yet published.
- **Critical.** yes - it guards key creation: seed authority is what decides
  the randomness every wallet key is derived from.
- **Exposure.** Same as KC-101: a captured seed authority affects every wallet
  created while it is live. Not estimated in USD; critical on key creation.
- **Fails on.**
- **Status.** holds.

### KC-104 The members hash of a 100-member DKG result is exactly the hash of the 99 non-excluded members in order

- **Statement.** `EcdsaDkgValidator.validateMembersHash` completes if and only
  if the exclusion is in 1..100, and then returns whether the declared hash
  equals the hash of the remaining 99 member IDs in their array order; an
  out-of-range exclusion reverts with a panic.
- **Why.** The members hash is what the registry commits to when it registers
  a wallet and what the authorization checks later; a wrong hash binds the
  wallet to the wrong operator set.
- **Enforcing code.** `EcdsaDkgValidator.validateMembersHash` in
  `solidity/ecdsa/contracts/EcdsaDkgValidator.sol` (called from
  `EcdsaDkgValidator.validate`, which `EcdsaDkg.challengeResult` and the
  `isResultValid` view reach; `submitDkgResult` does not call it).
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/dkg-members-hash/MembersHashSupport.t.sol` `check_actualMembersHash100Singleton` (:80), status: deferred; harness not yet published.
  - property, `solidity/ecdsa/test/DKGValidator.test.ts` (`validate`, case "Invalid members hash"), passing.
- **Critical.** yes - it guards key creation: the members hash defines which
  operators hold the key that the wallet is registered under.
- **Exposure.** Same as KC-101.
- **Fails on.**
- **Status.** holds.

## 2. Slashing and seize

### KC-201 A late relay entry slashes every member of the serving group in proportion to the delay, and no earlier entry slashes

- **Statement.** `submitRelayEntry` slashes all group members only when the
  entry arrives after the soft timeout, by an amount proportional to the delay
  past the soft timeout that reaches the configured `slashingAmount` at the
  hard timeout; an entry before the soft timeout slashes no one. The relay
  state machine reaches the soft-timeout branch only at its documented block.
  The proofs cover the zero side (no slash up to the soft timeout, the entry
  gates); the amount and its upper bound are not proved
  (`solidity/random-beacon/test/halmos/README.md`).
- **Why.** Slashing is the economic teeth of the beacon relay: a wrong amount
  either lets a malicious relay group escape the penalty or burns stake of an
  honest group, and the timeout boundary is what separates the two.
- **Enforcing code.** `Relay.calculateSlashingAmount` and the entry gating in
  `solidity/random-beacon/contracts/libraries/Relay.sol` (:108-201); the
  submit and slash wiring in `RandomBeacon.submitRelayEntry`
  (`solidity/random-beacon/contracts/RandomBeacon.sol:1084-1112`).
- **Checkers.**
  - proof, `solidity/random-beacon/test/halmos/RelayStateMachine.t.sol` `check_noSlashingUpToSoftTimeout` (:76), passing; harness not yet published.
  - proof, same file `check_submitEntryNeedsLiveRequest` (:111), passing; harness not yet published.
  - proof, same file `check_submitBeforeSoftTimeoutGate` (:135), passing; harness not yet published.
  - property, `solidity/random-beacon/test/RandomBeacon.Relay.test.ts` ("should slash a correct portion of the slashing amount for all group members", "should not slash any members"), passing in `contracts-build-and-test`.
- **Critical.** yes - it guards a slash of stake (local reading of MON-1, see
  the Critical rule above).
- **Exposure.** The slash amount is a governance parameter applied to a
  bounded group; not estimated in USD. The amount alone would not make it
  critical; the slash of stake does. Whether `staking.slash` moves value or is
  symbolic like the WalletRegistry seize (KC-102) is not checked here.
- **Fails on.**
- **Status.** holds.

### KC-202 The relay timeout state machine: no retry, cleanup or new request outside the timeout, and no change on a rejected entry

- **Statement.** `requestEntry` succeeds only when idle and the request count
  is below the uint64 horizon; `retryOnEntryTimeout` and `cleanupOnEntryTimeout`
  succeed only after the request timed out, and each leaves the request count
  and entry parameters unchanged; a rejected entry changes nothing; governance
  setters (`setTimeouts`, `setSlashingAmount`) run only when idle. The proofs
  assume a well-formed timeout configuration (hard timeout above zero, sums
  within uint32 and uint64); the two corners that violate it are KC-203.
- **Why.** The request/entry state is what the slashing decision reads; a
  request that can be retried early, or parameters that change under a live
  request, moves the soft-timeout boundary and with it who gets slashed.
- **Enforcing code.** `Relay.requestEntry` (:84), `Relay.retryOnEntryTimeout`
  (:232), `Relay.cleanupOnEntryTimeout` (:254), `Relay.submitEntry` (:128) in
  `solidity/random-beacon/contracts/libraries/Relay.sol`.
- **Checkers.**
  - proof, `solidity/random-beacon/test/halmos/RelayStateMachine.t.sol` `check_requestEntryOnlyWhenIdle` (:47), passing; harness not yet published.
  - proof, same file `check_retryOnlyAfterTimeout` (:158), passing; harness not yet published.
  - proof, same file `check_cleanupOnlyAfterTimeout` (:186), passing; harness not yet published.
  - proof, same file `check_setTimeoutsOnlyWhenIdle` (:241), passing; harness not yet published.
  - proof, same file `check_setSlashingAmountOnlyWhenIdle` (:265), passing; harness not yet published.
- **Critical.** yes - it guards the slash of stake: it decides the boundary
  that KC-201 reads.
- **Exposure.** Same bound as KC-201.
- **Fails on.**
- **Status.** holds.

### KC-203 The two relay timeout characterizations: a zero hard timeout, and an overflowed timeout sum

- **Statement.** Governance accepts a zero hard timeout only while idle; with
  a zero hard timeout `calculateSlashingAmount` reverts with a panic after the
  soft timeout block, so `submitEntry` reverts and a late entry cannot be
  accepted. If the soft + hard timeout sum overflows uint32, the request can
  be neither retried nor cleaned up and no new request can start.
- **Why.** These are the two parameter corners that break the KC-202 machine
  silently; a monitor watching only the normal paths never sees them.
- **Enforcing code.** `Relay.calculateSlashingAmount` arithmetic and the
  request gating in `solidity/random-beacon/contracts/libraries/Relay.sol`.
- **Checkers.**
  - proof, `solidity/random-beacon/test/halmos/RelayStateMachine.t.sol` `check_zeroHardTimeoutPanicsAfterSoftTimeout` (:90), passing (characterization); harness not yet published.
  - proof, same file `check_timeoutSumOverflowBlocksRecovery` (:213), passing (characterization); harness not yet published.
- **Critical.** no - characterization of corners of the KC-202 machine; the
  normal invariant is covered by KC-202.
- **Exposure.** Not estimated in USD; a mis-set timeout costs one round of
  relay reward, not custodied funds.
- **Fails on.**
- **Status.** holds.

### KC-204 Malicious DKG result slashing seizes from the submitter only, and the governance seize is guarded

- **Statement.** When a malicious DKG result is challenged, the configured
  slash (default 400 T, 1% of the minimum authorization) is seized only from
  the submitting operator; the governance `seize` entry point is callable
  only by the wallet owner and seizes only from the members of a registered
  wallet whose member IDs hash to the stored members hash.
- **Why.** Slashing the wrong operator, or an unguarded `seize`, destroys
  stake that the protocol did not authorize and hands out a reward that was
  not earned.
- **Enforcing code.** `WalletRegistry.challengeDkgResult` seize call
  (`solidity/ecdsa/contracts/WalletRegistry.sol:995-1022`);
  `WalletRegistry.seize` (:1152-1175); in the beacon, the `RandomBeacon.sol`
  `staking.seize` calls for a malicious DKG result (:935), a relay entry
  timeout (:1157) and unauthorized signing (:1250). After TIP-100 the
  WalletRegistry call is event-only (comment at `WalletRegistry.sol:973-990`);
  whether the beacon's calls are also symbolic is not checked here.
- **Checkers.**
  - property, `solidity/ecdsa/test/WalletRegistry.Slashing.test.ts` (`seize`: non-owner revert, invalid wallet members identifiers revert), passing in `contracts-build-and-test`. The cases for valid identifiers (slash amounts, providers, notifier reward) are skipped by `context.skip` (:128) and do not count.
  - property, `solidity/random-beacon/test/RandomBeacon.Relay.test.ts` (`TokensSeized` cases at :527, :688, :829, :1015), passing in `contracts-build-and-test`.
- **Critical.** yes - it guards the slash and seize of stake (local reading of
  MON-1, see the Critical rule above).
- **Exposure.** Not estimated in USD: a fixed parameter times a bounded
  provider set. The reason for critical is the seizure, not the amount.
- **Fails on.**
- **Status.** holds.

## 3. Threshold ECDSA signing

### KC-301 A tECDSA signing session produces a signature that verifies under the group public key, or none

- **Statement.** A signing session for a message completes only when the
  round protocol reaches its acceptance state; the signature it produces
  verifies under the group public key; a session that does not reach the
  threshold of honest signatures produces no signature; a corrupted or missing
  message is detected and fails the round instead of producing a bad
  signature.
- **Why.** A signature is the unit of funds movement: a wrong signature moves
  custodied funds, and two different signatures for the same nonce material
  destroy the key itself.
- **Enforcing code.** `Execute` and the round protocol in
  `pkg/tecdsa/signing/signing.go` and `pkg/tecdsa/signing/protocol.go` (share
  validation, finalize). The `pkg/tbtc` signing orchestration is not
  registered yet (see Open work).
- **Checkers.**
  - property, `pkg/tecdsa/signing/protocol_test.go` `TestTssFinalize` (:2203, verifies the signature with `ecdsa.Verify`), `TestTssFinalize_IncomingMessageCorrupted_WrongPayload` (:2296), `TestTssFinalize_IncomingMessageMissing` (:2336), `TestTssRoundOne` (:343), `TestTssRoundTwo` (:468), `TestTssRoundOne_OutgoingMessageTimeout` (:401), `TestTssRoundTwo_IncomingMessageCorrupted_WrongBroadcastPayload` (:533), `TestTssRoundTwo_IncomingMessageCorrupted_UndecryptablePeerPayload` (:573), `TestTssRoundTwo_IncomingMessageCorrupted_WrongPeerPayload` (:618), passing in `client-build-test-publish`.
- **Critical.** yes - it guards key use: the signature is the key's only
  output on chain.
- **Exposure.** The funds held by the signing wallet, limited by the Bridge
  wallet parameters (outside this repository). Not estimated in USD; critical
  on key use.
- **Fails on.**
- **Status.** holds.

### KC-302 Every tECDSA signing wire message round-trips and fuzzes clean

- **Statement.** Every ephemeral-public-key message and every
  `tssRoundOneMessage` to `tssRoundNineMessage` marshals to and back to the
  same value, and the protobuf unmarshalers reject out-of-domain bytes without
  panicking.
- **Why.** A share that unmarshals to a wrong value is a wrong signature
  input; a panic in the unmarshaler stops a node during a signing round.
- **Enforcing code.** The message types in `pkg/tecdsa/signing/message.go`
  and their generated protobuf in `pkg/tecdsa/signing/gen/pb/`.
- **Checkers.**
  - property, `pkg/tecdsa/signing/marshaling_test.go` (per-message roundtrip tests, `TestFuzzEphemeralPublicKeyMessage_Unmarshaler` and `TestFuzzTssRoundOneMessage_Unmarshaler` through `TestFuzzTssRoundNineMessage_Unmarshaler`), passing.
- **Critical.** no - it guards message integrity, not funds directly; the
  exposure is the signing session it feeds, bounded by the KC-301 bound.
- **Exposure.** Indirect via KC-301; a corrupted share costs one signing
  round, the session retries, and the custodied amount is the same bound as
  KC-301.
- **Fails on.**
- **Status.** holds.

## 4. Wallet lifecycle

### KC-401 A registered wallet keeps its approved identity and members hash; the owner can close it and a closed wallet is rejected

- **Statement.** After a real approval, the registered wallet has the approved
  group public key and members hash, membership checks use it, the wallet
  owner can close the wallet, a closed wallet is rejected by the close and
  reuse paths, and a fresh DKG submission can follow.
- **Why.** A wallet that loses its identity or that can be re-opened, or that
  can host two keys under one ID, can sign as a key that was never approved.
- **Enforcing code.** `wallets.addWallet`, close and reuse paths in
  `solidity/ecdsa/contracts/libraries/Wallets.sol`, called from
  `WalletRegistry.approveDkgResult` (:881-882) and closed by
  `WalletRegistry.closeWallet` (:825-829).
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepWalletLifecycle.t.sol` `check_registeredWalletIdentityAndMembership` (:199), status: deferred; harness not yet published.
  - proof, same file `check_walletOwnerClosureAndReuse` (:253), status: deferred; harness not yet published.
  - property, `solidity/ecdsa/test/WalletRegistry.Wallets.test.ts`, passing in `contracts-build-and-test`.
- **Critical.** yes - it guards key use: a re-registered or identity-corrupted
  wallet signs with a key nobody approved.
- **Exposure.** The funds held by the affected wallet, limited by the Bridge
  wallet parameters (outside this repository). Not estimated in USD.
- **Fails on.**
- **Status.** holds.

## 5. DKG request authority and pool locking

### KC-501 Only the wallet owner can request a new wallet, and the pool is locked for the whole round

- **Statement.** `requestNewWallet` reverts for every caller that is not the
  configured wallet owner; on success it locks the pool before the beacon
  call, and a beacon revert rolls the lock back; while locked, a join or a
  status update of an operator already in the pool is rejected.
- **Why.** Membership moving under a running DKG lets an operator in who did
  not participate sign for the key, or keeps one who left signable; the pool
  lock is what freezes the operator set.
- **Enforcing code.** `WalletRegistry.requestNewWallet`
  (`solidity/ecdsa/contracts/WalletRegistry.sol:818-821`, `onlyWalletOwner`
  modifier) and the EcdsaDkg library state machine in
  `solidity/ecdsa/contracts/libraries/EcdsaDkg.sol` (`lockState` :217).
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepDkgSeed.t.sol` `check_nonOwnerCannotRequest` (:171), `check_requestLocksBeforeBeaconAndRollsBack` (:180), passing; `check_lockedPoolRejectsChanges` (:233), status: deferred; harness not yet published.
  - property, `solidity/ecdsa/test/WalletRegistry.WalletCreation.test.ts` ("should lock the sortition pool", "should revert"), passing in `contracts-build-and-test`.
- **Critical.** yes - it guards key creation: the operator set it freezes is
  who holds the key.
- **Exposure.** Same as KC-101.
- **Fails on.**
- **Status.** holds.

### KC-502 Authorization changes reach WalletRegistry only through the Allowlist owner callbacks, and roll back on a callback failure

- **Statement.** Staking-provider authorization changes are applied to
  WalletRegistry only by the Allowlist's owner callbacks; unconfigured callers
  cannot submit authorization hooks; a non-owner cannot write weights or
  finalize a decrease directly; eligibility ignores a retired authorization
  source; the real pool tracks the selected pending weight; public decrease
  finalization uses the configured source and delay; legacy routing rolls back
  on a source mismatch.
- **Why.** Authorization is what makes an operator eligible to join a signing
  group; a weight or a decrease that bypasses the Allowlist route changes who
  can be selected outside the intended authorization path.
- **Enforcing code.** `WalletRegistry` authorization state and the Allowlist
  owner callbacks in `solidity/ecdsa/contracts/WalletRegistry.sol` and
  `solidity/ecdsa/contracts/Allowlist.sol`.
- **Checkers.**
  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepAuthorization.t.sol` `check_ownerAdditionUsesCallbackAndRollsBack` (:139), `check_ownerDecreaseUsesCallbackAndRollsBack` (:163), passing; harness not yet published.
  - proof, same file `check_unconfiguredCallersCannotSubmitHooks` (:186), status: deferred; harness not yet published.
  - proof, same file `check_nonOwnerCannotWriteOrDirectlyFinalize` (:205), passing; harness not yet published.
  - proof, same file `check_eligibilityIgnoresRetiredSource` (:221), passing; harness not yet published.
  - proof, same file `check_realPoolTracksSelectedPendingWeight` (:237), passing; harness not yet published.
  - proof, same file `check_publicFinalizationUsesSourceAndDelay` (:261), status: deferred; harness not yet published.
  - proof, same file `check_legacyRoutingAndSourceMismatchRollback` (:291), passing; harness not yet published.
  - proof, same file `check_unconsumedUpgradeInitializerIsPublic` (:313), passing (characterization: the version-2 upgrade initializer is public until consumed; not an attacker-controlled-source exploit theorem). Harness not yet published.
  - property, `solidity/ecdsa/test/WalletRegistry.Authorization.test.ts` (parts of the file are `describe.skip`), `WalletRegistry.DualMode.test.ts`, passing in `contracts-build-and-test`.
- **Critical.** yes - it guards key use: authorization is what decides who
  can be in a signing group at all.
- **Exposure.** The funds of the wallets the affected operator signs for,
  limited by the Bridge wallet parameters (outside this repository). Not
  estimated in USD.
- **Fails on.**
- **Status.** holds.

---

## Halmos bounds (FORM-4)

The harnesses, workflows and settings named in this section are not in this repository
yet; they are published with the proofs, and every checker that names one carries
`harness not yet published`.

Every `check_*` above runs against the checked-out commit. PR slice:
`.github/workflows/proofs-pr.yml` (8-minute wall clock; a proof with no time
left is unknown, which fails the PR gate unless `verification-unknowns.yml`
lists it with a reason, an unexpired date and the approval label; a proof
that `heavy.json` defers counts as a pass in the slice). Scheduled run:
`.github/workflows/heavy-proofs.yml`, called weekly (Wednesday 06:30 UTC)
from `security-tools.yml`, with a larger assertion timeout (600 s by default)
and where an unknown is a failure. Both go through
`.github/scripts/halmos-proofs.py`.

Prover settings: `cvc5-int` and loop bound 2 per `halmos.toml`, with a
60-second assertion timeout in the PR slice. The keep-core harness files set
the loop bound to 16 per contract (`@custom:halmos --loop 16`);
`check_publicChallengeValidatorRevertEffectsOrRollback` and the members-hash
harness use 128; seven keep-core proofs (two in `KeepApproval`, three in
`KeepChallenge`, two in `KeepWalletLifecycle`) use plain `cvc5`. The keep-core
harnesses fix two provider/operator pairs, of which only one is a member of
the result under test; the challenge harness runs the validator with two
result shapes (no misbehaved members, or 101); the members-hash proof uses a
100-member result. The beacon state-machine proofs assume a well-formed
timeout configuration (hard timeout above zero, sums within uint32 and
uint64), so the two characterization corners are excluded from them (a check
that hits one is the characterization check itself). BLS pairing verification
is not modelled in the beacon proofs.
