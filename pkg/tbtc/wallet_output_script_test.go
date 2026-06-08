package tbtc

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestWalletOutputScript_LegacyWalletID(t *testing.T) {
	walletPublicKeyHash := hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e")
	walletID := DeriveLegacyWalletID(walletPublicKeyHash)

	actualScript, err := WalletOutputScript(walletPublicKeyHash, walletID)
	if err != nil {
		t.Fatal(err)
	}

	expectedScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actualScript, expectedScript) {
		t.Fatalf("unexpected legacy wallet script: [%x]", actualScript)
	}
}

func TestWalletOutputScript_ZeroWalletIDFallsBackToLegacy(t *testing.T) {
	walletPublicKeyHash := hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e")

	actualScript, err := WalletOutputScript(walletPublicKeyHash, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}

	expectedScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actualScript, expectedScript) {
		t.Fatalf("unexpected zero-ID wallet script: [%x]", actualScript)
	}
}

func TestWalletOutputScript_FrostWalletID(t *testing.T) {
	walletPublicKeyHash := hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e")
	walletID := [32]byte{
		0x23, 0x36, 0xf6, 0x50, 0x04, 0xd8, 0xf1, 0x22,
		0xf1, 0xfe, 0x94, 0x7e, 0xbd, 0x00, 0x9a, 0x8b,
		0x4a, 0xdd, 0x3a, 0x0d, 0x93, 0x73, 0x56, 0xd5,
		0x68, 0xe3, 0x0f, 0x7f, 0xcc, 0x2e, 0x40, 0x08,
	}

	actualScript, err := WalletOutputScript(walletPublicKeyHash, walletID)
	if err != nil {
		t.Fatal(err)
	}

	expectedScript, err := bitcoin.PayToTaproot(walletID)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actualScript, expectedScript) {
		t.Fatalf("unexpected FROST wallet script: [%x]", actualScript)
	}
}

func TestWalletOutputScript_LegacyWalletIDMismatch(t *testing.T) {
	walletPublicKeyHash := hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e")
	walletID := DeriveLegacyWalletID(
		hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
	)

	_, err := WalletOutputScript(walletPublicKeyHash, walletID)
	if err == nil {
		t.Fatal("expected legacy wallet ID mismatch error")
	}
}
