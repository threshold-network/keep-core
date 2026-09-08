package electrum

import (
	"context"
	"sync"

	"github.com/checksum0/go-electrum/electrum"
)

// electrumClient is the RPC and lifecycle surface used by Connection.
type electrumClient interface {
	GetRawTransaction(context.Context, string) (string, error)
	GetHistory(context.Context, string) ([]*electrum.GetMempoolResult, error)
	GetMempool(context.Context, string) ([]*electrum.GetMempoolResult, error)
	BroadcastTransaction(context.Context, string) (string, error)
	SubscribeHeadersSingle(context.Context) (*electrum.SubscribeHeadersResult, error)
	GetBlockHeader(context.Context, uint32, ...uint32) (*electrum.GetBlockHeaderResult, error)
	GetMerkleProof(context.Context, string, uint32) (*electrum.GetMerkleProofResult, error)
	GetHashFromPosition(context.Context, uint32, uint32) (string, error)
	ListUnspent(context.Context, string) ([]*electrum.ListUnspentResult, error)
	GetFee(context.Context, uint32) (float32, error)
	ServerVersion(context.Context) (string, string, error)
	Ping(context.Context) error
	Shutdown()
	IsShutdown() bool
}

// watchClientCancellation closes client when ctx is cancelled, independently of
// the request mutex and keepalive loop. It also covers clients being verified.
// The returned function unregisters the callback and retires the client without
// waiting for a WebSocket close handshake. Both paths share a single shutdown.
func watchClientCancellation(ctx context.Context, client electrumClient) func() {
	shutdown := sync.OnceFunc(func() {
		if !client.IsShutdown() {
			client.Shutdown()
		}
	})
	stop := context.AfterFunc(ctx, shutdown)
	return func() {
		stop()
		go shutdown()
	}
}
