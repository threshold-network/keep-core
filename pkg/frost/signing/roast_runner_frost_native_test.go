//go:build frost_native

package signing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// fixedTestSigner returns a fixed non-empty signature so the wire envelopes
// Marshal (they reject empty signatures); paired with NoOpSignatureVerifier it
// authenticates without real operator-key crypto.
type fixedTestSigner struct{}

func (fixedTestSigner) Sign(_ []byte) ([]byte, error) { return []byte{0x01}, nil }

type harness struct {
	bus         RunnerBus
	runners     []*interactiveSigningRunner
	coords      []roast.Coordinator
	collectors  []*roast.Round2Collector
	handles     []roast.AttemptHandle
	contextHash [attempt.MessageDigestLength]byte
	includedSet []group.MemberIndex
}

// buildInteractiveSigningHarness wires n nodes - each with its own coordinator,
// collector, and fake engine - to one shared in-process bus. Every node
// subscribes in its constructor, so all are wired before any Run broadcasts.
func buildInteractiveSigningHarness(t *testing.T, n int, threshold uint16) harness {
	t.Helper()
	included := make([]group.MemberIndex, 0, n)
	for i := 1; i <= n; i++ {
		included = append(included, group.MemberIndex(i))
	}
	dkgKey := []byte{0x01, 0x02}
	ctx, err := attempt.NewAttemptContext(
		"session-1", "key-group-1", dkgKey,
		[attempt.MessageDigestLength]byte{0x42}, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}

	bus := NewInProcessRunnerBus(256)
	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()

	h := harness{bus: bus, contextHash: ctx.Hash(), includedSet: included}
	for i := 0; i < n; i++ {
		member := group.MemberIndex(i + 1)
		coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
		handle, err := coord.BeginAttempt(ctx)
		if err != nil {
			t.Fatalf("begin attempt (member %d): %v", member, err)
		}
		ara, err := NewActiveRoastAttempt(coord, handle, ctx, "session-1", nil, dkgKey)
		if err != nil {
			t.Fatalf("active attempt (member %d): %v", member, err)
		}
		collector := roast.NewRound2Collector(verifier)
		runner, err := newInteractiveSigningRunner(
			ara, member, threshold,
			newFakeInteractiveSigningEngine(),
			collector,
			coord, signer, bus,
		)
		if err != nil {
			t.Fatalf("runner (member %d): %v", member, err)
		}
		h.coords = append(h.coords, coord)
		h.collectors = append(h.collectors, collector)
		h.handles = append(h.handles, handle)
		h.runners = append(h.runners, runner)
	}
	return h
}

// runAll runs every node concurrently and asserts each reaches a successful
// aggregate and transitions its attempt to Succeeded.
func (h harness) runAndAssertAllSucceed(t *testing.T) {
	t.Helper()
	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sigs := make([][]byte, len(h.runners))
	errs := make([]error, len(h.runners))
	var wg sync.WaitGroup
	for i := range h.runners {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sigs[idx], errs[idx] = h.runners[idx].Run(runCtx)
		}(i)
	}
	wg.Wait()

	for i := range h.runners {
		member := i + 1
		if errs[i] != nil {
			t.Fatalf("member %d run failed: %v", member, errs[i])
		}
		if string(sigs[i]) != "fake-bip340-signature" {
			t.Fatalf("member %d unexpected signature: %q", member, sigs[i])
		}
		state, err := h.coords[i].State(h.handles[i])
		if err != nil {
			t.Fatalf("member %d state: %v", member, err)
		}
		if state != roast.AttemptStateSucceeded {
			t.Fatalf("member %d: expected Succeeded, got %v", member, state)
		}
	}
}

func TestInteractiveSigningRunner_HappyPath(t *testing.T) {
	buildInteractiveSigningHarness(t, 3, 2).runAndAssertAllSucceed(t)
}

// A concluded attempt must leave no retained round-2 collector state, per the
// collector's prune-on-conclusion contract (else a long-lived collector reused
// across attempts accumulates every attempt's envelopes). The collector exposes
// no presence query, but a SURVIVING record makes a re-begin of the same context
// hash under a DIFFERENT binding conflict, whereas a pruned collector accepts
// that fresh begin - so a clean re-begin proves the prune happened.
func TestInteractiveSigningRunner_PrunesCollectorStateAfterSuccess(t *testing.T) {
	h := buildInteractiveSigningHarness(t, 3, 2)
	h.runAndAssertAllSucceed(t)

	elected := h.runners[0].attempt.ElectedCoordinator()
	var differentElected group.MemberIndex
	for _, m := range h.includedSet {
		if m != elected {
			differentElected = m
			break
		}
	}
	for i, collector := range h.collectors {
		if err := collector.BeginAttempt(h.contextHash[:], differentElected, h.includedSet); err != nil {
			t.Fatalf("member %d: collector retained concluded attempt state (conflicting re-begin: %v)", i+1, err)
		}
	}
}

