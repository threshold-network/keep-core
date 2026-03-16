/**
 This code is copied from:
 https://github.com/ampleforth/token-geyser/tree/d8352f62a0432494c39416d090e68582e13b2b22/contracts
 */
pragma solidity ^0.8.0;

/**
 * @title Staking interface, as defined by EIP-900.
 * @dev https://github.com/ethereum/EIPs/blob/master/EIPS/eip-900.md
 */
abstract contract IStaking {
    event Staked(
        address indexed user,
        uint256 amount,
        uint256 total,
        bytes data
    );
    event Unstaked(
        address indexed user,
        uint256 amount,
        uint256 total,
        bytes data
    );

    function stake(uint256 amount, bytes calldata data) external virtual;

    function stakeFor(
        address user,
        uint256 amount,
        bytes calldata data
    ) external virtual;

    function unstake(uint256 amount, bytes calldata data) external virtual;

    function token() external view virtual returns (address);

    /**
     * @return False. This application does not support staking history.
     */
    function supportsHistory() external pure returns (bool) {
        return false;
    }

    function totalStakedFor(address addr) public view virtual returns (uint256);

    function totalStaked() public view virtual returns (uint256);
}
