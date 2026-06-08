package retransmission

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"

	"github.com/keep-network/keep-core/pkg/net"
)

func TestRetransmitExpectedNumberOfTimes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 510*time.Millisecond)
	defer cancel()

	var retransmissionsCount uint64
	ScheduleRetransmissions(
		ctx,
		&testutils.MockLogger{},
		NewTimeTicker(ctx, 50*time.Millisecond),
		func() error {
			atomic.AddUint64(&retransmissionsCount, 1)
			return nil
		},
		WithStandardStrategy(),
	)

	<-ctx.Done()

	if atomic.LoadUint64(&retransmissionsCount) != 10 {
		t.Errorf("expected [10] retransmissions, has [%v]", retransmissionsCount)
	}
}

func TestHandlerReceiveUniqueMessages(t *testing.T) {
	var received []net.Message

	handler := WithRetransmissionSupport(func(message net.Message) {
		received = append(received, message)
	})

	handler(&mockNetworkMessage{senderID: "a", seqno: 1})
	handler(&mockNetworkMessage{senderID: "a", seqno: 2})
	handler(&mockNetworkMessage{senderID: "a", seqno: 4})
	handler(&mockNetworkMessage{senderID: "b", seqno: 1})
	handler(&mockNetworkMessage{senderID: "b", seqno: 2})

	if len(received) != 5 {
		t.Fatalf(
			"unexpected number of accepted messages\nactual:   [%v]\nexpected: [5]",
			len(received),
		)
	}
}

func TestHandlerReceiveRetransmissions(t *testing.T) {
	var received []net.Message

	handler := WithRetransmissionSupport(func(message net.Message) {
		received = append(received, message)
	})

	handler(&mockNetworkMessage{senderID: "a", seqno: 1})
	handler(&mockNetworkMessage{senderID: "a", seqno: 2})
	handler(&mockNetworkMessage{senderID: "a", seqno: 2})
	handler(&mockNetworkMessage{senderID: "a", seqno: 1})
	handler(&mockNetworkMessage{senderID: "b", seqno: 2})
	handler(&mockNetworkMessage{senderID: "b", seqno: 1})
	handler(&mockNetworkMessage{senderID: "b", seqno: 1})

	if len(received) != 4 {
		t.Fatalf(
			"unexpected number of accepted messages\nactual:   [%v]\nexpected: [4]",
			len(received),
		)
	}
}

func TestHandlerEvictsOldRetransmissions(t *testing.T) {
	var received []net.Message

	handler := withRetransmissionSupport(func(message net.Message) {
		received = append(received, message)
	}, 2)

	firstMessage := &mockNetworkMessage{senderID: "a", seqno: 1}

	handler(firstMessage)
	handler(&mockNetworkMessage{senderID: "a", seqno: 2})
	handler(&mockNetworkMessage{senderID: "a", seqno: 3})
	handler(firstMessage)

	if len(received) != 4 {
		t.Fatalf(
			"unexpected number of accepted messages\nactual:   [%v]\nexpected: [4]",
			len(received),
		)
	}
}

func TestHandlerEvictsAcrossMultipleCycles(t *testing.T) {
	// cacheSize=3. Two full eviction cycles: seqno 1-6 all accepted (6 total).
	// After seqno 6, cache holds [4,5,6] and seqno 1-3 have been evicted.
	// Retransmitting 4, 5, 6 must be filtered (still cached).
	// Re-sending 1, 2, 3 must be accepted (evicted from cache).
	var received []net.Message

	handler := withRetransmissionSupport(func(message net.Message) {
		received = append(received, message)
	}, 3)

	for i := uint64(1); i <= 6; i++ {
		handler(&mockNetworkMessage{senderID: "a", seqno: i})
	}

	// Still in cache -- must be filtered.
	handler(&mockNetworkMessage{senderID: "a", seqno: 4})
	handler(&mockNetworkMessage{senderID: "a", seqno: 5})
	handler(&mockNetworkMessage{senderID: "a", seqno: 6})

	// Evicted -- must be re-accepted.
	handler(&mockNetworkMessage{senderID: "a", seqno: 1})
	handler(&mockNetworkMessage{senderID: "a", seqno: 2})
	handler(&mockNetworkMessage{senderID: "a", seqno: 3})

	if len(received) != 9 {
		t.Fatalf(
			"unexpected number of accepted messages\nactual:   [%v]\nexpected: [9]",
			len(received),
		)
	}
}

