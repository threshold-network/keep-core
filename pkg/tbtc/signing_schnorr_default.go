//go:build !frost_native

package tbtc

import "github.com/keep-network/keep-core/pkg/tecdsa"

func signingMaterialUsesSchnorrSignatures(signingMaterial any) bool {
	_, isLegacyMaterial := signingMaterial.(*tecdsa.PrivateKeyShare)
	return !isLegacyMaterial
}
