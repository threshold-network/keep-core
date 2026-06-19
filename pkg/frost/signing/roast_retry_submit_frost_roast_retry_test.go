//go:build frost_roast_retry

package signing

import (
	"errors"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// captureCoordinator is a roast.Coordinator wrapper that records
// every RecordEvidence call so tests can assert what was submitted.
// It delegates everything else to an embedded real coordinator.
type captureCoordinator struct {
	inner       roast.Coordinator
	mu          sync.Mutex
	recordedFor []roast.AttemptHandle
	recordedSnp []*roast.LocalEvidenceSnapshot
	recordErr   error
}

func newCaptureCoordinator(inner roast.Coordinator) *captureCoordinator {
	return &captureCoordinator{inner: inner}
}

func (c *captureCoordinator) BeginAttempt(ctx attempt.AttemptContext) (roast.AttemptHandle, error) {
	return c.inner.BeginAttempt(ctx)
}
func (c *captureCoordinator) State(h roast.AttemptHandle) (roast.AttemptState, error) {
	return c.inner.State(h)
}
func (c *captureCoordinator) SelectedCoordinator(h roast.AttemptHandle) (group.MemberIndex, error) {
	return c.inner.SelectedCoordinator(h)
}
func (c *captureCoordinator) RecordEvidence(h roast.AttemptHandle, s *roast.LocalEvidenceSnapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recordErr != nil {
		return c.recordErr
	}
	c.recordedFor = append(c.recordedFor, h)
	c.recordedSnp = append(c.recordedSnp, s)
	return c.inner.RecordEvidence(h, s)
}
func (c *captureCoordinator) AggregateBundle(h roast.AttemptHandle) (*roast.TransitionMessage, error) {
	return c.inner.AggregateBundle(h)
}
func (c *captureCoordinator) MarkSucceeded(h roast.AttemptHandle) error {
	return c.inner.MarkSucceeded(h)
}
func (c *captureCoordinator) VerifyBundle(h roast.AttemptHandle, m *roast.TransitionMessage) error {
	return c.inner.VerifyBundle(h, m)
}
func (c *captureCoordinator) NextAttempt(
	h roast.AttemptHandle, m *roast.TransitionMessage, t uint, pk []byte,
) (attempt.AttemptContext, error) {
	return c.inner.NextAttempt(h, m, t, pk)
}

// deterministicSigner produces SHA256(memberID || payload)-style
// signatures the captureSignatureVerifier accepts.
type deterministicSigner struct {
	id group.MemberIndex
}

func (d *deterministicSigner) Sign(payload []byte) ([]byte, error) {
	out := make([]byte, len(payload)+1)
	out[0] = byte(d.id)
	copy(out[1:], payload)
	return out, nil
}

type deterministicVerifier struct{}

func (deterministicVerifier) Verify(
	payload []byte, signature []byte, signer group.MemberIndex,
) error {
	if len(signature) != len(payload)+1 {
		return errors.New("deterministicVerifier: length mismatch")
	}
	if signature[0] != byte(signer) {
		return errors.New("deterministicVerifier: signer byte mismatch")
	}
	for i, b := range payload {
		if signature[i+1] != b {
			return errors.New("deterministicVerifier: payload byte mismatch")
		}
	}
	return nil
}

func newTestContextForSubmit(t *testing.T, sessionID string) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		sessionID,
		"key-group-submit",
		[]byte{0xAA},
		[attempt.MessageDigestLength]byte{0x42},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return ctx
}

func TestSubmitSnapshotIfActive_NoOpWhenRegistryEmpty(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// No registration, no binding. submit should be a no-op.
	recorder := attempt.NewBoundedRecorder()
	recorder.RecordOverflow(7)
	submitSnapshotIfActive("session-x", 1, recorder)
	// Nothing to assert observably: success is the absence of a
	// panic and no calls to a non-existent coordinator.
}

func TestSubmitSnapshotIfActive_NoOpWhenSessionUnbound(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	innerCoord := roast.NewInMemoryCoordinator()
	cap := newCaptureCoordinator(innerCoord)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: cap,
		Signer:      &deterministicSigner{id: 1},
		Verifier:    deterministicVerifier{},
		SelfMember:  1,
	})

	recorder := attempt.NewBoundedRecorder()
	recorder.RecordOverflow(7)
	submitSnapshotIfActive("session-with-no-binding", 1, recorder)

	if len(cap.recordedFor) != 0 {
		t.Fatalf(
			"expected no RecordEvidence calls when session unbound; got %d",
			len(cap.recordedFor),
		)
	}
}

