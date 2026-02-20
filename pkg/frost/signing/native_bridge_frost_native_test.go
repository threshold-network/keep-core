//go:build frost_native

package signing

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

type mockNativeExecutionFFIExecutor struct {
	executeCalls int
	lastRequest  *Request
	result       *Result
	err          error

	registerUnmarshallersCalls int
	lastChannel                net.BroadcastChannel
}

func (mnefe *mockNativeExecutionFFIExecutor) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	mnefe.executeCalls++
	mnefe.lastRequest = request
	return mnefe.result, mnefe.err
}

func (mnefe *mockNativeExecutionFFIExecutor) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	mnefe.registerUnmarshallersCalls++
	mnefe.lastChannel = channel
}

func staticNativeFFIExecutorProvider(
	executor NativeExecutionFFIExecutor,
) func() NativeExecutionFFIExecutor {
	return func() NativeExecutionFFIExecutor {
		return executor
	}
}

func TestBuildTaggedNativeExecutionBridge_Execute_UsesFFIExecutor(
	t *testing.T,
) {
	expectedResult := &Result{}
	ffiExecutor := &mockNativeExecutionFFIExecutor{
		result: expectedResult,
	}

	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: &Result{},
	}

	bridge := &buildTaggedNativeExecutionBridge{
		ffiExecutorProvider: staticNativeFFIExecutorProvider(ffiExecutor),
		delegate:            fallback,
	}

	result, err := bridge.Execute(context.Background(), nil, &Request{})
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

	if ffiExecutor.executeCalls != 1 {
		t.Fatalf(
			"unexpected ffi executor execute calls count: [%d]",
			ffiExecutor.executeCalls,
		)
	}

	if fallback.executeCalls != 0 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionBridge_Execute_StrictNoFallbackWithoutFFIExecutor(
	t *testing.T,
) {
	setNativeExecutionMode(nativeExecutionModeStrict)
	t.Cleanup(func() {
		setNativeExecutionMode(nativeExecutionModeFallbackAllowed)
	})

	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: &Result{},
	}

	bridge := &buildTaggedNativeExecutionBridge{
		ffiExecutorProvider: staticNativeFFIExecutorProvider(nil),
		delegate:            fallback,
	}

	_, err := bridge.Execute(context.Background(), nil, &Request{})
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

func TestBuildTaggedNativeExecutionBridge_Execute_FallsBackWithoutFFIExecutor(
	t *testing.T,
) {
	setNativeExecutionMode(nativeExecutionModeFallbackAllowed)

	expectedResult := &Result{}
	fallback := &mockExecutionBackend{
		name:   "fallback",
		result: expectedResult,
	}

	bridge := &buildTaggedNativeExecutionBridge{
		ffiExecutorProvider: staticNativeFFIExecutorProvider(nil),
		delegate:            fallback,
	}

	result, err := bridge.Execute(context.Background(), nil, &Request{})
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

	if fallback.executeCalls != 1 {
		t.Fatalf("unexpected fallback execute calls count: [%d]", fallback.executeCalls)
	}
}

func TestBuildTaggedNativeExecutionBridge_RegisterUnmarshallers_UsesFFIExecutor(
	t *testing.T,
) {
	ffiExecutor := &mockNativeExecutionFFIExecutor{}
	fallback := &mockExecutionBackend{name: "fallback"}

	bridge := &buildTaggedNativeExecutionBridge{
		ffiExecutorProvider: staticNativeFFIExecutorProvider(ffiExecutor),
		delegate:            fallback,
	}

	bridge.RegisterUnmarshallers(nil)

	if ffiExecutor.registerUnmarshallersCalls != 1 {
		t.Fatalf(
			"unexpected ffi executor register unmarshallers calls count: [%d]",
			ffiExecutor.registerUnmarshallersCalls,
		)
	}

	if fallback.registerUnmarshallersCalls != 0 {
		t.Fatalf(
			"unexpected fallback register unmarshallers calls count: [%d]",
			fallback.registerUnmarshallersCalls,
		)
	}
}

func TestBuildTaggedNativeExecutionBridge_RegisterUnmarshallers_StrictNoFallback(
	t *testing.T,
) {
	setNativeExecutionMode(nativeExecutionModeStrict)
	t.Cleanup(func() {
		setNativeExecutionMode(nativeExecutionModeFallbackAllowed)
	})

	fallback := &mockExecutionBackend{name: "fallback"}

	bridge := &buildTaggedNativeExecutionBridge{
		ffiExecutorProvider: staticNativeFFIExecutorProvider(nil),
		delegate:            fallback,
	}

	bridge.RegisterUnmarshallers(nil)

	if fallback.registerUnmarshallersCalls != 0 {
		t.Fatalf(
			"unexpected fallback register unmarshallers calls count: [%d]",
			fallback.registerUnmarshallersCalls,
		)
	}
}
