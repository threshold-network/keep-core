const { contract, accounts, web3 } = require("@openzeppelin/test-environment")
const { expectRevert, time } = require("@openzeppelin/test-helpers")
const { createSnapshot, restoreSnapshot } = require("../helpers/snapshot")
const { initTokenStaking } = require("../helpers/initContracts")
const stakeDelegate = require("../helpers/stakeDelegate")

const KeepToken = contract.fromArtifact("KeepToken")
const TokenGrant = contract.fromArtifact("TokenGrant")
const KeepRegistry = contract.fromArtifact("KeepRegistry")

const BN = web3.utils.BN
const chai = require("chai")
chai.use(require("bn-chai")(BN))
const expect = chai.expect

describe("TokenStaking/Punishment", () => {
  let token
  let registry
  let stakingContract

  const owner = accounts[0]
  const registryKeeper = accounts[1]
  const operator = accounts[2]
  const authorizer = accounts[3]
  const operatorContract = accounts[4]
  const tattletale = accounts[5]

  let largeStake
  let minimumStake

  const initializationPeriod = time.duration.seconds(10)

  before(async () => {
    token = await KeepToken.new({ from: owner })
    tokenGrant = await TokenGrant.new(token.address, { from: owner })
    registry = await KeepRegistry.new({ from: owner })
    const stakingContracts = await initTokenStaking(
      token.address,
      tokenGrant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStakingStub")
    )
    stakingContract = stakingContracts.tokenStaking

    await registry.setRegistryKeeper(registryKeeper, { from: owner })

    minimumStake = await stakingContract.minimumStake()
    largeStake = minimumStake.muln(2)

    await registry.approveOperatorContract(operatorContract, {
      from: registryKeeper,
    })

    await stakeDelegate(
      stakingContract,
      token,
      owner,
      operator,
      owner,
      authorizer,
      largeStake
    )

    await stakingContract.authorizeOperatorContract(
      operator,
      operatorContract,
      { from: authorizer }
    )
  })

  beforeEach(async () => {
    await createSnapshot()
  })

  afterEach(async () => {
    await restoreSnapshot()
  })

  describe("slash", () => {
    it("should slash token amount from stake", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))

      const amountToSlash = web3.utils.toBN(42000000)

      const balanceBeforeSlashing = await stakingContract.balanceOf(operator)
      await stakingContract.slash(amountToSlash, [operator], {
        from: operatorContract,
      })
      const balanceAfterSlashing = await stakingContract.balanceOf(operator)

      expect(balanceAfterSlashing).to.eq.BN(
        balanceBeforeSlashing.sub(amountToSlash)
      )
    })

    it("should slash no more than available on stake", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))

      const amountToSlash = largeStake.add(web3.utils.toBN(100))
      await stakingContract.slash(amountToSlash, [operator], {
        from: operatorContract,
      })
      const balanceAfterSlashing = await stakingContract.balanceOf(operator)

      expect(balanceAfterSlashing).to.eq.BN(0)
    })

    it("should not fail if operator is slashed to zero", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))

      const amountToSlash = largeStake

      // the first slash will slash to 0, the second one has nothing
      // to slash; it should not fail
      await stakingContract.slash(amountToSlash, [operator], {
        from: operatorContract,
      })
      await stakingContract.slash(amountToSlash, [operator], {
        from: operatorContract,
      })

      const balanceAfterSlashing = await stakingContract.balanceOf(operator)
      expect(balanceAfterSlashing).to.eq.BN(0)
    })

    it("should fail when operator stake is not active yet", async () => {
      const amountToSlash = web3.utils.toBN(1000)
      await expectRevert(
        stakingContract.slash(amountToSlash, [operator], {
          from: operatorContract,
        }),
        "Inactive stake"
      )
    })

    it("should fail when operator stake is released", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))
      await stakingContract.undelegate(operator, { from: owner })
      time.increase((await stakingContract.undelegationPeriod()).addn(1))

      const amountToSlash = web3.utils.toBN(100)
      await expectRevert(
        stakingContract.slash(amountToSlash, [operator], {
          from: operatorContract,
        }),
        "Stake is released"
      )
    })
  })

  describe("seize", () => {
    it("should seize token amount from stake", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))

      const operatorBalanceBeforeSeizing = await stakingContract.balanceOf(
        operator
      )
      const tattletaleBalanceBeforeSeizing = await token.balanceOf(tattletale)

      const amountToSeize = web3.utils.toBN(42000000)
      const rewardMultiplier = web3.utils.toBN(25)
      await stakingContract.seize(
        amountToSeize,
        rewardMultiplier,
        tattletale,
        [operator],
        { from: operatorContract }
      )

      const operatorBalanceAfterSeizing = await stakingContract.balanceOf(
        operator
      )
      const tattletaleBalanceAfterSeizing = await token.balanceOf(tattletale)

      expect(operatorBalanceAfterSeizing).to.eq.BN(
        operatorBalanceBeforeSeizing.sub(amountToSeize)
      )

      // 525000 = (42000000 * 5 / 100) * 25 / 100
      const expectedTattletaleReward = web3.utils.toBN(525000)
      expect(tattletaleBalanceAfterSeizing).to.eq.BN(
        tattletaleBalanceBeforeSeizing.add(expectedTattletaleReward)
      )
    })

    it("should seize no more than available on stake", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))

      const tattletaleBalanceBeforeSeizing = await token.balanceOf(tattletale)

      // we test with a higher excess to ensure that the tattletale reward
      // is calculated from the applied penalty, not the requested penalty
      const amountToSeize = largeStake.muln(2) // 400000000000000000000000
      const rewardMultiplier = web3.utils.toBN(10)
      await stakingContract.seize(
        amountToSeize,
        rewardMultiplier,
        tattletale,
        [operator],
        { from: operatorContract }
      )

      const operatorBalanceAfterSeizing = await stakingContract.balanceOf(
        operator
      )
      const tattletaleBalanceAfterSeizing = await token.balanceOf(tattletale)

      expect(operatorBalanceAfterSeizing).to.eq.BN(0)

      // 1000000000000000000000 = (200000000000000000000000 * 5 / 100) * 10 / 100
      const expectedTattletaleReward = web3.utils.toBN("1000000000000000000000")
      expect(tattletaleBalanceAfterSeizing).to.eq.BN(
        tattletaleBalanceBeforeSeizing.add(expectedTattletaleReward)
      )
    })

    it("should not fail if operator is slashed to zero", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))

      const amountToSlash = largeStake
      const amountToSeize = largeStake
      const rewardMultiplier = web3.utils.toBN(10)

      const tattletaleBalanceBeforeSeizing = await token.balanceOf(tattletale)

      // the first slash will slash to 0, the seize happening later
      // should not fail
      await stakingContract.slash(amountToSlash, [operator], {
        from: operatorContract,
      })
      await stakingContract.seize(
        amountToSeize,
        rewardMultiplier,
        tattletale,
        [operator],
        { from: operatorContract }
      )

      const operatorBalanceAfterSeizing = await stakingContract.balanceOf(
        operator
      )
      const tattletaleBalanceAfterSeizing = await token.balanceOf(tattletale)

      expect(operatorBalanceAfterSeizing).to.eq.BN(0)
      expect(tattletaleBalanceAfterSeizing).to.eq.BN(
        tattletaleBalanceBeforeSeizing
      )
    })

    it("should fail when operator stake is not active yet", async () => {
      const amountToSeize = web3.utils.toBN(42000000)
      const rewardMultiplier = web3.utils.toBN(25)
      await expectRevert(
        stakingContract.seize(
          amountToSeize,
          rewardMultiplier,
          tattletale,
          [operator],
          { from: operatorContract }
        ),
        "Inactive stake"
      )
    })

    it("should fail when operator stake is released", async () => {
      time.increase((await stakingContract.initializationPeriod()).addn(1))
      await stakingContract.undelegate(operator, { from: owner })
      time.increase((await stakingContract.undelegationPeriod()).addn(1))

      const amountToSeize = web3.utils.toBN(10000)
      const rewardMultiplier = web3.utils.toBN(25)
      await expectRevert(
        stakingContract.seize(
          amountToSeize,
          rewardMultiplier,
          tattletale,
          [operator],
          { from: operatorContract }
        ),
        "Stake is released"
      )
    })
  })
})
