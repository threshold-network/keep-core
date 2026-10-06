package tecdsa

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/common"
)

// Signature holds a signature in a form of two big.Int `r` and `s` values and
// a recovery ID value in {0, 1, 2, 3}.
//
// The signature is chain-agnostic. Some chains (e.g. Ethereum and BTC)
// require `v` to start from 27. Please consult the documentation about
// what the particular chain expects.
type Signature struct {
	R          *big.Int
	S          *big.Int
	RecoveryID int8
}

// NewSignature constructs a new instance of the tECDSA signature based on
// the signing result.
func NewSignature(data *common.SignatureData) *Signature {
	// `SignatureData` contains recovery ID as a byte slice. Only the first
	// byte is relevant and is converted to `int`.
	recoveryBytes := data.GetSignatureRecovery()
	recoveryInt := 0
	recoveryInt = (recoveryInt << 8) | int(recoveryBytes[0])

	return &Signature{
		R: new(big.Int).SetBytes(data.GetR()),
		S: new(big.Int).SetBytes(data.GetS()),
		// #nosec G115 -- tss-lib signing output supplies a recovery ID in {0, 1, 2, 3}.
		RecoveryID: int8(recoveryInt),
	}
}

// String formats Signature to a string that contains R and S values
// as hexadecimals.
func (s *Signature) String() string {
	return fmt.Sprintf(
		"R: 0x%s, S: 0x%s, RecoveryID: %d",
		hex.EncodeToString(s.R.Bytes()),
		hex.EncodeToString(s.S.Bytes()),
		s.RecoveryID,
	)
}

// Equals determines the equality of two signatures.
func (s *Signature) Equals(other *Signature) bool {
	if s == nil || other == nil {
		return s == other
	}

	if s.R == nil || other.R == nil {
		if s.R != other.R {
			return false
		}
	} else {
		if s.R.Cmp(other.R) != 0 {
			return false
		}
	}

	if s.S == nil || other.S == nil {
		if s.S != other.S {
			return false
		}
	} else {
		if s.S.Cmp(other.S) != 0 {
			return false
		}
	}

	if s.RecoveryID != other.RecoveryID {
		return false
	}

	return true
}
