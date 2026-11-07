// SPDX-License-Identifier: GPL-3.0-only
//

pragma solidity 0.8.17;

import "@keep-network/random-beacon/contracts/api/IRandomBeacon.sol";
import "@keep-network/random-beacon/contracts/api/IRandomBeaconConsumer.sol";
import "@keep-network/random-beacon/contracts/libraries/Callback.sol";

// TODO: get rid of this contract; use RandomBeacon implementation instead.
// This implementation is used to test callback's gas limit only. In most tests
// we use smock's FakeContract of IRandomBeacon.
contract RandomBeaconStub is IRandomBeacon {
    using Callback for Callback.Data;
    Callback.Data internal callback;

    // This value has to reflect the one set in the Random Beacon contract!
    uint256 public callbackGasLimit = 64000;

    event CallbackFailed(uint256 entry, uint256 entrySubmittedBlock);

    function requestRelayEntry(IRandomBeaconConsumer _callbackContract)
        external
    {
        callback.setCallbackContract(_callbackContract);
    }

    function submitRelayEntry(bytes calldata entry) external {
        callback.executeCallback(uint256(keccak256(entry)), callbackGasLimit);
    }
}
