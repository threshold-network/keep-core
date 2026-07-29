package electrum

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	electrumclient "github.com/checksum0/go-electrum/electrum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/internal/byteutils"
)

var _ bitcoin.CanonicalTransactionStatusSource = (*Connection)(nil)

type canonicalTransactionStatusReader interface {
	canonicalRawTransaction(string) (string, bool, error)
	canonicalScriptHistory([]byte) ([]*electrumclient.GetMempoolResult, error)
	GetLatestBlockHeight() (uint, error)
	GetBlockHeader(uint) (*bitcoin.BlockHeader, error)
	GetTransactionMerkleProof(
		bitcoin.Hash,
		uint,
	) (*bitcoin.TransactionMerkleProof, error)
}

// GetCanonicalTransactionStatus returns the transaction's current observation
// from Electrum's canonical transaction and script indexes. Confirmed results
// are bound to a block only after their Merkle branch matches that block's
// header. A successful transaction-not-found response is distinct from an RPC
// or index inconsistency and is the only case that returns Found=false.
func (c *Connection) GetCanonicalTransactionStatus(
	transactionHash bitcoin.Hash,
) (*bitcoin.CanonicalTransactionStatus, error) {
	return canonicalTransactionStatus(c, transactionHash)
}

func canonicalTransactionStatus(
	reader canonicalTransactionStatusReader,
	transactionHash bitcoin.Hash,
) (*bitcoin.CanonicalTransactionStatus, error) {
	if reader == nil {
		return nil, fmt.Errorf("canonical Electrum transaction reader is nil")
	}
	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)
	rawTransaction, found, err := reader.canonicalRawTransaction(txID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get canonical raw transaction with ID [%s]: [%w]",
			txID,
			err,
		)
	}
	if !found {
		return &bitcoin.CanonicalTransactionStatus{Found: false}, nil
	}
	transaction, err := convertRawTransaction(rawTransaction)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode canonical raw transaction with ID [%s]: [%w]",
			txID,
			err,
		)
	}
	if actualHash := transaction.Hash(); actualHash != transactionHash {
		return nil, fmt.Errorf(
			"canonical raw transaction hash mismatch: expected [%s], got [%s]",
			txID,
			actualHash.Hex(bitcoin.ReversedByteOrder),
		)
	}

	blockHeight, err := canonicalTransactionBlockHeight(
		reader,
		txID,
		transaction,
	)
	if err != nil {
		return nil, err
	}
	if blockHeight == 0 {
		return &bitcoin.CanonicalTransactionStatus{Found: true}, nil
	}

	header, err := reader.GetBlockHeader(blockHeight)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get canonical block header at height [%d]: [%w]",
			blockHeight,
			err,
		)
	}
	if header == nil {
		return nil, fmt.Errorf(
			"canonical block header at height [%d] is nil",
			blockHeight,
		)
	}
	proof, err := reader.GetTransactionMerkleProof(
		transactionHash,
		blockHeight,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get canonical transaction Merkle proof: [%w]",
			err,
		)
	}
	if err := verifyCanonicalTransactionMerkleProof(
		transactionHash,
		blockHeight,
		proof,
		header.MerkleRootHash,
	); err != nil {
		return nil, err
	}
	latestBlockHeight, err := reader.GetLatestBlockHeight()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get canonical Bitcoin tip: [%w]",
			err,
		)
	}
	if latestBlockHeight < blockHeight {
		return nil, fmt.Errorf(
			"canonical transaction height [%d] exceeds tip [%d]",
			blockHeight,
			latestBlockHeight,
		)
	}

	// Bind the proof and tip observations to one canonical chain snapshot.
	// A reorganization can replace the block after the proof is verified but
	// before the tip is read. Re-reading the header after observing the tip
	// detects that race instead of reporting the old branch as canonical.
	revalidatedHeader, err := reader.GetBlockHeader(blockHeight)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to revalidate canonical block header at height [%d]: [%w]",
			blockHeight,
			err,
		)
	}
	if revalidatedHeader == nil {
		return nil, fmt.Errorf(
			"revalidated canonical block header at height [%d] is nil",
			blockHeight,
		)
	}
	serializedHeader := header.Serialize()
	revalidatedSerializedHeader := revalidatedHeader.Serialize()
	if serializedHeader != revalidatedSerializedHeader {
		return nil, fmt.Errorf(
			"canonical block header at height [%d] changed while "+
				"transaction status was read",
			blockHeight,
		)
	}

	return &bitcoin.CanonicalTransactionStatus{
		Found:         true,
		Confirmations: latestBlockHeight - blockHeight + 1,
		BlockHeight:   blockHeight,
		BlockHash:     bitcoin.ComputeHash(revalidatedSerializedHeader[:]),
	}, nil
}

