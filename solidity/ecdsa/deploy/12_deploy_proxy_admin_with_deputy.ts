import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { ethers, getNamedAccounts, upgrades, deployments } = hre
  const { deployer, dao, esdm } = await getNamedAccounts()

  const WalletRegistryProxyAdminWithDeputy = await deployments.deploy(
    "WalletRegistryProxyAdminWithDeputy",
    {
      contract: "ProxyAdminWithDeputy",
      from: deployer,
      args: [dao, esdm],
      log: true,
      waitConfirmations: 1,
    }
  )

  const WalletRegistry = await deployments.get("WalletRegistry")

  const proxyAdmin = await ethers.getContractAt(
    ["function changeProxyAdmin(address,address)"],
    await upgrades.erc1967.getAdminAddress(WalletRegistry.address)
  )

  await (
    await proxyAdmin
      .connect(await ethers.getSigner(esdm))
      .changeProxyAdmin(
        WalletRegistry.address,
        WalletRegistryProxyAdminWithDeputy.address
      )
  ).wait()
}

export default func

func.tags = ["WalletRegistryProxyAdminWithDeputy"]
func.dependencies = ["WalletRegistry"]

// For now we skip this script as DAO is not yet established.
func.skip = async () => true
