package local

import (
	"context"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
)

// TestReleaseBroadcastChannel verifies ReleaseBroadcastChannel's actual
// effect, not just that it can be called: a released channel's
// retransmission ticker stops firing, and a name reused after release only
// delivers to the newly-registered channel, not any stale one left over
// from before the release.
//
// Delivery is observed via a raw messageHandler registered directly
// (bypassing Recv's retransmission.WithRetransmissionSupport dedup wrapper),
// because the standard retransmission strategy resends the same message
// with the same sequence number on every tick, and the dedup layer collapses
// those to a single callback invocation - counting through it would make
// "the ticker kept firing" indistinguishable from "the ticker fired once".
func TestReleaseBroadcastChannel(t *testing.T) {
	// Use a name unique to this test (not a shared literal like
	// "channel name", which broadcast_channel_test.go also uses) so a
	// channel this test forgets to release can never cross-contaminate
	// another test file's assertions in the same test binary.
	name := t.Name()
	t.Cleanup(func() { ReleaseBroadcastChannel(name) })

	_, pubKey, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	createChannel := func(name string) *localChannel {
		ch := getBroadcastChannel(name, pubKey)
		lc := ch.(*localChannel)
		lc.SetUnmarshaler(func() net.TaggedUnmarshaler {
			return &mockNetMessage{}
		})
		return lc
	}

	registerRawHandler := func(lc *localChannel) <-chan net.Message {
		handler := &messageHandler{
			ctx:     context.Background(),
			channel: make(chan net.Message, 64),
		}
		lc.messageHandlersMutex.Lock()
		lc.messageHandlers = append(lc.messageHandlers, handler)
		lc.messageHandlersMutex.Unlock()
		return handler.channel
	}

	// drain counts every message received on ch during window; used to
	// count raw delivery attempts (one per ticker firing), not distinct
	// messages.
	drain := func(ch <-chan net.Message, window time.Duration) int {
		deadline := time.After(window)
		count := 0
		for {
			select {
			case <-ch:
				count++
			case <-deadline:
				return count
			}
		}
	}

	// 1. Open a channel, let its retransmission ticker fire a few times,
	// release it, then assert no further deliveries occur.
	ch1 := createChannel(name)
	ch1Deliveries := registerRawHandler(ch1)

	if err := ch1.Send(context.Background(), &mockNetMessage{}); err != nil {
		t.Fatal(err)
	}

	if got := drain(ch1Deliveries, RetransmissionTick*3); got <= 1 {
		t.Fatalf(
			"expected repeated ticker deliveries before release, got %d",
			got,
		)
	}

	ReleaseBroadcastChannel(name)

	// NewTimeTicker's piping goroutine selects between an already-elapsed
	// timerTick.C and ctx.Done(); if both are ready when cancel() runs, Go's
	// pseudo-random select can let exactly one straggler tick through before
	// the goroutine observes cancellation and exits. That single straggler
	// is a harmless, already-in-flight retransmission of a message already
	// sent, not a sign the ticker "kept firing" - so absorb it in a short
	// settle window before asserting the real invariant this test cares
	// about: no further deliveries once release has taken effect.
	if got := drain(ch1Deliveries, RetransmissionTick); got > 1 {
		t.Errorf("expected at most one straggler tick after release, got %d", got)
	}

	if got := drain(ch1Deliveries, RetransmissionTick*3); got != 0 {
		t.Errorf("expected no deliveries after release, got %d", got)
	}

	// 2. Open a new channel under the same, just-released name, send a
	// message on it, and assert only the new channel's handler receives
	// it - proving the old channel's registration was actually dropped
	// by the release, not merely shadowed by a map pointer swap.
	ch2 := createChannel(name)
	ch2Deliveries := registerRawHandler(ch2)

	if err := ch2.Send(context.Background(), &mockNetMessage{}); err != nil {
		t.Fatal(err)
	}

	if got := drain(ch2Deliveries, RetransmissionTick*3); got <= 1 {
		t.Errorf(
			"expected repeated ticker deliveries from the new channel, got %d",
			got,
		)
	}
	if got := drain(ch1Deliveries, RetransmissionTick*2); got != 0 {
		t.Errorf(
			"expected the released channel to receive nothing further, got %d",
			got,
		)
	}

	// 3. Releasing a name with zero registered channels is a safe no-op.
	ReleaseBroadcastChannel("nonexistent")
}

