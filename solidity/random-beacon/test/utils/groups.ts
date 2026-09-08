import { helpers, ethers } from "hardhat"

import { constants, params } from "../fixtures"
import blsData from "../data/bls"

// eslint-disable-next-line import/no-cycle
import { noMisbehaved, signAndSubmitArbitraryDkgResult } from "./dkg"

import type { BigNumber, BigNumberish } from "ethers"
import type { Operator } from "./operators"
import type { RandomBeacon, SortitionPool } from "../../typechain"

const { keccak256, defaultAbiCoder } = ethers.utils
const { mineBlocks } = helpers.time

export async function createGroup(
  randomBeacon: RandomBeacon,
  signers: Operator[],
): Promise<void> {
  const { blockNumber: startBlock } = await (
    await randomBeacon.genesis()
  ).wait()

  await mineBlocks(constants.offchainDkgTime)

  const { dkgResult, submitter } = await signAndSubmitArbitraryDkgResult(
    randomBeacon,
    blsData.groupPubKey,
    signers,
    startBlock,
    noMisbehaved,
  )

  await mineBlocks(params.dkgResultChallengePeriodLength)

  await randomBeacon.connect(submitter).approveDkgResult(dkgResult)
}

export async function selectGroup(
  sortitionPool: SortitionPool,
  seed: BigNumber,
): Promise<Operator[]> {
  const identifiers = await sortitionPool.selectGroup(
    constants.groupSize,
    ethers.utils.hexZeroPad(seed.toHexString(), 32),
  )
  const addresses = await sortitionPool.getIDOperators(identifiers)

  return Promise.all(
    identifiers.map(async (identifier, i): Promise<Operator> => ({
      id: identifier,
      signer: await ethers.getSigner(addresses[i]),
    })),
  )
}

export function hashUint32Array(arrayToHash: BigNumberish[]): string {
  return keccak256(defaultAbiCoder.encode(["uint32[]"], [arrayToHash]))
}
