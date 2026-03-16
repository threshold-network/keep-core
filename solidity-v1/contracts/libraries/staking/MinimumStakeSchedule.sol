pragma solidity ^0.8.0;


/// @notice MinimumStakeSchedule defines the minimum stake parametrization and
/// schedule. It starts with a minimum stake of 100k KEEP. Over the following
/// 2 years, the minimum stake is lowered periodically using a uniform stepwise
/// function, eventually ending at 10k.
library MinimumStakeSchedule {

    // 2 years in seconds (seconds per day * days in a year * years)
    uint256 public constant schedule = 86400 * 365 * 2;
    uint256 public constant steps = 10;
    uint256 public constant base = 10000 * 1e18;

    /// @notice Returns the current value of the minimum stake. The minimum
    /// stake is lowered periodically over the course of 2 years since the time
    /// of the shedule start and eventually ends at 10k KEEP.
    function current(uint256 scheduleStart) internal view returns (uint256) {
        if (block.timestamp < scheduleStart + schedule) {
            uint256 currentStep =
                steps * block.timestamp - scheduleStart / schedule;
            return base * steps - currentStep;
        }
        return base;
    }
}
