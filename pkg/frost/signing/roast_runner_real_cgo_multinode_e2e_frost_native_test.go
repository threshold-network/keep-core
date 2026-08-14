//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	netlocal "github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This file is the "shape (A)" real-crypto multi-node-style e2e (RFC-21 Phase 7.3
// pre-production milestone): n interactive signing RUNNERS, each over its own REAL
// pkg/net BroadcastChannel bus, driving the REAL cgo FROST engine to a REAL BIP-340
// signature - all in one process. It is the union of the two half-faked e2es:
//
//   - roast_runner_bus_net_e2e_frost_native_test.go proves the runner+transport with
//     a FAKE engine (no crypto);
//   - roast_real_cgo_interactive_e2e_frost_native_test.go proves the real engine with
//     NO runner and NO transport (direct engine calls, one process).
//
// Neither exercises the real-engine <-> real-transport <-> runner seam together; this
// does. Per the Codex design consult (2026-06-21), shape (A) is the required
// pre-cutover harness; the single-OS-process / one-shared-engine simplification is the
// engine's process-global constraint (ENGINE_STATE is a process-global OnceLock<Mutex>),
// not a correctness gap - the multi-seat fix (#4098) is exactly what lets one engine
// serve every local member. Separate-process fidelity (shape B) is a later/nightly
// concern that only adds PROCESS-boundary coverage (per-node state isolation, per-process
// linking/env, libp2p outer framing), not protocol-message coverage.
//
// SERIALIZATION (the point of the test): although all runners share one engine, peer
// messages still make a real byte round-trip. The runner broadcasts byte payloads; the
// net bus wraps them in a runnerTransportMessage and calls channel.Send, and the local
// pkg/net channel Marshal()s then Unmarshal()s into a fresh payload object. So a peer's
// commitments, signing package, and shares are serialized over the production transport
// adapter. Two caveats: (i) pkg/net/local does not exercise libp2p's outer protobuf/
// pubsub path (that is shape B); (ii) a sender keeps its OWN produced value locally by
// design, so the round-trip coverage is on cross-runner (peer) messages.
//
// VERIFICATION ORACLE: the engine self-verifies the aggregate internally, so a non-error
// signature is a valid BIP-340 signature over the message (an existing low-level cgo test
// externally verifies the aggregate crypto; this test does not widen the engine surface
// with a keyGroup->pubkey accessor just to re-verify). The assertions here add that every
// seat obtains the SAME non-empty 64-byte signature and reaches Succeeded.

// realCgoNetSigningHarness wires n interactive signing runners over n real pkg/net runner
// buses against ONE real cgo engine.
type realCgoNetSigningHarness struct {
	runners []*interactiveSigningRunner
	coords  []roast.Coordinator
	handles []roast.AttemptHandle
}

// buildRealCgoNetHarness builds an n-seat interactive signing round over the real pkg/net
// transport, driving the real cgo engine. It runs a real DKG up front (skip-guarded: an
// absent/stale libfrost_tbtc skips the test there), then binds every seat's attempt
// context to the DKG key group so the Go RFC-21 coordinator election and the engine's own
// derivation agree.
func buildRealCgoNetHarness(
	t *testing.T,
	ctx context.Context,
	engine *buildTaggedTBTCSignerEngine,
	sessionID string,
	n int,
	threshold uint16,
) realCgoNetSigningHarness {
	t.Helper()

	participantIDs := make([]byte, 0, n)
	included := make([]group.MemberIndex, 0, n)
	for i := 1; i <= n; i++ {
		participantIDs = append(participantIDs, byte(i))
		included = append(included, group.MemberIndex(i))
	}

	// 1. Real DKG that persists a key group under sessionID. The returned handle string
	// is BOTH the id the engine resolves on InteractiveSessionOpen AND the seed material:
	// the engine derives the coordinator-shuffle seed as SHA256(keyGroup || sessionID ||
	// messageDigest), and keep-core's attempt.DeriveAttemptSeed hashes its
	// dkgGroupPublicKey argument identically - so passing []byte(keyGroup) as the
	// dkgGroupPublicKey makes the two independent derivations agree (the runner fails
	// closed if they do not).
	keyGroup := runRealCgoDKGKeyGroup(t, engine, sessionID, participantIDs, threshold)
	keyGroupSeed := []byte(keyGroup)

	var messageDigest [attempt.MessageDigestLength]byte
	for i := range messageDigest {
		messageDigest[i] = 0x42
	}

	attemptCtx, err := attempt.NewAttemptContext(
		sessionID, keyGroup, keyGroupSeed, messageDigest, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}

	// One operator per seat; the MembershipValidator maps each seat to that operator's
	// address so the bus authenticates every broadcast's claimed seat against the
	// authenticated sender key (same wiring as the fake-engine net harness).
	chainSigning := local_v1.Connect(n, n).Signing()
	publicKeys := make([]*operator.PublicKey, n)
	addresses := make([]chain.Address, n)
	for i := 0; i < n; i++ {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (seat %d): %v", i+1, err)
		}
		publicKeys[i] = publicKey
		addresses[i] = chainSigning.PublicKeyBytesToAddress(
			operator.MarshalUncompressed(publicKey),
		)
	}
	validator := group.NewMembershipValidator(&testutils.MockLogger{}, addresses, chainSigning)

	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	logger := &testutils.MockLogger{}

	h := realCgoNetSigningHarness{}
	for i := 0; i < n; i++ {
		member := group.MemberIndex(i + 1)

		channel, err := netlocal.ConnectWithKey(publicKeys[i]).
			BroadcastChannelFor("frost-roast-interactive-signing")
		if err != nil {
			t.Fatalf("broadcast channel (seat %d): %v", member, err)
		}
		bus, err := NewBroadcastChannelRunnerBus(ctx, logger, channel, validator)
		if err != nil {
			t.Fatalf("runner bus (seat %d): %v", member, err)
		}

		coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
		handle, err := coord.BeginAttempt(attemptCtx)
		if err != nil {
			t.Fatalf("begin attempt (seat %d): %v", member, err)
		}
		ara, err := NewActiveRoastAttempt(coord, handle, attemptCtx, sessionID, nil, keyGroupSeed)
		if err != nil {
			t.Fatalf("active attempt (seat %d): %v", member, err)
		}
		collector := roast.NewRound2Collector(verifier)
		// SAME engine instance for every seat - the multi-seat path keys interactive
		// state by member, so one engine serves all local seats.
		runner, err := newInteractiveSigningRunner(
			ara, member, threshold, nil, engine, collector, coord, signer, bus,
		)
		if err != nil {
			t.Fatalf("runner (seat %d): %v", member, err)
		}

		h.coords = append(h.coords, coord)
		h.handles = append(h.handles, handle)
		h.runners = append(h.runners, runner)
	}
	return h
}

