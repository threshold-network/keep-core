package btcdiff

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

var logger = log.Logger("keep-maintainer-btcdiff")

const (
	// Default value for back-off time which should be applied when the Bitcoin
	// difficulty maintainer is restarted. It helps to avoid being flooded with
	// error logs in case of a permanent error in the Bitcoin difficulty
	// maintainer.
	bitcoinDifficultyDefaultRestartBackoffTime = 120 * time.Second

	// Default value for back-off time which should be applied when there are no
	// more Bitcoin epochs to be proven because the difficulty maintainer is
	// up-to-date with the Bitcoin blockchain or there are not enough blocks yet
	// to prove the new epoch.
	bitcoinDifficultyDefaultIdleBackOffTime = 60 * time.Second

	// The number of blocks in a Bitcoin difficulty epoch.
	bitcoinDifficultyEpochLength = 2016
)

var (
	errNotAuthorized = fmt.Errorf(
		"bitcoin difficulty maintainer has not been authorized to submit " +
			"block headers",
	)
	errNoGenesis = fmt.Errorf(
		"genesis has not been performed in the Bitcoin difficulty chain",
	)
)

// lightRelayMinDifficultyTarget matches LightRelay.MIN_DIFFICULTY_TARGET /
// BTCUtils.DIFF1_TARGET (compact bits 0x1d00ffff).
var lightRelayMinDifficultyTarget = blockchain.CompactToBig(0x1d00ffff)

func Initialize(
	ctx context.Context,
	config Config,
	btcChain bitcoin.Chain,
	chain Chain,
) {
	if config.RestartBackOffTime == 0 {
		config.RestartBackOffTime = bitcoinDifficultyDefaultRestartBackoffTime
	}
	if config.IdleBackOffTime == 0 {
		config.IdleBackOffTime = bitcoinDifficultyDefaultIdleBackOffTime
	}

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		config:   config,
		btcChain: btcChain,
		chain:    chain,
	}

	go bitcoinDifficultyMaintainer.startControlLoop(ctx)
}

// bitcoinDifficultyMaintainer is the part of maintainer responsible for
// maintaining the state of the Bitcoin difficulty on-chain contract.
type bitcoinDifficultyMaintainer struct {
	config   Config
	btcChain bitcoin.Chain
	chain    Chain
}

// startControlLoop starts the loop responsible for controlling the Bitcoin
// difficulty maintainer.
func (bdm *bitcoinDifficultyMaintainer) startControlLoop(ctx context.Context) {
	logger.Info("starting Bitcoin difficulty maintainer")

	defer func() {
		logger.Info("stopping Bitcoin difficulty maintainer")
	}()

	for {
		err := bdm.proveEpochs(ctx)
		if err != nil {
			logger.Errorf(
				"error while proving Bitcoin epochs: [%v]; restarting maintainer",
				err,
			)
		}

		select {
		case <-time.After(bdm.config.RestartBackOffTime):
		case <-ctx.Done():
			return
		}
	}
}

