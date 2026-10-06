# Legacy sortition compatibility

These are the frozen runtime sources from `@keep-network/sortition-pools@2.0.0`.
The runtime files are byte-identical in `2.0.0-pre.16`, previously locked by
Random Beacon. Only the two imports in SortitionPool point to local copies of
the Thesis interfaces; their transitive IApproveAndCall interface is retained too.
`VENDOR.json` records the source revision, package manifest, and original hashes.
Licenses and notices are retained alongside the components.

The current source still needs this implementation for legacy ABI compatibility,
fixtures, and deployment replay. Its presence is not a statement about live
operator selection. Do not add new protocol features here or refresh from an
upstream branch. Retiring the implementation itself is a separate contract and
historical-deployment migration.

Frozen legacy sources retain their prior dependency treatment in lint and Slither
filters; the original files were excluded under `node_modules/`.
