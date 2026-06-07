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
	walletP2PKH, err := bitcoin.PayToPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("cannot construct P2PKH for wallet: [%v]", err)
	}
	walletP2WPKH, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("cannot construct P2WPKH for wallet: [%v]", err)
	}

	if bytes.Equal(outputScript, walletP2PKH) ||
		bytes.Equal(outputScript, walletP2WPKH) {
		return true, nil
	}

	if bitcoin.GetScriptType(outputScript) != bitcoin.P2TRScript {
		return false, nil
	}

	wallet, err := spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("cannot get wallet: [%v]", err)
	}

	walletScript, err := tbtc.WalletOutputScript(
		walletPublicKeyHash,
		wallet.WalletID,
	)
	if err != nil {
		return false, fmt.Errorf("cannot construct wallet output script: [%v]", err)
	}

	return bytes.Equal(outputScript, walletScript), nil
}
