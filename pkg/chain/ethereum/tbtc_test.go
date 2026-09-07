package ethereum

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
)

func TestConvertPubKeyToChainFormat(t *testing.T) {
	bytes30 := []byte{229, 19, 136, 216, 125, 157, 135, 142, 67, 130,
		136, 13, 76, 188, 32, 218, 243, 134, 95, 73, 155, 24, 38, 73, 117, 90,
		215, 95, 216, 19}
	bytes31 := []byte{182, 142, 176, 51, 131, 130, 111, 197, 191, 103, 180, 137,
		171, 101, 34, 78, 251, 234, 118, 184, 16, 116, 238, 82, 131, 153, 134,
		17, 46, 158, 94}

	expectedResult := [64]byte{
		// padding
		00, 00,
		// bytes30
		229, 19, 136, 216, 125, 157, 135, 142, 67, 130, 136, 13, 76, 188, 32,
		218, 243, 134, 95, 73, 155, 24, 38, 73, 117, 90, 215, 95, 216, 19,
		// padding
		00,
		// bytes31
		182, 142, 176, 51, 131, 130, 111, 197, 191, 103, 180, 137, 171, 101, 34,
		78, 251, 234, 118, 184, 16, 116, 238, 82, 131, 153, 134, 17, 46, 158, 94,
	}

	actualResult, err := convertPubKeyToChainFormat(
		&ecdsa.PublicKey{
			X: new(big.Int).SetBytes(bytes30),
			Y: new(big.Int).SetBytes(bytes31),
		},
	)

	if err != nil {
		t.Errorf("unexpected error [%v]", err)
	}

	testutils.AssertBytesEqual(
		t,
		expectedResult[:],
		actualResult[:],
	)
}