// TestHandlerRingBufferStableUnderManyCycles regression-tests the bounded
// cache: feeding many more unique IDs than the cache size must keep FIFO
// eviction correct cycle after cycle. The slice-shift implementation that
// preceded the ring buffer was functionally correct here too, but its backing
// array grew beyond maxCacheSize before Go reallocated; this test pins the
// observable behavior so a regression in either direction is caught.
func TestHandlerRingBufferStableUnderManyCycles(t *testing.T) {
	const cacheSize = 4
	const totalUnique = 1000

	var received []net.Message
	handler := withRetransmissionSupport(func(message net.Message) {
		received = append(received, message)
	}, cacheSize)

	for i := uint64(1); i <= totalUnique; i++ {
		handler(&mockNetworkMessage{senderID: "a", seqno: i})
	}

	if len(received) != totalUnique {
		t.Fatalf(
			"expected every unique message to be accepted exactly once\nactual:   [%d]\nexpected: [%d]",
			len(received),
			totalUnique,
		)
	}

	// The last cacheSize messages must still be deduplicated.
	for i := uint64(totalUnique - cacheSize + 1); i <= totalUnique; i++ {
		handler(&mockNetworkMessage{senderID: "a", seqno: i})
	}
	if len(received) != totalUnique {
		t.Fatalf(
			"recent messages must remain cached after many cycles\nactual:   [%d]\nexpected: [%d]",
			len(received),
			totalUnique,
		)
	}

	// Messages older than the window must be accepted again.
	for i := uint64(1); i <= uint64(cacheSize); i++ {
		handler(&mockNetworkMessage{senderID: "a", seqno: i})
	}
	if len(received) != totalUnique+cacheSize {
		t.Fatalf(
			"evicted messages must be re-accepted\nactual:   [%d]\nexpected: [%d]",
			len(received),
			totalUnique+cacheSize,
		)
	}
}

func TestHandlerConcurrentAccess(t *testing.T) {
	var mu sync.Mutex
	var received []net.Message

	handler := withRetransmissionSupport(func(message net.Message) {
		mu.Lock()
		received = append(received, message)
		mu.Unlock()
	}, 10)

	const goroutines = 20
	const msgsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < msgsPerGoroutine; i++ {
				handler(&mockNetworkMessage{
					senderID: fmt.Sprintf("peer-%d", g),
					seqno:    uint64(i),
				})
			}
		}(g)
	}
	wg.Wait()

	if len(received) == 0 {
		t.Fatal("expected at least one message to be received")
	}
}

type mockNetworkMessage struct {
	senderID string
	seqno    uint64
}

func (mnm *mockNetworkMessage) TransportSenderID() net.TransportIdentifier {
	return &mockTransportIdentifier{mnm.senderID}
}

func (mnm *mockNetworkMessage) Payload() interface{} {
	panic("not implemented")
}

func (mnm *mockNetworkMessage) Type() string {
	panic("not implemented")
}

func (mnm *mockNetworkMessage) SenderPublicKey() []byte {
	panic("not implemented")
}

func (mnm *mockNetworkMessage) Seqno() uint64 {
	return mnm.seqno
}

// TestScheduleRetransmissions_WithBackoffStrategy verifies that the integrated
// path of ScheduleRetransmissions + BackoffStrategy fires at the correct
// exponential-backoff ticks (1, 3, 6, 11, 20 out of the first 20 ticks).
func TestScheduleRetransmissions_WithBackoffStrategy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan uint64)
	ticker := NewTicker(ticks)

	var retransmissions uint64

	ScheduleRetransmissions(
		ctx,
		&testutils.MockLogger{},
		ticker,
		func() error {
			atomic.AddUint64(&retransmissions, 1)
			return nil
		},
		WithBackoffStrategy(),
	)

	// BackoffStrategy fires at ticks 1, 3, 6, 11, 20 -- 5 fires in 20 ticks.
	for i := uint64(1); i <= 20; i++ {
		ticks <- i
	}

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadUint64(&retransmissions) < 5 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	got := atomic.LoadUint64(&retransmissions)
	if got != 5 {
		t.Errorf(
			"expected 5 retransmissions with BackoffStrategy in 20 ticks, got %d",
			got,
		)
	}
}

// TestScheduleRetransmissions_LogsRetransmitError verifies that when the
// retransmit function returns an error the error is passed to the logger.
func TestScheduleRetransmissions_LogsRetransmitError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan uint64)
	ticker := NewTicker(ticks)

	logger := &capturingLogger{}

	ScheduleRetransmissions(
		ctx,
		logger,
		ticker,
		func() error { return fmt.Errorf("network unavailable") },
		WithStandardStrategy(),
	)

	ticks <- 1

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logger.mu.Lock()
		n := len(logger.errors)
		logger.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	logger.mu.Lock()
	errs := logger.errors
	logger.mu.Unlock()

	if len(errs) == 0 {
		t.Fatal("expected error to be logged, got none")
	}
	if !strings.Contains(errs[0], "network unavailable") {
		t.Errorf("unexpected logged error: %q", errs[0])
	}
}

// TestWithRetransmissionSupport_ConcurrentCallsAreSafe verifies that when
// many goroutines concurrently deliver the same message only one call reaches
// the delegate -- and there are no data races on the deduplication cache.
func TestWithRetransmissionSupport_ConcurrentCallsAreSafe(t *testing.T) {
	var delegateCount uint64

	handler := WithRetransmissionSupport(func(_ net.Message) {
		atomic.AddUint64(&delegateCount, 1)
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler(&mockNetworkMessage{senderID: "peer-a", seqno: 42})
		}()
	}
	wg.Wait()

	got := atomic.LoadUint64(&delegateCount)
	if got != 1 {
		t.Errorf("expected delegate called exactly once for duplicate messages, got %d", got)
	}
}

type mockTransportIdentifier struct {
	senderID string
}

func (mti *mockTransportIdentifier) String() string {
	return mti.senderID
}

// capturingLogger wraps MockLogger and records Errorf calls for assertions.
type capturingLogger struct {
	testutils.MockLogger
	mu     sync.Mutex
	errors []string
}

func (cl *capturingLogger) Errorf(format string, args ...interface{}) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.errors = append(cl.errors, fmt.Sprintf(format, args...))
}
