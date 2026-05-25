const { contract, accounts, web3 } = require("@openzeppelin/test-environment")
const {
  expectRevert,
  expectEvent,
  time,
} = require("@openzeppelin/test-helpers")
const { createSnapshot, restoreSnapshot } = require("../helpers/snapshot")
const { initTokenStaking } = require("../helpers/initContracts")

const BN = web3.utils.BN
const chai = require("chai")
chai.use(require("bn-chai")(BN))
const expect = chai.expect

// Depending on test network increaseTimeTo can be inconsistent and add
// extra time. As a workaround we subtract timeRoundMargin in all cases
// that test times before initialization/undelegation periods end.
const timeRoundMargin = time.duration.minutes(1)

const KeepToken = contract.fromArtifact("KeepToken")
const TokenGrant = contract.fromArtifact("TokenGrant")
const KeepRegistry = contract.fromArtifact("KeepRegistry")

describe("TokenStaking", function () {
  let token
  let tokenGrant
  let registry
  let stakingContract
  let stakingAmount
  let minimumStake

  const owner = accounts[0]
  const operatorOne = accounts[2]
  const operatorTwo = accounts[3]
  const beneficiary = accounts[4]
  const authorizer = accounts[5]
  const operatorContract = accounts[6]
  const thirdParty = accounts[7]

  const initializationPeriod = time.duration.minutes(10)
  let undelegationPeriod

  before(async () => {
    token = await KeepToken.new({ from: accounts[0] })
    tokenGrant = await TokenGrant.new(token.address, { from: accounts[0] })
    registry = await KeepRegistry.new({ from: accounts[0] })
    const stakingContracts = await initTokenStaking(
      token.address,
      tokenGrant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStaking")
    )
    stakingContract = stakingContracts.tokenStaking

    undelegationPeriod = await stakingContract.undelegationPeriod()

    await registry.approveOperatorContract(operatorContract, {
      from: accounts[0],
    })

    minimumStake = await stakingContract.minimumStake()
    stakingAmount = minimumStake.muln(20)
  })

  beforeEach(async () => {
    await createSnapshot()
  })

  afterEach(async () => {
    await restoreSnapshot()
  })

  async function delegate(operator, amount) {
    const data = Buffer.concat([
      Buffer.from(beneficiary.substr(2), "hex"),
      Buffer.from(operator.substr(2), "hex"),
      Buffer.from(authorizer.substr(2), "hex"),
    ])

    return token.approveAndCall(
      stakingContract.address,
      amount,
      "0x" + data.toString("hex"),
      { from: owner }
    )
  }

  describe("undelegationPeriod", async () => {
    const twoWeeks = web3.utils.toBN("1209600") // [sec]
    const twoMonths = web3.utils.toBN("5184000") // [sec]

    it("is two weeks right after deploying the contract", async () => {
      expect(await stakingContract.undelegationPeriod()).to.eq.BN(twoWeeks)
    })

    it("is two weeks one month after deploying the contract", async () => {
      await time.increase(time.duration.days(30))
      expect(await stakingContract.undelegationPeriod()).to.eq.BN(twoWeeks)
    })

    it("is two weeks before two months after the deployment passes", async () => {
      await time.increase(time.duration.days(59))
      await time.increase(time.duration.hours(23))
      expect(await stakingContract.undelegationPeriod()).to.eq.BN(twoWeeks)
    })

    it("is two months after two months after the deployment passes", async () => {
      await time.increase(time.duration.days(60))
      expect(await stakingContract.undelegationPeriod()).to.eq.BN(twoMonths)
    })

    it("remains as two months after two months after the deployment passes", async () => {
      await time.increase(time.duration.days(180))
      expect(await stakingContract.undelegationPeriod()).to.eq.BN(twoMonths)
      await time.increase(time.duration.days(360))
      expect(await stakingContract.undelegationPeriod()).to.eq.BN(twoMonths)
    })
  })

  describe("delegate", async () => {
    it("should update balances", async () => {
      const ownerStartBalance = await token.balanceOf.call(owner)

      await delegate(operatorOne, stakingAmount)

      const ownerEndBalance = await token.balanceOf.call(owner)
      const operatorEndStakeBalance = await stakingContract.balanceOf.call(
        operatorOne
      )

      expect(ownerEndBalance).to.eq.BN(
        ownerStartBalance.sub(stakingAmount),
        "Staking amount should be transferred from owner balance"
      )
      expect(operatorEndStakeBalance).to.eq.BN(
        stakingAmount,
        "Staking amount should be added to the operator balance"
      )
    })

    it("should not allow to delegate to the same operator after recovering stake", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: owner })
      await time.increase(undelegationPeriod.addn(1))
      await stakingContract.recoverStake(operatorOne)

      await expectRevert(
        delegate(operatorOne, stakingAmount),
        "Stake undelegated"
      )
    })

    it("should not allow to delegate less than the minimum stake", async () => {
      await expectRevert(
        delegate(operatorOne, minimumStake.subn(1)),
        "Less than the minimum stake"
      )
    })

    it("should allow to delegate the minimum stake", async () => {
      await delegate(operatorOne, minimumStake)
      // ok, no reverts
    })

    it("should allow to delegate to two different operators", async () => {
      const ownerStartBalance = await token.balanceOf.call(owner)

      await delegate(operatorOne, stakingAmount)
      await delegate(operatorTwo, stakingAmount)

      const ownerEndBalance = await token.balanceOf.call(owner)
      const operatorOneEndStakeBalance = await stakingContract.balanceOf.call(
        operatorOne
      )
      const operatorTwoEndStakeBalance = await stakingContract.balanceOf.call(
        operatorTwo
      )

      expect(ownerEndBalance).to.eq.BN(
        ownerStartBalance.sub(stakingAmount).sub(stakingAmount),
        "Staking amount should be transferred from owner balance"
      )
      expect(operatorOneEndStakeBalance).to.eq.BN(
        stakingAmount,
        "Staking amount should be added to the operator balance"
      )
      expect(operatorTwoEndStakeBalance).to.eq.BN(
        stakingAmount,
        "Staking amount should be added to the operator balance"
      )
    })

    it("should emit OperatorStaked event", async () => {
      await delegate(operatorOne, stakingAmount)

      const operatorStakedEvents = await stakingContract.getPastEvents(
        "OperatorStaked"
      )
      expect(operatorStakedEvents.length).to.equal(1)
      const operatorStakedEvent = operatorStakedEvents[0]
      expect(operatorStakedEvent.args["operator"]).to.equal(operatorOne)
      expect(operatorStakedEvent.args["beneficiary"]).to.equal(beneficiary)
      expect(operatorStakedEvent.args["authorizer"]).to.equal(authorizer)
    })

    it("should emit StakeDelegated event", async () => {
      await delegate(operatorOne, stakingAmount)

      const stakeDelegatedEvents = await stakingContract.getPastEvents(
        "StakeDelegated"
      )
      expect(stakeDelegatedEvents.length).to.equal(1)
      const stakeDelegatedEvent = stakeDelegatedEvents[0]
      expect(stakeDelegatedEvent.args["owner"]).to.equal(owner)
      expect(stakeDelegatedEvent.args["operator"]).to.equal(operatorOne)
    })
  })

  describe("cancelStake", async () => {
    it("should let operator cancel delegation", async () => {
      await delegate(operatorOne, stakingAmount)

      await stakingContract.cancelStake(operatorOne, { from: operatorOne })
      // ok, no revert
    })

    it("should not allow third party to cancel delegation", async () => {
      await delegate(operatorOne, stakingAmount)

      await expectRevert(
        stakingContract.cancelStake(operatorOne, { from: operatorTwo }),
        "Not authorized"
      )
    })

    it("should allow to cancel delegation right away", async () => {
      await delegate(operatorOne, stakingAmount)

      await stakingContract.cancelStake(operatorOne, { from: owner })
      // ok, no revert
    })

    it("should allow to cancel delegation just before initialization period is over", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(
        createdAt.add(initializationPeriod).sub(timeRoundMargin)
      )

      await stakingContract.cancelStake(operatorOne, { from: owner })
      // ok, no revert
    })

    it("should not allow to cancel delegation after initialization period is over", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      await expectRevert(
        stakingContract.cancelStake(operatorOne, { from: owner }),
        "Initialized stake"
      )
    })

    it("should transfer tokens back to the owner", async () => {
      const ownerStartBalance = await token.balanceOf.call(owner)

      await delegate(operatorOne, stakingAmount)

      await stakingContract.cancelStake(operatorOne, { from: owner })

      const ownerEndBalance = await token.balanceOf.call(owner)
      const operatorEndStakeBalance = await stakingContract.balanceOf.call(
        operatorOne
      )

      expect(ownerEndBalance).to.eq.BN(
        ownerStartBalance,
        "Staking amount should be transferred back to owner"
      )
      expect(operatorEndStakeBalance).to.eq.BN(
        0,
        "Staking amount should be removed from operator balance"
      )
    })

    it("should retain delegation info", async () => {
      await delegate(operatorOne, stakingAmount)

      const delegationInfoBefore = await stakingContract.getDelegationInfo.call(
        operatorOne
      )

      await stakingContract.cancelStake(operatorOne, { from: owner })

      const delegationInfoAfter = await stakingContract.getDelegationInfo.call(
        operatorOne
      )

      expect(delegationInfoAfter.createdAt).to.eq.BN(
        delegationInfoBefore.createdAt,
        "Unexpected delegation creation time"
      )
      expect(delegationInfoAfter.amount).to.eq.BN(
        0,
        "Should have no delegated tokens"
      )
      expect(delegationInfoAfter.undelegatedAt).to.eq.BN(
        0,
        "Unexpected undelegation time"
      )
    })

    it("should not allow top-up on a cancelled stake within initialization period", async () => {
      await delegate(operatorOne, stakingAmount)
      await stakingContract.cancelStake(operatorOne, { from: owner })

      // createdAt is preserved after cancel, so receiveApproval routes to topUp
      // (not a new delegation). instantComplete must reject the cancelled operator.
      await expectRevert(
        delegate(operatorOne, stakingAmount),
        "Operator stake already cancelled"
      )
    })

    it("should not allow top-up on a cancelled stake after initialization period passes", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await stakingContract.cancelStake(operatorOne, { from: owner })

      // Advance past init period so the topUp path hits initiate() instead of
      // instantComplete(). Both must reject the cancelled operator.
      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      await expectRevert(
        delegate(operatorOne, stakingAmount),
        "Operator stake already cancelled"
      )
    })
  })

  describe("undelegate", async () => {
    it("should let operator undelegate", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: operatorOne })
      // ok, no revert
    })

    it("should not allow third party to undelegate", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await expectRevert(
        stakingContract.undelegate(operatorOne, { from: operatorTwo }),
        "Not authorized"
      )
    })

    it(
      "should permit undelegating at the time when initialization " +
        "period passed",
      async () => {
        const tx = await delegate(operatorOne, stakingAmount)
        const createdAt = web3.utils.toBN(
          (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
        )

        await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
        await stakingContract.undelegate(operatorOne, { from: operatorOne })
        // ok, no revert
      }
    )

    it(
      "should not permit undelegating at the time before initialization " +
        "period passed",
      async () => {
        const tx = await delegate(operatorOne, stakingAmount)
        const createdAt = web3.utils.toBN(
          (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
        )
        await time.increaseTo(
          createdAt.add(initializationPeriod).sub(timeRoundMargin)
        )
        await expectRevert(
          stakingContract.undelegate(operatorOne, { from: operatorOne }),
          "Invalid timestamp"
        )
      }
    )

    it("should let the operator undelegate earlier", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      const currentTime = await time.latest()

      await stakingContract.undelegateAt(operatorOne, currentTime.addn(20), {
        from: operatorOne,
      })

      await stakingContract.undelegate(operatorOne, { from: operatorOne })
      // ok, no revert
    })

    it("should let the owner postpone undelegation", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: operatorOne })

      await stakingContract.undelegate(operatorOne, { from: owner })
      // ok, no revert
    })

    it("should not let the operator postpone undelegation", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: operatorOne })

      await expectRevert(
        stakingContract.undelegate(operatorOne, { from: operatorOne }),
        "Operator may not postpone"
      )
    })
  })

  describe("undelegateAt", async () => {
    it("should let operator undelegate", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const currentTime = await time.latest()

      await stakingContract.undelegateAt(operatorOne, currentTime.addn(1), {
        from: operatorOne,
      })
      // ok, no revert
    })

    it("should not allow third party to undelegate", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const currentTime = await time.latest()

      await expectRevert(
        stakingContract.undelegateAt(operatorOne, currentTime.addn(10), {
          from: operatorTwo,
        }),
        "Not authorized"
      )
    })

    it("should permit undelegating at the current time", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const currentTime = await time.latest()

      await stakingContract.undelegateAt(operatorOne, currentTime.addn(1), {
        from: operatorOne,
      })
      // ok, no revert
    })

    it(
      "should permit undelegating at the time when initialization " +
        "period passed",
      async () => {
        await delegate(operatorOne, stakingAmount)

        const currentTime = await time.latest()
        await stakingContract.undelegateAt(
          operatorOne,
          currentTime.add(initializationPeriod).addn(1),
          { from: operatorOne }
        )
        // ok, no revert
      }
    )

    it(
      "should not permit undelegating at the time before initialization " +
        "period passed",
      async () => {
        await delegate(operatorOne, stakingAmount)

        const currentTime = await time.latest()
        await expectRevert(
          stakingContract.undelegateAt(
            operatorOne,
            currentTime.add(initializationPeriod).sub(timeRoundMargin),
            { from: operatorOne }
          ),
          "Invalid timestamp"
        )
      }
    )

    it("should not permit undelegating in the past", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const currentTime = await time.latest()

      await expectRevert(
        stakingContract.undelegateAt(operatorOne, currentTime - 1, {
          from: operatorOne,
        }),
        "Invalid timestamp"
      )
    })

    it("should let the operator undelegate earlier", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      const currentTime = await time.latest()

      await stakingContract.undelegateAt(operatorOne, currentTime.addn(20), {
        from: operatorOne,
      })

      await stakingContract.undelegateAt(operatorOne, currentTime.addn(1), {
        from: operatorOne,
      })
      // ok, no revert
    })

    it("should let the owner postpone undelegation", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: operatorOne })

      const currentTime = await time.latest()

      await stakingContract.undelegateAt(operatorOne, currentTime.addn(1), {
        from: owner,
      })
      // ok, no revert
    })

    it("should not let the operator postpone undelegation", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: operatorOne })

      const currentTime = await time.latest()

      await expectRevert(
        stakingContract.undelegateAt(operatorOne, currentTime.addn(1), {
          from: operatorOne,
        }),
        "Operator may not postpone"
      )
    })
  })

  describe("recoverStake", async () => {
    it("should not allow to recover stake without undelegating first", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(
        createdAt.add(initializationPeriod).add(undelegationPeriod)
      )

      await expectRevert(
        stakingContract.recoverStake(operatorOne),
        "Not undelegated"
      )
    })

    it("should not allow to recover stake before undelegation period is over", async () => {
      let tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      tx = await stakingContract.undelegate(operatorOne, { from: owner })
      const undelegatedAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await time.increaseTo(
        undelegatedAt.add(undelegationPeriod).sub(timeRoundMargin)
      )

      await expectRevert(
        stakingContract.recoverStake(operatorOne),
        "Still undelegating"
      )
    })

    it("should transfer tokens back to the owner", async () => {
      const ownerStartBalance = await token.balanceOf.call(owner)

      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      await stakingContract.undelegate(operatorOne, { from: owner })
      await time.increase(undelegationPeriod.addn(1))
      await stakingContract.recoverStake(operatorOne)

      const ownerEndBalance = await token.balanceOf.call(owner)
      const operatorEndStakeBalance = await stakingContract.balanceOf.call(
        operatorOne
      )

      expect(ownerEndBalance).to.eq.BN(
        ownerStartBalance,
        "Staking amount should be transferred back to owner"
      )
      expect(operatorEndStakeBalance).to.eq.BN(
        0,
        "Staking amount should be removed from operator balance"
      )
    })

    it("should retain delegation info", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const delegationInfoBefore = await stakingContract.getDelegationInfo.call(
        operatorOne
      )

      await stakingContract.undelegate(operatorOne, { from: owner })
      const blockNumber = await web3.eth.getBlockNumber()
      const undelegationBlock = await web3.eth.getBlock(blockNumber)

      await time.increase(undelegationPeriod.addn(1))
      await stakingContract.recoverStake(operatorOne)

      const delegationInfoAfter = await stakingContract.getDelegationInfo.call(
        operatorOne
      )

      expect(delegationInfoAfter.createdAt).to.eq.BN(
        delegationInfoBefore.createdAt,
        "Unexpected delegation creation time"
      )
      expect(delegationInfoAfter.amount).to.eq.BN(
        0,
        "Should have no delegated tokens"
      )
      expect(delegationInfoAfter.undelegatedAt).to.eq.BN(
        undelegationBlock.timestamp,
        "Unexpected undelegation time"
      )
    })

    it("should withdraw no more tokens when called twice", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      await delegate(operatorTwo, stakingAmount)
      // staking contract should now have 2 stakingAmount of KEEP

      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      await stakingContract.undelegate(operatorOne, { from: owner })
      await time.increase(undelegationPeriod.addn(1))

      // recover stake and capture owner and staking contract KEEP balances
      await stakingContract.recoverStake(operatorOne)
      const contractBalanceAfter = await token.balanceOf(
        stakingContract.address
      )
      const ownerBalanceAfter = await token.balanceOf(owner)

      // recover stake one more time and see that:
      // - owner KEEP balance hasn't changed
      // - staking contract KEEP balance hasn't changed
      await stakingContract.recoverStake(operatorOne)

      await stakingContract.recoverStake(operatorOne)
      expect(await token.balanceOf.call(owner)).to.eq.BN(
        ownerBalanceAfter,
        "Owner should receive no more tokens"
      )
      expect(await token.balanceOf.call(stakingContract.address)).to.eq.BN(
        contractBalanceAfter,
        "Staking contract should send no more tokens"
      )
    })
  })

  describe("activeStake", async () => {
    it("should report active stake after initialization period is over", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const activeStake = await stakingContract.activeStake.call(
        operatorOne,
        operatorContract
      )

      expect(activeStake).to.eq.BN(
        stakingAmount,
        "Active stake should equal staked amount"
      )
    })

    it("should report no active stake before initialization period is over", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await time.increaseTo(
        createdAt.add(initializationPeriod).sub(timeRoundMargin)
      )

      const activeStake = await stakingContract.activeStake.call(
        operatorOne,
        operatorContract
      )

      expect(activeStake).to.eq.BN(0, "There should be no active stake")
    })

    it("should report no active stake for not authorized operator contract", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const activeStake = await stakingContract.activeStake.call(
        operatorOne,
        operatorContract
      )

      expect(activeStake).to.eq.BN(0, "There should be no active stake")
    })

    it("should report no active stake after cancelling delegation", async () => {
      await delegate(operatorOne, stakingAmount)
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await stakingContract.cancelStake(operatorOne, { from: owner })

      const activeStake = await stakingContract.activeStake.call(
        operatorOne,
        operatorContract
      )

      expect(activeStake).to.eq.BN(0, "There should be no active stake")
    })

    it("should report no active stake after undelegation is finished", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: owner })
      await time.increase(undelegationPeriod.addn(1))

      const activeStake = await stakingContract.activeStake.call(
        operatorOne,
        operatorContract
      )

      expect(activeStake).to.eq.BN(0, "There should be no active stake")
    })
  })

  describe("eligibleStake", async () => {
    it("should report eligible stake after initialization period is over", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )

      expect(eligibleStake).to.eq.BN(
        stakingAmount,
        "Eligible stake should equal staked amount"
      )
    })

    it("should report no eligible stake before initialization period is over", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )
      await time.increaseTo(
        createdAt.add(initializationPeriod).sub(timeRoundMargin)
      )
      const eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )

      expect(eligibleStake).to.eq.BN(0, "There should be no active stake")
    })

    it("should report no eligible stake for not authorized operator contract", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

      const eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )

      expect(eligibleStake).to.eq.BN(0, "There should be no eligible stake")
    })

    it("should report no eligible stake after cancelling delegation", async () => {
      await delegate(operatorOne, stakingAmount)
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await stakingContract.cancelStake(operatorOne, { from: owner })

      const eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )

      expect(eligibleStake).to.eq.BN(0, "There should be no eligible stake")
    })

    it("should report no eligible stake when undelegating", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      await stakingContract.undelegate(operatorOne, { from: owner })

      await time.increase(1)

      const eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )

      expect(eligibleStake).to.eq.BN(0, "There should be no eligible stake")
    })

    it("should report eligible stake for future undelegation", async () => {
      const tx = await delegate(operatorOne, stakingAmount)
      const createdAt = web3.utils.toBN(
        (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
      )
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      const delegationTime = time.duration.seconds(10)

      const currentTime = await time.latest()
      const undelegateAt = currentTime
        .add(initializationPeriod)
        .add(delegationTime)
      await stakingContract.undelegateAt(operatorOne, undelegateAt, {
        from: owner,
      })

      await time.increaseTo(createdAt.add(initializationPeriod).addn(1))
      let eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )
      expect(eligibleStake).to.eq.BN(
        stakingAmount,
        "Eligible stake should equal staked amount"
      )

      await time.increaseTo(undelegateAt.subn(1))
      eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )
      expect(eligibleStake).to.eq.BN(
        stakingAmount,
        "Eligible stake should equal staked amount"
      )
    })

    it("should report no eligible stake for passed future undelegation", async () => {
      await delegate(operatorOne, stakingAmount)
      await stakingContract.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizer }
      )

      const delegationTime = time.duration.seconds(10)

      const currentTime = await time.latest()
      const undelegateAt = currentTime
        .add(initializationPeriod)
        .add(delegationTime)

      await stakingContract.undelegateAt(operatorOne, undelegateAt, {
        from: owner,
      })

      await time.increaseTo(undelegateAt.addn(1))

      const eligibleStake = await stakingContract.eligibleStake.call(
        operatorOne,
        operatorContract
      )
      expect(eligibleStake).to.eq.BN(0, "There should be no active stake")
    })
  })

  describe("transferStakeOwnership", async () => {
    it("fails when not called by staking relationship owner", async () => {
      await delegate(operatorOne, stakingAmount)
      await expectRevert(
        stakingContract.transferStakeOwnership(operatorOne, thirdParty, {
          from: thirdParty,
        }),
        "Not authorized"
      )
    })

    it("transfers stake relationship ownership", async () => {
      await delegate(operatorOne, stakingAmount)
      await stakingContract.transferStakeOwnership(operatorOne, thirdParty, {
        from: owner,
      })
      const newOwner = await stakingContract.ownerOf(operatorOne)
      expect(newOwner).to.equal(thirdParty)
    })

    it("emits an event", async () => {
      await delegate(operatorOne, stakingAmount)
      const receipt = await stakingContract.transferStakeOwnership(
        operatorOne,
        thirdParty,
        { from: owner }
      )

      await expectEvent(receipt, "StakeOwnershipTransferred", {
        operator: operatorOne,
        newOwner: thirdParty,
      })
    })
  })
})
