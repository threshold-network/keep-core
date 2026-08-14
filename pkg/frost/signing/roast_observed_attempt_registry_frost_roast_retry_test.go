//go:build frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// TestClearObservedAttemptOnLocalSuccess asserts that clearing an observe binding
// on local success removes the active binding (so the observe handle no longer
// collects, and no failure transition can be synthesized for a succeeded attempt)
// while RETAINING the observed-history marker, so a later bundle for the attempt
// is a benign retransmit rather than lost sync (RFC-21 Phase 7.3 PR2b-1b B3).
func TestClearObservedAttemptOnLocalSuccess(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	sessionID := "registry-success-session"
	var member group.MemberIndex = 2
	var hash [attempt.MessageDigestLength]byte
	hash[0] = 0x5a

	recordObservedAttempt(sessionID, member, hash, observedAttemptBinding{})

	if _, ok := observedAttempt(sessionID, member, hash); !ok {
		t.Fatal("binding must exist after observing")
	}
	if !attemptEverObserved(sessionID, member, hash) {
		t.Fatal("history marker must exist after observing")
	}

	ClearObservedAttemptOnLocalSuccess(sessionID, member, hash)

	if _, ok := observedAttempt(sessionID, member, hash); ok {
		t.Fatal("binding must be cleared on local success")
	}
	if !attemptEverObserved(sessionID, member, hash) {
		t.Fatal("history marker must survive a local-success clear (so a retransmit is not lost sync)")
	}
}

// TestAttemptEverObservedUnobserved asserts an attempt this seat never observed is
// reported as never-observed -- the signal the transition listener uses to trip
// lost sync.
func TestAttemptEverObservedUnobserved(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	var hash [attempt.MessageDigestLength]byte
	hash[0] = 0x77
	if attemptEverObserved("never-session", 1, hash) {
		t.Fatal("an unobserved attempt must report not-ever-observed")
	}
}

// TestConsumeClearRetainsHistoryMarker asserts the per-attempt consume clear
// (clearObservedAttempt) drops the binding but keeps the observed-history marker,
// so a retransmit of a consumed bundle is distinguishable from a never-observed
// one.
func TestConsumeClearRetainsHistoryMarker(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	sessionID := "registry-consume-session"
	var member group.MemberIndex = 1
	var hash [attempt.MessageDigestLength]byte
	hash[0] = 0x2b
	recordObservedAttempt(sessionID, member, hash, observedAttemptBinding{})

	clearObservedAttempt(sessionID, member, hash)

	if _, ok := observedAttempt(sessionID, member, hash); ok {
		t.Fatal("binding must be cleared after consume")
	}
	if !attemptEverObserved(sessionID, member, hash) {
		t.Fatal("history marker must survive a consume clear")
	}
}

// TestClearObservedAttemptsForSessionClearsHistory asserts session-end clearing
// drops the observed-history markers too, so a stale marker does not suppress a
// genuine lost-sync signal in a later session that reuses the same id.
func TestClearObservedAttemptsForSessionClearsHistory(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	sessionID := "registry-session-end"
	var member group.MemberIndex = 1
	var hash [attempt.MessageDigestLength]byte
	hash[0] = 0x3c
	recordObservedAttempt(sessionID, member, hash, observedAttemptBinding{})

	clearObservedAttemptsForSession(sessionID, member)

	if attemptEverObserved(sessionID, member, hash) {
		t.Fatal("session-end clear must drop the observed-history marker")
	}
}
