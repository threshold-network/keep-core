//go:build !frost_native

package tbtc

import "context"

func newFrostOrphanedDKGReconciler(
	_ Chain,
	_ *walletRegistry,
	_ *frostNativeSignerAnchorAdmissionController,
) (
	func(context.Context, map[[32]byte]struct{}) error,
	error,
) {
	return nil, nil
}
