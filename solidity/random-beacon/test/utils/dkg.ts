import { ethers } from "hardhat"
import { expect } from "chai"

import requireResult from "../helpers/chain"

// eslint-disable-next-line import/no-cycle
import { selectGroup } from "./groups"

import type { Operator } from "./operators"
import type { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers"
import type { RandomBeacon, SortitionPool } from "../../typechain"
import type { ContractTransactionResponse } from "ethers"
import type {
  BeaconDkg as DKG,
  DkgResultSubmittedEvent,
} from "../../typechain/contracts/libraries/BeaconDkg"

const { provider } = ethers

// default Hardhat's networks blockchain, see https://hardhat.org/config/
export const hardhatNetworkId = 31337

export const noMisbehaved: number[] = []

export async function genesis(
  randomBeacon: RandomBeacon,
): Promise<[ContractTransactionResponse, bigint]> {
  const tx = await randomBeacon.genesis()

  const expectedSeed = BigInt(
    ethers.keccak256(
      ethers.solidityPacked(
        ["uint256", "uint256"],
        [
          "31415926535897932384626433832795028841971693993751058209749445923078164062862",
          receipt.blockNumber,
        ]
      )
    )
  )

  return [tx, expectedSeed]
}

// Sign and submit a correct DKG result which cannot be challenged because used
// signers belong to an actual group selected by the sortition pool for given
// seed.
export async function signAndSubmitCorrectDkgResult(
  randomBeacon: RandomBeacon,
  groupPublicKey: string,
  seed: bigint,
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  membersHash?: string,
  numberOfSignatures = 33,
): Promise<{
  transaction: ContractTransactionResponse
  dkgResult: DKG.ResultStruct
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
  submitterInitialBalance: bigint
}> {
  const sortitionPool = (await ethers.getContractAt(
    "SortitionPool",
    await randomBeacon.sortitionPool(),
  )) as SortitionPool

  return signAndSubmitArbitraryDkgResult(
    randomBeacon,
    groupPublicKey,
    await selectGroup(sortitionPool, seed),
    startBlock,
    misbehavedIndices,
    submitterIndex,
    membersHash,
    numberOfSignatures,
  )
}

// Sign and submit an arbitrary DKG result using given signers. Signers don't
// need to be part of the actual sortition pool group. This function is useful
// for preparing invalid or malicious results for testing purposes.
export async function signAndSubmitArbitraryDkgResult(
  randomBeacon: RandomBeacon,
  groupPublicKey: string,
  signers: Operator[],
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  groupMembersHash?: string,
  numberOfSignatures = 33,
): Promise<{
  transaction: ContractTransactionResponse
  dkgResult: DKG.ResultStruct
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
  submitterInitialBalance: bigint
}> {
  const { members, signingMembersIndices, signaturesBytes } =
    await signDkgResult(
      signers,
      groupPublicKey,
      misbehavedIndices,
      startBlock,
      numberOfSignatures,
    )

  let membersHash = groupMembersHash
  if (!membersHash) {
    membersHash = hashDKGMembers(members, misbehavedIndices)
  }

  const dkgResult: DKG.ResultStruct = {
    submitterMemberIndex: submitterIndex,
    groupPubKey: groupPublicKey,
    misbehavedMembersIndices: misbehavedIndices,
    signatures: signaturesBytes,
    signingMembersIndices,
    members,
    membersHash,
  }

  const dkgResultHash = ethers.keccak256(
    ethers.AbiCoder.defaultAbiCoder().encode(
      [
        "(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members, bytes32 membersHash)",
      ],
      [dkgResult],
    ),
  )

  const submitter = signers[submitterIndex - 1].signer
  const submitterInitialBalance = await provider.getBalance(
    await submitter.getAddress(),
  )

  const transaction = await randomBeacon
    .connect(submitter)
    .submitDkgResult(dkgResult)

  return {
    transaction,
    dkgResult,
    dkgResultHash,
    members,
    submitter,
    submitterInitialBalance,
  }
}

// Signs and submits a DKG result containing signatures with random bytes.
// Attempting to recover addresses from such signatures causes a revert. It is
// useful for preparing malicious DKG results.
export async function signAndSubmitUnrecoverableDkgResult(
  randomBeacon: RandomBeacon,
  groupPublicKey: string,
  signers: Operator[],
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  numberOfSignatures = 33,
): Promise<{
  transaction: ContractTransactionResponse
  dkgResult: DKG.ResultStruct
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
}> {
  const { members, signingMembersIndices } = await signDkgResult(
    signers,
    groupPublicKey,
    misbehavedIndices,
    startBlock,
    numberOfSignatures,
  )

  const signatureHexStrLength = 2 * 65
  const unrecoverableSignatures = `0x${"a".repeat(
    signatureHexStrLength * numberOfSignatures,
  )}`

  const membersHash = hashDKGMembers(members, misbehavedIndices)

  const dkgResult: DKG.ResultStruct = {
    submitterMemberIndex: submitterIndex,
    groupPubKey: groupPublicKey,
    misbehavedMembersIndices: misbehavedIndices,
    signatures: unrecoverableSignatures,
    signingMembersIndices,
    members,
    membersHash,
  }

  const dkgResultHash = ethers.keccak256(
    ethers.AbiCoder.defaultAbiCoder().encode(
      [
        "(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members, bytes32 membersHash)",
      ],
      [dkgResult],
    ),
  )

  const submitter = signers[submitterIndex - 1].signer

  const transaction = await randomBeacon
    .connect(submitter)
    .submitDkgResult(dkgResult)

  return { transaction, dkgResult, dkgResultHash, members, submitter }
}

export async function signDkgResult(
  signers: Operator[],
  groupPublicKey: string,
  misbehavedMembersIndices: number[],
  startBlock: number,
  numberOfSignatures: number,
): Promise<{
  members: number[]
  signingMembersIndices: number[]
  signaturesBytes: string
}> {
  const resultHash = ethers.keccak256(
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["uint256", "bytes", "uint8[]", "uint256"],
      [hardhatNetworkId, groupPublicKey, misbehavedMembersIndices, startBlock],
    ),
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
      ethers.getBytes(resultHash),
    )

    signatures.push(signature)
  }

  const signaturesBytes: string = ethers.concat(signatures)

  return { members, signingMembersIndices, signaturesBytes }
}

