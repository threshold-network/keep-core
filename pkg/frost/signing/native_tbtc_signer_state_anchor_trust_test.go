package signing

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeNativeTBTCSignerStateAnchorTrustTransitionResult(t *testing.T) {
	checkpoint := testNativeTBTCSignerStateAnchorCheckpointWire(9, 0x11)
	base := testNativeTBTCSignerStateAnchorCheckpointWire(8, 0x21)
	reference := nativeTBTCSignerStateAnchorTrustReferenceWire{
		ServiceEpoch:        "3",
		Revision:            "1",
		PreviousEventRoot:   testNativeTBTCSignerTrustHex32(0x31),
		EventRoot:           testNativeTBTCSignerTrustHex32(0x32),
		CheckpointAckDigest: testNativeTBTCSignerTrustHex32(0x33),
		Checkpoint:          checkpoint,
	}
	head := nativeTBTCSignerStateAnchorTrustHeadWire{
		Schema:                          NativeTBTCSignerStateAnchorTrustHeadSchema,
		CertificateSequence:             "3",
		CertificateDigest:               testNativeTBTCSignerTrustHex32(0x41),
		ActivationManifestSequence:      "5",
		ActivationManifestHash:          testNativeTBTCSignerTrustHex32(0x42),
		BindingHash:                     testNativeTBTCSignerTrustHex32(0x43),
		ResponsePublicKeySPKISHA256:     testNativeTBTCSignerTrustHex32(0x44),
		OfflineAuthoritySPKISHA256:      testNativeTBTCSignerTrustHex32(0x45),
		ServiceEpoch:                    "3",
		CertifiedFloor:                  reference,
		WitnessMaximumRecords:           "4096",
		WitnessRotationThresholdRecords: "1024",
	}
	wire := nativeTBTCSignerStateAnchorTrustTransitionResultWire{
		Schema:                  NativeTBTCSignerStateAnchorTrustTransitionResultSchema,
		Installed:               testNativeTBTCSignerTrustBool(true),
		Idempotent:              testNativeTBTCSignerTrustBool(false),
		AppliedCertificateCount: "2",
		TrustHead:               head,
		CurrentCheckpoint:       checkpoint,
		WitnessBaseCheckpoint:   base,
		CurrentAnchorReference:  reference,
	}
	payload, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	result, err := DecodeNativeTBTCSignerStateAnchorTrustTransitionResult(payload)
	if err != nil {
		t.Fatalf("valid trust transition result was rejected: %v", err)
	}
	if !result.Installed || result.Idempotent ||
		result.AppliedCertificateCount != 2 ||
		result.TrustHead.CertificateSequence != 3 ||
		result.CurrentCheckpoint.Generation != 9 ||
		result.WitnessBaseCheckpoint.Generation != 8 ||
		result.CurrentAnchorReference.PreviousEventRoot[0] != 0x31 {
		t.Fatalf("unexpected trust transition result: %+v", result)
	}
}

func TestDecodeNativeTBTCSignerStateAnchorTrustRejectsUnknownAndInconsistent(
	t *testing.T,
) {
	checkpoint := testNativeTBTCSignerStateAnchorCheckpointWire(9, 0x11)
	reference := nativeTBTCSignerStateAnchorTrustReferenceWire{
		ServiceEpoch:        "3",
		Revision:            "1",
		PreviousEventRoot:   testNativeTBTCSignerTrustHex32(0x31),
		EventRoot:           testNativeTBTCSignerTrustHex32(0x32),
		CheckpointAckDigest: testNativeTBTCSignerTrustHex32(0x33),
		Checkpoint:          checkpoint,
	}
	head := nativeTBTCSignerStateAnchorTrustHeadWire{
		Schema:                          NativeTBTCSignerStateAnchorTrustHeadSchema,
		CertificateSequence:             "3",
		CertificateDigest:               testNativeTBTCSignerTrustHex32(0x41),
		ActivationManifestSequence:      "5",
		ActivationManifestHash:          testNativeTBTCSignerTrustHex32(0x42),
		BindingHash:                     testNativeTBTCSignerTrustHex32(0x43),
		ResponsePublicKeySPKISHA256:     testNativeTBTCSignerTrustHex32(0x44),
		OfflineAuthoritySPKISHA256:      testNativeTBTCSignerTrustHex32(0x45),
		ServiceEpoch:                    "3",
		CertifiedFloor:                  reference,
		WitnessMaximumRecords:           "4096",
		WitnessRotationThresholdRecords: "1024",
	}
	payload, err := json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	unknown := []byte(strings.Replace(
		string(payload),
		`"certificateSequence"`,
		`"unknown":"x","certificateSequence"`,
		1,
	))
	if _, err := DecodeNativeTBTCSignerStateAnchorTrustHead(unknown); err == nil {
		t.Fatal("trust head with an unknown field was accepted")
	}

	head.ServiceEpoch = "4"
	payload, err = json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeNativeTBTCSignerStateAnchorTrustHead(payload); err == nil {
		t.Fatal("trust head whose service epoch differs from its floor was accepted")
	}

	head.ServiceEpoch = "3"
	head.CertifiedFloor.Revision = "2"
	payload, err = json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeNativeTBTCSignerStateAnchorTrustHead(payload); err == nil {
		t.Fatal("trust head whose certified floor is not revision one was accepted")
	}
}

func testNativeTBTCSignerStateAnchorCheckpointWire(
	generation uint64,
	seed byte,
) nativeTBTCSignerStateAnchorCheckpointWire {
	store := [32]byte{0x77}
	previous := [32]byte{seed}
	image := [32]byte{seed + 1}
	commitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		store,
		generation,
		previous,
		image,
	)
	return nativeTBTCSignerStateAnchorCheckpointWire{
		StoreFingerprint:        testNativeTBTCSignerTrustHexValue(store),
		Generation:              uint64ToCanonicalString(generation),
		PreviousStateCommitment: testNativeTBTCSignerTrustHexValue(previous),
		StateImageDigest:        testNativeTBTCSignerTrustHexValue(image),
		StateCommitment:         testNativeTBTCSignerTrustHexValue(commitment),
	}
}

func testNativeTBTCSignerTrustHex32(first byte) string {
	return testNativeTBTCSignerTrustHexValue([32]byte{first})
}

func testNativeTBTCSignerTrustHexValue(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

func testNativeTBTCSignerTrustBool(value bool) *bool {
	return &value
}

func uint64ToCanonicalString(value uint64) string {
	// Keep this test helper independent from the decoder under test.
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = digits[value%10]
		value /= 10
	}
	return string(buffer[index:])
}
