//go:build frost_native

package signing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

type mockNativeExecutionBridge struct {
	available bool

	executeCalls int
	lastRequest  *Request
	result       *Result
	err          error

	registerUnmarshallersCalls int
	lastChannel                net.BroadcastChannel
}

func (mneb *mockNativeExecutionBridge) IsAvailable() bool {
	return mneb.available
}

func (mneb *mockNativeExecutionBridge) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	mneb.executeCalls++
	mneb.lastRequest = request
	return mneb.result, mneb.err
}

func (mneb *mockNativeExecutionBridge) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	mneb.registerUnmarshallersCalls++
	mneb.lastChannel = channel
}

func staticNativeBridgeProvider(
	bridge NativeExecutionBridge,
) func() NativeExecutionBridge {
	return func() NativeExecutionBridge {
		return bridge
	}
}

func TestNativeExecutionBackend_FrostNativeBuildSelectable(t *testing.T) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	UnregisterNativeExecutionBridge()
	RegisterNativeExecutionAdapterForBuild()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)
	t.Cleanup(UnregisterNativeExecutionBridge)

	err := SetExecutionBackendByName("native")
	if err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}

	if CurrentExecutionBackendName() != NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			NativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	adapter := newBuildTaggedNativeExecutionAdapter()

	_, err = adapter.Execute(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected request validation error")
	}

	if !strings.Contains(err.Error(), "request is nil") {
		t.Fatalf(
			"unexpected native execution error\nexpected substring: [%s]\nactual:             [%v]",
			"request is nil",
			err,
		)
	}

	err = SetExecutionBackendByName("ffi")
	if err != nil {
		t.Fatalf("unexpected strict ffi backend config error: [%v]", err)
	}

	if CurrentExecutionBackendName() != NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for strict ffi config\nexpected: [%s]\nactual:   [%s]",
			NativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	UnregisterNativeExecutionBridge()

	err = SetExecutionBackendByName("ffi")
	if err == nil {
		t.Fatal("expected strict ffi backend unavailable error")
	}

	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected ffi backend error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected ffi native-availability error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	registeredBridge := &mockNativeExecutionBridge{
		available: true,
		result:    &Result{},
	}

	err = RegisterNativeExecutionBridge(registeredBridge)
	if err != nil {
		t.Fatalf("failed registering native execution bridge: [%v]", err)
	}

	err = SetExecutionBackendByName("ffi")
	if err != nil {
		t.Fatalf("unexpected strict ffi backend config error: [%v]", err)
	}

	if CurrentExecutionBackendName() != NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name for strict ffi config\nexpected: [%s]\nactual:   [%s]",
			NativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
}

