pragma solidity ^0.8.0;

import "../TokenStaking.sol";

contract DelegatedAuthorityStub {
    address recognizedContract;

    constructor(address _recognizedContract) {
        recognizedContract = _recognizedContract;
    }

    function __isRecognized(address _contract) public view returns (bool) {
        return _contract == recognizedContract;
    }

    function claimAuthorityRecursively(address stakingContract, address source)
        public
    {
        TokenStaking(stakingContract).claimDelegatedAuthority(source);
    }
}
