#!/bin/bash
# Script to check DKG metrics from all nodes
# Usage: ./scripts/check-dkg-metrics.sh [node-number]
#        If no node-number provided, checks all nodes

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Default metrics port (can be overridden per node)
DEFAULT_METRICS_PORT=9601

# Function to check metrics for a single node
check_node_metrics() {
    local node_num=$1
    local metrics_port=${2:-$DEFAULT_METRICS_PORT}
    
    echo ""
    echo "=========================================="
    echo -e "${CYAN}Node $node_num${NC} (Metrics Port: $metrics_port)"
    echo "=========================================="
    
    # Check if node is running
    local pid_file="logs/node${node_num}.pid"
    if [ ! -f "$pid_file" ]; then
        echo -e "  ${RED}✗ Node $node_num is not running (no PID file)${NC}"
        return 1
    fi
    
    local pid=$(cat "$pid_file" 2>/dev/null || echo "")
    if [ -z "$pid" ] || ! ps -p "$pid" > /dev/null 2>&1; then
        echo -e "  ${RED}✗ Node $node_num is not running (PID $pid not found)${NC}"
        return 1
    fi
    
    # Try to fetch metrics
    local metrics_url="http://localhost:${metrics_port}/metrics"
    local metrics_output=$(curl -s --max-time 5 "$metrics_url" 2>&1)
    
    if [ $? -ne 0 ] || echo "$metrics_output" | grep -qiE "connection refused|failed|timeout"; then
        echo -e "  ${YELLOW}⚠ Cannot connect to metrics endpoint at $metrics_url${NC}"
        echo "  Node may be starting up or metrics port may be different"
        return 1
    fi
    
    if [ -z "$metrics_output" ] || echo "$metrics_output" | grep -q "^$"; then
        echo -e "  ${YELLOW}⚠ Metrics endpoint returned empty response${NC}"
        return 1
    fi
    
    # Extract DKG-related metrics
    echo ""
    echo -e "  ${GREEN}DKG Metrics:${NC}"
    
    # Check for each DKG metric
    local dkg_requested=$(echo "$metrics_output" | grep -E "^performance_dkg_requested_total" | awk '{print $2}' || echo "0")
    local dkg_joined=$(echo "$metrics_output" | grep -E "^performance_dkg_joined_total" | awk '{print $2}' || echo "0")
    local dkg_failed=$(echo "$metrics_output" | grep -E "^performance_dkg_failed_total" | awk '{print $2}' || echo "0")
    local dkg_validation=$(echo "$metrics_output" | grep -E "^performance_dkg_validation_total" | awk '{print $2}' || echo "0")
    local dkg_challenges=$(echo "$metrics_output" | grep -E "^performance_dkg_challenges_submitted_total" | awk '{print $2}' || echo "0")
    local dkg_approvals=$(echo "$metrics_output" | grep -E "^performance_dkg_approvals_submitted_total" | awk '{print $2}' || echo "0")
    
    # DKG duration (histogram - show count and sum)
    local dkg_duration_count=$(echo "$metrics_output" | grep -E "^performance_dkg_duration_seconds_count" | awk '{print $2}' || echo "0")
    local dkg_duration_sum=$(echo "$metrics_output" | grep -E "^performance_dkg_duration_seconds_sum" | awk '{print $2}' || echo "0")
    
    # Calculate average duration if count > 0
    local dkg_duration_avg="N/A"
    if [ "$dkg_duration_count" != "0" ] && [ -n "$dkg_duration_count" ] && [ "$dkg_duration_count" != "" ]; then
        if command -v bc >/dev/null 2>&1; then
            dkg_duration_avg=$(echo "scale=2; $dkg_duration_sum / $dkg_duration_count" | bc 2>/dev/null || echo "N/A")
        fi
    fi
    
    # Display metrics
    printf "  %-45s %s\n" "DKG Requested:" "$dkg_requested"
    printf "  %-45s %s\n" "DKG Joined:" "$dkg_joined"
    printf "  %-45s %s\n" "DKG Failed:" "$dkg_failed"
    printf "  %-45s %s\n" "DKG Validations:" "$dkg_validation"
    printf "  %-45s %s\n" "DKG Challenges Submitted:" "$dkg_challenges"
    printf "  %-45s %s\n" "DKG Approvals Submitted:" "$dkg_approvals"
    
    if [ "$dkg_duration_count" != "0" ] && [ -n "$dkg_duration_count" ]; then
        printf "  %-45s %s\n" "DKG Duration (count):" "$dkg_duration_count"
        if [ "$dkg_duration_avg" != "N/A" ]; then
            printf "  %-45s %s seconds\n" "DKG Duration (avg):" "$dkg_duration_avg"
        fi
    fi
    
    # Show other relevant metrics
    echo ""
    echo -e "  ${GREEN}Other Performance Metrics:${NC}"
    
    local signing_ops=$(echo "$metrics_output" | grep -E "^performance_signing_operations_total" | awk '{print $2}' || echo "0")
    local signing_success=$(echo "$metrics_output" | grep -E "^performance_signing_success_total" | awk '{print $2}' || echo "0")
    local signing_failed=$(echo "$metrics_output" | grep -E "^performance_signing_failed_total" | awk '{print $2}' || echo "0")
    
    printf "  %-45s %s\n" "Signing Operations:" "$signing_ops"
    printf "  %-45s %s\n" "Signing Success:" "$signing_success"
    printf "  %-45s %s\n" "Signing Failed:" "$signing_failed"
    
    # Show network metrics
    echo ""
    echo -e "  ${GREEN}Network Metrics:${NC}"
    
    local connected_peers=$(echo "$metrics_output" | grep -E "^connected_peers_count" | awk '{print $2}' || echo "N/A")
    local connected_bootstraps=$(echo "$metrics_output" | grep -E "^connected_bootstrap_count" | awk '{print $2}' || echo "N/A")
    local eth_connectivity=$(echo "$metrics_output" | grep -E "^eth_connectivity" | awk '{print $2}' || echo "N/A")
    
    printf "  %-45s %s\n" "Connected Peers:" "$connected_peers"
    printf "  %-45s %s\n" "Connected Bootstraps:" "$connected_bootstraps"
    printf "  %-45s %s\n" "Ethereum Connectivity:" "$eth_connectivity"
    
    # Show full metrics if verbose
    if [ "${VERBOSE:-}" = "1" ]; then
        echo ""
        echo -e "  ${GREEN}All DKG-related metrics (raw):${NC}"
        echo "$metrics_output" | grep -E "performance_dkg|connected_peers|connected_bootstrap|eth_connectivity" | sed 's/^/    /'
    fi
    
    return 0
}

