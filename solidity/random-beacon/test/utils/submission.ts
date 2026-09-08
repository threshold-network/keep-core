import { BigNumber } from "ethers"

import { constants } from "../fixtures"

import type { BigNumberish } from "ethers"

export function firstEligibleIndex(
  seed: BigNumberish,
  groupSize?: number
): number {
  // eslint-disable-next-line no-param-reassign
  if (!groupSize) groupSize = constants.groupSize

  return BigNumber.from(seed).mod(groupSize).add(1).toNumber()
}

export function shiftEligibleIndex(
  // eslint-disable-next-line @typescript-eslint/no-shadow
  firstEligibleIndex: number,
  shift: number,
  groupSize?: number
): number {
  // eslint-disable-next-line no-param-reassign
  if (!groupSize) groupSize = constants.groupSize

  const result = firstEligibleIndex + shift

  return result > groupSize ? result - groupSize : result
}
