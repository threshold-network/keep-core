#!/usr/bin/env bash
cd tbtc-v2/solidity
env -u FORKING_URL -u FORKING_BLOCK TEST_USE_STUBS_TBTC=true USE_EXTERNAL_DEPLOY=true NODE_OPTIONS=--max-old-space-size=4096 npx hardhat test test/vault/ReservationVault.test.ts >out/kill_vault.out 2>&1
