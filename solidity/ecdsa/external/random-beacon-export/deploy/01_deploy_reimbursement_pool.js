"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const wait_for_confirmations_1 = __importDefault(require("../utils/wait-for-confirmations"));
const func = async (hre) => {
    const { getNamedAccounts, deployments, helpers } = hre;
    const { deployer } = await getNamedAccounts();
    const staticGas = 40800; // gas amount consumed by the refund() + tx cost
    const maxGasPrice = 500000000000; // 500 Gwei
    const ReimbursementPool = await deployments.deploy("ReimbursementPool", {
        from: deployer,
        args: [staticGas, maxGasPrice],
        log: true,
        waitConfirmations: 1,
    });
    if (hre.network.tags.etherscan) {
        if (ReimbursementPool.transactionHash) {
            await (0, wait_for_confirmations_1.default)(hre.ethers.provider, ReimbursementPool.transactionHash, 2, 300000);
        }
        await helpers.etherscan.verify(ReimbursementPool);
    }
    if (hre.network.tags.tenderly) {
        await hre.tenderly.verify({
            name: "ReimbursementPool",
            address: ReimbursementPool.address,
        });
    }
};
exports.default = func;
func.tags = ["ReimbursementPool"];
