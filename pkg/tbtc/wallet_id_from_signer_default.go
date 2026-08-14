//go:build !frost_native

package tbtc

import "crypto/ecdsa"

func calculateWalletIDForSigner(
	signer *signer,
	calculateLegacyWalletID func(*ecdsa.PublicKey) ([32]byte, error),
) ([32]byte, error) {
	return calculateLegacyWalletID(signer.wallet.publicKey)
}
