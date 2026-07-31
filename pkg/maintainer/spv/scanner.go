package spv

import (
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// scanStateEvictAfterRounds is the number of maintainer rounds a scan key's
// state may sit unused before it is evicted. This bounds scanner memory to
// recently-relevant wallets rather than the lifetime total of every wallet
// ever seen.
const scanStateEvictAfterRounds = 100

// walletTransactionScanKey identifies one persistent wallet-transaction scan:
// which proof kind it serves, which wallet the scan is being performed on
// behalf of, which wallet's confirmed transaction history is being examined
// (for moving funds, this differs from the actor wallet - see
// getUnprovenMovingFundsTransactions), and, where the same actor/lookup pair
// could recur under different circumstances, a context hash distinguishing
// them (moving funds' ordered target-wallet commitment).
type walletTransactionScanKey struct {
	actionType      tbtc.WalletActionType
	actorWalletPKH  [20]byte
	lookupWalletPKH [20]byte
	contextHash     [32]byte
}

// walletTransactionScanState is the persisted progress for one scan key: a
// circular cursor into the examined wallet's confirmed transaction history,
// and the set of matching (unproven protocol) transactions found so far.
type walletTransactionScanState struct {
	lastExaminedHash *bitcoin.Hash
	candidates       map[bitcoin.Hash]*bitcoin.Transaction
	lastUsedRound    uint64
}

// walletTransactionScanner bounds the per-tick cost of discovering unproven
// wallet transactions under TOB-TBTCACEXT-22's spam-resistant scan. Instead of
// re-walking a wallet's entire confirmed transaction history on every
// maintainer tick, it remembers a circular cursor per scan key and examines
// at most transactionScanLimit not-yet-classified hashes per call, resuming
// where the previous call on that key left off. The cursor wraps at the
// oldest hash, so given enough ticks every confirmed transaction is
// eventually re-examined - this matters because a transaction's
// unproven-match predicate is not permanently stable (for example, it can
// depend on the wallet's current main UTXO, which advances over time as
// earlier transactions are proven). Nothing is cached as a permanent
// negative; only current matches are cached, and they are re-tested every
// round so a just-proven transaction is dropped promptly.
//
// Not safe for concurrent use. The SPV maintainer drives all proof kinds
// sequentially from a single control-loop goroutine (see
// spvMaintainer.maintainSpv), so no locking is needed here.
type walletTransactionScanner struct {
	states       map[walletTransactionScanKey]*walletTransactionScanState
	currentRound uint64
}

func newWalletTransactionScanner() *walletTransactionScanner {
	return &walletTransactionScanner{
		states: make(map[walletTransactionScanKey]*walletTransactionScanState),
	}
}

// beginRound advances the scanner's round counter and evicts state for scan
// keys unused for scanStateEvictAfterRounds rounds. Call once per maintainSpv
// iteration, before processing any proof kind.
func (s *walletTransactionScanner) beginRound() {
	s.currentRound++

	for key, state := range s.states {
		if s.currentRound-state.lastUsedRound > scanStateEvictAfterRounds {
			delete(s.states, key)
		}
	}
}

// getUnprovenWalletTransactions scans a wallet's confirmed Bitcoin transaction
// history and returns up to transactionLimit transactions that satisfy the
// given predicate, i.e. unproven protocol transactions of the given
// description (used only to phrase wrapped predicate errors).
//
// The full confirmed history is fetched every call (its ordering is this
// function's source of truth for the returned ascending-block-height order),
// but at most transactionScanLimit not-yet-classified hashes are freshly
// examined (one GetTransaction call plus a predicate check each); previously
// found matches are re-tested directly instead of being re-discovered. This
// keeps per-call cost bounded regardless of how much history - real or spam -
// the wallet address has accumulated, while still guaranteeing that a real
// protocol transaction can never be permanently evicted from discovery: the
// persisted cursor (scoped to key) advances across calls and wraps at the
// oldest hash, so it eventually covers the whole history no matter how much
// unrelated history precedes it.
func (s *walletTransactionScanner) getUnprovenWalletTransactions(
	key walletTransactionScanKey,
	lookupPublicKeyHash [20]byte,
	transactionLimit int,
	transactionScanLimit int,
	description string,
	btcChain bitcoin.Chain,
	isUnproven func(*bitcoin.Transaction) (bool, error),
) ([]*bitcoin.Transaction, error) {
	txHashes, err := btcChain.GetTxHashesForPublicKeyHash(lookupPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transaction hashes for wallet: [%v]",
			err,
		)
	}

	state, exists := s.states[key]
	if !exists {
		state = &walletTransactionScanState{
			candidates: make(map[bitcoin.Hash]*bitcoin.Transaction),
		}
		s.states[key] = state
	}
	state.lastUsedRound = s.currentRound

	// txHashes is documented to be in ascending block-height order; index it
	// so the final result can be sorted back into that order regardless of
	// the (unordered) map iteration used to collect candidates below, and so
	// membership/resume-position checks below are O(1).
	hashIndex := make(map[bitcoin.Hash]int, len(txHashes))
	for i, hash := range txHashes {
		hashIndex[hash] = i
	}

	// Drop candidates the confirmed history no longer contains, e.g. after a
	// Bitcoin reorganization.
	for hash := range state.candidates {
		if _, present := hashIndex[hash]; !present {
			delete(state.candidates, hash)
		}
	}

	// A candidate remains a candidate only while still unproven; re-test
	// every round so a just-proven transaction is dropped promptly instead
	// of lingering until the cursor happens to pass it again.
	for hash, transaction := range state.candidates {
		stillUnproven, err := isUnproven(transaction)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to check if transaction is an unproven %s "+
					"transaction: [%v]",
				description,
				err,
			)
		}
		if !stillUnproven {
			delete(state.candidates, hash)
		}
	}

	if len(txHashes) > 0 && len(state.candidates) < transactionLimit {
		// Resume just before the last examined hash, scanning newest to
		// oldest. If there is no cursor yet, or its hash fell out of the
		// confirmed history, start from the newest hash.
		startIndex := len(txHashes)
		if state.lastExaminedHash != nil {
			if i, present := hashIndex[*state.lastExaminedHash]; present {
				startIndex = i
			}
		}

		examinedNew := 0
		index := startIndex
		// Bounding total iterations to len(txHashes) guarantees termination
		// in a single wrap of the history regardless of how transactionLimit
		// and transactionScanLimit relate to each other or to history size.
		// Checking the scan budget in the loop condition (rather than as a
		// post-work break) means a non-positive transactionScanLimit examines
		// zero new hashes instead of one.
		for steps := 0; steps < len(txHashes) &&
			examinedNew < transactionScanLimit; steps++ {
			index--
			if index < 0 {
				index = len(txHashes) - 1
			}

			hash := txHashes[index]
			state.lastExaminedHash = &hash

			if _, isCandidate := state.candidates[hash]; isCandidate {
				// Already classified as a matching candidate above; does not
				// use this round's new-examination budget.
				continue
			}

			transaction, err := btcChain.GetTransaction(hash)
			if err != nil {
				return nil, fmt.Errorf("cannot get transaction: [%v]", err)
			}

			matches, err := isUnproven(transaction)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to check if transaction is an unproven %s "+
						"transaction: [%v]",
					description,
					err,
				)
			}

			if matches {
				state.candidates[hash] = transaction
			}

			examinedNew++
			if len(state.candidates) >= transactionLimit {
				break
			}
		}
	}

	matches := make([]*bitcoin.Transaction, 0, len(state.candidates))
	for _, transaction := range state.candidates {
		matches = append(matches, transaction)
	}

	sort.Slice(matches, func(i, j int) bool {
		return hashIndex[matches[i].Hash()] < hashIndex[matches[j].Hash()]
	})

	return matches, nil
}
