package tbtc

import (
	"bytes"
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// WalletOutputScript constructs the Bitcoin locking script for the canonical
// wallet identity. Legacy ECDSA wallets keep using P2WPKH outputs. Native FROST
// wallets use their canonical wallet ID as the P2TR output key.
func WalletOutputScript(
	walletPublicKeyHash [20]byte,
	walletID [32]byte,
) (bitcoin.Script, error) {
	if walletID == [32]byte{} {
		walletID = DeriveLegacyWalletID(walletPublicKeyHash)
	}

	if legacyWalletPublicKeyHash, ok := WalletPublicKeyHashFromLegacyWalletID(
		walletID,
	); ok {
		if !bytes.Equal(
			legacyWalletPublicKeyHash[:],
			walletPublicKeyHash[:],
		) {
			return nil, fmt.Errorf(
				"legacy wallet ID does not match wallet public key hash",
			)
		}

		return bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	}

	return bitcoin.PayToTaproot(walletID)
}

type walletDataResolver interface {
	GetWallet(walletPublicKeyHash [20]byte) (*WalletChainData, error)
}

func walletOutputScriptForPublicKeyHash(
	chain walletDataResolver,
	walletPublicKeyHash [20]byte,
) (bitcoin.Script, error) {
	walletChainData, err := chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot get wallet's chain data: [%w]", err)
	}

	return WalletOutputScript(walletPublicKeyHash, walletChainData.WalletID)
}

func walletOutputScriptsForPublicKeyHashes(
	chain walletDataResolver,
	walletPublicKeyHashes [][20]byte,
) ([]bitcoin.Script, error) {
	outputScripts := make([]bitcoin.Script, len(walletPublicKeyHashes))

	for i, walletPublicKeyHash := range walletPublicKeyHashes {
		outputScript, err := walletOutputScriptForPublicKeyHash(
			chain,
			walletPublicKeyHash,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot compute output script for wallet [%d]: [%w]",
				i,
				err,
			)
		}

		outputScripts[i] = outputScript
	}

	return outputScripts, nil
}
