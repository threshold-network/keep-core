import hre, { deployments, ethers, helpers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import type { RandomBeacon, TokenStaking, T } from "../../typechain"

async function initializedOperator() {
  await deployments.fixture()
  const [owner, provider, operator] = (await ethers.getSigners()).slice(10)
  const args = {
    owner: owner.address,
    provider: provider.address,
    operator: operator.address,
    amount: 1_000_000,
    authorization: 500_000,
  }
  await hre.run("initialize", args)
  return {
    args,
    staking: await helpers.contracts.getContract<TokenStaking>("TokenStaking"),
    beacon: await helpers.contracts.getContract<RandomBeacon>("RandomBeacon"),
    token: await helpers.contracts.getContract<T>("T"),
  }
}

async function operatorWithDefaultAuthorization() {
  await deployments.fixture()
  const [owner, provider, operator] = (await ethers.getSigners()).slice(10)
  const args = {
    owner: owner.address,
    provider: provider.address,
    operator: operator.address,
    amount: 1_000_000,
  }
  await hre.run("initialize", args)
  return {
    args,
    staking: await helpers.contracts.getContract<TokenStaking>("TokenStaking"),
    beacon: await helpers.contracts.getContract<RandomBeacon>("RandomBeacon"),
  }
}

describe("Initialization tasks", () => {
  it("mints, stakes, authorizes and registers an operator", async () => {
    const { args, staking, beacon, token } =
      await loadFixture(initializedOperator)
    expect((await staking.stakes(args.provider)).tStake).to.equal(
      ethers.parseEther("1000000"),
    )
    expect(
      await staking.authorizedStake(args.provider, await beacon.getAddress()),
    ).to.equal(ethers.parseEther("500000"))
    expect(await beacon.operatorToStakingProvider(args.operator)).to.equal(
      args.provider,
    )
    expect(await token.balanceOf(args.owner)).to.equal(0n)
  })

  it("does not send another transaction when stake and registration already match", async () => {
    const { args } = await loadFixture(initializedOperator)
    const before = await ethers.provider.getBlockNumber()
    await hre.run("initialize:staking", args)
    await hre.run("authorize:beacon", args)
    await hre.run("register:beacon", args)
    // Re-run the full initialize task, exercising add_beta_operator too.
    await hre.run("initialize", args)
    expect(await ethers.provider.getBlockNumber()).to.equal(before)
  })

  it("tops up an existing stake and increases authorization", async () => {
    const { args, staking, beacon, token } =
      await loadFixture(initializedOperator)
    await hre.run("initialize:staking", { ...args, amount: 1_200_000 })
    await hre.run("authorize:beacon", { ...args, authorization: 700_000 })
    expect((await staking.stakes(args.provider)).tStake).to.equal(
      ethers.parseEther("1200000"),
    )
    expect(
      await staking.authorizedStake(args.provider, await beacon.getAddress()),
    ).to.equal(ethers.parseEther("700000"))
    expect(await token.balanceOf(args.owner)).to.equal(0n)
  })

  it("defaults authorization to the beacon's minimumAuthorization", async () => {
    const { args, staking, beacon } = await loadFixture(
      operatorWithDefaultAuthorization,
    )
    expect(
      await staking.authorizedStake(args.provider, await beacon.getAddress()),
    ).to.equal(await beacon.minimumAuthorization())
  })

})
