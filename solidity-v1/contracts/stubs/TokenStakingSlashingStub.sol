pragma solidity 0.5.17;

import "../TokenStaking.sol";
import "../TokenGrant.sol";
import "../KeepRegistry.sol";

contract TokenStakingSlashingStub is TokenStaking {
    constructor(
        ERC20Burnable _token,
        TokenGrant _tokenGrant,
        KeepRegistry _registry,
        uint256 _initializationPeriod
    )
        public
        TokenStaking(
            _token,
            _tokenGrant,
            _registry,
            _initializationPeriod
        )
    {}

    function slash(uint256 amountToSlash, address[] memory misbehavedOperators)
        public
    {
        for (uint256 i = 0; i < misbehavedOperators.length; i++) {
            address operator = misbehavedOperators[i];
            emit TokensSlashed(operator, 1 ether);
        }
    }

    function seize(
        uint256 amountToSeize,
        uint256 rewardMultiplier,
        address tattletale,
        address[] memory misbehavedOperators
    ) public {
        for (uint256 i = 0; i < misbehavedOperators.length; i++) {
            address operator = misbehavedOperators[i];
            emit TokensSeized(operator, 1 ether);
        }
    }
}
