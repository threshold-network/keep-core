#!/usr/bin/env node

/**
 * Validate Operator List Completeness
 *
 * Compare CSV operators with on-chain discovered operators
 */

const fs = require('fs');
const path = require('path');

const MAPPING_FILE = path.join(__dirname, 'wallet-operator-mapping.json');
const CSV_PATH = process.env.THRESHOLD_STAKERS_CSV_PATH ||
  path.join(__dirname, 'data', 'threshold_stakers_may_2025.csv');

console.log('🔍 Operator List Validation\n');
console.log('='.repeat(80));

// Load on-chain data
const mappingData = JSON.parse(fs.readFileSync(MAPPING_FILE));

// Extract all unique operators from on-chain data
const onChainOperators = new Set();
mappingData.wallets.forEach(wallet => {
  if (wallet.operators) {
    wallet.operators.forEach(op => {
      onChainOperators.add(op.address.toLowerCase());
    });
  }
});

console.log(`\nOperators found on-chain: ${onChainOperators.size}`);

// Parse CSV
const csvData = fs.readFileSync(CSV_PATH, 'utf8');
const csvLines = csvData.split('\n').slice(1).filter(line => line.trim() && !line.startsWith('Total'));

const csvOperators = new Set();
const csvOperatorDetails = [];

csvLines.forEach(line => {
  const parts = line.split(',');
  if (parts.length >= 3 && parts[2]) {
    const operatorAddr = parts[2].trim().toLowerCase();
    if (operatorAddr && operatorAddr.startsWith('0x')) {
      csvOperators.add(operatorAddr);
      csvOperatorDetails.push({
        identification: parts[0]?.trim(),
        stakingProvider: parts[1]?.trim(),
        operatorAddress: operatorAddr,
        stake: parts[4]?.trim()
      });
    }
  }
});

console.log(`Operators in CSV: ${csvOperators.size}\n`);

// Compare
console.log('='.repeat(80));
console.log('COMPARISON');
console.log('='.repeat(80));

// Operators in CSV but NOT found on-chain
const csvOnly = Array.from(csvOperators).filter(addr => !onChainOperators.has(addr));

console.log(`\n❌ Operators in CSV but NOT found on-chain: ${csvOnly.length}`);
if (csvOnly.length > 0) {
  csvOnly.forEach(addr => {
    const detail = csvOperatorDetails.find(d => d.operatorAddress === addr);
    console.log(`  - ${addr} (${detail?.identification || 'Unknown'})`);
  });
}

// Operators found on-chain but NOT in CSV
const onChainOnly = Array.from(onChainOperators).filter(addr => !csvOperators.has(addr));

console.log(`\n⚠️  Operators found on-chain but NOT in CSV: ${onChainOnly.length}`);
if (onChainOnly.length > 0) {
  onChainOnly.forEach(addr => {
    // Try to find this operator in our mapping data
    let foundInWallets = 0;
    let totalShare = 0;

    mappingData.wallets.forEach(wallet => {
      if (wallet.operators) {
        const op = wallet.operators.find(o => o.address.toLowerCase() === addr);
        if (op) {
          foundInWallets++;
          totalShare += wallet.btcBalance / wallet.memberCount;
        }
      }
    });

    console.log(`  - ${addr}`);
    console.log(`    Found in ${foundInWallets} wallets`);
    console.log(`    Estimated BTC share: ${totalShare.toFixed(2)} BTC`);
  });
}

// Operators in BOTH CSV and on-chain
const commonOperators = Array.from(csvOperators).filter(addr => onChainOperators.has(addr));

console.log(`\n✅ Operators in BOTH CSV and on-chain: ${commonOperators.length}`);

// Calculate total BTC for ALL operators
console.log('\n' + '='.repeat(80));
console.log('ALL OPERATORS BTC ANALYSIS');
console.log('='.repeat(80));

const allOperatorShares = {};

// Initialize with CSV operators
csvOperatorDetails.forEach(detail => {
  allOperatorShares[detail.operatorAddress] = {
    identification: detail.identification,
    stakingProvider: detail.stakingProvider,
    inCSV: true,
    btcShare: 0,
    walletCount: 0,
    status: 'UNKNOWN'
  };
});

// Add on-chain only operators
onChainOnly.forEach(addr => {
  allOperatorShares[addr] = {
    identification: 'Not in CSV',
    stakingProvider: 'Unknown',
    inCSV: false,
    btcShare: 0,
    walletCount: 0,
    status: 'UNKNOWN'
  };
});

// Calculate BTC shares from on-chain data
mappingData.wallets.forEach(wallet => {
  if (wallet.operators && wallet.memberCount > 0) {
    const sharePerOp = wallet.btcBalance / wallet.memberCount;

    wallet.operators.forEach(op => {
      const addr = op.address.toLowerCase();
      if (!allOperatorShares[addr]) {
        allOperatorShares[addr] = {
          identification: 'Found on-chain',
          stakingProvider: op.provider || 'Unknown',
          inCSV: false,
          btcShare: 0,
          walletCount: 0,
          status: op.status || 'UNKNOWN'
        };
      }

      allOperatorShares[addr].btcShare += sharePerOp;
      allOperatorShares[addr].walletCount++;
      allOperatorShares[addr].status = op.status || allOperatorShares[addr].status;
    });
  }
});

