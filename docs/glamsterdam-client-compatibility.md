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

## Regression coverage

`TestGetBlockHashByNumber_Glamsterdam` serves a real public Sepolia header
through an in-process JSON-RPC server and calls the production method. It
compares the result with the canonical hash saved in the original response.
The test fails with geth v1.17.3 and passes with the corrected decoder.

The fixture and its provenance are in
`pkg/chain/ethereum/testdata/sepolia-glamsterdam-header-12084523.*`.
