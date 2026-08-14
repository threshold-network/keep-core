package bitcoin

import (
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/wire"
)

// IsDustOutput reports whether the output is non-standard under Bitcoin
// Core's default minimum relay fee. Nil outputs are treated as invalid dust.
func IsDustOutput(output *TransactionOutput) bool {
	if output == nil {
		return true
	}

	return mempool.IsDust(
		wire.NewTxOut(output.Value, output.PublicKeyScript),
		mempool.DefaultMinRelayTxFee,
	)
}
