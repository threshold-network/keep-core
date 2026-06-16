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
	message := []byte("message-to-sign")

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
		runner, err := newInteractiveSigningRunner(
			ara, member, message, threshold,
			newFakeInteractiveSigningEngine(),
			roast.NewRound2Collector(verifier),
			coord, signer, bus,
		)
		if err != nil {
			t.Fatalf("runner (member %d): %v", member, err)
		}
		h.coords = append(h.coords, coord)
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
	msg := []byte("m")

	// Sanity: the baseline constructs.
	if _, err := newInteractiveSigningRunner(ara, 1, msg, 2, engine, collector, coord, signer, bus); err != nil {
		t.Fatalf("baseline construction failed: %v", err)
	}

	tests := map[string]func() (*interactiveSigningRunner, error){
		"nil attempt": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(nil, 1, msg, 2, engine, collector, coord, signer, bus)
		},
		"nil engine": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, msg, 2, nil, collector, coord, signer, bus)
		},
		"nil collector": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, msg, 2, engine, nil, coord, signer, bus)
		},
		"nil coordinator": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, msg, 2, engine, collector, nil, signer, bus)
		},
		"nil signer": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, msg, 2, engine, collector, coord, nil, bus)
		},
		"nil bus": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, msg, 2, engine, collector, coord, signer, nil)
		},
		"zero threshold": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, msg, 0, engine, collector, coord, signer, bus)
		},
		"empty message": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 1, nil, 2, engine, collector, coord, signer, bus)
		},
		"member not included": func() (*interactiveSigningRunner, error) {
			return newInteractiveSigningRunner(ara, 99, msg, 2, engine, collector, coord, signer, bus)
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
