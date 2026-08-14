//go:build frost_native

package signing

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
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

func TestCollectDistributedDKGSeatOutcomesReturnsEveryDivergentPersist(
	t *testing.T,
) {
	outcomes := make(chan distributedDKGSeatOutcome, 2)
	outcomes <- distributedDKGSeatOutcome{
		member: 1,
		persist: &NativeTBTCSignerDKGResult{
			KeyGroup: "key-group-a",
		},
	}
	outcomes <- distributedDKGSeatOutcome{
		member: 2,
		persist: &NativeTBTCSignerDKGResult{
			KeyGroup: "key-group-b",
		},
	}

	persistBySeat, err := collectDistributedDKGSeatOutcomes(outcomes, 2)
	if err == nil || !strings.Contains(err.Error(), "disagreed") {
		t.Fatalf("unexpected divergent-seat result: [%v]", err)
	}
	if len(persistBySeat) != 2 {
		t.Fatalf(
			"successful divergent persists were dropped: [%v]",
			persistBySeat,
		)
	}
	for seat, expectedKeyGroup := range map[group.MemberIndex]string{
		1: "key-group-a",
		2: "key-group-b",
	} {
		persisted := persistBySeat[seat]
		if persisted == nil || persisted.KeyGroup != expectedKeyGroup {
			t.Fatalf(
				"unexpected persisted outcome for seat [%d]: [%+v]",
				seat,
				persisted,
			)
		}
	}
}
