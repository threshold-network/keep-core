//go:build !frost_native

package tbtc

import (
	"fmt"
	"strings"
)

func installFrostShareRepairActivationRegistry(
	registryPath string,
	manifest FrostPreSignActivationRuntimeManifest,
	_ *frostDurableSessionStoreBinding,
) error {
	if manifest.ShareRepairActivationRegistryRoot != [32]byte{} ||
		strings.TrimSpace(registryPath) != "" {
		return fmt.Errorf("share-repair activation requires the frost_native build")
	}
	return nil
}

func currentFrostShareRepairActivationRegistryRoot() [32]byte {
	return [32]byte{}
}

func frostShareRepairActivationReady(expectedRoot [32]byte) bool {
	return expectedRoot == [32]byte{}
}

func prepareFrostShareRepairActivationForTest([32]byte) error { return nil }

func resetFrostShareRepairActivationForTest() {}
