//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
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

// This test closes the third real-crypto-under-failure gap: a Byzantine signer
// that DOUBLE-SIGNS its round-2 share (member equivocation), detected over the
// real bus by an honest Round2Collector as EquivocationKindShareConflict. Prior
// coverage was disjoint - the collector's conflict detection
// (round2_collector.go) was unit-tested with synthetic ShareSubmissions and
// fakes, never over a genuine engine-produced FROST share submitted through the
// real transport.
//
// Flow (a companion to the dropout/invalid-share tests, which force a retry;
// this one exercises the BLAME-EVIDENCE path - a share conflict is recorded as
// self-incriminating proof for the f+1 adjudication of Phase 7.2b-4, it does not
// abort the live attempt):
//
//	attempt 1 (REAL crypto): a selected signer runs round 2 and produces a
//	  genuine FROST signature share, then broadcasts TWO body-different signed
//	  share submissions for the SAME attempt - its real share and a re-signed copy
//	  with a mutated signature_share. Both name the elected coordinator and bind
//	  the authoritative signing package, so both are ACCEPTED aggregation shares
//	  (not divergent evidence): a second accepted-but-different signed body is
//	  member double-signing.
//	DETECTION (REAL collector): an honest Round2Collector fed the coordinator's
//	  authoritative package and then both shares flags the second as
//	  EquivocationKindShareConflict and returns ErrShareConflict, and the
//	  process-wide equivocation observer receives the evidence naming the
//	  Byzantine submitter.
//
// DETERMINISM. The Byzantine seat genuinely BROADCASTS both shares on the live
// bus (an honest receiver may also detect the conflict - a bonus we do not rely
// on), but the runner's collect-shares loop stops at the threshold and only
// opportunistically drains buffered duplicates, so which collector observes the
// second share over the wire is timing-dependent. To make DETECTION
// deterministic without depending on that race, the test also drives an
// independent honest collector directly with the exact bytes the Byzantine
// broadcast (captured at the point of broadcast) plus the coordinator's
// authoritative package (captured from its broadcast). Those are the real
// on-wire envelopes - the same bytes an honest node receives - so feeding them
// is behaviourally identical to reception, only order-guaranteed. Attempt 1 runs
// only {coordinator, target} (the deterministic co-signer trick from the sibling
// tests) so the target is necessarily the coordinator's sole co-signer and its
// real round-2 share is actually produced.

// signingPackageCapturingBus wraps a RunnerBus and records a copy of every
// signing-package envelope the wrapped seat broadcasts (the elected
// coordinator's authoritative package), so the test can feed the real,
// coordinator-signed package to an honest auditor collector. All traffic passes
// through unchanged.
type signingPackageCapturingBus struct {
	inner RunnerBus
	mu    *sync.Mutex
	pkgs  *[][]byte
}

func (b signingPackageCapturingBus) Broadcast(msg RunnerMessage) {
	if msg.Type == RunnerMsgSigningPackage {
		b.mu.Lock()
		*b.pkgs = append(*b.pkgs, append([]byte(nil), msg.Payload...))
		b.mu.Unlock()
	}
	b.inner.Broadcast(msg)
}

func (b signingPackageCapturingBus) Subscribe() *RunnerBusSubscriber { return b.inner.Subscribe() }

// shareEquivocatingBus wraps a RunnerBus so the wrapped seat DOUBLE-SIGNS its
// round-2 share: when it broadcasts its genuine share submission, the wrapper
// captures that real envelope, derives a body-different conflicting one (same
// attempt/coordinator/package binding, one bit flipped in the signature_share,
// re-signed by the same seat), captures it too, and broadcasts BOTH. Keeping the
// coordinator and signing-package binding identical is what makes the second
// share an ACCEPTED conflict (member double-signing) rather than a divergent
// share. All other traffic passes through untouched.
type shareEquivocatingBus struct {
	inner    RunnerBus
	signer   roast.Signer
	mu       *sync.Mutex
	real     *[][]byte
	conflict *[][]byte
	buildErr *[]error
}

