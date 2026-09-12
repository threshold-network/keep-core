import type { Provider, TransactionReceipt } from "ethers"

/** Wait through the transaction response; Hardhat's ethers v6 provider does not implement waitForTransaction. */
export default async function waitForConfirmations(
  provider: Pick<Provider, "getTransaction">,
  transactionHash: string,
  confirmations = 2,
  timeout = 300_000,
): Promise<TransactionReceipt> {
  const pollInterval = 2_000
  const deadline = Date.now() + timeout
  // ethers v5 waitForTransaction polled, so a load-balanced endpoint that does
  // not see the just-mined transaction yet must not fail the deployment.
  let transaction = await provider.getTransaction(transactionHash)
  while (!transaction) {
    const remaining = deadline - Date.now()
    if (remaining <= 0) {
      throw new Error(`Deployment transaction ${transactionHash} was not found`)
    }
    await new Promise((resolve) => {
      setTimeout(resolve, Math.min(pollInterval, remaining))
    })
    transaction = await provider.getTransaction(transactionHash)
  }
  const receipt = await transaction.wait(
    confirmations,
    Math.max(deadline - Date.now(), 1),
  )
  if (!receipt) {
    throw new Error(
      `Deployment transaction ${transactionHash} is not confirmed`,
    )
  }
  return receipt
}