// TestLocalBroadcastRelease_BarriersInFlightRetransmit verifies that release
// is a barrier: once a name has been released and re-created, a delivery
// issued by a channel of the released session must not reach the channel of
// the new session. Retransmissions and the initial send share this delivery
// path, so a send on the released channel models a retransmission that was
// already in flight when the release took effect.
//
// Before the fix, delivery re-resolved the channel list by name, so the
// released channel's message (old sender transport ID, old sequence number)
// landed in the new session's channel.
func TestLocalBroadcastRelease_BarriersInFlightRetransmit(t *testing.T) {
	name := t.Name()
	t.Cleanup(func() { ReleaseBroadcastChannel(name) })

	_, pubKey, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	createChannel := func(name string) *localChannel {
		ch := getBroadcastChannel(name, pubKey)
		lc := ch.(*localChannel)
		lc.SetUnmarshaler(func() net.TaggedUnmarshaler {
			return &mockNetMessage{}
		})
		return lc
	}

	registerRawHandler := func(lc *localChannel) <-chan net.Message {
		handler := &messageHandler{
			ctx:     context.Background(),
			channel: make(chan net.Message, 64),
		}
		lc.messageHandlersMutex.Lock()
		lc.messageHandlers = append(lc.messageHandlers, handler)
		lc.messageHandlersMutex.Unlock()
		return handler.channel
	}

	collect := func(ch <-chan net.Message, window time.Duration) []net.Message {
		deadline := time.After(window)
		var messages []net.Message
		for {
			select {
			case message := <-ch:
				messages = append(messages, message)
			case <-deadline:
				return messages
			}
		}
	}

	// Session A: open a channel and let its retransmissions run.
	chA := createChannel(name)
	deliveriesA := registerRawHandler(chA)

	if err := chA.Send(context.Background(), &mockNetMessage{}); err != nil {
		t.Fatal(err)
	}
	if got := len(collect(deliveriesA, RetransmissionTick*3)); got <= 1 {
		t.Fatalf(
			"expected repeated retransmissions in the first session, got %d",
			got,
		)
	}

	// Release session A and start session B under the same name.
	ReleaseBroadcastChannel(name)

	// A tick that snapshotted the channel list just before the release can
	// still deliver to session A's own channel; absorb it before asserting.
	collect(deliveriesA, RetransmissionTick)

	chB := createChannel(name)
	deliveriesB := registerRawHandler(chB)

	if err := chB.Send(context.Background(), &mockNetMessage{}); err != nil {
		t.Fatal(err)
	}
	if got := len(collect(deliveriesB, RetransmissionTick*3)); got <= 1 {
		t.Fatalf(
			"expected repeated retransmissions in the second session, got %d",
			got,
		)
	}

	// Deliver once more through the released session A.
	if err := chA.Send(context.Background(), &mockNetMessage{}); err != nil {
		t.Fatal(err)
	}

	crossed := 0
	own := 0
	for _, message := range collect(deliveriesB, RetransmissionTick*3) {
		if message.TransportSenderID() == chA.identifier {
			crossed++
		} else {
			own++
		}
	}

	if crossed != 0 {
		t.Errorf(
			"expected no message of the released session to reach the new "+
				"session's channel, got %d",
			crossed,
		)
	}
	if own == 0 {
		t.Error("expected the new session's channel to keep receiving its own messages")
	}
	if got := len(collect(deliveriesA, RetransmissionTick*2)); got != 0 {
		t.Errorf(
			"expected the released session's channel to receive nothing, got %d",
			got,
		)
	}
}
