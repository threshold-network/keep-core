package spv

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationProofScanState persists the incremental event-scan cursor and
// the set of still-pending action-request events across successive passes
// of runReservationProofLoop, so proveReservationAcceptanceActions and
// proveReservationReanchorActions scan only the event/Bitcoin history that
// has appeared since the previous pass instead of rescanning the full
// reservationDefaultLookBackBlocks window - and refetching Bitcoin history
// for every wallet in it - every config.IdleBackoffTime.
type reservationProofScanState struct {
	acceptanceLastScannedBlock uint64
	pendingAcceptanceEvents    map[string]*tbtc.ReservationAcceptanceRequestedEvent

	reanchorLastScannedBlock uint64
	pendingReanchorEvents    map[string]*tbtc.ReservationReanchorRequestedEvent

	// walletTransactionCache caches each wallet's confirmed Bitcoin
	// transaction hash set and bodies from the pass that fetched them, so
	// walletTransactionsForProof can skip the full GetTransactionsForPublicKeyHash
	// fetch on a later pass whose lightweight GetTxHashesForPublicKeyHash
	// check shows nothing changed for that wallet. Shared by
	// proveReservationAcceptanceActions and proveReservationReanchorActions,
	// since either can observe the same wallet's public key hash. Entries
	// are evicted once a wallet no longer appears in either pending-event
	// map, by evictStaleWalletTransactionCacheEntries at the end of each
	// runReservationProofLoop pass, so this cache does not grow
	// unboundedly for the life of the process as new wallets are observed.
	walletTransactionCache map[[20]byte]*walletTransactionCacheEntry
}

// walletTransactionCacheEntry holds one wallet's confirmed transaction hash
// set together with the corresponding transaction bodies fetched alongside
// it; see reservationProofScanState.walletTransactionCache.
type walletTransactionCacheEntry struct {
	txHashes     []bitcoin.Hash
	transactions []*bitcoin.Transaction
}

func newReservationProofScanState() *reservationProofScanState {
	return &reservationProofScanState{
		pendingAcceptanceEvents: make(map[string]*tbtc.ReservationAcceptanceRequestedEvent),
		pendingReanchorEvents:   make(map[string]*tbtc.ReservationReanchorRequestedEvent),
		walletTransactionCache:  make(map[[20]byte]*walletTransactionCacheEntry),
	}
}

// evictStaleWalletTransactionCacheEntries removes walletTransactionCache
// entries for wallets that no longer have any pending acceptance or
// re-anchor action tracked in state. It is called once per
// runReservationProofLoop pass, after both proveReservationAcceptanceActions
// and proveReservationReanchorActions have finished updating
// pendingAcceptanceEvents/pendingReanchorEvents for that pass, so a wallet
// whose last pending action just settled is evicted on the very next pass
// rather than lingering in memory - unbounded, one entry per distinct
// wallet ever observed - for the remaining lifetime of the process.
func evictStaleWalletTransactionCacheEntries(state *reservationProofScanState) {
	activeWallets := make(map[[20]byte]struct{}, len(state.walletTransactionCache))
	for _, event := range state.pendingAcceptanceEvents {
		activeWallets[event.WalletPublicKeyHash] = struct{}{}
	}
	for _, event := range state.pendingReanchorEvents {
		activeWallets[event.SourceWalletPublicKeyHash] = struct{}{}
	}

	for walletPublicKeyHash := range state.walletTransactionCache {
		if _, ok := activeWallets[walletPublicKeyHash]; !ok {
			delete(state.walletTransactionCache, walletPublicKeyHash)
		}
	}
}

