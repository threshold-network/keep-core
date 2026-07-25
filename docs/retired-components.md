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
