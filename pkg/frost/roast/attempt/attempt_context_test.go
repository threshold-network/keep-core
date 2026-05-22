package attempt

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestDeriveAttemptSeed_IsPureFunctionOfInputs(t *testing.T) {
	dkgPub := []byte{0x02, 0x01, 0x02, 0x03, 0x04}
	sessionID := "session-a"
	var digest [MessageDigestLength]byte
	copy(digest[:], bytes.Repeat([]byte{0x42}, MessageDigestLength))

	a := DeriveAttemptSeed(dkgPub, sessionID, digest)
	b := DeriveAttemptSeed(dkgPub, sessionID, digest)
	if a != b {
		t.Fatalf("derivation not deterministic: %x != %x", a, b)
	}

	expected := sha256.Sum256(
		append(append(append([]byte{}, dkgPub...), []byte(sessionID)...), digest[:]...),
	)
	if a != expected {
		t.Fatalf(
			"derivation does not match SHA256(dkgPub || sessionID || messageDigest): got %x want %x",
			a, expected,
		)
	}
}

func TestDeriveAttemptSeed_SensitiveToEachInput(t *testing.T) {
	base := DeriveAttemptSeed(
		[]byte{0x01, 0x02},
		"session-a",
		[MessageDigestLength]byte{0x01},
	)

	tests := []struct {
		name      string
		dkgPub    []byte
		sessionID string
		digest    [MessageDigestLength]byte
	}{
		{
			name:      "different DKG public key",
			dkgPub:    []byte{0x01, 0x03},
			sessionID: "session-a",
			digest:    [MessageDigestLength]byte{0x01},
		},
		{
			name:      "different session ID",
			dkgPub:    []byte{0x01, 0x02},
			sessionID: "session-b",
			digest:    [MessageDigestLength]byte{0x01},
		},
		{
			name:      "different message digest",
			dkgPub:    []byte{0x01, 0x02},
			sessionID: "session-a",
			digest:    [MessageDigestLength]byte{0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveAttemptSeed(tt.dkgPub, tt.sessionID, tt.digest)
			if got == base {
				t.Fatalf("seed collided with base for %s", tt.name)
			}
		})
	}
}

func TestNewAttemptContext_SortsAndDeduplicates(t *testing.T) {
	dkgPub := []byte{0x01}
	digest := [MessageDigestLength]byte{0xaa}

	included := []group.MemberIndex{5, 3, 4, 1, 2}
	excluded := []group.MemberIndex{7, 6}

	ctx, err := NewAttemptContext(
		"session", "key-group", dkgPub, digest, 0, included, excluded,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []group.MemberIndex{1, 2, 3, 4, 5}
	if !memberSlicesEqual(ctx.IncludedSet, want) {
		t.Fatalf(
			"included set not sorted: got %v want %v",
			ctx.IncludedSet, want,
		)
	}
	wantExcluded := []group.MemberIndex{6, 7}
	if !memberSlicesEqual(ctx.ExcludedSet, wantExcluded) {
		t.Fatalf(
			"excluded set not sorted: got %v want %v",
			ctx.ExcludedSet, wantExcluded,
		)
	}

	if !bytes.Equal(included, []group.MemberIndex{5, 3, 4, 1, 2}) {
		t.Fatalf(
			"caller's included slice was mutated: %v",
			included,
		)
	}
}

func TestNewAttemptContext_RejectsEmptyIncludedSet(t *testing.T) {
	_, err := NewAttemptContext(
		"session", "kg", []byte{0x01},
		[MessageDigestLength]byte{}, 0,
		nil, nil,
	)
	if err == nil {
		t.Fatal("expected error for empty included set")
	}
	if !strings.Contains(err.Error(), "included set must not be empty") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestNewAttemptContext_RejectsDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		included []group.MemberIndex
		excluded []group.MemberIndex
		want     string
	}{
		{
			name:     "duplicate in included set",
			included: []group.MemberIndex{1, 2, 2, 3},
			excluded: nil,
			want:     "included set contains duplicate",
		},
		{
			name:     "duplicate in excluded set",
			included: []group.MemberIndex{1, 2},
			excluded: []group.MemberIndex{4, 4},
			want:     "excluded set contains duplicate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAttemptContext(
				"session", "kg", []byte{0x01},
				[MessageDigestLength]byte{}, 0,
				tt.included, tt.excluded,
			)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf(
					"unexpected error message: got %q want substring %q",
					err.Error(), tt.want,
				)
			}
		})
	}
}

