import { HardhatRuntimeEnvironment } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { ZeroAddress } from "ethers";

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers } = hre;
  const { deployer } = await getNamedAccounts();

  // Get deployed contract addresses
  let WalletRegistry: any = null;
  try {
    WalletRegistry = await deployments.get("WalletRegistry");
  } catch (e) {
    // Try to get from ecdsa deployments
  }

  if (!WalletRegistry) {
    // Try to get from ecdsa deployments
    try {
      const fs = require("fs");
      const path = require("path");
      const ecdsaPath = path.resolve(
        __dirname,
        "../../ecdsa/deployments/development/WalletRegistry.json"
      );
      if (fs.existsSync(ecdsaPath)) {
        const data = JSON.parse(fs.readFileSync(ecdsaPath, "utf8"));
        WalletRegistry = { address: data.address };
      }
    } catch (e) {
      throw new Error("WalletRegistry not found. Please deploy ECDSA contracts first.");
    }
  }

  let ReimbursementPool: any = null;
  try {
    ReimbursementPool = await deployments.get("ReimbursementPool");
  } catch (e) {
    // Will try file system lookup below
  }

  if (!ReimbursementPool) {
    try {
      const fs = require("fs");
      const path = require("path");
      const rbPath = path.resolve(
        __dirname,
        "../../random-beacon/deployments/development/ReimbursementPool.json"
      );
      if (fs.existsSync(rbPath)) {
        const data = JSON.parse(fs.readFileSync(rbPath, "utf8"));
        ReimbursementPool = { address: data.address };
      }
      } catch (e) {
        // Use zero address if not found
        ReimbursementPool = { address: ZeroAddress };
      }
  }

  // Deploy Bridge stub
  const Bridge = await deployments.deploy("BridgeStub", {
    contract: "BridgeStub",
    from: deployer,
    args: [
      ZeroAddress, // bank
      ZeroAddress, // relay
      WalletRegistry.address, // ecdsaWalletRegistry
      ReimbursementPool.address || ZeroAddress, // reimbursementPool
    ],
    log: true,
    waitConfirmations: 1,
  });

  // Save as Bridge for compatibility
  await deployments.save("Bridge", {
    address: Bridge.address,
    abi: Bridge.abi,
  });

  // Deploy MaintainerProxy stub
  const MaintainerProxy = await deployments.deploy("MaintainerProxyStub", {
    contract: "MaintainerProxyStub",
    from: deployer,
    log: true,
    waitConfirmations: 1,
  });

  await deployments.save("MaintainerProxy", {
    address: MaintainerProxy.address,
    abi: MaintainerProxy.abi,
  });

  // Deploy WalletProposalValidator stub
  const WalletProposalValidator = await deployments.deploy(
    "WalletProposalValidatorStub",
    {
      contract: "WalletProposalValidatorStub",
      from: deployer,
      log: true,
      waitConfirmations: 1,
    }
  );

  await deployments.save("WalletProposalValidator", {
    address: WalletProposalValidator.address,
    abi: WalletProposalValidator.abi,
  });

  console.log(`Bridge stub deployed at: ${Bridge.address}`);
  console.log(`MaintainerProxy stub deployed at: ${MaintainerProxy.address}`);
  console.log(
    `WalletProposalValidator stub deployed at: ${WalletProposalValidator.address}`
  );
};

export default func;
func.tags = ["TBTCStubs"];
