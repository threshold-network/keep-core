"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const wait_for_confirmations_1 = __importDefault(require("../utils/wait-for-confirmations"));
const func = async (hre) => {
    const { getNamedAccounts, deployments, helpers } = hre;
    const { deployer } = await getNamedAccounts();
    const RandomBeacon = await deployments.get("RandomBeacon");
    const GOVERNANCE_DELAY = 604800; // 1 week
    const RandomBeaconGovernance = await deployments.deploy("RandomBeaconGovernance", {
        from: deployer,
        args: [RandomBeacon.address, GOVERNANCE_DELAY],
        log: true,
        waitConfirmations: 1,
    });
    if (hre.network.tags.etherscan) {
        if (RandomBeaconGovernance.transactionHash) {
            await (0, wait_for_confirmations_1.default)(hre.ethers.provider, RandomBeaconGovernance.transactionHash, 2, 300000);
        }
        await helpers.etherscan.verify(RandomBeaconGovernance);
    }
    if (hre.network.tags.tenderly) {
        await hre.tenderly.verify({
            name: "RandomBeaconGovernance",
            address: RandomBeaconGovernance.address,
        });
    }
};
exports.default = func;
func.tags = ["RandomBeaconGovernance"];
func.dependencies = ["RandomBeacon"];
