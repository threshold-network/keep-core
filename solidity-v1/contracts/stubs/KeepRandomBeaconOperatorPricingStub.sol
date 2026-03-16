pragma solidity ^0.8.0;

import "../KeepRandomBeaconOperator.sol";

contract KeepRandomBeaconOperatorPricingStub is KeepRandomBeaconOperator {
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

    function registerNewGroup(bytes memory groupPublicKey) public {
        groups.addGroup(groupPublicKey);
    }

    function setEntryVerificationGasEstimate(uint256 gasEstimate) public {
        entryVerificationGasEstimate = gasEstimate;
    }

    function setGroupMemberBaseReward(uint256 reward) public {
        groupMemberBaseReward = reward;
    }

    function setGroupSize(uint256 size) public {
        groupSize = size;
    }

    function setGasPriceCeiling(uint256 _gasPriceCeiling) public {
        gasPriceCeiling = _gasPriceCeiling;
    }

    function getNewEntryRewardsBreakdown()
        public
        view
        returns (
            uint256 groupMemberReward,
            uint256 submitterReward,
            uint256 subsidy
        )
    {
        return super.newEntryRewardsBreakdown();
    }

    function delayFactor() public view returns (uint256) {
        return
            DelayFactor.calculate(currentRequestStartBlock, relayEntryTimeout);
    }
}
