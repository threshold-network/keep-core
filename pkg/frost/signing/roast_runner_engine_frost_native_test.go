//go:build frost_native

package signing

import (
	"fmt"
	"sync"
	"testing"
)

// Compile-time assertion that the fake satisfies the runner's engine boundary.
var _ interactiveSigningEngine = (*fakeInteractiveSigningEngine)(nil)

// fakeInteractiveSigningEngine is a programmable interactiveSigningEngine for
// runner tests. It returns configured (non-crypto) responses and records call
// counts so the runner's orchestration is exercised deterministically without
// cgo or real FROST - the real engine's crypto is covered by the engine's own
// suite. Per-member commitments/shares default to member-derived bytes so each
// member's contribution is distinct; the aggregate result (or error) is
// programmable for the happy and, later, the sad path.
type fakeInteractiveSigningEngine struct {
	attemptID      string
	idempotent     bool
	signingPackage []byte
	signature      []byte
	aggregateErr   error

	commitmentsByMember map[uint16][]byte
	shareByMember       map[uint16][]byte

	mu                  sync.Mutex
	openCalls           int
	round1Calls         int
	newPackageCalls     int
	round2Calls         int
	aggregateCalls      int
	abortCalls          int
	lastAggregateShares []nativeFROSTSignatureShare
}

func (f *fakeInteractiveSigningEngine) InteractiveSessionAbort(
	sessionID string,
	attemptID *string,
) (*NativeInteractiveSessionAbortResult, error) {
	f.mu.Lock()
	f.abortCalls++
	f.mu.Unlock()
	return &NativeInteractiveSessionAbortResult{SessionID: sessionID, Aborted: true}, nil
}

func (f *fakeInteractiveSigningEngine) abortCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.abortCalls
}

func newFakeInteractiveSigningEngine() *fakeInteractiveSigningEngine {
	return &fakeInteractiveSigningEngine{
		attemptID:           "attempt-1",
		signingPackage:      []byte("fake-signing-package"),
		signature:           []byte("fake-bip340-signature"),
		commitmentsByMember: map[uint16][]byte{},
		shareByMember:       map[uint16][]byte{},
	}
}

func (f *fakeInteractiveSigningEngine) memberCommitments(member uint16) []byte {
	if c, ok := f.commitmentsByMember[member]; ok {
		return c
	}
	return []byte(fmt.Sprintf("commitments-%d", member))
}

func (f *fakeInteractiveSigningEngine) memberShare(member uint16) []byte {
	if s, ok := f.shareByMember[member]; ok {
		return s
	}
	return []byte(fmt.Sprintf("share-%d", member))
}

func (f *fakeInteractiveSigningEngine) InteractiveSessionOpen(
	sessionID string,
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	threshold uint16,
	taprootMerkleRoot *[32]byte,
	attemptContext NativeInteractiveAttemptContext,
) (*NativeInteractiveSessionOpenResult, error) {
	f.mu.Lock()
	f.openCalls++
	f.mu.Unlock()
	return &NativeInteractiveSessionOpenResult{
		SessionID:  sessionID,
		AttemptID:  f.attemptID,
		Idempotent: f.idempotent,
	}, nil
}

func (f *fakeInteractiveSigningEngine) InteractiveRound1(
	sessionID string,
	attemptID string,
	memberIdentifier uint16,
) ([]byte, error) {
	f.mu.Lock()
	f.round1Calls++
	f.mu.Unlock()
	return f.memberCommitments(memberIdentifier), nil
}

func (f *fakeInteractiveSigningEngine) NewSigningPackage(
	message []byte,
	commitments []nativeFROSTCommitment,
) ([]byte, error) {
	f.mu.Lock()
	f.newPackageCalls++
	f.mu.Unlock()
	return f.signingPackage, nil
}

func (f *fakeInteractiveSigningEngine) InteractiveRound2(
	sessionID string,
	attemptID string,
	memberIdentifier uint16,
	signingPackage []byte,
) ([]byte, error) {
	f.mu.Lock()
	f.round2Calls++
	f.mu.Unlock()
	return f.memberShare(memberIdentifier), nil
}

func (f *fakeInteractiveSigningEngine) InteractiveAggregate(
	sessionID string,
	attemptID string,
	signingPackage []byte,
	signatureShares []nativeFROSTSignatureShare,
	taprootMerkleRoot *[32]byte,
) ([]byte, error) {
	f.mu.Lock()
	f.aggregateCalls++
	f.lastAggregateShares = signatureShares
	f.mu.Unlock()
	if f.aggregateErr != nil {
		return nil, f.aggregateErr
	}
	return f.signature, nil
}

func TestFakeInteractiveSigningEngine_Programmable(t *testing.T) {
	engine := newFakeInteractiveSigningEngine()

	open, err := engine.InteractiveSessionOpen("s", 1, []byte("m"), "kg", 2, nil, NativeInteractiveAttemptContext{})
	if err != nil || open.AttemptID != "attempt-1" {
		t.Fatalf("unexpected open result: %+v err=%v", open, err)
	}

	// Per-member round-1/round-2 bytes are distinct by default.
	c1, _ := engine.InteractiveRound1("s", "a", 1)
	c2, _ := engine.InteractiveRound1("s", "a", 2)
	if string(c1) == string(c2) {
		t.Fatal("expected distinct per-member commitments")
	}

	pkg, _ := engine.NewSigningPackage([]byte("m"), nil)
	if string(pkg) != "fake-signing-package" {
		t.Fatalf("unexpected signing package: %s", pkg)
	}

	sig, err := engine.InteractiveAggregate("s", "a", pkg, nil, nil)
	if err != nil || string(sig) != "fake-bip340-signature" {
		t.Fatalf("unexpected aggregate result: %s err=%v", sig, err)
	}

	// A programmed aggregate error surfaces (the later sad path uses this).
	engine.aggregateErr = fmt.Errorf("boom")
	if _, err := engine.InteractiveAggregate("s", "a", pkg, nil, nil); err == nil {
		t.Fatal("expected the programmed aggregate error")
	}

	if engine.openCalls != 1 || engine.round1Calls != 2 || engine.newPackageCalls != 1 || engine.aggregateCalls != 2 {
		t.Fatalf("unexpected call counts: %+v", engine)
	}
}
