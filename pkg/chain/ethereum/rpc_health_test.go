package ethereum

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
)

func TestLatestBlockNumberContactsRPCOnEveryCall(t *testing.T) {
	var unavailable atomic.Bool
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if unavailable.Load() {
			http.Error(w, "Ethereum unavailable", http.StatusServiceUnavailable)
			return
		}
		var request struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("invalid JSON-RPC request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if request.Method != "eth_getBlockByNumber" || len(request.Params) != 2 ||
			string(request.Params[0]) != `"latest"` || string(request.Params[1]) != "false" {
			t.Errorf("expected latest-header RPC, got %+v", request)
			http.Error(w, "unexpected RPC", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result": &types.Header{
				Number:     big.NewInt(100),
				Difficulty: big.NewInt(1),
				Extra:      []byte{},
			},
		}); err != nil {
			t.Errorf("cannot write JSON-RPC response: %v", err)
		}
	}))
	defer server.Close()
	client, err := ethclient.Dial(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Deliberately omit the cached block counter. TbtcChain, as passed by both
	// startup paths, must use its existing RPC client for every health probe.
	chain := &TbtcChain{baseChain: &baseChain{client: client}}
	for i := 0; i < 2; i++ {
		height, err := chain.LatestBlockNumber(context.Background())
		if err != nil || height != 100 {
			t.Fatalf("expected live height 100, got %d, %v", height, err)
		}
	}
	unavailable.Store(true)
	if _, err := chain.LatestBlockNumber(context.Background()); err == nil {
		t.Fatal("RPC outage was hidden by the previous nonzero height")
	}
	unavailable.Store(false)
	if height, err := chain.LatestBlockNumber(context.Background()); err != nil || height != 100 {
		t.Fatalf("expected recovery at the same height, got %d, %v", height, err)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("expected four RPC requests, got %d", got)
	}
}

type latestHeaderClient struct {
	ethutil.EthereumClient
	getHeader func(context.Context, *big.Int) (*types.Header, error)
}

func (c *latestHeaderClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return c.getHeader(ctx, number)
}

func TestLatestBlockNumberRejectsInvalidHeaders(t *testing.T) {
	for name, header := range map[string]*types.Header{
		"missing header":  nil,
		"missing number":  {},
		"negative number": {Number: big.NewInt(-1)},
		"overflow":        {Number: new(big.Int).Lsh(big.NewInt(1), 64)},
	} {
		t.Run(name, func(t *testing.T) {
			chain := &baseChain{client: &latestHeaderClient{
				getHeader: func(context.Context, *big.Int) (*types.Header, error) {
					return header, nil
				},
			}}
			if _, err := chain.LatestBlockNumber(context.Background()); err == nil {
				t.Fatal("invalid header must fail the health probe")
			}
		})
	}
}

func TestLatestBlockNumberForwardsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chain := &baseChain{client: &latestHeaderClient{
		getHeader: func(received context.Context, number *big.Int) (*types.Header, error) {
			if received != ctx || number != nil {
				t.Fatal("expected caller context and latest-header query")
			}
			return nil, received.Err()
		},
	}}
	if _, err := chain.LatestBlockNumber(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected wrapped context cancellation, got %v", err)
	}
}