func TestBuildTaggedNativeExecutionAdapter_Execute_UsesNativeBridgeWhenAvailable(
	t *testing.T,
) {
	expectedResult := &Result{}
	bridge := &mockNativeExecutionBridge{
		available: true,
		result:    expectedResult,
	}

	fallback := &mockExecutionBackend{name: "fallback"}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	result, err := adapter.Execute(context.Background(), nil, &Request{})
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

	if bridge.executeCalls != 1 {
		t.Fatalf("unexpected bridge execute calls count: [%d]", bridge.executeCalls)
	}

	if fallback.executeCalls != 0 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionAdapter_Execute_FallsBackWhenBridgeUnavailable(
	t *testing.T,
) {
	expectedResult := &Result{}
	bridge := &mockNativeExecutionBridge{
		available: false,
	}

	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: expectedResult,
	}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	result, err := adapter.Execute(context.Background(), nil, &Request{})
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

	if bridge.executeCalls != 0 {
		t.Fatalf("unexpected bridge execute calls count: [%d]", bridge.executeCalls)
	}

	if fallback.executeCalls != 1 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionAdapter_Execute_FallsBackOnUnavailableBridgeError(
	t *testing.T,
) {
	expectedResult := &Result{}
	bridge := &mockNativeExecutionBridge{
		available: true,
		err:       ErrNativeCryptographyUnavailable,
	}

	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: expectedResult,
	}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	result, err := adapter.Execute(context.Background(), nil, &Request{})
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

	if bridge.executeCalls != 1 {
		t.Fatalf("unexpected bridge execute calls count: [%d]", bridge.executeCalls)
	}

	if fallback.executeCalls != 1 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionAdapter_Execute_ReturnsBridgeError(
	t *testing.T,
) {
	bridgeError := errors.New("bridge failure")
	bridge := &mockNativeExecutionBridge{
		available: true,
		err:       bridgeError,
	}

	fallback := &mockExecutionBackend{name: "fallback"}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	_, err := adapter.Execute(context.Background(), nil, &Request{})
	if err == nil {
		t.Fatal("expected execute error")
	}

	if !errors.Is(err, bridgeError) {
		t.Fatalf(
			"unexpected execute error\nexpected: [%v]\nactual:   [%v]",
			bridgeError,
			err,
		)
	}

	if fallback.executeCalls != 0 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionAdapter_Execute_StrictModeNoFallbackWhenUnavailable(
	t *testing.T,
) {
	setNativeExecutionMode(nativeExecutionModeStrict)
	t.Cleanup(func() {
		setNativeExecutionMode(nativeExecutionModeFallbackAllowed)
	})

	bridge := &mockNativeExecutionBridge{
		available: false,
	}

	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: &Result{},
	}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	_, err := adapter.Execute(context.Background(), nil, &Request{})
	if err == nil {
		t.Fatal("expected execute error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected execute error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if fallback.executeCalls != 0 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionAdapter_Execute_StrictModeNoFallbackOnUnavailableError(
	t *testing.T,
) {
	setNativeExecutionMode(nativeExecutionModeStrict)
	t.Cleanup(func() {
		setNativeExecutionMode(nativeExecutionModeFallbackAllowed)
	})

	bridge := &mockNativeExecutionBridge{
		available: true,
		err:       ErrNativeCryptographyUnavailable,
	}

	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: &Result{},
	}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	_, err := adapter.Execute(context.Background(), nil, &Request{})
	if err == nil {
		t.Fatal("expected execute error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected execute error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if fallback.executeCalls != 0 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionAdapter_RegisterUnmarshallers_UsesNativeWhenAvailable(
	t *testing.T,
) {
	bridge := &mockNativeExecutionBridge{
		available: true,
	}

	fallback := &mockExecutionBackend{name: "fallback"}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	adapter.RegisterUnmarshallers(nil)

	if bridge.registerUnmarshallersCalls != 1 {
		t.Fatalf(
			"unexpected bridge register unmarshallers calls count: [%d]",
			bridge.registerUnmarshallersCalls,
		)
	}

	if fallback.registerUnmarshallersCalls != 0 {
		t.Fatalf(
			"unexpected fallback register unmarshallers calls count: [%d]",
			fallback.registerUnmarshallersCalls,
		)
	}
}

func TestBuildTaggedNativeExecutionAdapter_RegisterUnmarshallers_FallsBackWhenUnavailable(
	t *testing.T,
) {
	bridge := &mockNativeExecutionBridge{
		available: false,
	}

	fallback := &mockExecutionBackend{name: "fallback"}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	adapter.RegisterUnmarshallers(nil)

	if bridge.registerUnmarshallersCalls != 0 {
		t.Fatalf(
			"unexpected bridge register unmarshallers calls count: [%d]",
			bridge.registerUnmarshallersCalls,
		)
	}

	if fallback.registerUnmarshallersCalls != 1 {
		t.Fatalf(
			"unexpected fallback register unmarshallers calls count: [%d]",
			fallback.registerUnmarshallersCalls,
		)
	}
}

func TestBuildTaggedNativeExecutionAdapter_RegisterUnmarshallers_StrictModeNoFallback(
	t *testing.T,
) {
	setNativeExecutionMode(nativeExecutionModeStrict)
	t.Cleanup(func() {
		setNativeExecutionMode(nativeExecutionModeFallbackAllowed)
	})

	bridge := &mockNativeExecutionBridge{
		available: false,
	}

	fallback := &mockExecutionBackend{name: "fallback"}

	adapter := &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: staticNativeBridgeProvider(bridge),
		fallback:             fallback,
	}

	adapter.RegisterUnmarshallers(nil)

	if fallback.registerUnmarshallersCalls != 0 {
		t.Fatalf(
			"unexpected fallback register unmarshallers calls count: [%d]",
			fallback.registerUnmarshallersCalls,
		)
	}
}
