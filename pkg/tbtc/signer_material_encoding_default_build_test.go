//go:build !frost_native

package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestUnmarshalSignerMaterialFromPersistence_LegacyEncoding_DefaultBuildReturnsLegacySignerMaterial(
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
		t.Fatal("expected private key share")
	}

	resolvedPrivateKeyShare, ok := unmarshaledSignerMaterial.signerMaterial.(*tecdsa.PrivateKeyShare)
	if !ok {
		t.Fatalf(
			"unexpected signer material type\nexpected: [%T]\nactual:   [%T]",
			&tecdsa.PrivateKeyShare{},
			unmarshaledSignerMaterial.signerMaterial,
		)
	}

	if resolvedPrivateKeyShare != unmarshaledSignerMaterial.privateKeyShare {
		t.Fatal("expected signer material to reference recovered private key share")
	}
}
