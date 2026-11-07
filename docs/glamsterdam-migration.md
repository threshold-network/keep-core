# Glamsterdam gas compatibility

The current Amsterdam gas rules are provisional until the fork specification is
finalized. This draft provides defensive gas-meter arithmetic and review tooling;
it does not establish a required mainnet contract upgrade.

## Live wallet-creation wiring

At mainnet block 26,363,834, WalletRegistry
`0x46d52E41C2F300BC82217Ce22b920c34995204eb` returned
`0xF23fc18EcE6Afe7fb57ba59979e8600bb4B0294a` from `randomBeacon()`.
That provider is RandomBeaconChaosnet, whose callback forwards available gas
without a fixed 64k cap. The registry switched to it on January 25, 2023 in
[this transaction](https://etherscan.io/tx/0xa7314c468cb059867cb9d544b964754ca95d228aa72d6d74e9c40e419083304a).

The separate RandomBeacon at `0x5499f54b4A1CB4816eefCf78962040461be3D80b`
has a 64k callback parameter, but it is not the live registry's provider.
Consequently, this draft does not change callback defaults or generate callback
governance updates. Always read the registry's actual provider before assessing
callback limits.

The EELS fixture in `solidity/ecdsa/test/glamsterdam/` uses verified deployed
registry bytecode with **synthetic storage assigning the capped beacon** and a
stub sortition pool. It demonstrates a hypothetical capped-callback failure,
not the current mainnet wallet-creation path. Its 240,768 threshold and 300k
success must not be used to justify a live callback update.

## DKG reimbursement: calldata floor matters

DKG submission can succeed while its fixed reimbursement underpays operators.
The draft leaves the historical defaults unchanged: 237,650 gas for RandomBeacon
and 290,000 for WalletRegistry. These are not post-fork recommendations.

For a WalletRegistry result with 100 members, 100 signatures and no misbehaved
members, both submission and approval calldata are 13,412 bytes. With no value
or access list, the pinned Amsterdam transaction floor is:

```text
12,000 base + 3,000 recipient access + 64 × 13,412 bytes = 873,368 gas
```

The floor depends on the actual encoding and signature count. It is imposed at
final transaction settlement, so `gasleft()` cannot observe it even below a
`2^24` transaction gas limit. Adding only the two fresh-slot cost increases
misses this mechanism; the earlier 450k/500k proposals were not justified.

A supplied Sepolia lab report says a WalletRegistry allowance of 920k covered
the full cost of a real 100-member DKG, and was then restored. Treat 920k as a
reported benchmark candidate until its receipts, signature count, gas prices,
approval measurement and reimbursement are independently checked. Do not copy it
to RandomBeacon: that DKG uses a different result shape and 64 members.

Benchmark the complete submission **and** approval transactions, using actual
receipts and credited ETH at the pool's capped reimbursement gas price. Include
the approval offset and `ReimbursementPool.staticGas` (40,800 in the observed
mainnet deployment). Recheck all timeout/inactivity offsets separately. Existing
governance setters can change reimbursement parameters without upgrading the
contract implementation.

## Read-only governance preparation

The script prints the actual registry provider, current gas parameters, owners,
delays and optional refund-update calldata at one pinned block. With neither
environment variable set, it prints no update calls. Supply a separately
benchmarked positive integer for each parameter you intend to review:

```sh
cd solidity/ecdsa
GLAMSTERDAM_WALLET_DKG_SUBMISSION_GAS=<benchmarked-value> \
  yarn hardhat run scripts/prepare-glamsterdam.ts --network mainnet
```

Use `GLAMSTERDAM_BEACON_DKG_SUBMISSION_GAS` only after a separate RandomBeacon
benchmark. The script never signs or broadcasts. Review the chain ID, live
provider, owners and pending updates; submit approved begin calls, wait the
on-chain governance delay, verify pending values, then submit matching finalize
calls. Never finalize an unrelated pending update. Read `gasParameters()` again
afterward. Neither historical defaults nor supplied environment values become
automatically validated by generating calldata.

## Gas measurement and payout limitations

The updated meter clamps a negative `gasStart - gasleft()` delta to zero,
including the negative DKG-timeout offset. This prevents arithmetic underflow
from reverting otherwise successful work. It adds no storage fields and preserves
proxy storage layouts. No ordinary production transaction has been demonstrated
to trigger such an underflow; this is defensive compatibility work.

Saturation does not make reimbursement exact. Besides the settlement floor,
reservoir-paid new-state gas above `2^24` is invisible to `gasleft()`. Monitor
actual transaction costs and use ordinary fresh estimates. A reimbursement
redesign needs an agreed capped fee or a state-aware quote; this draft does not
provide one.

ReimbursementPool treats failed ETH sends as best effort. Contract operators
should use a storage-free receiver and monitor `SendingEtherFailed`. Adding a
fresh retry-credit slot after an out-of-gas receiver call can itself exhaust gas
and revert protocol work; this draft does not add that failure path. A replacement
pool requires separate migration and gas benchmarks.

## References

- https://eips.ethereum.org/EIPS/eip-7976
- https://eips.ethereum.org/EIPS/eip-2780
- https://eips.ethereum.org/EIPS/eip-8037
- https://eips.ethereum.org/EIPS/eip-8038
- https://github.com/ethereum/glamsterdam-gas-invariants
