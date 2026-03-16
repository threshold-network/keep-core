pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorGroupSelectionStub is
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
        groupSelection.ticketSubmissionTimeout = 65;

        // setGroupSize
        groupSize = 3;
        groupSelection.groupSize = 3;
    }

    function getGroupSelectionRelayEntry() public view returns (uint256) {
        return groupSelection.seed;
    }

    function startGroupSelection(uint256 seed) public {
        groupSelection.start(seed);
    }
}
