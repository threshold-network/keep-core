pragma solidity ^0.8.0;

import "../TokenStaking.sol";
import "../TokenStakingEscrow.sol";
import "../TokenGrant.sol";
import "../KeepRegistry.sol";

contract TokenStakingSlashingStub is TokenStaking {
    constructor(
        ERC20Burnable _token,
        TokenGrant _tokenGrant,
        TokenStakingEscrow _escrow,
        KeepRegistry _registry,
        uint256 _initializationPeriod
    )
        public
        TokenStaking(
            _token,
            _tokenGrant,
            _escrow,
            _registry,
            _initializationPeriod
        )
    {}

    function slash(uint256 amountToSlash, address[] memory misbehavedOperators)
        public
        override
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
    ) public override {
        for (uint256 i = 0; i < misbehavedOperators.length; i++) {
            address operator = misbehavedOperators[i];
            emit TokensSeized(operator, 1 ether);
        }
    }
}
