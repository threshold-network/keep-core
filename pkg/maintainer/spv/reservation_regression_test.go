package spv

// Regression tests for the go-spv-chain hardening of the reservation
// proof loop and stranding watcher, in the package's local-chain style.

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/keep-network/keep-core/pkg/chain"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// confirmationsFailingLocalBitcoinChain is a full localBitcoinChain
// double whose GetTransactionConfirmations fails for one specific
// transaction hash (simulating a per-tick Electrum blip) and delegates
// every other read to the embedded chain. The hash's real body still
// fetches, so only the confirmations read is what breaks.
type confirmationsFailingLocalBitcoinChain struct {
	*localBitcoinChain
	failingHash *bitcoin.Hash
}

func (c *confirmationsFailingLocalBitcoinChain) GetTransactionConfirmations(
	_ context.Context,
	transactionHash bitcoin.Hash,
) (uint, error) {
	if c.failingHash != nil && transactionHash == *c.failingHash {
		return 0, errors.New("simulated electrum blip on this tx")
	}
	return c.localBitcoinChain.GetTransactionConfirmations(
		context.Background(),
		transactionHash,
	)
}

// reanchorFixtureAnchorTxHash returns the timed-out reanchor fixture's
// on-chain anchor transaction hash, so sibling-generation seeding can
// snapshot the same source anchor the fixture's TimedOut generation used.
func reanchorFixtureAnchorTxHash(
	t *testing.T,
	fixture *timedOutReanchorFixture,
) bitcoin.Hash {
	t.Helper()
	for _, event := range fixture.spvChain.reservationReanchorRequestedEvents {
		_ = event
	}
	// The fixture broadcasts its anchor transaction and stores the
	// reanchor transaction (which spends the anchor) as the coinbase
	// reference; the anchor outpoint is the reanchor transaction's input.
	// Recover it through the btc chain's transaction set: the reanchor
	// transaction's input is the anchor outpoint.
	for _, transaction := range fixture.btcChain.localBitcoinChain.transactions {
		if len(transaction.Inputs) == 1 && transaction.Inputs[0].Outpoint != nil {
			return transaction.Inputs[0].Outpoint.TransactionHash
		}
	}
	t.Fatal("could not recover the fixture's anchor transaction hash")
	return (bitcoin.Hash{})
}

// perKeyGetReservationFailingChain fails GetReservation for one specific
// reservation key and delegates every other key to the embedded chain. It
// backs the per-key retry-then-skip in
// checkReservationStrandingForWallet (withStrandingRetry around
// GetReservation): only the failing key is dropped, unlike a transient
// ethereum chain read error that aborts the whole-wallet scan.
type perKeyGetReservationFailingChain struct {
	*localChain
	failingKey *big.Int
}

func (c *perKeyGetReservationFailingChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	if c.failingKey != nil && reservationKey.Cmp(c.failingKey) == 0 {
		return nil, errors.New("simulated transient RPC failure on this key")
	}
	return c.localChain.GetReservation(reservationKey)
}

