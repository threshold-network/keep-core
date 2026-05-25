const { contract, accounts, web3 } = require("@openzeppelin/test-environment")

const { initTokenStaking } = require("../helpers/initContracts")

const PermissiveStakingPolicy = contract.fromArtifact("PermissiveStakingPolicy")
const GuaranteedMinimumStakingPolicy = contract.fromArtifact(
  "GuaranteedMinimumStakingPolicy"
)
const AdaptiveStakingPolicy = contract.fromArtifact("AdaptiveStakingPolicy")

const BN = web3.utils.BN
const chai = require("chai")
chai.use(require("bn-chai")(BN))
const expect = chai.expect

describe("PermissiveStakingPolicy", async () => {
  let policy
  const amount = 10000
  const start = 1000
  const duration = 2000
  const cliff = 1500
  const withdrawn = 4000

  before(async () => {
    policy = await PermissiveStakingPolicy.new()
  })

  async function calculate(atTimestamp, withdrawnAmount) {
    return await policy.getStakeableAmount(
      atTimestamp,
      amount,
      duration,
      start,
      cliff,
      withdrawnAmount
    )
  }

  it("should permit staking all tokens before cliff", async () => {
    expect(await calculate(1400, 0)).to.eq.BN(amount)
  })

  it("should permit staking all tokens after cliff", async () => {
    expect(await calculate(1600, 0)).to.eq.BN(amount)
  })

  it("should permit staking all tokens after unlocking", async () => {
    expect(await calculate(3100, 0)).to.eq.BN(amount)
  })

  it("should permit staking remaining tokens before cliff", async () => {
    expect(await calculate(1400, withdrawn)).to.eq.BN(amount - withdrawn)
  })

  it("should permit staking remaining tokens after cliff", async () => {
    expect(await calculate(1600, withdrawn)).to.eq.BN(amount - withdrawn)
  })

  it("should permit staking remaining tokens after unlocking", async () => {
    expect(await calculate(3100, withdrawn)).to.eq.BN(amount - withdrawn)
  })
})

