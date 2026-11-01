# Deploy-ceremony cross-parameter rules

Evidence pins and vocabulary: [parameters and governance index](03-parameters-governance.md).

### PAR-X11 RebateStaking and Bridge upgrade ceremony (script-only)

The reservation-capable Bridge implementation and the `RebateStaking`
implementation it calls into are installed by ONE Timelock
`scheduleBatch`/`executeBatch` pair, emitted by deploy script
`tbtc-v2@72b891eec:solidity/deploy/99_deploy_rebate_staking_reservation_implementation.ts`. The batch has
exactly two actions and their order is fixed: action `[0]` upgrades the
`RebateStaking` proxy, action `[1]` upgrades the Bridge proxy
(`tbtc-v2@72b891eec:solidity/deploy/99_deploy_rebate_staking_reservation_implementation.ts:123-141`;
the order is asserted by `assertBatchOrder` at `:202-210`). The order is
load-bearing, not cosmetic: the head `Redemption` library calls
`RebateStaking.isRebateAuthorized` (`0x96253dad`) on every callback
redemption, the implementation installed on mainnet today lacks that
selector, and `RebateStaking` has neither a fallback nor a receive
function, so a Bridge-first upgrade makes every vault redemption revert
until the staking upgrade lands. Two separate `schedule` calls are two
transactions and leave that window open for `getMinDelay()` seconds; one
`executeBatch` is atomic, so the window does not exist.

The Bridge implementation is an INPUT, never deployed by the script
(human decision R16-2). The address is read from the environment variable
`BRIDGE_RESERVATION_IMPLEMENTATION`
(`tbtc-v2@72b891eec:solidity/deploy/99_deploy_rebate_staking_reservation_implementation.ts:1617-1619`),
and the script refuses to run in two cases: when the variable is absent or
empty (`requireBridgeImplementationInput` at `:1062-1074`), and when the
named address has no code on the target network (`:1620-1626`). The script
contains no `deploy` call for the Bridge and no path that produces one:
deploying a second Bridge implementation here would point action `[1]` at a
build other than the one the release pipeline deployed, and the pre-flight
scan would certify that other build.

The router address that the reservation governance calldata embeds is
checked against the chain before the calldata is generated. Script 98
reconstructs the router's runtime from the deployment record with its
libraries substituted (`linkedRecordRuntime` at
`tbtc-v2@72b891eec:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:494-535`),
compares it with the code at the router address (`:640-663`), and refuses to
embed a mismatched address in mainnet governance calldata.

Test:
`tbtc-v2@72b891eec:solidity/test/deploy/99_deploy_rebate_staking_reservation_implementation.test.ts`
(the batch order, the missing Bridge implementation input, and the
no-code refusal) and
`tbtc-v2@72b891eec:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts`
(the linked-runtime reconstruction and the mismatched-address refusal).

Status: holds.
