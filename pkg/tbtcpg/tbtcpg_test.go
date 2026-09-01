package tbtcpg

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestProposalGenerator_Generate(t *testing.T) {
	walletPublicKeyHash := [20]byte{1, 2, 3}

	tests := map[string]struct {
		tasks            []ProposalTask
		actionsChecklist []tbtc.WalletActionType
		expectedProposal tbtc.CoordinationProposal
		expectedErr      error
	}{
		"first task generates a proposal": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionRedemption,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultProposal,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultProposal,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: &mockCoordinationProposal{tbtc.ActionRedemption},
		},
		"subsequent task generates a proposal": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionRedemption,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultEmpty,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultProposal,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: &mockCoordinationProposal{tbtc.ActionDepositSweep},
		},
		"first task returns error but second succeeds": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionRedemption,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultError,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultProposal,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: &mockCoordinationProposal{tbtc.ActionDepositSweep},
			expectedErr:      nil,
		},
		"all tasks return error": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionRedemption,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultError,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultError,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: nil,
			expectedErr:      fmt.Errorf("all proposal tasks failed: [task [Redemption]: [proposal task error] task [DepositSweep]: [proposal task error]]"),
		},
		"some tasks return error but others complete without result": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionRedemption,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultError,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultEmpty,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: &tbtc.NoopProposal{},
			expectedErr:      nil,
		},
		"first task is unsupported": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultProposal,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: &mockCoordinationProposal{tbtc.ActionDepositSweep},
		},
		"all tasks complete without result": {
			tasks: []ProposalTask{
				&mockProposalTask{
					action: tbtc.ActionRedemption,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultEmpty,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionDepositSweep,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultEmpty,
					},
				},
				&mockProposalTask{
					action: tbtc.ActionHeartbeat,
					results: map[[20]byte]mockProposalTaskResult{
						walletPublicKeyHash: resultProposal,
					},
				},
			},
			actionsChecklist: []tbtc.WalletActionType{
				tbtc.ActionRedemption,
				tbtc.ActionDepositSweep,
			},
			expectedProposal: &tbtc.NoopProposal{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			generator := &ProposalGenerator{
				tasks: test.tasks,
			}

			proposal, err := generator.Generate(
				&tbtc.CoordinationProposalRequest{
					WalletPublicKeyHash: walletPublicKeyHash,
					WalletOperators:     nil,
					ActionsChecklist:    test.actionsChecklist,
				},
			)

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v",
					test.expectedErr,
					err,
				)
			}

			if !reflect.DeepEqual(test.expectedProposal, proposal) {
				t.Errorf(
					"unexpected proposal\nexpected: %v\nactual:   %v",
					test.expectedProposal,
					proposal,
				)
			}
		})
	}
}

// TestNewProposalGenerator_ReservationsEnabled verifies the constructor's
// reservationsEnabled gate: when true, the reservation acceptance and
// re-anchor tasks must be wired into the generator's task list; when false,
// they must be entirely absent so the coordination loop never attempts
// them. Presence/absence is observed indirectly through Generate(), since
// pg.tasks is unexported: a checklist made up solely of reservation action
// types is either dispatched to a real task (which fails deterministically
// against the unconfigured chain double, proving the task was found) or
// falls through as unsupported to a nil-error no-op proposal (proving no
// task claims that action type).
func TestNewProposalGenerator_ReservationsEnabled(t *testing.T) {
	walletPublicKeyHash := [20]byte{1, 2, 3}

	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
		ActionsChecklist: []tbtc.WalletActionType{
			tbtc.ActionReservationAnchor,
			tbtc.ActionReservationReanchor,
		},
	}

	t.Run("enabled: reservation tasks are wired in", func(t *testing.T) {
		generator := NewProposalGenerator(
			NewLocalChain(),
			NewLocalBitcoinChain(),
			true,
		)

		for _, action := range []tbtc.WalletActionType{
			tbtc.ActionReservationAnchor,
			tbtc.ActionReservationReanchor,
		} {
			_, err := generator.Generate(&tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: walletPublicKeyHash,
				ActionsChecklist:    []tbtc.WalletActionType{action},
			})
			if err == nil {
				t.Errorf("expected error for action %v, got nil", action)
			}
		}
	})

	t.Run("disabled: reservation tasks are absent", func(t *testing.T) {
		generator := NewProposalGenerator(
			NewLocalChain(),
			NewLocalBitcoinChain(),
			false,
		)

		proposal, err := generator.Generate(request)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if !reflect.DeepEqual(&tbtc.NoopProposal{}, proposal) {
			t.Fatalf(
				"expected a no-op proposal since no task should claim "+
					"either reservation action type, got [%+v]",
				proposal,
			)
		}
	})
}

type mockProposalTaskResult uint8

const (
	resultProposal mockProposalTaskResult = iota
	resultEmpty
	resultError
)

type mockProposalTask struct {
	action  tbtc.WalletActionType
	results map[[20]byte]mockProposalTaskResult
}

func (mpt *mockProposalTask) Run(
	request *tbtc.CoordinationProposalRequest,
) (
	tbtc.CoordinationProposal,
	bool,
	error,
) {
	result, ok := mpt.results[request.WalletPublicKeyHash]
	if !ok {
		panic("unexpected wallet public key hash")
	}

	switch result {
	case resultProposal:
		return &mockCoordinationProposal{mpt.action}, true, nil
	case resultEmpty:
		return nil, false, nil
	case resultError:
		return nil, false, fmt.Errorf("proposal task error")
	default:
		panic("unexpected result")
	}
}

func (mpt *mockProposalTask) ActionType() tbtc.WalletActionType {
	return mpt.action
}

type mockCoordinationProposal struct {
	action tbtc.WalletActionType
}

func (mcp *mockCoordinationProposal) ActionType() tbtc.WalletActionType {
	return mcp.action
}

func (mcp *mockCoordinationProposal) ValidityBlocks() uint64 {
	panic("unsupported")
}

func (mcp *mockCoordinationProposal) Marshal() ([]byte, error) {
	panic("unsupported")
}

func (mcp *mockCoordinationProposal) Unmarshal(bytes []byte) error {
	panic("unsupported")
}
