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
// ReservationReanchorTask.Run across the multi-round scenarios in this
// file. The block counter starts at block 1000.
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
		ReservationActionTimeout: 86400,
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
		Amount:                    200000,
		TimeoutAt:                 uint32(timeoutAt.Unix()),
	})
}

// TestReservationReanchorTask_UnminedRequest_NextRoundRequestsAgain pins
// what Run does with a request it submitted but did not see mined within
// its wait: the pass ends without a proposal, and because Run keeps no
// memory of the submission, the next round dispatches on chain state
// alone. A reservation still Active on-chain (the request reverted, was
// dropped, or is still unmined) is eligible for a fresh request, which
// is safe because Reservation.sol only accepts a request against an
// Active reservation.
func TestReservationReanchorTask_UnminedRequest_NextRoundRequestsAgain(t *testing.T) {
	tbtcChain, _, blockCounter, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, _ :=
		newInFlightReanchorFixture(t)

	// Round 1: the request is submitted but writes no generation, so the
	// mined-wait cannot find it and the pass ends without a proposal.
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

	// Round 2: the reservation is still Active on-chain, so a fresh
	// request is issued and mined within the wait.
	blockCounter.SetCurrentBlock(1900)

	proposal, ok, err = task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf(
			"round 2: expected a proposal from a fresh request, got "+
				"ok=%v proposal=%v", ok, proposal,
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
		t.Fatalf("round 2: expected a second submission, got %d", got)
	}
}

// TestReservationReanchorTask_UnminedRequest_MinedBeforeNextRound_Resumes
// pins the other half of the chain-state dispatch: a request that was not
// seen mined within round 1's wait but is mined before round 2 leaves the
// reservation ActionPending with a Pending Reanchor generation, so round 2
// must resume that generation at its real nonce instead of requesting
// again.
func TestReservationReanchorTask_UnminedRequest_MinedBeforeNextRound_Resumes(t *testing.T) {
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
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 1 {
		t.Fatalf("round 1: expected exactly 1 submission, got %d", got)
	}

	// The submission is mined between rounds: the chain now holds the
	// new generation (ActionPending, nonce 1, a Pending Reanchor
	// authorizing the target).
	seedResumableReanchorGeneration(
		t, tbtcChain, targetWalletPublicKeyHash, reservationKey, 1,
		time.Now().Add(24*time.Hour),
	)
	blockCounter.SetCurrentBlock(1900)

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
}

// TestReservationReanchorTask_ReservationLeftWallet_NotRequestedAgain
// pins that a reservation whose custody moved away from the source wallet
// between rounds is not requested again: round 1 requests and proposes the
// re-anchor, its proof settles before round 2, and round 2 reads the
// source's now-empty reservation list from chain state and issues nothing.
func TestReservationReanchorTask_ReservationLeftWallet_NotRequestedAgain(t *testing.T) {
	tbtcChain, _, blockCounter, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
		newInFlightReanchorFixture(t)

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("round 1: unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf("round 1: expected a proposal, got ok=%v proposal=%v", ok, proposal)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 1 {
		t.Fatalf("round 1: expected exactly 1 submission, got %d", got)
	}

	// The re-anchor proof settles between rounds: custody moves to the
	// target and the source's reservation list no longer names the key.
	if err := tbtcChain.SettleReservationReanchor(
		reservationKey,
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{TransactionHash: bitcoin.Hash{0x99}},
			Value:    199450,
		},
	); err != nil {
		t.Fatal(err)
	}
	blockCounter.SetCurrentBlock(1900)

	proposal, ok, err = task.Run(&tbtc.CoordinationProposalRequest{
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
	if got := len(tbtcChain.GetReservationReanchorRequestAttempts()); got != 1 {
		t.Fatalf(
			"round 2: expected no new re-anchor request, got %d attempts "+
				"in total",
			got,
		)
	}
	if count, err := tbtcChain.WalletReservationsCount(targetWalletPublicKeyHash); err != nil || count != 1 {
		t.Fatalf("expected the target to custody the reservation, got count %d (err %v)", count, err)
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
