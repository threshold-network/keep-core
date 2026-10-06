// SPDX-License-Identifier: GPL-3.0-only

pragma solidity ^0.8.6;

import "../libraries/Callback.sol";

// Stub contract used in tests
contract CallbackContractStub is IRandomBeaconConsumer {
    uint256 public lastEntry;
    uint256 public blockNumber;
    bool public shouldFail;

    function __beaconCallback(
        uint256 _lastEntry,
        uint256 _blockNumber
    ) external override {
        if (shouldFail) {
            revert("error");
        }

        lastEntry = _lastEntry;
        blockNumber = _blockNumber;
    }

    function setFailureFlag(bool _shouldFail) external {
        shouldFail = _shouldFail;
    }
}
