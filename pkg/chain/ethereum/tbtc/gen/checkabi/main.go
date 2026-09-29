// Command checkabi extracts the ABI embedded in the committed generated
// Go bindings (the <Name>MetaData = &bind.MetaData{ABI: "..."} string in
// abi/<Name>.go) as canonical JSON, for the whole-ABI and
// reservation-methods comparisons that the Makefile's
// verify-vendored-fallback target runs.
//
// Usage: checkabi <ContractName> [methodName ...]
//
// It prints a compact JSON array of ABI entries: the full ABI when no
// method names are given, otherwise the entries of the given names, in
// the given order. The embedded string is a Go interpreted string
// literal holding JSON text; decoding it with encoding/json is exactly
// what the binding's own MetaData does under the hood, so no escaping
// assumptions are made about the literal.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
)

// Embedded ABIs keyed by contract name, matching the selectors
// verify-vendored-fallback runs in the Makefile.
var metaDatas = map[string]string{
	"Bridge":                  abi.BridgeMetaData.ABI,
	"ReservationRouter":       abi.ReservationRouterMetaData.ABI,
	"ReservationVault":        abi.ReservationVaultMetaData.ABI,
	"WalletProposalValidator": abi.WalletProposalValidatorMetaData.ABI,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: checkabi <ContractName> [methodName ...]")
		os.Exit(2)
	}
	name := os.Args[1]
	abiString, ok := metaDatas[name]
	if !ok {
		fmt.Fprintf(
			os.Stderr,
			"checkabi: unknown contract %q (known: Bridge, ReservationRouter, ReservationVault, WalletProposalValidator)",
			name,
		)
		os.Exit(2)
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(abiString), &items); err != nil {
		// A malformed embedded ABI means the binding file is corrupt;
		// failing here loudly is the point of the gate.
		fmt.Fprintf(
			os.Stderr,
			"checkabi: cannot parse the embedded ABI of %s: %v\n", name, err,
		)
		os.Exit(1)
	}

	// abigen's metadata packer strips the space after the struct/enum/
	// contract internalType prefix (e.g. "struct BitcoinTx.Info" ->
	// "structBitcoinTx.Info"); the compiled artifacts and the vendored
	// fallback JSON keep the space. Restore it so this side of the
	// comparison is in the compiled-artifact shape.
	restoreInternalTypeSpaces(items)

	entries := items
	methods := os.Args[2:]
	if len(methods) > 0 {
		entries = make([]map[string]any, 0, len(methods))
		for _, m := range methods {
			var found *map[string]any
			for i := range items {
				if items[i]["name"] == m {
					found = &items[i]
					break
				}
			}
			if found == nil {
				fmt.Fprintf(
					os.Stderr,
					"checkabi: method %q not found in the embedded ABI of %s\n",
					m, name,
				)
				os.Exit(1)
			}
			entries = append(entries, *found)
		}
	}

	b, err := json.Marshal(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkabi: cannot serialize: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

// restoreInternalTypeSpaces re-adds the space after the struct/enum/
// contract internalType prefix, recursively over every entry and
// parameter/return description, matching the compiled-artifact shape.
func restoreInternalTypeSpaces(items []map[string]any) {
	for _, item := range items {
		if it, ok := item["internalType"].(string); ok {
			item["internalType"] = internalTypeWithSpace(it)
		}
		restoreParams(item["inputs"])
		restoreParams(item["outputs"])
	}
}

func restoreParams(v any) {
	params, ok := v.([]any)
	if !ok {
		return
	}
	for _, p := range params {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if it, ok := pm["internalType"].(string); ok {
			pm["internalType"] = internalTypeWithSpace(it)
		}
		restoreParams(pm["components"])
	}
}

// internalTypeWithSpace re-inserts the space between the struct/enum/
// contract prefix and the qualified type name that abigen's packer
// strips. Non-prefixed internalType values pass through unchanged, so
// an entry that already has the space is a no-op.
func internalTypeWithSpace(v string) string {
	for _, prefix := range []string{"struct", "enum", "contract"} {
		if strings.HasPrefix(v, prefix) {
			return prefix + " " + strings.TrimPrefix(v, prefix)
		}
	}
	return v
}
