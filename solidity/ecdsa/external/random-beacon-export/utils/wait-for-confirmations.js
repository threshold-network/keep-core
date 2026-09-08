"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.default = waitForConfirmations;
/** Wait through the transaction response; Hardhat's ethers v6 provider does not implement waitForTransaction. */
async function waitForConfirmations(provider, transactionHash, confirmations = 2, timeout = 300000) {
    const transaction = await provider.getTransaction(transactionHash);
    if (!transaction) {
        throw new Error(`Deployment transaction ${transactionHash} was not found`);
    }
    const receipt = await transaction.wait(confirmations, timeout);
    if (!receipt) {
        throw new Error(`Deployment transaction ${transactionHash} is not confirmed`);
    }
    return receipt;
}