// proveEpochs proves Bitcoin blockchain epochs in the Bitcoin difficulty chain.
func (bdm *bitcoinDifficultyMaintainer) proveEpochs(ctx context.Context) error {
	if err := bdm.verifySubmissionEligibility(); err != nil {
		return fmt.Errorf(
			"cannot verify submission eligibility: [%w]",
			err,
		)
	}

	for {
		epochProven, err := bdm.proveNextEpoch(ctx)
		if err != nil {
			return fmt.Errorf(
				"cannot prove Bitcoin blockchain epoch: [%w]",
				err,
			)
		}

		// Sleep for some time if the Bitcoin epoch was not proven (i.e. Bitcoin
		// difficulty chain is up-to-date or there are not enough block headers
		// in the new epoch). Do not sleep if a Bitcoin epoch was proven as
		// there are likely more Bitcoin epochs to prove.
		if !epochProven {
			select {
			case <-time.After(bdm.config.IdleBackOffTime):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// verifySubmissionEligibility verifies whether a maintainer is eligible to
// submit block headers to the Bitcoin difficulty chain.
func (bdm *bitcoinDifficultyMaintainer) verifySubmissionEligibility() error {
	isReady, err := bdm.chain.Ready()
	if err != nil {
		return fmt.Errorf(
			"cannot check whether genesis has been performed: [%w]",
			err,
		)
	}

	if !isReady {
		return errNoGenesis
	}

	maintainerAddress := bdm.chain.Signing().Address()

	if bdm.config.DisableProxy {
		// Retargetting will be done directly via `LightRelay` without refunding.
		isAuthorized, err := bdm.chain.IsAuthorized(maintainerAddress)
		if err != nil {
			return fmt.Errorf(
				"cannot check whether Bitcoin difficulty maintainer is authorized "+
					"to submit block headers: [%w]",
				err,
			)
		}

		if !isAuthorized {
			return errNotAuthorized
		}

		return nil
	}

	// Retargetting will be done via `LightRelayMaintainerProxy` with refunding.
	isAuthorizedForRefund, err := bdm.chain.IsAuthorizedForRefund(
		maintainerAddress,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot check whether Bitcoin difficulty maintainer is authorized "+
				"to submit block headers with refunding: [%w]",
			err,
		)
	}

	if !isAuthorizedForRefund {
		return errNotAuthorized
	}

	return nil
}

// proveNextEpoch proves a single Bitcoin blockchain epoch in the Bitcoin
// difficulty chain if there is an epoch to be proven. If it was possible to
// prove an epoch, it returns true. If it was not possible (i.e. Bitcoin
// difficulty chain is up-to-date or there are not enough headers in the new
// Bitcoin epoch), it returns false.
func (bdm *bitcoinDifficultyMaintainer) proveNextEpoch(ctx context.Context) (
	bool,
	error,
) {
	// The height of the Bitcoin blockchain.
	currentBlockHeight, err := bdm.btcChain.GetLatestBlockHeight()
	if err != nil {
		return false, fmt.Errorf(
			"failed to get latest block height: [%w]",
			err,
		)
	}

	// The current epoch proven in the Bitcoin difficulty chain.
	currentEpoch, err := bdm.chain.CurrentEpoch()
	if err != nil {
		return false, fmt.Errorf(
			"failed to get current epoch: [%w]",
			err,
		)
	}

	// The number of blocks required for each side of a retarget proof.
	proofLength, err := bdm.chain.ProofLength()
	if err != nil {
		return false, fmt.Errorf(
			"failed to get proof length: [%w]",
			err,
		)
	}

	// The new epoch to be proven in the Bitcoin difficulty chain.
	newEpoch := uint(currentEpoch) + 1

	// Height of the first block of the new epoch.
	newEpochHeight := newEpoch * bitcoinDifficultyEpochLength

	// The range of block headers to be pulled from the Bitcoin chain should
	// start `proofLength` blocks before the first block of the new difficulty
	// epoch and end `proofLength`-1 after it.
	// For example, if the new epoch begins at block 522144 and `proofLength`
	// is 3, then the range should be [522141, 522146]:
	// 522141 <- old difficulty target
	// 522142 <- old difficulty target
	// 522143 <- old difficulty target
	// << difficulty retarget >>
	// 522144 <- new difficulty target (first block of the new epoch)
	// 522145 <- new difficulty target
	// 522146 <- new difficulty target
	firstBlockHeaderHeight := newEpochHeight - uint(proofLength)
	lastBlockHeaderHeight := newEpochHeight + uint(proofLength) - 1

	// The required range of block headers can be pulled from the Bitcoin
	// blockchain only if the blockchain height is equal to or greater than
	// the end of the range.
	if currentBlockHeight >= lastBlockHeaderHeight {
		lastPreRetargetHeight := newEpochHeight - 1
		if err := bdm.verifyUniformPreRetargetDifficulty(
			currentEpoch,
			firstBlockHeaderHeight,
			lastPreRetargetHeight,
		); err != nil {
			if bdm.config.IdleOnPreflightFailure &&
				errors.Is(err, ErrUniformPreRetargetDifficulty) {
				logger.Warnf(
					"idling bitcoin difficulty maintainer (%v); "+
						"set bitcoinDifficulty.idleOnPreflightFailure=false to "+
						"fail fast instead",
					err,
				)
				return false, nil
			}
			return false, err
		}

		headers, err := bdm.getBlockHeaders(
			firstBlockHeaderHeight,
			lastBlockHeaderHeight,
		)
		if err != nil {
			return false, fmt.Errorf(
				"failed to get block headers from Bitcoin chain: [%w]",
				err,
			)
		}

		if bdm.config.DisableProxy {
			if err := bdm.chain.Retarget(headers); err != nil {
				return false, fmt.Errorf(
					"failed to submit block headers from range [%d:%d] via "+
						"Retarget: [%w]",
					firstBlockHeaderHeight,
					lastBlockHeaderHeight,
					err,
				)
			}
		} else {
			if err := bdm.chain.RetargetWithRefund(headers); err != nil {
				return false, fmt.Errorf(
					"failed to submit block headers from range [%d:%d] via "+
						"RetargetWithRefund: [%w]",
					firstBlockHeaderHeight,
					lastBlockHeaderHeight,
					err,
				)
			}
		}

		if err := bdm.waitForCurrentEpochUpdate(ctx, uint64(newEpoch)); err != nil {
			return false, fmt.Errorf(
				"error while waiting for current Bitcoin difficulty epoch "+
					"update: [%w]",
				err,
			)
		}

		logger.Infof(
			"successfully submitted block headers [%d:%d] to the Bitcoin "+
				"difficulty chain; the current proven epoch is [%d]",
			firstBlockHeaderHeight,
			lastBlockHeaderHeight,
			newEpoch,
		)

		return true, nil
	}

	if currentBlockHeight >= newEpochHeight {
		logger.Infof(
			"the Bitcoin difficulty chain has to be synced with the "+
				"Bitcoin blockchain; waiting for [%d] new blocks to "+
				"be mined to form a headers chain for retarget",
			lastBlockHeaderHeight-currentBlockHeight,
		)
	} else {
		logger.Infof(
			"the Bitcoin difficulty chain is up-to-date with the Bitcoin " +
				"blockchain",
		)
	}

	return false, nil
}

// verifyUniformPreRetargetDifficulty checks that each Bitcoin header on the
// pre-retarget side of the relay proof is acceptable to LightRelay: each
// header's difficulty target must equal the current epoch's target (from the
// block at the epoch start) or the minimum-difficulty target (DIFF1), matching
// LightRelay.retarget's pre-retarget loop. Mainnet typically only uses the
// epoch target; Bitcoin testnet4 may insert minimum-difficulty blocks between
// retargets while other blocks use the epoch difficulty.
func (bdm *bitcoinDifficultyMaintainer) verifyUniformPreRetargetDifficulty(
	currentEpoch uint64,
	firstPreRetargetHeight uint,
	lastPreRetargetHeight uint,
) error {
	anchorHeight := uint(currentEpoch) * bitcoinDifficultyEpochLength
	anchor, err := bdm.btcChain.GetBlockHeader(anchorHeight)
	if err != nil {
		return fmt.Errorf(
			"cannot load anchor header at epoch start height %d for "+
				"pre-retarget difficulty check: %w",
			anchorHeight,
			err,
		)
	}

	oldTarget := anchor.Target()

	for h := firstPreRetargetHeight; h <= lastPreRetargetHeight; h++ {
		hdr, err := bdm.btcChain.GetBlockHeader(h)
		if err != nil {
			return fmt.Errorf(
				"cannot load header %d for pre-retarget difficulty check: %w",
				h,
				err,
			)
		}

		headerTarget := hdr.Target()
		if relayAllowsPreRetargetHeaderTarget(oldTarget, headerTarget) {
			continue
		}

		return fmt.Errorf(
			"%w: bitcoin header at height %d has target %v but epoch %d "+
				"anchor (height %d) has target %v; LightRelay allows the anchor "+
				"target or minimum-difficulty only",
			ErrUniformPreRetargetDifficulty,
			h,
			headerTarget,
			currentEpoch,
			anchorHeight,
			oldTarget,
		)
	}

	return nil
}

func relayAllowsPreRetargetHeaderTarget(oldEpochTarget, headerTarget *big.Int) bool {
	if oldEpochTarget.Cmp(headerTarget) == 0 {
		return true
	}
	return lightRelayMinDifficultyTarget.Cmp(headerTarget) == 0
}

// getBlockHeaders returns block headers from the given range.
func (bdm *bitcoinDifficultyMaintainer) getBlockHeaders(
	firstHeaderHeight,
	lastHeaderHeight uint,
) (
	[]*bitcoin.BlockHeader, error,
) {
	var headers []*bitcoin.BlockHeader

	// TODO: Consider modifying the Bitcoin chain to return multiple block
	//       headers: GetBlockHeaders(startHeight, count). Return multiple
	//       block headers with one call instead of iterating.
	for height := firstHeaderHeight; height <= lastHeaderHeight; height++ {
		header, err := bdm.btcChain.GetBlockHeader(height)
		if err != nil {
			return []*bitcoin.BlockHeader{}, fmt.Errorf(
				"failed to get block header at height %d: [%w]",
				height,
				err,
			)
		}

		headers = append(headers, header)
	}

	return headers, nil
}

// waitForCurrentEpochUpdate waits until the current epoch in the Bitcoin
// difficulty chain is equal to or higher than the provided target epoch.
func (bdm *bitcoinDifficultyMaintainer) waitForCurrentEpochUpdate(
	ctx context.Context,
	targetEpoch uint64,
) error {
	for {
		currentEpoch, err := bdm.chain.CurrentEpoch()
		if err != nil {
			return fmt.Errorf("failed to get current epoch: [%w]", err)
		}

		if currentEpoch >= targetEpoch {
			break
		}

		logger.Infof(
			"waiting for bitcoin difficulty chain to reach epoch %d, "+
				"current proven epoch is %d",
			targetEpoch,
			currentEpoch,
		)

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
