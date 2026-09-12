import waitForConfirmations from "../utils/wait-for-confirmations"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction, DeployOptions } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  const T = await deployments.get("T")
  const TokenStaking = await deployments.get("TokenStaking")
  const ReimbursementPool = await deployments.get("ReimbursementPool")
  const BeaconSortitionPool = await deployments.get("BeaconSortitionPool")
  const BeaconDkgValidator = await deployments.get("BeaconDkgValidator")

  const deployOptions: DeployOptions = {
    from: deployer,
    log: true,
    waitConfirmations: 1,
  }

  const BLS = await deployments.deploy("BLS", deployOptions)

  const BeaconAuthorization = await deployments.deploy(
    "BeaconAuthorization",
    deployOptions,
  )

  const BeaconDkg = await deployments.deploy("BeaconDkg", deployOptions)

  const BeaconInactivity = await deployments.deploy(
    "BeaconInactivity",
    deployOptions,
  )

  const RandomBeacon = await deployments.deploy("RandomBeacon", {
    contract:
      process.env.TEST_USE_STUBS_BEACON === "true"
        ? "RandomBeaconStub"
        : undefined,
    args: [
      BeaconSortitionPool.address,
      T.address,
      TokenStaking.address,
      BeaconDkgValidator.address,
      ReimbursementPool.address,
    ],
    libraries: {
      BLS: BLS.address,
      BeaconAuthorization: BeaconAuthorization.address,
      BeaconDkg: BeaconDkg.address,
      BeaconInactivity: BeaconInactivity.address,
    },
    ...deployOptions,
  })

  await helpers.ownable.transferOwnership(
    "BeaconSortitionPool",
    RandomBeacon.address,
    deployer,
  )

  if (hre.network.tags.etherscan) {
    if (RandomBeacon.transactionHash) {
      await waitForConfirmations(
        hre.ethers.provider,
        RandomBeacon.transactionHash,
        2,
        300000,
      )
    }
    await helpers.etherscan.verify(BLS)
    await helpers.etherscan.verify(BeaconAuthorization)
    await helpers.etherscan.verify(BeaconDkg)
    await helpers.etherscan.verify(BeaconInactivity)
    await helpers.etherscan.verify(RandomBeacon)
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "RandomBeacon",
      address: RandomBeacon.address,
    })
  }
}

export default func

func.tags = ["RandomBeacon"]
func.dependencies = [
  "T",
  "TokenStaking",
  "ReimbursementPool",
  "BeaconSortitionPool",
  "BeaconDkgValidator",
]
