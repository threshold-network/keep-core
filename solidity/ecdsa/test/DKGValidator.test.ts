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

// secp256k1 field prime, `2**256 - 2**32 - 977`.
const SECP256K1_P = BigNumber.from(
  "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F"
)

// Encodes a fabricated group public key. Both coordinates are encoded as
// 32-byte words, so the key passes the length check at submission time even
// when a coordinate is not a canonical field element.
function encodeGroupPublicKey(x: BigNumber, y: BigNumber): string {
  return ethers.utils.hexConcat([
    ethers.utils.zeroPad(x.toHexString(), 32),
    ethers.utils.zeroPad(y.toHexString(), 32),
  ])
}

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

      context("when x is not a canonical field element", () => {
        it("should return validation error", async () => {
          // `x = P + 1` reduces to 1 modulo P and `y` is chosen so that
          // `y^2 = 1 + 7 = 8`, so the curve equation holds for the reduced
          // coordinates. Only the `x >= P` bound rejects this key.
          const key = encodeGroupPublicKey(
            SECP256K1_P.add(1),
            BigNumber.from(
              "0x4218f20ae6c646b363db68605822fb14264ca8d2587fdd6fbc750d587e76a7ee"
            )
          )

          const result = await testValidateFields(
            selectedOperators,
            key,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })

      context("when y is not a canonical field element", () => {
        it("should return validation error", async () => {
          // `y = P + 1` reduces to 1 modulo P. `x` is a cube root of `-6`
          // modulo P, so `x^3 + 7 = 1 = y^2` for the reduced coordinates.
          // Only the `y >= P` bound rejects this key.
          const key = encodeGroupPublicKey(
            BigNumber.from(
              "0x1fe1e5ef3fceb5c135ab7741333ce5a6e80d68167653f6b2b24bcbcfaaaff507"
            ),
            SECP256K1_P.add(1)
          )

          const result = await testValidateFields(
            selectedOperators,
            key,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })

      // The curve equation is a single comparison, so the two sides of it need
      // their own cases: `y^2 < x^3 + 7` is still unequal, and a `<` or `>`
      // mutant of the equality test accepts one side each. Both coordinates
      // below are canonical field elements and `x` is not zero, so only the
      // equation decides.
      context("when y^2 is below x^3 + 7", () => {
        it("should return validation error", async () => {
          // `x = 1`, `y = 2`: `y^2 = 4` and `x^3 + 7 = 8`.
          const key = encodeGroupPublicKey(BigNumber.from(1), BigNumber.from(2))

          const result = await testValidateFields(
            selectedOperators,
            key,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })

      context("when y^2 is above x^3 + 7", () => {
        it("should return validation error", async () => {
          // `x = 1`, `y = 3`: `y^2 = 9` and `x^3 + 7 = 8`.
          const key = encodeGroupPublicKey(BigNumber.from(1), BigNumber.from(3))

          const result = await testValidateFields(
            selectedOperators,
            key,
            noMisbehaved
          )

          await expect(result.isValid).to.be.false
          await expect(result.errorMsg).to.equal("Malformed group public key")
        })
      })
    })

    context(
      "when misbehaved indices leave exactly the active threshold members",
      () => {
        it("should pass", async () => {
          const activeThreshold = (await validator.activeThreshold()).toNumber()
          // The rejecting case below uses 11 indices, leaving 89 active
          // members. Ten indices leave exactly `activeThreshold` (90) active
          // members, the accepting side of the `activeThreshold` guard.
          const atThreshold = [2, 4, 15, 17, 50, 53, 64, 72, 84, 86]
          await expect(atThreshold).to.have.lengthOf(
            constants.groupSize - activeThreshold
          )

          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            atThreshold
          )

          await expect(result.isValid).to.be.true
          await expect(result.errorMsg).to.equal("")
        })
      }
    )

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

    // The address comparison is a single `!=`, so a relational mutant of it
    // rejects only one side of a mismatch: `>` accepts a recovered address
    // above the signing member's and `<` accepts one below. Each side needs a
    // signature from a key that sorts on that side, which the deterministic
    // Hardhat accounts do not guarantee, so the key is searched at run time.
    const testMismatchedSignature = async (side: "above" | "below") => {
      const dkgResult = await prepareDkgResult(
        selectedOperators,
        selectedOperators,
        groupPublicKey,
        noMisbehaved,
        dkgStartBlock
      )
      // The hash `validateSignatures` signs over, without the EIP-191 prefix
      // `signMessage` adds on both sides.
      const resultHash = ethers.utils.keccak256(
        ethers.utils.defaultAbiCoder.encode(
          ["uint256", "bytes", "uint8[]", "uint256"],
          [hardhatNetworkId, groupPublicKey, noMisbehaved, dkgStartBlock]
        )
      )
      const signature = await foreignSignature(
        selectedOperators[0].signer.address,
        resultHash,
        side
      )
      // Only the first signature changes; every other member still signs the
      // same valid result.
      dkgResult.signatures = ethers.utils.hexConcat([
        signature,
        ethers.utils.hexDataSlice(dkgResult.signatures, 65),
      ])

      return validator.validateSignatures(dkgResult, dkgStartBlock)
    }

    context(
      "when a signature recovers an address above the signing member's",
      () => {
        it("should fail the validation", async () => {
          await expect(await testMismatchedSignature("above")).to.be.false
        })
      }
    )

    context(
      "when a signature recovers an address below the signing member's",
      () => {
        it("should fail the validation", async () => {
          await expect(await testMismatchedSignature("below")).to.be.false
        })
      }
    )

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
      it("should answer false instead of reverting", async () => {
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

        // 0xaaaa... has an out-of-range `s`, which ECDSA.recover rejects with
        // a revert. A validator that reverts cannot be acted on, so
        // validateSignatures answers false instead.
        const isValid = await validator.validateSignatures(
          dkgResult,
          dkgStartBlock
        )

        await expect(isValid).to.be.false
      })
    })

    context("when a signature recovery fails", () => {
      it("should answer false instead of comparing zero addresses", async () => {
        const dkgResult = await prepareDkgResult(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          dkgStartBlock
        )

        // Member id 0 has no operator in the sortition pool, so
        // `getIDOperators` answers address(0) for it. A signature with `v` 0
        // fails recovery and answers address(0) as well, so the address
        // comparison alone would pass; only the `recoverError` check rejects
        // the result.
        dkgResult.members = [0]
        dkgResult.signingMembersIndices = [1]
        dkgResult.signatures = ethers.utils.hexConcat([
          ethers.utils.hexZeroPad("0x01", 32),
          ethers.utils.hexZeroPad("0x01", 32),
          "0x00",
        ])

        const isValid = await validator.validateSignatures(
          dkgResult,
          dkgStartBlock
        )

        await expect(isValid).to.be.false
      })
    })

    context("when a signing member index is past the members array", () => {
      it("should revert with 'Signing member index out of bounds'", async () => {
        const dkgResult = await prepareDkgResult(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved,
          dkgStartBlock
        )
        // `validateFields` bounds the indices against `groupSize`, but
        // `validateSignatures` is public and can be called on a result whose
        // members array is shorter than the declared indices.
        dkgResult.members = dkgResult.members.slice(0, 2)

        await expect(
          validator.validateSignatures(dkgResult, dkgStartBlock)
        ).to.be.revertedWith("Signing member index out of bounds")

        // Reached through `validate`, the same shape is answered as an
        // invalid result rather than reverting.
        const [isValid, errorMsg] = await validator.validate(
          dkgResult,
          dkgSeed,
          dkgStartBlock
        )
        await expect(isValid).to.be.false
        expect(errorMsg).to.equal("Unexpected members count")
      })
    })
  })
})

// Signs `resultHash` with a key whose address is above or below `bound`. The
// private keys are small integers, so the search is deterministic; 1000 of them
// straddle any fixed address with overwhelming probability.
async function foreignSignature(
  bound: string,
  resultHash: string,
  side: "above" | "below"
): Promise<string> {
  const boundValue = BigNumber.from(bound.toLowerCase())
  for (let k = 1; k < 1000; k++) {
    const wallet = new ethers.Wallet(
      ethers.utils.hexZeroPad(ethers.utils.hexlify(k), 32)
    )
    const value = BigNumber.from(wallet.address.toLowerCase())
    const matches =
      side === "above" ? value.gt(boundValue) : value.lt(boundValue)
    if (matches) {
      return wallet.signMessage(ethers.utils.arrayify(resultHash))
    }
  }
  throw new Error(`no private key with an address ${side} ${bound}`)
}

function shuffle(operators: Operator[]): Operator[] {
  return operators
    .map((v) => ({ v, sort: Math.random() }))
    .sort((a, b) => a.sort - b.sort)
    .map(({ v }) => v)
}
