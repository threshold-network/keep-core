pragma solidity ^0.8.0;


library UnlockingSchedule {

    function getUnlockedAmount(
        uint256 _now,
        uint256 grantedAmount,
        uint256 duration,
        uint256 start,
        uint256 cliff
    ) internal pure returns (uint256) {
        bool cliffNotReached = _now < cliff;
        if (cliffNotReached) {
            return 0;
        }

        uint256 timeElapsed = _now - start;

        bool unlockingPeriodFinished = timeElapsed >= duration;
        if (unlockingPeriodFinished) {
            return grantedAmount;
        }

        return grantedAmount * timeElapsed / duration;
    }
}
