import { expect } from "chai"

import requireResult from "./chain"

import type { ContractTransactionResponse } from "ethers"

// TODO: Move to @keep-network/hardhat-helpers
// eslint-disable-next-line import/prefer-default-export
export async function assertGasUsed(
  tx: ContractTransactionResponse,
  expectedGasUsed: number,
  delta = 1000,
): Promise<void> {
  const receipt = requireResult(await tx.wait())
  expect(receipt.gasUsed, "invalid gas used").to.be.closeTo(
    BigInt(expectedGasUsed),
    delta,
  )
}
