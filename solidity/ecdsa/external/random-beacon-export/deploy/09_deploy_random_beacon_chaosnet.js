"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const wait_for_confirmations_1 = __importDefault(require("../utils/wait-for-confirmations"));
const func = async (hre) => {
    const { getNamedAccounts, deployments, helpers } = hre;
    const { deployer } = await getNamedAccounts();
    const deployOptions = {
        from: deployer,
        log: true,
        waitConfirmations: 1,
    };
    const RandomBeaconChaosnet = await deployments.deploy("RandomBeaconChaosnet", {
        ...deployOptions,
    });
    if (hre.network.tags.etherscan) {
        if (RandomBeaconChaosnet.transactionHash) {
            await (0, wait_for_confirmations_1.default)(hre.ethers.provider, RandomBeaconChaosnet.transactionHash, 2, 300000);
        }
        await helpers.etherscan.verify(RandomBeaconChaosnet);
    }
    if (hre.network.tags.tenderly) {
        await hre.tenderly.verify({
            name: "RandomBeaconChaosnet",
            address: RandomBeaconChaosnet.address,
        });
    }
};
exports.default = func;
func.tags = ["RandomBeaconChaosnet"];
