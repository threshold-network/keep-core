//go:build frost_native

package signing

import (
	"context"
	"fmt"
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
	engines     []*fakeInteractiveSigningEngine
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
		engine := newFakeInteractiveSigningEngine()
		// The engine derivation must agree with the binding's RFC-21 election, or
		// the runner's cross-check fails closed (a real engine derives the same
		// coordinator the binding did).
		engine.coordinatorIdentifier = uint16(ara.ElectedCoordinator())
		runner, err := newInteractiveSigningRunner(
			ara, member, threshold,
			engine,
			collector,
			coord, signer, bus,
		)
		if err != nil {
			t.Fatalf("runner (member %d): %v", member, err)
		}
		h.coords = append(h.coords, coord)
		h.collectors = append(h.collectors, collector)
		h.engines = append(h.engines, engine)
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

// A FAILED attempt must NOT be pruned by the runner: the retained signed
// evidence is what the blame/retry path reads (CoordinatorPackageProofs /
// ClassifyCandidateCulprits). Force aggregation to fail after every node has
// recorded its package and shares, then assert each collector still holds the
// attempt - a conflicting re-begin is rejected, whereas a pruned collector would
// accept it.
func TestInteractiveSigningRunner_PreservesEvidenceOnAggregateFailure(t *testing.T) {
	h := buildInteractiveSigningHarness(t, 3, 2)
	for _, e := range h.engines {
		e.aggregateErr = fmt.Errorf("aggregate share verification failed")
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	errs := make([]error, len(h.runners))
	var wg sync.WaitGroup
	for i := range h.runners {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = h.runners[idx].Run(runCtx)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err == nil {
			t.Fatalf("member %d: expected aggregate failure", i+1)
		}
	}

	// Retained evidence must survive the failure (not pruned): a surviving record
	// makes a re-begin under a DIFFERENT binding conflict.
	elected := h.runners[0].attempt.ElectedCoordinator()
	var differentElected group.MemberIndex
	for _, m := range h.includedSet {
		if m != elected {
			differentElected = m
			break
		}
	}
	for i, collector := range h.collectors {
		if err := collector.BeginAttempt(h.contextHash[:], differentElected, h.includedSet); err == nil {
			t.Fatalf("member %d: evidence pruned on failure (conflicting re-begin unexpectedly succeeded)", i+1)
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
	engine.coordinatorIdentifier = uint16(ara.ElectedCoordinator())
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

// The runner cross-checks the engine-derived coordinator against the binding's
// own RFC-21 election and fails closed on divergence - BEFORE opening a session -
// so it can never sign an attempt bound to the wrong coordinator.
func TestInteractiveSigningRunner_RejectsEngineCoordinatorMismatch(t *testing.T) {
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
	// Engine derives a coordinator the binding did NOT elect.
	engine.coordinatorIdentifier = uint16(ara.ElectedCoordinator()) + 100
	runner, err := newInteractiveSigningRunner(
		ara, 1, 2, engine, roast.NewRound2Collector(verifier), coord, signer, bus,
	)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runner.Run(runCtx); err == nil {
		t.Fatal("expected the engine/binding coordinator mismatch to be rejected")
	}
	// The mismatch is caught at the derive cross-check, before the session opens.
	if engine.openCalls != 0 {
		t.Fatalf("expected no session open on coordinator mismatch, got %d", engine.openCalls)
	}
}

// The happy path must key signing-package commitments and aggregate shares by the
// engine-derived FROST identifiers, not a Go-fabricated placeholder.
func TestInteractiveSigningRunner_UsesEngineDerivedFrostIdentifiers(t *testing.T) {
	h := buildInteractiveSigningHarness(t, 3, 2)
	h.runAndAssertAllSucceed(t)

	got := map[string]struct{}{}
	for _, share := range h.engines[0].lastAggregateShares {
		got[share.Identifier] = struct{}{}
	}
	for _, want := range []string{"frost-id-1", "frost-id-2", "frost-id-3"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("aggregate missing engine-derived identifier %q; got %v", want, got)
		}
	}
}

func TestSameMemberSet(t *testing.T) {
	cases := map[string]struct {
		derived  []uint16
		included []group.MemberIndex
		want     bool
	}{
		"equal, reordered":        {[]uint16{3, 1, 2}, []group.MemberIndex{1, 2, 3}, true},
		"different length":        {[]uint16{1, 2}, []group.MemberIndex{1, 2, 3}, false},
		"foreign member":          {[]uint16{1, 9}, []group.MemberIndex{1, 2}, false},
		"duplicate masks missing": {[]uint16{1, 1}, []group.MemberIndex{1, 2}, false},
		"empty equal":             {[]uint16{}, []group.MemberIndex{}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := sameMemberSet(tc.derived, tc.included); got != tc.want {
				t.Fatalf("sameMemberSet(%v, %v) = %v, want %v", tc.derived, tc.included, got, tc.want)
			}
		})
	}
}

// captureEquivocationEvidence registers a process-wide observer recording every
// emitted equivocation event for the test's duration, returning a snapshot
// accessor. Only one observer may be registered process-wide, so tests using it
// must not run in parallel.
func captureEquivocationEvidence(t *testing.T) func() []roast.EquivocationEvidence {
	t.Helper()
	var mu sync.Mutex
	var captured []roast.EquivocationEvidence
	if err := roast.RegisterEquivocationEvidenceObserver(func(ev roast.EquivocationEvidence) {
		mu.Lock()
		captured = append(captured, ev)
		mu.Unlock()
	}); err != nil {
		t.Fatalf("register equivocation observer: %v", err)
	}
	t.Cleanup(roast.UnregisterEquivocationEvidenceObserver)
	return func() []roast.EquivocationEvidence {
		mu.Lock()
		defer mu.Unlock()
		return append([]roast.EquivocationEvidence(nil), captured...)
	}
}

// craftSigningPackage builds a coordinator-signed signing-package envelope (and
// returns its body hash) over the given FROST package body.
func craftSigningPackage(
	t *testing.T,
	contextHash [attempt.MessageDigestLength]byte,
	elected group.MemberIndex,
	body []byte,
	signer roast.Signer,
) ([]byte, [32]byte) {
	t.Helper()
	pkg := &roast.SigningPackage{
		AttemptContextHash:  append([]byte(nil), contextHash[:]...),
		CoordinatorIDValue:  uint32(elected),
		SigningPackageBytes: append([]byte(nil), body...),
	}
	payload, err := pkg.SignableBytes()
	if err != nil {
		t.Fatalf("signing package signable bytes: %v", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign signing package: %v", err)
	}
	pkg.CoordinatorSignature = sig
	envelope, err := pkg.Marshal()
	if err != nil {
		t.Fatalf("marshal signing package: %v", err)
	}
	bodyHash, err := pkg.BodyHash()
	if err != nil {
		t.Fatalf("signing package body hash: %v", err)
	}
	return envelope, bodyHash
}

// craftShareSubmission builds a submitter-signed accepted-share envelope for a
// member, binding the elected coordinator and authoritative package body hash so
// the collector accepts it (a body-different share for the same member is then
// member equivocation).
func craftShareSubmission(
	t *testing.T,
	contextHash [attempt.MessageDigestLength]byte,
	member, elected group.MemberIndex,
	pkgBodyHash [32]byte,
	share []byte,
	signer roast.Signer,
) []byte {
	t.Helper()
	sub := &roast.ShareSubmission{
		AttemptContextHash: append([]byte(nil), contextHash[:]...),
		SubmitterIDValue:   uint32(member),
		CoordinatorIDValue: uint32(elected),
		SigningPackageHash: append([]byte(nil), pkgBodyHash[:]...),
		SignatureShare:     append([]byte(nil), share...),
	}
	payload, err := sub.SignableBytes()
	if err != nil {
		t.Fatalf("share submission signable bytes: %v", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign share submission: %v", err)
	}
	sub.SubmitterSignature = sig
	envelope, err := sub.Marshal()
	if err != nil {
		t.Fatalf("marshal share submission: %v", err)
	}
	return envelope
}

// buildEquivocationRunner builds a single runner with a fresh collector for the
// evidence-retention tests, returning the runner, its collector, the attempt
// context hash, and the elected coordinator.
func buildEquivocationRunner(t *testing.T, included []group.MemberIndex) (
	*interactiveSigningRunner, *roast.Round2Collector, [attempt.MessageDigestLength]byte, group.MemberIndex,
) {
	t.Helper()
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
	bus := NewInProcessRunnerBus(16)
	coord := roast.NewInMemoryCoordinatorWithSigning(included[0], signer, verifier)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	ara, err := NewActiveRoastAttempt(coord, handle, ctx, "session-1", nil, dkgKey)
	if err != nil {
		t.Fatalf("active attempt: %v", err)
	}
	collector := roast.NewRound2Collector(verifier)
	runner, err := newInteractiveSigningRunner(
		ara, included[0], 2, newFakeInteractiveSigningEngine(), collector, coord, signer, bus,
	)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	return runner, collector, ctx.Hash(), ara.ElectedCoordinator()
}

// A second, body-different package from the elected coordinator must be recorded
// as coordinator equivocation, not dropped because obtainSigningPackage already
// returned on the first one.
func TestInteractiveSigningRunner_RetainsCoordinatorPackageEquivocation(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	runner, collector, contextHash, elected := buildEquivocationRunner(t, included)
	signer := fixedTestSigner{}

	if err := collector.BeginAttempt(contextHash[:], elected, included); err != nil {
		t.Fatalf("collector begin: %v", err)
	}
	// The authoritative package is recorded first (as Run does).
	authEnvelope, _ := craftSigningPackage(t, contextHash, elected, []byte("authoritative-package"), signer)
	authPkg := &roast.SigningPackage{}
	if err := authPkg.Unmarshal(authEnvelope); err != nil {
		t.Fatalf("unmarshal authoritative: %v", err)
	}
	if err := collector.RecordSigningPackage(authPkg); err != nil {
		t.Fatalf("record authoritative: %v", err)
	}

	// A body-different package the coordinator also broadcast sits buffered.
	conflictEnvelope, _ := craftSigningPackage(t, contextHash, elected, []byte("equivocating-package"), signer)
	stream := make(chan RunnerMessage, 4)
	stream <- RunnerMessage{Type: RunnerMsgSigningPackage, Sender: elected, Attempt: contextHash, Payload: conflictEnvelope}

	evidence := captureEquivocationEvidence(t)
	runner.recordBufferedCoordinatorPackages(stream, elected, contextHash)

	got := evidence()
	if len(got) != 1 || got[0].Kind != roast.EquivocationKindSigningPackageConflict || got[0].Sender != elected {
		t.Fatalf("expected one signing-package conflict from member %d, got %+v", elected, got)
	}
}

// A member that double-signs (a body-different second accepted share) must be
// recorded as equivocation even after its first share was already counted -
// collectShares must not drop the later envelope before the collector sees it.
func TestInteractiveSigningRunner_RetainsMemberShareEquivocation(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	runner, collector, contextHash, elected := buildEquivocationRunner(t, included)
	signer := fixedTestSigner{}

	if err := collector.BeginAttempt(contextHash[:], elected, included); err != nil {
		t.Fatalf("collector begin: %v", err)
	}
	authEnvelope, pkgBodyHash := craftSigningPackage(t, contextHash, elected, []byte("authoritative-package"), signer)
	authPkg := &roast.SigningPackage{}
	if err := authPkg.Unmarshal(authEnvelope); err != nil {
		t.Fatalf("unmarshal authoritative: %v", err)
	}
	if err := collector.RecordSigningPackage(authPkg); err != nil {
		t.Fatalf("record authoritative: %v", err)
	}

	// Member 1 double-signs; member 2 sends one share. Ordered so member 1's
	// first share is counted before its conflicting second arrives.
	share1a := craftShareSubmission(t, contextHash, 1, elected, pkgBodyHash, []byte("share-1-a"), signer)
	share1b := craftShareSubmission(t, contextHash, 1, elected, pkgBodyHash, []byte("share-1-b"), signer)
	share2 := craftShareSubmission(t, contextHash, 2, elected, pkgBodyHash, []byte("share-2"), signer)
	stream := make(chan RunnerMessage, 8)
	stream <- RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 1, Attempt: contextHash, Payload: share1a}
	stream <- RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 1, Attempt: contextHash, Payload: share1b}
	stream <- RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 2, Attempt: contextHash, Payload: share2}

	evidence := captureEquivocationEvidence(t)
	into := map[group.MemberIndex][]byte{}
	if err := runner.collectShares(context.Background(), stream, contextHash, included, into); err != nil {
		t.Fatalf("collect shares: %v", err)
	}

	// The first accepted share per member is counted; the double-sign is retained.
	if string(into[1]) != "share-1-a" || string(into[2]) != "share-2" {
		t.Fatalf("unexpected counted shares: 1=%q 2=%q", into[1], into[2])
	}
	got := evidence()
	if len(got) != 1 || got[0].Kind != roast.EquivocationKindShareConflict || got[0].Sender != 1 {
		t.Fatalf("expected one share conflict from member 1, got %+v", got)
	}
}

// A body-different duplicate queued BEHIND the share that fills the final slot
// must still be retained: collectShares stops counting once `into` is full, so
// it must drain the remaining buffered shares before Run aggregates and prunes.
func TestInteractiveSigningRunner_RetainsQueuedShareEquivocationAfterCollection(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	runner, collector, contextHash, elected := buildEquivocationRunner(t, included)
	signer := fixedTestSigner{}

	if err := collector.BeginAttempt(contextHash[:], elected, included); err != nil {
		t.Fatalf("collector begin: %v", err)
	}
	authEnvelope, pkgBodyHash := craftSigningPackage(t, contextHash, elected, []byte("authoritative-package"), signer)
	authPkg := &roast.SigningPackage{}
	if err := authPkg.Unmarshal(authEnvelope); err != nil {
		t.Fatalf("unmarshal authoritative: %v", err)
	}
	if err := collector.RecordSigningPackage(authPkg); err != nil {
		t.Fatalf("record authoritative: %v", err)
	}

	// Member 2's first share fills the final slot; its body-different duplicate is
	// queued right behind it, so the collection loop exits before reading it and
	// only the post-completion drain can retain it.
	share1 := craftShareSubmission(t, contextHash, 1, elected, pkgBodyHash, []byte("share-1"), signer)
	share2a := craftShareSubmission(t, contextHash, 2, elected, pkgBodyHash, []byte("share-2-a"), signer)
	share2b := craftShareSubmission(t, contextHash, 2, elected, pkgBodyHash, []byte("share-2-b"), signer)
	stream := make(chan RunnerMessage, 8)
	stream <- RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 1, Attempt: contextHash, Payload: share1}
	stream <- RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 2, Attempt: contextHash, Payload: share2a}
	stream <- RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 2, Attempt: contextHash, Payload: share2b}

	evidence := captureEquivocationEvidence(t)
	into := map[group.MemberIndex][]byte{}
	if err := runner.collectShares(context.Background(), stream, contextHash, included, into); err != nil {
		t.Fatalf("collect shares: %v", err)
	}

	if string(into[1]) != "share-1" || string(into[2]) != "share-2-a" {
		t.Fatalf("unexpected counted shares: 1=%q 2=%q", into[1], into[2])
	}
	got := evidence()
	if len(got) != 1 || got[0].Kind != roast.EquivocationKindShareConflict || got[0].Sender != 2 {
		t.Fatalf("expected one queued share conflict from member 2, got %+v", got)
	}
}

