package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// headerHashFixtureRPC serves the original JSON response rather than marshaling
// types.Header, which would hide fields an older geth decoder does not know.
type headerHashFixtureRPC struct {
	header json.RawMessage
}

func (s *headerHashFixtureRPC) GetBlockByNumber(
	_ context.Context,
	number string,
	fullTransactions bool,
) (json.RawMessage, error) {
	if number != "0xb8652b" || fullTransactions {
		return nil, fmt.Errorf("unexpected block request: %s, %t", number, fullTransactions)
	}
	return s.header, nil
}

// TestGetBlockHashByNumber_Glamsterdam guards the hash used by wallet
// coordination against dropping post-fork header fields during JSON decoding.
func TestGetBlockHashByNumber_Glamsterdam(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sepolia-glamsterdam-header-12084523.json")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(fixture, &response); err != nil {
		t.Fatal(err)
	}

	server := rpc.NewServer()
	if err := server.RegisterName("eth", &headerHashFixtureRPC{response.Result}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Stop)
	client := ethclient.NewClient(rpc.DialInProc(server))
	t.Cleanup(client.Close)

	chain := &baseChain{client: client}
	hash, err := chain.GetBlockHashByNumber(12_084_523)
	if err != nil {
		t.Fatal(err)
	}

	// This is the canonical hash reported by the real RPC response, not one
	// derived from the locally decoded header under test.
	expected := common.HexToHash("0x1f1aade90c33979d5d0689c08d6ae81d2b075ea6e7834de1fa1f353872abd465")
	if common.Hash(hash) != expected {
		t.Fatalf("unexpected coordination block hash: got %x, want %x", hash, expected)
	}
}
