//go:build frost_native && frost_roast_retry

package signing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// noopBroadcastChannel is a net.BroadcastChannel that never delivers a message.
// The drive's happy-path test runs a 1-of-1 attempt, where the sole member
// records its own commitment and share and aggregates without awaiting any peer
// - so Send/Recv are exercised but no inbound delivery is needed. The channel's
// own behaviour (auth, demux, dedup, overflow) is covered by the bus net tests.
type noopBroadcastChannel struct{}

func (noopBroadcastChannel) Name() string { return "interactive-drive-test" }
func (noopBroadcastChannel) Send(context.Context, net.TaggedMarshaler, ...net.RetransmissionStrategy) error {
	return nil
}
func (noopBroadcastChannel) Recv(context.Context, func(net.Message))     {}
func (noopBroadcastChannel) SetUnmarshaler(func() net.TaggedUnmarshaler) {}
func (noopBroadcastChannel) SetFilter(net.BroadcastChannelFilter) error  { return nil }

func singleSeatValidator(t *testing.T) *group.MembershipValidator {
	t.Helper()
	sgn := local_v1.Connect(1, 1).Signing()
	_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		t.Fatalf("generate operator key: %v", err)
	}
	addr := sgn.PublicKeyBytesToAddress(operator.MarshalUncompressed(publicKey))
	return group.NewMembershipValidator(&testutils.MockLogger{}, []chain.Address{addr}, sgn)
}

// driveFixture is a consistent single-node (1-of-1) interactive signing setup: a
// registered coordinator, the attempt handle that coordinator minted, and a
// request whose persisted material's DKG public key matches the attempt-context
// seed. The handle + context are passed to the drive directly (as the executor
// entry does with the per-Execute handle), so no session-handle registry or
// readiness gate is involved. The fixture does NOT set the interactive audit
// gate or register the engine provider; each test opts into those so the
// front-door behaviour is isolated.
type driveFixture struct {
	request    *NativeExecutionFFISigningRequest
	engine     *fakeInteractiveSigningEngine
	handle     roast.AttemptHandle
	attemptCtx attempt.AttemptContext
}

func newDriveFixture(t *testing.T) driveFixture {
	t.Helper()

	const (
		// attemptSessionID is the attempt-specific (coarse) id; roastSessionID is
		// the STABLE one BuildAttemptContextFromRequest puts in ctx.SessionID.
		// They DIFFER, mirroring production, so the drive must bind the attempt
		// to ctx.SessionID (not request.SessionID, which NewActiveRoastAttempt
		// would reject).
		attemptSessionID = "interactive-attempt-session-1"
		roastSessionID   = "interactive-roast-session"
		keyGroup         = "interactive-key-group"
	)
	dkgKey := []byte(keyGroup)
	included := []group.MemberIndex{1}

	attemptCtx, err := attempt.NewAttemptContext(
		roastSessionID, keyGroup, dkgKey,
		[attempt.MessageDigestLength]byte{0x42}, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}

	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	coord := roast.NewInMemoryCoordinatorWithSigning(1, signer, verifier)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      signer,
		Verifier:    verifier,
		SelfMember:  1,
		// The drive looks up by attemptCtx.KeyGroupID, which the fixture's signer
		// material yields as keyGroup; register under the same handle so the
		// wallet-scoped lookup resolves.
		KeyGroupID: keyGroup,
	})
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetInteractiveSigningEngineProviderForTest)
	// Every drive fixture reuses the same (roastSessionID, attemptSessionID), so it
	// maps to one process-global aggregate-memo key. Clear it between drive tests or
	// a prior case's memoized success leaks in and suppresses this case's own engine
	// aggregate (e.g. an aggregateErr case would never reach its error). Real nodes
	// mint a unique session id per attempt, so this collision is test-only.
	t.Cleanup(ResetInteractiveAggregateMemoForTest)

	// The production executor owns the aggregate memo session for the outer
	// signing operation; the drive fixture stands in for it here.
	memoSession, err := BeginInteractiveAggregateMemoSession(roastSessionID)
	if err != nil {
		t.Fatalf("begin aggregate memo session: %v", err)
	}
	t.Cleanup(memoSession.Release)

	// The handle is minted by the registered coordinator - exactly the handle
	// the executor entry threads into the drive for this Execute.
	handle, err := coord.BeginAttempt(attemptCtx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}

	engine := newFakeInteractiveSigningEngine()
	// A real engine derives the same coordinator the binding elected; the sole
	// member (1) is the elected coordinator for a 1-of-1 attempt.
	engine.coordinatorIdentifier = 1

	request := &NativeExecutionFFISigningRequest{
		SessionID:           attemptSessionID,
		RoastSessionID:      roastSessionID,
		MemberIndex:         1,
		Channel:             noopBroadcastChannel{},
		MembershipValidator: singleSeatValidator(t),
		SignerMaterial:      persistedTBTCSignerMaterial(t, keyGroup, 1, 1),
	}

	return driveFixture{request: request, engine: engine, handle: handle, attemptCtx: attemptCtx}
}

