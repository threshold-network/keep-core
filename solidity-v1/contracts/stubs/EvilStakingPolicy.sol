pragma solidity ^0.8.0;

import "../GrantStakingPolicy.sol";

/// @title EvilStakingPolicy
/// @dev A staking policy which allows the grantee to stake
/// a million times more than the grant amount.
contract EvilStakingPolicy is GrantStakingPolicy {

    function getStakeableAmount(
        uint256 _now,
        uint256 grantedAmount,
        uint256 duration,
        uint256 start,
        uint256 cliff,
        uint256 withdrawn
    ) public view override returns (uint256) {
        return grantedAmount * 1000000;
    }
}
