pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorCallbackStub is KeepRandomBeaconOperator {
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

        // setGroupSize
        groupSize = 3;
        groupSelection.groupSize = 3;
        dkgResultVerification.groupSize = 3;

        // setGroupThreshold
        groupThreshold = 2;
        dkgResultVerification.signatureThreshold = 2;
    }

    function getGroupSelectionRelayEntry() public view returns (uint256) {
        return groupSelection.seed;
    }

    function getTicketSubmissionStartBlock() public view returns (uint256) {
        return groupSelection.ticketSubmissionStartBlock;
    }

    function timeDKG() public view returns (uint256) {
        return dkgResultVerification.timeDKG;
    }
}
