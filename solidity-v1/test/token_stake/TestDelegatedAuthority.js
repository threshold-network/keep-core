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
const DelegatedAuthorityStub = contract.fromArtifact("DelegatedAuthorityStub")

const initializationPeriod = time.duration.seconds(10)

let token
let registry
let stakingContract
let authorityDelegator
let badAuthorityDelegator
let innerRecursiveDelegator
let outerRecursiveDelegator
let minimumStake
let stakingAmount

describe("TokenStaking/DelegatedAuthority", async () => {
  const owner = accounts[0]
  const operator = accounts[1]
  const beneficiary = accounts[2]
  const authorizer = accounts[3]
  const recognizedContract = accounts[4]
  const unrecognizedContract = accounts[5]
  const unapprovedContract = accounts[6]
  const recursivelyAuthorizedContract = accounts[7]

  before(async () => {
    token = await KeepToken.new({ from: accounts[0] })
    grant = await TokenGrant.new(token.address, { from: accounts[0] })
    registry = await KeepRegistry.new()
    const stakingContracts = await initTokenStaking(
      token.address,
      grant.address,
      registry.address,
      initializationPeriod,
      contract.fromArtifact("TokenStaking")
    )
    stakingContract = stakingContracts.tokenStaking

    minimumStake = await stakingContract.minimumStake()
    stakingAmount = minimumStake.muln(20)
    const tx = await delegate(operator, stakingAmount)
    const createdAt = web3.utils.toBN(
      (await web3.eth.getBlock(tx.receipt.blockNumber)).timestamp
    )
    await time.increaseTo(createdAt.add(initializationPeriod).addn(1))

    authorityDelegator = await DelegatedAuthorityStub.new(recognizedContract)
    badAuthorityDelegator = await DelegatedAuthorityStub.new(unapprovedContract)
    await registry.approveOperatorContract(authorityDelegator.address)

    innerRecursiveDelegator = await DelegatedAuthorityStub.new(
      recursivelyAuthorizedContract
    )
    outerRecursiveDelegator = await DelegatedAuthorityStub.new(
      innerRecursiveDelegator.address
    )
    await registry.approveOperatorContract(outerRecursiveDelegator.address)
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

  async function hasDelegatedAuthorization(operatorContract) {
    return stakingContract.isAuthorizedForOperator(operator, operatorContract)
  }

  async function authorize(operatorContract) {
    stakingContract.authorizeOperatorContract(
      operator,
      operatorContract.address,
      { from: authorizer }
    )
  }

  async function disable(operatorContract) {
    await registry.disableOperatorContract(operatorContract.address)
  }

  describe("claimDelegatedAuthority", async () => {
    it("lets contracts claim delegated authority", async () => {
      await stakingContract.claimDelegatedAuthority(
        authorityDelegator.address,
        { from: recognizedContract }
      )

      expect(
        await stakingContract.getAuthoritySource(recognizedContract)
      ).to.equal(authorityDelegator.address)
    })

    it("doesn't give unrecognized contracts delegated authority", async () => {
      await expectRevert(
        stakingContract.claimDelegatedAuthority(authorityDelegator.address, {
          from: unrecognizedContract,
        }),
        "Unrecognized claimant"
      )
    })

    it("doesn't give delegated authority through unapproved contracts", async () => {
      await expectRevert(
        stakingContract.claimDelegatedAuthority(badAuthorityDelegator.address, {
          from: unapprovedContract,
        }),
        "Operator contract unapproved"
      )
    })

    it("delegates authority recursively", async () => {
      await innerRecursiveDelegator.claimAuthorityRecursively(
        stakingContract.address,
        outerRecursiveDelegator.address
      )
      await stakingContract.claimDelegatedAuthority(
        innerRecursiveDelegator.address,
        { from: recursivelyAuthorizedContract }
      )

      expect(
        await stakingContract.getAuthoritySource(recursivelyAuthorizedContract)
      ).to.equal(outerRecursiveDelegator.address)
    })
  })

  describe("after authority delegation", async () => {
    before(async () => {
      await stakingContract.claimDelegatedAuthority(
        authorityDelegator.address,
        { from: recognizedContract }
      )
    })

    describe("isAuthorizedForOperator", async () => {
      before(async () => {
        await stakingContract.claimDelegatedAuthority(
          authorityDelegator.address,
          { from: recognizedContract }
        )
      })

      it("delegates authorization correctly", async () => {
        expect(await hasDelegatedAuthorization(recognizedContract)).to.be.false
        await authorize(authorityDelegator)
        expect(await hasDelegatedAuthorization(recognizedContract)).to.be.true
      })

      it("disables delegated authorization with the panic button", async () => {
        await authorize(authorityDelegator)
        await disable(authorityDelegator)
        // Indirect test;
        // `claimDelegatedAuthority` checks `onlyApprovedOperatorContract`
        await expectRevert(
          stakingContract.claimDelegatedAuthority(recognizedContract, {
            from: unrecognizedContract,
          }),
          "Operator contract unapproved"
        )
      })

      it("works recursively", async () => {
        await innerRecursiveDelegator.claimAuthorityRecursively(
          stakingContract.address,
          outerRecursiveDelegator.address
        )
        await stakingContract.claimDelegatedAuthority(
          innerRecursiveDelegator.address,
          { from: recursivelyAuthorizedContract }
        )
        await authorize(outerRecursiveDelegator)
        expect(await hasDelegatedAuthorization(recursivelyAuthorizedContract))
          .to.be.true
      })
    })

    describe("authorizeOperatorContract", async () => {
      it("doesn't authorize contracts using delegated authority", async () => {
        await expectRevert(
          stakingContract.authorizeOperatorContract(
            operator,
            recognizedContract,
            { from: authorizer }
          ),
          "Delegated authority used"
        )
      })
    })

    describe("slash", async () => {
      it("uses delegated authorization correctly", async () => {
        await expectRevert(
          stakingContract.slash(minimumStake, [operator], {
            from: recognizedContract,
          }),
          "Not authorized"
        )
        await authorize(authorityDelegator)
        await stakingContract.slash(minimumStake, [operator], {
          from: recognizedContract,
        })
        // no error
      })
    })

    describe("seize", async () => {
      it("uses delegated authorization correctly", async () => {
        await expectRevert(
          stakingContract.seize(minimumStake, 100, beneficiary, [operator], {
            from: recognizedContract,
          }),
          "Not authorized"
        )
        await authorize(authorityDelegator)
        await stakingContract.seize(
          minimumStake,
          100,
          beneficiary,
          [operator],
          { from: recognizedContract }
        )
        // no error
      })
    })

    describe("lockStake", async () => {
      it("uses delegated authorization correctly", async () => {
        const lockPeriod = time.duration.weeks(12)
        await expectRevert(
          stakingContract.lockStake(operator, lockPeriod, {
            from: recognizedContract,
          }),
          "Not authorized"
        )
        await authorize(authorityDelegator)
        await stakingContract.lockStake(operator, lockPeriod, {
          from: recognizedContract,
        })
        // no error
      })
    })

    describe("releaseExpiredLock", async () => {
      it("reverts for authority delegator", async () => {
        await authorize(authorityDelegator)
        const lockPeriod = time.duration.weeks(12)
        await stakingContract.lockStake(operator, lockPeriod, {
          from: recognizedContract,
        })

        await expectRevert(
          stakingContract.releaseExpiredLock(
            operator,
            authorityDelegator.address
          ),
          "No matching lock present"
        )
      })

      it("uses delegated authorization correctly and validates expiration", async () => {
        await authorize(authorityDelegator)
        const lockPeriod = time.duration.weeks(12)
        await stakingContract.lockStake(operator, lockPeriod, {
          from: recognizedContract,
        })

        await expectRevert(
          stakingContract.releaseExpiredLock(
            operator,
            authorityDelegator.address
          ),
          "No matching lock present"
        )

        await expectRevert(
          stakingContract.releaseExpiredLock(operator, recognizedContract),
          "Lock still active and valid"
        )

        time.increase(lockPeriod.addn(1))

        await stakingContract.releaseExpiredLock(operator, recognizedContract)
        // no error
      })

      it("uses delegated authorization correctly and checks if operator contract is enabled", async () => {
        await authorize(authorityDelegator)
        const lockPeriod = time.duration.weeks(12)
        await stakingContract.lockStake(operator, lockPeriod, {
          from: recognizedContract,
        })

        await expectRevert(
          stakingContract.releaseExpiredLock(operator, recognizedContract),
          "Lock still active and valid"
        )

        await disable(authorityDelegator)

        await stakingContract.releaseExpiredLock(operator, recognizedContract)
        // no error
      })
    })

    describe("isStakeLocked", async () => {
      it("uses delegated authorization correctly", async () => {
        await authorize(authorityDelegator)
        const lockPeriod = time.duration.weeks(12)
        await stakingContract.lockStake(operator, lockPeriod, {
          from: recognizedContract,
        })

        expect(await stakingContract.isStakeLocked(operator)).to.be.true
      })
    })

    describe("eligibleStake", async () => {
      it("uses delegated authorization correctly", async () => {
        expect(
          await stakingContract.eligibleStake(operator, recognizedContract)
        ).to.eq.BN(0)
        await authorize(authorityDelegator)
        expect(
          await stakingContract.eligibleStake(operator, recognizedContract)
        ).to.eq.BN(stakingAmount)
      })
    })

    describe("activeStake", async () => {
      it("uses delegated authorization correctly", async () => {
        expect(
          await stakingContract.activeStake(operator, recognizedContract)
        ).to.eq.BN(0)
        await authorize(authorityDelegator)
        expect(
          await stakingContract.activeStake(operator, recognizedContract)
        ).to.eq.BN(stakingAmount)
      })
    })
  })
})
