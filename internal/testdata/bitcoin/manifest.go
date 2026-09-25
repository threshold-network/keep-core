package bitcoin

import "github.com/keep-network/keep-core/pkg/bitcoin"

// RequiredBlocks, RequiredTransactions, RequiredTxMerkleProofs, and
// RequiredTransactionsForPublicKeyHash each declare the networks the
// correspondingly-named fixture map in this package MUST carry a non-empty
// entry for.
//
// The Electrum integration suite connects to Testnet, Mainnet, and Testnet4,
// but data-dependent cases skip networks for which no fixture exists. These
// slices declare the fixture coverage that must not regress: Testnet vectors
// are required even though other configured networks are allowed to be
// unsupported.
//
// The assertions live in pkg/bitcoin/electrum/testdata_manifest_test.go, not
// beside this file. The Go tool excludes any directory named "testdata" from
// package patterns, so a test here would never be matched by `go test ./...`
// and would only run when named explicitly. Hosting it in the consuming
// package means deleting a required fixture entry breaks the default build
// immediately instead of silently reducing integration coverage.
//
// Add a network only after adding fixtures for it. Remove a network before
// deliberately removing its fixture entry.
var (
	// RequiredBlocks lists the networks Blocks must have an entry for.
	RequiredBlocks = []bitcoin.Network{bitcoin.Testnet}

	// RequiredTransactions lists the networks Transactions must have a
	// non-empty entry for. TestGetTransaction_Integration and
	// TestGetTransactionConfirmations_Integration skip when a network's entry
	// is missing, so a missing entry makes both pass vacuously as skips
	// rather than fail.
	RequiredTransactions = []bitcoin.Network{bitcoin.Testnet}

	// RequiredTxMerkleProofs lists the networks TxMerkleProofs must have an
	// entry for.
	RequiredTxMerkleProofs = []bitcoin.Network{bitcoin.Testnet}

	// RequiredTransactionsForPublicKeyHash lists the networks
	// TransactionsForPublicKeyHash must have an entry for.
	RequiredTransactionsForPublicKeyHash = []bitcoin.Network{bitcoin.Testnet}
)
