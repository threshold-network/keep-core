#!/bin/bash
# Script to check deployment status of all contracts

echo "=== Contract Deployment Status ==="
echo ""

# Check Threshold Network contracts
echo "📦 Threshold Network Contracts:"
if [ -d "tmp/solidity-contracts/deployments/development" ]; then
  for contract in TokenStaking T NuCypherToken VendingMachineNuCypher; do
    if [ -f "tmp/solidity-contracts/deployments/development/${contract}.json" ]; then
      address=$(jq -r '.address' "tmp/solidity-contracts/deployments/development/${contract}.json" 2>/dev/null)
      echo "  ✓ ${contract}: ${address}"
    else
      echo "  ✗ ${contract}: NOT DEPLOYED"
    fi
  done
else
  echo "  ✗ Threshold contracts directory not found"
fi

echo ""

# Check Random Beacon contracts
echo "📦 Random Beacon Contracts:"
if [ -d "solidity/random-beacon/deployments/development" ]; then
  for contract in RandomBeacon BeaconSortitionPool ReimbursementPool RandomBeaconGovernance; do
    if [ -f "solidity/random-beacon/deployments/development/${contract}.json" ]; then
      address=$(jq -r '.address' "solidity/random-beacon/deployments/development/${contract}.json" 2>/dev/null)
      echo "  ✓ ${contract}: ${address}"
    else
      echo "  ✗ ${contract}: NOT DEPLOYED"
    fi
  done
else
  echo "  ✗ Random Beacon contracts directory not found"
fi

echo ""

# Check ECDSA contracts
echo "📦 ECDSA Contracts:"
if [ -d "solidity/ecdsa/deployments/development" ]; then
  for contract in WalletRegistry EcdsaSortitionPool EcdsaDkgValidator EcdsaInactivity; do
    if [ -f "solidity/ecdsa/deployments/development/${contract}.json" ]; then
      address=$(jq -r '.address' "solidity/ecdsa/deployments/development/${contract}.json" 2>/dev/null)
      echo "  ✓ ${contract}: ${address}"
    else
      echo "  ✗ ${contract}: NOT DEPLOYED"
    fi
  done
else
  echo "  ✗ ECDSA contracts directory not found"
fi

echo ""

# Check TBTC contracts
echo "📦 TBTC Contracts:"
TBTC_PATH="tmp/tbtc-v2/solidity"
if [ -d "${TBTC_PATH}/deployments/development" ]; then
  for contract in Bridge MaintainerProxy LightRelay LightRelayMaintainerProxy WalletProposalValidator; do
    if [ -f "${TBTC_PATH}/deployments/development/${contract}.json" ]; then
      address=$(jq -r '.address' "${TBTC_PATH}/deployments/development/${contract}.json" 2>/dev/null)
      echo "  ✓ ${contract}: ${address}"
    else
      echo "  ✗ ${contract}: NOT DEPLOYED"
    fi
  done
else
  echo "  ✗ TBTC contracts directory not found"
fi

echo ""
echo "=== Summary ==="
echo "To deploy missing contracts, run:"
echo "  export GETH_DATA_DIR=~/ethereum/data"
echo "  export KEEP_ETHEREUM_PASSWORD=password"
echo "  ./scripts/install.sh --network development"

