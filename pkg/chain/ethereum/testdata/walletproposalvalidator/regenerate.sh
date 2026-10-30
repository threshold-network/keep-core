#!/usr/bin/env bash
#
# Regenerates the testdata artifacts consumed by
# pkg/chain/ethereum/tbtc_validator_harness_test.go:
#
#   - StubBridge.json:              recompiled from StubBridge.sol here.
#   - WalletProposalValidator.json: vendored (abi + bytecode +
#                                    deployedBytecode) from a tbtc-v2 hardhat
#                                    build directory. This is the *real*
#                                    tbtc-v2 contract, unmodified - the test
#                                    harness deploys and runs it in an
#                                    in-memory go-ethereum EVM
#                                    (core/vm/runtime over a shared
#                                    StateDB), never reimplementing it in Go.
#
# Provenance of the currently vendored WalletProposalValidator.json:
#   commit:        d3c5c13387d1fefca55c1b1fb8aae505b5f26292
#                  (threshold-network/tbtc-v2, reached from the
#                  reservations-upgrade branch: release to pool plus the
#                  hand-off capacity change; the validator no longer
#                  re-checks re-anchor target caps).
#                  Built with `yarn install && yarn build` in tbtc-v2's
#                  solidity/ directory and checked locally with verify.sh.
#                  The compiled validator is unchanged at tbtc-v2
#                  d1bf5bc281992885970f7c232cd9a66b7a6b5127, the commit
#                  TBTC_V2_REF in .github/workflows/client.yml names.
#   Re-run this script against a newer tbtc-v2 build whenever the
#   validator changes, bump TBTC_V2_REF to the same commit, and update
#   this note.
#
# Usage:
#   TBTC_V2_BUILD_DIR=/path/to/tbtc-v2/solidity/build ./regenerate.sh
#
# TBTC_V2_BUILD_DIR must point at a hardhat build/ directory (i.e. the
# directory containing contracts/bridge/WalletProposalValidator.sol/
# WalletProposalValidator.json) produced by `yarn build` in the tbtc-v2
# solidity package.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOLC="${SOLC:-solc}"

: "${TBTC_V2_BUILD_DIR:?set TBTC_V2_BUILD_DIR to a tbtc-v2 solidity/build directory (run yarn build there first)}"
VALIDATOR_ARTIFACT="$TBTC_V2_BUILD_DIR/contracts/bridge/WalletProposalValidator.sol/WalletProposalValidator.json"

if [[ ! -f "$VALIDATOR_ARTIFACT" ]]; then
    echo "error: validator artifact not found at $VALIDATOR_ARTIFACT" >&2
    echo "set TBTC_V2_BUILD_DIR to a tbtc-v2 solidity/build directory (run 'yarn build' there first)" >&2
    exit 1
fi

echo "compiling StubBridge.sol with $($SOLC --version | tail -1)..."
# Compile by relative path: solc embeds the source path in the bytecode
# metadata, so an absolute path would change StubBridge.json per machine.
(cd "$HERE" && "$SOLC" --optimize --optimize-runs 200 --via-ir \
    --combined-json abi,bin StubBridge.sol) >"$HERE/.stub-combined.json"

python3 - "$HERE/.stub-combined.json" "$HERE/StubBridge.json" <<'PY'
import json
import sys

combined_path, out_path = sys.argv[1], sys.argv[2]
combined = json.load(open(combined_path))
key = next(k for k in combined["contracts"] if k.endswith(":StubBridge"))
contract = combined["contracts"][key]
out = {"abi": contract["abi"], "bytecode": "0x" + contract["bin"]}
json.dump(out, open(out_path, "w"), indent=2)
PY
rm -f "$HERE/.stub-combined.json"

echo "vendoring WalletProposalValidator artifact from $VALIDATOR_ARTIFACT..."
python3 - "$VALIDATOR_ARTIFACT" "$HERE/WalletProposalValidator.json" <<'PY'
import json
import sys

src_path, out_path = sys.argv[1], sys.argv[2]
src = json.load(open(src_path))
out = {
    "contractName": src["contractName"],
    "abi": src["abi"],
    "bytecode": src["bytecode"],
    "deployedBytecode": src["deployedBytecode"],
}
json.dump(out, open(out_path, "w"), indent=2)
PY

echo "done. Update the provenance note at the top of this script if the" \
     "source tbtc-v2 commit changed."
