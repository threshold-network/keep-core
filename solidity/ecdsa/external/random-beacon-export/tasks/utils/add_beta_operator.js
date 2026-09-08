"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.addBetaOperator = addBetaOperator;
// eslint-disable-next-line import/prefer-default-export
async function addBetaOperator(hre, sortitionPoolDeploymentName, operator) {
    const { ethers, helpers } = hre;
    const sortitionPool = await helpers.contracts.getContract(sortitionPoolDeploymentName);
    const chaosnetOwner = await sortitionPool.chaosnetOwner();
    console.log(`Adding ${operator} to the set of beta operators...`);
    await (await sortitionPool
        .connect(await ethers.getSigner(chaosnetOwner))
        .getFunction("addBetaOperators")([operator])).wait();
}
