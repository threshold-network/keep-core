pragma solidity 0.8.17;

/// @title Chaosnet
/// @notice This is a beta staker program for stakers willing to go the extra
/// mile with monitoring, share their logs with the dev team, and allow to more
/// carefully monitor the bootstrapping network. As the network matures, the
/// beta program will be ended.
contract Chaosnet {
  /// @notice Indicates if the chaosnet is active. The chaosnet is active
  /// after the contract deployment and can be ended with a call to
  /// `deactivateChaosnet()`. Once deactivated chaosnet can not be activated
  /// again.
  bool public isChaosnetActive;

  /// @notice Indicates if the given operator is a beta operator for chaosnet.
  mapping(address => bool) public isBetaOperator;

  /// @notice Address controlling chaosnet status and beta operator addresses.
  address public chaosnetOwner;

  event BetaOperatorsAdded(address[] operators);

  event ChaosnetOwnerRoleTransferred(
    address oldChaosnetOwner,
    address newChaosnetOwner
  );

  event ChaosnetDeactivated();

  constructor() {
    _transferChaosnetOwner(msg.sender);
    isChaosnetActive = true;
  }

  modifier onlyChaosnetOwner() {
    require(msg.sender == chaosnetOwner, "Not the chaosnet owner");
    _;
  }

  modifier onlyOnChaosnet() {
    require(isChaosnetActive, "Chaosnet is not active");
    _;
  }

  /// @notice Adds beta operator to chaosnet. Can be called only by the
  /// chaosnet owner when the chaosnet is active. Once the operator is added
  /// as a beta operator, it can not be removed.
  function addBetaOperators(address[] calldata operators)
    public
    onlyOnChaosnet
    onlyChaosnetOwner
  {
    for (uint256 i = 0; i < operators.length; i++) {
      isBetaOperator[operators[i]] = true;
    }

    emit BetaOperatorsAdded(operators);
  }

  /// @notice Deactivates the chaosnet. Can be called only by the chaosnet
  /// owner. Once deactivated chaosnet can not be activated again.
  function deactivateChaosnet() public onlyOnChaosnet onlyChaosnetOwner {
    isChaosnetActive = false;
    emit ChaosnetDeactivated();
  }

  /// @notice Transfers the chaosnet owner role to another non-zero address.
  function transferChaosnetOwnerRole(address newChaosnetOwner)
    public
    onlyChaosnetOwner
  {
    require(
      newChaosnetOwner != address(0),
      "New chaosnet owner must not be zero address"
    );
    _transferChaosnetOwner(newChaosnetOwner);
  }

  function _transferChaosnetOwner(address newChaosnetOwner) internal {
    address oldChaosnetOwner = chaosnetOwner;
    chaosnetOwner = newChaosnetOwner;
    emit ChaosnetOwnerRoleTransferred(oldChaosnetOwner, newChaosnetOwner);
  }
}
