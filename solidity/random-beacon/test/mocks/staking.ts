import { createMock } from "../helpers/mock"

import type { Mock } from "../helpers/mock"
import type { RandomBeacon, TokenStaking } from "../../typechain"

// eslint-disable-next-line import/prefer-default-export
export async function fakeTokenStaking(
  randomBeacon: RandomBeacon
): Promise<Mock<TokenStaking>> {
  const tokenStaking = await createMock<TokenStaking>("TokenStaking", {
    address: await randomBeacon.callStatic.staking(),
  })

  return tokenStaking
}
