// SPDX-License-Identifier: GPL-3.0-only

pragma solidity ^0.8.0;

/// @notice Test-only interface exercising the shapes `MockContract` has to
///         answer: a view function reached by STATICCALL, a state-changing
///         function whose calls are asserted on, a function returning nothing,
///         a struct return and a multi-value return.
interface IMockTarget {
    struct Info {
        address owner;
        uint64 createdAt;
        bool active;
    }

    function doThing(address who, uint256 amount) external returns (bool);

    function noReturn(uint256 value) external;

    function readValue(uint256 key) external view returns (uint256);

    function readInfo(uint256 key) external view returns (Info memory);

    function readPair(uint256 key) external view returns (uint32, uint32);
}
