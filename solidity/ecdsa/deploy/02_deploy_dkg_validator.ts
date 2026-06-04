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

  // If a prior validator deployment exists and we're about to redeploy it (non-mainnet),
  // log a loud warning. The WalletRegistry stores the validator address in its constructor
  // args, so a new validator deploy that does not also re-deploy / re-initialize the
  // WalletRegistry will leave WR pointing at the OLD on-chain validator.
  if (!skipIfAlreadyDeployed) {
    const existing = await deployments.getOrNull("EcdsaDkgValidator")
    if (existing) {
      deployments.log(
        `WARN: redeploying EcdsaDkgValidator on "${hre.network.name}"; ` +
          `previous address ${existing.address} will be replaced. ` +
          "WalletRegistry still references the previous validator address until it is also redeployed " +
          "(03_deploy_wallet_registry.ts must run, or the WR proxy must be re-pointed manually)."
      )
    }
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
