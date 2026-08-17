package signing

import (
	"crypto/sha256"
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

func TestDecodeNativeTBTCSignerRetainedKeyPackageInventoryAcceptsCompressedKeyGroup(
	t *testing.T,
) {
	const compressedKeyGroup = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	walletIDBytes, err := hex.DecodeString(compressedKeyGroup[2:])
	if err != nil {
		t.Fatal(err)
	}
	var walletID [32]byte
	copy(walletID[:], walletIDBytes)

	wire := testNativeTBTCSignerRetainedKeyPackageInventoryWire()
	(*wire.Entries)[0].WalletID = nativeTBTCSignerBytes32(walletID)
	(*wire.Entries)[0].KeyGroup = compressedKeyGroup
	entries := []NativeTBTCSignerRetainedKeyGroup{
		{
			WalletID:                   walletID,
			KeyGroup:                   compressedKeyGroup,
			Threshold:                  51,
			ParticipantCount:           100,
			ShareEpoch:                 0,
			PublicKeyPackageCommitment: [32]byte{0x05},
			KeyPackages: []NativeTBTCSignerRetainedKeyPackage{
				{ParticipantSeat: 3, KeyPackageCommitment: [32]byte{0x06}},
			},
		},
	}
	wire.InventoryCommitment = nativeTBTCSignerBytes32(
		ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(entries),
	)

	payload, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := DecodeNativeTBTCSignerRetainedKeyPackageInventory(payload)
	if err != nil {
		t.Fatalf("valid compressed native key group was rejected: [%v]", err)
	}
	if inventory.Entries[0].KeyGroup != compressedKeyGroup ||
		inventory.Entries[0].WalletID != walletID {
		t.Fatalf("compressed key group was not retained exactly: [%+v]", inventory.Entries[0])
	}
}

func TestNativeTBTCSignerInventoryCommitmentMatchesRustFrozenVector(t *testing.T) {
	entries := []NativeTBTCSignerRetainedKeyGroup{
		{
			WalletID:                   repeatedNativeTBTCSignerBytes32(0x11),
			KeyGroup:                   "02" + strings.Repeat("11", 32),
			Threshold:                  2,
			ParticipantCount:           3,
			ShareEpoch:                 0,
			PublicKeyPackageCommitment: repeatedNativeTBTCSignerBytes32(0x33),
			KeyPackages: []NativeTBTCSignerRetainedKeyPackage{
				{
					ParticipantSeat:      1,
					KeyPackageCommitment: repeatedNativeTBTCSignerBytes32(0x44),
				},
				{
					ParticipantSeat:      3,
					KeyPackageCommitment: repeatedNativeTBTCSignerBytes32(0x55),
				},
			},
		},
	}

	actual := ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(entries)
	const expected = "bd6ec36fa27a57dd9926883bb2ff4dee7ececd28de940df7294f0e0f0dedd150"
	if hex.EncodeToString(actual[:]) != expected {
		t.Fatalf("unexpected inventory commitment: [%x]", actual)
	}
}

func TestNativeTBTCSignerStateWitnessCommitmentMatchesRustV2Vector(t *testing.T) {
	storeFingerprint := repeatedNativeTBTCSignerBytes32(0x11)
	genesis := ComputeNativeTBTCSignerStateWitnessGenesis(storeFingerprint)
	const expectedGenesis = "44085b42d29bf25f06207142f9e2db58eaf86f88d92b6e18104161ce59e98a89"
	if hex.EncodeToString(genesis[:]) != expectedGenesis {
		t.Fatalf("unexpected v2 state-witness genesis: [%x]", genesis)
	}

	actual := ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		42,
		repeatedNativeTBTCSignerBytes32(0x22),
		repeatedNativeTBTCSignerBytes32(0x33),
	)
	const expected = "ea5eb04a4776357e59875f683390a2ff4b7dd511ad394e588dfab147f94fa867"
	if hex.EncodeToString(actual[:]) != expected {
		t.Fatalf("unexpected v2 state-witness commitment: [%x]", actual)
	}

	retiredV1GenesisInput := append(
		[]byte("tbtc-signer-state-witness-genesis-v1\x00"),
		storeFingerprint[:]...,
	)
	retiredV1Genesis := sha256.Sum256(retiredV1GenesisInput)
	const expectedRetiredV1Genesis = "639ab6bce7b111044aa40cbe05d2a79a789c47d83e0dbf5ac83af3e2c8717775"
	if hex.EncodeToString(retiredV1Genesis[:]) != expectedRetiredV1Genesis {
		t.Fatalf("unexpected retired v1 state-witness genesis: [%x]", retiredV1Genesis)
	}
	if retiredV1Genesis == genesis {
		t.Fatal("v2 state-witness genesis aliases the retired v1 transcript")
	}
}

func TestDecodeNativeTBTCSignerRetainedKeyPackageInventoryRejectsRetiredV1Commitment(
	t *testing.T,
) {
	wire := testNativeTBTCSignerRetainedKeyPackageInventoryWire()
	wire.StateGeneration = 42
	wire.PreviousStateCommitment = nativeTBTCSignerBytes32(
		repeatedNativeTBTCSignerBytes32(0x22),
	)
	wire.StateImageDigest = nativeTBTCSignerBytes32(
		repeatedNativeTBTCSignerBytes32(0x33),
	)
	retiredV1, err := hex.DecodeString(
		"903d154bca4b0e46f2cadda81db9559bdf2d719956065266f55bd845e64b7ced",
	)
	if err != nil {
		t.Fatal(err)
	}
	var retiredV1Commitment [32]byte
	copy(retiredV1Commitment[:], retiredV1)
	wire.StoreFingerprint = nativeTBTCSignerBytes32(
		repeatedNativeTBTCSignerBytes32(0x11),
	)
	wire.StateCommitment = nativeTBTCSignerBytes32(retiredV1Commitment)

	payload, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeNativeTBTCSignerRetainedKeyPackageInventory(payload); err == nil {
		t.Fatal("retired v1 state-witness commitment was accepted under the v2 ABI")
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
		"uppercase key group": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			(*wire.Entries)[0].KeyGroup = strings.ToUpper(
				strings.Repeat("0a", 32),
			)
		},
		"0x-prefixed key group": func(wire *nativeTBTCSignerRetainedKeyPackageInventoryWire) {
			(*wire.Entries)[0].KeyGroup = "0x" + (*wire.Entries)[0].KeyGroup
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