func TestSubmitSnapshotIfActive_NoOpWhenRecorderEmpty(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	innerCoord := roast.NewInMemoryCoordinatorWithSigning(
		1,
		&deterministicSigner{id: 1},
		deterministicVerifier{},
	)
	cap := newCaptureCoordinator(innerCoord)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: cap,
		Signer:      &deterministicSigner{id: 1},
		Verifier:    deterministicVerifier{},
		SelfMember:  1,
	})

	ctx := newTestContextForSubmit(t, "session-empty")
	handle, err := cap.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	SetCurrentAttemptHandleForSession("session-empty", 1, handle, ctx)

	// Recorder is bounded but has captured zero events.
	recorder := attempt.NewBoundedRecorder()
	submitSnapshotIfActive("session-empty", 1, recorder)

	if len(cap.recordedFor) != 0 {
		t.Fatalf(
			"expected no RecordEvidence for empty snapshot; got %d",
			len(cap.recordedFor),
		)
	}
}

func TestSubmitSnapshotIfActive_SubmitsSignedSnapshotWhenBoundAndPopulated(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	const selfMember group.MemberIndex = 1
	innerCoord := roast.NewInMemoryCoordinatorWithSigning(
		selfMember,
		&deterministicSigner{id: selfMember},
		deterministicVerifier{},
	)
	cap := newCaptureCoordinator(innerCoord)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: cap,
		Signer:      &deterministicSigner{id: selfMember},
		Verifier:    deterministicVerifier{},
		SelfMember:  uint32(selfMember),
	})

	ctx := newTestContextForSubmit(t, "session-real")
	handle, err := cap.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	SetCurrentAttemptHandleForSession("session-real", selfMember, handle, ctx)

	recorder := attempt.NewBoundedRecorder()
	recorder.RecordOverflow(3)
	recorder.RecordOverflow(3)
	recorder.RecordOverflow(5)
	submitSnapshotIfActive("session-real", selfMember, recorder)

	if len(cap.recordedFor) != 1 {
		t.Fatalf("expected 1 RecordEvidence; got %d", len(cap.recordedFor))
	}
	if cap.recordedFor[0] != handle {
		t.Fatal("RecordEvidence handle mismatch")
	}
	snap := cap.recordedSnp[0]
	if snap.SenderID() != selfMember {
		t.Fatalf("snapshot sender: got %d want %d", snap.SenderID(), selfMember)
	}
	if len(snap.OperatorSignature) == 0 {
		t.Fatal("snapshot must be signed")
	}
	// 2 distinct senders observed.
	if len(snap.Overflows) != 2 {
		t.Fatalf("expected 2 overflow entries; got %d", len(snap.Overflows))
	}
}

// TestSubmitSnapshotIfActive_MultiSeatSubmitsPerSeat asserts the PR2b-2 member-
// aware submit path: with two local seats registered, each seat submits its own
// evidence against ITS OWN coordinator and attempt handle. The PR2b-1.5 count>1
// no-op guard is gone, so both submissions land -- and neither lands on the
// sibling's coordinator.
func TestSubmitSnapshotIfActive_MultiSeatSubmitsPerSeat(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	cap1 := newCaptureCoordinator(roast.NewInMemoryCoordinatorWithSigning(
		1, &deterministicSigner{id: 1}, deterministicVerifier{},
	))
	cap2 := newCaptureCoordinator(roast.NewInMemoryCoordinatorWithSigning(
		2, &deterministicSigner{id: 2}, deterministicVerifier{},
	))
	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{
		Coordinator: cap1,
		Signer:      &deterministicSigner{id: 1},
		Verifier:    deterministicVerifier{},
		SelfMember:  1,
	})
	RegisterRoastRetryCoordinatorForMember(2, RoastRetryDeps{
		Coordinator: cap2,
		Signer:      &deterministicSigner{id: 2},
		Verifier:    deterministicVerifier{},
		SelfMember:  2,
	})

	ctx := newTestContextForSubmit(t, "session-ms")
	handle1, err := cap1.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("seat 1 begin: %v", err)
	}
	handle2, err := cap2.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("seat 2 begin: %v", err)
	}
	SetCurrentAttemptHandleForSession("session-ms", 1, handle1, ctx)
	SetCurrentAttemptHandleForSession("session-ms", 2, handle2, ctx)

	rec1 := attempt.NewBoundedRecorder()
	rec1.RecordOverflow(3)
	rec2 := attempt.NewBoundedRecorder()
	rec2.RecordOverflow(4)

	submitSnapshotIfActive("session-ms", 1, rec1)
	submitSnapshotIfActive("session-ms", 2, rec2)

	// Each seat's evidence landed on ITS OWN coordinator (no mis-attribution),
	// each against its own handle and signed as its own member.
	if len(cap1.recordedFor) != 1 || cap1.recordedFor[0] != handle1 {
		t.Fatalf("seat 1 must record once against its own handle; got %+v", cap1.recordedFor)
	}
	if len(cap2.recordedFor) != 1 || cap2.recordedFor[0] != handle2 {
		t.Fatalf("seat 2 must record once against its own handle; got %+v", cap2.recordedFor)
	}
	if cap1.recordedSnp[0].SenderID() != 1 {
		t.Fatalf("seat 1 snapshot sender: got %d want 1", cap1.recordedSnp[0].SenderID())
	}
	if cap2.recordedSnp[0].SenderID() != 2 {
		t.Fatalf("seat 2 snapshot sender: got %d want 2", cap2.recordedSnp[0].SenderID())
	}
}

