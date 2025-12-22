#!/bin/bash
# Script to check if a node joined DKG ceremony
# Usage: ./scripts/check-node-dkg-joined.sh [node-number]
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

# Function to check if a node joined DKG
check_node_dkg_joined() {
    local node_num=$1
    
    echo ""
    echo "=========================================="
    echo -e "${CYAN}Node $node_num - DKG Participation Check${NC}"
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
    
    echo -e "  ${GREEN}✓ Node is running${NC}"
    
    # Get config file
    local config_file="configs/node${node_num}.toml"
    if [ ! -f "$config_file" ]; then
        config_file="config.toml"
    fi
    
    # Get operator address from config
    local operator=$(grep -E "^KeyFile\s*=" "$config_file" 2>/dev/null | head -1 | awk -F'=' '{print $2}' | tr -d ' "' || echo "")
    if [ -z "$operator" ]; then
        echo -e "  ${YELLOW}⚠ Could not extract operator address from config${NC}"
    else
        # Extract address from keyfile path or get it from keyfile
        if [[ "$operator" == *"UTC--"* ]]; then
            # Extract address from UTC keyfile name
            operator=$(basename "$operator" | grep -oE "0x[0-9a-fA-F]{40}" || echo "")
        fi
        
        if [ -n "$operator" ]; then
            echo "  Operator Address: $operator"
        fi
    fi
    
    # Check 1: Logs - Look for DKG joining messages
    echo ""
    echo -e "  ${BLUE}1. Checking logs for DKG participation...${NC}"
    
    # Initialize variables to avoid unbound variable errors
    local joining_count="0"
    local eligible_count="0"
    local checking_count="0"
    local dkg_states=""
    local dkg_result=""
    
    local log_file="logs/node${node_num}.log"
    if [ ! -f "$log_file" ]; then
        echo -e "    ${YELLOW}⚠ Log file not found: $log_file${NC}"
    else
        # Check for "joining DKG" messages
        joining_count=$(grep -i "joining DKG" "$log_file" 2>/dev/null | wc -l | tr -d ' ' || echo "0")
        eligible_count=$(grep -i "not eligible for DKG" "$log_file" 2>/dev/null | wc -l | tr -d ' ' || echo "0")
        checking_count=$(grep -i "checking eligibility for DKG" "$log_file" 2>/dev/null | wc -l | tr -d ' ' || echo "0")
        
        # Check for DKG state transitions (indicates active participation)
        dkg_states=$(grep -iE "member:.*state.*dkg|tssRound.*State|DKG.*state" "$log_file" 2>/dev/null | tail -5 || echo "")
        
        # Check for DKG completion/result messages
        dkg_result=$(grep -iE "DKG.*result|group.*registered|wallet.*registered" "$log_file" 2>/dev/null | tail -3 || echo "")
        
        if [ "$joining_count" != "0" ] && [ -n "$joining_count" ]; then
            echo -e "    ${GREEN}✓ Found $joining_count 'joining DKG' message(s)${NC}"
            # Show recent joining messages
            echo "    Recent joining messages:"
            grep -i "joining DKG" "$log_file" 2>/dev/null | tail -3 | sed 's/^/      /' || true
        else
            echo -e "    ${YELLOW}⚠ No 'joining DKG' messages found${NC}"
        fi
        
        if [ "$checking_count" != "0" ] && [ -n "$checking_count" ]; then
            echo "    Eligibility checks: $checking_count"
        fi
        
        if [ "$eligible_count" != "0" ] && [ -n "$eligible_count" ]; then
            echo -e "    ${YELLOW}⚠ Found $eligible_count 'not eligible' message(s)${NC}"
        fi
        
        if [ -n "$dkg_states" ] && [ "$dkg_states" != "" ]; then
            echo -e "    ${GREEN}✓ Found DKG state transitions (active participation)${NC}"
            echo "    Recent DKG states:"
            echo "$dkg_states" | sed 's/^/      /' || true
        fi
        
        if [ -n "$dkg_result" ] && [ "$dkg_result" != "" ]; then
            echo -e "    ${GREEN}✓ Found DKG result/completion messages${NC}"
            echo "    Recent results:"
            echo "$dkg_result" | sed 's/^/      /' || true
        fi
    fi
    
    # Check 2: Metrics - Check DKG joined metric
    echo ""
    echo -e "  ${BLUE}2. Checking metrics for DKG participation...${NC}"
    
    # Initialize variables to avoid unbound variable errors
    local dkg_joined="0"
    local dkg_requested="0"
    local dkg_failed="0"
    
    local metrics_port=9601
    local metrics_url="http://localhost:${metrics_port}/metrics"
    local metrics_output=$(curl -s --max-time 5 "$metrics_url" 2>&1)
    
    if [ $? -eq 0 ] && ! echo "$metrics_output" | grep -qiE "connection refused|failed|timeout"; then
        dkg_joined=$(echo "$metrics_output" | grep -E "^performance_dkg_joined_total" | awk '{print $2}' || echo "0")
        dkg_requested=$(echo "$metrics_output" | grep -E "^performance_dkg_requested_total" | awk '{print $2}' || echo "0")
        dkg_failed=$(echo "$metrics_output" | grep -E "^performance_dkg_failed_total" | awk '{print $2}' || echo "0")
        
        if [ "$dkg_joined" != "0" ] && [ -n "$dkg_joined" ] && [ "$dkg_joined" != "" ]; then
            echo -e "    ${GREEN}✓ DKG Joined: $dkg_joined${NC}"
        else
            echo -e "    ${YELLOW}⚠ DKG Joined: 0 (no joins recorded)${NC}"
        fi
        
        if [ "$dkg_requested" != "0" ] && [ -n "$dkg_requested" ]; then
            echo "    DKG Requested: $dkg_requested"
        fi
        
        if [ "$dkg_failed" != "0" ] && [ -n "$dkg_failed" ]; then
            echo -e "    ${RED}⚠ DKG Failed: $dkg_failed${NC}"
        fi
    else
        echo -e "    ${YELLOW}⚠ Could not fetch metrics from $metrics_url${NC}"
        echo "    (Metrics may be on a different port or node may be starting)"
    fi
    
    # Check 3: Contract state - Check if operator is in a wallet group
    echo ""
    echo -e "  ${BLUE}3. Checking contract state...${NC}"
    
    if [ -n "$operator" ] && [[ "$operator" =~ ^0x[0-9a-fA-F]{40}$ ]]; then
        # Try to check if operator is registered and in sortition pool
        local config_path="$config_file"
        if [ ! -f "$config_path" ]; then
            config_path="config.toml"
        fi
        
        # Check operator registration
        local reg_output=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry operator-to-staking-provider \
            "$operator" --config "$config_path" --developer 2>&1 || echo "")
        
        if echo "$reg_output" | grep -qE "0x[0-9a-fA-F]{40}"; then
            local staking_provider=$(echo "$reg_output" | grep -oE "0x[0-9a-fA-F]{40}" | tail -1)
            echo -e "    ${GREEN}✓ Operator is registered${NC}"
            echo "    Staking Provider: $staking_provider"
            
            # Check if in sortition pool
            local pool_output=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-operator-in-pool \
                "$operator" --config "$config_path" --developer 2>&1 || echo "")
            
            if echo "$pool_output" | grep -qiE "true|yes|operator.*in.*pool"; then
                echo -e "    ${GREEN}✓ Operator is in sortition pool${NC}"
            elif echo "$pool_output" | grep -qiE "false|no|not.*in.*pool"; then
                echo -e "    ${RED}✗ Operator is NOT in sortition pool${NC}"
            else
                echo -e "    ${YELLOW}⚠ Could not determine pool membership${NC}"
            fi
        else
            echo -e "    ${RED}✗ Operator is not registered${NC}"
        fi
    else
        echo -e "    ${YELLOW}⚠ Cannot check contract state (operator address not available)${NC}"
    fi
    
    # Summary
    echo ""
    echo -e "  ${BLUE}Summary:${NC}"
    
    local joined_indicator=false
    if [ "${joining_count:-0}" != "0" ] && [ -n "${joining_count:-}" ]; then
        joined_indicator=true
    fi
    if [ "${dkg_joined:-0}" != "0" ] && [ -n "${dkg_joined:-}" ] && [ "${dkg_joined:-}" != "" ]; then
        joined_indicator=true
    fi
    if [ -n "${dkg_states:-}" ] && [ "${dkg_states:-}" != "" ]; then
        joined_indicator=true
    fi
    
    if [ "$joined_indicator" = true ]; then
        echo -e "    ${GREEN}✓ Node appears to have joined/participated in DKG${NC}"
    else
        echo -e "    ${YELLOW}⚠ No clear evidence of DKG participation found${NC}"
        echo "    Check logs for eligibility issues or sortition pool membership"
    fi
    
    return 0
}

# Main execution
if [ $# -ge 1 ]; then
    NODE_NUM=$1
    if ! [[ "$NODE_NUM" =~ ^[0-9]+$ ]]; then
        echo -e "${RED}Error: Invalid node number: $NODE_NUM${NC}"
        exit 1
    fi
    check_node_dkg_joined "$NODE_NUM"
else
    echo "=========================================="
    echo "DKG Participation Checker"
    echo "=========================================="
    echo ""
    echo "Checking all nodes (1-10)..."
    echo ""
    
    for node_num in {1..10}; do
        check_node_dkg_joined "$node_num"
    done
    
    echo ""
    echo "=========================================="
    echo -e "${GREEN}Check complete${NC}"
    echo "=========================================="
    echo ""
    echo "Tip: Check specific node with:"
    echo "  $0 <node-number>"
fi

