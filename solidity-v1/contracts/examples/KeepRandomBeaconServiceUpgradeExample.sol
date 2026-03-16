pragma solidity ^0.8.0;

import "../KeepRandomBeaconServiceImplV1.sol";

/**
 * @title KeepRandomBeaconServiceUpgradeExample
 * @dev Example version of a new implementation contract to test upgradability
 * under Keep Random Beacon proxy.
 */
contract KeepRandomBeaconServiceUpgradeExample is
    KeepRandomBeaconServiceImplV1
{
    uint256 internal _newVar;

    constructor() {
        _initialized["KeepRandomBeaconImplV2"] = true;
    }

    /**
     * @dev Example of overriding existing function.
     * Reference http://solidity.readthedocs.io/en/develop/contracts.html#inheritance
     * >Functions can be overridden by another function with the same name and the
     * same number/types of inputs.
     */
    function initialize(uint256 dkgContributionMargin, address registry)
        public
        override
    {
        require(!initialized(), "Contract is already initialized.");
        require(registry != address(0), "Incorrect registry address");
        _initialized["KeepRandomBeaconImplV2"] = true;
        // Example of adding new data to the existing storage.
        _newVar = 1234;

        // silence solc warnings, it's just an example
        dkgContributionMargin;
        registry;
    }

    /**
     * @dev Example of overriding initialized function.
     */
    function initialized() public view override returns (bool) {
        return _initialized["KeepRandomBeaconImplV2"];
    }

    /**
     * @dev Example of adding a new function.
     */
    function getNewVar() public view returns (uint256) {
        return _newVar;
    }

    function version() public pure override returns (string memory) {
        return "V2";
    }
}
