pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorRewardsStub is KeepRandomBeaconOperator {
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
        groups.groupActiveTime = 5;
        groups.relayEntryTimeout = 10;
    }

    function registerNewGroup(
        bytes memory groupPublicKey,
        address[] memory members
    ) public {
        groups.addGroup(groupPublicKey);
        groups.setGroupMembers(groupPublicKey, members, hex"");
        emit DkgResultSubmittedEvent(0, groupPublicKey, "");
    }

    function addGroupMemberReward(
        bytes memory groupPubKey,
        uint256 groupMemberReward
    ) public {
        groups.addGroupMemberReward(groupPubKey, groupMemberReward);
    }

    function reportUnauthorizedSigning(uint256 groupIndex) public {
        // Marks the given group as terminated.
        groups.reportRelayEntryTimeout(groupIndex, groupSize);
        emit UnauthorizedSigningReported(groupIndex);
    }

    function reportRelayEntryTimeout(uint256 groupIndex) public {
        // Marks the given group as terminated.
        groups.reportRelayEntryTimeout(groupIndex, groupSize);
        emit RelayEntryTimeoutReported(groupIndex);
    }

    function isGroupTerminated(uint256 groupIndex) public view override returns (bool) {
        return groups.isGroupTerminated(groupIndex);
    }
}
