pragma solidity ^0.8.0;

import "../libraries/staking/GrantStaking.sol";

contract GrantStakingStub {
    using GrantStaking for GrantStaking.Storage;
    GrantStaking.Storage grantStaking;

    function hasGrantDelegated(address operator) public view returns (bool) {
        return grantStaking.hasGrantDelegated(operator);
    }

    function setGrantForOperator(address operator, uint256 grantId) public {
        grantStaking.setGrantForOperator(operator, grantId);
    }

    function getGrantForOperator(address operator)
        public
        view
        returns (uint256)
    {
        return grantStaking.getGrantForOperator(operator);
    }
}
