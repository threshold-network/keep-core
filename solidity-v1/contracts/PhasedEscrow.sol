pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

import "./Escrow.sol";

interface IBeneficiaryContract {
    function __escrowSentTokens(uint256 amount) external;
}

/// @title PhasedEscrow
/// @notice A token holder contract allowing contract owner to set beneficiary of
///         tokens held by the contract and allowing the owner to withdraw the
///         tokens to that beneficiary in phases.
contract PhasedEscrow is Ownable {
    using SafeERC20 for IERC20;

    event BeneficiaryUpdated(address beneficiary);
    event TokensWithdrawn(address beneficiary, uint256 amount);

    IERC20 public token;
    IBeneficiaryContract public beneficiary;

    constructor(IERC20 _token) {
        token = _token;
    }

    /// @notice Sets the provided address as a beneficiary allowing it to
    ///         withdraw all tokens from escrow. This function can be called only
    ///         by escrow owner.
    function setBeneficiary(IBeneficiaryContract _beneficiary)
        external
        onlyOwner
    {
        beneficiary = _beneficiary;
        emit BeneficiaryUpdated(address(beneficiary));
    }

    /// @notice Withdraws the specified number of tokens from escrow to the
    ///         beneficiary. If the beneficiary is not set, or there are
    ///         insufficient tokens in escrow, the function fails.
    function withdraw(uint256 amount) external onlyOwner {
        require(address(beneficiary) != address(0), "Beneficiary not assigned");

        uint256 balance = token.balanceOf(address(this));
        require(amount <= balance, "Not enough tokens for withdrawal");

        token.safeTransfer(address(beneficiary), amount);
        emit TokensWithdrawn(address(beneficiary), amount);

        beneficiary.__escrowSentTokens(amount);
    }

    /// @notice Funds the escrow by transferring all of the approved tokens
    ///         to the escrow.
    function receiveApproval(
        address _from,
        uint256 _value,
        address _token,
        bytes memory
    ) public {
        require(IERC20(_token) == token, "Unsupported token");
        token.safeTransferFrom(_from, address(this), _value);
    }

    /// @notice Withdraws all funds from a non-phased Escrow passed as
    ///         a parameter. For this function to succeed, this PhasedEscrow
    ///         has to be set as a beneficiary of the non-phased Escrow.
    function withdrawFromEscrow(Escrow _escrow) public {
        _escrow.withdraw();
    }
}

/// @title BatchedPhasedEscrow
/// @notice A token holder contract allowing contract owner to approve a set of
///         beneficiaries of tokens held by the contract, to appoint a separate
///         drawee role, and allowing that drawee to withdraw tokens to approved
///         beneficiaries in phases.
contract BatchedPhasedEscrow is Ownable {
    using SafeERC20 for IERC20;

    event BeneficiaryApproved(address beneficiary);
    event TokensWithdrawn(address beneficiary, uint256 amount);
    event DraweeRoleTransferred(address oldDrawee, address newDrawee);

    IERC20 public token;
    address public drawee;
    mapping(address => bool) private approvedBeneficiaries;

    modifier onlyDrawee() {
        require(drawee == msg.sender, "Caller is not the drawee");
        _;
    }

    constructor(IERC20 _token) {
        token = _token;
        drawee = msg.sender;
    }

    /// @notice Approves the provided address as a beneficiary of tokens held by
    ///         the escrow. Can be called only by escrow owner.
    function approveBeneficiary(IBeneficiaryContract _beneficiary)
        external
        onlyOwner
    {
        address beneficiaryAddress = address(_beneficiary);
        require(
            beneficiaryAddress != address(0),
            "Beneficiary can not be zero address"
        );
        approvedBeneficiaries[beneficiaryAddress] = true;
        emit BeneficiaryApproved(beneficiaryAddress);
    }

    /// @notice Returns `true` if the given address has been approved as a
    ///         beneficiary of the escrow, `false` otherwise.
    function isBeneficiaryApproved(IBeneficiaryContract _beneficiary)
        public
        view
        returns (bool)
    {
        return approvedBeneficiaries[address(_beneficiary)];
    }

    /// @notice Transfers the role of drawee to another address. Can be called
    ///         only by the contract owner.
    function setDrawee(address newDrawee) public onlyOwner {
        require(newDrawee != address(0), "New drawee can not be zero address");
        emit DraweeRoleTransferred(drawee, newDrawee);
        drawee = newDrawee;
    }

    /// @notice Funds the escrow by transferring all of the approved tokens
    ///         to the escrow.
    function receiveApproval(
        address _from,
        uint256 _value,
        address _token,
        bytes memory
    ) public {
        require(IERC20(_token) == token, "Unsupported token");
        token.safeTransferFrom(_from, address(this), _value);
    }

    /// @notice Withdraws tokens from escrow to selected beneficiaries,
    ///         transferring to each beneficiary the amount of tokens specified
    ///         as a parameter. Only beneficiaries previously approved by escrow
    ///         owner can receive funds.
    function batchedWithdraw(
        IBeneficiaryContract[] memory beneficiaries,
        uint256[] memory amounts
    ) public onlyDrawee {
        require(
            beneficiaries.length == amounts.length,
            "Mismatched arrays length"
        );

        for (uint256 i = 0; i < beneficiaries.length; i++) {
            IBeneficiaryContract beneficiary = beneficiaries[i];
            require(
                isBeneficiaryApproved(beneficiary),
                "Beneficiary was not approved"
            );
            withdraw(beneficiary, amounts[i]);
        }
    }

    function withdraw(IBeneficiaryContract beneficiary, uint256 amount)
        private
    {
        token.safeTransfer(address(beneficiary), amount);
        emit TokensWithdrawn(address(beneficiary), amount);
        beneficiary.__escrowSentTokens(amount);
    }
}

