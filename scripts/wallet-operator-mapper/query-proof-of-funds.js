#!/usr/bin/env node

/**
 * tBTC Proof of Funds Query Tool
 *
 * Queries ALL tBTC wallets from Bridge contract and their Bitcoin balances.
 * Output format matches the required proof-of-funds JSON structure.
 *
 * Usage: node query-proof-of-funds.js
 * Output: Saves to specified path or prints to stdout
 */

const { ethers } = require('ethers');
const https = require('https');
const fs = require('fs');
const path = require('path');
require('dotenv').config();

// Configuration
const CONFIG = {
  ethereumRpcUrl: process.env.ETHEREUM_RPC_URL || 'https://ethereum.publicnode.com',
  bridgeAddress: '0x5e4861a80B55f035D899f66772117F00FA0E8e7B',
  // Use mempool.space API for Bitcoin balance queries (no API key needed)
  bitcoinApiBase: 'https://mempool.space/api',
  outputPath: process.env.OUTPUT_PATH || null,
  debug: process.env.DEBUG === 'true'
};

// Logging utilities
const log = {
  info: (msg) => console.error(`[INFO] ${msg}`),
  success: (msg) => console.error(`[OK] ${msg}`),
  error: (msg) => console.error(`[ERROR] ${msg}`),
  debug: (msg) => CONFIG.debug && console.error(`[DEBUG] ${msg}`),
  warn: (msg) => console.error(`[WARN] ${msg}`)
};

/**
 * Convert public key hash (20 bytes) to bech32 Bitcoin address
 */
function pubKeyHashToBech32Address(pubKeyHash) {
  // Remove 0x prefix if present
  const cleanHash = pubKeyHash.replace(/^0x/i, '').toLowerCase();

  // Bech32 character set
  const CHARSET = 'qpzry9x8gf2tvdw0s3jn54khce6mua7l';

  // Convert hex to 5-bit groups for bech32
  const hexToBytes = (hex) => {
    const bytes = [];
    for (let i = 0; i < hex.length; i += 2) {
      bytes.push(parseInt(hex.substr(i, 2), 16));
    }
    return bytes;
  };

  // Convert 8-bit bytes to 5-bit groups
  const convertBits = (data, fromBits, toBits, pad) => {
    let acc = 0;
    let bits = 0;
    const result = [];
    const maxv = (1 << toBits) - 1;

    for (const value of data) {
      acc = (acc << fromBits) | value;
      bits += fromBits;
      while (bits >= toBits) {
        bits -= toBits;
        result.push((acc >> bits) & maxv);
      }
    }

    if (pad) {
      if (bits > 0) {
        result.push((acc << (toBits - bits)) & maxv);
      }
    }

    return result;
  };

  // Bech32 polymod for checksum
  const polymod = (values) => {
    const GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];
    let chk = 1;
    for (const v of values) {
      const b = chk >> 25;
      chk = ((chk & 0x1ffffff) << 5) ^ v;
      for (let i = 0; i < 5; i++) {
        if ((b >> i) & 1) {
          chk ^= GEN[i];
        }
      }
    }
    return chk;
  };

  // Create checksum
  const hrpExpand = (hrp) => {
    const result = [];
    for (const c of hrp) {
      result.push(c.charCodeAt(0) >> 5);
    }
    result.push(0);
    for (const c of hrp) {
      result.push(c.charCodeAt(0) & 31);
    }
    return result;
  };

  const createChecksum = (hrp, data) => {
    const values = hrpExpand(hrp).concat(data).concat([0, 0, 0, 0, 0, 0]);
    const mod = polymod(values) ^ 1;
    const result = [];
    for (let i = 0; i < 6; i++) {
      result.push((mod >> (5 * (5 - i))) & 31);
    }
    return result;
  };

  // Build address
  const hrp = 'bc'; // mainnet
  const witnessVersion = 0; // P2WPKH
  const pubKeyHashBytes = hexToBytes(cleanHash);
  const data = [witnessVersion].concat(convertBits(pubKeyHashBytes, 8, 5, true));
  const checksum = createChecksum(hrp, data);
  const combined = data.concat(checksum);

  let address = hrp + '1';
  for (const d of combined) {
    address += CHARSET[d];
  }

  return address;
}

/**
 * Fetch Bitcoin address balance from mempool.space API with retry logic
 */
