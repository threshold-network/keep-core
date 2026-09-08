import hre, { deployments, ethers, helpers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import type {
  WalletRegistry,
  TokenStaking,
  T,
  SortitionPool,
} from "../../typechain"

async function taskContracts() {
  await deployments.fixture()
  const [owner, provider, operator, beneficiary, authorizer] = (
    await ethers.getSigners()
  ).slice(10)
  return {
    args: {
      owner: owner.address,
      provider: provider.address,
      operator: operator.address,
      beneficiary: beneficiary.address,
      authorizer: authorizer.address,
      amount: 1_000_000,
    },
    staking: await helpers.contracts.getContract<TokenStaking>("TokenStaking"),
    registry:
      await helpers.contracts.getContract<WalletRegistry>("WalletRegistry"),
    token: await helpers.contracts.getContract<T>("T"),
    pool: await helpers.contracts.getContract<SortitionPool>(
      "EcdsaSortitionPool",
    ),
  }
}

async function initializedOperator() {
  const contracts = await taskContracts()
  await hre.run("initialize", contracts.args)
  return contracts
}

describe("ECDSA initialization tasks", () => {
  it("initializes staking with distinct beneficiary and authorizer accounts", async () => {
    const { args, staking, token } = await loadFixture(taskContracts)
    await hre.run("initialize:staking", args)
    expect((await staking.stakes(args.provider)).tStake).to.equal(
      ethers.parseEther("1000000"),
    )
    expect(await staking.rolesOf(args.provider)).to.deep.equal([
      args.owner,
      args.beneficiary,
      args.authorizer,
    ])
    expect(await token.balanceOf(args.owner)).to.equal(0n)
  })

  it("registers an operator through register:ecdsa", async () => {
    const { args, registry } = await loadFixture(taskContracts)
    await hre.run("register:ecdsa", args)
    expect(await registry.operatorToStakingProvider(args.operator)).to.equal(
      args.provider,
    )
  })

  it("initializes staking, minimum authorization, registration and beta membership", async () => {
    const { args, staking, registry, pool } =
      await loadFixture(initializedOperator)
    expect((await staking.stakes(args.provider)).tStake).to.equal(
      ethers.parseEther("1000000"),
    )
    expect(
      await staking.authorizedStake(args.provider, await registry.getAddress()),
    ).to.equal(await registry.minimumAuthorization())
    expect(await registry.operatorToStakingProvider(args.operator)).to.equal(
      args.provider,
    )
    expect(await pool.isBetaOperator(args.operator)).to.equal(true)
  })

  it("does not send another transaction when stake, authorization and registration match", async () => {
    const { args } = await loadFixture(initializedOperator)
    const before = await ethers.provider.getBlockNumber()
    await hre.run("initialize:staking", args)
    await hre.run("authorize:ecdsa", args)
    await hre.run("register:ecdsa", args)
    expect(await ethers.provider.getBlockNumber()).to.equal(before)
  })

  it("tops up an existing stake and increases ECDSA authorization", async () => {
    const { args, staking, registry, token } =
      await loadFixture(initializedOperator)
    await hre.run("initialize:staking", { ...args, amount: 1_200_000 })
    await hre.run("authorize:ecdsa", { ...args, authorization: 700_000 })
    expect((await staking.stakes(args.provider)).tStake).to.equal(
      ethers.parseEther("1200000"),
    )
    expect(
      await staking.authorizedStake(args.provider, await registry.getAddress()),
    ).to.equal(ethers.parseEther("700000"))
    expect(await token.balanceOf(args.owner)).to.equal(0n)
  })
})
