package signing

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type mockExecutionBackend struct {
	name string

	executeCalls int
	lastRequest  *Request
	result       *Result
	err          error

	registerUnmarshallersCalls int
	lastChannel                net.BroadcastChannel
}

type mockNativeExecutionAdapter struct {
	executeCalls int
	lastRequest  *Request
	result       *Result
	err          error

	registerUnmarshallersCalls int
	lastChannel                net.BroadcastChannel
}

type mockNativeExecutionAdapterWithAvailability struct {
	*mockNativeExecutionAdapter
	nativeExecutionAvailable bool
}

func (meb *mockExecutionBackend) Name() string {
	return meb.name
}

func (meb *mockExecutionBackend) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	meb.executeCalls++
	meb.lastRequest = request
	return meb.result, meb.err
}

func (meb *mockExecutionBackend) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	meb.registerUnmarshallersCalls++
	meb.lastChannel = channel
}

func (mnea *mockNativeExecutionAdapter) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	mnea.executeCalls++
	mnea.lastRequest = request
	return mnea.result, mnea.err
}

func (mnea *mockNativeExecutionAdapter) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	mnea.registerUnmarshallersCalls++
	mnea.lastChannel = channel
}

func (mneawa *mockNativeExecutionAdapterWithAvailability) NativeExecutionAvailable() bool {
	return mneawa.nativeExecutionAvailable
}

func TestCurrentExecutionBackendName_Default(t *testing.T) {
	ResetExecutionBackend()
	if CurrentExecutionBackendName() != legacyExecutionBackendName {
		t.Fatalf(
			"unexpected default backend name\nexpected: [%s]\nactual:   [%s]",
			legacyExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
}

func TestSetExecutionBackend_Nil(t *testing.T) {
	if err := SetExecutionBackend(nil); err == nil {
		t.Fatal("expected nil backend error")
	}
}

func TestSetExecutionBackendByName(t *testing.T) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	UnregisterNativeExecutionBridge()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)
	t.Cleanup(UnregisterNativeExecutionBridge)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	if err := SetExecutionBackendByName(""); err != nil {
		t.Fatalf("unexpected default backend config error: [%v]", err)
	}
	if CurrentExecutionBackendName() != legacyExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for default config\\nexpected: [%s]\\nactual:   [%s]",
			legacyExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	if err := SetExecutionBackendByName("LEGACY"); err != nil {
		t.Fatalf("unexpected legacy backend config error: [%v]", err)
	}
	if CurrentExecutionBackendName() != legacyExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for legacy config\\nexpected: [%s]\\nactual:   [%s]",
			legacyExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	err := SetExecutionBackendByName("native")
	if err == nil {
		t.Fatal("expected native backend unavailable error")
	}
	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected native backend error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
	if !nativeExecutionFallbackAllowed() {
		t.Fatal("expected fallback-allowed mode for native backend selection")
	}

	err = SetExecutionBackendByName("ffi")
	if err == nil {
		t.Fatal("expected ffi backend unavailable error")
	}
	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected ffi backend error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
	if !nativeExecutionFallbackAllowed() {
		t.Fatal(
			"expected previous fallback-allowed mode after failed ffi backend selection",
		)
	}
	if CurrentExecutionBackendName() != legacyExecutionBackendName {
		t.Fatalf(
			"unexpected backend name after failed ffi config\\nexpected: [%s]\\nactual:   [%s]",
			legacyExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	err = SetExecutionBackendByName("unknown")
	if err == nil {
		t.Fatal("expected unknown backend error")
	}
}

func TestNativeExecutionFallbackAllowed_SuppressedByInteractiveOnly(t *testing.T) {
	// Coarse-path retirement (Codex #4101 P2): interactive-only mode must close the
	// OUTER native fallbacks too - the bridge/adapter consult this single gate before
	// delegating to the legacy backend, so the flag has to flip it closed regardless
	// of the execution mode, or a node with an unavailable FFI path would still sign
	// over the legacy/coarse delegate.
	previousMode := currentNativeExecutionMode()
	t.Cleanup(func() { setNativeExecutionMode(previousMode) })

	setNativeExecutionMode(nativeExecutionModeFallbackAllowed)
	if !nativeExecutionFallbackAllowed() {
		t.Fatal("baseline: the fallback-allowed mode must permit the outer fallback")
	}

	t.Setenv(InteractiveSigningOnlyEnvVar, "true")
	if nativeExecutionFallbackAllowed() {
		t.Fatal("interactive-only mode must suppress the outer native fallback")
	}
}

func TestLegacyExecutionBackend_InteractiveOnlyRefusesTerminal(t *testing.T) {
	// Coarse-path retirement (Codex #4101 P2): the DEFAULT backend is the legacy
	// coarse/tECDSA signer, which the native bridge/adapter guards never touch. The
	// flag must fail closed here too, terminally, so it cannot fail open under the
	// documented default config.
	t.Setenv(InteractiveSigningOnlyEnvVar, "true")

	_, err := newLegacyExecutionBackend().Execute(
		context.Background(), log.Logger("test"), &Request{Message: big.NewInt(1)},
	)
	if !errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf("interactive-only must terminally refuse the legacy backend: %v", err)
	}
}

type unavailableNativeAdapter struct{}

func (unavailableNativeAdapter) Execute(
	context.Context, log.StandardLogger, *Request,
) (*Result, error) {
	return nil, ErrNativeCryptographyUnavailable
}

func (unavailableNativeAdapter) RegisterUnmarshallers(net.BroadcastChannel) {}

func TestNativeExecutionBackend_InteractiveOnlyPromotesUnavailableToTerminal(t *testing.T) {
	// Coarse-path retirement (Codex #4101 P2): when the native interactive path is
	// unavailable and every fallback is suppressed, the outer refusal surfaces as a
	// bare ErrNativeCryptographyUnavailable - promote it to terminal so the retry loop
	// aborts instead of spinning.
	backend, err := newNativeExecutionBackend(unavailableNativeAdapter{})
	if err != nil {
		t.Fatalf("unexpected backend setup error: %v", err)
	}

	t.Setenv(InteractiveSigningOnlyEnvVar, "true")
	_, err = backend.Execute(
		context.Background(), log.Logger("test"), &Request{Message: big.NewInt(1)},
	)
	if !errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf("interactive-only must promote the native unavailable error to terminal: %v", err)
	}

	// With the flag OFF the unavailable error passes through unpromoted (no regression).
	t.Setenv(InteractiveSigningOnlyEnvVar, "")
	_, offErr := backend.Execute(
		context.Background(), log.Logger("test"), &Request{Message: big.NewInt(1)},
	)
	if errors.Is(offErr, ErrTerminalSigningFailure) {
		t.Fatalf("must not promote to terminal when the flag is off: %v", offErr)
	}
	if !errors.Is(offErr, ErrNativeCryptographyUnavailable) {
		t.Fatalf("flag off must pass the native unavailable error through: %v", offErr)
	}
}

