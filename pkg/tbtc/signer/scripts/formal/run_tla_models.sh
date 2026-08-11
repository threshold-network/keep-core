#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# Path normalization (allowlisted-divergence per source manifest):
# canonical signer layout places TLA+ models at
# `<ROOT_DIR>/docs/formal/models` (where ROOT_DIR = pkg/tbtc/signer/).
# Monorepo source path was `docs/frost-migration/formal-verification/models`
# relative to monorepo root. Override via MODELS_PATH env var for
# alternate environments.
MODEL_DIR="${MODELS_PATH:-$ROOT_DIR/docs/formal/models}"
TLA_TOOLS_VERSION="${TLA_TOOLS_VERSION:-v1.8.0}"
TLA_TOOLS_JAR="${TLA_TOOLS_JAR:-/tmp/tla2tools-${TLA_TOOLS_VERSION}.jar}"
TLA_TOOLS_URL="${TLA_TOOLS_URL:-https://github.com/tlaplus/tlaplus/releases/download/${TLA_TOOLS_VERSION}/tla2tools.jar}"
# Pin the SHA-256 of the upstream tla2tools.jar (github.com/tlaplus/tlaplus
# rolling release v1.8.0 asset). Upstream may delete and rebuild this asset from
# master. Re-pin only after confirming the replacement digest in GitHub's
# official release metadata; the gate below must continue to fail closed.
TLA_TOOLS_SHA256="${TLA_TOOLS_SHA256:-ab323b79802aedc3203b3f9af37c6aca3ed43f4e0225b36f2aa77b26de46c05f}"

if ! command -v java >/dev/null 2>&1; then
  echo "java is required to run TLC model checks" >&2
  exit 1
fi
if ! java -version >/dev/null 2>&1; then
  echo "java runtime is required to run TLC model checks" >&2
  exit 1
fi

if [[ ! -d "$MODEL_DIR" ]]; then
  echo "model directory not found: $MODEL_DIR" >&2
  exit 1
fi

verify_tla_tools_jar_sha256() {
  local expected_sha256="$1"
  local jar_path="$2"

  if command -v shasum >/dev/null 2>&1; then
    local actual_sha256
    actual_sha256="$(shasum -a 256 "$jar_path" | awk '{print $1}')"
    if [[ "$actual_sha256" != "$expected_sha256" ]]; then
      echo "tla2tools jar checksum mismatch: expected [$expected_sha256], got [$actual_sha256]" >&2
      return 1
    fi
    return 0
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    local actual_sha256
    actual_sha256="$(sha256sum "$jar_path" | awk '{print $1}')"
    if [[ "$actual_sha256" != "$expected_sha256" ]]; then
      echo "tla2tools jar checksum mismatch: expected [$expected_sha256], got [$actual_sha256]" >&2
      return 1
    fi
    return 0
  fi

  echo "missing checksum tool: install shasum or sha256sum" >&2
  return 1
}

if [[ ! -f "$TLA_TOOLS_JAR" ]]; then
  echo "downloading tlaplus tools jar to $TLA_TOOLS_JAR"
  curl -fsSL "$TLA_TOOLS_URL" -o "$TLA_TOOLS_JAR"
fi

verify_tla_tools_jar_sha256 "$TLA_TOOLS_SHA256" "$TLA_TOOLS_JAR"

shopt -s nullglob
cfg_files=("$MODEL_DIR"/*.cfg)
shopt -u nullglob

if [[ ${#cfg_files[@]} -eq 0 ]]; then
  echo "no model cfg files found under $MODEL_DIR" >&2
  exit 1
fi

for cfg_path in "${cfg_files[@]}"; do
  cfg_name="$(basename "$cfg_path" .cfg)"
  module_name="${cfg_name%%.*}"
  tla_path="$MODEL_DIR/${module_name}.tla"
  if [[ ! -f "$tla_path" ]]; then
    echo "missing tla module for cfg [$cfg_path]: expected [$tla_path]" >&2
    exit 1
  fi

  echo "running tlc for ${cfg_name} (${module_name})"
  (
    cd "$MODEL_DIR"
    java -cp "$TLA_TOOLS_JAR" tlc2.TLC -cleanup -config "$(basename "$cfg_path")" "${module_name}.tla"
  )
done
