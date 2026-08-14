//go:build frost_native

package signing

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// buildTaggedNativeExecutionBridge is a transitional native bridge registered
// for frost_native builds.
//
// Until a real FFI-backed bridge is linked, this bridge delegates to the
// legacy signing backend while still surfacing native-bridge availability.
type buildTaggedNativeExecutionBridge struct {
	ffiExecutorProvider func() NativeExecutionFFIExecutor
	delegate            ExecutionBackend
}

func newBuildTaggedNativeExecutionBridge() NativeExecutionBridge {
	return &buildTaggedNativeExecutionBridge{
		ffiExecutorProvider: currentNativeExecutionFFIExecutor,
		delegate:            newLegacyExecutionBackend(),
	}
}

func (btneb *buildTaggedNativeExecutionBridge) IsAvailable() bool {
	if btneb.currentFFIExecutor() != nil {
		return true
	}

	return nativeExecutionFallbackAllowed() && btneb.delegate != nil
}

func (btneb *buildTaggedNativeExecutionBridge) currentFFIExecutor() NativeExecutionFFIExecutor {
	if btneb.ffiExecutorProvider == nil {
		return nil
	}

	return btneb.ffiExecutorProvider()
}

func (btneb *buildTaggedNativeExecutionBridge) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	ffiExecutor := btneb.currentFFIExecutor()
	if ffiExecutor != nil {
		result, err := ffiExecutor.Execute(ctx, logger, request)
		if err == nil {
			return result, nil
		}

		if !errors.Is(err, ErrNativeCryptographyUnavailable) {
			return nil, fmt.Errorf("native FFI executor execution failed: [%w]", err)
		}

		if !nativeExecutionFallbackAllowed() {
			return nil, err
		}

		if logger != nil {
			logger.Warnf(
				"native FFI executor unavailable; falling back to legacy bridge backend: [%v]",
				err,
			)
		}
	}

	if !nativeExecutionFallbackAllowed() {
		return nil, ErrNativeCryptographyUnavailable
	}

	if btneb.delegate == nil {
		return nil, ErrNativeCryptographyUnavailable
	}

	return btneb.delegate.Execute(ctx, logger, request)
}

func (btneb *buildTaggedNativeExecutionBridge) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	ffiExecutor := btneb.currentFFIExecutor()
	if ffiExecutor != nil {
		ffiExecutor.RegisterUnmarshallers(channel)
		return
	}

	if !nativeExecutionFallbackAllowed() {
		return
	}

	if btneb.delegate == nil {
		return
	}

	btneb.delegate.RegisterUnmarshallers(channel)
}
