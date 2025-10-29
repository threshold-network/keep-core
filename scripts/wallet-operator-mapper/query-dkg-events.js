#!/usr/bin/env node

/**
 * DKG Event Query Tool
 *
 * Queries WalletRegistry DkgResultSubmitted events to extract operator membership for each wallet.
 * This is the canonical on-chain way to get wallet-to-operator mappings.
 *
 * Usage: node query-dkg-events.js [--limit N]
 * Output: Updates wallet-operator-mapping.json with real operator data
 */

const { ethers } = require('ethers');
const fs = require('fs');
const path = require('path');
require('dotenv').config();

// Configuration
const CONFIG = {
  rpcUrl: process.env.ETHEREUM_RPC_URL || 'https://ethereum.publicnode.com',
  debug: process.env.DEBUG === 'true',
  limitWallets: parseInt(process.env.LIMIT_WALLETS) || null
};

// Parse CLI args
const limitArg = process.argv.find(arg => arg.startsWith('--limit='));
if (limitArg) {
  CONFIG.limitWallets = parseInt(limitArg.split('=')[1]);
}

// File paths
const PROOF_OF_FUNDS_PATH = process.env.PROOF_OF_FUNDS_PATH ||
  path.join(__dirname, 'data', 'tbtc-proof-of-funds.json');
const OPERATORS_PATH = path.join(__dirname, 'operators.json');
const BRIDGE_CONTRACT_PATH = path.join(__dirname, 'contracts', 'Bridge.json');
const OUTPUT_PATH = path.join(__dirname, 'wallet-operator-mapping.json');

// Contract addresses
const WALLET_REGISTRY_ADDRESS = '0x46d52E41C2F300BC82217Ce22b920c34995204eb';
const SORTITION_POOL_ADDRESS = '0xc2731fb2823af3Efc2694c9bC86F444d5c5bb4Dc'; // ECDSA sortition pool (from WalletRegistry.sortitionPool())

// Logging utilities
const log = {
  info: (msg) => console.log(`ℹ️  ${msg}`),
  success: (msg) => console.log(`✅ ${msg}`),
  error: (msg) => console.error(`❌ ${msg}`),
  debug: (msg) => CONFIG.debug && console.log(`🔍 ${msg}`),
  warn: (msg) => console.warn(`⚠️  ${msg}`)
};

// Load JSON file
function loadJSON(filePath) {
  try {
    if (!fs.existsSync(filePath)) {
      throw new Error(`File not found: ${filePath}`);
    }
    const data = fs.readFileSync(filePath, 'utf8');
    return JSON.parse(data);
  } catch (error) {
    log.error(`Failed to load ${filePath}: ${error.message}`);
    throw error;
  }
}

/**
 * Estimate block number from timestamp
 * Uses simple linear approximation: ~12 seconds per block
 */
async function estimateBlockFromTimestamp(provider, targetTimestamp) {
  const currentBlock = await provider.getBlock('latest');
  const currentTimestamp = currentBlock.timestamp;

  if (targetTimestamp >= currentTimestamp) {
    return currentBlock.number;
  }

  // Estimate: ~12 seconds per block on average
  const secondsDiff = currentTimestamp - targetTimestamp;
  const blocksDiff = Math.floor(secondsDiff / 12);

  return Math.max(0, currentBlock.number - blocksDiff);
}

/**
 * Query DKG events for a specific wallet
 * Two-step lookup: WalletCreated (walletID → dkgResultHash) → DkgResultSubmitted (dkgResultHash → members)
 */
