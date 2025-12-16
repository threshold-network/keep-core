import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers, helpers } = hre

  const { deployer, governance } = await getNamedAccounts()

  // Get the WalletRegistry deployment - this will be replaced by Allowlist
  // but we still need it for initialization
  const WalletRegistry = await deployments.get("WalletRegistry")

  // Deploy the Allowlist contract using upgradeable proxy pattern
  const [allowlist, proxyDeployment] = await helpers.upgrades.deployProxy(
    "Allowlist",
    {
      initializerArgs: [WalletRegistry.address],
      factoryOpts: {
        signer: await ethers.getSigner(deployer),
      },
      proxyOpts: {
        kind: "transparent",
      },
    }
  )

  // Transfer ownership to governance if specified and different from deployer
  if (governance && governance !== deployer) {
    await helpers.ownable.transferOwnership("Allowlist", governance, deployer)
  }

  // Log deployment information
  console.log(`Allowlist deployed at: ${allowlist.address}`)
  console.log(
    `Allowlist proxy admin: ${proxyDeployment.receipt.contractAddress}`
  )
  console.log(`Allowlist owner: ${await allowlist.owner()}`)

  return true
}

export default func

func.tags = ["Allowlist"]
func.dependencies = ["WalletRegistry"]
func.id = "deploy_allowlist"