// TestEarlierCandidateConfirmationsErrorDoesNotHideLaterReplacement pins
// that when the first matching candidate's proof-info read fails
// transiently, the pass still examines the later, already-confirmed
// replacement and proves it.
func TestEarlierCandidateConfirmationsErrorDoesNotHideLaterReplacement(
	t *testing.T,
) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	healthy := newLocalBitcoinChain()
	if err := populateBlockHeaders(
		healthy,
		proofStart,
		proofStart+19,
		func(uint) *big.Int { return diff(32) },
	); err != nil {
		t.Fatal(err)
	}
	spvChain := newLocalChain()
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	// A confirmed replacement that would be provable on a healthy chain.
	healthyReplacement := &bitcoin.Transaction{Locktime: 9}
	if err := healthy.addTransactionConfirmations(healthyReplacement.Hash(), 20); err != nil {
		t.Fatal(err)
	}

	// A still-pending earlier candidate whose confirmations read is what
	// breaks this pass.
	pendingEarlier := &bitcoin.Transaction{Locktime: 8}
	if err := healthy.addTransactionConfirmations(pendingEarlier.Hash(), 2); err != nil {
		t.Fatal(err)
	}

	// Control: the healthy chain proves the later confirmed replacement
	// even though the unconfirmed candidate is encountered first.
	submissions := 0
	_, err := proveReservationTransaction(
		[]*bitcoin.Transaction{pendingEarlier, healthyReplacement},
		alwaysMatchReservationTransaction,
		healthy,
		spvChain,
		spvChain,
		DefaultMaxProofHeaders,
		newProofInfoCache(),
		nil,
		func(hash bitcoin.Hash, requiredConfirmations uint) error {
			submissions++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("healthy control unexpectedly failed: %v", err)
	}
	if submissions != 1 {
		t.Fatalf("healthy control: expected the confirmed replacement to be proved, got %d submissions", submissions)
	}

	// The blip chain: the earlier candidate's confirmations read fails.
	pendingEarlierHash := pendingEarlier.Hash()
	blip := &confirmationsFailingLocalBitcoinChain{
		localBitcoinChain: healthy,
		failingHash:       &pendingEarlierHash,
	}
	submissions = 0
	outcome, err := proveReservationTransaction(
		[]*bitcoin.Transaction{pendingEarlier, healthyReplacement},
		alwaysMatchReservationTransaction,
		blip,
		spvChain,
		spvChain,
		DefaultMaxProofHeaders,
		newProofInfoCache(),
		nil,
		func(hash bitcoin.Hash, requiredConfirmations uint) error {
			submissions++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected the failed read to be skipped, got error: %v", err)
	}
	if outcome != reservationProofSubmitted {
		t.Fatalf("expected the confirmed replacement to be submitted, got outcome %d", outcome)
	}
	if submissions != 1 {
		t.Fatalf(
			"expected the confirmed later replacement to be proved after the earlier read failed, got %d submissions",
			submissions,
		)
	}
}

// TestFailedSubmissionDoesNotBlockSiblingGeneration pins that when two
// settleable generations of the same reservation - a Pending one and a
// TimedOut one - are in flight and the Pending generation's proof
// submission fails, the failed submit must not claim the reservation key:
// the TimedOut generation is still proved in the same pass.
func TestFailedSubmissionDoesNotBlockSiblingGeneration(t *testing.T) {
	fixture := newTimedOutAcceptanceFixture(t, encodingP2WPKH)

	// A second, Pending generation of the same reservation, re-requested at
	// nonce 2 with the same wallet: the same anchor transaction is a valid
	// candidate for it too (both generations spend the deposit UTXO to the
	// wallet).
	pendingKey := fixture.reservationKey
	fixture.spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      pendingKey,
		RequestNonce:        2,
		WalletPublicKeyHash: fixture.walletPKH,
		DepositAmount:       150000,
		BlockNumber:         501,
	})
	fixture.spvChain.setReservationAction(
		pendingKey,
		2,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: fixture.walletPKH,
		},
	)
	// The reservation record must not exist (Unknown) for the TimedOut
	// generation to stay settleable, which the fixture already guarantees
	// by not seeding a reservation for the key.

	// The submit hook fails on the FIRST call (the Pending generation is
	// sorted first) and succeeds on later calls: it records which nonces
	// actually reach a submission attempt.
	submittedNonces := make([]uint64, 0)
	fixture.spvChain.submitReservationAcceptanceProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		submittedNonces = append(submittedNonces, requestNonce)
		if len(submittedNonces) == 1 {
			return errors.New("simulated gas provider drop on first submit")
		}
		return nil
	}

	err := proveReservationAcceptanceActions(
		fixture.state,
		fixture.config,
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		newProofInfoCache(),
		nil,
	)
	if err == nil {
		t.Fatal("expected the failed first submission to surface as a per-item error")
	} else if _, isItemErrs := err.(spvItemErrors); !isItemErrs {
		t.Fatalf("expected the per-item aggregate (spvItemErrors), got %T: %v", err, err)
	}

	if len(submittedNonces) != 2 || submittedNonces[0] != 2 || submittedNonces[1] != fixture.requestNonce {
		t.Fatalf(
			"expected the failed Pending generation (nonce 2) followed by the TimedOut sibling (nonce %d), got %v",
			fixture.requestNonce,
			submittedNonces,
		)
	}
}

