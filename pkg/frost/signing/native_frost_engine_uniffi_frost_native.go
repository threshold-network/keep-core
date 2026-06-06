//go:build frost_native

package signing

import "fmt"

type uniFFINativeFROSTCommitment struct {
	Identifier string
	Data       []byte
}

type uniFFINativeFROSTSignatureShare struct {
	Identifier string
	Data       []byte
}

type uniFFINativeFROSTBridge interface {
	GenerateNoncesAndCommitments(
		keyPackageIdentifier string,
		keyPackageData []byte,
	) (noncesData []byte, commitmentIdentifier string, commitmentData []byte, err error)
	NewSigningPackage(
		message []byte,
		commitments []uniFFINativeFROSTCommitment,
	) (signingPackageData []byte, err error)
	Sign(
		signingPackageData []byte,
		noncesData []byte,
		keyPackageIdentifier string,
		keyPackageData []byte,
	) (signatureShareIdentifier string, signatureShareData []byte, err error)
	Aggregate(
		signingPackageData []byte,
		signatureShares []uniFFINativeFROSTSignatureShare,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
	) (signature []byte, err error)
}

type uniFFINativeFROSTSigningEngine struct {
	bridge uniFFINativeFROSTBridge
}

func newUniFFINativeFROSTSigningEngine(
	bridge uniFFINativeFROSTBridge,
) (NativeFROSTSigningEngine, error) {
	if bridge == nil {
		return nil, fmt.Errorf("uniffi native FROST bridge is nil")
	}

	return &uniFFINativeFROSTSigningEngine{
		bridge: bridge,
	}, nil
}

func (unfse *uniFFINativeFROSTSigningEngine) GenerateNoncesAndCommitments(
	keyPackage *NativeFROSTKeyPackage,
) (*NativeFROSTNonces, *NativeFROSTCommitment, error) {
	if keyPackage == nil {
		return nil, nil, fmt.Errorf("key package is nil")
	}

	if keyPackage.Identifier == "" {
		return nil, nil, fmt.Errorf("key package identifier is empty")
	}

	if len(keyPackage.Data) == 0 {
		return nil, nil, fmt.Errorf("key package data is empty")
	}

	noncesData, commitmentIdentifier, commitmentData, err := unfse.bridge.GenerateNoncesAndCommitments(
		keyPackage.Identifier,
		append([]byte{}, keyPackage.Data...),
	)
	if err != nil {
		return nil, nil, err
	}

	return &NativeFROSTNonces{
			Data: append([]byte{}, noncesData...),
		}, &NativeFROSTCommitment{
			Identifier: commitmentIdentifier,
			Data:       append([]byte{}, commitmentData...),
		}, nil
}

func (unfse *uniFFINativeFROSTSigningEngine) NewSigningPackage(
	message []byte,
	commitments []*NativeFROSTCommitment,
) (*NativeFROSTSigningPackage, error) {
	if len(commitments) == 0 {
		return nil, fmt.Errorf("commitments are empty")
	}

	bridgeCommitments := make([]uniFFINativeFROSTCommitment, 0, len(commitments))
	for i, commitment := range commitments {
		if commitment == nil {
			return nil, fmt.Errorf("commitment [%d] is nil", i)
		}

		if commitment.Identifier == "" {
			return nil, fmt.Errorf("commitment [%d] identifier is empty", i)
		}

		if len(commitment.Data) == 0 {
			return nil, fmt.Errorf("commitment [%d] data is empty", i)
		}

		bridgeCommitments = append(bridgeCommitments, uniFFINativeFROSTCommitment{
			Identifier: commitment.Identifier,
			Data:       append([]byte{}, commitment.Data...),
		})
	}

	signingPackageData, err := unfse.bridge.NewSigningPackage(
		append([]byte{}, message...),
		bridgeCommitments,
	)
	if err != nil {
		return nil, err
	}

	return &NativeFROSTSigningPackage{
		Data: append([]byte{}, signingPackageData...),
	}, nil
}

func (unfse *uniFFINativeFROSTSigningEngine) Sign(
	signingPackage *NativeFROSTSigningPackage,
	nonces *NativeFROSTNonces,
	keyPackage *NativeFROSTKeyPackage,
) (*NativeFROSTSignatureShare, error) {
	if signingPackage == nil {
		return nil, fmt.Errorf("signing package is nil")
	}

	if len(signingPackage.Data) == 0 {
		return nil, fmt.Errorf("signing package data is empty")
	}

	if keyPackage == nil {
		return nil, fmt.Errorf("key package is nil")
	}

	if keyPackage.Identifier == "" {
		return nil, fmt.Errorf("key package identifier is empty")
	}

	if len(keyPackage.Data) == 0 {
		return nil, fmt.Errorf("key package data is empty")
	}

	noncesData, err := nonces.consumeData()
	if err != nil {
		return nil, err
	}
	defer zeroBytes(noncesData)

	identifier, signatureShareData, err := unfse.bridge.Sign(
		append([]byte{}, signingPackage.Data...),
		noncesData,
		keyPackage.Identifier,
		append([]byte{}, keyPackage.Data...),
	)
	if err != nil {
		return nil, err
	}

	return &NativeFROSTSignatureShare{
		Identifier: identifier,
		Data:       append([]byte{}, signatureShareData...),
	}, nil
}

func (unfse *uniFFINativeFROSTSigningEngine) Aggregate(
	signingPackage *NativeFROSTSigningPackage,
	signatureShares []*NativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) ([]byte, error) {
	if signingPackage == nil {
		return nil, fmt.Errorf("signing package is nil")
	}

	if len(signingPackage.Data) == 0 {
		return nil, fmt.Errorf("signing package data is empty")
	}

	if len(signatureShares) == 0 {
		return nil, fmt.Errorf("signature shares are empty")
	}

	if publicKeyPackage == nil {
		return nil, fmt.Errorf("public key package is nil")
	}

	bridgeSignatureShares := make([]uniFFINativeFROSTSignatureShare, 0, len(signatureShares))
	for i, signatureShare := range signatureShares {
		if signatureShare == nil {
			return nil, fmt.Errorf("signature share [%d] is nil", i)
		}

		if signatureShare.Identifier == "" {
			return nil, fmt.Errorf("signature share [%d] identifier is empty", i)
		}

		if len(signatureShare.Data) == 0 {
			return nil, fmt.Errorf("signature share [%d] data is empty", i)
		}

		bridgeSignatureShares = append(
			bridgeSignatureShares,
			uniFFINativeFROSTSignatureShare{
				Identifier: signatureShare.Identifier,
				Data:       append([]byte{}, signatureShare.Data...),
			},
		)
	}

	signature, err := unfse.bridge.Aggregate(
		append([]byte{}, signingPackage.Data...),
		bridgeSignatureShares,
		publicKeyPackage,
	)
	if err != nil {
		return nil, err
	}

	return append([]byte{}, signature...), nil
}
