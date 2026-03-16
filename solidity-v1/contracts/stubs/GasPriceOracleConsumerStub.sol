pragma solidity ^0.8.0;

import "../GasPriceOracle.sol";

contract GasPriceOracleConsumerStub is GasPriceOracleConsumer {
    GasPriceOracle gasPriceOracle;

    uint256 public gasPrice;

    constructor(GasPriceOracle _gasPriceOracle) {
        gasPriceOracle = _gasPriceOracle;
    }

    function refreshGasPrice() public {
        gasPrice = gasPriceOracle.gasPrice();
    }
}
