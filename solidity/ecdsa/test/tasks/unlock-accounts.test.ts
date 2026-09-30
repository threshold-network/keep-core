import hre, { ethers } from "hardhat"
import { expect } from "chai"

import type { HttpNetworkConfig } from "hardhat/types"

describe("ECDSA account unlock task", () => {
  it("uses the ethers v6 provider and signer APIs on development", async () => {
    const account = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
    const requests: {
      method: string
      params: Array<unknown> | Record<string, unknown>
    }[] = []

    // Exercise the registered task without opening an RPC connection.
    class TestProvider extends ethers.JsonRpcProvider {
      private readonly rpcRequests = requests

      async listAccounts() {
        return [new ethers.JsonRpcSigner(this, account)]
      }

      async send(
        method: string,
        params: Array<unknown> | Record<string, unknown>,
      ) {
        this.rpcRequests.push({ method, params })
        return true
      }
    }

    const taskRuntime = {
      ...hre,
      ethers: { ...ethers, JsonRpcProvider: TestProvider },
      network: {
        ...hre.network,
        name: "development",
        config: { url: "http://unused.invalid" } as HttpNetworkConfig,
      },
    }
    const password = process.env.KEEP_ETHEREUM_PASSWORD
    process.env.KEEP_ETHEREUM_PASSWORD = "task-test-password"
    try {
      await hre.tasks["unlock-accounts"].action(
        {},
        taskRuntime,
        Object.assign(async () => undefined, { isDefined: false }),
      )
    } finally {
      if (password === undefined) {
        delete process.env.KEEP_ETHEREUM_PASSWORD
      } else {
        process.env.KEEP_ETHEREUM_PASSWORD = password
      }
    }

    expect(requests).to.deep.equal([
      {
        method: "personal_unlockAccount",
        params: [account.toLowerCase(), "task-test-password", 0],
      },
    ])
  })
})