// TestFailedReanchorSubmissionDoesNotBlockSiblingGeneration pins the
// re-anchor counterpart: a failed submission of one generation's proof
// must not claim the reservation key, so a later, independently
// settleable sibling generation is still proved in the same pass.
func TestFailedReanchorSubmissionDoesNotBlockSiblingGeneration(t *testing.T) {
	fixture := newTimedOutReanchorFixture(t, encodingP2WPKH)

	// A second, Pending reanchor generation of the same reservation,
	// re-requested at nonce 3 against the same source wallet: the same
	// reanchor transaction is a valid candidate for it too.
	pendingKey := fixture.reservationKey
	fixture.spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            pendingKey,
		RequestNonce:              3,
		SourceWalletPublicKeyHash: fixture.sourceWalletPKH,
		TargetWalletPublicKeyHash: fixture.sourceWalletPKH,
		TxMaxFee:                  20000,
		BlockNumber:               501,
	})
	fixture.spvChain.setReservationAction(
		pendingKey,
		3,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: fixture.sourceWalletPKH,
			// The Pending generation's source anchor must match the
			// reservation's current anchor for it to be settleable.
			SourceAnchorUtxoHash: reanchorSourceAnchorHash(
				&tbtc.Reservation{
					AnchorUtxo: &bitcoin.UnspentTransactionOutput{
						Outpoint: &bitcoin.TransactionOutpoint{
							TransactionHash: reanchorFixtureAnchorTxHash(t, fixture),
							OutputIndex:     1,
						},
						Value: 600000,
					},
				},
			),
		},
	)

	submittedNonces := make([]uint64, 0)
	fixture.spvChain.submitReservationReanchorProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		_ *big.Int,
		requestNonce uint64,
	) error {
		submittedNonces = append(submittedNonces, requestNonce)
		if len(submittedNonces) == 1 {
			return errors.New("simulated gas provider drop on first submit")
		}
		return nil
	}

	err := proveReservationReanchorActions(
		fixture.state,
		fixture.config,
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		newProofInfoCache(),
		nil,
	)
	if err == nil {
		t.Fatal("expected the failed first re-anchor submission to surface as a per-item error")
	} else if _, isItemErrs := err.(spvItemErrors); !isItemErrs {
		t.Fatalf("expected the per-item aggregate (spvItemErrors), got %T: %v", err, err)
	}

	if len(submittedNonces) != 2 || submittedNonces[0] != 3 || submittedNonces[1] != fixture.requestNonce {
		t.Fatalf(
			"expected the failed Pending generation (nonce 3) followed by the TimedOut sibling (nonce %d), got %v",
			fixture.requestNonce,
			submittedNonces,
		)
	}
}

