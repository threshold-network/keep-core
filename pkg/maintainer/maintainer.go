package maintainer

import (
	"context"

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
	// Configuration is validated at config-load time and again inside
	// spv.Initialize (the only module that requires pre-launch validation),
	// so we don't re-validate here to avoid a redundant pass.
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
			return err
		}
	}

	return nil
}
