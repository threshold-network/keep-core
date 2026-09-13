import { ethers } from "hardhat"

import type { Operator } from "./operators"

// default Hardhat's networks blockchain, see https://hardhat.org/config/
const hardhatNetworkId = 31337

// eslint-disable-next-line import/prefer-default-export
export async function signOperatorInactivityClaim(
  signers: Operator[],
  nonce: number,
  groupPubKey: string,
  inactiveMembersIndices: number[],
  numberOfSignatures: number,
): Promise<{
  signatures: string
  signingMembersIndices: number[]
}> {
  const messageHash = ethers.keccak256(
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["uint256", "uint256", "bytes", "uint8[]"],
      [hardhatNetworkId, nonce, groupPubKey, inactiveMembersIndices],
    ),
  )

  const signingMembersIndices: number[] = []
  const signatures: string[] = []

  for (let i = 0; i < signers.length; i++) {
    if (signatures.length === numberOfSignatures) {
      // eslint-disable-next-line no-continue
      continue
    }

    const signerIndex: number = i + 1

    signingMembersIndices.push(signerIndex)

    const ethersSigner = signers[i].signer

    const signature = await ethersSigner.signMessage(
      ethers.getBytes(messageHash),
    )

    signatures.push(signature)
  }

  return {
    signatures: ethers.concat(signatures),
    signingMembersIndices,
  }
}
