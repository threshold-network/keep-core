const hre = require("hardhat");

async function main() {
  const WalletRegistry = await ethers.getContract("WalletRegistry");
  const owner = await WalletRegistry.walletOwner();
  console.log("Current Wallet Owner:", owner);
  
  // Also check governance
  try {
    const WalletRegistryGovernance = await ethers.getContract("WalletRegistryGovernance");
    const governanceOwner = await WalletRegistryGovernance.owner();
    console.log("Governance Owner:", governanceOwner);
    
    const accounts = await ethers.getSigners();
    console.log("Available accounts:");
    for (let i = 0; i < Math.min(5, accounts.length); i++) {
      console.log(`  [${i}]: ${accounts[i].address}`);
    }
  } catch (e) {
    console.log("Could not get governance owner:", e.message);
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