async function getDkgEventForWallet(walletRegistry, ecdsaWalletID, createdAtTimestamp, provider) {
  try {
    // Convert timestamp to approximate block number
    // Use wallet creation time to narrow search range (±20,000 blocks ~ ±3 days)
    const currentBlock = await provider.getBlockNumber();
    const estimatedBlock = await estimateBlockFromTimestamp(provider, createdAtTimestamp);

    const fromBlock = Math.max(0, estimatedBlock - 20000);
    const toBlock = Math.min(currentBlock, estimatedBlock + 20000);

    log.debug(`Querying events for wallet ${ecdsaWalletID.slice(0, 10)}... (blocks ${fromBlock}-${toBlock})`);

    // Step 1: Query WalletCreated event to get dkgResultHash
    const walletCreatedFilter = walletRegistry.filters.WalletCreated(ecdsaWalletID);
    const walletCreatedEvents = await walletRegistry.queryFilter(walletCreatedFilter, fromBlock, toBlock);

    if (walletCreatedEvents.length === 0) {
      log.warn(`No WalletCreated event found for ${ecdsaWalletID.slice(0, 10)}...`);
      return null;
    }

    const dkgResultHash = walletCreatedEvents[0].args.dkgResultHash;
    log.debug(`  Found dkgResultHash: ${dkgResultHash.slice(0, 10)}...`);

    // Step 2: Query DkgResultSubmitted event using dkgResultHash
    // DKG event happens before WalletCreated, so search earlier blocks
    const dkgFromBlock = Math.max(0, fromBlock - 10000); // Search 10k blocks earlier
    const dkgResultFilter = walletRegistry.filters.DkgResultSubmitted(dkgResultHash);
    const dkgEvents = await walletRegistry.queryFilter(dkgResultFilter, dkgFromBlock, toBlock);

    if (dkgEvents.length === 0) {
      log.warn(`No DkgResultSubmitted event found for hash ${dkgResultHash.slice(0, 10)}...`);
      return null;
    }

    const event = dkgEvents[0];

    // Parse event data
    // event DkgResultSubmitted(
    //   uint256 indexed resultHash,
    //   bytes32 indexed seed,
    //   DkgResult result
    // )
    // where DkgResult contains: submitterMemberIndex, groupPubKey, misbehavedMembersIndices, signatures, signingMembersIndices, members, membersHash

    const result = event.args.result;
    const memberIds = result.members.map(id => parseInt(id.toString()));

    log.debug(`  Found ${memberIds.length} members`);

    return {
      blockNumber: event.blockNumber,
      transactionHash: event.transactionHash,
      memberIds: memberIds
    };

  } catch (error) {
    log.error(`Failed to query DKG event for ${ecdsaWalletID.slice(0, 10)}...: ${error.message}`);
    return null;
  }
}

/**
 * Get operator address from sortition pool by member ID
 */
async function getOperatorAddress(sortitionPool, memberId) {
  try {
    const operator = await sortitionPool.getIDOperator(memberId);
    return operator.toLowerCase();
  } catch (error) {
    log.warn(`Failed to get operator for member ID ${memberId}: ${error.message}`);
    return null;
  }
}

/**
 * Map operator address to provider info
 */
function getOperatorInfo(address, operators) {
  const normalizedAddr = address.toLowerCase();

  // Check KEEP operators
  const keepOp = operators.keep.find(op => op.address.toLowerCase() === normalizedAddr);
  if (keepOp) {
    return { ...keepOp, status: 'KEEP' };
  }

  // Check DISABLE operators
  const disableOp = operators.disable.find(op => op.address.toLowerCase() === normalizedAddr);
  if (disableOp) {
    return { ...disableOp, status: 'DISABLE' };
  }

  return {
    provider: 'UNKNOWN',
    address: address,
    status: 'UNKNOWN'
  };
}

/**
 * Main execution
 */
