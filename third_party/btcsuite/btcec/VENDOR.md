# Vendored `btcec` (pre-split, btcd v0.22.3)

## What this is

A copy of the `btcec/` directory from
[`github.com/btcsuite/btcd`](https://github.com/btcsuite/btcd) at tag
**`v0.22.3`**, plus four files added for vendoring only:

- `go.mod` (declares the pre-split module path `github.com/btcsuite/btcd/btcec`)
- `go.sum` (covers the two test-only dependencies, so the package is also
  testable standalone from inside this directory)
- `LICENSE` (copied verbatim from the btcd repository root at the same tag)
- `VENDOR.md` (this file)

The only modification to the upstream files is a mechanical `gofmt` pass
(comment reflow plus the two-line `//go:build` insertions that gofmt adds
alongside legacy `// +build` constraints in `genprecomps.go` and
`gensecp256k1.go` - semantically identical, no behavior change). No code
changes whatsoever; see the verification recipe below, which reproduces
the exact expected content from upstream.

## Why it exists

btcd v0.23 extracted `btcd/btcec` into the separate module
`btcd/btcec/v2`, and later btcd versions no longer ship the pre-split
package. Two consumers in this build still import the pre-split path
`github.com/btcsuite/btcd/btcec` and cannot easily migrate:

- the pinned `threshold-network/tss-lib` fork (audited threshold-ECDSA
  crypto; its import strings cannot be changed without cutting a new
  fork release), and
- first-party key-handling code (`pkg/bitcoin/transaction_builder`,
  `pkg/chain/local_v1`, `pkg/crypto/ephemeral`, `pkg/crypto/secp256k1`,
  `pkg/internal/pbutils`, `pkg/net/local/key.go`, plus a test in
  `pkg/operator/key_test.go`), which relies on the v1 `btcec.PublicKey` /
  `btcec.PrivateKey` types being aliases of the `crypto/ecdsa` types.

Previously this was solved by pinning the *entire* btcd module to
v0.22.3 via a `replace` directive, which downgraded the Bitcoin
consensus, wire, and script packages below versions that fix several
security advisories (GO-2022-1098, GO-2024-2818, GO-2024-3189, and
their GHSA equivalents) - three advisories, six identifiers counting
GHSA aliases. Serving only `btcec` from this directory lets the main
btcd module track a current, fully patched release while the btcec
consumers keep compiling against byte-identical crypto sources.

Note: the v0.22.3-to-v0.24.2 jump also reworked btcd's script
decoding. v0.22.3 pooled 12,500 individual 512-byte buffers
(`freeListMaxScriptSize = 512`; `Borrow` falls back to
`make([]byte, size)` for larger scripts); v0.24.2 replaces that
with a single fixed-size 4 MiB slab (`scriptSlabSize = 1 << 22`)
checked out per decode, advancing an offset after each script. The
global retention ceiling therefore rises from 6.4 MB (12,500 * 512 B)
to 524 MB (125 * 4 MiB), about an 82x increase. The per-script cap
also tightens from 32 MiB (`MaxMessagePayload`) to 4,000,000
(`maxWitnessItemSize`). Neither is a security issue; both are
worth knowing for future RSS-growth triage under high decode
concurrency. The slab-architecture change is also what introduces
the readScriptBuf slice-bounds panic guarded against in
`bitcoin.MaxTransactionByteLength` and `electrum.decodeTransaction`.

## Verifying against upstream

```sh
go mod download github.com/btcsuite/btcd@v0.22.3
cp -R "$(go env GOMODCACHE)/github.com/btcsuite/btcd@v0.22.3/btcec" /tmp/btcec-upstream
chmod -R u+w /tmp/btcec-upstream
gofmt -w /tmp/btcec-upstream
diff -r /tmp/btcec-upstream third_party/btcsuite/btcec
```

The only differences reported must be the four added files listed
above. The upstream tests are included and can be run either from the
main module:

```sh
go test github.com/btcsuite/btcd/btcec
```

or standalone:

```sh
cd third_party/btcsuite/btcec && go test ./...
```

## Exit path

This directory can be deleted once the tss-lib fork (and the
first-party key-handling code) migrate to
`github.com/btcsuite/btcd/btcec/v2`.
