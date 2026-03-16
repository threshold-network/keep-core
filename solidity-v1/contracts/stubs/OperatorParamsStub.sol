pragma solidity ^0.8.0;
import "../utils/OperatorParams.sol";

contract OperatorParamsStub {
    using OperatorParams for uint256;

    function publicPack(
        uint256 amount,
        uint256 createdAt,
        uint256 undelegatedAt
    ) public pure returns (uint256) {
        return OperatorParams.pack(amount, createdAt, undelegatedAt);
    }

    function publicGetAmount(uint256 packed) public pure returns (uint256) {
        return packed.getAmount();
    }

    function publicSetAmount(uint256 packed, uint256 amount)
        public
        pure
        returns (uint256)
    {
        return packed.setAmount(amount);
    }

    function publicGetCreationTimestamp(uint256 packed)
        public
        pure
        returns (uint256)
    {
        return packed.getCreationTimestamp();
    }

    function publicSetCreationTimestamp(
        uint256 packed,
        uint256 creationTimestamp
    ) public pure returns (uint256) {
        return packed.setCreationTimestamp(creationTimestamp);
    }

    function publicGetUndelegationTimestamp(uint256 packed)
        public
        pure
        returns (uint256)
    {
        return packed.getUndelegationTimestamp();
    }

    function publicSetUndelegationTimestamp(
        uint256 packed,
        uint256 undelegationTimestamp
    ) public pure returns (uint256) {
        return packed.setUndelegationTimestamp(undelegationTimestamp);
    }

    function publicSetAmountAndCreationTimestamp(
        uint256 packed,
        uint256 amount,
        uint256 creationTimestamp
    ) public pure returns (uint256) {
        return packed.setAmountAndCreationTimestamp(amount, creationTimestamp);
    }
}