async function main() {
  log.info('DKG Event Query Tool');
  log.info('===================\n');

  // Load input data
  log.info('Loading configuration files...');
  const proofOfFunds = loadJSON(PROOF_OF_FUNDS_PATH);
  const operators = loadJSON(OPERATORS_PATH);
  const bridgeContract = loadJSON(BRIDGE_CONTRACT_PATH);

  log.success(`Loaded ${proofOfFunds.wallets.length} wallets from proof-of-funds`);
  log.success(`Loaded ${operators.operators.keep.length} KEEP + ${operators.operators.disable.length} DISABLE operators\n`);

  // Connect to Ethereum
  log.info(`Connecting to Ethereum: ${CONFIG.rpcUrl}`);
  const provider = new ethers.JsonRpcProvider(CONFIG.rpcUrl);

  // Initialize contracts
  const bridge = new ethers.Contract(
    bridgeContract.contractAddress,
    bridgeContract.abi,
    provider
  );

  // WalletRegistry ABI for DKG events
  // Correct struct from EcdsaDkg.sol:
  // struct Result { uint256 submitterMemberIndex; bytes groupPubKey; uint8[] misbehavedMembersIndices; bytes signatures; uint256[] signingMembersIndices; uint32[] members; bytes32 membersHash; }
  const walletRegistryAbi = [
    'event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, tuple(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members, bytes32 membersHash) result)',
    'event WalletCreated(bytes32 indexed walletID, bytes32 indexed dkgResultHash)',
    'function getWallet(bytes32 walletID) external view returns (tuple(bytes32 membersIdsHash, bytes32 publicKeyX, bytes32 publicKeyY))'
  ];

  const walletRegistry = new ethers.Contract(
    WALLET_REGISTRY_ADDRESS,
    walletRegistryAbi,
    provider
  );

  // Sortition Pool ABI
  const sortitionPoolAbi = [
    'function getIDOperator(uint32) external view returns (address)'
  ];

  const sortitionPool = new ethers.Contract(
    SORTITION_POOL_ADDRESS,
    sortitionPoolAbi,
    provider
  );

  // Verify connection
  try {
    const network = await provider.getNetwork();
    log.success(`Connected to network: ${network.name} (chainId: ${network.chainId})`);
    log.success(`Bridge contract: ${bridgeContract.contractAddress}`);
    log.success(`WalletRegistry contract: ${WALLET_REGISTRY_ADDRESS}`);
    log.success(`SortitionPool contract: ${SORTITION_POOL_ADDRESS}\n`);
  } catch (error) {
    log.error(`Failed to connect to Ethereum: ${error.message}`);
    process.exit(1);
  }

  // Determine wallets to query
  const walletsToQuery = CONFIG.limitWallets
    ? proofOfFunds.wallets.slice(0, CONFIG.limitWallets)
    : proofOfFunds.wallets;

  if (CONFIG.limitWallets) {
    log.warn(`LIMIT MODE - Processing first ${CONFIG.limitWallets} wallets only\n`);
  }

  log.info(`Querying wallet data from Bridge + DKG events...`);
  log.info(`This may take 2-3 minutes for all wallets...\n`);

  const walletData = [];
  const startTime = Date.now();

  for (let i = 0; i < walletsToQuery.length; i++) {
    const wallet = walletsToQuery[i];
    const progress = `[${i + 1}/${walletsToQuery.length}]`;

    log.info(`${progress} ${wallet.walletPublicKeyHash.slice(0, 10)}...`);

    try {
      // Step 1: Get ecdsaWalletID from Bridge
      const bridgeWallet = await bridge.wallets(wallet.walletPublicKeyHash);
      const ecdsaWalletID = bridgeWallet.ecdsaWalletID;
      const createdAt = parseInt(bridgeWallet.createdAt.toString());

      if (ecdsaWalletID === ethers.ZeroHash) {
        log.warn(`  Wallet has no ecdsaWalletID (not created via DKG)`);
        walletData.push({
          walletPKH: wallet.walletPublicKeyHash,
          btcBalance: parseFloat(wallet.walletBitcoinBalance),
          ecdsaWalletID: ethers.ZeroHash,
          state: 'Unknown',
          memberCount: 0,
          operators: []
        });
        continue;
      }

      // Step 2: Query DKG event for this wallet
      const dkgEvent = await getDkgEventForWallet(walletRegistry, ecdsaWalletID, createdAt, provider);

      if (!dkgEvent) {
        log.warn(`  No DKG event found`);
        walletData.push({
          walletPKH: wallet.walletPublicKeyHash,
          btcBalance: parseFloat(wallet.walletBitcoinBalance),
          ecdsaWalletID: ecdsaWalletID,
          state: 'Live',
          memberCount: 0,
          operators: []
        });
        continue;
      }

      // Step 3: Resolve member IDs to operator addresses
      const operatorAddresses = [];

      for (const memberId of dkgEvent.memberIds) {
        const address = await getOperatorAddress(sortitionPool, memberId);
        if (address) {
          operatorAddresses.push(address);
        }
      }

      // Step 4: Map operators to provider info
      const operatorInfos = operatorAddresses.map(addr => getOperatorInfo(addr, operators.operators));

      const hasDeprecated = operatorInfos.some(op => op.status === 'DISABLE');
      const hasActive = operatorInfos.some(op => op.status === 'KEEP');

      log.success(`  Found ${operatorInfos.length} operators (${hasDeprecated ? 'HAS DEPRECATED' : 'no deprecated'})`);

      walletData.push({
        walletPKH: wallet.walletPublicKeyHash,
        btcBalance: parseFloat(wallet.walletBitcoinBalance),
        ecdsaWalletID: ecdsaWalletID,
        state: 'Live', // Simplified - we know from earlier queries
        memberCount: operatorInfos.length,
        memberIds: dkgEvent.memberIds,
        operators: operatorInfos,
        hasDeprecatedOperator: hasDeprecated,
        hasActiveOperator: hasActive,
        needsSweep: hasDeprecated
      });

    } catch (error) {
      log.error(`  Failed: ${error.message}`);
    }
  }

  const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
  log.success(`\nCompleted in ${elapsed}s\n`);

  // Generate summary
  log.info('Generating summary...');

  const summary = {
    totalWallets: walletData.length,
    totalBTC: walletData.reduce((sum, w) => sum + w.btcBalance, 0).toFixed(8),
    byProvider: {
      STAKED: { wallets: 0, btc: 0 },
      P2P: { wallets: 0, btc: 0 },
      BOAR: { wallets: 0, btc: 0 },
      NUCO: { wallets: 0, btc: 0 }
    }
  };

  // Calculate per-provider totals
  walletData.forEach(wallet => {
    if (wallet.hasDeprecatedOperator) {
      const deprecatedOps = wallet.operators.filter(op => op.status === 'DISABLE');
      const providers = new Set(deprecatedOps.map(op => op.provider));

      providers.forEach(provider => {
        if (summary.byProvider[provider]) {
          summary.byProvider[provider].wallets++;
          summary.byProvider[provider].btc += wallet.btcBalance;
        }
      });
    }
  });

  // Round BTC
  Object.keys(summary.byProvider).forEach(provider => {
    summary.byProvider[provider].btc = parseFloat(summary.byProvider[provider].btc.toFixed(8));
  });

  // Save output
  const output = {
    timestamp: new Date().toISOString(),
    summary,
    wallets: walletData
  };

  fs.writeFileSync(OUTPUT_PATH, JSON.stringify(output, null, 2));
  log.success(`Results saved to ${OUTPUT_PATH}\n`);

  // Print summary
  console.log('='.repeat(80));
  console.log('SUMMARY');
  console.log('='.repeat(80));
  console.log(`\nTotal Wallets: ${summary.totalWallets}`);
  console.log(`Total BTC: ${summary.totalBTC} BTC\n`);

  console.log('BTC to Sweep by Provider:');
  Object.entries(summary.byProvider).forEach(([provider, stats]) => {
    if (stats.wallets > 0) {
      console.log(`  ${provider}: ${stats.wallets} wallets, ${stats.btc} BTC`);
    }
  });

  console.log('\n' + '='.repeat(80));
}

// Run
main().catch(error => {
  log.error(`Fatal error: ${error.message}`);
  console.error(error);
  process.exit(1);
});
