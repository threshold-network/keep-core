//go:build frost_native

package signing

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestExtractDkgGroupPublicKey_RejectsNilMaterial(t *testing.T) {
	_, err := ExtractDkgGroupPublicKeyFromMaterial(nil)
	if err == nil {
		t.Fatal("expected error for nil material")
	}
}

func TestExtractDkgGroupPublicKey_FrostUniFFIV2_ReturnsUnsupportedSentinel(t *testing.T) {
	payload := unsupportedUniFFIV2Payload(
		t,
		"0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	)
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if !errors.Is(err, ErrUnsupportedSignerMaterialFormat) {
		t.Fatalf("expected ErrUnsupportedSignerMaterialFormat, got %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error must mention unsupported format; got %v", err)
	}
}

func TestExtractTaprootOutputKey_FrostUniFFIV2_ReturnsUnsupported(t *testing.T) {
	payload := unsupportedUniFFIV2Payload(
		t,
		"0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	)
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}

	_, err := ExtractTaprootOutputKeyFromMaterial(mat)
	if err == nil {
		t.Fatal("expected unsupported V2 taproot output key rejection")
	}
	if !strings.Contains(err.Error(), "unsupported signer-material format") {
		t.Fatalf("error must mention unsupported format; got %v", err)
	}
}

func TestExtractDkgGroupPublicKey_FrostTBTCSignerV1_ReturnsKeyGroupBytes(t *testing.T) {
	const keyGroup = "group-A"
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: keyGroup,
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}
	got, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if string(got) != keyGroup {
		t.Fatalf("got %q, want %q", string(got), keyGroup)
	}
}

func TestExtractTaprootOutputKey_FrostTBTCSignerV1_DKGPersistedHexDecodes(
	t *testing.T,
) {
	const hexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup:       hexKey,
		KeyGroupSource: NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	got, err := ExtractTaprootOutputKeyFromMaterial(mat)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want, _ := hex.DecodeString(hexKey)
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"taproot output key mismatch: got %x, want %x",
			got,
			want,
		)
	}
}

func TestExtractTaprootOutputKey_FrostTBTCSignerV1_DKGPersistedCompressedKeyGroup(
	t *testing.T,
) {
	const compressedKey = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	const xOnlyKey = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"

	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup:       compressedKey,
		KeyGroupSource: NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	got, err := ExtractTaprootOutputKeyFromMaterial(mat)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want, _ := hex.DecodeString(xOnlyKey)
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"taproot output key mismatch: got %x, want %x",
			got,
			want,
		)
	}
}

func TestExtractTaprootOutputKey_FrostTBTCSignerV1_DKGPersistedUsesExplicitOutputKey(
	t *testing.T,
) {
	const compressedKey = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	const outputKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup:         compressedKey,
		TaprootOutputKey: outputKey,
		KeyGroupSource:   NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	got, err := ExtractTaprootOutputKeyFromMaterial(mat)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want, _ := hex.DecodeString(outputKey)
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"taproot output key mismatch: got %x, want %x",
			got,
			want,
		)
	}
}

func TestExtractTaprootOutputKey_FrostTBTCSignerV1_RejectsNonDKGSource(
	t *testing.T,
) {
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup:       strings.Repeat("11", 32),
		KeyGroupSource: NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	_, err := ExtractTaprootOutputKeyFromMaterial(mat)
	if err == nil {
		t.Fatal("expected non-DKG source rejection")
	}
	if !strings.Contains(err.Error(), NativeTBTCSignerKeyGroupSourceDKGPersisted) {
		t.Fatalf("error should mention persisted DKG source: [%v]", err)
	}
}

func TestExtractDkgGroupPublicKey_FrostTBTCSignerV1_DeterministicAcrossCalls(t *testing.T) {
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: "deterministic-group",
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}
	a, _ := ExtractDkgGroupPublicKeyFromMaterial(mat)
	b, _ := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if !bytes.Equal(a, b) {
		t.Fatalf("extraction is non-deterministic: %x vs %x", a, b)
	}
}

func TestExtractDkgGroupPublicKey_FrostTBTCSignerV1_RejectsEmptyKeyGroup(t *testing.T) {
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: "",
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err == nil {
		t.Fatal("expected error for empty KeyGroup")
	}
}

func TestExtractDkgGroupPublicKey_FrostUniFFIV1_ReturnsUnsupportedSentinel(t *testing.T) {
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV1,
		Payload: []byte("{}"),
	}
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if !errors.Is(err, ErrUnsupportedSignerMaterialFormat) {
		t.Fatalf("expected ErrUnsupportedSignerMaterialFormat, got %v", err)
	}
	if !strings.Contains(err.Error(), "migrate to") {
		t.Fatalf("error must guide operator to migration; got %v", err)
	}
}

func TestExtractDkgGroupPublicKey_UnknownFormat_ReturnsUnsupportedSentinel(t *testing.T) {
	mat := &NativeSignerMaterial{
		Format:  "frost-some-future-format-v0",
		Payload: []byte("{}"),
	}
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if !errors.Is(err, ErrUnsupportedSignerMaterialFormat) {
		t.Fatalf("expected ErrUnsupportedSignerMaterialFormat, got %v", err)
	}
	if !strings.Contains(err.Error(), "frost-some-future-format-v0") {
		t.Fatalf("error must mention the unknown format; got %v", err)
	}
}
