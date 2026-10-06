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

import "./RandomBeacon.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/// @title Keep Random Beacon Governance
/// @notice Owns the `RandomBeacon` contract and is responsible for updating its
///         governable parameters in respect to governance delay individual
///         for each parameter.
contract RandomBeaconGovernance is Ownable {
    uint256 public newGovernanceDelay;
    uint256 public governanceDelayChangeInitiated;

    address public newRandomBeaconGovernance;
    uint256 public randomBeaconGovernanceTransferInitiated;

    uint256 public newRelayEntrySoftTimeout;
    uint256 public relayEntrySoftTimeoutChangeInitiated;

    uint256 public newRelayEntryHardTimeout;
    uint256 public relayEntryHardTimeoutChangeInitiated;

    uint256 public newCallbackGasLimit;
    uint256 public callbackGasLimitChangeInitiated;

    uint256 public newGroupCreationFrequency;
    uint256 public groupCreationFrequencyChangeInitiated;

    uint256 public newGroupLifetime;
    uint256 public groupLifetimeChangeInitiated;

    uint256 public newDkgResultChallengePeriodLength;
    uint256 public dkgResultChallengePeriodLengthChangeInitiated;

    uint256 public newDkgResultChallengeExtraGas;
    uint256 public dkgResultChallengeExtraGasChangeInitiated;

    uint256 public newDkgResultSubmissionTimeout;
    uint256 public dkgResultSubmissionTimeoutChangeInitiated;

    uint256 public newDkgSubmitterPrecedencePeriodLength;
    uint256 public dkgSubmitterPrecedencePeriodLengthChangeInitiated;

    uint96 public newRelayEntrySubmissionFailureSlashingAmount;
    uint256 public relayEntrySubmissionFailureSlashingAmountChangeInitiated;

    uint96 public newMaliciousDkgResultSlashingAmount;
    uint256 public maliciousDkgResultSlashingAmountChangeInitiated;

    uint96 public newUnauthorizedSigningSlashingAmount;
    uint256 public unauthorizedSigningSlashingAmountChangeInitiated;

    uint256 public newSortitionPoolRewardsBanDuration;
    uint256 public sortitionPoolRewardsBanDurationChangeInitiated;

    uint256 public newRelayEntryTimeoutNotificationRewardMultiplier;
    uint256 public relayEntryTimeoutNotificationRewardMultiplierChangeInitiated;

    uint256 public newUnauthorizedSigningNotificationRewardMultiplier;
    uint256
        public unauthorizedSigningNotificationRewardMultiplierChangeInitiated;

    uint96 public newMinimumAuthorization;
    uint256 public minimumAuthorizationChangeInitiated;

    uint64 public newAuthorizationDecreaseDelay;
    uint256 public authorizationDecreaseDelayChangeInitiated;

    uint64 public newAuthorizationDecreaseChangePeriod;
    uint256 public authorizationDecreaseChangePeriodChangeInitiated;

    uint256 public newDkgMaliciousResultNotificationRewardMultiplier;
    uint256
        public dkgMaliciousResultNotificationRewardMultiplierChangeInitiated;

    uint256 public newDkgResultSubmissionGas;
    uint256 public dkgResultSubmissionGasChangeInitiated;

    uint256 public newDkgResultApprovalGasOffset;
    uint256 public dkgResultApprovalGasOffsetChangeInitiated;

    uint256 public newNotifyOperatorInactivityGasOffset;
    uint256 public notifyOperatorInactivityGasOffsetChangeInitiated;

    uint256 public newRelayEntrySubmissionGasOffset;
    uint256 public relayEntrySubmissionGasOffsetChangeInitiated;

    RandomBeacon public immutable randomBeacon;

    uint256 public governanceDelay;

    event GovernanceDelayUpdateStarted(
        uint256 governanceDelay,
        uint256 timestamp
    );
    event GovernanceDelayUpdated(uint256 governanceDelay);

    event RandomBeaconGovernanceTransferStarted(
        address newRandomBeaconGovernance,
        uint256 timestamp
    );
    event RandomBeaconGovernanceTransferred(address newRandomBeaconGovernance);

    event RelayEntrySoftTimeoutUpdateStarted(
        uint256 relayEntrySoftTimeout,
        uint256 timestamp
    );
    event RelayEntrySoftTimeoutUpdated(uint256 relayEntrySoftTimeout);

    event RelayEntryHardTimeoutUpdateStarted(
        uint256 relayEntryHardTimeout,
        uint256 timestamp
    );
    event RelayEntryHardTimeoutUpdated(uint256 relayEntryHardTimeout);

    event CallbackGasLimitUpdateStarted(
        uint256 callbackGasLimit,
        uint256 timestamp
    );
    event CallbackGasLimitUpdated(uint256 callbackGasLimit);

    event GroupCreationFrequencyUpdateStarted(
        uint256 groupCreationFrequency,
        uint256 timestamp
    );
    event GroupCreationFrequencyUpdated(uint256 groupCreationFrequency);

    event GroupLifetimeUpdateStarted(uint256 groupLifetime, uint256 timestamp);
    event GroupLifetimeUpdated(uint256 groupLifetime);

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

    event RelayEntrySubmissionFailureSlashingAmountUpdateStarted(
        uint96 relayEntrySubmissionFailureSlashingAmount,
        uint256 timestamp
    );
    event RelayEntrySubmissionFailureSlashingAmountUpdated(
        uint96 relayEntrySubmissionFailureSlashingAmount
    );

    event MaliciousDkgResultSlashingAmountUpdateStarted(
        uint96 maliciousDkgResultSlashingAmount,
        uint256 timestamp
    );
    event MaliciousDkgResultSlashingAmountUpdated(
        uint96 maliciousDkgResultSlashingAmount
    );

    event UnauthorizedSigningSlashingAmountUpdateStarted(
        uint96 unauthorizedSigningSlashingAmount,
        uint256 timestamp
    );
    event UnauthorizedSigningSlashingAmountUpdated(
        uint96 unauthorizedSigningSlashingAmount
    );

    event SortitionPoolRewardsBanDurationUpdateStarted(
        uint256 sortitionPoolRewardsBanDuration,
        uint256 timestamp
    );
    event SortitionPoolRewardsBanDurationUpdated(
        uint256 sortitionPoolRewardsBanDuration
    );

    event RelayEntryTimeoutNotificationRewardMultiplierUpdateStarted(
        uint256 relayEntryTimeoutNotificationRewardMultiplier,
        uint256 timestamp
    );
    event RelayEntryTimeoutNotificationRewardMultiplierUpdated(
        uint256 relayEntryTimeoutNotificationRewardMultiplier
    );

    event UnauthorizedSigningNotificationRewardMultiplierUpdateStarted(
        uint256 unauthorizedSigningTimeoutNotificationRewardMultiplier,
        uint256 timestamp
    );
    event UnauthorizedSigningNotificationRewardMultiplierUpdated(
        uint256 unauthorizedSigningTimeoutNotificationRewardMultiplier
    );

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

    event DkgMaliciousResultNotificationRewardMultiplierUpdateStarted(
        uint256 dkgMaliciousResultNotificationRewardMultiplier,
        uint256 timestamp
    );
    event DkgMaliciousResultNotificationRewardMultiplierUpdated(
        uint256 dkgMaliciousResultNotificationRewardMultiplier
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

    event RelayEntrySubmissionGasOffsetUpdateStarted(
        uint256 relayEntrySubmissionGasOffset,
        uint256 timestamp
    );
    event RelayEntrySubmissionGasOffsetUpdated(
        uint256 relayEntrySubmissionGasOffset
    );

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

    constructor(RandomBeacon _randomBeacon, uint256 _governanceDelay) {
        require(address(_randomBeacon) != address(0), "Zero-address reference");
        require(_governanceDelay != 0, "No governance delay");

        randomBeacon = _randomBeacon;
        governanceDelay = _governanceDelay;
    }

    /// @notice Begins the governance delay update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newGovernanceDelay New governance delay
    function beginGovernanceDelayUpdate(
        uint256 _newGovernanceDelay
    ) external onlyOwner {
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

    /// @notice Begins the random beacon governance transfer process.
    /// @dev Can be called only by the current contract governance.
    function beginRandomBeaconGovernanceTransfer(
        address _newRandomBeaconGovernance
    ) external onlyOwner {
        require(
            address(_newRandomBeaconGovernance) != address(0),
            "New random beacon governance address cannot be zero"
        );
        newRandomBeaconGovernance = _newRandomBeaconGovernance;
        /* solhint-disable not-rely-on-time */
        randomBeaconGovernanceTransferInitiated = block.timestamp;
        emit RandomBeaconGovernanceTransferStarted(
            _newRandomBeaconGovernance,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the random beacon governance transfer process.
    /// @dev Can be called only by the current contract governance, after the
    ///      governance delay elapses.
    function finalizeRandomBeaconGovernanceTransfer()
        external
        onlyOwner
        onlyAfterGovernanceDelay(randomBeaconGovernanceTransferInitiated)
    {
        emit RandomBeaconGovernanceTransferred(newRandomBeaconGovernance);
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.transferGovernance(newRandomBeaconGovernance);
        randomBeaconGovernanceTransferInitiated = 0;
        newRandomBeaconGovernance = address(0);
    }

    /// @notice Begins the relay entry soft timeout update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newRelayEntrySoftTimeout New relay entry submission timeout in blocks
    function beginRelayEntrySoftTimeoutUpdate(
        uint256 _newRelayEntrySoftTimeout
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newRelayEntrySoftTimeout > 0,
            "Relay entry soft timeout must be > 0"
        );
        newRelayEntrySoftTimeout = _newRelayEntrySoftTimeout;
        relayEntrySoftTimeoutChangeInitiated = block.timestamp;
        emit RelayEntrySoftTimeoutUpdateStarted(
            _newRelayEntrySoftTimeout,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the relay entry soft timeout update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeRelayEntrySoftTimeoutUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(relayEntrySoftTimeoutChangeInitiated)
    {
        emit RelayEntrySoftTimeoutUpdated(newRelayEntrySoftTimeout);
        (
            ,
            uint256 relayEntryHardTimeout,
            uint256 callbackGasLimit
        ) = randomBeacon.relayEntryParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRelayEntryParameters(
            newRelayEntrySoftTimeout,
            relayEntryHardTimeout,
            callbackGasLimit
        );
        relayEntrySoftTimeoutChangeInitiated = 0;
        newRelayEntrySoftTimeout = 0;
    }

    /// @notice Begins the relay entry hard timeout update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newRelayEntryHardTimeout New relay entry hard timeout in blocks
    function beginRelayEntryHardTimeoutUpdate(
        uint256 _newRelayEntryHardTimeout
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newRelayEntryHardTimeout = _newRelayEntryHardTimeout;
        relayEntryHardTimeoutChangeInitiated = block.timestamp;
        emit RelayEntryHardTimeoutUpdateStarted(
            _newRelayEntryHardTimeout,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the relay entry hard timeout update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeRelayEntryHardTimeoutUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(relayEntryHardTimeoutChangeInitiated)
    {
        emit RelayEntryHardTimeoutUpdated(newRelayEntryHardTimeout);
        (
            uint256 relayEntrySoftTimeout,
            ,
            uint256 callbackGasLimit
        ) = randomBeacon.relayEntryParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRelayEntryParameters(
            relayEntrySoftTimeout,
            newRelayEntryHardTimeout,
            callbackGasLimit
        );
        relayEntryHardTimeoutChangeInitiated = 0;
        newRelayEntryHardTimeout = 0;
    }

    /// @notice Begins the callback gas limit update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newCallbackGasLimit New callback gas limit
    function beginCallbackGasLimitUpdate(
        uint256 _newCallbackGasLimit
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        // slither-disable-next-line too-many-digits
        require(
            _newCallbackGasLimit > 0 && _newCallbackGasLimit <= 1e6,
            "Callback gas limit must be > 0 and <= 1000000"
        );
        newCallbackGasLimit = _newCallbackGasLimit;
        callbackGasLimitChangeInitiated = block.timestamp;
        emit CallbackGasLimitUpdateStarted(
            _newCallbackGasLimit,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the callback gas limit update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeCallbackGasLimitUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(callbackGasLimitChangeInitiated)
    {
        emit CallbackGasLimitUpdated(newCallbackGasLimit);
        (
            uint256 relayEntrySoftTimeout,
            uint256 relayEntryHardTimeout,

        ) = randomBeacon.relayEntryParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRelayEntryParameters(
            relayEntrySoftTimeout,
            relayEntryHardTimeout,
            newCallbackGasLimit
        );
        callbackGasLimitChangeInitiated = 0;
        newCallbackGasLimit = 0;
    }

    /// @notice Begins the group creation frequency update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newGroupCreationFrequency New group creation frequency
    function beginGroupCreationFrequencyUpdate(
        uint256 _newGroupCreationFrequency
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newGroupCreationFrequency > 0,
            "Group creation frequency must be > 0"
        );
        newGroupCreationFrequency = _newGroupCreationFrequency;
        groupCreationFrequencyChangeInitiated = block.timestamp;
        emit GroupCreationFrequencyUpdateStarted(
            _newGroupCreationFrequency,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the group creation frequency update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeGroupCreationFrequencyUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(groupCreationFrequencyChangeInitiated)
    {
        emit GroupCreationFrequencyUpdated(newGroupCreationFrequency);
        (
            ,
            uint256 groupLifetime,
            uint256 dkgResultChallengePeriodLength,
            uint256 dkgResultChallengeExtraGas,
            uint256 dkgResultSubmissionTimeout,
            uint256 dkgSubmitterPrecedencePeriodLength
        ) = randomBeacon.groupCreationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGroupCreationParameters(
            newGroupCreationFrequency,
            groupLifetime,
            dkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
        );
        groupCreationFrequencyChangeInitiated = 0;
        newGroupCreationFrequency = 0;
    }

    /// @notice Begins the group lifetime update process. Group lifetime needs to
    ///         be shorter than the authorization decrease delay to ensure every
    ///         active group is backed by enough stake. A new group lifetime value
    ///         is in blocks and has to be calculated based on the average block
    ///         time and authorization decrease delay which value is in seconds.
    /// @dev Can be called only by the contract owner.
    /// @param _newGroupLifetime New group lifetime in blocks
    function beginGroupLifetimeUpdate(
        uint256 _newGroupLifetime
    ) external onlyOwner {
        require(_newGroupLifetime > 0, "Group lifetime must be greater than 0");
        /* solhint-disable not-rely-on-time */
        newGroupLifetime = _newGroupLifetime;
        groupLifetimeChangeInitiated = block.timestamp;
        emit GroupLifetimeUpdateStarted(_newGroupLifetime, block.timestamp);
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the group creation frequency update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeGroupLifetimeUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(groupLifetimeChangeInitiated)
    {
        emit GroupLifetimeUpdated(newGroupLifetime);
        (
            uint256 groupCreationFrequency,
            ,
            uint256 dkgResultChallengePeriodLength,
            uint256 dkgResultChallengeExtraGas,
            uint256 dkgResultSubmissionTimeout,
            uint256 dkgSubmitterPrecedencePeriodLength
        ) = randomBeacon.groupCreationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGroupCreationParameters(
            groupCreationFrequency,
            newGroupLifetime,
            dkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
        );
        groupLifetimeChangeInitiated = 0;
        newGroupLifetime = 0;
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
        (
            uint256 groupCreationFrequency,
            uint256 groupLifetime,
            ,
            uint256 dkgResultChallengeExtraGas,
            uint256 dkgResultSubmissionTimeout,
            uint256 dkgSubmitterPrecedencePeriodLength
        ) = randomBeacon.groupCreationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGroupCreationParameters(
            groupCreationFrequency,
            groupLifetime,
            newDkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
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
        (
            uint256 groupCreationFrequency,
            uint256 groupLifetime,
            uint256 dkgResultChallengePeriodLength,
            ,
            uint256 dkgResultSubmissionTimeout,
            uint256 dkgSubmitterPrecedencePeriodLength
        ) = randomBeacon.groupCreationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGroupCreationParameters(
            groupCreationFrequency,
            groupLifetime,
            dkgResultChallengePeriodLength,
            newDkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
        );
        dkgResultChallengeExtraGasChangeInitiated = 0;
        newDkgResultChallengeExtraGas = 0;
    }

    /// @notice Begins the DKG result submission timeout update
    ///         process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultSubmissionTimeout New DKG result submission
    ///        timeout in blocks
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
        (
            uint256 groupCreationFrequency,
            uint256 groupLifetime,
            uint256 dkgResultChallengePeriodLength,
            uint256 dkgResultChallengeExtraGas,
            ,
            uint256 dkgSubmitterPrecedencePeriodLength
        ) = randomBeacon.groupCreationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGroupCreationParameters(
            groupCreationFrequency,
            groupLifetime,
            dkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            newDkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
        );
        dkgResultSubmissionTimeoutChangeInitiated = 0;
        newDkgResultSubmissionTimeout = 0;
    }

    /// @notice Begins the DKG submitter precedence period length.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgSubmitterPrecedencePeriodLength New DKG submitter precedence
    ///        period length in blocks
    function beginDkgSubmitterPrecedencePeriodLengthUpdate(
        uint256 _newDkgSubmitterPrecedencePeriodLength
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newDkgSubmitterPrecedencePeriodLength > 0,
            "DKG submitter precedence period length must be > 0"
        );
        newDkgSubmitterPrecedencePeriodLength =
            _newDkgSubmitterPrecedencePeriodLength;
        dkgSubmitterPrecedencePeriodLengthChangeInitiated = block.timestamp;
        emit DkgSubmitterPrecedencePeriodLengthUpdateStarted(
            _newDkgSubmitterPrecedencePeriodLength,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG submitter precedence period length.
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
            newDkgSubmitterPrecedencePeriodLength
        );
        (
            uint256 groupCreationFrequency,
            uint256 groupLifetime,
            uint256 dkgResultChallengePeriodLength,
            uint256 dkgResultChallengeExtraGas,
            uint256 dkgResultSubmissionTimeout,

        ) = randomBeacon.groupCreationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGroupCreationParameters(
            groupCreationFrequency,
            groupLifetime,
            dkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            newDkgSubmitterPrecedencePeriodLength
        );
        dkgSubmitterPrecedencePeriodLengthChangeInitiated = 0;
        newDkgSubmitterPrecedencePeriodLength = 0;
    }

    /// @notice Begins the sortition pool rewards ban duration update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newSortitionPoolRewardsBanDuration New sortition pool rewards
    ///        ban duration.
    function beginSortitionPoolRewardsBanDurationUpdate(
        uint256 _newSortitionPoolRewardsBanDuration
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newSortitionPoolRewardsBanDuration =
            _newSortitionPoolRewardsBanDuration;
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
            ,
            uint256 relayEntryTimeoutNotificationRewardMultiplier,
            uint256 unauthorizedSigningNotificationRewardMultiplier,
            uint256 dkgMaliciousResultNotificationRewardMultiplier
        ) = randomBeacon.rewardParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRewardParameters(
            newSortitionPoolRewardsBanDuration,
            relayEntryTimeoutNotificationRewardMultiplier,
            unauthorizedSigningNotificationRewardMultiplier,
            dkgMaliciousResultNotificationRewardMultiplier
        );
        sortitionPoolRewardsBanDurationChangeInitiated = 0;
        newSortitionPoolRewardsBanDuration = 0;
    }

    /// @notice Begins the relay entry timeout notification reward multiplier
    ///         update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newRelayEntryTimeoutNotificationRewardMultiplier New relay
    ///        entry timeout notification reward multiplier.
    function beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(
        uint256 _newRelayEntryTimeoutNotificationRewardMultiplier
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newRelayEntryTimeoutNotificationRewardMultiplier <= 100,
            "Maximum value is 100"
        );

        newRelayEntryTimeoutNotificationRewardMultiplier =
            _newRelayEntryTimeoutNotificationRewardMultiplier;
        relayEntryTimeoutNotificationRewardMultiplierChangeInitiated = block
            .timestamp;
        emit RelayEntryTimeoutNotificationRewardMultiplierUpdateStarted(
            _newRelayEntryTimeoutNotificationRewardMultiplier,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Begins the unauthorized signing notification reward multiplier
    ///         update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newUnauthorizedSigningNotificationRewardMultiplier New unauthorized
    ///         signing notification reward multiplier.
    function beginUnauthorizedSigningNotificationRewardMultiplierUpdate(
        uint256 _newUnauthorizedSigningNotificationRewardMultiplier
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newUnauthorizedSigningNotificationRewardMultiplier <= 100,
            "Maximum value is 100"
        );

        newUnauthorizedSigningNotificationRewardMultiplier =
            _newUnauthorizedSigningNotificationRewardMultiplier;
        unauthorizedSigningNotificationRewardMultiplierChangeInitiated = block
            .timestamp;
        emit UnauthorizedSigningNotificationRewardMultiplierUpdateStarted(
            _newUnauthorizedSigningNotificationRewardMultiplier,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the unauthorized signing notification reward
    ///         multiplier update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            unauthorizedSigningNotificationRewardMultiplierChangeInitiated
        )
    {
        emit UnauthorizedSigningNotificationRewardMultiplierUpdated(
            newUnauthorizedSigningNotificationRewardMultiplier
        );
        (
            uint256 sortitionPoolRewardsBanDuration,
            uint256 relayEntryTimeoutNotificationRewardMultiplier,
            ,
            uint256 dkgMaliciousResultNotificationRewardMultiplier
        ) = randomBeacon.rewardParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRewardParameters(
            sortitionPoolRewardsBanDuration,
            relayEntryTimeoutNotificationRewardMultiplier,
            newUnauthorizedSigningNotificationRewardMultiplier,
            dkgMaliciousResultNotificationRewardMultiplier
        );
        unauthorizedSigningNotificationRewardMultiplierChangeInitiated = 0;
        newUnauthorizedSigningNotificationRewardMultiplier = 0;
    }

    /// @notice Finalizes the relay entry timeout notification reward
    ///         multiplier update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            relayEntryTimeoutNotificationRewardMultiplierChangeInitiated
        )
    {
        emit RelayEntryTimeoutNotificationRewardMultiplierUpdated(
            newRelayEntryTimeoutNotificationRewardMultiplier
        );
        (
            uint256 sortitionPoolRewardsBanDuration,
            ,
            uint256 unauthorizedSigningNotificationRewardMultiplier,
            uint256 dkgMaliciousResultNotificationRewardMultiplier
        ) = randomBeacon.rewardParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRewardParameters(
            sortitionPoolRewardsBanDuration,
            newRelayEntryTimeoutNotificationRewardMultiplier,
            unauthorizedSigningNotificationRewardMultiplier,
            dkgMaliciousResultNotificationRewardMultiplier
        );
        relayEntryTimeoutNotificationRewardMultiplierChangeInitiated = 0;
        newRelayEntryTimeoutNotificationRewardMultiplier = 0;
    }

    /// @notice Begins the DKG malicious result notification reward multiplier
    ///         update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgMaliciousResultNotificationRewardMultiplier New DKG
    ///        malicious result notification reward multiplier.
    function beginDkgMaliciousResultNotificationRewardMultiplierUpdate(
        uint256 _newDkgMaliciousResultNotificationRewardMultiplier
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        require(
            _newDkgMaliciousResultNotificationRewardMultiplier <= 100,
            "Maximum value is 100"
        );

        newDkgMaliciousResultNotificationRewardMultiplier =
            _newDkgMaliciousResultNotificationRewardMultiplier;
        dkgMaliciousResultNotificationRewardMultiplierChangeInitiated = block
            .timestamp;
        emit DkgMaliciousResultNotificationRewardMultiplierUpdateStarted(
            _newDkgMaliciousResultNotificationRewardMultiplier,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the DKG malicious result notification reward
    ///         multiplier update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            dkgMaliciousResultNotificationRewardMultiplierChangeInitiated
        )
    {
        emit DkgMaliciousResultNotificationRewardMultiplierUpdated(
            newDkgMaliciousResultNotificationRewardMultiplier
        );
        (
            uint256 sortitionPoolRewardsBanDuration,
            uint256 relayEntryTimeoutNotificationRewardMultiplier,
            uint256 unauthorizedSigningNotificationRewardMultiplier,

        ) = randomBeacon.rewardParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateRewardParameters(
            sortitionPoolRewardsBanDuration,
            relayEntryTimeoutNotificationRewardMultiplier,
            unauthorizedSigningNotificationRewardMultiplier,
            newDkgMaliciousResultNotificationRewardMultiplier
        );
        dkgMaliciousResultNotificationRewardMultiplierChangeInitiated = 0;
        newDkgMaliciousResultNotificationRewardMultiplier = 0;
    }

    /// @notice Begins the relay entry submission failure slashing amount update
    ///         process.
    /// @dev Can be called only by the contract owner.
    /// @param _newRelayEntrySubmissionFailureSlashingAmount New relay entry
    ///        submission failure slashing amount
    function beginRelayEntrySubmissionFailureSlashingAmountUpdate(
        uint96 _newRelayEntrySubmissionFailureSlashingAmount
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newRelayEntrySubmissionFailureSlashingAmount =
            _newRelayEntrySubmissionFailureSlashingAmount;
        relayEntrySubmissionFailureSlashingAmountChangeInitiated = block
            .timestamp;
        emit RelayEntrySubmissionFailureSlashingAmountUpdateStarted(
            _newRelayEntrySubmissionFailureSlashingAmount,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the relay entry submission failure slashing amount
    ///         update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeRelayEntrySubmissionFailureSlashingAmountUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            relayEntrySubmissionFailureSlashingAmountChangeInitiated
        )
    {
        emit RelayEntrySubmissionFailureSlashingAmountUpdated(
            newRelayEntrySubmissionFailureSlashingAmount
        );
        (
            ,
            uint96 maliciousDkgResultSlashingAmount,
            uint96 unauthorizedSigningSlashingAmount
        ) = randomBeacon.slashingParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateSlashingParameters(
            newRelayEntrySubmissionFailureSlashingAmount,
            maliciousDkgResultSlashingAmount,
            unauthorizedSigningSlashingAmount
        );
        relayEntrySubmissionFailureSlashingAmountChangeInitiated = 0;
        newRelayEntrySubmissionFailureSlashingAmount = 0;
    }

    /// @notice Begins the DKG result submission gas update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultSubmissionGas New relay entry submission gas offset
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

    /// @notice Finalizes DKG result submission gas update process.
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
            uint256 relayEntrySubmissionGasOffset
        ) = randomBeacon.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGasParameters(
            newDkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            relayEntrySubmissionGasOffset
        );
        dkgResultSubmissionGasChangeInitiated = 0;
        newDkgResultSubmissionGas = 0;
    }

    /// @notice Begins the DKG result approval gas offset update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newDkgResultApprovalGasOffset New DKG approval gas offset
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

    /// @notice Finalizes the DKG result approval gas offset update process.
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
            uint256 relayEntrySubmissionGasOffset
        ) = randomBeacon.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGasParameters(
            dkgResultSubmissionGas,
            newDkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            relayEntrySubmissionGasOffset
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
        newNotifyOperatorInactivityGasOffset =
            _newNotifyOperatorInactivityGasOffset;
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
            uint256 relayEntrySubmissionGasOffset
        ) = randomBeacon.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGasParameters(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            newNotifyOperatorInactivityGasOffset,
            relayEntrySubmissionGasOffset
        );
        notifyOperatorInactivityGasOffsetChangeInitiated = 0;
        newNotifyOperatorInactivityGasOffset = 0;
    }

    /// @notice Begins the relay entry submission gas offset update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newRelayEntrySubmissionGasOffset New relay entry submission gas offset
    function beginRelayEntrySubmissionGasOffsetUpdate(
        uint256 _newRelayEntrySubmissionGasOffset
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newRelayEntrySubmissionGasOffset = _newRelayEntrySubmissionGasOffset;
        relayEntrySubmissionGasOffsetChangeInitiated = block.timestamp;
        emit RelayEntrySubmissionGasOffsetUpdateStarted(
            _newRelayEntrySubmissionGasOffset,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes relay entry submission gas offset update process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeRelayEntrySubmissionGasOffsetUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(relayEntrySubmissionGasOffsetChangeInitiated)
    {
        emit RelayEntrySubmissionGasOffsetUpdated(
            newRelayEntrySubmissionGasOffset
        );
        (
            uint256 dkgResultSubmissionGas,
            uint256 dkgResultApprovalGasOffset,
            uint256 notifyOperatorInactivityGasOffset,

        ) = randomBeacon.gasParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateGasParameters(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            newRelayEntrySubmissionGasOffset
        );
        relayEntrySubmissionGasOffsetChangeInitiated = 0;
        newRelayEntrySubmissionGasOffset = 0;
    }

    /// @notice Begins the malicious DKG result slashing amount update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newMaliciousDkgResultSlashingAmount New malicious DKG result
    ///        slashing amount
    function beginMaliciousDkgResultSlashingAmountUpdate(
        uint96 _newMaliciousDkgResultSlashingAmount
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newMaliciousDkgResultSlashingAmount =
            _newMaliciousDkgResultSlashingAmount;
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
        (
            uint96 relayEntrySubmissionFailureSlashingAmount,
            ,
            uint96 unauthorizedSigningSlashingAmount
        ) = randomBeacon.slashingParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateSlashingParameters(
            relayEntrySubmissionFailureSlashingAmount,
            newMaliciousDkgResultSlashingAmount,
            unauthorizedSigningSlashingAmount
        );
        maliciousDkgResultSlashingAmountChangeInitiated = 0;
        newMaliciousDkgResultSlashingAmount = 0;
    }

    /// @notice Begins the unauthorized signing slashing amount update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newUnauthorizedSigningSlashingAmount New unauthorized signing
    ///        slashing amount
    function beginUnauthorizedSigningSlashingAmountUpdate(
        uint96 _newUnauthorizedSigningSlashingAmount
    ) external onlyOwner {
        /* solhint-disable not-rely-on-time */
        newUnauthorizedSigningSlashingAmount =
            _newUnauthorizedSigningSlashingAmount;
        unauthorizedSigningSlashingAmountChangeInitiated = block.timestamp;
        emit UnauthorizedSigningSlashingAmountUpdateStarted(
            _newUnauthorizedSigningSlashingAmount,
            block.timestamp
        );
        /* solhint-enable not-rely-on-time */
    }

    /// @notice Finalizes the unauthorized signing slashing amount update
    ///         process.
    /// @dev Can be called only by the contract owner, after the governance
    ///      delay elapses.
    function finalizeUnauthorizedSigningSlashingAmountUpdate()
        external
        onlyOwner
        onlyAfterGovernanceDelay(
            unauthorizedSigningSlashingAmountChangeInitiated
        )
    {
        emit UnauthorizedSigningSlashingAmountUpdated(
            newUnauthorizedSigningSlashingAmount
        );
        (
            uint96 relayEntrySubmissionFailureSlashingAmount,
            uint96 maliciousDkgResultSlashingAmount,

        ) = randomBeacon.slashingParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateSlashingParameters(
            relayEntrySubmissionFailureSlashingAmount,
            maliciousDkgResultSlashingAmount,
            newUnauthorizedSigningSlashingAmount
        );
        unauthorizedSigningSlashingAmountChangeInitiated = 0;
        newUnauthorizedSigningSlashingAmount = 0;
    }

    /// @notice Begins the minimum authorization amount update process.
    /// @dev Can be called only by the contract owner.
    /// @param _newMinimumAuthorization New minimum authorization amount.
    function beginMinimumAuthorizationUpdate(
        uint96 _newMinimumAuthorization
    ) external onlyOwner {
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
        ) = randomBeacon.authorizationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateAuthorizationParameters(
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
            uint64 authorizationDecreaseChangePeriod,

        ) = randomBeacon.authorizationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateAuthorizationParameters(
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
        newAuthorizationDecreaseChangePeriod =
            _newAuthorizationDecreaseChangePeriod;
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

        ) = randomBeacon.authorizationParameters();
        // slither-disable-next-line reentrancy-no-eth
        randomBeacon.updateAuthorizationParameters(
            minimumAuthorization,
            authorizationDecreaseDelay,
            newAuthorizationDecreaseChangePeriod
        );
        authorizationDecreaseChangePeriodChangeInitiated = 0;
        newAuthorizationDecreaseChangePeriod = 0;
    }

    /// @notice Set authorization for requesters that can request a relay
    ///         entry. It can be done by the governance only.
    /// @param requester Requester, can be a contract or EOA
    /// @param isAuthorized True or false
    function setRequesterAuthorization(
        address requester,
        bool isAuthorized
    ) external onlyOwner {
        randomBeacon.setRequesterAuthorization(requester, isAuthorized);
    }

    /// @notice Withdraws rewards belonging to operators marked as ineligible
    ///         for sortition pool rewards.
    /// @dev Can be called only by the contract owner.
    /// @param recipient Recipient of withdrawn rewards.
    function withdrawIneligibleRewards(address recipient) external onlyOwner {
        randomBeacon.withdrawIneligibleRewards(recipient);
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

    /// @notice Get the time remaining until the random beacon governance can
    ///         be transferred.
    /// @return Remaining time in seconds.
    function getRemainingRandomBeaconGovernanceTransferDelayTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(randomBeaconGovernanceTransferInitiated);
    }

    /// @notice Get the time remaining until the relay entry submission soft
    ///         timeout can be updated.
    /// @return Remaining time in seconds.
    function getRemainingRelayEntrySoftTimeoutUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(relayEntrySoftTimeoutChangeInitiated);
    }

    /// @notice Get the time remaining until the relay entry hard timeout can be
    ///         updated.
    /// @return Remaining time in seconds.
    function getRemainingRelayEntryHardTimeoutUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(relayEntryHardTimeoutChangeInitiated);
    }

    /// @notice Get the time remaining until the callback gas limit can be
    ///         updated.
    /// @return Remaining time in seconds.
    function getRemainingCallbackGasLimitUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(callbackGasLimitChangeInitiated);
    }

    /// @notice Get the time remaining until the group creation frequency can be
    ///         updated.
    /// @return Remaining time in seconds.
    function getRemainingGroupCreationFrequencyUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(groupCreationFrequencyChangeInitiated);
    }

    /// @notice Get the time remaining until the group lifetime can be updated.
    /// @return Remaining time in seconds.
    function getRemainingGroupLifetimeUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(groupLifetimeChangeInitiated);
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

    /// @notice Get the time remaining until the DKG result challenge extra
    ///         gas can be updated.
    /// @return Remaining time in seconds.
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

    /// @notice Get the time remaining until the relay entry submission failure
    ///         slashing amount can be updated.
    /// @return Remaining time in seconds.
    function getRemainingRelayEntrySubmissionFailureSlashingAmountUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                relayEntrySubmissionFailureSlashingAmountChangeInitiated
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

    /// @notice Get the time remaining until the unauthorized signing
    ///         slashing amount can be updated.
    /// @return Remaining time in seconds.
    function getRemainingUnauthorizedSigningSlashingAmountUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                unauthorizedSigningSlashingAmountChangeInitiated
            );
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

    /// @notice Get the time remaining until the relay entry timeout
    ///         notification reward multiplier duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingRelayEntryTimeoutNotificationRewardMultiplierUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                relayEntryTimeoutNotificationRewardMultiplierChangeInitiated
            );
    }

    /// @notice Get the time remaining until the unauthorized signing
    ///         notification reward multiplier duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingUnauthorizedSigningNotificationRewardMultiplierUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                unauthorizedSigningNotificationRewardMultiplierChangeInitiated
            );
    }

    /// @notice Get the time remaining until the DKG malicious result
    ///         notification reward multiplier duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgMaliciousResultNotificationRewardMultiplierUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                dkgMaliciousResultNotificationRewardMultiplierChangeInitiated
            );
    }

    /// @notice Get the time remaining until the DKG result submission gas
    ///         duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingDkgResultSubmissionGasUpdateTime()
        external
        view
        returns (uint256)
    {
        return getRemainingChangeTime(dkgResultSubmissionGasChangeInitiated);
    }

    /// @notice Get the time remaining until the DKG approval gas offset duration
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

    /// @notice Get the time remaining until the operator inactivity notification
    ///         gas offset duration can be updated.
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

    /// @notice Get the time remaining until the relay entry submission gas offset
    ///         duration can be updated.
    /// @return Remaining time in seconds.
    function getRemainingRelayEntrySubmissionGasOffsetUpdateTime()
        external
        view
        returns (uint256)
    {
        return
            getRemainingChangeTime(
                relayEntrySubmissionGasOffsetChangeInitiated
            );
    }

    /// @notice Gets the time remaining until the governable parameter update
    ///         can be committed.
    /// @param changeTimestamp Timestamp indicating the beginning of the change.
    /// @return Remaining time in seconds.
    function getRemainingChangeTime(
        uint256 changeTimestamp
    ) internal view returns (uint256) {
        require(changeTimestamp > 0, "Change not initiated");
        /* solhint-disable-next-line not-rely-on-time */
        uint256 elapsed = block.timestamp - changeTimestamp;
        if (elapsed >= governanceDelay) {
            return 0;
        } else {
            return governanceDelay - elapsed;
        }
    }
}
