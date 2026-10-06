import { expect } from "chai"

import type { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers"
import type { RandomBeacon } from "../../typechain"

const MAX_UINT64 = BigInt("18446744073709551615") // 2^64 - 1

export interface OverwritePreviousRequestContext {
  randomBeacon: RandomBeacon
  stakingProvider: SignerWithAddress
  deauthorizingSecond: bigint
}

export interface RequireUpdatingPoolContext {
  randomBeacon: RandomBeacon
  stakingProvider: SignerWithAddress
}

export function shouldOverwritePreviousRequest(
  getContext: () => OverwritePreviousRequestContext,
): void {
  it("should overwrite the previous request", async () => {
    const { randomBeacon, stakingProvider, deauthorizingSecond } = getContext()
    expect(
      await randomBeacon.pendingAuthorizationDecrease(stakingProvider.address),
    ).to.be.equal(deauthorizingSecond)
  })
}

export function shouldRequireUpdatingPoolBeforeApproving(
  getContext: () => RequireUpdatingPoolContext,
): void {
  it("should require updating the pool before approving", async () => {
    const { randomBeacon, stakingProvider } = getContext()
    expect(
      await randomBeacon.remainingAuthorizationDecreaseDelay(
        stakingProvider.address,
      ),
    ).to.equal(MAX_UINT64)
  })
}
