/* eslint-disable @typescript-eslint/no-unused-expressions */

import { BigNumber } from "ethers"
import {
  ethers,
  helpers,
  getUnnamedAccounts,
  waffle,
  deployments,
} from "hardhat"
import { expect } from "chai"

import blsData from "./data/bls"
import { constants } from "./fixtures"
import { selectGroup, hashUint32Array } from "./utils/groups"
import {
  signDkgResult,
  noMisbehaved,
  hashDKGMembers,
  hardhatNetworkId,
} from "./utils/dkg"

import type { BigNumberish } from "ethers"
import type { Operator } from "./utils/operators"
import type {
  SortitionPool,
  BeaconDkgValidator as DKGValidator,
  BeaconDkg as DKG,
  T,
} from "../typechain"

const { createSnapshot, restoreSnapshot } = helpers.snapshot
const { to1e18 } = helpers.number

const fixture = async () => {
  await deployments.fixture(["TokenStaking"])
  const t: T = await helpers.contracts.getContract("T")

  const SortitionPool = await ethers.getContractFactory("SortitionPool")
  const sortitionPool = (await SortitionPool.deploy(
    t.address,
    constants.poolWeightDivisor
  )) as SortitionPool

  await sortitionPool.deactivateChaosnet()

  const DKGValidator = await ethers.getContractFactory("BeaconDkgValidator")
  const dkgValidator = (await DKGValidator.deploy(
    sortitionPool.address
  )) as DKGValidator
  await dkgValidator.deployed()

  return {
    sortitionPool,
    dkgValidator,
  }
}

