package signing

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestRecordNativeTBTCSignerInstalledStateAnchorConfig(t *testing.T) {
	resetInstalledNativeTBTCSignerStateAnchorConfigForTest()
	t.Cleanup(resetInstalledNativeTBTCSignerStateAnchorConfigForTest)

	config := testNativeTBTCSignerInstalledAnchorConfig(1024)
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		config,
		"config-fingerprint",
	); err != nil {
		t.Fatalf("valid installed anchor config was rejected: %v", err)
	}
	result, err := ReadInstalledNativeTBTCSignerStateAnchorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if result.ProtocolID[0] != 0x01 ||
		result.StreamID[0] != 0x02 ||
		result.ActivationManifestHash[0] != 0x03 ||
		result.ActivationManifestSequence != 7 ||
		result.BindingHash[0] != 0x04 ||
		result.ResponsePublicKey[0] != 0x05 ||
		result.OfflineAuthorityPublicKey[0] != 0x06 ||
		result.TrustCertificateSequence != 9 ||
		result.TrustCertificateDigest[0] != 0x07 ||
		result.WitnessMaximumRecords != 1024 ||
		result.WitnessRotationThresholdRecords != 128 ||
		result.ConfigFingerprint != "config-fingerprint" {
		t.Fatalf("unexpected installed anchor config: %+v", result)
	}
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		config,
		"config-fingerprint",
	); err != nil {
		t.Fatalf("identical config reinstall was rejected: %v", err)
	}
}

func TestRecordNativeTBTCSignerInstalledStateAnchorConfigRejectsPartialOrChanged(
	t *testing.T,
) {
	resetInstalledNativeTBTCSignerStateAnchorConfigForTest()
	t.Cleanup(resetInstalledNativeTBTCSignerStateAnchorConfigForTest)

	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		[]byte(`{"state_witness_max_records":1024}`),
		"config-fingerprint",
	); err == nil {
		t.Fatal("partial installed anchor config was accepted")
	}

	valid := testNativeTBTCSignerInstalledAnchorConfig(1024)
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		valid,
		"config-fingerprint",
	); err != nil {
		t.Fatal(err)
	}
	changed := []byte(strings.Replace(
		string(valid),
		`"state_witness_max_records": 1024`,
		`"state_witness_max_records": 2048`,
		1,
	))
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		changed,
		"config-fingerprint-2",
	); err == nil {
		t.Fatal("conflicting installed anchor config was accepted")
	}
}

func TestRecordNativeTBTCSignerInstalledStateAnchorConfigAcceptsBootstrapProvisioning(
	t *testing.T,
) {
	resetInstalledNativeTBTCSignerStateAnchorConfigForTest()
	t.Cleanup(resetInstalledNativeTBTCSignerStateAnchorConfigForTest)

	config := []byte(`{
		"purpose": "state_anchor_bootstrap_provisioning",
		"profile": "production",
		"state_path": "/var/lib/keep-client/tbtc-signer",
		"state_witness_max_records": 4
	}`)
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		config,
		"bootstrap-fingerprint",
	); err != nil {
		t.Fatalf("bootstrap provisioning config was rejected: %v", err)
	}
	if _, err := ReadInstalledNativeTBTCSignerStateAnchorConfig(); err == nil {
		t.Fatal("bootstrap provisioning installed runtime anchor authority")
	}
}

func TestRecordNativeTBTCSignerInstalledStateAnchorConfigRejectsProvisioningAuthority(
	t *testing.T,
) {
	resetInstalledNativeTBTCSignerStateAnchorConfigForTest()
	t.Cleanup(resetInstalledNativeTBTCSignerStateAnchorConfigForTest)

	for _, config := range [][]byte{
		[]byte(`{
			"purpose": "state_anchor_bootstrap_provisioning",
			"profile": "production",
			"state_path": "/var/lib/keep-client/tbtc-signer",
			"state_witness_max_records": 5
		}`),
		[]byte(`{
			"purpose": "state_anchor_bootstrap_provisioning",
			"profile": "production",
			"state_path": "/var/lib/keep-client/tbtc-signer",
			"state_witness_max_records": 4,
			"state_anchor_binding_hash": "0x0100000000000000000000000000000000000000000000000000000000000000"
		}`),
	} {
		if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
			config,
			"bootstrap-fingerprint",
		); err == nil {
			t.Fatalf(
				"bootstrap provisioning authority or invalid witness bound was accepted: %s",
				config,
			)
		}
	}
}

func testNativeTBTCSignerInstalledAnchorConfig(maximum uint64) []byte {
	bytes32 := func(first byte) [32]byte {
		result := [32]byte{}
		result[0] = first
		return result
	}
	hex32 := func(value [32]byte) string {
		return "0x" + hex.EncodeToString(value[:])
	}
	responseKey := bytes32(0x05)
	authorityKey := bytes32(0x06)
	return []byte(fmt.Sprintf(`{
		"state_anchor_protocol_id": %q,
		"state_anchor_stream_id": %q,
		"state_anchor_activation_manifest_hash": %q,
		"state_anchor_activation_manifest_sequence": 7,
		"state_witness_max_records": %d,
		"state_anchor_binding_hash": %q,
		"state_anchor_response_public_key": %q,
		"state_anchor_response_public_key_spki_sha256": %q,
		"state_anchor_offline_authority_public_key": %q,
		"state_anchor_offline_authority_public_key_spki_sha256": %q,
		"state_anchor_trust_certificate_sequence": 9,
		"state_anchor_trust_certificate_digest": %q,
		"state_witness_rotation_threshold_records": 128
	}`,
		hex32(bytes32(0x01)),
		hex32(bytes32(0x02)),
		hex32(bytes32(0x03)),
		maximum,
		hex32(bytes32(0x04)),
		hex32(responseKey),
		hex32(nativeTBTCSignerEd25519SPKISHA256(responseKey)),
		hex32(authorityKey),
		hex32(nativeTBTCSignerEd25519SPKISHA256(authorityKey)),
		hex32(bytes32(0x07)),
	))
}

func resetInstalledNativeTBTCSignerStateAnchorConfigForTest() {
	nativeTBTCSignerInstalledStateAnchorConfig.Lock()
	defer nativeTBTCSignerInstalledStateAnchorConfig.Unlock()
	nativeTBTCSignerInstalledStateAnchorConfig.value = nil
}
