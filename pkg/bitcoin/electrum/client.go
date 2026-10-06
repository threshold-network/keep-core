package electrum

import (
	"context"

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
	ServerFeatures(context.Context) (*electrum.ServerFeaturesResult, error)
	Ping(context.Context) error
	Abort()
	Shutdown()
	IsShutdown() bool
}

// watchClientCancellation registers an Abort callback that fires when ctx is
// done, independently of the request mutex and keepalive loop; it also
// covers clients still being verified. The returned function unregisters
// that callback and, if ctx had not yet fired it, asynchronously calls
// Abort itself so the caller never blocks on transport cleanup. It does not
// retire the client from the Connection (clear c.client/c.closeClient) -
// callers that are done with a client must do that themselves. Abort must
// be idempotent and safe during active writes.
func watchClientCancellation(ctx context.Context, client electrumClient) func() {
	stop := context.AfterFunc(ctx, client.Abort)
	return func() {
		if stop() {
			go client.Abort()
		}
	}
}
