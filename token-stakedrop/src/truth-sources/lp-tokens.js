/** @typedef { import("../lib/context").Context } Context */
/** @typedef { import("../lib/ethereum-helper").Address } Address */

import BN from "bn.js"

import { ITruthSource } from "./truth-source.js"
import { getPastEvents, getChainID } from "../lib/ethereum-helper.js"
import { dumpDataToFile } from "../lib/file-helper.js"
import { logger } from "../lib/winston.js"
import { getDeploymentBlockNumber } from "../lib/contract-helper.js"
import { EthereumHelpers } from "@keep-network/tbtc.js"
const { callWithRetry } = EthereumHelpers

// https://etherscan.io/address/0xe6f19dab7d43317344282f803f8e8d240708174a#code
import keepEthTokenJson from "../../artifacts/KEEP-ETH-UNI-V2-Token.json"
// https://etherscan.io/address/0x38c8ffee49f286f25d25bad919ff7552e5daf081#code
import keepTbtcTokenJson from "../../artifacts/KEEP-TBTC-UNI-V2-Token.json"

import LPRewardsKEEPETHJson from "@keep-network/keep-ecdsa/artifacts/LPRewardsKEEPETH.json"
import LPRewardsKEEPTBTCJson from "@keep-network/keep-ecdsa/artifacts/LPRewardsKEEPTBTC.json"

// https://info.uniswap.org/pair/0xe6f19dab7d43317344282f803f8e8d240708174a
const KEEPETH_PAIR_ADDRESS = "0xe6f19dab7d43317344282f803f8e8d240708174a"
// https://info.uniswap.org/pair/0x38c8ffee49f286f25d25bad919ff7552e5daf081
const KEEPTBTC_PAIR_ADDRESS = "0x38c8ffee49f286f25d25bad919ff7552e5daf081"

const KEEP_IN_LP_KEEPETH_BALANCES_PATH =
  "./tmp/keep-in-lp-keepeth-token-balances.json"

const KEEP_IN_LP_KEEPTBTC_BALANCES_PATH =
  "./tmp/keep-in-lp-keeptbtc-token-balances.json"

export class LPTokenTruthSource extends ITruthSource {
  /**
   * @param {Context} context
   * @param {Number} targetBlock
   */
  constructor(context, targetBlock) {
    super(context, targetBlock)
  }

  async initialize() {
    const chainID = await getChainID(this.context.web3)

    const keepEthTokenContract = EthereumHelpers.buildContract(
      this.context.web3,
      JSON.parse(keepEthTokenJson.result),
      KEEPETH_PAIR_ADDRESS
    )

    const lpRewardKeepEthContract = EthereumHelpers.buildContract(
      this.context.web3,
      LPRewardsKEEPETHJson.abi,
      LPRewardsKEEPETHJson.networks[chainID].address
    )

    const lpRewardKeepEthDeploymentBlock = await getDeploymentBlockNumber(
      LPRewardsKEEPETHJson,
      this.context.web3
    )

    const keepTbtcTokenContract = EthereumHelpers.buildContract(
      this.context.web3,
      JSON.parse(keepTbtcTokenJson.result),
      KEEPTBTC_PAIR_ADDRESS
    )

    const lpRewardKeepTbtcContract = EthereumHelpers.buildContract(
      this.context.web3,
      LPRewardsKEEPTBTCJson.abi,
      LPRewardsKEEPTBTCJson.networks[chainID].address
    )

    const lpRewardKeepTbtcDeploymentBlock = await getDeploymentBlockNumber(
      LPRewardsKEEPTBTCJson,
      this.context.web3
    )

    this.liquidityStaking = {
      KEEPETH: {
        lpTokenContract: keepEthTokenContract,
        lpRewardsContract: lpRewardKeepEthContract,
        lpRewardsContractName: "LPRewardsKEEPETH",
        lpRewardsContractDeploymentBlock: lpRewardKeepEthDeploymentBlock,
        keepInLpTokenFilePath: KEEP_IN_LP_KEEPETH_BALANCES_PATH,
        lpPairAddress: KEEPETH_PAIR_ADDRESS,
      },
      KEEPTBTC: {
        lpTokenContract: keepTbtcTokenContract,
        lpRewardsContract: lpRewardKeepTbtcContract,
        lpRewardsContractName: "LPRewardsKEEPTBTC",
        lpRewardsContractDeploymentBlock: lpRewardKeepTbtcDeploymentBlock,
        keepInLpTokenFilePath: KEEP_IN_LP_KEEPTBTC_BALANCES_PATH,
        lpPairAddress: KEEPTBTC_PAIR_ADDRESS,
      },
    }
  }

