pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

/**
 * @title KeepRandomBeaconOperatorInitializationStub
 * @dev A simplified Random Beacon operator contract to help local development.
 */
contract KeepRandomBeaconOperatorInitializationStub is
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
    {}

    function getGroupsRelayEntryTimeout() public view returns (uint256) {
        return groups.relayEntryTimeout;
    }

    function getGroupsActiveTime() public view returns (uint256) {
        return groups.groupActiveTime;
    }
}
