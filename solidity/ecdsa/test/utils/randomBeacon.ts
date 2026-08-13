import { ethers } from "hardhat"

import { createMock } from "../helpers/mock"

import type { BigNumber } from "ethers"
import type { WalletRegistry, IRandomBeacon } from "../../typechain"
import type { Mock } from "../helpers/mock"

export async function fakeRandomBeacon(
  walletRegistry: WalletRegistry
): Promise<Mock<IRandomBeacon>> {
  const randomBeacon = await createMock<IRandomBeacon>("IRandomBeacon", {
    address: await walletRegistry.callStatic.randomBeacon(),
  })

  await (
    await ethers.getSigners()
  )[0].sendTransaction({
    to: randomBeacon.address,
    value: ethers.utils.parseEther("1000"),
  })

  return randomBeacon
}

export async function submitRelayEntry(
  walletRegistry: WalletRegistry,
  randomBeacon?: Mock<IRandomBeacon>
): Promise<{
  startBlock: number
  dkgSeed: BigNumber
}> {
  if (!randomBeacon) {
    // eslint-disable-next-line no-param-reassign
    randomBeacon = await fakeRandomBeacon(walletRegistry)
  }

  const relayEntry: BigNumber = ethers.BigNumber.from(
    ethers.utils.randomBytes(32)
  )

  // eslint-disable-next-line no-underscore-dangle
  const tx = await walletRegistry
    .connect(randomBeacon.wallet)
    .__beaconCallback(relayEntry, 0)

  return {
    startBlock: tx.blockNumber,
    dkgSeed: relayEntry,
  }
}
