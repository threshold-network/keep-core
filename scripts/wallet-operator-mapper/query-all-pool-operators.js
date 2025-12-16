#!/usr/bin/env node

/**
 * Query ALL operators from sortition pool by iterating member IDs
 */

const { ethers } = require('ethers');
require('dotenv').config();

const SORTITION_POOL_ADDRESS = '0xc2731fb2823af3Efc2694c9bC86F444d5c5bb4Dc';

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.ETHEREUM_RPC_URL);
  
  const sortitionPoolAbi = [
    'function operatorsInPool() view returns (uint256)',
    'function totalWeight() view returns (uint256)',
    'function getIDOperator(uint32 id) view returns (address)'
  ];
  
  const sortitionPool = new ethers.Contract(
    SORTITION_POOL_ADDRESS,
    sortitionPoolAbi,
    provider
  );
  
  const operatorCount = await sortitionPool.operatorsInPool();
  const totalWeight = await sortitionPool.totalWeight();
  
  console.log(`Operators in pool: ${operatorCount}`);
  console.log(`Total weight: ${totalWeight}\n`);
  
  // Try to find operator IDs by querying getIDOperator
  // IDs typically start from 1 and increment
  console.log('Querying operators by ID (1 to 100)...\n');
  
  const foundOperators = new Set();
  
  for (let id = 1; id <= 100; id++) {
    try {
      const operator = await sortitionPool.getIDOperator(id);
      if (operator !== ethers.ZeroAddress) {
        foundOperators.add(operator.toLowerCase());
      }
    } catch (err) {
      // ID doesn't exist
    }
  }
  
  console.log(`Found ${foundOperators.size} unique operators:\n`);
  
  // Load our operators.json and compare
  const stakersFile = require('./operators.json');
  const keepOperators = stakersFile.operators.keep.map(o => o.address.toLowerCase());
  const disableOperators = stakersFile.operators.disable.map(o => o.address.toLowerCase());
  const allKnownOperators = new Set([...keepOperators, ...disableOperators]);
  
  const operatorList = Array.from(foundOperators).sort();
  
  operatorList.forEach(addr => {
    const inKeep = keepOperators.includes(addr);
    const inDisable = disableOperators.includes(addr);
    const known = inKeep || inDisable;
    
    let status = 'UNKNOWN';
    let name = 'UNKNOWN';
    
    if (inKeep) {
      const op = stakersFile.operators.keep.find(o => o.address.toLowerCase() === addr);
      status = 'KEEP';
      name = op.identification;
    } else if (inDisable) {
      const op = stakersFile.operators.disable.find(o => o.address.toLowerCase() === addr);
      status = 'DISABLE';
      name = op.identification;
    }
    
    console.log(`${addr} - ${status.padEnd(8)} - ${name}`);
  });
  
  // Find operators in pool but NOT in our list
  const unknownInPool = operatorList.filter(addr => !allKnownOperators.has(addr));
  
  // Find operators in our list but NOT in pool
  const knownNotInPool = Array.from(allKnownOperators).filter(addr => !foundOperators.has(addr));
  
  console.log('\n' + '='.repeat(80));
  console.log('DISCREPANCIES');
  console.log('='.repeat(80));
  
  if (unknownInPool.length > 0) {
    console.log(`\nOperators IN POOL but NOT in operators.json (${unknownInPool.length}):`);
    unknownInPool.forEach(addr => console.log(`  ${addr}`));
  } else {
    console.log('\nNo unknown operators in pool.');
  }
  
  if (knownNotInPool.length > 0) {
    console.log(`\nOperators in operators.json but NOT IN POOL (${knownNotInPool.length}):`);
    knownNotInPool.forEach(addr => {
      const inKeep = keepOperators.includes(addr);
      const op = inKeep 
        ? stakersFile.operators.keep.find(o => o.address.toLowerCase() === addr)
        : stakersFile.operators.disable.find(o => o.address.toLowerCase() === addr);
      console.log(`  ${addr} - ${op?.identification || 'UNKNOWN'} (${op?.status || 'UNKNOWN'})`);
    });
  } else {
    console.log('\nAll known operators are in the pool.');
  }
  
  // Save results
  const output = {
    timestamp: new Date().toISOString(),
    poolStats: {
      operatorCount: operatorCount.toString(),
      totalWeight: totalWeight.toString()
    },
    operatorsInPool: operatorList,
    unknownInPool,
    knownNotInPool
  };
  
  require('fs').writeFileSync('pool-operators-complete.json', JSON.stringify(output, null, 2));
  console.log('\nSaved to pool-operators-complete.json');
}

main().catch(console.error);
