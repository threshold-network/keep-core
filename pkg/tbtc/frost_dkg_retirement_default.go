//go:build !frost_native

package tbtc

func newFrostOrphanedDKGReconciler(
	_ Chain,
	_ *walletRegistry,
	_ *frostNativeSignerAnchorAdmissionController,
) (
	frostOrphanedDKGReconcilerFunc,
	error,
) {
	return nil, nil
}
