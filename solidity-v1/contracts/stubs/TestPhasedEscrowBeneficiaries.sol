pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

// Simple beneficiary that does nothing when notified that it has received
// tokens.
contract TestSimpleBeneficiary {
    function __escrowSentTokens(uint256 amount) external {}
}

// CurveRewards contract mock for curvefi.
contract TestCurveRewards {
    using SafeERC20 for IERC20;

    IERC20 public token;

    event RewardAdded(uint256 reward);

    constructor(IERC20 _token) {
        token = _token;
    }

    function notifyRewardAmount(uint256 reward) external {
        token.safeTransferFrom(msg.sender, address(this), reward);
        emit RewardAdded(reward);
    }
}

// Simple reward contract mock for testing purposes.
contract TestSimpleStakerRewards {
    using SafeERC20 for IERC20;

    IERC20 public token;

    constructor(IERC20 _token) {
        token = _token;
    }

    function receiveApproval(
        address _from,
        uint256 _value,
        address _token,
        bytes memory
    ) public {
        token.safeTransferFrom(_from, address(this), _value);
    }
}

// Simple smart contract mock for testing purposes.
// Represents any contract receiving ERC20 tokens via plain transfer.
contract TestSimpleReceiver {

}
