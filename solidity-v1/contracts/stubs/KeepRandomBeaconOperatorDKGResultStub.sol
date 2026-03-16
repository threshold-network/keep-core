pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorDKGResultStub is KeepRandomBeaconOperator {
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
        groupSelection.ticketSubmissionTimeout = 100;
    }

    function setGroupSize(uint256 size) public {
        groupSize = size;
        groupSelection.groupSize = size;
        dkgResultVerification.groupSize = size;
    }

    function setGroupThreshold(uint256 threshold) public {
        groupThreshold = threshold;
        dkgResultVerification.signatureThreshold = threshold;
    }

    function setDKGResultSignatureThreshold(uint256 threshold) public {
        dkgResultVerification.signatureThreshold = threshold;
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

    function setGasPriceCeiling(uint256 _gasPriceCeiling) public {
        gasPriceCeiling = _gasPriceCeiling;
    }

    function timeDKG() public view returns (uint256) {
        return dkgResultVerification.timeDKG;
    }
}
