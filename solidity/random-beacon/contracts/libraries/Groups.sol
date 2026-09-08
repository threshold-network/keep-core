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
//

pragma solidity 0.8.17;

/// @notice This library is used as a registry of created groups.
/// @dev This library should be used along with DKG library that ensures linear
///      groups creation (only one group creation happens at a time). A candidate
///      group has to be popped or activated before adding a new candidate group.
library Groups {
    struct Group {
        bytes groupPubKey;
        uint256 registrationBlockNumber;
        // Keccak256 hash of group members identifiers array. Group members do not
        // include operators selected by the sortition pool that misbehaved during DKG.
        // See how `misbehavedMembersIndices` are used in `hashGroupMembers` function.
        bytes32 membersHash;
        // When selected group does not create a relay entry on-time it should
        // be marked as terminated.
        bool terminated;
    }

    struct Data {
        // Mapping of keccak256 hashes of group public keys to groups details.
        mapping(bytes32 => Group) groupsData;
        // Holds keccak256 hashes of group public keys in the order of registration.
        bytes32[] groupsRegistry;
        // Group ids that were active but failed creating a relay entry. When an
        // active-terminated group qualifies to become 'expired', then it will
        // be removed from this array.
        uint64[] activeTerminatedGroups;
        // Points to the first active group, it is also the expired groups counter.
        uint64 expiredGroupOffset;
        // Group lifetime in blocks. When a group reached its lifetime, it
        // is no longer selected for new relay requests but may still be
        // responsible for submitting relay entry if relay request assigned
        // to that group is still pending.
        uint256 groupLifetime;
    }

    event GroupRegistered(uint64 indexed groupId, bytes indexed groupPubKey);

    /// @notice Performs preliminary validation of a new group public key.
    ///         The group public key must be unique and have 128 bytes in length.
    ///         If the validation fails, the function reverts. This function
    ///         must be called first for a public key of a group added with
    ///         `addGroup` function.
    /// @param groupPubKey Candidate group public key
    function validatePublicKey(
        Data storage self,
        bytes calldata groupPubKey
    ) internal view {
        require(groupPubKey.length == 128, "Invalid length of the public key");

        bytes32 groupPubKeyHash = keccak256(groupPubKey);

        require(
            self.groupsData[groupPubKeyHash].registrationBlockNumber == 0,
            "Group with this public key was already registered"
        );
    }

    /// @notice Adds a new candidate group. The group is stored with group public
    ///         key and group members, but is not yet activated.
    /// @dev The group members list is stored with all misbehaved members filtered out.
    ///      The code calling this function should ensure that the number of
    ///      candidate (not activated) groups is never more than one.
    /// @param groupPubKey Generated candidate group public key
    /// @param membersHash Keccak256 hash of members that actively took part in DKG.
    function addGroup(
        Data storage self,
        bytes calldata groupPubKey,
        bytes32 membersHash
    ) internal {
        bytes32 groupPubKeyHash = keccak256(groupPubKey);

        // We use group from storage that is assumed to be a struct set to the
        // default values. We need to remember to overwrite fields in case a
        // candidate group was already registered before and popped.
        Group storage group = self.groupsData[groupPubKeyHash];
        group.groupPubKey = groupPubKey;
        group.membersHash = membersHash;
        group.registrationBlockNumber = block.number;

        self.groupsRegistry.push(groupPubKeyHash);

        emit GroupRegistered(
            uint64(self.groupsRegistry.length - 1),
            groupPubKey
        );
    }

    /// @notice Goes through groups starting from the oldest one that is still
    ///         active and checks if it hasn't expired. If so, updates the information
    ///         about expired groups so that all expired groups are marked as such.
    function expireOldGroups(Data storage self) internal {
        // Move expiredGroupOffset as long as there are some groups that should
        // be marked as expired. It is possible that expired group offset will
        // move out of the groups array by one position. It means that all groups
        // are expired (it points to the first active group) and that place in
        // groups array - currently empty - will be possibly filled later by
        // a new group.
        while (
            self.expiredGroupOffset < self.groupsRegistry.length &&
            groupLifetimeOf(
                self,
                self.groupsRegistry[self.expiredGroupOffset]
            ) <
                block.number
        ) {
            self.expiredGroupOffset++;
        }

        // Go through all activeTerminatedGroups and if some of the terminated
        // groups are expired, remove them from activeTerminatedGroups collection
        // and rearrange the array to preserve the original order.
        // This is needed because we evaluate the shift of selected group index
        // based on how many non-expired groups have been terminated. Hence it is
        // important that a number of terminated groups matches the length of
        // activeTerminatedGroups[].
        uint256 i = 0;
        while (i < self.activeTerminatedGroups.length) {
            if (self.expiredGroupOffset > self.activeTerminatedGroups[i]) {
                // When 'i'th group qualifies for expiration, we need to remove
                // it from the activeTerminatedGroups array manually by rearranging
                // the order starting from 'i'th group.
                for (
                    uint256 j = i;
                    j < self.activeTerminatedGroups.length - 1;
                    j++
                ) {
                    self.activeTerminatedGroups[j] = self
                        .activeTerminatedGroups[j + 1];
                }
                // Resizing the array length by 1. The last element was copied
                // over in the loop above to an index "second to last". This is
                // why we can safely remove it from here.
                self.activeTerminatedGroups.pop();
            } else {
                i++;
            }
        }
    }

    /// @notice Terminates group with the provided index. Reverts if the group
    ///         is already terminated.
    /// @param  groupId Index in the groupRegistry array.
    function terminateGroup(Data storage self, uint64 groupId) internal {
        require(
            !isGroupTerminated(self, groupId),
            "Group has been already terminated"
        );
        self.groupsData[self.groupsRegistry[groupId]].terminated = true;
        // Expanding array for a new terminated group that is added below during
        // sortition in ascending order.
        self.activeTerminatedGroups.push();

        // Sorting activeTerminatedGroups by groupId in ascending order so a
        // non-terminated group is properly selected.
        uint256 i;
        for (
            i = self.activeTerminatedGroups.length - 1;
            i > 0 && self.activeTerminatedGroups[i - 1] > groupId;
            i--
        ) {
            self.activeTerminatedGroups[i] = self.activeTerminatedGroups[i - 1];
        }
        self.activeTerminatedGroups[i] = groupId;
    }

    /// @notice Returns an index of a randomly selected active group. Terminated
    ///         and expired groups are not considered as active.
    ///         Before new group is selected, information about expired groups
    ///         is updated. At least one active group needs to be present for this
    ///         function to succeed.
    /// @param seed Random number used as a group selection seed.
    function selectGroup(
        Data storage self,
        uint256 seed
    ) internal returns (uint64) {
        expireOldGroups(self);

        require(numberOfActiveGroups(self) > 0, "No active groups");

        uint64 selectedGroup = uint64(seed % numberOfActiveGroups(self));
        uint64 result = shiftByTerminatedGroups(
            self,
            shiftByExpiredGroups(self, selectedGroup)
        );
        return result;
    }

    /// @notice Setter for group lifetime.
    /// @param lifetime Lifetime of a group in blocks.
    function setGroupLifetime(Data storage self, uint256 lifetime) internal {
        self.groupLifetime = lifetime;
    }

    /// @notice Checks if group with the given index is terminated.
    function isGroupTerminated(
        Data storage self,
        uint64 groupId
    ) internal view returns (bool) {
        return self.groupsData[self.groupsRegistry[groupId]].terminated;
    }

    /// @notice Gets the cutoff time until which the given group is considered
    ///         to be active assuming it hasn't been terminated before.
    function groupLifetimeOf(
        Data storage self,
        bytes32 groupPubKeyHash
    ) internal view returns (uint256) {
        return
            self.groupsData[groupPubKeyHash].registrationBlockNumber +
            self.groupLifetime;
    }

    /// @notice Checks if group with the given index is active and non-terminated.
    function isGroupActive(
        Data storage self,
        uint64 groupId
    ) internal view returns (bool) {
        return
            groupLifetimeOf(self, self.groupsRegistry[groupId]) >=
                block.number &&
            !isGroupTerminated(self, groupId);
    }

    function getGroup(
        Data storage self,
        uint64 groupId
    ) internal view returns (Group storage) {
        return self.groupsData[self.groupsRegistry[groupId]];
    }

    function getGroup(
        Data storage self,
        bytes memory groupPubKey
    ) internal view returns (Group storage) {
        return self.groupsData[keccak256(groupPubKey)];
    }

    /// @notice Gets the number of active groups. Expired and terminated
    ///         groups are not counted as active.
    function numberOfActiveGroups(
        Data storage self
    ) internal view returns (uint64) {
        if (self.groupsRegistry.length == 0) {
            return 0;
        }

        uint256 activeGroups =
            self.groupsRegistry.length -
                self.expiredGroupOffset -
                self.activeTerminatedGroups.length;

        return uint64(activeGroups);
    }

    /// @notice Evaluates the shift of a selected group index based on the number
    ///         of expired groups.
    function shiftByExpiredGroups(
        Data storage self,
        uint64 selectedIndex
    ) internal view returns (uint64) {
        return self.expiredGroupOffset + selectedIndex;
    }

    /// @notice Evaluates the shift of a selected group index based on the number
    ///         of non-expired but terminated groups.
    function shiftByTerminatedGroups(
        Data storage self,
        uint64 selectedIndex
    ) internal view returns (uint64) {
        uint64 shiftedIndex = selectedIndex;
        for (uint64 i = 0; i < self.activeTerminatedGroups.length; i++) {
            if (self.activeTerminatedGroups[i] <= shiftedIndex) {
                shiftedIndex++;
            }
        }

        return shiftedIndex;
    }
}
