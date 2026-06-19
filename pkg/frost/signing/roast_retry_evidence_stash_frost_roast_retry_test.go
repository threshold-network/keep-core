//go:build frost_roast_retry

package signing

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func stashTestHash(b byte) [attempt.MessageDigestLength]byte {
	var h [attempt.MessageDigestLength]byte
	h[0] = b
	return h
}

func stashTestEvidence() attempt.Evidence {
	return attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{2: 1},
		Rejects: map[group.MemberIndex][]attempt.RejectEntry{
			3: {{Reason: "r", Count: 1}},
		},
		Conflicts: map[group.MemberIndex]uint{4: 1},
	}
}

func TestPendingEvidenceStash_StoreTakeConsumes(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x11)
	stashPendingEvidence("s", 1, hash, stashTestEvidence())

	if !PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("entry must be present after store")
	}
	got, ok := takePendingEvidence("s", 1, hash)
	if !ok {
		t.Fatal("take must find the stored entry")
	}
	if got.Overflows[2] != 1 || got.Conflicts[4] != 1 || len(got.Rejects[3]) != 1 {
		t.Fatalf("taken evidence does not match stored: %+v", got)
	}
	if _, ok := takePendingEvidence("s", 1, hash); ok {
		t.Fatal("take must consume: a second take finds nothing")
	}
	if PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("entry must be gone after consume")
	}
}

// TestPendingEvidenceStash_DeepCopyIsolatesCallerMutation proves copyEvidence
// deep-copies on store: mutating the caller's Evidence (maps AND the per-sender
// reject slice) after stashing must not change what the exchange later reads.
func TestPendingEvidenceStash_DeepCopyIsolatesCallerMutation(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x22)
	src := stashTestEvidence()
	stashPendingEvidence("s", 1, hash, src)

	// Mutate every layer of the source after the store.
	src.Overflows[2] = 99
	src.Overflows[5] = 7 // new key
	src.Rejects[3][0].Count = 99
	src.Conflicts[4] = 99

	got, ok := takePendingEvidence("s", 1, hash)
	if !ok {
		t.Fatal("take must find the stored entry")
	}
	if got.Overflows[2] != 1 {
		t.Fatalf("overflow count must reflect store-time value 1; got %d", got.Overflows[2])
	}
	if _, present := got.Overflows[5]; present {
		t.Fatal("a key added to the source after store must not appear in the stash")
	}
	if got.Rejects[3][0].Count != 1 {
		t.Fatalf("reject slice element must reflect store-time value 1; got %d", got.Rejects[3][0].Count)
	}
	if got.Conflicts[4] != 1 {
		t.Fatalf("conflict count must reflect store-time value 1; got %d", got.Conflicts[4])
	}
}

// TestPendingEvidenceStash_MemberKeyedIsolation asserts two seats sharing the same
// (sessionID, attemptHash) keep separate entries -- the member-keying that prevents
// a sibling seat from overwriting another's evidence.
func TestPendingEvidenceStash_MemberKeyedIsolation(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x33)
	stashPendingEvidence("s", 1, hash, attempt.Evidence{Overflows: map[group.MemberIndex]uint{7: 1}})
	stashPendingEvidence("s", 2, hash, attempt.Evidence{Overflows: map[group.MemberIndex]uint{8: 1}})

	got1, ok1 := takePendingEvidence("s", 1, hash)
	got2, ok2 := takePendingEvidence("s", 2, hash)
	if !ok1 || !ok2 {
		t.Fatalf("both member entries must exist; ok1=%v ok2=%v", ok1, ok2)
	}
	if got1.Overflows[7] != 1 || got1.Overflows[8] != 0 {
		t.Fatalf("seat 1 entry bled; got %+v", got1.Overflows)
	}
	if got2.Overflows[8] != 1 || got2.Overflows[7] != 0 {
		t.Fatalf("seat 2 entry bled; got %+v", got2.Overflows)
	}
}

func TestPendingEvidenceStash_ClearForSession(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	stashPendingEvidence("s", 1, stashTestHash(0x01), stashTestEvidence())
	stashPendingEvidence("s", 1, stashTestHash(0x02), stashTestEvidence())
	stashPendingEvidence("other", 1, stashTestHash(0x03), stashTestEvidence())

	clearPendingEvidenceForSession("s", 1)

	if PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("clearForSession must remove every attempt of (s,1)")
	}
	if !PendingEvidenceStashedForTest("other", 1) {
		t.Fatal("clearForSession must not touch a different session")
	}
}

func TestClearPendingEvidenceOnLocalSuccess(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x44)
	stashPendingEvidence("s", 1, hash, stashTestEvidence())
	ClearPendingEvidenceOnLocalSuccess("s", 1, hash)
	if PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("success clear must remove the attempt's stash entry")
	}
}

// TestEvictStalePendingEvidence asserts the TTL backstop: a fresh entry survives a
// long TTL and is swept once the cutoff passes its creation time. A negative
// max-age sets the cutoff in the future, so the entry is deterministically stale
// without sleeping.
func TestEvictStalePendingEvidence(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	stashPendingEvidence("s", 1, stashTestHash(0x55), stashTestEvidence())

	if n := evictStalePendingEvidence(time.Hour); n != 0 {
		t.Fatalf("a fresh entry must survive a long TTL; evicted %d", n)
	}
	if !PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("entry must remain after a no-op sweep")
	}
	if n := evictStalePendingEvidence(-time.Hour); n != 1 {
		t.Fatalf("a future cutoff must evict the entry; evicted %d", n)
	}
	if PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("entry must be gone after the sweep")
	}
}
