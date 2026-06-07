//go:build frost_native

package signing

import (
	"fmt"
	"sync"
)

const (
	// NativeSignerMaterialFormatFrostUniFFIV2 is the unsupported generic UniFFI
	// FROST signer-material envelope. It is kept as a string constant so stale
	// local/test material can be identified and rejected explicitly.
	NativeSignerMaterialFormatFrostUniFFIV2 = "frost-uniffi-v2"
)

var nativeFROSTSigningEngine NativeFROSTSigningEngine

// NativeFROSTKeyPackage carries native key-package bytes and participant
// identifier expected by the native FROST engine.
type NativeFROSTKeyPackage struct {
	Identifier string `json:"identifier"`
	Data       []byte `json:"data"`
}

// NativeFROSTPublicKeyPackage carries native public-key-package payload.
type NativeFROSTPublicKeyPackage struct {
	VerifyingShares map[string]string `json:"verifyingShares"`
	VerifyingKey    string            `json:"verifyingKey"`
}

// NativeFROSTNonces is round-one signer-local nonce material. FROST signing
// nonces are one-time secrets: a NativeFROSTSigningEngine must consume them in
// exactly one Sign call and reject later reuse of the same object.
type NativeFROSTNonces struct {
	Data     []byte `json:"data"`
	lock     sync.Mutex
	consumed bool
}

// NativeFROSTCommitment is round-one commitment shared with the group.
type NativeFROSTCommitment struct {
	Identifier string `json:"identifier"`
	Data       []byte `json:"data"`
}

// NativeFROSTSigningPackage is coordinator-computed package used in round two.
type NativeFROSTSigningPackage struct {
	Data []byte `json:"data"`
}

// NativeFROSTSignatureShare is round-two signature share.
type NativeFROSTSignatureShare struct {
	Identifier string `json:"identifier"`
	Data       []byte `json:"data"`
}

type nativeFROSTCommitment struct {
	Identifier string
	Data       []byte
}

type nativeFROSTSignatureShare struct {
	Identifier string
	Data       []byte
}

func (nfn *NativeFROSTNonces) consumeData() ([]byte, error) {
	if nfn == nil {
		return nil, fmt.Errorf("nonces are nil")
	}

	nfn.lock.Lock()
	defer nfn.lock.Unlock()

	if nfn.consumed {
		return nil, fmt.Errorf("nonces are already consumed")
	}

	if len(nfn.Data) == 0 {
		return nil, fmt.Errorf("nonces data is empty")
	}

	consumedData := append([]byte{}, nfn.Data...)
	zeroBytes(nfn.Data)
	nfn.Data = nil
	nfn.consumed = true

	return consumedData, nil
}

func zeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// NativeFROSTSigningEngine executes cryptographic round operations needed by
// the native FROST signing protocol.
type NativeFROSTSigningEngine interface {
	GenerateNoncesAndCommitments(
		keyPackage *NativeFROSTKeyPackage,
	) (*NativeFROSTNonces, *NativeFROSTCommitment, error)
	NewSigningPackage(
		message []byte,
		commitments []*NativeFROSTCommitment,
	) (*NativeFROSTSigningPackage, error)
	Sign(
		signingPackage *NativeFROSTSigningPackage,
		nonces *NativeFROSTNonces,
		keyPackage *NativeFROSTKeyPackage,
	) (*NativeFROSTSignatureShare, error)
	Aggregate(
		signingPackage *NativeFROSTSigningPackage,
		signatureShares []*NativeFROSTSignatureShare,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
	) ([]byte, error)
}

// NativeFROSTTaprootTweakedSigningEngine executes BIP-341 tweaked FROST
// signing operations for Taproot key-path spends that commit to a script tree.
type NativeFROSTTaprootTweakedSigningEngine interface {
	SignWithTaprootTweak(
		signingPackage *NativeFROSTSigningPackage,
		nonces *NativeFROSTNonces,
		keyPackage *NativeFROSTKeyPackage,
		taprootMerkleRoot []byte,
	) (*NativeFROSTSignatureShare, error)
	AggregateWithTaprootTweak(
		signingPackage *NativeFROSTSigningPackage,
		signatureShares []*NativeFROSTSignatureShare,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
		taprootMerkleRoot []byte,
	) ([]byte, error)
}

// RegisterNativeFROSTSigningEngine registers the native FROST cryptographic
// engine used by the tagged native-signing primitive.
func RegisterNativeFROSTSigningEngine(
	engine NativeFROSTSigningEngine,
) error {
	if engine == nil {
		return fmt.Errorf("native FROST signing engine is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeFROSTSigningEngine = engine

	return nil
}

// UnregisterNativeFROSTSigningEngine clears native FROST signing engine
// registration.
func UnregisterNativeFROSTSigningEngine() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeFROSTSigningEngine = nil
}

func currentNativeFROSTSigningEngine() NativeFROSTSigningEngine {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeFROSTSigningEngine
}
