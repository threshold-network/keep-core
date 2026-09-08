# Vendored go-electrum

Source: https://github.com/keep-network/go-electrum/tree/6038cb594daa66c69ea0482fa849b165b115c97b

This is the same revision previously selected by the root `go.mod` replacement:
`github.com/keep-network/go-electrum v0.0.0-20240206170935-6038cb594daa`.
Its module checksum is `h1:AKTJr+STc4rP9NcN2ppP9Zft3GbYechFW8q/S8UNQrQ=`.
The original MIT license is retained in `LICENSE`.

The copy contains `electrum/*.go`, `go.mod`, `go.sum`, and `LICENSE` from that
module. Examples and README/media assets are omitted. Go files were formatted
with gofmt. RPC encoding, response types, and Bitcoin address logic are unchanged.

Local lifecycle changes:

- `network.go` adds idempotent `Client.Abort`. `Shutdown` uses the same abort.
  Closing the transport releases socket reads and writes and pending RPCs.
  Transport and handler references remain stable during shutdown; maps are
  reclaimed with the client instead of being cleared concurrently with RPCs.
  Error and response delivery can no longer strand the reader after abort.
  Response handlers are installed before sending to accept immediate replies.
- `transport.go` closes the TCP/TLS socket and unblocks reader channel sends.
- `transport_ws.go` closes Gorilla's underlying connection directly. It never
  sends a WebSocket close frame from cancellation: `Conn.Close` supports
  concurrent writes, whereas `WriteMessage` does not. Reader channel sends are
  also interrupted. Shutdown is deliberately an abort rather than a graceful
  WebSocket close handshake.
- `cancellation_test.go` exercises real Gorilla `ws` and `wss` writers with
  controlled socket backpressure, plus concurrent shutdown of a pending RPC
  and transport readers. The WebSocket cases reproduce the concurrent-write
  panic with the original transport.

Run from the repository root so the tests use the application's dependency graph:

```sh
go test -race -timeout 3m github.com/checksum0/go-electrum/electrum
```

The `electrum-lifecycle` CI job runs these tests explicitly, along with the
application's Electrum and config tests. Root `go test ./...` does not discover
tests within nested modules.

To compare the local patch with the pinned source:

```sh
go mod download github.com/keep-network/go-electrum@v0.0.0-20240206170935-6038cb594daa
upstream="$(go env GOMODCACHE)/github.com/keep-network/go-electrum@v0.0.0-20240206170935-6038cb594daa"
diff -ru "$upstream/electrum" third_party/go-electrum/electrum
```