func TestSetCurrentAttemptHandleForSession_LaterBindingOverwrites(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctxA := newTestContextForSubmit(t, "session-overwrite")
	ctxB, _ := attempt.NewAttemptContext(
		"session-overwrite", "key-group-submit", []byte{0xAA},
		[attempt.MessageDigestLength]byte{0x42}, 1,
		[]group.MemberIndex{1, 2, 3, 4, 5}, nil,
	)
	h1 := roast.AttemptHandle{}
	h2 := roast.AttemptHandle{}

	SetCurrentAttemptHandleForSession("session-overwrite", 1, h1, ctxA)
	gotHandle, gotCtx, ok := currentAttemptHandleForCollect("session-overwrite", 1)
	if !ok {
		t.Fatal("expected binding after first Set")
	}
	if gotHandle != h1 {
		t.Fatal("first binding handle mismatch")
	}
	if gotCtx.AttemptNumber != ctxA.AttemptNumber {
		t.Fatal("first binding context mismatch")
	}

	SetCurrentAttemptHandleForSession("session-overwrite", 1, h2, ctxB)
	_, gotCtx2, ok := currentAttemptHandleForCollect("session-overwrite", 1)
	if !ok {
		t.Fatal("expected binding after second Set")
	}
	if gotCtx2.AttemptNumber != ctxB.AttemptNumber {
		t.Fatal("second binding context did not overwrite first")
	}
}

func TestClearCurrentAttemptHandleForSession_RemovesBinding(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newTestContextForSubmit(t, "session-clear")
	SetCurrentAttemptHandleForSession("session-clear", 1, roast.AttemptHandle{}, ctx)
	if _, _, ok := currentAttemptHandleForCollect("session-clear", 1); !ok {
		t.Fatal("setup: binding must exist")
	}
	ClearCurrentAttemptHandleForSession("session-clear", 1)
	if _, _, ok := currentAttemptHandleForCollect("session-clear", 1); ok {
		t.Fatal("binding must be cleared")
	}
}

func TestSubmitSnapshotIfActive_RecordEvidenceFailureIsLoggedNotPropagated(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	innerCoord := roast.NewInMemoryCoordinatorWithSigning(
		1, &deterministicSigner{id: 1}, deterministicVerifier{},
	)
	cap := newCaptureCoordinator(innerCoord)
	cap.recordErr = errors.New("synthetic RecordEvidence failure")
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: cap,
		Signer:      &deterministicSigner{id: 1},
		Verifier:    deterministicVerifier{},
		SelfMember:  1,
	})

	ctx := newTestContextForSubmit(t, "session-failure")
	handle, _ := cap.BeginAttempt(ctx)
	SetCurrentAttemptHandleForSession("session-failure", 1, handle, ctx)

	recorder := attempt.NewBoundedRecorder()
	recorder.RecordOverflow(3)

	// Must not panic. Caller is unaffected.
	submitSnapshotIfActive("session-failure", 1, recorder)
}