func TestSetExecutionBackendByName_NativeFailureRestoresPreviousMode(
	t *testing.T,
) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	UnregisterNativeExecutionBridge()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)
	t.Cleanup(UnregisterNativeExecutionBridge)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	setNativeExecutionMode(nativeExecutionModeStrict)
	if nativeExecutionFallbackAllowed() {
		t.Fatal("expected strict mode before failed native backend selection")
	}

	err := SetExecutionBackendByName("native")
	if err == nil {
		t.Fatal("expected native backend unavailable error")
	}
	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected native backend error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}

	if nativeExecutionFallbackAllowed() {
		t.Fatal("expected strict mode to be restored after failed native selection")
	}
	if CurrentExecutionBackendName() != legacyExecutionBackendName {
		t.Fatalf(
			"unexpected backend name after failed native config\\nexpected: [%s]\\nactual:   [%s]",
			legacyExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
}

func TestSetExecutionBackendByName_FFIFailurePreservesNativeModeAndBackend(
	t *testing.T,
) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	UnregisterNativeExecutionBridge()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)
	t.Cleanup(UnregisterNativeExecutionBridge)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	adapter := &mockNativeExecutionAdapterWithAvailability{
		mockNativeExecutionAdapter: &mockNativeExecutionAdapter{},
		nativeExecutionAvailable:   false,
	}

	if err := RegisterNativeExecutionAdapter(adapter); err != nil {
		t.Fatalf("failed registering native execution adapter: [%v]", err)
	}

	if err := SetExecutionBackendByName("native"); err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}
	if !nativeExecutionFallbackAllowed() {
		t.Fatal("expected fallback-allowed mode after native backend selection")
	}
	if CurrentExecutionBackendName() != nativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for native config\\nexpected: [%s]\\nactual:   [%s]",
			nativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	err := SetExecutionBackendByName("ffi")
	if err == nil {
		t.Fatal("expected ffi backend unavailable error")
	}
	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected ffi backend error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected strict-mode availability error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if !nativeExecutionFallbackAllowed() {
		t.Fatal(
			"expected fallback-allowed mode to be preserved after failed ffi selection",
		)
	}
	if CurrentExecutionBackendName() != nativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name after failed ffi config\\nexpected: [%s]\\nactual:   [%s]",
			nativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
}

