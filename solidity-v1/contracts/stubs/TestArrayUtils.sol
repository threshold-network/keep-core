pragma solidity ^0.8.0;

import "../utils/AddressArrayUtils.sol";
import "../utils/UintArrayUtils.sol";

contract TestArrayUtils {
    using AddressArrayUtils for address[];
    using UintArrayUtils for uint256[];

    address[][4] public addressArrays;
    uint256[] public values;

    function runCanHandleEmptyArrayTest() public {
        address[] storage addresses = addressArrays[0];
        addresses.removeAddress(0x2222222222222222222222222222222222222222);
        require(
            addresses.length == 0,
            "Empty array should stay unchanged on attempt to remove address from it."
        );
    }

    function runCanRemoveAddressFromSingleElementArrayTest() public {
        address[] storage addresses = addressArrays[1];
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.removeAddress(0x2222222222222222222222222222222222222222);
        require(
            addresses.length == 0,
            "Occurrence of address in a single element array should be removed."
        );
    }

    function runCanRemoveIdenticalAddressesTest() public {
        address[] storage addresses = addressArrays[2];
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.removeAddress(0x2222222222222222222222222222222222222222);
        require(
            addresses.length == 0,
            "All occurrences should be removed and array length should become 0."
        );
    }

    function runCanRemoveAddressTest() public {
        address[] storage addresses = addressArrays[3];
        bool exists = false;
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.push(0x1111111111111111111111111111111111111111);
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.push(0x2222222222222222222222222222222222222222);
        addresses.push(0x3333333333333333333333333333333333333333);
        addresses.push(0x2222222222222222222222222222222222222222);

        addresses.removeAddress(0x2222222222222222222222222222222222222222);
        for (uint256 i = 0; i < addresses.length; i++) {
            if (addresses[i] == 0x2222222222222222222222222222222222222222) {
                exists = true;
            }
        }

        require(
            !exists,
            "All occurrences of the address should be removed from the array."
        );
        require(
            addresses.length == 2,
            "Array length should change accordingly."
        );
    }

    function runCanHandleEmptyValueArrayTest() public {
        values.removeValue(2);
        require(
            values.length == 0,
            "Empty array should stay unchanged on attempt to remove value from it."
        );
    }

    function runCanRemoveValueFromSingleElementArrayTest() public {
        values = [2];
        values.removeValue(2);
        require(
            values.length == 0,
            "Occurrence of a value in a single element array should be removed."
        );
    }

    function runCanRemoveIdenticalValuesTest() public {
        values = [2, 2, 2];
        values.removeValue(2);
        require(
            values.length == 0,
            "All occurrences should be removed and array length should become 0."
        );
    }

    function runCanRemoveValueTest() public {
        bool exists = false;
        values = [2, 1, 2, 2, 3, 2];

        values.removeValue(2);
        for (uint256 i = 0; i < values.length; i++) {
            if (values[i] == 2) {
                exists = true;
            }
        }

        require(
            !exists,
            "All occurrences of the value should be removed from the array."
        );
        require(values.length == 2, "Array length should change accordingly.");
    }
}
