pragma solidity ^0.8.0;

import "../libraries/staking/MinimumStakeSchedule.sol";

contract MinimumStakeScheduleStub {
    uint256 scheduleStart = now;

    function current() public view returns (uint256) {
        return MinimumStakeSchedule.current(scheduleStart);
    }
}
