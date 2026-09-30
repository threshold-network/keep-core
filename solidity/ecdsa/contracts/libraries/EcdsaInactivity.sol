// SPDX-License-Identifier: GPL-3.0-only
//
// ▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
//   ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
// ▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
//
//                           Trust math, not hardware.

pragma solidity 0.8.17;

import "@openzeppelin/contracts-upgradeable/utils/cryptography/ECDSAUpgradeable.sol";

import "../legacy/random-beacon/libraries/BytesLib.sol";
import "../legacy/sortition/SortitionPool.sol";

import "./Wallets.sol";

library EcdsaInactivity {
    using BytesLib for bytes;
    using ECDSAUpgradeable for bytes32;

    struct Claim {
        // ID of the wallet whose signing group is raising the inactivity claim.
        bytes32 walletID;
        // Indices of group members accused of being inactive. Indices must be in
        // range [1, groupMembers.length], unique, and sorted in ascending order.
        uint256[] inactiveMembersIndices;
        // Indicates if inactivity claim is a wallet-wide heartbeat failure.
        // If wallet failed a heartbeat, this is signalled to the wallet owner
        // who may decide to move responsibilities to another wallet
        // given that the wallet who failed the heartbeat is at risk of not
        // being able to sign messages soon.
        bool heartbeatFailed;
        // Concatenation of signatures from members supporting the claim.
        // The message to be signed by each member is keccak256 hash of the
        // concatenation of the chain ID, inactivity claim nonce for the given
        // wallet, wallet public key, inactive members indices, and boolean flag
        // indicating if this is a wallet-wide heartbeat failure. The calculated
        // hash should be prefixed with `\x19Ethereum signed message:\n` before
        // signing, so the message to sign is:
        // `\x19Ethereum signed message:\n${keccak256(
        //    chainID | nonce | walletPubKey | inactiveMembersIndices | heartbeatFailed
        // )}`
        bytes signatures;
        // Indices of members corresponding to each signature. Indices must be
        // in range [1, groupMembers.length], unique, and sorted in ascending
        // order.
        uint256[] signingMembersIndices;
        // This struct doesn't contain `__gap` property as the structure is not
        // stored, it is used as a function's calldata argument.
    }

    /// @notice The minimum number of wallet signing group members needed to
    ///         interact according to the protocol to produce a valid inactivity
    ///         claim.
    uint256 public constant groupThreshold = 51;

    /// @notice Size in bytes of a single signature produced by member
    ///         supporting the inactivity claim.
    uint256 public constant signatureByteSize = 65;

    /// @notice Verifies the inactivity claim according to the rules defined in
    ///         `Claim` struct documentation. Reverts if verification fails.
    /// @dev Wallet signing group members hash is validated upstream in
    ///      `WalletRegistry.notifyOperatorInactivity()`
    /// @param sortitionPool Sortition pool reference
    /// @param claim Inactivity claim
    /// @param walletPubKey Public key of the wallet
    /// @param nonce Current inactivity nonce for wallet used in the claim
    /// @param groupMembers Identifiers of group members
    /// @return inactiveMembers Identifiers of members who are inactive
    function verifyClaim(
        SortitionPool sortitionPool,
        Claim calldata claim,
        bytes memory walletPubKey,
        uint256 nonce,
        uint32[] calldata groupMembers
    ) external view returns (uint32[] memory inactiveMembers) {
        // Validate inactive members indices. Maximum indices count is equal to
        // the group size and is not limited deliberately to leave a theoretical
        // possibility to accuse more members than `groupSize - groupThreshold`.
        validateMembersIndices(
            claim.inactiveMembersIndices,
            groupMembers.length
        );

        // Validate signatures array is properly formed and number of
        // signatures and signers is correct.
        uint256 signaturesCount = claim.signatures.length / signatureByteSize;
        require(claim.signatures.length != 0, "No signatures provided");
        require(
            claim.signatures.length % signatureByteSize == 0,
            "Malformed signatures array"
        );
        require(
            signaturesCount == claim.signingMembersIndices.length,
            "Unexpected signatures count"
        );
        require(signaturesCount >= groupThreshold, "Too few signatures");
        require(signaturesCount <= groupMembers.length, "Too many signatures");

        // Validate signing members indices. Note that `signingMembersIndices`
        // were already partially validated during `signatures` parameter
        // validation.
        validateMembersIndices(
            claim.signingMembersIndices,
            groupMembers.length
        );

        bytes32 signedMessageHash = keccak256(
            abi.encode(
                block.chainid,
                nonce,
                walletPubKey,
                claim.inactiveMembersIndices,
                claim.heartbeatFailed
            )
        ).toEthSignedMessageHash();

        address[] memory groupMembersAddresses = sortitionPool.getIDOperators(
            groupMembers
        );

        // Verify each signature.
        bytes memory checkedSignature;
        bool senderSignatureExists = false;
        for (uint256 i = 0; i < signaturesCount; i++) {
            uint256 memberIndex = claim.signingMembersIndices[i];
            checkedSignature = claim.signatures.slice(
                signatureByteSize * i,
                signatureByteSize
            );
            address recoveredAddress = signedMessageHash.recover(
                checkedSignature
            );

            require(
                groupMembersAddresses[memberIndex - 1] == recoveredAddress,
                "Invalid signature"
            );

            if (!senderSignatureExists && msg.sender == recoveredAddress) {
                senderSignatureExists = true;
            }
        }

        require(senderSignatureExists, "Sender must be claim signer");

        inactiveMembers = new uint32[](claim.inactiveMembersIndices.length);
        for (uint256 i = 0; i < claim.inactiveMembersIndices.length; i++) {
            uint256 memberIndex = claim.inactiveMembersIndices[i];
            inactiveMembers[i] = groupMembers[memberIndex - 1];
        }

        return inactiveMembers;
    }

    /// @notice Validates members indices array. Array is considered valid
    ///         if its size and each single index are in [1, groupSize] range,
    ///         indexes are unique, and sorted in an ascending order.
    ///         Reverts if validation fails.
    /// @param indices Array to validate.
    /// @param groupSize Group size used as reference.
    function validateMembersIndices(
        uint256[] calldata indices,
        uint256 groupSize
    ) internal pure {
        require(
            indices.length > 0 && indices.length <= groupSize,
            "Corrupted members indices"
        );

        // Check if first and last indices are in range [1, groupSize].
        // This check combined with the loop below makes sure every single
        // index is in the correct range.
        require(
            indices[0] > 0 && indices[indices.length - 1] <= groupSize,
            "Corrupted members indices"
        );

        for (uint256 i = 0; i < indices.length - 1; i++) {
            // Check whether given index is smaller than the next one. This
            // way we are sure indexes are ordered in the ascending order
            // and there are no duplicates.
            require(indices[i] < indices[i + 1], "Corrupted members indices");
        }
    }
}