  /**
   * Finds all historic stakers in LP Rewards contracts based on "Staked" events
   *
   * @param {String} pairName LP pair name
   * @param {Object} pairObj LP pair object
   *
   * @return {Array<Address>} All historic LP token stakers
   * */
  async findStakers(pairName, pairObj) {
    const lpRewardsContractAddress = pairObj.lpRewardsContract.options.address

    logger.info(
      `looking for "Staked" events emitted from ${pairObj.lpRewardsContractName} ` +
        `contract at ${lpRewardsContractAddress} when staking ${pairName} pair ` +
        `${pairObj.lpTokenContract.options.address} between blocks ` +
        `${pairObj.lpRewardsContractDeploymentBlock} and ${this.targetBlock}`
    )

    const events = await getPastEvents(
      this.context.web3,
      pairObj.lpRewardsContract,
      "Staked",
      pairObj.lpRewardsContractDeploymentBlock,
      this.targetBlock
    )
    logger.info(`found ${events.length} LP ${pairName} token staked events`)

    const lpTokenStakersSet = new Set()
    events.forEach((event) => {
      lpTokenStakersSet.add(event.returnValues.user)
    })

    logger.info(`found ${lpTokenStakersSet.size} unique historic stakers`)

    return Array.from(lpTokenStakersSet)
  }

  /**
   * Retrieves balances of LP tokens locked in LPRewards contracts for stakers.
   *
   * @param {Array<Address>} lpStakers LP stakers addresses.
   * @param {Object} pairObj LP pair object.
   *
   * @return {Map<Address,BN>} Staked LP balances by lp stakers.
   */
  async getLpTokenStakersBalances(lpStakers, pairObj) {
    const lpBalanceByStaker = new Map()
    let expectedTotalSupply = new BN(0)

    logger.info(`looking for active stakers at block ${this.targetBlock}`)
    for (const staker of lpStakers) {
      const lpBalance = new BN(
        await callWithRetry(
          pairObj.lpRewardsContract.methods.balanceOf(staker),
          undefined,
          undefined,
          this.targetBlock
        )
      )
      if (!lpBalance.isZero()) {
        lpBalanceByStaker.set(staker, lpBalance)
        logger.info(
          `found an active staker ${staker} with the balance of ${lpBalance}`
        )
        expectedTotalSupply = expectedTotalSupply.add(lpBalance)
      }
    }
    const actualTotalSupply = new BN(
      await callWithRetry(
        pairObj.lpRewardsContract.methods.totalSupply(),
        undefined,
        undefined,
        this.targetBlock
      )
    )

    if (!expectedTotalSupply.eq(actualTotalSupply)) {
      logger.error(
        `sum of LP staker balances ${expectedTotalSupply} does not match
        the total supply ${actualTotalSupply} of locked LP tokens
        in ${pairObj.lpRewardsContractName} contract`
      )
    }

    logger.info(
      `total supply of LP Tokens locked in ${
        pairObj.lpRewardsContractName
      } is: ${expectedTotalSupply.toString()}`
    )

    return lpBalanceByStaker
  }

