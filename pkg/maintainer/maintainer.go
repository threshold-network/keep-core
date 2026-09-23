package maintainer

import (
	"context"
	"fmt"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
)

var logger = log.Logger("keep-maintainer")

func Initialize(
	ctx context.Context,
	config Config,
	btcChain bitcoin.Chain,
	btcDiffChain btcdiff.Chain,
	spvChain spv.Chain,
	metricsRecorder spv.MetricsRecorder,
) error {
	if err := config.Validate(); err != nil {
		return err
	}
	// If none of the maintainers was specified in the config (i.e. no option was
	// provided to the `maintainer` command), all maintainers should be launched.
	launchAll := !config.BitcoinDifficulty.Enabled &&
		!config.Spv.Enabled

	if launchAll {
		logger.Info("initializing all maintainer modules...")
	}

	if config.BitcoinDifficulty.Enabled || launchAll {
		btcdiff.Initialize(
			ctx,
			config.BitcoinDifficulty,
			btcChain,
			btcDiffChain,
		)
	}

	if config.Spv.Enabled || launchAll {
		err := spv.Initialize(
			ctx,
			config.Spv,
			spvChain,
			btcDiffChain,
			btcChain,
			metricsRecorder,
		)
		if err != nil {
			return fmt.Errorf("cannot initialize spv maintainer: [%w]", err)
		}
	}

	return nil
}
