# Allowlist Contract

The Allowlist contract replaces the TokenStaking contract as part of TIP-092 and TIP-100 governance decisions, enabling DAO-controlled beta staker selection without requiring token staking.

## Overview

The Allowlist contract maintains compatibility with the existing WalletRegistry interface while providing a simpler, DAO-controlled mechanism for managing beta stakers. Instead of requiring tokens to be staked, the DAO directly controls which operators can participate in the network and their respective weights.

## Key Features

- **DAO Control**: Only the contract owner (governance) can add staking providers or modify their weights
- **Weight Management**: Supports weight decrease operations with the same delay mechanism as TokenStaking
- **WalletRegistry Compatible**: Implements the same interface methods used by WalletRegistry
- **Beta Staker Consolidation**: Enables setting operator weights to zero for gradual consolidation
- **No Token Requirements**: Removes the need for operators to stake tokens

## Contract Functions

### Core Management

#### `addStakingProvider(address stakingProvider, uint96 weight)`
- **Access**: Owner only
- **Purpose**: Add a new staking provider with specified weight
- **Effects**: Calls `authorizationIncreased` on WalletRegistry
- **Reverts**: If provider already exists with non-zero weight

#### `requestWeightDecrease(address stakingProvider, uint96 newWeight)`
- **Access**: Owner only
- **Purpose**: Request weight decrease for existing provider
- **Effects**: Sets pending weight and calls `authorizationDecreaseRequested` on WalletRegistry
- **Notes**: Can set weight to zero for consolidation

#### `approveAuthorizationDecrease(address stakingProvider)`
- **Access**: WalletRegistry only
- **Purpose**: Approve previously requested weight decrease
- **Returns**: New weight value
- **Effects**: Updates provider weight and clears pending value

### Compatibility Interface

#### `authorizedStake(address stakingProvider, address application)`
- **Access**: Public view
- **Purpose**: Return current weight for the provider
- **Notes**: Second parameter (application) is ignored for compatibility

#### `seize(uint96 amount, uint256 rewardMultiplier, address notifier, address[] memory stakingProviders)`
- **Access**: Public
- **Purpose**: No-op seize operation (no tokens to seize)
- **Effects**: Only emits `MaliciousBehaviorIdentified` event

#### `rolesOf(address stakingProvider)`
- **Access**: Public view
- **Returns**: (owner, stakingProvider, address(0))
- **Purpose**: Return roles for compatibility

## Deployment Process

### 1. Deploy Allowlist Contract
```bash
npx hardhat deploy --tags Allowlist
```

### 2. Initialize with Existing Beta Stakers
```bash
MIGRATE_ALLOWLIST_WEIGHTS=true npx hardhat deploy --tags InitializeAllowlistWeights
```

### 3. Update WalletRegistry Integration
Update WalletRegistry constructor to use Allowlist instead of TokenStaking.

## Beta Staker Consolidation Workflow

### Phase 1: Migration
1. Deploy Allowlist contract
2. Initialize with current beta staker weights from TokenStaking
3. Verify all operators are properly migrated

### Phase 2: Consolidation
1. Identify redundant operators for each entity (Boar, P2P, Staked.us)
2. Use weight management script to set redundant operator weights to zero
3. Monitor natural fund drainage as redemptions occur

### Phase 3: Cleanup
1. Verify zero-weight operators have no remaining funds
2. Coordinate with operators to shut down redundant nodes
3. Confirm network operates correctly with ~20 operators instead of ~35

## Beta Staker Consolidation

#### Execute Full Consolidation
```bash
# Check current status
npx hardhat run scripts/consolidate_beta_stakers.ts -- status --allowlist <address>

# Dry run to see what would happen
npx hardhat run scripts/consolidate_beta_stakers.ts -- execute --allowlist <address> --dry-run

# Execute the consolidation (18 → 3 operators)
npx hardhat run scripts/consolidate_beta_stakers.ts -- execute --allowlist <address>
```

## Events

### `StakingProviderAdded(address indexed stakingProvider, uint96 weight)`
Emitted when a new staking provider is added to the allowlist.

### `WeightDecreaseRequested(address indexed stakingProvider, uint96 oldWeight, uint96 newWeight)`
Emitted when a weight decrease is requested.

### `WeightDecreaseFinalized(address indexed stakingProvider, uint96 oldWeight, uint96 newWeight)`
Emitted when a weight decrease is approved and finalized.

### `MaliciousBehaviorIdentified(address notifier, address[] stakingProviders)`
Emitted by the seize function for compatibility (no actual slashing occurs).

## Security Considerations

1. **Owner Control**: The contract owner has complete control over the operator set
2. **No Slashing**: Misbehavior cannot be punished through token slashing
3. **Weight Decreases**: Include delay mechanism to prevent sudden operator removal
4. **Gradual Changes**: Consolidation should be done gradually to maintain network stability

## Integration Notes

- WalletRegistry calls remain the same, ensuring minimal integration changes
- Authorization flow maintains existing delay mechanisms
- Event emissions preserve compatibility with monitoring systems
- No changes required to operator client software