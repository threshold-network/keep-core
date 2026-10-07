import verifyOnEtherscanOrContinue from "../deploy-utils/etherscanVerification"
import verifyOnTenderlyOrContinue from "../deploy-utils/tenderlyVerification"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers, ethers } = hre
  const { deployer } = await getNamedAccounts()

  const WalletRegistry = await deployments.get("WalletRegistry")
  // 60 seconds for Sepolia. 1 week otherwise.
  const GOVERNANCE_DELAY = hre.network.name === "sepolia" ? 60 : 604800
  const args = [WalletRegistry.address, GOVERNANCE_DELAY]
  let WalletRegistryGovernance = await deployments.getOrNull(
    "WalletRegistryGovernance"
  )

  const currentGovernance = await deployments.read(
    "WalletRegistry",
    "governance"
  )
  if (
    currentGovernance !== "0x0000000000000000000000000000000000000000" &&
    !helpers.address.equal(currentGovernance, deployer)
  ) {
    const artifact = await deployments.getArtifact("WalletRegistryGovernance")
    const liveGovernance = await ethers.getContractAt(
      artifact.abi,
      currentGovernance
    )
    let linkedRegistry: string
    try {
      linkedRegistry = await liveGovernance.walletRegistry()
    } catch {
      throw new Error(
        `WalletRegistry governance is ${currentGovernance}, which is not a WalletRegistryGovernance contract`
      )
    }
    if (!helpers.address.equal(linkedRegistry, WalletRegistry.address)) {
      throw new Error(
        `WalletRegistryGovernance at ${currentGovernance} points to ${linkedRegistry}, expected ${WalletRegistry.address}`
      )
    }
    if (
      !WalletRegistryGovernance ||
      !helpers.address.equal(
        WalletRegistryGovernance.address,
        currentGovernance
      )
    ) {
      // The constructor delay is not readable on chain (governance can change
      // the live delay), so the args are the deploy-time delay for this network.
      WalletRegistryGovernance = {
        address: currentGovernance,
        abi: artifact.abi,
        args,
      }
      await deployments.save(
        "WalletRegistryGovernance",
        WalletRegistryGovernance
      )
    }
    deployments.log(
      `using live WalletRegistryGovernance at ${currentGovernance}`
    )
  } else {
    // Reuse a saved record only if it was deployed for this registry. A record
    // left over from an earlier registry must not receive its governance.
    const sameArgs =
      WalletRegistryGovernance?.args?.length === args.length &&
      WalletRegistryGovernance.args.every(
        (arg, index) =>
          String(arg).toLowerCase() === String(args[index]).toLowerCase()
      )
    WalletRegistryGovernance = await deployments.deploy(
      "WalletRegistryGovernance",
      {
        from: deployer,
        skipIfAlreadyDeployed: sameArgs,
        args,
        log: true,
        waitConfirmations: 1,
      }
    )
  }

  // Verify on every run, including when reusing a live governance deployment:
  // the deployment record survives a failed attempt, so a later run retries it.
  // Failures are tolerated off mainnet only.
  if (
    hre.network.tags.etherscan &&
    process.env.DISABLE_HARDHAT_VERIFY !== "true"
  ) {
    await verifyOnEtherscanOrContinue(hre, () =>
      helpers.etherscan.verify(WalletRegistryGovernance)
    )
  }

  if (hre.network.tags.tenderly) {
    await verifyOnTenderlyOrContinue(hre, () =>
      hre.tenderly.verify({
        name: "WalletRegistryGovernance",
        address: WalletRegistryGovernance.address,
      })
    )
  }
}

export default func

func.tags = ["WalletRegistryGovernance"]
func.dependencies = ["WalletRegistry"]
