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
//

pragma solidity 0.8.17;

import "./WalletRegistry.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@keep-network/random-beacon/contracts/ReimbursementPool.sol";

import {IWalletOwner} from "./api/IWalletOwner.sol";
import {IRandomBeacon} from "@keep-network/random-beacon/contracts/api/IRandomBeacon.sol";

/// @title Wallet Registry Governance
/// @notice Owns the `WalletRegistry` contract and is responsible for updating
///         its governable parameters in respect to the governance delay.
contract WalletRegistryGovernance is Ownable {
    uint256 public newGovernanceDelay;
    uint256 public governanceDelayChangeInitiated;

    address public newWalletRegistryGovernance;
    uint256 public walletRegistryGovernanceTransferInitiated;

    address public newWalletOwner;
    uint256 public walletOwnerChangeInitiated;

    uint96 public newMinimumAuthorization;
    uint256 public minimumAuthorizationChangeInitiated;

    uint64 public newAuthorizationDecreaseDelay;
    uint256 public authorizationDecreaseDelayChangeInitiated;

    uint64 public newAuthorizationDecreaseChangePeriod;
    uint256 public authorizationDecreaseChangePeriodChangeInitiated;

    uint96 public newMaliciousDkgResultSlashingAmount;
    uint256 public maliciousDkgResultSlashingAmountChangeInitiated;

    uint256 public newMaliciousDkgResultNotificationRewardMultiplier;
    uint256
        public maliciousDkgResultNotificationRewardMultiplierChangeInitiated;

    uint256 public newSortitionPoolRewardsBanDuration;
    uint256 public sortitionPoolRewardsBanDurationChangeInitiated;

    uint256 public newDkgSeedTimeout;
    uint256 public dkgSeedTimeoutChangeInitiated;

    uint256 public newDkgResultChallengePeriodLength;
    uint256 public dkgResultChallengePeriodLengthChangeInitiated;

    uint256 public newDkgResultChallengeExtraGas;
    uint256 public dkgResultChallengeExtraGasChangeInitiated;

    uint256 public newDkgResultSubmissionTimeout;
    uint256 public dkgResultSubmissionTimeoutChangeInitiated;

    uint256 public newSubmitterPrecedencePeriodLength;
    uint256 public dkgSubmitterPrecedencePeriodLengthChangeInitiated;

    uint256 public newDkgResultSubmissionGas;
    uint256 public dkgResultSubmissionGasChangeInitiated;

    uint256 public newDkgResultApprovalGasOffset;
    uint256 public dkgResultApprovalGasOffsetChangeInitiated;

    uint256 public newNotifyOperatorInactivityGasOffset;
    uint256 public notifyOperatorInactivityGasOffsetChangeInitiated;

    uint256 public newNotifySeedTimeoutGasOffset;
    uint256 public notifySeedTimeoutGasOffsetChangeInitiated;

    uint256 public newNotifyDkgTimeoutNegativeGasOffset;
    uint256 public notifyDkgTimeoutNegativeGasOffsetChangeInitiated;

    address payable public newReimbursementPool;
    uint256 public reimbursementPoolChangeInitiated;

    WalletRegistry public immutable walletRegistry;

    uint256 public governanceDelay;

    event GovernanceDelayUpdateStarted(
        uint256 governanceDelay,
        uint256 timestamp
    );
    event GovernanceDelayUpdated(uint256 governanceDelay);

    event WalletRegistryGovernanceTransferStarted(
        address newWalletRegistryGovernance,
        uint256 timestamp
    );
    event WalletRegistryGovernanceTransferred(
        address newWalletRegistryGovernance
    );

    event WalletOwnerUpdateStarted(address walletOwner, uint256 timestamp);
    event WalletOwnerUpdated(address walletOwner);

    event MinimumAuthorizationUpdateStarted(
        uint96 minimumAuthorization,
        uint256 timestamp
    );
    event MinimumAuthorizationUpdated(uint96 minimumAuthorization);

    event AuthorizationDecreaseDelayUpdateStarted(
        uint64 authorizationDecreaseDelay,
        uint256 timestamp
    );
    event AuthorizationDecreaseDelayUpdated(uint64 authorizationDecreaseDelay);

    event AuthorizationDecreaseChangePeriodUpdateStarted(
        uint64 authorizationDecreaseChangePeriod,
        uint256 timestamp
    );

    event AuthorizationDecreaseChangePeriodUpdated(
        uint64 authorizationDecreaseChangePeriod
    );

    event MaliciousDkgResultSlashingAmountUpdateStarted(
        uint256 maliciousDkgResultSlashingAmount,
        uint256 timestamp
    );
    event MaliciousDkgResultSlashingAmountUpdated(
        uint256 maliciousDkgResultSlashingAmount
    );

    event MaliciousDkgResultNotificationRewardMultiplierUpdateStarted(
        uint256 maliciousDkgResultNotificationRewardMultiplier,
        uint256 timestamp
    );
    event MaliciousDkgResultNotificationRewardMultiplierUpdated(
        uint256 maliciousDkgResultNotificationRewardMultiplier
    );

    event SortitionPoolRewardsBanDurationUpdateStarted(
        uint256 sortitionPoolRewardsBanDuration,
        uint256 timestamp
    );
    event SortitionPoolRewardsBanDurationUpdated(
        uint256 sortitionPoolRewardsBanDuration
    );

    event DkgSeedTimeoutUpdateStarted(
        uint256 dkgSeedTimeout,
        uint256 timestamp
    );
    event DkgSeedTimeoutUpdated(uint256 dkgSeedTimeout);

    event DkgResultChallengePeriodLengthUpdateStarted(
        uint256 dkgResultChallengePeriodLength,
        uint256 timestamp
    );
    event DkgResultChallengePeriodLengthUpdated(
        uint256 dkgResultChallengePeriodLength
    );

    event DkgResultChallengeExtraGasUpdateStarted(
        uint256 dkgResultChallengeExtraGas,
        uint256 timestamp
    );
    event DkgResultChallengeExtraGasUpdated(uint256 dkgResultChallengeExtraGas);

    event DkgResultSubmissionTimeoutUpdateStarted(
        uint256 dkgResultSubmissionTimeout,
        uint256 timestamp
    );
    event DkgResultSubmissionTimeoutUpdated(uint256 dkgResultSubmissionTimeout);

    event DkgSubmitterPrecedencePeriodLengthUpdateStarted(
        uint256 submitterPrecedencePeriodLength,
        uint256 timestamp
    );
    event DkgSubmitterPrecedencePeriodLengthUpdated(
        uint256 submitterPrecedencePeriodLength
    );

    event DkgResultSubmissionGasUpdateStarted(
        uint256 dkgResultSubmissionGas,
        uint256 timestamp
    );
    event DkgResultSubmissionGasUpdated(uint256 dkgResultSubmissionGas);

    event DkgResultApprovalGasOffsetUpdateStarted(
        uint256 dkgResultApprovalGasOffset,
        uint256 timestamp
    );
    event DkgResultApprovalGasOffsetUpdated(uint256 dkgResultApprovalGasOffset);

    event NotifyOperatorInactivityGasOffsetUpdateStarted(
        uint256 notifyOperatorInactivityGasOffset,
        uint256 timestamp
    );
    event NotifyOperatorInactivityGasOffsetUpdated(
        uint256 notifyOperatorInactivityGasOffset
    );

    event NotifySeedTimeoutGasOffsetUpdateStarted(
        uint256 notifySeedTimeoutGasOffset,
        uint256 timestamp
    );
    event NotifySeedTimeoutGasOffsetUpdated(uint256 notifySeedTimeoutGasOffset);

    event NotifyDkgTimeoutNegativeGasOffsetUpdateStarted(
        uint256 notifyDkgTimeoutNegativeGasOffset,
        uint256 timestamp
    );
    event NotifyDkgTimeoutNegativeGasOffsetUpdated(
        uint256 notifyDkgTimeoutNegativeGasOffset
    );

    event ReimbursementPoolUpdateStarted(
        address reimbursementPool,
        uint256 timestamp
    );
    event ReimbursementPoolUpdated(address reimbursementPool);

    /// @notice Reverts if called before the governance delay elapses.
    /// @param changeInitiatedTimestamp Timestamp indicating the beginning
    ///        of the change.
    modifier onlyAfterGovernanceDelay(uint256 changeInitiatedTimestamp) {
        /* solhint-disable not-rely-on-time */
        require(changeInitiatedTimestamp > 0, "Change not initiated");
        require(
            block.timestamp - changeInitiatedTimestamp >= governanceDelay,
            "Governance delay has not elapsed"
        );
        _;
        /* solhint-enable not-rely-on-time */
    }

    constructor(WalletRegistry _walletRegistry, uint256 _governanceDelay) {
        walletRegistry = _walletRegistry;
        governanceDelay = _governanceDelay;
    }

    /// @notice Upgrades the random beacon.
    /// @dev Can be called only by the contract owner.
    /// @param _newRandomBeacon New random beacon address
    function upgradeRandomBeacon(address _newRandomBeacon) external onlyOwner {
        require(
            _newRandomBeacon != address(0),
            "New random beacon address cannot be zero"
        );

        walletRegistry.upgradeRandomBeacon(IRandomBeacon(_newRandomBeacon));
    }

    /// @notice Initializes the Wallet Owner's address.
    /// @dev Can be called only by the contract owner. It can be called only if
    ///      walletOwner has not been set before. It doesn't enforce a governance
    ///      delay for the initial update. Any subsequent updates should be performed
    ///      with beginWalletOwnerUpdate/finalizeWalletOwnerUpdate with respect
    ///      of a governance delay.
    /// @param _walletOwner The Wallet Owner's address
    function initializeWalletOwner(address _walletOwner) external onlyOwner {
        require(
            address(walletRegistry.walletOwner()) == address(0),
            "Wallet Owner already initialized"
        );
        require(
            _walletOwner != address(0),
            "Wallet Owner address cannot be zero"
        );

        walletRegistry.updateWalletOwner(IWalletOwner(_walletOwner));
    }

    /// @notice Begins the governance delay update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newGovernanceDelay New governance delay
    function beginGovernanceDelayUpdate(uint256 _newGovernanceDelay)
        external
        onlyOwner
    {
        newGovernanceDelay = _newGovernanceDelay;
        /* solhint-disable not-rely-on-time */
        governanceDelayChangeInitiated = block.timestamp;
        emit GovernanceDelayUpdateStarted(_newGovernanceDelay, block.timestamp);
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the governance delay update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeGovernanceDelayUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(governanceDelayChangeInitiated)
    {
        emit GovernanceDelayUpdated(newGovernanceDelay);
        governanceDelay = newGovernanceDelay;
        governanceDelayChangeInitiated = 0;
        newGovernanceDelay = 0;
    }

    /// @notice Begins the wallet registry governance transfer process.
    /// @dev Can be called only by the contract owner.
    function beginWalletRegistryGovernanceTransfer(
        address _newWalletRegistryGovernance
    ) external onlyOwner {
        require(
            address(_newWalletRegistryGovernance) != address(0),
            "New wallet registry governance address cannot be zero"
        );
        newWalletRegistryGovernance = _newWalletRegistryGovernance;
        /* solhint-disable not-rely-on-time */
        walletRegistryGovernanceTransferInitiated = block.timestamp;
        emit WalletRegistryGovernanceTransferStarted(
            _newWalletRegistryGovernance,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the wallet registry governance transfer process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeWalletRegistryGovernanceTransfer()
        external
        onlyOwner
        onlyAfterGovernanceDelay(walletRegistryGovernanceTransferInitiated)
    {
        emit WalletRegistryGovernanceTransferred(newWalletRegistryGovernance);
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.transferGovernance(newWalletRegistryGovernance);
        walletRegistryGovernanceTransferInitiated = 0;
        newWalletRegistryGovernance = address(0);
    }

    /// @notice Begins the wallet owner update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newWalletOwner New wallet owner address
    function beginWalletOwnerUpdate(address _newWalletOwner)
        external
        onlyOwner
    {
        require(
            address(_newWalletOwner) != address(0),
            "New wallet owner address cannot be zero"
        );
        /* solhint-disable not-rely-on-time */
        newWalletOwner = _newWalletOwner;
        walletOwnerChangeInitiated = block.timestamp;
        emit WalletOwnerUpdateStarted(_newWalletOwner, block.timestamp);
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the wallet owner update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeWalletOwnerUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(walletOwnerChangeInitiated)
    {
        emit WalletOwnerUpdated(newWalletOwner);
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateWalletOwner(IWalletOwner(newWalletOwner));
        walletOwnerChangeInitiated = 0;
        newWalletOwner = address(0);
    }

    /// @notice Begins the minimum authorization amount update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newMinimumAuthorization New minimum authorization amount.
    function beginMinimumAuthorizationUpdate(uint96 _newMinimumAuthorization)
        external
        onlyOwner
    {
        /* solhint-disable not-rely-on-time */
        newMinimumAuthorization = _newMinimumAuthorization;
        minimumAuthorizationChangeInitiated = block.timestamp;
        emit MinimumAuthorizationUpdateStarted(
            _newMinimumAuthorization,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the minimum authorization amount update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeMinimumAuthorizationUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(minimumAuthorizationChangeInitiated)
    {
        emit MinimumAuthorizationUpdated(newMinimumAuthorization);
        (
            ,
            uint64 authorizationDecreaseDelay,
            uint64 authorizationDecreaseChangePeriod
        ) = walletRegistry.authorizationParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateAuthorizationParameters(
            newMinimumAuthorization,
            authorizationDecreaseDelay,
            authorizationDecreaseChangePeriod
        );
        minimumAuthorizationChangeInitiated = 0;
        newMinimumAuthorization = 0;
    }

    /// @notice Begins the authorization decrease delay update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newAuthorizationDecreaseDelay New authorization decrease delay
    function beginAuthorizationDecreaseDelayUpdate(
        uint64 _newAuthorizationDecreaseDelay
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newAuthorizationDecreaseDelay = _newAuthorizationDecreaseDelay;
        authorizationDecreaseDelayChangeInitiated = block.timestamp;
        emit AuthorizationDecreaseDelayUpdateStarted(
            _newAuthorizationDecreaseDelay,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the authorization decrease delay update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeAuthorizationDecreaseDelayUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(authorizationDecreaseDelayChangeInitiated)
    {
        emit AuthorizationDecreaseDelayUpdated(newAuthorizationDecreaseDelay);
        (
            uint96 minimumAuthorization,
            ,
            uint64 authorizationDecreaseChangePeriod
        ) = walletRegistry.authorizationParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateAuthorizationParameters(
            minimumAuthorization,
            newAuthorizationDecreaseDelay,
            authorizationDecreaseChangePeriod
        );
        authorizationDecreaseDelayChangeInitiated = 0;
        newAuthorizationDecreaseDelay = 0;
    }

    /// @notice Begins the authorization decrease change period update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newAuthorizationDecreaseChangePeriod New authorization decrease change period
    function beginAuthorizationDecreaseChangePeriodUpdate(
        uint64 _newAuthorizationDecreaseChangePeriod
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newAuthorizationDecreaseChangePeriod = _newAuthorizationDecreaseChangePeriod;
        authorizationDecreaseChangePeriodChangeInitiated = block.timestamp;
        emit AuthorizationDecreaseChangePeriodUpdateStarted(
            _newAuthorizationDecreaseChangePeriod,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the authorization decrease change period update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeAuthorizationDecreaseChangePeriodUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            authorizationDecreaseChangePeriodChangeInitiated
        )
    {
        emit AuthorizationDecreaseChangePeriodUpdated(
            newAuthorizationDecreaseChangePeriod
        );
        (
            uint96 minimumAuthorization,
            uint64 authorizationDecreaseDelay,

        ) = walletRegistry.authorizationParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateAuthorizationParameters(
            minimumAuthorization,
            authorizationDecreaseDelay,
            newAuthorizationDecreaseChangePeriod
        );
        authorizationDecreaseChangePeriodChangeInitiated = 0;
        newAuthorizationDecreaseChangePeriod = 0;
    }

    /// @notice Begins the malicious DKG result slashing amount update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newMaliciousDkgResultSlashingAmount New malicious DKG result
    ///        slashing amount
    function beginMaliciousDkgResultSlashingAmountUpdate(
        uint96 _newMaliciousDkgResultSlashingAmount
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newMaliciousDkgResultSlashingAmount = _newMaliciousDkgResultSlashingAmount;
        maliciousDkgResultSlashingAmountChangeInitiated = block.timestamp;
        emit MaliciousDkgResultSlashingAmountUpdateStarted(
            _newMaliciousDkgResultSlashingAmount,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the malicious DKG result slashing amount update
    ///         process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeMaliciousDkgResultSlashingAmountUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            maliciousDkgResultSlashingAmountChangeInitiated
        )
    {
        emit MaliciousDkgResultSlashingAmountUpdated(
            newMaliciousDkgResultSlashingAmount
        );
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateSlashingParameters(
            newMaliciousDkgResultSlashingAmount
        );
        maliciousDkgResultSlashingAmountChangeInitiated = 0;
        newMaliciousDkgResultSlashingAmount = 0;
    }

    /// @notice Begins the DKG malicious result notification reward multiplier
    ///         update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newMaliciousDkgResultNotificationRewardMultiplier New DKG
    ///        malicious result notification reward multiplier.
    function beginMaliciousDkgResultNotificationRewardMultiplierUpdate(
        uint256 _newMaliciousDkgResultNotificationRewardMultiplier
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newMaliciousDkgResultNotificationRewardMultiplier <= 100,
            "Maximum value is 100"
        );

        newMaliciousDkgResultNotificationRewardMultiplier = _newMaliciousDkgResultNotificationRewardMultiplier;
        maliciousDkgResultNotificationRewardMultiplierChangeInitiated = block
            .timestamp;
        emit MaliciousDkgResultNotificationRewardMultiplierUpdateStarted(
            _newMaliciousDkgResultNotificationRewardMultiplier,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG malicious result notification reward
    ///         multiplier update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeMaliciousDkgResultNotificationRewardMultiplierUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            maliciousDkgResultNotificationRewardMultiplierChangeInitiated
        )
    {
        emit MaliciousDkgResultNotificationRewardMultiplierUpdated(
            newMaliciousDkgResultNotificationRewardMultiplier
        );
        (, uint256 sortitionPoolRewardsBanDuration) = walletRegistry
            .rewardParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateRewardParameters(
            newMaliciousDkgResultNotificationRewardMultiplier,
            sortitionPoolRewardsBanDuration
        );
        maliciousDkgResultNotificationRewardMultiplierChangeInitiated = 0;
        newMaliciousDkgResultNotificationRewardMultiplier = 0;
    }

    /// @notice Begins the dkg result submission gas update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultSubmissionGas New DKG result submission gas.
    function beginDkgResultSubmissionGasUpdate(
        uint256 _newDkgResultSubmissionGas
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newDkgResultSubmissionGas = _newDkgResultSubmissionGas;
        dkgResultSubmissionGasChangeInitiated = block.timestamp;
        emit DkgResultSubmissionGasUpdateStarted(
            _newDkgResultSubmissionGas,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the dkg result submission gas update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgResultSubmissionGasUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(dkgResultSubmissionGasChangeInitiated)
    {
        emit DkgResultSubmissionGasUpdated(newDkgResultSubmissionGas);
        (
            ,
            uint256 dkgResultApprovalGasOffset,
            uint256 notifyOperatorInactivityGasOffset,
            uint256 notifySeedTimeoutGasOffset,
            uint256 notifyDkgTimeoutNegativeGasOffset
        ) = walletRegistry.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateGasParameters(
            newDkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            notifySeedTimeoutGasOffset,
            notifyDkgTimeoutNegativeGasOffset
        );
        dkgResultSubmissionGasChangeInitiated = 0;
        newDkgResultSubmissionGas = 0;
    }

    /// @notice Begins the dkg approval gas offset update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultApprovalGasOffset New DKG result approval gas.
    function beginDkgResultApprovalGasOffsetUpdate(
        uint256 _newDkgResultApprovalGasOffset
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newDkgResultApprovalGasOffset = _newDkgResultApprovalGasOffset;
        dkgResultApprovalGasOffsetChangeInitiated = block.timestamp;
        emit DkgResultApprovalGasOffsetUpdateStarted(
            _newDkgResultApprovalGasOffset,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the dkg result approval gas offset update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgResultApprovalGasOffsetUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(dkgResultApprovalGasOffsetChangeInitiated)
    {
        emit DkgResultApprovalGasOffsetUpdated(newDkgResultApprovalGasOffset);
        (
            uint256 dkgResultSubmissionGas,
            ,
            uint256 notifyOperatorInactivityGasOffset,
            uint256 notifySeedTimeoutGasOffset,
            uint256 notifyDkgTimeoutNegativeGasOffset
        ) = walletRegistry.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateGasParameters(
            dkgResultSubmissionGas,
            newDkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            notifySeedTimeoutGasOffset,
            notifyDkgTimeoutNegativeGasOffset
        );
        dkgResultApprovalGasOffsetChangeInitiated = 0;
        newDkgResultApprovalGasOffset = 0;
    }

    /// @notice Begins the notify operator inactivity gas offset update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newNotifyOperatorInactivityGasOffset New operator inactivity
    ///        notification gas offset
    function beginNotifyOperatorInactivityGasOffsetUpdate(
        uint256 _newNotifyOperatorInactivityGasOffset
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newNotifyOperatorInactivityGasOffset = _newNotifyOperatorInactivityGasOffset;
        notifyOperatorInactivityGasOffsetChangeInitiated = block.timestamp;
        emit NotifyOperatorInactivityGasOffsetUpdateStarted(
            _newNotifyOperatorInactivityGasOffset,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the notify operator inactivity gas offset update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeNotifyOperatorInactivityGasOffsetUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            notifyOperatorInactivityGasOffsetChangeInitiated
        )
    {
        emit NotifyOperatorInactivityGasOffsetUpdated(
            newNotifyOperatorInactivityGasOffset
        );
        (
            uint256 dkgResultSubmissionGas,
            uint256 dkgResultApprovalGasOffset,
            ,
            uint256 notifySeedTimeoutGasOffset,
            uint256 notifyDkgTimeoutNegativeGasOffset
        ) = walletRegistry.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateGasParameters(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            newNotifyOperatorInactivityGasOffset,
            notifySeedTimeoutGasOffset,
            notifyDkgTimeoutNegativeGasOffset
        );
        notifyOperatorInactivityGasOffsetChangeInitiated = 0;
        newNotifyOperatorInactivityGasOffset = 0;
    }

    /// @notice Begins the notify seed for DKG delivery timeout gas offset update
    ///         process.
    /// @dev Can be called only by the contract owner.
    /// @param _newNotifySeedTimeoutGasOffset New seed for DKG delivery timeout
    ///        notification gas offset
    function beginNotifySeedTimeoutGasOffsetUpdate(
        uint256 _newNotifySeedTimeoutGasOffset
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newNotifySeedTimeoutGasOffset = _newNotifySeedTimeoutGasOffset;
        notifySeedTimeoutGasOffsetChangeInitiated = block.timestamp;
        emit NotifySeedTimeoutGasOffsetUpdateStarted(
            _newNotifySeedTimeoutGasOffset,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the notify seed for DKG delivery timeout gas offset
    ///         update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeNotifySeedTimeoutGasOffsetUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(notifySeedTimeoutGasOffsetChangeInitiated)
    {
        emit NotifySeedTimeoutGasOffsetUpdated(newNotifySeedTimeoutGasOffset);
        (
            uint256 dkgResultSubmissionGas,
            uint256 dkgResultApprovalGasOffset,
            uint256 notifyOperatorInactivityGasOffset,
            ,
            uint256 notifyDkgTimeoutNegativeGasOffset
        ) = walletRegistry.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateGasParameters(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            newNotifySeedTimeoutGasOffset,
            notifyDkgTimeoutNegativeGasOffset
        );
        notifySeedTimeoutGasOffsetChangeInitiated = 0;
        newNotifySeedTimeoutGasOffset = 0;
    }

    /// @notice Begins the notify DKG timeout negative gas offset update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newNotifyDkgTimeoutNegativeGasOffset New DKG timeout negative gas
    ///        notification gas offset
    function beginNotifyDkgTimeoutNegativeGasOffsetUpdate(
        uint256 _newNotifyDkgTimeoutNegativeGasOffset
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newNotifyDkgTimeoutNegativeGasOffset = _newNotifyDkgTimeoutNegativeGasOffset;
        notifyDkgTimeoutNegativeGasOffsetChangeInitiated = block.timestamp;
        emit NotifyDkgTimeoutNegativeGasOffsetUpdateStarted(
            _newNotifyDkgTimeoutNegativeGasOffset,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the notify DKG timeout negative gas offset update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeNotifyDkgTimeoutNegativeGasOffsetUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            notifyDkgTimeoutNegativeGasOffsetChangeInitiated
        )
    {
        emit NotifyDkgTimeoutNegativeGasOffsetUpdated(
            newNotifyDkgTimeoutNegativeGasOffset
        );
        (
            uint256 dkgResultSubmissionGas,
            uint256 dkgResultApprovalGasOffset,
            uint256 notifyOperatorInactivityGasOffset,
            uint256 notifySeedTimeoutGasOffset,

        ) = walletRegistry.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateGasParameters(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            notifySeedTimeoutGasOffset,
            newNotifyDkgTimeoutNegativeGasOffset
        );
        notifyDkgTimeoutNegativeGasOffsetChangeInitiated = 0;
        newNotifyDkgTimeoutNegativeGasOffset = 0;
    }

    /// @notice Begins the reimbursement pool update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newReimbursementPool New reimbursement pool.
    function beginReimbursementPoolUpdate(address payable _newReimbursementPool)
        external
        onlyOwner
    {
        require(
            address(_newReimbursementPool) != address(0),
            "New reimbursement pool address cannot be zero"
        );
        /* solhint-disable not-rely-on-time */
        newReimbursementPool = _newReimbursementPool;
        reimbursementPoolChangeInitiated = block.timestamp;
        emit ReimbursementPoolUpdateStarted(
            _newReimbursementPool,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the reimbursement pool update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeReimbursementPoolUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(reimbursementPoolChangeInitiated)
    {
        emit ReimbursementPoolUpdated(newReimbursementPool);
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateReimbursementPool(
            ReimbursementPool(newReimbursementPool)
        );
        reimbursementPoolChangeInitiated = 0;
        newReimbursementPool = payable(address(0));
    }

    /// @notice Begins the sortition pool rewards ban duration update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newSortitionPoolRewardsBanDuration New sortition pool rewards
    ///        ban duration.
    function beginSortitionPoolRewardsBanDurationUpdate(
        uint256 _newSortitionPoolRewardsBanDuration
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newSortitionPoolRewardsBanDuration = _newSortitionPoolRewardsBanDuration;
        sortitionPoolRewardsBanDurationChangeInitiated = block.timestamp;
        emit SortitionPoolRewardsBanDurationUpdateStarted(
            _newSortitionPoolRewardsBanDuration,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the sortition pool rewards ban duration update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeSortitionPoolRewardsBanDurationUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(sortitionPoolRewardsBanDurationChangeInitiated)
    {
        emit SortitionPoolRewardsBanDurationUpdated(
            newSortitionPoolRewardsBanDuration
        );
        (
            uint256 maliciousDkgResultNotificationRewardMultiplier,

        ) = walletRegistry.rewardParameters();
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateRewardParameters(
            maliciousDkgResultNotificationRewardMultiplier,
            newSortitionPoolRewardsBanDuration
        );
        sortitionPoolRewardsBanDurationChangeInitiated = 0;
        newSortitionPoolRewardsBanDuration = 0;
    }

    /// @notice Begins the DKG seed timeout update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgSeedTimeout New DKG seed timeout in blocks
    function beginDkgSeedTimeoutUpdate(uint256 _newDkgSeedTimeout)
        external
        onlyOwner
    {
        /* solhint-disable not-rely-on-time */
        require(_newDkgSeedTimeout > 0, "DKG seed timeout must be > 0");
        newDkgSeedTimeout = _newDkgSeedTimeout;
        dkgSeedTimeoutChangeInitiated = block.timestamp;
        emit DkgSeedTimeoutUpdateStarted(_newDkgSeedTimeout, block.timestamp);
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG seed timeout update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgSeedTimeoutUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(dkgSeedTimeoutChangeInitiated)
    {
        emit DkgSeedTimeoutUpdated(newDkgSeedTimeout);
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateDkgParameters(
            newDkgSeedTimeout,
            walletRegistry.dkgParameters().resultChallengePeriodLength,
            walletRegistry.dkgParameters().resultChallengeExtraGas,
            walletRegistry.dkgParameters().resultSubmissionTimeout,
            walletRegistry.dkgParameters().submitterPrecedencePeriodLength
        );
        dkgSeedTimeoutChangeInitiated = 0;
        newDkgSeedTimeout = 0;
    }

    /// @notice Begins the DKG result challenge period length update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultChallengePeriodLength New DKG result challenge
    ///        period length in blocks
    function beginDkgResultChallengePeriodLengthUpdate(
        uint256 _newDkgResultChallengePeriodLength
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newDkgResultChallengePeriodLength >= 10,
            "DKG result challenge period length must be >= 10"
        );
        newDkgResultChallengePeriodLength = _newDkgResultChallengePeriodLength;
        dkgResultChallengePeriodLengthChangeInitiated = block.timestamp;
        emit DkgResultChallengePeriodLengthUpdateStarted(
            _newDkgResultChallengePeriodLength,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG result challenge period length update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgResultChallengePeriodLengthUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(dkgResultChallengePeriodLengthChangeInitiated)
    {
        emit DkgResultChallengePeriodLengthUpdated(
            newDkgResultChallengePeriodLength
        );
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateDkgParameters(
            walletRegistry.dkgParameters().seedTimeout,
            newDkgResultChallengePeriodLength,
            walletRegistry.dkgParameters().resultChallengeExtraGas,
            walletRegistry.dkgParameters().resultSubmissionTimeout,
            walletRegistry.dkgParameters().submitterPrecedencePeriodLength
        );
        dkgResultChallengePeriodLengthChangeInitiated = 0;
        newDkgResultChallengePeriodLength = 0;
    }

    /// @notice Begins the DKG result challenge extra gas update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultChallengeExtraGas New DKG result challenge extra gas
    function beginDkgResultChallengeExtraGasUpdate(
        uint256 _newDkgResultChallengeExtraGas
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newDkgResultChallengeExtraGas = _newDkgResultChallengeExtraGas;
        dkgResultChallengeExtraGasChangeInitiated = block.timestamp;
        emit DkgResultChallengeExtraGasUpdateStarted(
            _newDkgResultChallengeExtraGas,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG result challenge extra gas update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgResultChallengeExtraGasUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(dkgResultChallengeExtraGasChangeInitiated)
    {
        emit DkgResultChallengeExtraGasUpdated(newDkgResultChallengeExtraGas);
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateDkgParameters(
            walletRegistry.dkgParameters().seedTimeout,
            walletRegistry.dkgParameters().resultChallengePeriodLength,
            newDkgResultChallengeExtraGas,
            walletRegistry.dkgParameters().resultSubmissionTimeout,
            walletRegistry.dkgParameters().submitterPrecedencePeriodLength
        );
        dkgResultChallengeExtraGasChangeInitiated = 0;
        newDkgResultChallengeExtraGas = 0;
    }

    /// @notice Begins the DKG result submission timeout update
    ///         process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultSubmissionTimeout New DKG result submission timeout
    ///        in blocks
    function beginDkgResultSubmissionTimeoutUpdate(
        uint256 _newDkgResultSubmissionTimeout
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newDkgResultSubmissionTimeout > 0,
            "DKG result submission timeout must be > 0"
        );
        newDkgResultSubmissionTimeout = _newDkgResultSubmissionTimeout;
        dkgResultSubmissionTimeoutChangeInitiated = block.timestamp;
        emit DkgResultSubmissionTimeoutUpdateStarted(
            _newDkgResultSubmissionTimeout,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG result submission timeout update
    ///         process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgResultSubmissionTimeoutUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(dkgResultSubmissionTimeoutChangeInitiated)
    {
        emit DkgResultSubmissionTimeoutUpdated(newDkgResultSubmissionTimeout);
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateDkgParameters(
            walletRegistry.dkgParameters().seedTimeout,
            walletRegistry.dkgParameters().resultChallengePeriodLength,
            walletRegistry.dkgParameters().resultChallengeExtraGas,
            newDkgResultSubmissionTimeout,
            walletRegistry.dkgParameters().submitterPrecedencePeriodLength
        );
        dkgResultSubmissionTimeoutChangeInitiated = 0;
        newDkgResultSubmissionTimeout = 0;
    }

    /// @notice Begins the DKG submitter precedence period length update
    ///         process.
    /// @dev Can be called only by the contract owner.
    /// @param _newSubmitterPrecedencePeriodLength New DKG submitter precedence
    ///        period length in blocks
    function beginDkgSubmitterPrecedencePeriodLengthUpdate(
        uint256 _newSubmitterPrecedencePeriodLength
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newSubmitterPrecedencePeriodLength > 0,
            "DKG submitter precedence period length must be > 0"
        );
        newSubmitterPrecedencePeriodLength = _newSubmitterPrecedencePeriodLength;
        dkgSubmitterPrecedencePeriodLengthChangeInitiated = block.timestamp;
        emit DkgSubmitterPrecedencePeriodLengthUpdateStarted(
            _newSubmitterPrecedencePeriodLength,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG submitter precedence period length update
    ///         process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgSubmitterPrecedencePeriodLengthUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            dkgSubmitterPrecedencePeriodLengthChangeInitiated
        )
    {
        emit DkgSubmitterPrecedencePeriodLengthUpdated(
            newSubmitterPrecedencePeriodLength
        );
        // slither-disable-next-line reentrancy-no-eth
        walletRegistry.updateDkgParameters(
            walletRegistry.dkgParameters().seedTimeout,
            walletRegistry.dkgParameters().resultChallengePeriodLength,
            walletRegistry.dkgParameters().resultChallengeExtraGas,
            walletRegistry.dkgParameters().resultSubmissionTimeout,
            newSubmitterPrecedencePeriodLength
        );
        dkgSubmitterPrecedencePeriodLengthChangeInitiated = 0;
        newSubmitterPrecedencePeriodLength = 0;
    }

    /// @notice Withdraws rewards belonging to operators marked as ineligible
    ///         for sortition pool rewards.
    /// @dev Can be called only by the contract owner.
    /// @param recipient Recipient of withdrawn rewards.
    function withdrawIneligibleRewards(address recipient) external onlyOwner {
        walletRegistry.withdrawIneligibleRewards(recipient);
    }

    /// @notice Get the time remaining until the governance delay can be updated.
    /// @return Remaining time in seconds.
    function getRemainingGovernanceDelayUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(governanceDelayChangeInitiated);
    }

    /// @notice Get the time remaining until the wallet registry governance can
    ///         be transferred.
    /// @return Remaining time in seconds.
    function getRemainingWalletRegistryGovernanceTransferDelayTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(walletRegistryGovernanceTransferInitiated);
    }

    /// @notice Get the time remaining until the minimum authorization amount
    ///         can be updated.
    /// @return Remaining time in seconds.
    function getRemainingMimimumAuthorizationUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(minimumAuthorizationChangeInitiated);
    }

    /// @notice Get the time remaining until the authorization decrease delay
    ///         can be updated.
    /// @return Remaining time in seconds.
    function getRemainingAuthorizationDecreaseDelayUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(authorizationDecreaseDelayChangeInitiated);
    }

    /// @notice Get the time remaining until the authorization decrease change
    ///         period can be updated.
    /// @return Remaining time in seconds.
    function getRemainingAuthorizationDecreaseChangePeriodUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                authorizationDecreaseChangePeriodChangeInitiated
            );
    }

    /// @notice Get the time remaining until the malicious DKG result
    ///         slashing amount can be updated.
    /// @return Remaining time in seconds.
    function getRemainingMaliciousDkgResultSlashingAmountUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                maliciousDkgResultSlashingAmountChangeInitiated
            );
    }

    /// @notice Get the time remaining until the DKG malicious result
    ///         notification reward multiplier duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingMaliciousDkgResultNotificationRewardMultiplierUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                maliciousDkgResultNotificationRewardMultiplierChangeInitiated
            );
    }

    /// @notice Get the time remaining until the sortition pool rewards ban
    ///         duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingSortitionPoolRewardsBanDurationUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                sortitionPoolRewardsBanDurationChangeInitiated
            );
    }

    /// @notice Get the time remaining until the DKG seed timeout can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgSeedTimeoutUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(dkgSeedTimeoutChangeInitiated);
    }

    /// @notice Get the time remaining until the DKG result challenge period
    ///         length can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgResultChallengePeriodLengthUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                dkgResultChallengePeriodLengthChangeInitiated
            );
    }

    function getRemainingDkgResultChallengeExtraGasUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(dkgResultChallengeExtraGasChangeInitiated);
    }

    /// @notice Get the time remaining until the DKG result submission timeout
    ///         can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgResultSubmissionTimeoutUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(dkgResultSubmissionTimeoutChangeInitiated);
    }

    /// @notice Get the time remaining until the wallet owner can be updated.
    /// @return Remaining time in seconds.
    function getRemainingWalletOwnerUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(walletOwnerChangeInitiated);
    }

    /// @notice Get the time remaining until the wallet owner can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgSubmitterPrecedencePeriodLengthUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                dkgSubmitterPrecedencePeriodLengthChangeInitiated
            );
    }

    /// @notice Get the time remaining until the dkg result submission gas can
    ///         be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgResultSubmissionGasUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(dkgResultSubmissionGasChangeInitiated);
    }

    /// @notice Get the time remaining until the dkg result approval gas offset
    ///         can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgResultApprovalGasOffsetUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(dkgResultApprovalGasOffsetChangeInitiated);
    }

    /// @notice Get the time remaining until the operator inactivity gas offset
    ///         can be updated.
    /// @return Remaining time in seconds.
    function getRemainingNotifyOperatorInactivityGasOffsetUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                notifyOperatorInactivityGasOffsetChangeInitiated
            );
    }

    /// @notice Get the time remaining until the seed for DKG delivery timeout
    /// gas offset can be updated.
    /// @return Remaining time in seconds.
    function getRemainingNotifySeedTimeoutGasOffsetUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(notifySeedTimeoutGasOffsetChangeInitiated);
    }

    /// @notice Get the time remaining until the DKG timeout negative gas offset
    ///         can be updated.
    /// @return Remaining time in seconds.
    function getRemainingNotifyDkgTimeoutNegativeGasOffsetUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                notifyDkgTimeoutNegativeGasOffsetChangeInitiated
            );
    }

    /// @notice Get the time remaining until reimbursement pool can be updated.
    /// @return Remaining time in seconds.
    function getRemainingReimbursementPoolUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(reimbursementPoolChangeInitiated);
    }

    /// @notice Gets the time remaining until the governable parameter update
    ///         can be committed.
    /// @param changeTimestamp Timestamp indicating the beginning of the change.
    /// @return Remaining time in seconds.
    function getRemainingChangeTime(uint256 changeTimestamp)
        internal
        view
        returns (uint256)
    {
        require(changeTimestamp > 0, "Change not initiated");
        /* solhint-disable-next-line not-rely-on-time */
        uint256 elapsed = block.timestamp - changeTimestamp;
        if (elapsed >= governanceDelay) {
            return 0;
        }

        return governanceDelay - elapsed;
    }
}
