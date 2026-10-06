pragma solidity ^0.8.6;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "../legacy/sortition/SortitionPool.sol";
import "../RandomBeacon.sol";
import "../libraries/Callback.sol";
import "../libraries/Groups.sol";
import {BeaconDkg as DKG} from "../libraries/BeaconDkg.sol";
import {BeaconDkgValidator as DKGValidator} from "../BeaconDkgValidator.sol";

contract RandomBeaconStub is RandomBeacon {
    using DKG for DKG.Data;
    using Groups for Groups.Data;

    constructor(
        SortitionPool _sortitionPool,
        IERC20 _tToken,
        IStaking _staking,
        DKGValidator _dkgValidator,
        ReimbursementPool _reimbursementPool
    )
        RandomBeacon(
            _sortitionPool,
            _tToken,
            _staking,
            _dkgValidator,
            _reimbursementPool
        )
    {}

    function getDkgData() external view returns (DKG.Data memory) {
        return dkg;
    }

    function getCallbackContract() external view returns (address) {
        return address(callback.callbackContract);
    }

    function roughlyAddGroup(
        bytes calldata groupPubKey,
        bytes32 groupMembersHash
    ) external {
        groups.addGroup(groupPubKey, groupMembersHash);
    }

    function roughlyTerminateGroup(uint64 groupId) public {
        groups.terminateGroup(groupId);
    }

    function dkgLockState() external {
        dkg.lockState();
    }
}
