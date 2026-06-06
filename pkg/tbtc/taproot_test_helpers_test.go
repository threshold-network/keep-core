package tbtc

import (
	"crypto/ecdsa"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func testWalletPublicKeyFromXOnly(t *testing.T, xOnlyHex string) *ecdsa.PublicKey {
	t.Helper()

	xOnlyBytes, err := hex.DecodeString(xOnlyHex)
	if err != nil {
		t.Fatalf("cannot decode x-only key: [%v]", err)
	}

	compressedPublicKey := append([]byte{0x02}, xOnlyBytes...)
	parsedPublicKey, err := btcec.ParsePubKey(compressedPublicKey, btcec.S256())
	if err != nil {
		t.Fatalf("cannot parse compressed public key: [%v]", err)
	}

	return &ecdsa.PublicKey{
		Curve: btcec.S256(),
		X:     parsedPublicKey.X,
		Y:     parsedPublicKey.Y,
	}
}

func testTaprootWalletMainUtxo(
	t *testing.T,
	bitcoinChain bitcoin.Chain,
	walletPublicKey *ecdsa.PublicKey,
) *bitcoin.UnspentTransactionOutput {
	t.Helper()

	walletXOnlyPublicKey, err := walletXOnlyPublicKey(walletPublicKey)
	if err != nil {
		t.Fatalf("cannot extract wallet x-only public key: [%v]", err)
	}

	taprootScript, err := bitcoin.PayToTaproot(walletXOnlyPublicKey)
	if err != nil {
		t.Fatalf("cannot compute Taproot wallet script: [%v]", err)
	}

	var previousTxHash bitcoin.Hash
	previousTxHash[0] = 0x01

	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: previousTxHash,
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: taprootScript,
			},
		},
	}

	if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatalf("cannot broadcast Taproot wallet main UTXO transaction: [%v]", err)
	}

	return &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}
}
