const { contract, accounts, web3 } = require("@openzeppelin/test-environment")
const { expectRevert, time } = require("@openzeppelin/test-helpers")
const { initTokenStaking } = require("../helpers/initContracts")
const { grantTokens } = require("../helpers/grantTokens")

const KeepToken = contract.fromArtifact("KeepToken")
const TokenGrant = contract.fromArtifact("TokenGrant")
const KeepRegistry = contract.fromArtifact("KeepRegistry")
const PermissiveStakingPolicy = contract.fromArtifact("PermissiveStakingPolicy")

const assert = require("chai").assert

describe("TokenGrant", function () {
  let token
  let registry
  let grantContract
  let stakingContract
  let permissivePolicy
  let amount
  let unlockingDuration
  let start
  let cliff
  const grantManager = accounts[0]
  const accountTwo = accounts[1]
  const grantee = accounts[2]

  before(async () => {
    token = await KeepToken.new({ from: accounts[0] })
    registry = await KeepRegistry.new({ from: accounts[0] })
    grantContract = await TokenGrant.new(token.address, { from: accounts[0] })
    const contracts = await initTokenStaking(
      token.address,
      grantContract.address,
      registry.address,
      time.duration.days(1),
      contract.fromArtifact("TokenStaking")
    )
    stakingContract = contracts.tokenStaking

    await grantContract.authorizeStakingContract(stakingContract.address, {
      from: accounts[0],
    })
    permissivePolicy = await PermissiveStakingPolicy.new()
    amount = web3.utils.toBN(100)
    unlockingDuration = time.duration.days(30)
    start = await time.latest()
    cliff = time.duration.days(0)
  })

  it("should grant tokens correctly", async function () {
    const amount = web3.utils.toBN(1000000000)
    const unlockingDuration = time.duration.days(30)
    const start = await time.latest()
    const cliff = time.duration.days(10)
    const revocable = true

    // Starting balances
    const grantManagerStartingBalance = await token.balanceOf.call(grantManager)
    const accountTwoStartingBalance = await token.balanceOf.call(accountTwo)

    // Grant tokens
    const id = await grantTokens(
      grantContract,
      token,
      amount,
      grantManager,
      accountTwo,
      unlockingDuration,
      start,
      cliff,
      revocable,
      permissivePolicy.address,
      { from: accounts[0] }
    )

    // Ending balances
    const grantManagerEndingBalance = await token.balanceOf.call(grantManager)
    let accountTwoEndingBalance = await token.balanceOf.call(accountTwo)
    let accountTwoGrantBalance = await grantContract.balanceOf.call(accountTwo)

    assert.equal(
      grantManagerEndingBalance.eq(grantManagerStartingBalance.sub(amount)),
      true,
      "Amount should be transfered from sender balance"
    )
    assert.equal(
      accountTwoGrantBalance.eq(amount),
      true,
      "Amount should be added to the grantee grant balance"
    )
    assert.equal(
      accountTwoEndingBalance.eq(accountTwoStartingBalance),
      true,
      "Grantee main balance should stay unchanged"
    )

    // Should not be able to withdraw token grant (0 withdrawable amount)
    await expectRevert(
      grantContract.withdraw(id),
      "Grant available to withdraw amount should be greater than zero"
    )

    // jump in time, third unlocking duration
    await time.increase(unlockingDuration.divn(3))

    // Should be able to withdraw token grant withdrawable amount
    await grantContract.withdraw(id)

    // should withdraw some of grant to the main balance
    accountTwoEndingBalance = await token.balanceOf.call(accountTwo)
    assert.equal(
      accountTwoEndingBalance.lte(
        accountTwoStartingBalance.add(amount.div(web3.utils.toBN(2)))
      ),
      true,
      "Should withdraw some of the grant to the main balance"
    )

    // jump in time, full unlocking duration
    await time.increase(unlockingDuration)
    await grantContract.withdraw(id)

    // should withdraw full grant amount to the main balance
    accountTwoEndingBalance = await token.balanceOf.call(accountTwo)
    assert.equal(
      accountTwoEndingBalance.eq(accountTwoStartingBalance.add(amount)),
      true,
      "Should withdraw full grant amount to the main balance"
    )

    accountTwoGrantBalance = await grantContract.balanceOf.call(accountTwo)
    assert.equal(accountTwoGrantBalance, 0, "Grant amount should become 0")
  })

  it("token holder should be able to grant it's tokens to a grantee.", async function () {
    const grantManagerStartingBalance = await token.balanceOf.call(grantManager)

    const id = await grantTokens(
      grantContract,
      token,
      amount,
      grantManager,
      grantee,
      unlockingDuration,
      start,
      cliff,
      true,
      permissivePolicy.address,
      { from: accounts[0] }
    )

    const grantManagerEndingBalance = await token.balanceOf.call(grantManager)

    assert.equal(
      grantManagerEndingBalance.eq(grantManagerStartingBalance.sub(amount)),
      true,
      "Amount should be taken out from grant manager main balance."
    )
    assert.equal(
      (await grantContract.balanceOf.call(grantee)).eq(amount),
      true,
      "Amount should be added to grantee's granted balance."
    )

    const grant = await grantContract.getGrant(id)
    assert.equal(
      grant[0].eq(amount),
      true,
      "Grant should maintain a record of the granted amount."
    )
    assert.equal(
      grant[1].isZero(),
      true,
      "Grant should have 0 amount withdrawn initially."
    )
    assert.equal(grant[2], false, "Grant should initially be undelegated.")
    assert.equal(
      grant[3],
      false,
      "Grant should not be marked as revoked initially."
    )

    const schedule = await grantContract.getGrantUnlockingSchedule(id)
    assert.equal(
      schedule[0],
      grantManager,
      "Grant should maintain a record of the grant manager."
    )
    assert.equal(
      schedule[1].eq(web3.utils.toBN(unlockingDuration)),
      true,
      "Grant should have unlocking schedule time.duration."
    )
    assert.equal(
      schedule[2].eq(web3.utils.toBN(start)),
      true,
      "Grant should have start time."
    )
    assert.equal(
      schedule[3].eq(web3.utils.toBN(start).add(web3.utils.toBN(cliff))),
      true,
      "Grant should have unlocking schedule cliff time.duration."
    )
  })

  it("can assign a different address than the sender as grant manager", async () => {
    // Assign `account_two` as grant manager
    const grantData = web3.eth.abi.encodeParameters(
      [
        "address",
        "address",
        "uint256",
        "uint256",
        "uint256",
        "bool",
        "address",
      ],
      [
        accountTwo,
        grantee,
        unlockingDuration.toNumber(),
        start.toNumber(),
        cliff.toNumber(),
        false,
        permissivePolicy.address,
      ]
    )

    await token.approveAndCall(grantContract.address, amount, grantData, {
      from: grantManager,
    })
    const grantId = (await grantContract.getPastEvents())[0].args[0].toNumber()

    const schedule = await grantContract.getGrantUnlockingSchedule(grantId)
    assert.equal(
      schedule[0],
      accountTwo,
      "The grant manager should be assignable to a non-sender"
    )
  })
})
