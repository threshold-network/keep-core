//go:build frost_native

package signing

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// testDkgGroupPublicKey is the DKG group public key the test attempt contexts
// are built from; NewActiveRoastAttempt now binds the passed key to ctx via the
// derived seed, so callers must pass this same key.
var testDkgGroupPublicKey = []byte{0x01, 0x02}

func testActiveAttemptContext(t *testing.T, sessionID string, digest byte) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		sessionID,
		"key-group-test",
		testDkgGroupPublicKey,
		[attempt.MessageDigestLength]byte{digest},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("new attempt context: %v", err)
	}
	return ctx
}

func TestNewActiveRoastAttempt_BindsAndValidates(t *testing.T) {
	coord := roast.NewInMemoryCoordinator()
	ctx := testActiveAttemptContext(t, "session-1", 0x42)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	root := [32]byte{0xaa, 0xbb}
	dkgKey := append([]byte(nil), testDkgGroupPublicKey...)

	ara, err := NewActiveRoastAttempt(coord, handle, ctx, "session-1", &root, dkgKey)
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	if ara.SessionID() != "session-1" {
		t.Fatalf("unexpected session id: %q", ara.SessionID())
	}
	if ara.ContextHash() != ctx.Hash() {
		t.Fatal("context hash does not match ctx.Hash()")
	}
	if ara.Handle() != handle {
		t.Fatal("handle not bound")
	}
	// Elected coordinator is taken authoritatively from the handle.
	elected, err := coord.SelectedCoordinator(handle)
	if err != nil {
		t.Fatalf("selected coordinator: %v", err)
	}
	if ara.ElectedCoordinator() != elected {
		t.Fatalf(
			"elected coordinator mismatch: got %d, want %d",
			ara.ElectedCoordinator(), elected,
		)
	}
	if got := ara.TaprootMerkleRoot(); got == nil || *got != root {
		t.Fatalf("unexpected taproot root: %v", got)
	}
	if !bytes.Equal(ara.DkgGroupPublicKey(), dkgKey) {
		t.Fatalf("unexpected dkg group public key: %x", ara.DkgGroupPublicKey())
	}
}

func TestNewActiveRoastAttempt_RejectsInconsistentBinding(t *testing.T) {
	coord := roast.NewInMemoryCoordinator()
	ctx := testActiveAttemptContext(t, "session-1", 0x42)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	// A handle minted for a DIFFERENT context (same session id, different digest)
	// so the session-id check passes but the handle/context-hash check fires.
	otherCtx := testActiveAttemptContext(t, "session-1", 0x77)

	tests := map[string]struct {
		coord     roast.Coordinator
		handle    roast.AttemptHandle
		ctx       attempt.AttemptContext
		sessionID string
		dkgKey    []byte
	}{
		"nil coordinator":           {nil, handle, ctx, "session-1", testDkgGroupPublicKey},
		"empty session id":          {coord, handle, ctx, "", testDkgGroupPublicKey},
		"session id mismatch":       {coord, handle, ctx, "other-session", testDkgGroupPublicKey},
		"handle / context mismatch": {coord, handle, otherCtx, "session-1", testDkgGroupPublicKey},
		"empty dkg group key":       {coord, handle, ctx, "session-1", nil},
		// Non-empty but NOT the key ctx was built from: rejected via the seed check.
		"dkg key mismatch": {coord, handle, ctx, "session-1", []byte{0xff, 0xfe}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewActiveRoastAttempt(
				test.coord, test.handle, test.ctx, test.sessionID, nil, test.dkgKey,
			); err == nil {
				t.Fatal("expected an inconsistent binding to be rejected")
			}
		})
	}
}

// The binding copies the taproot root and DKG group public key, so mutating the
// caller's inputs (or the accessors' returns) cannot change it - NextAttempt
// must derive every attempt's seed from the SAME dkg group public key bytes.
func TestActiveRoastAttempt_ImmutableAfterConstruction(t *testing.T) {
	coord := roast.NewInMemoryCoordinator()
	ctx := testActiveAttemptContext(t, "session-1", 0x42)
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	root := [32]byte{0xaa}
	// A copy of the bound key (so mutating it below is safe and it still matches
	// ctx for the seed check).
	dkgKey := append([]byte(nil), testDkgGroupPublicKey...)

	ara, err := NewActiveRoastAttempt(coord, handle, ctx, "session-1", &root, dkgKey)
	if err != nil {
		t.Fatalf("construction: %v", err)
	}

	// Mutate the caller's inputs after construction.
	root[0] = 0xff
	dkgKey[0] = 0xff
	if got := ara.TaprootMerkleRoot(); got[0] != 0xaa {
		t.Fatalf("taproot root not copied from caller: %x", got)
	}
	if got := ara.DkgGroupPublicKey(); got[0] != testDkgGroupPublicKey[0] {
		t.Fatalf("dkg group key not copied from caller: %x", got)
	}

	// Mutate the accessors' returns: the binding must be unaffected.
	ara.TaprootMerkleRoot()[0] = 0xee
	ara.DkgGroupPublicKey()[0] = 0xee
	if got := ara.TaprootMerkleRoot(); got[0] != 0xaa {
		t.Fatalf("taproot root accessor must return a fresh copy: %x", got)
	}
	if got := ara.DkgGroupPublicKey(); got[0] != 0x01 {
		t.Fatalf("dkg group key accessor must return a fresh copy: %x", got)
	}
}
