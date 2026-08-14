//go:build frost_roast_retry

package signing

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

func testTransitionRecord(coordinator uint32) RoastTransitionRecord {
	return RoastTransitionRecord{
		Bundle:            &roast.TransitionMessage{CoordinatorIDValue: coordinator},
		DkgGroupPublicKey: []byte{0x01, 0x02},
	}
}

func TestRoastTransitionRegistry_RoundTrip(t *testing.T) {
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	RecordRoastTransition("session-A", 1, testTransitionRecord(7))

	got, ok := RoastTransitionForSession("session-A", 1)
	if !ok {
		t.Fatal("expected record to be present after Record")
	}
	if got.Bundle.CoordinatorIDValue != 7 {
		t.Fatalf("record round-trip mismatch: got coordinator %d, want 7", got.Bundle.CoordinatorIDValue)
	}
}

func TestRoastTransitionRegistry_MemberScoped(t *testing.T) {
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	// Two seats of one operator share a session id; their records must NOT
	// collide (the #4081 multi-seat handle class applied to transition state).
	RecordRoastTransition("session", 1, testTransitionRecord(11))
	RecordRoastTransition("session", 2, testTransitionRecord(22))

	got1, ok1 := RoastTransitionForSession("session", 1)
	got2, ok2 := RoastTransitionForSession("session", 2)
	if !ok1 || !ok2 {
		t.Fatal("both members' records must be present")
	}
	if got1.Bundle.CoordinatorIDValue != 11 || got2.Bundle.CoordinatorIDValue != 22 {
		t.Fatalf("member records collided: got %d and %d, want 11 and 22",
			got1.Bundle.CoordinatorIDValue, got2.Bundle.CoordinatorIDValue)
	}
	// A member with no record reads absent (does not alias another seat's).
	if _, ok := RoastTransitionForSession("session", 3); ok {
		t.Fatal("member 3 has no record; lookup must be absent")
	}
}

func TestRoastTransitionRegistry_LaterRecordOverwrites(t *testing.T) {
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	RecordRoastTransition("session-B", 1, testTransitionRecord(1))
	RecordRoastTransition("session-B", 1, testTransitionRecord(2))
	got, ok := RoastTransitionForSession("session-B", 1)
	if !ok {
		t.Fatal("expected record to be present")
	}
	if got.Bundle.CoordinatorIDValue != 2 {
		t.Fatalf("later Record must overwrite earlier: got %d, want 2", got.Bundle.CoordinatorIDValue)
	}
}

func TestRoastTransitionRegistry_ClearRemovesRecord(t *testing.T) {
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	RecordRoastTransition("session-clear", 1, testTransitionRecord(1))
	if _, ok := RoastTransitionForSession("session-clear", 1); !ok {
		t.Fatal("setup: record must exist")
	}
	ClearRoastTransitionForSession("session-clear", 1)
	if _, ok := RoastTransitionForSession("session-clear", 1); ok {
		t.Fatal("record must be removed after Clear")
	}
}

func TestRoastTransitionRegistry_NilBundleIsIgnored(t *testing.T) {
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	RecordRoastTransition("session-nil", 1, RoastTransitionRecord{Bundle: nil})
	if _, ok := RoastTransitionForSession("session-nil", 1); ok {
		t.Fatal("a record with a nil bundle must be discarded")
	}
}

func TestEvictStaleRoastTransitions_RemovesOldEntries(t *testing.T) {
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	RecordRoastTransition("session-old", 1, testTransitionRecord(1))
	// Backdate.
	roastTransitionRegistryMu.Lock()
	key := roastTransitionKey{"session-old", 1}
	entry := roastTransitionRegistry[key]
	entry.createdAt = time.Now().Add(-10 * time.Minute)
	roastTransitionRegistry[key] = entry
	roastTransitionRegistryMu.Unlock()

	RecordRoastTransition("session-new", 1, testTransitionRecord(2))

	evicted := evictStaleRoastTransitions(5 * time.Minute)
	if evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if _, ok := RoastTransitionForSession("session-old", 1); ok {
		t.Fatal("old record must be evicted")
	}
	if _, ok := RoastTransitionForSession("session-new", 1); !ok {
		t.Fatal("new record must survive")
	}
}

func TestRoastTransitionRegistryTTL_MatchesSessionHandleTTL(t *testing.T) {
	if RoastTransitionRegistryTTL != SessionHandleBindingTTL {
		t.Fatalf(
			"transition-record TTL %s != session-handle TTL %s; records must not outlive sessions",
			RoastTransitionRegistryTTL,
			SessionHandleBindingTTL,
		)
	}
}
