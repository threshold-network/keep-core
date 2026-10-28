# Go reservation fuzz targets

- Branch: a fuzz-tooling branch (from the reservation review branch @ `fe73e9799`)
- Head: `9162e0c76`. Tests are in `59d15a614`; `9162e0c76` adds the developer run instructions.
- Worktree: `<checkout>` (removed 2026-10-15; recreate with `git worktree add <path> a fuzz-tooling branch`)
- New files: `pkg/tbtc/reservation_fuzz_test.go`, `pkg/tbtcpg/reservation_fuzz_test.go`, `pkg/maintainer/spv/reservation_fuzz_test.go`

No production source changed. Seed inputs are checked in through `f.Add`.
All retained campaigns passed, so no failing `testdata/fuzz/` input was
added. The branch contains the three test files and developer documentation.

## Targets

### pkg/tbtc (file: `pkg/tbtc/reservation_fuzz_test.go`)

Untrusted-input surface: the coordination proposal wire format that every
network peer's bytes land in.

1. `FuzzReservationAnchorProposal_Unmarshal`
   - Input: arbitrary bytes -> `ReservationAnchorProposal.Unmarshal`.
   - Invariant: no panic; any value that unmarshals must re-marshal without
     error (the same validation the marshal side enforces: positive fee
     within 8 bytes, non-zero nonce, non-zero funding tx hash) and
     unmarshal->marshal->unmarshal must yield an equal value. Justification:
     a follower that accepted a proposal must be able to carry it further
     into the coordinated transaction; a value the marshal side rejects can
     never have been produced by the marshal side, so it must not survive
     the unmarshal side.
   - Seeds: the valid example from `TestReservationProposals_MarshalingRoundtrip`
     plus an empty payload for the missing-field rejections.

2. `FuzzReservationReanchorProposal_Unmarshal`
   - Input: arbitrary bytes -> `ReservationReanchorProposal.Unmarshal`.
   - Invariant: same shape as above, plus the 32-byte EVM uint bound on
     `ReservationKey` (an oversized big-endian encoding must be rejected;
     anything accepted re-marshals and round-trips to an equal value).

3. `FuzzCoordinationMessage_Unmarshal`
   - Input: arbitrary bytes -> `coordinationMessage.Unmarshal`, which runs
     the `unmarshalCoordinationProposal` dispatch for all action types,
     including the two reservation types (wire values 6 and 8).
   - Invariant: no panic; a message that unmarshals re-marshals without
     error (its carried proposal is marshal-side-acceptable) and the
     round trip preserves sender id, coordination block, wallet public key
     hash, and the proposal's action type.

### pkg/maintainer/spv (file: `pkg/maintainer/spv/reservation_fuzz_test.go`)

Untrusted-input surface: Bitcoin transaction bytes from the Electrum backend
in the SPV reservation proof loops, plus the pure infeasibility decision.

