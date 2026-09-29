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

var TBTCCommand *cobra.Command

var tBTCDescription = `The t-b-t-c command allows calling the TBTC contract on an
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
	TBTCCommand := &cobra.Command{
		Use:   "t-b-t-c",
		Short: `Provides access to the TBTC contract.`,
		Long:  tBTCDescription,
	}

	TBTCCommand.AddCommand(
		tbtcAllowanceCommand(),
		tbtcBalanceOfCommand(),
		tbtcCachedChainIdCommand(),
		tbtcCachedDomainSeparatorCommand(),
		tbtcDOMAINSEPARATORCommand(),
		tbtcDecimalsCommand(),
		tbtcNameCommand(),
		tbtcNonceCommand(),
		tbtcOwnerCommand(),
		tbtcPERMITTYPEHASHCommand(),
		tbtcSymbolCommand(),
		tbtcTotalSupplyCommand(),
		tbtcApproveCommand(),
		tbtcApproveAndCallCommand(),
		tbtcBurnCommand(),
		tbtcBurnFromCommand(),
		tbtcMintCommand(),
		tbtcPermitCommand(),
		tbtcRecoverERC20Command(),
		tbtcRecoverERC721Command(),
		tbtcRenounceOwnershipCommand(),
		tbtcTransferCommand(),
		tbtcTransferFromCommand(),
		tbtcTransferOwnershipCommand(),
	)

	ModuleCommand.AddCommand(TBTCCommand)
}

/// ------------------- Const methods -------------------

func tbtcAllowanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "allowance [arg0] [arg1]",
		Short:                 "Calls the view method allowance on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tbtcAllowance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcAllowance(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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
	arg1, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg1, a address, from passed value %v",
			args[1],
		)
	}

	result, err := contract.AllowanceAtBlock(
		arg0,
		arg1,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcBalanceOfCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "balance-of [arg0]",
		Short:                 "Calls the view method balanceOf on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tbtcBalanceOf,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcBalanceOf(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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

	result, err := contract.BalanceOfAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcCachedChainIdCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "cached-chain-id",
		Short:                 "Calls the view method cachedChainId on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcCachedChainId,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcCachedChainId(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.CachedChainIdAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcCachedDomainSeparatorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "cached-domain-separator",
		Short:                 "Calls the view method cachedDomainSeparator on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcCachedDomainSeparator,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcCachedDomainSeparator(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.CachedDomainSeparatorAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcDOMAINSEPARATORCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "d-o-m-a-i-n-s-e-p-a-r-a-t-o-r",
		Short:                 "Calls the view method dOMAINSEPARATOR on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcDOMAINSEPARATOR,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcDOMAINSEPARATOR(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.DOMAINSEPARATORAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcDecimalsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "decimals",
		Short:                 "Calls the view method decimals on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcDecimals,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcDecimals(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.DecimalsAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcNameCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "name",
		Short:                 "Calls the view method name on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcName,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcName(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.NameAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcNonceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "nonce [arg0]",
		Short:                 "Calls the view method nonce on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tbtcNonce,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcNonce(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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

	result, err := contract.NonceAtBlock(
		arg0,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcOwnerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "owner",
		Short:                 "Calls the view method owner on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcOwner,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcOwner(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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

func tbtcPERMITTYPEHASHCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "p-e-r-m-i-t-t-y-p-e-h-a-s-h",
		Short:                 "Calls the view method pERMITTYPEHASH on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcPERMITTYPEHASH,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcPERMITTYPEHASH(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.PERMITTYPEHASHAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcSymbolCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "symbol",
		Short:                 "Calls the view method symbol on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcSymbol,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcSymbol(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.SymbolAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func tbtcTotalSupplyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "total-supply",
		Short:                 "Calls the view method totalSupply on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcTotalSupply,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func tbtcTotalSupply(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	result, err := contract.TotalSupplyAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func tbtcApproveCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve [arg_spender] [arg_amount]",
		Short:                 "Calls the nonpayable method approve on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tbtcApprove,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcApprove(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_spender, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_spender, a address, from passed value %v",
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

	var (
		transaction *types.Transaction
		result      bool
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Approve(
			arg_spender,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		result, err = contract.CallApprove(
			arg_spender,
			arg_amount,
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

func tbtcApproveAndCallCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "approve-and-call [arg_spender] [arg_amount] [arg_extraData]",
		Short:                 "Calls the nonpayable method approveAndCall on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  tbtcApproveAndCall,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcApproveAndCall(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_spender, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_spender, a address, from passed value %v",
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
	arg_extraData, err := hexutil.Decode(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_extraData, a bytes, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
		result      bool
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ApproveAndCall(
			arg_spender,
			arg_amount,
			arg_extraData,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		result, err = contract.CallApproveAndCall(
			arg_spender,
			arg_amount,
			arg_extraData,
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

func tbtcBurnCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "burn [arg_amount]",
		Short:                 "Calls the nonpayable method burn on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tbtcBurn,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcBurn(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_amount, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Burn(
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallBurn(
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

func tbtcBurnFromCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "burn-from [arg_account] [arg_amount]",
		Short:                 "Calls the nonpayable method burnFrom on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tbtcBurnFrom,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcBurnFrom(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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
	arg_amount, err := hexutil.DecodeBig(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.BurnFrom(
			arg_account,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallBurnFrom(
			arg_account,
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

func tbtcMintCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "mint [arg_recipient] [arg_amount]",
		Short:                 "Calls the nonpayable method mint on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tbtcMint,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcMint(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Mint(
			arg_recipient,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallMint(
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

func tbtcPermitCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "permit [arg_owner] [arg_spender] [arg_amount] [arg_deadline] [arg_v] [arg_r] [arg_s]",
		Short:                 "Calls the nonpayable method permit on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(7),
		RunE:                  tbtcPermit,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcPermit(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_owner, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_owner, a address, from passed value %v",
			args[0],
		)
	}
	arg_spender, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_spender, a address, from passed value %v",
			args[1],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[2],
		)
	}
	arg_deadline, err := hexutil.DecodeBig(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_deadline, a uint256, from passed value %v",
			args[3],
		)
	}
	arg_v, err := decode.ParseUint[uint8](args[4], 8)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_v, a uint8, from passed value %v",
			args[4],
		)
	}
	arg_r, err := decode.ParseBytes32(args[5])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_r, a bytes32, from passed value %v",
			args[5],
		)
	}
	arg_s, err := decode.ParseBytes32(args[6])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_s, a bytes32, from passed value %v",
			args[6],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Permit(
			arg_owner,
			arg_spender,
			arg_amount,
			arg_deadline,
			arg_v,
			arg_r,
			arg_s,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallPermit(
			arg_owner,
			arg_spender,
			arg_amount,
			arg_deadline,
			arg_v,
			arg_r,
			arg_s,
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

func tbtcRecoverERC20Command() *cobra.Command {
	c := &cobra.Command{
		Use:                   "recover-e-r-c20 [arg_token] [arg_recipient] [arg_amount]",
		Short:                 "Calls the nonpayable method recoverERC20 on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  tbtcRecoverERC20,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcRecoverERC20(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_token, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_token, a address, from passed value %v",
			args[0],
		)
	}
	arg_recipient, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_recipient, a address, from passed value %v",
			args[1],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RecoverERC20(
			arg_token,
			arg_recipient,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRecoverERC20(
			arg_token,
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

func tbtcRecoverERC721Command() *cobra.Command {
	c := &cobra.Command{
		Use:                   "recover-e-r-c721 [arg_token] [arg_recipient] [arg_tokenId] [arg_data]",
		Short:                 "Calls the nonpayable method recoverERC721 on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  tbtcRecoverERC721,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcRecoverERC721(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_token, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_token, a address, from passed value %v",
			args[0],
		)
	}
	arg_recipient, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_recipient, a address, from passed value %v",
			args[1],
		)
	}
	arg_tokenId, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_tokenId, a uint256, from passed value %v",
			args[2],
		)
	}
	arg_data, err := hexutil.Decode(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_data, a bytes, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RecoverERC721(
			arg_token,
			arg_recipient,
			arg_tokenId,
			arg_data,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRecoverERC721(
			arg_token,
			arg_recipient,
			arg_tokenId,
			arg_data,
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

func tbtcRenounceOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "renounce-ownership",
		Short:                 "Calls the nonpayable method renounceOwnership on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  tbtcRenounceOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcRenounceOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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

func tbtcTransferCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer [arg_recipient] [arg_amount]",
		Short:                 "Calls the nonpayable method transfer on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  tbtcTransfer,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcTransfer(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
		result      bool
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Transfer(
			arg_recipient,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		result, err = contract.CallTransfer(
			arg_recipient,
			arg_amount,
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

func tbtcTransferFromCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-from [arg_spender] [arg_recipient] [arg_amount]",
		Short:                 "Calls the nonpayable method transferFrom on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  tbtcTransferFrom,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcTransferFrom(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
	if err != nil {
		return err
	}

	arg_spender, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_spender, a address, from passed value %v",
			args[0],
		)
	}
	arg_recipient, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_recipient, a address, from passed value %v",
			args[1],
		)
	}
	arg_amount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint256, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
		result      bool
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.TransferFrom(
			arg_spender,
			arg_recipient,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		result, err = contract.CallTransferFrom(
			arg_spender,
			arg_recipient,
			arg_amount,
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

func tbtcTransferOwnershipCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-ownership [arg_newOwner]",
		Short:                 "Calls the nonpayable method transferOwnership on the TBTC contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  tbtcTransferOwnership,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func tbtcTransferOwnership(c *cobra.Command, args []string) error {
	contract, err := initializeTBTC(c)
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

func initializeTBTC(c *cobra.Command) (*contract.TBTC, error) {
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

	address, err := cfg.ContractAddress("TBTC")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"TBTC",
			err,
		)
	}

	return contract.NewTBTC(
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
