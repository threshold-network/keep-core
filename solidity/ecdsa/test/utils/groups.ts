import { ethers } from "hardhat"

import { constants } from "../fixtures"

import type { BigNumber, BigNumberish } from "ethers"
import type { Operator } from "./operators"
import type { SortitionPool } from "../../typechain"

const { keccak256, defaultAbiCoder } = ethers.utils

export async function selectGroup(
  sortitionPool: SortitionPool,
  seed: BigNumber
): Promise<Operator[]> {
  const identifiers = await sortitionPool.selectGroup(
    constants.groupSize,
    ethers.utils.hexZeroPad(seed.toHexString(), 32)
  )

  const addresses = await sortitionPool.getIDOperators(identifiers)

  return Promise.all(
    identifiers.map(
      async (identifier, i): Promise<Operator> => ({
        id: identifier,
        signer: await ethers.getSigner(addresses[i]),
      })
    )
  )
}

export function hashUint32Array(arrayToHash: BigNumberish[]): string {
  return keccak256(defaultAbiCoder.encode(["uint32[]"], [arrayToHash]))
}
