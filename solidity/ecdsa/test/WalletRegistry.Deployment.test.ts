import { deployments, ethers, helpers, upgrades } from "hardhat"
import chai, { expect } from "chai"
import chaiAsPromised from "chai-as-promised"

import type { Contract } from "ethers"
import type { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers"
import type {
  WalletRegistry,
  WalletRegistryGovernance,
  TransparentUpgradeableProxy,
} from "../typechain"

chai.use(chaiAsPromised)

const { ZeroAddress: AddressZero } = ethers

describe("WalletRegistry - Deployment", async () => {
  let deployer: SignerWithAddress
  let governance: SignerWithAddress
  let esdm: SignerWithAddress

  let walletRegistry: WalletRegistry
  let walletRegistryGovernance: WalletRegistryGovernance
  let walletRegistryProxy: TransparentUpgradeableProxy
  let proxyAdmin: Contract
  let walletRegistryImplementationAddress: string

  before(async () => {
    await deployments.fixture()
    ;({ deployer, governance, esdm } = await helpers.signers.getNamedSigners())

    walletRegistry =
      await helpers.contracts.getContract<WalletRegistry>("WalletRegistry")

    const { implementation } = await deployments.get("WalletRegistry")
    if (!implementation) {
      throw new Error("WalletRegistry deployment has no implementation address")
    }
    walletRegistryImplementationAddress = implementation

    walletRegistryGovernance =
      await helpers.contracts.getContract<WalletRegistryGovernance>(
        "WalletRegistryGovernance",
      )

    walletRegistryProxy = await ethers.getContractAt(
      "TransparentUpgradeableProxy",
      await walletRegistry.getAddress(),
    )

    proxyAdmin = await upgrades.admin.getInstance()

    expect(deployer.address, "deployer is the same as governance").not.equal(
      governance.address,
    )
  })

  it("should set WalletRegistry proxy admin", async () => {
    expect(
      await upgrades.erc1967.getAdminAddress(await walletRegistry.getAddress()),
      "invalid WalletRegistry proxy admin",
    ).to.be.equal(await proxyAdmin.getAddress())
  })

  it("should set ProxyAdmin owner", async () => {
    expect(await proxyAdmin.owner(), "invalid ProxyAdmin owner").to.be.equal(
      esdm.address,
    )
  })

  it("should set WalletRegistry implementation", async () => {
    expect(
      await upgrades.erc1967.getImplementationAddress(
        await walletRegistry.getAddress(),
      ),
      "invalid WalletRegistry implementation",
    ).to.be.equal(walletRegistryImplementationAddress)
  })

  it("should set WalletRegistry implementation in ProxyAdmin", async () => {
    expect(
      await proxyAdmin.getProxyImplementation(
        await walletRegistryProxy.getAddress(),
      ),
      "invalid proxy implementation",
    ).to.be.equal(walletRegistryImplementationAddress)
  })

  it("should set WalletRegistry governance", async () => {
    expect(
      await walletRegistry.governance(),
      "invalid WalletRegistry governance",
    ).equal(await walletRegistryGovernance.getAddress())
  })

  it("should set WalletRegistryGovernance owner", async () => {
    expect(
      await walletRegistryGovernance.owner(),
      "invalid WalletRegistryGovernance owner",
    ).equal(governance.address)
  })

  it("should set WalletRegistry address in artifact to the proxy address", async () => {
    expect(
      await walletRegistry.getAddress(),
      "invalid WalletRegistry address",
    ).equal(await walletRegistryProxy.getAddress())
  })

  it("should revert when initialize called again", async () => {
    await expect(
      walletRegistry.initialize(AddressZero, AddressZero, AddressZero),
    ).to.be.revertedWith("Initializable: contract is already initialized")
  })
})
