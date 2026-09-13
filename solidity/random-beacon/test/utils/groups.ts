import { toBeHex } from "ethers"
import { helpers, ethers } from "hardhat"

import requireResult from "../helpers/chain"
import { constants, params } from "../fixtures"
import blsData from "../data/bls"

// eslint-disable-next-line import/no-cycle
import { noMisbehaved, signAndSubmitArbitraryDkgResult } from "./dkg"

import type { BigNumberish } from "ethers"
import type { Operator } from "./operators"
import type { RandomBeacon, SortitionPool } from "../../typechain"

const { keccak256 } = ethers
const defaultAbiCoder = ethers.AbiCoder.defaultAbiCoder()
const { mineBlocks } = helpers.time

export async function createGroup(
  randomBeacon: RandomBeacon,
  signers: Operator[],
): Promise<void> {
  const { blockNumber: startBlock } = requireResult(
    await (await randomBeacon.genesis()).wait(),
  )

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
  seed: bigint,
): Promise<Operator[]> {
  // Copy the immutable ethers Result before passing these IDs to another call.
  const identifiers = Array.from(
    await sortitionPool.selectGroup(
      constants.groupSize,
      ethers.zeroPadValue(toBeHex(seed), 32),
    ),
  )
  const addresses = await sortitionPool.getIDOperators(identifiers)

  return Promise.all(
    identifiers.map(async (identifier, i): Promise<Operator> => ({
      id: Number(identifier),
      signer: await ethers.getSigner(addresses[i]),
    })),
  )
}

export function hashUint32Array(arrayToHash: BigNumberish[]): string {
  return keccak256(defaultAbiCoder.encode(["uint32[]"], [arrayToHash]))
}
