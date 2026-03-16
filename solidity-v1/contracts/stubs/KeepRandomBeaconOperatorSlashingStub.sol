pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorSlashingStub is KeepRandomBeaconOperator {
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
        relayEntryTimeout = 10;
        groupSelection.ticketSubmissionTimeout = 69;
        resultPublicationBlockStep = 3;
    }

    function registerNewGroup(
        bytes memory groupPublicKey,
        address[] memory members
    ) public {
        groups.addGroup(groupPublicKey);
        groups.setGroupMembers(groupPublicKey, members, hex"");
    }
}
