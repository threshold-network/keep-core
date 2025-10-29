#!/usr/bin/env node

/**
 * Proper Per-Operator BTC Analysis
 *
 * Calculates actual BTC share per operator/provider
 */

const fs = require('fs');
const path = require('path');

const MAPPING_FILE = path.join(__dirname, 'wallet-operator-mapping.json');
const OPERATORS_FILE = path.join(__dirname, 'operators.json');

const data = JSON.parse(fs.readFileSync(MAPPING_FILE));
const operatorsConfig = JSON.parse(fs.readFileSync(OPERATORS_FILE));

console.log('🔍 Proper BTC Analysis by Operator/Provider\n');
console.log('='.repeat(80));

// Get wallets with operator data
const walletsWithOps = data.wallets.filter(w => w.memberCount > 0);

console.log(`\nTotal wallets with operator data: ${walletsWithOps.length}`);
console.log(`Total BTC in these wallets: ${walletsWithOps.reduce((s, w) => s + w.btcBalance, 0).toFixed(8)} BTC`);

// Method 1: Per-wallet breakdown showing which providers are involved
console.log('\n' + '='.repeat(80));
console.log('METHOD 1: Per-Wallet Provider Involvement');
console.log('='.repeat(80));

const walletBreakdown = walletsWithOps.map(wallet => {
  const deprecatedOps = wallet.operators.filter(op => op.status === 'DISABLE');
  const providers = new Set(deprecatedOps.map(op => op.provider));

  return {
    walletPKH: wallet.walletPKH,
    btcBalance: wallet.btcBalance,
    totalOperators: wallet.memberCount,
    deprecatedCount: deprecatedOps.length,
    providersInvolved: Array.from(providers).sort(),
    activeOperators: wallet.operators.filter(op => op.status === 'KEEP').length
  };
});

// Group by provider combination
const providerGroups = {};
walletBreakdown.forEach(w => {
  const key = w.providersInvolved.join('+');
  if (!providerGroups[key]) {
    providerGroups[key] = {
      providers: w.providersInvolved,
      wallets: [],
      totalBTC: 0
    };
  }
  providerGroups[key].wallets.push(w);
  providerGroups[key].totalBTC += w.btcBalance;
});

console.log('\nWallets grouped by provider involvement:\n');
Object.entries(providerGroups).forEach(([key, group]) => {
  console.log(`Providers: ${group.providers.join(', ') || 'None'}`);
  console.log(`  Wallets: ${group.wallets.length}`);
  console.log(`  Total BTC: ${group.totalBTC.toFixed(8)} BTC`);
  console.log(`  Average BTC per wallet: ${(group.totalBTC / group.wallets.length).toFixed(2)} BTC`);
  console.log();
});

// Method 2: Equal-split calculation (BTC / operators in wallet)
console.log('='.repeat(80));
console.log('METHOD 2: Equal-Split Per-Operator Share');
console.log('='.repeat(80));
console.log('\nAssumption: BTC is split equally among all 100 operators in each wallet\n');

const operatorShares = {};

// Initialize
operatorsConfig.operators.keep.forEach(op => {
  operatorShares[op.address.toLowerCase()] = {
    provider: op.provider,
    status: 'KEEP',
    totalShare: 0,
    walletCount: 0
  };
});

operatorsConfig.operators.disable.forEach(op => {
  operatorShares[op.address.toLowerCase()] = {
    provider: op.provider,
    status: 'DISABLE',
    totalShare: 0,
    walletCount: 0
  };
});

// Calculate shares
walletsWithOps.forEach(wallet => {
  const sharePerOperator = wallet.btcBalance / wallet.memberCount;

  wallet.operators.forEach(op => {
    const addr = op.address.toLowerCase();
    if (operatorShares[addr]) {
      operatorShares[addr].totalShare += sharePerOperator;
      operatorShares[addr].walletCount++;
    }
  });
});

// Group by provider
const providerShares = {
  STAKED: { keep: 0, disable: 0, keepWallets: 0, disableWallets: 0 },
  P2P: { keep: 0, disable: 0, keepWallets: 0, disableWallets: 0 },
  BOAR: { keep: 0, disable: 0, keepWallets: 0, disableWallets: 0 },
  NUCO: { keep: 0, disable: 0, keepWallets: 0, disableWallets: 0 }
};

