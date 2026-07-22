package beacon

import (
	"fmt"
	"math/big"
	"testing"

	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
)

var relayEntryTimeout = uint64(15)

// filterErrorChannel is a broadcast channel whose SetFilter result is
// controllable, used to exercise the membership-filter abort path.
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

// TestSetBroadcastChannelFilter verifies that the membership filter is required
// before a node proceeds on a group channel: when the filter cannot be set the
// helper surfaces the error so the caller aborts, rather than proceeding on an
// unfiltered channel that would accept messages from operators outside the
// group.
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
