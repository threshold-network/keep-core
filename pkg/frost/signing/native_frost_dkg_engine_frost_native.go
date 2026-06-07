//go:build frost_native

package signing

import "fmt"

// NativeFROSTDKGRound1Package is the public package broadcast during FROST DKG
// round one.
type NativeFROSTDKGRound1Package struct {
	Identifier string `json:"identifier"`
	Data       []byte `json:"data"`
}

// NativeFROSTDKGRound2Package is the package sent to a specific DKG
// participant during FROST DKG round two.
type NativeFROSTDKGRound2Package struct {
	// Identifier is the recipient participant identifier embedded by the
	// native DKG package.
	Identifier string `json:"identifier"`
	// SenderIdentifier is filled by the Go coordinator for packages received
	// from peers. UniFFI Part3 needs to key round-two packages by the sender
	// while the package itself carries the recipient.
	SenderIdentifier string `json:"senderIdentifier,omitempty"`
	Data             []byte `json:"data"`
}

// NativeFROSTDKGRound1SecretPackage is signer-local secret material produced
// in DKG round one. It must never be broadcast.
type NativeFROSTDKGRound1SecretPackage struct {
	Data []byte `json:"data"`
}

// NativeFROSTDKGRound2SecretPackage is signer-local secret material produced
// in DKG round two. It must never be broadcast.
type NativeFROSTDKGRound2SecretPackage struct {
	Data []byte `json:"data"`
}

// NativeFROSTDKGPart1Result is the output of native FROST DKG part one.
type NativeFROSTDKGPart1Result struct {
	SecretPackage *NativeFROSTDKGRound1SecretPackage `json:"secretPackage"`
	Package       *NativeFROSTDKGRound1Package       `json:"package"`
}

// NativeFROSTDKGPart2Result is the output of native FROST DKG part two.
type NativeFROSTDKGPart2Result struct {
	SecretPackage *NativeFROSTDKGRound2SecretPackage `json:"secretPackage"`
	Packages      []*NativeFROSTDKGRound2Package     `json:"packages"`
}

// NativeFROSTDKGResult is the final native FROST DKG output consumed by the
// signing runtime and persisted by keep-core.
type NativeFROSTDKGResult struct {
	KeyPackage       *NativeFROSTKeyPackage       `json:"keyPackage"`
	PublicKeyPackage *NativeFROSTPublicKeyPackage `json:"publicKeyPackage"`
}

// SignerMaterial rejects the unsupported generic UniFFI FROST DKG output.
// FROST wallet material must be persisted through the tbtc-signer engine so
// Taproot tweaked signing is available for deposit sweeps.
func (nfdkg *NativeFROSTDKGResult) SignerMaterial() (*NativeSignerMaterial, error) {
	if nfdkg == nil {
		return nil, fmt.Errorf("native FROST DKG result is nil")
	}

	return nil, fmt.Errorf(
		"native FROST DKG result cannot be persisted as unsupported [%s] signer material; use [%s]",
		NativeSignerMaterialFormatFrostUniFFIV2,
		NativeSignerMaterialFormatFrostTBTCSignerV1,
	)
}

// NativeFROSTDKGEngine executes the cryptographic primitives for the three
// FROST DKG parts. It intentionally exposes only serializable package data to
// the coordinator; the bridge implementation is responsible for adapting these
// values to the underlying UniFFI/tbtc-signer handle model.
type NativeFROSTDKGEngine interface {
	Part1(
		participantIdentifier string,
		maxSigners uint16,
		minSigners uint16,
	) (*NativeFROSTDKGPart1Result, error)
	Part2(
		secretPackage *NativeFROSTDKGRound1SecretPackage,
		round1Packages []*NativeFROSTDKGRound1Package,
	) (*NativeFROSTDKGPart2Result, error)
	Part3(
		secretPackage *NativeFROSTDKGRound2SecretPackage,
		round1Packages []*NativeFROSTDKGRound1Package,
		round2Packages []*NativeFROSTDKGRound2Package,
	) (*NativeFROSTDKGResult, error)
}

var nativeFROSTDKGEngine NativeFROSTDKGEngine

// RegisterNativeFROSTDKGEngine registers the native FROST DKG cryptographic
// engine used by the FROST wallet-registry coordinator.
func RegisterNativeFROSTDKGEngine(engine NativeFROSTDKGEngine) error {
	if engine == nil {
		return fmt.Errorf("native FROST DKG engine is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeFROSTDKGEngine = engine

	return nil
}

// UnregisterNativeFROSTDKGEngine clears native FROST DKG engine registration.
func UnregisterNativeFROSTDKGEngine() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeFROSTDKGEngine = nil
}

func currentNativeFROSTDKGEngine() NativeFROSTDKGEngine {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeFROSTDKGEngine
}

// CurrentNativeFROSTDKGEngine returns the registered native FROST DKG engine.
func CurrentNativeFROSTDKGEngine() NativeFROSTDKGEngine {
	return currentNativeFROSTDKGEngine()
}
