pragma solidity ^0.8.0;

import "../libraries/staking/LockUtils.sol";

contract LockStub {
    using LockUtils for LockUtils.LockSet;

    LockUtils.LockSet _locks;

    function publicContains(address creator) public view returns (bool) {
        return _locks.contains(creator);
    }

    function publicGetLockTime(address creator) public view returns (uint256) {
        return uint256(_locks.getLockTime(creator));
    }

    function publicSetLock(address creator, uint256 expiresAt) public {
        _locks.setLock(creator, uint96(expiresAt));
    }

    function publicReleaseLock(address creator) public {
        _locks.releaseLock(creator);
    }

    function publicEnumerateCreators() public view returns (address[] memory) {
        address[] memory creators = new address[](_locks.locks.length);
        for (uint256 i = 0; i < _locks.locks.length; i++) {
            creators[i] = _locks.locks[i].creator;
        }
        return creators;
    }
}