describe("BeaconDkgValidator", () => {
  const dkgSeed: BigNumber = BigNumber.from(
    "31415926535897932384626433832795028841971693993751058209749445923078164062862"
  )
  const dkgStartBlock = 1337
  const groupPublicKey: string = ethers.utils.hexValue(blsData.groupPubKey)

  let selectedOperators: Operator[]

  let prepareDkgResult: (
    _groupMembers: Operator[],
    _signers: Operator[],
    _groupPublicKey: string,
    _misbehaved: number[],
    _startBlock: number,
    _numberOfSignatures?: number,
    _submitterIndex?: number
  ) => Promise<DKG.ResultStruct>

  let validator: DKGValidator

  before("load test fixture", async () => {
    const contracts = await waffle.loadFixture(fixture)
    const { sortitionPool } = contracts
    validator = contracts.dkgValidator

    const operators = (await getUnnamedAccounts()).slice(0, constants.groupSize)
    for (let i = 0; i < operators.length; i++) {
      await sortitionPool.insertOperator(operators[i], to1e18(100))
    }

    await sortitionPool.lock()

    selectedOperators = await selectGroup(sortitionPool, dkgSeed)

    prepareDkgResult = async (
      _groupMembers: Operator[],
      _signers: Operator[],
      _groupPublicKey: string,
      _misbehaved: number[],
      _startBlock: number,
      _numberOfSignatures = 33,
      _submitterIndex = 1
    ): Promise<DKG.ResultStruct> => {
      const { signingMembersIndices, signaturesBytes } = await signDkgResult(
        _signers,
        _groupPublicKey,
        _misbehaved,
        _startBlock,
        _numberOfSignatures
      )

      const dkgResult: DKG.ResultStruct = {
        submitterMemberIndex: _submitterIndex,
        groupPubKey: _groupPublicKey,
        misbehavedMembersIndices: _misbehaved,
        signatures: signaturesBytes,
        signingMembersIndices,
        members: _groupMembers.map((m) => m.id),
        membersHash: hashDKGMembers(
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
      _membersHash?: string
    ) => {
      const dkgResult = await prepareDkgResult(
        _groupMembers,
        _signers,
        _groupPublicKey,
        _misbehaved,
        dkgStartBlock
      )

      if (_membersHash) {
        dkgResult.membersHash = _membersHash
      }

      const result = await validator.validate(dkgResult, dkgSeed, dkgStartBlock)

      return {
        isValid: result[0],
        errorMsg: result[1],
      }
    }

    context("when DKG result contains misbehaved group members", () => {
      let groupMemberIds: number[]

      before(async () => {
        await createSnapshot()
        groupMemberIds = selectedOperators.map((m) => m.id)
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when hashed group members is correct", () => {
        context("when misbehaved index is present at the beginning", () => {
          it("should pass", async () => {
            const misbehavedMemberIds = [1]
            const expectedMembersIds = [...groupMemberIds]
            expectedMembersIds.splice(0, 1) // index -1

            const result = await testValidate(
              selectedOperators,
              selectedOperators,
              groupPublicKey,
              misbehavedMemberIds
            )

            expect(result.isValid).to.be.true
            expect(result.errorMsg).to.equal("")
          })
        })
        context("when misbehaved indices present in the middle", () => {
          it("should pass", async () => {
            const misbehavedMemberIds = [22, 28, 46, 53]
            const expectedMembersIds = [...groupMemberIds]
            expectedMembersIds.splice(21, 1) // index -1
            expectedMembersIds.splice(26, 1) // index -2 (cause expectedMembers already shrinked)
            expectedMembersIds.splice(43, 1) // index -3
            expectedMembersIds.splice(49, 1) // index -4

            const result = await testValidate(
              selectedOperators,
              selectedOperators,
              groupPublicKey,
              misbehavedMemberIds
            )

            expect(result.isValid).to.be.true
            expect(result.errorMsg).to.equal("")
          })
        })

        context("when misbehaved index is present at the end", () => {
          it("should pass", async () => {
            const misbehavedMemberIds = [64]
            const expectedMembersIds = [...groupMemberIds]
            expectedMembersIds.splice(63, 1) // index -1

            const result = await testValidate(
              selectedOperators,
              selectedOperators,
              groupPublicKey,
              misbehavedMemberIds
            )

            expect(result.isValid).to.be.true
            expect(result.errorMsg).to.equal("")
          })
        })
      })

      context("when hashed group members is incorrect", () => {
        it("should not pass", async () => {
          const misbehavedMemberIds = [42]
          const expectedMembersIds = [...groupMemberIds]
          expectedMembersIds.splice(21, 1) // index -1

          const result = await testValidate(
            selectedOperators,
            selectedOperators,
            groupPublicKey,
            misbehavedMemberIds,
            hashUint32Array(expectedMembersIds)
          )

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Invalid members hash")
        })
      })
    })

    context("when DKG result is valid", () => {
      it("should pass", async () => {
        const result = await testValidate(
          selectedOperators,
          selectedOperators,
          groupPublicKey,
          noMisbehaved
        )

        expect(result.isValid).to.be.true
        expect(result.errorMsg).to.equal("")
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
          noMisbehaved
        )

        expect(result.isValid).to.be.false
        expect(result.errorMsg).to.equal("Invalid group members")
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
          noMisbehaved
        )

        expect(result.isValid).to.be.false
        expect(result.errorMsg).to.equal("Invalid signatures")
      })
    })
  })

  describe("validateFields", () => {
    const testValidateFields = async (
      _groupMembers: Operator[],
      _groupPublicKey: string,
      _misbehaved: number[],
      _numberOfSignatures = 33
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

        expect(result.isValid).to.be.true
        expect(result.errorMsg).to.equal("")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Malformed group public key")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Malformed group public key")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Malformed group public key")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal(
            "Corrupted misbehaved members indices"
          )
        })
      })

      context("when index is higher than group size", () => {
        it("should return validation error", async () => {
          const higherThanGroupSize = [1, 2, 65]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            higherThanGroupSize
          )

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal(
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal(
            "Corrupted misbehaved members indices"
          )
        })
      })

      context("when there are too many indices", () => {
        it("should return validation error", async () => {
          const tooMany = [2, 4, 15, 17, 50, 53, 64]
          const result = await testValidateFields(
            selectedOperators,
            groupPublicKey,
            tooMany
          )

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal(
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("No signatures provided")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Malformed signatures array")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Unexpected signatures count")
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

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Too few signatures")
        })
      })

      context("when there are too many signatures", () => {
        it("should return validation error", async () => {
          const signatureHexStrLength = 130
          const maxSignatures = 64
          const dkgResult = await prepareDkgResult(
            selectedOperators,
            selectedOperators,
            groupPublicKey,
            noMisbehaved,
            dkgStartBlock,
            maxSignatures
          )
          dkgResult.signatures += "f".repeat(signatureHexStrLength)
          dkgResult.signingMembersIndices.push(65)
          const result = await validator.validateFields(dkgResult)

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Too many signatures")
        })
      })
    })

    context("when signing members indices array is malformed", async () => {
      const testSigningMembers = async (
        _signingMembersIndices: BigNumberish[]
      ) => {
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
            20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33,
          ]
          const result = await testSigningMembers(indicesStartWithZero)

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Corrupted signing member indices")
        })
      })

      context("when index is greater than group size", () => {
        it("should return validation error", async () => {
          const indicesEndWithTooBig = [
            1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 15, 17, 18, 19,
            20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 65,
          ]
          const result = await testSigningMembers(indicesEndWithTooBig)

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Corrupted signing member indices")
        })
      })

      context("when indices are unsorted", () => {
        it("should return validation error", async () => {
          const indicesUnsorted = [
            1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
            20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 32, 31, 33,
          ]
          const result = await testSigningMembers(indicesUnsorted)

          expect(result.isValid).to.be.false
          expect(result.errorMsg).to.equal("Corrupted signing member indices")
        })
      })
    })
  })

  describe("validateGroupMembers", () => {
    const testValidateGroupMembers = async (_groupMembers: Operator[]) => {
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
        expect(isValid).to.be.true
      })
    })

    context("when there are operators other then selected", () => {
      it("should fail the validation", async () => {
        const isValid = await testValidateGroupMembers(
          shuffle(selectedOperators)
        )
        expect(isValid).to.be.false
      })
    })
  })

  describe("validateSignatures", () => {
    const testValidateSignatures = async (
      _groupMembers: Operator[],
      _signers: Operator[]
    ) => {
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

        expect(isValid).to.be.true
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

          expect(isValid).to.be.false
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

        expect(isValid).to.be.false
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
        const numberOfSignatures = 33
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

        expect(isValid).to.be.false
      })
    })

    context("when signatures consist of random bytes", () => {
      it("should revert", async () => {
        const numberOfSignatures = 33
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
