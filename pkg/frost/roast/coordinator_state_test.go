package roast

import (
	"errors"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newTestContext(t *testing.T) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"session-test",
		"key-group-test",
		[]byte{0xab, 0xcd, 0xef},
		[attempt.MessageDigestLength]byte{0x42},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("test context: %v", err)
	}
	return ctx
}

func TestBeginAttempt_ReturnsHandleWithMatchingContextHash(t *testing.T) {
	coord := NewInMemoryCoordinator()
	ctx := newTestContext(t)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.ContextHash() != ctx.Hash() {
		t.Fatalf(
			"handle hash mismatch: got %x want %x",
			handle.ContextHash(), ctx.Hash(),
		)
	}
}

func TestBeginAttempt_HandlesAreDistinctAcrossAttempts(t *testing.T) {
	coord := NewInMemoryCoordinator()
	a, err := coord.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("first begin: %v", err)
	}
	b, err := coord.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("second begin: %v", err)
	}
	if a.id == b.id {
		t.Fatalf("two attempts shared handle id %d", a.id)
	}
}

func TestBeginAttempt_RejectsEmptyIncludedSet(t *testing.T) {
	coord := NewInMemoryCoordinator()
	// We bypass NewAttemptContext (which forbids empty included set)
	// to assert BeginAttempt's defence-in-depth check.
	ctx := attempt.AttemptContext{}
	_, err := coord.BeginAttempt(ctx)
	if err == nil {
		t.Fatal("expected error on empty included set")
	}
}

func TestState_ReturnsCollectingAfterBegin(t *testing.T) {
	coord := NewInMemoryCoordinator()
	handle, err := coord.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	state, err := coord.State(handle)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state != AttemptStateCollecting {
		t.Fatalf(
			"expected collecting, got %v",
			state,
		)
	}
}

func TestState_UnknownHandleReturnsSentinel(t *testing.T) {
	coord := NewInMemoryCoordinator()
	bogus := AttemptHandle{id: 999}
	state, err := coord.State(bogus)
	if !errors.Is(err, ErrUnknownAttempt) {
		t.Fatalf("expected ErrUnknownAttempt, got %v", err)
	}
	if state != AttemptStatePending {
		t.Fatalf("expected pending sentinel, got %v", state)
	}
}

func TestSelectedCoordinator_ReturnsMemberFromIncludedSet(t *testing.T) {
	coord := NewInMemoryCoordinator()
	ctx := newTestContext(t)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	got, err := coord.SelectedCoordinator(handle)
	if err != nil {
		t.Fatalf("selected coordinator: %v", err)
	}
	found := false
	for _, m := range ctx.IncludedSet {
		if m == got {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf(
			"selected coordinator %d not in included set %v",
			got, ctx.IncludedSet,
		)
	}
}

func TestSelectedCoordinator_IsDeterministicForSameContext(t *testing.T) {
	a := NewInMemoryCoordinator()
	b := NewInMemoryCoordinator()
	ctx := newTestContext(t)
	ha, err := a.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("a.begin: %v", err)
	}
	hb, err := b.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("b.begin: %v", err)
	}
	ca, err := a.SelectedCoordinator(ha)
	if err != nil {
		t.Fatalf("a.selected: %v", err)
	}
	cb, err := b.SelectedCoordinator(hb)
	if err != nil {
		t.Fatalf("b.selected: %v", err)
	}
	if ca != cb {
		t.Fatalf(
			"two coordinators disagreed on same context: %d != %d",
			ca, cb,
		)
	}
}

func TestSelectedCoordinator_DifferentAttemptNumbersCanProduceDifferentLeaders(t *testing.T) {
	coord := NewInMemoryCoordinator()
	build := func(attemptNumber uint32) attempt.AttemptContext {
		ctx, err := attempt.NewAttemptContext(
			"session-test",
			"key-group-test",
			[]byte{0x01},
			[attempt.MessageDigestLength]byte{0x42},
			attemptNumber,
			[]group.MemberIndex{1, 2, 3, 4, 5},
			nil,
		)
		if err != nil {
			t.Fatalf("build ctx: %v", err)
		}
		return ctx
	}

	// Sweep a few attempt numbers; verify the elected coordinator is
	// not always the same member -- otherwise the retry-rotation
	// property of ROAST does not hold.
	seen := map[group.MemberIndex]struct{}{}
	for n := uint32(0); n < 16; n++ {
		ctx := build(n)
		handle, err := coord.BeginAttempt(ctx)
		if err != nil {
			t.Fatalf("begin n=%d: %v", n, err)
		}
		c, err := coord.SelectedCoordinator(handle)
		if err != nil {
			t.Fatalf("selected n=%d: %v", n, err)
		}
		seen[c] = struct{}{}
	}
	if len(seen) < 2 {
		t.Fatalf(
			"coordinator rotation broken: 16 different attempts all "+
				"elected the same leader; seen=%v",
			seen,
		)
	}
}

