package electrum

import (
	"bytes"
	"encoding/hex"

	"github.com/btcsuite/btcd/wire"
	"github.com/checksum0/go-electrum/electrum"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// convertBlockHeader transforms a BlockHeader returned from Electrum protocol to
// the format expected by the bitcoin.Chain interface.
func convertBlockHeader(electrumResult *electrum.GetBlockHeaderResult) (*bitcoin.BlockHeader, error) {
	headerBytes, err := hex.DecodeString(electrumResult.Header)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(headerBytes)

	var b wire.BlockHeader
	if err := b.Deserialize(buf); err != nil {
		return nil, err
	}

	result := &bitcoin.BlockHeader{
		Version:                 b.Version,
		PreviousBlockHeaderHash: bitcoin.Hash(b.PrevBlock),
		MerkleRootHash:          bitcoin.Hash(b.MerkleRoot),
		Time:                    uint32(b.Timestamp.Unix()),
		Bits:                    b.Bits,
		Nonce:                   b.Nonce,
	}

	return result, nil
}
