// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/keepcommon/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/keepcommon/cmd"
	"github.com/keep-network/keep-core/pkg/keepcommon/utils/decode"

	"github.com/spf13/cobra"
)

var WalletRegistryCommand *cobra.Command

var walletRegistryDescription = `The wallet-registry command allows calling the WalletRegistry contract on an
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
	WalletRegistryCommand := &cobra.Command{
		Use:   "wallet-registry",
		Short: `Provides access to the WalletRegistry contract.`,
		Long:  walletRegistryDescription,
	}

	WalletRegistryCommand.AddCommand(
		wrAuthorizationParametersCommand(),
		wrAvailableRewardsCommand(),
		wrDkgParametersCommand(),
		wrEligibleStakeCommand(),
		wrGasParametersCommand(),
		wrGetWalletCommand(),
		wrGetWalletCreationStateCommand(),
		wrGetWalletPublicKeyCommand(),
		wrGovernanceCommand(),
		wrHasDkgTimedOutCommand(),
		wrHasSeedTimedOutCommand(),
		wrInactivityClaimNonceCommand(),
		wrIsDkgResultValidCommand(),
		wrIsOperatorInPoolCommand(),
		wrIsOperatorUpToDateCommand(),
		wrIsWalletRegisteredCommand(),
		wrMinimumAuthorizationCommand(),
		wrOperatorToStakingProviderCommand(),
		wrPendingAuthorizationDecreaseCommand(),
		wrRandomBeaconCommand(),
		wrReimbursementPoolCommand(),
		wrRemainingAuthorizationDecreaseDelayCommand(),
		wrRewardParametersCommand(),
		wrSelectGroupCommand(),
		wrSlashingParametersCommand(),
		wrSortitionPoolCommand(),
		wrStakingCommand(),
		wrStakingProviderToOperatorCommand(),
		wrWalletOwnerCommand(),
		wrApproveAuthorizationDecreaseCommand(),
		wrApproveDkgResultCommand(),
		wrAuthorizationDecreaseRequestedCommand(),
		wrAuthorizationIncreasedCommand(),
		wrBeaconCallbackCommand(),
		wrChallengeDkgResultCommand(),
		wrCloseWalletCommand(),
		wrInitializeCommand(),
		wrInvoluntaryAuthorizationDecreaseCommand(),
		wrJoinSortitionPoolCommand(),
		wrNotifyDkgTimeoutCommand(),
		wrNotifySeedTimeoutCommand(),
		wrRegisterOperatorCommand(),
		wrRequestNewWalletCommand(),
		wrSubmitDkgResultCommand(),
		wrTransferGovernanceCommand(),
		wrUpdateAuthorizationParametersCommand(),
		wrUpdateDkgParametersCommand(),
		wrUpdateGasParametersCommand(),
		wrUpdateOperatorStatusCommand(),
		wrUpdateReimbursementPoolCommand(),
		wrUpdateRewardParametersCommand(),
		wrUpdateSlashingParametersCommand(),
		wrUpdateWalletOwnerCommand(),
		wrUpgradeRandomBeaconCommand(),
		wrWithdrawIneligibleRewardsCommand(),
		wrWithdrawRewardsCommand(),
	)

	ModuleCommand.AddCommand(WalletRegistryCommand)
}

/// ------------------- Const methods -------------------

func wrAuthorizationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-parameters",
		Short:                 "Calls the view method authorizationParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrAuthorizationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrAuthorizationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrAvailableRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "available-rewards [arg_stakingProvider]",
		Short:                 "Calls the view method availableRewards on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrAvailableRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrAvailableRewards(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrDkgParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "dkg-parameters",
		Short:                 "Calls the view method dkgParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrDkgParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrDkgParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	result, err := contract.DkgParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrEligibleStakeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "eligible-stake [arg_stakingProvider]",
		Short:                 "Calls the view method eligibleStake on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrEligibleStake,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrEligibleStake(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrGasParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "gas-parameters",
		Short:                 "Calls the view method gasParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrGasParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrGasParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrGetWalletCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-wallet [arg_walletID]",
		Short:                 "Calls the view method getWallet on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrGetWallet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrGetWallet(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_walletID, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletID, a bytes32, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetWalletAtBlock(
		arg_walletID,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrGetWalletCreationStateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-wallet-creation-state",
		Short:                 "Calls the view method getWalletCreationState on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrGetWalletCreationState,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrGetWalletCreationState(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	result, err := contract.GetWalletCreationStateAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrGetWalletPublicKeyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-wallet-public-key [arg_walletID]",
		Short:                 "Calls the view method getWalletPublicKey on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrGetWalletPublicKey,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrGetWalletPublicKey(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_walletID, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletID, a bytes32, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetWalletPublicKeyAtBlock(
		arg_walletID,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "governance",
		Short:                 "Calls the view method governance on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrHasDkgTimedOutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "has-dkg-timed-out",
		Short:                 "Calls the view method hasDkgTimedOut on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrHasDkgTimedOut,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrHasDkgTimedOut(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrHasSeedTimedOutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "has-seed-timed-out",
		Short:                 "Calls the view method hasSeedTimedOut on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrHasSeedTimedOut,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrHasSeedTimedOut(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	result, err := contract.HasSeedTimedOutAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrInactivityClaimNonceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "inactivity-claim-nonce [arg0]",
		Short:                 "Calls the view method inactivityClaimNonce on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrInactivityClaimNonce,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrInactivityClaimNonce(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg0, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg0, a bytes32, from passed value %v",
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

func wrIsDkgResultValidCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-dkg-result-valid [arg_result_json]",
		Short:                 "Calls the view method isDkgResultValid on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrIsDkgResultValid,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrIsDkgResultValid(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_result_json := abi.EcdsaDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_result_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_result_json to abi.EcdsaDkgResult: %w", err)
	}

	result, err := contract.IsDkgResultValidAtBlock(
		arg_result_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrIsOperatorInPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-in-pool [arg_operator]",
		Short:                 "Calls the view method isOperatorInPool on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrIsOperatorInPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrIsOperatorInPool(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrIsOperatorUpToDateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-operator-up-to-date [arg_operator]",
		Short:                 "Calls the view method isOperatorUpToDate on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrIsOperatorUpToDate,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrIsOperatorUpToDate(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrIsWalletRegisteredCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-wallet-registered [arg_walletID]",
		Short:                 "Calls the view method isWalletRegistered on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrIsWalletRegistered,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrIsWalletRegistered(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_walletID, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletID, a bytes32, from passed value %v",
			args[0],
		)
	}

	result, err := contract.IsWalletRegisteredAtBlock(
		arg_walletID,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrMinimumAuthorizationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "minimum-authorization",
		Short:                 "Calls the view method minimumAuthorization on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrMinimumAuthorization,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrMinimumAuthorization(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrOperatorToStakingProviderCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "operator-to-staking-provider [arg_operator]",
		Short:                 "Calls the view method operatorToStakingProvider on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrOperatorToStakingProvider,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrOperatorToStakingProvider(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrPendingAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pending-authorization-decrease [arg_stakingProvider]",
		Short:                 "Calls the view method pendingAuthorizationDecrease on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrPendingAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrPendingAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrRandomBeaconCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "random-beacon",
		Short:                 "Calls the view method randomBeacon on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrRandomBeacon,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrRandomBeacon(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	result, err := contract.RandomBeaconAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wrReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reimbursement-pool",
		Short:                 "Calls the view method reimbursementPool on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrRemainingAuthorizationDecreaseDelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "remaining-authorization-decrease-delay [arg_stakingProvider]",
		Short:                 "Calls the view method remainingAuthorizationDecreaseDelay on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrRemainingAuthorizationDecreaseDelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrRemainingAuthorizationDecreaseDelay(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrRewardParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reward-parameters",
		Short:                 "Calls the view method rewardParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrRewardParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrRewardParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrSelectGroupCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "select-group",
		Short:                 "Calls the view method selectGroup on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrSelectGroup,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrSelectGroup(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrSlashingParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "slashing-parameters",
		Short:                 "Calls the view method slashingParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrSlashingParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrSlashingParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrSortitionPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "sortition-pool",
		Short:                 "Calls the view method sortitionPool on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrSortitionPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrSortitionPool(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrStakingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "staking",
		Short:                 "Calls the view method staking on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrStaking,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrStaking(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrStakingProviderToOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "staking-provider-to-operator [arg_stakingProvider]",
		Short:                 "Calls the view method stakingProviderToOperator on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrStakingProviderToOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrStakingProviderToOperator(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrWalletOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallet-owner",
		Short:                 "Calls the view method walletOwner on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrWalletOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wrWalletOwner(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	result, err := contract.WalletOwnerAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func wrApproveAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-authorization-decrease [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method approveAuthorizationDecrease on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrApproveAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrApproveAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrApproveDkgResultCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-dkg-result [arg_dkgResult_json]",
		Short:                 "Calls the nonpayable method approveDkgResult on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrApproveDkgResult,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrApproveDkgResult(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_dkgResult_json := abi.EcdsaDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_dkgResult_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_dkgResult_json to abi.EcdsaDkgResult: %w", err)
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

func wrAuthorizationDecreaseRequestedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-decrease-requested [arg_stakingProvider] [arg_fromAmount] [arg_toAmount]",
		Short:                 "Calls the nonpayable method authorizationDecreaseRequested on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  wrAuthorizationDecreaseRequested,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrAuthorizationDecreaseRequested(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrAuthorizationIncreasedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-increased [arg_stakingProvider] [arg_fromAmount] [arg_toAmount]",
		Short:                 "Calls the nonpayable method authorizationIncreased on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  wrAuthorizationIncreased,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrAuthorizationIncreased(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrBeaconCallbackCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "beacon-callback [arg_relayEntry] [arg1]",
		Short:                 "Calls the nonpayable method beaconCallback on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  wrBeaconCallback,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrBeaconCallback(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_relayEntry, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_relayEntry, a uint256, from passed value %v",
			args[0],
		)
	}
	arg1, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg1, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.BeaconCallback(
			arg_relayEntry,
			arg1,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallBeaconCallback(
			arg_relayEntry,
			arg1,
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

func wrChallengeDkgResultCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "challenge-dkg-result [arg_dkgResult_json]",
		Short:                 "Calls the nonpayable method challengeDkgResult on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrChallengeDkgResult,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrChallengeDkgResult(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_dkgResult_json := abi.EcdsaDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_dkgResult_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_dkgResult_json to abi.EcdsaDkgResult: %w", err)
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

func wrCloseWalletCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "close-wallet [arg_walletID]",
		Short:                 "Calls the nonpayable method closeWallet on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrCloseWallet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrCloseWallet(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_walletID, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletID, a bytes32, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.CloseWallet(
			arg_walletID,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallCloseWallet(
			arg_walletID,
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

func wrInitializeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "initialize [arg__ecdsaDkgValidator] [arg__randomBeacon] [arg__reimbursementPool]",
		Short:                 "Calls the nonpayable method initialize on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  wrInitialize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrInitialize(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg__ecdsaDkgValidator, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__ecdsaDkgValidator, a address, from passed value %v",
			args[0],
		)
	}
	arg__randomBeacon, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__randomBeacon, a address, from passed value %v",
			args[1],
		)
	}
	arg__reimbursementPool, err := chainutil.AddressFromHex(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__reimbursementPool, a address, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Initialize(
			arg__ecdsaDkgValidator,
			arg__randomBeacon,
			arg__reimbursementPool,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInitialize(
			arg__ecdsaDkgValidator,
			arg__randomBeacon,
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

func wrInvoluntaryAuthorizationDecreaseCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "involuntary-authorization-decrease [arg_stakingProvider] [arg_fromAmount] [arg_toAmount]",
		Short:                 "Calls the nonpayable method involuntaryAuthorizationDecrease on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  wrInvoluntaryAuthorizationDecrease,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrInvoluntaryAuthorizationDecrease(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrJoinSortitionPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "join-sortition-pool",
		Short:                 "Calls the nonpayable method joinSortitionPool on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrJoinSortitionPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrJoinSortitionPool(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrNotifyDkgTimeoutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-dkg-timeout",
		Short:                 "Calls the nonpayable method notifyDkgTimeout on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrNotifyDkgTimeout,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrNotifyDkgTimeout(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrNotifySeedTimeoutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-seed-timeout",
		Short:                 "Calls the nonpayable method notifySeedTimeout on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrNotifySeedTimeout,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrNotifySeedTimeout(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifySeedTimeout()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifySeedTimeout(
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

func wrRegisterOperatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "register-operator [arg_operator]",
		Short:                 "Calls the nonpayable method registerOperator on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrRegisterOperator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrRegisterOperator(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrRequestNewWalletCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-new-wallet",
		Short:                 "Calls the nonpayable method requestNewWallet on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wrRequestNewWallet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrRequestNewWallet(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestNewWallet()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestNewWallet(
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

func wrSubmitDkgResultCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-dkg-result [arg_dkgResult_json]",
		Short:                 "Calls the nonpayable method submitDkgResult on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrSubmitDkgResult,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrSubmitDkgResult(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_dkgResult_json := abi.EcdsaDkgResult{}
	if err := json.Unmarshal([]byte(args[0]), &arg_dkgResult_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_dkgResult_json to abi.EcdsaDkgResult: %w", err)
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

func wrTransferGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-governance [arg_newGovernance]",
		Short:                 "Calls the nonpayable method transferGovernance on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrTransferGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrTransferGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrUpdateAuthorizationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-authorization-parameters [arg__minimumAuthorization] [arg__authorizationDecreaseDelay] [arg__authorizationDecreaseChangePeriod]",
		Short:                 "Calls the nonpayable method updateAuthorizationParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  wrUpdateAuthorizationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateAuthorizationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrUpdateDkgParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-dkg-parameters [arg__seedTimeout] [arg__resultChallengePeriodLength] [arg__resultChallengeExtraGas] [arg__resultSubmissionTimeout] [arg__submitterPrecedencePeriodLength]",
		Short:                 "Calls the nonpayable method updateDkgParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(5),
		RunE:                  wrUpdateDkgParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateDkgParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg__seedTimeout, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__seedTimeout, a uint256, from passed value %v",
			args[0],
		)
	}
	arg__resultChallengePeriodLength, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__resultChallengePeriodLength, a uint256, from passed value %v",
			args[1],
		)
	}
	arg__resultChallengeExtraGas, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__resultChallengeExtraGas, a uint256, from passed value %v",
			args[2],
		)
	}
	arg__resultSubmissionTimeout, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__resultSubmissionTimeout, a uint256, from passed value %v",
			args[3],
		)
	}
	arg__submitterPrecedencePeriodLength, err := hexutil.DecodeBig(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__submitterPrecedencePeriodLength, a uint256, from passed value %v",
			args[4],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateDkgParameters(
			arg__seedTimeout,
			arg__resultChallengePeriodLength,
			arg__resultChallengeExtraGas,
			arg__resultSubmissionTimeout,
			arg__submitterPrecedencePeriodLength,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateDkgParameters(
			arg__seedTimeout,
			arg__resultChallengePeriodLength,
			arg__resultChallengeExtraGas,
			arg__resultSubmissionTimeout,
			arg__submitterPrecedencePeriodLength,
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

func wrUpdateGasParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-gas-parameters [arg_dkgResultSubmissionGas] [arg_dkgResultApprovalGasOffset] [arg_notifyOperatorInactivityGasOffset] [arg_notifySeedTimeoutGasOffset] [arg_notifyDkgTimeoutNegativeGasOffset]",
		Short:                 "Calls the nonpayable method updateGasParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(5),
		RunE:                  wrUpdateGasParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateGasParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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
	arg_notifySeedTimeoutGasOffset, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_notifySeedTimeoutGasOffset, a uint256, from passed value %v",
			args[3],
		)
	}
	arg_notifyDkgTimeoutNegativeGasOffset, err := hexutil.DecodeBig(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_notifyDkgTimeoutNegativeGasOffset, a uint256, from passed value %v",
			args[4],
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
			arg_notifySeedTimeoutGasOffset,
			arg_notifyDkgTimeoutNegativeGasOffset,
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
			arg_notifySeedTimeoutGasOffset,
			arg_notifyDkgTimeoutNegativeGasOffset,
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

func wrUpdateOperatorStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-operator-status [arg_operator]",
		Short:                 "Calls the nonpayable method updateOperatorStatus on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrUpdateOperatorStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateOperatorStatus(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrUpdateReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reimbursement-pool [arg__reimbursementPool]",
		Short:                 "Calls the nonpayable method updateReimbursementPool on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrUpdateReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrUpdateRewardParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reward-parameters [arg_maliciousDkgResultNotificationRewardMultiplier] [arg_sortitionPoolRewardsBanDuration]",
		Short:                 "Calls the nonpayable method updateRewardParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  wrUpdateRewardParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateRewardParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_maliciousDkgResultNotificationRewardMultiplier, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maliciousDkgResultNotificationRewardMultiplier, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_sortitionPoolRewardsBanDuration, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_sortitionPoolRewardsBanDuration, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateRewardParameters(
			arg_maliciousDkgResultNotificationRewardMultiplier,
			arg_sortitionPoolRewardsBanDuration,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateRewardParameters(
			arg_maliciousDkgResultNotificationRewardMultiplier,
			arg_sortitionPoolRewardsBanDuration,
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

func wrUpdateSlashingParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-slashing-parameters [arg_maliciousDkgResultSlashingAmount]",
		Short:                 "Calls the nonpayable method updateSlashingParameters on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrUpdateSlashingParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateSlashingParameters(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg_maliciousDkgResultSlashingAmount, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maliciousDkgResultSlashingAmount, a uint96, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateSlashingParameters(
			arg_maliciousDkgResultSlashingAmount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateSlashingParameters(
			arg_maliciousDkgResultSlashingAmount,
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

func wrUpdateWalletOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-wallet-owner [arg__walletOwner]",
		Short:                 "Calls the nonpayable method updateWalletOwner on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrUpdateWalletOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpdateWalletOwner(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg__walletOwner, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__walletOwner, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateWalletOwner(
			arg__walletOwner,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateWalletOwner(
			arg__walletOwner,
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

func wrUpgradeRandomBeaconCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "upgrade-random-beacon [arg__randomBeacon]",
		Short:                 "Calls the nonpayable method upgradeRandomBeacon on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrUpgradeRandomBeacon,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrUpgradeRandomBeacon(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
	if err != nil {
		return err
	}

	arg__randomBeacon, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__randomBeacon, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpgradeRandomBeacon(
			arg__randomBeacon,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpgradeRandomBeacon(
			arg__randomBeacon,
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

func wrWithdrawIneligibleRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-ineligible-rewards [arg_recipient]",
		Short:                 "Calls the nonpayable method withdrawIneligibleRewards on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrWithdrawIneligibleRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrWithdrawIneligibleRewards(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func wrWithdrawRewardsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "withdraw-rewards [arg_stakingProvider]",
		Short:                 "Calls the nonpayable method withdrawRewards on the WalletRegistry contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wrWithdrawRewards,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func wrWithdrawRewards(c *cobra.Command, args []string) error {
	contract, err := initializeWalletRegistry(c)
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

func initializeWalletRegistry(c *cobra.Command) (*contract.WalletRegistry, error) {
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

	address, err := cfg.ContractAddress("WalletRegistry")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"WalletRegistry",
			err,
		)
	}

	return contract.NewWalletRegistry(
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
