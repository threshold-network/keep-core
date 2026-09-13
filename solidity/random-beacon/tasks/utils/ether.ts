import type { HardhatRuntimeEnvironment } from "hardhat/types"

// eslint-disable-next-line import/prefer-default-export
export function parseValue(
  value: string,
  hre: HardhatRuntimeEnvironment,
): bigint {
  const parsed = String(value).trim().split(" ")

  if (parsed.length === 0 || parsed.length > 2) {
    throw new Error(`invalid value: ${value}`)
  }

  return hre.ethers.parseUnits(parsed[0], parsed[1] || "wei")
}
