pragma solidity ^0.8.0;

import "../libraries/RolesLookup.sol";

contract RolesLookupStub {
    TokenStaking internal tokenStaking;
    TokenGrant internal tokenGrant;

    constructor(TokenStaking _tokenStaking, TokenGrant _tokenGrant) {
        tokenStaking = _tokenStaking;
        tokenGrant = _tokenGrant;
    }

    function isTokenOwnerForOperator(address tokenOwner, address operator)
        public
        view
        returns (bool)
    {
        return
            RolesLookup.isTokenOwnerForOperator(
                tokenOwner,
                operator,
                tokenStaking
            );
    }

    function isGranteeForOperator(address grantee, address operator)
        public
        view
        returns (bool)
    {
        return RolesLookup.isGranteeForOperator(grantee, operator, tokenGrant);
    }

    function isManagedGranteeForOperator(
        address grantee,
        address operator,
        address granteeContract
    ) public view returns (bool) {
        return
            RolesLookup.isManagedGranteeForOperator(
                grantee,
                operator,
                granteeContract,
                tokenGrant
            );
    }

    function isManagedGranteeForGrant(address grantee, uint256 grantId)
        public
        returns (bool)
    {
        return
            RolesLookup.isManagedGranteeForGrant(grantee, grantId, tokenGrant);
    }
}
