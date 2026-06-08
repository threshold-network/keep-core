package spv

import (
	"bytes"
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func isWalletOutputScript(
	walletPublicKeyHash [20]byte,
	outputScript bitcoin.Script,
	spvChain Chain,
) (bool, error) {
	wallet, err := spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("cannot get wallet: [%v]", err)
	}

	walletID := wallet.WalletID
	if walletID == [32]byte{} {
		walletID = tbtc.DeriveLegacyWalletID(walletPublicKeyHash)
	}

	walletScript, err := tbtc.WalletOutputScript(
		walletPublicKeyHash,
		walletID,
	)
	if err != nil {
		return false, fmt.Errorf("cannot construct wallet output script: [%v]", err)
	}

	if bytes.Equal(outputScript, walletScript) {
		return true, nil
	}

	if bitcoin.GetScriptType(outputScript) != bitcoin.P2PKHScript {
		return false, nil
	}

	legacyWalletPublicKeyHash, ok :=
		tbtc.WalletPublicKeyHashFromLegacyWalletID(walletID)
	if !ok || !bytes.Equal(legacyWalletPublicKeyHash[:], walletPublicKeyHash[:]) {
		return false, nil
	}

	walletP2PKH, err := bitcoin.PayToPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("cannot construct P2PKH for wallet: [%v]", err)
	}

	return bytes.Equal(outputScript, walletP2PKH), nil
}
