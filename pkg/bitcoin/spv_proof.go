package bitcoin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/keep-network/keep-core/pkg/internal/byteutils"
)

// SpvProof contains data required to perform a proof that a given transaction
// was included in the Bitcoin blockchain.
type SpvProof struct {
	// MerkleProof is the Merkle proof of transaction inclusion in a block.
	MerkleProof []byte

	// TxIndexInBlock is the transaction index in the block (0-indexed).
	TxIndexInBlock uint

	// BitcoinHeaders is a chain of block headers that form confirmations of
	// blockchain inclusion.
	BitcoinHeaders []byte

	// CoinbasePreimage is the sha256 preimage of the coinbase transaction hash
	// i.e., the sha256 hash of the coinbase transaction.
	CoinbasePreimage [32]byte

	// CoinbaseProof is the Merkle proof of coinbase transaction inclusion in
	// a block.
	CoinbaseProof []byte
}

// AssembleSpvProof assembles a proof that a given transaction was included in
// the blockchain and has accumulated the required number of confirmations.
func AssembleSpvProof(
	transactionHash Hash,
	requiredConfirmations uint,
	btcChain Chain,
) (*Transaction, *SpvProof, error) {
	confirmations, err := btcChain.GetTransactionConfirmations(
		context.Background(),
		transactionHash,
	)
	if err != nil {
		return nil, nil, err
	}

	if confirmations < requiredConfirmations {
		return nil, nil, fmt.Errorf(
			"transaction confirmations number[%v] is not enough, required [%v]",
			confirmations,
			requiredConfirmations,
		)
	}

	transaction, err := btcChain.GetTransaction(transactionHash)
	if err != nil {
		return nil, nil, err
	}

	latestBlockHeight, err := btcChain.GetLatestBlockHeight()
	if err != nil {
		return nil, nil, err
	}

	txBlockHeight := latestBlockHeight - confirmations + 1

	headersChain, err := getHeadersChain(
		btcChain,
		txBlockHeight,
		requiredConfirmations,
	)
	if err != nil {
		return nil, nil, err
	}

	merkleBranch, err := btcChain.GetTransactionMerkleProof(
		transactionHash,
		txBlockHeight,
	)
	if err != nil {
		return nil, nil, err
	}

	merkleProof, err := createMerkleProof(merkleBranch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Merkle proof [%w]", err)
	}

	coinbaseTxHash, err := btcChain.GetCoinbaseTxHash(txBlockHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get coinbase tx hash [%w]", err)
	}

	coinbaseTx, err := btcChain.GetTransaction(coinbaseTxHash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get coinbase tx [%w]", err)
	}

	coinbasePreimage := sha256.Sum256(coinbaseTx.Serialize(Standard))

	coinbaseMerkleBranch, err := btcChain.GetTransactionMerkleProof(
		coinbaseTxHash,
		txBlockHeight,
	)
	if err != nil {
		return nil, nil, err
	}

	coinbaseMerkleProof, err := createMerkleProof(coinbaseMerkleBranch)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to create coinbase Merkle proof [%w]",
			err,
		)
	}

	proof := &SpvProof{
		MerkleProof:      merkleProof,
		TxIndexInBlock:   merkleBranch.Position,
		BitcoinHeaders:   headersChain,
		CoinbasePreimage: coinbasePreimage,
		CoinbaseProof:    coinbaseMerkleProof,
	}

	return transaction, proof, nil
}

// createMerkleProof creates a proof of transaction inclusion in the block by
// concatenating 32-byte-long hash values. The values are converted to the
// little endian form. The branch of a Merkle tree leading to a transaction
// needs to be provided. The transaction inclusion proof in hexadecimal form is
// returned.
func createMerkleProof(txMerkleBranch *TransactionMerkleProof) (
	[]byte,
	error,
) {
	var proof bytes.Buffer

	for _, node := range txMerkleBranch.MerkleNodes {
		hashBytes, err := hex.DecodeString(node)
		if err != nil {
			return nil, err
		}
		reversedHash := byteutils.Reverse(hashBytes)
		proof.Write(reversedHash)
	}
	return proof.Bytes(), nil
}

// getHeadersChain gets a chain of Bitcoin block headers that starts at the
// provided block height and has the specified chain length.
func getHeadersChain(
	btcChain Chain,
	blockHeight uint,
	chainLength uint,
) ([]byte, error) {
	// TODO: Consider exposing a function in the Bitcoin chain for returning
	//       multiple block headers with one call.
	var headersChain bytes.Buffer

	for i := blockHeight; i < blockHeight+chainLength; i++ {
		blockHeader, err := btcChain.GetBlockHeader(i)
		if err != nil {
			return nil, err
		}
		serializedBlockHeader := blockHeader.Serialize()
		headersChain.Write(serializedBlockHeader[:])
	}

	return headersChain.Bytes(), nil
}
