pragma solidity ^0.8.0;

library UintArrayUtils {
    function removeValue(uint256[] storage self, uint256 _value)
        internal
        returns (uint256[] storage)
    {
        for (uint256 i = 0; i < self.length; i++) {
            // If value is found in array.
            if (_value == self[i]) {
                // Delete element at index and shift array.
                for (uint256 j = i; j < self.length - 1; j++) {
                    self[j] = self[j + 1];
                }
                self.length--;
                i--;
            }
        }
        return self;
    }
}
