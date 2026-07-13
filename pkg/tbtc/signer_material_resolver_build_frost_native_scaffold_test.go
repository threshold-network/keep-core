//go:build frost_native && !(frost_tbtc_signer && cgo)

package tbtc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestRegisterSignerMaterialResolverForBuild_UsesDefaultProvider(
	t *testing.T,
) {
	// This transitional non-cgo resolver builds legacy-wallet-pubkey signer
	// material. The real cgo activation build keeps legacy shares on the legacy
	// bridge and has separate coverage for that contract.
	t.Setenv(frostsigning.AcceptScaffoldKeyGroupEnvVar, "true")

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

	nativeSignerMaterial, ok := result.(*frostsigning.NativeSignerMaterial)
	if !ok {
		t.Fatalf(
			"unexpected resolved signer material type\nexpected: [%T]\nactual:   [%T]",
			&frostsigning.NativeSignerMaterial{},
			result,
		)
	}

	expectedPayload, err := privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling expected private key share: [%v]", err)
	}

	if nativeSignerMaterial.Format != frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1 {
		t.Fatalf(
			"unexpected native signer material format\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			nativeSignerMaterial.Format,
		)
	}

	var payload frostsigning.NativeTBTCSignerMaterialPayload
	if err := json.Unmarshal(nativeSignerMaterial.Payload, &payload); err != nil {
		t.Fatalf("failed unmarshalling tbtc signer material payload: [%v]", err)
	}

	if payload.KeyGroup == "" {
		t.Fatal("expected non-empty tbtc-signer key group")
	}

	if payload.KeyGroupSource == "" {
		t.Fatal("expected non-empty tbtc-signer key group source")
	}

	legacyPrivateKeySharePayload, err := hex.DecodeString(payload.LegacyPrivateKeyShareHex)
	if err != nil {
		t.Fatalf("failed decoding legacy private key share payload: [%v]", err)
	}

	decodedPrivateKeyShare := &tecdsa.PrivateKeyShare{}
	if err := decodedPrivateKeyShare.Unmarshal(legacyPrivateKeySharePayload); err != nil {
		t.Fatalf("failed unmarshalling decoded private key share: [%v]", err)
	}

	actualPayload, err := decodedPrivateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling decoded private key share: [%v]", err)
	}

	if !bytes.Equal(expectedPayload, actualPayload) {
		t.Fatalf(
			"unexpected resolved signer payload\nexpected: [%x]\nactual:   [%x]",
			expectedPayload,
			actualPayload,
		)
	}
}
