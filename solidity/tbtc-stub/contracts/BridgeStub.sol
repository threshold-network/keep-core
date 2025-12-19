// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

/**
 * @title BridgeStub
 * @notice Minimal stub implementation of Bridge contract for local development
 */
contract BridgeStub {
    address public bank;
    address public relay;
    address public ecdsaWalletRegistry;
    address public reimbursementPool;

    constructor(
        address _bank,
        address _relay,
        address _ecdsaWalletRegistry,
        address _reimbursementPool
    ) {
        bank = _bank;
        relay = _relay;
        ecdsaWalletRegistry = _ecdsaWalletRegistry;
        reimbursementPool = _reimbursementPool;
    }

    function contractReferences()
        external
        view
        returns (
            address _bank,
            address _relay,
            address _ecdsaWalletRegistry,
            address _reimbursementPool
        )
    {
        return (bank, relay, ecdsaWalletRegistry, reimbursementPool);
    }

    function getRedemptionWatchtower()
        external
        pure
        returns (address)
    {
        return address(0);
    }
}