4. `FuzzReservationAcceptanceTransactionMatcher`
   - Input: arbitrary bytes -> `bitcoin.Transaction.Deserialize` (whose
     recover() turns the btcd decoder's crafted-input panics into errors)
     -> `isMatchingReservationAcceptanceTransaction`. The event's deposit
     key is derived from the transaction's own outpoint when present, so
     the match path stays reachable under arbitrary inputs.
   - Invariant: no panic anywhere through the deserializer and matcher
     (outpoint dereference, deposit key construction, script parsing, fee
     arithmetic `int64(depositAmount) - output value`); and a matched
     transaction satisfies the matcher's own rules, recomputed here with
     the same arithmetic: it spends the event's deposit outpoint exactly
     once, pays the event's wallet, and leaves a positive fee within the
     generation's snapshotted cap.

5. `FuzzReservationReanchorTransactionMatcher`
   - Input: arbitrary bytes -> `Transaction.Deserialize` ->
     `isMatchingReservationReanchorTransaction`.
   - Invariant: no panic (fee arithmetic `int64(anchorValue) - output value`
     wraps rather than panics on negative or huge output values); a matched
     transaction spends exactly the anchor outpoint, pays the event's
     target wallet, and leaves a positive fee under the cap.

6. `FuzzReservationParamsInfeasibleReasons`
   - Input: 40-byte packing of dust threshold, reveal-ahead period,
     reservation term, and the three amount caps ->
     `reservationParamsInfeasibleReasons`.
   - Invariant: no panic; the reason count equals one entry per numerically
     violated documented rule (a non-zero reveal-ahead period above the term
     plus the 24h refund margin; each non-zero cap below the dust
     threshold), recomputed independently; and a monotonicity check: at a
     MaxUint64 dust threshold the cap-based reasons equal exactly the
     count of non-zero caps below MaxUint64, a property of the
     documented rule that holds regardless of the implementation's
     concrete comparison.

### pkg/tbtcpg (file: `pkg/tbtcpg/reservation_fuzz_test.go`)

Pure decision functions with arithmetic on fees and caps, shared by the
reservation acceptance and re-anchor tasks.

7. `FuzzApplyWalletTxFeeFloor`
   - Input: 24-byte packing of estimated fee, tx virtual size, and maximum
     total fee -> `applyWalletTxFeeFloor`.
   - Invariant: no panic (every internal int64 multiplication is behind an
     explicit checked-arithmetic guard, out-of-domain inputs error instead
     of overflowing); an error result always carries a zero fee; a success
     result never exceeds the cap (the clamp) and never drops below the
     floor total `MinWalletTxSatPerVByteFee * txVsize` (the floor-vs-cap
     guard proves the cap dominates the floor on every success, and the
     buffered rate is always at least the floor rate).

8. `FuzzEstimateReservationAcceptanceFee`
   - Input: 16-byte packing of the oracle sat/vByte rate and the
     snapshotted cap, installed on a fresh `LocalBitcoinChain` per call ->
     `estimateReservationAcceptanceFee`.
   - Invariant: no panic (the rate-to-fee multiplication wraps for
     implausible rates; the wrapped or negative result is rejected by the
     "estimated fee is less than or equal zero" guard before further
     arithmetic); an error result carries a zero fee; a success result is
     positive and never exceeds the snapshotted cap (the raw estimate is
     rejected above the cap before the floor runs, and the floor clamps
     back down to it).

9. `FuzzEstimateReservationReanchorFee`
   - Same input layout and invariants as 8, for
     `estimateReservationReanchorFee` with the re-anchor transaction's own
     fixed virtual size (one P2WPKH input, one P2WPKH output).

10. `FuzzReservationReanchorTargetHeadroom`
    - Input: 44-byte packing of the candidate's reservation count, its
      reserved amount, the anchor value, and the two governance caps ->
      `reanchorTargetSearch.pick` via a fresh `LocalChain`.
    - Invariant: no panic; in the overflow-free domain (count below 2^24,
      all amounts and caps below 2^52 so no sum wraps) the candidate is
      picked if and only if it has headroom: the count cap allows one more
      reservation (`count + 1 <= cap`) and, when the amount cap is enabled
      (non-zero), reserved amount plus anchor value fits under it (zero cap
      disables the check, mirroring the on-chain convention). The age gate
      always passes (zero `CreatedAt`, host-clock head time), so the pick
      outcome is exactly the count-and-amount headroom conjunction.

## Reproduction

All commands run in the worktree root. The host's shared Go cache
(`/dev/shm/gocache`) was full from parallel jobs, so the build used a
private cache (`GOCACHE=<gocache>`) and
`GOMAXPROCS=2` per target to respect the host's parallelism cap; these
settings do not affect the results, only the exec/s rate.

```
cd <checkout>

# vet clean on the three touched packages
export GOCACHE=<gocache>
go vet ./pkg/tbtc/ ./pkg/tbtcpg/ ./pkg/maintainer/spv/

# seeds pass
go test -run 'Fuzz' ./pkg/tbtc/ ./pkg/tbtcpg/ ./pkg/maintainer/spv/

# each target fuzzed for 60s (one example; repeat per target)
GOMAXPROCS=2 go test -run='^$' -fuzz='^FuzzReservationAnchorProposal_Unmarshal$' -fuzztime=60s ./pkg/tbtc/
```

## Observed results (copied from the fuzzer output, 2026-10-12)

All ten targets reached at least 60 seconds and reported PASS, with
1,723,870 executions in total. Baseline, total and new are interesting
corpus-entry counts, not line or branch coverage percentages.

| Target | Package | execs | baseline | total | new | result |
|---|---|---:|---:|---:|---:|---|
| FuzzReservationAnchorProposal_Unmarshal | tbtc | 160096 | 96 | 108 | 12 | PASS |
| FuzzReservationReanchorProposal_Unmarshal | tbtc | 103468 | 87 | 100 | 13 | PASS |
| FuzzCoordinationMessage_Unmarshal | tbtc | 141121 | 102 | 127 | 25 | PASS |
| FuzzReservationAcceptanceTransactionMatcher | spv | 56917 | 30 | 33 | 3 | PASS |
| FuzzReservationReanchorTransactionMatcher | spv | 62403 | 26 | 34 | 8 | PASS |
| FuzzReservationParamsInfeasibleReasons | spv | 196918 | 13 | 17 | 4 | PASS |
| FuzzApplyWalletTxFeeFloor | tbtcpg | 233309 | 13 | 13 | 0 | PASS |
| FuzzEstimateReservationAcceptanceFee | tbtcpg | 287160 | 9 | 9 | 0 | PASS |
| FuzzEstimateReservationReanchorFee | tbtcpg | 241958 | 9 | 9 | 0 | PASS |
| FuzzReservationReanchorTargetHeadroom | tbtcpg | 240520 | 7 | 7 | 0 | PASS |

Raw logs are retained under `raw-fuzz/` beside this report, one per target.

Examples of final lines:

```
fuzz: elapsed: 1m1s, execs: 160096 (0/sec), new interesting: 12 (total: 108)
PASS

fuzz: elapsed: 1m0s, execs: 287160 (0/sec), new interesting: 0 (total: 9)
PASS
ok  github.com/keep-network/keep-core/pkg/tbtcpg  60.442s
```

The decision targets found no new interesting inputs beyond their cached
baselines. This does not establish complete branch coverage or prove the
arithmetic correct for every input.

## Coverage boundary

Covered: proposal wire unmarshal round-trip (both reservation proposal
types and the coordination message), SPV reservation transaction matching
(the fee-cap and fee-sign checks in `isMatchingReservationAcceptanceTransaction`
and `isMatchingReservationReanchorTransaction`, including wrap-around of
the int64 fee arithmetic), the reservation parameter infeasibility rules,
the wallet-tx fee floor with all its overflow guards, both reservation
fixed-size fee estimators (raw-estimate cap, floor raise, clamp), and the
re-anchor target count/amount headroom decision.

Not covered by these targets (out of scope or not reachable without
network/chain state): the SPV proof submission path
(`proveReservationTransaction` and `getProofInfo`, which need a
difficulty-epoch chain and a proof cache, not just bytes); the Electrum
JSON-RPC response parsing around `convertRawTransaction` (the
transaction deserializer itself is the external-data boundary and is
fuzzed here); the full `Run` methods of the acceptance/re-anchor tasks
(they need a chain, not just values); and mutation testing of the
contracts and Go, which is a separate workstream.

## Bugs found

No invariant violation was observed in the recorded runs. A passing
no-panic check does not prove that arithmetic outside the checked domain
is correct.

## Limits

- Each target fuzzed for the minimum 60s requested, at `GOMAXPROCS=2`
  under host memory pressure; longer runs would extend the coverage
  totals.
- The `new interesting` deltas are against the seed baseline only; the
  fuzzer's cache accumulates across runs on this machine, so rerunning
  will report a higher baseline rather than new discoveries.
- The headroom target constrains its invariant check to an overflow-free
  sub-domain (count < 2^24, amounts < 2^52); wilder values are still run
  for the no-panic guarantee but not invariant-checked, because the
  `count + 1` and `amount + anchorValue` expressions in the production
  code wrap outside that domain and the headroom statement is not
  meaningful there.

## Orchestrator verification

- `go vet` passed on all three touched packages.
- Seed inputs passed in all three packages.
- A 20-second re-anchor proposal campaign passed with 56,498 executions.
- A 20-second re-anchor matcher campaign passed with 4,147 executions.
