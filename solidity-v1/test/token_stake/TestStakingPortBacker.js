const { contract, accounts, web3 } = require("@openzeppelin/test-environment")
const {
  expectRevert,
  expectEvent,
  time,
} = require("@openzeppelin/test-helpers")
const { initTokenStaking } = require("../helpers/initContracts")
const { createSnapshot, restoreSnapshot } = require("../helpers/snapshot")

const {
  grantTokens,
  grantTokensToManagedGrant,
} = require("../helpers/grantTokens")
const {
  delegateStake,
  delegateStakeFromGrant,
  delegateStakeFromManagedGrant,
} = require("../helpers/delegateStake")

const KeepToken = contract.fromArtifact("KeepToken")
const KeepRegistry = contract.fromArtifact("KeepRegistry")
const TokenGrant = contract.fromArtifact("TokenGrant")
const PermissiveStakingPolicy = contract.fromArtifact("PermissiveStakingPolicy")
const ManagedGrantFactory = contract.fromArtifact("ManagedGrantFactory")
const ManagedGrant = contract.fromArtifact("ManagedGrant")
const StakingPortBacker = contract.fromArtifact("StakingPortBacker")

const BN = web3.utils.BN
const chai = require("chai")
chai.use(require("bn-chai")(BN))
const expect = chai.expect