func TestSelectedCoordinator_UnknownHandleReturnsSentinel(t *testing.T) {
	coord := NewInMemoryCoordinator()
	bogus := AttemptHandle{id: 999}
	got, err := coord.SelectedCoordinator(bogus)
	if !errors.Is(err, ErrUnknownAttempt) {
		t.Fatalf("expected ErrUnknownAttempt, got %v", err)
	}
	if got != 0 {
		t.Fatalf("expected zero member index, got %d", got)
	}
}

func TestInMemoryCoordinator_ConcurrentBeginAttemptsAreRaceSafe(t *testing.T) {
	const numGoroutines = 16
	const beginsPerGoroutine = 50

	coord := NewInMemoryCoordinator()
	var wg sync.WaitGroup
	handles := make(chan AttemptHandle, numGoroutines*beginsPerGoroutine)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < beginsPerGoroutine; i++ {
				h, err := coord.BeginAttempt(newTestContext(t))
				if err != nil {
					t.Errorf("concurrent begin: %v", err)
					return
				}
				handles <- h
			}
		}()
	}
	wg.Wait()
	close(handles)

	ids := map[uint64]struct{}{}
	for h := range handles {
		if _, dup := ids[h.id]; dup {
			t.Fatalf("duplicate handle id %d under concurrency", h.id)
		}
		ids[h.id] = struct{}{}
	}
	if len(ids) != numGoroutines*beginsPerGoroutine {
		t.Fatalf(
			"expected %d unique handles, got %d",
			numGoroutines*beginsPerGoroutine, len(ids),
		)
	}
}

func TestAttemptState_String(t *testing.T) {
	cases := map[AttemptState]string{
		AttemptStatePending:      "pending",
		AttemptStateCollecting:   "collecting",
		AttemptStateAggregating:  "aggregating",
		AttemptStateSucceeded:    "succeeded",
		AttemptStateTransitioned: "transitioned",
		AttemptState(99):         "unknown(99)",
	}
	for state, want := range cases {
		if got := state.String(); got != want {
			t.Errorf("State %d: got %q want %q", state, got, want)
		}
	}
}

func TestMarkSucceeded_TransitionsCollectingToSucceeded(t *testing.T) {
	coord := NewInMemoryCoordinator()
	handle, err := coord.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := coord.MarkSucceeded(handle); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}

	// State is now Succeeded, not Collecting - so the cleanup path's
	// state == Collecting guard skips it and no spurious TransitionMessage is
	// produced for an attempt that actually completed.
	state, err := coord.State(handle)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state != AttemptStateSucceeded {
		t.Fatalf("expected Succeeded, got %v", state)
	}
}

func TestMarkSucceeded_UnknownHandleReturnsSentinel(t *testing.T) {
	coord := NewInMemoryCoordinator()
	if err := coord.MarkSucceeded(AttemptHandle{id: 999}); !errors.Is(err, ErrUnknownAttempt) {
		t.Fatalf("expected ErrUnknownAttempt, got %v", err)
	}
}

func TestMarkSucceeded_RejectsNonCollectingAttempt(t *testing.T) {
	coord := NewInMemoryCoordinator()
	handle, err := coord.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := coord.MarkSucceeded(handle); err != nil {
		t.Fatalf("first mark succeeded: %v", err)
	}

	// A second mark on an already-succeeded (non-Collecting) attempt fails
	// closed rather than masking a caller bug.
	if err := coord.MarkSucceeded(handle); !errors.Is(err, ErrAttemptStateInvalid) {
		t.Fatalf("expected ErrAttemptStateInvalid, got %v", err)
	}
}
