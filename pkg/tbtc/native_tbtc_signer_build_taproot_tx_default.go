//go:build !(frost_native && frost_tbtc_signer && cgo)

package tbtc

import "github.com/keep-network/keep-core/pkg/bitcoin"

// buildTaprootTxViaNativeSigner is a no-op on builds that do not link the
// native tbtc-signer bridge.
func buildTaprootTxViaNativeSigner(
	unsignedTx *bitcoin.TransactionBuilder,
) (string, error) {
	return "", nil
}