// TestSingleReservationReadFailureFlagsWalletForRecheck pins that a
// reservation whose read fails after the retries is skipped without
// starving the wallet's other reservations, and the check reports that
// the wallet needs another pass; the action-timeout watcher's drain then
// keeps the wallet queued instead of dropping the unread key.
func TestSingleReservationReadFailureFlagsWalletForRecheck(t *testing.T) {
	wallet := walletPKHAt(0x22)
	failingKey := reservationKey(0x2201)
	healthyKey := reservationKey(0x2202)

	newChain := func() *perKeyGetReservationFailingChain {
		failing := &perKeyGetReservationFailingChain{
			localChain: newLocalChain(),
			failingKey: failingKey,
		}
		failing.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
		failing.setWalletReservations(wallet, []*big.Int{failingKey, healthyKey})
		failing.setReservation(failingKey, &tbtc.Reservation{State: tbtc.ReservationStateActive})
		failing.setReservation(healthyKey, &tbtc.Reservation{State: tbtc.ReservationStateActive})
		return failing
	}

	t.Run("the check reports a recheck and notifies the healthy key", func(t *testing.T) {
		failing := newChain()
		watcher := newReservationStrandingWatcher(failing)
		watcher.retryDelay = 0

		recheck, err := watcher.checkReservationStrandingForWallet(wallet)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !recheck {
			t.Fatal("expected the failed read to flag the wallet for a later pass")
		}

		notified := failing.getSubmittedReservationStrandedKeys()
		if len(notified) != 1 || notified[0].Cmp(healthyKey) != 0 {
			t.Fatalf("expected exactly the healthy key to be notified, got %v", notified)
		}
	})

	t.Run("the drain keeps the wallet queued", func(t *testing.T) {
		failing := newChain()
		strandingWatcher := newReservationStrandingWatcher(failing)
		strandingWatcher.retryDelay = 0
		ratw := NewReservationActionTimeoutWatcher(failing, 0, common.Address{}, strandingWatcher, 0, nil)
		ratw.strandingRecheckWallets[wallet] = struct{}{}

		ratw.drainStrandingRechecks()

		if _, queued := ratw.strandingRecheckWallets[wallet]; !queued {
			t.Fatal("expected the wallet with an unread reservation to stay queued for the next drain")
		}
	})
}

// reanchorProofScript is the P2WPKH output script of the fixture's source
// wallet, for transactions that must show up in that wallet's history.
func (f *timedOutReanchorFixture) reanchorProofScript() (bitcoin.Script, error) {
	return authorizedWalletScript(f.sourceWalletPKH, encodingP2WPKH)
}

// filterRecordingChain records the StartBlock of every
// PastNewWalletRegisteredEvents filter it sees.
type filterRecordingChain struct {
	*localChain
	pastNewWalletRegisteredStartBlocks []uint64
}

func (c *filterRecordingChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	c.pastNewWalletRegisteredStartBlocks = append(
		c.pastNewWalletRegisteredStartBlocks,
		filter.StartBlock,
	)
	return c.localChain.PastNewWalletRegisteredEvents(filter)
}

// actionReadCountingChain wraps a Chain and counts GetReservationAction
// calls, since localChain itself keeps no such counter. It backs two
// claims - that a still-Pending acceptance generation and a tracked
// action-timeout entry are each re-read once per pass/tick with no age
// bound - which this wrapper measures directly.
type actionReadCountingChain struct {
	*localChain
	getReservationActionCalls int
}

func (c *actionReadCountingChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	c.getReservationActionCalls++
	return c.localChain.GetReservationAction(reservationKey, requestNonce)
}

