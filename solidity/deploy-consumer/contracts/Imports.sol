// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;

// Proxy deployment needs consumer-side compiler output for OpenZeppelin's
// validations. All imported sources come from the packed packages.
import "@keep-network/ecdsa/contracts/WalletRegistry.sol";
import "@keep-network/ecdsa/contracts/Allowlist.sol";