// walletTransactionsForProof returns walletPublicKeyHash's confirmed
// Bitcoin transaction history needed to match pending reservation actions
// against. It first fetches only the wallet's confirmed transaction hashes
// (GetTxHashesForPublicKeyHash) - far cheaper than the full transaction
// bodies GetTransactionsForPublicKeyHash returns - and reuses the previous
// pass's fetched transaction bodies from state.walletTransactionCache when
// the hash set is unchanged since then, instead of unconditionally
// refetching every wallet's full history on every ~config.IdleBackoffTime
// pass regardless of whether anything happened on-chain for that wallet.
func walletTransactionsForProof(
	state *reservationProofScanState,
	btcChain bitcoin.Chain,
	walletPublicKeyHash [20]byte,
	limit int,
) ([]*bitcoin.Transaction, error) {
	txHashes, err := btcChain.GetTxHashesForPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transaction hashes for wallet: [%v]",
			err,
		)
	}

	if cached, ok := state.walletTransactionCache[walletPublicKeyHash]; ok &&
		reservationTransactionHashesEqual(cached.txHashes, txHashes) {
		return cached.transactions, nil
	}

	// Fetch transaction bodies for the already-known hash list instead of
	// calling GetTransactionsForPublicKeyHash, whose Electrum implementation
	// would otherwise re-fetch the same hash list via a second
	// GetTxHashesForPublicKeyHash call internally. Respect the limit by
	// taking the latest 'limit' hashes (hashes are ordered ascending, latest
	// at the end), matching GetTransactionsForPublicKeyHash's own semantics.
	selectedTxHashes := txHashes
	if len(txHashes) > limit {
		selectedTxHashes = txHashes[len(txHashes)-limit:]
	}

	transactions := make([]*bitcoin.Transaction, len(selectedTxHashes))
	for i, txHash := range selectedTxHashes {
		transaction, err := btcChain.GetTransaction(txHash)
		if err != nil {
			return nil, fmt.Errorf("cannot get transaction: [%v]", err)
		}

		transactions[i] = transaction
	}

	state.walletTransactionCache[walletPublicKeyHash] = &walletTransactionCacheEntry{
		txHashes:     txHashes,
		transactions: transactions,
	}

	return transactions, nil
}

// reservationTransactionHashesEqual reports whether a and b contain the
// same transaction hashes in the same order, as returned by
// GetTxHashesForPublicKeyHash across two passes.
func reservationTransactionHashesEqual(a, b []bitcoin.Hash) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// reservationEventKey identifies one reservation action generation, unique
// across both the acceptance and re-anchor pending-event maps.
func reservationEventKey(reservationKey *big.Int, requestNonce uint64) string {
	return fmt.Sprintf("%s:%d", reservationKey.String(), requestNonce)
}

// reservationProofNextScanRange returns the block range to scan for new
// pending-action-request events this pass: the bounded
// reservationDefaultLookBackBlocks catch-up window on the very first pass
// (lastScannedBlock == 0), or just the delta since the previous pass's
// cursor on every pass thereafter, so a steady-state loop no longer
// re-fetches the full ~30-day window on every config.IdleBackoffTime tick.
func reservationProofNextScanRange(
	spvChain Chain,
	lastScannedBlock uint64,
) (startBlock uint64, currentBlock uint64, err error) {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err = blockCounter.CurrentBlock()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current block: [%v]", err)
	}

	if lastScannedBlock == 0 {
		if currentBlock > reservationDefaultLookBackBlocks {
			return currentBlock - reservationDefaultLookBackBlocks, currentBlock, nil
		}
		return 0, currentBlock, nil
	}

	return lastScannedBlock + 1, currentBlock, nil
}

// maintainReservationProofs runs the SPV proof submission loop for
// reservation acceptance and re-anchor action generations. It is a
// dedicated loop, separate from spvMaintainer's generic proofTypes-driven
// control loop (see spv.go's Initialize), because
// SubmitReservationAcceptanceProof and SubmitReservationReanchorProof each
// require the (reservationKey, requestNonce) pair of the action generation
// being proven - context the generic
// unprovenTransactionsGetter/transactionProofSubmitter signatures (shared
// by deposit sweep, redemption, moving funds, and moved funds sweep) cannot
// carry.
//
// The loop shape mirrors spvMaintainer.startControlLoop/maintainSpv: an
// outer restart-backoff loop wraps an inner idle-backoff loop, so a
// transient error restarts after config.RestartBackoffTime and a clean pass
// with nothing to prove waits config.IdleBackoffTime before trying again.
func maintainReservationProofs(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	metricsRecorder MetricsRecorder,
) {
	logger.Info("starting reservation proof maintainer")

	defer func() {
		logger.Info("stopping reservation proof maintainer")
	}()

	for {
		err := runReservationProofLoop(ctx, config, spvChain, btcDiffChain, btcChain, metricsRecorder)
		if err != nil {
			logger.Errorf(
				"error while maintaining reservation proofs: [%v]; "+
					"restarting reservation proof maintainer",
				err,
			)
		}

		select {
		case <-time.After(config.RestartBackoffTime):
		case <-ctx.Done():
			return
		}
	}
}

