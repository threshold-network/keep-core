//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"fmt"
	"sort"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test closes the fifth real-under-failure gap: a member that FLOODS a
// receive loop faster than it drains genuinely OVERFLOWS the bounded inbound
// channel, and that real, recorded overflow evidence - carried by an f+1 quorum
// of honest observers - drives a TRANSIENT PARK of the flooder in the next
// attempt (transport pressure costs one attempt of liveness, then the member
// rejoins), NOT a permanent exclusion. Prior coverage was disjoint: the overflow
// primitive (enqueueOrRecordOverflow, evidence_overflow.go) was unit-tested with
// synthetic recorder-count assertions, and the park policy (next_attempt.go) was
// unit-tested with hand-authored OverflowEntry bundles - never wired together, so
// the overflow evidence the policy parked on was never the evidence the real
// primitive produced.
//
// This wires them end to end:
//
//	OVERFLOW (real primitive): two honest observers each drive the REAL
//	  enqueueOrRecordOverflow against a REAL full bounded channel with messages
//	  from the flooder; each channel rejects the enqueue and each observer's REAL
//	  bounded recorder records overflow against the flooder.
//	PARK (real policy): each observer's recorded Evidence is turned into a REAL
//	  LocalEvidenceSnapshot; an f+1 overflow-accusation quorum drives NextAttempt
//	  to TRANSIENTLY PARK the flooder (not exclude it). The flooder is itself a
//	  bundle sender, so silence-parking cannot account for the park - only the
//	  overflow quorum can.
//	REINSTATE (transient proof): a following attempt with no accusations
//	  re-includes the parked flooder, proving the park is transient, not permanent
//	  exclusion.
//
// A quorum-gate negative control (a single overflow accuser, below f+1) does NOT
// park the flooder, proving the quorum is genuinely enforced.
//
// Determinism: overflow is a transport phenomenon adjudicated in pure Go - no
// FROST rounds, bus, or shared engine - so the flood is driven synchronously
// against a pre-filled channel (every enqueue after the first deterministically
// hits the overflow branch) and the policy is a pure function of the bundle. A
// real DKG is run only for an authentic attempt context, as in the sibling
// real-cgo tests; it is not load-bearing for the overflow/park logic.

// recordRealOverflow drives the REAL enqueueOrRecordOverflow primitive against a
// REAL pre-filled bounded channel `floodCount` times with messages from
// `flooder`, and returns the observer's genuinely-recorded evidence plus the
// number of enqueues the full channel rejected. Every attempt after the channel
// is filled falls to the overflow branch, so the recorder observes real,
// primitive-produced overflow - not a hand-set counter.
func recordRealOverflow(t *testing.T, flooder group.MemberIndex, floodCount int) (attempt.Evidence, int) {
	t.Helper()
	ch := make(chan *buildTaggedTBTCSignerRoundContributionMessage, 1)
	ch <- &buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: 0xff} // fill it; no consumer drains
	recorder := attempt.NewBoundedRecorder()

	rejected := 0
	for i := 0; i < floodCount; i++ {
		enqueued := enqueueOrRecordOverflow(
			&buildTaggedTBTCSignerRoundContributionMessage{SenderIDValue: uint32(flooder)},
			ch,
			recorder,
		)
		if !enqueued {
			rejected++
		}
	}
	return recorder.Snapshot(), rejected
}

// overflowCountFor returns the OverflowEntry count a snapshot reports against
// `accused` (0 if none), and whether such an entry exists.
func overflowCountFor(snap *roast.LocalEvidenceSnapshot, accused group.MemberIndex) (uint, bool) {
	for _, e := range snap.Overflows {
		if e.Sender == accused {
			return e.Count, true
		}
	}
	return 0, false
}

