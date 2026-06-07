package spv

import (
	"encoding/hex"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestIsWalletOutputScript_AcceptsFrostP2TR(t *testing.T) {
	walletPublicKeyHash := bytes20FromHex(
		t,
		"c7302d75072d78be94eb8d36c4b77583c7abb06e",
	)
	walletID := [32]byte{
		0x23, 0x36, 0xf6, 0x50, 0x04, 0xd8, 0xf1, 0x22,
		0xf1, 0xfe, 0x94, 0x7e, 0xbd, 0x00, 0x9a, 0x8b,
		0x4a, 0xdd, 0x3a, 0x0d, 0x93, 0x73, 0x56, 0xd5,
		0x68, 0xe3, 0x0f, 0x7f, 0xcc, 0x2e, 0x40, 0x08,
	}

	spvChain := newLocalChain()
	spvChain.setWallet(walletPublicKeyHash, &tbtc.WalletChainData{
		WalletID: walletID,
	})

	outputScript, err := bitcoin.PayToTaproot(walletID)
	if err != nil {
		t.Fatal(err)
	}

	isWalletOutput, err := isWalletOutputScript(
		walletPublicKeyHash,
		outputScript,
		spvChain,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !isWalletOutput {
		t.Fatal("expected FROST P2TR output to be recognized")
	}
}

func TestIsWalletOutputScript_DoesNotAcceptLegacyIDAsP2TR(t *testing.T) {
	walletPublicKeyHash := bytes20FromHex(
		t,
		"c7302d75072d78be94eb8d36c4b77583c7abb06e",
	)
	walletID := tbtc.DeriveLegacyWalletID(walletPublicKeyHash)

	spvChain := newLocalChain()
	spvChain.setWallet(walletPublicKeyHash, &tbtc.WalletChainData{
		WalletID: walletID,
	})

	outputScript, err := bitcoin.PayToTaproot(walletID)
	if err != nil {
		t.Fatal(err)
	}

	isWalletOutput, err := isWalletOutputScript(
		walletPublicKeyHash,
		outputScript,
		spvChain,
	)
	if err != nil {
		t.Fatal(err)
	}

	if isWalletOutput {
		t.Fatal("expected legacy wallet ID P2TR output to be rejected")
	}
}

func bytes20FromHex(t *testing.T, hexString string) [20]byte {
	t.Helper()

	decoded, err := hex.DecodeString(hexString)
	if err != nil {
		t.Fatal(err)
	}

	var result [20]byte
	copy(result[:], decoded)

	return result
}
