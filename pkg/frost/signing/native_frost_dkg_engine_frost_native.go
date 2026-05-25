//go:build frost_native

package signing

import (
	"encoding/json"
	"fmt"
)

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

// SignerMaterial converts the DKG output into the existing FrostUniFFIV2
// signer-material envelope used by native FROST signing.
func (nfdkg *NativeFROSTDKGResult) SignerMaterial() (*NativeSignerMaterial, error) {
	if nfdkg == nil {
		return nil, fmt.Errorf("native FROST DKG result is nil")
	}

	material := &nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage:       nfdkg.KeyPackage,
		PublicKeyPackage: nfdkg.PublicKeyPackage,
	}
	if err := material.validate(); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(material)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal native FROST DKG signer material: [%w]", err)
	}

	return &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV2,
		Payload: payload,
	}, nil
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
