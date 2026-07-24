//go:build frost_native

package tbtc

import frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"

func currentFrostInteractiveSigningReadiness() bool {
	return completeFrostInteractiveSigningReadiness(
		frostsigning.InteractiveSigningReady(),
		frostsigning.InteractiveSigningOnlyEnabled(),
		frostsigning.NativeExecutionAvailable(),
		frostsigning.CurrentExecutionBackendName() == frostsigning.NativeExecutionBackendName,
	)
}
