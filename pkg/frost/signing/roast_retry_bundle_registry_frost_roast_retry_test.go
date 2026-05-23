//go:build frost_roast_retry

package signing

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

func TestTransitionBundleRegistry_RoundTrip(t *testing.T) {
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	bundle := &roast.TransitionMessage{
		CoordinatorIDValue: 7,
	}
	RecordTransitionBundleForSession("session-A", bundle)

	got, ok := TransitionBundleForSession("session-A")
	if !ok {
		t.Fatal("expected bundle to be present after Record")
	}
	if got.CoordinatorIDValue != 7 {
		t.Fatalf(
			"bundle round-trip mismatch: got coordinator %d, want 7",
			got.CoordinatorIDValue,
		)
	}
}

func TestTransitionBundleRegistry_LaterRecordOverwrites(t *testing.T) {
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	RecordTransitionBundleForSession("session-B", &roast.TransitionMessage{CoordinatorIDValue: 1})
	RecordTransitionBundleForSession("session-B", &roast.TransitionMessage{CoordinatorIDValue: 2})
	got, ok := TransitionBundleForSession("session-B")
	if !ok {
		t.Fatal("expected bundle to be present")
	}
	if got.CoordinatorIDValue != 2 {
		t.Fatalf(
			"later Record must overwrite earlier: got %d, want 2",
			got.CoordinatorIDValue,
		)
	}
}

func TestTransitionBundleRegistry_ClearRemovesBundle(t *testing.T) {
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	RecordTransitionBundleForSession("session-clear", &roast.TransitionMessage{})
	if _, ok := TransitionBundleForSession("session-clear"); !ok {
		t.Fatal("setup: bundle must exist")
	}
	ClearTransitionBundleForSession("session-clear")
	if _, ok := TransitionBundleForSession("session-clear"); ok {
		t.Fatal("bundle must be removed after Clear")
	}
}

func TestTransitionBundleRegistry_NilBundleIsIgnored(t *testing.T) {
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	RecordTransitionBundleForSession("session-nil", nil)
	if _, ok := TransitionBundleForSession("session-nil"); ok {
		t.Fatal("nil bundle must be discarded")
	}
}

func TestEvictStaleTransitionBundles_RemovesOldEntries(t *testing.T) {
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	RecordTransitionBundleForSession("session-old", &roast.TransitionMessage{CoordinatorIDValue: 1})
	// Backdate.
	sessionBundleRegistryMu.Lock()
	entry := sessionBundleRegistry["session-old"]
	entry.createdAt = time.Now().Add(-10 * time.Minute)
	sessionBundleRegistry["session-old"] = entry
	sessionBundleRegistryMu.Unlock()

	RecordTransitionBundleForSession("session-new", &roast.TransitionMessage{CoordinatorIDValue: 2})

	evicted := evictStaleTransitionBundles(5 * time.Minute)
	if evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if _, ok := TransitionBundleForSession("session-old"); ok {
		t.Fatal("old bundle must be evicted")
	}
	if _, ok := TransitionBundleForSession("session-new"); !ok {
		t.Fatal("new bundle must survive")
	}
}

func TestTransitionBundleRegistryTTL_MatchesSessionHandleTTL(t *testing.T) {
	if TransitionBundleRegistryTTL != SessionHandleBindingTTL {
		t.Fatalf(
			"bundle TTL %s != session-handle TTL %s; bundles must not outlive sessions",
			TransitionBundleRegistryTTL,
			SessionHandleBindingTTL,
		)
	}
}
