//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestRegisterSignerMaterialResolverForBuild_DefaultProviderRefusesScaffoldWithoutOptIn
// asserts the cgo frost_tbtc_signer resolver REFUSES to surface scaffold-era
// signer material unless the operator opts in via AcceptScaffoldKeyGroupEnvVar.
//
// This is cgo-only behaviour: only the frost_tbtc_signer (cgo) resolver carries
// the resolver-level refusal guard. The non-cgo frost_native resolver
// intentionally PERMITS scaffold resolution -- it is the transitional build, and
// the deeper native_ffi_primitive guard refuses scaffold material at signing
// time instead. The migration tests in signer_material_encoding_frost_native_test.go
// (tagged for the non-cgo build) assert that permissive behaviour. This test
// therefore lives behind the cgo tag so it exercises the resolver whose contract
// it describes; previously it was tagged plain frost_native and failed under any
// non-cgo frost_native build.
func TestRegisterSignerMaterialResolverForBuild_DefaultProviderRefusesScaffoldWithoutOptIn(
	t *testing.T,
) {
	// Force the env var to "" so a stray external value cannot suppress the
	// scaffold refusal during this regression test.
	t.Setenv(frostsigning.AcceptScaffoldKeyGroupEnvVar, "")

	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	err := RegisterSignerMaterialResolverForBuild()
	if err != nil {
		t.Fatalf("unexpected build resolver registration error: [%v]", err)
	}

	privateKeyShare := createMockSigner(t).privateKeyShare
	_, err = resolveSignerMaterial(privateKeyShare)
	if err == nil {
		t.Fatal(
			"expected scaffold-refusal error from default resolver without opt-in",
		)
	}

	if !strings.Contains(err.Error(), frostsigning.AcceptScaffoldKeyGroupEnvVar) {
		t.Fatalf(
			"expected scaffold-refusal error to reference %s; got: [%v]",
			frostsigning.AcceptScaffoldKeyGroupEnvVar,
			err,
		)
	}
	if !strings.Contains(err.Error(), frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey) {
		t.Fatalf(
			"expected scaffold-refusal error to reference %s; got: [%v]",
			frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
			err,
		)
	}
}
