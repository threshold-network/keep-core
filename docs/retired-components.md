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
- the `./infrastructure/` tree, with one exception noted below: KEEP-era GKE manifests under `kube/{keep-test,keep-dev,keep-prd,lcl}`, Terraform modules sourcing from the now-defunct `thesis/infrastructure` repository, the `provision-keep-client` initcontainer that consumed `solidity-v1/` contract JSONs (since extracted to `keep-core-v1`), and other private-testnet / Goerli-era assets
- `scripts/start_dashboard.sh`

These components were removed because they are no longer part of supported
operations, were tied to deprecated KEEP-token workflows, and had accumulated
unmaintained security risk. In particular, the old rewards withdrawal helper
contained a committed mainnet private key (since rotated and no longer active),
and the retired staking escrow had no remaining ETH, KEEP, or T balance on
Ethereum mainnet when checked before removal. The removed `infrastructure/`
tree also contained low-sensitivity testnet/dev credential material now
recoverable only via git history: a private Ethereum testnet keystore
passphrase and a hardcoded local-dev dashboard `WS_SECRET`. Neither is a
production credential.

**Exception: `infrastructure/kube/keep-test/tbtc-v2-maintainer/` was kept.**
Unlike the rest of the tree, this Kubernetes overlay is actively deployed
(`kubectl apply -k ./`, independent of the retired Terraform) and was last
patched to fix its Electrum endpoint shortly before this cleanup. It remains
in the repository at its original path.

**GCP projects referenced by the retired Terraform remain live.**
`keep-test-f3e0` and `keep-prd-210b` (see `.github/workflows/client.yml`,
`docs/run-keep-node.adoc`, and `docs/registration.adoc`) are still used for
CI image publishing and client-binary distribution. They are managed
out-of-band from the removed Terraform, which had not been applied since
2020 and sourced from the same now-defunct `thesis/infrastructure` remote.

Historical documents under the `docs/` tree of `keep-core-v1` (formerly
`docs-v1/` here) may still mention these components for release history and
archival context. They should not be used as operational runbooks for current
Threshold Network deployments.