// Creates a members hash that actively participated in dkg
export function hashDKGMembers(
  members: number[],
  misbehavedMembersIndices: number[],
): string {
  if (misbehavedMembersIndices.length > 0) {
    const activeDkgMembers = [...members]
    for (let i = 0; i < misbehavedMembersIndices.length; i++) {
      if (misbehavedMembersIndices[i] !== 0) {
        activeDkgMembers.splice(misbehavedMembersIndices[i] - i - 1, 1)
      }
    }

    return ethers.keccak256(
      ethers.AbiCoder.defaultAbiCoder().encode(
        ["uint32[]"],
        [activeDkgMembers],
      ),
    )
  }

  return ethers.keccak256(
    ethers.AbiCoder.defaultAbiCoder().encode(["uint32[]"], [members]),
  )
}

export interface DkgResultSubmittedEventArgs {
  resultHash: string
  seed: bigint
  result: DKG.ResultStruct
}

// Compare each field explicitly so nested arrays in the result struct produce
// useful assertion failures.
export async function expectDkgResultSubmittedEvent(
  tx: ContractTransactionResponse,
  expectedArgs: DkgResultSubmittedEventArgs,
): Promise<void> {
  const eventName = "DkgResultSubmitted"

  const event = requireResult(await tx.wait()).logs.find(
    (log): log is DkgResultSubmittedEvent.Log =>
      log instanceof ethers.EventLog && log.eventName === eventName,
  )

  if (!event) {
    throw new Error(`Event ${eventName} not emitted`)
  }

  const actualArgs = event.args

  await expect(actualArgs.length, "invalid event args length").to.be.equal(
    Object.keys(expectedArgs).length,
  )

  await expect(
    actualArgs.result.length,
    "invalid result args length",
  ).to.be.equal(Object.keys(expectedArgs.result).length)

  await expect(actualArgs.resultHash, "invalid resultHash").to.be.equal(
    expectedArgs.resultHash,
  )

  await expect(actualArgs.seed, "invalid seed").to.be.equal(expectedArgs.seed)

  await expect(
    actualArgs.result.submitterMemberIndex,
    "invalid submitterMemberIndex",
  ).to.be.equal(expectedArgs.result.submitterMemberIndex)

  await expect(
    actualArgs.result.groupPubKey,
    "invalid groupPubKey",
  ).to.be.equal(expectedArgs.result.groupPubKey)

  await expect(
    actualArgs.result.misbehavedMembersIndices,
    "invalid misbehavedMembersIndices",
  ).to.be.deep.equal(expectedArgs.result.misbehavedMembersIndices.map(BigInt))

  await expect(actualArgs.result.signatures, "invalid signatures").to.be.equal(
    expectedArgs.result.signatures,
  )

  await expect(
    actualArgs.result.signingMembersIndices,
    "invalid signingMembersIndices",
  ).to.be.deep.equal(expectedArgs.result.signingMembersIndices.map(BigInt))

  await expect(actualArgs.result.members, "invalid members").to.be.deep.equal(
    expectedArgs.result.members.map(BigInt),
  )

  await expect(
    actualArgs.result.membersHash,
    "invalid membersHash",
  ).to.be.equal(expectedArgs.result.membersHash)
}
