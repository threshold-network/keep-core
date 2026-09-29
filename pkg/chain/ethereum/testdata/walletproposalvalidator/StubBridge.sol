// SPDX-License-Identifier: GPL-3.0-only
pragma solidity 0.8.17;

/// @title Stub bridge for the WalletProposalValidator in-memory EVM
///        harness.
/// @notice Implements exactly the Bridge/IReservationBridge view methods
///         called by `WalletProposalValidator.validateReservationAnchorProposal`
///         and `validateReservationReanchorProposal`
///         (see tbtc-v2's contracts/bridge/WalletProposalValidator.sol),
///         plus setters so the Go test harness (tbtc_validator_harness_test.go)
///         can seed the exact state each test case needs. This is a test
///         fixture, not a partial reimplementation of Bridge.sol: it does
///         not enforce any of the Bridge's own invariants, it just stores
///         whatever the setters are given and returns it verbatim so the
///         real, unmodified WalletProposalValidator bytecode can be
///         exercised end-to-end.
/// @dev Struct field orders below mirror Deposit.DepositRequest,
///      Wallets.Wallet, Reservation.ReservationRequest and
///      Reservation.ReservationAction in tbtc-v2 exactly: the validator
///      decodes these as ABI tuples, so the field order (not the struct or
///      field names) is what has to match.
contract StubBridge {
    struct DepositReq {
        address depositor;
        uint64 amount;
        uint32 revealedAt;
        address vault;
        uint64 treasuryFee;
        uint32 sweptAt;
        bytes32 extraData;
    }

    struct Wallet {
        bytes32 ecdsaWalletID;
        bytes32 mainUtxoHash;
        uint64 pendingRedemptionsValue;
        uint32 createdAt;
        uint32 movingFundsRequestedAt;
        uint32 closingStartedAt;
        uint32 pendingMovedFundsSweepRequestsCount;
        uint8 state;
        bytes32 movingFundsTargetWalletsCommitmentHash;
    }

    struct ResReq {
        address owner;
        uint64 mintedAmount;
        uint32 acceptedAt;
        bytes20 walletPubKeyHash;
        uint64 anchorAmount;
        uint32 expiresAt;
        bytes32 anchorTxHash;
        uint32 anchorTxOutputIndex;
        uint8 state;
        uint64 requestNonce;
        bool retryCredit;
        uint32 dissolutionEligibleAt;
        uint64 cumulativeReanchorFee;
        uint32 reanchorCooldownUntil;
    }

    struct Action {
        bytes20 targetWalletPubKeyHash;
        uint32 requestedAt;
        uint32 timeoutAt;
        uint64 txMaxFee;
        uint8 actionType;
        uint8 state;
        bool feePaid;
        address redeemer;
        bytes32 actionDataHash;
        bytes32 sourceAnchorUtxoHash;
        uint64 amount;
        bool usedRetryCredit;
        uint32 watchtowerDefaultDelay;
        uint32 watchtowerLevelOneDelay;
        uint32 watchtowerLevelTwoDelay;
        uint64 retryCreditSourceNonce;
        bool isPartial;
        uint32 termSeconds;
        uint32 dissolutionDelay;
        uint64 minAmount;
    }

    struct Parameters {
        address reservationVault;
        uint64 reservationMinAmount;
        uint64 reservationTxMaxFee;
        uint32 reservationTermSeconds;
        uint32 reservationDissolutionDelay;
        uint64 reservationMaxTotalAmount;
        uint64 reservationTotalAmount;
        uint32 maxReservationsPerWallet;
        uint32 reservationActionTimeout;
        uint32 reservationRenewalWindowSeconds;
    }

    struct Caps {
        uint64 maxReservationsAmountPerWallet;
        uint64 reservationMaxSingleAmount;
        uint32 maxActiveReservations;
    }

    mapping(uint256 => DepositReq) public deposits;
    mapping(uint256 => bool) public reserved;
    mapping(bytes20 => Wallet) public wallets;
    mapping(uint256 => ResReq) public reservations;
    mapping(uint256 => mapping(uint64 => Action)) public actions;
    mapping(bytes20 => uint32) public walletReservationsCounts;
    mapping(bytes20 => uint64) public walletReservationsAmounts;

    Parameters public params;
    Caps public caps;

    // ----- Setters (seeding surface for the Go test harness) -----

    function setDeposit(uint256 key, DepositReq calldata req) external {
        deposits[key] = req;
    }

    function setReservedDeposit(uint256 key, bool isReserved) external {
        reserved[key] = isReserved;
    }

    function setWallet(bytes20 pubKeyHash, Wallet calldata wallet) external {
        wallets[pubKeyHash] = wallet;
    }

    function setReservation(uint256 key, ResReq calldata req) external {
        reservations[key] = req;
    }

    function setReservationAction(
        uint256 key,
        uint64 nonce,
        Action calldata action
    ) external {
        actions[key][nonce] = action;
    }

    function setReservationParameters(Parameters calldata p) external {
        params = p;
    }

    function setReservationCaps(Caps calldata c) external {
        caps = c;
    }

    function setWalletReservationsCount(bytes20 pubKeyHash, uint32 count)
        external
    {
        walletReservationsCounts[pubKeyHash] = count;
    }

    function setWalletReservationsAmount(bytes20 pubKeyHash, uint64 amount)
        external
    {
        walletReservationsAmounts[pubKeyHash] = amount;
    }

    // ----- Views matching Bridge's / IReservationBridge's selectors -----

    function isReservedDeposit(uint256 depositKey)
        external
        view
        returns (bool)
    {
        return reserved[depositKey];
    }

    function reservationActions(uint256 reservationKey, uint64 requestNonce)
        external
        view
        returns (Action memory)
    {
        return actions[reservationKey][requestNonce];
    }

    function reservationParameters()
        external
        view
        returns (
            address,
            uint64,
            uint64,
            uint32,
            uint32,
            uint64,
            uint64,
            uint32,
            uint32,
            uint32
        )
    {
        Parameters memory p = params;
        return (
            p.reservationVault,
            p.reservationMinAmount,
            p.reservationTxMaxFee,
            p.reservationTermSeconds,
            p.reservationDissolutionDelay,
            p.reservationMaxTotalAmount,
            p.reservationTotalAmount,
            p.maxReservationsPerWallet,
            p.reservationActionTimeout,
            p.reservationRenewalWindowSeconds
        );
    }

    function reservationCaps()
        external
        view
        returns (
            uint64,
            uint64,
            uint32
        )
    {
        Caps memory c = caps;
        return (
            c.maxReservationsAmountPerWallet,
            c.reservationMaxSingleAmount,
            c.maxActiveReservations
        );
    }

    function walletReservationsCount(bytes20 pubKeyHash)
        external
        view
        returns (uint32)
    {
        return walletReservationsCounts[pubKeyHash];
    }

    function walletReservationsAmount(bytes20 pubKeyHash)
        external
        view
        returns (uint64)
    {
        return walletReservationsAmounts[pubKeyHash];
    }
}
