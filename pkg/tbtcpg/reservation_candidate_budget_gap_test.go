package tbtcpg_test

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestLocalChain_ValidateReservationAnchorProposal_RejectsNonPendingAction
// pins the LocalChain validator fake's action-state precondition: the fake
// must enforce the same rule as
// WalletProposalValidator.validateReservationAnchorProposal, which requires
// the action at the proposal's RequestNonce to be a Pending Acceptance.
//
// The strict-nonce test (TestLocalChain_ValidateReservationAnchorProposal_StrictNonceGate)
// only asserts that a MISSING action record is rejected, so it never reaches
// the fake's state check. Dropping that check from the fake therefore left
// every existing test green. This test drives the state check directly: a
// proposal whose action record exists but is Settled, TimedOut, Vetoed or
// Superseded must be rejected, and a record whose action type is not
// Acceptance must be rejected too.
func TestLocalChain_ValidateReservationAnchorProposal_RejectsNonPendingAction(t *testing.T) {
	lc := tbtcpg.NewLocalChain()
	proposal, extraInfo, depositKey := strictValidatorFixture(t, lc, testWalletPublicKeyHash)

	// The seeded fixture is a Pending Acceptance action at nonce 2, and it
	// validates; this is the baseline the mutations below break.
	if err := lc.ValidateReservationAnchorProposal(
		testWalletPublicKeyHash,
		proposal,
		extraInfo,
	); err != nil {
		t.Fatalf(
			"the seeded Pending Acceptance action should validate: %v",
			err,
		)
	}

	seeded, err := lc.GetReservationAction(depositKey, 2)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("an action that is not Pending is rejected", func(t *testing.T) {
		for _, state := range []tbtc.ReservationActionState{
			tbtc.ReservationActionStateSettled,
			tbtc.ReservationActionStateTimedOut,
			tbtc.ReservationActionStateVetoed,
			tbtc.ReservationActionStateSuperseded,
			tbtc.ReservationActionStateUnknown,
		} {
			notPending := *seeded
			notPending.State = state
			lc.SetReservationAction(depositKey, 2, &notPending)

			err := lc.ValidateReservationAnchorProposal(
				testWalletPublicKeyHash,
				proposal,
				extraInfo,
			)
			if err == nil {
				t.Fatalf(
					"the fake accepted an action in state %v; Solidity's "+
						"validator requires a Pending Acceptance action",
					state,
				)
			}
			if err.Error() != "acceptance action is not pending" {
				t.Errorf(
					"expected the fake's not-pending rejection, got [%v]",
					err,
				)
			}
		}

		// Restore the seeded record so a later subtest starts from the
		// baseline.
		lc.SetReservationAction(depositKey, 2, seeded)
	})

	t.Run("an action that is not an Acceptance is rejected", func(t *testing.T) {
		for _, actionType := range []tbtc.ReservationActionType{
			tbtc.ReservationActionTypeNone,
			tbtc.ReservationActionTypeRedemption,
			tbtc.ReservationActionTypeReanchor,
			tbtc.ReservationActionTypeDissolution,
		} {
			notAcceptance := *seeded
			notAcceptance.ActionType = actionType
			lc.SetReservationAction(depositKey, 2, &notAcceptance)

			err := lc.ValidateReservationAnchorProposal(
				testWalletPublicKeyHash,
				proposal,
				extraInfo,
			)
			if err == nil {
				t.Fatalf(
					"the fake accepted an action of type %v; Solidity's "+
						"validator requires an Acceptance action",
					actionType,
				)
			}
			if err.Error() != "not a pending acceptance action" {
				t.Errorf(
					"expected the fake's wrong-action-type rejection, got [%v]",
					err,
				)
			}
		}

		lc.SetReservationAction(depositKey, 2, seeded)
	})

	t.Run("the action is read at the proposal's own nonce", func(t *testing.T) {
		// A Pending Acceptance action at nonce 3, but one that authorizes a
		// different wallet. Solidity reads reservationActions(depositKey,
		// proposal.requestNonce) and compares that record's target wallet
		// with the proposal's, so a proposal at nonce 3 must be rejected
		// with the target-wallet mismatch -- not with the "no action
		// record" rejection, which would mean the fake looked the record
		// up at some other nonce.
		otherWallet := [20]byte{0xde, 0xad, 0xbe, 0xef}
		otherTarget := *seeded
		otherTarget.TargetWalletPublicKeyHash = otherWallet
		lc.SetReservationAction(depositKey, 3, &otherTarget)

		proposalAtNonce3 := *proposal
		proposalAtNonce3.RequestNonce = 3

		err := lc.ValidateReservationAnchorProposal(
			testWalletPublicKeyHash,
			&proposalAtNonce3,
			extraInfo,
		)
		if err == nil {
			t.Fatal(
				"the fake accepted a proposal whose nonce authorizes " +
					"another wallet",
			)
		}
		if err.Error() != "wallet does not match the authorized action" {
			t.Errorf(
				"expected the fake's target-wallet rejection at the "+
					"proposal's own nonce, got [%v]",
				err,
			)
		}
	})
}

