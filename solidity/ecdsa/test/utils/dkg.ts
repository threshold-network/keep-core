/* eslint-disable no-await-in-loop */

import { ethers } from "hardhat"
import { expect } from "chai"
import { BigNumber } from "ethers"

import { selectGroup } from "./groups"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { BigNumberish, ContractTransaction, BytesLike } from "ethers"
import type { SortitionPool, WalletRegistry } from "../../typechain"
import type { Operator } from "./operators"
import type {
  DkgResultSubmittedEvent,
  ResultStruct,
} from "../../typechain/EcdsaDkg"

const { provider } = ethers

// default Hardhat's networks blockchain, see https://hardhat.org/config/
export const hardhatNetworkId = 31337

export interface DkgResult {
  submitterMemberIndex: number
  groupPubKey: BytesLike
  misbehavedMembersIndices: number[]
  signatures: string
  signingMembersIndices: number[]
  members: number[]
  membersHash: string
}

export const noMisbehaved: number[] = []

export function calculateDkgSeed(
  relayEntry: BigNumberish,
  blockNumber: BigNumberish
): BigNumber {
  return ethers.BigNumber.from(
    ethers.utils.keccak256(
      ethers.utils.solidityPack(
        ["uint256", "uint256"],
        [ethers.BigNumber.from(relayEntry), ethers.BigNumber.from(blockNumber)]
      )
    )
  )
}

// Sign and submit a correct DKG result which cannot be challenged because used
// signers belong to an actual group selected by the sortition pool for given
// seed.
export async function signAndSubmitCorrectDkgResult(
  walletRegistry: WalletRegistry,
  groupPublicKey: BytesLike,
  seed: BigNumber,
  startBlock: number,
  misbehavedIndices = noMisbehaved,
  submitterIndex = 1,
  numberOfSignatures = 51
): Promise<{
  signers: Operator[]
  dkgResult: DkgResult
  dkgResultHash: string
  submitter: SignerWithAddress
  submitterInitialBalance: BigNumber
  transaction: ContractTransaction
}> {
  const sortitionPool = (await ethers.getContractAt(
    "SortitionPool",
    await walletRegistry.sortitionPool()
  )) as SortitionPool

  const signers = await selectGroup(sortitionPool, seed)

  return {
    signers,
    ...(await signAndSubmitArbitraryDkgResult(
      walletRegistry,
      groupPublicKey,
      signers,
      startBlock,
      misbehavedIndices,
      submitterIndex,
      numberOfSignatures
    )),
  }
}

const DKG_RESULT_PARAMS_SIGNATURE =
  "(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members, bytes32 membersHash)"

// Sign and submit an arbitrary DKG result using given signers. Signers don't
// need to be part of the actual sortition pool group. This function is useful
// for preparing invalid or malicious results for testing purposes.
export async function signAndSubmitArbitraryDkgResult(
  walletRegistry: WalletRegistry,
  groupPublicKey: BytesLike,
  signers: Operator[],
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  numberOfSignatures = 51
): Promise<{
  dkgResult: DkgResult
  dkgResultHash: string
  submitter: SignerWithAddress
  submitterInitialBalance: BigNumber
  transaction: ContractTransaction
}> {
  const { dkgResult } = await signDkgResult(
    signers,
    groupPublicKey,
    misbehavedIndices,
    startBlock,
    submitterIndex,
    numberOfSignatures
  )

  const dkgResultHash = ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(
      [DKG_RESULT_PARAMS_SIGNATURE],
      [dkgResult]
    )
  )

  const submitter = signers[submitterIndex - 1].signer
  const submitterInitialBalance = await provider.getBalance(
    await submitter.getAddress()
  )

  return {
    dkgResult,
    dkgResultHash,
    submitter,
    submitterInitialBalance,
    ...(await submitDkgResult(walletRegistry, dkgResult, submitter)),
  }
}

// Signs and submits a DKG result containing signatures with random bytes.
// Attempting to recover addresses from such signatures causes a revert. It is
// useful for preparing malicious DKG results.
export async function signAndSubmitUnrecoverableDkgResult(
  walletRegistry: WalletRegistry,
  groupPublicKey: string,
  signers: Operator[],
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  numberOfSignatures = 51
): Promise<{
  dkgResult: DkgResult
  dkgResultHash: string
  submitter: SignerWithAddress
  transaction: ContractTransaction
}> {
  const { dkgResult } = await signDkgResult(
    signers,
    groupPublicKey,
    misbehavedIndices,
    startBlock,
    submitterIndex,
    numberOfSignatures
  )

  // Break the result
  const signatureHexStrLength = 2 * 65
  const unrecoverableSignatures = `0x${"a".repeat(
    signatureHexStrLength * numberOfSignatures
  )}`
  dkgResult.signatures = unrecoverableSignatures

  const dkgResultHash = ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(
      [DKG_RESULT_PARAMS_SIGNATURE],
      [dkgResult]
    )
  )

  const submitter = signers[submitterIndex - 1].signer

  return {
    dkgResult,
    dkgResultHash,
    submitter,
    ...(await submitDkgResult(walletRegistry, dkgResult, submitter)),
  }
}

