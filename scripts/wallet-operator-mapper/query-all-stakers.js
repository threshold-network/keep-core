#!/usr/bin/env node

/**
 * Query ALL stakers from TokenStaking contract to find the COMPLETE list
 * This will identify any operators NOT in our threshold_stakers_may_2025.json
 */

const { ethers } = require('ethers');
require('dotenv').config();

const TOKEN_STAKING_ADDRESS = '0x01B67b1194C75264d06F808A921228a95C765dd7';
const WALLET_REGISTRY_ADDRESS = '0x46d52E41C2F300BC82217Ce22b920c34995204eb';

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.ETHEREUM_RPC_URL);
  
  console.log('Querying all stakers from TokenStaking...\n');
  
  // Query StakeAuthorized events for WalletRegistry
  const tokenStakingAbi = [
    'event AuthorizationIncreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)'
  ];
  
  const tokenStaking = new ethers.Contract(
    TOKEN_STAKING_ADDRESS,
    tokenStakingAbi,
    provider
  );
  
  // Query from TokenStaking deployment (block ~14500000, March 2022)
  const startBlock = 14500000;
  const currentBlock = await provider.getBlockNumber();
  
  console.log(`Querying AuthorizationIncreased events for WalletRegistry...`);
  console.log(`Block range: ${startBlock} to ${currentBlock}\n`);
  
  const allStakingProviders = new Set();
  const chunkSize = 100000;
  
  for (let fromBlock = startBlock; fromBlock < currentBlock; fromBlock += chunkSize) {
    const toBlock = Math.min(fromBlock + chunkSize - 1, currentBlock);
    
    try {
      const filter = tokenStaking.filters.AuthorizationIncreased(null, WALLET_REGISTRY_ADDRESS);
      const events = await tokenStaking.queryFilter(filter, fromBlock, toBlock);
      
      events.forEach(e => {
        allStakingProviders.add(e.args.stakingProvider.toLowerCase());
      });
      
      if (events.length > 0) {
        console.log(`Blocks ${fromBlock}-${toBlock}: found ${events.length} events`);
      }
    } catch (err) {
      console.log(`Error in blocks ${fromBlock}-${toBlock}: ${err.message}`);
    }
  }
  
  console.log(`\nTotal unique staking providers: ${allStakingProviders.size}\n`);
  
  // Load our known list
  const stakersJson = require('/Users/leonardosaturnino/Documents/GitHub/memory-bank/20250809-beta-staker-consolidation/knowledge/threshold_stakers_may_2025.json');
  const knownProviders = new Set(stakersJson.stakers.map(s => s.staking_provider.toLowerCase()));
  
  // Find providers NOT in our list
  const unknownProviders = Array.from(allStakingProviders).filter(p => !knownProviders.has(p));
  
  console.log('='.repeat(80));
  console.log('COMPARISON');
  console.log('='.repeat(80));
  console.log(`Providers in our JSON: ${knownProviders.size}`);
  console.log(`Providers on-chain: ${allStakingProviders.size}`);
  console.log(`Unknown providers (on-chain but not in JSON): ${unknownProviders.length}`);
  
  if (unknownProviders.length > 0) {
    console.log('\n--- UNKNOWN PROVIDERS ---');
    
    // Query each unknown provider's current authorization
    const walletRegistryAbi = [
      'function stakingProviderToOperator(address) view returns (address)'
    ];
    const tokenStakingView = new ethers.Contract(
      TOKEN_STAKING_ADDRESS,
      ['function stakes(address) view returns (uint96, uint96, uint96)', 'function authorizedStake(address, address) view returns (uint96)'],
      provider
    );
    const walletRegistry = new ethers.Contract(WALLET_REGISTRY_ADDRESS, walletRegistryAbi, provider);
    
    for (const provider of unknownProviders) {
      try {
        const operator = await walletRegistry.stakingProviderToOperator(provider);
        const authorization = await tokenStakingView.authorizedStake(provider, WALLET_REGISTRY_ADDRESS);
        const stakes = await tokenStakingView.stakes(provider);
        
        console.log(`\nStaking Provider: ${provider}`);
        console.log(`  Operator: ${operator}`);
        console.log(`  Authorization: ${ethers.formatUnits(authorization, 18)} T`);
        console.log(`  Total Stake: ${ethers.formatUnits(stakes[0] + stakes[1] + stakes[2], 18)} T`);
      } catch (err) {
        console.log(`\nStaking Provider: ${provider}`);
        console.log(`  Error: ${err.message}`);
      }
    }
  }
  
  // Also output all providers for reference
  console.log('\n\n--- ALL STAKING PROVIDERS ON-CHAIN ---');
  Array.from(allStakingProviders).sort().forEach(p => {
    const known = knownProviders.has(p) ? '✓' : '?';
    console.log(`${known} ${p}`);
  });
}

main().catch(console.error);
