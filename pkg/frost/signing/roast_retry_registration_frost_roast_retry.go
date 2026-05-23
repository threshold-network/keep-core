//go:build frost_roast_retry

package signing

import (
	"sync"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

// RoastRetryDeps bundles the per-process dependencies the FROST
// receive loops need under the frost_roast_retry build tag. See the
// default-build file for the doc contract; this declaration is the
// real one used when the build tag is active.
type RoastRetryDeps struct {
	Coordinator roast.Coordinator
	Signer      roast.Signer
	Verifier    roast.SignatureVerifier
	SelfMember  uint32
}

// roastRetryRegistration is the package-private registry slot. Only
// one set of dependencies can be registered at a time; later
// registrations overwrite earlier ones. Callers wanting to test
// reset behaviour use ResetRoastRetryRegistrationForTest.
var (
	roastRetryRegistrationMu sync.RWMutex
	roastRetryRegistration   RoastRetryDeps
	roastRetryRegistered     bool
)

// RegisterRoastRetryCoordinator stores the per-process ROAST-retry
// dependencies the receive loops will pick up on their next call.
// Safe for concurrent registration / lookup; a later registration
// fully replaces an earlier one (this is the documented behaviour --
// reconfiguring at runtime is intentional).
func RegisterRoastRetryCoordinator(deps RoastRetryDeps) {
	roastRetryRegistrationMu.Lock()
	defer roastRetryRegistrationMu.Unlock()
	roastRetryRegistration = deps
	roastRetryRegistered = true
}

// RegisteredRoastRetryCoordinator returns the currently-registered
// dependencies and true, or the zero value and false if nothing has
// been registered yet. Receivers use the boolean to decide between
// the bounded recorder path and the Phase-2 NoOp fallback.
func RegisteredRoastRetryCoordinator() (RoastRetryDeps, bool) {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	if !roastRetryRegistered {
		return RoastRetryDeps{}, false
	}
	return roastRetryRegistration, true
}

// ResetRoastRetryRegistrationForTest clears the registry. Exposed
// so tests in this and downstream packages can reset between cases
// without leaking state. Not intended for production code paths.
func ResetRoastRetryRegistrationForTest() {
	roastRetryRegistrationMu.Lock()
	defer roastRetryRegistrationMu.Unlock()
	roastRetryRegistration = RoastRetryDeps{}
	roastRetryRegistered = false
}
