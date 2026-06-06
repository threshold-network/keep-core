package tbtc

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/keep-network/keep-core/pkg/internal/byteutils"
)

func walletXOnlyPublicKey(walletPublicKey *ecdsa.PublicKey) ([32]byte, error) {
	x, err := byteutils.LeftPadTo32Bytes(walletPublicKey.X.Bytes())
	if err != nil {
		return [32]byte{}, fmt.Errorf("cannot encode wallet x-only key: [%w]", err)
	}

	var result [32]byte
	copy(result[:], x)

	return result, nil
}
