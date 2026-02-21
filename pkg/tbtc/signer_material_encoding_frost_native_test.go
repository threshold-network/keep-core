//go:build frost_native

package tbtc

import (
	"bytes"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestUnmarshalSignerMaterialFromPersistence_LegacyEncodingResolvesNativeMaterialOnFrostNativeBuild(
	t *testing.T,
) {
	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	if err := RegisterSignerMaterialResolverForBuild(); err != nil {
		t.Fatalf("unexpected build resolver registration error: [%v]", err)
	}

	privateKeyShare := createMockSigner(t).privateKeyShare
	legacyEncoded, err := privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling legacy private key share: [%v]", err)
	}

	unmarshaledSignerMaterial, err := unmarshalSignerMaterialFromPersistence(
		legacyEncoded,
	)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: [%v]", err)
	}

	if unmarshaledSignerMaterial.privateKeyShare == nil {
		t.Fatal("expected legacy private key share to be preserved")
	}

	nativeSignerMaterial, ok := unmarshaledSignerMaterial.signerMaterial.(*frostsigning.NativeSignerMaterial)
	if !ok {
		t.Fatalf(
			"unexpected resolved signer material type\nexpected: [%T]\nactual:   [%T]",
			&frostsigning.NativeSignerMaterial{},
			unmarshaledSignerMaterial.signerMaterial,
		)
	}

	if nativeSignerMaterial.Format != frostsigning.NativeSignerMaterialFormatFrostUniFFIV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
			frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			nativeSignerMaterial.Format,
		)
	}

	decodedPrivateKeyShare := &tecdsa.PrivateKeyShare{}
	if err := decodedPrivateKeyShare.Unmarshal(nativeSignerMaterial.Payload); err != nil {
		t.Fatalf("failed unmarshalling native signer material payload: [%v]", err)
	}

	actualPayload, err := decodedPrivateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling decoded private key share: [%v]", err)
	}

	if !bytes.Equal(actualPayload, legacyEncoded) {
		t.Fatalf(
			"unexpected resolved signer payload\nexpected: [%x]\nactual:   [%x]",
			legacyEncoded,
			actualPayload,
		)
	}
}
