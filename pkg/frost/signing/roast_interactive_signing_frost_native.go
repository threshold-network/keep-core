//go:build frost_native

package signing

import (
	"fmt"
	"sync"
)

// interactiveSigningEngineProvider is the package-level injection seam for the
// interactiveSigningEngine the executor drives in the interactive ROAST path.
// It is a provider (factory) rather than a stored instance so each attempt gets
// a fresh engine handle and so the registration site need not import the
// concrete engine where the executor lives.
//
// Tests register a provider returning a programmable fake (under frost_native
// alone, no cgo). Production registers a provider returning the cgo-backed
// buildTaggedTBTCSignerEngine under frost_native && frost_tbtc_signer && cgo.
// Registration alone does NOT activate interactive signing: the executor still
// requires the InteractiveSigningOptInEnvVar audit gate (see
// roast_interactive_signing_gate.go), so the cgo engine stays dormant until the
// engine audit clears even on a build that has registered it.
var (
	interactiveSigningEngineProviderMu sync.RWMutex
	interactiveSigningEngineProvider   func() interactiveSigningEngine
)

// RegisterInteractiveSigningEngineProvider installs the provider the executor
// uses to obtain an interactiveSigningEngine. A later registration fully
// replaces an earlier one. Passing nil clears the registration (the executor
// then falls back to the coarse path).
func RegisterInteractiveSigningEngineProvider(provider func() interactiveSigningEngine) {
	interactiveSigningEngineProviderMu.Lock()
	defer interactiveSigningEngineProviderMu.Unlock()
	interactiveSigningEngineProvider = provider
}

// registeredInteractiveSigningEngine returns a fresh engine from the registered
// provider, or nil when no provider is registered (or the provider itself
// returns nil). A nil result tells the executor to use the coarse path.
func registeredInteractiveSigningEngine() interactiveSigningEngine {
	interactiveSigningEngineProviderMu.RLock()
	provider := interactiveSigningEngineProvider
	interactiveSigningEngineProviderMu.RUnlock()
	if provider == nil {
		return nil
	}
	return provider()
}

// InteractiveSigningReady reports whether this process has every pre-wallet
// prerequisite needed to sign material produced by distributed DKG: both
// operator opt-ins, the ROAST transition producer, and an interactive engine.
// It intentionally does not require a wallet-scoped coordinator registration;
// that registration can happen only after DKG has persisted the wallet's key
// group and is enforced by the signing path when the wallet is used.
func InteractiveSigningReady() bool {
	return InteractiveSigningOptInEnabled() &&
		RoastRetryInfrastructureReady() &&
		registeredInteractiveSigningEngine() != nil
}

// ResetInteractiveSigningEngineProviderForTest clears the registered provider.
// Tests defer it so a registration does not leak into other tests.
func ResetInteractiveSigningEngineProviderForTest() {
	RegisterInteractiveSigningEngineProvider(nil)
}

// interactiveRoastSigningThreshold resolves the FROST signing threshold for the
// interactive attempt from the persisted DKG key-group material.
//
// The threshold for a persisted FROST key group is fixed at DKG time
// (payload.DKGThreshold); it is NOT the per-attempt dishonest-threshold the
// coarse bootstrap path derives via DishonestThreshold+1. Driving the runner
// with DishonestThreshold+1 would sign under the wrong t-of-n for a real
// persisted group, so the interactive path reads DKGThreshold directly and
// hard-checks it against the participant set (mirroring the coarse persisted
// path's validation in buildTaggedTBTCSignerRunDKGInputsForPayload).
func interactiveRoastSigningThreshold(request *NativeExecutionFFISigningRequest) (uint16, error) {
	if request == nil {
		return 0, fmt.Errorf("request is nil")
	}
	payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(request.SignerMaterial)
	if err != nil {
		return 0, fmt.Errorf("decode signer material payload: %w", err)
	}
	if payload.KeyGroupSource != NativeTBTCSignerKeyGroupSourceDKGPersisted {
		return 0, fmt.Errorf(
			"interactive signing requires a persisted DKG key group, got key-group source %q",
			payload.KeyGroupSource,
		)
	}
	if payload.DKGThreshold == 0 {
		return 0, fmt.Errorf("persisted DKG threshold is zero")
	}
	if int(payload.DKGThreshold) > len(payload.DKGParticipants) {
		return 0, fmt.Errorf(
			"persisted DKG threshold exceeds participant count: [%d] > [%d]",
			payload.DKGThreshold,
			len(payload.DKGParticipants),
		)
	}
	return payload.DKGThreshold, nil
}
