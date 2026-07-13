"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
// ApplicationStatus enum: NOT_APPROVED=0, APPROVED=1, PAUSED=2, DISABLED=3
var APPLICATION_STATUS_APPROVED = 1;
function ifaceHasFunction(iface, name) {
    try {
        iface.getFunction(name);
        return true;
    }
    catch (_a) {
        return false;
    }
}
async function func(hre) {
    var deployer = (await hre.getNamedAccounts()).deployer;
    var execute = hre.deployments.execute, get = hre.deployments.get;
    var ethers = hre.ethers;
    var RandomBeacon = await hre.deployments.get("RandomBeacon");
    var TokenStaking = await get("TokenStaking");
    var iface = new ethers.utils.Interface(TokenStaking.abi);
    if (!ifaceHasFunction(iface, "approveApplication")) {
        hre.deployments.log("TokenStaking does not have approveApplication (Threshold TokenStaking); skipping");
        return;
    }
    try {
        var tokenStakingContract = await ethers.getContractAt(TokenStaking.abi, TokenStaking.address);
        if (ifaceHasFunction(iface, "applicationInfo")) {
            var appInfo = await tokenStakingContract.applicationInfo(RandomBeacon.address);
            if (appInfo.status === APPLICATION_STATUS_APPROVED) {
                hre.deployments.log("RandomBeacon already approved in TokenStaking; skipping");
                return;
            }
        }
    }
    catch (e) {
        hre.deployments.log("Could not read TokenStaking application status (continuing): ".concat(e));
    }
    await execute("TokenStaking", { from: deployer, log: true, waitConfirmations: 1 }, "approveApplication", RandomBeacon.address);
}
exports.default = func;
func.tags = ["RandomBeaconApprove"];
func.dependencies = ["TokenStaking", "RandomBeacon"];
// Skip for mainnet (already approved).
func.skip = async function (hre) { return hre.network.name === "mainnet"; };
