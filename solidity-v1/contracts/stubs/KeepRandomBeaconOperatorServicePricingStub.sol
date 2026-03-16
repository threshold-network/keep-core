pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

/**
 * @title KeepRandomBeaconOperatorServicePricingStub
 * @dev A simplified Random Beacon operator contract to help local development.
 */
contract KeepRandomBeaconOperatorServicePricingStub is
    KeepRandomBeaconOperator
{
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
        groupSize = 3;
        groupSelection.groupSize = 3;
        dkgResultVerification.groupSize = 3;
    }

    function registerNewGroup(bytes memory groupPublicKey) public {
        groups.addGroup(groupPublicKey);
    }

    function setGroupMembers(
        bytes memory groupPublicKey,
        address[] memory members
    ) public {
        groups.setGroupMembers(groupPublicKey, members, hex"");
    }

    function getGroupPublicKey(uint256 groupIndex)
        public
        view
        override
        returns (bytes memory)
    {
        return groups.groups[groupIndex].groupPubKey;
    }

    function setGasPriceCeiling(uint256 _gasPriceCeiling) public {
        gasPriceCeiling = _gasPriceCeiling;
    }
}
