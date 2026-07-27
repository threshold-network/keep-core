//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	netlocal "github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test closes the second real-crypto-under-failure gap: an INVALID signature
// share, detected by the REAL cgo engine, driving a PERMANENT exclusion. Prior
// coverage was disjoint - the real engine's share-verification and the f+1
// reject-blame exclusion policy (next_attempt.go) were only ever exercised
// separately, the latter with fakes.
//
// Flow (companion to the dropout/transient-park test, which parks; this one
// EXCLUDES):
//	attempt 1 (REAL crypto): a selected signer submits a structurally valid but
//	  cryptographically WRONG round-2 share; the coordinator's real aggregate
//	  fails with a typed share-verification error that NAMES the culprit;
//	NextAttempt (REAL policy): an f+1 reject-accusation quorum against the culprit
//	  PERMANENTLY excludes it (ExcludedSet, not a transient park);
//	attempt 2 (REAL crypto): the surviving subset aggregates a real BIP-340
//	  signature without the excluded member.
//
// Determinism: as in the dropout test, attempt 1 runs only {coordinator, target}
// so the target is necessarily the coordinator's co-signer and its bad share is
// aggregated (not observed away). The reject quorum is supplied from the genuine
// verdict - the target's share IS invalid (the coordinator's real aggregate
// proves it), so every honest observer would reject it; we encode the
// ExclusionAccuserQuorum-many accusers a live deployment would produce.

// corruptingRound2Engine wraps the engine so the wrapped seat's round-2 share is
// mangled after the engine produces it: still a well-formed scalar (low byte
// flipped), but the wrong value. The seat then signs and broadcasts that share,
// modelling an invalid-share submitter. Every other engine call passes through
// the embedded engine unchanged.
type corruptingRound2Engine struct {
	interactiveSigningEngine
}

func (e corruptingRound2Engine) InteractiveRound2(
	sessionID string, attemptID string, memberIdentifier uint16, signingPackage []byte,
) ([]byte, error) {
	share, err := e.interactiveSigningEngine.InteractiveRound2(sessionID, attemptID, memberIdentifier, signingPackage)
	if err != nil || len(share) == 0 {
		return share, err
	}
	corrupted := append([]byte{}, share...)
	corrupted[len(corrupted)-1] ^= 0x01 // valid scalar, wrong value
	return corrupted, nil
}

