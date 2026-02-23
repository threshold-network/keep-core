//go:build frost_native && frost_tbtc_signer && cgo && !frost_uniffi_sdk

package signing

import "fmt"

type buildTaggedTBTCSignerNativeFROSTBridge struct{}

func registerBuildTaggedNativeFROSTSigningEngine() error {
	engine, err := newUniFFINativeFROSTSigningEngine(
		&buildTaggedTBTCSignerNativeFROSTBridge{},
	)
	if err != nil {
		return err
	}

	return RegisterNativeFROSTSigningEngine(engine)
}

func (bttsnfb *buildTaggedTBTCSignerNativeFROSTBridge) GenerateNoncesAndCommitments(
	keyPackageIdentifier string,
	keyPackageData []byte,
) (
	noncesData []byte,
	commitmentIdentifier string,
	commitmentData []byte,
	err error,
) {
	return nil, "", nil, buildTaggedTBTCSignerBridgeNotImplementedError(
		"GenerateNoncesAndCommitments",
	)
}

func (bttsnfb *buildTaggedTBTCSignerNativeFROSTBridge) NewSigningPackage(
	message []byte,
	commitments []uniFFINativeFROSTCommitment,
) (signingPackageData []byte, err error) {
	return nil, buildTaggedTBTCSignerBridgeNotImplementedError("NewSigningPackage")
}

func (bttsnfb *buildTaggedTBTCSignerNativeFROSTBridge) Sign(
	signingPackageData []byte,
	noncesData []byte,
	keyPackageIdentifier string,
	keyPackageData []byte,
) (signatureShareIdentifier string, signatureShareData []byte, err error) {
	return "", nil, buildTaggedTBTCSignerBridgeNotImplementedError("Sign")
}

func (bttsnfb *buildTaggedTBTCSignerNativeFROSTBridge) Aggregate(
	signingPackageData []byte,
	signatureShares []uniFFINativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) (signature []byte, err error) {
	return nil, buildTaggedTBTCSignerBridgeNotImplementedError("Aggregate")
}

func buildTaggedTBTCSignerBridgeNotImplementedError(operation string) error {
	return fmt.Errorf(
		"tbtc-signer bridge operation [%v] is not implemented",
		operation,
	)
}
