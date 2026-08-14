//go:build !(frost_native && frost_roast_retry)

package tbtc

// registerRoastRetryCoordinatorForSeats is a no-op in builds without the
// frost_native && frost_roast_retry tags. The ROAST-retry coordinator registry
// only takes effect under those tags (the interactive engine needs frost_native
// and the real registry needs frost_roast_retry), so there is nothing to wire in
// other builds. Kept as a plain function so node.getSigningExecutor can call it
// unconditionally, mirroring the newRoastTransitionController default/tagged split.
func registerRoastRetryCoordinatorForSeats(_ *node, _ []*signer) {}

// roastSelectorKeyGroupID returns "" in builds without frost_native &&
// frost_roast_retry: no ROAST key group is derivable and the registry is a no-op,
// so the participant selector's per-wallet activation gate finds nothing and
// selection stays on the legacy path — matching pre-RFC-21 behaviour. Kept as a
// plain function so signing.go can call it unconditionally.
func roastSelectorKeyGroupID(_ *signer) string { return "" }