// revealLookupCountingChain wraps the shared acceptance mock chain and counts the
// deposit-reveal lookups that reached one of the candidates' own acceptance
// request blocks. The reveal lookup is the first Electrum-backed step of the
// candidate pipeline, so the count of such lookups is the number of
// candidates the per-run budget actually admitted.
type revealLookupCountingChain struct {
	*reservationAcceptanceLocalChain

	candidateRequestBlocks map[uint64]struct{}
	revealLookups          map[uint64]int
}

func newRevealLookupCountingChain(
	ralc *reservationAcceptanceLocalChain,
	requestBlocks []uint64,
) *revealLookupCountingChain {
	chain := &revealLookupCountingChain{
		reservationAcceptanceLocalChain: ralc,
		candidateRequestBlocks:          make(map[uint64]struct{}),
		revealLookups:                   make(map[uint64]int),
	}
	for _, block := range requestBlocks {
		chain.candidateRequestBlocks[block] = struct{}{}
	}
	return chain
}

// PastDepositRevealedEvents records the lookups that ended on one of the
// candidates' request blocks and delegates to the embedded mock.
func (c *revealLookupCountingChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	if filter != nil && filter.EndBlock != nil {
		if _, ok := c.candidateRequestBlocks[*filter.EndBlock]; ok {
			c.revealLookups[*filter.EndBlock]++
		}
	}
	return c.reservationAcceptanceLocalChain.PastDepositRevealedEvents(filter)
}

// revealLookupCount returns how many candidates reached the reveal lookup.
func (c *revealLookupCountingChain) revealLookupCount() int {
	return len(c.revealLookups)
}

// TestReservationAcceptanceTask_CandidateBudgetCeiling pins the per-run
// candidate budget (maxReservationAcceptanceCandidatesPerRun = 50): it must
// actually stop the candidate scan.
//
// The existing budget test
// (TestReservationAcceptanceTask_BudgetCountsOnlyPendingAcceptances)
// covers only the "settled generations do not count" half: its 51 settled
// generations are rejected by the pending-acceptance check before they
// reach the budget gate, so the 50-candidate ceiling itself is never
// exercised. Removing the ceiling entirely leaves that test green.
//
// This test seeds 50 qualifying candidates followed by one more that would
// qualify after them. The 50 first candidates are made to fail after the
// budget gate but before they become proposable (they were revealed to
// another vault), so the pass has to walk the whole budgeted set; the 51st
// candidate is fully viable and is only reachable past the ceiling. The task
// must therefore stop after the 50th: the 51st candidate is never examined,
// and no proposal is produced.
func TestReservationAcceptanceTask_CandidateBudgetCeiling(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	otherVault := chain.Address("0xOtherVaultAddress1234567890abcdef1234567890")

	// The candidates are tried in timeout order, so the 51st candidate has
	// the latest timeout and is only reachable past the budget ceiling.
	// Every candidate gets its own request block, so the reveal lookups
	// that reached a candidate can be told apart.
	var requestBlocks []uint64
	for i := range 51 {
		opts := defaultPendingDepositOptions(currentBlock)
		opts.seed = byte(0x60 + i)
		opts.requestBlock = currentBlock - 100 - uint64(i)
		opts.revealBlock = opts.requestBlock - 100
		opts.timeoutAt = uint32(
			time.Now().Add(time.Duration(20+i) * time.Hour).Unix(),
		)
		requestBlocks = append(requestBlocks, opts.requestBlock)

		if i < 50 {
			// A deposit revealed to another vault is not anchorable, so
			// these candidates are examined (they consume the budget) but
			// never become proposable.
			opts.revealVault = &otherVault
		}

		addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)
	}

	chain := newRevealLookupCountingChain(ralc, requestBlocks)

	task := tbtcpg.NewReservationAcceptanceTask(chain, btcChain)
	rawProposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: testWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldExecute || rawProposal != nil {
		t.Fatalf(
			"expected no proposal, got shouldExecute=%v proposal=%+v",
			shouldExecute,
			rawProposal,
		)
	}

	// Exactly the 50 budgeted candidates reached the reveal lookup; the
	// 51st was never examined.
	if chain.revealLookupCount() != 50 {
		t.Errorf(
			"expected the pass to examine exactly the 50 candidates the "+
				"budget allows, got %d",
			chain.revealLookupCount(),
		)
	}

	// The 51st candidate is viable, so a pass that ignored the ceiling
	// would have proposed it.
	if chain.revealLookups[requestBlocks[50]] != 0 {
		t.Errorf(
			"expected the candidate past the budget ceiling to be skipped, "+
				"got %d reveal lookups for it",
			chain.revealLookups[requestBlocks[50]],
		)
	}
}

