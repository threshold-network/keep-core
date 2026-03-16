pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

import "./utils/BytesLib.sol";

import "./KeepToken.sol";
import "./ManagedGrant.sol";
import "./TokenGrant.sol";
import "./GrantStakingPolicy.sol";

/// @title ManagedGrantFactory
/// @notice Creates managed grants that permit grantee reassignment
/// and use pre-defined staking policies.
contract ManagedGrantFactory {
    using SafeERC20 for KeepToken;
    using BytesLib for bytes;
    using BytesLib for address;

    KeepToken public token;
    TokenGrant public tokenGrant;

    struct Params {
        address grantCreator;
        address grantee;
        uint256 amount;
        uint256 duration;
        uint256 start;
        uint256 cliffDuration;
        bool revocable;
        address policy;
    }

    event ManagedGrantCreated(address grantAddress, address indexed grantee);

    constructor(address _tokenAddress, address _tokenGrant) {
        token = KeepToken(_tokenAddress);
        tokenGrant = TokenGrant(_tokenGrant);
    }

    /// @notice Create a managed grant
    /// with the parameters specified in `_extraData`.
    /// @dev Requires no setup beforehand,
    /// but only provides the managed grant address through an event.
    /// The sender of the tokens is assigned as the grant manager.
    /// @param _from The owner of the tokens who approved them to transfer.
    /// @param _amount Approved amount for the transfer to create the grant.
    /// @param _token Address of the token contract;
    /// must match the token specified when the factory was created.
    /// @param _extraData The following values encoded with `abi.encode`:
    /// grantee (address) Address of the grantee.
    /// duration (uint256) Duration in seconds of the unlocking period.
    /// start (uint256) Timestamp at which unlocking will start.
    /// cliffDuration (uint256) Duration in seconds of the cliff before which no tokens will unlock.
    /// revocable (bool) Whether the token grant is revocable or not.
    /// policy (address) Address of the staking policy to be used.
    function receiveApproval(
        address _from,
        uint256 _amount,
        address _token,
        bytes memory _extraData
    ) public {
        require(KeepToken(_token) == token, "Invalid token contract");
        (
            address _grantee,
            uint256 _duration,
            uint256 _start,
            uint256 _cliffDuration,
            bool _revocable,
            address _policy
        ) =
            abi.decode(
                _extraData,
                (address, uint256, uint256, uint256, bool, address)
            );
        Params memory params =
            Params(
                _from,
                _grantee,
                _amount,
                _duration,
                _start,
                _cliffDuration,
                _revocable,
                _policy
            );
        _createGrant(params);
    }

    /// @notice Create a managed grant with the given parameters.
    /// @dev At least `amount` tokens to be approved for the factory beforehand.
    /// The grant will use the staking policy specified for its type
    /// (revocable or non-revocable)
    /// when the factory was created.
    /// The msg.sender is assigned as the grant manager.
    /// @param grantee The initial grantee.
    /// @param amount The number of tokens to grant.
    /// @param duration Duration in seconds of the unlocking period.
    /// @param start Timestamp at which unlocking will start.
    /// @param cliffDuration Duration in seconds of the cliff before which no tokens will unlock.
    /// @param revocable Whether the token grant is revocable or not.
    /// @param policy Address of the staking policy to be used.
    /// @return _managedGrant The address of the managed grant.
    function createManagedGrant(
        address grantee,
        uint256 amount,
        uint256 duration,
        uint256 start,
        uint256 cliffDuration,
        bool revocable,
        address policy
    ) public returns (address _managedGrant) {
        Params memory params =
            Params(
                msg.sender,
                grantee,
                amount,
                duration,
                start,
                cliffDuration,
                revocable,
                policy
            );
        return _createGrant(params);
    }

    function _createGrant(Params memory params)
        internal
        returns (address _managedGrant)
    {
        require(params.grantee != address(0), "Grantee address can't be zero.");
        require(
            params.cliffDuration <= params.duration,
            "Unlocking cliff duration must be less or equal total unlocking duration."
        );

        token.safeTransferFrom(
            params.grantCreator,
            address(this),
            params.amount
        );

        // Grant ID is predictable in advance
        uint256 grantId = tokenGrant.numGrants();

        ManagedGrant managedGrant =
            new ManagedGrant(
                address(token),
                address(tokenGrant),
                params.grantCreator,
                grantId,
                params.grantee
            );
        _managedGrant = address(managedGrant);

        bytes memory grantData =
            abi.encode(
                params.grantCreator,
                _managedGrant,
                params.duration,
                params.start,
                params.cliffDuration,
                params.revocable,
                params.policy
            );

        token.approveAndCall(address(tokenGrant), params.amount, grantData);

        emit ManagedGrantCreated(_managedGrant, params.grantee);
        return _managedGrant;
    }
}