async function fetchBitcoinBalance(address, retries = 3) {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      const result = await new Promise((resolve, reject) => {
        const url = `${CONFIG.bitcoinApiBase}/address/${address}`;

        https.get(url, (res) => {
          let data = '';

          res.on('data', (chunk) => {
            data += chunk;
          });

          res.on('end', () => {
            try {
              if (res.statusCode === 429) {
                // Rate limited - signal for retry
                reject(new Error('RATE_LIMITED'));
                return;
              }

              if (res.statusCode !== 200) {
                log.warn(`API returned ${res.statusCode} for ${address}`);
                resolve(0);
                return;
              }

              const json = JSON.parse(data);
              // Balance is in satoshis
              const funded = json.chain_stats?.funded_txo_sum || 0;
              const spent = json.chain_stats?.spent_txo_sum || 0;
              const mempoolFunded = json.mempool_stats?.funded_txo_sum || 0;
              const mempoolSpent = json.mempool_stats?.spent_txo_sum || 0;

              // Total balance = chain + mempool
              const balanceSats = (funded - spent) + (mempoolFunded - mempoolSpent);
              resolve(balanceSats);
            } catch (err) {
              log.error(`Failed to parse balance for ${address}: ${err.message}`);
              resolve(0);
            }
          });
        }).on('error', (err) => {
          reject(err);
        });
      });

      return result;
    } catch (err) {
      if (err.message === 'RATE_LIMITED' && attempt < retries) {
        // Exponential backoff: 2s, 4s, 8s
        const waitTime = Math.pow(2, attempt) * 1000;
        log.warn(`Rate limited, waiting ${waitTime / 1000}s before retry ${attempt + 1}/${retries}...`);
        await sleep(waitTime);
      } else if (attempt < retries) {
        log.warn(`Network error for ${address}, retry ${attempt + 1}/${retries}: ${err.message}`);
        await sleep(1000);
      } else {
        log.error(`Failed after ${retries} attempts for ${address}: ${err.message}`);
        return 0;
      }
    }
  }
  return 0;
}

/**
 * Sleep helper for rate limiting
 */
function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Query all wallet events from Bridge contract
 */
async function getAllWalletPKHs(bridge, provider) {
  log.info('Querying NewWalletRegistered events from Bridge contract...');

  // Bridge deployment block - tBTC v2 launched late 2022/early 2023
  // Using block 16400000 (Jan 2023) to capture all wallets
  const startBlock = 16400000;
  const currentBlock = await provider.getBlockNumber();

  // Query in chunks to avoid RPC limits
  const chunkSize = 50000;
  const allPKHs = new Set();

  for (let fromBlock = startBlock; fromBlock < currentBlock; fromBlock += chunkSize) {
    const toBlock = Math.min(fromBlock + chunkSize - 1, currentBlock);
    log.debug(`Querying blocks ${fromBlock} to ${toBlock}...`);

    try {
      const filter = bridge.filters.NewWalletRegistered();
      const events = await bridge.queryFilter(filter, fromBlock, toBlock);

      for (const event of events) {
        // Event: NewWalletRegistered(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
        const pkh = event.args.walletPubKeyHash;
        allPKHs.add(pkh.toLowerCase());
      }

      log.debug(`  Found ${events.length} events, total unique PKHs: ${allPKHs.size}`);
    } catch (err) {
      log.warn(`Failed to query blocks ${fromBlock}-${toBlock}: ${err.message}`);
    }
  }

  log.success(`Found ${allPKHs.size} unique wallet PKHs`);
  return Array.from(allPKHs);
}

/**
 * Get wallet state from Bridge contract
 */
async function getWalletState(bridge, pkh) {
  try {
    const wallet = await bridge.wallets(pkh);
    // State enum: Unknown=0, Live=1, MovingFunds=2, Closing=3, Closed=4, Terminated=5
    const stateNames = ['Unknown', 'Live', 'MovingFunds', 'Closing', 'Closed', 'Terminated'];
    return {
      state: stateNames[wallet.state] || 'Unknown',
      createdAt: parseInt(wallet.createdAt.toString())
    };
  } catch (err) {
    return { state: 'Unknown', createdAt: 0 };
  }
}

/**
 * Main execution
 */
