// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/cmd"
	"github.com/keep-network/keep-core/pkg/decode"

	"github.com/spf13/cobra"
)

var BeaconSortitionPoolCommand *cobra.Command

var beaconSortitionPoolDescription = `The beacon-sortition-pool command allows calling the BeaconSortitionPool contract on an
	Ethereum network. It has subcommands corresponding to each contract method,
	which respectively each take parameters based on the contract method's
	parameters.

	Subcommands will submit a non-mutating call to the network and output the
	result.

	All subcommands can be called against a specific block by passing the
	-b/--block flag.

	Subcommands for mutating methods may be submitted as a mutating transaction
	by passing the -s/--submit flag. In this mode, this command will terminate
	successfully once the transaction has been submitted, but will not wait for
	the transaction to be included in a block. They return the transaction hash.

	Calls that require ether to be paid will get 0 ether by default, which can
	be changed by passing the -v/--value flag.`

func init() {
	BeaconSortitionPoolCommand := &cobra.Command{
		Use:   "beacon-sortition-pool",
		Short: `Provides access to the BeaconSortitionPool contract.`,
		Long:  beaconSortitionPoolDescription,
	}

	BeaconSortitionPoolCommand.AddCommand(
		bspCanRestoreRewardEligibilityCommand(),
		bspChaosnetOwnerCommand(),
		bspGetAvailableRewardsCommand(),
		bspGetIDOperatorCommand(),
		bspGetOperatorIDCommand(),
		bspGetPoolWeightCommand(),
		bspIneligibleEarnedRewardsCommand(),
		bspIsBetaOperatorCommand(),
		bspIsChaosnetActiveCommand(),
		bspIsEligibleForRewardsCommand(),
		bspIsLockedCommand(),
		bspIsOperatorInPoolCommand(),
		bspIsOperatorRegisteredCommand(),
		bspIsOperatorUpToDateCommand(),
		bspOperatorsInPoolCommand(),
		bspOwnerCommand(),
		bspPoolWeightDivisorCommand(),
		bspRewardTokenCommand(),
		bspRewardsEligibilityRestorableAtCommand(),
		bspSelectGroupCommand(),
		bspTotalWeightCommand(),
		bspDeactivateChaosnetCommand(),
		bspInsertOperatorCommand(),
		bspLockCommand(),
		bspReceiveApprovalCommand(),
		bspRenounceOwnershipCommand(),
		bspRestoreRewardEligibilityCommand(),
		bspTransferChaosnetOwnerRoleCommand(),
		bspTransferOwnershipCommand(),
		bspUnlockCommand(),
		bspUpdateOperatorStatusCommand(),
		bspWithdrawIneligibleCommand(),
		bspWithdrawRewardsCommand(),
	)

	ModuleCommand.AddCommand(BeaconSortitionPoolCommand)
}

/// ------------------- Const methods -------------------

func bspCanRestoreRewardEligibilityCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "can-restore-reward-eligibility [arg_operator]",
		Short:                 "Calls the view method canRestoreRewardEligibility on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspCanRestoreRewardEligibility,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspCanRestoreRewardEligibility(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.CanRestoreRewardEligibilityAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspChaosnetOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "chaosnet-owner",
		Short:                 "Calls the view method chaosnetOwner on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspChaosnetOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspChaosnetOwner(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.ChaosnetOwnerAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspGetAvailableRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-available-rewards [arg_operator]",
		Short:                 "Calls the view method getAvailableRewards on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspGetAvailableRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspGetAvailableRewards(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetAvailableRewardsAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspGetIDOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-i-d-operator [arg_id]",
		Short:                 "Calls the view method getIDOperator on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspGetIDOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspGetIDOperator(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_id, err := decode.ParseUint[uint32](args[0], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_id, a uint32, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetIDOperatorAtBlock(
		arg_id,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspGetOperatorIDCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-operator-i-d [arg_operator]",
		Short:                 "Calls the view method getOperatorID on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspGetOperatorID,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspGetOperatorID(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetOperatorIDAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspGetPoolWeightCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-pool-weight [arg_operator]",
		Short:                 "Calls the view method getPoolWeight on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspGetPoolWeight,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspGetPoolWeight(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetPoolWeightAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIneligibleEarnedRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "ineligible-earned-rewards",
		Short:                 "Calls the view method ineligibleEarnedRewards on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspIneligibleEarnedRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIneligibleEarnedRewards(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.IneligibleEarnedRewardsAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsBetaOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-beta-operator [arg0]",
		Short:                 "Calls the view method isBetaOperator on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspIsBetaOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsBetaOperator(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg0, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg0, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.IsBetaOperatorAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsChaosnetActiveCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-chaosnet-active",
		Short:                 "Calls the view method isChaosnetActive on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspIsChaosnetActive,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsChaosnetActive(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.IsChaosnetActiveAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsEligibleForRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-eligible-for-rewards [arg_operator]",
		Short:                 "Calls the view method isEligibleForRewards on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspIsEligibleForRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsEligibleForRewards(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.IsEligibleForRewardsAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsLockedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-locked",
		Short:                 "Calls the view method isLocked on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspIsLocked,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsLocked(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.IsLockedAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsOperatorInPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-in-pool [arg_operator]",
		Short:                 "Calls the view method isOperatorInPool on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspIsOperatorInPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsOperatorInPool(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.IsOperatorInPoolAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsOperatorRegisteredCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-registered [arg_operator]",
		Short:                 "Calls the view method isOperatorRegistered on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspIsOperatorRegistered,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsOperatorRegistered(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.IsOperatorRegisteredAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspIsOperatorUpToDateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-up-to-date [arg_operator] [arg_authorizedStake]",
		Short:                 "Calls the view method isOperatorUpToDate on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bspIsOperatorUpToDate,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspIsOperatorUpToDate(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}
	arg_authorizedStake, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_authorizedStake, a uint256, from passed value %v",
			args[1],
		)
	}

	result, err := contract.IsOperatorUpToDateAtBlock(
		arg_operator,
		arg_authorizedStake,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspOperatorsInPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "operators-in-pool",
		Short:                 "Calls the view method operatorsInPool on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspOperatorsInPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspOperatorsInPool(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.OperatorsInPoolAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspOwner(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.OwnerAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspPoolWeightDivisorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pool-weight-divisor",
		Short:                 "Calls the view method poolWeightDivisor on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspPoolWeightDivisor,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspPoolWeightDivisor(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.PoolWeightDivisorAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspRewardTokenCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reward-token",
		Short:                 "Calls the view method rewardToken on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspRewardToken,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspRewardToken(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.RewardTokenAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspRewardsEligibilityRestorableAtCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "rewards-eligibility-restorable-at [arg_operator]",
		Short:                 "Calls the view method rewardsEligibilityRestorableAt on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspRewardsEligibilityRestorableAt,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspRewardsEligibilityRestorableAt(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.RewardsEligibilityRestorableAtAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspSelectGroupCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "select-group [arg_groupSize] [arg_seed]",
		Short:                 "Calls the view method selectGroup on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bspSelectGroup,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspSelectGroup(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_groupSize, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_groupSize, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_seed, err := decode.ParseBytes32(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_seed, a bytes32, from passed value %v",
			args[1],
		)
	}

	result, err := contract.SelectGroupAtBlock(
		arg_groupSize,
		arg_seed,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bspTotalWeightCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "total-weight",
		Short:                 "Calls the view method totalWeight on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspTotalWeight,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bspTotalWeight(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	result, err := contract.TotalWeightAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func bspDeactivateChaosnetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "deactivate-chaosnet",
		Short:                 "Calls the nonpayable method deactivateChaosnet on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspDeactivateChaosnet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspDeactivateChaosnet(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.DeactivateChaosnet()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDeactivateChaosnet(
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspInsertOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "insert-operator [arg_operator] [arg_authorizedStake]",
		Short:                 "Calls the nonpayable method insertOperator on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bspInsertOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspInsertOperator(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}
	arg_authorizedStake, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_authorizedStake, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.InsertOperator(
			arg_operator,
			arg_authorizedStake,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInsertOperator(
			arg_operator,
			arg_authorizedStake,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspLockCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "lock",
		Short:                 "Calls the nonpayable method lock on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspLock,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspLock(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Lock()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallLock(
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspReceiveApprovalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "receive-approval [arg_sender] [arg_amount] [arg_token] [arg3]",
		Short:                 "Calls the nonpayable method receiveApproval on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bspReceiveApproval,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspReceiveApproval(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_sender, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_sender, a address, from passed value %v",
			args[0],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_token, err := chainutil.AddressFromHex(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_token, a address, from passed value %v",
			args[2],
		)
	}
	arg3, err := hexutil.Decode(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg3, a bytes, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ReceiveApproval(
			arg_sender,
			arg_amount,
			arg_token,
			arg3,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallReceiveApproval(
			arg_sender,
			arg_amount,
			arg_token,
			arg3,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RenounceOwnership()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRenounceOwnership(
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspRestoreRewardEligibilityCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "restore-reward-eligibility [arg_operator]",
		Short:                 "Calls the nonpayable method restoreRewardEligibility on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspRestoreRewardEligibility,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspRestoreRewardEligibility(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RestoreRewardEligibility(
			arg_operator,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRestoreRewardEligibility(
			arg_operator,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspTransferChaosnetOwnerRoleCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-chaosnet-owner-role [arg_newChaosnetOwner]",
		Short:                 "Calls the nonpayable method transferChaosnetOwnerRole on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspTransferChaosnetOwnerRole,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspTransferChaosnetOwnerRole(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_newChaosnetOwner, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newChaosnetOwner, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TransferChaosnetOwnerRole(
			arg_newChaosnetOwner,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTransferChaosnetOwnerRole(
			arg_newChaosnetOwner,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_newOwner, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newOwner, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TransferOwnership(
			arg_newOwner,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTransferOwnership(
			arg_newOwner,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspUnlockCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unlock",
		Short:                 "Calls the nonpayable method unlock on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bspUnlock,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspUnlock(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Unlock()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnlock(
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspUpdateOperatorStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-operator-status [arg_operator] [arg_authorizedStake]",
		Short:                 "Calls the nonpayable method updateOperatorStatus on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bspUpdateOperatorStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspUpdateOperatorStatus(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}
	arg_authorizedStake, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_authorizedStake, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateOperatorStatus(
			arg_operator,
			arg_authorizedStake,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateOperatorStatus(
			arg_operator,
			arg_authorizedStake,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspWithdrawIneligibleCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-ineligible [arg_recipient]",
		Short:                 "Calls the nonpayable method withdrawIneligible on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bspWithdrawIneligible,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspWithdrawIneligible(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_recipient, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_recipient, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.WithdrawIneligible(
			arg_recipient,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallWithdrawIneligible(
			arg_recipient,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput("success")

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

func bspWithdrawRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-rewards [arg_operator] [arg_beneficiary]",
		Short:                 "Calls the nonpayable method withdrawRewards on the BeaconSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bspWithdrawRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bspWithdrawRewards(c *cobra.Command, args []string) error {
	contract, err := initializeBeaconSortitionPool(c)
	if err != nil {
		return err
	}

	arg_operator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_operator, a address, from passed value %v",
			args[0],
		)
	}
	arg_beneficiary, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_beneficiary, a address, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
		result      *big.Int
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.WithdrawRewards(
			arg_operator,
			arg_beneficiary,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		result, err = contract.CallWithdrawRewards(
			arg_operator,
			arg_beneficiary,
			cmd.BlockFlagValue.Int,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(result)

		cmd.PrintOutput(
			"the transaction was not submitted to the chain; " +
				"please add the `--submit` flag",
		)
	}

	return nil
}

/// ------------------- Initialization -------------------

func initializeBeaconSortitionPool(c *cobra.Command) (*contract.BeaconSortitionPool, error) {
	cfg := *ModuleCommand.GetConfig()

	client, err := ethclient.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to host chain node: [%v]", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve host chain id: [%v]",
			err,
		)
	}

	key, err := chainutil.DecryptKeyFile(
		cfg.Account.KeyFile,
		cfg.Account.KeyFilePassword,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read KeyFile: %s: [%v]",
			cfg.Account.KeyFile,
			err,
		)
	}

	miningWaiter := chainutil.NewMiningWaiter(client, cfg)

	blockCounter, err := chainutil.NewBlockCounter(client)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create block counter: [%v]",
			err,
		)
	}

	address, err := cfg.ContractAddress("BeaconSortitionPool")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"BeaconSortitionPool",
			err,
		)
	}

	return contract.NewBeaconSortitionPool(
		address,
		chainID,
		key,
		client,
		chainutil.NewNonceManager(client, key.Address),
		miningWaiter,
		blockCounter,
		&sync.Mutex{},
	)
}