// The post-completion drain must be bounded: a peer that keeps the share stream
// non-empty (flooding body-different shares) must not livelock collectShares
// once enough valid shares are already collected. With `into` pre-filled only
// the drain runs, and it must return promptly despite the continuous flood.
func TestInteractiveSigningRunner_DrainDoesNotLivelockUnderShareFlood(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	runner, collector, contextHash, elected := buildEquivocationRunner(t, included)
	signer := fixedTestSigner{}
	if err := collector.BeginAttempt(contextHash[:], elected, included); err != nil {
		t.Fatalf("collector begin: %v", err)
	}
	authEnvelope, pkgBodyHash := craftSigningPackage(t, contextHash, elected, []byte("authoritative-package"), signer)
	authPkg := &roast.SigningPackage{}
	if err := authPkg.Unmarshal(authEnvelope); err != nil {
		t.Fatalf("unmarshal authoritative: %v", err)
	}
	if err := collector.RecordSigningPackage(authPkg); err != nil {
		t.Fatalf("record authoritative: %v", err)
	}

	floodEnvelope := craftShareSubmission(t, contextHash, 2, elected, pkgBodyHash, []byte("flood"), signer)
	floodMsg := RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 2, Attempt: contextHash, Payload: floodEnvelope}
	stream := make(chan RunnerMessage, 8)
	for i := 0; i < cap(stream); i++ {
		stream <- floodMsg // full at entry, so the bound is actually exercised
	}
	stop := make(chan struct{})
	floodDone := make(chan struct{})
	go func() {
		defer close(floodDone)
		for {
			select {
			case <-stop:
				return
			case stream <- floodMsg: // keep the stream non-empty as the drain reads
			}
		}
	}()

	// `into` already complete -> the collection loop is skipped; only the drain runs.
	into := map[group.MemberIndex][]byte{1: []byte("a"), 2: []byte("b")}
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		_ = runner.collectShares(context.Background(), stream, contextHash, included, into)
	}()
	select {
	case <-returned:
		// Bounded drain returned despite the flood.
	case <-time.After(2 * time.Second):
		t.Fatal("collectShares drain livelocked under share flood")
	}
	close(stop)
	<-floodDone
}

