package retransmission

import (
	"reflect"
	"sync"
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

// TestBackoffStrategy_ConcurrentTick verifies that BackoffStrategy.Tick is safe
// to call concurrently, as ScheduleRetransmissions does by invoking Tick from a
// new goroutine on every tick. It is meant to be run with the race detector
// (go test -race); without the mutex guarding the retransmission counters, the
// concurrent read/modify/write of tickCounter, delay, and retransmitTick is a
// data race.
func TestBackoffStrategy_ConcurrentTick(t *testing.T) {
	strategy := WithBackoffStrategy()

	const goroutines = 50

	// A barrier releases all goroutines at once to maximize the overlap of the
	// counter updates inside Tick.
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			_ = strategy.Tick(func() error {
				return nil
			})
		}()
	}

	close(start)
	wg.Wait()
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
