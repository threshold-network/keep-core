#!/usr/bin/env node
/**
 * Script to approve T tokens for TokenStaking contract
 * Usage: node scripts/approve-tokens.js <operator-address> <amount-hex> <keyfile-path> <password>
 * Example: node scripts/approve-tokens.js 0x123... 0xa968163f0a57b400000 ./keystore/operator1.json password
 * 
 * Must be run from solidity/ecdsa directory or with proper Hardhat setup
 */

const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
  const operatorAddress = process.argv[2];
  const amountHex = process.argv[3];
  const keyfilePath = process.argv[4];
  const password = process.argv[5] || process.env.KEEP_ETHEREUM_PASSWORD || "";

  if (!operatorAddress || !amountHex || !keyfilePath) {
    console.error("Usage: node scripts/approve-tokens.js <operator-address> <amount-hex> <keyfile-path> [password]");
    console.error("Must be run from solidity/ecdsa directory");
    process.exit(1);
  }

  try {
    // Initialize Hardhat with development network
    const hre = require("hardhat");
    // Set network to development
    process.env.HARDHAT_NETWORK = "development";
    
    await hre.run("compile", { quiet: true }).catch(() => {}); // Ensure contracts are compiled
    
    // Get contracts (requires Hardhat environment)
    const { helpers } = require("hardhat");
    const t = await helpers.contracts.getContract("T");
    const staking = await helpers.contracts.getContract("TokenStaking");

    // Read keyfile
    const keyfile = JSON.parse(fs.readFileSync(keyfilePath, "utf8"));
    
    // Decrypt keyfile to get private key
    let wallet;
    if (password) {
      wallet = await ethers.Wallet.fromEncryptedJson(JSON.stringify(keyfile), password);
    } else {
      // Try without password (for unencrypted keyfiles)
      const privateKey = "0x" + keyfile.crypto?.kdfparams?.salt || keyfile.privateKey;
      if (!privateKey || privateKey === "0x") {
        throw new Error("Could not extract private key from keyfile. Password may be required.");
      }
      wallet = new ethers.Wallet(privateKey);
    }

    // Connect wallet to provider (use development network)
    const provider = new hre.ethers.providers.JsonRpcProvider("http://localhost:8545");
    const operatorSigner = wallet.connect(provider);

    // Verify address matches
    if (operatorSigner.address.toLowerCase() !== operatorAddress.toLowerCase()) {
      console.error(`⚠ Warning: Keyfile address (${operatorSigner.address}) doesn't match operator address (${operatorAddress})`);
      console.error(`   Using keyfile address: ${operatorSigner.address}`);
    }

    // Connect T contract to the provider with the operator signer
    const tWithSigner = t.connect(operatorSigner);
    
    // Check current allowance
    const currentAllowance = await tWithSigner.allowance(operatorSigner.address, staking.address);
    const amount = ethers.BigNumber.from(amountHex);
    const { from1e18 } = helpers.number;

    console.log(`Operator: ${operatorSigner.address}`);
    console.log(`TokenStaking address: ${staking.address}`);
    console.log(`Current allowance: ${from1e18(currentAllowance)} T`);
    console.log(`Requested amount: ${from1e18(amount)} T`);

    if (currentAllowance.gte(amount)) {
      console.log("✓ Already approved");
      process.exit(0);
    }

    // Approve tokens
    console.log(`Approving ${from1e18(amount)} T for TokenStaking (${staking.address})...`);
    const tx = await tWithSigner.approve(staking.address, amount);
    console.log(`Transaction hash: ${tx.hash}`);
    console.log("Waiting for confirmation...");
    await tx.wait();
    console.log("✓ Approval successful!");

    // Verify new allowance
    const newAllowance = await tWithSigner.allowance(operatorSigner.address, staking.address);
    console.log(`New allowance: ${from1e18(newAllowance)} T`);

  } catch (error) {
    console.error("Error:", error.message);
    console.error("Stack:", error.stack);
    if (error.message.includes("invalid password") || error.message.includes("wrong password")) {
      console.error("⚠ Password incorrect or keyfile is encrypted");
    }
    if (error.message.includes("network") || error.message.includes("provider")) {
      console.error("⚠ Network connection issue. Make sure Geth is running on localhost:8545");
    }
    process.exit(1);
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});

