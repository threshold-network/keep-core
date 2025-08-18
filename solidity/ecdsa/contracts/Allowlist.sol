// SPDX-License-Identifier: GPL-3.0-only
//
// ▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
//   ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
// ▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
//
//                           Trust math, not hardware.

pragma solidity 0.8.17;

import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/access/Ownable2StepUpgradeable.sol";
import "./WalletRegistry.sol";

/// @title Allowlist
/// @notice The allowlist contract replaces the Threshold TokenStaking contract
///         as an outcome of TIP-092 and TIP-100 governance decisions.
///         Staking tokens is no longer required to operate nodes. Beta stakers
///         are selected by the DAO and operate the network based on the
///         allowlist maintained by the DAO.
/// @dev The allowlist contract maintains the maximum possible compatibility
///      with the old TokenStaking contract interface, as utilized by the
///      WalletRegistry contract.
contract Allowlist is Ownable2StepUpgradeable {
    struct StakingProviderInfo {
        uint96 weight;
        uint96 pendingNewWeight;
    }

    /// @notice Mapping between the staking provider address and a struct
    ///         maintaining weight settings for that staking provider.
    mapping(address => StakingProviderInfo) public stakingProviders;

    WalletRegistry public walletRegistry;

    event StakingProviderAdded(address indexed stakingProvider, uint96 weight);
    event WeightDecreaseRequested(
        address indexed stakingProvider,
        uint96 oldWeight,
        uint96 newWeight
    );
    event WeightDecreaseFinalized(
        address indexed stakingProvider,
        uint96 oldWeight,
        uint96 newWeight
    );
    event MaliciousBehaviorIdentified(
        address notifier,
        address[] stakingProviders
    );

    error StakingProviderAlreadyAdded();
    error StakingProviderUnknown();
    error RequestedWeightNotBelowCurrentWeight();
    error NotWalletRegistry();

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(address _walletRegistry) external initializer {
        __Ownable2Step_init();

        walletRegistry = WalletRegistry(_walletRegistry);
    }

    /// @notice Allows the governance to add a new staking provider with the
    ///         provided weight. If the staking provider address already has
    ///         a non-zero weight, the function reverts.
    /// @param stakingProvider The staking provider's address
    /// @param weight The weight of the new staking provider
    function addStakingProvider(address stakingProvider, uint96 weight)
        external
        onlyOwner
    {
        StakingProviderInfo storage info = stakingProviders[stakingProvider];

        if (info.weight != 0) {
            revert StakingProviderAlreadyAdded();
        }

        emit StakingProviderAdded(stakingProvider, weight);

        info.weight = weight;
        walletRegistry.authorizationIncreased(stakingProvider, 0, weight);
    }

    /// @notice Allows the governance to request weight decrease for the given
    ///         staking provider. The change does not take the effect immediately
    ///         as it has to be approved by the WalletRegistry contract based on
    ///         decrease delays required. Overwrites pending weight decrease
    ///         request for the given staking provider. Reverts if the staking
    ///         provider is now known or if the proposed new weight is higher
    ///         or equal the current weight.
    ///
    ///         BE EXTREMELY CAREFUL MAKING CHANGES TO THE BETA STAKER SET!
    ///         ENSURE WALLET LIVENESS IS NOT AT RISK AND FAILED HEARTBEATS
    ///         ARE NOT GOING TO TRIGGER CASCADING MOVING FUNDS OPERATIONS!
    ///
    /// @param stakingProvider The staking provider's address
    /// @param newWeight The new requested weight of this staking provider
    function requestWeightDecrease(address stakingProvider, uint96 newWeight)
        external
        onlyOwner
    {
        StakingProviderInfo storage info = stakingProviders[stakingProvider];
        uint96 currentWeight = info.weight;

        if (currentWeight == 0) {
            revert StakingProviderUnknown();
        }

        if (newWeight >= currentWeight) {
            revert RequestedWeightNotBelowCurrentWeight();
        }

        emit WeightDecreaseRequested(stakingProvider, currentWeight, newWeight);

        info.pendingNewWeight = newWeight;
        walletRegistry.authorizationDecreaseRequested(
            stakingProvider,
            currentWeight,
            newWeight
        );
    }

    /// @notice Called by WalletRegistry contract to approve the previously
    ///         requested weight decrease for the given staking provider.
    /// @param stakingProvider The staking provider's address
    /// @return The new weight of the staking provider
    function approveAuthorizationDecrease(address stakingProvider)
        external
        returns (uint96)
    {
        if (msg.sender != address(walletRegistry)) {
            revert NotWalletRegistry();
        }

        StakingProviderInfo storage info = stakingProviders[stakingProvider];
        uint96 currentWeight = info.weight;
        uint96 newWeight = info.pendingNewWeight;

        if (currentWeight == 0) {
            revert StakingProviderUnknown();
        }

        emit WeightDecreaseFinalized(stakingProvider, currentWeight, newWeight);

        info.weight = newWeight;
        info.pendingNewWeight = 0;
        return newWeight;
    }

    /// @notice Returns the current weight of the staking provider.
    /// @dev The function signature maintains compatibility with Threshold
    ///      TokenStaking contract to minimize the TIP-092 impact on the
    ///      WalletRegistry contract.
    function authorizedStake(address stakingProvider, address)
        external
        view
        returns (uint96)
    {
        return stakingProviders[stakingProvider].weight;
    }

    /// @notice No-op stake seize operation. After TIP-092 tokens are not staked
    ///         so there is nothing to seize from.
    /// @dev The function signature maintains compatibility with Threshold
    ///      TokenStaking contract to minimize the TIP-092 impact on the
    ///      WalletRegistry contract.
    function seize(
        uint96,
        uint256,
        address notifier,
        address[] memory _stakingProviders
    ) external {
        emit MaliciousBehaviorIdentified(notifier, _stakingProviders);
    }

    /// @notice Returns the stake owner, beneficiary, and authorizer roles for
    ///         the given staking provider. After TIP-092 those roles are no
    ///         longer relevant as no tokens are staked. The owner is set to the
    ///         allowlist owner, the beneficiary is the staking provider itself
    ///         and the authorizer is the zero address.
    /// @dev The function signature maintains compatibility with Threshold
    ///      TokenStaking contract to minimize the TIP-092 impact on the
    ///      WalletRegistry contract.
    function rolesOf(address stakingProvider)
        external
        view
        returns (
            address stakeOwner,
            address payable beneficiary,
            address authorizer
        )
    {
        return (owner(), payable(stakingProvider), address(0));
    }
}