describe("GuaranteedMinimumStakingPolicy", async () => {
  let policy
  let stakingContract
  let minimumStake
  let largeGrant
  let mediumGrant
  let smallGrant
  const start = 1000
  const duration = 2000
  const cliff = 1500

  // Minimum stake is 100,000 KEEP tokens at the beginning.
  // `tokens(n)` returns a BN whose value equals `n` KEEP.
  function tokens(n) {
    return minimumStake.divn(100000).muln(n)
  }

  before(async () => {
    const contracts = await initTokenStaking(
      accounts[9],
      accounts[9],
      accounts[9],
      0,
      contract.fromArtifact("TokenStaking")
    )
    stakingContract = contracts.tokenStaking
    policy = await GuaranteedMinimumStakingPolicy.new(stakingContract.address)
    minimumStake = await stakingContract.minimumStake()
    largeGrant = tokens(500000) // 5x minimum stake
    mediumGrant = tokens(250000) // 2.5x minimum stake
    smallGrant = tokens(50000) // half of minimum stake
  })

  async function calculate(atTimestamp, givenAmount, withdrawnAmount) {
    return await policy.getStakeableAmount(
      atTimestamp,
      givenAmount,
      duration,
      start,
      cliff,
      withdrawnAmount
    )
  }

  describe("with nothing withdrawn", async () => {
    it("should calculate stakeable amount correctly before cliff", async () => {
      expect(await calculate(1499, largeGrant, 0)).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with large grant before cliff"
      )
      expect(await calculate(1499, mediumGrant, 0)).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with medium grant before cliff"
      )
      expect(await calculate(1499, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant before cliff"
      )
    })

    it("should calculate stakeable amount correctly just after cliff", async () => {
      expect(await calculate(1500, largeGrant, 0)).to.eq.BN(
        tokens(125000),
        "Should permit unlocked amount with large grant just after cliff"
      )
      expect(await calculate(1500, mediumGrant, 0)).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with medium grant just after cliff"
      )
      expect(await calculate(1500, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant just after cliff"
      )
    })

    it("should calculate stakeable amount correctly halfway through", async () => {
      expect(await calculate(2000, largeGrant, 0)).to.eq.BN(
        tokens(250000),
        "Should permit unlocked amount with large grant halfway through"
      )
      expect(await calculate(2000, mediumGrant, 0)).to.eq.BN(
        tokens(125000),
        "Should permit unlocked amount with medium grant halfway through"
      )
      expect(await calculate(2000, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant halfway through"
      )
    })

    it("should calculate stakeable amount correctly after unlocking period", async () => {
      expect(await calculate(3000, largeGrant, 0)).to.eq.BN(
        largeGrant,
        "Should permit unlocked amount with large grant after unlocking period"
      )
      expect(await calculate(3000, mediumGrant, 0)).to.eq.BN(
        mediumGrant,
        "Should permit unlocked amount with medium grant after unlocking period"
      )
      expect(await calculate(3000, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant after unlocking period"
      )
    })
  })

  describe("with all unlocked tokens withdrawn", async () => {
    it("should calculate stakeable amount correctly just after cliff", async () => {
      expect(await calculate(1500, largeGrant, tokens(125000))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with large grant just after cliff"
      )
      expect(await calculate(1500, mediumGrant, tokens(62500))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with medium grant just after cliff"
      )
      expect(await calculate(1500, smallGrant, tokens(12500))).to.eq.BN(
        tokens(37500),
        "Should permit remaining amount with small grant just after cliff"
      )
    })

    it("should calculate stakeable amount correctly halfway through", async () => {
      expect(await calculate(2000, largeGrant, tokens(250000))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with large grant halfway through"
      )
      expect(await calculate(2000, mediumGrant, tokens(125000))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with medium grant halfway through"
      )
      expect(await calculate(2000, smallGrant, tokens(25000))).to.eq.BN(
        tokens(25000),
        "Should permit remaining amount with small grant halfway through"
      )
    })

    it("should calculate stakeable amount correctly at three quarters", async () => {
      expect(await calculate(2500, largeGrant, tokens(375000))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with large grant at three quarters"
      )
      expect(await calculate(2500, mediumGrant, tokens(187500))).to.eq.BN(
        tokens(62500),
        "Should permit remaining amount with medium grant at three quarters"
      )
      expect(await calculate(2500, smallGrant, tokens(37500))).to.eq.BN(
        tokens(12500),
        "Should permit remaining amount with small grant at three quarters"
      )
    })
  })

  describe("with half of unlocked tokens withdrawn", async () => {
    it("should calculate stakeable amount correctly just after cliff", async () => {
      expect(await calculate(1500, largeGrant, tokens(62500))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with large grant just after cliff"
      )
      expect(await calculate(1500, mediumGrant, tokens(31250))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with medium grant just after cliff"
      )
      expect(await calculate(1500, smallGrant, tokens(6250))).to.eq.BN(
        tokens(43750),
        "Should permit remaining amount with small grant just after cliff"
      )
    })

    it("should calculate stakeable amount correctly halfway through", async () => {
      expect(await calculate(2000, largeGrant, tokens(125000))).to.eq.BN(
        tokens(125000),
        "Should permit remaining unlocked amount with large grant halfway through"
      )
      expect(await calculate(2000, mediumGrant, tokens(62500))).to.eq.BN(
        minimumStake,
        "Should permit minimum stake with medium grant halfway through"
      )
      expect(await calculate(2000, smallGrant, tokens(12500))).to.eq.BN(
        tokens(37500),
        "Should permit remaining amount with small grant halfway through"
      )
    })
  })
})

describe("AdaptiveStakingPolicy", async () => {
  let cliffPolicy
  let noCliffPolicy
  let stakingContract
  let minimumStake
  let largeGrant
  let mediumGrant
  let smallGrant
  const start = 1000
  const duration = 2000
  const cliff = 2000
  const minimumMultiplier = 4
  const stakeahead = 500

  // Minimum stake is 100,000 KEEP tokens at the beginning.
  // `tokens(n)` returns a BN whose value equals `n` KEEP.
  function tokens(n) {
    return minimumStake.divn(100000).muln(n)
  }

  before(async () => {
    const contracts = await initTokenStaking(
      accounts[9],
      accounts[9],
      accounts[9],
      0,
      contract.fromArtifact("TokenStaking")
    )
    stakingContract = contracts.tokenStaking
    cliffPolicy = await AdaptiveStakingPolicy.new(
      stakingContract.address,
      minimumMultiplier,
      stakeahead,
      true
    )
    noCliffPolicy = await AdaptiveStakingPolicy.new(
      stakingContract.address,
      minimumMultiplier,
      stakeahead,
      false
    )
    minimumStake = await stakingContract.minimumStake()
    largeGrant = tokens(5000000) // 50x minimum stake
    mediumGrant = tokens(500000) // 5x minimum stake
    smallGrant = tokens(50000) // half of minimum stake
  })

  async function withCliff(atTimestamp, givenAmount, withdrawnAmount) {
    return await cliffPolicy.getStakeableAmount(
      atTimestamp,
      givenAmount,
      duration,
      start,
      cliff,
      withdrawnAmount
    )
  }
  async function withoutCliff(atTimestamp, givenAmount, withdrawnAmount) {
    return await noCliffPolicy.getStakeableAmount(
      atTimestamp,
      givenAmount,
      duration,
      start,
      cliff,
      withdrawnAmount
    )
  }

  describe("with nothing withdrawn", async () => {
    it("should calculate stakeable amount correctly before cliff", async () => {
      expect(await withCliff(1499, largeGrant, 0)).to.eq.BN(
        minimumStake.muln(minimumMultiplier),
        "Should permit multiple of minimum stake with large grant before cliff"
      )
      expect(await withCliff(1499, mediumGrant, 0)).to.eq.BN(
        minimumStake.muln(minimumMultiplier),
        "Should permit multiple of minimum stake with medium grant before cliff"
      )
      expect(await withCliff(1499, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant before cliff"
      )
    })

    it("should ignore cliff if specified", async () => {
      expect(await withoutCliff(1250, largeGrant, 0)).to.eq.BN(
        tokens(1875000),
        "Should permit stakeahead with large grant before cliff"
      )
    })

    // cliff at 1000, stakeahead of 500
    it("should calculate stakeable amount correctly just after cliff", async () => {
      expect(await withCliff(1500, largeGrant, 0)).to.eq.BN(
        tokens(2500000),
        "Should permit half with large grant just after cliff"
      )
      expect(await withCliff(1500, mediumGrant, 0)).to.eq.BN(
        minimumStake.muln(minimumMultiplier),
        "Should permit multiple of minimum with medium grant just after cliff"
      )
      expect(await withCliff(1500, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant just after cliff"
      )
    })

    // stakeahead of 500, so 75% is unlocked
    it("should calculate stakeable amount correctly halfway through", async () => {
      expect(await withCliff(2000, largeGrant, 0)).to.eq.BN(
        tokens(3750000),
        "Should permit three quarters with large grant halfway through"
      )
      expect(await withCliff(2000, mediumGrant, 0)).to.eq.BN(
        minimumStake.muln(minimumMultiplier),
        "Should permit multiple of minimum with medium grant halfway through"
      )
      expect(await withCliff(2000, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant halfway through"
      )
    })

    it("should calculate stakeable amount correctly after unlocking period", async () => {
      expect(await withCliff(3000, largeGrant, 0)).to.eq.BN(
        largeGrant,
        "Should permit unlocked amount with large grant after unlocking period"
      )
      expect(await withCliff(3000, mediumGrant, 0)).to.eq.BN(
        mediumGrant,
        "Should permit unlocked amount with medium grant after unlocking period"
      )
      expect(await withCliff(3000, smallGrant, 0)).to.eq.BN(
        smallGrant,
        "Should permit entire grant with small grant after unlocking period"
      )
    })
  })

  describe("with all unlocked tokens withdrawn", async () => {
    it("should calculate stakeable amount correctly just after cliff", async () => {
      expect(await withCliff(2000, largeGrant, tokens(2500000))).to.eq.BN(
        tokens(1250000),
        "Should permit a quarter with large grant just after cliff"
      )
      expect(await withCliff(2000, mediumGrant, tokens(250000))).to.eq.BN(
        tokens(250000),
        "Should permit remaining amount with medium grant just after cliff"
      )
      expect(await withCliff(2000, smallGrant, tokens(25000))).to.eq.BN(
        tokens(25000),
        "Should permit remaining amount with small grant just after cliff"
      )
    })

    it("should calculate stakeable amount correctly at three quarters", async () => {
      expect(await withCliff(2500, largeGrant, tokens(3750000))).to.eq.BN(
        tokens(1250000),
        "Should permit remaining amount with large grant at three quarters"
      )
      expect(await withCliff(2500, mediumGrant, tokens(375000))).to.eq.BN(
        tokens(125000),
        "Should permit remaining amount with medium grant at three quarters"
      )
      expect(await withCliff(2500, smallGrant, tokens(37500))).to.eq.BN(
        tokens(12500),
        "Should permit remaining amount with small grant at three quarters"
      )
    })
  })

  describe("with half of unlocked tokens withdrawn", async () => {
    it("should calculate stakeable amount correctly just after cliff", async () => {
      expect(await withCliff(2000, largeGrant, tokens(1250000))).to.eq.BN(
        tokens(2500000),
        "Should permit half with large grant just after cliff"
      )
      expect(await withCliff(2000, mediumGrant, tokens(125000))).to.eq.BN(
        tokens(375000),
        "Should permit remaining amount with medium grant just after cliff"
      )
      expect(await withCliff(2000, smallGrant, tokens(12500))).to.eq.BN(
        tokens(37500),
        "Should permit remaining amount with small grant just after cliff"
      )
    })

    it("should calculate stakeable amount correctly at three quarters", async () => {
      expect(await withCliff(2500, largeGrant, tokens(1875000))).to.eq.BN(
        tokens(3125000),
        "Should permit remaining amount with large grant at three quarters"
      )
      expect(await withCliff(2500, mediumGrant, tokens(187500))).to.eq.BN(
        tokens(312500),
        "Should permit remaining amount with medium grant at three quarters"
      )
      expect(await withCliff(2500, smallGrant, tokens(18750))).to.eq.BN(
        tokens(31250),
        "Should permit remaining amount with small grant at three quarters"
      )
    })
  })
})
