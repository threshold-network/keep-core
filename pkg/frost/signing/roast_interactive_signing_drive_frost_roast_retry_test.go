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

// driveFixture is a consistent single-node (1-of-1) interactive signing setup:
// a registered coordinator, a stashed orchestration handle, and a request whose
// persisted material's DKG public key matches the attempt-context seed. It sets
// the readiness gate (so BeginOrchestrationForSession mints a handle) but NOT
// the interactive audit gate, and does NOT register the engine provider; each
// test opts into those so the front-door behaviour is isolated.
type driveFixture struct {
	request   *NativeExecutionFFISigningRequest
	engine    *fakeInteractiveSigningEngine
	sessionID string
}

func newDriveFixture(t *testing.T) driveFixture {
	t.Helper()

	const (
		sessionID = "interactive-session-1"
		keyGroup  = "interactive-key-group"
	)
	dkgKey := []byte(keyGroup)
	included := []group.MemberIndex{1}

	ctx, err := attempt.NewAttemptContext(
		sessionID, keyGroup, dkgKey,
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
	})
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetInteractiveSigningEngineProviderForTest)

	// Readiness gate enables BeginOrchestrationForSession to mint + stash the
	// handle the drive later retrieves. The interactive audit gate is left to
	// the individual tests.
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	_, cleanup, err := BeginOrchestrationForSession(sessionID, ctx)
	if err != nil {
		t.Fatalf("begin orchestration: %v", err)
	}
	t.Cleanup(cleanup)

	engine := newFakeInteractiveSigningEngine()
	// A real engine derives the same coordinator the binding elected; the sole
	// member (1) is the elected coordinator for a 1-of-1 attempt.
	engine.coordinatorIdentifier = 1

	request := &NativeExecutionFFISigningRequest{
		SessionID:           sessionID,
		MemberIndex:         1,
		Channel:             noopBroadcastChannel{},
		MembershipValidator: singleSeatValidator(t),
		SignerMaterial:      persistedTBTCSignerMaterial(t, keyGroup, 1, 1),
	}

	return driveFixture{request: request, engine: engine, sessionID: sessionID}
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

func runDrive(t *testing.T, request *NativeExecutionFFISigningRequest) (*frost.Signature, bool, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return driveInteractiveRoastSigningIfEnabled(ctx, &testutils.MockLogger{}, request)
}

func TestDriveInteractiveRoastSigning_HappyPath(t *testing.T) {
	f := newDriveFixture(t)
	want := validBIP340Signature(t)
	f.engine.signature = want
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	sig, handled, err := runDrive(t, f.request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true on the interactive path")
	}
	if sig == nil {
		t.Fatal("expected a signature")
	}
	serialized := sig.Serialize()
	if string(serialized[:]) != string(want) {
		t.Fatalf("signature mismatch:\n got %x\nwant %x", serialized[:], want)
	}
}

func TestDriveInteractiveRoastSigning_GateOffFallsBackToCoarse(t *testing.T) {
	f := newDriveFixture(t)
	// Everything is ready (handle stashed, engine registered) EXCEPT the audit
	// gate, proving the gate alone gates the interactive path.
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })

	sig, handled, err := runDrive(t, f.request)
	if err != nil || handled || sig != nil {
		t.Fatalf("expected coarse fallback (nil,false,nil), got sig=%v handled=%v err=%v", sig, handled, err)
	}
}

func TestDriveInteractiveRoastSigning_NoEngineFallsBackToCoarse(t *testing.T) {
	f := newDriveFixture(t)
	// Gate on, handle stashed, but no engine registered.
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	sig, handled, err := runDrive(t, f.request)
	if err != nil || handled || sig != nil {
		t.Fatalf("expected coarse fallback (nil,false,nil), got sig=%v handled=%v err=%v", sig, handled, err)
	}
}

func TestDriveInteractiveRoastSigning_NoHandleFallsBackToCoarse(t *testing.T) {
	// Gate on + engine registered, but orchestration is not active for this
	// session (no stashed handle).
	t.Setenv(InteractiveSigningOptInEnvVar, "true")
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine {
		return newFakeInteractiveSigningEngine()
	})
	t.Cleanup(ResetInteractiveSigningEngineProviderForTest)

	request := &NativeExecutionFFISigningRequest{SessionID: "orchestration-inactive-session"}
	sig, handled, err := runDrive(t, request)
	if err != nil || handled || sig != nil {
		t.Fatalf("expected coarse fallback (nil,false,nil), got sig=%v handled=%v err=%v", sig, handled, err)
	}
}

func TestDriveInteractiveRoastSigning_RunnerFailureHardFails(t *testing.T) {
	f := newDriveFixture(t)
	// The node has COMMITTED to interactive signing (gate on, engine present,
	// orchestration active); a runner failure must propagate as an error so the
	// outer tBTC signingRetryLoop advances, NOT silently drop to coarse.
	f.engine.aggregateErr = fmt.Errorf("aggregate share verification failed")
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return f.engine })
	t.Setenv(InteractiveSigningOptInEnvVar, "true")

	sig, _, err := runDrive(t, f.request)
	if err == nil {
		t.Fatal("expected a hard-fail error on runner failure")
	}
	if sig != nil {
		t.Fatalf("expected no signature on failure, got %v", sig)
	}
}
