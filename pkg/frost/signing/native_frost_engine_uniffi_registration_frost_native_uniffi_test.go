//go:build frost_native && frost_uniffi_sdk && cgo

package signing

import (
	"testing"

	frostuniffi "github.com/zecdev/frost-uniffi-sdk/frost_go_ffi"
)

func TestBuildTaggedUniFFINativeFROSTBridge_EndToEndSigning(t *testing.T) {
	engine, err := newUniFFINativeFROSTSigningEngine(
		&buildTaggedUniFFINativeFROSTBridge{},
	)
	if err != nil {
		t.Fatalf("unexpected engine constructor error: [%v]", err)
	}

	keygen, err := frostuniffi.TrustedDealerKeygenFrom(
		frostuniffi.Configuration{
			MinSigners: 2,
			MaxSigners: 2,
			Secret:     []byte{},
		},
	)
	if err != nil {
		t.Fatalf("cannot generate trusted dealer key shares: [%v]", err)
	}

	keyPackages := make([]*NativeFROSTKeyPackage, 0, len(keygen.SecretShares))
	for _, secretShare := range keygen.SecretShares {
		keyPackage, err := frostuniffi.VerifyAndGetKeyPackageFrom(secretShare)
		if err != nil {
			t.Fatalf("cannot verify key package from secret share: [%v]", err)
		}

		keyPackages = append(
			keyPackages,
			&NativeFROSTKeyPackage{
				Identifier: keyPackage.Identifier.Data,
				Data:       append([]byte{}, keyPackage.Data...),
			},
		)
	}

	if len(keyPackages) != 2 {
		t.Fatalf(
			"unexpected key package count\nexpected: [%v]\nactual:   [%v]",
			2,
			len(keyPackages),
		)
	}

	nonces := make([]*NativeFROSTNonces, 0, len(keyPackages))
	commitments := make([]*NativeFROSTCommitment, 0, len(keyPackages))
	for _, keyPackage := range keyPackages {
		generatedNonces, generatedCommitment, err := engine.GenerateNoncesAndCommitments(
			keyPackage,
		)
		if err != nil {
			t.Fatalf("cannot generate nonces and commitments: [%v]", err)
		}

		nonces = append(nonces, generatedNonces)
		commitments = append(commitments, generatedCommitment)
	}

	message := []byte("keep-core uniffi bridge integration test")
	signingPackage, err := engine.NewSigningPackage(message, commitments)
	if err != nil {
		t.Fatalf("cannot build signing package: [%v]", err)
	}

	signatureShares := make([]*NativeFROSTSignatureShare, 0, len(keyPackages))
	for i, keyPackage := range keyPackages {
		signatureShare, err := engine.Sign(signingPackage, nonces[i], keyPackage)
		if err != nil {
			t.Fatalf("cannot produce signature share: [%v]", err)
		}

		signatureShares = append(signatureShares, signatureShare)
	}

	verifyingShares := make(map[string]string, len(keygen.PublicKeyPackage.VerifyingShares))
	for identifier, verifyingShare := range keygen.PublicKeyPackage.VerifyingShares {
		verifyingShares[identifier.Data] = verifyingShare
	}

	signatureBytes, err := engine.Aggregate(
		signingPackage,
		signatureShares,
		&NativeFROSTPublicKeyPackage{
			VerifyingShares: verifyingShares,
			VerifyingKey:    keygen.PublicKeyPackage.VerifyingKey,
		},
	)
	if err != nil {
		t.Fatalf("cannot aggregate signature shares: [%v]", err)
	}

	err = frostuniffi.VerifySignature(
		frostuniffi.Message{
			Data: message,
		},
		frostuniffi.FrostSignature{
			Data: signatureBytes,
		},
		keygen.PublicKeyPackage,
	)
	if err != nil {
		t.Fatalf("cannot verify aggregated signature: [%v]", err)
	}
}
