pragma solidity ^0.8.0;
import "../libraries/operator/Groups.sol";

contract GroupsTerminationStub {
    using Groups for Groups.Storage;
    Groups.Storage groups;

    constructor() {
        groups.groupActiveTime = 5;
    }

    function addGroup(bytes memory groupPubKey) public {
        groups.addGroup(groupPubKey);
    }

    function registerNewGroups(uint256 groupsCount) public {
        for (uint256 i = 1; i <= groupsCount; i++) {
            groups.addGroup(new bytes(i));
        }
    }

    function terminateGroup(uint256 groupIndex) public {
        groups.terminateGroup(groupIndex);
    }

    function selectGroup(uint256 seed) public returns (uint256) {
        return groups.selectGroup(seed);
    }
}
