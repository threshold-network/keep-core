# Retired Legacy Components

KEEP-era staking and distribution surfaces have been retired in favor of the
Threshold Network T token and the current contracts under `solidity/`.

This repository no longer carries the following legacy components. The paths
below are the original locations under the now-extracted v1 tree (formerly
`solidity-v1/` in this repo); the v1 history they appear in has been moved to
[`threshold-network/keep-core-v1`](https://github.com/threshold-network/keep-core-v1):

- `solidity-v1/contracts/TokenStakingEscrow.sol`
- `token-stakedrop/`
- `solidity-v1/scripts/withdraw-old-rewards.js`
- `solidity-v1/dashboard/`
- KEEP token dashboard Kubernetes manifests under `infrastructure/kube/keep-*`
- `scripts/start_dashboard.sh`

These components were removed because they are no longer part of supported
operations, were tied to deprecated KEEP-token workflows, and had accumulated
unmaintained security risk. In particular, the old rewards withdrawal helper
contained a committed mainnet private key (since rotated and no longer active),
and the retired staking escrow had no remaining ETH, KEEP, or T balance on
Ethereum mainnet when checked before removal.

Historical documents under the `docs/` tree of `keep-core-v1` (formerly
`docs-v1/` here) may still mention these components for release history and
archival context. They should not be used as operational runbooks for current
Threshold Network deployments.

## Beacon branch as peer-admission authority (PR #4333)

The random-beacon branch of the peer-admission firewall has been retired.
Admission now consults the tBTC application alone: a peer is admitted when it
is a registered tBTC operator whose provider has positive
`WalletRegistry.eligibleStake`, which the Council controls through the
Allowlist. The beacon recognition method, its reader interface, the production
adapter, and the admission field are removed rather than stubbed, so
`*BeaconChain` no longer satisfies `firewall.Application` and re-adding the
beacon to the application list fails to compile. Beacon startup registration,
`beacon.Initialize`, the Chaosnet seed source, and the shared `RolesOf`
accessor are untouched; none of them was an admission authority, and the
startup-registration requirement is documented where the application list is
built rather than removed.

### Measured census split

Measured at a pinned mainnet block over every operator ever registered on
either registry:

- 20 identities are admitted under the new tBTC-only predicate.
- 262 identities lose admission.
- 19 of the 20 admitted identities are the embedded bootstrap seeds.

None of the 262 retired identities holds tBTC sortition-pool membership or
weight. Rolling this change back re-admits the whole legacy set, including any
identity the Council has since revoked; that is a policy decision, not an
operator convenience.

### Release gate (wallet-continuity precondition)

Exactly one of the excluded identities still holds shares in live wallets. A
release carrying this change MUST NOT be tagged until a wallet-continuity
measurement has been recorded for the affected wallets, demonstrating
coordination and signing without the affected operator. The minimum bar is a
heartbeat with at least seventy active retained members, and the change MUST
be rolled out through the embedded seed operators first.

The specific excluded operator's address is not available from the reviewed
PR/issue materials and MUST be filled in by the PR author before this section
is considered complete:

> Excluded operator holding live wallet shares: `<operator address — TBD by PR author>`

Three known residuals are deliberately out of scope of PR #4333 and tracked
separately: the client installs go-libp2p's default transports for outbound
dials with no explicit transport restriction, the pubsub validator filters on
the original author rather than the connection, and the mainnet-anchored
integration tests still skip in CI because no RPC endpoint is forwarded to
that job.

