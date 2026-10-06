"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.TASK_ADD_BETA_OPERATOR = exports.TASK_REGISTER = exports.TASK_AUTHORIZE = exports.TASK_STAKE = exports.TASK_MINT = exports.TASK_INITIALIZE_STAKING = exports.TASK_INITIALIZE = void 0;
const ethers_1 = require("ethers");
const config_1 = require("hardhat/config");
const utils_1 = require("./utils");
// Main task executing all child tasks.
exports.TASK_INITIALIZE = "initialize";
// Subtask for staking.
exports.TASK_INITIALIZE_STAKING = `${exports.TASK_INITIALIZE}:staking`;
// Staking tasks.
exports.TASK_MINT = "mint";
exports.TASK_STAKE = "stake";
// Name prefix that should be used in tasks implementation for specific application.
exports.TASK_AUTHORIZE = "authorize";
exports.TASK_REGISTER = "register";
exports.TASK_ADD_BETA_OPERATOR = "add_beta_operator";
// Subtask for the Random Beacon application.
const TASK_INITIALIZE_BEACON = `${exports.TASK_INITIALIZE}:beacon`;
const TASK_AUTHORIZE_BEACON = `${exports.TASK_AUTHORIZE}:beacon`;
const TASK_REGISTER_BEACON = `${exports.TASK_REGISTER}:beacon`;
const TASK_ADD_BETA_OPERATOR_BEACON = `${exports.TASK_ADD_BETA_OPERATOR}:beacon`;
(0, config_1.task)(exports.TASK_INITIALIZE, "Initializes staking and the Random Beacon application for a staking provider and an operator")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addParam("operator", "Staking Operator", undefined, config_1.types.string)
    .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .addOptionalParam("authorization", "Authorization amount (default: minimumAuthorization)", undefined, config_1.types.int)
    .setAction(async (args, hre) => {
    // Initialize staking
    await hre.run(exports.TASK_INITIALIZE_STAKING, args);
    // Initialize Beacon
    await hre.run(TASK_INITIALIZE_BEACON, args);
    // Set the operator as a beta operator
    await hre.run(TASK_ADD_BETA_OPERATOR_BEACON, args);
});
(0, config_1.task)(exports.TASK_INITIALIZE_STAKING, "Initializes staking for a service provider")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .setAction(async (args, hre) => {
    const tokensToMint = await (0, utils_1.calculateTokensNeededForStake)(hre, args.provider, args.amount);
    if (tokensToMint !== 0n) {
        await hre.run(exports.TASK_MINT, { ...args, amount: (0, ethers_1.getNumber)(tokensToMint) });
    }
    await hre.run(exports.TASK_STAKE, args);
});
(0, config_1.task)(exports.TASK_MINT, "Mints T tokens")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .setAction(async (args, hre) => {
    await (0, utils_1.mint)(hre, args.owner, args.amount);
});
(0, config_1.task)(exports.TASK_STAKE, "Stakes T tokens")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .setAction(async (args, hre) => {
    await (0, utils_1.stake)(hre, args.owner, args.provider, args.amount, args.beneficiary, args.authorizer);
});
(0, config_1.task)(TASK_INITIALIZE_BEACON, "Initializes operator for Beacon").setAction(async (args, hre) => {
    await hre.run(TASK_AUTHORIZE_BEACON, args);
    await hre.run(TASK_REGISTER_BEACON, args);
});
(0, config_1.task)(TASK_AUTHORIZE_BEACON, "Sets authorization for Beacon")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("authorization", "Authorization amount (default: minimumAuthorization)", undefined, config_1.types.int)
    .setAction(async (args, hre) => {
    await (0, utils_1.authorize)(hre, "RandomBeacon", args.owner, args.provider, args.authorizer, args.authorization);
});
(0, config_1.task)(TASK_REGISTER_BEACON, "Registers an operator for a staking provider in Beacon")
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addParam("operator", "Operator Address", undefined, config_1.types.string)
    .setAction(async (args, hre) => {
    await (0, utils_1.register)(hre, "RandomBeacon", args.provider, args.operator);
});
(0, config_1.task)(TASK_ADD_BETA_OPERATOR_BEACON, "Adds an operator to the set of beta operators in Beacon")
    .addParam("operator", "Operator Address", undefined, config_1.types.string)
    .setAction(async (args, hre) => {
    await (0, utils_1.addBetaOperator)(hre, "BeaconSortitionPool", args.operator);
});
