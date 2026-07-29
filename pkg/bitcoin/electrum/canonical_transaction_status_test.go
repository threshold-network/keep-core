package electrum

import (
	"encoding/hex"
	"testing"

	electrumclient "github.com/checksum0/go-electrum/electrum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

type canonicalTransactionStatusTestReader struct {
	rawTransaction string
	found          bool
	rawError       error
	histories      map[string][]*electrumclient.GetMempoolResult
	historyError   error
	latestHeight   uint
	header         *bitcoin.BlockHeader
	headerError    error
	proof          *bitcoin.TransactionMerkleProof
	proofError     error
}

func (reader *canonicalTransactionStatusTestReader) canonicalRawTransaction(
	string,
) (string, bool, error) {
	return reader.rawTransaction, reader.found, reader.rawError
}

func (reader *canonicalTransactionStatusTestReader) canonicalScriptHistory(
	script []byte,
) ([]*electrumclient.GetMempoolResult, error) {
	return reader.histories[hex.EncodeToString(script)], reader.historyError
}

func (reader *canonicalTransactionStatusTestReader) GetLatestBlockHeight() (
	uint,
	error,
) {
	return reader.latestHeight, nil
}

func (reader *canonicalTransactionStatusTestReader) GetBlockHeader(
	uint,
) (*bitcoin.BlockHeader, error) {
	return reader.header, reader.headerError
}

func (reader *canonicalTransactionStatusTestReader) GetTransactionMerkleProof(
	bitcoin.Hash,
	uint,
) (*bitcoin.TransactionMerkleProof, error) {
	return reader.proof, reader.proofError
}

func TestConnectionImplementsCanonicalTransactionStatusSource(t *testing.T) {
	var backend interface{} = &Connection{}
	if _, ok := backend.(bitcoin.CanonicalTransactionStatusSource); !ok {
		t.Fatal("Electrum connection does not implement canonical transaction status")
	}
}

func TestCanonicalTransactionStatus(t *testing.T) {
	transaction := testCanonicalStatusTransaction()
	transactionHash := transaction.Hash()
	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)
	rawTransaction := hex.EncodeToString(transaction.Serialize())
	scriptKey := hex.EncodeToString(transaction.Outputs[0].PublicKeyScript)

	t.Run("not found", func(t *testing.T) {
		status, err := canonicalTransactionStatus(
			&canonicalTransactionStatusTestReader{},
			transactionHash,
		)
		if err != nil {
			t.Fatal(err)
		}
		if status == nil || status.Found {
			t.Fatalf("unexpected absent transaction status: [%+v]", status)
		}
	})

	t.Run("mempool", func(t *testing.T) {
		reader := &canonicalTransactionStatusTestReader{
			rawTransaction: rawTransaction,
			found:          true,
			histories: map[string][]*electrumclient.GetMempoolResult{
				scriptKey: {{Hash: txID, Height: 0}},
			},
		}
		status, err := canonicalTransactionStatus(reader, transactionHash)
		if err != nil {
			t.Fatal(err)
		}
		if status == nil || !status.Found || status.Confirmations != 0 ||
			status.BlockHeight != 0 || status.BlockHash != (bitcoin.Hash{}) {
			t.Fatalf("unexpected mempool transaction status: [%+v]", status)
		}
	})

	t.Run("confirmed", func(t *testing.T) {
		const blockHeight = 100
		header := &bitcoin.BlockHeader{
			Version:        1,
			MerkleRootHash: transactionHash,
			Time:           1234,
			Bits:           0x1d00ffff,
			Nonce:          10,
		}
		reader := &canonicalTransactionStatusTestReader{
			rawTransaction: rawTransaction,
			found:          true,
			histories: map[string][]*electrumclient.GetMempoolResult{
				scriptKey: {{Hash: txID, Height: blockHeight}},
			},
			latestHeight: 120,
			header:       header,
			proof: &bitcoin.TransactionMerkleProof{
				BlockHeight: blockHeight,
				Position:    0,
			},
		}
		status, err := canonicalTransactionStatus(reader, transactionHash)
		if err != nil {
			t.Fatal(err)
		}
		serializedHeader := header.Serialize()
		expectedBlockHash := bitcoin.ComputeHash(serializedHeader[:])
		if status == nil || !status.Found || status.Confirmations != 21 ||
			status.BlockHeight != blockHeight ||
			status.BlockHash != expectedBlockHash {
			t.Fatalf("unexpected confirmed transaction status: [%+v]", status)
		}
	})

	t.Run("invalid merkle proof", func(t *testing.T) {
		reader := &canonicalTransactionStatusTestReader{
			rawTransaction: rawTransaction,
			found:          true,
			histories: map[string][]*electrumclient.GetMempoolResult{
				scriptKey: {{Hash: txID, Height: 100}},
			},
			header: &bitcoin.BlockHeader{MerkleRootHash: bitcoin.Hash{1}},
			proof: &bitcoin.TransactionMerkleProof{
				BlockHeight: 100,
			},
		}
		if _, err := canonicalTransactionStatus(
			reader,
			transactionHash,
		); err == nil {
			t.Fatal("invalid canonical transaction Merkle proof was accepted")
		}
	})

	t.Run("incomplete index", func(t *testing.T) {
		reader := &canonicalTransactionStatusTestReader{
			rawTransaction: rawTransaction,
			found:          true,
			histories:      map[string][]*electrumclient.GetMempoolResult{},
		}
		if _, err := canonicalTransactionStatus(
			reader,
			transactionHash,
		); err == nil {
			t.Fatal("incomplete canonical transaction index reported absence")
		}
	})

	t.Run("inconsistent output indexes", func(t *testing.T) {
		transaction := testCanonicalStatusTransaction()
		transaction.Outputs = append(
			transaction.Outputs,
			&bitcoin.TransactionOutput{
				Value:           2000,
				PublicKeyScript: []byte{0x52},
			},
		)
		transactionHash := transaction.Hash()
		txID := transactionHash.Hex(bitcoin.ReversedByteOrder)
		reader := &canonicalTransactionStatusTestReader{
			rawTransaction: hex.EncodeToString(transaction.Serialize()),
			found:          true,
			histories: map[string][]*electrumclient.GetMempoolResult{
				hex.EncodeToString(
					transaction.Outputs[0].PublicKeyScript,
				): {{Hash: txID, Height: 0}},
				hex.EncodeToString(
					transaction.Outputs[1].PublicKeyScript,
				): {{Hash: txID, Height: 100}},
			},
		}
		if _, err := canonicalTransactionStatus(
			reader,
			transactionHash,
		); err == nil {
			t.Fatal("inconsistent canonical output indexes were accepted")
		}
	})
}

