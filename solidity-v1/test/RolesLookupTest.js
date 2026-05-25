const { accounts, contract } = require("@openzeppelin/test-environment")
const { time, expectRevert } = require("@openzeppelin/test-helpers")
const { initTokenStaking } = require("./helpers/initContracts")
const { grantTokens } = require("./helpers/grantTokens")
const {
  delegateStake,
  delegateStakeFromGrant,
  delegateStakeFromManagedGrant,
} = require("./helpers/delegateStake")
const { createSnapshot, restoreSnapshot } = require("./helpers/snapshot")
const assert = require("chai").assert

const KeepToken = contract.fromArtifact("KeepToken")
const TokenGrant = contract.fromArtifact("TokenGrant")
const KeepRegistry = contract.fromArtifact("KeepRegistry")
const PermissiveStakingPolicy = contract.fromArtifact("PermissiveStakingPolicy")
const ManagedGrantFactory = contract.fromArtifact("ManagedGrantFactory")
const ManagedGrant = contract.fromArtifact("ManagedGrant")
const RolesLookup = contract.fromArtifact("RolesLookup")
const RolesLookupStub = contract.fromArtifact("RolesLookupStub")

describe("RolesLookup", () => {
  const deployer = accounts[0]
  const tokenOwner1 = accounts[1]
  const tokenOwner2 = accounts[2]
  const nonTokenOwner = accounts[3]
  const operator1 = accounts[4]
  const operator2 = accounts[5]
  const nonOperator = accounts[6]
  const beneficiary1 = accounts[7]
  const beneficiary2 = accounts[8]
  const authorizer = accounts[9]
  const grantee1 = accounts[10]
  const grantee2 = accounts[11]
  const nonGrantee = accounts[12]

  const initializationPeriod = time.duration.seconds(0)
  const grantUnlockingDuration = time.duration.seconds(0)
  const grantStart = time.duration.seconds(0)
  const grantCliff = time.duration.seconds(0)
  const grantRevocable = true

  let token
  let tokenGrant
  let tokenStaking
  let tokenGrantStakingPolicy
  let managedGrantFactory
  let lookup

  before(async () => {
    const registry = await KeepRegistry.new({ from: deployer })
    token = await KeepToken.new({ from: deployer })
    tokenGrant = await TokenGrant.new(token.address, { from: deployer })
    const stakingContracts = await initTokenStaking(
      token.address,
      tokenGrant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStaking")
    )
    tokenStaking = stakingContracts.tokenStaking
    tokenGrantStakingPolicy = await PermissiveStakingPolicy.new()
    managedGrantFactory = await ManagedGrantFactory.new(
      token.address,
      tokenGrant.address,
      { from: deployer }
    )

    await tokenGrant.authorizeStakingContract(tokenStaking.address, {
      from: deployer,
    })

    const lookupLib = await RolesLookup.new({ from: deployer })
    await RolesLookupStub.detectNetwork()
    await RolesLookupStub.link("RolesLookup", lookupLib.address)
    lookup = await RolesLookupStub.new(
      tokenStaking.address,
      tokenGrant.address,
      { from: deployer }
    )
  })

  describe("isTokenOwnerForOperator", async () => {
    before(async () => {
      await createSnapshot()
      const amount = await tokenStaking.minimumStake()

      await token.transfer(tokenOwner1, amount, { from: deployer })
      await delegateStake(
        token,
        tokenStaking,
        tokenOwner1,
        operator1,
        beneficiary1,
        authorizer,
        amount,
        { from: tokenOwner1 }
      )

      await token.transfer(tokenOwner2, amount, { from: deployer })
      await delegateStake(
        token,
        tokenStaking,
        tokenOwner2,
        operator2,
        beneficiary2,
        authorizer,
        amount,
        { from: tokenOwner2 }
      )
    })

    after(async () => {
      await restoreSnapshot()
    })

    it("returns true for token owner and its operator", async () => {
      assert.isTrue(
        await lookup.isTokenOwnerForOperator(tokenOwner1, operator1)
      )
    })

    it("returns false for mismatched token owner and operator", async () => {
      assert.isFalse(
        await lookup.isTokenOwnerForOperator(tokenOwner1, operator2)
      )
    })

    it("returns false for incorrect operator", async () => {
      assert.isFalse(
        await lookup.isTokenOwnerForOperator(tokenOwner1, nonOperator)
      )
    })

    it("returns false for non-token-owner", async () => {
      assert.isFalse(
        await lookup.isTokenOwnerForOperator(nonTokenOwner, operator1)
      )
    })
  })

  describe("isGranteeForOperator", async () => {
    let amount

    before(async () => {
      await createSnapshot()
      amount = await tokenStaking.minimumStake()

      const grantId1 = await grantTokens(
        tokenGrant,
        token,
        amount,
        deployer,
        grantee1,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      await delegateStakeFromGrant(
        tokenGrant,
        tokenStaking.address,
        grantee1,
        operator1,
        beneficiary1,
        authorizer,
        amount,
        grantId1
      )

      const grantId2 = await grantTokens(
        tokenGrant,
        token,
        amount,
        deployer,
        grantee2,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      await delegateStakeFromGrant(
        tokenGrant,
        tokenStaking.address,
        grantee2,
        operator2,
        beneficiary2,
        authorizer,
        amount,
        grantId2
      )
    })

    after(async () => {
      await restoreSnapshot()
    })

    it("returns true for grantee and its operator", async () => {
      assert.isTrue(await lookup.isGranteeForOperator(grantee1, operator1))
    })

    it("returns false for mismatched grantee and operator", async () => {
      assert.isFalse(await lookup.isGranteeForOperator(grantee1, operator2))
    })

    it("returns false for incorrect operator", async () => {
      assert.isFalse(await lookup.isGranteeForOperator(grantee1, nonOperator))
    })

    it("returns false for non-grantee", async () => {
      assert.isFalse(await lookup.isGranteeForOperator(nonGrantee, operator1))
    })
  })

  describe("isManagedGranteeForOperator", async () => {
    let managedGrantAddress1
    let managedGrantAddress2

    before(async () => {
      await createSnapshot()
      const amount = await tokenStaking.minimumStake()

      await token.approve(managedGrantFactory.address, amount, {
        from: deployer,
      })

      managedGrantAddress1 = await managedGrantFactory.createManagedGrant.call(
        grantee1,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      await managedGrantFactory.createManagedGrant(
        grantee1,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      const managedGrant1 = await ManagedGrant.at(managedGrantAddress1)
      await delegateStakeFromManagedGrant(
        managedGrant1,
        tokenStaking.address,
        grantee1,
        operator1,
        beneficiary1,
        authorizer,
        amount
      )

      await token.approve(managedGrantFactory.address, amount, {
        from: deployer,
      })
      managedGrantAddress2 = await managedGrantFactory.createManagedGrant.call(
        grantee2,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      await managedGrantFactory.createManagedGrant(
        grantee2,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      const managedGrant2 = await ManagedGrant.at(managedGrantAddress2)
      await delegateStakeFromManagedGrant(
        managedGrant2,
        tokenStaking.address,
        grantee2,
        operator2,
        beneficiary2,
        authorizer,
        amount
      )
    })

    after(async () => {
      await restoreSnapshot()
    })

    it("returns true for grantee and its operator", async () => {
      assert.isTrue(
        await lookup.isManagedGranteeForOperator(
          grantee1,
          operator1,
          managedGrantAddress1
        )
      )
    })

    it("reverts for mismatched grantee", async () => {
      await expectRevert(
        lookup.isManagedGranteeForOperator(
          grantee2,
          operator1,
          managedGrantAddress1
        ),
        "Not a grantee of the provided contract"
      )
    })

    it("returns false for mismatched operator", async () => {
      assert.isFalse(
        await lookup.isManagedGranteeForOperator(
          grantee1,
          operator2,
          managedGrantAddress1
        )
      )
    })

    it("returns false for mismatched managed grant", async () => {
      await expectRevert(
        lookup.isManagedGranteeForOperator(
          grantee1,
          operator1,
          managedGrantAddress2
        ),
        "Not a grantee of the provided contract"
      )
    })

    it("reverts for mismatched operator and managed grant", async () => {
      await expectRevert(
        lookup.isManagedGranteeForOperator(
          grantee1,
          operator2,
          managedGrantAddress2
        ),
        "Not a grantee of the provided contract"
      )
    })

    it("returns false for incorrect operator", async () => {
      assert.isFalse(
        await lookup.isManagedGranteeForOperator(
          grantee1,
          nonOperator,
          managedGrantAddress1
        )
      )
    })

    it("reverts for non-grantee", async () => {
      await expectRevert(
        lookup.isManagedGranteeForOperator(
          nonGrantee,
          nonOperator,
          managedGrantAddress1
        ),
        "Not a grantee of the provided contract"
      )
    })
  })

  describe("isManagedGranteeForGrant", async () => {
    let managedGrant1Address
    let managedGrant2Address
    let managedGrant1Id
    let managedGrant2Id

    before(async () => {
      await createSnapshot()
      const amount = await tokenStaking.minimumStake()

      await token.approve(managedGrantFactory.address, amount, {
        from: deployer,
      })

      managedGrant1Address = await managedGrantFactory.createManagedGrant.call(
        grantee1,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      await managedGrantFactory.createManagedGrant(
        grantee1,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      const managedGrant1 = await ManagedGrant.at(managedGrant1Address)
      managedGrant1Id = await managedGrant1.grantId()

      await token.approve(managedGrantFactory.address, amount, {
        from: deployer,
      })
      managedGrant2Address = await managedGrantFactory.createManagedGrant.call(
        grantee2,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      await managedGrantFactory.createManagedGrant(
        grantee2,
        amount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        grantRevocable,
        tokenGrantStakingPolicy.address,
        { from: deployer }
      )
      const managedGrant2 = await ManagedGrant.at(managedGrant2Address)
      managedGrant2Id = await managedGrant2.grantId()
    })

    after(async () => {
      await restoreSnapshot()
    })

    it("returns true for managed grant and correct operator and grant ID", async () => {
      assert.isTrue(
        await lookup.isManagedGranteeForGrant.call(grantee1, managedGrant1Id)
      )
      assert.isTrue(
        await lookup.isManagedGranteeForGrant.call(grantee2, managedGrant2Id)
      )
    })

    it("returns false for mismatched managed grant", async () => {
      assert.isFalse(
        await lookup.isManagedGranteeForGrant.call(grantee1, managedGrant2Id)
      )
    })

    it("returns false for mismatched grantee", async () => {
      assert.isFalse(
        await lookup.isManagedGranteeForGrant.call(grantee2, managedGrant1Id)
      )
    })

    it("returns false for not a grantee", async () => {
      assert.isFalse(
        await lookup.isManagedGranteeForGrant.call(nonGrantee, managedGrant1Id)
      )
    })

    it("returns false for not a managed grant", async () => {
      assert.isFalse(
        await lookup.isManagedGranteeForGrant.call(
          grantee1,
          token.address // not a managed grant contract
        )
      )
      assert.isFalse(
        await lookup.isManagedGranteeForGrant.call(
          grantee1,
          deployer // not a contract
        )
      )
    })
  })
})