// TestStaleDepositPollTickReadsAreBoundedPerTick pins that a poll
// tick re-checks at most reservationStaleDepositMaxChecksPerTick
// past-deadline deposits, resuming after the last one on the next tick,
// and a deposit this watcher just notified is not re-read until the
// renotify interval elapses.
func TestStaleDepositPollTickReadsAreBoundedPerTick(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(200000)
	spvChain.setBlockCounter(blockCounter)

	const (
		deadline = uint32(1_000_000)
		count    = reservationStaleDepositMaxChecksPerTick + 10
	)
	// now is past the deadline and past the maximum operator stagger
	// offset, so the first-check stagger gate holds no deposit back.
	now := deadline + 601
	// The stagger gate is judged on the chain head's block time,
	// so the head sits at the same point.
	spvChain.setLatestBlockTimestamp(time.Unix(int64(now), 0))

	vault := chain.Address("")
	spvChain.setReservationParameters(&tbtc.ReservationParameters{})

	for i := 1; i <= count; i++ {
		fundingTxHash := bitcoin.Hash{byte(0xF3), byte(i)}
		spvChain.pastDepositRevealedEvents = append(
			spvChain.pastDepositRevealedEvents,
			&tbtc.DepositRevealedEvent{
				Vault:               &vault,
				Amount:              100_000,
				RefundLocktime:      locktimeForDeadline(deadline),
				BlockNumber:         150_000,
				FundingTxHash:       fundingTxHash,
				FundingOutputIndex:  0,
				WalletPublicKeyHash: walletPKH(),
			},
		)
		depositKey := spvChain.BuildDepositKey(fundingTxHash, 0)
		spvChain.setReservedDeposit(depositKey, walletPKH(), true)
		spvChain.setReservation(depositKey, &tbtc.Reservation{
			State:        tbtc.ReservationStateActive,
			RequestNonce: 1,
		})
		spvChain.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
			State:      tbtc.ReservationActionStateTimedOut,
			ActionType: tbtc.ReservationActionTypeAcceptance,
		})
	}

	counting := &staleReadCountingChain{Chain: spvChain}
	watcher := NewReservationStaleDepositWatcher(
		counting,
		common.HexToAddress("0x00000000000000000000000000000000000000aa"),
		tbtc.ReservationsActivationBlock(ethereum.Developer),
	)

	tracked, ok := watcher.pollTick(now)
	if !ok {
		t.Fatal("expected the tick's scans to succeed")
	}
	if tracked != count {
		t.Fatalf("expected all %d deposits to be tracked, got %d", count, tracked)
	}
	if reads := counting.reservedDepositWalletCalls; reads != reservationStaleDepositMaxChecksPerTick {
		t.Fatalf(
			"expected the first tick to read %d deposits (the per-tick cap), got %d",
			reservationStaleDepositMaxChecksPerTick,
			reads,
		)
	}

	// The second tick covers the deposits the first one left over; the
	// ones notified a second ago are not read again.
	watcher.pollTick(now + 1)
	if reads := counting.reservedDepositWalletCalls; reads != count {
		t.Fatalf(
			"expected the second tick to read only the %d deposits left over (%d reads in total), got %d",
			count-reservationStaleDepositMaxChecksPerTick,
			count,
			reads,
		)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != count {
		t.Fatalf("expected %d stale-deposit notifications, got %d", count, len(calls))
	}
}

