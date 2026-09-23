package bitcoin

import "github.com/keep-network/keep-core/pkg/bitcoin"

// RequiredBlocks, RequiredTransactions, RequiredTxMerkleProofs, and
// RequiredTransactionsForPublicKeyHash each declare the networks the
// correspondingly-named fixture map in this package MUST carry a non-empty
// entry for.
//
// The set is derived from pkg/bitcoin/electrum/electrum_integration_test.go:
// every testConfig in that file's testConfigs map runs against
// bitcoin.Testnet only (see runParallel), so Testnet is the only network the
// integration suite actually looks up in these fixtures today.
//
// The assertions live in pkg/bitcoin/electrum/testdata_manifest_test.go, not
// beside this file. The Go tool excludes any directory named "testdata" from
// package patterns, so a test here would never be matched by `go test ./...`
// and would only run when named explicitly. Hosting it in the consuming
// package means deleting a fixture entry breaks the default build immediately,
// instead of merely causing the `-tags=integration` electrum suite - which most
// contributors never run locally - to fail its own guard.
//
// If a new network is added to testConfigs, first add a fixture entry for it to
// Blocks/Transactions/TxMerkleProofs/TransactionsForPublicKeyHash, then add it
// here. Conversely, if the integration suite drops a network, remove it here
// before removing its fixture entry.
var (
	// RequiredBlocks lists the networks Blocks must have an entry for.
	RequiredBlocks = []bitcoin.Network{bitcoin.Testnet}

	// RequiredTransactions lists the networks Transactions must have a
	// non-empty entry for. TestGetTransaction_Integration and
	// TestGetTransactionConfirmations_Integration range over this map without
	// a presence check, so a missing entry makes both run zero subtests and
	// pass vacuously rather than fail.
	RequiredTransactions = []bitcoin.Network{bitcoin.Testnet}

	// RequiredTxMerkleProofs lists the networks TxMerkleProofs must have an
	// entry for.
	RequiredTxMerkleProofs = []bitcoin.Network{bitcoin.Testnet}

	// RequiredTransactionsForPublicKeyHash lists the networks
	// TransactionsForPublicKeyHash must have an entry for.
	RequiredTransactionsForPublicKeyHash = []bitcoin.Network{bitcoin.Testnet}
)