Object.entries(operatorShares).forEach(([addr, data]) => {
  if (providerShares[data.provider]) {
    if (data.status === 'KEEP') {
      providerShares[data.provider].keep += data.totalShare;
      providerShares[data.provider].keepWallets += data.walletCount;
    } else {
      providerShares[data.provider].disable += data.totalShare;
      providerShares[data.provider].disableWallets += data.walletCount;
    }
  }
});

console.log('Per-Provider BTC Shares (Equal-Split Method):\n');
console.log('Provider | KEEP Ops Share | DISABLE Ops Share | Total Share');
console.log('-'.repeat(80));

Object.entries(providerShares).forEach(([provider, shares]) => {
  const total = shares.keep + shares.disable;
  console.log(`${provider.padEnd(8)} | ${shares.keep.toFixed(2).padStart(13)} BTC | ${shares.disable.toFixed(2).padStart(17)} BTC | ${total.toFixed(2)} BTC`);
});

console.log('\n' + '='.repeat(80));
console.log('METHOD 3: Deprecated Operator BTC (What needs to be "moved")');
console.log('='.repeat(80));
console.log('\nThis shows the BTC share held by deprecated operators that');
console.log('needs to remain accessible during the draining period.\n');

Object.entries(providerShares).forEach(([provider, shares]) => {
  console.log(`${provider}:`);
  console.log(`  Deprecated operator share: ${shares.disable.toFixed(2)} BTC`);
  console.log(`  Number of wallets involved: ${shares.disableWallets}`);
  console.log(`  Average per wallet: ${shares.disableWallets > 0 ? (shares.disable / shares.disableWallets).toFixed(2) : 0} BTC`);
  console.log();
});

// Method 4: Detailed operator-by-operator breakdown
console.log('='.repeat(80));
console.log('METHOD 4: Individual Operator Breakdown (Top 20 by BTC)');
console.log('='.repeat(80));

const operatorList = Object.entries(operatorShares)
  .map(([addr, data]) => ({
    address: addr,
    ...data
  }))
  .sort((a, b) => b.totalShare - a.totalShare)
  .slice(0, 20);

console.log('\nOperator Address | Provider | Status | BTC Share | Wallets');
console.log('-'.repeat(80));
operatorList.forEach(op => {
  const addrShort = op.address.slice(0, 10) + '...' + op.address.slice(-6);
  console.log(`${addrShort} | ${op.provider.padEnd(7)} | ${op.status.padEnd(7)} | ${op.totalShare.toFixed(2).padStart(8)} BTC | ${op.walletCount}`);
});

// Summary
console.log('\n' + '='.repeat(80));
console.log('SUMMARY & INTERPRETATION');
console.log('='.repeat(80));

const totalBTC = walletsWithOps.reduce((s, w) => s + w.btcBalance, 0);
const totalDeprecatedShare = Object.values(providerShares).reduce((s, p) => s + p.disable, 0);

console.log(`\nTotal BTC in analyzed wallets: ${totalBTC.toFixed(8)} BTC`);
console.log(`Total BTC share of deprecated operators: ${totalDeprecatedShare.toFixed(2)} BTC`);
console.log(`Percentage held by deprecated operators: ${(totalDeprecatedShare / totalBTC * 100).toFixed(2)}%`);

console.log('\n⚠️  IMPORTANT NOTES:');
console.log('1. The "equal-split" is a CALCULATION METHOD, not how threshold signatures work');
console.log('2. In reality, ALL 100 operators must participate for wallet actions (51/100 threshold)');
console.log('3. The deprecated operators do not individually "own" their share');
console.log('4. What matters: which WALLETS contain deprecated operators (all 24 do)');
console.log('5. For sweeps: need active operators to coordinate, not individual BTC shares');

console.log('\n✅ CORRECT INTERPRETATION:');
console.log('- All 24 wallets (5,923.91 BTC) contain deprecated operators');
console.log('- Natural draining or manual sweeps affect the ENTIRE wallet, not per-operator');
console.log('- Coordination needed: active operators from STAKED, P2P, BOAR (and NUCO for 20 wallets)');