// Sort by BTC share
const sortedOperators = Object.entries(allOperatorShares)
  .map(([addr, data]) => ({ address: addr, ...data }))
  .sort((a, b) => b.btcShare - a.btcShare);

// Summary by category
console.log('\nBTC Distribution by Operator Type:\n');

const csvOpsTotal = sortedOperators
  .filter(op => op.inCSV && op.btcShare > 0)
  .reduce((sum, op) => sum + op.btcShare, 0);

const nonCsvOpsTotal = sortedOperators
  .filter(op => !op.inCSV && op.btcShare > 0)
  .reduce((sum, op) => sum + op.btcShare, 0);

const csvOpsActive = sortedOperators
  .filter(op => op.inCSV && op.btcShare > 0);

const nonCsvOpsActive = sortedOperators
  .filter(op => !op.inCSV && op.btcShare > 0);

console.log(`CSV Operators (active on-chain): ${csvOpsActive.length}`);
console.log(`  Total BTC share: ${csvOpsTotal.toFixed(2)} BTC`);
console.log(`  Percentage: ${(csvOpsTotal / 5923.91 * 100).toFixed(2)}%`);

console.log(`\nNon-CSV Operators (active on-chain): ${nonCsvOpsActive.length}`);
console.log(`  Total BTC share: ${nonCsvOpsTotal.toFixed(2)} BTC`);
console.log(`  Percentage: ${(nonCsvOpsTotal / 5923.91 * 100).toFixed(2)}%`);

console.log(`\nTotal BTC accounted for: ${(csvOpsTotal + nonCsvOpsTotal).toFixed(2)} BTC`);

// Top 50 operators by BTC
console.log('\n' + '='.repeat(80));
console.log('TOP 50 OPERATORS BY BTC SHARE');
console.log('='.repeat(80));

console.log('\nRank | Address (short) | Identification | In CSV? | BTC Share | Wallets | Status');
console.log('-'.repeat(100));

sortedOperators.slice(0, 50).forEach((op, i) => {
  const addrShort = op.address.slice(0, 10) + '...' + op.address.slice(-6);
  const idShort = (op.identification || 'Unknown').substring(0, 20).padEnd(20);
  const inCsv = op.inCSV ? '✅' : '❌';
  const btc = op.btcShare.toFixed(2).padStart(8);
  const wallets = op.walletCount.toString().padStart(3);

  console.log(`${(i+1).toString().padStart(4)} | ${addrShort} | ${idShort} | ${inCsv}      | ${btc} BTC | ${wallets} | ${op.status}`);
});

// Check completeness
console.log('\n' + '='.repeat(80));
console.log('COMPLETENESS ASSESSMENT');
console.log('='.repeat(80));

const csvMissingOperators = csvOperators.size - commonOperators.length;
const onChainMissingFromCsv = onChainOperators.size - commonOperators.length;

console.log(`\n CSV completeness: ${commonOperators.length}/${csvOperators.size} operators found on-chain (${((commonOperators.length/csvOperators.size)*100).toFixed(1)}%)`);
console.log(`On-chain coverage: ${commonOperators.length}/${onChainOperators.size} operators in CSV (${((commonOperators.length/onChainOperators.size)*100).toFixed(1)}%)`);

if (csvMissingOperators > 0) {
  console.log(`\n⚠️  ${csvMissingOperators} CSV operators NOT found on-chain (may be inactive or never participated in DKG)`);
}

if (onChainMissingFromCsv > 0) {
  console.log(`\n⚠️  ${onChainMissingFromCsv} active operators NOT in CSV (missing from documentation!)`);
}

if (csvMissingOperators === 0 && onChainMissingFromCsv === 0) {
  console.log('\n✅ CSV is COMPLETE - all operators documented');
} else {
  console.log('\n❌ CSV is INCOMPLETE - missing operators or contains inactive ones');
}

// Final summary
console.log('\n' + '='.repeat(80));
console.log('SUMMARY');
console.log('='.repeat(80));

console.log(`\nTotal unique operators in system: ${sortedOperators.filter(op => op.btcShare > 0).length}`);
console.log(`Total BTC in analyzed wallets: 5,923.91 BTC`);
console.log(`\nOperators by status:`);

const keepOps = sortedOperators.filter(op => op.status === 'KEEP' && op.btcShare > 0);
const disableOps = sortedOperators.filter(op => op.status === 'DISABLE' && op.btcShare > 0);
const unknownOps = sortedOperators.filter(op => op.status === 'UNKNOWN' && op.btcShare > 0);

console.log(`  KEEP (active): ${keepOps.length} operators, ${keepOps.reduce((s, op) => s + op.btcShare, 0).toFixed(2)} BTC`);
console.log(`  DISABLE (deprecated): ${disableOps.length} operators, ${disableOps.reduce((s, op) => s + op.btcShare, 0).toFixed(2)} BTC`);
console.log(`  UNKNOWN: ${unknownOps.length} operators, ${unknownOps.reduce((s, op) => s + op.btcShare, 0).toFixed(2)} BTC`);
