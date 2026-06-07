//go:build frost_native

package signing

import (
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestEnqueueOrRecordOverflow_EnqueuesWhenChannelHasRoom(t *testing.T) {
	ch := make(chan *buildTaggedTBTCSignerRoundContributionMessage, 4)
	rec := attempt.NewBoundedRecorder()
	payload := &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 1}

	if !enqueueOrRecordOverflow(payload, ch, rec) {
		t.Fatal("enqueue should succeed when channel has room")
	}
	if got := rec.Snapshot().Overflows[1]; got != 0 {
		t.Fatalf("no overflow expected on successful enqueue; got %d", got)
	}
	if len(ch) != 1 {
		t.Fatalf("channel length expected 1, got %d", len(ch))
	}
}

func TestEnqueueOrRecordOverflow_RecordsOverflowWhenChannelIsFull(t *testing.T) {
	ch := make(chan *buildTaggedTBTCSignerRoundContributionMessage, 1)
	ch <- &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 99} // fill it
	rec := attempt.NewBoundedRecorder()

	payload := &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 7}
	if enqueueOrRecordOverflow(payload, ch, rec) {
		t.Fatal("enqueue should fail when channel is full")
	}
	if got := rec.Snapshot().Overflows[7]; got != 1 {
		t.Fatalf(
			"overflow should be recorded against sender 7; got count %d",
			got,
		)
	}
	if got := rec.Snapshot().Overflows[99]; got != 0 {
		t.Fatal(
			"sender 99 is the pre-filled payload's sender, not the overflow sender",
		)
	}
}

func TestEnqueueOrRecordOverflow_NoOpRecorderHasNoObservableEffect(t *testing.T) {
	ch := make(chan *buildTaggedTBTCSignerRoundContributionMessage, 1)
	ch <- &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 1}
	rec := attempt.NoOpRecorder()

	payload := &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 7}
	if enqueueOrRecordOverflow(payload, ch, rec) {
		t.Fatal("enqueue should fail when channel is full")
	}
	if got := rec.Snapshot().Overflows[7]; got != 0 {
		t.Fatalf(
			"NoOp recorder must show zero overflow count even when called; got %d",
			got,
		)
	}
}

func TestEnqueueOrRecordOverflow_RepeatedOverflowsSaturateAtQuota(t *testing.T) {
	ch := make(chan *buildTaggedTBTCSignerRoundContributionMessage, 1)
	ch <- &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 1}
	rec := attempt.NewBoundedRecorderWithQuota(3)

	for i := 0; i < 10; i++ {
		_ = enqueueOrRecordOverflow(
			&buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 2},
			ch,
			rec,
		)
	}
	if got := rec.Snapshot().Overflows[2]; got != 3 {
		t.Fatalf("expected saturation at quota 3, got %d", got)
	}
}

func TestEnqueueOrRecordOverflow_ConcurrentCallersAreRaceSafe(t *testing.T) {
	const numProducers = 8
	const recordsPerProducer = 100
	ch := make(chan *buildTaggedTBTCSignerRoundContributionMessage, 1)
	ch <- &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 1} // fill it once
	rec := attempt.NewBoundedRecorderWithQuota(uint(numProducers * recordsPerProducer))

	var wg sync.WaitGroup
	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		sender := group.MemberIndex(p + 2)
		go func() {
			defer wg.Done()
			for i := 0; i < recordsPerProducer; i++ {
				_ = enqueueOrRecordOverflow(
					&buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: uint32(sender)},
					ch,
					rec,
				)
			}
		}()
	}
	wg.Wait()

	snap := rec.Snapshot()
	totalRecorded := uint(0)
	for _, v := range snap.Overflows {
		totalRecorded += v
	}
	// Every producer's records either enqueued (replacing previously-
	// dequeued items, but there's no consumer here so the channel stays
	// full and all subsequent enqueue attempts fall to the default
	// branch) or recorded. Since the channel starts pre-filled and has
	// no consumer, all 800 records hit the overflow path.
	const expected = numProducers * recordsPerProducer
	if totalRecorded != expected {
		t.Fatalf(
			"concurrent overflow count: got %d, want %d (sum across senders)",
			totalRecorded, expected,
		)
	}
}
