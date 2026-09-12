"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const func = async (hre) => {
    const { getNamedAccounts, deployments } = hre;
    const { deployer } = await getNamedAccounts();
    const { execute } = deployments;
    const RandomBeacon = await deployments.get("RandomBeacon");
    await execute("ReimbursementPool", { from: deployer, log: true, waitConfirmations: 1 }, "authorize", RandomBeacon.address);
};
exports.default = func;
func.tags = ["RandomBeaconAuthorize"];
func.dependencies = ["ReimbursementPool", "RandomBeacon"];
