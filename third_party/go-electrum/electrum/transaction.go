package electrum

import "context"

// BroadcastTransaction sends a raw transaction to the remote server to
// be broadcasted on the server network.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-broadcast
func (s *Client) BroadcastTransaction(ctx context.Context, rawTx string) (string, error) {
	resp := &basicResp{}
	err := s.request(ctx, "blockchain.transaction.broadcast", []interface{}{rawTx}, &resp)
	if err != nil {
		return "", err
	}

	return resp.Result, nil
}

// GetTransactionResp represents the response to GetTransaction().
type GetTransactionResp struct {
	Result *GetTransactionResult `json:"result"`
}

// GetTransactionResult represents the content of the result field in the response to GetTransaction().
type GetTransactionResult struct {
	Blockhash     string               `json:"blockhash"`
	Blocktime     uint64               `json:"blocktime"`
	Confirmations int32                `json:"confirmations"`
	Hash          string               `json:"hash"`
	Hex           string               `json:"hex"`
	Locktime      uint32               `json:"locktime"`
	Size          uint32               `json:"size"`
	Time          uint64               `json:"time"`
	Version       uint32               `json:"version"`
	Vin           []Vin                `json:"vin"`
	Vout          []Vout               `json:"vout"`
	Merkle        GetMerkleProofResult `json:"merkle,omitempty"` // For protocol v1.5 and up.
}

// Vin represents the input side of a transaction.
type Vin struct {
	Coinbase  string     `json:"coinbase"`
	ScriptSig *ScriptSig `json:"scriptsig"`
	Sequence  uint32     `json:"sequence"`
	TxID      string     `json:"txid"`
	Vout      uint32     `json:"vout"`
}

// ScriptSig represents the signature script for that transaction input.
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// Vout represents the output side of a transaction.
type Vout struct {
	N            uint32       `json:"n"`
	ScriptPubkey ScriptPubkey `json:"scriptpubkey"`
	Value        float64      `json:"value"`
}

// ScriptPubkey represents the script of that transaction output.
type ScriptPubkey struct {
	Addresses []string `json:"addresses,omitempty"`
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex,omitempty"`
	ReqSigs   uint32   `json:"reqsigs,omitempty"`
	Type      string   `json:"type"`
}

// GetTransaction gets the detailed information for a transaction.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get
func (s *Client) GetTransaction(ctx context.Context, txHash string) (*GetTransactionResult, error) {
	var resp GetTransactionResp

	err := s.request(ctx, "blockchain.transaction.get", []interface{}{txHash, true}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// GetRawTransaction gets a raw encoded transaction.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get
func (s *Client) GetRawTransaction(ctx context.Context, txHash string) (string, error) {
	var resp basicResp

	err := s.request(ctx, "blockchain.transaction.get", []interface{}{txHash, false}, &resp)
	if err != nil {
		return "", err
	}

	return resp.Result, nil
}

// GetMerkleProofResp represents the response to GetMerkleProof().
type GetMerkleProofResp struct {
	Result *GetMerkleProofResult `json:"result"`
}

// GetMerkleProofResult represents the content of the result field in the response to GetMerkleProof().
type GetMerkleProofResult struct {
	Merkle   []string `json:"merkle"`
	Height   uint32   `json:"block_height"`
	Position uint32   `json:"pos"`
}

// GetMerkleProof returns the merkle proof for a confirmed transaction.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get-merkle
func (s *Client) GetMerkleProof(ctx context.Context, txHash string, height uint32) (*GetMerkleProofResult, error) {
	var resp GetMerkleProofResp

	err := s.request(ctx, "blockchain.transaction.get_merkle", []interface{}{txHash, height}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Result, err
}

// GetHashFromPosition returns the transaction hash for a specific position in a block.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-id-from-pos
func (s *Client) GetHashFromPosition(ctx context.Context, height, position uint32) (string, error) {
	var resp basicResp

	err := s.request(ctx, "blockchain.transaction.id_from_pos", []interface{}{height, position, false}, &resp)
	if err != nil {
		return "", err
	}

	return resp.Result, err
}

// GetMerkleProofFromPosResp represents the response to GetMerkleProofFromPosition().
type GetMerkleProofFromPosResp struct {
	Result *GetMerkleProofFromPosResult `json:"result"`
}

// GetMerkleProofFromPosResult represents the content of the result field in the response
// to GetMerkleProofFromPosition().
type GetMerkleProofFromPosResult struct {
	Hash   string   `json:"tx_hash"`
	Merkle []string `json:"merkle"`
}

// GetMerkleProofFromPosition returns the merkle proof for a specific position in a block.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-id-from-pos
func (s *Client) GetMerkleProofFromPosition(ctx context.Context, height, position uint32) (*GetMerkleProofFromPosResult, error) {
	var resp GetMerkleProofFromPosResp

	err := s.request(ctx, "blockchain.transaction.id_from_pos", []interface{}{height, position, true}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Result, err
}
