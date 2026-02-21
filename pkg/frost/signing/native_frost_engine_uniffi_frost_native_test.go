//go:build frost_native

package signing

import (
	"bytes"
	"errors"
	"testing"
)

type mockUniFFINativeFROSTBridge struct {
	generateNoncesAndCommitmentsFn func(
		keyPackageIdentifier string,
		keyPackageData []byte,
	) ([]byte, string, []byte, error)
	newSigningPackageFn func(
		message []byte,
		commitments []uniFFINativeFROSTCommitment,
	) ([]byte, error)
	signFn func(
		signingPackageData []byte,
		noncesData []byte,
		keyPackageIdentifier string,
		keyPackageData []byte,
	) (string, []byte, error)
	aggregateFn func(
		signingPackageData []byte,
		signatureShares []uniFFINativeFROSTSignatureShare,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
	) ([]byte, error)
}

func (munfsb *mockUniFFINativeFROSTBridge) GenerateNoncesAndCommitments(
	keyPackageIdentifier string,
	keyPackageData []byte,
) ([]byte, string, []byte, error) {
	return munfsb.generateNoncesAndCommitmentsFn(
		keyPackageIdentifier,
		keyPackageData,
	)
}

func (munfsb *mockUniFFINativeFROSTBridge) NewSigningPackage(
	message []byte,
	commitments []uniFFINativeFROSTCommitment,
) ([]byte, error) {
	return munfsb.newSigningPackageFn(message, commitments)
}

func (munfsb *mockUniFFINativeFROSTBridge) Sign(
	signingPackageData []byte,
	noncesData []byte,
	keyPackageIdentifier string,
	keyPackageData []byte,
) (string, []byte, error) {
	return munfsb.signFn(
		signingPackageData,
		noncesData,
		keyPackageIdentifier,
		keyPackageData,
	)
}

func (munfsb *mockUniFFINativeFROSTBridge) Aggregate(
	signingPackageData []byte,
	signatureShares []uniFFINativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) ([]byte, error) {
	return munfsb.aggregateFn(signingPackageData, signatureShares, publicKeyPackage)
}

func TestNewUniFFINativeFROSTSigningEngine_NilBridge(t *testing.T) {
	_, err := newUniFFINativeFROSTSigningEngine(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUniFFINativeFROSTSigningEngine_GenerateNoncesAndCommitments(t *testing.T) {
	var capturedIdentifier string
	var capturedData []byte

	engine, err := newUniFFINativeFROSTSigningEngine(&mockUniFFINativeFROSTBridge{
		generateNoncesAndCommitmentsFn: func(
			keyPackageIdentifier string,
			keyPackageData []byte,
		) ([]byte, string, []byte, error) {
			capturedIdentifier = keyPackageIdentifier
			capturedData = append([]byte{}, keyPackageData...)
			return []byte{0x01, 0x02}, "id-1", []byte{0x03, 0x04}, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: [%v]", err)
	}

	nonces, commitment, err := engine.GenerateNoncesAndCommitments(
		&NativeFROSTKeyPackage{
			Identifier: "member-1",
			Data:       []byte{0xaa, 0xbb},
		},
	)
	if err != nil {
		t.Fatalf("unexpected generation error: [%v]", err)
	}

	if capturedIdentifier != "member-1" {
		t.Fatalf(
			"unexpected key package identifier\nexpected: [%v]\nactual:   [%v]",
			"member-1",
			capturedIdentifier,
		)
	}

	if !bytes.Equal(capturedData, []byte{0xaa, 0xbb}) {
		t.Fatalf(
			"unexpected key package data\nexpected: [%x]\nactual:   [%x]",
			[]byte{0xaa, 0xbb},
			capturedData,
		)
	}

	if !bytes.Equal(nonces.Data, []byte{0x01, 0x02}) {
		t.Fatalf(
			"unexpected nonces data\nexpected: [%x]\nactual:   [%x]",
			[]byte{0x01, 0x02},
			nonces.Data,
		)
	}

	if commitment.Identifier != "id-1" {
		t.Fatalf(
			"unexpected commitment identifier\nexpected: [%v]\nactual:   [%v]",
			"id-1",
			commitment.Identifier,
		)
	}

	if !bytes.Equal(commitment.Data, []byte{0x03, 0x04}) {
		t.Fatalf(
			"unexpected commitment data\nexpected: [%x]\nactual:   [%x]",
			[]byte{0x03, 0x04},
			commitment.Data,
		)
	}
}

func TestUniFFINativeFROSTSigningEngine_SignAndAggregate(t *testing.T) {
	expectedErr := errors.New("aggregate error")

	engine, err := newUniFFINativeFROSTSigningEngine(&mockUniFFINativeFROSTBridge{
		generateNoncesAndCommitmentsFn: func(
			keyPackageIdentifier string,
			keyPackageData []byte,
		) ([]byte, string, []byte, error) {
			return nil, "", nil, nil
		},
		newSigningPackageFn: func(
			message []byte,
			commitments []uniFFINativeFROSTCommitment,
		) ([]byte, error) {
			return []byte{0x01}, nil
		},
		signFn: func(
			signingPackageData []byte,
			noncesData []byte,
			keyPackageIdentifier string,
			keyPackageData []byte,
		) (string, []byte, error) {
			return "member-1", []byte{0x99}, nil
		},
		aggregateFn: func(
			signingPackageData []byte,
			signatureShares []uniFFINativeFROSTSignatureShare,
			publicKeyPackage *NativeFROSTPublicKeyPackage,
		) ([]byte, error) {
			return nil, expectedErr
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: [%v]", err)
	}

	signingPackage, err := engine.NewSigningPackage(
		[]byte{0xab},
		[]*NativeFROSTCommitment{
			{
				Identifier: "member-1",
				Data:       []byte{0x11},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected signing package error: [%v]", err)
	}

	signatureShare, err := engine.Sign(
		signingPackage,
		&NativeFROSTNonces{
			Data: []byte{0x22},
		},
		&NativeFROSTKeyPackage{
			Identifier: "member-1",
			Data:       []byte{0x33},
		},
	)
	if err != nil {
		t.Fatalf("unexpected sign error: [%v]", err)
	}

	if signatureShare.Identifier != "member-1" {
		t.Fatalf(
			"unexpected signature share identifier\nexpected: [%v]\nactual:   [%v]",
			"member-1",
			signatureShare.Identifier,
		)
	}

	if !bytes.Equal(signatureShare.Data, []byte{0x99}) {
		t.Fatalf(
			"unexpected signature share data\nexpected: [%x]\nactual:   [%x]",
			[]byte{0x99},
			signatureShare.Data,
		)
	}

	_, err = engine.Aggregate(
		signingPackage,
		[]*NativeFROSTSignatureShare{
			signatureShare,
		},
		&NativeFROSTPublicKeyPackage{
			VerifyingShares: map[string]string{
				"member-1": "share-1",
			},
			VerifyingKey: "pubkey",
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"unexpected aggregate error\nexpected: [%v]\nactual:   [%v]",
			expectedErr,
			err,
		)
	}
}
