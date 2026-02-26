//go:build frost_native

package tbtc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"google.golang.org/protobuf/proto"
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

	if nativeSignerMaterial.Format != frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
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

	if !bytes.Equal(actualPayload, legacyEncoded) {
		t.Fatalf(
			"unexpected resolved signer payload\nexpected: [%x]\nactual:   [%x]",
			legacyEncoded,
			actualPayload,
		)
	}
}

func TestSignerMarshalling_LegacyRoundtripMigratesToNativeEnvelopeOnFrostNativeBuild(
	t *testing.T,
) {
	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	if err := RegisterSignerMaterialResolverForBuild(); err != nil {
		t.Fatalf("unexpected build resolver registration error: [%v]", err)
	}

	legacySigner := createMockSigner(t)
	legacySigner.signerMaterial = legacySigner.privateKeyShare

	initialEncodedSigner, err := legacySigner.Marshal()
	if err != nil {
		t.Fatalf("unexpected initial signer marshal error: [%v]", err)
	}

	initialPBSigner := &pb.Signer{}
	if err := proto.Unmarshal(initialEncodedSigner, initialPBSigner); err != nil {
		t.Fatalf("unexpected initial proto unmarshal error: [%v]", err)
	}

	if bytes.HasPrefix(initialPBSigner.PrivateKeyShare, signerMaterialEnvelopePrefix) {
		t.Fatal("expected initial legacy signer encoding without native envelope")
	}

	unmarshaledSigner := &signer{}
	if err := unmarshaledSigner.Unmarshal(initialEncodedSigner); err != nil {
		t.Fatalf("unexpected signer unmarshal error: [%v]", err)
	}

	if _, ok := unmarshaledSigner.signerMaterial.(*frostsigning.NativeSignerMaterial); !ok {
		t.Fatalf(
			"unexpected signer material type after legacy unmarshal\nexpected: [%T]\nactual:   [%T]",
			&frostsigning.NativeSignerMaterial{},
			unmarshaledSigner.signerMaterial,
		)
	}

	migratedEncodedSigner, err := unmarshaledSigner.Marshal()
	if err != nil {
		t.Fatalf("unexpected migrated signer marshal error: [%v]", err)
	}

	migratedPBSigner := &pb.Signer{}
	if err := proto.Unmarshal(migratedEncodedSigner, migratedPBSigner); err != nil {
		t.Fatalf("unexpected migrated proto unmarshal error: [%v]", err)
	}

	if !bytes.HasPrefix(migratedPBSigner.PrivateKeyShare, signerMaterialEnvelopePrefix) {
		t.Fatal("expected migrated signer encoding with native envelope prefix")
	}
}
