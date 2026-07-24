package signing

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeNativeTBTCSignerRetainedKeyPackageInventory(t *testing.T) {
	wire := testNativeTBTCSignerRetainedKeyPackageInventoryWire()
	payload, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := DecodeNativeTBTCSignerRetainedKeyPackageInventory(payload)
	if err != nil {
		t.Fatalf("valid native inventory was rejected: [%v]", err)
	}
	if inventory.StateGeneration != wire.StateGeneration || len(inventory.Entries) != 1 ||
		inventory.Entries[0].ShareEpoch != 0 || len(inventory.Entries[0].KeyPackages) != 1 {
		t.Fatalf("unexpected decoded inventory: %+v", inventory)
	}
}

func TestDecodeNativeTBTCSignerRetainedKeyPackageInventoryRejectsSubstitution(
	t *testing.T,
) {
	tests := map[string]func(*nativeTBTCSignerRetainedKeyPackageInventoryWire){
		"missing entries": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			wire.Entries = nil
		},
		"missing share epoch": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			(*wire.Entries)[0].ShareEpoch = nil
		},
		"wrong key group": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			(*wire.Entries)[0].KeyGroup = strings.Repeat("09", 32)
		},
		"wrong state image": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			wire.StateImageDigest = nativeTBTCSignerBytes32([32]byte{0x7f})
		},
		"wrong inventory commitment": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			wire.InventoryCommitment = nativeTBTCSignerBytes32([32]byte{0x7e})
		},
		"duplicate seat": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			(*wire.Entries)[0].KeyPackages = append(
				(*wire.Entries)[0].KeyPackages,
				(*wire.Entries)[0].KeyPackages[0],
			)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			wire := testNativeTBTCSignerRetainedKeyPackageInventoryWire()
			mutate(wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeNativeTBTCSignerRetainedKeyPackageInventory(payload); err == nil {
				t.Fatal("substituted native inventory was accepted")
			}
		})
	}
}

func TestDecodeNativeTBTCSignerStateWitnessProof(t *testing.T) {
	storeFingerprint := [32]byte{0x11}
	ancestorCommitment := [32]byte{0x12}
	firstImage := [32]byte{0x13}
	firstCommitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		8,
		ancestorCommitment,
		firstImage,
	)
	secondImage := [32]byte{0x14}
	secondCommitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		9,
		firstCommitment,
		secondImage,
	)
	complete := true
	proofEntries := []nativeTBTCSignerStateWitnessProofEntryWire{
		{
			Generation:              8,
			PreviousStateCommitment: nativeTBTCSignerBytes32(ancestorCommitment),
			StateCommitment:         nativeTBTCSignerBytes32(firstCommitment),
			StateImageDigest:        nativeTBTCSignerBytes32(firstImage),
		},
		{
			Generation:              9,
			PreviousStateCommitment: nativeTBTCSignerBytes32(firstCommitment),
			StateCommitment:         nativeTBTCSignerBytes32(secondCommitment),
			StateImageDigest:        nativeTBTCSignerBytes32(secondImage),
		},
	}
	wire := &nativeTBTCSignerStateWitnessProofWire{
		Schema:             NativeTBTCSignerStateWitnessProofSchema,
		StoreFingerprint:   nativeTBTCSignerBytes32(storeFingerprint),
		AncestorGeneration: 7,
		AncestorCommitment: nativeTBTCSignerBytes32(ancestorCommitment),
		TargetGeneration:   9,
		TargetCommitment:   nativeTBTCSignerBytes32(secondCommitment),
		Complete:           &complete,
		Entries:            &proofEntries,
	}
	payload, _ := json.Marshal(wire)
	proof, err := DecodeNativeTBTCSignerStateWitnessProof(payload)
	if err != nil {
		t.Fatalf("valid state-witness proof was rejected: [%v]", err)
	}
	if !proof.Complete || len(proof.Entries) != 2 {
		t.Fatalf("unexpected proof: %+v", proof)
	}

	(*wire.Entries)[1].StateImageDigest = nativeTBTCSignerBytes32([32]byte{0xff})
	payload, _ = json.Marshal(wire)
	if _, err := DecodeNativeTBTCSignerStateWitnessProof(payload); err == nil {
		t.Fatal("state-witness proof with a forged image digest was accepted")
	}
}

func testNativeTBTCSignerRetainedKeyPackageInventoryWire() *nativeTBTCSignerRetainedKeyPackageInventoryWire {
	storeFingerprint := [32]byte{0x01}
	previousCommitment := [32]byte{0x02}
	stateImageDigest := [32]byte{0x03}
	walletID := [32]byte{0x04}
	entries := []NativeTBTCSignerRetainedKeyGroup{
		{
			WalletID:                   walletID,
			KeyGroup:                   hex.EncodeToString(walletID[:]),
			Threshold:                  51,
			ParticipantCount:           100,
			ShareEpoch:                 0,
			PublicKeyPackageCommitment: [32]byte{0x05},
			KeyPackages: []NativeTBTCSignerRetainedKeyPackage{
				{ParticipantSeat: 3, KeyPackageCommitment: [32]byte{0x06}},
			},
		},
	}
	shareEpoch := uint64(0)
	wireEntries := []nativeTBTCSignerRetainedKeyGroupWire{
		{
			WalletID:                   nativeTBTCSignerBytes32(walletID),
			KeyGroup:                   hex.EncodeToString(walletID[:]),
			Threshold:                  51,
			ParticipantCount:           100,
			ShareEpoch:                 &shareEpoch,
			PublicKeyPackageCommitment: nativeTBTCSignerBytes32([32]byte{0x05}),
			KeyPackages: []nativeTBTCSignerRetainedKeyPackageWire{
				{ParticipantSeat: 3, KeyPackageCommitment: nativeTBTCSignerBytes32([32]byte{0x06})},
			},
		},
	}
	return &nativeTBTCSignerRetainedKeyPackageInventoryWire{
		Schema:           NativeTBTCSignerRetainedKeyPackageInventorySchema,
		StoreFingerprint: nativeTBTCSignerBytes32(storeFingerprint),
		StateGeneration:  7,
		StateCommitment: nativeTBTCSignerBytes32(
			ComputeNativeTBTCSignerStateWitnessCommitment(
				storeFingerprint,
				7,
				previousCommitment,
				stateImageDigest,
			),
		),
		PreviousStateCommitment: nativeTBTCSignerBytes32(previousCommitment),
		StateImageDigest:        nativeTBTCSignerBytes32(stateImageDigest),
		InventoryCommitment: nativeTBTCSignerBytes32(
			ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(entries),
		),
		Entries: &wireEntries,
	}
}