// beginSyntheticAttempt makes a second attempt live in the SAME collector, keyed
// by a synthetic context hash with the given elected coordinator, and records an
// authoritative package for it - so a cross-attempt test can prove a payload
// signed for THIS attempt would be accepted here absent the signed-hash guard.
// Returns that attempt's authoritative package body hash.
func beginSyntheticAttempt(
	t *testing.T,
	collector *roast.Round2Collector,
	hash [attempt.MessageDigestLength]byte,
	elected group.MemberIndex,
	included []group.MemberIndex,
	signer roast.Signer,
) [32]byte {
	t.Helper()
	if err := collector.BeginAttempt(hash[:], elected, included); err != nil {
		t.Fatalf("begin synthetic attempt: %v", err)
	}
	envelope, bodyHash := craftSigningPackage(t, hash, elected, []byte("authoritative-B"), signer)
	pkg := &roast.SigningPackage{}
	if err := pkg.Unmarshal(envelope); err != nil {
		t.Fatalf("unmarshal synthetic authoritative: %v", err)
	}
	if err := collector.RecordSigningPackage(pkg); err != nil {
		t.Fatalf("record synthetic authoritative: %v", err)
	}
	return bodyHash
}

func recordAuthoritativePackage(
	t *testing.T,
	collector *roast.Round2Collector,
	hash [attempt.MessageDigestLength]byte,
	elected group.MemberIndex,
	signer roast.Signer,
) {
	t.Helper()
	envelope, _ := craftSigningPackage(t, hash, elected, []byte("authoritative-A"), signer)
	pkg := &roast.SigningPackage{}
	if err := pkg.Unmarshal(envelope); err != nil {
		t.Fatalf("unmarshal authoritative: %v", err)
	}
	if err := collector.RecordSigningPackage(pkg); err != nil {
		t.Fatalf("record authoritative: %v", err)
	}
}

