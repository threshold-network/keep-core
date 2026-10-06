import type { BigNumberish } from "ethers"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

// The resolved Beacon task exports are generated JavaScript. Keep this narrow
// interface in sync with the ethers v6 source in random-beacon/tasks.
export interface InitializationTasks {
  TASK_INITIALIZE: string
  TASK_INITIALIZE_STAKING: string
  TASK_AUTHORIZE: string
  TASK_REGISTER: string
  TASK_ADD_BETA_OPERATOR: string
}

export interface TaskUtils {
  authorize(
    hre: HardhatRuntimeEnvironment,
    deploymentName: string,
    owner: string,
    provider: string,
    authorizer?: string,
    authorization?: BigNumberish,
  ): Promise<void>

  register(
    hre: HardhatRuntimeEnvironment,
    deploymentName: string,
    provider: string,
    operator: string,
  ): Promise<void>

  addBetaOperator(
    hre: HardhatRuntimeEnvironment,
    sortitionPoolDeploymentName: string,
    operator: string,
  ): Promise<void>
}