func (c *Connection) canonicalRawTransaction(
	txID string,
) (string, bool, error) {
	type result struct {
		raw   string
		found bool
	}
	observation, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrumclient.Client,
		) (result, error) {
			rawTransaction, err := client.GetRawTransaction(ctx, txID)
			if err != nil {
				if isTxNotFoundErr(err) {
					return result{}, nil
				}
				return result{}, err
			}
			if rawTransaction == "" {
				return result{}, fmt.Errorf(
					"Electrum returned an empty raw transaction",
				)
			}
			return result{raw: rawTransaction, found: true}, nil
		},
		"GetCanonicalRawTransaction",
	)
	if err != nil {
		return "", false, err
	}
	return observation.raw, observation.found, nil
}

func (c *Connection) canonicalScriptHistory(
	script []byte,
) ([]*electrumclient.GetMempoolResult, error) {
	scriptHash := sha256.Sum256(script)
	reversedScriptHash := byteutils.Reverse(scriptHash[:])
	return requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrumclient.Client,
		) ([]*electrumclient.GetMempoolResult, error) {
			return client.GetHistory(
				ctx,
				hex.EncodeToString(reversedScriptHash),
			)
		},
		"GetCanonicalScriptHistory",
	)
}

func canonicalTransactionBlockHeight(
	reader canonicalTransactionStatusReader,
	txID string,
	transaction *bitcoin.Transaction,
) (uint, error) {
	if transaction == nil || len(transaction.Outputs) == 0 {
		return 0, fmt.Errorf(
			"canonical transaction [%s] has no indexed outputs",
			txID,
		)
	}
	seenScripts := make(map[string]bool)
	matched := false
	var matchedHeight uint
	for _, output := range transaction.Outputs {
		if output == nil {
			return 0, fmt.Errorf(
				"canonical transaction [%s] has a nil output",
				txID,
			)
		}
		scriptKey := string(output.PublicKeyScript)
		if seenScripts[scriptKey] {
			continue
		}
		seenScripts[scriptKey] = true
		history, err := reader.canonicalScriptHistory(
			output.PublicKeyScript,
		)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to get canonical script history for transaction [%s]: [%w]",
				txID,
				err,
			)
		}
		for _, item := range history {
			if item == nil || item.Hash != txID {
				continue
			}
			height := uint(0)
			if item.Height > 0 {
				height = uint(item.Height)
			}
			if matched && matchedHeight != height {
				return 0, fmt.Errorf(
					"canonical transaction [%s] has inconsistent block heights [%d/%d]",
					txID,
					matchedHeight,
					height,
				)
			}
			matched = true
			matchedHeight = height
		}
	}
	if matched {
		// Every unique output index agreed on the candidate height. Confirmed
		// candidates are independently checked against the block header and
		// Merkle proof by canonicalTransactionStatus.
		return matchedHeight, nil
	}
	return 0, fmt.Errorf(
		"canonical transaction [%s] is missing from all output-script histories",
		txID,
	)
}

func verifyCanonicalTransactionMerkleProof(
	transactionHash bitcoin.Hash,
	blockHeight uint,
	proof *bitcoin.TransactionMerkleProof,
	expectedRoot bitcoin.Hash,
) error {
	if proof == nil || proof.BlockHeight != blockHeight {
		return fmt.Errorf(
			"canonical transaction Merkle proof has an unexpected block height",
		)
	}
	current := transactionHash
	position := proof.Position
	for _, node := range proof.MerkleNodes {
		sibling, err := bitcoin.NewHashFromString(
			node,
			bitcoin.ReversedByteOrder,
		)
		if err != nil {
			return fmt.Errorf(
				"canonical transaction Merkle proof contains an invalid node: [%w]",
				err,
			)
		}
		var pair [2 * bitcoin.HashByteLength]byte
		if position&1 == 0 {
			copy(pair[:bitcoin.HashByteLength], current[:])
			copy(pair[bitcoin.HashByteLength:], sibling[:])
		} else {
			copy(pair[:bitcoin.HashByteLength], sibling[:])
			copy(pair[bitcoin.HashByteLength:], current[:])
		}
		current = bitcoin.ComputeHash(pair[:])
		position >>= 1
	}
	if position != 0 || current != expectedRoot {
		return fmt.Errorf(
			"canonical transaction Merkle proof does not match the block header",
		)
	}
	return nil
}