// TestAbandonedPendingAcceptanceGenerationIsEvicted pins that a
// Pending acceptance generation is tracked until the chain head passes
// TimeoutAt + 4 * (TimeoutAt - RequestedAt) of its action, then evicted,
// so a generation whose timeout notice never lands is not re-read for
// the life of the process.
func TestAbandonedPendingAcceptanceGenerationIsEvicted(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	key := big.NewInt(0xF1600)
	walletPKH := [20]byte{0x0F, 0x16}

	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      key,
		RequestNonce:        1,
		WalletPublicKeyHash: walletPKH,
		DepositAmount:       150000,
		BlockNumber:         500,
	})
	// The action spans 1000s, so the abandonment bound is 2000 + 4000.
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:       tbtc.ReservationActionStatePending,
		ActionType:  tbtc.ReservationActionTypeAcceptance,
		RequestedAt: 1000,
		TimeoutAt:   2000,
	})

	state := newReservationProofScanState()
	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}

	counting := &actionReadCountingChain{localChain: spvChain}
	pass := func(headTime int64) {
		t.Helper()
		spvChain.setLatestBlockTimestamp(time.Unix(headTime, 0))
		if err := proveReservationAcceptanceActions(
			state,
			config,
			counting,
			counting,
			btcChain,
			newProofInfoCache(),
			nil,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	eventKey := reservationEventKey(key, 1)

	for i := 0; i < 3; i++ {
		pass(6000)
	}
	if _, tracked := state.pendingAcceptanceEvents[eventKey]; !tracked {
		t.Fatal("expected the Pending generation to stay tracked at the bound")
	}
	if reads := counting.getReservationActionCalls; reads != 3 {
		t.Fatalf("expected one read per pass inside the bound (3 reads), got %d", reads)
	}

	pass(6001)
	if _, tracked := state.pendingAcceptanceEvents[eventKey]; tracked {
		t.Fatal("expected the abandoned Pending generation to be evicted past the bound")
	}
	pass(6002)
	if reads := counting.getReservationActionCalls; reads != 4 {
		t.Fatalf("expected no reads after the eviction (4 reads in total), got %d", reads)
	}
}

// TestStuckNotifiedPendingActionIsEvicted pins that a tracked
// action whose timeout notice was submitted but which stays Pending
// past TimeoutAt + 4 * (TimeoutAt - RequestedAt) of chain time is
// evicted instead of being re-read on every tick for the life of the
// process. An action that was never notified stays tracked so its
// first notice is attempted.
func TestStuckNotifiedPendingActionIsEvicted(t *testing.T) {
	newFixture := func(notifiedAt uint32) (*ReservationActionTimeoutWatcher, *actionReadCountingChain, *localChain, string) {
		spvChain := newLocalChain()
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(1000)
		spvChain.setBlockCounter(blockCounter)

		wallet := walletPKHAt(0x4B)
		key := reservationKey(0x4B00)
		seededReservation(
			t,
			spvChain,
			key,
			wallet,
			[]*tbtc.ReservationAction{
				{
					ActionType: tbtc.ReservationActionTypeReanchor,
					State:      tbtc.ReservationActionStatePending,
					TimeoutAt:  5_000,
				},
			},
			1,
		)

		counting := &actionReadCountingChain{localChain: spvChain}
		watcher := NewReservationActionTimeoutWatcher(counting, 0, common.Address{}, nil, 0, nil)
		// Before the deadline: nothing is notified, so the only question
		// is whether the entry is kept.
		watcher.nowFn = func() uint32 { return 100 }
		eventKey := actionEventKey(key, 1)
		watcher.pendingActions[eventKey] = &pendingAction{
			reservationKey: key,
			requestNonce:   1,
			notifiedAt:     notifiedAt,
		}
		return watcher, counting, spvChain, eventKey
	}

	t.Run("a notified entry is kept inside the bound and evicted past it", func(t *testing.T) {
		watcher, counting, spvChain, eventKey := newFixture(1)

		// The action spans 5000s from 0, so the bound is 5000 + 20000.
		spvChain.setLatestBlockTimestamp(time.Unix(25_000, 0))
		for tick := 0; tick < 3; tick++ {
			if err := watcher.pollPendingActions(); err != nil {
				t.Fatalf("tick %d unexpected error: %v", tick, err)
			}
		}
		if _, tracked := watcher.pendingActions[eventKey]; !tracked {
			t.Fatal("expected the entry to stay tracked inside the bound")
		}
		if reads := counting.getReservationActionCalls; reads != 3 {
			t.Fatalf("expected one read per tick inside the bound, got %d", reads)
		}

		spvChain.setLatestBlockTimestamp(time.Unix(25_001, 0))
		if err := watcher.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, tracked := watcher.pendingActions[eventKey]; tracked {
			t.Fatal("expected the stuck notified entry to be evicted past the bound")
		}
		if err := watcher.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reads := counting.getReservationActionCalls; reads != 4 {
			t.Fatalf("expected no reads after the eviction (4 in total), got %d", reads)
		}
	})

	t.Run("an entry that was never notified is kept", func(t *testing.T) {
		watcher, _, spvChain, eventKey := newFixture(0)

		spvChain.setLatestBlockTimestamp(time.Unix(1_000_000, 0))
		if err := watcher.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, tracked := watcher.pendingActions[eventKey]; !tracked {
			t.Fatal("expected a never-notified entry to stay tracked")
		}
	})
}

// TestLateReanchorProofSurvivesNewerWalletTransactions pins that a
// TimedOut re-anchor generation whose re-anchor transaction is no
// longer among the wallet's newest config.TransactionLimit transactions
// is still proved, because the wallet's history is fetched back to the
// anchor transaction the generation spends.
func TestLateReanchorProofSurvivesNewerWalletTransactions(t *testing.T) {
	const fillerCount = 25
	const defaultLimit = 20

	fixture := newTimedOutReanchorFixture(t, encodingP2WPKH)

	// Fill the source wallet's confirmed history after the re-anchor
	// transaction, pushing it out of the newest-defaultLimit window.
	fillerScript, err := fixture.reanchorProofScript()
	if err != nil {
		t.Fatalf("cannot build filler output script: %v", err)
	}
	for i := 0; i < fillerCount; i++ {
		filler := &bitcoin.Transaction{
			Locktime: uint32(0xE000 + i),
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           1000,
				PublicKeyScript: fillerScript,
			}},
		}
		if err := fixture.btcChain.BroadcastTransaction(filler); err != nil {
			t.Fatalf("cannot broadcast filler transaction: %v", err)
		}
		if err := fixture.btcChain.addTransactionConfirmations(filler.Hash(), 20); err != nil {
			t.Fatalf("cannot confirm filler transaction: %v", err)
		}
	}

	if err := proveReservationReanchorActions(
		fixture.state,
		Config{
			TransactionLimit: defaultLimit,
			MaxProofHeaders:  DefaultMaxProofHeaders,
			EthereumNetwork:  ethereum.Developer,
		},
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		newProofInfoCache(),
		nil,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fixture.submissions != 1 {
		t.Fatalf(
			"expected the re-anchor proof beyond the newest-%d window to be submitted, got %d submissions",
			defaultLimit,
			fixture.submissions,
		)
	}
}

