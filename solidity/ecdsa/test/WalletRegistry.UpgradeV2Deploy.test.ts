import { deployments, ethers, helpers } from "hardhat"
import chai, { expect } from "chai"
import chaiAsPromised from "chai-as-promised"

import type { Allowlist, WalletRegistry } from "../typechain"

chai.use(chaiAsPromised)

// EIP-1967 implementation slot: bytes32(uint256(keccak256("eip1967.proxy.implementation")) - 1)
const IMPL_SLOT =
  "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"

// Covers deploy/17_upgrade_wallet_registry_v2.ts. The script is gated behind
// UPGRADE_WALLET_REGISTRY_V2 and is skipped by default, so it is otherwise
// never exercised by `yarn test` (WalletRegistry.Upgrade.test.ts drives its
// upgrade scenario directly via `upgrades.upgradeProxy`, not via this deploy
// script). This test actually runs the script's body on the hardhat network.
describe("deploy script - UpgradeWalletRegistryV2", () => {
  let walletRegistry: WalletRegistry
  let allowlist: Allowlist
  let governanceBeforeUpgrade: string

  before(async () => {
    // Ensure the shared base deployment exists (ReimbursementPool, RandomBeacon,
    // TokenStaking, WalletRegistry, Allowlist, ...). This reuses the `::global`
    // deployments.fixture() snapshot most other test files already populate; if
    // this file happens to run first in the suite it deploys everything fresh.
    await deployments.fixture()

    // Governance is not a named EOA here: deploy/10_transfer_governance.ts
    // points WalletRegistry.governance() at the deployed
    // WalletRegistryGovernance contract, not at the "governance" named
    // account. The invariant under test is that the upgrade preserves
    // whatever it already was, so capture it before running the upgrade
    // fixture below.
    const preUpgradeDeployment = await deployments.get("WalletRegistry")
    const preUpgradeWalletRegistry = await ethers.getContractAt(
      "WalletRegistry",
      preUpgradeDeployment.address,
    )
    governanceBeforeUpgrade = await preUpgradeWalletRegistry.governance()

    // The upgrade script is gated behind this env var and is skipped by default,
    // so it is otherwise never run by `yarn test` (WalletRegistry.Upgrade.test.ts
    // drives its upgrade scenario directly via `upgrades.upgradeProxy`, not via
    // this deploy script).
    process.env.UPGRADE_WALLET_REGISTRY_V2 = "true"

    // Run only the two additional scripts needed on top of the base deployment:
    // - "TransferProxyAdminOwnership" (deploy 11): the upgrade script's testnet
    //   path requires `esdm` to already own the ProxyAdmin, and that transfer is
    //   not one of its declared `dependencies`.
    // - "UpgradeWalletRegistryV2" (deploy 17): the script under test.
    // `keepExistingDeployments: true` reuses the deployments from the base
    // fixture above instead of wiping and trying to redeploy everything under
    // this narrow tag set (which would fail: ReimbursementPool/RandomBeacon/
    // TokenStaking come from an external deploy step that is filtered by this
    // same narrow `tags` argument and would be skipped). `fallbackToGlobal:
    // false` ensures this specific tag combination actually executes instead of
    // silently reverting to the (env-var-unaware) `::global` snapshot cached
    // above.
    await deployments.fixture(
      ["UpgradeWalletRegistryV2", "TransferProxyAdminOwnership"],
      { keepExistingDeployments: true, fallbackToGlobal: false },
    )

    // The deployments.json record for "WalletRegistry" still carries the
    // pre-upgrade ABI (the deploy script never re-saves it), so attach with
    // the compiled V2 interface directly, exactly as the deploy script does
    // for its own post-upgrade verification.
    const walletRegistryDeployment = await deployments.get("WalletRegistry")
    walletRegistry = await ethers.getContractAt(
      "WalletRegistry",
      walletRegistryDeployment.address,
    )

    allowlist = await helpers.contracts.getContract<Allowlist>("Allowlist")
  })

  after(() => {
    delete process.env.UPGRADE_WALLET_REGISTRY_V2
  })

  it("should save the new implementation deployment artifact", async () => {
    const implementationDeployment = await deployments.get(
      "WalletRegistryV2Implementation",
    )
    expect(implementationDeployment.address).to.not.equal(ethers.ZeroAddress)
  })

  it("should point the WalletRegistry proxy at the new implementation", async () => {
    const implementationDeployment = await deployments.get(
      "WalletRegistryV2Implementation",
    )

    const implSlot = await ethers.provider.getStorage(
      await walletRegistry.getAddress(),
      IMPL_SLOT,
    )
    const currentImplementation = ethers.getAddress(`0x${implSlot.slice(-40)}`)

    expect(currentImplementation).to.equal(implementationDeployment.address)
  })

  it("should initialize the upgraded WalletRegistry with the deployed Allowlist address", async () => {
    expect(await walletRegistry.allowlist()).to.equal(
      await allowlist.getAddress(),
    )
  })

  it("should preserve WalletRegistry governance across the upgrade", async () => {
    expect(await walletRegistry.governance()).to.equal(governanceBeforeUpgrade)
  })
})
