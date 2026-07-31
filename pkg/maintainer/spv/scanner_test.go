package spv

import (
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// buildScannerTestWalletTx builds a distinct confirmed transaction paying to
// the given wallet address. The input outpoint index keeps each transaction's
// hash unique.
func buildScannerTestWalletTx(
	walletScript bitcoin.Script,
	index uint32,
) *bitcoin.Transaction {
	return &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{},
					OutputIndex:     index,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           546,
				PublicKeyScript: walletScript,
			},
		},
		Locktime: 0,
	}
}

// TestWalletTransactionScanner_ReclassifiesOnceMatching verifies that a
// transaction whose predicate result is initially false is still found once
// the predicate later starts returning true for it - matching the real
// isUnproven* predicates, whose answer can depend on mutable wallet state
// (e.g. the current main UTXO) rather than being a permanent property of the
// transaction. The scanner must not cache the earlier negative result.
func TestWalletTransactionScanner_ReclassifiesOnceMatching(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{1, 2, 3}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := buildScannerTestWalletTx(walletScript, 0)
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	transactionHash := transaction.Hash()

	matches := false
	predicate := func(candidate *bitcoin.Transaction) (bool, error) {
		return candidate.Hash() == transactionHash && matches, nil
	}

	scanner := newWalletTransactionScanner()
	key := walletTransactionScanKey{lookupWalletPKH: walletPublicKeyHash}

	scanner.beginRound()
	found, err := scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("expected no matches before the predicate flips, got %d", len(found))
	}

	// Simulate the wallet's main UTXO advancing so the transaction now
	// matches.
	matches = true

	scanner.beginRound()
	found, err = scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Hash() != transactionHash {
		t.Fatal("expected the transaction to be found once it starts matching")
	}
}

// TestWalletTransactionScanner_DropsCandidateOnceProven verifies that a
// transaction previously reported as unproven stops being reported, without
// needing rediscovery, once its predicate starts returning false (i.e. once
// its SPV proof has been submitted).
func TestWalletTransactionScanner_DropsCandidateOnceProven(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{4, 5, 6}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := buildScannerTestWalletTx(walletScript, 0)
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	transactionHash := transaction.Hash()

	unproven := true
	predicate := func(candidate *bitcoin.Transaction) (bool, error) {
		return candidate.Hash() == transactionHash && unproven, nil
	}

	scanner := newWalletTransactionScanner()
	key := walletTransactionScanKey{lookupWalletPKH: walletPublicKeyHash}

	scanner.beginRound()
	found, err := scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("expected the transaction to be found as a candidate, got %d", len(found))
	}

	// Simulate the SPV proof having been submitted for this transaction.
	unproven = false

	scanner.beginRound()
	found, err = scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("expected the proven transaction to be dropped, got %d", len(found))
	}
}

// TestWalletTransactionScanner_DropsCandidateOnReorg verifies that a
// previously-found candidate is dropped, without error, if a later call no
// longer sees its hash in the confirmed transaction history (e.g. after a
// Bitcoin reorganization).
func TestWalletTransactionScanner_DropsCandidateOnReorg(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{7, 8, 9}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := buildScannerTestWalletTx(walletScript, 0)
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	transactionHash := transaction.Hash()

	predicate := func(candidate *bitcoin.Transaction) (bool, error) {
		return candidate.Hash() == transactionHash, nil
	}

	scanner := newWalletTransactionScanner()
	key := walletTransactionScanKey{lookupWalletPKH: walletPublicKeyHash}

	scanner.beginRound()
	found, err := scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("expected the transaction to be found as a candidate, got %d", len(found))
	}

	// Simulate a reorganization dropping the transaction from the confirmed
	// history by replacing the chain's transaction list.
	btcChain.mutex.Lock()
	btcChain.transactions = nil
	btcChain.mutex.Unlock()

	scanner.beginRound()
	found, err = scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("expected the reorged-out candidate to be dropped, got %d", len(found))
	}
}