  /**
   * Calculates KEEP amount for all LP stakers.
   *
   * @param {Map<Address, BN>} stakersBalances LP Token amounts by stakers
   * @param {String} pairName LP pair name
   * @param {Object} pairObj LP pair object
   *
   * @return {Map<Address,BN>} KEEP amounts in LP Token at the target block
   */
  async calcKeepInStakersBalances(stakersBalances, pairName, pairObj) {
    const totalSupply = await callWithRetry(
      pairObj.lpTokenContract.methods.totalSupply(),
      undefined,
      undefined,
      this.targetBlock
    )
    logger.info(
      `total supply of LP ${pairName} token at block ${this.targetBlock} is: ${totalSupply}`
    )

    const lpReserves = await callWithRetry(
      pairObj.lpTokenContract.methods.getReserves(),
      undefined,
      undefined,
      this.targetBlock
    )
    logger.info(
      `KEEP reserve in the KEEP liquidity pool at block ${this.targetBlock} is: ${lpReserves._reserve0}`
    )

    // Token reserve0 must be KEEP token 0x85eee30c52b0b379b046fb0f85f4f3dc3009afec
    // which makes a LP pair token.
    const lpPairData = {
      keepLiquidityPool: lpReserves._reserve0,
      lpTotalSupply: totalSupply,
    }

    let totalLpStaked = new BN(0)
    let totalKeepInLpStaked = new BN(0)
    const keepInLpByStakers = new Map()

    for (const [stakerAddress, lpStakerBalance] of stakersBalances.entries()) {
      const keepInLPToken = await this.calcKeepTokenfromLPToken(
        lpStakerBalance,
        lpPairData
      )
      keepInLpByStakers.set(stakerAddress, keepInLPToken)
      totalLpStaked = totalLpStaked.add(lpStakerBalance)
      totalKeepInLpStaked = totalKeepInLpStaked.add(keepInLPToken)

      logger.info(
        `staker: ${stakerAddress} - LP ${pairName} Balance: ${lpStakerBalance} - KEEP in LP: ${keepInLPToken}`
      )
    }

    logger.info(
      `found ${keepInLpByStakers.size} active stakers with the total of ` +
        `${totalLpStaked} LP ${pairName} tokens staked. The total of KEEP asset is ${totalKeepInLpStaked}`
    )

    if (totalKeepInLpStaked.lte(lpPairData.keepLiquidityPool)) {
      logger.error(
        `total of KEEP asset ${totalKeepInLpStaked} in LP tokens must be less or equal ` +
          `the total amount in the KEEP liquidity reserve ${lpPairData.keepLiquidityPool}`
      )
    }

    dumpDataToFile(keepInLpByStakers, pairObj.keepInLpTokenFilePath)

    return keepInLpByStakers
  }

  /**
   * Calculates amount of KEEP token which makes a KEEP-ETH / KEEP-TBTC Uniswap pair.
   * A Uniswap LP pair is a bookeeping tool to keep track of how much the liquidity
   * stakers are owed. They store two assets of equivalent value of each, ex. KEEP-ETH.
   * This means that the value of KEEP owned is dependent on the ratio of staked
   * LP tokens and the total LP supply. Ratio between LP tokens and KEEP tokens
   * should be equal:
   * LP_staker_balance / LP_total_supply_pool == KEEP_staker_owed / KEEP_total_liquidity_pool
   * Now, the number of KEEP tokens which makes a KEEP-ETH pair can be calculated
   * using the following equation:
   * KEEP_staker_owed = (LP_staker_balance * KEEP_total_liquidity_pool) / LP_total_supply_pool
   * where:
   * LP_staker_balance is retrieved from LPRewardsContract
   * KEEP_total_liquidity_pool is fetched from Uniswap LP Token - lpToken.getReserves()._reserve0
   * LP_total_supply_pool is fetched from Uniswap LP Token - lpToken.totalSupply()
   *
   * Another way to look at asset calculation in LP tokens is referring to a mint()
   * function in UniswapV2Pair contract, which produces the same equation as above.
   *
   * References:
   * Returns in Uniswap: https://app.uniswap.org/
   * LP minting: https://github.com/Uniswap/uniswap-v2-core/blob/4dd59067c76dea4a0e8e4bfdda41877a6b16dedc/contracts/UniswapV2Pair.sol#L123
   *
   * @param {BN} lpStakerBalance LP amount staked by a staker in a LPRewardsContract
   * @param {PairData} lpPairData Pair data fetched from LP Token Contract
   *
   * @return {BN} KEEP token amounts in LP token balance
   */
  async calcKeepTokenfromLPToken(lpStakerBalance, lpPairData) {
    return lpStakerBalance
      .mul(new BN(lpPairData.keepLiquidityPool))
      .div(new BN(lpPairData.lpTotalSupply))
  }

  /**
   * @return {Map<Address,BN>} KEEP token amounts staked by stakers at the target block
   */
  async getTokenHoldingsAtTargetBlock() {
    await this.initialize()
    const keepInLPsByStakers = new Map()

    for (const [pairName, pairObj] of Object.entries(this.liquidityStaking)) {
      const lpStakers = await this.findStakers(pairName, pairObj)
      const stakersBalances = await this.getLpTokenStakersBalances(
        lpStakers,
        pairObj
      )
      const keepInLpByStakers = await this.calcKeepInStakersBalances(
        stakersBalances,
        pairName,
        pairObj
      )

      keepInLpByStakers.forEach((balance, staker) => {
        if (keepInLPsByStakers.has(staker)) {
          keepInLPsByStakers.get(staker).iadd(balance)
        } else {
          keepInLPsByStakers.set(staker, new BN(balance))
        }
      })
    }

    return keepInLPsByStakers
  }
}
