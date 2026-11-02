#!/usr/bin/env bash
#
# Checks that the vendored WalletProposalValidator.json, the real tbtc-v2
# validator that pkg/chain/ethereum/tbtc_validator_harness_test.go
# deploys, matches a fresh compile of the tbtc-v2 commit it was vendored
# from (see the provenance note in regenerate.sh):
#
#   - abi:              exactly equal,
#   - deployedBytecode: equal once the trailing CBOR metadata is removed,
#   - bytecode:         (the creation code the harness deploys) equal once
#                       that same metadata blob is removed.
#
# The metadata is excluded because it hashes the full compiler input, and
# the CI job's tbtc-v2 `yarn install` is not lockfile-frozen, so the hash
# can move while the executed code stays the same.
#
# Usage:
#   ./verify.sh /path/to/compiled/WalletProposalValidator.json
#
# Run by the reservation-router-vendored-fallback-verify job in
# .github/workflows/client.yml against its compile of TBTC_V2_REF.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ $# -ne 1 || ! -f "$1" ]]; then
    echo "usage: $0 /path/to/compiled/WalletProposalValidator.json" >&2
    exit 2
fi

python3 - "$HERE/WalletProposalValidator.json" "$1" <<'PY'
import json
import sys

vendored_path, compiled_path = sys.argv[1], sys.argv[2]
vendored = json.load(open(vendored_path))
compiled = json.load(open(compiled_path))

REMEDY = (
    "the vendored harness validator ({}) does not match the compiled "
    "tbtc-v2 artifact ({}). Re-run regenerate.sh against a build of "
    "TBTC_V2_REF and update its provenance note, or pin TBTC_V2_REF to "
    "the commit the vendored file came from."
).format(vendored_path, compiled_path)


def fail(message):
    print("error: " + message, file=sys.stderr)
    print("error: " + REMEDY, file=sys.stderr)
    sys.exit(1)


def code_of(artifact, field, label):
    value = artifact.get(field)
    if not isinstance(value, str) or not value.startswith("0x") or len(value) < 6:
        fail("{} {} is missing or not 0x-prefixed hex".format(label, field))
    return bytes.fromhex(value[2:])


def split_metadata(code, label):
    # solc appends CBOR-encoded metadata to the runtime code, followed by
    # its length as a big-endian uint16. The encoding is a CBOR map, so
    # its first byte is 0xa0-0xb7; anything else means the length read
    # is wrong and stripping would compare garbage.
    length = int.from_bytes(code[-2:], "big")
    if length + 2 > len(code) or not 0xA0 <= code[-(length + 2)] <= 0xB7:
        fail("{} deployedBytecode has no recognizable trailing CBOR metadata".format(label))
    return code[: -(length + 2)], code[-(length + 2) :]


def first_difference(a, b):
    for i, (x, y) in enumerate(zip(a, b)):
        if x != y:
            return i
    return min(len(a), len(b))


def compare_code(field, vendored_code, compiled_code):
    if vendored_code != compiled_code:
        fail(
            "{} differs (metadata excluded): vendored {} bytes, compiled {} "
            "bytes, first difference at byte {}".format(
                field,
                len(vendored_code),
                len(compiled_code),
                first_difference(vendored_code, compiled_code),
            )
        )


if vendored.get("abi") != compiled.get("abi"):
    fail("abi differs")

vendored_runtime, vendored_meta = split_metadata(
    code_of(vendored, "deployedBytecode", "vendored"), "vendored"
)
compiled_runtime, compiled_meta = split_metadata(
    code_of(compiled, "deployedBytecode", "compiled"), "compiled"
)
compare_code("deployedBytecode", vendored_runtime, compiled_runtime)


def strip_embedded(code, meta, label):
    # The creation code carries the runtime code, metadata included.
    if meta not in code:
        fail("{} bytecode does not embed its deployedBytecode metadata".format(label))
    return code.replace(meta, b"")


compare_code(
    "bytecode",
    strip_embedded(code_of(vendored, "bytecode", "vendored"), vendored_meta, "vendored"),
    strip_embedded(code_of(compiled, "bytecode", "compiled"), compiled_meta, "compiled"),
)

print(
    "WalletProposalValidator.json harness artifact matches the compiled "
    "artifact (abi exact; bytecode and deployedBytecode without metadata)"
)
PY
