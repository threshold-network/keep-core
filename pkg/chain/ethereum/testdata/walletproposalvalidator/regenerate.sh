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
#   commit:        e635e2292fbb6f2c39d835fb0b3861c032934bd0
#                  (threshold-network/tbtc-v2 branch fix/m1-cross-repo-review,
#                  on top of reservations-upgrade @ 9f8f5ef1). Includes the
#                  snapshotted-minAmount check in
#                  validateReservationAnchorProposal.
#   Re-run this script against a newer tbtc-v2 build whenever the
#   validator changes, and update this note.
#
# Usage:
#   TBTC_V2_BUILD_DIR=/path/to/tbtc-v2/solidity/build ./regenerate.sh
#
# TBTC_V2_BUILD_DIR must point at a hardhat build/ directory (i.e. the
# directory containing contracts/bridge/WalletProposalValidator.sol/
# WalletProposalValidator.json) produced by `yarn build` in the tbtc-v2
# solidity package. Defaults to the sibling tbtc-v2-m1fix worktree used
# during development of this harness.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOLC="${SOLC:-solc}"

TBTC_V2_BUILD_DIR="${TBTC_V2_BUILD_DIR:-$HERE/../../../../../../tbtc-v2-m1fix/solidity/build}"
VALIDATOR_ARTIFACT="$TBTC_V2_BUILD_DIR/contracts/bridge/WalletProposalValidator.sol/WalletProposalValidator.json"

if [[ ! -f "$VALIDATOR_ARTIFACT" ]]; then
    echo "error: validator artifact not found at $VALIDATOR_ARTIFACT" >&2
    echo "set TBTC_V2_BUILD_DIR to a tbtc-v2 solidity/build directory (run 'yarn build' there first)" >&2
    exit 1
fi

echo "compiling StubBridge.sol with $($SOLC --version | tail -1)..."
"$SOLC" --optimize --optimize-runs 200 --via-ir --combined-json abi,bin \
    "$HERE/StubBridge.sol" >"$HERE/.stub-combined.json"

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
