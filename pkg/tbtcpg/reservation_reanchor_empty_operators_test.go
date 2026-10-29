package tbtcpg

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestReservationReanchorTask_EmptyWalletOperators_SendsNoRequest(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, _, _ :=
		newInFlightReanchorFixture(t)

	executingOperator := chain.Address("0x00000000000000000000000000000000000000cd")
	if err := tbtcChain.SetOperatorID(executingOperator, 7); err != nil {
		t.Fatal(err)
	}

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		WalletOperators:     []chain.Address{},
		ExecutingOperator:   executingOperator,
	})
	if err != nil {
		t.Fatalf("expected a benign no-op, got error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"expected no proposal for an empty operator list, got ok=%v proposal=%v",
			ok, proposal,
		)
	}
	if got := tbtcChain.GetReservationReanchorRequestSubmissions(); len(got) != 0 {
		t.Fatalf(
			"expected zero re-anchor request submissions for an empty "+
				"operator list, got %d",
			len(got),
		)
	}
	if got := tbtcChain.GetReservationReanchorRequestAttempts(); len(got) != 0 {
		t.Fatalf(
			"expected zero re-anchor request attempts for an empty "+
				"operator list, got %d",
			len(got),
		)
	}
}
