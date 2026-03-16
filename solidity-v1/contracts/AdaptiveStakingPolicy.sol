pragma solidity ^0.8.0;

import "./libraries/grant/UnlockingSchedule.sol";
import "./GrantStakingPolicy.sol";
import "./TokenStaking.sol";

/// @title AdaptiveStakingPolicy
/// @notice A staking policy which allows the grantee
/// to always stake a certain multiple of the defined minimum stake,
/// or the unlocked amount at a specified time in the future,
/// if it is greater.
///
/// When creating a policy,
/// the minimum stake multiplier and stakeahead time can be customized,
/// and whether the cliff is considered in the stakeahead
/// can also be chosen.
contract AdaptiveStakingPolicy is GrantStakingPolicy {
    using UnlockingSchedule for uint256;
    uint256 minimumStake;
    uint256 stakeaheadTime;
    bool useCliff;

    constructor(
        // Address of the staking contract,
        // from which the minimum stake is fetched at the time of creation.
        address _stakingContract,
        // Multiplier for the minimum stake;
        // with a minimumMultiplier = 5
        // the policy permits staking 5 times the minimum stake.
        // If the multiplier is 0,
        // only the unlocked amount,
        // including stakeahead if applicable,
        // can be staked.
        uint256 minimumMultiplier,
        // Stakeahead time in seconds;
        // the policy permits staking the amount that will be unlocked
        // `stakeaheadTime` seconds in the future.
        // For example, on a 12-month grant
        // a stakeahead time of 7,884,000 (3 months in seconds)
        // means that 25% of the grant will be unlocked within the stakeahead
        // and thus be stakeable on top of the unlocked amount.
        // On a 24-month grant the same stakeahead results in
        // 12.5% of the grant being added to the unlocked amount.
        // With a stakeahead of 0,
        // only the unlocked amount can be staked.
        uint256 _stakeaheadTime,
        // Whether the cliff is used when calculating stakeahead.
        // If `useCliff = true`,
        // a 12-month grant with a 6-month cliff and 3-month stakeahead
        // will only permit the minimum until 3 months,
        // when the current time plus stakeahead reaches the cliff.
        // If the cliff is not used,
        // the grantee could instead stake 25% right away.
        bool _useCliff
    ) public {
        minimumStake = TokenStaking(_stakingContract).minimumStake().mul(
            minimumMultiplier
        );
        stakeaheadTime = _stakeaheadTime;
        useCliff = _useCliff;
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
            _now + stakeaheadTime.getUnlockedAmount(
                grantedAmount,
                duration,
                start,
                (useCliff ? cliff : 0)
            );
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
