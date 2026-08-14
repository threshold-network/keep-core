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

func TestWalletPublicKeyHashFromLegacyWalletID(t *testing.T) {
	walletIDBytes, err := hex.DecodeString(
		"000000000000000000000000e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0",
	)
	if err != nil {
		t.Fatalf("failed to decode wallet ID: [%v]", err)
	}

	var walletID [32]byte
	copy(walletID[:], walletIDBytes)

	expectedWalletPublicKeyHashBytes, err := hex.DecodeString(
		"e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0",
	)
	if err != nil {
		t.Fatalf("failed to decode expected wallet public key hash: [%v]", err)
	}

	var expectedWalletPublicKeyHash [20]byte
	copy(expectedWalletPublicKeyHash[:], expectedWalletPublicKeyHashBytes)

	actualWalletPublicKeyHash, ok := WalletPublicKeyHashFromLegacyWalletID(walletID)
	if !ok {
		t.Fatal("expected wallet ID to be recognized as legacy")
	}

	if actualWalletPublicKeyHash != expectedWalletPublicKeyHash {
		t.Fatalf(
			"unexpected wallet public key hash\nexpected: [%x]\nactual:   [%x]",
			expectedWalletPublicKeyHash,
			actualWalletPublicKeyHash,
		)
	}
}

func TestWalletPublicKeyHashFromLegacyWalletID_NonLegacy(t *testing.T) {
	walletIDBytes, err := hex.DecodeString(
		"010000000000000000000000e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0",
	)
	if err != nil {
		t.Fatalf("failed to decode wallet ID: [%v]", err)
	}

	var walletID [32]byte
	copy(walletID[:], walletIDBytes)

	_, ok := WalletPublicKeyHashFromLegacyWalletID(walletID)
	if ok {
		t.Fatal("expected wallet ID to be recognized as non-legacy")
	}
}