func TestRealCgoInteractiveSigning_TransportOverflowTransientlyParksFlooder(t *testing.T) {
	setupRealCgoSignerState(t)

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-overflow-park-%d", realCgoSessionSeq.Add(1))

	const n = 3
	const threshold uint16 = 2
	participantIDs := []byte{1, 2, 3}
	included := []group.MemberIndex{1, 2, 3}

	// Authentic attempt context (family consistency; not load-bearing here).
	keyGroup := runRealCgoDKGKeyGroup(t, engine, sessionID, participantIDs, threshold)
	keyGroupSeed := []byte(keyGroup)

	var messageDigest [attempt.MessageDigestLength]byte
	for i := range messageDigest {
		messageDigest[i] = 0x42
	}
	attempt1Ctx, err := attempt.NewAttemptContext(
		sessionID, keyGroup, keyGroupSeed, messageDigest, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt 1 context: %v", err)
	}

	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()

	// Resolve the deterministic elected coordinator so the flooder is a
	// non-coordinator (matches the sibling structure; the overflow park itself is
	// coordinator-agnostic).
	probeCoord := roast.NewInMemoryCoordinatorWithSigning(1, signer, verifier)
	probeHandle, err := probeCoord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	probeBinding, err := NewActiveRoastAttempt(probeCoord, probeHandle, attempt1Ctx, sessionID, nil, keyGroupSeed)
	if err != nil {
		t.Fatalf("probe binding: %v", err)
	}
	coordinator := probeBinding.ElectedCoordinator()
	nonCoord := make([]group.MemberIndex, 0, n-1)
	for _, m := range included {
		if m != coordinator {
			nonCoord = append(nonCoord, m)
		}
	}
	sort.Slice(nonCoord, func(i, j int) bool { return nonCoord[i] < nonCoord[j] })
	flooder := nonCoord[0]
	// The two honest overflow accusers (f+1 = n-t+1 = 2 for n=3, t=2).
	observers := []group.MemberIndex{coordinator, nonCoord[1]}
	t.Logf("coordinator=%d flooder=%d observers=%v", coordinator, flooder, observers)

	quorum := roast.ExclusionAccuserQuorum(uint(n), uint(threshold))
	if uint(len(observers)) < quorum {
		t.Fatalf("test setup: need >= %d overflow accusers, have %d", quorum, len(observers))
	}

	prevHash := attempt1Ctx.Hash()

	// ---- OVERFLOW: each observer genuinely overflows the real bounded channel. ----
	const floodCount = 12 // > OverflowQuotaDefault (8); the channel holds 1 and never drains
	observerSnaps := make([]roast.LocalEvidenceSnapshot, 0, len(observers))
	for _, obs := range observers {
		evidence, rejected := recordRealOverflow(t, flooder, floodCount)

		// FAULT REACHED: the channel really rejected enqueues, and the real
		// recorder really recorded overflow against the flooder - not a
		// hand-authored entry.
		if rejected == 0 {
			t.Fatalf("observer %d: the full channel never rejected an enqueue; overflow was not reached", obs)
		}
		if got := evidence.Overflows[flooder]; got == 0 {
			t.Fatalf("observer %d: recorder shows no overflow against flooder %d", obs, flooder)
		}
		snap := roast.NewLocalEvidenceSnapshot(obs, prevHash, evidence)
		// The real recorded overflow must have propagated into the wire snapshot.
		count, ok := overflowCountFor(snap, flooder)
		if !ok || count == 0 {
			t.Fatalf("observer %d snapshot must carry an overflow entry (count>0) against flooder %d; overflows=%v",
				obs, flooder, snap.Overflows)
		}
		observerSnaps = append(observerSnaps, *snap)
	}

	// The flooder is ALSO a bundle sender (an empty self-snapshot), so it cannot
	// be silence-parked: any park it receives is due to the overflow quorum alone.
	flooderSnap := roast.NewLocalEvidenceSnapshot(flooder, prevHash, attempt.Evidence{})

	buildBundle := func(snaps []roast.LocalEvidenceSnapshot) *roast.TransitionMessage {
		sorted := append([]roast.LocalEvidenceSnapshot{}, snaps...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].SenderIDValue < sorted[j].SenderIDValue })
		return &roast.TransitionMessage{
			AttemptContextHash: append([]byte(nil), prevHash[:]...),
			CoordinatorIDValue: uint32(coordinator),
			Bundle:             sorted,
		}
	}

	bundle := buildBundle(append(append([]roast.LocalEvidenceSnapshot{}, observerSnaps...), *flooderSnap))

	// ---- PARK: an f+1 overflow quorum transiently parks the flooder. ----
	coord := roast.NewInMemoryCoordinatorWithSigning(coordinator, signer, verifier)
	handle, err := coord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	attempt2Ctx, err := coord.NextAttempt(handle, bundle, uint(threshold), keyGroupSeed)
	if err != nil {
		t.Fatalf("NextAttempt: %v", err)
	}

	if !containsMember(attempt2Ctx.TransientlyParked, flooder) {
		t.Fatalf("flooder %d must be transiently parked; parked=%v", flooder, attempt2Ctx.TransientlyParked)
	}
	if containsMember(attempt2Ctx.ExcludedSet, flooder) {
		t.Fatalf("overflow must PARK, not permanently exclude; flooder %d in excluded=%v", flooder, attempt2Ctx.ExcludedSet)
	}
	if containsMember(attempt2Ctx.IncludedSet, flooder) {
		t.Fatalf("parked flooder %d must not be in attempt 2's included set %v", flooder, attempt2Ctx.IncludedSet)
	}
	if attempt2Ctx.AttemptNumber != attempt1Ctx.AttemptNumber+1 {
		t.Fatalf("attempt 2 number = %d, want %d", attempt2Ctx.AttemptNumber, attempt1Ctx.AttemptNumber+1)
	}
	t.Logf("flooder %d transiently parked (excluded=%v included=%v parked=%v)",
		flooder, attempt2Ctx.ExcludedSet, attempt2Ctx.IncludedSet, attempt2Ctx.TransientlyParked)

	// ---- REINSTATE: the following attempt with no accusations re-includes the
	// parked flooder, proving the park is transient (not permanent exclusion). ----
	prevHash2 := attempt2Ctx.Hash()
	reinstateSnaps := make([]roast.LocalEvidenceSnapshot, 0, len(attempt2Ctx.IncludedSet))
	for _, m := range attempt2Ctx.IncludedSet {
		reinstateSnaps = append(reinstateSnaps, *roast.NewLocalEvidenceSnapshot(m, prevHash2, attempt.Evidence{}))
	}
	sort.Slice(reinstateSnaps, func(i, j int) bool { return reinstateSnaps[i].SenderIDValue < reinstateSnaps[j].SenderIDValue })
	reinstateBundle := &roast.TransitionMessage{
		AttemptContextHash: append([]byte(nil), prevHash2[:]...),
		CoordinatorIDValue: uint32(coordinator),
		Bundle:             reinstateSnaps,
	}
	handle2, err := coord.BeginAttempt(attempt2Ctx)
	if err != nil {
		t.Fatalf("begin attempt (reinstate): %v", err)
	}
	attempt3Ctx, err := coord.NextAttempt(handle2, reinstateBundle, uint(threshold), keyGroupSeed)
	if err != nil {
		t.Fatalf("NextAttempt (reinstate): %v", err)
	}
	if !containsMember(attempt3Ctx.IncludedSet, flooder) {
		t.Fatalf("transiently-parked flooder %d must rejoin the following attempt; included=%v parked=%v",
			flooder, attempt3Ctx.IncludedSet, attempt3Ctx.TransientlyParked)
	}
	if containsMember(attempt3Ctx.TransientlyParked, flooder) {
		t.Fatalf("flooder %d must not remain parked after reinstatement; parked=%v", flooder, attempt3Ctx.TransientlyParked)
	}
	t.Logf("flooder %d reinstated on attempt 3 (included=%v) - the park was transient", flooder, attempt3Ctx.IncludedSet)

	// ---- QUORUM GATE (negative control): a single overflow accuser (below f+1)
	// does NOT park the flooder. Same real overflow evidence, one accuser. ----
	singleEvidence, rejected := recordRealOverflow(t, flooder, floodCount)
	if rejected == 0 || singleEvidence.Overflows[flooder] == 0 {
		t.Fatalf("single-accuser control: overflow evidence was not genuinely produced")
	}
	singleAccuserBundle := buildBundle([]roast.LocalEvidenceSnapshot{
		*roast.NewLocalEvidenceSnapshot(observers[0], prevHash, singleEvidence), // lone accuser
		*roast.NewLocalEvidenceSnapshot(observers[1], prevHash, attempt.Evidence{}),
		*flooderSnap,
	})
	handle3, err := coord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("begin attempt (quorum gate): %v", err)
	}
	singleAccuserCtx, err := coord.NextAttempt(handle3, singleAccuserBundle, uint(threshold), keyGroupSeed)
	if err != nil {
		t.Fatalf("NextAttempt (quorum gate): %v", err)
	}
	if containsMember(singleAccuserCtx.TransientlyParked, flooder) {
		t.Fatalf("one overflow accuser is below the f+1 quorum (%d); flooder %d must NOT be parked; parked=%v",
			quorum, flooder, singleAccuserCtx.TransientlyParked)
	}
	if !containsMember(singleAccuserCtx.IncludedSet, flooder) {
		t.Fatalf("un-parked flooder %d must remain in the included set %v", flooder, singleAccuserCtx.IncludedSet)
	}
	t.Logf("quorum gate holds: one accuser (< f+1=%d) did not park flooder %d", quorum, flooder)
}
