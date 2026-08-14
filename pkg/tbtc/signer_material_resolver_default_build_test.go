//go:build !frost_native

package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestRegisterSignerMaterialResolverForBuild_DefaultBuildNoop(t *testing.T) {
	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	err := RegisterSignerMaterialResolverForBuild()
	if err != nil {
		t.Fatalf("unexpected build resolver registration error: [%v]", err)
	}

	privateKeyShare := createMockSigner(t).privateKeyShare
	result, err := resolveSignerMaterial(privateKeyShare)
	if err != nil {
		t.Fatalf("unexpected resolver error: [%v]", err)
	}

	resolvedPrivateKeyShare, ok := result.(*tecdsa.PrivateKeyShare)
	if !ok {
		t.Fatalf(
			"unexpected resolved signer material type\nexpected: [%T]\nactual:   [%T]",
			&tecdsa.PrivateKeyShare{},
			result,
		)
	}

	if resolvedPrivateKeyShare != privateKeyShare {
		t.Fatalf(
			"unexpected resolved private key share\nexpected: [%v]\nactual:   [%v]",
			privateKeyShare,
			resolvedPrivateKeyShare,
		)
	}
}