func TestStaleDepositCursorPreservedAcrossTicks(t *testing.T) {
	inner := newLocalChain()
	spvChain := &staleReadCountingChain{Chain: inner}

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	const totalDeposits = 75
	endBlock := currentBlock - reservationEventScanConfirmationBlocks
	for i := range totalDeposits {
		fundingTxHash := bitcoin.Hash{byte(i + 1)}
		fundingOutputIndex := uint32(0)
		depositKey := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)
		if err := inner.addPastDepositRevealedEvent(
			&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &endBlock},
			&tbtc.DepositRevealedEvent{
				FundingTxHash:      fundingTxHash,
				FundingOutputIndex: fundingOutputIndex,
				Vault:              &vault,
				RefundLocktime:     locktimeForDeadline(100),
			},
		); err != nil {
			t.Fatal(err)
		}
		wallet := walletPKHAt(byte(0x80 + i))
		inner.setReservedDeposit(depositKey, wallet, true)
		inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
		inner.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
		inner.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
			State:     tbtc.ReservationActionStatePending,
			TimeoutAt: 10_000,
		})
	}

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	inner.setLatestBlockTimestamp(time.Unix(5_000, 0))

	trackedCount, ok := watcher.pollTick(5_000)
	if !ok {
		t.Fatal("pollTick failed")
	}
	if trackedCount != totalDeposits {
		t.Fatalf("expected %d tracked deposits, got %d", totalDeposits, trackedCount)
	}
	if spvChain.reservedDepositWalletCalls != 50 {
		t.Fatalf("expected 50 wallet reads on tick 1, got %d", spvChain.reservedDepositWalletCalls)
	}
	if watcher.checkCursor == "" {
		t.Fatalf("expected non-empty checkCursor after tick 1 hitting budget limit")
	}

	spvChain.reservedDepositWalletCalls = 0
	cursorBeforeTick2 := watcher.checkCursor
	trackedCount, ok = watcher.pollTick(5_000)
	if !ok {
		t.Fatal("pollTick failed")
	}
	if spvChain.reservedDepositWalletCalls != 50 {
		t.Fatalf("expected 50 wallet reads on tick 2, got %d", spvChain.reservedDepositWalletCalls)
	}
	if watcher.checkCursor == cursorBeforeTick2 {
		t.Fatalf("checkCursor did not advance across ticks; still %s", watcher.checkCursor)
	}
}

