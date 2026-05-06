package retransmission

import (
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
)

func TestStandardStrategy(t *testing.T) {
	strategy := WithStandardStrategy()

	retransmitInvocations := make(map[int]bool)

	for i := 1; i <= 10; i++ {
		err := strategy.Tick(func() error {
			retransmitInvocations[i] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	expectedRetransmitInvocations := map[int]bool{
		1:  true,
		2:  true,
		3:  true,
		4:  true,
		5:  true,
		6:  true,
		7:  true,
		8:  true,
		9:  true,
		10: true,
	}
	if !reflect.DeepEqual(expectedRetransmitInvocations, retransmitInvocations) {
		t.Errorf(
			"unexpected invocations\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetransmitInvocations,
			retransmitInvocations,
		)
	}
}

func TestBackoffStrategy(t *testing.T) {
	strategy := WithBackoffStrategy()

	retransmitInvocations := make(map[int]bool)

	for i := 1; i <= 100; i++ {
		err := strategy.Tick(func() error {
			retransmitInvocations[i] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	expectedRetransmitInvocations := map[int]bool{
		1:  true,
		3:  true,
		6:  true,
		11: true,
		20: true,
		37: true,
		70: true,
	}
	if !reflect.DeepEqual(expectedRetransmitInvocations, retransmitInvocations) {
		t.Errorf(
			"unexpected invocations\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetransmitInvocations,
			retransmitInvocations,
		)
	}
}

// TestBackoffStrategy_TickSequence verifies the complete ordered fire sequence
// across 200 ticks. The sequence must be deterministic: each fire advances
// retransmitTick by delay+1 and doubles delay, so the gaps are 2, 3, 5, 9, 17,
// 33, 65, ... producing fires at ticks 1, 3, 6, 11, 20, 37, 70, 135.
func TestBackoffStrategy_TickSequence(t *testing.T) {
	strategy := WithBackoffStrategy()

	var fired []int
	for i := 1; i <= 200; i++ {
		tick := i
		_ = strategy.Tick(func() error {
			fired = append(fired, tick)
			return nil
		})
	}

	sort.Ints(fired)

	expected := []int{1, 3, 6, 11, 20, 37, 70, 135}
	if !reflect.DeepEqual(expected, fired) {
		t.Errorf(
			"unexpected fire sequence\nexpected: %v\nactual:   %v",
			expected,
			fired,
		)
	}
}

// TestBackoffStrategy_ConcurrentTick is a regression test for the data race
// fixed in this package: ScheduleRetransmissions launches a goroutine per
// ticker callback, so overlapping ticks can call Tick concurrently when the
// retransmit function is slow. The mutex serialises counter access; this
// test would fail under `go test -race` without it. The total number of
// fires is deterministic regardless of goroutine interleaving because the
// counter increments and the retransmitTick comparison are atomic together
// under the mutex.
func TestBackoffStrategy_ConcurrentTick(t *testing.T) {
	const (
		goroutines  = 16
		ticksPerG   = 50
		totalTicks  = goroutines * ticksPerG
		expectedHit = 10
	)

	strategy := WithBackoffStrategy()

	var fires atomic.Uint64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range [goroutines]struct{}{} {
		go func() {
			defer wg.Done()
			for range [ticksPerG]struct{}{} {
				_ = strategy.Tick(func() error {
					fires.Add(1)
					return nil
				})
			}
		}()
	}

	wg.Wait()

	// For 800 total ticks the backoff sequence fires at ticks
	// 1, 3, 6, 11, 20, 37, 70, 135, 264, 521 -- ten fires. The next fire
	// would land at tick 1034, beyond the total.
	if got := fires.Load(); got != expectedHit {
		t.Errorf(
			"unexpected fire count after %d concurrent ticks\nexpected: %d\nactual:   %d",
			totalTicks,
			expectedHit,
			got,
		)
	}
}

// --- Benchmarks ---

func BenchmarkBackoffStrategyTick(b *testing.B) {
	strategy := WithBackoffStrategy()
	noop := func() error { return nil }
	b.ResetTimer()
	for range b.N {
		_ = strategy.Tick(noop)
	}
}

func BenchmarkStandardStrategyTick(b *testing.B) {
	strategy := WithStandardStrategy()
	noop := func() error { return nil }
	b.ResetTimer()
	for range b.N {
		_ = strategy.Tick(noop)
	}
}