// Adversarial bus traffic - a garbage signing package from a non-elected sender
// and a share from a member outside the included set - must be ignored by the
// runner's sender/membership/attempt filters, leaving the honest run to succeed.
func TestInteractiveSigningRunner_IgnoresAdversarialBusMessages(t *testing.T) {
	h := buildInteractiveSigningHarness(t, 3, 2)

	// Determine the elected coordinator, then have a DIFFERENT included member
	// spam a garbage package (wrong sender) and an outsider (member 99) spam a
	// share. Injected before Run starts, so they sit first in every buffer.
	elected := h.runners[0].attempt.ElectedCoordinator()
	var nonElected group.MemberIndex
	for _, m := range h.includedSet {
		if m != elected {
			nonElected = m
			break
		}
	}
	h.bus.Broadcast(RunnerMessage{
		Type: RunnerMsgSigningPackage, Sender: nonElected, Attempt: h.contextHash,
		Payload: []byte("garbage package from a non-coordinator"),
	})
	h.bus.Broadcast(RunnerMessage{
		Type: RunnerMsgShareSubmission, Sender: group.MemberIndex(99), Attempt: h.contextHash,
		Payload: []byte("garbage share from an outsider"),
	})

	h.runAndAssertAllSucceed(t)
}

// When Run exits early after the engine session is open (here: ctx expires
// while collecting commitments, before round 2 consumes the nonces), the runner
// must abort the native attempt so the engine drops the resident secret
// nonces/session state rather than leaking it.
func TestInteractiveSigningRunner_AbortsNativeAttemptOnEarlyExit(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	dkgKey := []byte{0x01, 0x02}
	ctx, err := attempt.NewAttemptContext(
		"session-1", "key-group-1", dkgKey,
		[attempt.MessageDigestLength]byte{0x42}, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}
	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	bus := NewInProcessRunnerBus(8)
	coord := roast.NewInMemoryCoordinatorWithSigning(1, signer, verifier)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	ara, err := NewActiveRoastAttempt(coord, handle, ctx, "session-1", nil, dkgKey)
	if err != nil {
		t.Fatalf("active attempt: %v", err)
	}
	engine := newFakeInteractiveSigningEngine()
	runner, err := newInteractiveSigningRunner(
		ara, 1, 2, engine, roast.NewRound2Collector(verifier), coord, signer, bus,
	)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}

	// No second node ever broadcasts, so Run blocks in collectCommitments until
	// the short deadline fires - an early exit with the session already open.
	runCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.Run(runCtx); err == nil {
		t.Fatal("expected Run to fail on the expired context")
	}
	if got := engine.abortCallCount(); got != 1 {
		t.Fatalf("expected exactly one native abort on early exit, got %d", got)
	}
}

func TestNewInteractiveSigningRunner_RejectsInvalidConstruction(t *testing.T) {
	// A valid baseline to vary one field at a time.
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x01, 0x02}
	ctx, err := attempt.NewAttemptContext(
		"session-1", "key-group-1", dkgKey,
		[attempt.MessageDigestLength]byte{0x42}, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}
	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	bus := NewInProcessRunnerBus(8)
	coord := roast.NewInMemoryCoordinatorWithSigning(1, signer, verifier)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	ara, err := NewActiveRoastAttempt(coord, handle, ctx, "session-1", nil, dkgKey)
	if err != nil {
		t.Fatalf("active attempt: %v", err)
	}
	engine := newFakeInteractiveSigningEngine()
	collector := roast.NewRound2Collector(verifier)

	// Sanity: the baseline constructs.
	if _, err := newInteractiveSigningRunner(ara, 1, 2, engine, collector, coord, signer, bus); err != nil {
		t.Fatalf("baseline construction failed: %v", err)
	}

	tests := map[string]func() (*interactiveSigningRunner, error){
		"nil attempt": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(nil, 1, 2, engine, collector, coord, signer, bus)
		},
		"nil engine": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, 2, nil, collector, coord, signer, bus)
		},
		"nil collector": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, 2, engine, nil, coord, signer, bus)
		},
		"nil coordinator": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, 2, engine, collector, nil, signer, bus)
		},
		"nil signer": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, 2, engine, collector, coord, nil, bus)
		},
		"nil bus": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, 2, engine, collector, coord, signer, nil)
		},
		"zero threshold": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, 0, engine, collector, coord, signer, bus)
		},
		"member not included": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 99, 2, engine, collector, coord, signer, bus)
		},
	}
	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); err == nil {
				t.Fatal("expected invalid construction to be rejected")
			}
		})
	}
}
