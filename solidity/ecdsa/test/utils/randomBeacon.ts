import { toBigInt } from "ethers"
import { ethers } from "hardhat"

import requireResult from "../helpers/chain"
import { createMock } from "../helpers/mock"

import type { WalletRegistry, IRandomBeacon } from "../../typechain"
import type { Mock } from "../helpers/mock"

export async function fakeRandomBeacon(
  walletRegistry: WalletRegistry,
): Promise<Mock<IRandomBeacon>> {
  const randomBeacon = await createMock<IRandomBeacon>("IRandomBeacon", {
    address: await walletRegistry.randomBeacon.staticCall(),
  })

  await (
    await ethers.getSigners()
  )[0].sendTransaction({
    to: randomBeacon.address,
    value: ethers.parseEther("1000"),
  })

  return randomBeacon
}

export async function submitRelayEntry(
  walletRegistry: WalletRegistry,
  randomBeacon?: Mock<IRandomBeacon>,
): Promise<{
  startBlock: number
  dkgSeed: bigint
}> {
  if (!randomBeacon) {
    // eslint-disable-next-line no-param-reassign
    randomBeacon = await fakeRandomBeacon(walletRegistry)
  }

  const relayEntry: bigint = toBigInt(ethers.randomBytes(32))

  // eslint-disable-next-line no-underscore-dangle
  const tx = await walletRegistry
    .connect(randomBeacon.wallet)
    .__beaconCallback(ethers.toBigInt(relayEntry), 0)

  return {
    startBlock: requireResult(await tx.wait()).blockNumber,
    dkgSeed: relayEntry,
  }
}
