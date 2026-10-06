"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const wait_for_confirmations_1 = __importDefault(require("../utils/wait-for-confirmations"));
const func = async (hre) => {
    const { getNamedAccounts, deployments, helpers } = hre;
    const { deployer } = await getNamedAccounts();
    const BeaconSortitionPool = await deployments.get("BeaconSortitionPool");
    const BeaconDkgValidator = await deployments.deploy("BeaconDkgValidator", {
        from: deployer,
        args: [BeaconSortitionPool.address],
        log: true,
        waitConfirmations: 1,
    });
    if (hre.network.tags.etherscan) {
        if (BeaconDkgValidator.transactionHash) {
            await (0, wait_for_confirmations_1.default)(hre.ethers.provider, BeaconDkgValidator.transactionHash, 2, 300000);
        }
        await helpers.etherscan.verify(BeaconDkgValidator);
    }
    if (hre.network.tags.tenderly) {
        await hre.tenderly.verify({
            name: "BeaconDkgValidator",
            address: BeaconDkgValidator.address,
        });
    }
};
exports.default = func;
func.tags = ["BeaconDkgValidator"];
func.dependencies = ["BeaconSortitionPool"];
