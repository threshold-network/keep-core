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

// WalletPublicKeyHashFromLegacyWalletID extracts the compatibility wallet
// public key hash from a canonical legacy wallet ID.
//
// Legacy wallet ID format is a left-padded bytes20 hash:
// bytes32(uint256(uint160(walletPubKeyHash))).
func WalletPublicKeyHashFromLegacyWalletID(walletID [32]byte) ([20]byte, bool) {
	for i := 0; i < 12; i++ {
		if walletID[i] != 0 {
			return [20]byte{}, false
		}
	}

	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], walletID[12:])

	return walletPublicKeyHash, true
}
