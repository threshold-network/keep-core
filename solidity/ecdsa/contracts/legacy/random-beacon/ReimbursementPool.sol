// SPDX-License-Identifier: GPL-3.0-only
//
// ▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
//   ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
// ▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
//
//                           Trust math, not hardware.

pragma solidity 0.8.17;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

contract ReimbursementPool is Ownable, ReentrancyGuard {
    /// @notice Authorized contracts that can interact with the reimbursment pool.
    ///         Authorization can be granted and removed by the owner.
    mapping(address => bool) public isAuthorized;

    /// @notice Static gas includes:
    ///         - cost of the refund function
    ///         - base transaction cost
    uint256 public staticGas;

    /// @notice Max gas price used to reimburse a transaction submitter. Protects
    ///         against malicious operator-miners.
    uint256 public maxGasPrice;

    event StaticGasUpdated(uint256 newStaticGas);

    event MaxGasPriceUpdated(uint256 newMaxGasPrice);

    event SendingEtherFailed(uint256 refundAmount, address receiver);

    event AuthorizedContract(address thirdPartyContract);

    event UnauthorizedContract(address thirdPartyContract);

    event FundsWithdrawn(uint256 withdrawnAmount, address receiver);

    constructor(uint256 _staticGas, uint256 _maxGasPrice) {
        staticGas = _staticGas;
        maxGasPrice = _maxGasPrice;
    }

    /// @notice Receive ETH
    receive() external payable {}

    /// @notice Refunds ETH to a spender for executing specific transactions.
    /// @dev Ignoring the result of sending ETH to a receiver is made on purpose.
    ///      For EOA receiving ETH should always work. If a receiver is a smart
    ///      contract, then we do not want to fail a transaction, because in some
    ///      cases the refund is done at the very end of multiple calls where all
    ///      the previous calls were already paid off. It is a receiver's smart
    ///      contract resposibility to make sure it can receive ETH.
    /// @dev Only authorized contracts are allowed calling this function.
    /// @param gasSpent Gas spent on a transaction that needs to be reimbursed.
    /// @param receiver Address where the reimbursment is sent.
    function refund(uint256 gasSpent, address receiver) external nonReentrant {
        require(
            isAuthorized[msg.sender],
            "Contract is not authorized for a refund"
        );
        require(receiver != address(0), "Receiver's address cannot be zero");

        uint256 gasPrice = tx.gasprice < maxGasPrice
            ? tx.gasprice
            : maxGasPrice;

        uint256 refundAmount = (gasSpent + staticGas) * gasPrice;

        /* solhint-disable avoid-low-level-calls */
        // slither-disable-next-line low-level-calls,unchecked-lowlevel
        (bool sent, ) = receiver.call{value: refundAmount}("");
        /* solhint-enable avoid-low-level-calls */
        if (!sent) {
            // slither-disable-next-line reentrancy-events
            emit SendingEtherFailed(refundAmount, receiver);
        }
    }

    /// @notice Authorize a contract that can interact with this reimbursment pool.
    ///         Can be authorized by the owner only.
    /// @param _contract Authorized contract.
    function authorize(address _contract) external onlyOwner {
        isAuthorized[_contract] = true;

        emit AuthorizedContract(_contract);
    }

    /// @notice Unauthorize a contract that was previously authorized to interact
    ///         with this reimbursment pool. Can be unauthorized by the
    ///         owner only.
    /// @param _contract Authorized contract.
    function unauthorize(address _contract) external onlyOwner {
        delete isAuthorized[_contract];

        emit UnauthorizedContract(_contract);
    }

    /// @notice Setting a static gas cost for executing a transaction. Can be set
    ///         by the owner only.
    /// @param _staticGas Static gas cost.
    function setStaticGas(uint256 _staticGas) external onlyOwner {
        staticGas = _staticGas;

        emit StaticGasUpdated(_staticGas);
    }

    /// @notice Setting a max gas price for transactions. Can be set by the
    ///         owner only.
    /// @param _maxGasPrice Max gas price used to reimburse tx submitters.
    function setMaxGasPrice(uint256 _maxGasPrice) external onlyOwner {
        maxGasPrice = _maxGasPrice;

        emit MaxGasPriceUpdated(_maxGasPrice);
    }

    /// @notice Withdraws all ETH from this pool which are sent to a given
    ///         address. Can be set by the owner only.
    /// @param receiver An address where ETH is sent.
    function withdrawAll(address receiver) external onlyOwner {
        withdraw(address(this).balance, receiver);
    }

    /// @notice Withdraws ETH amount from this pool which are sent to a given
    ///         address. Can be set by the owner only.
    /// @param amount Amount to withdraw from the pool.
    /// @param receiver An address where ETH is sent.
    function withdraw(uint256 amount, address receiver) public onlyOwner {
        require(
            address(this).balance >= amount,
            "Insufficient contract balance"
        );
        require(receiver != address(0), "Receiver's address cannot be zero");

        emit FundsWithdrawn(amount, receiver);

        /* solhint-disable avoid-low-level-calls */
        // slither-disable-next-line low-level-calls,arbitrary-send
        (bool sent, ) = receiver.call{value: amount}("");
        /* solhint-enable avoid-low-level-calls */
        require(sent, "Failed to send Ether");
    }
}
