/* eslint-disable no-console */
import { task } from "hardhat/config"

import type { HardhatRuntimeEnvironment } from "hardhat/types"

task("initialize-wallet-owner", "Initializes Wallet Owner for Wallet Registry")
  .addParam("walletOwnerAddress", "The Wallet Owner's address")
  .setAction(async (args, hre) => {
    const { walletOwnerAddress } = args

    await initializeWalletOwner(hre, walletOwnerAddress)
  })

async function initializeWalletOwner(
  hre: HardhatRuntimeEnvironment,
  walletOwnerAddress: string
): Promise<void> {
  const { getNamedAccounts, ethers, deployments, helpers } = hre
  const { read, execute } = deployments
  const { deployer, governance } = await getNamedAccounts()
  const ZERO = ethers.constants.AddressZero

  if (!ethers.utils.isAddress(walletOwnerAddress)) {
    throw Error(`invalid address: ${walletOwnerAddress}`)
  }

  const wrg = await deployments.get("WalletRegistryGovernance")
  const wr = await deployments.get("WalletRegistry")
  const wrGovernance = await read("WalletRegistry", {}, "governance")
  const wrLinked = await read("WalletRegistryGovernance", {}, "walletRegistry")
  if (!helpers.address.equal(wr.address, wrLinked)) {
    throw new Error(
      `WalletRegistryGovernance (${wrg.address}) is wired to WalletRegistry ${wrLinked} but deployments WalletRegistry is ${wr.address}. ` +
        "Align keep-core Phase F artifacts with tbtc-v2 Phase G copies before initializeWalletOwner."
    )
  }

  if (!helpers.address.equal(wrg.address, wrGovernance)) {
    console.log(
      "WalletRegistry governance is not this WalletRegistryGovernance deployment; skipping initializeWalletOwner " +
        "(use the on-chain governance contract from keep-core Phase F, or governance-delay updates)."
    )
    return
  }

  const woRaw = String(
    await read("WalletRegistry", {}, "walletOwner")
  ).toLowerCase()
  if (woRaw !== ZERO.toLowerCase()) {
    console.log(
      "WalletRegistry wallet owner already initialized; skipping initializeWalletOwner"
    )
    return
  }

  // TOCTOU recheck: governance can be transferred between the early read
  // above and this execute() on shared networks. Re-read immediately before
  // the tx so a concurrent transferGovernance doesn't slip past the gate.
  const wrGovernanceNow = await read("WalletRegistry", {}, "governance")
  if (!helpers.address.equal(wrg.address, wrGovernanceNow)) {
    throw new Error(
      `WalletRegistry.governance() drifted before initializeWalletOwner (now ${wrGovernanceNow}, expected WRG ${wrg.address}).`
    )
  }

  // `initializeWalletOwner` on WalletRegistryGovernance is `onlyOwner` (Ownable),
  // not `onlyGovernance`. The named account `governance` often matches `deployer`
  // on Sepolia but can differ once ownership is moved; always match the on-chain
  // owner to avoid sending from the wrong key while still hitting WR Governable.
  const wrgOwner = await read("WalletRegistryGovernance", {}, "owner")
  let from: string
  if (helpers.address.equal(wrgOwner, deployer)) {
    from = deployer
  } else if (helpers.address.equal(wrgOwner, governance)) {
    from = governance
  } else {
    throw new Error(
      `WalletRegistryGovernance owner is ${wrgOwner}; expected deployer (${deployer}) or governance (${governance}). ` +
        "Ownable.transferOwnership must leave the deploy key as owner for this step, or call initializeWalletOwner with the current owner key."
    )
  }

  const tx = await execute(
    "WalletRegistryGovernance",
    { from, log: true, waitConfirmations: 1 },
    "initializeWalletOwner",
    walletOwnerAddress
  )

  console.log(
    `Initialized Wallet Owner address: ${walletOwnerAddress} in transaction: ${tx.transactionHash}`
  )
}

export default initializeWalletOwner
