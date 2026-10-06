"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const func = async (hre) => {
    const { getNamedAccounts, deployments, helpers } = hre;
    const { deployer, governance } = await getNamedAccounts();
    const RandomBeaconGovernance = await deployments.get("RandomBeaconGovernance");
    await helpers.ownable.transferOwnership("RandomBeaconGovernance", governance, deployer);
    await deployments.execute("RandomBeacon", { from: deployer, log: true, waitConfirmations: 1 }, "transferGovernance", RandomBeaconGovernance.address);
};
exports.default = func;
func.tags = ["RandomBeaconTransferGovernance"];
func.dependencies = ["RandomBeaconGovernance"];
