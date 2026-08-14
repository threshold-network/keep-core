//go:build !frost_native

package tbtc

func currentFrostInteractiveSigningReadiness() bool {
	return false
}
