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
    ;({ deployer } = await helpers.signers.getNamedSigners())
    ;[admin, thirdParty, contractToUpdate] =
      await helpers.signers.getUnnamedSigners()

    const ReimbursableImplStub = await ethers.getContractFactory(
      "ReimbursableImplStub",
      deployer,
    )
    reimbursableImplStub = (await ReimbursableImplStub.deploy(
      admin.address,
    )) as ReimbursableImplStub
  })

  describe("updateReimbursementPool", () => {
    context("when a caller is the deployer", () => {
      it("should revert", async () => {
        await expect(
          reimbursableImplStub
            .connect(deployer)
            .updateReimbursementPool(contractToUpdate.address),
        ).to.be.revertedWith("Caller is not the admin")
      })
    })

    context("when a caller is not the admin", () => {
      it("should revert", async () => {
        await expect(
          reimbursableImplStub
            .connect(thirdParty)
            .updateReimbursementPool(contractToUpdate.address),
        ).to.be.revertedWith("Caller is not the admin")
      })
    })

    context("when a caller is the admin", () => {
      it("should update a reimbursement contract", async () => {
        await reimbursableImplStub
          .connect(admin)
          .updateReimbursementPool(contractToUpdate.address)

        expect(await reimbursableImplStub.reimbursementPool()).to.be.equal(
          contractToUpdate.address,
        )
      })

      it("should emit ReimbursementPoolUpdated event", async () => {
        await expect(
          reimbursableImplStub
            .connect(admin)
            .updateReimbursementPool(contractToUpdate.address),
        )
          .to.emit(reimbursableImplStub, "ReimbursementPoolUpdated")
          .withArgs(contractToUpdate.address)
      })
    })
  })
})
