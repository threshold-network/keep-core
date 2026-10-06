import { getNumber } from "ethers"
import { task, types } from "hardhat/config"

import {
  calculateTokensNeededForStake,
  mint,
  stake,
  authorize,
  register,
  addBetaOperator,
} from "./utils"

// Main task executing all child tasks.
export const TASK_INITIALIZE = "initialize"

// Subtask for staking.
export const TASK_INITIALIZE_STAKING = `${TASK_INITIALIZE}:staking`
// Staking tasks.
export const TASK_MINT = "mint"
export const TASK_STAKE = "stake"
// Name prefix that should be used in tasks implementation for specific application.
export const TASK_AUTHORIZE = "authorize"
export const TASK_REGISTER = "register"
export const TASK_ADD_BETA_OPERATOR = "add_beta_operator"
// Subtask for the Random Beacon application.
const TASK_INITIALIZE_BEACON = `${TASK_INITIALIZE}:beacon`
const TASK_AUTHORIZE_BEACON = `${TASK_AUTHORIZE}:beacon`
const TASK_REGISTER_BEACON = `${TASK_REGISTER}:beacon`
const TASK_ADD_BETA_OPERATOR_BEACON = `${TASK_ADD_BETA_OPERATOR}:beacon`

task(
  TASK_INITIALIZE,
  "Initializes staking and the Random Beacon application for a staking provider and an operator",
)
  .addParam("owner", "Stake Owner address", undefined, types.string)
  .addParam("provider", "Staking Provider", undefined, types.string)
  .addParam("operator", "Staking Operator", undefined, types.string)
  .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, types.string)
  .addOptionalParam("authorizer", "Stake Authorizer", undefined, types.string)
  .addOptionalParam("amount", "Stake amount", 1_000_000, types.int)
  .addOptionalParam(
    "authorization",
    "Authorization amount (default: minimumAuthorization)",
    undefined,
    types.int,
  )
  .setAction(async (args, hre) => {
    // Initialize staking
    await hre.run(TASK_INITIALIZE_STAKING, args)
    // Initialize Beacon
    await hre.run(TASK_INITIALIZE_BEACON, args)
    // Set the operator as a beta operator
    await hre.run(TASK_ADD_BETA_OPERATOR_BEACON, args)
  })

task(TASK_INITIALIZE_STAKING, "Initializes staking for a service provider")
  .addParam("owner", "Stake Owner address", undefined, types.string)
  .addParam("provider", "Staking Provider", undefined, types.string)
  .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, types.string)
  .addOptionalParam("authorizer", "Stake Authorizer", undefined, types.string)
  .addOptionalParam("amount", "Stake amount", 1_000_000, types.int)
  .setAction(async (args, hre) => {
    const tokensToMint = await calculateTokensNeededForStake(
      hre,
      args.provider,
      args.amount,
    )

    if (tokensToMint !== 0n) {
      await hre.run(TASK_MINT, { ...args, amount: getNumber(tokensToMint) })
    }

    await hre.run(TASK_STAKE, args)
  })

task(TASK_MINT, "Mints T tokens")
  .addParam("owner", "Stake Owner address", undefined, types.string)
  .addOptionalParam("amount", "Stake amount", 1_000_000, types.int)
  .setAction(async (args, hre) => {
    await mint(hre, args.owner, args.amount)
  })

task(TASK_STAKE, "Stakes T tokens")
  .addParam("owner", "Stake Owner address", undefined, types.string)
  .addParam("provider", "Staking Provider", undefined, types.string)
  .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, types.string)
  .addOptionalParam("authorizer", "Stake Authorizer", undefined, types.string)
  .addOptionalParam("amount", "Stake amount", 1_000_000, types.int)
  .setAction(async (args, hre) => {
    await stake(
      hre,
      args.owner,
      args.provider,
      args.amount,
      args.beneficiary,
      args.authorizer,
    )
  })

task(TASK_INITIALIZE_BEACON, "Initializes operator for Beacon").setAction(
  async (args, hre) => {
    await hre.run(TASK_AUTHORIZE_BEACON, args)
    await hre.run(TASK_REGISTER_BEACON, args)
  },
)

task(TASK_AUTHORIZE_BEACON, "Sets authorization for Beacon")
  .addParam("owner", "Stake Owner address", undefined, types.string)
  .addParam("provider", "Staking Provider", undefined, types.string)
  .addOptionalParam("authorizer", "Stake Authorizer", undefined, types.string)
  .addOptionalParam(
    "authorization",
    "Authorization amount (default: minimumAuthorization)",
    undefined,
    types.int,
  )
  .setAction(async (args, hre) => {
    await authorize(
      hre,
      "RandomBeacon",
      args.owner,
      args.provider,
      args.authorizer,
      args.authorization,
    )
  })

task(
  TASK_REGISTER_BEACON,
  "Registers an operator for a staking provider in Beacon",
)
  .addParam("provider", "Staking Provider", undefined, types.string)
  .addParam("operator", "Operator Address", undefined, types.string)
  .setAction(async (args, hre) => {
    await register(hre, "RandomBeacon", args.provider, args.operator)
  })

task(
  TASK_ADD_BETA_OPERATOR_BEACON,
  "Adds an operator to the set of beta operators in Beacon",
)
  .addParam("operator", "Operator Address", undefined, types.string)
  .setAction(async (args, hre) => {
    await addBetaOperator(hre, "BeaconSortitionPool", args.operator)
  })
