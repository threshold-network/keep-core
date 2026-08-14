//go:build frost_native

package signing

const (
	// NativeSignerMaterialFormatFrostUniFFIV2 is the unsupported generic UniFFI
	// FROST signer-material envelope. It is kept as a string constant so stale
	// local/test material can be identified and rejected explicitly.
	NativeSignerMaterialFormatFrostUniFFIV2 = "frost-uniffi-v2"
)

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
// nonces are one-time secrets. The generic UniFFI signing protocol that used
// this type is no longer registered; it remains only as a tbtc-signer FFI DTO.
type NativeFROSTNonces struct {
	Data []byte `json:"data"`
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

func zeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
