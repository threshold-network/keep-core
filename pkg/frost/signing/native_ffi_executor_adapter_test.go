package signing

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type mockNativeExecutionFFISigningPrimitive struct {
	signCalls     int
	lastRequest   *NativeExecutionFFISigningRequest
	signature     *frost.Signature
	signErr       error
	afterSign     func()
	registerCalls int
	lastChannel   net.BroadcastChannel
}

func (mnefsp *mockNativeExecutionFFISigningPrimitive) Sign(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	mnefsp.signCalls++
	mnefsp.lastRequest = request
	if mnefsp.afterSign != nil {
		mnefsp.afterSign()
	}
	return mnefsp.signature, mnefsp.signErr
}

func (mnefsp *mockNativeExecutionFFISigningPrimitive) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	mnefsp.registerCalls++
	mnefsp.lastChannel = channel
}

// panickingFFISigningPrimitive panics in Sign, standing in for a panic raised
// along the cgo/FFI path (e.g. a C.GoBytes length-out-of-range on a malformed
// native-signer response).
type panickingFFISigningPrimitive struct{}

func (panickingFFISigningPrimitive) Sign(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	panic("simulated cgo boundary panic")
}

func (panickingFFISigningPrimitive) RegisterUnmarshallers(channel net.BroadcastChannel) {}

func TestNativeExecutionFFIExecutorAdapter_Execute_RecoversCgoBoundaryPanic(
	t *testing.T,
) {
	executor, err := NewNativeExecutionFFIExecutorAdapter(panickingFFISigningPrimitive{})
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	result, err := executor.Execute(context.Background(), nil, &Request{
		Message:        big.NewInt(1),
		SignerMaterial: []byte{0x01},
	})
	if err == nil {
		t.Fatal("expected an error from the recovered panic, got nil")
	}
	if result != nil {
		t.Fatalf("expected a nil result on a recovered panic, got [%v]", result)
	}
	if !strings.Contains(err.Error(), "panicked at the cgo boundary") {
		t.Fatalf(
			"expected a cgo-boundary panic error, got: [%v]",
			err,
		)
	}
}