func (b shareEquivocatingBus) Broadcast(msg RunnerMessage) {
	if msg.Type != RunnerMsgShareSubmission {
		b.inner.Broadcast(msg)
		return
	}
	realEnvelope := append([]byte(nil), msg.Payload...)
	conflictEnvelope, err := buildConflictingShareEnvelope(realEnvelope, b.signer)

	b.mu.Lock()
	*b.real = append(*b.real, realEnvelope)
	if err != nil {
		*b.buildErr = append(*b.buildErr, err)
	} else {
		*b.conflict = append(*b.conflict, conflictEnvelope)
	}
	b.mu.Unlock()

	// The real share first, then the body-different double-sign. Both are
	// delivered by the bus (it never dedups body-different messages from a
	// sender - that suppression would destroy the very equivocation evidence).
	b.inner.Broadcast(msg)
	if err == nil {
		b.inner.Broadcast(RunnerMessage{
			Type:    RunnerMsgShareSubmission,
			Sender:  msg.Sender,
			Attempt: msg.Attempt,
			Payload: conflictEnvelope,
		})
	}
}

func (b shareEquivocatingBus) Subscribe() *RunnerBusSubscriber { return b.inner.Subscribe() }

// buildConflictingShareEnvelope derives a second, body-different signed share
// submission from a genuine one: it keeps the attempt, submitter, coordinator,
// and signing-package binding (so an honest collector classifies it as an
// ACCEPTED share, eligible for aggregation - the prerequisite for the conflict,
// not a divergent share) and flips one bit of the FROST signature_share so the
// signed BODY differs. It re-signs with the same seat's signer, modelling a
// member that authored two different shares for one instruction.
func buildConflictingShareEnvelope(realEnvelope []byte, signer roast.Signer) ([]byte, error) {
	var real roast.ShareSubmission
	if err := real.Unmarshal(realEnvelope); err != nil {
		return nil, fmt.Errorf("unmarshal real share: %w", err)
	}
	if len(real.SignatureShare) == 0 {
		return nil, errors.New("real share has an empty signature share")
	}
	conflict := &roast.ShareSubmission{
		AttemptContextHash: append([]byte(nil), real.AttemptContextHash...),
		SubmitterIDValue:   real.SubmitterIDValue,
		CoordinatorIDValue: real.CoordinatorIDValue,
		SigningPackageHash: append([]byte(nil), real.SigningPackageHash...),
		SignatureShare:     append([]byte(nil), real.SignatureShare...),
	}
	conflict.SignatureShare[len(conflict.SignatureShare)-1] ^= 0x01
	payload, err := conflict.SignableBytes()
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		return nil, err
	}
	conflict.SubmitterSignature = sig
	envelope, err := conflict.Marshal()
	if err != nil {
		return nil, err
	}
	return envelope, nil
}

