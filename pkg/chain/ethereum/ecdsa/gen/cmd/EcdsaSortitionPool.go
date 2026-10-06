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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/cmd"
	"github.com/keep-network/keep-core/pkg/decode"

	"github.com/spf13/cobra"
)

var EcdsaSortitionPoolCommand *cobra.Command

var ecdsaSortitionPoolDescription = `The ecdsa-sortition-pool command allows calling the EcdsaSortitionPool contract on an
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
	EcdsaSortitionPoolCommand := &cobra.Command{
		Use:   "ecdsa-sortition-pool",
		Short: `Provides access to the EcdsaSortitionPool contract.`,
		Long:  ecdsaSortitionPoolDescription,
	}

	EcdsaSortitionPoolCommand.AddCommand(
		espCanRestoreRewardEligibilityCommand(),
		espChaosnetOwnerCommand(),
		espGetAvailableRewardsCommand(),
		espGetIDOperatorCommand(),
		espGetOperatorIDCommand(),
		espGetPoolWeightCommand(),
		espIneligibleEarnedRewardsCommand(),
		espIsBetaOperatorCommand(),
		espIsChaosnetActiveCommand(),
		espIsEligibleForRewardsCommand(),
		espIsLockedCommand(),
		espIsOperatorInPoolCommand(),
		espIsOperatorRegisteredCommand(),
		espIsOperatorUpToDateCommand(),
		espOperatorsInPoolCommand(),
		espOwnerCommand(),
		espPoolWeightDivisorCommand(),
		espRewardTokenCommand(),
		espRewardsEligibilityRestorableAtCommand(),
		espSelectGroupCommand(),
		espTotalWeightCommand(),
		espDeactivateChaosnetCommand(),
		espInsertOperatorCommand(),
		espLockCommand(),
		espReceiveApprovalCommand(),
		espRenounceOwnershipCommand(),
		espRestoreRewardEligibilityCommand(),
		espTransferChaosnetOwnerRoleCommand(),
		espTransferOwnershipCommand(),
		espUnlockCommand(),
		espUpdateOperatorStatusCommand(),
		espWithdrawIneligibleCommand(),
		espWithdrawRewardsCommand(),
	)

	ModuleCommand.AddCommand(EcdsaSortitionPoolCommand)
}

/// ------------------- Const methods -------------------

func espCanRestoreRewardEligibilityCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "can-restore-reward-eligibility [arg_operator]",
		Short:                 "Calls the view method canRestoreRewardEligibility on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espCanRestoreRewardEligibility,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espCanRestoreRewardEligibility(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espChaosnetOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "chaosnet-owner",
		Short:                 "Calls the view method chaosnetOwner on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espChaosnetOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espChaosnetOwner(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espGetAvailableRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-available-rewards [arg_operator]",
		Short:                 "Calls the view method getAvailableRewards on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espGetAvailableRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espGetAvailableRewards(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espGetIDOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-i-d-operator [arg_id]",
		Short:                 "Calls the view method getIDOperator on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espGetIDOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espGetIDOperator(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espGetOperatorIDCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-operator-i-d [arg_operator]",
		Short:                 "Calls the view method getOperatorID on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espGetOperatorID,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espGetOperatorID(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espGetPoolWeightCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-pool-weight [arg_operator]",
		Short:                 "Calls the view method getPoolWeight on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espGetPoolWeight,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espGetPoolWeight(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIneligibleEarnedRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "ineligible-earned-rewards",
		Short:                 "Calls the view method ineligibleEarnedRewards on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espIneligibleEarnedRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIneligibleEarnedRewards(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsBetaOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-beta-operator [arg0]",
		Short:                 "Calls the view method isBetaOperator on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espIsBetaOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsBetaOperator(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsChaosnetActiveCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-chaosnet-active",
		Short:                 "Calls the view method isChaosnetActive on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espIsChaosnetActive,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsChaosnetActive(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsEligibleForRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-eligible-for-rewards [arg_operator]",
		Short:                 "Calls the view method isEligibleForRewards on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espIsEligibleForRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsEligibleForRewards(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsLockedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-locked",
		Short:                 "Calls the view method isLocked on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espIsLocked,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsLocked(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsOperatorInPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-in-pool [arg_operator]",
		Short:                 "Calls the view method isOperatorInPool on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espIsOperatorInPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsOperatorInPool(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsOperatorRegisteredCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-registered [arg_operator]",
		Short:                 "Calls the view method isOperatorRegistered on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espIsOperatorRegistered,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsOperatorRegistered(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espIsOperatorUpToDateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-up-to-date [arg_operator] [arg_authorizedStake]",
		Short:                 "Calls the view method isOperatorUpToDate on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  espIsOperatorUpToDate,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espIsOperatorUpToDate(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espOperatorsInPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "operators-in-pool",
		Short:                 "Calls the view method operatorsInPool on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espOperatorsInPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espOperatorsInPool(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espOwner(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espPoolWeightDivisorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pool-weight-divisor",
		Short:                 "Calls the view method poolWeightDivisor on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espPoolWeightDivisor,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espPoolWeightDivisor(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espRewardTokenCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reward-token",
		Short:                 "Calls the view method rewardToken on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espRewardToken,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espRewardToken(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espRewardsEligibilityRestorableAtCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "rewards-eligibility-restorable-at [arg_operator]",
		Short:                 "Calls the view method rewardsEligibilityRestorableAt on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espRewardsEligibilityRestorableAt,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espRewardsEligibilityRestorableAt(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espSelectGroupCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "select-group [arg_groupSize] [arg_seed]",
		Short:                 "Calls the view method selectGroup on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  espSelectGroup,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espSelectGroup(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espTotalWeightCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "total-weight",
		Short:                 "Calls the view method totalWeight on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espTotalWeight,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func espTotalWeight(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espDeactivateChaosnetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "deactivate-chaosnet",
		Short:                 "Calls the nonpayable method deactivateChaosnet on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espDeactivateChaosnet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espDeactivateChaosnet(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espInsertOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "insert-operator [arg_operator] [arg_authorizedStake]",
		Short:                 "Calls the nonpayable method insertOperator on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  espInsertOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espInsertOperator(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espLockCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "lock",
		Short:                 "Calls the nonpayable method lock on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espLock,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espLock(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espReceiveApprovalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "receive-approval [arg_sender] [arg_amount] [arg_token] [arg3]",
		Short:                 "Calls the nonpayable method receiveApproval on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  espReceiveApproval,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espReceiveApproval(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espRestoreRewardEligibilityCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "restore-reward-eligibility [arg_operator]",
		Short:                 "Calls the nonpayable method restoreRewardEligibility on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espRestoreRewardEligibility,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espRestoreRewardEligibility(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espTransferChaosnetOwnerRoleCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-chaosnet-owner-role [arg_newChaosnetOwner]",
		Short:                 "Calls the nonpayable method transferChaosnetOwnerRole on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espTransferChaosnetOwnerRole,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espTransferChaosnetOwnerRole(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espUnlockCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unlock",
		Short:                 "Calls the nonpayable method unlock on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  espUnlock,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espUnlock(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espUpdateOperatorStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-operator-status [arg_operator] [arg_authorizedStake]",
		Short:                 "Calls the nonpayable method updateOperatorStatus on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  espUpdateOperatorStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espUpdateOperatorStatus(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espWithdrawIneligibleCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-ineligible [arg_recipient]",
		Short:                 "Calls the nonpayable method withdrawIneligible on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  espWithdrawIneligible,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espWithdrawIneligible(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func espWithdrawRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-rewards [arg_operator] [arg_beneficiary]",
		Short:                 "Calls the nonpayable method withdrawRewards on the EcdsaSortitionPool contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  espWithdrawRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func espWithdrawRewards(c *cobra.Command, args []string) error {
	contract, err := initializeEcdsaSortitionPool(c)
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

func initializeEcdsaSortitionPool(c *cobra.Command) (*contract.EcdsaSortitionPool, error) {
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

	address, err := cfg.ContractAddress("EcdsaSortitionPool")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"EcdsaSortitionPool",
			err,
		)
	}

	return contract.NewEcdsaSortitionPool(
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
