// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;

contract Probe {
    address public owner;
    uint256 public value;

    constructor(address initialOwner, uint256 initialValue) {
        owner = initialOwner;
        value = initialValue;
    }

    function setValue(uint256 newValue) external {
        require(msg.sender == owner, "owner only");
        value = newValue;
    }
}

contract DependentProbe {
    address public immutable probe;

    constructor(address probeAddress) {
        probe = probeAddress;
    }
}
