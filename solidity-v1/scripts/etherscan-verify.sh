#!/bin/bash

set -e

echo "Verifying contracts on Etherscan..."

npx truffle run verify \
    AltBn128 \
    BeaconRewards \
    BLS \
    DelayFactor \
    DKGResultVerification \
    GasPriceOracle \
    GrantStaking \
    Groups \
    GroupSelection \
    GuaranteedMinimumStakingPolicy \
    KeepRandomBeaconOperator \
    KeepRandomBeaconOperatorStatistics \
    KeepRandomBeaconService \
    KeepRandomBeaconServiceImplV1 \
    KeepRegistry \
    KeepToken \
    KeepVault \
    Locks \
    ManagedGrantFactory \
    Migrations \
    MinimumStakeSchedule \
    ModUtils \
    OldTokenStaking \
    PermissiveStakingPolicy \
    Reimbursements \
    StakingPortBacker \
    TokenGrant \
    TokenStaking \
    TopUps \
    --network $TRUFFLE_NETWORK
