// SPDX-License-Identifier: GPL-3.0-only

pragma solidity 0.8.17;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "../legacy/sortition/SortitionPool.sol";
import "../legacy/random-beacon/ReimbursementPool.sol";
import "../WalletRegistry.sol";
import "../EcdsaDkgValidator.sol";
import "../libraries/Wallets.sol";

contract WalletRegistryStub is WalletRegistry {
    using Wallets for Wallets.Data;

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor(SortitionPool _sortitionPool, IStaking _staking)
        WalletRegistry(_sortitionPool, _staking)
    {}

    function forceAddWallet(bytes calldata groupPubKey, bytes32 membersIdsHash)
        external
    {
        wallets.addWallet(membersIdsHash, groupPubKey);
    }

    function getDkgData() external view returns (EcdsaDkg.Data memory) {
        return dkg;
    }
}
