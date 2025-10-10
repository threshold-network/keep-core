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

import "./api/IWalletRegistry.sol";
import "./api/IWalletOwner.sol";
import "./Allowlist.sol";
import "./libraries/Wallets.sol";
import {EcdsaAuthorization as Authorization} from "./libraries/EcdsaAuthorization.sol";
import {EcdsaDkg as DKG} from "./libraries/EcdsaDkg.sol";
import {EcdsaInactivity as Inactivity} from "./libraries/EcdsaInactivity.sol";
import {EcdsaDkgValidator as DKGValidator} from "./EcdsaDkgValidator.sol";

import "@keep-network/sortition-pools/contracts/SortitionPool.sol";
import "@keep-network/random-beacon/contracts/api/IRandomBeacon.sol";
import "@keep-network/random-beacon/contracts/api/IRandomBeaconConsumer.sol";
import "@keep-network/random-beacon/contracts/Reimbursable.sol";
import "@keep-network/random-beacon/contracts/ReimbursementPool.sol";
import "@keep-network/random-beacon/contracts/Governable.sol";

import "@threshold-network/solidity-contracts/contracts/staking/IApplication.sol";
import "@threshold-network/solidity-contracts/contracts/staking/IStaking.sol";

import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

contract WalletRegistry is
    IWalletRegistry,
    IRandomBeaconConsumer,
    IApplication,
    Governable,
    Reimbursable,
    Initializable
{
    using Authorization for Authorization.Data;
    using DKG for DKG.Data;
    using Wallets for Wallets.Data;

    // Libraries data storages
    Authorization.Data internal authorization;
    DKG.Data internal dkg;
    Wallets.Data internal wallets;

    /// @notice Slashing amount for submitting a malicious DKG result. Every
    ///         DKG result submitted can be challenged for the time of
    ///         `dkg.resultChallengePeriodLength`. If the DKG result submitted
    ///         is challenged and proven to be malicious, the operator who
    ///         submitted the malicious result is slashed for
    ///         `_maliciousDkgResultSlashingAmount`.
    uint96 internal _maliciousDkgResultSlashingAmount;

    /// @notice Percentage of the staking contract malicious behavior
    ///         notification reward which will be transferred to the notifier
    ///         reporting about a malicious DKG result. Notifiers are rewarded
    ///         from a notifiers treasury pool. For example, if
    ///         notification reward is 1000 and the value of the multiplier is
    ///         5, the notifier will receive: 5% of 1000 = 50 per each
    ///         operator affected.
    uint256 internal _maliciousDkgResultNotificationRewardMultiplier;

    /// @notice Duration of the sortition pool rewards ban imposed on operators
    ///         who missed their turn for DKG result submission or who failed
    ///         a heartbeat.
    uint256 internal _sortitionPoolRewardsBanDuration;

    /// @notice Calculated max gas cost for submitting a DKG result. This will
    ///         be refunded as part of the DKG approval process. It is in the
    ///         submitter's interest to not skip his priority turn on the approval,
    ///         otherwise the refund of the DKG submission will be refunded to
    ///         another group member that will call the DKG approve function.
    uint256 internal _dkgResultSubmissionGas;

    /// @notice Gas that is meant to balance the DKG result approval's overall
    ///         cost. It can be updated by the governance based on the current
    ///         market conditions.
    uint256 internal _dkgResultApprovalGasOffset;

    /// @notice Gas that is meant to balance the notification of an operator
    ///         inactivity. It can be updated by the governance based on the
    ///         current market conditions.
    uint256 internal _notifyOperatorInactivityGasOffset;

    /// @notice Gas that is meant to balance the notification of a seed for DKG
    ///         delivery timeout. It can be updated by the governance based on the
    ///         current market conditions.
    uint256 internal _notifySeedTimeoutGasOffset;

    /// @notice Gas that is meant to balance the notification of a DKG protocol
    ///         execution timeout. It can be updated by the governance based on the
    ///         current market conditions.
    /// @dev The value is subtracted for the refundable gas calculation, as the
    ///      DKG timeout notification transaction recovers some gas when cleaning
    ///      up the storage.
    uint256 internal _notifyDkgTimeoutNegativeGasOffset;

    /// @notice Stores current operator inactivity claim nonce for the given
    ///         wallet signing group. Each claim is made with a unique nonce
    ///         which protects against claim replay.
    mapping(bytes32 => uint256) public inactivityClaimNonce; // walletID -> nonce

    // Address that is set as owner of all wallets. Only this address can request
    // new wallets creation and manage their state.
    IWalletOwner public walletOwner;

    // External dependencies

    /// @custom:oz-upgrades-unsafe-allow state-variable-immutable
    SortitionPool public immutable sortitionPool;
    /// @custom:oz-upgrades-unsafe-allow state-variable-immutable
    IStaking public immutable staking;
    IRandomBeacon public randomBeacon;

    /// @notice Allowlist contract for weight-based operator authorization.
    ///         When set (non-zero address), takes precedence over legacy TokenStaking.
    ///         This enables gradual migration from T staking to allowlist-based
    ///         authorization following TIP-092, while maintaining backward
    ///         compatibility with existing deployments.
    /// @dev Set via initializeV2() during proxy upgrade. When allowlist is zero
    ///      address (default), the legacy TokenStaking authorization path is used.
    Allowlist public allowlist;

    // Events
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

    event DkgStateLocked();

    event DkgSeedTimedOut();

    event WalletCreated(
        bytes32 indexed walletID,
        bytes32 indexed dkgResultHash
    );

    event WalletClosed(bytes32 indexed walletID);

    event DkgMaliciousResultSlashed(
        bytes32 indexed resultHash,
        uint256 slashingAmount,
        address maliciousSubmitter
    );

    event AuthorizationParametersUpdated(
        uint96 minimumAuthorization,
        uint64 authorizationDecreaseDelay,
        uint64 authorizationDecreaseChangePeriod
    );

    event RewardParametersUpdated(
        uint256 maliciousDkgResultNotificationRewardMultiplier,
        uint256 sortitionPoolRewardsBanDuration
    );

    event SlashingParametersUpdated(uint256 maliciousDkgResultSlashingAmount);

    event DkgParametersUpdated(
        uint256 seedTimeout,
        uint256 resultChallengePeriodLength,
        uint256 resultChallengeExtraGas,
        uint256 resultSubmissionTimeout,
        uint256 resultSubmitterPrecedencePeriodLength
    );

    event GasParametersUpdated(
        uint256 dkgResultSubmissionGas,
        uint256 dkgResultApprovalGasOffset,
        uint256 notifyOperatorInactivityGasOffset,
        uint256 notifySeedTimeoutGasOffset,
        uint256 notifyDkgTimeoutNegativeGasOffset
    );

    event RandomBeaconUpgraded(address randomBeacon);

    event WalletOwnerUpdated(address walletOwner);

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

    event InactivityClaimed(
        bytes32 indexed walletID,
        uint256 nonce,
        address notifier
    );

    // Custom Errors

    // Authorization Errors

    /// @notice Raised when caller is not the staking contract or allowlist contract.
    error CallerNotStakingContract();

    /// @notice Raised when caller is not the designated wallet owner contract.
    error CallerNotWalletOwner();

    /// @notice Raised when caller is not the governance address.
    error CallerNotGovernance();

    /// @notice Raised when caller is not the authorized random beacon contract.
    error CallerNotRandomBeacon();

    // Validation Errors

    /// @notice Raised when allowlist address provided is zero address.
    error AllowlistAddressZero();

    /// @notice Raised when querying an operator that has not been registered.
    error UnknownOperator();

    /// @notice Raised when provided nonce does not match the expected inactivity claim nonce.
    error InvalidNonce();

    /// @notice Raised when the hash of provided group members does not match wallet's stored hash.
    error InvalidGroupMembers();

    /// @notice Raised when the hash of provided wallet member IDs does not match stored hash.
    error InvalidWalletMembersIdentifiers();

    /// @notice Raised when querying with an address that is not a sortition pool operator.
    error NotSortitionPoolOperator();

    /// @notice Raised when provided wallet member index is outside valid range [1, length].
    error WalletMemberIndexOutOfRange();

    // State Errors

    /// @notice Raised when DKG parameter update attempted while DKG state is not IDLE.
    error CurrentStateNotIdle();

    // Configuration Errors

    /// @notice Raised when insufficient gas remains after challengeDkgResult execution.
    error NotEnoughExtraGasLeft();

    /// @notice Dual-mode authorization modifier supporting both Allowlist and
    ///         legacy TokenStaking authorization paths.
    /// @dev Authorization precedence:
    ///      1. If allowlist is set (non-zero), only allowlist contract can call
    ///      2. If allowlist is NOT set (zero), only legacy staking contract can call
    ///      This ensures a clean migration path while maintaining backward compatibility.
    ///      The address is cached in a local variable to minimize gas costs from
    ///      storage reads (SLOAD operation).
    modifier onlyStakingContract() {
        address _allowlist = address(allowlist);
        if (_allowlist != address(0)) {
            // Allowlist authorization path (post-TIP-092)
            if (msg.sender != _allowlist) revert CallerNotStakingContract();
        } else {
            // Legacy staking authorization path (pre-TIP-092, backward compatible)
            if (msg.sender != address(staking))
                revert CallerNotStakingContract();
        }
        _;
    }

    /// @notice Reverts if called not by the Wallet Owner.
    modifier onlyWalletOwner() {
        if (msg.sender != address(walletOwner)) revert CallerNotWalletOwner();
        _;
    }

    /// @notice Reverts if called not by the governance.
    modifier onlyReimbursableAdmin() override {
        if (msg.sender != governance) revert CallerNotGovernance();
        _;
    }

    /// @dev Used to initialize immutable variables only, use `initialize` function
    ///      for upgradable contract initialization on deployment.
    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor(SortitionPool _sortitionPool, IStaking _staking) {
        sortitionPool = _sortitionPool;
        staking = _staking;

        _disableInitializers();
    }

    /// @dev Initializes upgradable contract on deployment.
    function initialize(
        DKGValidator _ecdsaDkgValidator,
        IRandomBeacon _randomBeacon,
        ReimbursementPool _reimbursementPool
    ) external initializer {
        randomBeacon = _randomBeacon;
        reimbursementPool = _reimbursementPool;

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
        authorization.setMinimumAuthorization(40_000e18);
        authorization.setAuthorizationDecreaseDelay(3_888_000);
        authorization.setAuthorizationDecreaseChangePeriod(3_888_000);

        // Malicious DKG result slashing amount is set initially to 1% of the
        // minimum authorization (400 T). This values needs to be increased
        // significantly once the system is fully launched.
        //
        // Notifier of a malicious DKG result receives 100% of the notifier
        // reward from the staking contract.
        //
        // Inactive operators are set as ineligible for rewards for 2 weeks.
        _maliciousDkgResultSlashingAmount = 400e18;
        _maliciousDkgResultNotificationRewardMultiplier = 100;
        _sortitionPoolRewardsBanDuration = 2 weeks;

        // DKG seed timeout is set to 48h assuming 15s block time. The same
        // value is used by the Random Beacon as a relay entry hard timeout.
        //
        // DKG result challenge period length is set to 48h as well, assuming
        // 15s block time.
        //
        // DKG result submission timeout covers:
        // - 20 blocks required to confirm the DkgStarted event off-chain
        // - 1 attempt of the off-chain protocol that takes 216 blocks at most
        // - 3 blocks to submit the result for each of the 100 members
        // That gives: 20 + (1 * 216) + (3 * 100) = 536
        //
        //
        // The original DKG result submitter has 20 blocks to approve it before
        // anyone else can do that.
        //
        // With these parameters, the happy path takes no more than 104 hours.
        // In practice, it should take about 48 hours (just the challenge time).
        dkg.init(sortitionPool, _ecdsaDkgValidator);
        dkg.setSeedTimeout(11_520);
        dkg.setResultChallengePeriodLength(11_520);
        dkg.setResultChallengeExtraGas(50_000);
        dkg.setResultSubmissionTimeout(536);
        dkg.setSubmitterPrecedencePeriodLength(20);

        // Gas parameters were adjusted based on Ethereum state in April 2022.
        // If the cost of EVM opcodes change over time, these parameters will
        // have to be updated.
        _dkgResultSubmissionGas = 290_000;
        _dkgResultApprovalGasOffset = 72_000;
        _notifyOperatorInactivityGasOffset = 93_000;
        _notifySeedTimeoutGasOffset = 7_250;
        _notifyDkgTimeoutNegativeGasOffset = 2_300;
    }

    /// @notice Upgrades WalletRegistry to support allowlist-based authorization.
    ///         This function enables the migration from legacy TokenStaking to the
    ///         new Allowlist contract following TIP-092 governance decision.
    ///         Once called, the allowlist contract becomes the sole authority for
    ///         operator authorization, replacing the TokenStaking contract.
    /// @param _allowlist Address of the Allowlist contract
    /// @dev Uses reinitializer(2) for proxy upgrade compatibility. Can only be
    ///      called once per proxy upgrade. The zero address check prevents
    ///      misconfiguration that would break authorization.
    ///      After successful execution, the onlyStakingContract modifier will
    ///      only accept calls from the allowlist contract.
    function initializeV2(address _allowlist) external reinitializer(2) {
        if (_allowlist == address(0)) revert AllowlistAddressZero();
        allowlist = Allowlist(_allowlist);
    }

    /// @notice Withdraws application rewards for the given staking provider.
    ///         Rewards are withdrawn to the staking provider's beneficiary
    ///         address set in the staking contract. Reverts if staking provider
    ///         has not registered the operator address.
    /// @dev Emits `RewardsWithdrawn` event.
    function withdrawRewards(address stakingProvider) external {
        address operator = stakingProviderToOperator(stakingProvider);
        if (operator == address(0)) revert UnknownOperator();
        (, address beneficiary, ) = staking.rolesOf(stakingProvider);
        uint96 amount = sortitionPool.withdrawRewards(operator, beneficiary);
        // slither-disable-next-line reentrancy-events
        emit RewardsWithdrawn(stakingProvider, amount);
    }

    /// @notice Withdraws rewards belonging to operators marked as ineligible
    ///         for sortition pool rewards.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract.
    /// @param recipient Recipient of withdrawn rewards.
    function withdrawIneligibleRewards(address recipient)
        external
        onlyGovernance
    {
        sortitionPool.withdrawIneligible(recipient);
    }

    /// @notice Used by staking provider to set operator address that will
    ///         operate ECDSA node. The given staking provider can set operator
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
    ///         by ECDSA. Function reverts if there is no minimum stake
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

    /// @notice Used by T staking contract to inform the application that the
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

    /// @notice Used by T staking contract to inform the application that the
    ///         authorization decrease for the given staking provider has been
    ///         requested.
    ///
    ///         Reverts if the amount after deauthorization would be non-zero
    ///         and lower than the minimum authorization.
    ///
    ///         If the operator is not known (`registerOperator` was not called)
    ///         it lets to `approveAuthorizationDecrease` immediatelly. If the
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
    ///         overwritten.
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

    /// @notice Used by T staking contract to inform the application the
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

    /// @notice Updates address of the Random Beacon.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param _randomBeacon Random Beacon address.
    function upgradeRandomBeacon(IRandomBeacon _randomBeacon)
        external
        onlyGovernance
    {
        randomBeacon = _randomBeacon;
        emit RandomBeaconUpgraded(address(_randomBeacon));
    }

    /// @notice Updates the wallet owner.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters. The wallet owner has to implement `IWalletOwner`
    ///      interface.
    /// @param _walletOwner New wallet owner address.
    function updateWalletOwner(IWalletOwner _walletOwner)
        external
        onlyGovernance
    {
        walletOwner = _walletOwner;
        emit WalletOwnerUpdated(address(_walletOwner));
    }

    /// @notice Updates the values of authorization parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param _minimumAuthorization New minimum authorization amount.
    /// @param _authorizationDecreaseDelay New authorization decrease delay in
    ///        seconds.
    /// @param _authorizationDecreaseChangePeriod New authorization decrease
    ///        change period in seconds.
    function updateAuthorizationParameters(
        uint96 _minimumAuthorization,
        uint64 _authorizationDecreaseDelay,
        uint64 _authorizationDecreaseChangePeriod
    ) external onlyGovernance {
        authorization.setMinimumAuthorization(_minimumAuthorization);
        authorization.setAuthorizationDecreaseDelay(
            _authorizationDecreaseDelay
        );
        authorization.setAuthorizationDecreaseChangePeriod(
            _authorizationDecreaseChangePeriod
        );

        emit AuthorizationParametersUpdated(
            _minimumAuthorization,
            _authorizationDecreaseDelay,
            _authorizationDecreaseChangePeriod
        );
    }

    /// @notice Updates the values of DKG parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param _seedTimeout New seed timeout.
    /// @param _resultChallengePeriodLength New DKG result challenge period
    ///        length.
    /// @param _resultChallengeExtraGas New extra gas value required to be left
    ///        at the end of the DKG result challenge transaction.
    /// @param _resultSubmissionTimeout New DKG result submission timeout.
    /// @param _submitterPrecedencePeriodLength New submitter precedence period
    ///        length.
    function updateDkgParameters(
        uint256 _seedTimeout,
        uint256 _resultChallengePeriodLength,
        uint256 _resultChallengeExtraGas,
        uint256 _resultSubmissionTimeout,
        uint256 _submitterPrecedencePeriodLength
    ) external onlyGovernance {
        // Consolidated state validation for all DKG parameter setters. Since all
        // setters are called exclusively from this function, we perform the state
        // check once here instead of in each individual setter to reduce bytecode size.
        if (dkg.currentState() != DKG.State.IDLE) revert CurrentStateNotIdle();

        dkg.setSeedTimeout(_seedTimeout);
        dkg.setResultChallengePeriodLength(_resultChallengePeriodLength);
        dkg.setResultChallengeExtraGas(_resultChallengeExtraGas);
        dkg.setResultSubmissionTimeout(_resultSubmissionTimeout);
        dkg.setSubmitterPrecedencePeriodLength(
            _submitterPrecedencePeriodLength
        );

        // slither-disable-next-line reentrancy-events
        emit DkgParametersUpdated(
            _seedTimeout,
            _resultChallengePeriodLength,
            _resultChallengeExtraGas,
            _resultSubmissionTimeout,
            _submitterPrecedencePeriodLength
        );
    }

    /// @notice Updates the values of reward parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param maliciousDkgResultNotificationRewardMultiplier New value of the
    ///        DKG malicious result notification reward multiplier.
    /// @param sortitionPoolRewardsBanDuration New sortition pool rewards
    ///        ban duration in seconds.
    function updateRewardParameters(
        uint256 maliciousDkgResultNotificationRewardMultiplier,
        uint256 sortitionPoolRewardsBanDuration
    ) external onlyGovernance {
        _maliciousDkgResultNotificationRewardMultiplier = maliciousDkgResultNotificationRewardMultiplier;
        _sortitionPoolRewardsBanDuration = sortitionPoolRewardsBanDuration;
        emit RewardParametersUpdated(
            maliciousDkgResultNotificationRewardMultiplier,
            sortitionPoolRewardsBanDuration
        );
    }

    /// @notice Updates the values of slashing parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param maliciousDkgResultSlashingAmount New malicious DKG result
    ///        slashing amount.
    function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount)
        external
        onlyGovernance
    {
        _maliciousDkgResultSlashingAmount = maliciousDkgResultSlashingAmount;
        emit SlashingParametersUpdated(maliciousDkgResultSlashingAmount);
    }

    /// @notice Updates the values of gas-related parameters.
    /// @dev Can be called only by the contract guvnor, which should be the
    ///      wallet registry governance contract. The caller is responsible for
    ///      validating parameters.
    /// @param dkgResultSubmissionGas New DKG result submission gas.
    /// @param dkgResultApprovalGasOffset New DKG result approval gas offset.
    /// @param notifyOperatorInactivityGasOffset New operator inactivity
    ///        notification gas offset.
    /// @param notifySeedTimeoutGasOffset New seed for DKG delivery timeout
    ///        notification gas offset.
    /// @param notifyDkgTimeoutNegativeGasOffset New DKG timeout notification gas
    ///        offset.
    function updateGasParameters(
        uint256 dkgResultSubmissionGas,
        uint256 dkgResultApprovalGasOffset,
        uint256 notifyOperatorInactivityGasOffset,
        uint256 notifySeedTimeoutGasOffset,
        uint256 notifyDkgTimeoutNegativeGasOffset
    ) external onlyGovernance {
        _dkgResultSubmissionGas = dkgResultSubmissionGas;
        _dkgResultApprovalGasOffset = dkgResultApprovalGasOffset;
        _notifyOperatorInactivityGasOffset = notifyOperatorInactivityGasOffset;
        _notifySeedTimeoutGasOffset = notifySeedTimeoutGasOffset;
        _notifyDkgTimeoutNegativeGasOffset = notifyDkgTimeoutNegativeGasOffset;

        emit GasParametersUpdated(
            dkgResultSubmissionGas,
            dkgResultApprovalGasOffset,
            notifyOperatorInactivityGasOffset,
            _notifySeedTimeoutGasOffset,
            _notifyDkgTimeoutNegativeGasOffset
        );
    }

    /// @notice Requests a new wallet creation.
    /// @dev Can be called only by the owner of wallets.
    ///      It locks the DKG and request a new relay entry. It expects
    ///      that the DKG process will be started once a new relay entry
    ///      gets generated.
    function requestNewWallet() external onlyWalletOwner {
        dkg.lockState();

        randomBeacon.requestRelayEntry(this);
    }

    /// @notice Closes an existing wallet. Reverts if wallet with the given ID
    ///         does not exist or if it has already been closed.
    /// @param walletID ID of the wallet.
    /// @dev Only a Wallet Owner can call this function.
    function closeWallet(bytes32 walletID) external onlyWalletOwner {
        wallets.deleteWallet(walletID);
        emit WalletClosed(walletID);
    }

    /// @notice A callback that is executed once a new relay entry gets
    ///         generated. It starts the DKG process.
    /// @dev Can be called only by the random beacon contract.
    /// @param relayEntry Relay entry.
    function __beaconCallback(uint256 relayEntry, uint256) external {
        if (msg.sender != address(randomBeacon)) {
            revert CallerNotRandomBeacon();
        }

        dkg.start(relayEntry);
    }

    /// @notice Submits result of DKG protocol.
    ///         The DKG result consists of result submitting member index,
    ///         calculated group public key, bytes array of misbehaved members,
    ///         concatenation of signatures from group members, indices of members
    ///         corresponding to each signature and the list of group members.
    ///         The result is registered optimistically and waits for an approval.
    ///         The result can be challenged when it is believed to be incorrect.
    ///         The challenge verifies the registered result i.a. it checks if members
    ///         list corresponds to the expected set of members determined
    ///         by the sortition pool.
    /// @dev The message to be signed by each member is keccak256 hash of the
    ///      chain ID, calculated group public key, misbehaved members indices
    ///      and DKG start block. The calculated hash should be prefixed with
    ///      `\x19Ethereum signed message:\n` before signing, so the message to
    ///      sign is:
    ///      `\x19Ethereum signed message:\n${keccak256(chainID,groupPubKey,misbehavedIndices,startBlock)}`
    /// @param dkgResult DKG result.
    function submitDkgResult(DKG.Result calldata dkgResult) external {
        wallets.validatePublicKey(dkgResult.groupPubKey);
        dkg.submitResult(dkgResult);
    }

    /// @notice Approves DKG result. Can be called when the challenge period for
    ///         the submitted result is finished. Considers the submitted result
    ///         as valid, bans misbehaved group members from the sortition pool
    ///         rewards, and completes the group creation by activating the
    ///         candidate group. For the first `resultSubmissionTimeout` blocks
    ///         after the end of the challenge period can be called only by the
    ///         DKG result submitter. After that time, can be called by anyone.
    ///         A new wallet based on the DKG result details.
    /// @param dkgResult Result to approve. Must match the submitted result
    ///        stored during `submitDkgResult`.
    function approveDkgResult(DKG.Result calldata dkgResult) external {
        uint256 gasStart = gasleft();
        uint32[] memory misbehavedMembers = dkg.approveResult(dkgResult);

        (bytes32 walletID, bytes32 publicKeyX, bytes32 publicKeyY) = wallets
            .addWallet(dkgResult.membersHash, dkgResult.groupPubKey);

        emit WalletCreated(walletID, keccak256(abi.encode(dkgResult)));

        if (misbehavedMembers.length > 0) {
            sortitionPool.setRewardIneligibility(
                misbehavedMembers,
                // solhint-disable-next-line not-rely-on-time
                block.timestamp + _sortitionPoolRewardsBanDuration
            );
        }

        walletOwner.__ecdsaWalletCreatedCallback(
            walletID,
            publicKeyX,
            publicKeyY
        );

        dkg.complete();

        // Refund msg.sender's ETH for DKG result submission and result approval
        reimbursementPool.refund(
            _dkgResultSubmissionGas +
                (gasStart - gasleft()) +
                _dkgResultApprovalGasOffset,
            msg.sender
        );
    }

    /// @notice Notifies about seed for DKG delivery timeout. It is expected
    ///         that a seed is delivered by the Random Beacon as a relay entry in a
    ///         callback function.
    function notifySeedTimeout() external {
        uint256 gasStart = gasleft();

        dkg.notifySeedTimeout();

        reimbursementPool.refund(
            (gasStart - gasleft()) + _notifySeedTimeoutGasOffset,
            msg.sender
        );
    }

    /// @notice Notifies about DKG timeout.
    function notifyDkgTimeout() external {
        uint256 gasStart = gasleft();

        dkg.notifyDkgTimeout();

        // Note that the offset is subtracted as it is expected that the cleanup
        // performed on DKG timeout notification removes data from the storage
        // which is recovering gas for the transaction.
        reimbursementPool.refund(
            (gasStart - gasleft()) - _notifyDkgTimeoutNegativeGasOffset,
            msg.sender
        );
    }

    /// @notice Challenges DKG result. If the submitted result is proved to be
    ///         invalid it reverts the DKG back to the result submission phase.
    /// @param dkgResult Result to challenge. Must match the submitted result
    ///        stored during `submitDkgResult`.
    /// @dev Due to EIP-150 1/64 of the gas is not forwarded to the call, and
    ///      will be kept to execute the remaining operations in the function
    ///      after the call inside the try-catch. To eliminate a class of
    ///      attacks related to the gas limit manipulation, this function
    ///      requires an extra amount of gas to be left at the end of the
    ///      execution.
    ///
    ///      This function is EIP-7702 compatible - it does not restrict
    ///      callers to EOAs, allowing accounts with delegated code execution
    ///      to participate in DKG result challenges. Gas manipulation
    ///      protection is enforced via inline gas check regardless of caller
    ///      type.
    function challengeDkgResult(DKG.Result calldata dkgResult) external {
        (
            bytes32 maliciousDkgResultHash,
            uint32 maliciousDkgResultSubmitterId
        ) = dkg.challengeResult(dkgResult);

        address maliciousDkgResultSubmitterAddress = sortitionPool
            .getIDOperator(maliciousDkgResultSubmitterId);

        address[] memory operatorWrapper = new address[](1);
        operatorWrapper[0] = operatorToStakingProvider(
            maliciousDkgResultSubmitterAddress
        );

        // Attempt to slash malicious submitter. Slashing may fail silently
        // if the staking contract reverts, but challenge must complete
        // regardless. Bytecode optimization: empty catch block reduces
        // contract size by ~800 bytes (see commit 412a8e6d).
        // slither-disable-next-line reentrancy-events
        try
            staking.seize(
                _maliciousDkgResultSlashingAmount,
                _maliciousDkgResultNotificationRewardMultiplier,
                msg.sender,
                operatorWrapper
            )
        {
            emit DkgMaliciousResultSlashed(
                maliciousDkgResultHash,
                _maliciousDkgResultSlashingAmount,
                maliciousDkgResultSubmitterAddress
            );
        } catch {
            // Challenge completion is critical; slashing failure is acceptable.
        }

        // Due to EIP-150, 1/64 of the gas is not forwarded to the call, and
        // will be kept to execute the remaining operations in the function
        // after the call inside the try-catch.
        //
        // To ensure there is no way for the caller to manipulate gas limit in
        // such a way that the call inside try-catch fails with out-of-gas and
        // the rest of the function is executed with the remaining 1/64 of gas,
        // we require an extra gas amount to be left at the end of the call to
        // `challengeDkgResult`. This check enforces EIP-150 gas protection by
        // ensuring sufficient gas remains for safe execution.
        if (gasleft() < dkg.parameters.resultChallengeExtraGas) {
            revert NotEnoughExtraGasLeft();
        }
    }

    /// @notice Notifies about operators who are inactive. Using this function,
    ///         a majority of the wallet signing group can decide about
    ///         punishing specific group members who constantly fail doing their
    ///         job. If the provided claim is proved to be valid and signed by
    ///         sufficient number of group members, operators of members deemed
    ///         as inactive are banned from sortition pool rewards for the
    ///         duration specified by `sortitionPoolRewardsBanDuration` parameter.
    ///         The function allows to signal about single operators being
    ///         inactive as well as to signal wallet-wide heartbeat failures
    ///         that are propagated to the wallet owner who should begin the
    ///         procedure of moving responsibilities to another wallet given
    ///         that the wallet who failed the heartbeat may soon be not able to
    ///         function and provide new signatures.
    ///         The sender of the claim must be one of the claim signers. This
    ///         function can be called only for registered wallets
    /// @param claim Operator inactivity claim.
    /// @param nonce Current inactivity claim nonce for the given wallet signing
    ///              group. Must be the same as the stored one.
    /// @param groupMembers Identifiers of the wallet signing group members.
    function notifyOperatorInactivity(
        Inactivity.Claim calldata claim,
        uint256 nonce,
        uint32[] calldata groupMembers
    ) external {
        uint256 gasStart = gasleft();

        bytes32 walletID = claim.walletID;

        if (nonce != inactivityClaimNonce[walletID]) {
            revert InvalidNonce();
        }

        (bytes32 pubKeyX, bytes32 pubKeyY) = wallets
            .getWalletPublicKeyCoordinates(walletID);
        bytes32 memberIdsHash = wallets.getWalletMembersIdsHash(walletID);

        if (memberIdsHash != keccak256(abi.encode(groupMembers))) {
            revert InvalidGroupMembers();
        }

        uint32[] memory ineligibleOperators = Inactivity.verifyClaim(
            sortitionPool,
            claim,
            bytes.concat(pubKeyX, pubKeyY),
            nonce,
            groupMembers
        );

        inactivityClaimNonce[walletID]++;

        emit InactivityClaimed(walletID, nonce, msg.sender);

        sortitionPool.setRewardIneligibility(
            ineligibleOperators,
            // solhint-disable-next-line not-rely-on-time
            block.timestamp + _sortitionPoolRewardsBanDuration
        );

        if (claim.heartbeatFailed) {
            walletOwner.__ecdsaWalletHeartbeatFailedCallback(
                walletID,
                pubKeyX,
                pubKeyY
            );
        }

        reimbursementPool.refund(
            (gasStart - gasleft()) + _notifyOperatorInactivityGasOffset,
            msg.sender
        );
    }

    /// @notice Allows the wallet owner to add all signing group members of the
    ///         wallet with the given ID to the slashing queue of the staking .
    ///         contract. The notifier will receive reward per each group member
    ///         from the staking contract notifiers treasury. The reward is
    ///         scaled by the `rewardMultiplier` provided as a parameter.
    /// @param amount Amount of tokens to seize from each signing group member.
    /// @param rewardMultiplier Fraction of the staking contract notifiers
    ///        reward the notifier should receive; should be between [0, 100].
    /// @param notifier Address of the misbehavior notifier.
    /// @param walletID ID of the wallet.
    /// @param walletMembersIDs Identifiers of the wallet signing group members.
    /// @dev Requirements:
    ///      - The expression `keccak256(abi.encode(walletMembersIDs))` must
    ///        be exactly the same as the hash stored under `membersIdsHash`
    ///        for the given `walletID`. Those IDs are not directly stored
    ///        in the contract for gas efficiency purposes but they can be
    ///        read from appropriate `DkgResultSubmitted` and `DkgResultApproved`
    ///        events.
    ///      - `rewardMultiplier` must be between [0, 100].
    ///      - This function does revert if staking contract call reverts.
    ///        The calling code needs to handle the potential revert.
    function seize(
        uint96 amount,
        uint256 rewardMultiplier,
        address notifier,
        bytes32 walletID,
        uint32[] calldata walletMembersIDs
    ) external onlyWalletOwner {
        bytes32 memberIdsHash = wallets.getWalletMembersIdsHash(walletID);
        if (memberIdsHash != keccak256(abi.encode(walletMembersIDs)))
            revert InvalidWalletMembersIdentifiers();

        address[] memory groupMembersAddresses = sortitionPool.getIDOperators(
            walletMembersIDs
        );
        address[] memory stakingProvidersAddresses = new address[](
            walletMembersIDs.length
        );
        for (uint256 i = 0; i < groupMembersAddresses.length; i++) {
            stakingProvidersAddresses[i] = operatorToStakingProvider(
                groupMembersAddresses[i]
            );
        }

        staking.seize(
            amount,
            rewardMultiplier,
            notifier,
            stakingProvidersAddresses
        );
    }

    /// @notice Checks if DKG result is valid for the current DKG.
    /// @param result DKG result.
    /// @return True if the result is valid. If the result is invalid it returns
    ///         false and an error message.
    function isDkgResultValid(DKG.Result calldata result)
        external
        view
        returns (bool, string memory)
    {
        return dkg.isResultValid(result);
    }

    /// @notice Check current wallet creation state.
    function getWalletCreationState() external view returns (DKG.State) {
        return dkg.currentState();
    }

    /// @notice Checks whether the given operator is a member of the given
    ///         wallet signing group.
    /// @param walletID ID of the wallet.
    /// @param walletMembersIDs Identifiers of the wallet signing group members.
    /// @param operator Address of the checked operator.
    /// @param walletMemberIndex Position of the operator in the wallet signing
    ///        group members list.
    /// @return True - if the operator is a member of the given wallet signing
    ///         group. False - otherwise.
    /// @dev Requirements:
    ///      - The `operator` parameter must be an actual sortition pool operator.
    ///      - The expression `keccak256(abi.encode(walletMembersIDs))` must
    ///        be exactly the same as the hash stored under `membersIdsHash`
    ///        for the given `walletID`. Those IDs are not directly stored
    ///        in the contract for gas efficiency purposes but they can be
    ///        read from appropriate `DkgResultSubmitted` and `DkgResultApproved`
    ///        events.
    ///      - The `walletMemberIndex` must be in range [1, walletMembersIDs.length]
    function isWalletMember(
        bytes32 walletID,
        uint32[] calldata walletMembersIDs,
        address operator,
        uint256 walletMemberIndex
    ) external view returns (bool) {
        uint32 operatorID = sortitionPool.getOperatorID(operator);

        if (operatorID == 0) revert NotSortitionPoolOperator();

        bytes32 memberIdsHash = wallets.getWalletMembersIdsHash(walletID);

        if (memberIdsHash != keccak256(abi.encode(walletMembersIDs)))
            revert InvalidWalletMembersIdentifiers();

        if (walletMemberIndex < 1 || walletMemberIndex > walletMembersIDs.length)
            revert WalletMemberIndexOutOfRange();

        return walletMembersIDs[walletMemberIndex - 1] == operatorID;
    }

    /// @notice Checks if awaiting seed timed out.
    /// @return True if awaiting seed timed out, false otherwise.
    function hasSeedTimedOut() external view returns (bool) {
        return dkg.hasSeedTimedOut();
    }

    /// @notice Checks if DKG timed out. The DKG timeout period includes time required
    ///         for off-chain protocol execution and time for the result publication
    ///         for all group members. After this time result cannot be submitted
    ///         and DKG can be notified about the timeout.
    /// @return True if DKG timed out, false otherwise.
    function hasDkgTimedOut() external view returns (bool) {
        return dkg.hasDkgTimedOut();
    }

    function getWallet(bytes32 walletID)
        external
        view
        returns (Wallets.Wallet memory)
    {
        return wallets.registry[walletID];
    }

    /// @notice Gets public key of a wallet with a given wallet ID.
    ///         The public key is returned in an uncompressed format as a 64-byte
    ///         concatenation of X and Y coordinates.
    /// @param walletID ID of the wallet.
    /// @return Uncompressed public key of the wallet.
    function getWalletPublicKey(bytes32 walletID)
        external
        view
        returns (bytes memory)
    {
        return wallets.getWalletPublicKey(walletID);
    }

    /// @notice Checks if a wallet with the given ID is registered.
    /// @param walletID Wallet's ID.
    /// @return True if wallet is registered, false otherwise.
    function isWalletRegistered(bytes32 walletID) external view returns (bool) {
        return wallets.isWalletRegistered(walletID);
    }

    /// @notice The minimum authorization amount required so that operator can
    ///         participate in ECDSA Wallet operations.
    function minimumAuthorization() external view returns (uint96) {
        return authorization.parameters.minimumAuthorization;
    }

    /// @notice Returns the current value of the staking provider's eligible
    ///         stake. Eligible stake is defined as the currently authorized
    ///         stake minus the pending authorization decrease. Eligible stake
    ///         is what is used for operator's weight in the sortition pool.
    ///         If the authorized stake minus the pending authorization decrease
    ///         is below the minimum authorization, eligible stake is 0.
    function eligibleStake(address stakingProvider)
        external
        view
        returns (uint96)
    {
        return authorization.eligibleStake(staking, stakingProvider);
    }

    /// @notice Returns the amount of rewards available for withdrawal for the
    ///         given staking provider. Reverts if staking provider has not
    ///         registered the operator address.
    function availableRewards(address stakingProvider)
        external
        view
        returns (uint96)
    {
        address operator = stakingProviderToOperator(stakingProvider);
        if (operator == address(0)) revert UnknownOperator();
        return sortitionPool.getAvailableRewards(operator);
    }

    /// @notice Returns the amount of stake that is pending authorization
    ///         decrease for the given staking provider. If no authorization
    ///         decrease has been requested, returns zero.
    function pendingAuthorizationDecrease(address stakingProvider)
        external
        view
        returns (uint96)
    {
        return authorization.pendingAuthorizationDecrease(stakingProvider);
    }

    /// @notice Returns the remaining time in seconds that needs to pass before
    ///         the requested authorization decrease can be approved.
    ///         If the sortition pool state was not updated yet by the operator
    ///         after requesting the authorization decrease, returns
    ///         `type(uint64).max`.
    function remainingAuthorizationDecreaseDelay(address stakingProvider)
        external
        view
        returns (uint64)
    {
        return
            authorization.remainingAuthorizationDecreaseDelay(stakingProvider);
    }

    /// @notice Returns operator registered for the given staking provider.
    function stakingProviderToOperator(address stakingProvider)
        public
        view
        returns (address)
    {
        return authorization.stakingProviderToOperator[stakingProvider];
    }

    /// @notice Returns staking provider of the given operator.
    function operatorToStakingProvider(address operator)
        public
        view
        returns (address)
    {
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

    /// @notice Retrieves dkg parameters that were set in DKG library.
    function dkgParameters() external view returns (DKG.Parameters memory) {
        return dkg.parameters;
    }

    /// @notice Returns authorization-related parameters.
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
    ///         period in seconds. It is the time, before authorization decrease
    ///         delay end, during which the pending authorization decrease
    ///         request can be overwritten.
    ///         If set to 0, pending authorization decrease request can not be
    ///         overwritten until the entire `authorizationDecreaseDelay` ends.
    ///         If set to value equal `authorizationDecreaseDelay`, request can
    ///         always be overwritten.
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

    /// @notice Retrieves reward-related parameters.
    /// @return maliciousDkgResultNotificationRewardMultiplier Percentage of the
    ///         staking contract malicious behavior notification reward which
    ///         will be transferred to the notifier reporting about a malicious
    ///         DKG result. Notifiers are rewarded from a notifiers treasury
    ///         pool. For example, if notification reward is 1000 and the value
    ///         of the multiplier is 5, the notifier will receive:
    ///         5% of 1000 = 50 per each operator affected.
    /// @return sortitionPoolRewardsBanDuration Duration of the sortition pool
    ///         rewards ban imposed on operators who missed their turn for DKG
    ///         result submission or who failed a heartbeat.
    function rewardParameters()
        external
        view
        returns (
            uint256 maliciousDkgResultNotificationRewardMultiplier,
            uint256 sortitionPoolRewardsBanDuration
        )
    {
        return (
            _maliciousDkgResultNotificationRewardMultiplier,
            _sortitionPoolRewardsBanDuration
        );
    }

    /// @notice Retrieves slashing-related parameters.
    /// @return maliciousDkgResultSlashingAmount Slashing amount for submitting
    ///         a malicious DKG result. Every DKG result submitted can be
    ///         challenged for the time of `dkg.resultChallengePeriodLength`.
    ///         If the DKG result submitted is challenged and proven to be
    ///         malicious, the operator who submitted the malicious result is
    ///         slashed for `_maliciousDkgResultSlashingAmount`.
    function slashingParameters()
        external
        view
        returns (uint96 maliciousDkgResultSlashingAmount)
    {
        return _maliciousDkgResultSlashingAmount;
    }

    /// @notice Retrieves gas-related parameters.
    /// @return dkgResultSubmissionGas Calculated max gas cost for submitting
    ///         a DKG result. This will be refunded as part of the DKG approval
    ///         process. It is in the submitter's interest to not skip his
    ///         priority turn on the approval, otherwise the refund of the DKG
    ///         submission will be refunded to another group member that will
    ///         call the DKG approve function.
    /// @return dkgResultApprovalGasOffset Gas that is meant to balance the DKG
    ///         result approval's overall cost. It can be updated by the
    ///         governance based on the current market conditions.
    /// @return notifyOperatorInactivityGasOffset Gas that is meant to balance
    ///         the notification of an operator inactivity. It can be updated by
    ///         the governance based on the current market conditions.
    /// @return notifySeedTimeoutGasOffset Gas that is meant to balance the
    ///         notification of a seed for DKG delivery timeout. It can be updated
    ///         by the governance based on the current market conditions.
    /// @return notifyDkgTimeoutNegativeGasOffset Gas that is meant to balance
    ///         the notification of a DKG protocol execution timeout. It can be
    ///         updated by the governance based on the current market conditions.
    function gasParameters()
        external
        view
        returns (
            uint256 dkgResultSubmissionGas,
            uint256 dkgResultApprovalGasOffset,
            uint256 notifyOperatorInactivityGasOffset,
            uint256 notifySeedTimeoutGasOffset,
            uint256 notifyDkgTimeoutNegativeGasOffset
        )
    {
        return (
            _dkgResultSubmissionGas,
            _dkgResultApprovalGasOffset,
            _notifyOperatorInactivityGasOffset,
            _notifySeedTimeoutGasOffset,
            _notifyDkgTimeoutNegativeGasOffset
        );
    }
}