func TestRealCgoInteractiveSigning_DoubleSignedShareIsDetectedAsEquivocation(t *testing.T) {
	setupRealCgoSignerState(t)

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-share-conflict-%d", realCgoSessionSeq.Add(1))

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

	// Capture the equivocation evidence the collector emits process-wide. Only a
	// single observer is supported; clear any stale registration first so serial
	// tests in this package do not collide, and unregister on exit.
	var evMu sync.Mutex
	var conflictEvents []roast.EquivocationEvidence
	roast.UnregisterEquivocationEvidenceObserver()
	if err := roast.RegisterEquivocationEvidenceObserver(func(ev roast.EquivocationEvidence) {
		evMu.Lock()
		defer evMu.Unlock()
		conflictEvents = append(conflictEvents, ev)
	}); err != nil {
		t.Fatalf("register equivocation observer: %v", err)
	}
	defer roast.UnregisterEquivocationEvidenceObserver()

	// Resolve attempt 1's elected coordinator from a probe binding, so the
	// non-coordinator target is necessarily the coordinator's co-signer.
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
	target := nonCoordinators[0] // double-signs its round-2 share
	t.Logf("attempt 1: coordinator=%d target(double-signer)=%d", coordinator, target)

	// Shared operator identities + membership validator over all 3 seats.
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

	var (
		captureMu        sync.Mutex
		capturedPackages [][]byte
		capturedReal     [][]byte
		capturedConflict [][]byte
		buildErrs        []error
	)

	newSeat := func(ctx context.Context, member group.MemberIndex) *dropoutSeat {
		channel, err := netlocal.ConnectWithKey(publicKeys[member]).
			BroadcastChannelFor("frost-roast-interactive-signing")
		if err != nil {
			t.Fatalf("broadcast channel (seat %d): %v", member, err)
		}
		var bus RunnerBus
		bus, err = NewBroadcastChannelRunnerBus(ctx, logger, channel, validator)
		if err != nil {
			t.Fatalf("runner bus (seat %d): %v", member, err)
		}
		switch member {
		case coordinator:
			bus = signingPackageCapturingBus{inner: bus, mu: &captureMu, pkgs: &capturedPackages}
		case target:
			bus = shareEquivocatingBus{
				inner:    bus,
				signer:   signer,
				mu:       &captureMu,
				real:     &capturedReal,
				conflict: &capturedConflict,
				buildErr: &buildErrs,
			}
		}
		coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
		handle, err := coord.BeginAttempt(attempt1Ctx)
		if err != nil {
			t.Fatalf("begin attempt (seat %d): %v", member, err)
		}
		binding, err := NewActiveRoastAttempt(coord, handle, attempt1Ctx, sessionID, nil, keyGroupSeed)
		if err != nil {
			t.Fatalf("active attempt (seat %d): %v", member, err)
		}
		collector := roast.NewRound2Collector(verifier)
		runner, err := newInteractiveSigningRunner(binding, member, threshold, nil, engine, collector, coord, signer, bus)
		if err != nil {
			t.Fatalf("runner (seat %d): %v", member, err)
		}
		return &dropoutSeat{member: member, coord: coord, handle: handle, binding: binding, runner: runner}
	}

	// ---- Attempt 1: coordinator + double-signing target. ----
	ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()

	coordSeat := newSeat(ctx1, coordinator)
	targetSeat := newSeat(ctx1, target)
	coordRes := runSeatAsync(coordSeat, ctx1)
	targetRes := runSeatAsync(targetSeat, ctx1)
	<-coordRes.done
	<-targetRes.done
	t.Logf("attempt 1 outcomes: coordinator err=%v sigLen=%d, target err=%v sigLen=%d",
		coordRes.err, len(coordRes.sig), targetRes.err, len(targetRes.sig))

	captureMu.Lock()
	packages := append([][]byte{}, capturedPackages...)
	realShares := append([][]byte{}, capturedReal...)
	conflictShares := append([][]byte{}, capturedConflict...)
	buildErrsCopy := append([]error{}, buildErrs...)
	captureMu.Unlock()

	if len(buildErrsCopy) != 0 {
		t.Fatalf("building the conflicting share failed: %v", buildErrsCopy)
	}

	// FAULT REACHED (crypto side): the target actually ran round 2 and produced a
	// genuine FROST share, then broadcast it. Without a captured real share, the
	// equivocation would be synthetic and the test vacuous.
	if len(realShares) == 0 {
		t.Fatalf("target %d never broadcast a round-2 share; its equivocation was not reached", target)
	}
	if len(conflictShares) == 0 {
		t.Fatalf("no conflicting share was derived from the target's real share")
	}
	if len(packages) == 0 {
		t.Fatalf("coordinator %d never broadcast an authoritative signing package", coordinator)
	}

	realEnvelope := realShares[0]
	conflictEnvelope := conflictShares[0]

	// FAULT REACHED (equivocation is genuine): two byte-different signed share
	// envelopes, both parseable, both binding the SAME attempt / coordinator /
	// authoritative package - so both are ACCEPTED shares and the second is a true
	// double-sign, not a divergent share that would be classified differently.
	if bytes.Equal(realEnvelope, conflictEnvelope) {
		t.Fatalf("the two share envelopes are byte-identical; no equivocation was produced")
	}
	var realSub, conflictSub roast.ShareSubmission
	if err := realSub.Unmarshal(realEnvelope); err != nil {
		t.Fatalf("unmarshal real share: %v", err)
	}
	if err := conflictSub.Unmarshal(conflictEnvelope); err != nil {
		t.Fatalf("unmarshal conflicting share: %v", err)
	}
	if realSub.SubmitterID() != target || conflictSub.SubmitterID() != target {
		t.Fatalf("both shares must be submitted by target %d; got real=%d conflict=%d",
			target, realSub.SubmitterID(), conflictSub.SubmitterID())
	}
	if realSub.CoordinatorID() != coordinator || conflictSub.CoordinatorID() != coordinator {
		t.Fatalf("both shares must name coordinator %d; got real=%d conflict=%d",
			coordinator, realSub.CoordinatorID(), conflictSub.CoordinatorID())
	}
	if !bytes.Equal(realSub.SigningPackageHash, conflictSub.SigningPackageHash) {
		t.Fatalf("both shares must bind the same authoritative package (else it is a divergent share, not a conflict)")
	}
	if bytes.Equal(realSub.SignatureShare, conflictSub.SignatureShare) {
		t.Fatalf("the two shares must differ in their signature_share (the double-signed field)")
	}

	// ---- Deterministic detection: an honest collector fed the authoritative
	// package and both shares must flag the second as a member conflict. ----
	pkg := &roast.SigningPackage{}
	if err := pkg.Unmarshal(packages[0]); err != nil {
		t.Fatalf("unmarshal authoritative signing package: %v", err)
	}
	// The shares must answer THIS captured package, or they would be divergent.
	pkgBodyHash, err := pkg.BodyHash()
	if err != nil {
		t.Fatalf("authoritative package body hash: %v", err)
	}
	if !bytes.Equal(realSub.SigningPackageHash, pkgBodyHash[:]) {
		t.Fatalf("the target's shares do not bind the coordinator's authoritative package; they would be divergent, not a conflict")
	}

	prevHash := attempt1Ctx.Hash()
	auditor := roast.NewRound2Collector(verifier)
	if err := auditor.BeginAttempt(prevHash[:], coordinator, included); err != nil {
		t.Fatalf("auditor begin attempt: %v", err)
	}
	if err := auditor.RecordSigningPackage(pkg); err != nil {
		t.Fatalf("auditor record authoritative package: %v", err)
	}
	// The genuine share is accepted; the second, body-different accepted share
	// from the same submitter is member double-signing.
	if err := auditor.RecordShareSubmission(&realSub); err != nil {
		t.Fatalf("auditor must accept the target's genuine share; got: %v", err)
	}
	conflictErr := auditor.RecordShareSubmission(&conflictSub)
	if !errors.Is(conflictErr, roast.ErrShareConflict) {
		t.Fatalf("auditor must reject the double-signed share as a conflict (ErrShareConflict); got: %v", conflictErr)
	}

	// ---- The equivocation evidence names the culprit. ----
	evMu.Lock()
	events := append([]roast.EquivocationEvidence{}, conflictEvents...)
	evMu.Unlock()

	var conflictEvidence *roast.EquivocationEvidence
	for i := range events {
		if events[i].Kind == roast.EquivocationKindShareConflict && events[i].Sender == target {
			conflictEvidence = &events[i]
			break
		}
	}
	if conflictEvidence == nil {
		t.Fatalf("expected a %s equivocation event naming submitter %d; got events=%v",
			roast.EquivocationKindShareConflict, target, events)
	}
	// The evidence must carry the two distinct signed envelopes (the proof
	// material an f+1 adjudication compares), not empty placeholders.
	if len(conflictEvidence.ExistingEnvelope) == 0 || len(conflictEvidence.ConflictingEnvelope) == 0 {
		t.Fatalf("conflict evidence must carry both signed envelopes; got existing=%d conflicting=%d bytes",
			len(conflictEvidence.ExistingEnvelope), len(conflictEvidence.ConflictingEnvelope))
	}
	if bytes.Equal(conflictEvidence.ExistingEnvelope, conflictEvidence.ConflictingEnvelope) {
		t.Fatalf("conflict evidence envelopes must differ (self-incriminating double-sign)")
	}
	t.Logf("honest collector detected member double-signing: %s from submitter %d (existing=%d conflicting=%d bytes)",
		conflictEvidence.Kind, conflictEvidence.Sender,
		len(conflictEvidence.ExistingEnvelope), len(conflictEvidence.ConflictingEnvelope))
}
