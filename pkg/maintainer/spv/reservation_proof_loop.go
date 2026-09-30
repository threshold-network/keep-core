package spv

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationProofScanState persists the incremental event-scan cursor and
// the set of still-settleable action-request events across successive
// passes of runReservationProofLoop, so proveReservationAcceptanceActions
// and proveReservationReanchorActions scan only the event/Bitcoin history
// that has appeared since the previous pass instead of rescanning the full
// reservationDefaultLookBackBlocks window - and refetching Bitcoin history
// for every wallet in it - every config.IdleBackoffTime.
type reservationProofScanState struct {
	acceptanceLastScannedBlock uint64
	pendingAcceptanceEvents    map[string]*tbtc.ReservationAcceptanceRequestedEvent

	reanchorLastScannedBlock uint64
	pendingReanchorEvents    map[string]*tbtc.ReservationReanchorRequestedEvent

	// reanchorPass counts proveReservationReanchorActions passes;
	// reanchorBackoff holds, per pendingReanchorEvents key, the backoff
	// delay of a TimedOut generation whose last check found nothing to
	// prove (see reanchorProofBackoff).
	reanchorPass    uint64
	reanchorBackoff map[string]*reanchorProofBackoff

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

	// nowFn returns the current UNIX timestamp the scans use to evaluate
	// the Bridge's late-settlement window for TimedOut acceptance
	// generations. Production runs on wall time; tests override it to
	// drive the deadline deterministically.
	nowFn func() uint32
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
		reanchorBackoff:         make(map[string]*reanchorProofBackoff),
		walletTransactionCache:  make(map[[20]byte]*walletTransactionCacheEntry),
		nowFn:                   defaultReservationProofNowFn,
	}
}

// defaultReservationProofNowFn returns time.Now() as a uint32 UNIX
// timestamp. Kept separate from the struct so tests can swap it
// deterministically, mirroring the watcher's nowFn pattern.
func defaultReservationProofNowFn() uint32 {
	return uint32(time.Now().Unix())
}

