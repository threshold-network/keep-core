/**
 * Augments generated TokenStaking types with methods present on deployed TokenStaking
 * (see external/mainnet/TokenStaking.json) but absent from the compiled
 * @threshold-network/solidity-contracts TokenStaking.sol used for typechain.
 *
 * Kept outside `typechain/` so `yarn clean` does not remove it.
 */
import type { BigNumber, BigNumberish, ContractTransaction } from "ethers"
import type { CallOverrides } from "@ethersproject/contracts"

declare module "../typechain/TokenStaking" {
  export interface TokenStaking {
    stake(
      stakingProvider: string,
      beneficiary: string,
      authorizer: string,
      amount: BigNumberish,
      overrides?: CallOverrides
    ): Promise<ContractTransaction>

    increaseAuthorization(
      stakingProvider: string,
      application: string,
      amount: BigNumberish,
      overrides?: CallOverrides
    ): Promise<ContractTransaction>

    processSlashing(
      count: BigNumberish,
      overrides?: CallOverrides
    ): Promise<ContractTransaction>

    getSlashingQueueLength(overrides?: CallOverrides): Promise<BigNumber>

    slashingQueue(
      index: BigNumberish,
      overrides?: CallOverrides
    ): Promise<
      [string, BigNumber] & { stakingProvider: string; amount: BigNumber }
    >
  }
}
