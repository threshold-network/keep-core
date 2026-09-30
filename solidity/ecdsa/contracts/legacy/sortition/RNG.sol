pragma solidity 0.8.17;

import "./Leaf.sol";
import "./Constants.sol";

library RNG {
  /// @notice Get an index in the range `[0 .. range-1]`
  /// and the new state of the RNG,
  /// using the provided `state` of the RNG.
  ///
  /// @param range The upper bound of the index, exclusive.
  ///
  /// @param state The previous state of the RNG.
  /// The initial state needs to be obtained
  /// from a trusted randomness oracle (the random beacon),
  /// or from a chain of earlier calls to `RNG.getIndex()`
  /// on an originally trusted seed.
  ///
  /// @dev Calculates the number of bits required for the desired range,
  /// takes the least significant bits of `state`
  /// and checks if the obtained index is within the desired range.
  /// The original state is hashed with `keccak256` to get a new state.
  /// If the index is outside the range,
  /// the function retries until it gets a suitable index.
  ///
  /// @return index A random integer between `0` and `range - 1`, inclusive.
  ///
  /// @return newState The new state of the RNG.
  /// When `getIndex()` is called one or more times,
  /// care must be taken to always use the output `state`
  /// of the most recent call as the input `state` of a subsequent call.
  /// At the end of a transaction calling `RNG.getIndex()`,
  /// the previous stored state must be overwritten with the latest output.
  function getIndex(
    uint256 range,
    bytes32 state,
    uint256 bits
  ) internal view returns (uint256, bytes32) {
    bool found = false;
    uint256 index = 0;
    bytes32 newState = state;
    while (!found) {
      index = truncate(bits, uint256(newState));
      newState = keccak256(abi.encodePacked(newState, address(this)));
      if (index < range) {
        found = true;
      }
    }
    return (index, newState);
  }

  /// @notice Calculate how many bits are required
  /// for an index in the range `[0 .. range-1]`.
  ///
  /// @param range The upper bound of the desired range, exclusive.
  ///
  /// @return uint The smallest number of bits
  /// that can contain the number `range-1`.
  function bitsRequired(uint256 range) internal pure returns (uint256) {
    unchecked {
      if (range == 1) {
        return 0;
      }

      uint256 bits = Constants.WEIGHT_WIDTH - 1;

      // Left shift by `bits`,
      // so we have a 1 in the (bits + 1)th least significant bit
      // and 0 in other bits.
      // If this number is equal or greater than `range`,
      // the range [0, range-1] fits in `bits` bits.
      //
      // Because we loop from high bits to low bits,
      // we find the highest number of bits that doesn't fit the range,
      // and return that number + 1.
      while (1 << bits >= range) {
        bits--;
      }

      return bits + 1;
    }
  }

  /// @notice Truncate `input` to the `bits` least significant bits.
  function truncate(uint256 bits, uint256 input)
    internal
    pure
    returns (uint256)
  {
    unchecked {
      return input & ((1 << bits) - 1);
    }
  }
}
