//go:build !(frost_native && frost_roast_retry)

package signing

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
)

// driveInteractiveRoastSigningIfEnabled is a permanent no-op unless BOTH the
// frost_native and frost_roast_retry build tags are set. Interactive ROAST
// signing needs the native runner/engine/bus (frost_native) AND a live
// orchestration handle + coordinator registry (frost_roast_retry); without both
// the executor always uses the coarse signing path. Returning handled=false
// tells the executor to fall through to the coarse primitive.
func driveInteractiveRoastSigningIfEnabled(
	_ context.Context,
	_ log.StandardLogger,
	_ *NativeExecutionFFISigningRequest,
) (*frost.Signature, bool, error) {
	return nil, false, nil
}
