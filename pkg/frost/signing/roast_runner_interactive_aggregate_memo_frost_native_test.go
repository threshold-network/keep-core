//go:build frost_native

package signing

import (
	"fmt"
	"sync"
	"testing"
)

// TestAggregateInteractiveOnce_RunsAtMostOncePerKey pins the core contract that
// lets a multi-seat operator's local seats share one aggregation: for a given
// key, aggregate executes at most once and every caller of that key observes the
// same result. This is why the runner tests inject passThroughAggregateOnce -
// they simulate SEPARATE operators and must not share an aggregation.
func TestAggregateInteractiveOnce_RunsAtMostOncePerKey(t *testing.T) {
	ResetInteractiveAggregateMemoForTest()
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	var calls int
	first, err := aggregateInteractiveOnce("k", func() ([]byte, error) {
		calls++
		return []byte("sig-1"), nil
	})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// A second call under the same key must NOT run its aggregate; it returns the
	// first result even though this func would produce a different signature.
	second, err := aggregateInteractiveOnce("k", func() ([]byte, error) {
		calls++
		return []byte("sig-2"), nil
	})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if calls != 1 {
		t.Fatalf("aggregate ran %d times, want exactly 1", calls)
	}
	if string(first) != "sig-1" || string(second) != "sig-1" {
		t.Fatalf("siblings disagree: first=%q second=%q, want both sig-1", first, second)
	}
}

// TestAggregateInteractiveOnce_PropagatesFirstErrorToSiblings shows the memo is
// result-faithful, not success-only: if the first aggregation errors, siblings
// get that same error rather than silently re-running and diverging.
func TestAggregateInteractiveOnce_PropagatesFirstErrorToSiblings(t *testing.T) {
	ResetInteractiveAggregateMemoForTest()
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	wantErr := fmt.Errorf("aggregate boom")
	var calls int
	_, err1 := aggregateInteractiveOnce("k", func() ([]byte, error) {
		calls++
		return nil, wantErr
	})
	_, err2 := aggregateInteractiveOnce("k", func() ([]byte, error) {
		calls++
		return []byte("should-not-run"), nil
	})

	if calls != 1 {
		t.Fatalf("aggregate ran %d times, want exactly 1", calls)
	}
	if err1 != wantErr || err2 != wantErr {
		t.Fatalf("siblings must share the first error: err1=%v err2=%v", err1, err2)
	}
}

// TestAggregateInteractiveOnce_DistinctKeysRunIndependently confirms the memo
// scopes to the key: separate (session,attempt) pairs each aggregate once, so
// distinct signing attempts never suppress one another.
func TestAggregateInteractiveOnce_DistinctKeysRunIndependently(t *testing.T) {
	ResetInteractiveAggregateMemoForTest()
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	a, _ := aggregateInteractiveOnce("k1", func() ([]byte, error) { return []byte("a"), nil })
	b, _ := aggregateInteractiveOnce("k2", func() ([]byte, error) { return []byte("b"), nil })
	if string(a) != "a" || string(b) != "b" {
		t.Fatalf("distinct keys must run independently: k1=%q k2=%q", a, b)
	}
}

// TestAggregateInteractiveOnce_ConcurrentCallersDedup exercises the real
// multi-seat race: many goroutines hit the same key at once and exactly one
// aggregation runs, with every caller getting that result. Run under -race.
func TestAggregateInteractiveOnce_ConcurrentCallersDedup(t *testing.T) {
	ResetInteractiveAggregateMemoForTest()
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	const goroutines = 32
	var mu sync.Mutex
	var calls int

	results := make([][]byte, goroutines)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			sig, _ := aggregateInteractiveOnce("shared", func() ([]byte, error) {
				mu.Lock()
				calls++
				mu.Unlock()
				return []byte("winner"), nil
			})
			results[idx] = sig
		}(i)
	}
	close(start)
	wg.Wait()

	if calls != 1 {
		t.Fatalf("aggregate ran %d times under contention, want exactly 1", calls)
	}
	for i, r := range results {
		if string(r) != "winner" {
			t.Fatalf("goroutine %d got %q, want winner", i, r)
		}
	}
}
