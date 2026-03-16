pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorBeaconRewardsStub is KeepRandomBeaconOperator {
    constructor(
        address _serviceContract,
        address _stakingContract,
        address _registryContract,
        address _gasPriceOracle
    )
        public
        KeepRandomBeaconOperator(
            _serviceContract,
            _stakingContract,
            _registryContract,
            _gasPriceOracle
        )
    {
        groupSize = 3;
        groups.groupActiveTime = 5;
        groups.relayEntryTimeout = 10;
    }

    function registerNewGroup(
        bytes memory groupPublicKey,
        address[] memory members,
        uint256 creationTimestamp
    ) public {
        groups.addGroup(groupPublicKey);
        groups.groups[groups.groups.length - 1].registrationTime = uint248(
            creationTimestamp
        );

        groups.setGroupMembers(groupPublicKey, members, hex"");
    }

    function terminateGroup(uint256 groupIndex) public {
        groups.terminateGroup(groupIndex);
    }

    function expireOldGroups() public {
        groups.expireOldGroups();
    }
}