func validBIP340Signature(t *testing.T) []byte {
	t.Helper()
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("private key: %v", err)
	}
	sig, err := schnorr.Sign(priv, make([]byte, 32))
	if err != nil {
		t.Fatalf("schnorr sign: %v", err)
	}
	return sig.Serialize()
}

func (f driveFixture) run(t *testing.T) (*frost.Signature, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return driveInteractiveRoastSigningIfEnabled(
		ctx, &testutils.MockLogger{}, f.request, f.handle, f.attemptCtx,
	)
}

func TestDriveInteractiveRoastSigning_HappyPath(t *testing.T) {
	f := newDriveFixture(t)
	want := validBIP340Signature(t)
	f.engine.signature = want
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	sig, err := f.run(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil {
		t.Fatal("expected a signature on the interactive path")
	}
	serialized := sig.Serialize()
	if string(serialized[:]) != string(want) {
		t.Fatalf("signature mismatch:\n got %x\nwant %x", serialized[:], want)
	}
}

func TestDriveInteractiveRoastSigning_GateOffFallsBackToCoarse(t *testing.T) {
	f := newDriveFixture(t)
	// Everything is ready (handle minted, engine registered) EXCEPT the audit
	// gate, proving the gate alone gates the interactive path.
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })

	sig, err := f.run(t)
	if err != nil || sig != nil {
		t.Fatalf("expected coarse fallback (nil,nil), got sig=%v err=%v", sig, err)
	}
}

func TestDriveInteractiveRoastSigning_NoEngineFallsBackToCoarse(t *testing.T) {
	f := newDriveFixture(t)
	// Gate on, handle minted, but no engine registered.
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	sig, err := f.run(t)
	if err != nil || sig != nil {
		t.Fatalf("expected coarse fallback (nil,nil), got sig=%v err=%v", sig, err)
	}
}

func TestDriveInteractiveRoastSigning_RunnerFailureHardFails(t *testing.T) {
	f := newDriveFixture(t)
	// The node has COMMITTED to interactive signing (gate on, engine present);
	// a runner failure must propagate as an error so the outer tBTC
	// signingRetryLoop advances, NOT silently drop to coarse.
	f.engine.aggregateErr = fmt.Errorf("aggregate share verification failed")
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	sig, err := f.run(t)
	if err == nil {
		t.Fatal("expected a hard-fail error on runner failure")
	}
	if sig != nil {
		t.Fatalf("expected no signature on failure, got %v", sig)
	}
}

// TestDriveInteractiveRoastSigning_StashesCoordinatorProofsOnFailure covers the
// RFC-21 Phase 7.3 PR2b-2 step 2b extraction seam end-to-end through the real
// drive: when a committed interactive attempt fails AFTER the runner recorded the
// coordinator-signed package, the drive surfaces collector.CoordinatorPackageProofs
// and stashes them so BroadcastForcedSnapshot can carry them into the bundle (where
// NextAttempt's cross-observer tally adjudicates coordinator equivocation). A 1-of-1
// attempt retains only its own authoritative package (no equivocation), so exactly
// one proof is stashed -- proving the seam fires through the real runner+collector,
// not a directly-seeded stash.
func TestDriveInteractiveRoastSigning_StashesCoordinatorProofsOnFailure(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	f := newDriveFixture(t)
	// Fail at aggregation -- the runner records the coordinator's signing package
	// (obtainSigningPackage -> RecordSigningPackage) BEFORE InteractiveAggregate, so
	// the collector still holds it when the drive extracts on failure.
	f.engine.aggregateErr = fmt.Errorf("aggregate share verification failed")
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	if _, err := f.run(t); err == nil {
		t.Fatal("expected a hard-fail error on runner failure")
	}

	// The proofs are stashed under the attempt's (RoastSessionID==ctx.SessionID,
	// member, attemptHash) -- the same key BroadcastForcedSnapshot reads.
	evidence, proofs, ok := takePendingEvidence(f.attemptCtx.SessionID, 1, f.attemptCtx.Hash())
	if !ok {
		t.Fatal("a committed interactive failure must stash the retained coordinator package proof")
	}
	if len(proofs) != 1 {
		t.Fatalf("a 1-of-1 attempt retains exactly its own authoritative package; got %d proofs", len(proofs))
	}
	if len(evidence.Overflows)+len(evidence.Rejects)+len(evidence.Conflicts) != 0 {
		t.Fatalf("interactive failure must stash proofs only, no coarse evidence; got %+v", evidence)
	}
}

