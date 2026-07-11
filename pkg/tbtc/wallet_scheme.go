package tbtc

import "fmt"

// normalizeWalletScheme preserves compatibility with chain adapters created
// before WalletClosedEvent carried a scheme. Their zero value represents the
// legacy ECDSA registry.
func normalizeWalletScheme(walletScheme WalletScheme) WalletScheme {
	if walletScheme == WalletSchemeUnknown {
		return WalletSchemeECDSA
	}

	return walletScheme
}

// walletSchemeAndRegistryID resolves the wallet scheme and the identifier used
// by that scheme's wallet registry. During the migration, Bridge data carries
// both a canonical WalletID and the legacy ECDSA registry ID. FROST wallets do
// not have an ECDSA registry entry, so their EcdsaWalletID is zero.
func walletSchemeAndRegistryID(
	walletData *WalletChainData,
) (WalletScheme, [32]byte, error) {
	if walletData == nil {
		return WalletSchemeUnknown, [32]byte{}, fmt.Errorf("wallet data is nil")
	}

	if walletData.EcdsaWalletID != ([32]byte{}) {
		return WalletSchemeECDSA, walletData.EcdsaWalletID, nil
	}

	if walletData.WalletID != ([32]byte{}) {
		return WalletSchemeFROST, walletData.WalletID, nil
	}

	return WalletSchemeUnknown, [32]byte{}, fmt.Errorf(
		"wallet has neither an ECDSA nor a FROST registry ID",
	)
}