// Interface representing staking pool rewards contract such as CurveRewards
// contract deployed for Keep (0xAF379f0228ad0d46bB7B4f38f9dc9bCC1ad0360c) or
// LPRewards contract from keep-ecdsa repository deployed for Uniswap.
interface IStakingPoolRewards {
    function notifyRewardAmount(uint256 amount) external;
}

/// @title PlainTransferEscrowBeneficiary
/// @notice  PlainTransferEscrowBeneficiary is a beneficiary contract that can
///          receive a withdrawal order from PhasedEscrow and BatchedPhasedEscrow
///          contracts. It immediately sends the received tokens to the recipient
///          set by the beneficiary deployer.
///          The recipient does not have to implement any custom functions, it can
///          be a smart contract or EOA - beneficiary is executing a plain ERC20
///          transfer.
contract PlainTransferEscrowBeneficiary is Ownable, IBeneficiaryContract {
    using SafeERC20 for IERC20;

    IERC20 public token;
    address public recipient;

    constructor(IERC20 _token, address _recipient) {
        token = _token;
        recipient = _recipient;
    }

    function __escrowSentTokens(uint256 amount) external onlyOwner {
        token.safeTransfer(recipient, amount);
    }
}

/// @title StakingPoolRewardsEscrowBeneficiary
/// @notice A beneficiary contract that can receive a withdrawal phase from a
///         PhasedEscrow contract. Immediately stakes the received tokens on a
///         designated IStakingPoolRewards contract.
contract StakingPoolRewardsEscrowBeneficiary is Ownable, IBeneficiaryContract {
    IERC20 public token;
    IStakingPoolRewards public rewards;

    constructor(IERC20 _token, IStakingPoolRewards _rewards) {
        token = _token;
        rewards = _rewards;
    }

    function __escrowSentTokens(uint256 amount) external onlyOwner {
        token.approve(address(rewards), amount);
        rewards.notifyRewardAmount(amount);
    }
}

/// @dev Interface of recipient contract for approveAndCall pattern.
interface IStakerRewards {
    function receiveApproval(
        address _from,
        uint256 _value,
        address _token,
        bytes calldata _extraData
    ) external;
}

/// @title StakerRewardsBeneficiary
/// @notice An abstract beneficiary contract that can receive a withdrawal phase
///         from a PhasedEscrow contract. The received tokens are immediately
///         funded for a designated rewards escrow beneficiary contract.
contract StakerRewardsBeneficiary is Ownable {
    IERC20 public token;
    IStakerRewards public stakerRewards;

    constructor(IERC20 _token, IStakerRewards _stakerRewards) {
        token = _token;
        stakerRewards = _stakerRewards;
    }

    function __escrowSentTokens(uint256 amount) external onlyOwner {
        bool success = token.approve(address(stakerRewards), amount);
        require(success, "Token transfer approval failed");

        stakerRewards.receiveApproval(
            address(this),
            amount,
            address(token),
            ""
        );
    }
}

/// @title BeaconBackportRewardsEscrowBeneficiary
/// @notice Transfer the received tokens to a designated
///         BeaconBackportRewardsEscrowBeneficiary contract.
contract BeaconBackportRewardsEscrowBeneficiary is StakerRewardsBeneficiary {
    constructor(IERC20 _token, IStakerRewards _stakerRewards)
        public
        StakerRewardsBeneficiary(_token, _stakerRewards)
    {}
}

/// @title BeaconRewardsEscrowBeneficiary
/// @notice Transfer the received tokens to a designated
///         BeaconRewardsEscrowBeneficiary contract.
contract BeaconRewardsEscrowBeneficiary is StakerRewardsBeneficiary {
    constructor(IERC20 _token, IStakerRewards _stakerRewards)
        public
        StakerRewardsBeneficiary(_token, _stakerRewards)
    {}
}
