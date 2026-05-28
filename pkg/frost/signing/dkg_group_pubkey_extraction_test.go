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

func TestExtractDkgGroupPublicKey_FrostUniFFIV2_HexDecodes(t *testing.T) {
	const hexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	payload, err := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: hexKey,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}
	got, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want, _ := hex.DecodeString(hexKey)
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"hex decode mismatch: got %x, want %x",
			got, want,
		)
	}
	if len(got) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(got))
	}
}

func TestExtractDkgGroupPublicKey_FrostUniFFIV2_RejectsEmptyVerifyingKey(t *testing.T) {
	payload, _ := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: "",
		},
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}
	// The pre-existing decodeNativeFROSTUniFFIV2SignerMaterial
	// validator may reject this before our helper sees it; either
	// way an error must be returned.
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err == nil {
		t.Fatal("expected error for empty VerifyingKey")
	}
}

func TestExtractDkgGroupPublicKey_FrostUniFFIV2_RejectsNonHexVerifyingKey(t *testing.T) {
	payload, _ := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: "not-hex-zzz!",
		},
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err == nil {
		t.Fatal("expected error for non-hex VerifyingKey")
	}
	if !strings.Contains(err.Error(), "not hex") {
		t.Fatalf("error must mention hex problem; got %v", err)
	}
}

func TestExtractDkgGroupPublicKey_FrostUniFFIV2_RejectsWrongLength(t *testing.T) {
	payload, _ := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: strings.Repeat("11", 31),
		},
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}
	_, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err == nil {
		t.Fatal("expected error for wrong-length VerifyingKey")
	}
	if !strings.Contains(err.Error(), "must be 32 bytes") {
		t.Fatalf("error must mention length problem; got %v", err)
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

func TestExtractDkgGroupPublicKey_FrostUniFFIV2_GoldenFixture(t *testing.T) {
	// Lock the canonical byte output for a specific hex input. If a
	// future change to extractDkgGroupPublicKeyFromUniFFIV2 alters
	// the result, this test catches the drift at code review.
	const hexKey = "deadbeefcafebabe0000000000000000000000000000000000000000000000ff"
	payload, _ := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "fixture",
			Data:       []byte{0xFF},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: hexKey,
		},
	})
	mat := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}
	got, err := ExtractDkgGroupPublicKeyFromMaterial(mat)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want, _ := hex.DecodeString(hexKey)
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"golden fixture mismatch: got %x, want %x",
			got, want,
		)
	}
}