func TestSetExecutionBackendByName_NativeAdapterRegistered(t *testing.T) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	UnregisterNativeExecutionBridge()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)
	t.Cleanup(UnregisterNativeExecutionBridge)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	expectedResult := &Result{Signature: &frost.Signature{}}
	adapter := &mockNativeExecutionAdapter{
		result: expectedResult,
	}

	if err := RegisterNativeExecutionAdapter(adapter); err != nil {
		t.Fatalf("failed registering native execution adapter: [%v]", err)
	}

	if err := SetExecutionBackendByName("ffi"); err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}

	if CurrentExecutionBackendName() != nativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for native config\\nexpected: [%s]\\nactual:   [%s]",
			nativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
	if nativeExecutionFallbackAllowed() {
		t.Fatal("expected strict mode for ffi backend selection")
	}

	if err := SetExecutionBackendByName("native"); err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}
	if !nativeExecutionFallbackAllowed() {
		t.Fatal("expected fallback-allowed mode for native backend selection")
	}

	executeResult, err := Execute(
		context.Background(),
		nil,
		big.NewInt(100),
		"session-id",
		1,
		nil,
		10,
		4,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}

	if executeResult != expectedResult {
		t.Fatalf(
			"unexpected execute result\\nexpected: [%+v]\\nactual:   [%+v]",
			expectedResult,
			executeResult,
		)
	}

	if adapter.executeCalls != 1 {
		t.Fatalf("unexpected native execute calls count: [%d]", adapter.executeCalls)
	}

	RegisterUnmarshallers(nil)

	if adapter.registerUnmarshallersCalls != 1 {
		t.Fatalf(
			"unexpected native register unmarshallers calls count: [%d]",
			adapter.registerUnmarshallersCalls,
		)
	}
}

func TestSetExecutionBackendByName_FFIStrictAvailabilityCheck(t *testing.T) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	UnregisterNativeExecutionBridge()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)
	t.Cleanup(UnregisterNativeExecutionBridge)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	adapter := &mockNativeExecutionAdapterWithAvailability{
		mockNativeExecutionAdapter: &mockNativeExecutionAdapter{},
		nativeExecutionAvailable:   false,
	}

	if err := RegisterNativeExecutionAdapter(adapter); err != nil {
		t.Fatalf("failed registering native execution adapter: [%v]", err)
	}

	err := SetExecutionBackendByName("ffi")
	if err == nil {
		t.Fatal("expected ffi backend unavailable error")
	}
	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected ffi backend error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected strict-mode availability error\\nexpected: [%v]\\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if err := SetExecutionBackendByName("native"); err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}
	if CurrentExecutionBackendName() != nativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for native config\\nexpected: [%s]\\nactual:   [%s]",
			nativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
}

func TestRegisterNativeExecutionAdapter_Nil(t *testing.T) {
	if err := RegisterNativeExecutionAdapter(nil); err == nil {
		t.Fatal("expected nil native adapter error")
	}
}

func TestRegisterNativeExecutionBridge_Nil(t *testing.T) {
	if err := RegisterNativeExecutionBridge(nil); err == nil {
		t.Fatal("expected nil native bridge error")
	}
}

func TestRegisterNativeExecutionFFIExecutor_Nil(t *testing.T) {
	if err := RegisterNativeExecutionFFIExecutor(nil); err == nil {
		t.Fatal("expected nil native FFI executor error")
	}
}

func TestExecute_DelegatesToCurrentBackend(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	expectedResult := &Result{Signature: &frost.Signature{}}
	backend := &mockExecutionBackend{
		name:   "mock",
		result: expectedResult,
	}

	if err := SetExecutionBackend(backend); err != nil {
		t.Fatalf("failed setting backend: [%v]", err)
	}

	attempt := &Attempt{
		Number:                 2,
		CoordinatorMemberIndex: 5,
		IncludedMembersIndexes: []group.MemberIndex{1, 2, 5},
		ExcludedMembersIndexes: []group.MemberIndex{3, 4, 6},
	}

	result, err := Execute(
		context.Background(),
		nil,
		big.NewInt(100),
		"session-id",
		1,
		nil,
		10,
		4,
		nil,
		nil,
		attempt,
	)
	if err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}

	if result != expectedResult {
		t.Fatalf(
			"unexpected result\nexpected: [%+v]\nactual:   [%+v]",
			expectedResult,
			result,
		)
	}

	if backend.executeCalls != 1 {
		t.Fatalf("unexpected execute calls count: [%d]", backend.executeCalls)
	}

	received := backend.lastRequest
	if received == nil {
		t.Fatal("expected backend request")
	}

	if received.Attempt == attempt {
		t.Fatal("expected request attempt clone, got same pointer")
	}

	if !reflect.DeepEqual(received.Attempt, attempt) {
		t.Fatalf(
			"unexpected request attempt\nexpected: [%+v]\nactual:   [%+v]",
			attempt,
			received.Attempt,
		)
	}

	received.Attempt.IncludedMembersIndexes[0] = 99
	if attempt.IncludedMembersIndexes[0] == 99 {
		t.Fatal("mutating backend request attempt should not mutate caller attempt")
	}
}

func TestRegisterUnmarshallers_DelegatesToCurrentBackend(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	backend := &mockExecutionBackend{name: "mock"}
	if err := SetExecutionBackend(backend); err != nil {
		t.Fatalf("failed setting backend: [%v]", err)
	}

	RegisterUnmarshallers(nil)

	if backend.registerUnmarshallersCalls != 1 {
		t.Fatalf(
			"unexpected register unmarshallers calls count: [%d]",
			backend.registerUnmarshallersCalls,
		)
	}

	if backend.lastChannel != nil {
		t.Fatal("expected nil channel to be forwarded unchanged")
	}
}
