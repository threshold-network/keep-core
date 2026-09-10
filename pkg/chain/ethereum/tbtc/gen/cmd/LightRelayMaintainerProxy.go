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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/keepcommon/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/keepcommon/cmd"

	"github.com/spf13/cobra"
)

var LightRelayMaintainerProxyCommand *cobra.Command

var lightRelayMaintainerProxyDescription = `The light-relay-maintainer-proxy command allows calling the LightRelayMaintainerProxy contract on an
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
	LightRelayMaintainerProxyCommand := &cobra.Command{
		Use:   "light-relay-maintainer-proxy",
		Short: `Provides access to the LightRelayMaintainerProxy contract.`,
		Long:  lightRelayMaintainerProxyDescription,
	}

	LightRelayMaintainerProxyCommand.AddCommand(
		lrmpIsAuthorizedCommand(),
		lrmpLightRelayCommand(),
		lrmpOwnerCommand(),
		lrmpReimbursementPoolCommand(),
		lrmpRetargetGasOffsetCommand(),
		lrmpAuthorizeCommand(),
		lrmpDeauthorizeCommand(),
		lrmpRenounceOwnershipCommand(),
		lrmpRetargetCommand(),
		lrmpTransferOwnershipCommand(),
		lrmpUpdateLightRelayCommand(),
		lrmpUpdateReimbursementPoolCommand(),
		lrmpUpdateRetargetGasOffsetCommand(),
	)

	ModuleCommand.AddCommand(LightRelayMaintainerProxyCommand)
}

/// ------------------- Const methods -------------------

func lrmpIsAuthorizedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-authorized [arg0]",
		Short:                 "Calls the view method isAuthorized on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpIsAuthorized,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrmpIsAuthorized(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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

	result, err := contract.IsAuthorizedAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrmpLightRelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "light-relay",
		Short:                 "Calls the view method lightRelay on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrmpLightRelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrmpLightRelay(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.LightRelayAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrmpOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrmpOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrmpOwner(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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

func lrmpReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reimbursement-pool",
		Short:                 "Calls the view method reimbursementPool on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrmpReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrmpReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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

func lrmpRetargetGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "retarget-gas-offset",
		Short:                 "Calls the view method retargetGasOffset on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrmpRetargetGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrmpRetargetGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
	if err != nil {
		return err
	}

	result, err := contract.RetargetGasOffsetAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func lrmpAuthorizeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorize [arg_maintainer]",
		Short:                 "Calls the nonpayable method authorize on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpAuthorize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpAuthorize(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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
		transaction, err = contract.Authorize(
			arg_maintainer,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAuthorize(
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

func lrmpDeauthorizeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "deauthorize [arg_maintainer]",
		Short:                 "Calls the nonpayable method deauthorize on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpDeauthorize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpDeauthorize(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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
		transaction, err = contract.Deauthorize(
			arg_maintainer,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDeauthorize(
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

func lrmpRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrmpRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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

func lrmpRetargetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "retarget [arg_headers]",
		Short:                 "Calls the nonpayable method retarget on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpRetarget,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpRetarget(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_headers, err := hexutil.Decode(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_headers, a bytes, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Retarget(
			arg_headers,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRetarget(
			arg_headers,
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

func lrmpTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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

func lrmpUpdateLightRelayCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-light-relay [arg__lightRelay]",
		Short:                 "Calls the nonpayable method updateLightRelay on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpUpdateLightRelay,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpUpdateLightRelay(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg__lightRelay, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__lightRelay, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateLightRelay(
			arg__lightRelay,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateLightRelay(
			arg__lightRelay,
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

func lrmpUpdateReimbursementPoolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reimbursement-pool [arg__reimbursementPool]",
		Short:                 "Calls the nonpayable method updateReimbursementPool on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpUpdateReimbursementPool,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpUpdateReimbursementPool(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
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

func lrmpUpdateRetargetGasOffsetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-retarget-gas-offset [arg_newRetargetGasOffset]",
		Short:                 "Calls the nonpayable method updateRetargetGasOffset on the LightRelayMaintainerProxy contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrmpUpdateRetargetGasOffset,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrmpUpdateRetargetGasOffset(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelayMaintainerProxy(c)
	if err != nil {
		return err
	}

	arg_newRetargetGasOffset, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newRetargetGasOffset, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateRetargetGasOffset(
			arg_newRetargetGasOffset,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateRetargetGasOffset(
			arg_newRetargetGasOffset,
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

func initializeLightRelayMaintainerProxy(c *cobra.Command) (*contract.LightRelayMaintainerProxy, error) {
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

	address, err := cfg.ContractAddress("LightRelayMaintainerProxy")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"LightRelayMaintainerProxy",
			err,
		)
	}

	return contract.NewLightRelayMaintainerProxy(
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
