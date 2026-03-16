pragma solidity ^0.8.0;

/// @dev Stub used in TokenStakingEscrow to test migration of tokens between
/// escrows.
contract ReceivingEscrowStub {
    struct Received {
        uint256 grantId;
        uint256 amount;
    }

    mapping(address => Received) public received;

    function receiveApproval(
        address from,
        uint256 value,
        address token,
        bytes memory extraData
    ) public {
        (address operator, uint256 grantId) =
            abi.decode(extraData, (address, uint256));
        received[operator] = Received(grantId, value);
    }

    function depositedAmount(address operator) public view returns (uint256) {
        return received[operator].amount;
    }

    function depositGrantId(address operator) public view returns (uint256) {
        return received[operator].grantId;
    }
}
