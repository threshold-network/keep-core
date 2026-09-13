import { createRequire } from "module"
import path from "path"

import resolveRandomBeaconExport from "../utils/random-beacon-export"

import type { InitializationTasks, TaskUtils } from "../types/random-beacon"

const requireBeaconTask = createRequire(
  path.join(resolveRandomBeaconExport("tasks"), "initialize.js"),
)

export const {
  TASK_INITIALIZE,
  TASK_INITIALIZE_STAKING,
  TASK_AUTHORIZE,
  TASK_REGISTER,
  TASK_ADD_BETA_OPERATOR,
}: InitializationTasks = requireBeaconTask("./initialize")

export const { authorize, register, addBetaOperator }: TaskUtils =
  requireBeaconTask("./utils")

requireBeaconTask("./unlock-eth-accounts")
