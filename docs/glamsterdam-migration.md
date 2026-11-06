# Glamsterdam gas compatibility

The current EIP-8037/EIP-8038 proposal changes storage and gas accounting.
These changes are provisional until the fork specification is finalized.

## Existing deployments

Changing constructor/initializer defaults does not update an existing contract.
Before activation, use the existing governance setters for these parameters:

| Contract | Parameter | Previous mainnet value | Proposed draft value |
| --- | --- | ---: | ---: |
| RandomBeacon | callbackGasLimit | 64,000 | 300,000 |
| RandomBeacon | dkgResultSubmissionGas | 237,650 | 450,000 |
| WalletRegistry | dkgResultSubmissionGas | 290,000 | 500,000 |

The callback limit includes margin over 240,768 gas measured with the deployed
WalletRegistry proxy and implementation under Amsterdam EELS. At 64,000 gas the
callback runs out of gas while writing `startBlock` and `seed`; the beacon catches
the failure, leaving WalletRegistry awaiting its seed until timeout recovery.

The DKG reimbursement values are provisional budgets, not measured exact costs.
Both result-submission paths create two fresh slots. Their storage costs alone
increase by 175,840 gas versus Osaka. The proposed budgets add margin above the
old budgets plus that delta. Benchmark both complete paths under the final fork
schedule before signing updates; also remeasure every approval/timeout/inactivity
offset and ReimbursementPool.staticGas. Do not blindly add a single opcode delta
to staticGas because refunds and their cap affect the total.

Generate reviewable begin/finalize calls from live governance references:

```sh
cd solidity/ecdsa
yarn hardhat run scripts/prepare-glamsterdam.ts --network mainnet
```

The script only reads chain state and prints calldata. It does not sign or send.
Review chain ID, owners, current parameters, and any already-pending updates.
Submit the begin calls through the relevant governance owner, wait the on-chain
delay (currently seven days on mainnet), verify each pending value, then submit
the matching finalize calls. Re-read `relayEntryParameters()` and both
`gasParameters()` getters afterward. Never finalize an unrelated pending update.

Run a full RandomBeacon -> WalletRegistry proxy callback on a fork using the new
rules with transaction gas below and above 2^24. Check that `DkgStarted` occurs,
`CallbackFailed` does not occur, and both startBlock and seed are recorded. Ensure
the outer relay submission has enough gas for its own state changes and refund.

The public deployed bytecode regression is reproducible separately from Hardhat:

```sh
python3 -m venv /tmp/keep-glamsterdam-eels
/tmp/keep-glamsterdam-eels/bin/pip install 'ethereum-execution @ git+https://github.com/ethereum/execution-specs.git@a7dbafac0c0ec82e7b2e20959cc7c00ce958442a'
/tmp/keep-glamsterdam-eels/bin/python solidity/ecdsa/test/glamsterdam/wallet_registry_callback_eels.py
```

The fixture contains only the verified mainnet implementation runtime for
`0x420c99a4B0e4CB65c4CaBf2b4D2526372aA1ac6e`, obtained from Sourcify; the script
embeds proxy runtime read from `0x46d52E41C2F300BC82217Ce22b920c34995204eb`.
It models the required `AWAITING_SEED` state and a locked sortition-pool response.
Assertions cover Osaka success, Amsterdam failure without a reservoir, Amsterdam
success at 300k, reservoir behavior, and the pinned proxy threshold. It excludes
the outer RandomBeacon frame and does not prove a production relay gas estimate.

## Gas measurement

The updated meter clamps a negative `gasStart - gasleft()` delta to zero, including
the negative DKG-timeout offset. This prevents a gas credit from reverting otherwise
successful work. It adds no storage fields and preserves proxy storage layouts.

This does **not** measure reservoir-paid state gas. For transaction gas above 2^24,
EIP-8037 pays new-state charges from a reservoir invisible to `gasleft()`. Operators
must not expect exact reimbursement for that gas. Use ordinary estimated gas
limits, monitor actual transaction cost, and do not rely on a reservoir to bypass
the old callback cap. A future reimbursement redesign needs an explicitly agreed,
capped fee or a state-aware quote; fixed offsets cannot reconstruct hidden dynamic
state costs.

ReimbursementPool deliberately treats failed ETH sends as best effort. Contract
operators should use a storage-free receiver and alert on `SendingEtherFailed`.
Adding a fresh retry-credit slot after an out-of-gas receiver call could itself
run out of gas and revert protocol work; this patch does not add that failure path.
A pull-payment replacement pool requires a separate migration and gas benchmark.

## References

- https://eips.ethereum.org/EIPS/eip-8037
- https://eips.ethereum.org/EIPS/eip-8038
- https://github.com/ethereum/glamsterdam-gas-invariants