func TestVerifyCanonicalTransactionMerkleProof(t *testing.T) {
	transactionHash := bitcoin.Hash{1}
	sibling := bitcoin.Hash{2}
	for _, test := range []struct {
		name     string
		position uint
		left     bitcoin.Hash
		right    bitcoin.Hash
	}{
		{
			name:     "transaction on left",
			position: 0,
			left:     transactionHash,
			right:    sibling,
		},
		{
			name:     "transaction on right",
			position: 1,
			left:     sibling,
			right:    transactionHash,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var pair [2 * bitcoin.HashByteLength]byte
			copy(pair[:bitcoin.HashByteLength], test.left[:])
			copy(pair[bitcoin.HashByteLength:], test.right[:])
			expectedRoot := bitcoin.ComputeHash(pair[:])
			proof := &bitcoin.TransactionMerkleProof{
				BlockHeight: 100,
				MerkleNodes: []string{
					sibling.Hex(bitcoin.ReversedByteOrder),
				},
				Position: test.position,
			}
			if err := verifyCanonicalTransactionMerkleProof(
				transactionHash,
				100,
				proof,
				expectedRoot,
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func testCanonicalStatusTransaction() *bitcoin.Transaction {
	return &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{1},
				OutputIndex:     0,
			},
			Sequence: 0xffffffff,
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           1000,
			PublicKeyScript: []byte{0x51},
		}},
	}
}
