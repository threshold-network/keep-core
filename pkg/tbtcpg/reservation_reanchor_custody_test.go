package tbtcpg

import (
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationReanchorTask_SkipsYoungResumeTarget verifies the resume path
// age gate: a pending re-anchor generation targeting a wallet that has not
// yet aged out of its creation period is skipped, and no proposal is issued.
//
// G1-F2: the leader must not re-anchor to a target that has not aged out
// of its creation period. The tolerance is 0 (strict), matching the leader's
// own gate.
func TestReservationReanchorTask_SkipsYoungResumeTarget(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
		newInFlightReanchorFixture(t)

	// Set wallet creation period to 1 hour, so we can create a young target.
	tbtcChain.SetWalletParameters(
		3600, // creationPeriod: 1 hour
		0,    // creationMinBtcBalance
		0,    // creationMaxBtcBalance
		0,    // closureMinBtcBalance
		0,    // maxAge
		0,    // maxBtcTransfer
		0,    // closingPeriod
	)

	wall := time.Now()
	period := time.Duration(3600) * time.Second

	// Young subtest: target created 1 second before it would age.
	// createdAt + period = wall - 3599s + 3600s = wall + 1s > headTime (wall) → not aged.
	t.Run("young target: skipped, no proposal", func(t *testing.T) {
		youngCreatedAt := wall.Add(-period + time.Second)
		tbtcChain.SetLatestBlockTimestamp(wall)
		tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: youngCreatedAt,
		})

		timeoutAt := wall.Add(24 * time.Hour) // far in the future, margin gate passes
		seedResumableReanchorGeneration(
			t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 4, timeoutAt,
		)

		proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if shouldExecute || proposal != nil {
			t.Fatalf("expected no proposal, got shouldExecute=%v proposal=%v", shouldExecute, proposal)
		}

		// Old code (pre-fix): no age gate in resume path → ProposeReservationReanchor
		// is called → validation succeeds → proposal returned. New code skips at
		// the age gate. Asserting no proposal fails on old code.
	})

	// Aged subtest: target created 1 hour + 1 second ago → aged.
	t.Run("aged target: proposed", func(t *testing.T) {
		agedCreatedAt := wall.Add(-period - time.Second)
		tbtcChain.SetLatestBlockTimestamp(wall)
		tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: agedCreatedAt,
		})

		timeoutAt := wall.Add(24 * time.Hour)
		seedResumableReanchorGeneration(
			t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 5, timeoutAt,
		)

		proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if !shouldExecute || proposal == nil {
			t.Fatalf("expected a proposal, got shouldExecute=%v proposal=%v", shouldExecute, proposal)
		}
	})
}

// TestReanchorTargetSearch_SkipsYoungCandidate verifies that pick excludes
// candidates that have not yet aged out of their creation period.
//
// G1-F12: a candidate wallet must have aged out of its creation period
// before it can be selected as a re-anchor destination.
func TestReanchorTargetSearch_SkipsYoungCandidate(t *testing.T) {
	wall := time.Now()
	period := uint32(3600) // 1 hour creation period

	lc := NewLocalChain()
	lc.SetWalletParameters(
		period, // creationPeriod
		0,      // creationMinBtcBalance
		0,      // creationMaxBtcBalance
		0,      // closureMinBtcBalance
		0,      // maxAge
		0,      // maxBtcTransfer
		0,      // closingPeriod
	)
	lc.SetLatestBlockTimestamp(wall)

	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	lc.SetBlockCounter(blockCounter)

	// Old wallet: aged (created far in the past).
	oldWallet := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	// Young wallet: created just before the age cutoff.
	// createdAt + period = wall - 3599s + 3600s = wall + 1s > headTime (wall) → not aged.
	youngWallet := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}

	// Register old first (block 100), then young (block 200, newest).
	lc.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: oldWallet},
	)
	lc.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: youngWallet},
	)

	lc.SetWallet(oldWallet, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: wall.Add(-time.Duration(period)*time.Second - time.Hour),
	})
	lc.SetWallet(youngWallet, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: wall.Add(-time.Duration(period-1) * time.Second), // young
	})

	lc.SetLiveWalletsCount(2)

	task := NewReservationReanchorTask(lc, NewLocalBitcoinChain())
	headTime, _ := lc.GetLatestBlockTimestamp()
	walletParams, _ := lc.GetWalletParameters()

	// Both wallets registered; candidates order = [young (newest), old].
	// pick should skip young (not aged), pick old.
	t.Run("young candidate skipped, aged picked", func(t *testing.T) {
		search := task.newReanchorTargetSearch(
			logger, [20]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
			5, 100000000, headTime, walletParams.CreationPeriod,
		)
		got, err := search.find(100000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != oldWallet {
			t.Fatalf("expected old wallet [0x%x], got [0x%x]", oldWallet, got)
		}
	})

	// Only the young wallet registered → search finds no aged candidate.
	t.Run("no aged candidate: errNoLiveTargetWallet with waiting flag", func(t *testing.T) {
		lcOnlyYoung := NewLocalChain()
		lcOnlyYoung.SetWalletParameters(
			period, // creationPeriod
			0,      // creationMinBtcBalance
			0,      // creationMaxBtcBalance
			0,      // closureMinBtcBalance
			0,      // maxAge
			0,      // maxBtcTransfer
			0,      // closingPeriod
		)
		lcOnlyYoung.SetLatestBlockTimestamp(wall)
		lcOnlyYoung.SetBlockCounter(blockCounter)
		lcOnlyYoung.AddPastNewWalletRegisteredEvent(
			&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
			&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: youngWallet},
		)
		lcOnlyYoung.SetWallet(youngWallet, &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: wall.Add(-time.Duration(period-1) * time.Second),
		})
		lcOnlyYoung.SetLiveWalletsCount(1)

		taskYoung := NewReservationReanchorTask(lcOnlyYoung, NewLocalBitcoinChain())
		search := taskYoung.newReanchorTargetSearch(
			logger, [20]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
			5, 100000000, headTime, walletParams.CreationPeriod,
		)
		_, err := search.find(100000)
		if !errors.Is(err, errNoLiveTargetWallet) {
			t.Fatalf("expected errNoLiveTargetWallet, got: %v", err)
		}
		if !search.waitingOnAgedTarget {
			t.Fatalf("expected waitingOnAgedTarget to be true")
		}
	})
}

