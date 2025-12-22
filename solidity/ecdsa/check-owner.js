const { ethers } = require("hardhat");

async function main() {
  const deployedAddress = "0xbd49D2e3E501918CD08Eb4cCa34984F428c83464";
  const WalletRegistry = await ethers.getContractAt("WalletRegistry", deployedAddress);
  
  const owner = await WalletRegistry.walletOwner();
  console.log("Wallet Owner:", owner);
  
  const governance = await WalletRegistry.governance();
  console.log("Governance:", governance);
  
  const WalletRegistryGovernance = await ethers.getContractAt("WalletRegistryGovernance", governance);
  const govOwner = await WalletRegistryGovernance.owner();
  console.log("Governance Owner:", govOwner);
  
  const signers = await ethers.getSigners();
  console.log("\nAvailable accounts:");
  for (let i = 0; i < Math.min(5, signers.length); i++) {
    console.log(`  [${i}]: ${signers[i].address}`);
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
