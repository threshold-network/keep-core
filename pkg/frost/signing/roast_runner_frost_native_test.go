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

// TestInteractiveSigningRunner_HappyPath wires N nodes - each with its own
// coordinator, collector, and fake engine - to one shared in-process bus and
// runs them concurrently. Every node must reach a successful aggregate, emit the
// signature, and transition its attempt to Succeeded.
func TestInteractiveSigningRunner_HappyPath(t *testing.T) {
	const n = 3
	const threshold = uint16(2)

	included := make([]group.MemberIndex, 0, n)
	for i := 1; i <= n; i++ {
		included = append(included, group.MemberIndex(i))
	}
	dkgKey := []byte{0x01, 0x02}
	ctx, err := attempt.NewAttemptContext(
		"session-1",
		"key-group-1",
		dkgKey,
		[attempt.MessageDigestLength]byte{0x42},
		0,
		included,
		nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}

	bus := NewInProcessRunnerBus(256)
	// A fixed non-empty signature: the wire envelopes' Marshal rejects an empty
	// signature (so NoOpSigner, which signs nil, cannot be used), while the
	// accept-anything verifier authenticates it - the runner test exercises the
	// envelope/signing plumbing but not real operator-key crypto.
	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	message := []byte("message-to-sign")

	coords := make([]roast.Coordinator, n)
	handles := make([]roast.AttemptHandle, n)
	runners := make([]*interactiveSigningRunner, n)

	// Construct every node (each subscribes to the bus in its constructor)
	// BEFORE any Run broadcasts, so no node misses a peer's first message.
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
			ara,
			member,
			message,
			threshold,
			newFakeInteractiveSigningEngine(),
			roast.NewRound2Collector(verifier),
			coord,
			signer,
			bus,
		)
		if err != nil {
			t.Fatalf("runner (member %d): %v", member, err)
		}
		coords[i], handles[i], runners[i] = coord, handle, runner
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type result struct {
		sig []byte
		err error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sig, runErr := runners[idx].Run(runCtx)
			results[idx] = result{sig, runErr}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		member := i + 1
		if results[i].err != nil {
			t.Fatalf("member %d run failed: %v", member, results[i].err)
		}
		if string(results[i].sig) != "fake-bip340-signature" {
			t.Fatalf("member %d unexpected signature: %q", member, results[i].sig)
		}
		state, err := coords[i].State(handles[i])
		if err != nil {
			t.Fatalf("member %d state: %v", member, err)
		}
		if state != roast.AttemptStateSucceeded {
			t.Fatalf("member %d: expected Succeeded, got %v", member, state)
		}
	}
}
