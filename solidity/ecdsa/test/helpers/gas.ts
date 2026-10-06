import { expect } from "chai"

import requireResult from "./chain"

import type { ContractTransactionResponse } from "ethers"

// TODO: Move to @keep-network/hardhat-helpers
//
// The expected values passed here are measured baselines, not protocol limits: no
// block, DoS, or economic ceiling is documented for these operations. They exist as
// cost-regression tripwires. When a change moves gas, update the baseline
// deliberately and confirm the move was intended; do not widen `delta` to silence it.
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
