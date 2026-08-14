package tbtc

import (
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/frost"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func mustFrostSignatureFromBigInts(r *big.Int, s *big.Int) *frost.Signature {
	return mustFrostSignatureFromTECDSA(&tecdsa.Signature{R: r, S: s})
}

func mustFrostSignatureFromTECDSA(signature *tecdsa.Signature) *frost.Signature {
	result, err := frostsigning.FromTECDSASignature(signature)
	if err != nil {
		panic(fmt.Sprintf("signature conversion failed: %v", err))
	}

	return result
}
