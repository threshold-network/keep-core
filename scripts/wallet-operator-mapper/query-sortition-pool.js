#!/usr/bin/env node

/**
 * Query Sortition Pool for registered operators
 * This is the canonical on-chain source of truth for tBTC operators
 */

const { ethers } = require('ethers');
require('dotenv').config();

// Contract addresses
const WALLET_REGISTRY_ADDRESS = '0x46d52E41C2F300BC82217Ce22b920c34995204eb';
const SORTITION_POOL_ADDRESS = '0xc2731fb2823af3Efc2694c9bC86F444d5c5bb4Dc';

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.ETHEREUM_RPC_URL);
  
  console.log('Querying Sortition Pool for registered operators...\n');
  
  // Sortition Pool ABI - need to find operators
  const sortitionPoolAbi = [
    'function operatorsInPool() view returns (uint256)',
    'function totalWeight() view returns (uint256)',
    'function getIDOperator(uint32 id) view returns (address)',
    'function getOperatorID(address operator) view returns (uint32)',
    'function isOperatorInPool(address operator) view returns (bool)',
    'function isOperatorUpToDate(address operator) view returns (bool)',
    'function getPoolWeight(address operator) view returns (uint256)'
  ];
  
  const sortitionPool = new ethers.Contract(
    SORTITION_POOL_ADDRESS,
    sortitionPoolAbi,
    provider
  );
  
  // Get pool stats
  const operatorCount = await sortitionPool.operatorsInPool();
  const totalWeight = await sortitionPool.totalWeight();
  
  console.log(`Operators in pool: ${operatorCount}`);
  console.log(`Total weight: ${totalWeight}\n`);
  
  // Query each operator from the stakers list to check if in pool
  const stakersFile = require('./operators.json');
  const allOperators = [...stakersFile.operators.keep, ...stakersFile.operators.disable];
  
  console.log('Checking operators from operators.json against sortition pool:\n');
  console.log('Address | In Pool | Up To Date | Pool Weight | Status in JSON');
  console.log('-'.repeat(100));
  
  const inPoolOperators = [];
  const notInPoolOperators = [];
  
  for (const op of allOperators) {
    const addr = op.address;
    try {
      const inPool = await sortitionPool.isOperatorInPool(addr);
      const upToDate = inPool ? await sortitionPool.isOperatorUpToDate(addr) : false;
      const poolWeight = inPool ? await sortitionPool.getPoolWeight(addr) : 0n;
      
      const status = op.status;
      const shortAddr = addr.slice(0, 10) + '...' + addr.slice(-6);
      
      console.log(`${shortAddr} | ${inPool ? 'YES' : 'NO '} | ${upToDate ? 'YES' : 'NO '} | ${poolWeight.toString().padStart(10)} | ${status} (${op.identification})`);
      
      if (inPool) {
        inPoolOperators.push({ ...op, poolWeight: poolWeight.toString() });
      } else {
        notInPoolOperators.push(op);
      }
    } catch (err) {
      console.log(`${addr.slice(0, 10)}... | ERROR: ${err.message}`);
    }
  }
  
  console.log('\n' + '='.repeat(100));
  console.log('SUMMARY');
  console.log('='.repeat(100));
  console.log(`\nOperators IN pool: ${inPoolOperators.length}`);
  console.log(`Operators NOT in pool: ${notInPoolOperators.length}`);
  
  console.log('\nOperators NOT in pool:');
  notInPoolOperators.forEach(op => {
    console.log(`  - ${op.identification} (${op.address.slice(0, 10)}...) - ${op.status}`);
  });
  
  // Output JSON for further analysis
  const output = {
    timestamp: new Date().toISOString(),
    poolStats: {
      operatorCount: operatorCount.toString(),
      totalWeight: totalWeight.toString()
    },
    operatorsInPool: inPoolOperators,
    operatorsNotInPool: notInPoolOperators
  };
  
  require('fs').writeFileSync('sortition-pool-operators.json', JSON.stringify(output, null, 2));
  console.log('\nSaved detailed results to sortition-pool-operators.json');
}

main().catch(console.error);
