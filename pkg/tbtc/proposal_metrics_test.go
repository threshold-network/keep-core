package tbtc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type proposalCounter map[string]float64

func (m proposalCounter) IncrementCounter(name string, value float64) { m[name] += value }
func (m proposalCounter) SetGauge(string, float64)                    {}
func (m proposalCounter) RecordDuration(string, time.Duration)        {}

type proposalBroadcast struct {
	net.BroadcastChannel
	err     error
	message *coordinationMessage
}

func (b *proposalBroadcast) Send(_ context.Context, message net.TaggedMarshaler, _ ...net.RetransmissionStrategy) error {
	b.message = message.(*coordinationMessage)
	return b.err
}

func TestRedemptionProposalBroadcastMetrics(t *testing.T) {
	for _, test := range []struct {
		name     string
		proposal CoordinationProposal
		err      error
		disabled bool
		sent     float64
		failed   float64
	}{
		{"redemption sent", &RedemptionProposal{}, nil, false, 1, 0},
		{"redemption send fails", &RedemptionProposal{}, errors.New("broadcast unavailable"), false, 0, 1},
		{"no-op is not a redemption", &NoopProposal{}, nil, false, 0, 0},
		{"other action fails", &HeartbeatProposal{}, errors.New("broadcast unavailable"), false, 0, 0},
		{"disabled metrics", &RedemptionProposal{}, nil, true, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			channel := &proposalBroadcast{err: test.err}
			executor := &coordinationExecutor{
				coordinatedWallet: createMockSigner(t).wallet,
				membersIndexes:    []group.MemberIndex{2, 1},
				broadcastChannel:  channel,
				proposalGenerator: newMockCoordinationProposalGenerator(func([20]byte, []WalletActionType, uint) (CoordinationProposal, error) {
					return test.proposal, nil
				}),
			}
			recorder := proposalCounter{}
			if !test.disabled {
				executor.setMetricsRecorder(recorder)
			}
			_, err := executor.executeLeaderRoutine(context.Background(), 900, []WalletActionType{ActionRedemption})
			if (err != nil) != (test.err != nil) {
				t.Fatalf("unexpected result: %v", err)
			}
			if channel.message == nil || channel.message.coordinationBlock != 900 || channel.message.proposal != test.proposal {
				t.Fatal("expected the original proposal to be passed to broadcast")
			}
			if recorder[clientinfo.MetricRedemptionProposalsBroadcastTotal] != test.sent || recorder[clientinfo.MetricRedemptionProposalBroadcastFailuresTotal] != test.failed {
				t.Fatalf("unexpected broadcast counters: %v", recorder)
			}
		})
	}
}

type proposalMetricsSetter struct {
	CoordinationProposalGenerator
	recorder interface{ IncrementCounter(string, float64) }
}

func (p *proposalMetricsSetter) SetProposalMetricsRecorder(recorder interface{ IncrementCounter(string, float64) }) {
	p.recorder = recorder
}

func TestNodeWiresProposalMetrics(t *testing.T) {
	generator := &proposalMetricsSetter{}
	n := &node{proposalGenerator: generator}
	recorder := proposalCounter{}
	n.setPerformanceMetrics(recorder)
	if generator.recorder == nil {
		t.Fatal("proposal generator did not receive the metrics recorder")
	}
	generator.recorder.IncrementCounter("wiring-test", 1)
	if recorder["wiring-test"] != 1 {
		t.Fatal("node passed the wrong recorder")
	}
	n.setPerformanceMetrics(nil)
	if generator.recorder != nil {
		t.Fatal("disabling node metrics must clear the generator recorder")
	}
}
