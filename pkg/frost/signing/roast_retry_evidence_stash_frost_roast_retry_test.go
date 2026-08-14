//go:build frost_roast_retry

package signing

import (
	"bytes"
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
	got, _, ok := takePendingEvidence("s", 1, hash)
	if !ok {
		t.Fatal("take must find the stored entry")
	}
	if got.Overflows[2] != 1 || got.Conflicts[4] != 1 || len(got.Rejects[3]) != 1 {
		t.Fatalf("taken evidence does not match stored: %+v", got)
	}
	if _, _, ok := takePendingEvidence("s", 1, hash); ok {
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

	got, _, ok := takePendingEvidence("s", 1, hash)
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

	got1, _, ok1 := takePendingEvidence("s", 1, hash)
	got2, _, ok2 := takePendingEvidence("s", 2, hash)
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

// TestStashPendingCoordinatorProofs_StoreTakeConsumes is the 2b proof path: the
// interactive drive stashes coordinator-package proofs; takePendingEvidence returns
// them (with empty Evidence) and consumes the entry.
func TestStashPendingCoordinatorProofs_StoreTakeConsumes(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x61)
	proofs := [][]byte{[]byte("auth-package"), []byte("conflicting-package")}
	stashPendingCoordinatorProofs("s", 1, hash, proofs)

	if !PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("entry must be present after stashing proofs")
	}
	gotEv, gotProofs, ok := takePendingEvidence("s", 1, hash)
	if !ok {
		t.Fatal("take must find the stored proofs")
	}
	if len(gotEv.Overflows)+len(gotEv.Rejects)+len(gotEv.Conflicts) != 0 {
		t.Fatalf("proof-only entry must carry empty evidence; got %+v", gotEv)
	}
	if len(gotProofs) != 2 ||
		!bytes.Equal(gotProofs[0], proofs[0]) ||
		!bytes.Equal(gotProofs[1], proofs[1]) {
		t.Fatalf("taken proofs do not match stored: %q", gotProofs)
	}
	if _, _, ok := takePendingEvidence("s", 1, hash); ok {
		t.Fatal("take must consume the proof entry")
	}
}

// TestStashPendingCoordinatorProofs_EmptyIsNoOp guards the empty guard: an attempt
// with no retained packages (CoordinatorPackageProofs returned nothing) stashes
// nothing.
func TestStashPendingCoordinatorProofs_EmptyIsNoOp(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	stashPendingCoordinatorProofs("s", 1, stashTestHash(0x62), nil)
	stashPendingCoordinatorProofs("s", 1, stashTestHash(0x62), [][]byte{})
	if PendingEvidenceStashedForTest("s", 1) {
		t.Fatal("empty proofs must not create a stash entry")
	}
}

// TestStashPendingCoordinatorProofs_DeepCopied proves copyProofs isolates the
// stash from later caller mutation of the proof bytes.
func TestStashPendingCoordinatorProofs_DeepCopied(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x63)
	proof := []byte("package-bytes")
	stashPendingCoordinatorProofs("s", 1, hash, [][]byte{proof})

	proof[0] = 'X' // mutate the caller's slice after the store

	_, gotProofs, ok := takePendingEvidence("s", 1, hash)
	if !ok || len(gotProofs) != 1 {
		t.Fatalf("expected one stashed proof; ok=%v got=%q", ok, gotProofs)
	}
	if !bytes.Equal(gotProofs[0], []byte("package-bytes")) {
		t.Fatalf("proof must reflect store-time bytes, not the mutation; got %q", gotProofs[0])
	}
}

// TestPendingEvidenceStash_EvidenceAndProofsUnion asserts the union entry: stashing
// evidence and proofs under the SAME key (which the mutually-exclusive coarse and
// interactive paths normally never do) carries BOTH -- neither writer clobbers the
// other's field (Codex's "never an XOR assumption that drops data").
func TestPendingEvidenceStash_EvidenceAndProofsUnion(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	hash := stashTestHash(0x64)
	stashPendingEvidence("s", 1, hash, attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{9: 1},
	})
	stashPendingCoordinatorProofs("s", 1, hash, [][]byte{[]byte("pkg")})

	gotEv, gotProofs, ok := takePendingEvidence("s", 1, hash)
	if !ok {
		t.Fatal("union entry must exist")
	}
	if gotEv.Overflows[9] != 1 {
		t.Fatalf("proof stash must not clobber the evidence field; got %+v", gotEv.Overflows)
	}
	if len(gotProofs) != 1 || !bytes.Equal(gotProofs[0], []byte("pkg")) {
		t.Fatalf("evidence stash must not clobber the proofs field; got %q", gotProofs)
	}
}
