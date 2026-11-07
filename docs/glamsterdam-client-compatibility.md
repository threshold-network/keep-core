# Glamsterdam client compatibility

The geth v1.17.10 update corrects the JSON name of the block access list hash
from `balHash` to `blockAccessListHash`. Without that field, decoding a
Glamsterdam header and calling `Header.Hash()` computes a different hash from
the canonical block hash reported by Ethereum.

`baseChain.GetBlockHashByNumber` returns that decoded header's hash. Wallet
coordination uses it in `SHA256(wallet public-key hash || safe-block hash)`,
where the safe block is 32 blocks before the coordination block. The seed
controls the operator shuffle that selects the leader and the heartbeat
choice. A fleet using one old hash rule can agree internally while using an
incorrect canonical hash; mixing the old and corrected rules can produce
different leaders and action checklists for the same wallet and window.

## Release coordination

This dependency update changes a shared coordination rule for post-fork
headers. It must be included in a coordinated client release:

- Every operator must run the corrected client before mainnet activation of
  the header fields that change the coordination hash.
- Sepolia already carries these fields. An ungated rolling release there
  mixes the old and corrected hash rules immediately. Its release plan needs
  a coordinated fleet transition or a compatible activation gate, with a
  rehearsal that verifies the same seed, leader, and heartbeat choice across
  the fleet.
- Before release, the coordinated corrected fleet must also complete an
  actual heartbeat and deposit-sweep signing and live execution rehearsal.
  Agreement on the computed leader alone does not satisfy this gate. This
  patch does not claim that this release rehearsal has been completed.
- Before release, compare the corrected client's computed hashes with the
  canonical hashes returned by the actual RPC providers operators use,
  across shared post-fork block numbers. Include more than the saved fixture
  and confirm that every provider serves `blockAccessListHash` and
  `slotNumber` consistently.
- A successful client build, a passing unit test, or unchanged contract
  interfaces does not establish mixed-version coordination compatibility.

This patch chooses no cutover block and performs no deployment. It does not
add an application activation gate; release owners must resolve the
transition before deploying it on an already forked network. The header
hash correction itself requires no smart-contract upgrade.

## Transactions across activation

Generated contract `ForceMining` replacement callbacks still preserve an
original nonzero `GasLimit`. Raising the replacement fee therefore does not
refresh that gas limit after repricing. The geth update does not fix this
behavior.

As an interim release gate, release owners must rehearse and document a
bounded, coordinated drain of pending calls and calls estimated before the
fork:

1. Choose a bounded submission pause and resumption plan for affected
   Ethereum calls. Inventory outstanding transactions by sender and nonce,
   and drain or reconcile them across the activation boundary.
2. Do not use pre-fork gas estimates for post-fork inclusion. Estimate new
   submissions against the active post-fork rules and current business state.
3. Before retrying unresolved work, check its receipt, the account's confirmed
   and pending nonces, and current contract/business state. Distinguish mined,
   still-pending, replaced, and dropped transactions; a missing receipt alone
   does not establish that the original call is safe to repeat.
4. Only after that reconciliation, obtain a fresh estimate and choose a
   nonce-appropriate retry for work that is still needed. Do not blindly
   rebroadcast work that may remain pending or may already have succeeded.
   Resume affected submissions under the bounded plan once these checks and
   post-fork estimation are in place.

This runbook is a release prerequisite, not an implemented transaction-drain
mechanism. This patch performs no live action and chooses no fork date or
activation block.

## Regression coverage

`TestGetBlockHashByNumber_Glamsterdam` serves a real public Sepolia header
through an in-process JSON-RPC server and calls the production method. It
compares the result with the canonical hash saved in the original response.
The test fails with geth v1.17.3 and passes with the corrected decoder.

The fixture and its provenance are in
`pkg/chain/ethereum/testdata/sepolia-glamsterdam-header-12084523.*`.
