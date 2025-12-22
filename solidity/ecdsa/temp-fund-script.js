const { ethers, helpers } = require("hardhat");

(async () => {
  try {
    const mainAccount = process.env.MAIN_ACCOUNT;
    const stakingProvider = process.env.STAKING_PROVIDER;
    const ethAmount = process.env.ETH_AMOUNT;
    const tAmount = process.env.T_AMOUNT;
    
    const t = await helpers.contracts.getContract("T");
    const mainSigner = await ethers.getSigner(mainAccount);
    
    // Fund with ETH
    const ethTx = await mainSigner.sendTransaction({
      to: stakingProvider,
      value: ethers.utils.parseEther(ethAmount)
    });
    await ethTx.wait();
    console.log(`  ✓ Funded with ${ethAmount} ETH`);
    
    // Mint T tokens
    const tokenOwner = await t.owner();
    const ownerSigner = await ethers.getSigner(tokenOwner);
    const mintTx = await t.connect(ownerSigner).mint(stakingProvider, ethers.utils.parseEther(tAmount));
    await mintTx.wait();
    console.log(`  ✓ Minted ${tAmount} T tokens`);
    
    process.exit(0);
  } catch (error) {
    console.error("  Error:", error.message);
    process.exit(1);
  }
})();
