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
	return mnefsp.signature, mnefsp.signErr
}

func (mnefsp *mockNativeExecutionFFISigningPrimitive) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	mnefsp.registerCalls++
	mnefsp.lastChannel = channel
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
		Attempt: attempt,
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
