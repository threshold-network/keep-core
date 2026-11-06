import { ethers, helpers } from "hardhat"
import { expect } from "chai"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { ReimbursableImplStub } from "../typechain"

describe("Reimbursable", () => {
  let reimbursableImplStub: ReimbursableImplStub
  let deployer: SignerWithAddress
  let admin: SignerWithAddress
  let thirdParty: SignerWithAddress
  let contractToUpdate: SignerWithAddress

  before(async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ deployer } = await helpers.signers.getNamedSigners())

    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;[admin, thirdParty, contractToUpdate] =
      await helpers.signers.getUnnamedSigners()

    const ReimbursableImplStub = await ethers.getContractFactory(
      "ReimbursableImplStub",
      deployer
    )
    reimbursableImplStub = (await ReimbursableImplStub.deploy(
      admin.address
    )) as ReimbursableImplStub
  })

  describe("gas measurement after state-gas credits", () => {
    it("clamps a reading larger than the starting gas to zero", async () => {
      expect(await reimbursableImplStub.gasSpent(0)).to.equal(0)
    })

    it("keeps a positive execution-gas delta", async () => {
      const spent = await reimbursableImplStub.gasSpent(1_000_000, {
        gasLimit: 100_000,
      })
      expect(spent).to.be.gt(900_000)
      expect(spent).to.be.lt(1_000_000)
    })

    it("clamps a negative offset larger than the measured gas", async () => {
      expect(await reimbursableImplStub.gasSpentAfterOffset(0, 2_300)).to.equal(
        0
      )
      expect(
        await reimbursableImplStub.gasSpentAfterOffset(1_000_000, 1_000_000, {
          gasLimit: 100_000,
        })
      ).to.equal(0)
    })
  })

  describe("updateReimbursementPool", () => {
    context("when a caller is the deployer", () => {
      it("should revert", async () => {
        await expect(
          reimbursableImplStub
            .connect(deployer)
            .updateReimbursementPool(contractToUpdate.address)
        ).to.be.revertedWith("Caller is not the admin")
      })
    })

    context("when a caller is not the admin", () => {
      it("should revert", async () => {
        await expect(
          reimbursableImplStub
            .connect(thirdParty)
            .updateReimbursementPool(contractToUpdate.address)
        ).to.be.revertedWith("Caller is not the admin")
      })
    })

    context("when a caller is the admin", () => {
      it("should update a reimbursement contract", async () => {
        await reimbursableImplStub
          .connect(admin)
          .updateReimbursementPool(contractToUpdate.address)

        expect(await reimbursableImplStub.reimbursementPool()).to.be.equal(
          contractToUpdate.address
        )
      })

      it("should emit ReimbursementPoolUpdated event", async () => {
        await expect(
          reimbursableImplStub
            .connect(admin)
            .updateReimbursementPool(contractToUpdate.address)
        )
          .to.emit(reimbursableImplStub, "ReimbursementPoolUpdated")
          .withArgs(contractToUpdate.address)
      })
    })
  })
})
