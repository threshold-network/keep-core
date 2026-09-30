// Code generated - DO NOT EDIT.
// This file is a generated command and any manual changes will be lost.

package cmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"

	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/cmd"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"

	"github.com/spf13/cobra"
)

var ReservationVaultCommand *cobra.Command

var reservationVaultDescription = `The reservation-vault command allows calling the ReservationVault contract on an
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
	ReservationVaultCommand := &cobra.Command{
		Use:   "reservation-vault",
		Short: `Provides access to the ReservationVault contract.`,
		Long:  reservationVaultDescription,
	}

	ReservationVaultCommand.AddCommand(
		rvInKindFeeDebtSatCommand(),
		rvTbtcTokenCommand(),
	)

	ModuleCommand.AddCommand(ReservationVaultCommand)
}

/// ------------------- Const methods -------------------

func rvInKindFeeDebtSatCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "in-kind-fee-debt-sat",
		Short:                 "Calls the view method inKindFeeDebtSat on the ReservationVault contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rvInKindFeeDebtSat,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rvInKindFeeDebtSat(c *cobra.Command, args []string) error {
	contract, err := initializeReservationVault(c)
	if err != nil {
		return err
	}

	result, err := contract.InKindFeeDebtSatAtBlock(
		cmd.BlockFlagValue.Int,
	)

	if err != nil {
		return err
	}

	cmd.PrintOutput(result)

	return nil
}

func rvTbtcTokenCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "tbtc-token",
		Short:                 "Calls the view method tbtcToken on the ReservationVault contract.",
		Args:                  cmd.ArgCountChecker(0),
		RunE:                  rvTbtcToken,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.InitConstFlags(c)

	return c
}

func rvTbtcToken(c *cobra.Command, args []string) error {
	contract, err := initializeReservationVault(c)
	if err != nil {
		return err
	}

	result, err := contract.TbtcTokenAtBlock(
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

func initializeReservationVault(c *cobra.Command) (*contract.ReservationVault, error) {
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

	address, err := cfg.ContractAddress("ReservationVault")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get %s address: [%w]",
			"ReservationVault",
			err,
		)
	}

	return contract.NewReservationVault(
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
