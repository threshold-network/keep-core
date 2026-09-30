package tbtcpg

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// newInFlightReanchorFixture wires a fresh LocalChain / LocalBitcoinChain
// pair with a single Active reservation, its anchor UTXO, and a
// registered Live target wallet, ready to drive
// ReservationReanchorTask.Run across the receipt-resolution scenarios in
// this file. The block counter starts at block 1000.
func newInFlightReanchorFixture(t *testing.T) (
	tbtcChain *LocalChain,
	btcChain *LocalBitcoinChain,
	blockCounter *MockBlockCounter,
	task *ReservationReanchorTask,
	sourceWalletPublicKeyHash [20]byte,
	targetWalletPublicKeyHash [20]byte,
	reservationKey *big.Int,
) {
	t.Helper()

	sourceWalletPublicKeyHash = [20]byte{
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	}
	targetWalletPublicKeyHash = [20]byte{
		5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	}

	tbtcChain = NewLocalChain()
	btcChain = NewLocalBitcoinChain()

	blockCounter = NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	if err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: targetWalletPublicKeyHash},
	); err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(1)

	tbtcChain.SetMovingFundsParameters(
		1000000, 1000000, 0, 0, nil, 0, 0, 0, 0, nil, 0,
	)
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
		ReservationMinAmount:     1000,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	reservationKey = big.NewInt(7001)
	anchorTxHashHex := "7777777777777777777777777777777777777777777777777777777777777777"[:64]
	anchorTxHash, err := bitcoin.NewHashFromString(anchorTxHashHex, bitcoin.ReversedByteOrder)
	if err != nil {
		t.Fatal(err)
	}
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 200000, PublicKeyScript: sourceWalletScript},
		},
	})
	tbtcChain.SetReservation(reservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			Value: 200000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{reservationKey})

	task = NewReservationReanchorTask(tbtcChain, btcChain)

	return
}

// seedResumableReanchorGeneration moves the fixture's reservation to
// ActionPending at the given nonce, with a Pending Reanchor action
// authorizing targetWalletPublicKeyHash and expiring at timeoutAt --
// modeling a generation an earlier Run (or another party) already
// authorized, ready for the resume path to pick up.
func seedResumableReanchorGeneration(
	t *testing.T,
	tbtcChain *LocalChain,
	targetWalletPublicKeyHash [20]byte,
	reservationKey *big.Int,
	nonce uint64,
	timeoutAt time.Time,
) {
	t.Helper()

	current, err := tbtcChain.GetReservation(reservationKey)
	if err != nil {
		t.Fatalf("cannot read reservation: %v", err)
	}
	updated := *current
	updated.State = tbtc.ReservationStateActionPending
	updated.RequestNonce = nonce
	tbtcChain.SetReservation(reservationKey, &updated)

	tbtcChain.SetReservationAction(reservationKey, nonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		TxMaxFee:                  100000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		TimeoutAt:                 uint32(timeoutAt.Unix()),
	})
}

// TestReservationReanchorTask_InFlight_PendingWithinBound_SkipsWithoutNewRequest
// pins the "still pending, within the drop bound" branch of
// resolveReanchorRequestInFlight: a submission that has not yet
// confirmed must suppress a second RequestReservationReanchor call for
// the same reservation, instead of re-requesting every round.
func TestReservationReanchorTask_InFlight_PendingWithinBound_SkipsWithoutNewRequest(t *testing.T) {
	tbtcChain, _, blockCounter, task, sourceWalletPublicKeyHash, _, _ :=
		newInFlightReanchorFixture(t)

	// Round 1: the request is submitted but stays unconfirmed (Pending
	// receipt, no generation written), so the same-round fast path
	// cannot find the mined generation and this reservation is skipped
	// for the round -- but the submission is now tracked in-flight.
	tbtcChain.SetNextReservationReanchorRequestPending()

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 1: unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"round 1: expected no proposal while the request is "+
				"unconfirmed, got ok=%v proposal=%v", ok, proposal,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 1 {
		t.Fatalf("round 1: expected exactly 1 submission, got %d", got)
	}

	// Round 2: the block has advanced by less than the drop bound (6)
	// and the receipt is still Pending, so the in-flight entry must
	// resolve to "still unconfirmed" and this round must not submit a
	// second request.
	blockCounter.SetCurrentBlock(1003)

	proposal, ok, err = task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"round 2: expected no proposal while the pending request "+
				"remains within its window, got ok=%v proposal=%v", ok, proposal,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 1 {
		t.Fatalf(
			"round 2: expected the in-flight pending request to "+
				"suppress a second submission, got %d submissions", got,
		)
	}
}

