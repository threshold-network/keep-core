# Sepolia deploy - handoff for the key holder

Decision D21: the human runs the rehearsed script with the deployer key. The agent never handles the key; afterwards it checks the results read-only.

## What you need

- The private key for the deployer EOA `0xb9B8a92677fcad86dc8d36fe947184283B03b3C8`. It owns the Sepolia ProxyAdmin `0x0E9bD483CC8Fb649636e0cd7a60BC48b6C040103` and BridgeGovernance `0x61700ebaCCfd9FB0100C35DDd38e4cC3e9057c5A` (checked read-only on 2026-10-09).
- A checkout of the reservation contracts at the reconciled contracts commit. Do not use an earlier Sepolia rehearsal build, and not the older rehearsal build either: neither contains the moved public PR head `c27b71281`, and the older one also lacks the 8 public contract fixes and the funding-proof acceptance request. The deploy script is unchanged; the contracts come from the integrated reservation line, which the published `reservations-upgrade` branch carries. It contains the runbook, the deploy script, and the contracts with the hold cap, the feasibility guard, release to pool and the hand-off reserve. Run `yarn install --immutable` and `yarn build` in `solidity/`.
- npm publish rights for `@keep-network/tbtc-v2` (step 7).

## Order

1. **Rehearse first** (no key needed). From `solidity/`:
   ```sh
   FORKING_URL="$SEPOLIA_RPC_URL" FORKING_BLOCK=<latest Sepolia block - 20> \
     npx hardhat run --network hardhat scripts/reservations-sepolia-rehearsal.ts
   ```
   Expect exit 0 and no failed step. The off-chain steps (npm publish, keep-core build) show as SKIP; the fee reserve step also shows SKIP, because the fresh vault already reads 0.
2. **Run the script on Sepolia, phase 1.** From `solidity/`, with the deployer key (never paste it into chat or logs):
   ```sh
   export CHAIN_API_URL="$SEPOLIA_RPC_URL"
   export ACCOUNTS_PRIVATE_KEYS="$SEPOLIA_PRIVATE_KEY"   # the deployer EOA key
   export GAS_PRICE_GWEI=2
   PHASE=prepare npx hardhat run --network sepolia scripts/reservations-sepolia-rehearsal.ts
   ```
   It does runbook steps 1-6 and 10 (deploy, upgrade, new governance, router, caps and parameters, fee reserve 0), waits out the two 60 s governance delays in real time, and prints the addresses keep-core needs. If the first account is not the deployer EOA it stops before sending anything. If it fails or you interrupt it, run the same command again: it checks each step on-chain and resumes. Send the agent the printed addresses.
3. **Off-chain, runbook steps 7-9:** publish npm `sepolia` (step 7); the agent sets the activation block B in keep-core on the drill-fix branch (`9c4e1b221`: the five-argument caps setter and member-proof hand-off bindings, the keeper gauges and the keeper wallet-discovery fix), builds and releases the client (step 8; tell the agent when to pick B, it must be a future block); confirm every drill operator runs that build (step 9).
4. **Run the script, phase 2**, only when the chain is at or past B:
   ```sh
   PHASE=activate ACTIVATION_BLOCK=<B> npx hardhat run --network sepolia scripts/reservations-sepolia-rehearsal.ts
   ```
   It does steps 11-12 (`setVaultStatus(vault, true)` and the post-activation checks). It refuses to run if the current block is below B or anything staged differs from the runbook values.
   The manual `cast` steps in `solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md` remain as a fallback only.
5. **Stop and tell the agent** if any on-chain check differs from the runbook's expected value. Do not continue past an irreversible step (see the runbook's irreversibility table) on a mismatch.

## Values (re-check against live state on the day)

Read first: `depositParameters()` (on 2026-10-09: dust 1,000,000, reveal-ahead 1,296,000 s) and `liveWalletsCount()` (4).

Caps: per-wallet 2,000,000; single 2,000,000; max active 2; third-party holds 1; hand-off reserve 2.
Parameters: min 10,000; tx max fee 1,000; term 7,776,000 s; dissolution 86,400 s; max total 4,000,000; max per wallet 1; action timeout 86,400 s; renewal window 86,400 s.

With 4 Live wallets, reserve 2 and one ordinary reservation per wallet, the admission budget is `(4 - 2) * 1 = 2`. The reserve does not keep an identified slot permanently empty. If fewer than 4 Live wallets remain, stop and review drill capacity with the agent. Do not lower the reserve without a separate decision. Privileged hand-offs from retiring wallets may exceed a target's ordinary count and amount caps; the owner, maintainer or source member must still satisfy every other request rule.

The Bridge now refuses any value set that makes reserved deposits impossible, so a wrong cap or term reverts at finalize instead of activating a useless vault.

## After the deploy

The agent:
- re-reads every address and parameter read-only;
- regenerates the keep-core bindings from the published package;
- fills in the drill runbook's TBD items.