// A share carried in a current-attempt (A) bus message but SIGNED for another
// live attempt (B) must not be counted toward A - the runner must check the
// signed AttemptContextHash, not the unsigned outer bus field. Without the
// guard, the collector records it under B (accepted) and returns nil, so this
// code would count it toward A.
func TestInteractiveSigningRunner_RejectsCrossAttemptShare(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	runner, collector, hashA, electedA := buildEquivocationRunner(t, included)
	signer := fixedTestSigner{}
	if err := collector.BeginAttempt(hashA[:], electedA, included); err != nil {
		t.Fatalf("collector begin A: %v", err)
	}
	recordAuthoritativePackage(t, collector, hashA, electedA, signer)

	// Attempt B live in the same collector (electedB == electedA so an electedA-
	// bound payload is acceptable there).
	hashB := [attempt.MessageDigestLength]byte{0x99}
	pkgBodyHashB := beginSyntheticAttempt(t, collector, hashB, electedA, included, signer)

	shareForB := craftShareSubmission(t, hashB, 1, electedA, pkgBodyHashB, []byte("share-for-B"), signer)
	msg := RunnerMessage{Type: RunnerMsgShareSubmission, Sender: 1, Attempt: hashA, Payload: shareForB}

	into := map[group.MemberIndex][]byte{}
	runner.recordShareMessage(msg, hashA, setOf(included), into)
	if _, counted := into[1]; counted {
		t.Fatal("share signed for attempt B was counted toward attempt A")
	}
}