// TestReservationReanchorTask_InFlight_Reverted_AllowsNewRequest pins the
// Reverted branch of resolveReanchorRequestInFlight: once a submission's
// receipt resolves as reverted, the reservation (left Active on-chain,
// since the revert wrote no generation) must be eligible for a fresh
// request on the very next round.
func TestReservationReanchorTask_InFlight_Reverted_AllowsNewRequest(t *testing.T) {
	tbtcChain, _, blockCounter, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, _ :=
		newInFlightReanchorFixture(t)

	// Round 1: the request reverts on-chain; no generation is written,
	// so the fast path cannot find it and the reservation is skipped
	// this round, tracked in-flight by its reverted receipt.
	tbtcChain.SetRevertNextReservationReanchorRequest()

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 1: unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"round 1: expected no proposal from a reverted request, "+
				"got ok=%v proposal=%v", ok, proposal,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 1 {
		t.Fatalf("round 1: expected exactly 1 submission, got %d", got)
	}

	// Round 2: the receipt resolves as Reverted, so a fresh request
	// must be allowed and mined via the same-round fast path.
	blockCounter.SetCurrentBlock(1001)

	proposal, ok, err = task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf(
			"round 2: expected a proposal from a freshly allowed "+
				"request, got ok=%v proposal=%v", ok, proposal,
		)
	}
	reanchorProposal, ok := proposal.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("round 2: unexpected proposal type: %T", proposal)
	}
	if reanchorProposal.RequestNonce != 1 {
		t.Fatalf(
			"round 2: expected the fresh request's nonce (1), got [%d]",
			reanchorProposal.RequestNonce,
		)
	}
	if reanchorProposal.TargetWalletPublicKeyHash != targetWalletPublicKeyHash {
		t.Fatalf(
			"round 2: unexpected target wallet [%x]",
			reanchorProposal.TargetWalletPublicKeyHash,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 2 {
		t.Fatalf(
			"round 2: expected the reverted request to allow a second "+
				"submission, got %d submissions", got,
		)
	}
}

// TestReservationReanchorTask_InFlight_DroppedPastBound_AllowsNewRequest
// pins the "unobserved past the drop bound" branch of
// resolveReanchorRequestInFlight: a submission whose receipt is no
// longer found at all, once the block count has advanced more than
// reservationReanchorRequestWaitBlocks (6) past the submission block,
// must be treated as dropped and a fresh request must be allowed.
func TestReservationReanchorTask_InFlight_DroppedPastBound_AllowsNewRequest(t *testing.T) {
	tbtcChain, _, blockCounter, task, sourceWalletPublicKeyHash, _, _ :=
		newInFlightReanchorFixture(t)

	tbtcChain.SetNextReservationReanchorRequestPending()

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 1: unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"round 1: expected no proposal while the request is "+
				"unconfirmed, got ok=%v proposal=%v", ok, proposal,
		)
	}
	submissions := tbtcChain.GetReservationReanchorRequestSubmissions()
	if len(submissions) != 1 {
		t.Fatalf("round 1: expected exactly 1 submission, got %d", len(submissions))
	}

	// The submission is no longer observable at all (e.g. it fell out
	// of the mempool) and the block has advanced past the drop bound
	// (6): the in-flight entry must be treated as dropped and a fresh
	// request must be allowed.
	tbtcChain.SetReservationReanchorRequestReceipt(
		submissions[0].TxHash,
		tbtc.ReservationReanchorRequestReceiptNotFound,
	)
	blockCounter.SetCurrentBlock(1007)

	proposal, ok, err = task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf(
			"round 2: expected a proposal from a freshly allowed "+
				"request once the submission is treated as dropped, "+
				"got ok=%v proposal=%v", ok, proposal,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 2 {
		t.Fatalf(
			"round 2: expected the dropped request to allow a second "+
				"submission, got %d submissions", got,
		)
	}
}

