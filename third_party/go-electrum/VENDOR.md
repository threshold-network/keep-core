# Vendored go-electrum

Source: https://github.com/keep-network/go-electrum/tree/6038cb594daa66c69ea0482fa849b165b115c97b

This is the same revision previously selected by the root `go.mod` replacement:
`github.com/keep-network/go-electrum v0.0.0-20240206170935-6038cb594daa`.
Its module checksum is `h1:AKTJr+STc4rP9NcN2ppP9Zft3GbYechFW8q/S8UNQrQ=`.
The original MIT license is retained in `LICENSE`.

The copy is based on that module's `electrum/*.go`, `go.mod`, `go.sum`, and
`LICENSE`. Examples and README/media assets are omitted. Go files were
formatted with gofmt. Four files materially differ from upstream —
`network.go`, `transport.go`, `transport_ws.go`, and `scripthash.go` — and are
documented below. `cancellation_test.go` and `network_test.go` are local
additions with no upstream counterpart. RPC encoding, response types, and
Bitcoin address logic are otherwise unchanged.

Local patches:

- `network.go` adds idempotent `Client.Abort`. `Shutdown` uses the same abort.
  Closing the transport releases socket reads and writes and pending RPCs.
  Transport and handler references remain stable during shutdown; maps are
  reclaimed with the client instead of being cleared concurrently with RPCs.
  Error and response delivery can no longer strand the reader after abort.
  Response handlers are installed before sending to accept immediate replies.
  `apiErr.UnmarshalJSON` now guards the `code` field with a checked type
  assertion instead of an unchecked cast, so a server that sends a
  non-numeric `code` (e.g. electrs/esplora) cannot panic the read loop.
  `Client.listen` recovers from a panic raised while processing a single
  message and keeps the loop running instead of letting the panic escape the
  read-loop goroutine.
- `transport.go` closes the TCP/TLS socket and unblocks reader channel sends.
  It also bounds a single line read to 1 MiB (`readBoundedLine`), closing the
  transport with an error instead of buffering without limit when a server
  withholds the trailing newline.
- `transport_ws.go` closes Gorilla's underlying connection directly. It never
  sends a WebSocket close frame from cancellation: `Conn.Close` supports
  concurrent writes, whereas `WriteMessage` does not. Reader channel sends are
  also interrupted. Shutdown is deliberately an abort rather than a graceful
  WebSocket close handshake. It also calls `Conn.SetReadLimit(1 << 20)` after
  dialing to bound per-message allocation.
- `scripthash.go` changes `ListUnspentResult.Height` from `uint32` to
  `int32`, matching `GetMempoolResult.Height` and the Electrum protocol's use
  of `-1` for a mempool entry.
- `cancellation_test.go` exercises real Gorilla `ws` and `wss` writers with
  controlled socket backpressure, a TCPTransport parity case, plus concurrent
  shutdown of a pending RPC and transport readers. The WebSocket and TCP
  cases reproduce the concurrent-write panic and un-abortable write with the
  original transport.
- `network_test.go` regression-tests that unmarshaling an `apiErr` with a
  non-numeric `code` field does not panic.

Run from the repository root so the tests use the application's dependency graph:

```sh
go test -race -timeout 3m github.com/checksum0/go-electrum/electrum
```

The `electrum-lifecycle` CI job runs these tests explicitly, along with the
application's Electrum and config tests. Root `go test ./...` does not discover
tests within nested modules.

To compare the local patch with the pinned source, mirroring the
verification recipe used for `third_party/btcsuite/btcec`:

```sh
go mod download github.com/keep-network/go-electrum@v0.0.0-20240206170935-6038cb594daa
upstream="$(go env GOMODCACHE)/github.com/keep-network/go-electrum@v0.0.0-20240206170935-6038cb594daa"
cp -R "$upstream/electrum" /tmp/go-electrum-upstream
chmod -R u+w /tmp/go-electrum-upstream
gofmt -w /tmp/go-electrum-upstream
diff -r /tmp/go-electrum-upstream third_party/go-electrum/electrum
```

The only differences reported must be content diffs in the four patched
files (`network.go`, `transport.go`, `transport_ws.go`, `scripthash.go`) and
two files present only on our side (`cancellation_test.go`,
`network_test.go`). `go.mod`, `go.sum`, `LICENSE`, and this file live one
directory up, in `third_party/go-electrum/`, alongside the `electrum/`
package copy, so they are outside this `electrum/`-scoped diff; `go.mod`'s
module path intentionally differs from upstream
(`github.com/checksum0/go-electrum` here vs.
`github.com/keep-network/go-electrum` upstream) to match the root module's
`replace` target, and `VENDOR.md` has no upstream counterpart by definition.

## Exit path

The lifecycle, hardening, and `Height`-typing patches in `network.go`,
`transport.go`, `transport_ws.go`, and `scripthash.go` should be upstreamed
to `keep-network/go-electrum` (the previous `replace` target, pinned at
`v0.0.0-20240206170935-6038cb594daa`). Once that fork carries the patch, this
directory can be removed and the root `go.mod` restored to a `replace`
pointing at the fork. If the patch is instead adopted directly in
`checksum0/go-electrum` (the module path this copy currently declares), this
vendored copy can be archived in favor of a normal module dependency on that
fork.
