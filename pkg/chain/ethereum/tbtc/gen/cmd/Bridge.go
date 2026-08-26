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

	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/cmd"
	"github.com/keep-network/keep-common/pkg/utils/decode"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"

	"github.com/spf13/cobra"
)

var BridgeCommand *cobra.Command

var bridgeDescription = `The bridge command allows calling the Bridge contract on an
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
	BridgeCommand := &cobra.Command{
		Use:   "bridge",
		Short: `Provides access to the Bridge contract.`,
		Long:  bridgeDescription,
	}

	BridgeCommand.AddCommand(
		bActiveWalletPubKeyHashCommand(),
		bContractReferencesCommand(),
		bDepositParametersCommand(),
		bDepositsCommand(),
		bFraudChallengesCommand(),
		bFraudParametersCommand(),
		bGetRebateStakingCommand(),
		bGetRedemptionWatchtowerCommand(),
		bGetReservationRouterCommand(),
		bGovernanceCommand(),
		bIsReservedDepositCommand(),
		bIsVaultTrustedCommand(),
		bLiveWalletsCountCommand(),
		bMovedFundsSweepRequestsCommand(),
		bMovingFundsParametersCommand(),
		bPendingRedemptionsCommand(),
		bRedemptionParametersCommand(),
		bSpentMainUTXOsCommand(),
		bTimedOutRedemptionsCommand(),
		bTreasuryCommand(),
		bTxProofDifficultyFactorCommand(),
		bWalletParametersCommand(),
		bWalletsCommand(),
		bDefeatFraudChallengeCommand(),
		bDefeatFraudChallengeWithHeartbeatCommand(),
		bEcdsaWalletCreatedCallbackCommand(),
		bEcdsaWalletHeartbeatFailedCallbackCommand(),
		bInitializeCommand(),
		bInitializeV2FixVaultZeroDepositCommand(),
		bInitializeV5RepairRebateStakingCommand(),
		bNotifyMovingFundsBelowDustCommand(),
		bNotifyRedemptionVetoCommand(),
		bNotifyWalletCloseableCommand(),
		bNotifyWalletClosingPeriodElapsedCommand(),
		bReceiveBalanceApprovalCommand(),
		bRequestNewWalletCommand(),
		bRequestRedemptionCommand(),
		bResetMovingFundsTimeoutCommand(),
		bRevealDepositCommand(),
		bRevealDepositWithExtraDataCommand(),
		bSetRebateStakingCommand(),
		bSetRedemptionWatchtowerCommand(),
		bSetReservationRouterCommand(),
		bSetSpvMaintainerStatusCommand(),
		bSetVaultStatusCommand(),
		bSubmitDepositSweepProofCommand(),
		bSubmitFraudChallengeCommand(),
		bSubmitMovedFundsSweepProofCommand(),
		bSubmitMovingFundsProofCommand(),
		bSubmitRedemptionProofCommand(),
		bTransferGovernanceCommand(),
		bUpdateDepositParametersCommand(),
		bUpdateFraudParametersCommand(),
		bUpdateMovingFundsParametersCommand(),
		bUpdateRedemptionParametersCommand(),
		bUpdateTreasuryCommand(),
		bUpdateWalletParametersCommand(),
	)

	ModuleCommand.AddCommand(BridgeCommand)
}

/// ------------------- Const methods -------------------

func bActiveWalletPubKeyHashCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "active-wallet-pub-key-hash",
		Short:                 "Calls the view method activeWalletPubKeyHash on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bActiveWalletPubKeyHash,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bActiveWalletPubKeyHash(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.ActiveWalletPubKeyHashAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bContractReferencesCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "contract-references",
		Short:                 "Calls the view method contractReferences on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bContractReferences,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bContractReferences(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.ContractReferencesAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bDepositParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "deposit-parameters",
		Short:                 "Calls the view method depositParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bDepositParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bDepositParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.DepositParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bDepositsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "deposits [arg_depositKey]",
		Short:                 "Calls the view method deposits on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bDeposits,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bDeposits(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	result, err := contract.DepositsAtBlock(
		arg_depositKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bFraudChallengesCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "fraud-challenges [arg_challengeKey]",
		Short:                 "Calls the view method fraudChallenges on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bFraudChallenges,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bFraudChallenges(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_challengeKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_challengeKey, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.FraudChallengesAtBlock(
		arg_challengeKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bFraudParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "fraud-parameters",
		Short:                 "Calls the view method fraudParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bFraudParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bFraudParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.FraudParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bGetRebateStakingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-rebate-staking",
		Short:                 "Calls the view method getRebateStaking on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bGetRebateStaking,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bGetRebateStaking(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.GetRebateStakingAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bGetRedemptionWatchtowerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-redemption-watchtower",
		Short:                 "Calls the view method getRedemptionWatchtower on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bGetRedemptionWatchtower,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bGetRedemptionWatchtower(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.GetRedemptionWatchtowerAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bGetReservationRouterCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "get-reservation-router",
		Short:                 "Calls the view method getReservationRouter on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bGetReservationRouter,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bGetReservationRouter(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.GetReservationRouterAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "governance",
		Short:                 "Calls the view method governance on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

func bIsReservedDepositCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-reserved-deposit [arg_depositKey]",
		Short:                 "Calls the view method isReservedDeposit on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bIsReservedDeposit,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bIsReservedDeposit(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	result, err := contract.IsReservedDepositAtBlock(
		arg_depositKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bIsVaultTrustedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "is-vault-trusted [arg_vault]",
		Short:                 "Calls the view method isVaultTrusted on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bIsVaultTrusted,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bIsVaultTrusted(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_vault, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_vault, a address, from passed value %v",
			args[0],
		)
	}

	result, err := contract.IsVaultTrustedAtBlock(
		arg_vault,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bLiveWalletsCountCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "live-wallets-count",
		Short:                 "Calls the view method liveWalletsCount on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bLiveWalletsCount,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bLiveWalletsCount(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.LiveWalletsCountAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bMovedFundsSweepRequestsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "moved-funds-sweep-requests [arg_requestKey]",
		Short:                 "Calls the view method movedFundsSweepRequests on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bMovedFundsSweepRequests,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bMovedFundsSweepRequests(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_requestKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_requestKey, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.MovedFundsSweepRequestsAtBlock(
		arg_requestKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bMovingFundsParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "moving-funds-parameters",
		Short:                 "Calls the view method movingFundsParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bMovingFundsParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bMovingFundsParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.MovingFundsParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bPendingRedemptionsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "pending-redemptions [arg_redemptionKey]",
		Short:                 "Calls the view method pendingRedemptions on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bPendingRedemptions,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bPendingRedemptions(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	result, err := contract.PendingRedemptionsAtBlock(
		arg_redemptionKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bRedemptionParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "redemption-parameters",
		Short:                 "Calls the view method redemptionParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bRedemptionParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bRedemptionParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.RedemptionParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bSpentMainUTXOsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "spent-main-u-t-x-os [arg_utxoKey]",
		Short:                 "Calls the view method spentMainUTXOs on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bSpentMainUTXOs,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bSpentMainUTXOs(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_utxoKey, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_utxoKey, a uint256, from passed value %v",
			args[0],
		)
	}

	result, err := contract.SpentMainUTXOsAtBlock(
		arg_utxoKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bTimedOutRedemptionsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "timed-out-redemptions [arg_redemptionKey]",
		Short:                 "Calls the view method timedOutRedemptions on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bTimedOutRedemptions,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bTimedOutRedemptions(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	result, err := contract.TimedOutRedemptionsAtBlock(
		arg_redemptionKey,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bTreasuryCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "treasury",
		Short:                 "Calls the view method treasury on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bTreasury,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bTreasury(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.TreasuryAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bTxProofDifficultyFactorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "tx-proof-difficulty-factor",
		Short:                 "Calls the view method txProofDifficultyFactor on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bTxProofDifficultyFactor,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bTxProofDifficultyFactor(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.TxProofDifficultyFactorAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bWalletParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallet-parameters",
		Short:                 "Calls the view method walletParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bWalletParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bWalletParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	result, err := contract.WalletParametersAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func bWalletsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "wallets [arg_walletPubKeyHash]",
		Short:                 "Calls the view method wallets on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bWallets,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func bWallets(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	result, err := contract.WalletsAtBlock(
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

func bDefeatFraudChallengeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "defeat-fraud-challenge [arg_walletPublicKey] [arg_preimage] [arg_witness]",
		Short:                 "Calls the nonpayable method defeatFraudChallenge on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bDefeatFraudChallenge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bDefeatFraudChallenge(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

func bDefeatFraudChallengeWithHeartbeatCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "defeat-fraud-challenge-with-heartbeat [arg_walletPublicKey] [arg_heartbeatMessage]",
		Short:                 "Calls the nonpayable method defeatFraudChallengeWithHeartbeat on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bDefeatFraudChallengeWithHeartbeat,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bDefeatFraudChallengeWithHeartbeat(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

func bEcdsaWalletCreatedCallbackCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "ecdsa-wallet-created-callback [arg_ecdsaWalletID] [arg_publicKeyX] [arg_publicKeyY]",
		Short:                 "Calls the nonpayable method ecdsaWalletCreatedCallback on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bEcdsaWalletCreatedCallback,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bEcdsaWalletCreatedCallback(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_ecdsaWalletID, err := decode.ParseBytes32(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_ecdsaWalletID, a bytes32, from passed value %v",
			args[0],
		)
	}
	arg_publicKeyX, err := decode.ParseBytes32(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_publicKeyX, a bytes32, from passed value %v",
			args[1],
		)
	}
	arg_publicKeyY, err := decode.ParseBytes32(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_publicKeyY, a bytes32, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.EcdsaWalletCreatedCallback(
			arg_ecdsaWalletID,
			arg_publicKeyX,
			arg_publicKeyY,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallEcdsaWalletCreatedCallback(
			arg_ecdsaWalletID,
			arg_publicKeyX,
			arg_publicKeyY,
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

func bEcdsaWalletHeartbeatFailedCallbackCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "ecdsa-wallet-heartbeat-failed-callback [arg0] [arg_publicKeyX] [arg_publicKeyY]",
		Short:                 "Calls the nonpayable method ecdsaWalletHeartbeatFailedCallback on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bEcdsaWalletHeartbeatFailedCallback,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bEcdsaWalletHeartbeatFailedCallback(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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
	arg_publicKeyX, err := decode.ParseBytes32(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_publicKeyX, a bytes32, from passed value %v",
			args[1],
		)
	}
	arg_publicKeyY, err := decode.ParseBytes32(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_publicKeyY, a bytes32, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.EcdsaWalletHeartbeatFailedCallback(
			arg0,
			arg_publicKeyX,
			arg_publicKeyY,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallEcdsaWalletHeartbeatFailedCallback(
			arg0,
			arg_publicKeyX,
			arg_publicKeyY,
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

func bInitializeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "initialize [arg__bank] [arg__relay] [arg__treasury] [arg__ecdsaWalletRegistry] [arg__reimbursementPool] [arg__txProofDifficultyFactor]",
		Short:                 "Calls the nonpayable method initialize on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(6),
		RunE:                  bInitialize,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bInitialize(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg__bank, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__bank, a address, from passed value %v",
			args[0],
		)
	}
	arg__relay, err := chainutil.AddressFromHex(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__relay, a address, from passed value %v",
			args[1],
		)
	}
	arg__treasury, err := chainutil.AddressFromHex(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__treasury, a address, from passed value %v",
			args[2],
		)
	}
	arg__ecdsaWalletRegistry, err := chainutil.AddressFromHex(args[3])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__ecdsaWalletRegistry, a address, from passed value %v",
			args[3],
		)
	}
	arg__reimbursementPool, err := chainutil.AddressFromHex(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__reimbursementPool, a address, from passed value %v",
			args[4],
		)
	}
	arg__txProofDifficultyFactor, err := hexutil.DecodeBig(args[5])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__txProofDifficultyFactor, a uint96, from passed value %v",
			args[5],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.Initialize(
			arg__bank,
			arg__relay,
			arg__treasury,
			arg__ecdsaWalletRegistry,
			arg__reimbursementPool,
			arg__txProofDifficultyFactor,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInitialize(
			arg__bank,
			arg__relay,
			arg__treasury,
			arg__ecdsaWalletRegistry,
			arg__reimbursementPool,
			arg__txProofDifficultyFactor,
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

func bInitializeV2FixVaultZeroDepositCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "initialize-v2-fix-vault-zero-deposit",
		Short:                 "Calls the nonpayable method initializeV2FixVaultZeroDeposit on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  bInitializeV2FixVaultZeroDeposit,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bInitializeV2FixVaultZeroDeposit(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.InitializeV2FixVaultZeroDeposit()
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInitializeV2FixVaultZeroDeposit(
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

func bInitializeV5RepairRebateStakingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "initialize-v5-repair-rebate-staking [arg_newRebateStaking]",
		Short:                 "Calls the nonpayable method initializeV5RepairRebateStaking on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bInitializeV5RepairRebateStaking,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bInitializeV5RepairRebateStaking(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_newRebateStaking, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_newRebateStaking, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.InitializeV5RepairRebateStaking(
			arg_newRebateStaking,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallInitializeV5RepairRebateStaking(
			arg_newRebateStaking,
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

func bNotifyMovingFundsBelowDustCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-moving-funds-below-dust [arg_walletPubKeyHash] [arg_mainUtxo_json]",
		Short:                 "Calls the nonpayable method notifyMovingFundsBelowDust on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bNotifyMovingFundsBelowDust,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bNotifyMovingFundsBelowDust(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	arg_mainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[1]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bNotifyRedemptionVetoCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-redemption-veto [arg_walletPubKeyHash] [arg_redeemerOutputScript]",
		Short:                 "Calls the nonpayable method notifyRedemptionVeto on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bNotifyRedemptionVeto,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bNotifyRedemptionVeto(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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
		transaction, err = contract.NotifyRedemptionVeto(
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallNotifyRedemptionVeto(
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

func bNotifyWalletCloseableCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-wallet-closeable [arg_walletPubKeyHash] [arg_walletMainUtxo_json]",
		Short:                 "Calls the nonpayable method notifyWalletCloseable on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bNotifyWalletCloseable,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bNotifyWalletCloseable(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	arg_walletMainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[1]), &arg_walletMainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_walletMainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bNotifyWalletClosingPeriodElapsedCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "notify-wallet-closing-period-elapsed [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method notifyWalletClosingPeriodElapsed on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bNotifyWalletClosingPeriodElapsed,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bNotifyWalletClosingPeriodElapsed(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

func bReceiveBalanceApprovalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "receive-balance-approval [arg_balanceOwner] [arg_amount] [arg_redemptionData]",
		Short:                 "Calls the nonpayable method receiveBalanceApproval on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bReceiveBalanceApproval,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bReceiveBalanceApproval(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_balanceOwner, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_balanceOwner, a address, from passed value %v",
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
	arg_redemptionData, err := hexutil.Decode(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionData, a bytes, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.ReceiveBalanceApproval(
			arg_balanceOwner,
			arg_amount,
			arg_redemptionData,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallReceiveBalanceApproval(
			arg_balanceOwner,
			arg_amount,
			arg_redemptionData,
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

func bRequestNewWalletCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-new-wallet [arg_activeWalletMainUtxo_json]",
		Short:                 "Calls the nonpayable method requestNewWallet on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bRequestNewWallet,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bRequestNewWallet(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_activeWalletMainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[0]), &arg_activeWalletMainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_activeWalletMainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bRequestRedemptionCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "request-redemption [arg_walletPubKeyHash] [arg_mainUtxo_json] [arg_redeemerOutputScript] [arg_amount]",
		Short:                 "Calls the nonpayable method requestRedemption on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bRequestRedemption,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bRequestRedemption(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

	arg_mainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[1]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO: %w", err)
	}
	arg_redeemerOutputScript, err := hexutil.Decode(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redeemerOutputScript, a bytes, from passed value %v",
			args[2],
		)
	}
	arg_amount, err := decode.ParseUint[uint64](args[3], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_amount, a uint64, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RequestRedemption(
			arg_walletPubKeyHash,
			arg_mainUtxo_json,
			arg_redeemerOutputScript,
			arg_amount,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRequestRedemption(
			arg_walletPubKeyHash,
			arg_mainUtxo_json,
			arg_redeemerOutputScript,
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

func bResetMovingFundsTimeoutCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reset-moving-funds-timeout [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method resetMovingFundsTimeout on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bResetMovingFundsTimeout,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bResetMovingFundsTimeout(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

func bRevealDepositCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reveal-deposit [arg_fundingTx_json] [arg_reveal_json]",
		Short:                 "Calls the nonpayable method revealDeposit on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bRevealDeposit,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bRevealDeposit(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_fundingTx_json := abi.BitcoinTxInfo{}
	if err := json.Unmarshal([]byte(args[0]), &arg_fundingTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_fundingTx_json to abi.BitcoinTxInfo: %w", err)
	}

	arg_reveal_json := abi.DepositDepositRevealInfo{}
	if err := json.Unmarshal([]byte(args[1]), &arg_reveal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_reveal_json to abi.DepositDepositRevealInfo: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RevealDeposit(
			arg_fundingTx_json,
			arg_reveal_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRevealDeposit(
			arg_fundingTx_json,
			arg_reveal_json,
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

func bRevealDepositWithExtraDataCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "reveal-deposit-with-extra-data [arg_fundingTx_json] [arg_reveal_json] [arg_extraData]",
		Short:                 "Calls the nonpayable method revealDepositWithExtraData on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bRevealDepositWithExtraData,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bRevealDepositWithExtraData(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_fundingTx_json := abi.BitcoinTxInfo{}
	if err := json.Unmarshal([]byte(args[0]), &arg_fundingTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_fundingTx_json to abi.BitcoinTxInfo: %w", err)
	}

	arg_reveal_json := abi.DepositDepositRevealInfo{}
	if err := json.Unmarshal([]byte(args[1]), &arg_reveal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_reveal_json to abi.DepositDepositRevealInfo: %w", err)
	}
	arg_extraData, err := decode.ParseBytes32(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_extraData, a bytes32, from passed value %v",
			args[2],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.RevealDepositWithExtraData(
			arg_fundingTx_json,
			arg_reveal_json,
			arg_extraData,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallRevealDepositWithExtraData(
			arg_fundingTx_json,
			arg_reveal_json,
			arg_extraData,
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

func bSetRebateStakingCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-rebate-staking [arg_rebateStaking]",
		Short:                 "Calls the nonpayable method setRebateStaking on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bSetRebateStaking,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSetRebateStaking(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_rebateStaking, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_rebateStaking, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetRebateStaking(
			arg_rebateStaking,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetRebateStaking(
			arg_rebateStaking,
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

func bSetRedemptionWatchtowerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-redemption-watchtower [arg_redemptionWatchtower]",
		Short:                 "Calls the nonpayable method setRedemptionWatchtower on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bSetRedemptionWatchtower,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSetRedemptionWatchtower(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_redemptionWatchtower, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionWatchtower, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetRedemptionWatchtower(
			arg_redemptionWatchtower,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetRedemptionWatchtower(
			arg_redemptionWatchtower,
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

func bSetReservationRouterCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-reservation-router [arg__reservationRouter]",
		Short:                 "Calls the nonpayable method setReservationRouter on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bSetReservationRouter,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSetReservationRouter(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg__reservationRouter, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg__reservationRouter, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetReservationRouter(
			arg__reservationRouter,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetReservationRouter(
			arg__reservationRouter,
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

func bSetSpvMaintainerStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-spv-maintainer-status [arg_spvMaintainer] [arg_isTrusted]",
		Short:                 "Calls the nonpayable method setSpvMaintainerStatus on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bSetSpvMaintainerStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSetSpvMaintainerStatus(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_spvMaintainer, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_spvMaintainer, a address, from passed value %v",
			args[0],
		)
	}
	arg_isTrusted, err := strconv.ParseBool(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_isTrusted, a bool, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetSpvMaintainerStatus(
			arg_spvMaintainer,
			arg_isTrusted,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetSpvMaintainerStatus(
			arg_spvMaintainer,
			arg_isTrusted,
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

func bSetVaultStatusCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "set-vault-status [arg_vault] [arg_isTrusted]",
		Short:                 "Calls the nonpayable method setVaultStatus on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  bSetVaultStatus,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSetVaultStatus(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_vault, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_vault, a address, from passed value %v",
			args[0],
		)
	}
	arg_isTrusted, err := strconv.ParseBool(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_isTrusted, a bool, from passed value %v",
			args[1],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SetVaultStatus(
			arg_vault,
			arg_isTrusted,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSetVaultStatus(
			arg_vault,
			arg_isTrusted,
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

func bSubmitDepositSweepProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-deposit-sweep-proof [arg_sweepTx_json] [arg_sweepProof_json] [arg_mainUtxo_json] [arg_vault]",
		Short:                 "Calls the nonpayable method submitDepositSweepProof on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bSubmitDepositSweepProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSubmitDepositSweepProof(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_sweepTx_json := abi.BitcoinTxInfo{}
	if err := json.Unmarshal([]byte(args[0]), &arg_sweepTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepTx_json to abi.BitcoinTxInfo: %w", err)
	}

	arg_sweepProof_json := abi.BitcoinTxProof{}
	if err := json.Unmarshal([]byte(args[1]), &arg_sweepProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepProof_json to abi.BitcoinTxProof: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bSubmitFraudChallengeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-fraud-challenge [arg_walletPublicKey] [arg_preimageSha256] [arg_signature_json]",
		Short:                 "Calls the payable method submitFraudChallenge on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bSubmitFraudChallenge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSubmitFraudChallenge(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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
	arg_preimageSha256, err := hexutil.Decode(args[1])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_preimageSha256, a bytes, from passed value %v",
			args[1],
		)
	}

	arg_signature_json := abi.BitcoinTxRSVSignature{}
	if err := json.Unmarshal([]byte(args[2]), &arg_signature_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_signature_json to abi.BitcoinTxRSVSignature: %w", err)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.SubmitFraudChallenge(
			arg_walletPublicKey,
			arg_preimageSha256,
			arg_signature_json,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallSubmitFraudChallenge(
			arg_walletPublicKey,
			arg_preimageSha256,
			arg_signature_json,
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

func bSubmitMovedFundsSweepProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-moved-funds-sweep-proof [arg_sweepTx_json] [arg_sweepProof_json] [arg_mainUtxo_json]",
		Short:                 "Calls the nonpayable method submitMovedFundsSweepProof on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(3),
		RunE:                  bSubmitMovedFundsSweepProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSubmitMovedFundsSweepProof(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_sweepTx_json := abi.BitcoinTxInfo{}
	if err := json.Unmarshal([]byte(args[0]), &arg_sweepTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepTx_json to abi.BitcoinTxInfo: %w", err)
	}

	arg_sweepProof_json := abi.BitcoinTxProof{}
	if err := json.Unmarshal([]byte(args[1]), &arg_sweepProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_sweepProof_json to abi.BitcoinTxProof: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bSubmitMovingFundsProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-moving-funds-proof [arg_movingFundsTx_json] [arg_movingFundsProof_json] [arg_mainUtxo_json] [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method submitMovingFundsProof on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bSubmitMovingFundsProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSubmitMovingFundsProof(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_movingFundsTx_json := abi.BitcoinTxInfo{}
	if err := json.Unmarshal([]byte(args[0]), &arg_movingFundsTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_movingFundsTx_json to abi.BitcoinTxInfo: %w", err)
	}

	arg_movingFundsProof_json := abi.BitcoinTxProof{}
	if err := json.Unmarshal([]byte(args[1]), &arg_movingFundsProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_movingFundsProof_json to abi.BitcoinTxProof: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bSubmitRedemptionProofCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "submit-redemption-proof [arg_redemptionTx_json] [arg_redemptionProof_json] [arg_mainUtxo_json] [arg_walletPubKeyHash]",
		Short:                 "Calls the nonpayable method submitRedemptionProof on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bSubmitRedemptionProof,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bSubmitRedemptionProof(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_redemptionTx_json := abi.BitcoinTxInfo{}
	if err := json.Unmarshal([]byte(args[0]), &arg_redemptionTx_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_redemptionTx_json to abi.BitcoinTxInfo: %w", err)
	}

	arg_redemptionProof_json := abi.BitcoinTxProof{}
	if err := json.Unmarshal([]byte(args[1]), &arg_redemptionProof_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_redemptionProof_json to abi.BitcoinTxProof: %w", err)
	}

	arg_mainUtxo_json := abi.BitcoinTxUTXO{}
	if err := json.Unmarshal([]byte(args[2]), &arg_mainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_mainUtxo_json to abi.BitcoinTxUTXO: %w", err)
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

func bTransferGovernanceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "transfer-governance [arg_newGovernance]",
		Short:                 "Calls the nonpayable method transferGovernance on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bTransferGovernance,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bTransferGovernance(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
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

func bUpdateDepositParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-deposit-parameters [arg_depositDustThreshold] [arg_depositTreasuryFeeDivisor] [arg_depositTxMaxFee] [arg_depositRevealAheadPeriod]",
		Short:                 "Calls the nonpayable method updateDepositParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bUpdateDepositParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bUpdateDepositParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_depositDustThreshold, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_depositDustThreshold, a uint64, from passed value %v",
			args[0],
		)
	}
	arg_depositTreasuryFeeDivisor, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_depositTreasuryFeeDivisor, a uint64, from passed value %v",
			args[1],
		)
	}
	arg_depositTxMaxFee, err := decode.ParseUint[uint64](args[2], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_depositTxMaxFee, a uint64, from passed value %v",
			args[2],
		)
	}
	arg_depositRevealAheadPeriod, err := decode.ParseUint[uint32](args[3], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_depositRevealAheadPeriod, a uint32, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateDepositParameters(
			arg_depositDustThreshold,
			arg_depositTreasuryFeeDivisor,
			arg_depositTxMaxFee,
			arg_depositRevealAheadPeriod,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateDepositParameters(
			arg_depositDustThreshold,
			arg_depositTreasuryFeeDivisor,
			arg_depositTxMaxFee,
			arg_depositRevealAheadPeriod,
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

func bUpdateFraudParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-fraud-parameters [arg_fraudChallengeDepositAmount] [arg_fraudChallengeDefeatTimeout] [arg_fraudSlashingAmount] [arg_fraudNotifierRewardMultiplier]",
		Short:                 "Calls the nonpayable method updateFraudParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(4),
		RunE:                  bUpdateFraudParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bUpdateFraudParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_fraudChallengeDepositAmount, err := hexutil.DecodeBig(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fraudChallengeDepositAmount, a uint96, from passed value %v",
			args[0],
		)
	}
	arg_fraudChallengeDefeatTimeout, err := decode.ParseUint[uint32](args[1], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fraudChallengeDefeatTimeout, a uint32, from passed value %v",
			args[1],
		)
	}
	arg_fraudSlashingAmount, err := hexutil.DecodeBig(args[2])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fraudSlashingAmount, a uint96, from passed value %v",
			args[2],
		)
	}
	arg_fraudNotifierRewardMultiplier, err := decode.ParseUint[uint32](args[3], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_fraudNotifierRewardMultiplier, a uint32, from passed value %v",
			args[3],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateFraudParameters(
			arg_fraudChallengeDepositAmount,
			arg_fraudChallengeDefeatTimeout,
			arg_fraudSlashingAmount,
			arg_fraudNotifierRewardMultiplier,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateFraudParameters(
			arg_fraudChallengeDepositAmount,
			arg_fraudChallengeDefeatTimeout,
			arg_fraudSlashingAmount,
			arg_fraudNotifierRewardMultiplier,
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

func bUpdateMovingFundsParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-moving-funds-parameters [arg_movingFundsTxMaxTotalFee] [arg_movingFundsDustThreshold] [arg_movingFundsTimeoutResetDelay] [arg_movingFundsTimeout] [arg_movingFundsTimeoutSlashingAmount] [arg_movingFundsTimeoutNotifierRewardMultiplier] [arg_movingFundsCommitmentGasOffset] [arg_movedFundsSweepTxMaxTotalFee] [arg_movedFundsSweepTimeout] [arg_movedFundsSweepTimeoutSlashingAmount] [arg_movedFundsSweepTimeoutNotifierRewardMultiplier]",
		Short:                 "Calls the nonpayable method updateMovingFundsParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(11),
		RunE:                  bUpdateMovingFundsParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bUpdateMovingFundsParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_movingFundsTxMaxTotalFee, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsTxMaxTotalFee, a uint64, from passed value %v",
			args[0],
		)
	}
	arg_movingFundsDustThreshold, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsDustThreshold, a uint64, from passed value %v",
			args[1],
		)
	}
	arg_movingFundsTimeoutResetDelay, err := decode.ParseUint[uint32](args[2], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsTimeoutResetDelay, a uint32, from passed value %v",
			args[2],
		)
	}
	arg_movingFundsTimeout, err := decode.ParseUint[uint32](args[3], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsTimeout, a uint32, from passed value %v",
			args[3],
		)
	}
	arg_movingFundsTimeoutSlashingAmount, err := hexutil.DecodeBig(args[4])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsTimeoutSlashingAmount, a uint96, from passed value %v",
			args[4],
		)
	}
	arg_movingFundsTimeoutNotifierRewardMultiplier, err := decode.ParseUint[uint32](args[5], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsTimeoutNotifierRewardMultiplier, a uint32, from passed value %v",
			args[5],
		)
	}
	arg_movingFundsCommitmentGasOffset, err := decode.ParseUint[uint16](args[6], 16)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movingFundsCommitmentGasOffset, a uint16, from passed value %v",
			args[6],
		)
	}
	arg_movedFundsSweepTxMaxTotalFee, err := decode.ParseUint[uint64](args[7], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movedFundsSweepTxMaxTotalFee, a uint64, from passed value %v",
			args[7],
		)
	}
	arg_movedFundsSweepTimeout, err := decode.ParseUint[uint32](args[8], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movedFundsSweepTimeout, a uint32, from passed value %v",
			args[8],
		)
	}
	arg_movedFundsSweepTimeoutSlashingAmount, err := hexutil.DecodeBig(args[9])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movedFundsSweepTimeoutSlashingAmount, a uint96, from passed value %v",
			args[9],
		)
	}
	arg_movedFundsSweepTimeoutNotifierRewardMultiplier, err := decode.ParseUint[uint32](args[10], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_movedFundsSweepTimeoutNotifierRewardMultiplier, a uint32, from passed value %v",
			args[10],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateMovingFundsParameters(
			arg_movingFundsTxMaxTotalFee,
			arg_movingFundsDustThreshold,
			arg_movingFundsTimeoutResetDelay,
			arg_movingFundsTimeout,
			arg_movingFundsTimeoutSlashingAmount,
			arg_movingFundsTimeoutNotifierRewardMultiplier,
			arg_movingFundsCommitmentGasOffset,
			arg_movedFundsSweepTxMaxTotalFee,
			arg_movedFundsSweepTimeout,
			arg_movedFundsSweepTimeoutSlashingAmount,
			arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateMovingFundsParameters(
			arg_movingFundsTxMaxTotalFee,
			arg_movingFundsDustThreshold,
			arg_movingFundsTimeoutResetDelay,
			arg_movingFundsTimeout,
			arg_movingFundsTimeoutSlashingAmount,
			arg_movingFundsTimeoutNotifierRewardMultiplier,
			arg_movingFundsCommitmentGasOffset,
			arg_movedFundsSweepTxMaxTotalFee,
			arg_movedFundsSweepTimeout,
			arg_movedFundsSweepTimeoutSlashingAmount,
			arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
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

func bUpdateRedemptionParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-redemption-parameters [arg_redemptionDustThreshold] [arg_redemptionTreasuryFeeDivisor] [arg_redemptionTxMaxFee] [arg_redemptionTxMaxTotalFee] [arg_redemptionTimeout] [arg_redemptionTimeoutSlashingAmount] [arg_redemptionTimeoutNotifierRewardMultiplier]",
		Short:                 "Calls the nonpayable method updateRedemptionParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(7),
		RunE:                  bUpdateRedemptionParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bUpdateRedemptionParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_redemptionDustThreshold, err := decode.ParseUint[uint64](args[0], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionDustThreshold, a uint64, from passed value %v",
			args[0],
		)
	}
	arg_redemptionTreasuryFeeDivisor, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionTreasuryFeeDivisor, a uint64, from passed value %v",
			args[1],
		)
	}
	arg_redemptionTxMaxFee, err := decode.ParseUint[uint64](args[2], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionTxMaxFee, a uint64, from passed value %v",
			args[2],
		)
	}
	arg_redemptionTxMaxTotalFee, err := decode.ParseUint[uint64](args[3], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionTxMaxTotalFee, a uint64, from passed value %v",
			args[3],
		)
	}
	arg_redemptionTimeout, err := decode.ParseUint[uint32](args[4], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionTimeout, a uint32, from passed value %v",
			args[4],
		)
	}
	arg_redemptionTimeoutSlashingAmount, err := hexutil.DecodeBig(args[5])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionTimeoutSlashingAmount, a uint96, from passed value %v",
			args[5],
		)
	}
	arg_redemptionTimeoutNotifierRewardMultiplier, err := decode.ParseUint[uint32](args[6], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_redemptionTimeoutNotifierRewardMultiplier, a uint32, from passed value %v",
			args[6],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateRedemptionParameters(
			arg_redemptionDustThreshold,
			arg_redemptionTreasuryFeeDivisor,
			arg_redemptionTxMaxFee,
			arg_redemptionTxMaxTotalFee,
			arg_redemptionTimeout,
			arg_redemptionTimeoutSlashingAmount,
			arg_redemptionTimeoutNotifierRewardMultiplier,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateRedemptionParameters(
			arg_redemptionDustThreshold,
			arg_redemptionTreasuryFeeDivisor,
			arg_redemptionTxMaxFee,
			arg_redemptionTxMaxTotalFee,
			arg_redemptionTimeout,
			arg_redemptionTimeoutSlashingAmount,
			arg_redemptionTimeoutNotifierRewardMultiplier,
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

func bUpdateTreasuryCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-treasury [arg_treasury]",
		Short:                 "Calls the nonpayable method updateTreasury on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  bUpdateTreasury,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bUpdateTreasury(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_treasury, err := chainutil.AddressFromHex(args[0])
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_treasury, a address, from passed value %v",
			args[0],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateTreasury(
			arg_treasury,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateTreasury(
			arg_treasury,
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

func bUpdateWalletParametersCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "update-wallet-parameters [arg_walletCreationPeriod] [arg_walletCreationMinBtcBalance] [arg_walletCreationMaxBtcBalance] [arg_walletClosureMinBtcBalance] [arg_walletMaxAge] [arg_walletMaxBtcTransfer] [arg_walletClosingPeriod]",
		Short:                 "Calls the nonpayable method updateWalletParameters on the Bridge contract.",
		Args:                  cmd.ArgCountChecker(7),
		RunE:                  bUpdateWalletParameters,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	c.PreRunE = cmd.NonConstArgsChecker
	cmd.InitNonConstFlags(c)

	return c
}

func bUpdateWalletParameters(c *cobra.Command, args []string) error {
	contract, err := initializeBridge(c)
	if err != nil {
		return err
	}

	arg_walletCreationPeriod, err := decode.ParseUint[uint32](args[0], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletCreationPeriod, a uint32, from passed value %v",
			args[0],
		)
	}
	arg_walletCreationMinBtcBalance, err := decode.ParseUint[uint64](args[1], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletCreationMinBtcBalance, a uint64, from passed value %v",
			args[1],
		)
	}
	arg_walletCreationMaxBtcBalance, err := decode.ParseUint[uint64](args[2], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletCreationMaxBtcBalance, a uint64, from passed value %v",
			args[2],
		)
	}
	arg_walletClosureMinBtcBalance, err := decode.ParseUint[uint64](args[3], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletClosureMinBtcBalance, a uint64, from passed value %v",
			args[3],
		)
	}
	arg_walletMaxAge, err := decode.ParseUint[uint32](args[4], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletMaxAge, a uint32, from passed value %v",
			args[4],
		)
	}
	arg_walletMaxBtcTransfer, err := decode.ParseUint[uint64](args[5], 64)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletMaxBtcTransfer, a uint64, from passed value %v",
			args[5],
		)
	}
	arg_walletClosingPeriod, err := decode.ParseUint[uint32](args[6], 32)
	if err != nil {
		return fmt.Errorf(
			"couldn't parse parameter arg_walletClosingPeriod, a uint32, from passed value %v",
			args[6],
		)
	}

	var (
		transaction *types.Transaction
	)

	if shouldSubmit, _ := c.Flags().GetBool(cmd.SubmitFlag); shouldSubmit {
		// Do a regular submission. Take payable into account.
		transaction, err = contract.UpdateWalletParameters(
			arg_walletCreationPeriod,
			arg_walletCreationMinBtcBalance,
			arg_walletCreationMaxBtcBalance,
			arg_walletClosureMinBtcBalance,
			arg_walletMaxAge,
			arg_walletMaxBtcTransfer,
			arg_walletClosingPeriod,
		)
		if err != nil {
			return err
		}

		cmd.PrintOutput(transaction.Hash())
	} else {
		// Do a call.
		err = contract.CallUpdateWalletParameters(
			arg_walletCreationPeriod,
			arg_walletCreationMinBtcBalance,
			arg_walletCreationMaxBtcBalance,
			arg_walletClosureMinBtcBalance,
			arg_walletMaxAge,
			arg_walletMaxBtcTransfer,
			arg_walletClosingPeriod,
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

func initializeBridge(c *cobra.Command) (*contract.Bridge, error) {
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

	address, err := cfg.ContractAddress("Bridge")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"Bridge",
			err,
		)
	}

	return contract.NewBridge(
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
