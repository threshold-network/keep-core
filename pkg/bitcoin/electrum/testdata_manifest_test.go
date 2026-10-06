package electrum

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	testData "github.com/keep-network/keep-core/internal/testdata/bitcoin"
)

// TestRequiredVectorsPresent guards the fixtures declared required by
// internal/testdata/bitcoin/manifest.go against accidental deletion.
//
// It deliberately lives here rather than beside the manifest: the Go tool
// excludes any directory named "testdata" from package patterns, so a test
// inside internal/testdata/bitcoin is never matched by `go test ./...` and
// would only run when named explicitly. Hosting it in the package that
// consumes the fixtures puts it in the ordinary suite, so deleting a vector
// the electrum integration suite depends on fails the default build rather
// than silently turning those integration tests into skips.
func TestRequiredVectorsPresent(t *testing.T) {
	for _, network := range testData.RequiredBlocks {
		data, ok := testData.Blocks[network]
		if !ok {
			t.Errorf(
				"Blocks is missing a required entry for network [%s]; add it, "+
					"or remove [%s] from RequiredBlocks in manifest.go if the "+
					"electrum integration suite no longer needs it",
				network, network,
			)
			continue
		}
		if data.BlockHeight == 0 {
			t.Errorf("Blocks[%s].BlockHeight is zero-valued", network)
		}
		if data.BlockHeader == nil {
			t.Errorf("Blocks[%s].BlockHeader is nil", network)
		}
		if data.CoinbaseTxHash == (bitcoin.Hash{}) {
			t.Errorf("Blocks[%s].CoinbaseTxHash is zero-valued", network)
		}
	}

	for _, network := range testData.RequiredTransactions {
		transactions, ok := testData.Transactions[network]
		if !ok || len(transactions) == 0 {
			t.Errorf(
				"Transactions is missing a required non-empty entry for "+
					"network [%s]; add it, or remove [%s] from "+
					"RequiredTransactions in manifest.go if the electrum "+
					"integration suite no longer needs it",
				network, network,
			)
			continue
		}
		for name, tx := range transactions {
			if tx.TxHash == (bitcoin.Hash{}) {
				t.Errorf("Transactions[%s][%s].TxHash is zero-valued", network, name)
			}
			if tx.BlockHeight == 0 {
				t.Errorf(
					"Transactions[%s][%s].BlockHeight is zero-valued",
					network,
					name,
				)
			}
		}
	}

	for _, network := range testData.RequiredTxMerkleProofs {
		data, ok := testData.TxMerkleProofs[network]
		if !ok {
			t.Errorf(
				"TxMerkleProofs is missing a required entry for network [%s]; "+
					"add it, or remove [%s] from RequiredTxMerkleProofs in "+
					"manifest.go if the electrum integration suite no longer "+
					"needs it",
				network, network,
			)
			continue
		}
		if data.TxHash == (bitcoin.Hash{}) {
			t.Errorf("TxMerkleProofs[%s].TxHash is zero-valued", network)
		}
		if data.BlockHeight == 0 {
			t.Errorf("TxMerkleProofs[%s].BlockHeight is zero-valued", network)
		}
		if data.MerkleProof == nil {
			t.Errorf("TxMerkleProofs[%s].MerkleProof is nil", network)
		} else if len(data.MerkleProof.MerkleNodes) == 0 {
			t.Errorf(
				"TxMerkleProofs[%s].MerkleProof.MerkleNodes is empty",
				network,
			)
		}
	}

	for _, network := range testData.RequiredTransactionsForPublicKeyHash {
		data, ok := testData.TransactionsForPublicKeyHash[network]
		if !ok {
			t.Errorf(
				"TransactionsForPublicKeyHash is missing a required entry for "+
					"network [%s]; add it, or remove [%s] from "+
					"RequiredTransactionsForPublicKeyHash in manifest.go if the "+
					"electrum integration suite no longer needs it",
				network, network,
			)
			continue
		}
		if len(data.PublicKeyHash) == 0 {
			t.Errorf(
				"TransactionsForPublicKeyHash[%s].PublicKeyHash is empty",
				network,
			)
		}
		if len(data.Transactions) == 0 {
			t.Errorf(
				"TransactionsForPublicKeyHash[%s].Transactions is empty",
				network,
			)
		}
		if len(data.Utxos) == 0 {
			t.Errorf(
				"TransactionsForPublicKeyHash[%s].Utxos is empty",
				network,
			)
		}
	}
}