// TestReservationAcceptanceTask_ProposalErrorSkipsCandidate pins that an
// error raised while building or validating a candidate's proposal must skip
// that candidate, not abort the window.
//
// The existing fee-error test (TestReservationAcceptanceTask_FeeErrorSkipsCandidate)
// covers only the fee-estimate half of the rule: the failing candidate is
// skipped earlier, inside completeReservationAcceptanceCandidate, so the
// proposal-error path is never reached. Making the proposal error abort the
// window therefore left that test green.
//
// This test drives the proposal-error path directly: the first candidate's
// proposal fails the validator, and the task must continue to the next
// candidate and propose it instead of ending the window.
func TestReservationAcceptanceTask_ProposalErrorSkipsCandidate(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	// The first candidate's deposit was never revealed, so the strict
	// validator rejects its proposal. The candidate itself passes every
	// discovery check, so the failure happens on the proposal path, not
	// on the candidate-skipping path.
	broken := defaultPendingDepositOptions(currentBlock)
	broken.seed = 0x30
	broken.timeoutAt = uint32(time.Now().Add(20 * time.Hour).Unix())
	broken.revealedAt = time.Time{}
	addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, broken)

	good := defaultPendingDepositOptions(currentBlock)
	good.seed = 0x31
	good.timeoutAt = uint32(time.Now().Add(23 * time.Hour).Unix())
	viable := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, good)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, viable, 1)

	if ralc.validateCalls != 2 {
		t.Fatalf(
			"expected both candidates to reach validation, got %d calls",
			ralc.validateCalls,
		)
	}
}

// TestReservationAcceptanceTask_ProposalErrorDoesNotEndTheWindow pins the
// same rule from the other side: the proposal error must be logged and
// skipped, and the window's own result must stay a clean no-op rather than
// an error, so the coordination window is not aborted by one bad candidate.
func TestReservationAcceptanceTask_ProposalErrorDoesNotEndTheWindow(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	broken := defaultPendingDepositOptions(currentBlock)
	broken.seed = 0x32
	broken.timeoutAt = uint32(time.Now().Add(20 * time.Hour).Unix())
	broken.revealedAt = time.Time{}
	addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, broken)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	if err != nil {
		t.Fatalf(
			"a candidate's proposal error must not end the window: %v",
			err,
		)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf(
			"expected no proposal, got shouldExecute=%v proposal=%+v",
			shouldExecute,
			proposal,
		)
	}
	if ralc.validateCalls != 1 {
		t.Errorf(
			"expected the single candidate to reach validation, got %d calls",
			ralc.validateCalls,
		)
	}
}

// TestLocalChain_ValidateReservationAnchorProposal_RejectsNonPendingAction
// is kept next to the acceptance task's own proposal-error test so the
// fake's strictness and the task's skip-on-error rule stay together.

// compile-time guard: the mock chain wrapper must satisfy the production
// Chain interface the acceptance task is built on.
var _ tbtcpg.Chain = (*revealLookupCountingChain)(nil)
