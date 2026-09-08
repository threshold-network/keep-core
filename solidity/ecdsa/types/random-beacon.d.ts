// The published ethers-v5 Random Beacon package does not include declarations
// for these task exports. Keep this narrow compatibility surface in sync with
// random-beacon/tasks until its published package supplies the declarations.
declare module "@keep-network/random-beacon/export/tasks/initialize" {
  export const TASK_INITIALIZE: string
  export const TASK_INITIALIZE_STAKING: string
  export const TASK_AUTHORIZE: string
  export const TASK_REGISTER: string
  export const TASK_ADD_BETA_OPERATOR: string
}

declare module "@keep-network/random-beacon/export/tasks/utils" {
  import type { BigNumberish } from "ethers"
  import type { HardhatRuntimeEnvironment } from "hardhat/types"

  export function authorize(
    hre: HardhatRuntimeEnvironment,
    deploymentName: string,
    owner: string,
    provider: string,
    authorizer?: string,
    authorization?: BigNumberish
  ): Promise<void>

  export function register(
    hre: HardhatRuntimeEnvironment,
    deploymentName: string,
    provider: string,
    operator: string
  ): Promise<void>

  export function addBetaOperator(
    hre: HardhatRuntimeEnvironment,
    sortitionPoolDeploymentName: string,
    operator: string
  ): Promise<void>
}