export async function signDkgResult(
  signers: Operator[],
  groupPublicKey: BytesLike,
  misbehavedMembersIndices: number[],
  startBlock: number,
  submitterIndex = 1,
  numberOfSignatures = 51
): Promise<{
  dkgResult: DkgResult
  signingMembersIndices: number[]
  signaturesBytes: string
}> {
  const resultHash = ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(
      ["uint256", "bytes", "uint8[]", "uint256"],
      [hardhatNetworkId, groupPublicKey, misbehavedMembersIndices, startBlock]
    )
  )

  const members: number[] = []
  const signingMembersIndices: number[] = []
  const signatures: string[] = []
  for (let i = 0; i < signers.length; i++) {
    const { id, signer: ethersSigner } = signers[i]
    members.push(id)

    if (signatures.length === numberOfSignatures) {
      // eslint-disable-next-line no-continue
      continue
    }

    const signerIndex: number = i + 1

    signingMembersIndices.push(signerIndex)

    const signature = await ethersSigner.signMessage(
      ethers.utils.arrayify(resultHash)
    )

    signatures.push(signature)
  }

  const signaturesBytes: string = ethers.utils.hexConcat(signatures)

  const dkgResult: DkgResult = {
    submitterMemberIndex: submitterIndex,
    groupPubKey: groupPublicKey,
    misbehavedMembersIndices,
    signatures: signaturesBytes,
    signingMembersIndices,
    members,
    membersHash: hashDKGMembers(members, misbehavedMembersIndices),
  }

  return { dkgResult, signingMembersIndices, signaturesBytes }
}

export async function submitDkgResult(
  walletRegistry: WalletRegistry,
  dkgResult: DkgResult,
  submitter: SignerWithAddress
): Promise<{
  transaction: ContractTransaction
}> {
  const transaction = await walletRegistry
    .connect(submitter)
    .submitDkgResult(dkgResult)

  return { transaction }
}

// Creates a members hash that actively participated in dkg
export function hashDKGMembers(
  members: number[],
  misbehavedMembersIndices?: number[]
): string {
  if (misbehavedMembersIndices && misbehavedMembersIndices.length > 0) {
    const activeDkgMembers = [...members]
    for (let i = 0; i < misbehavedMembersIndices.length; i++) {
      if (misbehavedMembersIndices[i] !== 0) {
        activeDkgMembers.splice(misbehavedMembersIndices[i] - i - 1, 1)
      }
    }

    return ethers.utils.keccak256(
      ethers.utils.defaultAbiCoder.encode(["uint32[]"], [activeDkgMembers])
    )
  }

  return ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(["uint32[]"], [members])
  )
}

export interface DkgResultSubmittedEventArgs {
  resultHash: string
  seed: BigNumber
  result: ResultStruct
}

// Decode this event explicitly. The assertion was introduced as a workaround
// for Waffle's inability to verify events with an array nested in a struct.
// See: https://github.com/EthWorks/Waffle/issues/245
export async function expectDkgResultSubmittedEvent(
  tx: ContractTransaction,
  expectedArgs: DkgResultSubmittedEventArgs
): Promise<void> {
  const eventName = "DkgResultSubmitted"

  const event: DkgResultSubmittedEvent = (await tx.wait()).events.find(
    (e) => e.event === eventName
  ) as unknown as DkgResultSubmittedEvent

  await expect(event, `Event ${eventName} not emitted`).to.be.not.null

  const actualArgs = event.args

  await expect(actualArgs.length, "invalid event args length").to.be.equal(
    Object.keys(expectedArgs).length
  )

  await expect(
    actualArgs.result.length,
    "invalid result args length"
  ).to.be.equal(Object.keys(expectedArgs.result).length)

  await expect(actualArgs.resultHash, "invalid resultHash").to.be.equal(
    expectedArgs.resultHash
  )

  await expect(actualArgs.seed, "invalid seed").to.be.equal(expectedArgs.seed)

  await expect(
    actualArgs.result.submitterMemberIndex,
    "invalid submitterMemberIndex"
  ).to.be.equal(expectedArgs.result.submitterMemberIndex)

  await expect(
    actualArgs.result.groupPubKey,
    "invalid groupPubKey"
  ).to.be.equal(expectedArgs.result.groupPubKey)

  await expect(
    actualArgs.result.misbehavedMembersIndices,
    "invalid misbehavedMembersIndices"
  ).to.be.deep.equal(expectedArgs.result.misbehavedMembersIndices)

  await expect(actualArgs.result.signatures, "invalid signatures").to.be.equal(
    expectedArgs.result.signatures
  )

  await expect(
    actualArgs.result.signingMembersIndices,
    "invalid signingMembersIndices"
  ).to.be.deep.equal(
    expectedArgs.result.signingMembersIndices.map(BigNumber.from)
  )

  await expect(actualArgs.result.members, "invalid members").to.be.deep.equal(
    expectedArgs.result.members
  )

  await expect(
    actualArgs.result.membersHash,
    "invalid membersHash"
  ).to.be.equal(expectedArgs.result.membersHash)
}
