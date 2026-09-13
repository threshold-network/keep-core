"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const wait_for_confirmations_1 = __importDefault(require("../utils/wait-for-confirmations"));
const func = async (hre) => {
    const { getNamedAccounts, deployments, helpers } = hre;
    const { deployer, chaosnetOwner } = await getNamedAccounts();
    const { execute } = deployments;
    const { to1e18 } = helpers.number;
    const POOL_WEIGHT_DIVISOR = to1e18(1);
    const T = await deployments.get("T");
    const BeaconSortitionPool = await deployments.deploy("BeaconSortitionPool", {
        contract: "SortitionPool",
        from: deployer,
        args: [T.address, POOL_WEIGHT_DIVISOR],
        log: true,
        waitConfirmations: 1,
    });
    await execute("BeaconSortitionPool", { from: deployer, log: true, waitConfirmations: 1 }, "transferChaosnetOwnerRole", chaosnetOwner);
    if (hre.network.tags.etherscan) {
        if (BeaconSortitionPool.transactionHash) {
            await (0, wait_for_confirmations_1.default)(hre.ethers.provider, BeaconSortitionPool.transactionHash, 2, 300000);
        }
        await helpers.etherscan.verify(BeaconSortitionPool);
    }
    if (hre.network.tags.tenderly) {
        await hre.tenderly.verify({
            name: "BeaconSortitionPool",
            address: BeaconSortitionPool.address,
        });
    }
};
exports.default = func;
func.tags = ["BeaconSortitionPool"];
// TokenStaking and T deployments are expected to be resolved from
// @threshold-network/solidity-contracts
func.dependencies = ["TokenStaking", "T"];
