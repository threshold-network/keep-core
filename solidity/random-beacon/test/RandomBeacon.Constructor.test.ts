import { ethers, getUnnamedAccounts, helpers } from "hardhat"
import { expect } from "chai"

import type { RandomBeacon__factory, SortitionPool } from "../typechain"

const ZERO_ADDRESS = ethers.ZeroAddress

const { to1e18 } = helpers.number

describe("RandomBeacon - Constructor", () => {
  let sortitionPool: SortitionPool
  let tToken: string
  let staking: string
  let dkgValidator: string
  let reimbursementPool: string

  let RandomBeacon: RandomBeacon__factory

  before(async () => {
    ;[tToken, staking, dkgValidator, reimbursementPool] =
      await getUnnamedAccounts()

    // we need real SortitionPool contract instead of just string address
    // because BeaconDkg.currentState() is called from the RandomBeacon
    // constructor
    const SortitionPool = await ethers.getContractFactory("SortitionPool")
    sortitionPool = (await SortitionPool.deploy(
      tToken,
      to1e18(1),
    )) as SortitionPool

    const BLS = await ethers.getContractFactory("BLS")
    const bls = await BLS.deploy()
    await bls.waitForDeployment()

    const Authorization = await ethers.getContractFactory("BeaconAuthorization")
    const authorization = await Authorization.deploy()
    await authorization.waitForDeployment()

    const BeaconDkg = await ethers.getContractFactory("BeaconDkg")
    const dkg = await BeaconDkg.deploy()
    await dkg.waitForDeployment()

    const BeaconInactivity = await ethers.getContractFactory("BeaconInactivity")
    const inactivity = await BeaconInactivity.deploy()
    await inactivity.waitForDeployment()

    RandomBeacon = await ethers.getContractFactory("RandomBeacon", {
      libraries: {
        BLS: await bls.getAddress(),
        BeaconAuthorization: await authorization.getAddress(),
        BeaconDkg: await dkg.getAddress(),
        BeaconInactivity: await inactivity.getAddress(),
      },
    })
  })

  describe("constructor", () => {
    context("when all passed addresses are valid", () => {
      it("should work", async () => {
        await expect(
          RandomBeacon.deploy(
            await sortitionPool.getAddress(),
            tToken,
            staking,
            dkgValidator,
            reimbursementPool,
          ),
        ).not.to.be.reverted
      })
    })

    context("when sortition pool is 0-address", () => {
      it("should revert", async () => {
        await expect(
          RandomBeacon.deploy(
            ZERO_ADDRESS,
            tToken,
            staking,
            dkgValidator,
            reimbursementPool,
          ),
        ).to.be.revertedWith("Zero-address reference")
      })
    })

    context("when T token is 0-address", () => {
      it("should revert", async () => {
        await expect(
          RandomBeacon.deploy(
            await sortitionPool.getAddress(),
            ZERO_ADDRESS,
            staking,
            dkgValidator,
            reimbursementPool,
          ),
        ).to.be.revertedWith("Zero-address reference")
      })
    })

    context("when token staking is 0-address", () => {
      it("should revert", async () => {
        await expect(
          RandomBeacon.deploy(
            await sortitionPool.getAddress(),
            tToken,
            ZERO_ADDRESS,
            dkgValidator,
            reimbursementPool,
          ),
        ).to.be.revertedWith("Zero-address reference")
      })
    })

    context("when DKG validator is 0-address", () => {
      it("should revert", async () => {
        await expect(
          RandomBeacon.deploy(
            await sortitionPool.getAddress(),
            tToken,
            staking,
            ZERO_ADDRESS,
            reimbursementPool,
          ),
        ).to.be.revertedWith("Zero-address reference")
      })
    })

    context("when reimbursement pool is 0-address", () => {
      it("should revert", async () => {
        await expect(
          RandomBeacon.deploy(
            await sortitionPool.getAddress(),
            tToken,
            staking,
            dkgValidator,
            ZERO_ADDRESS,
          ),
        ).to.be.revertedWith("Zero-address reference")
      })
    })
  })
})
