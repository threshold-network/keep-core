package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/maintainer"
)

// MaintainerCommand contains the definition of the maintainer command-line
// subcommand.
var MaintainerCommand = &cobra.Command{
	Use:   "maintainer",
	Short: `(experimental) Starts maintainers`,
	Long:  `(experimental) The maintainer command starts maintainers`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if err := clientConfig.ReadConfig(
			configFilePath,
			cmd.Flags(),
			config.MaintainerCategories...,
		); err != nil {
			logger.Fatalf("error reading config: %v", err)
		}
	},
	RunE: maintainers,
}

func init() {
	initFlags(
		MaintainerCommand,
		&configFilePath,
		clientConfig,
		config.MaintainerCategories...,
	)
}

// maintainers initializes maintainer tasks specified by flags passed to the
// maintainer command.
func maintainers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	btcChain, err := electrum.Connect(ctx, clientConfig.Bitcoin.Network, clientConfig.Bitcoin.Electrum)
	if err != nil {
		return fmt.Errorf("could not connect to Electrum chain: [%v]", err)
	}

	btcDiffChain, err := ethereum.ConnectBitcoinDifficulty(
		ctx,
		clientConfig.Ethereum,
		clientConfig.Maintainer,
	)
	if err != nil {
		return fmt.Errorf(
			"could not connect to Bitcoin difficulty chain: [%v]",
			err,
		)
	}

	_, tbtcChain, _, _, _, err := ethereum.Connect(
		ctx,
		clientConfig.Ethereum,
	)
	if err != nil {
		return fmt.Errorf(
			"could not connect to tBTC chain: [%v]",
			err,
		)
	}

	maintainer.Initialize(
		ctx,
		clientConfig.Maintainer,
		btcChain,
		btcDiffChain,
		tbtcChain,
	)

	<-ctx.Done()
	return fmt.Errorf("unexpected context cancellation")
}
