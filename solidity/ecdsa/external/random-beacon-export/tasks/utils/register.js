"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.register = register;
// eslint-disable-next-line import/prefer-default-export
async function register(hre, deploymentName, provider, operator) {
    const { ethers, helpers } = hre;
    const providerAddress = ethers.getAddress(provider);
    const operatorAddress = ethers.getAddress(operator);
    const application = await helpers.contracts.getContract(deploymentName);
    console.log(`Registering operator ${operatorAddress} in ${deploymentName} application (${await application.getAddress()})`);
    const currentProvider = ethers.getAddress(await application.operatorToStakingProvider.staticCall(operatorAddress));
    switch (currentProvider) {
        case providerAddress: {
            console.log(`Current staking provider for operator ${operatorAddress} is ${currentProvider}`);
            return;
        }
        case ethers.ZeroAddress: {
            console.log(`Registering operator ${operatorAddress} for a staking provider ${providerAddress}...`);
            await (await application
                .connect(await ethers.getSigner(providerAddress))
                .getFunction("registerOperator")(operatorAddress)).wait();
            break;
        }
        default: {
            throw new Error(`Operator [${operatorAddress}] has already been registered for another staking provider [${currentProvider}]`);
        }
    }
}