describe("TokenStaking/StakingPortBacker", () => {
  const deployer = accounts[0]
  const owner = accounts[1]
  ;(grantManager = accounts[2]),
    (grantee = accounts[3]),
    (managedGrantee = accounts[4]),
    (tokenOwner = accounts[5]),
    (operatorOne = accounts[6]),
    (operatorTwo = accounts[7]),
    (operatorThree = accounts[8]),
    (operatorFour = accounts[9]),
    (thirdParty = accounts[10]),
    (authorizerOne = accounts[11]),
    (authorizerTwo = "0x79a6a0eaA71954Bfe9F5bEE10B5AF2FbadE44994"),
    (authorizerThree = "0x2E9D84B5c9330903314C9312617138FA3735563a")
  ;(beneficiaryOne = "0xa52f52B17dcbDCFEd54C6b9eA5878920974FC69a"),
    (beneficiaryTwo = "0x0A8298210F3037AF8c1526F536683D5E4AEA3803"),
    (beneficiaryThree = "0xbcD762a1493b5350070E6eB93baeC06EE3D47Ea7")

  const initializationPeriod = time.duration.seconds(10)
  const grantStart = time.duration.seconds(0)
  const grantUnlockingDuration = time.duration.years(100)
  const grantCliff = time.duration.seconds(0)
  const grantRevocable = true

  let token
  let tokenGrant
  let registry

  let oldTokenStaking
  let newTokenStaking

  let stakingPortBacker

  let grantId
  let managedGrant
  let grantedAmount
  let delegatedAmount

  before(async () => {
    //
    // Deploy KEEP token contract.
    // Transfer 50% of all tokens to grant manager and 1% of tokens
    // to token owner (account delegating liquid tokens in tests).
    //
    token = await KeepToken.new({ from: deployer })
    const allTokens = await token.balanceOf(deployer)
    await token.transfer(grantManager, allTokens.divn(2), { from: deployer })
    await token.transfer(tokenOwner, allTokens.divn(100), { from: deployer })

    //
    // Deploy TokenGrant, ManagedGrantFactory, KeepRegistry
    //
    tokenGrant = await TokenGrant.new(token.address, { from: deployer })
    const permissivePolicy = await PermissiveStakingPolicy.new()
    const managedGrantFactory = await ManagedGrantFactory.new(
      token.address,
      tokenGrant.address,
      { from: deployer }
    )
    registry = await KeepRegistry.new({ from: deployer })

    //
    // Deploy TokenStaking that will act as the previous
    // staking contract instance.
    //
    const oldStakingContracts = await initTokenStaking(
      token.address,
      tokenGrant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStaking")
    )
    oldTokenStaking = oldStakingContracts.tokenStaking
    await tokenGrant.authorizeStakingContract(oldTokenStaking.address, {
      from: grantManager,
    })

    //
    // Deploy TokenStaking that will act as the new
    // staking contract instance.
    //
    const newStakingContracts = await initTokenStaking(
      token.address,
      tokenGrant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStaking")
    )
    newTokenStaking = newStakingContracts.tokenStaking
    await tokenGrant.authorizeStakingContract(newTokenStaking.address, {
      from: grantManager,
    })

    //
    // Deploy StakingPortBacker - the contract under the test.
    //
    stakingPortBacker = await StakingPortBacker.new(
      token.address,
      tokenGrant.address,
      oldTokenStaking.address,
      newTokenStaking.address,
      { from: deployer }
    )
    await stakingPortBacker.transferOwnership(owner, { from: deployer })

    //
    // Create two grants: standard grant and managed grant.
    //
    const minimumStake = await oldTokenStaking.minimumStake()
    grantedAmount = minimumStake.muln(40)
    delegatedAmount = minimumStake.muln(20)

    grantId = await grantTokens(
      tokenGrant,
      token,
      grantedAmount,
      grantManager,
      grantee,
      grantUnlockingDuration,
      grantStart,
      grantCliff,
      grantRevocable,
      permissivePolicy.address
    )
    const managedGrantAddress = await grantTokensToManagedGrant(
      managedGrantFactory,
      token,
      grantedAmount,
      grantManager,
      managedGrantee,
      grantUnlockingDuration,
      grantStart,
      grantCliff,
      false,
      permissivePolicy.address
    )
    managedGrant = await ManagedGrant.at(managedGrantAddress)

    //
    // Delegate stakes to the old staking contract:
    // - operatorOne receives delegation from liquid tokens of tokenOwner
    // - operatorTwo receives delegation from granted tokens of grantee
    //   from the first grant
    // - operatorThree receives delegation from granted tokens of managed
    //   grantee.
    //
    await delegateStake(
      token,
      oldTokenStaking,
      tokenOwner,
      operatorOne,
      beneficiaryOne,
      authorizerOne,
      delegatedAmount
    )
    await delegateStakeFromGrant(
      tokenGrant,
      oldTokenStaking.address,
      grantee,
      operatorTwo,
      beneficiaryTwo,
      authorizerTwo,
      delegatedAmount,
      grantId
    )
    await delegateStakeFromManagedGrant(
      managedGrant,
      oldTokenStaking.address,
      managedGrantee,
      operatorThree,
      beneficiaryThree,
      authorizerThree,
      delegatedAmount
    )

    // transfer the same amount of tokens as staked in old staking contract
    // to StakingPortBacker.
    const allDelegatedTokens = await token.balanceOf(oldTokenStaking.address)
    await token.transfer(stakingPortBacker.address, allDelegatedTokens, {
      from: deployer,
    })
  })

  beforeEach(async () => {
    await createSnapshot()
  })

  afterEach(async () => {
    await restoreSnapshot()
  })

  describe("allowOperator", async () => {
    it("fails when not called by the owner", async () => {
      await expectRevert(
        stakingPortBacker.allowOperator(operatorOne, { from: deployer }),
        "Ownable: caller is not the owner"
      )
      await expectRevert(
        stakingPortBacker.allowOperator(operatorOne, { from: operatorOne }),
        "Ownable: caller is not the owner"
      )
      await expectRevert(
        stakingPortBacker.allowOperator(operatorOne, { from: tokenOwner }),
        "Ownable: caller is not the owner"
      )
    })

    it("can be called by the owner", async () => {
      await stakingPortBacker.allowOperator(operatorOne, { from: owner })
      // ok, no revert
    })

    it("lets the allowed operator copy its stake", async () => {
      await stakingPortBacker.allowOperator(operatorOne, { from: owner })
      await stakingPortBacker.allowOperator(operatorTwo, { from: owner })
      await stakingPortBacker.allowOperator(operatorThree, { from: owner })

      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })
      // ok, no revert
    })
  })

  describe("allowOperators", async () => {
    it("fails when not called by the owner", async () => {
      await expectRevert(
        stakingPortBacker.allowOperators([operatorOne, operatorTwo], {
          from: deployer,
        }),
        "Ownable: caller is not the owner"
      )
      await expectRevert(
        stakingPortBacker.allowOperators([operatorOne, operatorTwo], {
          from: operatorOne,
        }),
        "Ownable: caller is not the owner"
      )
      await expectRevert(
        stakingPortBacker.allowOperators([operatorOne, operatorTwo], {
          from: tokenOwner,
        }),
        "Ownable: caller is not the owner"
      )
    })

    it("can be called by the owner", async () => {
      await stakingPortBacker.allowOperators(
        [operatorOne, operatorTwo, operatorThree],
        { from: owner }
      )
    })

    it("lets all allowed operators copy their stake", async () => {
      await stakingPortBacker.allowOperators(
        [operatorOne, operatorTwo, operatorThree],
        { from: owner }
      )

      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })
      // ok, no revert
    })
  })

  describe("copyStake", async () => {
    beforeEach(async () => {
      await stakingPortBacker.allowOperators(
        [operatorOne, operatorTwo, operatorThree],
        { from: owner }
      )
    })

    it("fails when operator is not staked on the old contract", async () => {
      await stakingPortBacker.allowOperator(operatorFour, { from: owner })
      await expectRevert(
        stakingPortBacker.copyStake(operatorFour),
        "No stake on the old staking contract"
      )
    })

    it("fails when the operator is not on the allowed operators list", async () => {
      await delegateStake(
        token,
        oldTokenStaking,
        tokenOwner,
        operatorFour,
        beneficiaryOne,
        authorizerOne,
        delegatedAmount
      )

      await expectRevert(
        stakingPortBacker.copyStake(operatorFour),
        "Operator not allowed"
      )
    })

    it("copies liquid tokens stake to the new staking contract", async () => {
      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })

      expect(await newTokenStaking.beneficiaryOf(operatorOne)).to.equal(
        beneficiaryOne
      )
      expect(await newTokenStaking.authorizerOf(operatorOne)).to.equal(
        authorizerOne
      )
      expect(await newTokenStaking.balanceOf(operatorOne)).to.eq.BN(
        delegatedAmount
      )
      expect(await newTokenStaking.ownerOf(operatorOne)).to.equal(
        stakingPortBacker.address
      )
    })

    it("copies grant stake to the new staking contract", async () => {
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })

      expect(await newTokenStaking.beneficiaryOf(operatorTwo)).to.equal(
        beneficiaryTwo
      )
      expect(await newTokenStaking.authorizerOf(operatorTwo)).to.equal(
        authorizerTwo
      )
      expect(await newTokenStaking.balanceOf(operatorTwo)).to.eq.BN(
        delegatedAmount
      )
      expect(await newTokenStaking.ownerOf(operatorTwo)).to.equal(
        stakingPortBacker.address
      )
    })

    it("fails when there is no grant delegation for grantee and operator", async () => {
      await expectRevert(
        stakingPortBacker.copyStake(operatorOne, { from: grantee }),
        "No grant delegated for the operator"
      )
    })

    it("fails when not called by grant delegation owner", async () => {
      await expectRevert(
        stakingPortBacker.copyStake(operatorTwo, { from: operatorTwo }),
        "Not authorized"
      )
      await expectRevert(
        stakingPortBacker.copyStake(operatorTwo, { from: thirdParty }),
        "Not authorized"
      )
    })

    it("copies managed grant stake to the new staking contract", async () => {
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })

      expect(await newTokenStaking.beneficiaryOf(operatorThree)).to.equal(
        beneficiaryThree
      )
      expect(await newTokenStaking.authorizerOf(operatorThree)).to.equal(
        authorizerThree
      )
      expect(await newTokenStaking.balanceOf(operatorThree)).to.eq.BN(
        delegatedAmount
      )
      expect(await newTokenStaking.ownerOf(operatorThree)).to.equal(
        stakingPortBacker.address
      )
    })

    it("allows to copy stake only one time", async () => {
      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })

      await expectRevert(
        stakingPortBacker.copyStake(operatorOne, { from: tokenOwner }),
        "Stake already copied"
      )
      await expectRevert(
        stakingPortBacker.copyStake(operatorTwo, { from: grantee }),
        "Stake already copied"
      )
      await expectRevert(
        stakingPortBacker.copyStake(operatorThree, { from: managedGrantee }),
        "Stake already copied"
      )
    })

    it("allows to copy stake only one time even if it's been recovered", async () => {
      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })

      await time.increase(time.duration.days(91))

      await stakingPortBacker.undelegate(operatorOne, { from: tokenOwner })
      await stakingPortBacker.undelegate(operatorTwo, { from: grantee })
      await stakingPortBacker.undelegate(operatorThree, {
        from: managedGrantee,
      })

      const undelegationPeriod = await newTokenStaking.undelegationPeriod()
      await time.increase(undelegationPeriod.addn(1))

      await stakingPortBacker.recoverStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.recoverStake(operatorTwo, { from: grantee })
      await stakingPortBacker.recoverStake(operatorThree, {
        from: managedGrantee,
      })

      await expectRevert(
        stakingPortBacker.copyStake(operatorOne, { from: tokenOwner }),
        "Stake already copied"
      )
      await expectRevert(
        stakingPortBacker.copyStake(operatorTwo, { from: grantee }),
        "Stake already copied"
      )
      await expectRevert(
        stakingPortBacker.copyStake(operatorThree, { from: managedGrantee }),
        "Stake already copied"
      )
    })

    it("emits an event", async () => {
      const receipt = await stakingPortBacker.copyStake(operatorOne, {
        from: tokenOwner,
      })
      await expectEvent(receipt, "StakeCopied", {
        owner: tokenOwner,
        operator: operatorOne,
        value: delegatedAmount,
      })
    })
  })

  describe("repaying backed balances", async () => {
    beforeEach(async () => {
      await stakingPortBacker.allowOperators(
        [operatorOne, operatorTwo, operatorThree],
        { from: owner }
      )

      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })

      await time.increase(initializationPeriod.addn(1))
    })

    it("fails for unknown token", async () => {
      const anotherToken = await KeepToken.new({ from: deployer })
      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await expectRevert(
        anotherToken.approveAndCall(
          stakingPortBacker.address,
          delegatedAmount,
          data,
          { from: tokenOwner }
        ),
        "Not a KEEP token"
      )
    })

    it("fails for less tokens than backed", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await expectRevert(
        token.approveAndCall(
          stakingPortBacker.address,
          delegatedAmount.subn(1),
          data,
          { from: tokenOwner }
        ),
        "Unexpected amount"
      )
    })

    it("fails for more tokens than backed", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await expectRevert(
        token.approveAndCall(
          stakingPortBacker.address,
          delegatedAmount.addn(1),
          data,
          { from: tokenOwner }
        ),
        "Unexpected amount"
      )
    })

    it("fails for operator with no tokens backed", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorFour])
      await expectRevert(
        token.approveAndCall(stakingPortBacker.address, delegatedAmount, data, {
          from: tokenOwner,
        }),
        "Stake not copied for the operator"
      )
    })

    it("fails for corrupted input data", async () => {
      await expectRevert(
        token.approveAndCall(
          stakingPortBacker.address,
          delegatedAmount,
          web3.eth.abi.encodeParameters(["uint256", "uint256"], [1, 2]),
          { from: tokenOwner }
        ),
        "Corrupted input data"
      )
    })

    it("can be done only one time for the operator", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await token.approveAndCall(
        stakingPortBacker.address,
        delegatedAmount,
        data,
        { from: tokenOwner }
      )
      await expectRevert(
        token.approveAndCall(stakingPortBacker.address, delegatedAmount, data, {
          from: tokenOwner,
        }),
        "Already paid back"
      )
    })

    it("changes liquid tokens staking relationship owner to token owner", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await token.approveAndCall(
        stakingPortBacker.address,
        delegatedAmount,
        data,
        { from: tokenOwner }
      )

      expect(await newTokenStaking.ownerOf(operatorOne)).to.equal(tokenOwner)
    })

    it("changes grant staking relationship owner to grantee", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorTwo])
      await token.transfer(grantee, delegatedAmount, { from: deployer })
      await token.approveAndCall(
        stakingPortBacker.address,
        delegatedAmount,
        data,
        { from: grantee }
      )

      expect(await newTokenStaking.ownerOf(operatorTwo)).to.equal(grantee)
    })

    it("changes managed grant staking relationship owner to managed grantee", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorThree])
      await token.transfer(managedGrantee, delegatedAmount, { from: deployer })
      await token.approveAndCall(
        stakingPortBacker.address,
        delegatedAmount,
        data,
        { from: managedGrantee }
      )

      expect(await newTokenStaking.ownerOf(operatorThree)).to.equal(
        managedGrantee
      )
    })

    it("fails when not done by the eventual staking relationship owner", async () => {
      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await expectRevert(
        token.approveAndCall(stakingPortBacker.address, delegatedAmount, data, {
          from: operatorOne,
        }),
        "Not authorized to pay back"
      )
      await expectRevert(
        token.approveAndCall(stakingPortBacker.address, delegatedAmount, data, {
          from: thirdParty,
        }),
        "Not authorized to pay back"
      )
    })

    it("expects the original amount to be repaid if slashed", async () => {
      // let's assume third party address is the operator contract
      const operatorContract = thirdParty

      await registry.approveOperatorContract(operatorContract, {
        from: deployer,
      })
      await newTokenStaking.authorizeOperatorContract(
        operatorOne,
        operatorContract,
        { from: authorizerOne }
      )

      const amountToSlash = 10000
      await newTokenStaking.slash(amountToSlash, [operatorOne], {
        from: operatorContract,
      })
      const currentBalance = await newTokenStaking.balanceOf(operatorOne)

      const data = web3.eth.abi.encodeParameters(["address"], [operatorOne])
      await expectRevert(
        token.approveAndCall(stakingPortBacker.address, currentBalance, data, {
          from: tokenOwner,
        }),
        "Unexpected amount"
      ) // reverts - tokens were slashed but we expect the original amount to
      // be repaid;

      await token.approveAndCall(
        stakingPortBacker.address,
        delegatedAmount,
        data,
        { from: tokenOwner }
      )
      // ok, no revert - the original copied amount has been paid back
    })
  })

  describe("withdraw", async () => {
    it("fails when not called by the owner", async () => {
      await expectRevert(
        stakingPortBacker.withdraw(1000, { from: thirdParty }),
        "Ownable: caller is not the owner"
      )
    })

    it("allows owner to withdraw tokens", async () => {
      const balanceBefore = await token.balanceOf(owner)
      await stakingPortBacker.withdraw(9999, { from: owner })
      const balanceAfter = await token.balanceOf(owner)

      expect(balanceAfter.sub(balanceBefore)).to.eq.BN(9999)
    })
  })

  describe("undelegate", async () => {
    beforeEach(async () => {
      await stakingPortBacker.allowOperators(
        [operatorOne, operatorTwo, operatorThree],
        { from: owner }
      )
      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
      await stakingPortBacker.copyStake(operatorTwo, { from: grantee })
      await stakingPortBacker.copyStake(operatorThree, { from: managedGrantee })

      await time.increase(initializationPeriod.addn(1))
    })

    it("fails when not called by the relationship owner or operator", async () => {
      await expectRevert(
        stakingPortBacker.undelegate(operatorOne, { from: owner }),
        "Not authorized"
      )
      await expectRevert(
        stakingPortBacker.undelegate(operatorOne, { from: grantee }),
        "Not authorized"
      )
      await expectRevert(
        stakingPortBacker.undelegate(operatorTwo, { from: tokenOwner }),
        "Not authorized"
      )
    })

    it("can be called by the relationship owner", async () => {
      await stakingPortBacker.undelegate(operatorOne, { from: tokenOwner })
      await stakingPortBacker.undelegate(operatorTwo, { from: grantee })
      await stakingPortBacker.undelegate(operatorThree, {
        from: managedGrantee,
      })
      // ok, no revert
    })

    it("can be called by the operator", async () => {
      await stakingPortBacker.undelegate(operatorOne, { from: operatorOne })
      await stakingPortBacker.undelegate(operatorTwo, { from: operatorTwo })
      await stakingPortBacker.undelegate(operatorThree, { from: operatorThree })
      // ok, no revert
    })

    it("undelegates stake from the operator", async () => {
      await stakingPortBacker.undelegate(operatorThree, {
        from: managedGrantee,
      })
      const delegationInfo = await newTokenStaking.getDelegationInfo(
        operatorThree
      )
      expect(delegationInfo.undelegatedAt).to.gt.BN(0)
      expect(delegationInfo.undelegatedAt).to.lte.BN(await time.latest())
    })
  })

  describe("forceUndelegate", async () => {
    beforeEach(async () => {
      await stakingPortBacker.allowOperator(operatorOne, { from: owner })
      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
    })

    it("fails when not called by the owner", async () => {
      await expectRevert(
        stakingPortBacker.forceUndelegate(operatorOne, { from: operatorOne }),
        "Ownable: caller is not the owner"
      )
    })

    it("fails when the maximum backing duration has not passed yet", async () => {
      await time.increase(time.duration.days(59))
      await expectRevert(
        stakingPortBacker.forceUndelegate(operatorOne, { from: owner }),
        "Maximum allowed backing duration not exceeded yet"
      )
    })

    it("undelegates stake from the operator", async () => {
      await time.increase(time.duration.days(91))
      await stakingPortBacker.forceUndelegate(operatorOne, { from: owner })
      const delegationInfo = await newTokenStaking.getDelegationInfo(
        operatorOne
      )
      expect(delegationInfo.undelegatedAt).to.gt.BN(0)
      expect(delegationInfo.undelegatedAt).to.lte.BN(await time.latest())
    })
  })

  describe("recoverStake", async () => {
    beforeEach(async () => {
      await stakingPortBacker.allowOperator(operatorOne, { from: owner })
      await stakingPortBacker.copyStake(operatorOne, { from: tokenOwner })
    })

    it("allows to recover previously undelegated stake", async () => {
      await time.increase(time.duration.days(91))
      await stakingPortBacker.undelegate(operatorOne, { from: tokenOwner })
      const undelegationPeriod = await newTokenStaking.undelegationPeriod()
      await time.increase(undelegationPeriod.addn(1))

      const balanceBefore = await token.balanceOf(stakingPortBacker.address)
      await stakingPortBacker.recoverStake(operatorOne, { from: owner })
      const balanceAfter = await token.balanceOf(stakingPortBacker.address)

      expect(balanceAfter.sub(balanceBefore)).to.eq.BN(delegatedAmount)
    })
  })
})
