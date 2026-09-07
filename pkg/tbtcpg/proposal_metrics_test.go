package tbtcpg

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// The node cannot import tbtcpg. Pin its exact structural setter signature.
var _ interface {
	SetProposalMetricsRecorder(interface {
		IncrementCounter(string, float64)
	})
} = (*ProposalGenerator)(nil)

type proposalMetrics map[string]float64

func (m proposalMetrics) IncrementCounter(name string, value float64) {
	m[name] += value
}

func TestProposalGenerationMetrics(t *testing.T) {
	for _, test := range []struct {
		name       string
		redemption mockProposalTaskResult
		fallback   mockProposalTaskResult
		attempts   float64
		failures   float64
		generated  float64
		checklist  []tbtc.WalletActionType
	}{
		{"generated", resultProposal, resultEmpty, 1, 0, 1, nil},
		{"no pending requests", resultEmpty, resultEmpty, 1, 0, 0, nil},
		{"failure followed by successful fallback", resultError, resultProposal, 1, 1, 0, nil},
		{"failure followed by no-op fallback", resultError, resultEmpty, 1, 1, 0, nil},
		{"all tasks fail", resultError, resultError, 1, 1, 0, nil},
		{"redemption not scheduled", resultError, resultProposal, 0, 0, 0, []tbtc.WalletActionType{tbtc.ActionDepositSweep}},
		{"earlier task succeeds", resultError, resultProposal, 0, 0, 0, []tbtc.WalletActionType{tbtc.ActionDepositSweep, tbtc.ActionRedemption}},
	} {
		t.Run(test.name, func(t *testing.T) {
			walletPKH := [20]byte{1}
			generator := &ProposalGenerator{tasks: []ProposalTask{
				&mockProposalTask{action: tbtc.ActionRedemption, results: map[[20]byte]mockProposalTaskResult{walletPKH: test.redemption}},
				&mockProposalTask{action: tbtc.ActionDepositSweep, results: map[[20]byte]mockProposalTaskResult{walletPKH: test.fallback}},
			}}
			recorder := proposalMetrics{}
			generator.SetProposalMetricsRecorder(recorder)
			checklist := test.checklist
			if checklist == nil {
				checklist = []tbtc.WalletActionType{tbtc.ActionRedemption, tbtc.ActionDepositSweep}
			}
			_, _ = generator.Generate(&tbtc.CoordinationProposalRequest{WalletPublicKeyHash: walletPKH, ActionsChecklist: checklist})
			expected := map[string]float64{
				clientinfo.MetricRedemptionProposalGenerationAttemptsTotal: test.attempts,
				clientinfo.MetricRedemptionProposalGenerationFailuresTotal: test.failures,
				clientinfo.MetricRedemptionProposalsGeneratedTotal:         test.generated,
			}
			for name, value := range expected {
				if recorder[name] != value {
					t.Errorf("%s: got %v, want %v", name, recorder[name], value)
				}
			}
			before := make(proposalMetrics)
			for name, value := range recorder {
				before[name] = value
			}
			generator.SetProposalMetricsRecorder(nil)
			_, _ = generator.Generate(&tbtc.CoordinationProposalRequest{WalletPublicKeyHash: walletPKH, ActionsChecklist: checklist})
			if !reflect.DeepEqual(before, recorder) {
				t.Fatal("disabled recorder was still updated")
			}
		})
	}
}
