// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"

	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/cmd"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"

	"github.com/spf13/cobra"
)

var WalletProposalValidatorCommand *cobra.Command

var walletProposalValidatorDescription = `The wallet-proposal-validator command allows calling the WalletProposalValidator contract on an
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
	WalletProposalValidatorCommand := &cobra.Command{
		Use:   "wallet-proposal-validator",
		Short: `Provides access to the WalletProposalValidator contract.`,
		Long:  walletProposalValidatorDescription,
	}

	WalletProposalValidatorCommand.AddCommand(
		wpvBridgeCommand(),
		wpvDEPOSITMINAGECommand(),
		wpvDEPOSITREFUNDSAFETYMARGINCommand(),
		wpvDEPOSITSWEEPMAXSIZECommand(),
		wpvREDEMPTIONMAXSIZECommand(),
		wpvREDEMPTIONREQUESTMINAGECommand(),
		wpvREDEMPTIONREQUESTTIMEOUTSAFETYMARGINCommand(),
		wpvValidateHeartbeatProposalCommand(),
		wpvValidateMovedFundsSweepProposalCommand(),
		wpvValidateMovingFundsProposalCommand(),
		wpvValidateRedemptionProposalCommand(),
		wpvValidateReservationAnchorProposalCommand(),
		wpvValidateReservationReanchorProposalCommand(),
	)

	ModuleCommand.AddCommand(WalletProposalValidatorCommand)
}

/// ------------------- Const methods -------------------

func wpvBridgeCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "bridge",
		Short:                 "Calls the view method bridge on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvBridge,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvBridge(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
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

func wpvDEPOSITMINAGECommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "d-e-p-o-s-i-t-m-i-n-a-g-e",
		Short:                 "Calls the view method dEPOSITMINAGE on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvDEPOSITMINAGE,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvDEPOSITMINAGE(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	result, err := contract.DEPOSITMINAGEAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvDEPOSITREFUNDSAFETYMARGINCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "d-e-p-o-s-i-t-r-e-f-u-n-d-s-a-f-e-t-y-m-a-r-g-i-n",
		Short:                 "Calls the view method dEPOSITREFUNDSAFETYMARGIN on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvDEPOSITREFUNDSAFETYMARGIN,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvDEPOSITREFUNDSAFETYMARGIN(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	result, err := contract.DEPOSITREFUNDSAFETYMARGINAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvDEPOSITSWEEPMAXSIZECommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "d-e-p-o-s-i-t-s-w-e-e-p-m-a-x-s-i-z-e",
		Short:                 "Calls the view method dEPOSITSWEEPMAXSIZE on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvDEPOSITSWEEPMAXSIZE,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvDEPOSITSWEEPMAXSIZE(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	result, err := contract.DEPOSITSWEEPMAXSIZEAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvREDEMPTIONMAXSIZECommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "r-e-d-e-m-p-t-i-o-n-m-a-x-s-i-z-e",
		Short:                 "Calls the view method rEDEMPTIONMAXSIZE on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvREDEMPTIONMAXSIZE,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvREDEMPTIONMAXSIZE(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	result, err := contract.REDEMPTIONMAXSIZEAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvREDEMPTIONREQUESTMINAGECommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "r-e-d-e-m-p-t-i-o-n-r-e-q-u-e-s-t-m-i-n-a-g-e",
		Short:                 "Calls the view method rEDEMPTIONREQUESTMINAGE on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvREDEMPTIONREQUESTMINAGE,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvREDEMPTIONREQUESTMINAGE(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	result, err := contract.REDEMPTIONREQUESTMINAGEAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvREDEMPTIONREQUESTTIMEOUTSAFETYMARGINCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "r-e-d-e-m-p-t-i-o-n-r-e-q-u-e-s-t-t-i-m-e-o-u-t-s-a-f-e-t-y-m-a-r-g-i-n",
		Short:                 "Calls the view method rEDEMPTIONREQUESTTIMEOUTSAFETYMARGIN on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  wpvREDEMPTIONREQUESTTIMEOUTSAFETYMARGIN,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvREDEMPTIONREQUESTTIMEOUTSAFETYMARGIN(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	result, err := contract.REDEMPTIONREQUESTTIMEOUTSAFETYMARGINAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvValidateHeartbeatProposalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-heartbeat-proposal [arg_proposal_json]",
		Short:                 "Calls the view method validateHeartbeatProposal on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wpvValidateHeartbeatProposal,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvValidateHeartbeatProposal(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	arg_proposal_json := abi.WalletProposalValidatorHeartbeatProposal{}
	if err := json.Unmarshal([]byte(args[0]), &arg_proposal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proposal_json to abi.WalletProposalValidatorHeartbeatProposal: %w", err)
	}

	result, err := contract.ValidateHeartbeatProposalAtBlock(
		arg_proposal_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvValidateMovedFundsSweepProposalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-moved-funds-sweep-proposal [arg_proposal_json]",
		Short:                 "Calls the view method validateMovedFundsSweepProposal on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wpvValidateMovedFundsSweepProposal,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvValidateMovedFundsSweepProposal(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	arg_proposal_json := abi.WalletProposalValidatorMovedFundsSweepProposal{}
	if err := json.Unmarshal([]byte(args[0]), &arg_proposal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proposal_json to abi.WalletProposalValidatorMovedFundsSweepProposal: %w", err)
	}

	result, err := contract.ValidateMovedFundsSweepProposalAtBlock(
		arg_proposal_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvValidateMovingFundsProposalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-moving-funds-proposal [arg_proposal_json] [arg_walletMainUtxo_json]",
		Short:                 "Calls the view method validateMovingFundsProposal on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  wpvValidateMovingFundsProposal,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvValidateMovingFundsProposal(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	arg_proposal_json := abi.WalletProposalValidatorMovingFundsProposal{}
	if err := json.Unmarshal([]byte(args[0]), &arg_proposal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proposal_json to abi.WalletProposalValidatorMovingFundsProposal: %w", err)
	}

	arg_walletMainUtxo_json := abi.BitcoinTxUTXO3{}
	if err := json.Unmarshal([]byte(args[1]), &arg_walletMainUtxo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_walletMainUtxo_json to abi.BitcoinTxUTXO3: %w", err)
	}

	result, err := contract.ValidateMovingFundsProposalAtBlock(
		arg_proposal_json,
		arg_walletMainUtxo_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvValidateRedemptionProposalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-redemption-proposal [arg_proposal_json]",
		Short:                 "Calls the view method validateRedemptionProposal on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wpvValidateRedemptionProposal,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvValidateRedemptionProposal(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	arg_proposal_json := abi.WalletProposalValidatorRedemptionProposal{}
	if err := json.Unmarshal([]byte(args[0]), &arg_proposal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proposal_json to abi.WalletProposalValidatorRedemptionProposal: %w", err)
	}

	result, err := contract.ValidateRedemptionProposalAtBlock(
		arg_proposal_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvValidateReservationAnchorProposalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-reservation-anchor-proposal [arg_proposal_json] [arg_depositExtraInfo_json]",
		Short:                 "Calls the view method validateReservationAnchorProposal on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(2),
		RunE:                  wpvValidateReservationAnchorProposal,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvValidateReservationAnchorProposal(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	arg_proposal_json := abi.WalletProposalValidatorReservationAnchorProposal{}
	if err := json.Unmarshal([]byte(args[0]), &arg_proposal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proposal_json to abi.WalletProposalValidatorReservationAnchorProposal: %w", err)
	}

	arg_depositExtraInfo_json := abi.WalletProposalValidatorDepositExtraInfo{}
	if err := json.Unmarshal([]byte(args[1]), &arg_depositExtraInfo_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_depositExtraInfo_json to abi.WalletProposalValidatorDepositExtraInfo: %w", err)
	}

	result, err := contract.ValidateReservationAnchorProposalAtBlock(
		arg_proposal_json,
		arg_depositExtraInfo_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func wpvValidateReservationReanchorProposalCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "validate-reservation-reanchor-proposal [arg_proposal_json]",
		Short:                 "Calls the view method validateReservationReanchorProposal on the WalletProposalValidator contract.",
		Args:                  cmd.ArgCountChecker(1),
		RunE:                  wpvValidateReservationReanchorProposal,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func wpvValidateReservationReanchorProposal(c *cobra.Command, args []string) error {
	contract, err := initializeWalletProposalValidator(c)
	if err != nil {
		return err
	}

	arg_proposal_json := abi.WalletProposalValidatorReservationReanchorProposal{}
	if err := json.Unmarshal([]byte(args[0]), &arg_proposal_json); err != nil {
		return fmt.Errorf("failed to unmarshal arg_proposal_json to abi.WalletProposalValidatorReservationReanchorProposal: %w", err)
	}

	result, err := contract.ValidateReservationReanchorProposalAtBlock(
		arg_proposal_json,
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

/// ------------------- Non-const methods -------------------

/// ------------------- Initialization -------------------

func initializeWalletProposalValidator(c *cobra.Command) (*contract.WalletProposalValidator, error) {
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

	address, err := cfg.ContractAddress("WalletProposalValidator")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"WalletProposalValidator",
			err,
		)
	}

	return contract.NewWalletProposalValidator(
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