// TestReservationReanchorTask_InFlight_Mined_ResumesWithoutNewRequest
// pins the Mined branch of resolveReanchorRequestInFlight: once a
// submission's receipt resolves as mined, the next round must resume
// from the chain-written Pending Reanchor generation (validated with its
// real request nonce) instead of issuing a new request.
func TestReservationReanchorTask_InFlight_Mined_ResumesWithoutNewRequest(t *testing.T) {
	tbtcChain, _, blockCounter, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
		newInFlightReanchorFixture(t)

	tbtcChain.SetNextReservationReanchorRequestPending()

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 1: unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"round 1: expected no proposal while the request is "+
				"unconfirmed, got ok=%v proposal=%v", ok, proposal,
		)
	}
	submissions := tbtcChain.GetReservationReanchorRequestSubmissions()
	if len(submissions) != 1 {
		t.Fatalf("round 1: expected exactly 1 submission, got %d", len(submissions))
	}

	// Simulate the submission being mined between rounds: the chain now
	// holds the new generation (ActionPending, nonce 1, a Pending
	// Reanchor authorizing the target) and the receipt reports Mined.
	seedResumableReanchorGeneration(
		t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 1,
		time.Now().Add(24*time.Hour),
	)
	tbtcChain.SetReservationReanchorRequestReceipt(
		submissions[0].TxHash,
		tbtc.ReservationReanchorRequestReceiptMined,
	)
	blockCounter.SetCurrentBlock(1001)

	proposal, ok, err = task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf(
			"round 2: expected the mined request to resume from chain "+
				"state, got ok=%v proposal=%v", ok, proposal,
		)
	}
	reanchorProposal, ok := proposal.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("round 2: unexpected proposal type: %T", proposal)
	}
	if reanchorProposal.RequestNonce != 1 {
		t.Fatalf(
			"round 2: expected the resumed proposal to carry the "+
				"generation's real nonce (1), got [%d]",
			reanchorProposal.RequestNonce,
		)
	}
	if reanchorProposal.TargetWalletPublicKeyHash != targetWalletPublicKeyHash {
		t.Fatalf(
			"round 2: unexpected target wallet [%x]",
			reanchorProposal.TargetWalletPublicKeyHash,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 1 {
		t.Fatalf(
			"round 2: expected the mined request to resume without a "+
				"new submission, got %d submissions", got,
		)
	}
	if got := len(task.inFlightReanchorRequests); got != 0 {
		t.Fatalf(
			"round 2: expected the mined in-flight entry to be "+
				"forgotten once resolved, got %d entries", got,
		)
	}
}

// TestReservationReanchorTask_InFlight_ReservationLeftWallet_ForgottenOnNextRun
// pins the in-flight reconciliation: a submission still unconfirmed when
// its reservation's custody has since moved away from the source wallet
// (the wallet's reservation list no longer names it) must be forgotten on
// the next Run rather than tracked for the task's lifetime, while a
// concurrently unconfirmed submission for a different wallet must survive
// the same pass untouched.
func TestReservationReanchorTask_InFlight_ReservationLeftWallet_ForgottenOnNextRun(t *testing.T) {
	tbtcChain, btcChain, _, task, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	// A second MovingFunds wallet with its own active reservation, so
	// its own in-flight entry can be checked for survival.
	secondSourceWalletPublicKeyHash := [20]byte{
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	}
	secondReservationKey := big.NewInt(7002)
	tbtcChain.SetWallet(
		secondSourceWalletPublicKeyHash,
		&tbtc.WalletChainData{State: tbtc.StateMovingFunds},
	)
	secondAnchorTxHash, err := bitcoin.NewHashFromString(
		"8888888888888888888888888888888888888888888888888888888888888888"[:64],
		bitcoin.ReversedByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	secondSourceScript, err := bitcoin.PayToWitnessPublicKeyHash(
		secondSourceWalletPublicKeyHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(secondAnchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 200000, PublicKeyScript: secondSourceScript},
		},
	})
	tbtcChain.SetReservation(secondReservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: secondSourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: secondAnchorTxHash,
				OutputIndex:     1,
			},
			Value: 200000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(
		secondSourceWalletPublicKeyHash,
		[]*big.Int{secondReservationKey},
	)

	// Round 1 for both wallets: each submits a request that stays
	// unconfirmed (Pending receipt, no generation written), so both are
	// tracked in-flight by their transaction hashes.
	tbtcChain.SetNextReservationReanchorRequestPending()
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	}); err != nil {
		t.Fatalf("round 1 (first wallet): unexpected error: %v", err)
	}
	if _, ok := task.getReanchorRequestInFlight(reservationKey); !ok {
		t.Fatalf(
			"round 1: expected the first wallet's unconfirmed request to be " +
				"tracked in-flight",
		)
	}

	tbtcChain.SetNextReservationReanchorRequestPending()
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: secondSourceWalletPublicKeyHash,
	}); err != nil {
		t.Fatalf("round 1 (second wallet): unexpected error: %v", err)
	}
	if _, ok := task.getReanchorRequestInFlight(secondReservationKey); !ok {
		t.Fatalf(
			"round 1: expected the second wallet's unconfirmed request to be " +
				"tracked in-flight",
		)
	}

	// The first wallet's reservation moves custody elsewhere between
	// rounds: it no longer appears in the wallet's reservation list.
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, nil)

	// Round 2 for the first wallet: the wallet's list is now empty, so
	// the departure must be reconciled -- the first wallet's in-flight
	// entry is forgotten, while the second wallet's entry (a different
	// source wallet, untouched by this pass) survives.
	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"round 2: expected no proposal for a drained wallet, got "+
				"ok=%v proposal=%v",
			ok,
			proposal,
		)
	}
	if _, found := task.getReanchorRequestInFlight(reservationKey); found {
		t.Error(
			"round 2: expected the departed reservation's in-flight " +
				"entry to be forgotten, but it was still tracked",
		)
	}
	if _, found := task.getReanchorRequestInFlight(secondReservationKey); !found {
		t.Error(
			"round 2: expected another wallet's in-flight entry to " +
				"survive the reconciliation pass",
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 2 {
		t.Fatalf(
			"round 2: expected no new re-anchor request submissions, "+
				"got %d",
			got,
		)
	}
}

// TestReservationReanchorTask_ResumeMarginGate pins the resume path's
// timeout safety margin gate: a Pending Reanchor generation whose
// TimeoutAt sits inside REQUEST_TIMEOUT_SAFETY_MARGIN (2 hours, mirrored
// here as reservationRequestTimeoutSafetyMarginSeconds) must be skipped
// without a proposal and without ever reaching on-chain validation, while
// one whose TimeoutAt sits comfortably outside the margin must be
// proposed and validated normally.
func TestReservationReanchorTask_ResumeMarginGate(t *testing.T) {
	t.Run("inside the safety margin is skipped", func(t *testing.T) {
		tbtcChain, _, _, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
			newInFlightReanchorFixture(t)

		// 7100s < the 7200s margin: now + 7200 >= TimeoutAt always
		// holds here, well clear of clock-drift flakiness.
		seedResumableReanchorGeneration(
			t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 4,
			time.Now().Add(7100*time.Second),
		)

		proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || proposal != nil {
			t.Fatalf(
				"expected no proposal for a generation inside the "+
					"safety margin, got ok=%v proposal=%v", ok, proposal,
			)
		}
		if got := tbtcChain.GetReservationReanchorValidationCallCount(); got != 0 {
			t.Fatalf(
				"expected the margin gate to skip this generation "+
					"before reaching on-chain validation, got %d "+
					"validation calls", got,
			)
		}
		if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 0 {
			t.Fatalf("expected no re-anchor request submissions, got %d", got)
		}
	})

	t.Run("just outside the safety margin is proposed", func(t *testing.T) {
		tbtcChain, _, _, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
			newInFlightReanchorFixture(t)

		// 7300s > the 7200s margin: now + 7200 >= TimeoutAt never
		// holds here, well clear of clock-drift flakiness.
		seedResumableReanchorGeneration(
			t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 4,
			time.Now().Add(7300*time.Second),
		)

		proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: sourceWalletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || proposal == nil {
			t.Fatalf(
				"expected a proposal for a generation outside the "+
					"safety margin, got ok=%v proposal=%v", ok, proposal,
			)
		}
		reanchorProposal, ok := proposal.(*tbtc.ReservationReanchorProposal)
		if !ok {
			t.Fatalf("unexpected proposal type: %T", proposal)
		}
		if reanchorProposal.RequestNonce != 4 {
			t.Fatalf(
				"expected the resumed proposal to carry the "+
					"generation's real nonce (4), got [%d]",
				reanchorProposal.RequestNonce,
			)
		}
		if got := tbtcChain.GetReservationReanchorValidationCallCount(); got != 1 {
			t.Fatalf("expected exactly 1 validation call, got %d", got)
		}
		if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 0 {
			t.Fatalf(
				"expected the resume path to issue no new request, "+
					"got %d submissions", got,
			)
		}
	})
}
