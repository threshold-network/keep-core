package beacon

import (
	"fmt"
	"math/big"
	"testing"

	"go.uber.org/zap"

	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
)

var relayEntryTimeout = uint64(15)

// filterErrorChannel is a broadcast channel whose SetFilter result is
// controllable.
type filterErrorChannel struct {
	net.BroadcastChannel
	setFilterErr error
}

func (c *filterErrorChannel) SetFilter(net.BroadcastChannelFilter) error {
	return c.setFilterErr
}

func (c *filterErrorChannel) Name() string {
	return "test-channel"
}

// TestSetBroadcastChannelFilter verifies that setBroadcastChannelFilter
// propagates the success or error result from the broadcast channel's
// SetFilter method.
func TestSetBroadcastChannelFilter(t *testing.T) {
	filter := func(*operator.PublicKey) bool { return true }

	tests := map[string]struct {
		setFilterErr error
		expectError  bool
	}{
		"filter set successfully": {
			setFilterErr: nil,
			expectError:  false,
		},
		"filter cannot be set": {
			setFilterErr: fmt.Errorf("cannot set filter"),
			expectError:  true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			channel := &filterErrorChannel{setFilterErr: test.setFilterErr}

			err := setBroadcastChannelFilter(
				zap.NewNop().Sugar(),
				channel,
				filter,
			)

			if test.expectError && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !test.expectError && err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
		})
	}
}

func TestMonitorRelayEntryOnChain_EntrySubmitted(t *testing.T) {
	localChain := local_v1.Connect(5, 3)

	node := &node{
		beaconChain: localChain,
	}

	blockCounter, err := node.beaconChain.BlockCounter()
	if err != nil {
		fmt.Printf("failed to setup a block counter: [%v]", err)
	}

	startBlockHeight, err := blockCounter.CurrentBlock()
	if err != nil {
		t.Fatal(err)
	}

	go node.MonitorRelayEntry(startBlockHeight)

	// the window to get a relay entry is from currentBlock to (currentBlock+relayEntryTimeout)
	// we subtract arbitarly 5 blocks to be within this window. Ex. 0 + 15 - 5
	relayEntrySubmissionWindow := startBlockHeight + relayEntryTimeout - 5
	err = blockCounter.WaitForBlockHeight(relayEntrySubmissionWindow)
	if err != nil {
		fmt.Printf(
			"failed to wait for a block: [%v]: [%v]",
			relayEntrySubmissionWindow,
			err,
		)
	}

	err = localChain.SubmitRelayEntry(big.NewInt(1).Bytes())
	if err != nil {
		t.Fatal(err)
	}

	err = blockCounter.WaitForBlockHeight(startBlockHeight + relayEntryTimeout)
	if err != nil {
		t.Fatal(err)
	}

	timeoutsReport := localChain.GetRelayEntryTimeoutReports()
	numberOfReports := len(timeoutsReport)

	if numberOfReports != 0 {
		t.Fatalf(
			"expected 0 relay entry timeout reports; has: [%v]",
			numberOfReports,
		)
	}
}

func TestMonitorRelayEntryOnChain_EntryNotSubmitted(t *testing.T) {
	localChain := local_v1.Connect(5, 3)

	node := &node{
		beaconChain: localChain,
	}

	blockCounter, err := node.beaconChain.BlockCounter()
	if err != nil {
		fmt.Printf("failed to setup a block counter: [%v]", err)
	}

	startBlockHeight, err := blockCounter.CurrentBlock()
	if err != nil {
		t.Fatal(err)
	}

	go node.MonitorRelayEntry(startBlockHeight)

	relayEntryTimeoutFromStart := startBlockHeight + relayEntryTimeout

	// we want to exceed the relay entry timeout to report that a relay entry
	// was not submitted. 5 is an arbitrary number to exceed relayEntryTimeout.
	err = blockCounter.WaitForBlockHeight(relayEntryTimeoutFromStart + 5)
	if err != nil {
		t.Fatal(err)
	}

	timeoutsReport := localChain.GetRelayEntryTimeoutReports()
	numberOfReports := len(timeoutsReport)

	if numberOfReports != 1 {
		t.Fatalf(
			"Number of timeout reports does not match\nexpected: [%v]\nactual:   [%v]",
			1,
			numberOfReports,
		)
	}

	if timeoutsReport[0] != relayEntryTimeoutFromStart {
		t.Fatalf(
			"Timeout reporting must happen only after a relay entry timeout\nexpected: [%v]\nactual:   [%v]",
			relayEntryTimeoutFromStart,
			timeoutsReport[0],
		)
	}
}

// selectGroupChain wraps a beaconchain.Interface and overrides SelectGroup,
// which the embedded implementation does not support, so that a
// deterministic set of selected operators can be returned for
// JoinDKGIfEligible tests.
type selectGroupChain struct {
	beaconchain.Interface
	selectedOperators chain.Addresses
}

func (c *selectGroupChain) SelectGroup(seed *big.Int) (chain.Addresses, error) {
	return c.selectedOperators, nil
}

// fakeNetProvider is a net.Provider that always returns the given channel
// from BroadcastChannelFor, regardless of the requested channel name.
type fakeNetProvider struct {
	net.Provider
	channel net.BroadcastChannel
}

func (p *fakeNetProvider) BroadcastChannelFor(
	name string,
) (net.BroadcastChannel, error) {
	return p.channel, nil
}

// TestJoinDKGIfEligible_AbortsWhenFilterCannotBeSet verifies that, for an
// otherwise-eligible operator, a SetFilter failure on the broadcast channel
// causes JoinDKGIfEligible to return before spawning any DKG protocol
// goroutine. The fake broadcast channel embeds a nil net.BroadcastChannel, so
// a goroutine that incorrectly used it to run the DKG protocol would panic;
// groupRegistry is left nil for the same reason, since RegisterGroup is only
// reached from within that goroutine.
func TestJoinDKGIfEligible_AbortsWhenFilterCannotBeSet(t *testing.T) {
	localChain := local_v1.Connect(5, 3)

	_, operatorPublicKey, err := localChain.OperatorKeyPair()
	if err != nil {
		t.Fatalf("failed to get operator key pair: [%v]", err)
	}

	operatorAddress, err := localChain.Signing().PublicKeyToAddress(operatorPublicKey)
	if err != nil {
		t.Fatalf("failed to get operator address: [%v]", err)
	}

	beaconChain := &selectGroupChain{
		Interface:         localChain,
		selectedOperators: chain.Addresses{operatorAddress},
	}

	testNode := &node{
		beaconChain: beaconChain,
		netProvider: &fakeNetProvider{
			channel: &filterErrorChannel{
				setFilterErr: fmt.Errorf("cannot set filter"),
			},
		},
		groupRegistry: nil,
		protocolLatch: generator.NewProtocolLatch(),
	}

	testNode.JoinDKGIfEligible(big.NewInt(1), 0)
}
