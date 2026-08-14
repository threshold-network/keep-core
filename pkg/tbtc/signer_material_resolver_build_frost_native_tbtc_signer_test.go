//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"bytes"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"google.golang.org/protobuf/proto"
)

func TestRegisterSignerMaterialResolverForBuild_DefaultProviderPreservesLegacyShare(
	t *testing.T,
) {
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
	resolvedSignerMaterial, err := resolveSignerMaterial(privateKeyShare)
	if err != nil {
		t.Fatalf("unexpected resolver error: [%v]", err)
	}

	resolvedPrivateKeyShare, ok := resolvedSignerMaterial.(*tecdsa.PrivateKeyShare)
	if !ok {
		t.Fatalf(
			"unexpected resolved signer material type\nexpected: [%T]\nactual:   [%T]",
			&tecdsa.PrivateKeyShare{},
			resolvedSignerMaterial,
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

func TestSignerMarshalling_LegacyRoundtripStaysOnLegacyBridgeInTBTCSignerBuild(
	t *testing.T,
) {
	t.Setenv(frostsigning.AcceptScaffoldKeyGroupEnvVar, "")

	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	if err := RegisterSignerMaterialResolverForBuild(); err != nil {
		t.Fatalf("unexpected build resolver registration error: [%v]", err)
	}

	legacySigner := createMockSigner(t)
	legacySigner.signerMaterial = legacySigner.privateKeyShare

	legacyPrivateKeyShareBytes, err := legacySigner.privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("unexpected private key share marshal error: [%v]", err)
	}

	encodedSigner, err := legacySigner.Marshal()
	if err != nil {
		t.Fatalf("unexpected signer marshal error: [%v]", err)
	}

	unmarshaledSigner := &signer{}
	if err := unmarshaledSigner.Unmarshal(encodedSigner); err != nil {
		t.Fatalf("unexpected signer unmarshal error: [%v]", err)
	}

	resolvedPrivateKeyShare, ok := unmarshaledSigner.signerMaterial.(*tecdsa.PrivateKeyShare)
	if !ok {
		t.Fatalf(
			"unexpected signer material type after legacy unmarshal\nexpected: [%T]\nactual:   [%T]",
			&tecdsa.PrivateKeyShare{},
			unmarshaledSigner.signerMaterial,
		)
	}

	resolvedPrivateKeyShareBytes, err := resolvedPrivateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("unexpected resolved private key share marshal error: [%v]", err)
	}

	if !bytes.Equal(legacyPrivateKeyShareBytes, resolvedPrivateKeyShareBytes) {
		t.Fatalf(
			"unexpected resolved private key share\nexpected: [%x]\nactual:   [%x]",
			legacyPrivateKeyShareBytes,
			resolvedPrivateKeyShareBytes,
		)
	}

	roundtripEncodedSigner, err := unmarshaledSigner.Marshal()
	if err != nil {
		t.Fatalf("unexpected roundtrip signer marshal error: [%v]", err)
	}

	roundtripPBSigner := &pb.Signer{}
	if err := proto.Unmarshal(roundtripEncodedSigner, roundtripPBSigner); err != nil {
		t.Fatalf("unexpected roundtrip proto unmarshal error: [%v]", err)
	}

	if bytes.HasPrefix(roundtripPBSigner.PrivateKeyShare, signerMaterialEnvelopePrefix) {
		t.Fatal("expected legacy signer material to remain outside the native envelope")
	}

	if !bytes.Equal(legacyPrivateKeyShareBytes, roundtripPBSigner.PrivateKeyShare) {
		t.Fatalf(
			"unexpected roundtrip private key share\nexpected: [%x]\nactual:   [%x]",
			legacyPrivateKeyShareBytes,
			roundtripPBSigner.PrivateKeyShare,
		)
	}
}