// failingTimestampChain wraps LocalChain and makes GetLatestBlockTimestamp
// fail, modelling an Ethereum node that is unreachable or returning bad data.
type failingTimestampChain struct {
	*LocalChain
}

func (f *failingTimestampChain) GetLatestBlockTimestamp() (time.Time, error) {
	return time.Time{}, errors.New("chain head timestamp unavailable")
}

// TestReanchorCustodyGate_ChainHeadReadFailureFailsClosed verifies that when
// the chain head timestamp read fails, the leader skips the pass entirely
// rather than proceeding with a potentially incorrect gate evaluation.
func TestReanchorCustodyGate_ChainHeadReadFailureFailsClosed(t *testing.T) {
	tbtcChain, _, _, _, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	failingChain := &failingTimestampChain{LocalChain: tbtcChain}
	failingTask := NewReservationReanchorTask(failingChain, NewLocalBitcoinChain())

	t.Run("resume path: fails closed on head read error", func(t *testing.T) {
		wall := time.Now()
		timeoutAt := wall.Add(24 * time.Hour)
		seedResumableReanchorGeneration(
			t, tbtcChain, [20]byte{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
			reservationKey, 4, timeoutAt,
		)

		proposal, shouldExecute, err := failingTask.Run(
			&tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: sourceWalletPublicKeyHash,
			},
		)
		// On read error, Run must return an error (fail closed, retry next pass).
		// Old code (pre-fix): no GetLatestBlockTimestamp call → no error → may propose.
		if err == nil {
			t.Fatalf("expected an error on chain head read failure, got nil")
		}
		if shouldExecute || proposal != nil {
			t.Fatalf("expected no proposal on head read failure, got shouldExecute=%v proposal=%v",
				shouldExecute, proposal)
		}
	})

	t.Run("active path: fails closed on head read error", func(t *testing.T) {
		// The Active reservation is set up by newInFlightReanchorFixture.
		proposal, shouldExecute, err := failingTask.Run(
			&tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: sourceWalletPublicKeyHash,
			},
		)
		if err == nil {
			t.Fatalf("expected an error on chain head read failure, got nil")
		}
		if shouldExecute || proposal != nil {
			t.Fatalf("expected no proposal on head read failure, got shouldExecute=%v proposal=%v",
				shouldExecute, proposal)
		}
	})
}

