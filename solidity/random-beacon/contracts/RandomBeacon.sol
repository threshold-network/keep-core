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

import "./api/IRandomBeacon.sol";
import "./libraries/Groups.sol";
import "./libraries/Relay.sol";
import "./libraries/Groups.sol";
import "./libraries/Callback.sol";
import "./Reimbursable.sol";
import "./Governable.sol";
import {BeaconInactivity as Inactivity} from "./libraries/BeaconInactivity.sol";
import {BeaconAuthorization as Authorization} from "./libraries/BeaconAuthorization.sol";
import {BeaconDkg as DKG} from "./libraries/BeaconDkg.sol";
import {BeaconDkgValidator as DKGValidator} from "./BeaconDkgValidator.sol";

import "@keep-network/sortition-pools/contracts/SortitionPool.sol";
import "@threshold-network/solidity-contracts/contracts/staking/IApplication.sol";
import "@threshold-network/solidity-contracts/contracts/staking/IStaking.sol";

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/math/Math.sol";

/// @title Keep Random Beacon
/// @notice Keep Random Beacon contract. It lets to request a new
///         relay entry and validates the new relay entry provided by the
///         network. This contract is in charge of all other Random Beacon
///         activities such as group lifecycle or slashing.
/// @dev Should be owned by the governance contract controlling Random Beacon
///      parameters.
contract RandomBeacon is IRandomBeacon, IApplication, Governable, Reimbursable {
    using SafeERC20 for IERC20;
    using Authorization for Authorization.Data;
    using DKG for DKG.Data;
    using Groups for Groups.Data;
    using Relay for Relay.Data;
    using Callback for Callback.Data;

    // Constant parameters

    /// @notice Seed value used for the genesis group selection.
    /// https://www.wolframalpha.com/input/?i=pi+to+78+digits
    uint256 internal constant genesisSeed =
        31415926535897932384626433832795028841971693993751058209749445923078164062862;

    // Governable parameters

    /// @notice Relay entry callback gas limit. This is the gas limit with which
    ///         callback function provided in the relay request transaction is
    ///         executed. The callback is executed with a new relay entry value
    ///         in the same transaction the relay entry is submitted.
    uint256 internal _callbackGasLimit;

    /// @notice The frequency of new group creation. Groups are created with
    ///         a fixed frequency of relay requests.
    uint256 internal _groupCreationFrequency;

    /// @notice Slashing amount for submitting a malicious DKG result. Every
    ///         DKG result submitted can be challenged for the time of
    ///         `dkg.ResultChallengePeriodLength`. If the DKG result submitted
    ///         is challenged and proven to be malicious, the operator who
    ///         submitted the malicious result is slashed for
    ///         `_maliciousDkgResultSlashingAmount`.
    uint96 internal _maliciousDkgResultSlashingAmount;

    /// @notice Slashing amount when an unauthorized signing has been proved,
    ///         which means the private key leaked and all the group members
    ///         should be punished.
    uint96 internal _unauthorizedSigningSlashingAmount;

    /// @notice Duration of the sortition pool rewards ban imposed on operators
    ///         who misbehaved during DKG by being inactive or disqualified and
    ///         for operators that were identified by the rest of group members
    ///         as inactive via `notifyOperatorInactivity`.
    uint256 internal _sortitionPoolRewardsBanDuration;

    /// @notice Percentage of the staking contract malicious behavior
    ///         notification reward which will be transferred to the notifier
    ///         reporting about relay entry timeout. Notifiers are rewarded
    ///         from a notifiers treasury pool. For example, if
    ///         notification reward is 1000 and the value of the multiplier is
    ///         5, the notifier will receive: 5% of 1000 = 50 per each
    ///         operator affected.
    uint256 internal _relayEntryTimeoutNotificationRewardMultiplier;

    /// @notice Percentage of the staking contract malicious behavior
    ///         notification reward which will be transferred to the notifier
    ///         reporting about unauthorized signing. Notifiers are rewarded
    ///         from a notifiers treasury pool. For example, if a
    ///         notification reward is 1000 and the value of the multiplier is
    ///         5, the notifier will receive: 5% of 1000 = 50 per each
    ///         operator affected.
    uint256 internal _unauthorizedSigningNotificationRewardMultiplier;

    /// @notice Percentage of the staking contract malicious behavior
    ///         notification reward which will be transferred to the notifier
    ///         reporting about a malicious DKG result. Notifiers are rewarded
    ///         from a notifiers treasury pool. For example, if
    ///         notification reward is 1000 and the value of the multiplier is
    ///         5, the notifier will receive: 5% of 1000 = 50 per each
    ///         operator affected.
    uint256 internal _dkgMaliciousResultNotificationRewardMultiplier;

    /// @notice Calculated gas cost for submitting a DKG result. This will
    ///         be refunded as part of the DKG approval process. It is in the
    ///         submitter's interest to not skip his priority turn on the approval,
    ///         otherwise the refund of the DKG submission will be refunded to
    ///         another group member that will call the DKG approve function.
    uint256 internal _dkgResultSubmissionGas;

    /// @notice Gas that is meant to balance the DKG result approval's overall
    ///         cost. Can be updated by the governance based on the current
    ///         market conditions.
    uint256 internal _dkgResultApprovalGasOffset;

    /// @notice Gas that is meant to balance the operator inactivity notification
    ///         cost. Can be updated by the governance based on the current
    ///         market conditions.
    uint256 internal _notifyOperatorInactivityGasOffset;

    /// @notice Gas that is meant to balance the relay entry submission cost.
    ///         Can be updated by the governance based on the current market
    ///         conditions.
    uint256 internal _relayEntrySubmissionGasOffset;

    // Other parameters

    /// @notice Stores current operator inactivity claim nonce for given group.
    ///         Each claim is made with an unique nonce which protects
    ///         against claim replay.
    mapping(uint64 => uint256) public inactivityClaimNonce; // groupId -> nonce

    /// @notice Authorized addresses that can request a relay entry.
    mapping(address => bool) public authorizedRequesters;

    // External dependencies

    SortitionPool public sortitionPool;
    IERC20 public tToken;
    IStaking public staking;

    // Libraries data storages

    Authorization.Data internal authorization;
    DKG.Data internal dkg;
    Groups.Data internal groups;
    Relay.Data internal relay;
    Callback.Data internal callback;

    // Events

    event AuthorizationParametersUpdated(
        uint96 minimumAuthorization,
        uint64 authorizationDecreaseDelay,
        uint64 authorizationDecreaseChangePeriod
    );

    event RelayEntryParametersUpdated(
        uint256 relayEntrySoftTimeout,
        uint256 relayEntryHardTimeout,
        uint256 callbackGasLimit
    );

    event GroupCreationParametersUpdated(
        uint256 groupCreationFrequency,
        uint256 groupLifetime,
        uint256 dkgResultChallengePeriodLength,
        uint256 dkgResultChallengeExtraGas,
        uint256 dkgResultSubmissionTimeout,
        uint256 dkgResultSubmitterPrecedencePeriodLength
    );

    event RewardParametersUpdated(
        uint256 sortitionPoolRewardsBanDuration,
        uint256 relayEntryTimeoutNotificationRewardMultiplier,
        uint256 unauthorizedSigningNotificationRewardMultiplier,
        uint256 dkgMaliciousResultNotificationRewardMultiplier
    );

    event SlashingParametersUpdated(
        uint256 relayEntrySubmissionFailureSlashingAmount,
        uint256 maliciousDkgResultSlashingAmount,
        uint256 unauthorizedSigningSlashingAmount
    );

    event GasParametersUpdated(
        uint256 dkgResultSubmissionGas,
        uint256 dkgResultApprovalGasOffset,
        uint256 notifyOperatorInactivityGasOffset,
        uint256 relayEntrySubmissionGasOffset
    );

    event RequesterAuthorizationUpdated(
        address indexed requester,
        bool isAuthorized
    );

    event DkgStarted(uint256 indexed seed);

    event DkgResultSubmitted(
        bytes32 indexed resultHash,
        uint256 indexed seed,
        DKG.Result result
    );

    event DkgTimedOut();

    event DkgResultApproved(
        bytes32 indexed resultHash,
        address indexed approver
    );

    event DkgResultChallenged(
        bytes32 indexed resultHash,
        address indexed challenger,
        string reason
    );

    event DkgMaliciousResultSlashed(
        bytes32 indexed resultHash,
        uint256 slashingAmount,
        address maliciousSubmitter
    );

    event DkgMaliciousResultSlashingFailed(
        bytes32 indexed resultHash,
        uint256 slashingAmount,
        address maliciousSubmitter
    );

    event DkgStateLocked();

    event DkgSeedTimedOut();

    event GroupRegistered(uint64 indexed groupId, bytes indexed groupPubKey);

    event RelayEntryRequested(
        uint256 indexed requestId,
        uint64 groupId,
        bytes previousEntry
    );

    event RelayEntrySubmitted(
        uint256 indexed requestId,
        address submitter,
        bytes entry
    );

    event RelayEntryTimedOut(
        uint256 indexed requestId,
        uint64 terminatedGroupId
    );

    event RelayEntryDelaySlashed(
        uint256 indexed requestId,
        uint256 slashingAmount,
        address[] groupMembers
    );

    event RelayEntryDelaySlashingFailed(
        uint256 indexed requestId,
        uint256 slashingAmount,
        address[] groupMembers
    );

    event RelayEntryTimeoutSlashed(
        uint256 indexed requestId,
        uint256 slashingAmount,
        address[] groupMembers
    );

    event RelayEntryTimeoutSlashingFailed(
        uint256 indexed requestId,
        uint256 slashingAmount,
        address[] groupMembers
    );

    event UnauthorizedSigningSlashed(
        uint64 indexed groupId,
        uint256 unauthorizedSigningSlashingAmount,
        address[] groupMembers
    );

    event UnauthorizedSigningSlashingFailed(
        uint64 indexed groupId,
        uint256 unauthorizedSigningSlashingAmount,
        address[] groupMembers
    );

    event CallbackFailed(uint256 entry, uint256 entrySubmittedBlock);

    event InactivityClaimed(
        uint64 indexed groupId,
        uint256 nonce,
        address notifier
    );

    event OperatorRegistered(
        address indexed stakingProvider,
        address indexed operator
    );

    event AuthorizationIncreased(
        address indexed stakingProvider,
        address indexed operator,
        uint96 fromAmount,
        uint96 toAmount
    );

    event AuthorizationDecreaseRequested(
        address indexed stakingProvider,
        address indexed operator,
        uint96 fromAmount,
        uint96 toAmount,
        uint64 decreasingAt
    );

    event AuthorizationDecreaseApproved(address indexed stakingProvider);

    event InvoluntaryAuthorizationDecreaseFailed(
        address indexed stakingProvider,
        address indexed operator,
        uint96 fromAmount,
        uint96 toAmount
    );

    event OperatorJoinedSortitionPool(
        address indexed stakingProvider,
        address indexed operator
    );

    event OperatorStatusUpdated(
        address indexed stakingProvider,
        address indexed operator
    );

    /// @dev Assigns initial values to parameters to make the beacon work
    ///      safely. These parameters are just proposed defaults and they might
    ///      be updated with `update*` functions after the contract deployment
    ///      and before transferring the ownership to the governance contract.
    constructor(
        SortitionPool _sortitionPool,
        IERC20 _tToken,
        IStaking _staking,
        DKGValidator _dkgValidator,
        ReimbursementPool _reimbursementPool
    ) {
        sortitionPool = _sortitionPool;
        tToken = _tToken;
        staking = _staking;
        reimbursementPool = _reimbursementPool;

        require(
            address(_sortitionPool) != address(0),
            "Zero-address reference"
        );
        require(address(_tToken) != address(0), "Zero-address reference");
        require(address(_staking) != address(0), "Zero-address reference");
        require(address(_dkgValidator) != address(0), "Zero-address reference");
        require(
            address(_reimbursementPool) != address(0),
            "Zero-address reference"
        );

        dkg.init(_sortitionPool, _dkgValidator);
        relay.initSeedEntry();

        _transferGovernance(msg.sender);

        //
        // All parameters set in the constructor are initial ones, used at the
        // moment contracts were deployed for the first time. Parameters are
        // governable and values assigned in the constructor do not need to
        // reflect the current ones.
        //

        // Minimum authorization is 40k T.
        //
        // Authorization decrease delay is 45 days.
        //
        // Authorization decrease change period is 45 days. It means pending
        // authorization decrease can be overwritten all the time.
        authorization.setParameters(40_000e18, 3_888_000, 3_888_000);

        // Malicious DKG result slashing amount is set initially to 1% of the
        // minimum authorization (400 T). This values needs to be increased
        // significantly once the system is fully launched.
        //
        // Unauthorized signing slashing amount is set initially to 1% of the
        // minimum authorization (400 T). This values needs to be increased
        // significantly once the system is fully launched.
        //
        // Slashing amount for not providing relay entry on time is set
        // initially to 1% of the minimum authorization (400 T). This values
        // needs to be increased significantly once the system is fully launched.
        //
        // Inactive operators are set as ineligible for rewards for 2 weeks.
        _maliciousDkgResultSlashingAmount = 400e18;
        _unauthorizedSigningSlashingAmount = 400e18;
        relay.setRelayEntrySubmissionFailureSlashingAmount(400e18);

        // Notifier of a malicious DKG result receives 100% of the notifier
        // reward from the staking contract.
        //
        // Notifier of unauthorized signing receives 100% of the notifier
        // reward from the staking contract.
        //
        // Notifier of relay entry timeout receives 100% of the notifier
        // reward from the staking contract.
        _dkgMaliciousResultNotificationRewardMultiplier = 100;
        _unauthorizedSigningNotificationRewardMultiplier = 100;
        _relayEntryTimeoutNotificationRewardMultiplier = 100;

        // Inactive operators are set as ineligible for rewards for 2 weeks.
        _sortitionPoolRewardsBanDuration = 2 weeks;

        // DKG result challenge period length is set to 48h, assuming
        // 15s block time.
        //
        // The extra gas required for DKG result challenge is 50k units.
        //
        // DKG result submission timeout, gives each member 20 blocks to submit
        // the result. Assuming 15s block time, it is ~8h to submit the result
        // in the pessimistic case.
        //
        // The original DKG result submitter has 20 blocks to approve it before
        // anyone else can do that.
        //
        // With these parameters, the happy path takes no more than 56 hours.
        // In practice, it should take about 48 hours (just the challenge time).
        dkg.setParameters(11_520, 50_000, 1_280, 20);

        // Relay entry soft timeout gives each of 64 members 20 blocks to submit
        // the result.
        //
        // Relay entry hard timeout is set to ~48h assuming 15s block time.
        relay.setTimeouts(1_280, 5_760);

        // Callback gas limit is set to 56k units of gas. As of April 2022, it
        // is enough to store new entry and block number on-chain.
        // If the cost of EVM opcodes change over time, these parameters will
        // have to be updated.
        _callbackGasLimit = 64_000;

        // Group lifetime is set to 45 days assuming 15s block time.
        //
        // New group is created every 2 relay requests.
        //
        // This way, even if ECDSA WalletRegistry is the only consumer of the
        // beacon initially, and relay request is executed every week, we should
        // have 2 active groups in the system all the time.
        groups.setGroupLifetime(259_200);
        _groupCreationFrequency = 2;

        // Gas parameters were adjusted based on Ethereum state in April 2022.
        // If the cost of EVM opcodes change over time, these parameters will
        // have to be updated.
        _dkgResultSubmissionGas = 237_650;
        _dkgResultApprovalGasOffset = 41_500;
        _notifyOperatorInactivityGasOffset = 54_500;
        _relayEntrySubmissionGasOffset = 11_250;
    }

    modifier onlyStakingContract() {
        require(
            msg.sender == address(staking),
            "Caller is not the staking contract"
        );
        _;
    }

    modifier onlyReimbursableAdmin() override {
        require(governance == msg.sender, "Caller is not the governance");
        _;
    }

    /// @notice Updates the values of authorization parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param _minimumAuthorization New minimum authorization amount
    /// @param _authorizationDecreaseDelay New authorization decrease delay in
    ///        seconds
    /// @param _authorizationDecreaseChangePeriod New authorization decrease
    ///        change period in seconds
    function updateAuthorizationParameters(
        uint96 _minimumAuthorization,
        uint64 _authorizationDecreaseDelay,
        uint64 _authorizationDecreaseChangePeriod
    ) external onlyGovernance {
        authorization.setParameters(
            _minimumAuthorization,
            _authorizationDecreaseDelay,
            _authorizationDecreaseChangePeriod
        );

        emit AuthorizationParametersUpdated(
            _minimumAuthorization,
            _authorizationDecreaseDelay,
            _authorizationDecreaseChangePeriod
        );
    }

    /// @notice Updates the values of relay entry parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param relayEntrySoftTimeout New relay entry submission soft timeout
    /// @param relayEntryHardTimeout New relay entry hard timeout
    /// @param callbackGasLimit New callback gas limit
    function updateRelayEntryParameters(
        uint256 relayEntrySoftTimeout,
        uint256 relayEntryHardTimeout,
        uint256 callbackGasLimit
    ) external onlyGovernance {
        _callbackGasLimit = callbackGasLimit;
        relay.setTimeouts(relayEntrySoftTimeout, relayEntryHardTimeout);

        emit RelayEntryParametersUpdated(
            relayEntrySoftTimeout,
            relayEntryHardTimeout,
            callbackGasLimit
        );
    }

    /// @notice Updates the values of group creation parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param groupCreationFrequency New group creation frequency
    /// @param groupLifetime New group lifetime in blocks
    /// @param dkgResultChallengePeriodLength New DKG result challenge period
    ///        length
    /// @param dkgResultChallengeExtraGas New DKG result challenge extra gas
    /// @param dkgResultSubmissionTimeout New DKG result submission timeout
    /// @param dkgSubmitterPrecedencePeriodLength New DKG result submitter
    ///        precedence period length
    function updateGroupCreationParameters(
        uint256 groupCreationFrequency,
        uint256 groupLifetime,
        uint256 dkgResultChallengePeriodLength,
        uint256 dkgResultChallengeExtraGas,
        uint256 dkgResultSubmissionTimeout,
        uint256 dkgSubmitterPrecedencePeriodLength
    ) external onlyGovernance {
        _groupCreationFrequency = groupCreationFrequency;
        groups.setGroupLifetime(groupLifetime);
        dkg.setParameters(
            dkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
        );

        emit GroupCreationParametersUpdated(
            groupCreationFrequency,
            groupLifetime,
            dkgResultChallengePeriodLength,
            dkgResultChallengeExtraGas,
            dkgResultSubmissionTimeout,
            dkgSubmitterPrecedencePeriodLength
        );
    }

    /// @notice Updates the values of reward parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param sortitionPoolRewardsBanDuration New sortition pool rewards
    ///        ban duration in seconds.
    /// @param relayEntryTimeoutNotificationRewardMultiplier New value of the
    ///        relay entry timeout notification reward multiplier
    /// @param unauthorizedSigningNotificationRewardMultiplier New value of the
    ///        unauthorized signing notification reward multiplier
    /// @param dkgMaliciousResultNotificationRewardMultiplier New value of the
    ///        DKG malicious result notification reward multiplier
    function updateRewardParameters(
        uint256 sortitionPoolRewardsBanDuration,
        uint256 relayEntryTimeoutNotificationRewardMultiplier,
        uint256 unauthorizedSigningNotificationRewardMultiplier,
        uint256 dkgMaliciousResultNotificationRewardMultiplier
    ) external onlyGovernance {
        _sortitionPoolRewardsBanDuration = sortitionPoolRewardsBanDuration;
        _relayEntryTimeoutNotificationRewardMultiplier =
            relayEntryTimeoutNotificationRewardMultiplier;
        _unauthorizedSigningNotificationRewardMultiplier =
            unauthorizedSigningNotificationRewardMultiplier;
        _dkgMaliciousResultNotificationRewardMultiplier =
            dkgMaliciousResultNotificationRewardMultiplier;
        emit RewardParametersUpdated(
            sortitionPoolRewardsBanDuration,
            relayEntryTimeoutNotificationRewardMultiplier,
            unauthorizedSigningNotificationRewardMultiplier,
            dkgMaliciousResultNotificationRewardMultiplier
        );
    }

    /// @notice Updates the values of slashing parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param relayEntrySubmissionFailureSlashingAmount New relay entry
    ///        submission failure amount
    /// @param maliciousDkgResultSlashingAmount New malicious DKG result
    ///        slashing amount
    /// @param unauthorizedSigningSlashingAmount New unauthorized signing
    ///        slashing amount
    function updateSlashingParameters(
        uint96 relayEntrySubmissionFailureSlashingAmount,
        uint96 maliciousDkgResultSlashingAmount,
        uint96 unauthorizedSigningSlashingAmount
    ) external onlyGovernance {
        relay.setRelayEntrySubmissionFailureSlashingAmount(
            relayEntrySubmissionFailureSlashingAmount
        );
        _maliciousDkgResultSlashingAmount = maliciousDkgResultSlashingAmount;
        _unauthorizedSigningSlashingAmount = unauthorizedSigningSlashingAmount;
        emit SlashingParametersUpdated(
            relayEntrySubmissionFailureSlashingAmount,
            maliciousDkgResultSlashingAmount,
            unauthorizedSigningSlashingAmount
        );
    }

    /// @notice Updates the values of gas parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param dkgResultSubmissionGas New DKG result submission gas
    /// @param dkgResultApprovalGasOffset New DKG result approval gas offset
    /// @param notifyOperatorInactivityGasOffset New operator inactivity
    ///        notification gas offset
    /// @param relayEntrySubmissionGasOffset New relay entry submission gas
    ///        offset
    function updateGasParameters(
        uint256 dkgResultSubmissionGas,
        uint256 dkgResultApprovalGasOffset,
        uint256 notifyOperatorInactivityGasOffset,
        uint256 relayEntrySubmissionGasOffset
    ) external onlyGovernance {
        _dkgResultSubmissionGas = dkgResultSubmissionGas;
        _dkgResultApprovalGasOffset = dkgResultApprovalGasOffset;
        _notifyOperatorInactivityGasOffset = notifyOperatorInactivityGasOffset;
        _relayEntrySubmissionGasOffset = relayEntrySubmissionGasOffset;

        emit GasParametersUpdated(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            relayEntrySubmissionGasOffset
        );
    }

    /// @notice Set authorization for requesters that can request a relay
    ///         entry.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract.
    /// @param requester Requester, can be a contract or EOA
    /// @param isAuthorized True or false
    function setRequesterAuthorization(
        address requester,
        bool isAuthorized
    ) external onlyGovernance {
        authorizedRequesters[requester] = isAuthorized;

        emit RequesterAuthorizationUpdated(requester, isAuthorized);
    }

    /// @notice Withdraws application rewards for the given staking provider.
    ///         Rewards are withdrawn to the staking provider's beneficiary
    ///         address set in the staking contract. Reverts if staking provider
    ///         has not registered the operator address.
    /// @dev Emits `RewardsWithdrawn` event.
    function withdrawRewards(address stakingProvider) external {
        address operator = stakingProviderToOperator(stakingProvider);
        require(operator != address(0), "Unknown operator");
        (, address beneficiary, ) = staking.rolesOf(stakingProvider);
        uint96 amount = sortitionPool.withdrawRewards(operator, beneficiary);
        // slither-disable-next-line reentrancy-events
        emit RewardsWithdrawn(stakingProvider, amount);
    }

    /// @notice Withdraws rewards belonging to operators marked as ineligible
    ///         for sortition pool rewards.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      random beacon governance contract.
    /// @param recipient Recipient of withdrawn rewards.
    function withdrawIneligibleRewards(
        address recipient
    ) external onlyGovernance {
        sortitionPool.withdrawIneligible(recipient);
    }

    /// @notice Used by staking provider to set operator address that will
    ///         operate a node. The given staking provider can set operator
    ///         address only one time. The operator address can not be changed
    ///         and must be unique. Reverts if the operator is already set for
    ///         the staking provider or if the operator address is already in
    ///         use. Reverts if there is a pending authorization decrease for
    ///         the staking provider.
    function registerOperator(address operator) external {
        authorization.registerOperator(operator);
    }

    /// @notice Lets the operator join the sortition pool. The operator address
    ///         must be known - before calling this function, it has to be
    ///         appointed by the staking provider by calling `registerOperator`.
    ///         Also, the operator must have the minimum authorization required
    ///         by the beacon. Function reverts if there is no minimum stake
    ///         authorized or if the operator is not known. If there was an
    ///         authorization decrease requested, it is activated by starting
    ///         the authorization decrease delay.
    function joinSortitionPool() external {
        authorization.joinSortitionPool(staking, sortitionPool);
    }

    /// @notice Updates status of the operator in the sortition pool. If there
    ///         was an authorization decrease requested, it is activated by
    ///         starting the authorization decrease delay.
    ///         Function reverts if the operator is not known.
    function updateOperatorStatus(address operator) external {
        authorization.updateOperatorStatus(staking, sortitionPool, operator);
    }

    /// @notice Used by T staking contract to inform the beacon that the
    ///         authorized stake amount for the given staking provider increased.
    ///
    ///         Reverts if the authorization amount is below the minimum.
    ///
    ///         The function is not updating the sortition pool. Sortition pool
    ///         state needs to be updated by the operator with a call to
    ///         `joinSortitionPool` or `updateOperatorStatus`.
    ///
    /// @dev Can only be called by T staking contract.
    function authorizationIncreased(
        address stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    ) external onlyStakingContract {
        authorization.authorizationIncreased(
            stakingProvider,
            fromAmount,
            toAmount
        );
    }

    /// @notice Used by T staking contract to inform the beacon that the
    ///         authorization decrease for the given staking provider has been
    ///         requested.
    ///
    ///         Reverts if the amount after deauthorization would be non-zero
    ///         and lower than the minimum authorization.
    ///
    ///         Reverts if another authorization decrease request is pending for
    ///         the staking provider and not enough time passed since the
    ///         original request (see `authorizationDecreaseChangePeriod`).
    ///
    ///         If the operator is not known (`registerOperator` was not called)
    ///         it lets to `approveAuthorizationDecrease` immediately. If the
    ///         operator is known (`registerOperator` was called), the operator
    ///         needs to update state of the sortition pool with a call to
    ///         `joinSortitionPool` or `updateOperatorStatus`. After the
    ///         sortition pool state is in sync, authorization decrease delay
    ///         starts.
    ///
    ///         After authorization decrease delay passes, authorization
    ///         decrease request needs to be approved with a call to
    ///         `approveAuthorizationDecrease` function.
    ///
    ///         If there is a pending authorization decrease request, it is
    ///         overwritten, but only if enough time passed since the original
    ///         request. Otherwise, the function reverts.
    ///
    /// @dev Can only be called by T staking contract.
    function authorizationDecreaseRequested(
        address stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    ) external onlyStakingContract {
        authorization.authorizationDecreaseRequested(
            stakingProvider,
            fromAmount,
            toAmount
        );
    }

    /// @notice Approves the previously registered authorization decrease
    ///         request. Reverts if authorization decrease delay has not passed
    ///         yet or if the authorization decrease was not requested for the
    ///         given staking provider.
    function approveAuthorizationDecrease(address stakingProvider) external {
        authorization.approveAuthorizationDecrease(staking, stakingProvider);
    }

    /// @notice Used by T staking contract to inform the beacon the
    ///         authorization has been decreased for the given staking provider
    ///         involuntarily, as a result of slashing.
    ///
    ///         If the operator is not known (`registerOperator` was not called)
    ///         the function does nothing. The operator was never in a sortition
    ///         pool so there is nothing to update.
    ///
    ///         If the operator is known, sortition pool is unlocked, and the
    ///         operator is in the sortition pool, the sortition pool state is
    ///         updated. If the sortition pool is locked, update needs to be
    ///         postponed. Every other staker is incentivized to call
    ///         `updateOperatorStatus` for the problematic operator to increase
    ///         their own rewards in the pool.
    function involuntaryAuthorizationDecrease(
        address stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    ) external onlyStakingContract {
        authorization.involuntaryAuthorizationDecrease(
            staking,
            sortitionPool,
            stakingProvider,
            fromAmount,
            toAmount
        );
    }

    /// @notice Triggers group selection if there are no active groups.
    function genesis() external {
        require(groups.numberOfActiveGroups() == 0, "Not awaiting genesis");

        dkg.lockState();
        dkg.start(
            uint256(keccak256(abi.encodePacked(genesisSeed, block.number)))
        );
    }

    /// @notice Submits result of DKG protocol. It is on-chain part of phase 14 of
    ///         the protocol. The DKG result consists of result submitting member
    ///         index, calculated group public key, bytes array of misbehaved
    ///         members, concatenation of signatures from group members,
    ///         indices of members corresponding to each signature and
    ///         the list of group members.
    ///         When the result is verified successfully it gets registered and
    ///         waits for an approval. A result can be challenged to verify the
    ///         members list corresponds to the expected set of members determined
    ///         by the sortition pool.
    ///         A candidate group is registered based on the submitted DKG result
    ///         details.
    /// @dev The message to be signed by each member is keccak256 hash of the
    ///      chain ID, calculated group public key, misbehaved members as bytes
    ///      and DKG start block. The calculated hash should be prefixed with
    //       prefixed with
    ///      `\x19Ethereum signed message:\n` before signing, so the message to
    ///      sign is:
    ///      `\x19Ethereum signed message:\n${keccak256(chainID,groupPubKey,misbehaved,startBlock)}`
    /// @param dkgResult DKG result.
    function submitDkgResult(DKG.Result calldata dkgResult) external {
        groups.validatePublicKey(dkgResult.groupPubKey);
        dkg.submitResult(dkgResult);
    }

    /// @notice Notifies about DKG timeout.
    function notifyDkgTimeout() external refundable(msg.sender) {
        dkg.notifyTimeout();
    }

    /// @notice Approves DKG result. Can be called when the challenge period for
    ///         the submitted result is finished. Considers the submitted result
    ///         as valid, bans misbehaved group members from the sortition pool
    ///         rewards, and completes the group creation by activating the
    ///         candidate group. For the first `submitterPrecedencePeriodLength`
    ///         blocks after the end of the challenge period can be called only
    ///         by the DKG result submitter. After that time, can be called by
    ///         anyone.
    /// @param dkgResult Result to approve. Must match the submitted result
    ///        stored during `submitDkgResult`.
    function approveDkgResult(DKG.Result calldata dkgResult) external {
        uint256 gasStart = gasleft();

        uint32[] memory misbehavedMembers = dkg.approveResult(dkgResult);

        if (misbehavedMembers.length > 0) {
            sortitionPool.setRewardIneligibility(
                misbehavedMembers,
                // solhint-disable-next-line not-rely-on-time
                block.timestamp + _sortitionPoolRewardsBanDuration
            );
        }

        groups.addGroup(dkgResult.groupPubKey, dkgResult.membersHash);
        dkg.complete();

        // Refund msg.sender's ETH for DKG result submission and result approval
        reimbursementPool.refund(
            _dkgResultSubmissionGas +
                (gasStart - gasleft()) +
                _dkgResultApprovalGasOffset,
            msg.sender
        );
    }

    /// @notice Challenges DKG result. If the submitted result is proved to be
    ///         invalid it reverts the DKG back to the result submission phase.
    ///         It removes a candidate group that was previously registered with
    ///         the DKG result submission.
    /// @param dkgResult Result to challenge. Must match the submitted result
    ///        stored during `submitDkgResult`.
    /// @dev Due to EIP-150 1/64 of the gas is not forwarded to the call, and
    ///      will be kept to execute the remaining operations in the function
    ///      after the call inside the try-catch. To eliminate a class of
    ///      attacks related to the gas limit manipulation, this function
    ///      requires an extra amount of gas to be left at the end of the
    ///      execution.
    function challengeDkgResult(DKG.Result calldata dkgResult) external {
        (bytes32 maliciousResultHash, uint32 maliciousSubmitter) = dkg
            .challengeResult(dkgResult);

        uint96 slashingAmount = _maliciousDkgResultSlashingAmount;
        address maliciousSubmitterAddresses = sortitionPool.getIDOperator(
            maliciousSubmitter
        );

        address[] memory stakingProviderWrapper = new address[](1);
        stakingProviderWrapper[0] = operatorToStakingProvider(
            maliciousSubmitterAddresses
        );
        try
            staking.seize(
                slashingAmount,
                _dkgMaliciousResultNotificationRewardMultiplier,
                msg.sender,
                stakingProviderWrapper
            )
        {
            // slither-disable-next-line reentrancy-events
            emit DkgMaliciousResultSlashed(
                maliciousResultHash,
                slashingAmount,
                maliciousSubmitterAddresses
            );
        } catch {
            // Should never happen but we want to ensure a non-critical path
            // failure from an external contract does not stop the challenge
            // to complete.
            emit DkgMaliciousResultSlashingFailed(
                maliciousResultHash,
                slashingAmount,
                maliciousSubmitterAddresses
            );
        }

        // Due to EIP-150, 1/64 of the gas is not forwarded to the call, and
        // will be kept to execute the remaining operations in the function
        // after the call inside the try-catch.
        //
        // To ensure there is no way for the caller to manipulate gas limit in
        // such a way that the call inside try-catch fails with out-of-gas and
        // the rest of the function is executed with the remaining 1/64 of gas,
        // we require an extra gas amount to be left at the end of the call to
        // `challengeDkgResult`.
        dkg.requireChallengeExtraGas();
    }

    /// @notice Check current group creation state.
    function getGroupCreationState() external view returns (DKG.State) {
        return dkg.currentState();
    }

    /// @notice Checks if DKG timed out. The DKG timeout period includes time required
    ///         for off-chain protocol execution and time for the result publication
    ///         for all group members. After this time result cannot be submitted
    ///         and DKG can be notified about the timeout.
    /// @return True if DKG timed out, false otherwise.
    function hasDkgTimedOut() external view returns (bool) {
        return dkg.hasDkgTimedOut();
    }

    function getGroupsRegistry() external view returns (bytes32[] memory) {
        return groups.groupsRegistry;
    }

    function getGroup(
        uint64 groupId
    ) external view returns (Groups.Group memory) {
        return groups.getGroup(groupId);
    }

    function getGroup(
        bytes memory groupPubKey
    ) external view returns (Groups.Group memory) {
        return groups.getGroup(groupPubKey);
    }

    /// @notice Creates a request to generate a new relay entry, which will
    ///         include a random number (by signing the previous entry's
    ///         random number). Requester must be previously authorized by the
    ///         governance.
    /// @param callbackContract Beacon consumer callback contract.
    function requestRelayEntry(
        IRandomBeaconConsumer callbackContract
    ) external {
        require(
            authorizedRequesters[msg.sender],
            "Requester must be authorized"
        );

        uint64 groupId = groups.selectGroup(
            uint256(keccak256(AltBn128.g1Marshal(relay.previousEntry)))
        );

        relay.requestEntry(groupId);

        callback.setCallbackContract(callbackContract);

        // If the current request should trigger group creation we need to lock
        // DKG state (lock sortition pool) to prevent operators from changing
        // its state before relay entry is known. That entry will be used as a
        // group selection seed.
        if (
            relay.requestCount % _groupCreationFrequency == 0 &&
            dkg.currentState() == DKG.State.IDLE
        ) {
            dkg.lockState();
        }
    }

    /// @notice Creates a new relay entry. Gas-optimized version that can be
    ///         called only before the soft timeout. This should be the majority
    ///         of cases.
    /// @param entry Group BLS signature over the previous entry.
    function submitRelayEntry(bytes calldata entry) external {
        uint256 gasStart = gasleft();

        Groups.Group storage group = groups.getGroup(
            relay.currentRequestGroupID
        );

        relay.submitEntryBeforeSoftTimeout(entry, group.groupPubKey);

        // If DKG is awaiting a seed, that means the we should start the actual
        // group creation process.
        if (dkg.currentState() == DKG.State.AWAITING_SEED) {
            dkg.start(uint256(keccak256(entry)));
        }

        callback.executeCallback(uint256(keccak256(entry)), _callbackGasLimit);

        reimbursementPool.refund(
            (gasStart - gasleft()) + _relayEntrySubmissionGasOffset,
            msg.sender
        );
    }

    /// @notice Creates a new relay entry.
    /// @param entry Group BLS signature over the previous entry.
    /// @param groupMembers Identifiers of group members.
    function submitRelayEntry(
        bytes calldata entry,
        uint32[] calldata groupMembers
    ) external {
        uint256 gasStart = gasleft();
        uint256 currentRequestId = relay.currentRequestID;

        Groups.Group storage group = groups.getGroup(
            relay.currentRequestGroupID
        );

        require(
            group.membersHash == keccak256(abi.encode(groupMembers)),
            "Invalid group members"
        );

        uint96 slashingAmount = relay.submitEntry(entry, group.groupPubKey);

        if (slashingAmount > 0) {
            address[] memory groupMembersAddresses = sortitionPool
                .getIDOperators(groupMembers);

            address[] memory stakingProvidersAddresses = new address[](
                groupMembersAddresses.length
            );
            for (uint256 i = 0; i < groupMembersAddresses.length; i++) {
                stakingProvidersAddresses[i] = operatorToStakingProvider(
                    groupMembersAddresses[i]
                );
            }

            try staking.slash(slashingAmount, stakingProvidersAddresses) {
                // slither-disable-next-line reentrancy-events
                emit RelayEntryDelaySlashed(
                    currentRequestId,
                    slashingAmount,
                    groupMembersAddresses
                );
            } catch {
                // Should never happen but we want to ensure a non-critical path
                // failure from an external contract does not stop group members
                // from submitting a valid relay entry.
                emit RelayEntryDelaySlashingFailed(
                    currentRequestId,
                    slashingAmount,
                    groupMembersAddresses
                );
            }
        }

        // If DKG is awaiting a seed, that means the we should start the actual
        // group creation process.
        if (dkg.currentState() == DKG.State.AWAITING_SEED) {
            dkg.start(uint256(keccak256(entry)));
        }

        callback.executeCallback(uint256(keccak256(entry)), _callbackGasLimit);
        reimbursementPool.refund(
            (gasStart - gasleft()) + _relayEntrySubmissionGasOffset,
            msg.sender
        );
    }

    /// @notice Reports a relay entry timeout.
    /// @param groupMembers Identifiers of group members.
    function reportRelayEntryTimeout(uint32[] calldata groupMembers) external {
        uint64 groupId = relay.currentRequestGroupID;
        Groups.Group storage group = groups.getGroup(groupId);

        require(
            group.membersHash == keccak256(abi.encode(groupMembers)),
            "Invalid group members"
        );

        uint96 slashingAmount = relay.relayEntrySubmissionFailureSlashingAmount;
        address[] memory groupMembersAddresses = sortitionPool.getIDOperators(
            groupMembers
        );

        address[] memory stakingProvidersAddresses = new address[](
            groupMembersAddresses.length
        );
        for (uint256 i = 0; i < groupMembersAddresses.length; i++) {
            stakingProvidersAddresses[i] = operatorToStakingProvider(
                groupMembersAddresses[i]
            );
        }

        try
            staking.seize(
                slashingAmount,
                _relayEntryTimeoutNotificationRewardMultiplier,
                msg.sender,
                stakingProvidersAddresses
            )
        {
            // slither-disable-next-line reentrancy-events
            emit RelayEntryTimeoutSlashed(
                relay.currentRequestID,
                slashingAmount,
                groupMembersAddresses
            );
        } catch {
            // Should never happen but we want to ensure a non-critical path
            // failure from an external contract does not stop the challenge
            // to complete.
            emit RelayEntryTimeoutSlashingFailed(
                relay.currentRequestID,
                slashingAmount,
                groupMembersAddresses
            );
        }

        groups.terminateGroup(groupId);
        groups.expireOldGroups();

        if (groups.numberOfActiveGroups() > 0) {
            groupId = groups.selectGroup(
                uint256(keccak256(AltBn128.g1Marshal(relay.previousEntry)))
            );
            relay.retryOnEntryTimeout(groupId);
        } else {
            relay.cleanupOnEntryTimeout();

            // If DKG is awaiting a seed, we should notify about its timeout to
            // avoid blocking the future group creation.
            if (dkg.currentState() == DKG.State.AWAITING_SEED) {
                dkg.notifySeedTimedOut();
            }
        }
    }

    /// @notice Reports unauthorized groups signing. Must provide a valid signature
    ///         of the sender's address as a message. Successful signature
    ///         verification means the private key has been leaked and all group
    ///         members should be punished by slashing their tokens. Group has
    ///         to be active or expired. Unauthorized signing cannot be reported
    ///         for a terminated group. In case of reporting unauthorized
    ///         signing for a terminated group, or when the signature is invalid,
    ///         function reverts.
    /// @param signedMsgSender Signature of the sender's address as a message.
    /// @param groupId Group that is being reported for leaking a private key.
    /// @param groupMembers Identifiers of group members.
    function reportUnauthorizedSigning(
        bytes memory signedMsgSender,
        uint64 groupId,
        uint32[] calldata groupMembers
    ) external {
        Groups.Group memory group = groups.getGroup(groupId);

        require(
            group.membersHash == keccak256(abi.encode(groupMembers)),
            "Invalid group members"
        );

        require(!group.terminated, "Group cannot be terminated");

        require(
            BLS.verifyBytes(
                group.groupPubKey,
                abi.encodePacked(msg.sender),
                signedMsgSender
            ),
            "Invalid signature"
        );

        groups.terminateGroup(groupId);

        address[] memory groupMembersAddresses = sortitionPool.getIDOperators(
            groupMembers
        );

        address[] memory stakingProvidersAddresses = new address[](
            groupMembersAddresses.length
        );
        for (uint256 i = 0; i < groupMembersAddresses.length; i++) {
            stakingProvidersAddresses[i] = operatorToStakingProvider(
                groupMembersAddresses[i]
            );
        }

        try
            staking.seize(
                _unauthorizedSigningSlashingAmount,
                _unauthorizedSigningNotificationRewardMultiplier,
                msg.sender,
                stakingProvidersAddresses
            )
        {
            // slither-disable-next-line reentrancy-events
            emit UnauthorizedSigningSlashed(
                groupId,
                _unauthorizedSigningSlashingAmount,
                groupMembersAddresses
            );
        } catch {
            // Should never happen but we want to ensure a non-critical path
            // failure from an external contract does not stop the challenge
            // to complete.
            emit UnauthorizedSigningSlashingFailed(
                groupId,
                _unauthorizedSigningSlashingAmount,
                groupMembersAddresses
            );
        }
    }

    /// @notice Notifies about operators who are inactive. Using this function,
    ///         a majority of the group can decide about punishing specific
    ///         group members who constantly fail doing their job. If the provided
    ///         claim is proved to be valid and signed by sufficient number
    ///         of group members, operators of members deemed as inactive are
    ///         banned for sortition pool rewards for duration specified by
    ///         `_sortitionPoolRewardsBanDuration` parameter. The sender of
    ///         the claim must be one of the claim signers. This function can be
    ///         called only for active and non-terminated groups.
    /// @param claim Operator inactivity claim.
    /// @param nonce Current inactivity claim nonce for the given group. Must
    ///        be the same as the stored one.
    /// @param groupMembers Identifiers of group members.
    function notifyOperatorInactivity(
        Inactivity.Claim calldata claim,
        uint256 nonce,
        uint32[] calldata groupMembers
    ) external {
        uint256 gasStart = gasleft();
        uint64 groupId = claim.groupId;

        require(nonce == inactivityClaimNonce[groupId], "Invalid nonce");

        require(groups.isGroupActive(groupId), "Group is not active");

        Groups.Group storage group = groups.getGroup(groupId);

        require(
            group.membersHash == keccak256(abi.encode(groupMembers)),
            "Invalid group members"
        );

        uint32[] memory ineligibleOperators = Inactivity.verifyClaim(
            sortitionPool,
            claim,
            group.groupPubKey,
            nonce,
            groupMembers
        );

        inactivityClaimNonce[groupId]++;

        emit InactivityClaimed(groupId, nonce, msg.sender);

        sortitionPool.setRewardIneligibility(
            ineligibleOperators,
            // solhint-disable-next-line not-rely-on-time
            block.timestamp + _sortitionPoolRewardsBanDuration
        );

        reimbursementPool.refund(
            (gasStart - gasleft()) + _notifyOperatorInactivityGasOffset,
            msg.sender
        );
    }

    /// @notice The minimum authorization amount required so that operator can
    ///         participate in the random beacon. This amount is required to
    ///         execute slashing for providing a malicious DKG result or when
    ///         a relay entry times out.
    function minimumAuthorization() external view returns (uint96) {
        return authorization.parameters.minimumAuthorization;
    }

    /// @return Flag indicating whether a relay entry request is currently
    ///         in progress.
    function isRelayRequestInProgress() external view returns (bool) {
        return relay.isRequestInProgress();
    }

    /// @notice Returns the current value of the staking provider's eligible
    ///         stake. Eligible stake is defined as the currently authorized
    ///         stake minus the pending authorization decrease. Eligible stake
    ///         is what is used for operator's weight in the sortition pool.
    ///         If the authorized stake minus the pending authorization decrease
    ///         is below the minimum authorization, eligible stake is 0.
    function eligibleStake(
        address stakingProvider
    ) external view returns (uint96) {
        return authorization.eligibleStake(staking, stakingProvider);
    }

    /// @notice Returns the amount of rewards available for withdrawal for the
    ///         given staking provider. Reverts if staking provider has not
    ///         registered the operator address.
    function availableRewards(
        address stakingProvider
    ) external view returns (uint96) {
        address operator = stakingProviderToOperator(stakingProvider);
        require(operator != address(0), "Unknown operator");
        return sortitionPool.getAvailableRewards(operator);
    }

    /// @notice Returns the amount of stake that is pending authorization
    ///         decrease for the given staking provider. If no authorization
    ///         decrease has been requested, returns zero.
    function pendingAuthorizationDecrease(
        address stakingProvider
    ) external view returns (uint96) {
        return authorization.pendingAuthorizationDecrease(stakingProvider);
    }

    /// @notice Returns the remaining time in seconds that needs to pass before
    ///         the requested authorization decrease can be approved.
    ///         If the sortition pool state was not updated yet by the operator
    ///         after requesting the authorization decrease, returns
    ///         `type(uint64).max`.
    function remainingAuthorizationDecreaseDelay(
        address stakingProvider
    ) external view returns (uint64) {
        return
            authorization.remainingAuthorizationDecreaseDelay(stakingProvider);
    }

    /// @notice Returns operator registered for the given staking provider.
    function stakingProviderToOperator(
        address stakingProvider
    ) public view returns (address) {
        return authorization.stakingProviderToOperator[stakingProvider];
    }

    /// @notice Returns staking provider of the given operator.
    function operatorToStakingProvider(
        address operator
    ) public view returns (address) {
        return authorization.operatorToStakingProvider[operator];
    }

    /// @notice Checks if the operator's authorized stake is in sync with
    ///         operator's weight in the sortition pool.
    ///         If the operator is not in the sortition pool and their
    ///         authorized stake is non-zero, function returns false.
    function isOperatorUpToDate(address operator) external view returns (bool) {
        return
            authorization.isOperatorUpToDate(staking, sortitionPool, operator);
    }

    /// @notice Returns true if the given operator is in the sortition pool.
    ///         Otherwise, returns false.
    function isOperatorInPool(address operator) external view returns (bool) {
        return sortitionPool.isOperatorInPool(operator);
    }

    /// @notice Selects a new group of operators. Can only be called when DKG
    ///         is in progress and the pool is locked.
    ///         At least one operator has to be registered in the pool,
    ///         otherwise the function fails reverting the transaction.
    /// @return IDs of selected group members.
    function selectGroup() external view returns (uint32[] memory) {
        return sortitionPool.selectGroup(DKG.groupSize, bytes32(dkg.seed));
    }

    /// @notice Returns authorization-related parameters of the beacon.
    /// @dev The minimum authorization is also returned by `minimumAuthorization()`
    ///      function, as a requirement of `IApplication` interface.
    /// @return minimumAuthorization The minimum authorization amount required
    ///         so that operator can participate in the random beacon. This
    ///         amount is required to execute slashing for providing a malicious
    ///         DKG result or when a relay entry times out.
    /// @return authorizationDecreaseDelay Delay in seconds that needs to pass
    ///         between the time authorization decrease is requested and the
    ///         time that request gets approved. Protects against free-riders
    ///         earning rewards and not being active in the network.
    /// @return authorizationDecreaseChangePeriod Authorization decrease change
    ///        period in seconds. It is the time, before authorization decrease
    ///        delay end, during which the pending authorization decrease
    ///        request can be overwritten.
    ///        If set to 0, pending authorization decrease request can not be
    ///        overwritten until the entire `authorizationDecreaseDelay` ends.
    ///        If set to value equal `authorizationDecreaseDelay`, request can
    ///        always be overwritten.
    function authorizationParameters()
        external
        view
        returns (
            uint96 minimumAuthorization,
            uint64 authorizationDecreaseDelay,
            uint64 authorizationDecreaseChangePeriod
        )
    {
        return (
            authorization.parameters.minimumAuthorization,
            authorization.parameters.authorizationDecreaseDelay,
            authorization.parameters.authorizationDecreaseChangePeriod
        );
    }

    /// @notice Returns relay-entry-related parameters of the beacon.
    /// @return relayEntrySoftTimeout Soft timeout in blocks for a group to
    ///         submit the relay entry. If the soft timeout is reached for
    ///         submitting the relay entry, the slashing starts.
    /// @return relayEntryHardTimeout Hard timeout in blocks for a group to
    ///         submit the relay entry. After the soft timeout passes without
    ///         relay entry submitted, all group members start getting slashed.
    ///         The slashing amount increases linearly until the group submits
    ///         the relay entry or until `relayEntryHardTimeout` is reached.
    ///         When the hard timeout is reached, each group member will get
    ///         slashed for `_relayEntrySubmissionFailureSlashingAmount`.
    /// @return callbackGasLimit Relay entry callback gas limit. This is the gas
    ///         limit with which callback function provided in the relay request
    ///         transaction is executed. The callback is executed with a new
    ///         relay entry value in the same transaction the relay entry is
    ///         submitted.
    function relayEntryParameters()
        external
        view
        returns (
            uint256 relayEntrySoftTimeout,
            uint256 relayEntryHardTimeout,
            uint256 callbackGasLimit
        )
    {
        return (
            relay.relayEntrySoftTimeout,
            relay.relayEntryHardTimeout,
            _callbackGasLimit
        );
    }

    /// @notice Returns group-creation-related parameters of the beacon.
    /// @return groupCreationFrequency The frequency of a new group creation.
    ///         Groups are created with a fixed frequency of relay requests.
    /// @return groupLifetime Group lifetime in blocks. When a group reached its
    ///         lifetime, it is no longer selected for new relay requests but
    ///         may still be responsible for submitting relay entry if relay
    ///         request assigned to that group is still pending.
    /// @return dkgResultChallengePeriodLength The number of blocks for which
    ///         a DKG result can be challenged. Anyone can challenge DKG result
    ///         for a certain number of blocks before the result is fully
    ///         accepted and the group registered in the pool of active groups.
    ///         If the challenge gets accepted, all operators who signed the
    ///         malicious result get slashed for and the notifier gets rewarded.
    /// @return dkgResultChallengeExtraGas The extra gas required to be left at
    ///         the end of the challenge DKG result transaction.
    /// @return dkgResultSubmissionTimeout Timeout in blocks for a group to
    ///         submit the DKG result. All members are eligible to submit the
    ///         DKG result. If `dkgResultSubmissionTimeout` passes without the
    ///         DKG result submitted, DKG is considered as timed out and no DKG
    ///         result for this group creation can be submitted anymore.
    /// @return dkgSubmitterPrecedencePeriodLength Time during the DKG result
    ///         approval stage when the submitter of the DKG result takes the
    ///         precedence to approve the DKG result. After this time passes
    ///         anyone can approve the DKG result.
    function groupCreationParameters()
        external
        view
        returns (
            uint256 groupCreationFrequency,
            uint256 groupLifetime,
            uint256 dkgResultChallengePeriodLength,
            uint256 dkgResultChallengeExtraGas,
            uint256 dkgResultSubmissionTimeout,
            uint256 dkgSubmitterPrecedencePeriodLength
        )
    {
        return (
            _groupCreationFrequency,
            groups.groupLifetime,
            dkg.parameters.resultChallengePeriodLength,
            dkg.parameters.resultChallengeExtraGas,
            dkg.parameters.resultSubmissionTimeout,
            dkg.parameters.submitterPrecedencePeriodLength
        );
    }

    /// @notice Returns reward-related parameters of the beacon.
    /// @return sortitionPoolRewardsBanDuration Duration of the sortition pool
    ///         rewards ban imposed on operators who misbehaved during DKG by
    ///         being inactive or disqualified and for operators that were
    ///         identified by the rest of group members as inactive via
    ///         `notifyOperatorInactivity`.
    /// @return relayEntryTimeoutNotificationRewardMultiplier Percentage of the
    ///         staking contract malicious behavior notification reward which
    ///         will be transferred to the notifier reporting about relay entry
    ///         timeout. Notifiers are rewarded from a notifiers treasury pool.
    ///         For example, if notification reward is 1000 and the value of the
    ///         multiplier is 5, the notifier will receive: 5% of 1000 = 50 per
    ///         each operator affected.
    /// @return unauthorizedSigningNotificationRewardMultiplier Percentage of the
    ///         staking contract malicious behavior notification reward which
    ///         will be transferred to the notifier reporting about unauthorized
    ///         signing. Notifiers are rewarded from a notifiers treasury pool.
    ///         For example, if a notification reward is 1000 and the value of
    ///         the multiplier is 5, the notifier will receive: 5% of 1000 = 50
    ///         per each operator affected.
    /// @return dkgMaliciousResultNotificationRewardMultiplier Percentage of the
    ///         staking contract malicious behavior notification reward which
    ///         will be transferred to the notifier reporting about a malicious
    ///         DKG result. Notifiers are rewarded from a notifiers treasury
    ///         pool. For example, if notification reward is 1000 and the value
    ///         of the multiplier is 5, the notifier will receive:
    ///         5% of 1000 = 50 per each operator affected.
    function rewardParameters()
        external
        view
        returns (
            uint256 sortitionPoolRewardsBanDuration,
            uint256 relayEntryTimeoutNotificationRewardMultiplier,
            uint256 unauthorizedSigningNotificationRewardMultiplier,
            uint256 dkgMaliciousResultNotificationRewardMultiplier
        )
    {
        return (
            _sortitionPoolRewardsBanDuration,
            _relayEntryTimeoutNotificationRewardMultiplier,
            _unauthorizedSigningNotificationRewardMultiplier,
            _dkgMaliciousResultNotificationRewardMultiplier
        );
    }

    /// @notice Returns slashing-related parameters of the beacon.
    /// @return relayEntrySubmissionFailureSlashingAmount Slashing amount for
    ///         not submitting relay entry. When relay entry hard timeout is
    ///         reached without the relay entry submitted, each group member
    ///         gets slashed for `relayEntrySubmissionFailureSlashingAmount`.
    ///         If the relay entry gets submitted after the soft timeout, but
    ///         before the hard timeout, each group member gets slashed
    ///         proportionally to `relayEntrySubmissionFailureSlashingAmount`
    ///         and the time passed since the soft deadline.
    /// @return maliciousDkgResultSlashingAmount Slashing amount for submitting
    ///         a malicious DKG result. Every DKG result submitted can be
    ///         challenged for the time of `dkg.ResultChallengePeriodLength`.
    ///         If the DKG result submitted is challenged and proven to be
    ///         malicious, the operator who submitted the malicious result is
    ///         slashed for `maliciousDkgResultSlashingAmount`.
    /// @return unauthorizedSigningSlashingAmount Slashing amount when an
    ///         unauthorized signing has been proved, which means the private
    ///         key leaked and all the group members should be punished.
    function slashingParameters()
        external
        view
        returns (
            uint96 relayEntrySubmissionFailureSlashingAmount,
            uint96 maliciousDkgResultSlashingAmount,
            uint96 unauthorizedSigningSlashingAmount
        )
    {
        return (
            relay.relayEntrySubmissionFailureSlashingAmount,
            _maliciousDkgResultSlashingAmount,
            _unauthorizedSigningSlashingAmount
        );
    }

    /// @notice Returns gas-related parameters of the beacon.
    /// @return dkgResultSubmissionGas Calculated gas cost for submitting a DKG
    ///         result. This will be refunded as part of the DKG approval
    ///         process.
    /// @return dkgResultApprovalGasOffset Gas that is meant to balance the DKG
    ///         result approval's overall cost.
    /// @return notifyOperatorInactivityGasOffset Gas that is meant to balance
    ///         the operator inactivity notification cost.
    /// @return relayEntrySubmissionGasOffset Gas that is meant to balance the
    ///         relay entry submission cost.
    function gasParameters()
        external
        view
        returns (
            uint256 dkgResultSubmissionGas,
            uint256 dkgResultApprovalGasOffset,
            uint256 notifyOperatorInactivityGasOffset,
            uint256 relayEntrySubmissionGasOffset
        )
    {
        return (
            _dkgResultSubmissionGas,
            _dkgResultApprovalGasOffset,
            _notifyOperatorInactivityGasOffset,
            _relayEntrySubmissionGasOffset
        );
    }
}
