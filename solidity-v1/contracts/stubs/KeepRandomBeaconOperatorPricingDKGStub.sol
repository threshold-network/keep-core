pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

/**
 * @title KeepRandomBeaconOperatorPricingDKGStub
 * @dev A simplified Random Beacon operator contract to help local development.
 */
contract KeepRandomBeaconOperatorPricingDKGStub is KeepRandomBeaconOperator {
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
        groupSize = 20;
        groupSelection.groupSize = 20;
        dkgResultVerification.groupSize = 20;

        // setGroupThreshold
        groupThreshold = 11;
        dkgResultVerification.signatureThreshold = 11;
    }

    function getGroupSelectionRelayEntry() public view returns (uint256) {
        return groupSelection.seed;
    }

    function getTicketSubmissionStartBlock() public view returns (uint256) {
        return groupSelection.ticketSubmissionStartBlock;
    }

    function isGroupSelectionInProgress() public view returns (bool) {
        return groupSelection.inProgress;
    }

    function timeDKG() public view returns (uint256) {
        return dkgResultVerification.timeDKG;
    }
}
