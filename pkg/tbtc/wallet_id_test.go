package tbtc

import (
	"encoding/hex"
	"testing"
)

func TestDeriveLegacyWalletID(t *testing.T) {
	walletPublicKeyHashBytes, err := hex.DecodeString(
		"e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0",
	)
	if err != nil {
		t.Fatalf("failed to decode wallet public key hash: [%v]", err)
	}

	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], walletPublicKeyHashBytes)

	expectedWalletIDBytes, err := hex.DecodeString(
		"000000000000000000000000e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0",
	)
	if err != nil {
		t.Fatalf("failed to decode expected wallet ID: [%v]", err)
	}

	var expectedWalletID [32]byte
	copy(expectedWalletID[:], expectedWalletIDBytes)

	actualWalletID := DeriveLegacyWalletID(walletPublicKeyHash)
	if actualWalletID != expectedWalletID {
		t.Fatalf(
			"unexpected wallet ID\nexpected: [%x]\nactual:   [%x]",
			expectedWalletID,
			actualWalletID,
		)
	}
}