type headTimeCountingChain struct {
	Chain
	getLatestBlockTimestampCalls int
}

func (c *headTimeCountingChain) GetLatestBlockTimestamp() (time.Time, error) {
	c.getLatestBlockTimestampCalls++
	return c.Chain.GetLatestBlockTimestamp()
}

func TestStaleDepositWatcherSingleHeadTimePerTick(t *testing.T) {
	inner := newLocalChain()
	headCountingChain := &headTimeCountingChain{Chain: inner}

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	const depositCount = 30
	endBlock := currentBlock - reservationEventScanConfirmationBlocks
	for i := range depositCount {
		fundingTxHash := bitcoin.Hash{byte(i + 1)}
		fundingOutputIndex := uint32(0)
		depositKey := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)
		if err := inner.addPastDepositRevealedEvent(
			&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &endBlock},
			&tbtc.DepositRevealedEvent{
				FundingTxHash:      fundingTxHash,
				FundingOutputIndex: fundingOutputIndex,
				Vault:              &vault,
				RefundLocktime:     locktimeForDeadline(100),
			},
		); err != nil {
			t.Fatal(err)
		}
		wallet := walletPKHAt(byte(0x80 + i))
		inner.setReservedDeposit(depositKey, wallet, true)
		inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
		inner.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
		inner.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
			State:     tbtc.ReservationActionStatePending,
			TimeoutAt: 10_000,
		})
	}

	watcher := NewReservationStaleDepositWatcher(headCountingChain, common.Address{}, 0)
	inner.setLatestBlockTimestamp(time.Unix(5_000, 0))

	headCountingChain.getLatestBlockTimestampCalls = 0
	_, ok := watcher.pollTick(5_000)
	if !ok {
		t.Fatal("pollTick failed")
	}

	if headCountingChain.getLatestBlockTimestampCalls > 1 {
		t.Fatalf(
			"expected at most 1 GetLatestBlockTimestamp call per pollTick, got %d",
			headCountingChain.getLatestBlockTimestampCalls,
		)
	}
}

func TestStaleDepositPollTickBudgetUsesChainHeadTime(t *testing.T) {
	inner := newLocalChain()
	spvChain := &staleReadCountingChain{Chain: inner}

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	const totalDeposits = 60
	endBlock := currentBlock - reservationEventScanConfirmationBlocks
	for i := range totalDeposits {
		fundingTxHash := bitcoin.Hash{byte(i + 1)}
		fundingOutputIndex := uint32(0)
		depositKey := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)
		if err := inner.addPastDepositRevealedEvent(
			&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &endBlock},
			&tbtc.DepositRevealedEvent{
				FundingTxHash:      fundingTxHash,
				FundingOutputIndex: fundingOutputIndex,
				Vault:              &vault,
				RefundLocktime:     locktimeForDeadline(500),
			},
		); err != nil {
			t.Fatal(err)
		}
		wallet := walletPKHAt(byte(0x80 + i))
		inner.setReservedDeposit(depositKey, wallet, true)
		inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
		inner.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
		inner.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
			State:     tbtc.ReservationActionStatePending,
			TimeoutAt: 10_000,
		})
	}

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)

	inner.setLatestBlockTimestamp(time.Unix(600, 0))

	_, ok := watcher.pollTick(400)
	if !ok {
		t.Fatal("pollTick failed")
	}

	if spvChain.reservedDepositWalletCalls != 50 {
		t.Fatalf(
			"expected budget to cap checks at 50 based on chain head time, got %d reads",
			spvChain.reservedDepositWalletCalls,
		)
	}
}
