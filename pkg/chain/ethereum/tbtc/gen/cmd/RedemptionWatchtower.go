// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/cmd"
	"github.com/keep-network/keep-common/pkg/utils/decode"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"

	"github.com/spf13/cobra"
)

var RedemptionWatchtowerCommand *cobra.Command

var redemptionWatchtowerDescription = `The redemption-watchtower command allows calling the RedemptionWatchtower contract on an
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
	RedemptionWatchtowerCommand := &cobra.Command{
		Use:   "redemption-watchtower",
		Short: `Provides access to the RedemptionWatchtower contract.`,
		Long:  redemptionWatchtowerDescription,
	}

	RedemptionWatchtowerCommand.AddCommand(
		rwBankCommand(),
		rwBridgeCommand(),
		rwDefaultDelayCommand(),
		rwGetRedemptionDelayCommand(),
		rwIsBannedCommand(),
		rwIsGuardianCommand(),
		rwIsSafeRedemptionCommand(),
		rwLevelOneDelayCommand(),
		rwLevelTwoDelayCommand(),
		rwManagerCommand(),
		rwObjectionsCommand(),
		rwOwnerCommand(),
		rwREQUIREDOBJECTIONSCOUNTCommand(),
		rwVetoFreezePeriodCommand(),
		rwVetoPenaltyFeeDivisorCommand(),
		rwVetoProposalsCommand(),
		rwWaivedAmountLimitCommand(),
		rwWatchtowerDisabledAtCommand(),
		rwWatchtowerEnabledAtCommand(),
		rwWatchtowerLifetimeCommand(),
		rwAddGuardianCommand(),
		rwDisableWatchtowerCommand(),
		rwInitializeCommand(),
		rwRaiseObjectionCommand(),
		rwRemoveGuardianCommand(),
		rwRenounceOwnershipCommand(),
		rwTransferOwnershipCommand(),
		rwUnbanCommand(),
		rwUpdateWatchtowerParametersCommand(),
		rwWithdrawVetoedFundsCommand(),
	)

	ModuleCommand.AddCommand(RedemptionWatchtowerCommand)
}

/// ------------------- Const methods -------------------

func rwBankCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "bank",
		Short:                 "Calls the view method bank on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwBank,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwBank(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.BankAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwBridgeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "bridge",
		Short:                 "Calls the view method bridge on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwBridge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwBridge(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.BridgeAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwDefaultDelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "default-delay",
		Short:                 "Calls the view method defaultDelay on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwDefaultDelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwDefaultDelay(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.DefaultDelayAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwGetRedemptionDelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-redemption-delay [arg_redemptionKey]",
		Short:                 "Calls the view method getRedemptionDelay on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwGetRedemptionDelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwGetRedemptionDelay(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_redemptionKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionKey, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetRedemptionDelayAtBlock(
		arg_redemptionKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwIsBannedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-banned [arg0]",
		Short:                 "Calls the view method isBanned on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwIsBanned,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwIsBanned(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
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

	result, err := contract.IsBannedAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwIsGuardianCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-guardian [arg0]",
		Short:                 "Calls the view method isGuardian on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwIsGuardian,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwIsGuardian(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
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

	result, err := contract.IsGuardianAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwIsSafeRedemptionCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-safe-redemption [arg_walletPubKeyHash] [arg_redeemerOutputScript] [arg_balanceOwner] [arg_redeemer]",
		Short:                 "Calls the view method isSafeRedemption on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  rwIsSafeRedemption,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwIsSafeRedemption(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_walletPubKeyHash, err := decode.ParseBytes20(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPubKeyHash, a bytes20, from passed value %v",
			args[0],
		)
	}
	arg_redeemerOutputScript, err := hexutil.Decode(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redeemerOutputScript, a bytes, from passed value %v",
			args[1],
		)
	}
	arg_balanceOwner, err := chainutil.AddressFromHex(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_balanceOwner, a address, from passed value %v",
			args[2],
		)
	}
	arg_redeemer, err := chainutil.AddressFromHex(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redeemer, a address, from passed value %v",
			args[3],
		)
	}

	result, err := contract.IsSafeRedemptionAtBlock(
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
		arg_balanceOwner,
		arg_redeemer,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwLevelOneDelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "level-one-delay",
		Short:                 "Calls the view method levelOneDelay on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwLevelOneDelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwLevelOneDelay(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.LevelOneDelayAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwLevelTwoDelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "level-two-delay",
		Short:                 "Calls the view method levelTwoDelay on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwLevelTwoDelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwLevelTwoDelay(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.LevelTwoDelayAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwManagerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "manager",
		Short:                 "Calls the view method manager on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwManager,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwManager(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.ManagerAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwObjectionsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "objections [arg0]",
		Short:                 "Calls the view method objections on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwObjections,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwObjections(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg0, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg0, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.ObjectionsAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwOwner(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
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

func rwREQUIREDOBJECTIONSCOUNTCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "r-e-q-u-i-r-e-d-o-b-j-e-c-t-i-o-n-s-c-o-u-n-t",
		Short:                 "Calls the view method rEQUIREDOBJECTIONSCOUNT on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwREQUIREDOBJECTIONSCOUNT,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwREQUIREDOBJECTIONSCOUNT(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.REQUIREDOBJECTIONSCOUNTAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwVetoFreezePeriodCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "veto-freeze-period",
		Short:                 "Calls the view method vetoFreezePeriod on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwVetoFreezePeriod,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwVetoFreezePeriod(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.VetoFreezePeriodAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwVetoPenaltyFeeDivisorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "veto-penalty-fee-divisor",
		Short:                 "Calls the view method vetoPenaltyFeeDivisor on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwVetoPenaltyFeeDivisor,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwVetoPenaltyFeeDivisor(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.VetoPenaltyFeeDivisorAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwVetoProposalsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "veto-proposals [arg0]",
		Short:                 "Calls the view method vetoProposals on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwVetoProposals,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwVetoProposals(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg0, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg0, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.VetoProposalsAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwWaivedAmountLimitCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "waived-amount-limit",
		Short:                 "Calls the view method waivedAmountLimit on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwWaivedAmountLimit,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwWaivedAmountLimit(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.WaivedAmountLimitAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwWatchtowerDisabledAtCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "watchtower-disabled-at",
		Short:                 "Calls the view method watchtowerDisabledAt on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwWatchtowerDisabledAt,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwWatchtowerDisabledAt(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.WatchtowerDisabledAtAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwWatchtowerEnabledAtCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "watchtower-enabled-at",
		Short:                 "Calls the view method watchtowerEnabledAt on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwWatchtowerEnabledAt,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwWatchtowerEnabledAt(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.WatchtowerEnabledAtAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rwWatchtowerLifetimeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "watchtower-lifetime",
		Short:                 "Calls the view method watchtowerLifetime on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwWatchtowerLifetime,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rwWatchtowerLifetime(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	result, err := contract.WatchtowerLifetimeAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func rwAddGuardianCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "add-guardian [arg_guardian]",
		Short:                 "Calls the nonpayable method addGuardian on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwAddGuardian,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwAddGuardian(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_guardian, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_guardian, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.AddGuardian(
			arg_guardian,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAddGuardian(
			arg_guardian,
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

func rwDisableWatchtowerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "disable-watchtower",
		Short:                 "Calls the nonpayable method disableWatchtower on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwDisableWatchtower,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwDisableWatchtower(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.DisableWatchtower()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDisableWatchtower(
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

func rwInitializeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "initialize [arg__bridge]",
		Short:                 "Calls the nonpayable method initialize on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwInitialize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwInitialize(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg__bridge, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__bridge, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Initialize(
			arg__bridge,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInitialize(
			arg__bridge,
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

func rwRaiseObjectionCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "raise-objection [arg_walletPubKeyHash] [arg_redeemerOutputScript]",
		Short:                 "Calls the nonpayable method raiseObjection on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  rwRaiseObjection,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwRaiseObjection(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_walletPubKeyHash, err := decode.ParseBytes20(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPubKeyHash, a bytes20, from passed value %v",
			args[0],
		)
	}
	arg_redeemerOutputScript, err := hexutil.Decode(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redeemerOutputScript, a bytes, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RaiseObjection(
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRaiseObjection(
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
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

func rwRemoveGuardianCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "remove-guardian [arg_guardian]",
		Short:                 "Calls the nonpayable method removeGuardian on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwRemoveGuardian,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwRemoveGuardian(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_guardian, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_guardian, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RemoveGuardian(
			arg_guardian,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRemoveGuardian(
			arg_guardian,
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

func rwRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rwRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
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

func rwTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
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

func rwUnbanCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unban [arg_redeemer]",
		Short:                 "Calls the nonpayable method unban on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwUnban,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwUnban(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_redeemer, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redeemer, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Unban(
			arg_redeemer,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnban(
			arg_redeemer,
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

func rwUpdateWatchtowerParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-watchtower-parameters [arg__watchtowerLifetime] [arg__vetoPenaltyFeeDivisor] [arg__vetoFreezePeriod] [arg__defaultDelay] [arg__levelOneDelay] [arg__levelTwoDelay] [arg__waivedAmountLimit]",
		Short:                 "Calls the nonpayable method updateWatchtowerParameters on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(7),
		RunE:                  rwUpdateWatchtowerParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwUpdateWatchtowerParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg__watchtowerLifetime, err := decode.ParseUint[uint32](args[0], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__watchtowerLifetime, a uint32, from passed value %v",
			args[0],
		)
	}
	arg__vetoPenaltyFeeDivisor, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__vetoPenaltyFeeDivisor, a uint64, from passed value %v",
			args[1],
		)
	}
	arg__vetoFreezePeriod, err := decode.ParseUint[uint32](args[2], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__vetoFreezePeriod, a uint32, from passed value %v",
			args[2],
		)
	}
	arg__defaultDelay, err := decode.ParseUint[uint32](args[3], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__defaultDelay, a uint32, from passed value %v",
			args[3],
		)
	}
	arg__levelOneDelay, err := decode.ParseUint[uint32](args[4], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__levelOneDelay, a uint32, from passed value %v",
			args[4],
		)
	}
	arg__levelTwoDelay, err := decode.ParseUint[uint32](args[5], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__levelTwoDelay, a uint32, from passed value %v",
			args[5],
		)
	}
	arg__waivedAmountLimit, err := decode.ParseUint[uint64](args[6], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__waivedAmountLimit, a uint64, from passed value %v",
			args[6],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateWatchtowerParameters(
			arg__watchtowerLifetime,
			arg__vetoPenaltyFeeDivisor,
			arg__vetoFreezePeriod,
			arg__defaultDelay,
			arg__levelOneDelay,
			arg__levelTwoDelay,
			arg__waivedAmountLimit,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateWatchtowerParameters(
			arg__watchtowerLifetime,
			arg__vetoPenaltyFeeDivisor,
			arg__vetoFreezePeriod,
			arg__defaultDelay,
			arg__levelOneDelay,
			arg__levelTwoDelay,
			arg__waivedAmountLimit,
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

func rwWithdrawVetoedFundsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-vetoed-funds [arg_redemptionKey]",
		Short:                 "Calls the nonpayable method withdrawVetoedFunds on the RedemptionWatchtower contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rwWithdrawVetoedFunds,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rwWithdrawVetoedFunds(c *cobra.Command, args []string) error {
	contract, err := initializeRedemptionWatchtower(c)
	if err != nil {
		return err
	}

	arg_redemptionKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionKey, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.WithdrawVetoedFunds(
			arg_redemptionKey,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallWithdrawVetoedFunds(
			arg_redemptionKey,
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

/// ------------------- Initialization -------------------

func initializeRedemptionWatchtower(c *cobra.Command) (*contract.RedemptionWatchtower, error) {
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

	address, err := cfg.ContractAddress("RedemptionWatchtower")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"RedemptionWatchtower",
			err,
		)
	}

	return contract.NewRedemptionWatchtower(
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
