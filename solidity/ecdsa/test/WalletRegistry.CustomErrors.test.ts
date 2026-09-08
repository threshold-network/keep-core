/* eslint-disable @typescript-eslint/no-unused-expressions */
import { ethers, helpers } from "hardhat"
import { expect } from "chai"

import { constants, walletRegistryFixture } from "./fixtures"
import ecdsaData from "./data/ecdsa"
import { hashUint32Array, selectGroup } from "./utils/groups"
import { submitRelayEntry } from "./utils/randomBeacon"
import { noMisbehaved, signAndSubmitUnrecoverableDkgResult } from "./utils/dkg"

import type {
  IWalletOwner,
  WalletRegistry,
  WalletRegistryStub,
  WalletRegistryGovernance,
} from "../typechain"
import type { Mock } from "./helpers/mock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"

const { to1e18 } = helpers.number
const { createSnapshot, restoreSnapshot } = helpers.snapshot
const ZERO_ADDRESS = ethers.constants.AddressZero

describe("WalletRegistry - Custom Errors", () => {
  let walletRegistry: WalletRegistry & WalletRegistryStub
  let walletRegistryGovernance: WalletRegistryGovernance
  let governance: SignerWithAddress
  let deployer: SignerWithAddress
  let unauthorized: SignerWithAddress
  let stakingProvider: SignerWithAddress
  let operator: SignerWithAddress
  let walletOwner: Mock<IWalletOwner>
  let membersIDs: number[]
  const walletPublicKey = ecdsaData.group1.publicKey
  const walletID = ethers.utils.keccak256(walletPublicKey)

  before("load Allowlist fixture and register a wallet", async () => {
    const fixture = await walletRegistryFixture({ useAllowlist: true })
    ;({
      walletRegistry,
      walletRegistryGovernance,
      governance,
      deployer,
      walletOwner,
      thirdParty: unauthorized,
    } = fixture)
    operator = fixture.operators[0].signer
    stakingProvider = fixture.operators[0].stakingProvider
    membersIDs = fixture.operators.slice(0, 3).map(({ id }) => id)
    await walletRegistry.forceAddWallet(
      walletPublicKey,
      hashUint32Array(membersIDs)
    )
  })

  beforeEach(async () => {
    await createSnapshot()
  })

  afterEach(async () => {
    await restoreSnapshot()
  })

  describe("CallerNotStakingContract", () => {
    const callbacks = [
      "authorizationIncreased",
      "authorizationDecreaseRequested",
      "involuntaryAuthorizationDecrease",
    ] as const
    callbacks.forEach((method) => {
      it(`should reject ${method} from an unauthorized caller`, async () => {
        await expect(
          walletRegistry
            .connect(unauthorized)
            [method](stakingProvider.address, to1e18(50000), to1e18(40000))
        ).to.be.revertedWithCustomError(
          walletRegistry,
          "CallerNotStakingContract"
        )
      })
    })
  })

  describe("CallerNotWalletOwner", () => {
    it("should reject requesting a wallet", async () => {
      await expect(
        walletRegistry.connect(unauthorized).requestNewWallet()
      ).to.be.revertedWithCustomError(walletRegistry, "CallerNotWalletOwner")
    })

    it("should reject closing a wallet", async () => {
      await expect(
        walletRegistry.connect(unauthorized).closeWallet(walletID)
      ).to.be.revertedWithCustomError(walletRegistry, "CallerNotWalletOwner")
    })

    it("should reject seizing wallet members", async () => {
      await expect(
        walletRegistry
          .connect(unauthorized)
          .seize(to1e18(1000), 100, unauthorized.address, walletID, membersIDs)
      ).to.be.revertedWithCustomError(walletRegistry, "CallerNotWalletOwner")
    })
  })

  describe("CallerNotGovernance", () => {
    it("should reject updating DKG parameters", async () => {
      await expect(
        walletRegistry
          .connect(unauthorized)
          .updateDkgParameters(100, 100, 50000, 100, 10)
      ).to.be.revertedWithCustomError(walletRegistry, "CallerNotGovernance")
    })

    it("should reject updating authorization parameters", async () => {
      await expect(
        walletRegistry
          .connect(unauthorized)
          .updateAuthorizationParameters(to1e18(40000), 3888000, 3888000)
      ).to.be.revertedWithCustomError(walletRegistry, "CallerNotGovernance")
    })
  })

  it("should reject a callback from anyone other than the random beacon", async () => {
    await expect(
      // eslint-disable-next-line no-underscore-dangle
      walletRegistry.connect(unauthorized).__beaconCallback(12345, 0)
    ).to.be.revertedWithCustomError(walletRegistry, "CallerNotRandomBeacon")
  })

  it("should reject initializing an Allowlist with the zero address", async () => {
    // Initializers are disabled on implementations, so exercise the validation
    // through a fresh proxy instead of accepting an unrelated initializer error.
    const implementation = await ethers.getContractFactory("WalletRegistry", {
      libraries: {
        EcdsaInactivity: (
          await helpers.contracts.getContract("EcdsaInactivity")
        ).address,
      },
    })
    const deployed = await implementation.deploy(
      await walletRegistry.sortitionPool(),
      await walletRegistry.staking()
    )
    const proxyFactory = await ethers.getContractFactory(
      "TransparentUpgradeableProxy"
    )
    const proxy = await proxyFactory.deploy(
      deployed.address,
      deployer.address,
      "0x"
    )
    const registry = implementation.attach(proxy.address).connect(unauthorized)
    await expect(
      registry.initializeV2(ZERO_ADDRESS)
    ).to.be.revertedWithCustomError(registry, "AllowlistAddressZero")
  })

  describe("UnknownOperator", () => {
    it("should reject withdrawing rewards for an unregistered provider", async () => {
      await expect(
        walletRegistry.withdrawRewards(unauthorized.address)
      ).to.be.revertedWithCustomError(walletRegistry, "UnknownOperator")
    })

    it("should reject querying rewards for an unregistered provider", async () => {
      await expect(
        walletRegistry.availableRewards(unauthorized.address)
      ).to.be.revertedWithCustomError(walletRegistry, "UnknownOperator")
    })
  })

  describe("inactivity claims", () => {
    const claim = {
      walletID,
      inactiveMembersIndices: [],
      heartbeatFailed: false,
      signatures: "0x",
      signingMembersIndices: [],
    }

    it("should reject an incorrect nonce", async () => {
      await expect(
        walletRegistry
          .connect(unauthorized)
          .notifyOperatorInactivity(claim, 999, membersIDs)
      ).to.be.revertedWithCustomError(walletRegistry, "InvalidNonce")
    })

    it("should reject group members that do not match the registered wallet", async () => {
      await expect(
        walletRegistry
          .connect(unauthorized)
          .notifyOperatorInactivity(claim, 0, [...membersIDs].reverse())
      ).to.be.revertedWithCustomError(walletRegistry, "InvalidGroupMembers")
    })
  })

  describe("wallet membership", () => {
    it("should reject seizing with incorrect member identifiers", async () => {
      await expect(
        walletRegistry
          .connect(walletOwner.wallet)
          .seize(
            to1e18(1000),
            100,
            unauthorized.address,
            walletID,
            [...membersIDs].reverse()
          )
      ).to.be.revertedWithCustomError(
        walletRegistry,
        "InvalidWalletMembersIdentifiers"
      )
    })

    it("should reject a membership query with incorrect identifiers", async () => {
      await expect(
        walletRegistry.isWalletMember(
          walletID,
          [...membersIDs].reverse(),
          operator.address,
          1
        )
      ).to.be.revertedWithCustomError(
        walletRegistry,
        "InvalidWalletMembersIdentifiers"
      )
    })

    it("should reject a membership query for an operator outside the pool", async () => {
      await expect(
        walletRegistry.isWalletMember(
          walletID,
          membersIDs,
          unauthorized.address,
          1
        )
      ).to.be.revertedWithCustomError(
        walletRegistry,
        "NotSortitionPoolOperator"
      )
    })

    const invalidIndices = [0, 4]
    invalidIndices.forEach((index) => {
      it(`should reject member index ${index} for a three-member wallet`, async () => {
        await expect(
          walletRegistry.isWalletMember(
            walletID,
            membersIDs,
            operator.address,
            index
          )
        ).to.be.revertedWithCustomError(
          walletRegistry,
          "WalletMemberIndexOutOfRange"
        )
      })
    })
  })

  it("should reject a DKG parameter update while wallet creation is in progress", async () => {
    await walletRegistryGovernance
      .connect(governance)
      .beginDkgSeedTimeoutUpdate(100)
    await helpers.time.increaseTime(constants.governanceDelay)
    await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
    await expect(
      walletRegistryGovernance
        .connect(governance)
        .finalizeDkgSeedTimeoutUpdate()
    ).to.be.revertedWithCustomError(walletRegistry, "CurrentStateNotIdle")
  })

  it("should require the configured extra gas after challenging an invalid result", async () => {
    await walletRegistryGovernance
      .connect(governance)
      .beginDkgResultChallengeExtraGasUpdate(3000000)
    await helpers.time.increaseTime(constants.governanceDelay)
    await walletRegistryGovernance
      .connect(governance)
      .finalizeDkgResultChallengeExtraGasUpdate()
    await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
    const { startBlock, dkgSeed } = await submitRelayEntry(walletRegistry)
    const sortitionPool = await helpers.contracts.getContract(
      "EcdsaSortitionPool"
    )
    const { dkgResult } = await signAndSubmitUnrecoverableDkgResult(
      walletRegistry,
      ecdsaData.group2.publicKey,
      await selectGroup(sortitionPool, dkgSeed),
      startBlock,
      noMisbehaved
    )
    await expect(
      walletRegistry
        .connect(unauthorized)
        .challengeDkgResult(dkgResult, { gasLimit: 2000000 })
    ).to.be.revertedWithCustomError(walletRegistry, "NotEnoughExtraGasLeft")
  })
})
