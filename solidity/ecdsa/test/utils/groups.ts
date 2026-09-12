import { toBeHex } from "ethers"
import { ethers } from "hardhat"

import { constants } from "../fixtures"

import type { BigNumberish } from "ethers"
import type { Operator } from "./operators"
import type { SortitionPool } from "../../typechain"

const { keccak256 } = ethers
const defaultAbiCoder = ethers.AbiCoder.defaultAbiCoder()

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
