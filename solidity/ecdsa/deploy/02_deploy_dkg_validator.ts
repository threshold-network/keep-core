import verifyOnEtherscanOrContinue from "./etherscanVerification"
import verifyOnTenderlyOrContinue from "./tenderlyVerification"

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

  // Allowlist of networks where bytecode redeploy on artifact change is safe
  // (e.g. groupSize 100 → 3 during local/testnet iteration). Any other network
  // (mainnet and any future production-like alias) keeps the existing
  // deployment record so bytecode/artifact drift cannot silently overwrite
  // deployments/<network>/EcdsaDkgValidator.json while WalletRegistry still
  // points at the old on-chain validator. THRESHOLD_FORCE_DKG_COMPILE only
  // forces compile, not redeploy.
  const redeploySafeNetworks = new Set(["hardhat", "development", "sepolia"])
  const skipIfAlreadyDeployed = !redeploySafeNetworks.has(hre.network.name)

  // Fail-fast guard against EcdsaDkgValidator / WalletRegistry binding drift.
  // The validator is bound into WalletRegistry exactly once, at proxy init
  // (EcdsaDkg.init reverts when a validator is already set), and no deploy step
  // re-points it. On a redeploy-safe network a validator bytecode/args change
  // would deploy a NEW validator address and overwrite the artifact, while
  // 03_deploy_wallet_registry skips an already-deployed WalletRegistry — leaving
  // the on-chain WalletRegistry validating DKG against the OLD validator. Refuse
  // that partial redeploy: the validator and WalletRegistry must be wiped and
  // redeployed together (see full-redeploy-sepolia-stack.sh). A re-run that does
  // not change the validator bytecode/args is unaffected (fetchIfDifferent ==
  // false), so idempotent re-runs still work.
  if (!skipIfAlreadyDeployed) {
    const { differences: validatorWouldRedeploy } =
      await deployments.fetchIfDifferent("EcdsaDkgValidator", {
        from: deployer,
        args: [EcdsaSortitionPool.address],
      })
    const existingWalletRegistry = await deployments.getOrNull("WalletRegistry")
    if (validatorWouldRedeploy && existingWalletRegistry) {
      throw new Error(
        `Refusing to redeploy EcdsaDkgValidator on '${hre.network.name}': a ` +
          "WalletRegistry deployment already exists and binds the validator " +
          "once-only, so a validator-only redeploy would desync " +
          `deployments/${hre.network.name}/EcdsaDkgValidator.json from the ` +
          "on-chain binding. Wipe and redeploy the WalletRegistry in the same " +
          "run (e.g. full-redeploy-sepolia-stack.sh) before redeploying the validator."
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
    await verifyOnTenderlyOrContinue(hre, () =>
      hre.tenderly.verify({
        name: "EcdsaDkgValidator",
        address: EcdsaDkgValidator.address,
      })
    )
  }

  return true
}

export default func

func.tags = ["EcdsaDkgValidator"]
func.dependencies = ["EcdsaSortitionPool"]
func.id = "deploy_ecdsa_dkg_validator"
