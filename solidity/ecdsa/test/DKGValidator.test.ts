/* eslint-disable no-await-in-loop */
import { BigNumber } from "ethers"
import { ethers, helpers } from "hardhat"
import { expect } from "chai"

import { constants, walletRegistryFixture } from "./fixtures"
import { selectGroup, hashUint32Array } from "./utils/groups"
import {
  signDkgResult,
  noMisbehaved,
  hashDKGMembers,
  hardhatNetworkId,
} from "./utils/dkg"
import ecdsaData from "./data/ecdsa"

import type { IWalletOwner } from "../typechain/IWalletOwner"
import type { Mock } from "./helpers/mock"
import type { DkgResult } from "./utils/dkg"
import type { Operator } from "./utils/operators"
import type {
  SortitionPool,
  EcdsaDkgValidator,
  WalletRegistry,
} from "../typechain"

const { createSnapshot, restoreSnapshot } = helpers.snapshot

describe("EcdsaDkgValidator", () => {
  const dkgSeed: BigNumber = BigNumber.from(
    "31415926535897932384626433832795028841971693993751058209749445923078164062862"
  )
  const dkgStartBlock = 1337
  const groupPublicKey: string = ethers.utils.hexValue(
    ecdsaData.group1.publicKey
  )

  let selectedOperators

  let prepareDkgResult: (
    _groupMembers: Operator[],
    _signers: Operator[],
    _groupPublicKey: string,
    _misbehaved: number[],
    _startBlock: number,
    _numberOfSignatures?: number,
    _submitterIndex?: number,
    _membersHash?: string
  ) => Promise<DkgResult>

  let walletRegistry: WalletRegistry
  let sortitionPool: SortitionPool
  let walletOwner: Mock<IWalletOwner>
  let validator: EcdsaDkgValidator

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ walletRegistry, sortitionPool, walletOwner } =
      await walletRegistryFixture({ useAllowlist: true }))

    validator = await helpers.contracts.getContract("EcdsaDkgValidator")

    await walletRegistry.connect(walletOwner.wallet).requestNewWallet()

    selectedOperators = await selectGroup(sortitionPool, dkgSeed)

    prepareDkgResult = async (
      _groupMembers: Operator[],
      _signers: Operator[],
      _groupPublicKey: string,
      _misbehaved: number[],
      _startBlock: number,
      _numberOfSignatures = 51,
      _submitterIndex = 1,
      _membersHash?: string
    ): Promise<DkgResult> => {
      const { signingMembersIndices, signaturesBytes } = await signDkgResult(
        _signers,
        _groupPublicKey,
        _misbehaved,
        _startBlock,
        _submitterIndex,
        _numberOfSignatures
      )

      const dkgResult: DkgResult = {
        submitterMemberIndex: _submitterIndex,
        groupPubKey: _groupPublicKey,
        misbehavedMembersIndices: _misbehaved,
        signatures: signaturesBytes,
        signingMembersIndices,
        members: _groupMembers.map((m) => m.id),
        membersHash:
          _membersHash ||
          hashDKGMembers(
            _groupMembers.map((m) => m.id),
            _misbehaved
          ),
      }

      return dkgResult
    }
  })

  describe("validate", () => {
    const testValidate = async (
      _groupMembers: Operator[],
      _signers: Operator[],
      _groupPublicKey: string,
      _misbehaved: number[],
      _membersHash: string
    ) => {
      const dkgResult = await prepareDkgResult(
        _groupMembers,
        _signers,
        _groupPublicKey,
        _misbehaved,
        dkgStartBlock,
        undefined,
        undefined,
        _membersHash
      )

      const result = await validator.validate(dkgResult, dkgSeed, dkgStartBlock)

      return {
        isValid: result[0],
        errorMsg: result[1],
      }
    }

    context("when DKG result contains misbehaved group members", () => {
      let selectedOperatorsIds: number[]

      before(async () => {
        await createSnapshot()
        selectedOperatorsIds = selectedOperators.map((m) => m.id)
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when hashed group members is correct", () => {
        context("when misbehaved index is present at the beginning", () => {
          it("should pass", async () => {
            const misbehavedMemberIds = [1]
            const expectedMembersIds = [...selectedOperatorsIds]
            expectedMembersIds.splice(0, 1) // index -1

            const result = await testValidate(
              selectedOperators,
              selectedOperators,
              groupPublicKey,
              misbehavedMemberIds,
              hashUint32Array(expectedMembersIds)
            )

            await expect(result.isValid).to.be.true
            await expect(result.errorMsg).to.equal("")
          })
        })
        context("when misbehaved indices present in the middle", () => {
          it("should pass", async () => {
            const misbehavedMemberIds = [22, 28, 46, 53]
            const expectedMembersIds = [...selectedOperatorsIds]
            expectedMembersIds.splice(21, 1) // index -1
            expectedMembersIds.splice(26, 1) // index -2 (cause expectedMembers already shrinked)
            expectedMembersIds.splice(43, 1) // index -3
            expectedMembersIds.splice(49, 1) // index -4

            const result = await testValidate(
              selectedOperators,
              selectedOperators,
              groupPublicKey,
              misbehavedMemberIds,
              hashUint32Array(expectedMembersIds)
            )

            await expect(result.isValid).to.be.true
            await expect(result.errorMsg).to.equal("")
          })
        })

        context("when misbehaved index is present at the end", () => {
          it("should pass", async () => {
            const misbehavedMemberIds = [constants.groupSize]
            const expectedMembersIds = [...selectedOperatorsIds]
            expectedMembersIds.splice(constants.groupSize - 1, 1) // index -1

            const result = await testValidate(
              selectedOperators,
              selectedOperators,
              groupPublicKey,
              misbehavedMemberIds,
              hashUint32Array(expectedMembersIds)
            )

            await expect(result.isValid).to.be.true
            await expect(result.errorMsg).to.equal("")
          })
        })
      })

      context("when hashed group members is incorrect", () => {
        it("should not pass", async () => {
          const misbehavedMemberIndices = [42]
          const invalidMembersIndices = [...selectedOperatorsIds]
          invalidMembersIndices.splice(21, 1) // index -1

          const result = await testValidate(
            selectedOperators,
            selectedOperators,
            groupPublicKey,
            misbehavedMemberIndices,
            hashUint32Array(invalidMembersIndices)
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Invalid members hash")
        })
      })
    })

    context("when DKG result is valid", () => {
      it("should pass", async () => {
        const result = await testValidate(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          hashUint32Array(selectedOperators.map((m) => m.id))
        )

        await expect(result.isValid).to.be.true
        await expect(result.errorMsg).to.equal("")
      })
    })

    // just a basic test ensuring group members validation
    // is called; detailed test cases are implemented for
    // validateGroupMembers function
    context("when DKG result contains invalid group members", () => {
      it("should return validation error", async () => {
        const shuffledOperators = shuffle(selectedOperators)

        const result = await testValidate(
          shuffledOperators,
          shuffledOperators,
          groupPublicKey,
          noMisbehaved,
          hashUint32Array(selectedOperators.map((m) => m.id))
        )

        await expect(result.isValid).to.be.false
        await expect(result.errorMsg).to.equal("Invalid group members")
      })
    })

    // just a basic test ensuring signatures validation
    // is called; detailed test cases are implemented for
    // validateSignatures function
    context("when DKG result contains invalid signatures", () => {
      it("should return validation error", async () => {
        const shuffledOperators = shuffle(selectedOperators)

        const result = await testValidate(
          selectedOperators,
          shuffledOperators,
          groupPublicKey,
          noMisbehaved,
          hashUint32Array(selectedOperators.map((m) => m.id))
        )

        await expect(result.isValid).to.be.false
        await expect(result.errorMsg).to.equal("Invalid signatures")
      })
    })
  })

  describe("validateFields", () => {
    const testValidateFields = async (
      _groupMembers,
      _groupPublicKey,
      _misbehaved,
      _numberOfSignatures = 51
    ) => {
      const dkgResult = await prepareDkgResult(
        _groupMembers,
        _groupMembers,
        _groupPublicKey,
        _misbehaved,
        dkgStartBlock,
        _numberOfSignatures
      )

      const result = await validator.validateFields(dkgResult)

      return {
        isValid: result[0],
        errorMsg: result[1],
      }
    }

    context("when DKG result is valid", () => {
      it("should pass", async () => {
        const result = await testValidateFields(
          selectedOperators,
          groupPublicKey,
          noMisbehaved
        )

        await expect(result.isValid).to.be.true
        await expect(result.errorMsg).to.equal("")
      })
    })

    context("when group public key is malformed", () => {
      context("when key is empty", () => {
        it("should return validation error", async () => {
          const empty = "0x"
          const result = await testValidateFields(
            selectedOperators,
            empty,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })

      context("when key is too short", () => {
        it("should return validation error", async () => {
          const tooShort = groupPublicKey.substring(
            0,
            groupPublicKey.length - 2
          )
          const result = await testValidateFields(
            selectedOperators,
            tooShort,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })

      context("when key is too long", () => {
        it("should return validation error", async () => {
          const tooLong = `${groupPublicKey}ff`
          const result = await testValidateFields(
            selectedOperators,
            tooLong,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })
    })

    context("when misbehaved array is malformed", () => {
      context("when index is less than one", async () => {
        it("should return validation error", async () => {
          const lessThanOne = [0, 1, 2]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            lessThanOne
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted misbehaved members indices"
          )
        })
      })

      context("when index is higher than group size", () => {
        it("should return validation error", async () => {
          const higherThanGroupSize = [1, 2, constants.groupSize + 1]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            higherThanGroupSize
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted misbehaved members indices"
          )
        })
      })

      context("when indices are unsorted", () => {
        it("should return validation error", async () => {
          const unsorted = [1, 2, 3, 60, 4]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            unsorted
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted misbehaved members indices"
          )
        })
      })

      context("when indices are duplicated", () => {
        it("should return validation error", async () => {
          const duplicated = [1, 2, 3, 3]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            duplicated
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted misbehaved members indices"
          )
        })
      })

      context("when there are too many indices", () => {
        it("should return validation error", async () => {
          const tooMany = [2, 4, 15, 17, 50, 53, 64, 72, 84, 86, 92]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            tooMany
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Too many members misbehaving during DKG"
          )
        })
      })
    })

    context("when signatures array is malformed", async () => {
      context("when there are no signatures", () => {
        it("should return validation error", async () => {
          const noSignatures = 0
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            noMisbehaved,
            noSignatures
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("No signatures provided")
        })
      })

      context("when signatures are of incorrect length", () => {
        it("should return validation error", async () => {
          const dkgResult = await prepareDkgResult(
            selectedOperators,
            selectedOperators,
            groupPublicKey,
            noMisbehaved,
            dkgStartBlock
          )
          dkgResult.signatures += "ff"
          const result = await validator.validateFields(dkgResult)

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed signatures array")
        })
      })

      context("when there are more signatures than signers", () => {
        it("should return validation error", async () => {
          const signatureHexStrLength = 130
          const dkgResult = await prepareDkgResult(
            selectedOperators,
            selectedOperators,
            groupPublicKey,
            noMisbehaved,
            dkgStartBlock
          )
          dkgResult.signatures += "f".repeat(signatureHexStrLength)
          const result = await validator.validateFields(dkgResult)

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Unexpected signatures count")
        })
      })

      context("when there are too few signatures", () => {
        it("should return validation error", async () => {
          const tooFewSignatures = 32
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            noMisbehaved,
            tooFewSignatures
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Too few signatures")
        })
      })

      context("when there are too many signatures", () => {
        it("should return validation error", async () => {
          const signatureHexStrLength = 130
          const maxSignatures = constants.groupSize
          const dkgResult = await prepareDkgResult(
            selectedOperators,
            selectedOperators,
            groupPublicKey,
            noMisbehaved,
            dkgStartBlock,
            maxSignatures
          )
          dkgResult.signatures += "f".repeat(signatureHexStrLength)
          dkgResult.signingMembersIndices.push(maxSignatures + 1)
          const result = await validator.validateFields(dkgResult)

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Too many signatures")
        })
      })
    })

    context("when signing members indices array is malformed", async () => {
      const testSigningMembers = async (_signingMembersIndices) => {
        const dkgResult = await prepareDkgResult(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          dkgStartBlock
        )

        dkgResult.signingMembersIndices = _signingMembersIndices
        const result = await validator.validateFields(dkgResult)

        return {
          isValid: result[0],
          errorMsg: result[1],
        }
      }

      context("when index is zero", () => {
        it("should return validation error", async () => {
          const indicesStartWithZero = [
            0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 15, 17, 18, 19,
            20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
            37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51,
          ]

          const result = await testSigningMembers(indicesStartWithZero)

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted signing member indices"
          )
        })
      })

      context("when index is greater than group size", () => {
        it("should return validation error", async () => {
          const indicesEndWithTooBig = [
            1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 15, 17, 18, 19,
            20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
            37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 101,
          ]
          const result = await testSigningMembers(indicesEndWithTooBig)

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted signing member indices"
          )
        })
      })

      context("when indices are unsorted", () => {
        it("should return validation error", async () => {
          const indicesUnsorted = [
            1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 15, 17, 18, 19,
            20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
            37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 49, 48, 50, 51,
          ]
          const result = await testSigningMembers(indicesUnsorted)

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal(
            "Corrupted signing member indices"
          )
        })
      })
    })
  })

  describe("validateGroupMembers", () => {
    const testValidateGroupMembers = async (_groupMembers) => {
      const dkgResult = await prepareDkgResult(
        _groupMembers,
        _groupMembers,
        groupPublicKey,
        noMisbehaved,
        dkgStartBlock
      )

      return validator.validateGroupMembers(dkgResult, dkgSeed)
    }

    context("when DKG result is valid", () => {
      it("should pass", async () => {
        const isValid = await testValidateGroupMembers(selectedOperators)
        await expect(isValid).to.be.true
      })
    })

    context("when there are operators other then selected", () => {
      it("should fail the validation", async () => {
        const isValid = await testValidateGroupMembers(
          shuffle(selectedOperators)
        )
        await expect(isValid).to.be.false
      })
    })
  })

  describe("validateSignatures", () => {
    const testValidateSignatures = async (_groupMembers, _signers) => {
      const dkgResult = await prepareDkgResult(
        _groupMembers,
        _signers,
        groupPublicKey,
        noMisbehaved,
        dkgStartBlock
      )

      return validator.validateSignatures(dkgResult, dkgStartBlock)
    }

    context("when DKG result is valid", () => {
      it("should pass", async () => {
        const isValid = await testValidateSignatures(
          selectedOperators,
          selectedOperators
        )

        await expect(isValid).to.be.true
      })
    })

    context(
      "when signatures provided by one malicious, selected operator",
      () => {
        it("should fail the validation", async () => {
          const maliciousSigners = Array(constants.groupSize).fill(
            selectedOperators[0]
          )
          const isValid = await testValidateSignatures(
            selectedOperators,
            maliciousSigners
          )

          await expect(isValid).to.be.false
        })
      }
    )

    context("when signatures do not matching signers", () => {
      it("should fail the validation", async () => {
        const dkgResult = await prepareDkgResult(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          dkgStartBlock
        )
        // reverse order of signers
        ;[
          dkgResult.signingMembersIndices[8],
          dkgResult.signingMembersIndices[9],
        ] = [
          dkgResult.signingMembersIndices[9],
          dkgResult.signingMembersIndices[8],
        ]
        const isValid = await validator.validateSignatures(
          dkgResult,
          dkgStartBlock
        )

        await expect(isValid).to.be.false
      })
    })

    context("when signatures contain wrong result hash", () => {
      const signWithWrongResultHash = async (signingOperators: Operator[]) => {
        const wrongResultHash = ethers.utils.keccak256(
          ethers.utils.defaultAbiCoder.encode(
            ["uint256", "bytes", "uint8[]", "uint256"],
            [
              hardhatNetworkId,
              groupPublicKey,
              noMisbehaved,
              dkgStartBlock + 12345,
            ]
          )
        )
        const signatures = []
        for (let i = 0; i < signingOperators.length; i++) {
          const { signer: ethersSigner } = signingOperators[i]
          const signature = await ethersSigner.signMessage(
            ethers.utils.arrayify(wrongResultHash)
          )
          signatures.push(signature)
        }
        const signaturesBytes = ethers.utils.hexConcat(signatures)
        return signaturesBytes
      }

      it("should fail the validation", async () => {
        const numberOfSignatures = constants.groupThreshold
        const dkgResult = await prepareDkgResult(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          dkgStartBlock,
          numberOfSignatures
        )
        dkgResult.signatures = await signWithWrongResultHash(
          selectedOperators.slice(numberOfSignatures - 1)
        )
        const isValid = await validator.validateSignatures(
          dkgResult,
          dkgStartBlock
        )

        await expect(isValid).to.be.false
      })
    })

    context("when signatures consist of random bytes", () => {
      it("should revert", async () => {
        const numberOfSignatures = constants.groupThreshold
        const signatureHexStrLength = 2 * 65
        const dkgResult = await prepareDkgResult(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          dkgStartBlock,
          numberOfSignatures
        )
        const wrongSignatures = `0x${"a".repeat(
          signatureHexStrLength * numberOfSignatures
        )}`
        dkgResult.signatures = wrongSignatures

        await expect(
          validator.validateSignatures(dkgResult, dkgStartBlock)
        ).to.be.revertedWith("ECDSA: invalid signature 's' value")
      })
    })
  })
})

function shuffle(operators: Operator[]): Operator[] {
  return operators
    .map((v) => ({ v, sort: Math.random() }))
    .sort((a, b) => a.sort - b.sort)
    .map(({ v }) => v)
}
