import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction, DeployOptions } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  let RandomBeacon = await deployments.getOrNull("RandomBeacon")
  const poolOwner = await deployments.read("BeaconSortitionPool", "owner")
  if (RandomBeacon && helpers.address.equal(poolOwner, RandomBeacon.address)) {
    deployments.log(`using existing RandomBeacon at ${RandomBeacon.address}`)
  } else {
    if (!helpers.address.equal(poolOwner, deployer)) {
      throw new Error(
        `BeaconSortitionPool is owned by ${poolOwner}; cannot deploy a new RandomBeacon from ${deployer}`
      )
    }

    const T = await deployments.get("T")
    const TokenStaking = await deployments.get("TokenStaking")
    const ReimbursementPool = await deployments.get("ReimbursementPool")
    const BeaconSortitionPool = await deployments.get("BeaconSortitionPool")
    const BeaconDkgValidator = await deployments.get("BeaconDkgValidator")

    const deployOptions: DeployOptions = {
      from: deployer,
      log: true,
      waitConfirmations: hre.network.tags.etherscan ? 2 : 1,
    }

    const BLS = await deployments.deploy("BLS", deployOptions)

    const BeaconAuthorization = await deployments.deploy(
      "BeaconAuthorization",
      deployOptions
    )

    const BeaconDkg = await deployments.deploy("BeaconDkg", deployOptions)

    const BeaconInactivity = await deployments.deploy(
      "BeaconInactivity",
      deployOptions
    )

    RandomBeacon = await deployments.deploy("RandomBeacon", {
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
      deployer
    )
  }

  if (hre.network.tags.etherscan) {
    // Deployment records survive verification failures. Verify them on every
    // run, including when the beacon already owns its sortition pool.
    // A consumer may hold only the RandomBeacon record.
    const verifyRecorded = async (name: string) => {
      const record = await deployments.getOrNull(name)
      if (record) {
        await helpers.etherscan.verify(record)
      } else {
        deployments.log(`no ${name} deployment record; skipping verification`)
      }
    }
    await verifyRecorded("BLS")
    await verifyRecorded("BeaconAuthorization")
    await verifyRecorded("BeaconDkg")
    await verifyRecorded("BeaconInactivity")
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
