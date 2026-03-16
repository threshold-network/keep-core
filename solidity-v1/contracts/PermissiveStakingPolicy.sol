pragma solidity ^0.8.0;

import "./GrantStakingPolicy.sol";

/// @title PermissiveStakingPolicy
/// @notice A staking policy which allows the grantee to stake the entire grant,
/// regardless of its unlocking status.
contract PermissiveStakingPolicy is GrantStakingPolicy {

    function getStakeableAmount(
        uint256 _now,
        uint256 grantedAmount,
        uint256 duration,
        uint256 start,
        uint256 cliff,
        uint256 withdrawn
    ) public view override returns (uint256) {
        // Can always stake the entire remaining amount.
        return grantedAmount - withdrawn;
    }
}
