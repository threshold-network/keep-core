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

	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/cmd"
	"github.com/keep-network/keep-common/pkg/utils/decode"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"

	"github.com/spf13/cobra"
)

var ReservationRouterCommand *cobra.Command

var reservationRouterDescription = `The reservation-router command allows calling the ReservationRouter contract on an
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
	ReservationRouterCommand := &cobra.Command{
		Use:   "reservation-router",
		Short: `Provides access to the ReservationRouter contract.`,
		Long:  reservationRouterDescription,
	}

	ReservationRouterCommand.AddCommand(
		rrActiveReservationsCountCommand(),
		rrGovernanceCommand(),
		rrPendingReservedDepositsCommand(),
		rrReservationActionsCommand(),
		rrReservationByAnchorUtxoCommand(),
		rrReservationCapsCommand(),
		rrReservationParametersCommand(),
		rrReservationRouterCommand(),
		rrReservationsCommand(),
		rrReservedDepositWalletCommand(),
		rrWalletReservationsCommand(),
		rrWalletReservationsAmountCommand(),
		rrWalletReservationsCountCommand(),
		rrNotifyReservationStrandedCommand(),
		rrNotifyStaleReservedDepositCommand(),
		rrRequestReservationAcceptanceCommand(),
		rrRequestReservationReanchorCommand(),
		rrSubmitReservationProofCommand(),
		rrTransferGovernanceCommand(),
		rrUpdateReservationCapsCommand(),
		rrUpdateReservationParametersCommand(),
	)

	ModuleCommand.AddCommand(ReservationRouterCommand)
}

/// ------------------- Const methods -------------------

func rrActiveReservationsCountCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "active-reservations-count",
		Short:                 "Calls the view method activeReservationsCount on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rrActiveReservationsCount,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrActiveReservationsCount(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	result, err := contract.ActiveReservationsCountAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "governance",
		Short:                 "Calls the view method governance on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rrGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
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

func rrPendingReservedDepositsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pending-reserved-deposits",
		Short:                 "Calls the view method pendingReservedDeposits on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rrPendingReservedDeposits,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrPendingReservedDeposits(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	result, err := contract.PendingReservedDepositsAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservationActionsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reservation-actions [arg_reservationKey] [arg_requestNonce]",
		Short:                 "Calls the view method reservationActions on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  rrReservationActions,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservationActions(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_reservationKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationKey, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_requestNonce, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_requestNonce, a uint64, from passed value %v",
			args[1],
		)
	}

	result, err := contract.ReservationActionsAtBlock(
		arg_reservationKey,
		arg_requestNonce,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservationByAnchorUtxoCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reservation-by-anchor-utxo [arg_anchorTxHash] [arg_anchorTxOutputIndex]",
		Short:                 "Calls the view method reservationByAnchorUtxo on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  rrReservationByAnchorUtxo,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservationByAnchorUtxo(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_anchorTxHash, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_anchorTxHash, a bytes32, from passed value %v",
			args[0],
		)
	}
	arg_anchorTxOutputIndex, err := decode.ParseUint[uint32](args[1], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_anchorTxOutputIndex, a uint32, from passed value %v",
			args[1],
		)
	}

	result, err := contract.ReservationByAnchorUtxoAtBlock(
		arg_anchorTxHash,
		arg_anchorTxOutputIndex,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservationCapsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reservation-caps",
		Short:                 "Calls the view method reservationCaps on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rrReservationCaps,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservationCaps(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	result, err := contract.ReservationCapsAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reservation-parameters",
		Short:                 "Calls the view method reservationParameters on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rrReservationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	result, err := contract.ReservationParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservationRouterCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reservation-router",
		Short:                 "Calls the view method reservationRouter on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rrReservationRouter,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservationRouter(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	result, err := contract.ReservationRouterAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservationsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reservations [arg_reservationKey]",
		Short:                 "Calls the view method reservations on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrReservations,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservations(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_reservationKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationKey, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.ReservationsAtBlock(
		arg_reservationKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrReservedDepositWalletCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reserved-deposit-wallet [arg_depositKey]",
		Short:                 "Calls the view method reservedDepositWallet on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrReservedDepositWallet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrReservedDepositWallet(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_depositKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_depositKey, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.ReservedDepositWalletAtBlock(
		arg_depositKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrWalletReservationsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallet-reservations [arg_walletPubKeyHash]",
		Short:                 "Calls the view method walletReservations on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrWalletReservations,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrWalletReservations(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
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

	result, err := contract.WalletReservationsAtBlock(
		arg_walletPubKeyHash,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrWalletReservationsAmountCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallet-reservations-amount [arg_walletPubKeyHash]",
		Short:                 "Calls the view method walletReservationsAmount on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrWalletReservationsAmount,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrWalletReservationsAmount(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
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

	result, err := contract.WalletReservationsAmountAtBlock(
		arg_walletPubKeyHash,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rrWalletReservationsCountCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallet-reservations-count [arg_walletPubKeyHash]",
		Short:                 "Calls the view method walletReservationsCount on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrWalletReservationsCount,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rrWalletReservationsCount(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
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

	result, err := contract.WalletReservationsCountAtBlock(
		arg_walletPubKeyHash,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

func rrNotifyReservationStrandedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-reservation-stranded [arg_reservationKey]",
		Short:                 "Calls the nonpayable method notifyReservationStranded on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrNotifyReservationStranded,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrNotifyReservationStranded(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_reservationKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationKey, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyReservationStranded(
			arg_reservationKey,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyReservationStranded(
			arg_reservationKey,
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

func rrNotifyStaleReservedDepositCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-stale-reserved-deposit [arg_depositKey]",
		Short:                 "Calls the nonpayable method notifyStaleReservedDeposit on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrNotifyStaleReservedDeposit,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrNotifyStaleReservedDeposit(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_depositKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_depositKey, a uint256, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.NotifyStaleReservedDeposit(
			arg_depositKey,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyStaleReservedDeposit(
			arg_depositKey,
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

func rrRequestReservationAcceptanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-reservation-acceptance [arg_reservationKey] [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method requestReservationAcceptance on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  rrRequestReservationAcceptance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrRequestReservationAcceptance(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_reservationKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationKey, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_walletPubKeyHash, err := decode.ParseBytes20(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletPubKeyHash, a bytes20, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestReservationAcceptance(
			arg_reservationKey,
			arg_walletPubKeyHash,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestReservationAcceptance(
			arg_reservationKey,
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

func rrRequestReservationReanchorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-reservation-reanchor [arg_reservationKey] [arg_targetWalletPubKeyHash]",
		Short:                 "Calls the nonpayable method requestReservationReanchor on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  rrRequestReservationReanchor,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrRequestReservationReanchor(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_reservationKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationKey, a uint256, from passed value %v",
			args[0],
		)
	}
	arg_targetWalletPubKeyHash, err := decode.ParseBytes20(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_targetWalletPubKeyHash, a bytes20, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestReservationReanchor(
			arg_reservationKey,
			arg_targetWalletPubKeyHash,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestReservationReanchor(
			arg_reservationKey,
			arg_targetWalletPubKeyHash,
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

func rrSubmitReservationProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-reservation-proof [arg_proofType] [arg_txInfo_json] [arg_proof_json] [arg_mainUtxo_json] [arg_reservationKey] [arg_requestNonce]",
		Short:                 "Calls the nonpayable method submitReservationProof on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(6),
		RunE:                  rrSubmitReservationProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrSubmitReservationProof(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_proofType, err := decode.ParseUint[uint8](args[0], 8)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_proofType, a uint8, from passed value %v",
			args[0],
		)
	}

	arg_txInfo_json := abi.BitcoinTxInfo4{}
	if err := json.Unmarshal([]byte(args[1]), &arg_txInfo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_txInfo_json to abi.BitcoinTxInfo4: %w", err)
	}

	arg_proof_json := abi.BitcoinTxProof3{}
	if err := json.Unmarshal([]byte(args[2]), &arg_proof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proof_json to abi.BitcoinTxProof3: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO4{}
	if err := json.Unmarshal([]byte(args[3]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO4: %w", err)
	}
	arg_reservationKey, err := hexutil.DecodeBig(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationKey, a uint256, from passed value %v",
			args[4],
		)
	}
	arg_requestNonce, err := decode.ParseUint[uint64](args[5], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_requestNonce, a uint64, from passed value %v",
			args[5],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitReservationProof(
			arg_proofType,
			arg_txInfo_json,
			arg_proof_json,
			arg_mainUtxo_json,
			arg_reservationKey,
			arg_requestNonce,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitReservationProof(
			arg_proofType,
			arg_txInfo_json,
			arg_proof_json,
			arg_mainUtxo_json,
			arg_reservationKey,
			arg_requestNonce,
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

func rrTransferGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-governance [arg_newGovernance]",
		Short:                 "Calls the nonpayable method transferGovernance on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  rrTransferGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrTransferGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
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

func rrUpdateReservationCapsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reservation-caps [arg_maxReservationsAmountPerWallet] [arg_reservationMaxSingleAmount] [arg_maxActiveReservations]",
		Short:                 "Calls the nonpayable method updateReservationCaps on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  rrUpdateReservationCaps,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrUpdateReservationCaps(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_maxReservationsAmountPerWallet, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maxReservationsAmountPerWallet, a uint64, from passed value %v",
			args[0],
		)
	}
	arg_reservationMaxSingleAmount, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationMaxSingleAmount, a uint64, from passed value %v",
			args[1],
		)
	}
	arg_maxActiveReservations, err := decode.ParseUint[uint32](args[2], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maxActiveReservations, a uint32, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateReservationCaps(
			arg_maxReservationsAmountPerWallet,
			arg_reservationMaxSingleAmount,
			arg_maxActiveReservations,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateReservationCaps(
			arg_maxReservationsAmountPerWallet,
			arg_reservationMaxSingleAmount,
			arg_maxActiveReservations,
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

func rrUpdateReservationParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-reservation-parameters [arg_reservationVault] [arg_reservationMinAmount] [arg_reservationTxMaxFee] [arg_reservationTermSeconds] [arg_reservationDissolutionDelay] [arg_reservationMaxTotalAmount] [arg_maxReservationsPerWallet] [arg_reservationActionTimeout] [arg_reservationRenewalWindowSeconds]",
		Short:                 "Calls the nonpayable method updateReservationParameters on the ReservationRouter contract.",
		Args:                  cmd.ArgCountChecker(9),
		RunE:                  rrUpdateReservationParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func rrUpdateReservationParameters(c *cobra.Command, args []string) error {
	contract, err := initializeReservationRouter(c)
	if err != nil {
		return err
	}

	arg_reservationVault, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationVault, a address, from passed value %v",
			args[0],
		)
	}
	arg_reservationMinAmount, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationMinAmount, a uint64, from passed value %v",
			args[1],
		)
	}
	arg_reservationTxMaxFee, err := decode.ParseUint[uint64](args[2], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationTxMaxFee, a uint64, from passed value %v",
			args[2],
		)
	}
	arg_reservationTermSeconds, err := decode.ParseUint[uint32](args[3], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationTermSeconds, a uint32, from passed value %v",
			args[3],
		)
	}
	arg_reservationDissolutionDelay, err := decode.ParseUint[uint32](args[4], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationDissolutionDelay, a uint32, from passed value %v",
			args[4],
		)
	}
	arg_reservationMaxTotalAmount, err := decode.ParseUint[uint64](args[5], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationMaxTotalAmount, a uint64, from passed value %v",
			args[5],
		)
	}
	arg_maxReservationsPerWallet, err := decode.ParseUint[uint32](args[6], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_maxReservationsPerWallet, a uint32, from passed value %v",
			args[6],
		)
	}
	arg_reservationActionTimeout, err := decode.ParseUint[uint32](args[7], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationActionTimeout, a uint32, from passed value %v",
			args[7],
		)
	}
	arg_reservationRenewalWindowSeconds, err := decode.ParseUint[uint32](args[8], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_reservationRenewalWindowSeconds, a uint32, from passed value %v",
			args[8],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateReservationParameters(
			arg_reservationVault,
			arg_reservationMinAmount,
			arg_reservationTxMaxFee,
			arg_reservationTermSeconds,
			arg_reservationDissolutionDelay,
			arg_reservationMaxTotalAmount,
			arg_maxReservationsPerWallet,
			arg_reservationActionTimeout,
			arg_reservationRenewalWindowSeconds,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateReservationParameters(
			arg_reservationVault,
			arg_reservationMinAmount,
			arg_reservationTxMaxFee,
			arg_reservationTermSeconds,
			arg_reservationDissolutionDelay,
			arg_reservationMaxTotalAmount,
			arg_maxReservationsPerWallet,
			arg_reservationActionTimeout,
			arg_reservationRenewalWindowSeconds,
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

func initializeReservationRouter(c *cobra.Command) (*contract.ReservationRouter, error) {
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

	address, err := cfg.ContractAddress("ReservationRouter")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"ReservationRouter",
			err,
		)
	}

	return contract.NewReservationRouter(
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
