// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;

import "@openzeppelin/contracts-v4/proxy/transparent/TransparentUpgradeableProxy.sol";
import "@openzeppelin/contracts-v4/proxy/transparent/ProxyAdmin.sol";

contract ProxyProbe {
    uint256 public value;
    address public owner;

    function initialize(uint256 initialValue) external {
        require(owner == address(0), "already initialized");
        owner = msg.sender;
        value = initialValue;
    }
}

contract ProxyProbeV2 is ProxyProbe {
    function version() external pure returns (uint256) {
        return 2;
    }
}

// Deliberately incompatible storage layout for a rejection check.
contract ProxyProbeBadLayout {
    address public value;
    address public owner;

    function initialize(address initialValue) external {
        require(owner == address(0), "already initialized");
        owner = msg.sender;
        value = initialValue;
    }
}