// TestReservationReanchorTask_CooldownGateUsesChainTime verifies that the
// cooldown gate uses the chain head timestamp instead of the host wall clock.
// A host clock running ahead of the chain can no longer cause early action
// that the on-chain validator would revert. Dually, a host clock behind the
// chain can no longer cause late action that the validator would accept.
//
// G1-L28/G1-S1.
func TestReservationReanchorTask_CooldownGateUsesChainTime(t *testing.T) {
	_, _, _, task, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	wall := time.Now()

	t.Run("fast host clock: no early action (old code attempts, chain rejects)", func(t *testing.T) {
		// Chain head is 2 hours behind the host wall clock.
		// A cooldown set to expire 1 hour after the head would be past on
		// the host clock but not yet passed on the chain.
		chainHead := wall.Add(-2 * time.Hour)
		tbtcChain, ok := task.chain.(*LocalChain)
		if !ok {
			t.Fatalf("expected *LocalChain, got %T", task.chain)
		}
		tbtcChain.SetLatestBlockTimestamp(chainHead)

		// Cooldown until = chainHead + 3600s (1h after chain head).
		// Host wall clock (wall = chainHead + 2h) > cooldown → old gate passes.
		// Chain time (chainHead) < cooldown → on-chain check would reject.
		reservation, _ := tbtcChain.GetReservation(reservationKey)
		reservation.ReanchorCooldownUntil = uint32(chainHead.Add(time.Hour).Unix())
		tbtcChain.SetReservation(reservationKey, reservation)

		proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		// New code: chainHead < cooldownUntil → skip → no proposal.
		if shouldExecute || proposal != nil {
			t.Fatalf("expected no proposal with fast host clock, got shouldExecute=%v proposal=%v",
				shouldExecute, proposal)
		}

		// Old code (pre-fix): wall clock > cooldownUntil → gate passes →
		// RequestReservationReanchor is called → fake's cooldown check
		// (chain time chainHead < cooldownUntil) rejects → attempts = 1,
		// no proposal. New code attempts = 0. Asserting attempts == 0
		// fails on old code.
		if len(tbtcChain.GetReservationReanchorRequestAttempts()) != 0 {
			t.Fatalf("expected no request attempts, got %d",
				len(tbtcChain.GetReservationReanchorRequestAttempts()))
		}
	})

	t.Run("slow host clock: no late action (old code skips, chain accepts)", func(t *testing.T) {
		// Chain head is 2 hours ahead of host wall clock.
		// A cooldown that has expired on the chain (chain past it) but not
		// yet on the host should be actioned by the new code, not skipped.
		chainHead := wall.Add(2 * time.Hour)
		tbtcChain, ok := task.chain.(*LocalChain)
		if !ok {
			t.Fatalf("expected *LocalChain, got %T", task.chain)
		}
		tbtcChain.SetLatestBlockTimestamp(chainHead)

		// Cooldown until = chainHead - 1h (expired 1 hour ago on chain).
		// Host wall clock (wall = chainHead - 2h) < cooldown → old gate skips.
		// Chain time (chainHead) > cooldown → on-chain check accepts.
		reservation, _ := tbtcChain.GetReservation(reservationKey)
		reservation.ReanchorCooldownUntil = uint32(chainHead.Add(-time.Hour).Unix())
		tbtcChain.SetReservation(reservationKey, reservation)

		proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		// Both old and new code should propose (cooldown expired on chain).
		if !shouldExecute || proposal == nil {
			t.Fatalf("expected a proposal, got shouldExecute=%v proposal=%v",
				shouldExecute, proposal)
		}
	})
}

// TestReservationReanchorTask_ChainTimeReadFailureSkipsPass verifies that
// a failure to read the chain head timestamp causes the entire pass to fail
// closed, giving the coordinator a chance to retry.
func TestReservationReanchorTask_ChainTimeReadFailureSkipsPass(t *testing.T) {
	tbtcChain, _, _, _, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	failingChain := &failingTimestampChain{LocalChain: tbtcChain}
	failingTask := NewReservationReanchorTask(failingChain, NewLocalBitcoinChain())

	// Seed a timed-out generation so the Active path is taken.
	wall := time.Now()
	timeoutAt := wall.Add(100 * time.Second)
	seedResumableReanchorGeneration(
		t, tbtcChain,
		[20]byte{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		reservationKey, 4, timeoutAt,
	)

	proposal, shouldExecute, err := failingTask.Run(
		&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		},
	)
	if err == nil {
		t.Fatalf("expected an error on chain head read failure, got nil")
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected no proposal, got shouldExecute=%v proposal=%v",
			shouldExecute, proposal)
	}
}

// TestReservationReanchorTask_WaitingOnAgedTargetGauge verifies the
// operator-visible signal for the liveness cost of the custody gate: the
// gauge is 1 while the only target is younger than the creation period and
// back to 0 once that target has aged.
func TestReservationReanchorTask_WaitingOnAgedTargetGauge(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
		newInFlightReanchorFixture(t)

	const creationPeriod = uint32(3600)
	tbtcChain.SetWalletParameters(creationPeriod, 0, 0, 0, 0, 0, 0)

	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	wall := time.Now()
	tbtcChain.SetLatestBlockTimestamp(wall)
	seedResumableReanchorGeneration(
		t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 4,
		wall.Add(24*time.Hour),
	)

	run := func() (bool, error) {
		_, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		return shouldExecute, err
	}

	// Young target: waiting.
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: wall.Add(-time.Duration(creationPeriod-1) * time.Second),
	})
	if shouldExecute, err := run(); err != nil || shouldExecute {
		t.Fatalf("expected no proposal and no error, got %v, %v", shouldExecute, err)
	}
	if got := recorder.calls["reanchor_waiting_on_aged_target"]; got != 1 {
		t.Fatalf("expected waiting gauge 1 for a young target, got %v", got)
	}

	// Aged target: proposal, gauge cleared.
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: wall.Add(-time.Duration(creationPeriod) * time.Second),
	})
	if shouldExecute, err := run(); err != nil || !shouldExecute {
		t.Fatalf("expected a proposal, got %v, %v", shouldExecute, err)
	}
	if got := recorder.calls["reanchor_waiting_on_aged_target"]; got != 0 {
		t.Fatalf("expected waiting gauge 0 for an aged target, got %v", got)
	}
}