// verifierCapableFakeEngine wraps the fake interactive engine with a configurable
// share re-verification verdict, so it ALSO satisfies Round2ShareVerifyingEngine
// (the plain fake does not). Used to drive the share-blame path (RFC-21 Phase 7.3).
type verifierCapableFakeEngine struct {
	*fakeInteractiveSigningEngine
	shareVerdict NativeShareVerificationVerdict
	shareErr     error
}

func (e *verifierCapableFakeEngine) VerifySignatureShare(
	_ string, _ []byte, _ []byte, _ uint16, _ *[32]byte,
) (NativeShareVerificationVerdict, error) {
	return e.shareVerdict, e.shareErr
}

// TestDriveInteractiveRoastSigning_SkipsShareBlameWithoutVerifierEngine asserts the
// share-blame path fails safe when the engine cannot re-verify shares: the plain
// fake engine does not implement Round2ShareVerifyingEngine, so the type-assert in
// the drive skips classification. The 2b coordinator-proof stash still fires (a
// 1-of-1 attempt retains its authoritative package), so the entry exists but
// carries NO reject evidence -- no false blame from a missing verifier.
func TestDriveInteractiveRoastSigning_SkipsShareBlameWithoutVerifierEngine(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	f := newDriveFixture(t)
	f.engine.aggregateErr = &InteractiveAggregateShareVerificationError{CandidateCulprits: []uint16{1}}
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	if _, err := f.run(t); err == nil {
		t.Fatal("expected a hard-fail error on share-verification failure")
	}

	evidence, proofs, ok := takePendingEvidence(f.attemptCtx.SessionID, 1, f.attemptCtx.Hash())
	if !ok {
		t.Fatal("the 2b proof stash should still produce an entry")
	}
	if len(evidence.Rejects) != 0 {
		t.Fatalf("share-blame must be skipped without a verifier engine; got rejects %+v", evidence.Rejects)
	}
	if len(proofs) == 0 {
		t.Fatal("the 2b authoritative package proof should still be stashed")
	}
}

// TestDriveInteractiveRoastSigning_StashesShareRejectBlameOnVerifiedInvalidShare is
// the share-blame happy path (RFC-21 Phase 7.3, the third fault source): an
// interactive aggregate that fails naming member 1 a candidate culprit, with an
// engine that re-verifies member 1's retained share INVALID, stashes an f+1 reject
// accusation against member 1 (alongside the 2b authoritative proof in the same
// union entry).
func TestDriveInteractiveRoastSigning_StashesShareRejectBlameOnVerifiedInvalidShare(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	f := newDriveFixture(t)
	f.engine.aggregateErr = &InteractiveAggregateShareVerificationError{CandidateCulprits: []uint16{1}}
	verifierEngine := &verifierCapableFakeEngine{
		fakeInteractiveSigningEngine: f.engine,
		shareVerdict:                 NativeShareVerdictInvalid,
	}
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return verifierEngine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	if _, err := f.run(t); err == nil {
		t.Fatal("expected a hard-fail error on share-verification failure")
	}

	evidence, _, ok := takePendingEvidence(f.attemptCtx.SessionID, 1, f.attemptCtx.Hash())
	if !ok {
		t.Fatal("share-verification failure must stash reject evidence")
	}
	if len(evidence.Rejects[1]) == 0 {
		t.Fatalf("member 1's engine-verified-invalid share must produce a reject accusation; got %+v", evidence.Rejects)
	}
}

// TestDriveInteractiveRoastSigning_SkipsShareBlameForMalformedCandidates guards the
// uint16->MemberIndex conversion: a zero or out-of-range (> uint8 max) candidate is
// dropped BEFORE classification, so a malformed engine candidate can never truncate
// into -- and falsely blame -- an honest seat. With every candidate dropped, no
// reject evidence is stashed (the 2b proof still is).
func TestDriveInteractiveRoastSigning_SkipsShareBlameForMalformedCandidates(t *testing.T) {
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	f := newDriveFixture(t)
	f.engine.aggregateErr = &InteractiveAggregateShareVerificationError{CandidateCulprits: []uint16{0, 300}}
	verifierEngine := &verifierCapableFakeEngine{
		fakeInteractiveSigningEngine: f.engine,
		shareVerdict:                 NativeShareVerdictInvalid,
	}
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return verifierEngine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	if _, err := f.run(t); err == nil {
		t.Fatal("expected a hard-fail error")
	}
	evidence, _, ok := takePendingEvidence(f.attemptCtx.SessionID, 1, f.attemptCtx.Hash())
	if !ok {
		t.Fatal("the 2b proof stash should still produce an entry")
	}
	if len(evidence.Rejects) != 0 {
		t.Fatalf("malformed candidates must produce no reject blame; got %+v", evidence.Rejects)
	}
}