// runAllAndAssertRealSignature runs every seat's runner concurrently against the shared
// engine and asserts every co-resident seat obtains the SAME real BIP-340 signature.
//
// All seats share ONE process-global engine session AND the runner's per-(session,attempt)
// aggregate memo (aggregateInteractiveOnce). The first seat to reach InteractiveAggregate
// runs the single real engine aggregate and marks the attempt finalized; every sibling seat
// then receives that same result from the memo instead of hitting the engine's per-attempt
// idempotency marker. So NO seat surfaces interactive_attempt_already_aggregated any more
// (the memo intercepts the co-resident collision) - all seats reach Succeeded with the
// identical signature. In production each node has its own engine and aggregates the shares
// it collected independently; the memo only dedups the co-resident seats of ONE operator.
// Crucially, every seat still drives its FULL transport (broadcasting its commitments and
// share, and collecting peers' commitments/package/shares over the real pkg/net bus) BEFORE
// the aggregate step - so the transport seam this test exists for is exercised by all seats.
//
// So the assertion is: EVERY seat aggregates the same real 64-byte BIP-340 signature and
// reaches Succeeded, and no seat fails for any reason (an aggregate error - including the
// idempotency marker - would fail the test, asserting the memo prevented the collision).
func (h realCgoNetSigningHarness) runAllAndAssertRealSignature(t *testing.T, ctx context.Context) {
	t.Helper()
	sigs := make([][]byte, len(h.runners))
	errs := make([]error, len(h.runners))
	var wg sync.WaitGroup
	for i := range h.runners {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sigs[idx], errs[idx] = h.runners[idx].Run(ctx)
		}(i)
	}
	wg.Wait()

	var sharedSignature []byte
	for i := range h.runners {
		member := i + 1
		// With the per-(session,attempt) aggregate memo, the first local seat to reach
		// step 9 runs the single real engine aggregate and every co-resident seat
		// receives that same result, so ALL seats succeed with the identical BIP-340
		// signature and none surfaces interactive_attempt_already_aggregated. An
		// aggregate error here (idempotency marker or otherwise) fails the test,
		// asserting the memo intercepted the shared-engine collision.
		if errs[i] != nil {
			t.Fatalf("seat %d run failed: %v", member, errs[i])
		}
		if len(sigs[i]) != 64 {
			t.Fatalf("seat %d: expected a 64-byte BIP-340 signature, got %d bytes", member, len(sigs[i]))
		}
		if sharedSignature == nil {
			sharedSignature = sigs[i]
		} else if !bytes.Equal(sigs[i], sharedSignature) {
			t.Fatalf("seat %d produced a different signature than another aggregating seat", member)
		}
		state, err := h.coords[i].State(h.handles[i])
		if err != nil {
			t.Fatalf("seat %d state: %v", member, err)
		}
		if state != roast.AttemptStateSucceeded {
			t.Fatalf("seat %d aggregated but did not reach Succeeded, got %v", member, state)
		}
	}
}

// TestRealCgoInteractiveSigning_NetTransport_FullIncludedRound runs a complete interactive
// signing round over the REAL pkg/net transport against the REAL cgo engine with a
// full-included attempt (group size == threshold == 2, every seat signs) - the real-crypto
// counterpart of TestInteractiveSigningRunner_NetTransport_FullIncludedRound. It proves the
// real engine's commitments / signing package / shares serialize over the production
// transport adapter and aggregate to a real BIP-340 signature.
func TestRealCgoInteractiveSigning_NetTransport_FullIncludedRound(t *testing.T) {
	setupRealCgoSignerState(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-multinode-full-%d", realCgoSessionSeq.Add(1))
	buildRealCgoNetHarness(t, ctx, engine, sessionID, 2, 2).
		runAllAndAssertRealSignature(t, ctx)
}

// TestRealCgoInteractiveSigning_NetTransport_ThresholdSubsetRound runs the round over the
// real transport against the real engine with an oversized included set (group size 3,
// threshold 2): the coordinator finalizes over a t-subset and the remaining committed seat
// is an observer (RFC-21 Phase 7.3 t-of-included). Every seat still obtains the same
// signature and reaches Succeeded, proving the subset/observer flow works with real crypto
// across the production transport.
func TestRealCgoInteractiveSigning_NetTransport_ThresholdSubsetRound(t *testing.T) {
	setupRealCgoSignerState(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-multinode-subset-%d", realCgoSessionSeq.Add(1))
	buildRealCgoNetHarness(t, ctx, engine, sessionID, 3, 2).
		runAllAndAssertRealSignature(t, ctx)
}
