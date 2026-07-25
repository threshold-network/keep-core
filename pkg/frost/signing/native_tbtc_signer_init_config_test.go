package signing

import (
	"strings"
	"testing"
)

func TestRecordNativeTBTCSignerInstalledStateAnchorConfig(t *testing.T) {
	resetInstalledNativeTBTCSignerStateAnchorConfigForTest()
	t.Cleanup(resetInstalledNativeTBTCSignerStateAnchorConfigForTest)

	hash := func(value string) string {
		return "0x" + strings.Repeat(value, 64)
	}
	config := []byte(`{
		"state_witness_max_records": 1024,
		"state_anchor_binding_hash": "` + hash("1") + `",
		"state_anchor_response_public_key": "` + hash("2") + `",
		"state_anchor_response_public_key_spki_sha256": "` + hash("3") + `",
		"state_witness_rotation_threshold_records": 128
	}`)
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
	if result.BindingHash[0] != 0x11 ||
		result.ResponsePublicKey[0] != 0x22 ||
		result.ResponsePublicKeySPKISHA256[0] != 0x33 ||
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

	hash := func(value string) string {
		return "0x" + strings.Repeat(value, 64)
	}
	valid := []byte(`{
		"state_witness_max_records": 1024,
		"state_anchor_binding_hash": "` + hash("1") + `",
		"state_anchor_response_public_key": "` + hash("2") + `",
		"state_anchor_response_public_key_spki_sha256": "` + hash("3") + `",
		"state_witness_rotation_threshold_records": 128
	}`)
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

func resetInstalledNativeTBTCSignerStateAnchorConfigForTest() {
	nativeTBTCSignerInstalledStateAnchorConfig.Lock()
	defer nativeTBTCSignerInstalledStateAnchorConfig.Unlock()
	nativeTBTCSignerInstalledStateAnchorConfig.value = nil
}
