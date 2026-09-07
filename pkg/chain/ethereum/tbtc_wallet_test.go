package ethereum

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
)

func TestCalculateWalletID(t *testing.T) {
	hexToByte32 := func(hexStr string) [32]byte {
		if len(hexStr) != 64 {
			t.Fatal("hex string length incorrect")
		}

		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			t.Fatal(err)
		}

		var result [32]byte
		copy(result[:], decoded)

		return result
	}

	xBytes := hexToByte32(
		"9a0544440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf03291c4",
	)

	yBytes := hexToByte32(
		"73e661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
	)

	walletPublicKey := &ecdsa.PublicKey{
		Curve: local_v1.DefaultCurve,
		X:     new(big.Int).SetBytes(xBytes[:]),
		Y:     new(big.Int).SetBytes(yBytes[:]),
	}

	actualWalletID, err := calculateWalletID(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	expectedWalletID := hexToByte32(
		"a6602e554b8cf7c23538fd040e4ff3520ec680e5e5ce9a075259e613a3e5aa79",
	)

	testutils.AssertBytesEqual(t, expectedWalletID[:], actualWalletID[:])
}

func TestComputeMainUtxoHash(t *testing.T) {
	transactionHash, err := bitcoin.NewHashFromString(
		"089bd0671a4481c3584919b4b9b6751cb3f8586dab41cb157adec43fd10ccc00",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	mainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: transactionHash,
			OutputIndex:     5,
		},
		Value: 143565433,
	}

	mainUtxoHash := computeMainUtxoHash(mainUtxo)

	expectedMainUtxoHash, err := hex.DecodeString(
		"1216f8e993c4c57d3c4c971c0d2651140fc4ab09d41960d9ccd7b41fdcd270d6",
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBytesEqual(t, expectedMainUtxoHash, mainUtxoHash[:])
}
