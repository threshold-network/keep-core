// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	chainutil "github.com/keep-network/keep-core/pkg/keepcommon/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/keepcommon/cmd"
	"github.com/keep-network/keep-core/pkg/keepcommon/utils/decode"

	"github.com/spf13/cobra"
)

var LightRelayCommand *cobra.Command

var lightRelayDescription = `The light-relay command allows calling the LightRelay contract on an
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
	LightRelayCommand := &cobra.Command{
		Use:   "light-relay",
		Short: `Provides access to the LightRelay contract.`,
		Long:  lightRelayDescription,
	}

	LightRelayCommand.AddCommand(
		lrAuthorizationRequiredCommand(),
		lrCurrentEpochCommand(),
		lrGenesisEpochCommand(),
		lrGetBlockDifficultyCommand(),
		lrGetCurrentAndPrevEpochDifficultyCommand(),
		lrGetCurrentEpochDifficultyCommand(),
		lrGetEpochDifficultyCommand(),
		lrGetPrevEpochDifficultyCommand(),
		lrGetRelayRangeCommand(),
		lrIsAuthorizedCommand(),
		lrOwnerCommand(),
		lrProofLengthCommand(),
		lrReadyCommand(),
		lrValidateChainCommand(),
		lrAuthorizeCommand(),
		lrDeauthorizeCommand(),
		lrGenesisCommand(),
		lrRenounceOwnershipCommand(),
		lrRetargetCommand(),
		lrSetAuthorizationStatusCommand(),
		lrSetProofLengthCommand(),
		lrTransferOwnershipCommand(),
	)

	ModuleCommand.AddCommand(LightRelayCommand)
}

/// ------------------- Const methods -------------------

func lrAuthorizationRequiredCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorization-required",
		Short:                 "Calls the view method authorizationRequired on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrAuthorizationRequired,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrAuthorizationRequired(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.AuthorizationRequiredAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrCurrentEpochCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "current-epoch",
		Short:                 "Calls the view method currentEpoch on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrCurrentEpoch,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrCurrentEpoch(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.CurrentEpochAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGenesisEpochCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "genesis-epoch",
		Short:                 "Calls the view method genesisEpoch on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrGenesisEpoch,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGenesisEpoch(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.GenesisEpochAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGetBlockDifficultyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-block-difficulty [arg_blockNumber]",
		Short:                 "Calls the view method getBlockDifficulty on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrGetBlockDifficulty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGetBlockDifficulty(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

	result, err := contract.GetBlockDifficultyAtBlock(
		arg_blockNumber,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGetCurrentAndPrevEpochDifficultyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-current-and-prev-epoch-difficulty",
		Short:                 "Calls the view method getCurrentAndPrevEpochDifficulty on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrGetCurrentAndPrevEpochDifficulty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGetCurrentAndPrevEpochDifficulty(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.GetCurrentAndPrevEpochDifficultyAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGetCurrentEpochDifficultyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-current-epoch-difficulty",
		Short:                 "Calls the view method getCurrentEpochDifficulty on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrGetCurrentEpochDifficulty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGetCurrentEpochDifficulty(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.GetCurrentEpochDifficultyAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGetEpochDifficultyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-epoch-difficulty [arg_epochNumber]",
		Short:                 "Calls the view method getEpochDifficulty on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrGetEpochDifficulty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGetEpochDifficulty(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	arg_epochNumber, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_epochNumber, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.GetEpochDifficultyAtBlock(
		arg_epochNumber,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGetPrevEpochDifficultyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-prev-epoch-difficulty",
		Short:                 "Calls the view method getPrevEpochDifficulty on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrGetPrevEpochDifficulty,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGetPrevEpochDifficulty(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.GetPrevEpochDifficultyAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrGetRelayRangeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-relay-range",
		Short:                 "Calls the view method getRelayRange on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrGetRelayRange,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrGetRelayRange(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.GetRelayRangeAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrIsAuthorizedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-authorized [arg0]",
		Short:                 "Calls the view method isAuthorized on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrIsAuthorized,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrIsAuthorized(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

func lrOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrOwner(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

func lrProofLengthCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "proof-length",
		Short:                 "Calls the view method proofLength on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrProofLength,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrProofLength(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.ProofLengthAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrReadyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "ready",
		Short:                 "Calls the view method ready on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrReady,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrReady(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	result, err := contract.ReadyAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func lrValidateChainCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-chain [arg_headers]",
		Short:                 "Calls the view method validateChain on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrValidateChain,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func lrValidateChain(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

	result, err := contract.ValidateChainAtBlock(
		arg_headers,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func lrAuthorizeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "authorize [arg_submitter]",
		Short:                 "Calls the nonpayable method authorize on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrAuthorize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrAuthorize(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	arg_submitter, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_submitter, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Authorize(
			arg_submitter,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallAuthorize(
			arg_submitter,
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

func lrDeauthorizeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "deauthorize [arg_submitter]",
		Short:                 "Calls the nonpayable method deauthorize on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrDeauthorize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrDeauthorize(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	arg_submitter, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_submitter, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Deauthorize(
			arg_submitter,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallDeauthorize(
			arg_submitter,
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

func lrGenesisCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "genesis [arg_genesisHeader] [arg_genesisHeight] [arg_genesisProofLength]",
		Short:                 "Calls the nonpayable method genesis on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  lrGenesis,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrGenesis(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	arg_genesisHeader, err := hexutil.Decode(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_genesisHeader, a bytes, from passed value %v",
			args[0],
		)
	}
	arg_genesisHeight, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_genesisHeight, a uint256, from passed value %v",
			args[1],
		)
	}
	arg_genesisProofLength, err := decode.ParseUint[uint64](args[2], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_genesisProofLength, a uint64, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Genesis(
			arg_genesisHeader,
			arg_genesisHeight,
			arg_genesisProofLength,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallGenesis(
			arg_genesisHeader,
			arg_genesisHeight,
			arg_genesisProofLength,
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

func lrRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  lrRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

func lrRetargetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "retarget [arg_headers]",
		Short:                 "Calls the nonpayable method retarget on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrRetarget,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrRetarget(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

func lrSetAuthorizationStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-authorization-status [arg_status]",
		Short:                 "Calls the nonpayable method setAuthorizationStatus on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrSetAuthorizationStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrSetAuthorizationStatus(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	arg_status, err := strconv.ParseBool(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_status, a bool, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetAuthorizationStatus(
			arg_status,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetAuthorizationStatus(
			arg_status,
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

func lrSetProofLengthCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-proof-length [arg_newLength]",
		Short:                 "Calls the nonpayable method setProofLength on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrSetProofLength,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrSetProofLength(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
	if err != nil {
		return err
	}

	arg_newLength, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newLength, a uint64, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetProofLength(
			arg_newLength,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetProofLength(
			arg_newLength,
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

func lrTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the LightRelay contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  lrTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func lrTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeLightRelay(c)
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

/// ------------------- Initialization -------------------

func initializeLightRelay(c *cobra.Command) (*contract.LightRelay, error) {
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

	address, err := cfg.ContractAddress("LightRelay")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"LightRelay",
			err,
		)
	}

	return contract.NewLightRelay(
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
