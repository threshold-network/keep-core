import { expect } from "chai"

import type { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers"
import type { WalletRegistry } from "../../typechain"

const MAX_UINT64 = BigInt("18446744073709551615") // 2^64 - 1

export interface OverwritePreviousRequestContext {
  walletRegistry: WalletRegistry
  stakingProvider: SignerWithAddress
  deauthorizingSecond: bigint
}

export interface RequireUpdatingPoolContext {
  walletRegistry: WalletRegistry
  stakingProvider: SignerWithAddress
}

export function shouldOverwritePreviousRequest(
  getContext: () => OverwritePreviousRequestContext,
): void {
  it("should overwrite the previous request", async () => {
    const { walletRegistry, stakingProvider, deauthorizingSecond } =
      getContext()
    expect(
      await walletRegistry.pendingAuthorizationDecrease(
        stakingProvider.address,
      ),
    ).to.be.equal(deauthorizingSecond)
  })
}

export function shouldRequireUpdatingPoolBeforeApproving(
  getContext: () => RequireUpdatingPoolContext,
): void {
  it("should require updating the pool before approving", async () => {
    const { walletRegistry, stakingProvider } = getContext()
    expect(
      await walletRegistry.remainingAuthorizationDecreaseDelay(
        stakingProvider.address,
      ),
    ).to.equal(MAX_UINT64)
  })
}