// runReservationProofLoop repeatedly proves pending reservation acceptance
// and re-anchor action generations until ctx is done or an unrecoverable
// error occurs. Per-action errors (a single reservation's proof failing to
// assemble or submit) are logged and skipped rather than propagated, so one
// bad action generation does not block the rest; only a chain-wide failure
// (e.g. cannot read the current block) aborts the pass and triggers the
// outer restart backoff.
//
// A single reservationProofScanState is created once and threaded through
// every pass for the lifetime of the loop, carrying the incremental event
// cursor and pending-action set described on that type.
func runReservationProofLoop(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	metricsRecorder MetricsRecorder,
) error {
	state := newReservationProofScanState()

	for {
		// One cache per pass, shared by the acceptance and re-anchor proof
		// rounds below; see proofInfoCache in spv.go.
		cache := newProofInfoCache()
		if err := proveReservationAcceptanceActions(
			state,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
			cache,
			metricsRecorder,
		); err != nil {
			return fmt.Errorf(
				"error while proving reservation acceptance actions: [%v]",
				err,
			)
		}

		if err := proveReservationReanchorActions(
			state,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
			cache,
			metricsRecorder,
		); err != nil {
			return fmt.Errorf(
				"error while proving reservation re-anchor actions: [%v]",
				err,
			)
		}

		// Evict cache entries for wallets with no remaining pending
		// acceptance or re-anchor actions now that both proof-generation
		// passes for this iteration have updated the pending-event sets;
		// see evictStaleWalletTransactionCacheEntries.
		evictStaleWalletTransactionCacheEntries(state)

		select {
		case <-time.After(config.IdleBackoffTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// proveReservationAcceptanceActions finds pending ReservationAcceptance
// action generations, locates each one's already-broadcast anchor
// transaction on the Bitcoin chain (if any), and submits its SPV proof once
// it has accumulated enough confirmations.
func proveReservationAcceptanceActions(
	state *reservationProofScanState,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	cache *proofInfoCache,
	metricsRecorder MetricsRecorder,
) error {
	startBlock, currentBlock, err := reservationProofNextScanRange(
		spvChain,
		state.acceptanceLastScannedBlock,
	)
	if err != nil {
		return err
	}

	newEvents, err := spvChain.PastReservationAcceptanceRequestedEvents(
		&tbtc.ReservationAcceptanceRequestedEventFilter{
			StartBlock: startBlock,
			EndBlock:   &currentBlock,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation acceptance requested "+
				"events: [%v]",
			err,
		)
	}

	for _, event := range newEvents {
		key := reservationEventKey(event.ReservationKey, event.RequestNonce)
		state.pendingAcceptanceEvents[key] = event
	}

	// Re-check every tracked event's on-chain action state, evict settled/stale
	// ones, and group still-pending events by wallet public key hash.
	walletEvents := make(map[[20]byte][]*tbtc.ReservationAcceptanceRequestedEvent)
	for key, event := range state.pendingAcceptanceEvents {
		action, err := spvChain.GetReservationAction(
			event.ReservationKey,
			event.RequestNonce,
		)
		if err != nil {
			logger.Errorf(
				"failed to load reservation acceptance action [%v]/%d: [%v]",
				event.ReservationKey,
				event.RequestNonce,
				err,
			)
			continue
		}

		if action.State != tbtc.ReservationActionStatePending {
			delete(state.pendingAcceptanceEvents, key)
			continue
		}

		walletEvents[event.WalletPublicKeyHash] = append(
			walletEvents[event.WalletPublicKeyHash],
			event,
		)
	}

	for walletPublicKeyHash, events := range walletEvents {
		walletTransactions, err := walletTransactionsForProof(
			state,
			btcChain,
			walletPublicKeyHash,
			config.TransactionLimit,
		)
		if err != nil {
			logger.Errorf("failed to get transactions for wallet: [%v]", err)
			continue
		}

		// Index wallet transactions by deposit key for O(1) matching. A
		// wallet that broadcasts an RBF replacement chain for the same
		// deposit key produces multiple candidates per key, so every
		// candidate is kept rather than only the last one seen.
		candidateTransactions := make(map[string][]*bitcoin.Transaction)
		for _, transaction := range walletTransactions {
			if len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1 && transaction.Inputs[0].Outpoint != nil {
				input := transaction.Inputs[0]
				depositKey := spvChain.BuildDepositKey(
					input.Outpoint.TransactionHash,
					input.Outpoint.OutputIndex,
				)
				key := depositKey.String()
				candidateTransactions[key] = append(candidateTransactions[key], transaction)
			}
		}

		for _, event := range events {
			candidates, ok := candidateTransactions[event.ReservationKey.String()]
			if !ok {
				continue
			}

			if err := proveReservationTransaction(
				candidates,
				func(transaction *bitcoin.Transaction) bool {
					return isMatchingReservationAcceptanceTransaction(spvChain, event, transaction)
				},
				btcChain,
				spvChain,
				btcDiffChain,
				config.MaxProofHeaders,
				cache,
				metricsRecorder,
				func(transactionHash bitcoin.Hash, requiredConfirmations uint) error {
					return SubmitReservationAcceptanceProof(
						transactionHash,
						requiredConfirmations,
						event.ReservationKey,
						event.RequestNonce,
						btcChain,
						spvChain,
						metricsRecorder,
					)
				},
			); err != nil {
				logger.Errorf(
					"failed to prove reservation acceptance transaction "+
						"for reservation [%v]: [%v]",
					event.ReservationKey,
					err,
				)
				continue
			}
		}
	}

	state.acceptanceLastScannedBlock = currentBlock

	return nil
}

func isMatchingReservationAcceptanceTransaction(
	spvChain Chain,
	event *tbtc.ReservationAcceptanceRequestedEvent,
	transaction *bitcoin.Transaction,
) bool {
	if len(transaction.Inputs) != 1 || len(transaction.Outputs) != 1 || transaction.Inputs[0].Outpoint == nil {
		return false
	}

	input := transaction.Inputs[0]
	depositKey := spvChain.BuildDepositKey(
		input.Outpoint.TransactionHash,
		input.Outpoint.OutputIndex,
	)

	if depositKey.Cmp(event.ReservationKey) != 0 {
		return false
	}

	expectedScript, err := bitcoin.PayToWitnessPublicKeyHash(
		event.WalletPublicKeyHash,
	)
	if err != nil || !bytes.Equal(transaction.Outputs[0].PublicKeyScript, expectedScript) {
		return false
	}

	if depositRequest, found, err := spvChain.GetDepositRequest(
		input.Outpoint.TransactionHash,
		input.Outpoint.OutputIndex,
	); err != nil {
		return false
	} else if found {
		fee := int64(depositRequest.Amount) - transaction.Outputs[0].Value
		if fee <= 0 || (event.TxMaxFee > 0 && uint64(fee) > event.TxMaxFee) {
			return false
		}
	} else {
		fee := int64(event.DepositAmount) - transaction.Outputs[0].Value
		if fee <= 0 || (event.TxMaxFee > 0 && uint64(fee) > event.TxMaxFee) {
			return false
		}
	}

	return true
}

// proveReservationReanchorActions finds pending ReservationReanchor action
// generations, locates each one's already-broadcast re-anchor transaction
// on the Bitcoin chain (if any), and submits its SPV proof once it has
// accumulated enough confirmations.
func proveReservationReanchorActions(
	state *reservationProofScanState,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	cache *proofInfoCache,
	metricsRecorder MetricsRecorder,
) error {
	startBlock, currentBlock, err := reservationProofNextScanRange(
		spvChain,
		state.reanchorLastScannedBlock,
	)
	if err != nil {
		return err
	}

	newEvents, err := spvChain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{
			StartBlock: startBlock,
			EndBlock:   &currentBlock,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation re-anchor requested events: [%v]",
			err,
		)
	}

	for _, event := range newEvents {
		key := reservationEventKey(event.ReservationKey, event.RequestNonce)
		state.pendingReanchorEvents[key] = event
	}

	// Re-check every tracked event's on-chain action state, evict settled/stale
	// ones, and group still-pending events by source wallet public key hash.
	walletEvents := make(map[[20]byte][]*tbtc.ReservationReanchorRequestedEvent)
	for key, event := range state.pendingReanchorEvents {
		action, err := spvChain.GetReservationAction(
			event.ReservationKey,
			event.RequestNonce,
		)
		if err != nil {
			logger.Errorf(
				"failed to load reservation re-anchor action [%v]/%d: [%v]",
				event.ReservationKey,
				event.RequestNonce,
				err,
			)
			continue
		}

		if action.State != tbtc.ReservationActionStatePending {
			delete(state.pendingReanchorEvents, key)
			continue
		}

		walletEvents[event.SourceWalletPublicKeyHash] = append(
			walletEvents[event.SourceWalletPublicKeyHash],
			event,
		)
	}

	for walletPublicKeyHash, events := range walletEvents {
		walletTransactions, err := walletTransactionsForProof(
			state,
			btcChain,
			walletPublicKeyHash,
			config.TransactionLimit,
		)
		if err != nil {
			logger.Errorf("failed to get transactions for wallet: [%v]", err)
			continue
		}

		// Index wallet transactions by spent outpoint for O(1) matching. A
		// wallet that broadcasts an RBF replacement chain for the same
		// anchor UTXO produces multiple candidates per outpoint, so every
		// candidate is kept rather than only the last one seen.
		candidateTransactions := make(map[bitcoin.TransactionOutpoint][]*bitcoin.Transaction)
		for _, transaction := range walletTransactions {
			if len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1 && transaction.Inputs[0].Outpoint != nil {
				outpoint := *transaction.Inputs[0].Outpoint
				candidateTransactions[outpoint] = append(candidateTransactions[outpoint], transaction)
			}
		}

		for _, event := range events {
			reservation, err := spvChain.GetReservation(event.ReservationKey)
			if err != nil {
				logger.Errorf(
					"failed to load reservation [%v]: [%v]",
					event.ReservationKey,
					err,
				)
				continue
			}
			if reservation.AnchorUtxo == nil ||
				reservation.AnchorUtxo.Value == 0 ||
				reservation.AnchorUtxo.Outpoint == nil ||
				reservation.AnchorUtxo.Outpoint.TransactionHash == (bitcoin.Hash{}) {
				logger.Errorf(
					"reservation [%v] has no anchor UTXO to re-anchor from",
					event.ReservationKey,
				)
				continue
			}

			candidates, ok := candidateTransactions[*reservation.AnchorUtxo.Outpoint]
			if !ok {
				continue
			}

			if err := proveReservationTransaction(
				candidates,
				func(transaction *bitcoin.Transaction) bool {
					return isMatchingReservationReanchorTransaction(event, reservation.AnchorUtxo, transaction)
				},
				btcChain,
				spvChain,
				btcDiffChain,
				config.MaxProofHeaders,
				cache,
				metricsRecorder,
				func(transactionHash bitcoin.Hash, requiredConfirmations uint) error {
					return SubmitReservationReanchorProof(
						transactionHash,
						requiredConfirmations,
						event.ReservationKey,
						event.RequestNonce,
						btcChain,
						spvChain,
						metricsRecorder,
					)
				},
			); err != nil {
				logger.Errorf(
					"failed to prove reservation re-anchor transaction "+
						"for reservation [%v]: [%v]",
					event.ReservationKey,
					err,
				)
				continue
			}
		}
	}

	state.reanchorLastScannedBlock = currentBlock

	return nil
}

func isMatchingReservationReanchorTransaction(
	event *tbtc.ReservationReanchorRequestedEvent,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	transaction *bitcoin.Transaction,
) bool {
	if len(transaction.Inputs) != 1 || len(transaction.Outputs) != 1 || transaction.Inputs[0].Outpoint == nil {
		return false
	}

	input := transaction.Inputs[0]
	if input.Outpoint.TransactionHash != anchorUtxo.Outpoint.TransactionHash ||
		input.Outpoint.OutputIndex != anchorUtxo.Outpoint.OutputIndex {
		return false
	}

	expectedScript, err := bitcoin.PayToWitnessPublicKeyHash(
		event.TargetWalletPublicKeyHash,
	)
	if err != nil || !bytes.Equal(transaction.Outputs[0].PublicKeyScript, expectedScript) {
		return false
	}

	fee := int64(anchorUtxo.Value) - transaction.Outputs[0].Value
	if fee <= 0 || (event.TxMaxFee > 0 && uint64(fee) > event.TxMaxFee) {
		return false
	}

	return true
}

// proveReservationTransaction assembles and submits the SPV proof for a
// reservation acceptance or re-anchor transaction, once it has accumulated
// enough confirmations and its proof falls within the relay's difficulty
// range.
//
// candidates holds every wallet transaction spending the acceptance/
// re-anchor action's expected outpoint that was observed on this pass; a
// wallet that broadcasts an RBF (replace-by-fee) replacement chain for the
// same spend can have more than one, and the order candidates were
// collected in is not guaranteed to match confirmation order. candidates
// are walked in order and the first one that both matches the expected
// transaction shape (isMatch) and has already accumulated enough
// confirmations is proved, so a still-pending earlier-seen replacement
// never silently blocks a later, already-confirmed one. If no candidate
// has enough confirmations yet, the first matching candidate's skip/
// confirmation state is logged, mirroring the previous single-candidate
// behavior.
//
// cache carries the pass-invariant chain reads shared with every other
// transaction proved in the same pass; see proofInfoCache.
func proveReservationTransaction(
	candidates []*bitcoin.Transaction,
	isMatch func(transaction *bitcoin.Transaction) bool,
	btcChain bitcoin.Chain,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	maxProofHeaders uint,
	cache *proofInfoCache,
	metricsRecorder MetricsRecorder,
	submit func(transactionHash bitcoin.Hash, requiredConfirmations uint) error,
) error {
	var pending *bitcoin.Transaction
	var pendingConfirmations, pendingRequired uint
	var pendingSkipReason proofSkipReason

	for _, transaction := range candidates {
		if !isMatch(transaction) {
			continue
		}

		accumulatedConfirmations, requiredConfirmations, skipReason, err := getProofInfo(
			transaction.Hash(),
			btcChain,
			spvChain,
			btcDiffChain,
			maxProofHeaders,
			cache,
		)
		if err != nil {
			return fmt.Errorf("failed to get proof info: [%v]", err)
		}

		if skipReason == proofSkipNone && accumulatedConfirmations >= requiredConfirmations {
			if err := submit(transaction.Hash(), requiredConfirmations); err != nil {
				return err
			}

			logger.Infof(
				"successfully submitted proof for transaction [%s]",
				transaction.Hash().Hex(bitcoin.ReversedByteOrder),
			)

			return nil
		}

		if pending == nil {
			pending = transaction
			pendingConfirmations = accumulatedConfirmations
			pendingRequired = requiredConfirmations
			pendingSkipReason = skipReason
		}
	}

	if pending == nil {
		return nil
	}

	transactionHashStr := pending.Hash().Hex(bitcoin.ReversedByteOrder)

	switch pendingSkipReason {
	case proofSkipOutsideRelayRange:
		logger.Warnf(
			"skipped proving transaction [%s]; the range of the "+
				"required proof goes outside the previous and current "+
				"difficulty epochs as seen by the relay",
			transactionHashStr,
		)
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(
				clientinfo.MetricSpvProofSkippedOutsideRelayRangeTotal,
				1,
			)
		}
		return nil
	case proofSkipExceededMaxHeaders:
		logger.Errorf(
			"skipped proving transaction [%s]; could not find a decisive "+
				"header or accumulate enough difficulty within [%d] "+
				"headers; the transaction may be permanently unprovable",
			transactionHashStr,
			maxProofHeaders,
		)
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(
				clientinfo.MetricSpvProofSkippedExceededMaxHeadersTotal,
				1,
			)
		}
		return nil
	case proofSkipNone:
		logger.Infof(
			"skipped proving transaction [%s]; transaction has [%v/%v] "+
				"confirmations",
			transactionHashStr,
			pendingConfirmations,
			pendingRequired,
		)
		return nil
	default:
		return fmt.Errorf(
			"unexpected proof skip reason [%d] for transaction [%s]",
			pendingSkipReason,
			transactionHashStr,
		)
	}
}
