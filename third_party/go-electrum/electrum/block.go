package electrum

import (
	"context"
	"errors"
)

var (
	// ErrCheckpointHeight is thrown if the checkpoint height is smaller than the block height.
	ErrCheckpointHeight = errors.New("checkpoint height must be greater than or equal to block height")
)

// GetBlockHeaderResp represents the response to GetBlockHeader().
type GetBlockHeaderResp struct {
	Result *GetBlockHeaderResult `json:"result"`
}

// GetBlockHeaderResult represents the content of the result field in the response to GetBlockHeader().
type GetBlockHeaderResult struct {
	Branch []string `json:"branch"`
	Header string   `json:"header"`
	Root   string   `json:"root"`
}

// GetBlockHeader returns the block header at a specific height.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-block-header
func (s *Client) GetBlockHeader(ctx context.Context, height uint32, checkpointHeight ...uint32) (*GetBlockHeaderResult, error) {
	if checkpointHeight != nil && checkpointHeight[0] != 0 {
		if height > checkpointHeight[0] {
			return nil, ErrCheckpointHeight
		}

		var resp GetBlockHeaderResp
		err := s.request(ctx, "blockchain.block.header", []interface{}{height, checkpointHeight[0]}, &resp)

		return resp.Result, err
	}

	var resp basicResp
	err := s.request(ctx, "blockchain.block.header", []interface{}{height, 0}, &resp)
	if err != nil {
		return nil, err
	}

	result := &GetBlockHeaderResult{
		Branch: nil,
		Header: resp.Result,
		Root:   "",
	}

	return result, err
}

// GetBlockHeadersResp represents the response to GetBlockHeaders().
type GetBlockHeadersResp struct {
	Result *GetBlockHeadersResult `json:"result"`
}

// GetBlockHeadersResult represents the content of the result field in the response to GetBlockHeaders().
type GetBlockHeadersResult struct {
	Count   uint32   `json:"count"`
	Headers string   `json:"hex"`
	Max     uint32   `json:"max"`
	Branch  []string `json:"branch,omitempty"`
	Root    string   `json:"root,omitempty"`
}

// GetBlockHeaders return a concatenated chunk of block headers.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-block-headers
func (s *Client) GetBlockHeaders(ctx context.Context, startHeight, count uint32,
	checkpointHeight ...uint32) (*GetBlockHeadersResult, error) {

	var resp GetBlockHeadersResp
	var err error

	if checkpointHeight != nil && checkpointHeight[0] != 0 {
		if (startHeight + (count - 1)) > checkpointHeight[0] {
			return nil, ErrCheckpointHeight
		}

		err = s.request(ctx, "blockchain.block.headers", []interface{}{startHeight, count, checkpointHeight[0]}, &resp)
	} else {
		err = s.request(ctx, "blockchain.block.headers", []interface{}{startHeight, count, 0}, &resp)
	}

	if err != nil {
		return nil, err
	}

	return resp.Result, err
}
