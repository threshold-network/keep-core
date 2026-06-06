//go:build frost_native

package tbtc

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestCalculateWalletIDForSigner_FrostUniFFIV2UsesXOnlyOutputKey(t *testing.T) {
	const xOnlyOutputKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	payload, err := json.Marshal(struct {
		KeyPackage       *frostsigning.NativeFROSTKeyPackage       `json:"keyPackage"`
		PublicKeyPackage *frostsigning.NativeFROSTPublicKeyPackage `json:"publicKeyPackage"`
	}{
		KeyPackage: &frostsigning.NativeFROSTKeyPackage{
			Identifier: "member-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &frostsigning.NativeFROSTPublicKeyPackage{
			VerifyingKey: xOnlyOutputKey,
		},
	})
	if err != nil {
		t.Fatalf("unexpected payload marshal error: [%v]", err)
	}

	signer := createMockSigner(t)
	signer.signerMaterial = &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}

	walletID, err := calculateWalletIDForSigner(
		signer,
		func(_ *ecdsa.PublicKey) ([32]byte, error) {
			return [32]byte{0xff}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected wallet ID calculation error: [%v]", err)
	}

	var expectedWalletID [32]byte
	expectedBytes, err := hex.DecodeString(xOnlyOutputKey)
	if err != nil {
		t.Fatalf("unexpected hex decode error: [%v]", err)
	}
	copy(expectedWalletID[:], expectedBytes)

	if walletID != expectedWalletID {
		t.Fatalf(
			"unexpected FROST wallet ID\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedWalletID,
			walletID,
		)
	}
}

func TestCalculateWalletIDForSigner_TBTCSignerDkgPersistedUsesXOnlyOutputKey(
	t *testing.T,
) {
	const xOnlyOutputKey = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:       xOnlyOutputKey,
		KeyGroupSource: frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	if err != nil {
		t.Fatalf("unexpected payload marshal error: [%v]", err)
	}

	signer := createMockSigner(t)
	signer.signerMaterial = &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	legacyCalculatorCalled := false
	walletID, err := calculateWalletIDForSigner(
		signer,
		func(_ *ecdsa.PublicKey) ([32]byte, error) {
			legacyCalculatorCalled = true
			return [32]byte{0xff}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected wallet ID calculation error: [%v]", err)
	}
	if legacyCalculatorCalled {
		t.Fatal("legacy wallet ID calculator should not have been called")
	}

	var expectedWalletID [32]byte
	expectedBytes, err := hex.DecodeString(xOnlyOutputKey)
	if err != nil {
		t.Fatalf("unexpected hex decode error: [%v]", err)
	}
	copy(expectedWalletID[:], expectedBytes)

	if walletID != expectedWalletID {
		t.Fatalf(
			"unexpected FROST wallet ID\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedWalletID,
			walletID,
		)
	}
}

func TestCalculateWalletIDForSigner_TBTCSignerLegacyKeyGroupSourceUsesLegacyWalletID(
	t *testing.T,
) {
	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:       "legacy-key-group",
		KeyGroupSource: frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
	})
	if err != nil {
		t.Fatalf("unexpected payload marshal error: [%v]", err)
	}

	signer := createMockSigner(t)
	signer.signerMaterial = &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	expectedWalletID := [32]byte{0x11, 0x22, 0x33}
	legacyCalculatorCalled := false
	walletID, err := calculateWalletIDForSigner(
		signer,
		func(_ *ecdsa.PublicKey) ([32]byte, error) {
			legacyCalculatorCalled = true
			return expectedWalletID, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected wallet ID calculation error: [%v]", err)
	}
	if !legacyCalculatorCalled {
		t.Fatal("legacy wallet ID calculator was not called")
	}
	if walletID != expectedWalletID {
		t.Fatalf(
			"unexpected legacy wallet ID\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedWalletID,
			walletID,
		)
	}
}
