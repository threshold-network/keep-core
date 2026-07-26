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

func TestAggregateInteractiveOnce_LateSeatUsesOuterSessionLifetime(t *testing.T) {
	ResetInteractiveAggregateMemoForTest()
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	session, err := BeginInteractiveAggregateMemoSession("late-seat-session")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Release()

	var calls int
	first, err := aggregateInteractiveOnce(
		"late-seat-session|attempt-1",
		func() ([]byte, error) {
			calls++
			return []byte("first"), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// This call represents a sibling seat arriving arbitrarily late, including
	// after the former five-minute timer. Memo lifetime is now tied only to the
	// outer executor join, so elapsed wall time cannot evict the result.
	late, err := aggregateInteractiveOnce(
		"late-seat-session|attempt-1",
		func() ([]byte, error) {
			calls++
			return []byte("late"), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || string(first) != "first" || string(late) != "first" {
		t.Fatalf(
			"late sibling repeated aggregate [calls %d first %q late %q]",
			calls,
			first,
			late,
		)
	}
}

func TestInteractiveAggregateMemoSessionCleanupIsExactAndIdentityGuarded(
	t *testing.T,
) {
	ResetInteractiveAggregateMemoForTest()
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	firstA, err := BeginInteractiveAggregateMemoSession("session-a")
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err := BeginInteractiveAggregateMemoSession("session-a-long")
	if err != nil {
		t.Fatal(err)
	}
	defer sessionB.Release()

	var callsA int
	var callsB int
	if _, err := aggregateInteractiveOnce(
		"session-a|attempt-1",
		func() ([]byte, error) {
			callsA++
			return []byte("a-1"), nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregateInteractiveOnce(
		"session-a-long|attempt-1",
		func() ([]byte, error) {
			callsB++
			return []byte("b-1"), nil
		},
	); err != nil {
		t.Fatal(err)
	}

	firstA.Release()
	if result, err := aggregateInteractiveOnce(
		"session-a-long|attempt-1",
		func() ([]byte, error) {
			callsB++
			return []byte("b-2"), nil
		},
	); err != nil || string(result) != "b-1" || callsB != 1 {
		t.Fatalf(
			"session-a cleanup crossed into another session [result %q calls %d err %v]",
			result,
			callsB,
			err,
		)
	}

	secondA, err := BeginInteractiveAggregateMemoSession("session-a")
	if err != nil {
		t.Fatal(err)
	}
	defer secondA.Release()
	if result, err := aggregateInteractiveOnce(
		"session-a|attempt-1",
		func() ([]byte, error) {
			callsA++
			return []byte("a-2"), nil
		},
	); err != nil || string(result) != "a-2" || callsA != 2 {
		t.Fatalf(
			"new session did not receive a fresh memo [result %q calls %d err %v]",
			result,
			callsA,
			err,
		)
	}

	// Simulate a delayed cleanup callback from the prior textual session. Its
	// owner identity must not delete secondA's entry.
	releaseInteractiveAggregateMemoSession(firstA)
	if result, err := aggregateInteractiveOnce(
		"session-a|attempt-1",
		func() ([]byte, error) {
			callsA++
			return []byte("a-3"), nil
		},
	); err != nil || string(result) != "a-2" || callsA != 2 {
		t.Fatalf(
			"stale cleanup deleted a newer session [result %q calls %d err %v]",
			result,
			callsA,
			err,
		)
	}
}
