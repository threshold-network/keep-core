/* eslint-disable @typescript-eslint/no-unused-expressions */
import { ethers } from "hardhat"
import { smock } from "@defi-wonderland/smock"
import { expect } from "chai"

import type { FakeContract } from "@defi-wonderland/smock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { Allowlist, WalletRegistry } from "../typechain"

const ZERO_ADDRESS = ethers.constants.AddressZero

describe("Allowlist", () => {
  let allowlist: Allowlist
  let walletRegistry: FakeContract<WalletRegistry>
  let governance: SignerWithAddress
  let stakingProvider1: SignerWithAddress
  let stakingProvider2: SignerWithAddress
  let thirdParty: SignerWithAddress

  beforeEach(async () => {
    // Deploy Allowlist contract
    const AllowlistFactory = await ethers.getContractFactory("Allowlist")
    allowlist = await AllowlistFactory.deploy()

    // Create fake WalletRegistry
    walletRegistry = await smock.fake<WalletRegistry>("WalletRegistry")

    const [deployer, sp1, sp2, tp] = await ethers.getSigners()

    governance = deployer // Use deployer as governance for simplicity
    stakingProvider1 = sp1
    stakingProvider2 = sp2
    thirdParty = tp

    // Initialize the Allowlist contract - this sets deployer as the owner
    await allowlist.initialize(walletRegistry.address)
  })

  describe("initialization", () => {
    it("should set the wallet registry address", async () => {
      expect(await allowlist.walletRegistry()).to.equal(walletRegistry.address)
    })

    it("should set the owner to deployer initially", async () => {
      expect(await allowlist.owner()).to.equal(governance.address)
    })

    it("should not allow re-initialization", async () => {
      await expect(
        allowlist.initialize(walletRegistry.address)
      ).to.be.revertedWith("Initializable: contract is already initialized")
    })

    it("should revert if initialized with zero address", async () => {
      const AllowlistFactory = await ethers.getContractFactory("Allowlist")
      const newAllowlist = await AllowlistFactory.deploy()

      await expect(newAllowlist.initialize(ZERO_ADDRESS)).to.be.revertedWith(
        "ZeroAddress"
      )
    })
  })

  describe("addStakingProvider", () => {
    context("when called by the owner", () => {
      it("should add a new staking provider with the specified weight", async () => {
        const weight = ethers.utils.parseEther("40000") // 40k T equivalent

        await expect(
          allowlist
            .connect(governance)
            .addStakingProvider(stakingProvider1.address, weight)
        )
          .to.emit(allowlist, "StakingProviderAdded")
          .withArgs(stakingProvider1.address, weight)

        const providerInfo = await allowlist.stakingProviders(
          stakingProvider1.address
        )
        expect(providerInfo.weight).to.equal(weight)
        expect(providerInfo.pendingNewWeight).to.equal(0)
      })

      it("should call authorizationIncreased on WalletRegistry", async () => {
        const weight = ethers.utils.parseEther("50000")

        await allowlist
          .connect(governance)
          .addStakingProvider(stakingProvider2.address, weight)

        expect(walletRegistry.authorizationIncreased).to.have.been.calledWith(
          stakingProvider2.address,
          0,
          weight
        )
      })

      it("should revert if staking provider already exists", async () => {
        const weight = ethers.utils.parseEther("40000")

        await allowlist
          .connect(governance)
          .addStakingProvider(stakingProvider1.address, weight)

        await expect(
          allowlist
            .connect(governance)
            .addStakingProvider(stakingProvider1.address, weight)
        ).to.be.reverted
      })

      it("should revert if staking provider is zero address", async () => {
        const weight = ethers.utils.parseEther("40000")

        await expect(
          allowlist.connect(governance).addStakingProvider(ZERO_ADDRESS, weight)
        ).to.be.revertedWith("ZeroAddress")
      })

      it("should revert if weight is zero", async () => {
        await expect(
          allowlist
            .connect(governance)
            .addStakingProvider(stakingProvider1.address, 0)
        ).to.be.revertedWith("ZeroWeight")
      })
    })

    context("when called by non-owner", () => {
      it("should revert", async () => {
        const weight = ethers.utils.parseEther("40000")

        await expect(
          allowlist
            .connect(thirdParty)
            .addStakingProvider(stakingProvider1.address, weight)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })
  })

  describe("requestWeightDecrease", () => {
    const initialWeight = ethers.utils.parseEther("50000")

    beforeEach(async () => {
      // Add a staking provider first
      await allowlist
        .connect(governance)
        .addStakingProvider(stakingProvider1.address, initialWeight)
    })

    context("when called by the owner", () => {
      it("should request weight decrease for existing provider", async () => {
        const newWeight = ethers.utils.parseEther("30000")

        await expect(
          allowlist
            .connect(governance)
            .requestWeightDecrease(stakingProvider1.address, newWeight)
        )
          .to.emit(allowlist, "WeightDecreaseRequested")
          .withArgs(stakingProvider1.address, initialWeight, newWeight)

        const providerInfo = await allowlist.stakingProviders(
          stakingProvider1.address
        )
        expect(providerInfo.weight).to.equal(initialWeight) // unchanged
        expect(providerInfo.pendingNewWeight).to.equal(newWeight)
      })

      it("should call authorizationDecreaseRequested on WalletRegistry", async () => {
        const newWeight = ethers.utils.parseEther("30000")

        await allowlist
          .connect(governance)
          .requestWeightDecrease(stakingProvider1.address, newWeight)

        expect(
          walletRegistry.authorizationDecreaseRequested
        ).to.have.been.calledWith(
          stakingProvider1.address,
          initialWeight,
          newWeight
        )
      })

      it("should allow setting weight to zero", async () => {
        const newWeight = 0

        await expect(
          allowlist
            .connect(governance)
            .requestWeightDecrease(stakingProvider1.address, newWeight)
        )
          .to.emit(allowlist, "WeightDecreaseRequested")
          .withArgs(stakingProvider1.address, initialWeight, newWeight)

        const providerInfo = await allowlist.stakingProviders(
          stakingProvider1.address
        )
        expect(providerInfo.pendingNewWeight).to.equal(newWeight)
      })

      it("should overwrite pending weight decrease request", async () => {
        const firstNewWeight = ethers.utils.parseEther("30000")
        const secondNewWeight = ethers.utils.parseEther("20000")

        await allowlist
          .connect(governance)
          .requestWeightDecrease(stakingProvider1.address, firstNewWeight)
        await allowlist
          .connect(governance)
          .requestWeightDecrease(stakingProvider1.address, secondNewWeight)

        const providerInfo = await allowlist.stakingProviders(
          stakingProvider1.address
        )
        expect(providerInfo.pendingNewWeight).to.equal(secondNewWeight)
      })

      it("should revert if staking provider is unknown", async () => {
        const newWeight = ethers.utils.parseEther("30000")

        await expect(
          allowlist
            .connect(governance)
            .requestWeightDecrease(stakingProvider2.address, newWeight)
        ).to.be.reverted
      })

      it("should revert if new weight is not below current weight", async () => {
        await expect(
          allowlist
            .connect(governance)
            .requestWeightDecrease(stakingProvider1.address, initialWeight)
        ).to.be.reverted

        const higherWeight = ethers.utils.parseEther("60000")
        await expect(
          allowlist
            .connect(governance)
            .requestWeightDecrease(stakingProvider1.address, higherWeight)
        ).to.be.reverted
      })
    })

    context("when called by non-owner", () => {
      it("should revert", async () => {
        const newWeight = ethers.utils.parseEther("30000")

        await expect(
          allowlist
            .connect(thirdParty)
            .requestWeightDecrease(stakingProvider1.address, newWeight)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })
  })

  describe("approveAuthorizationDecrease", () => {
    const initialWeight = ethers.utils.parseEther("50000")
    const newWeight = ethers.utils.parseEther("30000")

    beforeEach(async () => {
      // Add a staking provider and request weight decrease
      await allowlist
        .connect(governance)
        .addStakingProvider(stakingProvider1.address, initialWeight)
      await allowlist
        .connect(governance)
        .requestWeightDecrease(stakingProvider1.address, newWeight)
    })

    context("when called by WalletRegistry", () => {
      it("should approve the authorization decrease and return new weight", async () => {
        await expect(
          allowlist
            .connect(walletRegistry.wallet)
            .approveAuthorizationDecrease(stakingProvider1.address)
        )
          .to.emit(allowlist, "WeightDecreaseFinalized")
          .withArgs(stakingProvider1.address, initialWeight, newWeight)

        const providerInfo = await allowlist.stakingProviders(
          stakingProvider1.address
        )
        expect(providerInfo.weight).to.equal(newWeight)
        expect(providerInfo.pendingNewWeight).to.equal(0)
      })

      it("should return the new weight", async () => {
        const result = await allowlist
          .connect(walletRegistry.wallet)
          .callStatic.approveAuthorizationDecrease(stakingProvider1.address)
        expect(result).to.equal(newWeight)
      })

      it("should revert if staking provider is unknown", async () => {
        await expect(
          allowlist
            .connect(walletRegistry.wallet)
            .approveAuthorizationDecrease(stakingProvider2.address)
        ).to.be.reverted
      })

      it("should revert if no decrease was requested (bypass protection)", async () => {
        // Add a staking provider but don't request decrease
        await allowlist
          .connect(governance)
          .addStakingProvider(stakingProvider2.address, initialWeight)

        // Trying to approve decrease without requesting it first should fail
        await expect(
          allowlist
            .connect(walletRegistry.wallet)
            .approveAuthorizationDecrease(stakingProvider2.address)
        ).to.be.revertedWith("NoDecreasePending")
      })
    })

    context("when called by non-WalletRegistry", () => {
      it("should revert", async () => {
        await expect(
          allowlist
            .connect(thirdParty)
            .approveAuthorizationDecrease(stakingProvider1.address)
        ).to.be.reverted
      })
    })
  })

  describe("authorizedStake", () => {
    const weight = ethers.utils.parseEther("40000")

    beforeEach(async () => {
      await allowlist
        .connect(governance)
        .addStakingProvider(stakingProvider1.address, weight)
    })

    it("should return the current weight for existing provider", async () => {
      const result = await allowlist.authorizedStake(
        stakingProvider1.address,
        ZERO_ADDRESS
      )
      expect(result).to.equal(weight)
    })

    it("should return zero for non-existing provider", async () => {
      const result = await allowlist.authorizedStake(
        stakingProvider2.address,
        ZERO_ADDRESS
      )
      expect(result).to.equal(0)
    })

    it("should ignore the second parameter (application address)", async () => {
      const result1 = await allowlist.authorizedStake(
        stakingProvider1.address,
        ZERO_ADDRESS
      )
      const result2 = await allowlist.authorizedStake(
        stakingProvider1.address,
        thirdParty.address
      )
      expect(result1).to.equal(result2)
    })
  })

  describe("seize", () => {
    it("should emit MaliciousBehaviorIdentified event without seizing tokens", async () => {
      const stakingProviders = [
        stakingProvider1.address,
        stakingProvider2.address,
      ]

      await expect(
        allowlist.seize(
          ethers.utils.parseEther("1000"), // amount (ignored)
          100, // rewardMultiplier (ignored)
          thirdParty.address, // notifier
          stakingProviders
        )
      )
        .to.emit(allowlist, "MaliciousBehaviorIdentified")
        .withArgs(thirdParty.address, stakingProviders)
    })

    it("should be callable by anyone", async () => {
      const stakingProviders = [stakingProvider1.address]

      await expect(
        allowlist
          .connect(thirdParty)
          .seize(
            ethers.utils.parseEther("500"),
            50,
            governance.address,
            stakingProviders
          )
      ).to.not.be.reverted
    })
  })

  describe("rolesOf", () => {
    it("should return owner as stakeOwner, stakingProvider as beneficiary, and zero address as authorizer", async () => {
      const [stakeOwner, beneficiary, authorizer] = await allowlist.rolesOf(
        stakingProvider1.address
      )

      expect(stakeOwner).to.equal(governance.address) // owner of allowlist
      expect(beneficiary).to.equal(stakingProvider1.address) // staking provider itself
      expect(authorizer).to.equal(ZERO_ADDRESS) // no authorizer role in allowlist
    })

    it("should work for any address", async () => {
      const [stakeOwner, beneficiary, authorizer] = await allowlist.rolesOf(
        thirdParty.address
      )

      expect(stakeOwner).to.equal(governance.address)
      expect(beneficiary).to.equal(thirdParty.address)
      expect(authorizer).to.equal(ZERO_ADDRESS)
    })
  })

  describe("integration scenarios", () => {
    it("should support beta staker consolidation workflow", async () => {
      // Add multiple staking providers (simulating existing beta stakers)
      const providers = [stakingProvider1.address, stakingProvider2.address]
      const initialWeight = ethers.utils.parseEther("40000")

      for (const provider of providers) {
        await allowlist
          .connect(governance)
          .addStakingProvider(provider, initialWeight)
      }

      // Verify initial state
      for (const provider of providers) {
        const stake = await allowlist.authorizedStake(provider, ZERO_ADDRESS)
        expect(stake).to.equal(initialWeight)
      }

      // Consolidation: Set one provider's weight to 0 (redundant node)
      await allowlist
        .connect(governance)
        .requestWeightDecrease(stakingProvider1.address, 0)

      // Simulate WalletRegistry approval
      await allowlist
        .connect(walletRegistry.wallet)
        .approveAuthorizationDecrease(stakingProvider1.address)

      // Verify consolidation result
      const consolidatedStake = await allowlist.authorizedStake(
        stakingProvider1.address,
        ZERO_ADDRESS
      )
      const activeStake = await allowlist.authorizedStake(
        stakingProvider2.address,
        ZERO_ADDRESS
      )

      expect(consolidatedStake).to.equal(0) // Consolidated provider has no weight
      expect(activeStake).to.equal(initialWeight) // Active provider unchanged
    })
  })
})
