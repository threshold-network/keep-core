import type { Provider, TransactionReceipt } from "ethers"

/** Wait through the transaction response; Hardhat's ethers v6 provider does not implement waitForTransaction. */
export default async function waitForConfirmations(
  provider: Pick<Provider, "getTransaction">,
  transactionHash: string,
  confirmations = 2,
  timeout = 300_000,
): Promise<TransactionReceipt> {
  const transaction = await provider.getTransaction(transactionHash)
  if (!transaction) {
    throw new Error(`Deployment transaction ${transactionHash} was not found`)
  }
  const receipt = await transaction.wait(confirmations, timeout)
  if (!receipt) {
    throw new Error(
      `Deployment transaction ${transactionHash} is not confirmed`,
    )
  }
  return receipt
}
