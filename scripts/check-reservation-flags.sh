#!/usr/bin/env bash
#
# scripts/check-reservation-flags.sh - deploy-time check that the paired
# reservation flags are consistent across the start and maintainer config files.
#
# Compares:
#   - tbtc.reservationsEnabled                 in the start config
#   - maintainer.spv.reservationProofsEnabled  in the maintainer config
#
# A mismatch (start enabled, maintainer disabled) means reservation anchors
# would be proposed with no SPV proof submission, stranding them. The
# inverse (start disabled, maintainer enabled) is a valid split deployment
# where the start process does not propose reservations and the maintainer
# process has no anchors to settle; it is not flagged.
#
# Usage:
#   check-reservation-flags.sh --start-config <path> --maintainer-config <path>
#
# Exit codes:
#   0 - flags are consistent (or both disabled)
#   1 - flags are inconsistent: start enabled, maintainer disabled
#   2 - usage error
#   3 - config file not found or unreadable

set -euo pipefail

# Path to the start process config file.
start_config=""
# Path to the maintainer process config file.
maintainer_config=""

usage() {
    cat <<'EOF'
Usage: check-reservation-flags.sh --start-config <path> --maintainer-config <path>

Compares the reservation proposal flags in the start and maintainer config files.
Both flags must be enabled together for end-to-end reservation operation.

Options:
  --start-config <path>       Path to the start process config file.
  --maintainer-config <path>  Path to the maintainer process config file.
  --help                      Show this help message and exit.

Exit codes:
  0  flags are consistent
  1  flags are inconsistent (start enabled, maintainer disabled)
  2  usage error
  3  config file not found
EOF
}

# Read a TOML boolean for a dotted key path (for example
# "maintainer.spv.reservationProofsEnabled") from a config file. Prints the
# value (true/false) on stdout, or "missing" if the key is not present.
#
# The key may be written either under its table header
# ("[maintainer.spv]" followed by "reservationProofsEnabled = true") or as a
# dotted key ("maintainer.spv.reservationProofsEnabled = true"). Matching is
# case-insensitive, like viper's key lookup. Values may be quoted. Only plain
# "key = value" lines are considered; trailing comments are ignored.
read_toml_bool() {
    local file="$1"
    local key="$2"

    if [ ! -f "$file" ]; then
        echo "missing"
        return
    fi

    awk -v want="$(echo "$key" | tr '[:upper:]' '[:lower:]')" '
        function trim(s) {
            gsub(/^[[:space:]]+/, "", s)
            gsub(/[[:space:]]+$/, "", s)
            return s
        }
        BEGIN { table = ""; found = "missing" }
        {
            line = $0
            sub(/#.*$/, "", line)
            line = trim(line)
            if (line == "") next
            if (line ~ /^\[\[.*\]\]$/) { table = "(array)"; next }
            if (line ~ /^\[.*\]$/) {
                table = substr(line, 2, length(line) - 2)
                gsub(/[[:space:]"]/, "", table)
                table = tolower(table)
                next
            }
            eq = index(line, "=")
            if (eq == 0) next
            name = substr(line, 1, eq - 1)
            value = substr(line, eq + 1)
            gsub(/[[:space:]"]/, "", name)
            gsub(/[[:space:]"]/, "", value)
            full = (table == "") ? name : table "." name
            if (tolower(full) == want) {
                value = tolower(value)
                if (value == "true" || value == "false") {
                    found = value
                    exit
                }
            }
        }
        END { print found }
    ' "$file"
}

while [ $# -gt 0 ]; do
    case "$1" in
        --start-config)
            start_config="$2"
            shift 2
            ;;
        --maintainer-config)
            maintainer_config="$2"
            shift 2
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            echo "error: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [ -z "$start_config" ] || [ -z "$maintainer_config" ]; then
    echo "error: --start-config and --maintainer-config are required" >&2
    usage >&2
    exit 2
fi

if [ ! -f "$start_config" ]; then
    echo "error: start config file not found: $start_config" >&2
    exit 3
fi
if [ ! -f "$maintainer_config" ]; then
    echo "error: maintainer config file not found: $maintainer_config" >&2
    exit 3
fi

# Read the two flags.
start_enabled=$(read_toml_bool "$start_config" "tbtc.reservationsEnabled")
maintainer_enabled=$(read_toml_bool "$maintainer_config" "maintainer.spv.reservationProofsEnabled")

echo "tbtc.reservationsEnabled in start config:                    $start_enabled"
echo "maintainer.spv.reservationProofsEnabled in maintainer config: $maintainer_enabled"

# A missing flag defaults to false (viper's zero value for booleans).
# An explicit "false" is the same as missing.
if [ "$start_enabled" = "missing" ]; then
    start_enabled="false"
fi
if [ "$maintainer_enabled" = "missing" ]; then
    maintainer_enabled="false"
fi

# Check the consistency. The only problematic case is start=true, maintainer=false.
# The inverse (start=false, maintainer=true) is a valid split deployment
# where the start process does not propose reservations and the maintainer
# process has no anchors to settle.
if [ "$start_enabled" = "true" ] && [ "$maintainer_enabled" = "false" ]; then
    echo "" >&2
    echo "ERROR: inconsistent reservation flags" >&2
    echo "  tbtc.reservationsEnabled is enabled in $start_config" >&2
    echo "  but maintainer.spv.reservationProofsEnabled is disabled in $maintainer_config" >&2
    echo "  reservation anchors would be proposed with no SPV proof submission" >&2
    echo "  and would be stranded." >&2
    echo "  Enable both flags for end-to-end reservation operation." >&2
    exit 1
fi

echo ""
echo "OK: reservation flags are consistent"
exit 0
