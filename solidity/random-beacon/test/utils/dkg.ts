import { ethers, waffle } from "hardhat"
import { expect } from "chai"
import { BigNumber } from "ethers"

// eslint-disable-next-line import/no-cycle
import { selectGroup } from "./groups"

import type { Operator } from "./operators"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { RandomBeacon, SortitionPool } from "../../typechain"
import type { ContractTransaction } from "ethers"
import type {
  BeaconDkg as DKG,
  DkgResultSubmittedEvent,
} from "../../typechain/contracts/libraries/BeaconDkg"

const { provider } = waffle

// default Hardhat's networks blockchain, see https://hardhat.org/config/
export const hardhatNetworkId = 31337

export const noMisbehaved: number[] = []

export async function genesis(
  randomBeacon: RandomBeacon
): Promise<[ContractTransaction, BigNumber]> {
  const tx = await randomBeacon.genesis()

  const expectedSeed = ethers.BigNumber.from(
    ethers.utils.keccak256(
      ethers.utils.solidityPack(
        ["uint256", "uint256"],
        [
          "31415926535897932384626433832795028841971693993751058209749445923078164062862",
          tx.blockNumber,
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
  seed: BigNumber,
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  membersHash?: string,
  numberOfSignatures = 33
): Promise<{
  transaction: ContractTransaction
  dkgResult: DKG.ResultStruct
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
  submitterInitialBalance: BigNumber
}> {
  const sortitionPool = (await ethers.getContractAt(
    "SortitionPool",
    await randomBeacon.sortitionPool()
  )) as SortitionPool

  return signAndSubmitArbitraryDkgResult(
    randomBeacon,
    groupPublicKey,
    await selectGroup(sortitionPool, seed),
    startBlock,
    misbehavedIndices,
    submitterIndex,
    membersHash,
    numberOfSignatures
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
  numberOfSignatures = 33
): Promise<{
  transaction: ContractTransaction
  dkgResult: DKG.ResultStruct
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
  submitterInitialBalance: BigNumber
}> {
  const { members, signingMembersIndices, signaturesBytes } =
    await signDkgResult(
      signers,
      groupPublicKey,
      misbehavedIndices,
      startBlock,
      numberOfSignatures
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

  const dkgResultHash = ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(
      [
        "(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members, bytes32 membersHash)",
      ],
      [dkgResult]
    )
  )

  const submitter = signers[submitterIndex - 1].signer
  const submitterInitialBalance = await provider.getBalance(
    await submitter.getAddress()
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
  numberOfSignatures = 33
): Promise<{
  transaction: ContractTransaction
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
    numberOfSignatures
  )

  const signatureHexStrLength = 2 * 65
  const unrecoverableSignatures = `0x${"a".repeat(
    signatureHexStrLength * numberOfSignatures
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

  const dkgResultHash = ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(
      [
        "(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members, bytes32 membersHash)",
      ],
      [dkgResult]
    )
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
  numberOfSignatures: number
): Promise<{
  members: number[]
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

  return { members, signingMembersIndices, signaturesBytes }
}

// Creates a members hash that actively participated in dkg
export function hashDKGMembers(
  members: number[],
  misbehavedMembersIndices: number[]
): string {
  if (misbehavedMembersIndices.length > 0) {
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
  result: DKG.ResultStruct
}

// This is a workaround for a bug in ethereum-waffle library that doesn't let
// verify events that have an array nested in a struct.
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
