import chai, { expect } from "chai"
import chaiAsPromised from "chai-as-promised"

import verifyOnEtherscanOrContinue from "../deploy/etherscanVerification"

import type { HardhatRuntimeEnvironment } from "hardhat/types"

chai.use(chaiAsPromised)

function fakeHre(networkName: string): {
  hre: HardhatRuntimeEnvironment
  logs: string[]
} {
  const logs: string[] = []
  const hre = {
    network: { name: networkName },
    deployments: {
      log: (msg: string) => {
        logs.push(msg)
      },
    },
  } as unknown as HardhatRuntimeEnvironment
  return { hre, logs }
}

describe("etherscanVerification - verifyOnEtherscanOrContinue", () => {
  it("returns normally when verify succeeds (sepolia)", async () => {
    const { hre, logs } = fakeHre("sepolia")
    let called = false
    await verifyOnEtherscanOrContinue(hre, async () => {
      called = true
    })
    expect(called).to.equal(true)
    expect(logs).to.have.length(0)
  })

  it("returns normally when verify succeeds (mainnet)", async () => {
    const { hre, logs } = fakeHre("mainnet")
    await verifyOnEtherscanOrContinue(hre, async () => {
      // no-op success
    })
    expect(logs).to.have.length(0)
  })

  it("swallows verify failure on non-mainnet networks", async () => {
    const { hre, logs } = fakeHre("sepolia")
    await verifyOnEtherscanOrContinue(hre, async () => {
      throw new Error("rate limited")
    })
    expect(logs).to.have.length(1)
    expect(logs[0]).to.match(/Etherscan verification skipped/)
    expect(logs[0]).to.match(/rate limited/)
  })

  it("swallows verify failure on hardhat / development networks", async () => {
    for (const name of ["hardhat", "development"]) {
      const { hre, logs } = fakeHre(name)
      await verifyOnEtherscanOrContinue(hre, async () => {
        throw new Error(`flake on ${name}`)
      })
      expect(logs).to.have.length(1)
      expect(logs[0]).to.match(new RegExp(`flake on ${name}`))
    }
  })

  it("rethrows verify failure on mainnet (no swallow)", async () => {
    const { hre, logs } = fakeHre("mainnet")
    const boom = new Error("bytecode mismatch")
    await expect(
      verifyOnEtherscanOrContinue(hre, async () => {
        throw boom
      })
    ).to.be.rejectedWith("bytecode mismatch")
    expect(logs).to.have.length(0)
  })
})
