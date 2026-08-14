//go:build frost_native

package signing

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
	// from peers. The tbtc-signer DKG Part3 request keys round-two packages by
	// sender while the package itself carries the recipient.
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
