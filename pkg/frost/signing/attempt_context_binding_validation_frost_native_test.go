//go:build frost_native && frost_roast_retry

package signing

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// stubMessage is a minimal attemptContextHashCarrier implementation
// for unit tests. The receive callbacks use the three real message
// types; the helper itself is exercised via this stub so the test
// surface stays small.
type stubMessage struct {
	hash    [AttemptContextHashFieldLength]byte
	present bool
}

func (s stubMessage) GetAttemptContextHash() (
	[AttemptContextHashFieldLength]byte, bool,
) {
	return s.hash, s.present
}

func (s *stubMessage) SetAttemptContextHash(
	hash [AttemptContextHashFieldLength]byte,
) {
	s.hash = hash
	s.present = true
}

func newOrchestrationTestContextForValidation(t *testing.T) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"validation-test",
		"key-group",
		[]byte{0x01, 0x02},
		[attempt.MessageDigestLength]byte{0x77},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return ctx
}

func TestVerifyMessageAttemptContextHash_NoBindingPasses(t *testing.T) {
	// In the default build, no session-handle bindings exist so
	// every call returns nil regardless of message contents. The
	// receive loop's other gates still apply.
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	cases := []stubMessage{
		{present: false},
		{present: true, hash: [AttemptContextHashFieldLength]byte{0x01}},
	}
	for _, msg := range cases {
		if err := verifyMessageAttemptContextHash(msg, "session-x", 1); err != nil {
			t.Fatalf(
				"no-binding path must pass; got %v for msg %+v",
				err, msg,
			)
		}
	}
}

func TestVerifyMessageAttemptContextHash_BindingPresent_MatchingHashPasses(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-match", 1, roast.AttemptHandle{}, ctx)

	expected := ctx.Hash()
	msg := stubMessage{hash: expected, present: true}
	if err := verifyMessageAttemptContextHash(msg, "session-match", 1); err != nil {
		t.Fatalf("matching hash must pass; got %v", err)
	}
}

func TestVerifyMessageAttemptContextHash_BindingPresent_MissingHashFails(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-missing", 1, roast.AttemptHandle{}, ctx)

	msg := stubMessage{present: false}
	err := verifyMessageAttemptContextHash(msg, "session-missing", 1)
	if !errors.Is(err, ErrAttemptContextHashMissing) {
		t.Fatalf(
			"expected ErrAttemptContextHashMissing; got %v",
			err,
		)
	}
}

func TestVerifyMessageAttemptContextHash_BindingPresent_MismatchedHashFails(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-mismatch", 1, roast.AttemptHandle{}, ctx)

	wrong := [AttemptContextHashFieldLength]byte{}
	for i := range wrong {
		wrong[i] = 0xff
	}
	msg := stubMessage{hash: wrong, present: true}
	err := verifyMessageAttemptContextHash(msg, "session-mismatch", 1)
	if !errors.Is(err, ErrAttemptContextHashMismatch) {
		t.Fatalf(
			"expected ErrAttemptContextHashMismatch; got %v",
			err,
		)
	}
}

// TestVerifyMessageAttemptContextHash_BindingIsMemberScoped asserts the binding
// lookup is keyed by the LOCAL receiver seat's member (request.MemberIndex), not
// shared across seats: a binding set for member 1 enforces the hash for member 1
// but is invisible to member 2's receive loop (which has its own binding or, here,
// none -> passes through). This is the PR2b-2 member-keying applied to the receive
// validation path; under the old sessionID-only key, member 2 would have enforced
// member 1's binding.
func TestVerifyMessageAttemptContextHash_BindingIsMemberScoped(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-scoped", 1, roast.AttemptHandle{}, ctx)

	// A message that does NOT match the bound context.
	wrong := [AttemptContextHashFieldLength]byte{}
	for i := range wrong {
		wrong[i] = 0xff
	}
	msg := stubMessage{hash: wrong, present: true}

	// Member 1 has the binding -> enforcement runs -> mismatch.
	if err := verifyMessageAttemptContextHash(msg, "session-scoped", 1); !errors.Is(err, ErrAttemptContextHashMismatch) {
		t.Fatalf("member 1 must enforce its binding; got %v", err)
	}
	// Member 2 has no binding for this session -> passes through (no enforcement).
	if err := verifyMessageAttemptContextHash(msg, "session-scoped", 2); err != nil {
		t.Fatalf("member 2 (no binding) must pass through; got %v", err)
	}
}

func TestVerifyMessageAttemptContextHash_RealMessageTypeIntegration(t *testing.T) {
	// Exercise the helper against a real protocol message type
	// (the tbtc-signer round contribution) rather than just the stub,
	// so the test surface covers the actual Set/Get
	// helpers code path.
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-real-msg", 1, roast.AttemptHandle{}, ctx)

	expected := ctx.Hash()
	msg := &testRoundContributionMessage{
		SenderIDValue:          1,
		SessionIDValue:         "session-real-msg",
		ContributionIdentifier: 1,
		ContributionData:       []byte{0x01},
	}
	msg.SetAttemptContextHash(expected)

	if err := verifyMessageAttemptContextHash(msg, "session-real-msg", 1); err != nil {
		t.Fatalf("real-message integration must pass; got %v", err)
	}

	// Now mutate the context to break the binding.
	differentCtx, _ := attempt.NewAttemptContext(
		"session-real-msg",
		"key-group",
		[]byte{0x99},
		[attempt.MessageDigestLength]byte{0x77},
		1,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	SetCurrentAttemptHandleForSession("session-real-msg", 1, roast.AttemptHandle{}, differentCtx)

	err := verifyMessageAttemptContextHash(msg, "session-real-msg", 1)
	if !errors.Is(err, ErrAttemptContextHashMismatch) {
		t.Fatalf("rebinding must cause mismatch; got %v", err)
	}
}

func TestSetMessageAttemptContextHashIfBound_AttachesBoundHash(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-outbound", 1, roast.AttemptHandle{}, ctx)

	msg := &stubMessage{}
	setMessageAttemptContextHashIfBound(msg, "session-outbound", 1)

	got, present := msg.GetAttemptContextHash()
	if !present {
		t.Fatal("expected outbound message to carry attempt context hash")
	}
	if got != ctx.Hash() {
		t.Fatalf("unexpected attempt context hash: got %x want %x", got, ctx.Hash())
	}
}

func TestSetMessageAttemptContextHashIfBound_NoBindingLeavesAbsent(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	msg := &stubMessage{}
	setMessageAttemptContextHashIfBound(msg, "session-no-binding", 1)

	if _, present := msg.GetAttemptContextHash(); present {
		t.Fatal("expected no attempt context hash without a session binding")
	}
}

func TestSetMessageAttemptContextHashIfBound_AllOutboundMessageTypes(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-all-types", 1, roast.AttemptHandle{}, ctx)
	expected := ctx.Hash()

	messages := []attemptContextHashCarrier{
		&testRoundContributionMessage{},
	}

	for _, msg := range messages {
		outbound, ok := msg.(outboundAttemptContextHashCarrier)
		if !ok {
			t.Fatalf("%T does not implement outbound carrier", msg)
		}

		setMessageAttemptContextHashIfBound(outbound, "session-all-types", 1)

		got, present := msg.GetAttemptContextHash()
		if !present {
			t.Fatalf("%T did not get attempt context hash", msg)
		}
		if got != expected {
			t.Fatalf("%T hash mismatch: got %x want %x", msg, got, expected)
		}
	}
}
