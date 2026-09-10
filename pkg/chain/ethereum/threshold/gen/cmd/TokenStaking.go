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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/keepcommon/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/keepcommon/cmd"
	"github.com/keep-network/keep-core/pkg/keepcommon/utils/decode"

	"github.com/spf13/cobra"
)

var TokenStakingCommand *cobra.Command

var tokenStakingDescription = `The token-staking command allows calling the TokenStaking contract on an
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
	TokenStakingCommand := &cobra.Command{
		Use:   "token-staking",
		Short: `Provides access to the TokenStaking contract.`,
		Long:  tokenStakingDescription,
	}

	TokenStakingCommand.AddCommand(
		tsApplicationInfoCommand(),
		tsApplicationsCommand(),
		tsAuthorizationCeilingCommand(),
		tsAuthorizedStakeCommand(),
		tsCheckpointsCommand(),
		tsDelegatesCommand(),
		tsGetApplicationsLengthCommand(),
		tsGetAvailableToAuthorizeCommand(),
		tsGetMinStakedCommand(),
		tsGetPastTotalSupplyCommand(),
		tsGetPastVotesCommand(),
		tsGetSlashingQueueLengthCommand(),
		tsGetStartStakingTimestampCommand(),
		tsGetVotesCommand(),
		tsGovernanceCommand(),
		tsMinTStakeAmountCommand(),
		tsNotificationRewardCommand(),
		tsNotifiersTreasuryCommand(),
		tsNumCheckpointsCommand(),
		tsRolesOfCommand(),
		tsSlashingQueueCommand(),
		tsSlashingQueueIndexCommand(),
		tsStakeDiscrepancyPenaltyCommand(),
		tsStakeDiscrepancyRewardMultiplierCommand(),
		tsStakedNuCommand(),
		tsStakesCommand(),
		tsApproveApplicationCommand(),
		tsApproveAuthorizationDecreaseCommand(),
		tsDelegateVotingCommand(),
		tsDisableApplicationCommand(),
		tsForceDecreaseAuthorizationCommand(),
		tsIncreaseAuthorizationCommand(),
		tsInitializeCommand(),
		tsNotifyKeepStakeDiscrepancyCommand(),
		tsNotifyNuStakeDiscrepancyCommand(),
		tsPauseApplicationCommand(),
		tsProcessSlashingCommand(),
		tsPushNotificationRewardCommand(),
		tsRefreshKeepStakeOwnerCommand(),
		tsRequestAuthorizationDecreaseCommand(),
		tsRequestAuthorizationDecrease0Command(),
		tsSetAuthorizationCeilingCommand(),
		tsSetMinimumStakeAmountCommand(),
		tsSetNotificationRewardCommand(),
		tsSetPanicButtonCommand(),
		tsSetStakeDiscrepancyPenaltyCommand(),
		tsStakeCommand(),
		tsStakeKeepCommand(),
		tsStakeNuCommand(),
		tsTopUpCommand(),
		tsTopUpKeepCommand(),
		tsTopUpNuCommand(),
		tsTransferGovernanceCommand(),
		tsUnstakeAllCommand(),
		tsUnstakeKeepCommand(),
		tsUnstakeNuCommand(),
		tsUnstakeTCommand(),
		tsWithdrawNotificationRewardCommand(),
	)

	ModuleCommand.AddCommand(TokenStakingCommand)
}

/// ------------------- Const methods -------------------

func tsApplicationInfoCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "application-info [arg0]",
		Short:                 "Calls the view method applicationInfo on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsApplicationInfo,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsApplicationInfo(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
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

	result, err := contract.ApplicationInfoAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsApplicationsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "applications [arg0]",
		Short:                 "Calls the view method applications on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsApplications,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsApplications(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
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

	result, err := contract.ApplicationsAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsAuthorizationCeilingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-ceiling",
		Short:                 "Calls the view method authorizationCeiling on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsAuthorizationCeiling,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsAuthorizationCeiling(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.AuthorizationCeilingAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsAuthorizedStakeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorized-stake [arg_stakingProvider] [arg_application]",
		Short:                 "Calls the view method authorizedStake on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsAuthorizedStake,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsAuthorizedStake(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_application, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[1],
		)
	}

	result, err := contract.AuthorizedStakeAtBlock(
		arg_stakingProvider,
		arg_application,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsCheckpointsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "checkpoints [arg_account] [arg_pos]",
		Short:                 "Calls the view method checkpoints on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsCheckpoints,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsCheckpoints(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_account, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_account, a address, from passed value %v",
			args[0],
		)
	}
	arg_pos, err := decode.ParseUint[uint32](args[1], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_pos, a uint32, from passed value %v",
			args[1],
		)
	}

	result, err := contract.CheckpointsAtBlock(
		arg_account,
		arg_pos,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsDelegatesCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "delegates [arg_account]",
		Short:                 "Calls the view method delegates on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsDelegates,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsDelegates(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_account, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_account, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.DelegatesAtBlock(
		arg_account,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetApplicationsLengthCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-applications-length",
		Short:                 "Calls the view method getApplicationsLength on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsGetApplicationsLength,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetApplicationsLength(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.GetApplicationsLengthAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetAvailableToAuthorizeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-available-to-authorize [arg_stakingProvider] [arg_application]",
		Short:                 "Calls the view method getAvailableToAuthorize on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsGetAvailableToAuthorize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetAvailableToAuthorize(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_application, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[1],
		)
	}

	result, err := contract.GetAvailableToAuthorizeAtBlock(
		arg_stakingProvider,
		arg_application,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetMinStakedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-min-staked [arg_stakingProvider] [arg_stakeTypes]",
		Short:                 "Calls the view method getMinStaked on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsGetMinStaked,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetMinStaked(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_stakeTypes, err := decode.ParseUint[uint8](args[1], 8)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakeTypes, a uint8, from passed value %v",
			args[1],
		)
	}

	result, err := contract.GetMinStakedAtBlock(
		arg_stakingProvider,
		arg_stakeTypes,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetPastTotalSupplyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-past-total-supply [arg_blockNumber]",
		Short:                 "Calls the view method getPastTotalSupply on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsGetPastTotalSupply,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetPastTotalSupply(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_blockNumber, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_blockNumber, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetPastTotalSupplyAtBlock(
		arg_blockNumber,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetPastVotesCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-past-votes [arg_account] [arg_blockNumber]",
		Short:                 "Calls the view method getPastVotes on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsGetPastVotes,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetPastVotes(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_account, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_account, a address, from passed value %v",
			args[0],
		)
	}
	arg_blockNumber, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_blockNumber, a uint256, from passed value %v",
			args[1],
		)
	}

	result, err := contract.GetPastVotesAtBlock(
		arg_account,
		arg_blockNumber,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetSlashingQueueLengthCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-slashing-queue-length",
		Short:                 "Calls the view method getSlashingQueueLength on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsGetSlashingQueueLength,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetSlashingQueueLength(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.GetSlashingQueueLengthAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetStartStakingTimestampCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-start-staking-timestamp [arg_stakingProvider]",
		Short:                 "Calls the view method getStartStakingTimestamp on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsGetStartStakingTimestamp,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetStartStakingTimestamp(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetStartStakingTimestampAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGetVotesCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-votes [arg_account]",
		Short:                 "Calls the view method getVotes on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsGetVotes,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGetVotes(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_account, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_account, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetVotesAtBlock(
		arg_account,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "governance",
		Short:                 "Calls the view method governance on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.GovernanceAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsMinTStakeAmountCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "min-t-stake-amount",
		Short:                 "Calls the view method minTStakeAmount on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsMinTStakeAmount,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsMinTStakeAmount(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.MinTStakeAmountAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsNotificationRewardCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notification-reward",
		Short:                 "Calls the view method notificationReward on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsNotificationReward,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsNotificationReward(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.NotificationRewardAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsNotifiersTreasuryCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notifiers-treasury",
		Short:                 "Calls the view method notifiersTreasury on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsNotifiersTreasury,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsNotifiersTreasury(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.NotifiersTreasuryAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsNumCheckpointsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "num-checkpoints [arg_account]",
		Short:                 "Calls the view method numCheckpoints on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsNumCheckpoints,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsNumCheckpoints(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_account, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_account, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.NumCheckpointsAtBlock(
		arg_account,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsRolesOfCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "roles-of [arg_stakingProvider]",
		Short:                 "Calls the view method rolesOf on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsRolesOf,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsRolesOf(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.RolesOfAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsSlashingQueueCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "slashing-queue [arg0]",
		Short:                 "Calls the view method slashingQueue on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsSlashingQueue,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsSlashingQueue(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
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

	result, err := contract.SlashingQueueAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsSlashingQueueIndexCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "slashing-queue-index",
		Short:                 "Calls the view method slashingQueueIndex on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsSlashingQueueIndex,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsSlashingQueueIndex(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.SlashingQueueIndexAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsStakeDiscrepancyPenaltyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "stake-discrepancy-penalty",
		Short:                 "Calls the view method stakeDiscrepancyPenalty on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsStakeDiscrepancyPenalty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsStakeDiscrepancyPenalty(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.StakeDiscrepancyPenaltyAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsStakeDiscrepancyRewardMultiplierCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "stake-discrepancy-reward-multiplier",
		Short:                 "Calls the view method stakeDiscrepancyRewardMultiplier on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsStakeDiscrepancyRewardMultiplier,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsStakeDiscrepancyRewardMultiplier(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	result, err := contract.StakeDiscrepancyRewardMultiplierAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsStakedNuCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "staked-nu [arg_stakingProvider]",
		Short:                 "Calls the view method stakedNu on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsStakedNu,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsStakedNu(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.StakedNuAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tsStakesCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "stakes [arg_stakingProvider]",
		Short:                 "Calls the view method stakes on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsStakes,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tsStakes(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.StakesAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func tsApproveApplicationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-application [arg_application]",
		Short:                 "Calls the nonpayable method approveApplication on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsApproveApplication,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsApproveApplication(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_application, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ApproveApplication(
			arg_application,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallApproveApplication(
			arg_application,
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

func tsApproveAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-authorization-decrease [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method approveAuthorizationDecrease on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsApproveAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsApproveAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
		result      *big.Int
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ApproveAuthorizationDecrease(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		result, err = contract.CallApproveAuthorizationDecrease(
			arg_stakingProvider,
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

func tsDelegateVotingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "delegate-voting [arg_stakingProvider] [arg_delegatee]",
		Short:                 "Calls the nonpayable method delegateVoting on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsDelegateVoting,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsDelegateVoting(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_delegatee, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_delegatee, a address, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.DelegateVoting(
			arg_stakingProvider,
			arg_delegatee,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDelegateVoting(
			arg_stakingProvider,
			arg_delegatee,
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

func tsDisableApplicationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "disable-application [arg_application]",
		Short:                 "Calls the nonpayable method disableApplication on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsDisableApplication,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsDisableApplication(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_application, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.DisableApplication(
			arg_application,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDisableApplication(
			arg_application,
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

func tsForceDecreaseAuthorizationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "force-decrease-authorization [arg_stakingProvider] [arg_application]",
		Short:                 "Calls the nonpayable method forceDecreaseAuthorization on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsForceDecreaseAuthorization,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsForceDecreaseAuthorization(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_application, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ForceDecreaseAuthorization(
			arg_stakingProvider,
			arg_application,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallForceDecreaseAuthorization(
			arg_stakingProvider,
			arg_application,
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

func tsIncreaseAuthorizationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "increase-authorization [arg_stakingProvider] [arg_application] [arg_amount]",
		Short:                 "Calls the nonpayable method increaseAuthorization on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  tsIncreaseAuthorization,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsIncreaseAuthorization(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_application, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[1],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.IncreaseAuthorization(
			arg_stakingProvider,
			arg_application,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallIncreaseAuthorization(
			arg_stakingProvider,
			arg_application,
			arg_amount,
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

func tsInitializeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "initialize",
		Short:                 "Calls the nonpayable method initialize on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tsInitialize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsInitialize(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Initialize()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInitialize(
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

func tsNotifyKeepStakeDiscrepancyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-keep-stake-discrepancy [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method notifyKeepStakeDiscrepancy on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsNotifyKeepStakeDiscrepancy,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsNotifyKeepStakeDiscrepancy(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyKeepStakeDiscrepancy(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyKeepStakeDiscrepancy(
			arg_stakingProvider,
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

func tsNotifyNuStakeDiscrepancyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-nu-stake-discrepancy [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method notifyNuStakeDiscrepancy on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsNotifyNuStakeDiscrepancy,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsNotifyNuStakeDiscrepancy(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyNuStakeDiscrepancy(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyNuStakeDiscrepancy(
			arg_stakingProvider,
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

func tsPauseApplicationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pause-application [arg_application]",
		Short:                 "Calls the nonpayable method pauseApplication on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsPauseApplication,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsPauseApplication(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_application, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.PauseApplication(
			arg_application,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallPauseApplication(
			arg_application,
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

func tsProcessSlashingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "process-slashing [arg_count]",
		Short:                 "Calls the nonpayable method processSlashing on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsProcessSlashing,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsProcessSlashing(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_count, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_count, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ProcessSlashing(
			arg_count,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallProcessSlashing(
			arg_count,
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

func tsPushNotificationRewardCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "push-notification-reward [arg_reward]",
		Short:                 "Calls the nonpayable method pushNotificationReward on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsPushNotificationReward,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsPushNotificationReward(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_reward, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reward, a uint96, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.PushNotificationReward(
			arg_reward,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallPushNotificationReward(
			arg_reward,
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

func tsRefreshKeepStakeOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "refresh-keep-stake-owner [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method refreshKeepStakeOwner on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsRefreshKeepStakeOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsRefreshKeepStakeOwner(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RefreshKeepStakeOwner(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRefreshKeepStakeOwner(
			arg_stakingProvider,
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

func tsRequestAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-authorization-decrease [arg_stakingProvider] [arg_application] [arg_amount]",
		Short:                 "Calls the nonpayable method requestAuthorizationDecrease on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  tsRequestAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsRequestAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_application, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[1],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestAuthorizationDecrease(
			arg_stakingProvider,
			arg_application,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestAuthorizationDecrease(
			arg_stakingProvider,
			arg_application,
			arg_amount,
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

func tsRequestAuthorizationDecrease0Command() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-authorization-decrease0 [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method requestAuthorizationDecrease0 on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsRequestAuthorizationDecrease0,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsRequestAuthorizationDecrease0(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestAuthorizationDecrease0(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestAuthorizationDecrease0(
			arg_stakingProvider,
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

func tsSetAuthorizationCeilingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-authorization-ceiling [arg_ceiling]",
		Short:                 "Calls the nonpayable method setAuthorizationCeiling on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsSetAuthorizationCeiling,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsSetAuthorizationCeiling(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_ceiling, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_ceiling, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetAuthorizationCeiling(
			arg_ceiling,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetAuthorizationCeiling(
			arg_ceiling,
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

func tsSetMinimumStakeAmountCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-minimum-stake-amount [arg_amount]",
		Short:                 "Calls the nonpayable method setMinimumStakeAmount on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsSetMinimumStakeAmount,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsSetMinimumStakeAmount(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_amount, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetMinimumStakeAmount(
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetMinimumStakeAmount(
			arg_amount,
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

func tsSetNotificationRewardCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-notification-reward [arg_reward]",
		Short:                 "Calls the nonpayable method setNotificationReward on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsSetNotificationReward,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsSetNotificationReward(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_reward, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reward, a uint96, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetNotificationReward(
			arg_reward,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetNotificationReward(
			arg_reward,
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

func tsSetPanicButtonCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-panic-button [arg_application] [arg_panicButton]",
		Short:                 "Calls the nonpayable method setPanicButton on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsSetPanicButton,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsSetPanicButton(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_application, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_application, a address, from passed value %v",
			args[0],
		)
	}
	arg_panicButton, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_panicButton, a address, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetPanicButton(
			arg_application,
			arg_panicButton,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetPanicButton(
			arg_application,
			arg_panicButton,
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

func tsSetStakeDiscrepancyPenaltyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-stake-discrepancy-penalty [arg_penalty] [arg_rewardMultiplier]",
		Short:                 "Calls the nonpayable method setStakeDiscrepancyPenalty on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsSetStakeDiscrepancyPenalty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsSetStakeDiscrepancyPenalty(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_penalty, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_penalty, a uint96, from passed value %v",
			args[0],
		)
	}
	arg_rewardMultiplier, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_rewardMultiplier, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetStakeDiscrepancyPenalty(
			arg_penalty,
			arg_rewardMultiplier,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetStakeDiscrepancyPenalty(
			arg_penalty,
			arg_rewardMultiplier,
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

func tsStakeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "stake [arg_stakingProvider] [arg_beneficiary] [arg_authorizer] [arg_amount]",
		Short:                 "Calls the nonpayable method stake on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  tsStake,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsStake(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
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
	arg_authorizer, err := chainutil.AddressFromHex(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_authorizer, a address, from passed value %v",
			args[2],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Stake(
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallStake(
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
			arg_amount,
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

func tsStakeKeepCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "stake-keep [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method stakeKeep on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsStakeKeep,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsStakeKeep(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.StakeKeep(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallStakeKeep(
			arg_stakingProvider,
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

func tsStakeNuCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "stake-nu [arg_stakingProvider] [arg_beneficiary] [arg_authorizer]",
		Short:                 "Calls the nonpayable method stakeNu on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  tsStakeNu,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsStakeNu(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
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
	arg_authorizer, err := chainutil.AddressFromHex(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_authorizer, a address, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.StakeNu(
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallStakeNu(
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
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

func tsTopUpCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "top-up [arg_stakingProvider] [arg_amount]",
		Short:                 "Calls the nonpayable method topUp on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsTopUp,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsTopUp(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TopUp(
			arg_stakingProvider,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTopUp(
			arg_stakingProvider,
			arg_amount,
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

func tsTopUpKeepCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "top-up-keep [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method topUpKeep on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsTopUpKeep,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsTopUpKeep(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TopUpKeep(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTopUpKeep(
			arg_stakingProvider,
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

func tsTopUpNuCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "top-up-nu [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method topUpNu on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsTopUpNu,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsTopUpNu(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TopUpNu(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTopUpNu(
			arg_stakingProvider,
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

func tsTransferGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-governance [arg_newGuvnor]",
		Short:                 "Calls the nonpayable method transferGovernance on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsTransferGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsTransferGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_newGuvnor, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newGuvnor, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TransferGovernance(
			arg_newGuvnor,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTransferGovernance(
			arg_newGuvnor,
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

func tsUnstakeAllCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unstake-all [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method unstakeAll on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsUnstakeAll,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsUnstakeAll(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UnstakeAll(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnstakeAll(
			arg_stakingProvider,
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

func tsUnstakeKeepCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unstake-keep [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method unstakeKeep on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tsUnstakeKeep,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsUnstakeKeep(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UnstakeKeep(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnstakeKeep(
			arg_stakingProvider,
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

func tsUnstakeNuCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unstake-nu [arg_stakingProvider] [arg_amount]",
		Short:                 "Calls the nonpayable method unstakeNu on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsUnstakeNu,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsUnstakeNu(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UnstakeNu(
			arg_stakingProvider,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnstakeNu(
			arg_stakingProvider,
			arg_amount,
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

func tsUnstakeTCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unstake-t [arg_stakingProvider] [arg_amount]",
		Short:                 "Calls the nonpayable method unstakeT on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsUnstakeT,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsUnstakeT(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
	if err != nil {
		return err
	}

	arg_stakingProvider, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_stakingProvider, a address, from passed value %v",
			args[0],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UnstakeT(
			arg_stakingProvider,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnstakeT(
			arg_stakingProvider,
			arg_amount,
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

func tsWithdrawNotificationRewardCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-notification-reward [arg_recipient] [arg_amount]",
		Short:                 "Calls the nonpayable method withdrawNotificationReward on the TokenStaking contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tsWithdrawNotificationReward,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tsWithdrawNotificationReward(c *cobra.Command, args []string) error {
	contract, err := initializeTokenStaking(c)
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
	arg_amount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint96, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.WithdrawNotificationReward(
			arg_recipient,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallWithdrawNotificationReward(
			arg_recipient,
			arg_amount,
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

func initializeTokenStaking(c *cobra.Command) (*contract.TokenStaking, error) {
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

	address, err := cfg.ContractAddress("TokenStaking")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"TokenStaking",
			err,
		)
	}

	return contract.NewTokenStaking(
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
