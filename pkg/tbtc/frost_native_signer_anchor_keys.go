package tbtc

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func loadFrostNativeSignerAnchorClientPrivateKey(
	path string,
) (ed25519.PrivateKey, error) {
	data, err := readSecureFrostActivationFile(path, 16*1024)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read FROST native signer anchor client key: %w",
			err,
		)
	}
	defer zeroFrostNativeSignerKeyBytes(data)
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" ||
		len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf(
			"FROST native signer anchor client key must be one PKCS#8 PEM block",
		)
	}
	defer zeroFrostNativeSignerKeyBytes(block.Bytes)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse FROST native signer anchor client key: %w",
			err,
		)
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf(
			"FROST native signer anchor client key is not Ed25519",
		)
	}
	result := append(ed25519.PrivateKey{}, privateKey...)
	zeroFrostNativeSignerKeyBytes(privateKey)
	return result, nil
}

func loadFrostNativeSignerAnchorOnlinePublicKeySPKI(
	path string,
) ([]byte, ed25519.PublicKey, error) {
	der, err := readSecureFrostActivationFile(path, 16*1024)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot read FROST native signer anchor online key: %w",
			err,
		)
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot parse FROST native signer anchor online SPKI: %w",
			err,
		)
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return nil, nil, fmt.Errorf(
			"FROST native signer anchor online key is not Ed25519",
		)
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		publicKey,
	); err != nil {
		return nil, nil, fmt.Errorf(
			"FROST native signer anchor online key point is invalid: %w",
			err,
		)
	}
	canonical, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil || !bytes.Equal(canonical, der) {
		return nil, nil, fmt.Errorf(
			"FROST native signer anchor online key is not canonical DER SPKI",
		)
	}
	return append([]byte{}, der...), append(ed25519.PublicKey{}, publicKey...), nil
}

func loadFrostNativeSignerAnchorTrustCertificateChain(
	path string,
) ([]byte, error) {
	data, err := readSecureFrostActivationFile(
		path,
		frostNativeSignerAnchorTrustMaximumTransitionRequestBytes,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read FROST native signer anchor trust-certificate chain: %w",
			err,
		)
	}
	return data, nil
}

func zeroFrostNativeSignerKeyBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