# Function to find metrics port from config file
get_metrics_port_from_config() {
    local config_file="$1"
    if [ -f "$config_file" ]; then
        # Try to extract Port from [clientInfo] or [ClientInfo] section
        local port=$(grep -A 5 -E "^\[clientInfo\]|^\[ClientInfo\]" "$config_file" 2>/dev/null | grep -E "^Port\s*=" | awk -F'=' '{print $2}' | tr -d ' "' || echo "")
        if [ -n "$port" ]; then
            echo "$port"
        fi
    fi
}

# Main execution
echo "=========================================="
echo "DKG Metrics Checker"
echo "=========================================="

# Check if a specific node was requested
if [ $# -ge 1 ]; then
    NODE_NUM=$1
    if ! [[ "$NODE_NUM" =~ ^[0-9]+$ ]]; then
        echo -e "${RED}Error: Invalid node number: $NODE_NUM${NC}"
        exit 1
    fi
    
    # Try to find config file and get metrics port
    CONFIG_FILE="configs/node${NODE_NUM}.toml"
    if [ ! -f "$CONFIG_FILE" ]; then
        CONFIG_FILE="config.toml"
    fi
    
    METRICS_PORT=$(get_metrics_port_from_config "$CONFIG_FILE")
    METRICS_PORT=${METRICS_PORT:-$DEFAULT_METRICS_PORT}
    
    check_node_metrics "$NODE_NUM" "$METRICS_PORT"
else
    # Check all nodes (1-10)
    ALL_SUCCESS=true
    for node_num in {1..10}; do
        # Try to find config file and get metrics port
        CONFIG_FILE="configs/node${node_num}.toml"
        if [ ! -f "$CONFIG_FILE" ]; then
            CONFIG_FILE="config.toml"
        fi
        
        METRICS_PORT=$(get_metrics_port_from_config "$CONFIG_FILE")
        METRICS_PORT=${METRICS_PORT:-$DEFAULT_METRICS_PORT}
        
        if ! check_node_metrics "$node_num" "$METRICS_PORT"; then
            ALL_SUCCESS=false
        fi
    done
    
    echo ""
    echo "=========================================="
    if [ "$ALL_SUCCESS" = true ]; then
        echo -e "${GREEN}✓ All nodes checked${NC}"
    else
        echo -e "${YELLOW}⚠ Some nodes had errors or are not running${NC}"
    fi
    echo "=========================================="
fi

echo ""
echo "Tip: Use VERBOSE=1 to see raw metrics output:"
echo "  VERBOSE=1 $0 [node-number]"
