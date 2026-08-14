//go:build !(frost_native && frost_tbtc_signer && cgo)

package tbtc

import (
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// buildTaprootTxViaNativeSigner is a no-op on builds that do not link the
// native tbtc-signer bridge.
func buildTaprootTxViaNativeSigner(
	unsignedTx *bitcoin.TransactionBuilder,
) (string, error) {
	return "", nil
}

// bindTaprootTxViaNativeSigner is a no-op on builds that do not link the
// native tbtc-signer bridge.
func bindTaprootTxViaNativeSigner(
	sessionID string,
	unsignedTx *bitcoin.TransactionBuilder,
	inputIndex int,
	expectedSighash *big.Int,
) error {
	return nil
}
