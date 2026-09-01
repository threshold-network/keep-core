#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# --locked: build strictly against the committed Cargo.lock so a release
# binary is never produced from an unaudited, re-resolved dependency set.
cargo build --release --locked
