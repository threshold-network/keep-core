//go:build frost_native

package signing

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// buildTaggedNativeExecutionAdapter is a transitional adapter wired when the
// frost_native build tag is enabled.
//
// The adapter uses a native execution bridge when available.
//
// Backend mode behavior:
//   - `native`: fallback to legacy bridge when native cryptography is unavailable
//   - `ffi`: no fallback; native cryptographic execution is required
type buildTaggedNativeExecutionAdapter struct {
	nativeBridgeProvider func() NativeExecutionBridge
	fallback             ExecutionBackend
}

func registerNativeExecutionAdapterForBuild() {
	err := RegisterNativeExecutionAdapter(newBuildTaggedNativeExecutionAdapter())
	if err != nil {
		panic(fmt.Sprintf("failed to register build-tagged native adapter: [%v]", err))
	}
}

func newBuildTaggedNativeExecutionAdapter() *buildTaggedNativeExecutionAdapter {
	return &buildTaggedNativeExecutionAdapter{
		nativeBridgeProvider: newNativeExecutionBridge,
		fallback:             newLegacyExecutionBackend(),
	}
}

func (btnea *buildTaggedNativeExecutionAdapter) NativeExecutionAvailable() bool {
	nativeBridge := btnea.currentNativeBridge()
	return nativeBridge != nil && nativeBridge.IsAvailable()
}

func (btnea *buildTaggedNativeExecutionAdapter) currentNativeBridge() NativeExecutionBridge {
	if btnea.nativeBridgeProvider == nil {
		return nil
	}

	return btnea.nativeBridgeProvider()
}

func (btnea *buildTaggedNativeExecutionAdapter) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	nativeBridge := btnea.currentNativeBridge()
	if nativeBridge != nil && nativeBridge.IsAvailable() {
		result, err := nativeBridge.Execute(ctx, logger, request)
		if err == nil {
			return result, nil
		}

		if !errors.Is(err, ErrNativeCryptographyUnavailable) {
			return nil, fmt.Errorf("native bridge execution failed: [%w]", err)
		}

		if !nativeExecutionFallbackAllowed() {
			return nil, err
		}

		if logger != nil {
			logger.Warnf(
				"native FROST cryptography unavailable; falling back to legacy bridge backend: [%v]",
				err,
			)
		}
	}

	if !nativeExecutionFallbackAllowed() {
		return nil, ErrNativeCryptographyUnavailable
	}

	if btnea.fallback == nil {
		return nil, fmt.Errorf("fallback execution backend is nil")
	}

	return btnea.fallback.Execute(ctx, logger, request)
}

func (btnea *buildTaggedNativeExecutionAdapter) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	nativeBridge := btnea.currentNativeBridge()
	if nativeBridge != nil && nativeBridge.IsAvailable() {
		nativeBridge.RegisterUnmarshallers(channel)
		return
	}

	if !nativeExecutionFallbackAllowed() {
		return
	}

	if btnea.fallback == nil {
		return
	}

	btnea.fallback.RegisterUnmarshallers(channel)
}
