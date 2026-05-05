import verifyOnEtherscanOrContinue from "./etherscanVerification"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  // full-redeploy-sepolia-stack.sh sets this when --dkg-group-size 3: incremental compile
  // can still leave groupSize=100 bytecode in artifacts without a forced compile.
  if (process.env.THRESHOLD_FORCE_DKG_COMPILE === "1") {
    await hre.run("compile", { force: true })
  }

  const EcdsaSortitionPool = await deployments.get("EcdsaSortitionPool")

  // Non-mainnet: skipIfAlreadyDeployed false so hardhat-deploy can redeploy when bytecode
  // changes (e.g. groupSize 100 → 3). Mainnet: true so bytecode/artifact drift cannot
  // silently overwrite deployments/mainnet/EcdsaDkgValidator.json while WalletRegistry
  // still points at the old on-chain validator (THRESHOLD_FORCE_DKG_COMPILE only forces compile).
  const skipIfAlreadyDeployed = hre.network.name === "mainnet"

  const existingDeployment = await deployments.getOrNull("EcdsaDkgValidator")
  if (existingDeployment && !skipIfAlreadyDeployed) {
    hre.deployments.log(
      `WARNING: redeploying EcdsaDkgValidator on ${hre.network.name} ` +
        `(previous address ${existingDeployment.address}). ` +
        "Ensure WalletRegistry is updated to point to the new address."
    )
  }

  const EcdsaDkgValidator = await deployments.deploy("EcdsaDkgValidator", {
    from: deployer,
    args: [EcdsaSortitionPool.address],
    log: true,
    waitConfirmations: 1,
    skipIfAlreadyDeployed,
  })

  if (
    hre.network.tags.etherscan &&
    process.env.DISABLE_HARDHAT_VERIFY !== "true"
  ) {
    await verifyOnEtherscanOrContinue(hre, () =>
      helpers.etherscan.verify(EcdsaDkgValidator)
    )
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "EcdsaDkgValidator",
      address: EcdsaDkgValidator.address,
    })
  }

  return true
}

export default func

func.tags = ["EcdsaDkgValidator"]
func.dependencies = ["EcdsaSortitionPool"]
func.id = "deploy_ecdsa_dkg_validator"
