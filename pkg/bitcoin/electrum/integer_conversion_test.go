package electrum

import (
	"math"
	"strconv"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestElectrumBlockHeightBounds(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("uint cannot exceed the Electrum height range")
	}
	height64 := uint64(math.MaxUint32) + 1
	height := uint(height64)
	connection := new(Connection)
	if _, err := connection.GetBlockHeader(height); err == nil {
		t.Fatal("accepted out-of-range block header height")
	}
	if _, err := connection.GetTransactionMerkleProof(bitcoin.Hash{}, height); err == nil {
		t.Fatal("accepted out-of-range merkle proof height")
	}
	if _, err := connection.GetCoinbaseTxHash(height); err == nil {
		t.Fatal("accepted out-of-range coinbase height")
	}
}

func TestConvertUtxoItemsAmountBounds(t *testing.T) {
	for _, value := range []uint64{0, 1, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64} {
		utxos, err := convertUtxoItems([]*scriptUtxoItem{{value: value, outputIndex: 3}})
		if value > math.MaxInt64 {
			if err == nil {
				t.Fatalf("accepted amount %d", value)
			}
		} else if err != nil || len(utxos) != 1 ||
			uint64(utxos[0].Value) != value || utxos[0].Outpoint.OutputIndex != 3 {
			t.Fatalf("amount %d: got %v, %v", value, utxos, err)
		}
	}
}
