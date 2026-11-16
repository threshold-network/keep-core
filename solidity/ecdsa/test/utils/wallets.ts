import { helpers, ethers } from "hardhat"

import { params } from "../fixtures"
import ecdsaData from "../data/ecdsa"

import { noMisbehaved, signAndSubmitCorrectDkgResult } from "./dkg"
import { submitRelayEntry } from "./randomBeacon"

import type { Mock } from "../helpers/mock"
import type { DkgResult } from "./dkg"
import type { IRandomBeacon, WalletRegistry } from "../../typechain"
import type { Operator } from "./operators"
import type { BigNumber, BytesLike, ContractTransaction, Signer } from "ethers"

const { mineBlocks } = helpers.time
const { keccak256 } = ethers.utils

// eslint-disable-next-line import/prefer-default-export
export async function createNewWallet(
  walletRegistry: WalletRegistry,
  walletOwner: Signer,
  randomBeacon: Mock<IRandomBeacon>,
  publicKey: BytesLike = ecdsaData.group1.publicKey,
  // Optional fixed relay entry. Callers that assert on positions within the
  // selected group pass one, because a random seed can select the same
  // operator at two positions.
  relayEntry?: BigNumber
): Promise<{
  members: Operator[]
  dkgResult: DkgResult
  walletID: string
  tx: ContractTransaction
}> {
  await walletRegistry.connect(walletOwner).requestNewWallet()

  let startBlock: number
  let dkgSeed: BigNumber

  if (relayEntry) {
    // eslint-disable-next-line no-underscore-dangle
    const tx = await walletRegistry
      .connect(randomBeacon.wallet)
      .__beaconCallback(relayEntry, 0)
    startBlock = tx.blockNumber
    dkgSeed = relayEntry
  } else {
    // The seed the DKG is validated against is the relay entry itself, as
    // passed to `__beaconCallback`; `submitRelayEntry` returns exactly that
    // value.
    const entry = await submitRelayEntry(walletRegistry, randomBeacon)
    startBlock = entry.startBlock
    dkgSeed = entry.dkgSeed
  }

  const {
    dkgResult,
    submitter,
    signers: members,
  } = await signAndSubmitCorrectDkgResult(
    walletRegistry,
    publicKey,
    dkgSeed,
    startBlock,
    noMisbehaved
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