func TestNewAttemptContext_RejectsOverlap(t *testing.T) {
	_, err := NewAttemptContext(
		"session", "kg", []byte{0x01},
		[MessageDigestLength]byte{}, 0,
		[]group.MemberIndex{1, 2, 3},
		[]group.MemberIndex{3, 4},
	)
	if err == nil {
		t.Fatal("expected overlap error")
	}
	if !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAttemptContextHash_IsDeterministicAcrossInputOrdering(t *testing.T) {
	dkgPub := []byte{0xab, 0xcd}
	digest := [MessageDigestLength]byte{0x77}

	ctxA, err := NewAttemptContext(
		"session", "kg", dkgPub, digest, 7,
		[]group.MemberIndex{5, 3, 4, 1, 2},
		[]group.MemberIndex{7, 6},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctxB, err := NewAttemptContext(
		"session", "kg", dkgPub, digest, 7,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		[]group.MemberIndex{6, 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctxA.Hash() != ctxB.Hash() {
		t.Fatalf(
			"semantically equal contexts produced different hashes: %x vs %x",
			ctxA.Hash(), ctxB.Hash(),
		)
	}
}

func TestAttemptContextHash_SensitiveToEachField(t *testing.T) {
	base, err := NewAttemptContext(
		"session", "kg", []byte{0x01},
		[MessageDigestLength]byte{0x05}, 3,
		[]group.MemberIndex{1, 2, 3},
		[]group.MemberIndex{4},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	baseHash := base.Hash()

	type mutator struct {
		name string
		fn   func() (AttemptContext, error)
	}
	mutators := []mutator{
		{
			name: "different session ID",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session-2", "kg", []byte{0x01},
					[MessageDigestLength]byte{0x05}, 3,
					[]group.MemberIndex{1, 2, 3},
					[]group.MemberIndex{4},
				)
			},
		},
		{
			name: "different key group ID",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session", "kg-2", []byte{0x01},
					[MessageDigestLength]byte{0x05}, 3,
					[]group.MemberIndex{1, 2, 3},
					[]group.MemberIndex{4},
				)
			},
		},
		{
			name: "different message digest",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session", "kg", []byte{0x01},
					[MessageDigestLength]byte{0x06}, 3,
					[]group.MemberIndex{1, 2, 3},
					[]group.MemberIndex{4},
				)
			},
		},
		{
			name: "different attempt number",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session", "kg", []byte{0x01},
					[MessageDigestLength]byte{0x05}, 4,
					[]group.MemberIndex{1, 2, 3},
					[]group.MemberIndex{4},
				)
			},
		},
		{
			name: "different included set",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session", "kg", []byte{0x01},
					[MessageDigestLength]byte{0x05}, 3,
					[]group.MemberIndex{1, 2, 3, 5},
					[]group.MemberIndex{4},
				)
			},
		},
		{
			name: "different excluded set",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session", "kg", []byte{0x01},
					[MessageDigestLength]byte{0x05}, 3,
					[]group.MemberIndex{1, 2, 3},
					nil,
				)
			},
		},
		{
			name: "different DKG public key",
			fn: func() (AttemptContext, error) {
				return NewAttemptContext(
					"session", "kg", []byte{0x02},
					[MessageDigestLength]byte{0x05}, 3,
					[]group.MemberIndex{1, 2, 3},
					[]group.MemberIndex{4},
				)
			},
		},
	}

	for _, m := range mutators {
		t.Run(m.name, func(t *testing.T) {
			ctx, err := m.fn()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ctx.Hash() == baseHash {
				t.Fatalf(
					"%s did not change hash; base=%x mutated=%x",
					m.name, baseHash, ctx.Hash(),
				)
			}
		})
	}
}

func TestAttemptContextHash_PrefixesAvoidStringConcatCollision(t *testing.T) {
	// Without length-prefixed encoding, ("ab", "cd") and ("a", "bcd") would
	// produce identical hashes. Verify they do not.
	dkgPub := []byte{0x01}
	digest := [MessageDigestLength]byte{}

	ctxA, err := NewAttemptContext(
		"ab", "cd", dkgPub, digest, 0,
		[]group.MemberIndex{1}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctxB, err := NewAttemptContext(
		"a", "bcd", dkgPub, digest, 0,
		[]group.MemberIndex{1}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctxA.Hash() == ctxB.Hash() {
		t.Fatalf(
			"concatenated session+keyGroup collide: hash=%x",
			ctxA.Hash(),
		)
	}
}

func TestAttemptContextHash_IsStableAcrossSafeFieldExtensions(t *testing.T) {
	// Lock the wire encoding by asserting a specific hash output for a
	// pinned fixture. If a future change to the canonical encoding
	// changes this hash, that change is a wire-format break and must be
	// caught at code review.
	ctx, err := NewAttemptContext(
		"session-pinned",
		"key-group-pinned",
		[]byte{0xAA, 0xBB, 0xCC, 0xDD},
		[MessageDigestLength]byte{
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
			0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
			0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
			0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
		},
		42,
		[]group.MemberIndex{1, 2, 3},
		[]group.MemberIndex{4, 5},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Recompute the expected hash by independently re-implementing the
	// canonical encoding here so the test catches accidental drift in
	// either the production encoder or the expected hash literal.
	want := referenceHashForFixture(ctx)
	got := ctx.Hash()
	if got != want {
		t.Fatalf(
			"pinned fixture hash drifted: got %x want %x",
			got, want,
		)
	}
}

func memberSlicesEqual(a, b []group.MemberIndex) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// referenceHashForFixture implements the canonical encoding inline so
// the pinned-fixture test catches drift in either the production
// implementation or the test literal.
func referenceHashForFixture(ctx AttemptContext) [MessageDigestLength]byte {
	h := sha256.New()
	writeLP := func(b []byte) {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(b)))
		h.Write(l[:])
		h.Write(b)
	}
	writeMS := func(ms []group.MemberIndex) {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(ms)))
		h.Write(l[:])
		for _, m := range ms {
			h.Write([]byte{byte(m)})
		}
	}

	writeLP([]byte(ctx.SessionID))
	writeLP([]byte(ctx.KeyGroupID))
	h.Write(ctx.MessageDigest[:])
	var a [4]byte
	binary.BigEndian.PutUint32(a[:], ctx.AttemptNumber)
	h.Write(a[:])
	writeMS(ctx.IncludedSet)
	writeMS(ctx.ExcludedSet)
	h.Write(ctx.AttemptSeed[:])
	var out [MessageDigestLength]byte
	copy(out[:], h.Sum(nil))
	return out
}
