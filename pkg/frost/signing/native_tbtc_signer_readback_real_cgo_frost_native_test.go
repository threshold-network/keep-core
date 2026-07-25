//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"fmt"
	"testing"
)

// TestRealCgoSignerReadinessReadbacks exercises every ABI-4.2 readiness
// readback against the actually linked libfrost_tbtc. Pure Go decoder tests
// cannot detect a stale library that reports a compatible-looking ABI version
// while omitting a symbol or emitting a different transcript.
func TestRealCgoSignerReadinessReadbacks(t *testing.T) {
	setupRealCgoSignerState(t)

	identity, err := ReadNativeTBTCSignerDurableStoreIdentity()
	skipFrostUnavailable(t, "durable store identity", err)
	if identity == nil {
		t.Fatal("durable store identity readback is nil")
	}

	initialInventory, err := ReadNativeTBTCSignerRetainedKeyPackageInventory()
	skipFrostUnavailable(t, "retained key-package inventory", err)
	if initialInventory == nil {
		t.Fatal("retained key-package inventory readback is nil")
	}
	if initialInventory.StoreFingerprint != identity.Fingerprint {
		t.Fatal("inventory and identity readbacks belong to different stores")
	}

	proof, err := ReadNativeTBTCSignerStateWitnessProof(
		&NativeTBTCSignerStateWitnessProofRequest{
			Schema:             NativeTBTCSignerStateWitnessProofRequestSchema,
			StoreFingerprint:   initialInventory.StoreFingerprint,
			AncestorGeneration: initialInventory.StateGeneration,
			AncestorCommitment: initialInventory.StateCommitment,
			TargetGeneration:   initialInventory.StateGeneration,
			TargetCommitment:   initialInventory.StateCommitment,
			MaximumEntries:     16,
		},
	)
	skipFrostUnavailable(t, "state-witness proof", err)
	if proof == nil || !proof.Complete || len(proof.Entries) != 0 {
		t.Fatalf("unexpected equal-tip state-witness proof: [%+v]", proof)
	}

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf(
		"real-cgo-readback-session-%d",
		realCgoSessionSeq.Add(1),
	)
	keyGroup := runRealCgoDKGKeyGroup(
		t,
		engine,
		sessionID,
		[]byte{1, 2},
		2,
	)
	if len(keyGroup) != 66 {
		t.Fatalf("Rust DKG returned a non-compressed key-group handle: [%s]", keyGroup)
	}
	outputKey, err := TaprootOutputKeyFromTBTCSignerKey(keyGroup)
	if err != nil {
		t.Fatalf("cannot derive x-only wallet ID from real key group: [%v]", err)
	}

	inventory, err := ReadNativeTBTCSignerRetainedKeyPackageInventory()
	skipFrostUnavailable(t, "retained key-package inventory after DKG", err)
	found := false
	for _, entry := range inventory.Entries {
		if entry.KeyGroup != keyGroup {
			continue
		}
		if !bytes.Equal(entry.WalletID[:], outputKey) {
			t.Fatal("real compressed key group does not identify its inventory wallet")
		}
		if entry.Threshold != 2 || entry.ParticipantCount != 2 ||
			len(entry.KeyPackages) != 2 {
			t.Fatalf("unexpected real retained key group: [%+v]", entry)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("real persisted key group [%s] is absent from inventory", keyGroup)
	}
}
