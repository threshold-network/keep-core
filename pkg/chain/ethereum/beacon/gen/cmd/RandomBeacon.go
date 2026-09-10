// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/cmd"
	"github.com/keep-network/keep-core/pkg/decode"

	"github.com/spf13/cobra"
)

var RandomBeaconCommand *cobra.Command

var randomBeaconDescription = `The random-beacon command allows calling the RandomBeacon contract on an
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
	RandomBeaconCommand := &cobra.Command{
		Use:   "random-beacon",
		Short: `Provides access to the RandomBeacon contract.`,
		Long:  randomBeaconDescription,
	}

	RandomBeaconCommand.AddCommand(
		rbAuthorizationParametersCommand(),
		rbAuthorizedRequestersCommand(),
		rbAvailableRewardsCommand(),
		rbEligibleStakeCommand(),
		rbGasParametersCommand(),
		rbGetGroupCommand(),
		rbGetGroup0Command(),
		rbGetGroupCreationStateCommand(),
		rbGetGroupsRegistryCommand(),
		rbGovernanceCommand(),
		rbGroupCreationParametersCommand(),
		rbHasDkgTimedOutCommand(),
		rbInactivityClaimNonceCommand(),
		rbIsOperatorInPoolCommand(),
		rbIsOperatorUpToDateCommand(),
		rbIsRelayRequestInProgressCommand(),
		rbMinimumAuthorizationCommand(),
		rbOperatorToStakingProviderCommand(),
		rbPendingAuthorizationDecreaseCommand(),
		rbReimbursementPoolCommand(),
		rbRelayEntryParametersCommand(),
		rbRemainingAuthorizationDecreaseDelayCommand(),
		rbRewardParametersCommand(),
		rbSelectGroupCommand(),
		rbSlashingParametersCommand(),
		rbSortitionPoolCommand(),
		rbStakingCommand(),
		rbStakingProviderToOperatorCommand(),
		rbTTokenCommand(),
		rbApproveAuthorizationDecreaseCommand(),
		rbApproveDkgResultCommand(),
		rbAuthorizationDecreaseRequestedCommand(),
		rbAuthorizationIncreasedCommand(),
		rbChallengeDkgResultCommand(),
		rbGenesisCommand(),
		rbInvoluntaryAuthorizationDecreaseCommand(),
		rbJoinSortitionPoolCommand(),
		rbNotifyDkgTimeoutCommand(),
		rbRegisterOperatorCommand(),
		rbRequestRelayEntryCommand(),
		rbSetRequesterAuthorizationCommand(),
		rbSubmitDkgResultCommand(),
		rbSubmitRelayEntry0Command(),
		rbTransferGovernanceCommand(),
		rbUpdateAuthorizationParametersCommand(),
		rbUpdateGasParametersCommand(),
		rbUpdateGroupCreationParametersCommand(),
		rbUpdateOperatorStatusCommand(),
		rbUpdateReimbursementPoolCommand(),
		rbUpdateRelayEntryParametersCommand(),
		rbUpdateRewardParametersCommand(),
		rbUpdateSlashingParametersCommand(),
		rbWithdrawIneligibleRewardsCommand(),
		rbWithdrawRewardsCommand(),
	)

	ModuleCommand.AddCommand(RandomBeaconCommand)
}

/// ------------------- Const methods -------------------

func rbAuthorizationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-parameters",
		Short:                 "Calls the view method authorizationParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbAuthorizationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbAuthorizationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.AuthorizationParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbAuthorizedRequestersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorized-requesters [arg0]",
		Short:                 "Calls the view method authorizedRequesters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbAuthorizedRequesters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbAuthorizedRequesters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.AuthorizedRequestersAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbAvailableRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "available-rewards [arg_stakingProvider]",
		Short:                 "Calls the view method availableRewards on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbAvailableRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbAvailableRewards(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.AvailableRewardsAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbEligibleStakeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "eligible-stake [arg_stakingProvider]",
		Short:                 "Calls the view method eligibleStake on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbEligibleStake,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbEligibleStake(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.EligibleStakeAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbGasParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "gas-parameters",
		Short:                 "Calls the view method gasParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbGasParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGasParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.GasParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbGetGroupCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-group [arg_groupId]",
		Short:                 "Calls the view method getGroup on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbGetGroup,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGetGroup(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_groupId, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_groupId, a uint64, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetGroupAtBlock(
		arg_groupId,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbGetGroup0Command() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-group0 [arg_groupPubKey]",
		Short:                 "Calls the view method getGroup0 on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbGetGroup0,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGetGroup0(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_groupPubKey, err := hexutil.Decode(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_groupPubKey, a bytes, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetGroup0AtBlock(
		arg_groupPubKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbGetGroupCreationStateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-group-creation-state",
		Short:                 "Calls the view method getGroupCreationState on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbGetGroupCreationState,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGetGroupCreationState(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.GetGroupCreationStateAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbGetGroupsRegistryCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-groups-registry",
		Short:                 "Calls the view method getGroupsRegistry on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbGetGroupsRegistry,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGetGroupsRegistry(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.GetGroupsRegistryAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "governance",
		Short:                 "Calls the view method governance on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

func rbGroupCreationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "group-creation-parameters",
		Short:                 "Calls the view method groupCreationParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbGroupCreationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbGroupCreationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.GroupCreationParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbHasDkgTimedOutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "has-dkg-timed-out",
		Short:                 "Calls the view method hasDkgTimedOut on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbHasDkgTimedOut,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbHasDkgTimedOut(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.HasDkgTimedOutAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbInactivityClaimNonceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "inactivity-claim-nonce [arg0]",
		Short:                 "Calls the view method inactivityClaimNonce on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbInactivityClaimNonce,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbInactivityClaimNonce(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg0, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg0, a uint64, from passed value %v",
			args[0],
		)
	}

	result, err := contract.InactivityClaimNonceAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbIsOperatorInPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-in-pool [arg_operator]",
		Short:                 "Calls the view method isOperatorInPool on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbIsOperatorInPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbIsOperatorInPool(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

func rbIsOperatorUpToDateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-up-to-date [arg_operator]",
		Short:                 "Calls the view method isOperatorUpToDate on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbIsOperatorUpToDate,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbIsOperatorUpToDate(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.IsOperatorUpToDateAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbIsRelayRequestInProgressCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-relay-request-in-progress",
		Short:                 "Calls the view method isRelayRequestInProgress on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbIsRelayRequestInProgress,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbIsRelayRequestInProgress(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.IsRelayRequestInProgressAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbMinimumAuthorizationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "minimum-authorization",
		Short:                 "Calls the view method minimumAuthorization on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbMinimumAuthorization,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbMinimumAuthorization(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.MinimumAuthorizationAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbOperatorToStakingProviderCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "operator-to-staking-provider [arg_operator]",
		Short:                 "Calls the view method operatorToStakingProvider on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbOperatorToStakingProvider,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbOperatorToStakingProvider(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.OperatorToStakingProviderAtBlock(
		arg_operator,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbPendingAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pending-authorization-decrease [arg_stakingProvider]",
		Short:                 "Calls the view method pendingAuthorizationDecrease on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbPendingAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbPendingAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.PendingAuthorizationDecreaseAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reimbursement-pool",
		Short:                 "Calls the view method reimbursementPool on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.ReimbursementPoolAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbRelayEntryParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "relay-entry-parameters",
		Short:                 "Calls the view method relayEntryParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbRelayEntryParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbRelayEntryParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.RelayEntryParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbRemainingAuthorizationDecreaseDelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "remaining-authorization-decrease-delay [arg_stakingProvider]",
		Short:                 "Calls the view method remainingAuthorizationDecreaseDelay on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbRemainingAuthorizationDecreaseDelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbRemainingAuthorizationDecreaseDelay(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.RemainingAuthorizationDecreaseDelayAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbRewardParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reward-parameters",
		Short:                 "Calls the view method rewardParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbRewardParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbRewardParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.RewardParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbSelectGroupCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "select-group",
		Short:                 "Calls the view method selectGroup on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbSelectGroup,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbSelectGroup(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.SelectGroupAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbSlashingParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "slashing-parameters",
		Short:                 "Calls the view method slashingParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbSlashingParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbSlashingParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.SlashingParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbSortitionPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "sortition-pool",
		Short:                 "Calls the view method sortitionPool on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbSortitionPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbSortitionPool(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.SortitionPoolAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbStakingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "staking",
		Short:                 "Calls the view method staking on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbStaking,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbStaking(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.StakingAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbStakingProviderToOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "staking-provider-to-operator [arg_stakingProvider]",
		Short:                 "Calls the view method stakingProviderToOperator on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbStakingProviderToOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbStakingProviderToOperator(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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

	result, err := contract.StakingProviderToOperatorAtBlock(
		arg_stakingProvider,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rbTTokenCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "t-token",
		Short:                 "Calls the view method tToken on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbTToken,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rbTToken(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	result, err := contract.TTokenAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func rbApproveAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-authorization-decrease [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method approveAuthorizationDecrease on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbApproveAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbApproveAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
		transaction, err = contract.ApproveAuthorizationDecrease(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallApproveAuthorizationDecrease(
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

func rbApproveDkgResultCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-dkg-result [arg_dkgResult_json]",
		Short:                 "Calls the nonpayable method approveDkgResult on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbApproveDkgResult,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbApproveDkgResult(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_dkgResult_json := abi.BeaconDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_dkgResult_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_dkgResult_json to abi.BeaconDkgResult: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ApproveDkgResult(
			arg_dkgResult_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallApproveDkgResult(
			arg_dkgResult_json,
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

func rbAuthorizationDecreaseRequestedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-decrease-requested [arg_stakingProvider] [arg_fromAmount] [arg_toAmount]",
		Short:                 "Calls the nonpayable method authorizationDecreaseRequested on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rbAuthorizationDecreaseRequested,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbAuthorizationDecreaseRequested(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
	arg_fromAmount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fromAmount, a uint96, from passed value %v",
			args[1],
		)
	}
	arg_toAmount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_toAmount, a uint96, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.AuthorizationDecreaseRequested(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAuthorizationDecreaseRequested(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
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

func rbAuthorizationIncreasedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-increased [arg_stakingProvider] [arg_fromAmount] [arg_toAmount]",
		Short:                 "Calls the nonpayable method authorizationIncreased on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rbAuthorizationIncreased,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbAuthorizationIncreased(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
	arg_fromAmount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fromAmount, a uint96, from passed value %v",
			args[1],
		)
	}
	arg_toAmount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_toAmount, a uint96, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.AuthorizationIncreased(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAuthorizationIncreased(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
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

func rbChallengeDkgResultCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "challenge-dkg-result [arg_dkgResult_json]",
		Short:                 "Calls the nonpayable method challengeDkgResult on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbChallengeDkgResult,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbChallengeDkgResult(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_dkgResult_json := abi.BeaconDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_dkgResult_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_dkgResult_json to abi.BeaconDkgResult: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ChallengeDkgResult(
			arg_dkgResult_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallChallengeDkgResult(
			arg_dkgResult_json,
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

func rbGenesisCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "genesis",
		Short:                 "Calls the nonpayable method genesis on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbGenesis,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbGenesis(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Genesis()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallGenesis(
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

func rbInvoluntaryAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "involuntary-authorization-decrease [arg_stakingProvider] [arg_fromAmount] [arg_toAmount]",
		Short:                 "Calls the nonpayable method involuntaryAuthorizationDecrease on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rbInvoluntaryAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbInvoluntaryAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
	arg_fromAmount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fromAmount, a uint96, from passed value %v",
			args[1],
		)
	}
	arg_toAmount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_toAmount, a uint96, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.InvoluntaryAuthorizationDecrease(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInvoluntaryAuthorizationDecrease(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
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

func rbJoinSortitionPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "join-sortition-pool",
		Short:                 "Calls the nonpayable method joinSortitionPool on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbJoinSortitionPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbJoinSortitionPool(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.JoinSortitionPool()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallJoinSortitionPool(
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

func rbNotifyDkgTimeoutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-dkg-timeout",
		Short:                 "Calls the nonpayable method notifyDkgTimeout on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rbNotifyDkgTimeout,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbNotifyDkgTimeout(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyDkgTimeout()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyDkgTimeout(
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

func rbRegisterOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "register-operator [arg_operator]",
		Short:                 "Calls the nonpayable method registerOperator on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbRegisterOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbRegisterOperator(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
		transaction, err = contract.RegisterOperator(
			arg_operator,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRegisterOperator(
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

func rbRequestRelayEntryCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-relay-entry [arg_callbackContract]",
		Short:                 "Calls the nonpayable method requestRelayEntry on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbRequestRelayEntry,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbRequestRelayEntry(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_callbackContract, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_callbackContract, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestRelayEntry(
			arg_callbackContract,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestRelayEntry(
			arg_callbackContract,
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

func rbSetRequesterAuthorizationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-requester-authorization [arg_requester] [arg_isAuthorized]",
		Short:                 "Calls the nonpayable method setRequesterAuthorization on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  rbSetRequesterAuthorization,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbSetRequesterAuthorization(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_requester, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_requester, a address, from passed value %v",
			args[0],
		)
	}
	arg_isAuthorized, err := strconv.ParseBool(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_isAuthorized, a bool, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetRequesterAuthorization(
			arg_requester,
			arg_isAuthorized,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetRequesterAuthorization(
			arg_requester,
			arg_isAuthorized,
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

func rbSubmitDkgResultCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-dkg-result [arg_dkgResult_json]",
		Short:                 "Calls the nonpayable method submitDkgResult on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbSubmitDkgResult,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbSubmitDkgResult(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_dkgResult_json := abi.BeaconDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_dkgResult_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_dkgResult_json to abi.BeaconDkgResult: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitDkgResult(
			arg_dkgResult_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitDkgResult(
			arg_dkgResult_json,
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

func rbSubmitRelayEntry0Command() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-relay-entry0 [arg_entry]",
		Short:                 "Calls the nonpayable method submitRelayEntry0 on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbSubmitRelayEntry0,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbSubmitRelayEntry0(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_entry, err := hexutil.Decode(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_entry, a bytes, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitRelayEntry0(
			arg_entry,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitRelayEntry0(
			arg_entry,
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

func rbTransferGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-governance [arg_newGovernance]",
		Short:                 "Calls the nonpayable method transferGovernance on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbTransferGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbTransferGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_newGovernance, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newGovernance, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TransferGovernance(
			arg_newGovernance,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallTransferGovernance(
			arg_newGovernance,
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

func rbUpdateAuthorizationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-authorization-parameters [arg__minimumAuthorization] [arg__authorizationDecreaseDelay] [arg__authorizationDecreaseChangePeriod]",
		Short:                 "Calls the nonpayable method updateAuthorizationParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rbUpdateAuthorizationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateAuthorizationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg__minimumAuthorization, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__minimumAuthorization, a uint96, from passed value %v",
			args[0],
		)
	}
	arg__authorizationDecreaseDelay, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__authorizationDecreaseDelay, a uint64, from passed value %v",
			args[1],
		)
	}
	arg__authorizationDecreaseChangePeriod, err := decode.ParseUint[uint64](args[2], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__authorizationDecreaseChangePeriod, a uint64, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateAuthorizationParameters(
			arg__minimumAuthorization,
			arg__authorizationDecreaseDelay,
			arg__authorizationDecreaseChangePeriod,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateAuthorizationParameters(
			arg__minimumAuthorization,
			arg__authorizationDecreaseDelay,
			arg__authorizationDecreaseChangePeriod,
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

func rbUpdateGasParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-gas-parameters [arg_dkgResultSubmissionGas] [arg_dkgResultApprovalGasOffset] [arg_notifyOperatorInactivityGasOffset] [arg_relayEntrySubmissionGasOffset]",
		Short:                 "Calls the nonpayable method updateGasParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  rbUpdateGasParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateGasParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_dkgResultSubmissionGas, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgResultSubmissionGas, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_dkgResultApprovalGasOffset, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgResultApprovalGasOffset, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_notifyOperatorInactivityGasOffset, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_notifyOperatorInactivityGasOffset, a uint256, from passed value %v",
			args[2],
		)
	}
	arg_relayEntrySubmissionGasOffset, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_relayEntrySubmissionGasOffset, a uint256, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateGasParameters(
			arg_dkgResultSubmissionGas,
			arg_dkgResultApprovalGasOffset,
			arg_notifyOperatorInactivityGasOffset,
			arg_relayEntrySubmissionGasOffset,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateGasParameters(
			arg_dkgResultSubmissionGas,
			arg_dkgResultApprovalGasOffset,
			arg_notifyOperatorInactivityGasOffset,
			arg_relayEntrySubmissionGasOffset,
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

func rbUpdateGroupCreationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-group-creation-parameters [arg_groupCreationFrequency] [arg_groupLifetime] [arg_dkgResultChallengePeriodLength] [arg_dkgResultChallengeExtraGas] [arg_dkgResultSubmissionTimeout] [arg_dkgSubmitterPrecedencePeriodLength]",
		Short:                 "Calls the nonpayable method updateGroupCreationParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(6),
		RunE:                  rbUpdateGroupCreationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateGroupCreationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_groupCreationFrequency, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_groupCreationFrequency, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_groupLifetime, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_groupLifetime, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_dkgResultChallengePeriodLength, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgResultChallengePeriodLength, a uint256, from passed value %v",
			args[2],
		)
	}
	arg_dkgResultChallengeExtraGas, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgResultChallengeExtraGas, a uint256, from passed value %v",
			args[3],
		)
	}
	arg_dkgResultSubmissionTimeout, err := hexutil.DecodeBig(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgResultSubmissionTimeout, a uint256, from passed value %v",
			args[4],
		)
	}
	arg_dkgSubmitterPrecedencePeriodLength, err := hexutil.DecodeBig(args[5])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgSubmitterPrecedencePeriodLength, a uint256, from passed value %v",
			args[5],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateGroupCreationParameters(
			arg_groupCreationFrequency,
			arg_groupLifetime,
			arg_dkgResultChallengePeriodLength,
			arg_dkgResultChallengeExtraGas,
			arg_dkgResultSubmissionTimeout,
			arg_dkgSubmitterPrecedencePeriodLength,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateGroupCreationParameters(
			arg_groupCreationFrequency,
			arg_groupLifetime,
			arg_dkgResultChallengePeriodLength,
			arg_dkgResultChallengeExtraGas,
			arg_dkgResultSubmissionTimeout,
			arg_dkgSubmitterPrecedencePeriodLength,
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

func rbUpdateOperatorStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-operator-status [arg_operator]",
		Short:                 "Calls the nonpayable method updateOperatorStatus on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbUpdateOperatorStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateOperatorStatus(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
		transaction, err = contract.UpdateOperatorStatus(
			arg_operator,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateOperatorStatus(
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

func rbUpdateReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reimbursement-pool [arg__reimbursementPool]",
		Short:                 "Calls the nonpayable method updateReimbursementPool on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbUpdateReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg__reimbursementPool, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__reimbursementPool, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateReimbursementPool(
			arg__reimbursementPool,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateReimbursementPool(
			arg__reimbursementPool,
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

func rbUpdateRelayEntryParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-relay-entry-parameters [arg_relayEntrySoftTimeout] [arg_relayEntryHardTimeout] [arg_callbackGasLimit]",
		Short:                 "Calls the nonpayable method updateRelayEntryParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rbUpdateRelayEntryParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateRelayEntryParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_relayEntrySoftTimeout, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_relayEntrySoftTimeout, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_relayEntryHardTimeout, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_relayEntryHardTimeout, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_callbackGasLimit, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_callbackGasLimit, a uint256, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateRelayEntryParameters(
			arg_relayEntrySoftTimeout,
			arg_relayEntryHardTimeout,
			arg_callbackGasLimit,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateRelayEntryParameters(
			arg_relayEntrySoftTimeout,
			arg_relayEntryHardTimeout,
			arg_callbackGasLimit,
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

func rbUpdateRewardParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reward-parameters [arg_sortitionPoolRewardsBanDuration] [arg_relayEntryTimeoutNotificationRewardMultiplier] [arg_unauthorizedSigningNotificationRewardMultiplier] [arg_dkgMaliciousResultNotificationRewardMultiplier]",
		Short:                 "Calls the nonpayable method updateRewardParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  rbUpdateRewardParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateRewardParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_sortitionPoolRewardsBanDuration, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_sortitionPoolRewardsBanDuration, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_relayEntryTimeoutNotificationRewardMultiplier, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_relayEntryTimeoutNotificationRewardMultiplier, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_unauthorizedSigningNotificationRewardMultiplier, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_unauthorizedSigningNotificationRewardMultiplier, a uint256, from passed value %v",
			args[2],
		)
	}
	arg_dkgMaliciousResultNotificationRewardMultiplier, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_dkgMaliciousResultNotificationRewardMultiplier, a uint256, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateRewardParameters(
			arg_sortitionPoolRewardsBanDuration,
			arg_relayEntryTimeoutNotificationRewardMultiplier,
			arg_unauthorizedSigningNotificationRewardMultiplier,
			arg_dkgMaliciousResultNotificationRewardMultiplier,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateRewardParameters(
			arg_sortitionPoolRewardsBanDuration,
			arg_relayEntryTimeoutNotificationRewardMultiplier,
			arg_unauthorizedSigningNotificationRewardMultiplier,
			arg_dkgMaliciousResultNotificationRewardMultiplier,
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

func rbUpdateSlashingParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-slashing-parameters [arg_relayEntrySubmissionFailureSlashingAmount] [arg_maliciousDkgResultSlashingAmount] [arg_unauthorizedSigningSlashingAmount]",
		Short:                 "Calls the nonpayable method updateSlashingParameters on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rbUpdateSlashingParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbUpdateSlashingParameters(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
	if err != nil {
		return err
	}

	arg_relayEntrySubmissionFailureSlashingAmount, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_relayEntrySubmissionFailureSlashingAmount, a uint96, from passed value %v",
			args[0],
		)
	}
	arg_maliciousDkgResultSlashingAmount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maliciousDkgResultSlashingAmount, a uint96, from passed value %v",
			args[1],
		)
	}
	arg_unauthorizedSigningSlashingAmount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_unauthorizedSigningSlashingAmount, a uint96, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateSlashingParameters(
			arg_relayEntrySubmissionFailureSlashingAmount,
			arg_maliciousDkgResultSlashingAmount,
			arg_unauthorizedSigningSlashingAmount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateSlashingParameters(
			arg_relayEntrySubmissionFailureSlashingAmount,
			arg_maliciousDkgResultSlashingAmount,
			arg_unauthorizedSigningSlashingAmount,
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

func rbWithdrawIneligibleRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-ineligible-rewards [arg_recipient]",
		Short:                 "Calls the nonpayable method withdrawIneligibleRewards on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbWithdrawIneligibleRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbWithdrawIneligibleRewards(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
		transaction, err = contract.WithdrawIneligibleRewards(
			arg_recipient,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallWithdrawIneligibleRewards(
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

func rbWithdrawRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-rewards [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method withdrawRewards on the RandomBeacon contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rbWithdrawRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rbWithdrawRewards(c *cobra.Command, args []string) error {
	contract, err := initializeRandomBeacon(c)
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
		transaction, err = contract.WithdrawRewards(
			arg_stakingProvider,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallWithdrawRewards(
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

/// ------------------- Initialization -------------------

func initializeRandomBeacon(c *cobra.Command) (*contract.RandomBeacon, error) {
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

	address, err := cfg.ContractAddress("RandomBeacon")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"RandomBeacon",
			err,
		)
	}

	return contract.NewRandomBeacon(
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