// TestWalletTransactionScanner_IsolatesDistinctScanKeys verifies that two
// distinct scan keys over the same lookup wallet (e.g. two different action
// types, or - as with moving funds - two different context hashes) do not
// share cached state.
func TestWalletTransactionScanner_IsolatesDistinctScanKeys(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{10, 11, 12}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := buildScannerTestWalletTx(walletScript, 0)
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	transactionHash := transaction.Hash()

	predicate := func(candidate *bitcoin.Transaction) (bool, error) {
		return candidate.Hash() == transactionHash, nil
	}

	scanner := newWalletTransactionScanner()
	keyA := walletTransactionScanKey{
		actionType:      tbtc.ActionDepositSweep,
		lookupWalletPKH: walletPublicKeyHash,
	}
	keyB := walletTransactionScanKey{
		actionType:      tbtc.ActionRedemption,
		lookupWalletPKH: walletPublicKeyHash,
	}

	scanner.beginRound()
	if _, err := scanner.getUnprovenWalletTransactions(
		keyA, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	); err != nil {
		t.Fatal(err)
	}

	if len(scanner.states) != 1 {
		t.Fatalf("expected exactly one scan state after the first key, got %d", len(scanner.states))
	}

	scanner.beginRound()
	if _, err := scanner.getUnprovenWalletTransactions(
		keyB, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	); err != nil {
		t.Fatal(err)
	}

	if len(scanner.states) != 2 {
		t.Fatalf("expected a second, independent scan state for the second key, got %d", len(scanner.states))
	}
}

// TestWalletTransactionScanner_EvictsUnusedState verifies that a scan key's
// state is evicted after it goes unused for more than
// scanStateEvictAfterRounds rounds, bounding scanner memory to recently
// relevant wallets.
func TestWalletTransactionScanner_EvictsUnusedState(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{13, 14, 15}

	predicate := func(*bitcoin.Transaction) (bool, error) {
		return false, nil
	}

	scanner := newWalletTransactionScanner()
	key := walletTransactionScanKey{lookupWalletPKH: walletPublicKeyHash}

	scanner.beginRound()
	if _, err := scanner.getUnprovenWalletTransactions(
		key, walletPublicKeyHash, 20, 20, "test", btcChain, predicate,
	); err != nil {
		t.Fatal(err)
	}

	if len(scanner.states) != 1 {
		t.Fatalf("expected the scan state to exist, got %d entries", len(scanner.states))
	}

	for i := 0; i < scanStateEvictAfterRounds+1; i++ {
		scanner.beginRound()
	}

	testutils.AssertIntsEqual(
		t,
		"scanner states after eviction window elapses",
		0,
		len(scanner.states),
	)
}

// TestWalletTransactionScanner_NonPositiveScanLimitExaminesNothing verifies
// that a zero or negative transactionScanLimit examines zero new hashes
// (rather than one, which a post-work budget check would otherwise allow),
// so a misconfigured limit cannot cause even a single unbounded-cost
// GetTransaction call.
func TestWalletTransactionScanner_NonPositiveScanLimitExaminesNothing(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{16, 17, 18}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := buildScannerTestWalletTx(walletScript, 0)
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	transactionHash := transaction.Hash()

	examined := 0
	predicate := func(candidate *bitcoin.Transaction) (bool, error) {
		examined++
		return candidate.Hash() == transactionHash, nil
	}

	scanner := newWalletTransactionScanner()
	key := walletTransactionScanKey{lookupWalletPKH: walletPublicKeyHash}

	for _, scanLimit := range []int{0, -1} {
		scanner.beginRound()
		found, err := scanner.getUnprovenWalletTransactions(
			key, walletPublicKeyHash, 20, scanLimit, "test", btcChain, predicate,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 0 {
			t.Fatalf("scanLimit=%d: expected no matches, got %d", scanLimit, len(found))
		}
	}

	testutils.AssertIntsEqual(t, "predicate calls with a non-positive scan limit", 0, examined)
}
