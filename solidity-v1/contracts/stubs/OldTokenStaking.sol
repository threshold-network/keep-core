pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "../utils/OperatorParams.sol";
import "../utils/BytesLib.sol";

/// Staking contract stub for testing purposes of copy stake flow.
contract OldTokenStaking {
    using OperatorParams for uint256;
    using BytesLib for bytes;
    ERC20Burnable public token;
    using SafeERC20 for ERC20Burnable;

    event Undelegated(address indexed operator, uint256 undelegatedAt);
    event Staked(address indexed from, uint256 value);

    mapping(address => address[]) public ownerOperators;
    mapping(address => Operator) public operators;

    struct Operator {
        uint256 packedParams;
        address owner;
        address payable beneficiary;
        address authorizer;
    }

    constructor(address _tokenAddress) {
        require(_tokenAddress != address(0x0), "Token address can't be zero.");
        token = ERC20Burnable(_tokenAddress);
    }

    function operatorsOf(address _address)
        public
        view
        returns (address[] memory)
    {
        return ownerOperators[_address];
    }

    function balanceOf(address _address) public view returns (uint256 balance) {
        return operators[_address].packedParams.getAmount();
    }

    function ownerOf(address _operator) public view returns (address) {
        return operators[_operator].owner;
    }

    function beneficiaryOf(address _operator)
        public
        view
        returns (address payable)
    {
        return operators[_operator].beneficiary;
    }

    function authorizerOf(address _operator) public view returns (address) {
        return operators[_operator].authorizer;
    }

    function getDelegationInfo(address _operator)
        public
        view
        returns (
            uint256 amount,
            uint256 createdAt,
            uint256 undelegatedAt
        )
    {
        return operators[_operator].packedParams.unpack();
    }

    function undelegationPeriod() public view returns (uint256) {
        return 5184000; // two months
    }

    function initializationPeriod() public view returns (uint256) {
        return 120;
    }

    function minimumStake() public view returns (uint256) {
        return 10000 * 1e18;
    }

    function undelegate(address _operator) public {
        uint256 oldParams = operators[_operator].packedParams;
        operators[_operator].packedParams = oldParams.setUndelegationTimestamp(
            block.timestamp
        );
        emit Undelegated(_operator, block.timestamp);
    }

    function setOperator(
        address _owner,
        address _operator,
        address payable _beneficiary,
        address _authorizer,
        uint256 _value
    ) public {
        operators[_operator] = Operator(
            OperatorParams.pack(_value, block.timestamp, 0),
            _owner,
            _beneficiary,
            _authorizer
        );
        ownerOperators[_owner].push(_operator);
    }

    function receiveApproval(
        address _from,
        uint256 _value,
        address _token,
        bytes memory _extraData
    ) public {
        require(
            ERC20Burnable(_token) == token,
            "Token contract must be the same one linked to this contract."
        );
        require(
            _value >= minimumStake(),
            "Tokens amount must be greater than the minimum stake"
        );
        require(
            _extraData.length == 60,
            "Stake delegation data must be provided."
        );

        address payable beneficiary = address(uint160(_extraData.toAddress(0)));
        address operator = _extraData.toAddress(20);
        require(
            operators[operator].owner == address(0),
            "Operator address is already in use."
        );
        address authorizer = _extraData.toAddress(40);

        // Transfer tokens to this contract.
        token.safeTransferFrom(_from, address(this), _value);

        operators[operator] = Operator(
            OperatorParams.pack(_value, block.timestamp, 0),
            _from,
            beneficiary,
            authorizer
        );
        ownerOperators[_from].push(operator);

        emit Staked(operator, _value);
    }
}
