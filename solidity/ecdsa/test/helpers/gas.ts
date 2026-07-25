import { expect } from "chai"
import { ethers } from "hardhat"

import type { ContractTransaction } from "ethers"

const { BigNumber } = ethers

// TODO: Move to @keep-network/hardhat-helpers
// eslint-disable-next-line import/prefer-default-export
export async function assertGasUsed(
  tx: ContractTransaction,
  expectedGasUsed: number,
  delta = 1000
): Promise<void> {
  expect((await tx.wait()).gasUsed, "invalid gas used").to.be.closeTo(
    BigNumber.from(expectedGasUsed),
    delta
  )
}
