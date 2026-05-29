const { contract, accounts, web3 } = require("@openzeppelin/test-environment")
const { expectRevert, time } = require("@openzeppelin/test-helpers")
const { createSnapshot, restoreSnapshot } = require("../helpers/snapshot")
const { initTokenStaking } = require("../helpers/initContracts")

const BN = web3.utils.BN
const chai = require("chai")
chai.use(require("bn-chai")(BN))
const expect = chai.expect

const KeepToken = contract.fromArtifact("KeepToken")
const TokenGrant = contract.fromArtifact("TokenGrant")
const KeepRegistry = contract.fromArtifact("KeepRegistry")
const PermissiveStakingPolicy = contract.fromArtifact("PermissiveStakingPolicy")
const GuaranteedMinimumStakingPolicy = contract.fromArtifact(
  "GuaranteedMinimumStakingPolicy"
)

const ManagedGrant = contract.fromArtifact("ManagedGrant")
const ManagedGrantFactory = contract.fromArtifact("ManagedGrantFactory")

describe("TokenGrant/ManagedGrantFactory", () => {
  let token
  let registry
  let tokenGrant
  let staking
  let permissivePolicy
  let minimumStake
  let grantAmount

  const grantCreator = accounts[0]
  const grantee = accounts[2]
  const unrelatedAddress = accounts[3]

  let grantStart

  const grantUnlockingDuration = time.duration.days(60)
  const grantCliff = time.duration.days(10)

  const initializationPeriod = time.duration.minutes(10)

  let factory

  before(async () => {
    token = await KeepToken.new({ from: grantCreator })
    tokenGrant = await TokenGrant.new(token.address, { from: grantCreator })
    registry = await KeepRegistry.new({ from: grantCreator })
    const contracts = await initTokenStaking(
      token.address,
      tokenGrant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStaking")
    )
    staking = contracts.tokenStaking

    await tokenGrant.authorizeStakingContract(staking.address, {
      from: grantCreator,
    })

    minimumStake = await staking.minimumStake()

    permissivePolicy = await PermissiveStakingPolicy.new()
    minimumPolicy = await GuaranteedMinimumStakingPolicy.new(staking.address)
    grantAmount = minimumStake.muln(10)

    factory = await ManagedGrantFactory.new(token.address, tokenGrant.address, {
      from: grantCreator,
    })
  })

  beforeEach(async () => {
    await createSnapshot()
  })

  afterEach(async () => {
    await restoreSnapshot()
  })

  describe("creating managed grants", async () => {
    it("works with a two-step call", async () => {
      await token.approve(factory.address, grantAmount, { from: grantCreator })
      grantStart = await time.latest()
      const managedGrantAddress = await factory.createManagedGrant.call(
        grantee,
        grantAmount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        false,
        permissivePolicy.address,
        { from: grantCreator }
      )
      await factory.createManagedGrant(
        grantee,
        grantAmount,
        grantUnlockingDuration,
        grantStart,
        grantCliff,
        false,
        permissivePolicy.address,
        { from: grantCreator }
      )
      const event = (await factory.getPastEvents())[0]
      const managedGrant = await ManagedGrant.at(managedGrantAddress)
      const grantId = await managedGrant.grantId()
      expect(await tokenGrant.availableToStake(grantId)).to.eq.BN(grantAmount)
      expect(await managedGrant.grantee()).to.equal(grantee)
      expect(await managedGrant.grantManager()).to.equal(grantCreator)
      expect(event.args["grantAddress"]).to.equal(managedGrantAddress)
      expect(event.args["grantee"]).to.equal(grantee)

      const schedule = await tokenGrant.getGrantUnlockingSchedule(grantId)
      const returnedGrantManager = schedule[0]
      expect(returnedGrantManager).to.equal(grantCreator)

      const returnedDuration = schedule[1]
      expect(returnedDuration).to.eq.BN(grantUnlockingDuration)

      const returnedStart = schedule[2]
      expect(returnedStart).to.eq.BN(grantStart)

      const returnedCliff = schedule[3]
      expect(returnedCliff).to.eq.BN(grantStart.add(grantCliff))

      const returnedPolicy = schedule[4]
      expect(returnedPolicy).to.equal(permissivePolicy.address)
    })

    it("works with receiveApproval", async () => {
      grantStart = await time.latest()
      const extraData = web3.eth.abi.encodeParameters(
        ["address", "uint256", "uint256", "uint256", "bool", "address"],
        [
          grantee,
          grantUnlockingDuration.toNumber(),
          grantStart.toNumber(),
          grantCliff.toNumber(),
          false,
          permissivePolicy.address,
        ]
      )
      await token.approveAndCall(factory.address, grantAmount, extraData, {
        from: grantCreator,
      })
      const event = (await factory.getPastEvents())[0]
      expect(event.args["grantee"]).to.equal(grantee)
      const managedGrantAddress = event.args["grantAddress"]
      const managedGrant = await ManagedGrant.at(managedGrantAddress)
      const grantId = await managedGrant.grantId()
      expect(await tokenGrant.availableToStake(grantId)).to.eq.BN(grantAmount)
      expect(await managedGrant.grantee()).to.equal(grantee)
      expect(await managedGrant.grantManager()).to.equal(grantCreator)

      const schedule = await tokenGrant.getGrantUnlockingSchedule(grantId)
      const returnedGrantManager = schedule[0]
      expect(returnedGrantManager).to.equal(grantCreator)

      const returnedDuration = schedule[1]
      expect(returnedDuration).to.eq.BN(grantUnlockingDuration)

      const returnedStart = schedule[2]
      expect(returnedStart).to.eq.BN(grantStart)

      const returnedCliff = schedule[3]
      expect(returnedCliff).to.eq.BN(grantStart.add(grantCliff))

      const returnedPolicy = schedule[4]
      expect(returnedPolicy).to.equal(permissivePolicy.address)
    })

    it("doesn't let one grant more than they've approved on the token", async () => {
      await token.transfer(unrelatedAddress, grantAmount, {
        from: grantCreator,
      })
      await token.approve(factory.address, grantAmount.subn(1), {
        from: unrelatedAddress,
      })
      grantStart = await time.latest()
      await expectRevert(
        factory.createManagedGrant(
          grantee,
          grantAmount,
          grantUnlockingDuration,
          grantStart,
          grantCliff,
          false,
          permissivePolicy.address,
          { from: unrelatedAddress }
        ),
        "SafeERC20: low-level call failed -- Reason given: SafeERC20: low-level call failed."
      )
    })

    it("doesn't let one grant more than they have on the token", async () => {
      await token.transfer(unrelatedAddress, grantAmount, {
        from: grantCreator,
      })
      await token.approve(factory.address, grantAmount.addn(1), {
        from: unrelatedAddress,
      })
      grantStart = await time.latest()
      await expectRevert(
        factory.createManagedGrant(
          grantee,
          grantAmount.addn(1),
          grantUnlockingDuration,
          grantStart,
          grantCliff,
          false,
          permissivePolicy.address,
          { from: unrelatedAddress }
        ),
        "SafeERC20: low-level call failed -- Reason given: SafeERC20: low-level call failed."
      )
    })
  })
})
