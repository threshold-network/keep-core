package roast

import (
	"encoding/binary"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

func TestFoldAttemptSeed_IsDeterministic(t *testing.T) {
	seed := [attempt.AttemptSeedLength]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	}
	a := foldAttemptSeed(seed)
	b := foldAttemptSeed(seed)
	if a != b {
		t.Fatalf("foldAttemptSeed not deterministic: %d != %d", a, b)
	}
}

func TestFoldAttemptSeed_TakesFirst8BytesBigEndian(t *testing.T) {
	seed := [attempt.AttemptSeedLength]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	got := foldAttemptSeed(seed)
	if got != 1 {
		t.Fatalf("first-8 BE decode wrong: got %d want 1", got)
	}
}

func TestFoldAttemptSeed_IgnoresBytesAfterIndex7(t *testing.T) {
	// Document the contract: bytes 8..31 do not influence the output.
	// Any change to those bytes is still caught at the
	// AttemptContext.Hash() layer; the bridge merely surfaces the
	// first 8.
	base := [attempt.AttemptSeedLength]byte{
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	mutated := base
	for i := 8; i < attempt.AttemptSeedLength; i++ {
		mutated[i] ^= 0xff
	}
	if foldAttemptSeed(base) != foldAttemptSeed(mutated) {
		t.Fatal(
			"bridge must ignore bytes 8..31 by contract; honest signers " +
				"will desynchronise if this assumption changes",
		)
	}
}

func TestFoldAttemptSeed_FirstByteSwept(t *testing.T) {
	// Sweep the high byte of the leading uint64; every value must
	// produce a distinct int64.
	seen := map[int64]struct{}{}
	for hi := 0; hi < 256; hi++ {
		var seed [attempt.AttemptSeedLength]byte
		seed[0] = byte(hi)
		got := foldAttemptSeed(seed)
		if _, dup := seen[got]; dup {
			t.Fatalf("collision on high-byte sweep at %d", hi)
		}
		seen[got] = struct{}{}
	}
	if len(seen) != 256 {
		t.Fatalf("expected 256 distinct outputs, got %d", len(seen))
	}
}

func TestFoldAttemptSeed_GoldenFixture(t *testing.T) {
	// Locks the wire-format reduction so any future change to the
	// bridge implementation is caught at code review. Two coordinator
	// instances that disagree on this constant will produce
	// divergent SelectCoordinator outputs and fracture the network.
	seed := [attempt.AttemptSeedLength]byte{
		0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0xba, 0xbe,
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
	}
	want := int64(binary.BigEndian.Uint64(seed[:8]))
	got := foldAttemptSeed(seed)
	if got != want {
		t.Fatalf(
			"golden fixture drift: got %d want %d (seed=%x)",
			got, want, seed[:8],
		)
	}
	// Also assert the literal integer so a typo in the reference
	// computation above is caught: 0xdeadbeefcafebabe (16045690984503098046
	// as uint64) reinterpreted as int64.
	const wantLiteral int64 = -2401053089206453570
	if got != wantLiteral {
		t.Fatalf(
			"golden fixture int64 drift: got %d want %d",
			got, wantLiteral,
		)
	}
}
