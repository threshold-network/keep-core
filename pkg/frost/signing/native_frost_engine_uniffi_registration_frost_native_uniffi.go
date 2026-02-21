//go:build frost_native && frost_uniffi_sdk && cgo

package signing

import (
	"fmt"

	frostuniffi "github.com/zecdev/frost-uniffi-sdk/frost_go_ffi"
)

type buildTaggedUniFFINativeFROSTBridge struct{}

func registerBuildTaggedNativeFROSTSigningEngine() error {
	engine, err := newUniFFINativeFROSTSigningEngine(
		&buildTaggedUniFFINativeFROSTBridge{},
	)
	if err != nil {
		return err
	}

	return RegisterNativeFROSTSigningEngine(engine)
}

func recoverUniFFIPanic(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("uniffi panic: [%v]", r)
	}
}

func (btnufb *buildTaggedUniFFINativeFROSTBridge) GenerateNoncesAndCommitments(
	keyPackageIdentifier string,
	keyPackageData []byte,
) (
	noncesData []byte,
	commitmentIdentifier string,
	commitmentData []byte,
	err error,
) {
	defer recoverUniFFIPanic(&err)

	firstRoundCommitment, err := frostuniffi.GenerateNoncesAndCommitments(
		frostuniffi.FrostKeyPackage{
			Identifier: frostuniffi.ParticipantIdentifier{
				Data: keyPackageIdentifier,
			},
			Data: append([]byte{}, keyPackageData...),
		},
	)
	if err != nil {
		return nil, "", nil, fmt.Errorf(
			"cannot generate nonces and commitments: [%w]",
			err,
		)
	}

	return append([]byte{}, firstRoundCommitment.Nonces.Data...),
		firstRoundCommitment.Commitments.Identifier.Data,
		append([]byte{}, firstRoundCommitment.Commitments.Data...),
		nil
}

func (btnufb *buildTaggedUniFFINativeFROSTBridge) NewSigningPackage(
	message []byte,
	commitments []uniFFINativeFROSTCommitment,
) (signingPackageData []byte, err error) {
	defer recoverUniFFIPanic(&err)

	uniffiCommitments := make(
		[]frostuniffi.FrostSigningCommitments,
		0,
		len(commitments),
	)

	for _, commitment := range commitments {
		uniffiCommitments = append(
			uniffiCommitments,
			frostuniffi.FrostSigningCommitments{
				Identifier: frostuniffi.ParticipantIdentifier{
					Data: commitment.Identifier,
				},
				Data: append([]byte{}, commitment.Data...),
			},
		)
	}

	signingPackage, err := frostuniffi.NewSigningPackage(
		frostuniffi.Message{
			Data: append([]byte{}, message...),
		},
		uniffiCommitments,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot build signing package: [%w]", err)
	}

	return append([]byte{}, signingPackage.Data...), nil
}

func (btnufb *buildTaggedUniFFINativeFROSTBridge) Sign(
	signingPackageData []byte,
	noncesData []byte,
	keyPackageIdentifier string,
	keyPackageData []byte,
) (signatureShareIdentifier string, signatureShareData []byte, err error) {
	defer recoverUniFFIPanic(&err)

	signatureShare, err := frostuniffi.Sign(
		frostuniffi.FrostSigningPackage{
			Data: append([]byte{}, signingPackageData...),
		},
		frostuniffi.FrostSigningNonces{
			Data: append([]byte{}, noncesData...),
		},
		frostuniffi.FrostKeyPackage{
			Identifier: frostuniffi.ParticipantIdentifier{
				Data: keyPackageIdentifier,
			},
			Data: append([]byte{}, keyPackageData...),
		},
	)
	if err != nil {
		return "", nil, fmt.Errorf("cannot produce signature share: [%w]", err)
	}

	return signatureShare.Identifier.Data, append([]byte{}, signatureShare.Data...), nil
}

func (btnufb *buildTaggedUniFFINativeFROSTBridge) Aggregate(
	signingPackageData []byte,
	signatureShares []uniFFINativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) (signature []byte, err error) {
	defer recoverUniFFIPanic(&err)

	uniffiSignatureShares := make(
		[]frostuniffi.FrostSignatureShare,
		0,
		len(signatureShares),
	)
	for _, signatureShare := range signatureShares {
		uniffiSignatureShares = append(
			uniffiSignatureShares,
			frostuniffi.FrostSignatureShare{
				Identifier: frostuniffi.ParticipantIdentifier{
					Data: signatureShare.Identifier,
				},
				Data: append([]byte{}, signatureShare.Data...),
			},
		)
	}

	uniffiVerifyingShares := make(
		map[frostuniffi.ParticipantIdentifier]string,
		len(publicKeyPackage.VerifyingShares),
	)
	for identifier, verifyingShare := range publicKeyPackage.VerifyingShares {
		uniffiVerifyingShares[frostuniffi.ParticipantIdentifier{
			Data: identifier,
		}] = verifyingShare
	}

	resultSignature, err := frostuniffi.Aggregate(
		frostuniffi.FrostSigningPackage{
			Data: append([]byte{}, signingPackageData...),
		},
		uniffiSignatureShares,
		frostuniffi.FrostPublicKeyPackage{
			VerifyingShares: uniffiVerifyingShares,
			VerifyingKey:    publicKeyPackage.VerifyingKey,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot aggregate signature shares: [%w]", err)
	}

	return append([]byte{}, resultSignature.Data...), nil
}
