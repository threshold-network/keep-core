pragma solidity ^0.8.0;

import "../Rewards.sol";

contract RewardsStub is Rewards {
    uint256[] creationTimes;
    uint256 closedTime;
    mapping(uint256 => bool) terminated;

    constructor(
        address _token,
        uint256 _minimumKeepsPerInterval,
        uint256 _firstIntervalStart,
        uint256[] memory _intervalWeights,
        uint256[] memory _creationTimes,
        uint256 _termLength
    )
        public
        Rewards(
            _token,
            _firstIntervalStart,
            _intervalWeights,
            _termLength,
            _minimumKeepsPerInterval
        )
    {
        creationTimes = _creationTimes;
    }

    function receiveReward(uint256 i) public {
        receiveReward(bytes32(i));
    }

    function receiveRewards(uint256[] memory identifiers) public {
        uint256 len = identifiers.length;
        bytes32[] memory bytes32identifiers = new bytes32[](len);
        for (uint256 i = 0; i < identifiers.length; i++) {
            bytes32identifiers[i] = bytes32(identifiers[i]);
        }
        receiveRewards(bytes32identifiers);
    }

    function reportTermination(uint256 i) public {
        reportTermination(bytes32(i));
    }

    function reportTerminations(uint256[] memory identifiers) public {
        uint256 len = identifiers.length;
        bytes32[] memory bytes32identifiers = new bytes32[](len);
        for (uint256 i = 0; i < identifiers.length; i++) {
            bytes32identifiers[i] = bytes32(identifiers[i]);
        }
        reportTerminations(bytes32identifiers);
    }

    function eligibleForReward(uint256 i) public view returns (bool) {
        return eligibleForReward(bytes32(i));
    }

    function eligibleButTerminatedWithUint(uint256 i)
        public
        view
        returns (bool)
    {
        return eligibleButTerminated(bytes32(i));
    }

    function findEndpoint(uint256 intervalEndpoint)
        public
        view
        returns (uint256)
    {
        return _findEndpoint(intervalEndpoint);
    }

    function getEndpoint(uint256 interval) public returns (uint256) {
        return _getEndpoint(interval);
    }

    function baseAllocation(uint256 interval) public view returns (uint256) {
        return _baseAllocation(interval);
    }

    function adjustedAllocation(uint256 interval) public returns (uint256) {
        return _adjustedAllocation(interval);
    }

    function rewardPerKeep(uint256 interval) public returns (uint256) {
        uint256 __adjustedAllocation = _adjustedAllocation(interval);
        if (__adjustedAllocation == 0) {
            return 0;
        }
        uint256 keepCount = keepsInInterval(interval);
        // Adjusted allocation would be zero if keep count was zero
        assert(keepCount > 0);
        return __adjustedAllocation / keepCount;
    }

    function terminate(uint256 i) public {
        terminated[i] = true;
    }

    function setCloseTime(uint256 i) public {
        closedTime = i;
    }

    function _getKeepCount() internal view override returns (uint256) {
        return creationTimes.length;
    }

    function _getKeepAtIndex(uint256 i) internal view override returns (bytes32) {
        return bytes32(i);
    }

    function _getCreationTime(bytes32 groupIndexBytes)
        internal
        view
        override
        returns (uint256)
    {
        return creationTimes[uint256(groupIndexBytes)];
    }

    function _isClosed(bytes32 groupIndexBytes) internal view override returns (bool) {
        return _getCreationTime(groupIndexBytes) <= closedTime;
    }

    function _isTerminated(bytes32 groupIndexBytes)
        internal
        view
        override
        returns (bool)
    {
        return terminated[uint256(groupIndexBytes)];
    }

    function _recognizedByFactory(bytes32 groupIndexBytes)
        internal
        view
        override
        returns (bool)
    {
        return _getKeepCount() > uint256(groupIndexBytes);
    }

    function _distributeReward(bytes32 groupIndexBytes, uint256 _value)
        internal
        override
    {
        token.safeTransfer(msg.sender, _value);
    }
}
