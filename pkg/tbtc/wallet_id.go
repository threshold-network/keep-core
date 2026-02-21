package tbtc

// DeriveLegacyWalletID derives the canonical bridge wallet ID for legacy
// ECDSA wallets from their 20-byte wallet public key hash.
//
// Legacy wallet ID format is a left-padded bytes20 hash:
// bytes32(uint256(uint160(walletPubKeyHash))).
func DeriveLegacyWalletID(walletPublicKeyHash [20]byte) [32]byte {
	var walletID [32]byte
	copy(walletID[12:], walletPublicKeyHash[:])
	return walletID
}