// A package carried in a current-attempt (A) bus message but SIGNED for another
// live attempt (B) must not be recorded by A's buffered-package drain. Without
// the guard the drain records it under B (a body-different conflict there),
// emitting a B equivocation event A had no business producing.
func TestInteractiveSigningRunner_RejectsCrossAttemptPackageInDrain(t *testing.T) {
	included := []group.MemberIndex{1, 2}
	runner, collector, hashA, electedA := buildEquivocationRunner(t, included)
	signer := fixedTestSigner{}
	if err := collector.BeginAttempt(hashA[:], electedA, included); err != nil {
		t.Fatalf("collector begin A: %v", err)
	}
	recordAuthoritativePackage(t, collector, hashA, electedA, signer)

	hashB := [attempt.MessageDigestLength]byte{0x99}
	_ = beginSyntheticAttempt(t, collector, hashB, electedA, included, signer)

	// A body-different package signed for B, rewrapped as a current-attempt (A)
	// message from electedA.
	crossEnvelope, _ := craftSigningPackage(t, hashB, electedA, []byte("equivocating-B"), signer)
	stream := make(chan RunnerMessage, 4)
	stream <- RunnerMessage{Type: RunnerMsgSigningPackage, Sender: electedA, Attempt: hashA, Payload: crossEnvelope}

	evidence := captureEquivocationEvidence(t)
	runner.recordBufferedCoordinatorPackages(stream, electedA, hashA)
	if got := evidence(); len(got) != 0 {
		t.Fatalf("package signed for attempt B was recorded under it via A's drain: %+v", got)
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
