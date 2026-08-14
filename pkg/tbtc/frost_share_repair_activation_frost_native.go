//go:build frost_native

package tbtc

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func installFrostShareRepairActivationRegistry(
	registryPath string,
	manifest FrostPreSignActivationRuntimeManifest,
	storeBinding *frostDurableSessionStoreBinding,
) error {
	expectedRoot := manifest.ShareRepairActivationRegistryRoot
	registryPath = strings.TrimSpace(registryPath)
	if storeBinding == nil {
		return fmt.Errorf("share-repair activation store binding is absent")
	}
	storeFingerprint, err := storeBinding.verify()
	if err != nil {
		return fmt.Errorf("cannot verify share-repair activation store binding: %w", err)
	}
	inventory, err := frostsigning.ReadNativeTBTCSignerRetainedKeyPackageInventory()
	if err != nil {
		return fmt.Errorf("cannot read native share-repair activation inventory: %w", err)
	}
	if inventory == nil || inventory.StoreFingerprint != storeFingerprint {
		return fmt.Errorf("native share-repair inventory differs from the bound durable store")
	}
	if err := frostsigning.ConfigureShareRepairActivationGuard(inventory); err != nil {
		return fmt.Errorf("cannot configure repaired-seat activation guard: %w", err)
	}
	if expectedRoot == [32]byte{} {
		if registryPath != "" {
			return fmt.Errorf(
				"registry path is configured but the signed manifest declares no registry",
			)
		}
		if !frostsigning.ShareRepairActivationReady(expectedRoot) {
			return fmt.Errorf(
				"native signer contains repaired seats pending an authority-signed activation registry",
			)
		}
		return nil
	}
	if registryPath == "" {
		return fmt.Errorf(
			"signed manifest declares a share-repair registry but its secure file is absent",
		)
	}
	payload, err := readSecureFrostActivationFile(registryPath, 8*1024*1024)
	if err != nil {
		return fmt.Errorf("cannot read secure share-repair registry: %w", err)
	}
	if err := frostsigning.InstallShareRepairActivationRegistry(
		payload,
		ed25519.PublicKey(manifest.ActivationAuthorityPublicKey[:]),
		expectedRoot,
		storeFingerprint,
	); err != nil {
		return err
	}
	if !frostsigning.ShareRepairActivationReady(expectedRoot) {
		return fmt.Errorf("share-repair activation registry did not satisfy native recovery facts")
	}
	return nil
}

func currentFrostShareRepairActivationRegistryRoot() [32]byte {
	return frostsigning.CurrentShareRepairActivationRegistryRoot()
}

func frostShareRepairActivationReady(expectedRoot [32]byte) bool {
	return frostsigning.ShareRepairActivationReady(expectedRoot)
}

func prepareFrostShareRepairActivationForTest(storeFingerprint [32]byte) error {
	frostsigning.ResetShareRepairActivationStateForTest()
	return frostsigning.ConfigureShareRepairActivationGuard(
		&frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
			Schema:           frostsigning.NativeTBTCSignerRetainedKeyPackageInventorySchema,
			StoreFingerprint: storeFingerprint,
		},
	)
}

func resetFrostShareRepairActivationForTest() {
	frostsigning.ResetShareRepairActivationStateForTest()
}
