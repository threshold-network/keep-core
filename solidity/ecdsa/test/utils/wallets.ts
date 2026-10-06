import { helpers, ethers } from "hardhat"

import requireResult from "../helpers/chain"
import { params } from "../fixtures"
import ecdsaData from "../data/ecdsa"

import { noMisbehaved, signAndSubmitCorrectDkgResult } from "./dkg"

import type { Mock } from "../helpers/mock"
import type { DkgResult } from "./dkg"
import type { IRandomBeacon, WalletRegistry } from "../../typechain"
import type { Operator } from "./operators"
import type { BytesLike, ContractTransactionResponse, Signer } from "ethers"

const { mineBlocks } = helpers.time
const { keccak256 } = ethers

// eslint-disable-next-line import/prefer-default-export
export async function createNewWallet(
  walletRegistry: WalletRegistry,
  walletOwner: Signer,
  randomBeacon: Mock<IRandomBeacon>,
  publicKey: BytesLike = ecdsaData.group1.publicKey,
): Promise<{
  members: Operator[]
  dkgResult: DkgResult
  walletID: string
  tx: ContractTransactionResponse
}> {
  const requestNewWalletTx = await walletRegistry
    .connect(walletOwner)
    .requestNewWallet()

  const relayEntry = ethers.randomBytes(32)

  const dkgSeed = BigInt(keccak256(relayEntry))

  // eslint-disable-next-line no-underscore-dangle
  await walletRegistry
    .connect(randomBeacon.wallet)
    .__beaconCallback(ethers.toBigInt(relayEntry), 0)

  const {
    dkgResult,
    submitter,
    signers: members,
  } = await signAndSubmitCorrectDkgResult(
    walletRegistry,
    publicKey,
    dkgSeed,
    requireResult(await requestNewWalletTx.wait()).blockNumber,
    noMisbehaved,
  )

  await mineBlocks(params.dkgResultChallengePeriodLength)

  const approveDkgResultTx = await walletRegistry
    .connect(submitter)
    .approveDkgResult(dkgResult)

  return {
    members,
    dkgResult,
    walletID: keccak256(publicKey),
    tx: approveDkgResultTx,
  }
}
