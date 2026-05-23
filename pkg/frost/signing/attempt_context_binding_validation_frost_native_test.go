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
		if err := verifyMessageAttemptContextHash(msg, "session-x"); err != nil {
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
	SetCurrentAttemptHandleForSession("session-match", roast.AttemptHandle{}, ctx)

	expected := ctx.Hash()
	msg := stubMessage{hash: expected, present: true}
	if err := verifyMessageAttemptContextHash(msg, "session-match"); err != nil {
		t.Fatalf("matching hash must pass; got %v", err)
	}
}

func TestVerifyMessageAttemptContextHash_BindingPresent_MissingHashFails(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-missing", roast.AttemptHandle{}, ctx)

	msg := stubMessage{present: false}
	err := verifyMessageAttemptContextHash(msg, "session-missing")
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
	SetCurrentAttemptHandleForSession("session-mismatch", roast.AttemptHandle{}, ctx)

	wrong := [AttemptContextHashFieldLength]byte{}
	for i := range wrong {
		wrong[i] = 0xff
	}
	msg := stubMessage{hash: wrong, present: true}
	err := verifyMessageAttemptContextHash(msg, "session-mismatch")
	if !errors.Is(err, ErrAttemptContextHashMismatch) {
		t.Fatalf(
			"expected ErrAttemptContextHashMismatch; got %v",
			err,
		)
	}
}

func TestVerifyMessageAttemptContextHash_RealMessageTypeIntegration(t *testing.T) {
	// Exercise the helper against a real protocol message type
	// (the round-one commitment from Phase 1B) rather than just
	// the stub, so the test surface covers the actual Set/Get
	// helpers code path.
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContextForValidation(t)
	SetCurrentAttemptHandleForSession("session-real-msg", roast.AttemptHandle{}, ctx)

	expected := ctx.Hash()
	msg := &nativeFROSTRoundOneCommitmentMessage{
		SenderIDValue:         1,
		SessionIDValue:        "session-real-msg",
		ParticipantIdentifier: "p1",
		CommitmentData:        []byte{0x01},
	}
	msg.SetAttemptContextHash(expected)

	if err := verifyMessageAttemptContextHash(msg, "session-real-msg"); err != nil {
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
	SetCurrentAttemptHandleForSession("session-real-msg", roast.AttemptHandle{}, differentCtx)

	err := verifyMessageAttemptContextHash(msg, "session-real-msg")
	if !errors.Is(err, ErrAttemptContextHashMismatch) {
		t.Fatalf("rebinding must cause mismatch; got %v", err)
	}
}
