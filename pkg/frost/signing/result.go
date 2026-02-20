package signing

import "github.com/keep-network/keep-core/pkg/frost"

// Result of the FROST signing protocol.
type Result struct {
	// Signature is the BIP-340-style signature produced as result of signing.
	Signature *frost.Signature
}
