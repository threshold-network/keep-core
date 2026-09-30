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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/cmd"
	"github.com/keep-network/keep-core/pkg/decode"

	"github.com/spf13/cobra"
)

var MaintainerProxyCommand *cobra.Command

var maintainerProxyDescription = `The maintainer-proxy command allows calling the MaintainerProxy contract on an
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
	MaintainerProxyCommand := &cobra.Command{
		Use:   "maintainer-proxy",
		Short: `Provides access to the MaintainerProxy contract.`,
		Long:  maintainerProxyDescription,
	}

	MaintainerProxyCommand.AddCommand(
		mpAllSpvMaintainersCommand(),
		mpAllWalletMaintainersCommand(),
		mpBridgeCommand(),
		mpDefeatFraudChallengeGasOffsetCommand(),
		mpDefeatFraudChallengeWithHeartbeatGasOffsetCommand(),
		mpIsSpvMaintainerCommand(),
		mpIsWalletMaintainerCommand(),
		mpNotifyMovingFundsBelowDustGasOffsetCommand(),
		mpNotifyWalletCloseableGasOffsetCommand(),
		mpNotifyWalletClosingPeriodElapsedGasOffsetCommand(),
		mpOwnerCommand(),
		mpReimbursementPoolCommand(),
		mpRequestNewWalletGasOffsetCommand(),
		mpResetMovingFundsTimeoutGasOffsetCommand(),
		mpSpvMaintainersCommand(),
		mpSubmitDepositSweepProofGasOffsetCommand(),
		mpSubmitMovedFundsSweepProofGasOffsetCommand(),
		mpSubmitMovingFundsProofGasOffsetCommand(),
		mpSubmitRedemptionProofGasOffsetCommand(),
		mpWalletMaintainersCommand(),
		mpAuthorizeSpvMaintainerCommand(),
		mpAuthorizeWalletMaintainerCommand(),
		mpDefeatFraudChallengeCommand(),
		mpDefeatFraudChallengeWithHeartbeatCommand(),
		mpNotifyMovingFundsBelowDustCommand(),
		mpNotifyWalletCloseableCommand(),
		mpNotifyWalletClosingPeriodElapsedCommand(),
		mpRenounceOwnershipCommand(),
		mpRequestNewWalletCommand(),
		mpResetMovingFundsTimeoutCommand(),
		mpSubmitDepositSweepProofCommand(),
		mpSubmitMovedFundsSweepProofCommand(),
		mpSubmitMovingFundsProofCommand(),
		mpSubmitRedemptionProofCommand(),
		mpTransferOwnershipCommand(),
		mpUnauthorizeSpvMaintainerCommand(),
		mpUnauthorizeWalletMaintainerCommand(),
		mpUpdateBridgeCommand(),
		mpUpdateGasOffsetParametersCommand(),
		mpUpdateReimbursementPoolCommand(),
	)

	ModuleCommand.AddCommand(MaintainerProxyCommand)
}

/// ------------------- Const methods -------------------

func mpAllSpvMaintainersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "all-spv-maintainers",
		Short:                 "Calls the view method allSpvMaintainers on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpAllSpvMaintainers,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpAllSpvMaintainers(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.AllSpvMaintainersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpAllWalletMaintainersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "all-wallet-maintainers",
		Short:                 "Calls the view method allWalletMaintainers on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpAllWalletMaintainers,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpAllWalletMaintainers(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.AllWalletMaintainersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpBridgeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "bridge",
		Short:                 "Calls the view method bridge on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpBridge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpBridge(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

func mpDefeatFraudChallengeGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "defeat-fraud-challenge-gas-offset",
		Short:                 "Calls the view method defeatFraudChallengeGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpDefeatFraudChallengeGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpDefeatFraudChallengeGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.DefeatFraudChallengeGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpDefeatFraudChallengeWithHeartbeatGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "defeat-fraud-challenge-with-heartbeat-gas-offset",
		Short:                 "Calls the view method defeatFraudChallengeWithHeartbeatGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpDefeatFraudChallengeWithHeartbeatGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpDefeatFraudChallengeWithHeartbeatGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.DefeatFraudChallengeWithHeartbeatGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpIsSpvMaintainerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-spv-maintainer [arg0]",
		Short:                 "Calls the view method isSpvMaintainer on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpIsSpvMaintainer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpIsSpvMaintainer(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	result, err := contract.IsSpvMaintainerAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpIsWalletMaintainerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-wallet-maintainer [arg0]",
		Short:                 "Calls the view method isWalletMaintainer on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpIsWalletMaintainer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpIsWalletMaintainer(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	result, err := contract.IsWalletMaintainerAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpNotifyMovingFundsBelowDustGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-moving-funds-below-dust-gas-offset",
		Short:                 "Calls the view method notifyMovingFundsBelowDustGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpNotifyMovingFundsBelowDustGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpNotifyMovingFundsBelowDustGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.NotifyMovingFundsBelowDustGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpNotifyWalletCloseableGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-wallet-closeable-gas-offset",
		Short:                 "Calls the view method notifyWalletCloseableGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpNotifyWalletCloseableGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpNotifyWalletCloseableGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.NotifyWalletCloseableGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpNotifyWalletClosingPeriodElapsedGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-wallet-closing-period-elapsed-gas-offset",
		Short:                 "Calls the view method notifyWalletClosingPeriodElapsedGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpNotifyWalletClosingPeriodElapsedGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpNotifyWalletClosingPeriodElapsedGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.NotifyWalletClosingPeriodElapsedGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpOwner(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

func mpReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reimbursement-pool",
		Short:                 "Calls the view method reimbursementPool on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

func mpRequestNewWalletGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-new-wallet-gas-offset",
		Short:                 "Calls the view method requestNewWalletGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpRequestNewWalletGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpRequestNewWalletGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.RequestNewWalletGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpResetMovingFundsTimeoutGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reset-moving-funds-timeout-gas-offset",
		Short:                 "Calls the view method resetMovingFundsTimeoutGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpResetMovingFundsTimeoutGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpResetMovingFundsTimeoutGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.ResetMovingFundsTimeoutGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpSpvMaintainersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "spv-maintainers [arg0]",
		Short:                 "Calls the view method spvMaintainers on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpSpvMaintainers,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpSpvMaintainers(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	result, err := contract.SpvMaintainersAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpSubmitDepositSweepProofGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-deposit-sweep-proof-gas-offset",
		Short:                 "Calls the view method submitDepositSweepProofGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpSubmitDepositSweepProofGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpSubmitDepositSweepProofGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.SubmitDepositSweepProofGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpSubmitMovedFundsSweepProofGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-moved-funds-sweep-proof-gas-offset",
		Short:                 "Calls the view method submitMovedFundsSweepProofGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpSubmitMovedFundsSweepProofGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpSubmitMovedFundsSweepProofGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.SubmitMovedFundsSweepProofGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpSubmitMovingFundsProofGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-moving-funds-proof-gas-offset",
		Short:                 "Calls the view method submitMovingFundsProofGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpSubmitMovingFundsProofGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpSubmitMovingFundsProofGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.SubmitMovingFundsProofGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpSubmitRedemptionProofGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-redemption-proof-gas-offset",
		Short:                 "Calls the view method submitRedemptionProofGasOffset on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpSubmitRedemptionProofGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpSubmitRedemptionProofGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.SubmitRedemptionProofGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func mpWalletMaintainersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallet-maintainers [arg0]",
		Short:                 "Calls the view method walletMaintainers on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpWalletMaintainers,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func mpWalletMaintainers(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	result, err := contract.WalletMaintainersAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func mpAuthorizeSpvMaintainerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorize-spv-maintainer [arg_maintainer]",
		Short:                 "Calls the nonpayable method authorizeSpvMaintainer on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpAuthorizeSpvMaintainer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpAuthorizeSpvMaintainer(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_maintainer, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maintainer, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.AuthorizeSpvMaintainer(
			arg_maintainer,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAuthorizeSpvMaintainer(
			arg_maintainer,
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

func mpAuthorizeWalletMaintainerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorize-wallet-maintainer [arg_maintainer]",
		Short:                 "Calls the nonpayable method authorizeWalletMaintainer on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpAuthorizeWalletMaintainer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpAuthorizeWalletMaintainer(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_maintainer, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maintainer, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.AuthorizeWalletMaintainer(
			arg_maintainer,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAuthorizeWalletMaintainer(
			arg_maintainer,
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

func mpDefeatFraudChallengeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "defeat-fraud-challenge [arg_walletPublicKey] [arg_preimage] [arg_witness]",
		Short:                 "Calls the nonpayable method defeatFraudChallenge on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  mpDefeatFraudChallenge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpDefeatFraudChallenge(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_walletPublicKey, err := hexutil.Decode(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPublicKey, a bytes, from passed value %v",
			args[0],
		)
	}
	arg_preimage, err := hexutil.Decode(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_preimage, a bytes, from passed value %v",
			args[1],
		)
	}
	arg_witness, err := strconv.ParseBool(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_witness, a bool, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.DefeatFraudChallenge(
			arg_walletPublicKey,
			arg_preimage,
			arg_witness,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDefeatFraudChallenge(
			arg_walletPublicKey,
			arg_preimage,
			arg_witness,
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

func mpDefeatFraudChallengeWithHeartbeatCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "defeat-fraud-challenge-with-heartbeat [arg_walletPublicKey] [arg_heartbeatMessage]",
		Short:                 "Calls the nonpayable method defeatFraudChallengeWithHeartbeat on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  mpDefeatFraudChallengeWithHeartbeat,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpDefeatFraudChallengeWithHeartbeat(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_walletPublicKey, err := hexutil.Decode(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPublicKey, a bytes, from passed value %v",
			args[0],
		)
	}
	arg_heartbeatMessage, err := hexutil.Decode(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_heartbeatMessage, a bytes, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.DefeatFraudChallengeWithHeartbeat(
			arg_walletPublicKey,
			arg_heartbeatMessage,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDefeatFraudChallengeWithHeartbeat(
			arg_walletPublicKey,
			arg_heartbeatMessage,
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

func mpNotifyMovingFundsBelowDustCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-moving-funds-below-dust [arg_walletPubKeyHash] [arg_mainUtxo_json]",
		Short:                 "Calls the nonpayable method notifyMovingFundsBelowDust on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  mpNotifyMovingFundsBelowDust,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpNotifyMovingFundsBelowDust(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	arg_mainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[1]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyMovingFundsBelowDust(
			arg_walletPubKeyHash,
			arg_mainUtxo_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyMovingFundsBelowDust(
			arg_walletPubKeyHash,
			arg_mainUtxo_json,
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

func mpNotifyWalletCloseableCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-wallet-closeable [arg_walletPubKeyHash] [arg_walletMainUtxo_json]",
		Short:                 "Calls the nonpayable method notifyWalletCloseable on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  mpNotifyWalletCloseable,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpNotifyWalletCloseable(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	arg_walletMainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[1]), &arg_walletMainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_walletMainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyWalletCloseable(
			arg_walletPubKeyHash,
			arg_walletMainUtxo_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyWalletCloseable(
			arg_walletPubKeyHash,
			arg_walletMainUtxo_json,
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

func mpNotifyWalletClosingPeriodElapsedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-wallet-closing-period-elapsed [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method notifyWalletClosingPeriodElapsed on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpNotifyWalletClosingPeriodElapsed,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpNotifyWalletClosingPeriodElapsed(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyWalletClosingPeriodElapsed(
			arg_walletPubKeyHash,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyWalletClosingPeriodElapsed(
			arg_walletPubKeyHash,
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

func mpRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  mpRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

func mpRequestNewWalletCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-new-wallet [arg_activeWalletMainUtxo_json]",
		Short:                 "Calls the nonpayable method requestNewWallet on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpRequestNewWallet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpRequestNewWallet(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_activeWalletMainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[0]), &arg_activeWalletMainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_activeWalletMainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestNewWallet(
			arg_activeWalletMainUtxo_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestNewWallet(
			arg_activeWalletMainUtxo_json,
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

func mpResetMovingFundsTimeoutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reset-moving-funds-timeout [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method resetMovingFundsTimeout on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpResetMovingFundsTimeout,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpResetMovingFundsTimeout(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ResetMovingFundsTimeout(
			arg_walletPubKeyHash,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallResetMovingFundsTimeout(
			arg_walletPubKeyHash,
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

func mpSubmitDepositSweepProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-deposit-sweep-proof [arg_sweepTx_json] [arg_sweepProof_json] [arg_mainUtxo_json] [arg_vault]",
		Short:                 "Calls the nonpayable method submitDepositSweepProof on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  mpSubmitDepositSweepProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpSubmitDepositSweepProof(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_sweepTx_json := abi.BitcoinTxInfo3{}
	if err := json.Unmarshal([]byte(args[0]), &arg_sweepTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepTx_json to abi.BitcoinTxInfo3: %w", err)
	}

	arg_sweepProof_json := abi.BitcoinTxProof2{}
	if err := json.Unmarshal([]byte(args[1]), &arg_sweepProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepProof_json to abi.BitcoinTxProof2: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}
	arg_vault, err := chainutil.AddressFromHex(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_vault, a address, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitDepositSweepProof(
			arg_sweepTx_json,
			arg_sweepProof_json,
			arg_mainUtxo_json,
			arg_vault,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitDepositSweepProof(
			arg_sweepTx_json,
			arg_sweepProof_json,
			arg_mainUtxo_json,
			arg_vault,
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

func mpSubmitMovedFundsSweepProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-moved-funds-sweep-proof [arg_sweepTx_json] [arg_sweepProof_json] [arg_mainUtxo_json]",
		Short:                 "Calls the nonpayable method submitMovedFundsSweepProof on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  mpSubmitMovedFundsSweepProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpSubmitMovedFundsSweepProof(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_sweepTx_json := abi.BitcoinTxInfo3{}
	if err := json.Unmarshal([]byte(args[0]), &arg_sweepTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepTx_json to abi.BitcoinTxInfo3: %w", err)
	}

	arg_sweepProof_json := abi.BitcoinTxProof2{}
	if err := json.Unmarshal([]byte(args[1]), &arg_sweepProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepProof_json to abi.BitcoinTxProof2: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitMovedFundsSweepProof(
			arg_sweepTx_json,
			arg_sweepProof_json,
			arg_mainUtxo_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitMovedFundsSweepProof(
			arg_sweepTx_json,
			arg_sweepProof_json,
			arg_mainUtxo_json,
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

func mpSubmitMovingFundsProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-moving-funds-proof [arg_movingFundsTx_json] [arg_movingFundsProof_json] [arg_mainUtxo_json] [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method submitMovingFundsProof on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  mpSubmitMovingFundsProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpSubmitMovingFundsProof(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_movingFundsTx_json := abi.BitcoinTxInfo3{}
	if err := json.Unmarshal([]byte(args[0]), &arg_movingFundsTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_movingFundsTx_json to abi.BitcoinTxInfo3: %w", err)
	}

	arg_movingFundsProof_json := abi.BitcoinTxProof2{}
	if err := json.Unmarshal([]byte(args[1]), &arg_movingFundsProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_movingFundsProof_json to abi.BitcoinTxProof2: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}
	arg_walletPubKeyHash, err := decode.ParseBytes20(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPubKeyHash, a bytes20, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitMovingFundsProof(
			arg_movingFundsTx_json,
			arg_movingFundsProof_json,
			arg_mainUtxo_json,
			arg_walletPubKeyHash,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitMovingFundsProof(
			arg_movingFundsTx_json,
			arg_movingFundsProof_json,
			arg_mainUtxo_json,
			arg_walletPubKeyHash,
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

func mpSubmitRedemptionProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-redemption-proof [arg_redemptionTx_json] [arg_redemptionProof_json] [arg_mainUtxo_json] [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method submitRedemptionProof on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  mpSubmitRedemptionProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpSubmitRedemptionProof(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_redemptionTx_json := abi.BitcoinTxInfo3{}
	if err := json.Unmarshal([]byte(args[0]), &arg_redemptionTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_redemptionTx_json to abi.BitcoinTxInfo3: %w", err)
	}

	arg_redemptionProof_json := abi.BitcoinTxProof2{}
	if err := json.Unmarshal([]byte(args[1]), &arg_redemptionProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_redemptionProof_json to abi.BitcoinTxProof2: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO2{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO2: %w", err)
	}
	arg_walletPubKeyHash, err := decode.ParseBytes20(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPubKeyHash, a bytes20, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitRedemptionProof(
			arg_redemptionTx_json,
			arg_redemptionProof_json,
			arg_mainUtxo_json,
			arg_walletPubKeyHash,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitRedemptionProof(
			arg_redemptionTx_json,
			arg_redemptionProof_json,
			arg_mainUtxo_json,
			arg_walletPubKeyHash,
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

func mpTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

func mpUnauthorizeSpvMaintainerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unauthorize-spv-maintainer [arg_maintainerToUnauthorize]",
		Short:                 "Calls the nonpayable method unauthorizeSpvMaintainer on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpUnauthorizeSpvMaintainer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpUnauthorizeSpvMaintainer(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_maintainerToUnauthorize, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maintainerToUnauthorize, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UnauthorizeSpvMaintainer(
			arg_maintainerToUnauthorize,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnauthorizeSpvMaintainer(
			arg_maintainerToUnauthorize,
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

func mpUnauthorizeWalletMaintainerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "unauthorize-wallet-maintainer [arg_maintainerToUnauthorize]",
		Short:                 "Calls the nonpayable method unauthorizeWalletMaintainer on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpUnauthorizeWalletMaintainer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpUnauthorizeWalletMaintainer(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_maintainerToUnauthorize, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maintainerToUnauthorize, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UnauthorizeWalletMaintainer(
			arg_maintainerToUnauthorize,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUnauthorizeWalletMaintainer(
			arg_maintainerToUnauthorize,
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

func mpUpdateBridgeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-bridge [arg__bridge]",
		Short:                 "Calls the nonpayable method updateBridge on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpUpdateBridge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpUpdateBridge(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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
		transaction, err = contract.UpdateBridge(
			arg__bridge,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateBridge(
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

func mpUpdateGasOffsetParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-gas-offset-parameters [arg_newSubmitDepositSweepProofGasOffset] [arg_newSubmitRedemptionProofGasOffset] [arg_newResetMovingFundsTimeoutGasOffset] [arg_newSubmitMovingFundsProofGasOffset] [arg_newNotifyMovingFundsBelowDustGasOffset] [arg_newSubmitMovedFundsSweepProofGasOffset] [arg_newRequestNewWalletGasOffset] [arg_newNotifyWalletCloseableGasOffset] [arg_newNotifyWalletClosingPeriodElapsedGasOffset] [arg_newDefeatFraudChallengeGasOffset] [arg_newDefeatFraudChallengeWithHeartbeatGasOffset]",
		Short:                 "Calls the nonpayable method updateGasOffsetParameters on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(11),
		RunE:                  mpUpdateGasOffsetParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpUpdateGasOffsetParameters(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_newSubmitDepositSweepProofGasOffset, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newSubmitDepositSweepProofGasOffset, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_newSubmitRedemptionProofGasOffset, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newSubmitRedemptionProofGasOffset, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_newResetMovingFundsTimeoutGasOffset, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newResetMovingFundsTimeoutGasOffset, a uint256, from passed value %v",
			args[2],
		)
	}
	arg_newSubmitMovingFundsProofGasOffset, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newSubmitMovingFundsProofGasOffset, a uint256, from passed value %v",
			args[3],
		)
	}
	arg_newNotifyMovingFundsBelowDustGasOffset, err := hexutil.DecodeBig(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newNotifyMovingFundsBelowDustGasOffset, a uint256, from passed value %v",
			args[4],
		)
	}
	arg_newSubmitMovedFundsSweepProofGasOffset, err := hexutil.DecodeBig(args[5])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newSubmitMovedFundsSweepProofGasOffset, a uint256, from passed value %v",
			args[5],
		)
	}
	arg_newRequestNewWalletGasOffset, err := hexutil.DecodeBig(args[6])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newRequestNewWalletGasOffset, a uint256, from passed value %v",
			args[6],
		)
	}
	arg_newNotifyWalletCloseableGasOffset, err := hexutil.DecodeBig(args[7])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newNotifyWalletCloseableGasOffset, a uint256, from passed value %v",
			args[7],
		)
	}
	arg_newNotifyWalletClosingPeriodElapsedGasOffset, err := hexutil.DecodeBig(args[8])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newNotifyWalletClosingPeriodElapsedGasOffset, a uint256, from passed value %v",
			args[8],
		)
	}
	arg_newDefeatFraudChallengeGasOffset, err := hexutil.DecodeBig(args[9])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newDefeatFraudChallengeGasOffset, a uint256, from passed value %v",
			args[9],
		)
	}
	arg_newDefeatFraudChallengeWithHeartbeatGasOffset, err := hexutil.DecodeBig(args[10])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newDefeatFraudChallengeWithHeartbeatGasOffset, a uint256, from passed value %v",
			args[10],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateGasOffsetParameters(
			arg_newSubmitDepositSweepProofGasOffset,
			arg_newSubmitRedemptionProofGasOffset,
			arg_newResetMovingFundsTimeoutGasOffset,
			arg_newSubmitMovingFundsProofGasOffset,
			arg_newNotifyMovingFundsBelowDustGasOffset,
			arg_newSubmitMovedFundsSweepProofGasOffset,
			arg_newRequestNewWalletGasOffset,
			arg_newNotifyWalletCloseableGasOffset,
			arg_newNotifyWalletClosingPeriodElapsedGasOffset,
			arg_newDefeatFraudChallengeGasOffset,
			arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateGasOffsetParameters(
			arg_newSubmitDepositSweepProofGasOffset,
			arg_newSubmitRedemptionProofGasOffset,
			arg_newResetMovingFundsTimeoutGasOffset,
			arg_newSubmitMovingFundsProofGasOffset,
			arg_newNotifyMovingFundsBelowDustGasOffset,
			arg_newSubmitMovedFundsSweepProofGasOffset,
			arg_newRequestNewWalletGasOffset,
			arg_newNotifyWalletCloseableGasOffset,
			arg_newNotifyWalletClosingPeriodElapsedGasOffset,
			arg_newDefeatFraudChallengeGasOffset,
			arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
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

func mpUpdateReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reimbursement-pool [arg__reimbursementPool]",
		Short:                 "Calls the nonpayable method updateReimbursementPool on the MaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  mpUpdateReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func mpUpdateReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeMaintainerProxy(c)
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

/// ------------------- Initialization -------------------

func initializeMaintainerProxy(c *cobra.Command) (*contract.MaintainerProxy, error) {
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

	address, err := cfg.ContractAddress("MaintainerProxy")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"MaintainerProxy",
			err,
		)
	}

	return contract.NewMaintainerProxy(
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
