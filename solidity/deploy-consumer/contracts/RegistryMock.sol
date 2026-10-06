// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;

// Stands in for a WalletRegistry whose governance points at a registry other
// than itself. edge.js uses it to reach the deploy scripts' consistency checks.
contract RegistryMock {
    address public governance;

    constructor(address _governance) {
        governance = _governance;
    }
}