async function main() {
  log.info('='.repeat(60));
  log.info('tBTC Proof of Funds Query Tool');
  log.info('='.repeat(60));

  // Connect to Ethereum
  log.info(`Connecting to Ethereum: ${CONFIG.ethereumRpcUrl.substring(0, 50)}...`);
  const provider = new ethers.JsonRpcProvider(CONFIG.ethereumRpcUrl);

  // Verify connection
  const network = await provider.getNetwork();
  const currentBlock = await provider.getBlockNumber();
  log.success(`Connected to ${network.name} (chainId: ${network.chainId}), block: ${currentBlock}`);

  // Bridge ABI
  const bridgeAbi = [
    'event NewWalletRegistered(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)',
    'function wallets(bytes20 walletPubKeyHash) view returns (tuple(bytes32 ecdsaWalletID, bytes32 mainUtxoHash, uint64 pendingRedemptionsValue, uint32 createdAt, uint32 movingFundsRequestedAt, uint32 closingStartedAt, uint32 pendingMovedFundsSweepRequestsCount, uint8 state, bytes32 movingFundsTargetWalletsCommitmentHash))'
  ];

  const bridge = new ethers.Contract(CONFIG.bridgeAddress, bridgeAbi, provider);

  // Get all wallet PKHs
  const allPKHs = await getAllWalletPKHs(bridge, provider);

  log.info(`\nQuerying Bitcoin balances for ${allPKHs.length} wallets...`);
  log.info('(This will take a few minutes due to rate limiting)\n');

  const wallets = [];
  let totalBalance = 0n;

  for (let i = 0; i < allPKHs.length; i++) {
    const pkh = allPKHs[i];
    const cleanPKH = pkh.replace(/^0x/i, '').toLowerCase();
    const btcAddress = pubKeyHashToBech32Address(cleanPKH);

    log.info(`[${i + 1}/${allPKHs.length}] ${btcAddress.substring(0, 20)}...`);

    // Get wallet state from Ethereum
    const walletInfo = await getWalletState(bridge, pkh);

    // Get Bitcoin balance
    const balanceSats = await fetchBitcoinBalance(btcAddress);
    const balanceBTC = balanceSats / 100000000;

    totalBalance += BigInt(balanceSats);

    // Convert satoshis to BTC as integer string (truncated, no decimals)
    const balanceBTCInt = Math.floor(balanceSats / 100000000);

    wallets.push({
      walletPublicKeyHash: cleanPKH,
      walletBitcoinAddress: btcAddress,
      walletBitcoinBalance: balanceBTCInt.toString()
    });

    log.debug(`  State: ${walletInfo.state}, Balance: ${balanceBTC.toFixed(8)} BTC`);

    // Rate limit: more conservative - 1 request per 500ms to avoid 429s
    await sleep(500);
  }

  // Sort by balance descending
  wallets.sort((a, b) => {
    const balA = BigInt(a.walletBitcoinBalance);
    const balB = BigInt(b.walletBitcoinBalance);
    if (balB > balA) return 1;
    if (balB < balA) return -1;
    return 0;
  });

  // Calculate total
  const totalBTC = Number(totalBalance) / 100000000;

  // Calculate total in BTC (integer)
  const totalBTCInt = Math.floor(Number(totalBalance) / 100000000);

  // Build output
  const output = {
    wallets: wallets,
    totalBitcoinBalance: totalBTCInt.toString()
  };

  // Output
  log.info('\n' + '='.repeat(60));
  log.info('SUMMARY');
  log.info('='.repeat(60));
  log.info(`Total wallets: ${wallets.length}`);
  log.info(`Total BTC: ${totalBTC.toFixed(8)} BTC (${totalBalance.toString()} satoshis)`);
  log.info(`Last block: ${currentBlock}`);

  // Count wallets with balance
  const withBalance = wallets.filter(w => BigInt(w.walletBitcoinBalance) > 0n);
  log.info(`Wallets with balance: ${withBalance.length}`);

  // Output JSON
  if (CONFIG.outputPath) {
    fs.writeFileSync(CONFIG.outputPath, JSON.stringify(output, null, 2));
    log.success(`\nSaved to: ${CONFIG.outputPath}`);
  } else {
    console.log(JSON.stringify(output, null, 2));
  }
}

// Run
main().catch(err => {
  log.error(`Fatal error: ${err.message}`);
  console.error(err);
  process.exit(1);
});
