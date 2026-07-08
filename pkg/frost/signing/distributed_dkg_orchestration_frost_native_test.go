//go:build frost_native

package signing

import (
	"encoding/hex"
	"fmt"
	"testing"
)

// TestCanonicalFROSTIdentifier pins the canonical identifier string the node
// builds for the DKG to the engine's participant_identifier_to_frost_identifier:
// the participant as a 32-byte big-endian scalar (value in the LEAST-significant
// byte), hex-encoded and JSON-quoted. The persist op re-derives and rejects a
// mismatch, so a regression here would silently break DKG-to-signing.
func TestCanonicalFROSTIdentifier(t *testing.T) {
	// Member 1 -> 31 zero bytes then 0x01.
	id1 := make([]byte, 32)
	id1[31] = 0x01
	if got, want := CanonicalFROSTIdentifier(1), fmt.Sprintf("%q", hex.EncodeToString(id1)); got != want {
		t.Fatalf("CanonicalFROSTIdentifier(1) = %s, want %s", got, want)
	}

	// Member 256 -> the high byte sits at id[30], id[31] is zero (big-endian).
	id256 := make([]byte, 32)
	id256[30] = 0x01
	if got, want := CanonicalFROSTIdentifier(256), fmt.Sprintf("%q", hex.EncodeToString(id256)); got != want {
		t.Fatalf("CanonicalFROSTIdentifier(256) = %s, want %s", got, want)
	}

	// The value is quoted (the engine returns the JSON/textual form of the frost
	// identifier, not a bare hex string).
	got := CanonicalFROSTIdentifier(1)
	if len(got) < 2 || got[0] != '"' || got[len(got)-1] != '"' {
		t.Fatalf("CanonicalFROSTIdentifier(1) = %s, want a quoted string", got)
	}

	// Injective over a range spanning the single-byte boundary (distinct scalars
	// give distinct identifiers, which newDistributedDKGRunner requires).
	seen := make(map[string]struct{}, 260)
	for m := uint16(1); m <= 260; m++ {
		id := CanonicalFROSTIdentifier(m)
		if _, dup := seen[id]; dup {
			t.Fatalf("CanonicalFROSTIdentifier is not injective at member %d", m)
		}
		seen[id] = struct{}{}
	}
}
