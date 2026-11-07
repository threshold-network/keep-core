# Sepolia Glamsterdam header fixture

`sepolia-glamsterdam-header-12084523.json` is the complete public JSON-RPC
response for Sepolia block 12,084,523, fetched on 2026-11-07 from
`https://ethereum-sepolia-rpc.publicnode.com` using `eth_getBlockByNumber` with
parameters `["latest", false]`. The returned block's timestamp is
2026-11-07 15:22:36 UTC. Querying `["0xb8652b", false]` selects this same block
explicitly for reproduction.

The canonical hash reported in the response is
`0x1f1aade90c33979d5d0689c08d6ae81d2b075ea6e7834de1fa1f353872abd465`.
The header includes `requestsHash`, `blockAccessListHash`, and `slotNumber`.

The regression test serves the saved JSON directly, then exercises
`baseChain.GetBlockHashByNumber` through geth's actual JSON decoder. Serving
a constructed `types.Header` would omit fields missing from that same type
and could hide this compatibility regression. The fixture needs no live RPC.

With geth v1.17.3, the BAL field is decoded under the old name `balHash` and
the locally computed hash is instead
`0xb0c2d2961f1c17c6ca0ee1249d088b391a6f175548585076ec2344a6a74ae63e`.
The corrected v1.17.10 decoder recognizes `blockAccessListHash` and computes
the canonical hash.

Primary sources:

- [geth v1.17.3 header schema](https://github.com/ethereum/go-ethereum/blob/v1.17.3/core/types/block.go#L103)
- [geth v1.17.10 header schema](https://github.com/ethereum/go-ethereum/blob/v1.17.10/core/types/block.go#L103)
