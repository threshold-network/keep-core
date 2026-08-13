// SPDX-License-Identifier: GPL-3.0-only

pragma solidity ^0.8.0;

import "./IMockTarget.sol";

/// @notice Test-only contract under test. It reaches the mocked interface the
///         way production code does — in particular, `view` functions are
///         reached by STATICCALL, which is the case a recording mock has to
///         survive without reverting.
contract MockTargetConsumer {
    IMockTarget public immutable target;

    uint256 public lastValue;
    bool public lastResult;

    constructor(IMockTarget _target) {
        target = _target;
    }

    /// @dev STATICCALL inside a state-changing function.
    function cacheValue(uint256 key) external {
        lastValue = target.readValue(key);
    }

    /// @dev CALL: recorded by the mock and asserted on by the test.
    function doThing(address who, uint256 amount) external {
        lastResult = target.doThing(who, amount);
    }

    function noReturn(uint256 value) external {
        target.noReturn(value);
    }

    /// @dev STATICCALL: `readValue` is `view` on the interface.
    function readValueThroughStaticCall(uint256 key)
        external
        view
        returns (uint256)
    {
        return target.readValue(key);
    }

    function readInfoThroughStaticCall(uint256 key)
        external
        view
        returns (IMockTarget.Info memory)
    {
        return target.readInfo(key);
    }

    function readPairThroughStaticCall(uint256 key)
        external
        view
        returns (uint32, uint32)
    {
        return target.readPair(key);
    }
}
