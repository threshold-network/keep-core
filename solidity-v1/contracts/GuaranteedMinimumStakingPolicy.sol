pragma solidity ^0.8.0;

import "./libraries/grant/UnlockingSchedule.sol";
import "./GrantStakingPolicy.sol";
import "./TokenStaking.sol";

/// @title GuaranteedMinimumStakingPolicy
/// @notice A staking policy which allows the grantee
/// to always stake the defined minimum stake,
/// or the unlocked amount if greater.
///
/// This is necessary for staking revocable token grants safely.
/// If the entire revocable grant can be staked,
/// the yet-to-be-unlocked amount becomes ineffective as collateral
/// if the grant is revoked.
/// To avoid this issue,
/// only the unlocked amount can be staked.
///
/// However, grants that feature a cliff pose a problem
/// as no tokens are unlocked until the cliff is reached.
/// Small grants may also take a long time
/// to unlock enough tokens to be able to stake.
/// To permit all grants to stake from the beginning,
/// the policy defines a minimum which can always be staked
/// even if the grant doesn't have enough unlocked tokens.
contract GuaranteedMinimumStakingPolicy is GrantStakingPolicy {
    using UnlockingSchedule for uint256;
    uint256 minimumStake;

    constructor(address _stakingContract) {
        minimumStake = TokenStaking(_stakingContract).minimumStake();
    }

    function getStakeableAmount(
        uint256 _now,
        uint256 grantedAmount,
        uint256 duration,
        uint256 start,
        uint256 cliff,
        uint256 withdrawn
    ) public view override returns (uint256) {
        uint256 unlocked =
            _now.getUnlockedAmount(grantedAmount, duration, start, cliff);
        uint256 remainingInGrant = grantedAmount - withdrawn;
        uint256 unlockedInGrant = unlocked - withdrawn;

        // Less than minimum stake remaining
        //   -> may stake what is remaining in grant
        if (remainingInGrant < minimumStake) {
            return remainingInGrant;
        }
        // At least minimum stake remaining in grant,
        // but unlocked amount is less than the minimum stake
        //   -> may stake the minimum stake
        if (unlockedInGrant < minimumStake) {
            return minimumStake;
        }
        // More than minimum stake unlocked in grant
        //   -> may stake the unlocked amount
        return unlockedInGrant;
    }
}
