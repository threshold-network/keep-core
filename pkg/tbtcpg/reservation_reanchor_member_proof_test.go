package tbtcpg

// Tests for the source-wallet member proof carried by a fresh reservation
// re-anchor request: the request must carry the source wallet signing
// group's member IDs and this operator's 1-based position, built the same
// way as the moving funds commitment (GetWalletMembersInfo), and a caller
// that is not a member of the source wallet must not send a request at all.

import (
	"errors"
	"testing"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationReanchorTask_ReanchorRequestCarriesSourceWalletMemberProof
// pins the exact proof a fresh re-anchor request carries for a 3-operator
// source wallet whose executing operator is NOT the first member: the
// submission must record all three member IDs in wallet order and the
// operator's 1-based position (3), catching off-by-one index mistakes.
func TestReservationReanchorTask_ReanchorRequestCarriesSourceWalletMemberProof(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, _, _ :=
		newInFlightReanchorFixture(t)

	op1 := chain.Address("0x00000000000000000000000000000000000000aa")
	op2 := chain.Address("0x00000000000000000000000000000000000000bb")
	op3 := chain.Address("0x00000000000000000000000000000000000000cc")
	for operator, id := range map[chain.Address]chain.OperatorID{
		op1: 11,
		op2: 22,
		op3: 33,
	} {
		if err := tbtcChain.SetOperatorID(operator, id); err != nil {
			t.Fatal(err)
		}
	}

	// The executing operator is the third signing-group member.
	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		WalletOperators:     []chain.Address{op1, op2, op3},
		ExecutingOperator:   op3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf("expected a proposal, got ok=%v proposal=%v", ok, proposal)
	}

	submissions := tbtcChain.GetReservationReanchorRequestSubmissions()
	if len(submissions) != 1 {
		t.Fatalf("expected exactly 1 re-anchor request submission, got %d", len(submissions))
	}
	submission := submissions[0]
	if got := submission.WalletMembersIDs; len(got) != 3 || got[0] != 11 || got[1] != 22 || got[2] != 33 {
		t.Fatalf(
			"expected the request to carry the source wallet's member IDs "+
				"[11 22 33], got %v",
			got,
		)
	}
	if submission.WalletMemberIndex != 3 {
		t.Fatalf(
			"expected the executing operator's 1-based position 3, got %d",
			submission.WalletMemberIndex,
		)
	}
}

// TestReservationReanchorTask_NonMemberOperator_SendsNoRequest pins that an
// operator who is not in the source wallet's signing group sends no re-anchor
// request: the fresh path of ProposeReservationReanchor fails with
// errReanchorNotSourceWalletMember and Run neither records a submission nor
// attempts a request for this pass.
func TestReservationReanchorTask_NonMemberOperator_SendsNoRequest(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
		newInFlightReanchorFixture(t)

	// The request carries a one-member signing group that does not include
	// the executing operator: GetWalletMembersInfo returns
	// ErrNoExecutingOperator, so Run leaves the proof unset.
	otherOperator := chain.Address("0x00000000000000000000000000000000000000ee")
	if err := tbtcChain.SetOperatorID(otherOperator, 44); err != nil {
		t.Fatal(err)
	}
	nonMemberOperator := chain.Address("0x00000000000000000000000000000000000000ff")
	if err := tbtcChain.SetOperatorID(nonMemberOperator, 55); err != nil {
		t.Fatal(err)
	}

	_, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		WalletOperators:     []chain.Address{otherOperator},
		ExecutingOperator:   nonMemberOperator,
	})
	if err != nil {
		t.Fatalf("expected a benign no-op, got error: %v", err)
	}
	if ok {
		t.Fatalf("expected no proposal for a non-member operator, got ok=true")
	}
	if got := tbtcChain.GetReservationReanchorRequestSubmissions(); len(got) != 0 {
		t.Fatalf(
			"expected zero re-anchor request submissions for a non-member "+
				"operator, got %d",
			len(got),
		)
	}
	if got := tbtcChain.GetReservationReanchorRequestAttempts(); len(got) != 0 {
		t.Fatalf(
			"expected zero re-anchor request attempts for a non-member "+
				"operator, got %d",
			len(got),
		)
	}

	// The fresh path itself must fail with the member-proof sentinel when
	// called with an unset proof.
	if _, perr := task.ProposeReservationReanchor(
		log.Logger("keep-tbtcpg-test"),
		sourceWalletPublicKeyHash,
		reservationKey,
		targetWalletPublicKeyHash,
		0,
		nil,
		0,
	); !errors.Is(perr, errReanchorNotSourceWalletMember) {
		t.Fatalf(
			"expected errReanchorNotSourceWalletMember for an unset proof, got %v",
			perr,
		)
	}
}