// reanchorSourceAnchorHash computes exactly the Bridge's
// anchorUtxoHash(reservation) from Reservation.sol:
// keccak256(abi.encodePacked(anchorTxHash, uint32 anchorTxOutputIndex)) -
// the 32-byte anchor transaction hash in its Bitcoin internal byte order
// (the order the Bridge stores it, matching the
// transaction.Hash bytes), followed by the output index as a
// big-endian uint32. It is the hash the Bridge's
// requireCurrentSourceAnchor check in ReservationProofs.sol compares
// against each tracked generation's on-chain source anchor snapshot. A
// reservation with no anchor outpoint yields the zero hash.
func reanchorSourceAnchorHash(reservation *tbtc.Reservation) [32]byte {
	if reservation == nil ||
		reservation.AnchorUtxo == nil ||
		reservation.AnchorUtxo.Outpoint == nil ||
		reservation.AnchorUtxo.Outpoint.TransactionHash == (bitcoin.Hash{}) {
		return [32]byte{}
	}
	outpoint := reservation.AnchorUtxo.Outpoint
	packed := make([]byte, 36)
	copy(packed, outpoint.TransactionHash[:])
	binary.BigEndian.PutUint32(packed[32:], outpoint.OutputIndex)
	return crypto.Keccak256Hash(packed)
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
// A single reservationProofScanState is created once here and threaded
// through every restarted pass, so after a transient chain-wide read error
// a restart resumes each scan from its last completed position instead
// of replaying the activation-to-tip history.
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

	// Created once for the maintainer's lifetime and threaded through
	// every restarted inner loop, so a restart after a chain-wide read
	// error resumes each scan from its last completed position instead
	// of re-scanning activation to tip; see runReservationProofLoop.
	state := newReservationProofScanState()

	for {
		err := runReservationProofLoop(
			ctx,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
			metricsRecorder,
			state,
		)
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
// The caller creates the single reservationProofScanState and passes it in
// (maintainReservationProofs creates one per maintainer lifetime),
// carrying the incremental event cursor and pending-action set described
// on that type across every pass and every restart of the loop.
func runReservationProofLoop(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	metricsRecorder MetricsRecorder,
	state *reservationProofScanState,
) error {
	for {
		// One cache per pass, shared by the acceptance and re-anchor proof
		// rounds below; see proofInfoCache in spv.go.
		cache := newProofInfoCache()
		// The re-anchor round runs even when the acceptance round fails,
		// so one failing event scan does not hold back the other kind of
		// proof; the first error is returned once both rounds are done.
		acceptanceErr := proveReservationAcceptanceActions(
			state,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
			cache,
			metricsRecorder,
		)

		reanchorErr := proveReservationReanchorActions(
			state,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
			cache,
			metricsRecorder,
		)

		// Evict cache entries for wallets with no remaining pending
		// acceptance or re-anchor actions now that both proof-generation
		// passes for this iteration have updated the pending-event sets;
		// see evictStaleWalletTransactionCacheEntries.
		evictStaleWalletTransactionCacheEntries(state)

		if acceptanceErr != nil {
			return fmt.Errorf(
				"error while proving reservation acceptance actions: [%v]",
				acceptanceErr,
			)
		}
		if reanchorErr != nil {
			return fmt.Errorf(
				"error while proving reservation re-anchor actions: [%v]",
				reanchorErr,
			)
		}

		select {
		case <-time.After(config.IdleBackoffTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// proveReservationAcceptanceActions finds settleable ReservationAcceptance
// action generations, locates each one's already-broadcast anchor
// transaction on the Bitcoin chain (if any), and submits its SPV proof
// once it has accumulated enough confirmations. TimedOut generations
// remain candidates through the Bridge's late settlement window.
func proveReservationAcceptanceActions(
	state *reservationProofScanState,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	cache *proofInfoCache,
	metricsRecorder MetricsRecorder,
) error {
	startBlock, endBlock, scan, err := nextReservationScanRange(
		spvChain,
		state.acceptanceLastScannedBlock,
		tbtc.ReservationsActivationBlock(config.EthereumNetwork),
	)
	if err != nil {
		return err
	}

	// A scan error does not stop this pass: the chunks fetched before it
	// are already tracked, and every tracked generation is still proved
	// below. The error is returned once the pass is done.
	var scanErr error
	if scan {
		if err := fetchPastEventsInChunks(
			func(chunkStart, chunkEnd uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				return spvChain.PastReservationAcceptanceRequestedEvents(
					&tbtc.ReservationAcceptanceRequestedEventFilter{
						StartBlock: chunkStart,
						EndBlock:   &chunkEnd,
					},
				)
			},
			startBlock,
			endBlock,
			func(events []*tbtc.ReservationAcceptanceRequestedEvent, chunkEnd uint64) {
				for _, event := range events {
					key := reservationEventKey(event.ReservationKey, event.RequestNonce)
					state.pendingAcceptanceEvents[key] = event
				}
				state.acceptanceLastScannedBlock = chunkEnd
			},
		); err != nil {
			scanErr = fmt.Errorf(
				"failed to get past reservation acceptance requested "+
					"events: [%v]",
				err,
			)
		}
	} else {
		state.acceptanceLastScannedBlock = endBlock
	}

	// Re-check every tracked event's on-chain action state and evict the
	// non-settleable ones. The Bridge settles a generation while its
	// action is Pending or TimedOut: a TimedOut acceptance generation
	// stays settleable only through timeoutAt + termSeconds (see
	// loadSettleableAction in ReservationProofs.sol), so the scans keep
	// TimedOut candidates until that window has closed instead of
	// deleting them the moment the state flips.
	now := state.nowFn()
	var generations []reservationProofGeneration[*tbtc.ReservationAcceptanceRequestedEvent]
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

		switch action.State {
		case tbtc.ReservationActionStateTimedOut:
			// The Bridge settles this generation while
			// now <= timeoutAt + termSeconds, where termSeconds is the
			// custody term snapshotted onto the action when the
			// generation was requested. A governance change to the live
			// reservationTermSeconds parameter does not shrink that
			// window, so evict only once now has passed the snapshotted
			// bound.
			if uint64(now) > uint64(action.TimeoutAt)+uint64(action.TermSeconds) {
				delete(state.pendingAcceptanceEvents, key)
				continue
			}

			// Once any generation of this reservation has settled, the
			// Bridge rejects every other one ("Reservation already
			// exists" in submitReservationAcceptanceProof). Settling
			// also marks the deposit swept, and a reserved deposit
			// cannot be swept any other way, so the reservation state
			// alone decides it.
			reservation, err := spvChain.GetReservation(event.ReservationKey)
			if err != nil {
				logger.Errorf(
					"failed to load reservation [%v] for timed-out "+
						"acceptance generation %d: [%v]",
					event.ReservationKey,
					event.RequestNonce,
					err,
				)
				continue
			}
			if reservation.State != tbtc.ReservationStateUnknown {
				delete(state.pendingAcceptanceEvents, key)
				continue
			}
		case tbtc.ReservationActionStatePending:
		default:
			// Settled, Superseded, Vetoed and absent generations are not
			// settleable on the Bridge; stop tracking them.
			delete(state.pendingAcceptanceEvents, key)
			continue
		}

		generations = append(
			generations,
			reservationProofGeneration[*tbtc.ReservationAcceptanceRequestedEvent]{
				event:          event,
				reservationKey: event.ReservationKey,
				requestNonce:   event.RequestNonce,
				pending:        action.State == tbtc.ReservationActionStatePending,
			},
		)
	}
	sortReservationProofGenerations(generations)

	// Each wallet's candidate transactions are indexed once per pass, by
	// deposit key. A wallet that broadcasts an RBF replacement chain for
	// the same deposit key produces multiple candidates per key, so
	// every candidate is kept rather than only the last one seen. A nil
	// entry marks a wallet whose transactions could not be fetched this
	// pass.
	walletCandidates := make(map[[20]byte]map[string][]*bitcoin.Transaction)
	submittedReservations := make(map[string]struct{})
	for _, generation := range generations {
		event := generation.event
		reservationKey := event.ReservationKey.String()
		if _, submitted := submittedReservations[reservationKey]; submitted {
			continue
		}

		candidateTransactions, indexed := walletCandidates[event.WalletPublicKeyHash]
		if !indexed {
			walletTransactions, err := walletTransactionsForProof(
				state,
				btcChain,
				event.WalletPublicKeyHash,
				config.TransactionLimit,
			)
			if err != nil {
				logger.Errorf("failed to get transactions for wallet: [%v]", err)
			} else {
				candidateTransactions = make(map[string][]*bitcoin.Transaction)
				for _, transaction := range walletTransactions {
					if len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1 && transaction.Inputs[0].Outpoint != nil {
						input := transaction.Inputs[0]
						depositKey := spvChain.BuildDepositKey(
							input.Outpoint.TransactionHash,
							input.Outpoint.OutputIndex,
						).String()
						candidateTransactions[depositKey] = append(
							candidateTransactions[depositKey],
							transaction,
						)
					}
				}
			}
			walletCandidates[event.WalletPublicKeyHash] = candidateTransactions
		}

		candidates, ok := candidateTransactions[reservationKey]
		if !ok {
			continue
		}

		outcome, err := proveReservationTransaction(
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
		)
		if outcome == reservationProofSubmitted {
			submittedReservations[reservationKey] = struct{}{}
		}
		if err != nil {
			logger.Errorf(
				"failed to prove reservation acceptance transaction "+
					"for reservation [%v]: [%v]",
				event.ReservationKey,
				err,
			)
			continue
		}
	}

	return scanErr
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

	// The output must lock the authorized wallet's 20-byte public key
	// hash, mirroring extractPubKeyHash in BitcoinTx.sol which accepts
	// both the P2PKH and P2WPKH encodings of that hash; the authorized
	// signer may broadcast either encoding.
	actualPublicKeyHash, err := bitcoin.ExtractPublicKeyHash(
		transaction.Outputs[0].PublicKeyScript,
	)
	if err != nil || actualPublicKeyHash != event.WalletPublicKeyHash {
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

// reservationProofGeneration is one still-settleable action generation a
// proof loop pass may prove. reservation is the reservation record read
// while classifying a re-anchor generation, reused when proving it; it is
// nil for acceptance generations.
type reservationProofGeneration[E any] struct {
	key            string
	event          E
	reservationKey *big.Int
	requestNonce   uint64
	pending        bool
	reservation    *tbtc.Reservation
}

// sortReservationProofGenerations orders generations for proof
// submission: grouped by reservation key, the Pending generation first,
// then TimedOut generations from the highest nonce down. Several
// generations of one reservation can match the same Bitcoin transaction,
// but only one proof can settle it; the proof loops submit at most one
// proof per reservation key per pass, walking generations in this order.
func sortReservationProofGenerations[E any](
	generations []reservationProofGeneration[E],
) {
	sort.Slice(generations, func(i, j int) bool {
		a, b := generations[i], generations[j]
		if c := a.reservationKey.Cmp(b.reservationKey); c != 0 {
			return c < 0
		}
		if a.pending != b.pending {
			return a.pending
		}
		return a.requestNonce > b.requestNonce
	})
}

// settleableReanchorReservation loads the reservation of the tracked
// re-anchor generation identified by key and reports whether the
// generation is still worth proving this pass. A read error keeps the
// generation tracked but skips it for this pass.
//
// A TimedOut generation that can never settle again is evicted from
// state. The Bridge settles a late re-anchor proof only while the
// reservation is Active, ActionPending or Stranded
// (prepareReservationForSettlement in ReservationProofs.sol) and while
// the generation's snapshotted source anchor still matches the
// reservation's current anchor (requireCurrentSourceAnchor). The
// comparison is keyed on the action's on-chain sourceAnchorUtxoHash
// snapshot, not on any Go-side first-observation snapshot, so a process
// restart still evicts a generation whose source anchor was already
// replaced before the loop ever observed it. Re-anchor late settlement
// is otherwise unbounded, so age alone never triggers an eviction.
func settleableReanchorReservation(
	state *reservationProofScanState,
	spvChain Chain,
	key string,
	event *tbtc.ReservationReanchorRequestedEvent,
	action *tbtc.ReservationAction,
) (*tbtc.Reservation, bool) {
	reservation, err := spvChain.GetReservation(event.ReservationKey)
	if err != nil {
		logger.Errorf(
			"failed to load reservation [%v]: [%v]",
			event.ReservationKey,
			err,
		)
		return nil, false
	}

	if action.State != tbtc.ReservationActionStateTimedOut {
		return reservation, true
	}

	switch reservation.State {
	case tbtc.ReservationStateActive,
		tbtc.ReservationStateActionPending,
		tbtc.ReservationStateStranded:
	default:
		forgetReanchorGeneration(state, key)
		return nil, false
	}

	if reanchorSourceAnchorHash(reservation) != action.SourceAnchorUtxoHash {
		// The anchor outpoint moved past this generation's on-chain
		// source anchor; the Bridge will reject its late settlement
		// forever.
		forgetReanchorGeneration(state, key)
		return nil, false
	}

	return reservation, true
}

// reanchorProofBackoffMaxPasses caps the number of proof loop passes a
// TimedOut re-anchor generation is skipped for after a check that found
// nothing to prove.
const reanchorProofBackoffMaxPasses = 32

// reanchorProofBackoff delays re-checking a TimedOut re-anchor generation
// whose last check found no matching transaction, or only transactions
// whose proof cannot be built (outside the relay's difficulty range or
// longer than MaxProofHeaders). Such a generation can stay tracked
// indefinitely while its source anchor does not move, and re-checking it
// costs chain reads every pass. Some of those skip reasons clear with
// time (a transaction too fresh for the relay), so the delay is bounded.
type reanchorProofBackoff struct {
	// retryPass is the first proof loop pass that checks the generation
	// again.
	retryPass uint64
	// skipPasses is the current delay in passes; it doubles after each
	// fruitless check, up to reanchorProofBackoffMaxPasses.
	skipPasses uint64
	// walletTxHashes is the source wallet's confirmed transaction hash
	// list when the delay was set; a different list in the wallet
	// transaction cache ends the delay early.
	walletTxHashes []bitcoin.Hash
}

// reanchorGenerationBackedOff reports whether the re-anchor generation
// identified by key is still inside its backoff delay this pass. The
// delay ends early once the source wallet's cached transaction list
// differs from the one recorded when the delay was set.
func reanchorGenerationBackedOff(
	state *reservationProofScanState,
	key string,
	sourceWalletPublicKeyHash [20]byte,
) bool {
	backoff, ok := state.reanchorBackoff[key]
	if !ok {
		return false
	}

	if cached, ok := state.walletTransactionCache[sourceWalletPublicKeyHash]; ok &&
		!reservationTransactionHashesEqual(cached.txHashes, backoff.walletTxHashes) {
		delete(state.reanchorBackoff, key)
		return false
	}

	return state.reanchorPass < backoff.retryPass
}

// backOffReanchorGeneration starts or extends the backoff delay of the
// re-anchor generation identified by key after a fruitless check.
func backOffReanchorGeneration(
	state *reservationProofScanState,
	key string,
	sourceWalletPublicKeyHash [20]byte,
) {
	skipPasses := uint64(1)
	if previous, ok := state.reanchorBackoff[key]; ok {
		skipPasses = min(previous.skipPasses*2, reanchorProofBackoffMaxPasses)
	}

	var walletTxHashes []bitcoin.Hash
	if cached, ok := state.walletTransactionCache[sourceWalletPublicKeyHash]; ok {
		walletTxHashes = cached.txHashes
	}

	state.reanchorBackoff[key] = &reanchorProofBackoff{
		retryPass:      state.reanchorPass + skipPasses + 1,
		skipPasses:     skipPasses,
		walletTxHashes: walletTxHashes,
	}
}

// forgetReanchorGeneration stops tracking the re-anchor generation
// identified by key.
func forgetReanchorGeneration(state *reservationProofScanState, key string) {
	delete(state.pendingReanchorEvents, key)
	delete(state.reanchorBackoff, key)
}

// proveReservationReanchorActions finds settleable ReservationReanchor
// action generations, locates each one's already-broadcast re-anchor
// transaction on the Bitcoin chain (if any), and submits its SPV proof
// once it has accumulated enough confirmations. A TimedOut generation
// remains a candidate for late settlement without bound while the
// reservation is settleable and its on-chain source anchor is still the
// reservation's current anchor outpoint; otherwise it is evicted (see
// settleableReanchorReservation). A TimedOut generation with nothing to
// prove is re-checked on a bounded backoff (see reanchorProofBackoff).
func proveReservationReanchorActions(
	state *reservationProofScanState,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	cache *proofInfoCache,
	metricsRecorder MetricsRecorder,
) error {
	startBlock, endBlock, scan, err := nextReservationScanRange(
		spvChain,
		state.reanchorLastScannedBlock,
		tbtc.ReservationsActivationBlock(config.EthereumNetwork),
	)
	if err != nil {
		return err
	}

	// See proveReservationAcceptanceActions: a scan error is returned
	// after the already-tracked generations have been proved.
	var scanErr error
	if scan {
		if err := fetchPastEventsInChunks(
			func(chunkStart, chunkEnd uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				return spvChain.PastReservationReanchorRequestedEvents(
					&tbtc.ReservationReanchorRequestedEventFilter{
						StartBlock: chunkStart,
						EndBlock:   &chunkEnd,
					},
				)
			},
			startBlock,
			endBlock,
			func(events []*tbtc.ReservationReanchorRequestedEvent, chunkEnd uint64) {
				for _, event := range events {
					key := reservationEventKey(event.ReservationKey, event.RequestNonce)
					state.pendingReanchorEvents[key] = event
				}
				state.reanchorLastScannedBlock = chunkEnd
			},
		); err != nil {
			scanErr = fmt.Errorf(
				"failed to get past reservation re-anchor requested events: [%v]",
				err,
			)
		}
	} else {
		state.reanchorLastScannedBlock = endBlock
	}

	state.reanchorPass++

	// Re-check every tracked event's on-chain action state and evict the
	// non-settleable ones. The Bridge settles a re-anchor generation
	// while its action is Pending or TimedOut, and re-anchor late
	// settlement is unbounded (see loadSettleableAction in
	// ReservationProofs.sol), so a TimedOut generation is kept for as
	// long as settleableReanchorReservation finds it can still settle.
	var generations []reservationProofGeneration[*tbtc.ReservationReanchorRequestedEvent]
	for key, event := range state.pendingReanchorEvents {
		if reanchorGenerationBackedOff(state, key, event.SourceWalletPublicKeyHash) {
			continue
		}

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

		switch action.State {
		case tbtc.ReservationActionStatePending, tbtc.ReservationActionStateTimedOut:
		default:
			// Settled, Superseded, Vetoed and absent generations are not
			// settleable on the Bridge; stop tracking them.
			forgetReanchorGeneration(state, key)
			continue
		}

		reservation, settleable := settleableReanchorReservation(
			state,
			spvChain,
			key,
			event,
			action,
		)
		if !settleable {
			continue
		}

		generations = append(
			generations,
			reservationProofGeneration[*tbtc.ReservationReanchorRequestedEvent]{
				key:            key,
				event:          event,
				reservationKey: event.ReservationKey,
				requestNonce:   event.RequestNonce,
				pending:        action.State == tbtc.ReservationActionStatePending,
				reservation:    reservation,
			},
		)
	}
	sortReservationProofGenerations(generations)

	// Each source wallet's candidate transactions are indexed once per
	// pass, by spent outpoint. A wallet that broadcasts an RBF
	// replacement chain for the same anchor UTXO produces multiple
	// candidates per outpoint, so every candidate is kept rather than
	// only the last one seen. A nil entry marks a wallet whose
	// transactions could not be fetched this pass.
	walletCandidates := make(map[[20]byte]map[bitcoin.TransactionOutpoint][]*bitcoin.Transaction)
	submittedReservations := make(map[string]struct{})
	for _, generation := range generations {
		event := generation.event
		reservation := generation.reservation
		reservationKey := event.ReservationKey.String()
		if _, submitted := submittedReservations[reservationKey]; submitted {
			continue
		}

		candidateTransactions, indexed := walletCandidates[event.SourceWalletPublicKeyHash]
		if !indexed {
			walletTransactions, err := walletTransactionsForProof(
				state,
				btcChain,
				event.SourceWalletPublicKeyHash,
				config.TransactionLimit,
			)
			if err != nil {
				logger.Errorf("failed to get transactions for wallet: [%v]", err)
			} else {
				candidateTransactions = make(map[bitcoin.TransactionOutpoint][]*bitcoin.Transaction)
				for _, transaction := range walletTransactions {
					if len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1 && transaction.Inputs[0].Outpoint != nil {
						outpoint := *transaction.Inputs[0].Outpoint
						candidateTransactions[outpoint] = append(candidateTransactions[outpoint], transaction)
					}
				}
			}
			walletCandidates[event.SourceWalletPublicKeyHash] = candidateTransactions
		}
		if candidateTransactions == nil {
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

		outcome := reservationProofNoMatch
		var err error
		if candidates, ok := candidateTransactions[*reservation.AnchorUtxo.Outpoint]; ok {
			outcome, err = proveReservationTransaction(
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
			)
		}
		if outcome == reservationProofSubmitted {
			submittedReservations[reservationKey] = struct{}{}
		}
		if err != nil {
			logger.Errorf(
				"failed to prove reservation re-anchor transaction "+
					"for reservation [%v]: [%v]",
				event.ReservationKey,
				err,
			)
			continue
		}

		if !generation.pending &&
			(outcome == reservationProofNoMatch || outcome == reservationProofUnprovable) {
			backOffReanchorGeneration(state, generation.key, event.SourceWalletPublicKeyHash)
		} else {
			delete(state.reanchorBackoff, generation.key)
		}
	}

	return scanErr
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

	// The output must lock the authorized target wallet's 20-byte public
	// key hash, mirroring extractPubKeyHash in BitcoinTx.sol which
	// accepts both the P2PKH and P2WPKH encodings of that hash; the
	// authorized signer may broadcast either encoding.
	actualPublicKeyHash, err := bitcoin.ExtractPublicKeyHash(
		transaction.Outputs[0].PublicKeyScript,
	)
	if err != nil || actualPublicKeyHash != event.TargetWalletPublicKeyHash {
		return false
	}

	fee := int64(anchorUtxo.Value) - transaction.Outputs[0].Value
	if fee <= 0 || (event.TxMaxFee > 0 && uint64(fee) > event.TxMaxFee) {
		return false
	}

	return true
}

// reservationProofOutcome reports what proveReservationTransaction did
// with one action generation's candidate transactions.
type reservationProofOutcome uint8

const (
	// reservationProofNoMatch means no candidate matched the generation
	// (or proof info could not be read, reported with an error).
	reservationProofNoMatch reservationProofOutcome = iota
	// reservationProofAwaitingConfirmations means a matching candidate is
	// still accumulating confirmations.
	reservationProofAwaitingConfirmations
	// reservationProofUnprovable means every matching candidate was
	// skipped because its proof falls outside the relay's difficulty
	// range or needs more than maxProofHeaders headers.
	reservationProofUnprovable
	// reservationProofSubmitted means a proof submission was attempted;
	// the returned error reports whether it failed.
	reservationProofSubmitted
)

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
// transaction proved in the same pass; see proofInfoCache. The returned
// outcome tells callers whether a submission was attempted and, if not,
// whether a later pass can expect anything to prove.
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
) (reservationProofOutcome, error) {
	var pending *bitcoin.Transaction
	awaitingConfirmations := false
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
			return reservationProofNoMatch, fmt.Errorf("failed to get proof info: [%v]", err)
		}

		if skipReason == proofSkipNone && accumulatedConfirmations >= requiredConfirmations {
			if err := submit(transaction.Hash(), requiredConfirmations); err != nil {
				return reservationProofSubmitted, err
			}

			logger.Infof(
				"successfully submitted proof for transaction [%s]",
				transaction.Hash().Hex(bitcoin.ReversedByteOrder),
			)

			return reservationProofSubmitted, nil
		}

		if skipReason == proofSkipNone {
			awaitingConfirmations = true
		}
		if pending == nil {
			pending = transaction
			pendingConfirmations = accumulatedConfirmations
			pendingRequired = requiredConfirmations
			pendingSkipReason = skipReason
		}
	}

	if pending == nil {
		return reservationProofNoMatch, nil
	}

	outcome := reservationProofUnprovable
	if awaitingConfirmations {
		outcome = reservationProofAwaitingConfirmations
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
		return outcome, nil
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
		return outcome, nil
	case proofSkipNone:
		logger.Infof(
			"skipped proving transaction [%s]; transaction has [%v/%v] "+
				"confirmations",
			transactionHashStr,
			pendingConfirmations,
			pendingRequired,
		)
		return outcome, nil
	default:
		return reservationProofNoMatch, fmt.Errorf(
			"unexpected proof skip reason [%d] for transaction [%s]",
			pendingSkipReason,
			transactionHashStr,
		)
	}
}