func TestRealCgoInteractiveSigning_InvalidShareBlameForcesPermanentExclusion(t *testing.T) {
	setupRealCgoSignerState(t)

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-invalid-share-%d", realCgoSessionSeq.Add(1))
	// The production executor owns the aggregate memo session for the outer
	// signing operation; the harness stands in for it here.
	memoSession, err := BeginInteractiveAggregateMemoSession(sessionID)
	if err != nil {
		t.Fatalf("begin aggregate memo session: %v", err)
	}
	t.Cleanup(memoSession.Release)

	const n = 3
	const threshold uint16 = 2
	participantIDs := []byte{1, 2, 3}
	included := []group.MemberIndex{1, 2, 3}

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
	logger := &testutils.MockLogger{}

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

	nonCoordinators := make([]group.MemberIndex, 0, n-1)
	for _, m := range included {
		if m != coordinator {
			nonCoordinators = append(nonCoordinators, m)
		}
	}
	sort.Slice(nonCoordinators, func(i, j int) bool { return nonCoordinators[i] < nonCoordinators[j] })
	target := nonCoordinators[0]  // submits the invalid share
	offline := nonCoordinators[1] // absent in attempt 1, second live accuser + attempt-2 signer
	t.Logf("attempt 1: coordinator=%d target(invalid-share)=%d offline=%d", coordinator, target, offline)

	chainSigning := local_v1.Connect(n, n).Signing()
	publicKeys := make(map[group.MemberIndex]*operator.PublicKey, n)
	addresses := make([]chain.Address, 0, n)
	for _, m := range included {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (seat %d): %v", m, err)
		}
		publicKeys[m] = publicKey
		addresses = append(addresses, chainSigning.PublicKeyBytesToAddress(
			operator.MarshalUncompressed(publicKey),
		))
	}
	validator := group.NewMembershipValidator(&testutils.MockLogger{}, addresses, chainSigning)

	newSeat := func(ctx context.Context, member group.MemberIndex, attemptCtx attempt.AttemptContext, corruptShare bool) *dropoutSeat {
		channel, err := netlocal.ConnectWithKey(publicKeys[member]).
			BroadcastChannelFor("frost-roast-interactive-signing")
		if err != nil {
			t.Fatalf("broadcast channel (seat %d): %v", member, err)
		}
		bus, err := NewBroadcastChannelRunnerBus(ctx, logger, channel, validator)
		if err != nil {
			t.Fatalf("runner bus (seat %d): %v", member, err)
		}
		var eng interactiveSigningEngine = engine
		if corruptShare {
			eng = corruptingRound2Engine{interactiveSigningEngine: engine}
		}
		coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
		handle, err := coord.BeginAttempt(attemptCtx)
		if err != nil {
			t.Fatalf("begin attempt (seat %d): %v", member, err)
		}
		binding, err := NewActiveRoastAttempt(coord, handle, attemptCtx, sessionID, nil, keyGroupSeed)
		if err != nil {
			t.Fatalf("active attempt (seat %d): %v", member, err)
		}
		collector := roast.NewRound2Collector(verifier)
		runner, err := newInteractiveSigningRunner(binding, member, threshold, nil, eng, collector, coord, signer, bus)
		if err != nil {
			t.Fatalf("runner (seat %d): %v", member, err)
		}
		return &dropoutSeat{member: member, coord: coord, handle: handle, binding: binding, runner: runner}
	}

	// ---- Attempt 1: coordinator + invalid-share target; aggregate must name it. ----
	ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()

	coordSeat := newSeat(ctx1, coordinator, attempt1Ctx, false)
	targetSeat := newSeat(ctx1, target, attempt1Ctx, true)
	coordRes := runSeatAsync(coordSeat, ctx1)
	targetRes := runSeatAsync(targetSeat, ctx1)
	<-coordRes.done
	<-targetRes.done

	// The REAL engine's aggregate must detect the invalid share and name the
	// target. Accept the detection from whichever co-resident seat aggregated it
	// (shared engine); both run aggregate and see the same bad share.
	var svErr *InteractiveAggregateShareVerificationError
	switch {
	case errors.As(coordRes.err, &svErr):
	case errors.As(targetRes.err, &svErr):
	default:
		t.Fatalf(
			"attempt 1: want a share-verification failure naming the culprit; coord err=%v target err=%v",
			coordRes.err, targetRes.err,
		)
	}
	if !containsUint16(svErr.CandidateCulprits, uint16(target)) {
		t.Fatalf("aggregate must name target %d as a candidate culprit; culprits=%v", target, svErr.CandidateCulprits)
	}
	t.Logf("attempt 1: real aggregate named culprit(s) %v (target=%d)", svErr.CandidateCulprits, target)

	// ---- Transition: an f+1 reject-accusation quorum against the target. ----
	quorum := roast.ExclusionAccuserQuorum(uint(n), uint(threshold)) // = n - t + 1 = 2
	accusers := []group.MemberIndex{coordinator, offline}
	if uint(len(accusers)) < quorum {
		t.Fatalf("test setup: need >= %d accusers, have %d", quorum, len(accusers))
	}
	sort.Slice(accusers, func(i, j int) bool { return accusers[i] < accusers[j] })

	prevHash := attempt1Ctx.Hash()
	snapshots := make([]roast.LocalEvidenceSnapshot, 0, len(accusers))
	for _, a := range accusers {
		snapshots = append(snapshots, roast.LocalEvidenceSnapshot{
			SenderIDValue:      uint32(a),
			AttemptContextHash: append([]byte{}, prevHash[:]...),
			Rejects: []roast.RejectEntry{{
				Sender: target, // the accused
				Reason: "aggregate_share_verification_failed",
				Count:  1,
			}},
		})
	}
	bundle := &roast.TransitionMessage{
		AttemptContextHash: append([]byte{}, prevHash[:]...),
		CoordinatorIDValue: uint32(coordinator),
		Bundle:             snapshots,
	}

	attempt2Ctx, err := coordSeat.coord.NextAttempt(coordSeat.handle, bundle, uint(threshold), keyGroupSeed)
	if err != nil {
		t.Fatalf("NextAttempt: %v", err)
	}

	// ---- Assert PERMANENT exclusion (not a transient park). ----
	if !containsMember(attempt2Ctx.ExcludedSet, target) {
		t.Fatalf("invalid-share target %d must be PERMANENTLY excluded; excluded=%v", target, attempt2Ctx.ExcludedSet)
	}
	if containsMember(attempt2Ctx.TransientlyParked, target) {
		t.Fatalf("target %d must be excluded, not merely parked; parked=%v", target, attempt2Ctx.TransientlyParked)
	}
	if containsMember(attempt2Ctx.IncludedSet, target) {
		t.Fatalf("excluded target %d must not be in attempt 2's included set %v", target, attempt2Ctx.IncludedSet)
	}
	wantIncluded := []group.MemberIndex{coordinator, offline}
	sort.Slice(wantIncluded, func(i, j int) bool { return wantIncluded[i] < wantIncluded[j] })
	gotIncluded := append([]group.MemberIndex{}, attempt2Ctx.IncludedSet...)
	sort.Slice(gotIncluded, func(i, j int) bool { return gotIncluded[i] < gotIncluded[j] })
	if !memberSlicesEqualLocal(gotIncluded, wantIncluded) {
		t.Fatalf("attempt 2 included set = %v, want %v (culprit excluded)", gotIncluded, wantIncluded)
	}

	// ---- Attempt 2: surviving subset finalizes for real without the culprit. ----
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	var seats2 []*dropoutSeat
	for _, m := range attempt2Ctx.IncludedSet {
		seats2 = append(seats2, newSeat(ctx2, m, attempt2Ctx, false))
	}
	dones := make([]*seatRunResult, len(seats2))
	for i, s := range seats2 {
		dones[i] = runSeatAsync(s, ctx2)
	}
	// With the per-(session,attempt) aggregate memo, every co-resident surviving-subset
	// seat receives the single real engine aggregate's result, so ALL succeed with the
	// identical signature and none surfaces interactive_attempt_already_aggregated (that
	// error now fails the test).
	winners := 0
	var sharedSignature []byte
	for i := range dones {
		<-dones[i].done
		if dones[i].err != nil {
			t.Fatalf("attempt 2 seat %d failed: %v", seats2[i].member, dones[i].err)
		}
		if len(dones[i].sig) != 64 {
			t.Fatalf("attempt 2 seat %d: want a 64-byte signature, got %d", seats2[i].member, len(dones[i].sig))
		}
		if sharedSignature == nil {
			sharedSignature = dones[i].sig
		} else if !bytes.Equal(dones[i].sig, sharedSignature) {
			t.Fatalf("attempt 2 seat %d produced a different signature than another aggregating seat", seats2[i].member)
		}
		winners++
		state, err := seats2[i].coord.State(seats2[i].handle)
		if err != nil {
			t.Fatalf("attempt 2 seat %d state: %v", seats2[i].member, err)
		}
		if state != roast.AttemptStateSucceeded {
			t.Fatalf("attempt 2 seat %d not Succeeded: %v", seats2[i].member, state)
		}
	}
	if winners != len(seats2) {
		t.Fatalf("attempt 2: expected every surviving-subset seat to aggregate the shared signature, got %d of %d", winners, len(seats2))
	}
}

func containsUint16(set []uint16, m uint16) bool {
	for _, x := range set {
		if x == m {
			return true
		}
	}
	return false
}