func TestNewNativeExecutionFFIExecutorAdapter_NilPrimitive(t *testing.T) {
	_, err := NewNativeExecutionFFIExecutorAdapter(nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "native execution FFI signing primitive is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native execution FFI signing primitive is nil",
			err,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_ValidatesRequest(t *testing.T) {
	executor, err := NewNativeExecutionFFIExecutorAdapter(
		&mockNativeExecutionFFISigningPrimitive{},
	)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "request is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"request is nil",
			err,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_ValidatesMessage(t *testing.T) {
	executor, err := NewNativeExecutionFFIExecutorAdapter(
		&mockNativeExecutionFFISigningPrimitive{},
	)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, &Request{
		SignerMaterial: []byte{0x01},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "request message is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"request message is nil",
			err,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_ValidatesSignerMaterial(
	t *testing.T,
) {
	executor, err := NewNativeExecutionFFIExecutorAdapter(
		&mockNativeExecutionFFISigningPrimitive{},
	)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, &Request{
		Message:        big.NewInt(1),
		SignerMaterial: "invalid",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if !strings.Contains(err.Error(), "native signer material has wrong type") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native signer material has wrong type",
			err,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_DelegatesToPrimitive(
	t *testing.T,
) {
	expectedSignature := &frost.Signature{
		R: [frost.SignatureComponentSize]byte{0x01},
		S: [frost.SignatureComponentSize]byte{0x02},
	}

	primitive := &mockNativeExecutionFFISigningPrimitive{
		signature: expectedSignature,
	}

	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	attempt := &Attempt{
		Number:                 3,
		CoordinatorMemberIndex: 1,
		IncludedMembersIndexes: []group.MemberIndex{1, 2, 3},
		ExcludedMembersIndexes: []group.MemberIndex{4},
	}
	heartbeatMessage := [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}
	signingIntent := NewHeartbeatSigningIntent(heartbeatMessage)

	result, err := executor.Execute(context.Background(), nil, &Request{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        2,
		GroupSize:          5,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0xaa},
		},
		SigningIntent: signingIntent,
		Attempt:       attempt,
	})
	if err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}

	if result == nil || result.Signature != expectedSignature {
		t.Fatalf(
			"unexpected result signature\nexpected: [%+v]\nactual:   [%+v]",
			expectedSignature,
			result,
		)
	}

	if primitive.signCalls != 1 {
		t.Fatalf("unexpected primitive sign calls count: [%d]", primitive.signCalls)
	}

	if primitive.lastRequest == nil {
		t.Fatal("expected primitive request")
	}

	if primitive.lastRequest.SignerMaterial == nil {
		t.Fatal("expected signer material in primitive request")
	}

	if primitive.lastRequest.Attempt == attempt {
		t.Fatal("expected attempt clone in primitive request")
	}
	if primitive.lastRequest.SigningIntent == signingIntent {
		t.Fatal("expected signing intent clone in primitive request")
	}
	gotHeartbeatMessage, ok := primitive.lastRequest.SigningIntent.HeartbeatMessage()
	if !ok || gotHeartbeatMessage != heartbeatMessage {
		t.Fatalf(
			"unexpected heartbeat signing intent: got [%x], present [%v], want [%x]",
			gotHeartbeatMessage,
			ok,
			heartbeatMessage,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_RevalidatesBeforeSignatureRelease(
	t *testing.T,
) {
	authorized := true
	primitive := &mockNativeExecutionFFISigningPrimitive{
		signature: &frost.Signature{R: [frost.SignatureComponentSize]byte{0x01}},
		afterSign: func() { authorized = false },
	}
	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(context.Background(), nil, &Request{
		Message: big.NewInt(123),
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0xaa},
		},
		AuthorizationGuard: func(context.Context) error {
			if !authorized {
				return errors.New("authorization reorged")
			}
			return nil
		},
	})
	if result != nil || err == nil ||
		!errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf("unexpected post-sign authorization result: [%v] [%v]", result, err)
	}
	if primitive.signCalls != 1 {
		t.Fatal("test did not reach the native signature boundary")
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_GenericSigningIntentIsNil(
	t *testing.T,
) {
	primitive := &mockNativeExecutionFFISigningPrimitive{
		signature: &frost.Signature{},
	}
	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, &Request{
		Message: big.NewInt(1),
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0xaa},
		},
	})
	if err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}
	if primitive.lastRequest == nil {
		t.Fatal("expected primitive request")
	}
	if primitive.lastRequest.SigningIntent != nil {
		t.Fatalf(
			"generic signing must keep signing intent nil, got [%+v]",
			primitive.lastRequest.SigningIntent,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_PropagatesPrimitiveError(
	t *testing.T,
) {
	expectedErr := errors.New("native signer failure")
	primitive := &mockNativeExecutionFFISigningPrimitive{
		signErr: expectedErr,
	}

	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, &Request{
		Message:        big.NewInt(1),
		SignerMaterial: []byte{0x01},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			expectedErr,
			err,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_RejectsNilSignature(
	t *testing.T,
) {
	primitive := &mockNativeExecutionFFISigningPrimitive{}

	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, &Request{
		Message:        big.NewInt(1),
		SignerMaterial: []byte{0x01},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "returned nil signature") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"returned nil signature",
			err,
		)
	}
}

func TestNativeExecutionFFIExecutorAdapter_Execute_InteractiveOnlyRefusesCoarse(
	t *testing.T,
) {
	// Coarse-path retirement: with interactive-only mode ON, the adapter must NOT fall
	// through to the coarse primitive on ANY no-interactive-signature path. In the
	// default build attemptRoastRetryOrchestrationFromRequest is a no-op (nil,nil,nil)
	// - the ultimate static fallback - so this exercises exactly the gap Codex flagged:
	// a fall-through that an executor-level check (which runs only after orchestration
	// activates) would have bypassed. The adapter is the single coarse-invocation
	// point, so the refusal here covers every path.
	t.Setenv(InteractiveSigningOnlyEnvVar, "true")

	primitive := &mockNativeExecutionFFISigningPrimitive{
		signature: &frost.Signature{
			R: [frost.SignatureComponentSize]byte{0x01},
			S: [frost.SignatureComponentSize]byte{0x02},
		},
	}

	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	_, err = executor.Execute(context.Background(), nil, &Request{
		Message:            big.NewInt(123),
		SessionID:          "session-interactive-only",
		MemberIndex:        2,
		GroupSize:          5,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0xaa},
		},
		Attempt: &Attempt{
			Number:                 3,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2, 3},
		},
	})
	if err == nil {
		t.Fatal("interactive-only mode must fail closed instead of using the coarse primitive")
	}
	if !errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf("interactive-only refusal must be TERMINAL so the retry loop aborts: %v", err)
	}
	if !strings.Contains(err.Error(), InteractiveSigningOnlyEnvVar) {
		t.Fatalf("unexpected error (want a refusal naming %s): %v", InteractiveSigningOnlyEnvVar, err)
	}
	if primitive.signCalls != 0 {
		t.Fatalf(
			"coarse primitive must NOT be called in interactive-only mode, got %d call(s)",
			primitive.signCalls,
		)
	}
}

func TestInteractiveSigningOnlyEnabled_ParsesFlag(t *testing.T) {
	t.Setenv(InteractiveSigningOnlyEnvVar, "")
	if InteractiveSigningOnlyEnabled() {
		t.Fatal("unset must be off")
	}
	t.Setenv(InteractiveSigningOnlyEnvVar, "  TrUe ")
	if !InteractiveSigningOnlyEnabled() {
		t.Fatal("case-insensitive, trimmed true must be on")
	}
	t.Setenv(InteractiveSigningOnlyEnvVar, "false")
	if InteractiveSigningOnlyEnabled() {
		t.Fatal("false must be off")
	}
}

func TestNativeExecutionFFIExecutorAdapter_RegisterUnmarshallers_Delegates(
	t *testing.T,
) {
	primitive := &mockNativeExecutionFFISigningPrimitive{}

	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		t.Fatalf("unexpected adapter setup error: [%v]", err)
	}

	var channel net.BroadcastChannel
	executor.RegisterUnmarshallers(channel)

	if primitive.registerCalls != 1 {
		t.Fatalf(
			"unexpected register unmarshallers calls count: [%d]",
			primitive.registerCalls,
		)
	}
}

func TestRegisterNativeExecutionFFISigningPrimitive_Nil(t *testing.T) {
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	err := RegisterNativeExecutionFFISigningPrimitive(nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "native execution FFI signing primitive is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native execution FFI signing primitive is nil",
			err,
		)
	}
}

func TestRegisterNativeExecutionFFISigningPrimitive_RegistersExecutor(t *testing.T) {
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	err := RegisterNativeExecutionFFISigningPrimitive(
		&mockNativeExecutionFFISigningPrimitive{
			signature: &frost.Signature{},
		},
	)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	executor := currentNativeExecutionFFIExecutor()
	if executor == nil {
		t.Fatal("expected native FFI executor registration")
	}
}
