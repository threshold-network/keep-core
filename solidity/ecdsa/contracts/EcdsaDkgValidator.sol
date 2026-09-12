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

// Initial version copied from Keep Network Random Beacon:
// https://github.com/keep-network/keep-core/blob/5138c7628868dbeed3ae2164f76fccc6c1fbb9e8/solidity/random-beacon/contracts/DKGValidator.sol
//
// With the following differences:
// - group public key length,
// - group size and related thresholds,
// - documentation.

pragma solidity 0.8.17;

import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@keep-network/random-beacon/contracts/libraries/BytesLib.sol";
import "@keep-network/sortition-pools/contracts/SortitionPool.sol";
import "./libraries/EcdsaDkg.sol";

/// @title DKG result validator
/// @notice EcdsaDkgValidator allows performing a full validation of DKG result,
///         including checking the format of fields in the result, declared
///         selected group members, and signatures of operators supporting the
///         result. The operator submitting the result should perform the
///         validation using a free contract call before submitting the result
///         to ensure their result is valid and can not be challenged. All other
///         network operators should perform validation of the submitted result
///         using a free contract call and challenge the result if the
///         validation fails.
contract EcdsaDkgValidator {
    using BytesLib for bytes;
    using ECDSA for bytes32;

    /// @dev Size of a group in DKG.
    uint256 public constant groupSize = 100;

    /// @dev The minimum number of group members needed to interact according to
    ///      the protocol to produce a signature. The adversary can not learn
    ///      anything about the key as long as it does not break into
    ///      groupThreshold+1 of members.
    uint256 public constant groupThreshold = 51;

    /// @dev The minimum number of active and properly behaving group members
    ///      during the DKG needed to accept the result. This number is higher
    ///      than `groupThreshold` to keep a safety margin for members becoming
    ///      inactive after DKG so that the group can still produce signature.
    uint256 public constant activeThreshold = 90; // 90% of groupSize

    /// @dev Size in bytes of a public key produced by group members during the
    /// the DKG. The length assumes uncompressed ECDSA public key.
    uint256 public constant publicKeyByteSize = 64;

    /// @dev Size in bytes of a single signature produced by operator supporting
    ///      DKG result.
    uint256 public constant signatureByteSize = 65;

    SortitionPool public immutable sortitionPool;

    constructor(SortitionPool _sortitionPool) {
        sortitionPool = _sortitionPool;
    }

    /// @notice Performs a full validation of DKG result, including checking the
    ///         format of fields in the result, declared selected group members,
    ///         and signatures of operators supporting the result.
    /// @param seed seed used to start the DKG and select group members
    /// @param startBlock DKG start block
    /// @return isValid true if the result is valid, false otherwise
    /// @return errorMsg validation error message; empty for a valid result
    function validate(
        EcdsaDkg.Result calldata result,
        uint256 seed,
        uint256 startBlock
    ) external view returns (bool isValid, string memory errorMsg) {
        (bool hasValidFields, string memory error) = validateFields(result);
        if (!hasValidFields) {
            return (false, error);
        }

        if (!validateSignatures(result, startBlock)) {
            return (false, "Invalid signatures");
        }

        if (!validateGroupMembers(result, seed)) {
            return (false, "Invalid group members");
        }

        // At this point all group members and misbehaved members were verified
        if (!validateMembersHash(result)) {
            return (false, "Invalid members hash");
        }

        return (true, "");
    }

    /// @notice Performs a static validation of DKG result fields: lengths,
    ///         ranges, and order of arrays.
    /// @return isValid true if the result is valid, false otherwise
    /// @return errorMsg validation error message; empty for a valid result
    function validateFields(
        EcdsaDkg.Result calldata result
    ) public pure returns (bool isValid, string memory errorMsg) {
        if (result.groupPubKey.length != publicKeyByteSize) {
            return (false, "Malformed group public key");
        }

        // The number of misbehaved members can not exceed the threshold.
        // Misbehaved member indices needs to be unique, between [1, groupSize],
        // and sorted in ascending order.
        uint8[] calldata misbehavedMembersIndices = result
            .misbehavedMembersIndices;
        if (groupSize - misbehavedMembersIndices.length < activeThreshold) {
            return (false, "Too many members misbehaving during DKG");
        }
        if (misbehavedMembersIndices.length > 1) {
            if (
                misbehavedMembersIndices[0] < 1 ||
                misbehavedMembersIndices[misbehavedMembersIndices.length - 1] >
                    groupSize
            ) {
                return (false, "Corrupted misbehaved members indices");
            }
            for (uint256 i = 1; i < misbehavedMembersIndices.length; i++) {
                if (
                    misbehavedMembersIndices[i - 1] >=
                    misbehavedMembersIndices[i]
                ) {
                    return (false, "Corrupted misbehaved members indices");
                }
            }
        }

        // Each signature needs to have a correct length and signatures need to
        // be provided.
        uint256 signaturesCount = result.signatures.length / signatureByteSize;
        if (result.signatures.length == 0) {
            return (false, "No signatures provided");
        }
        if (result.signatures.length % signatureByteSize != 0) {
            return (false, "Malformed signatures array");
        }

        // We expect the same amount of signatures as the number of declared
        // group member indices that signed the result.
        uint256[] calldata signingMembersIndices = result.signingMembersIndices;
        if (signaturesCount != signingMembersIndices.length) {
            return (false, "Unexpected signatures count");
        }
        if (signaturesCount < groupThreshold) {
            return (false, "Too few signatures");
        }
        if (signaturesCount > groupSize) {
            return (false, "Too many signatures");
        }

        // Signing member indices needs to be unique, between [1,groupSize],
        // and sorted in ascending order.
        if (
            signingMembersIndices[0] < 1 ||
            signingMembersIndices[signingMembersIndices.length - 1] > groupSize
        ) {
            return (false, "Corrupted signing member indices");
        }
        for (uint256 i = 1; i < signingMembersIndices.length; i++) {
            if (signingMembersIndices[i - 1] >= signingMembersIndices[i]) {
                return (false, "Corrupted signing member indices");
            }
        }

        return (true, "");
    }

    /// @notice Performs validation of group members as declared in DKG
    ///         result against group members selected by the sortition pool.
    /// @param seed seed used to start the DKG and select group members
    /// @return true if group members matches; false otherwise
    function validateGroupMembers(
        EcdsaDkg.Result calldata result,
        uint256 seed
    ) public view returns (bool) {
        uint32[] calldata resultMembers = result.members;
        uint32[] memory actualGroupMembers = sortitionPool.selectGroup(
            groupSize,
            bytes32(seed)
        );
        if (resultMembers.length != actualGroupMembers.length) {
            return false;
        }
        for (uint256 i = 0; i < resultMembers.length; i++) {
            if (resultMembers[i] != actualGroupMembers[i]) {
                return false;
            }
        }
        return true;
    }

    /// @notice Performs validation of signatures supplied in DKG result.
    ///         Note that this function does not check if addresses which
    ///         supplied signatures supporting the result are the ones selected
    ///         to the group by sortition pool. This function should be used
    ///         together with `validateGroupMembers`.
    /// @param startBlock DKG start block
    /// @return true if group members matches; false otherwise
    function validateSignatures(
        EcdsaDkg.Result calldata result,
        uint256 startBlock
    ) public view returns (bool) {
        bytes32 hash = keccak256(
            abi.encode(
                block.chainid,
                result.groupPubKey,
                result.misbehavedMembersIndices,
                startBlock
            )
        ).toEthSignedMessageHash();

        uint256[] calldata signingMembersIndices = result.signingMembersIndices;
        uint32[] memory signingMemberIds = new uint32[](
            signingMembersIndices.length
        );
        for (uint256 i = 0; i < signingMembersIndices.length; i++) {
            signingMemberIds[i] = result.members[signingMembersIndices[i] - 1];
        }

        address[] memory signingMemberAddresses = sortitionPool.getIDOperators(
            signingMemberIds
        );

        bytes memory current; // Current signature to be checked.

        uint256 signaturesCount = result.signatures.length / signatureByteSize;
        for (uint256 i = 0; i < signaturesCount; i++) {
            current = result.signatures.slice(
                signatureByteSize * i,
                signatureByteSize
            );
            address recoveredAddress = hash.recover(current);

            if (signingMemberAddresses[i] != recoveredAddress) {
                return false;
            }
        }

        return true;
    }

    /// @notice Performs validation of hashed group members that actively took
    ///         part in DKG.
    /// @param result DKG result
    /// @return true if calculated result's group members hash matches with the
    /// one that is challenged.
    function validateMembersHash(
        EcdsaDkg.Result calldata result
    ) public pure returns (bool) {
        if (result.misbehavedMembersIndices.length > 0) {
            // members that generated a group signing key
            uint32[] memory groupMembers = new uint32[](
                result.members.length - result.misbehavedMembersIndices.length
            );
            uint256 k = 0; // misbehaved members counter
            uint256 j = 0; // group members counter
            for (uint256 i = 0; i < result.members.length; i++) {
                // misbehaved member indices start from 1, so we need to -1 on misbehaved
                if (i != result.misbehavedMembersIndices[k] - 1) {
                    groupMembers[j] = result.members[i];
                    j++;
                } else if (k < result.misbehavedMembersIndices.length - 1) {
                    k++;
                }
            }

            return keccak256(abi.encode(groupMembers)) == result.membersHash;
        }

        return keccak256(abi.encode(result.members)) == result.membersHash;
    }
}
