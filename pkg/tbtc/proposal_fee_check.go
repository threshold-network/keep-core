package tbtc

import (
	"math/big"

	"github.com/ipfs/go-log/v2"
)

// warnIfProposedWalletTxFeeBelowBufferedFloor is the follower-side soft
// (log-only, never rejects) check used by every wallet-tx proposal
// validator in this package: deposit sweep, redemption, and moving
// funds. It compares the leader's proposed total fee against the safe
// buffered minimum that tbtcpg.applyWalletTxFeeFloor would enforce for
// the same bare floor and vsize, so patched followers warn at the same
// threshold the leader was supposed to produce. The buffer is reapplied
// here because a leader proposing exactly-at-floor would otherwise slip
// past a bare-floor check and undersell the tx.
//
// The floor and the buffer ratio are read from the canonical package
// vars MinWalletTxSatPerVByteFee / WalletTxFeeBufferNumerator /
// WalletTxFeeBufferDenominator (declared in tbtc.go), which Initialize
// populates from Config. tbtcpg.applyWalletTxFeeFloor reads the same
// vars, so a single source of truth is enforced - tuning the policy
// from the operator side automatically tunes both the leader-side
// floor application and the follower-side soft check.
//
// The threshold is computed with arbitrary-precision big.Int arithmetic
// (the proposed fee is already *big.Int, so this avoids both an int64
// overflow on the buffered-rate product and a lossy conversion of
// minBufferedFee back into int64). The leader-side floor helper applies
// the same buffer formula with checked-arithmetic guards and returns
// ErrMaxFeeTooLow on implausible inputs; on such inputs the follower
// just sees a buffered fee above any reasonable proposed total and the
// check stays quiet, which is the same observable behavior as a leader
// that refused to broadcast.
//
// This is intentionally log-only, not a rejection: rejecting a
// below-floor proposal here would, during a mixed-version rollout,
// split signers (patched nodes reject, unpatched nodes sign) and could
// stall signing. Hard enforcement belongs on-chain in the
// WalletProposalValidator, or behind a coordinated all-nodes upgrade;
// see threshold-network/keep-core#4171.
//
// satPerVByteFloor is the bare minimum per-vByte fee rate (sat/vByte);
// pass MinWalletTxSatPerVByteFee for sweep/redemption/moving-funds
// validators.
// txVsize is the estimated transaction virtual size in vBytes, as
// returned by the caller-specific bitcoin.TransactionSizeEstimator.
// proposedFee is the leader's proposed total fee in satoshis; nil is
// treated as "no fee set" (defense-in-depth for test/mock chains;
// unreachable on the real production path where on-chain validation has
// already ABI-packed the fee and panicked on nil).
// actionLabel identifies the proposal type in the log message
// (e.g. "deposit sweep", "redemption", "moving funds").
func warnIfProposedWalletTxFeeBelowBufferedFloor(
	logger log.StandardLogger,
	satPerVByteFloor int64,
	txVsize int64,
	proposedFee *big.Int,
	actionLabel string,
) {
	// Silently skip on degenerate inputs; the caller has already surfaced
	// the underlying estimation error (size estimator failure, nil fee
	// that panicked in on-chain validation, etc.). This helper never
	// escalates failures; it only adds a warning when the inputs are
	// usable.
	if satPerVByteFloor <= 0 || txVsize <= 0 {
		return
	}
	if WalletTxFeeBufferNumerator <= 0 || WalletTxFeeBufferDenominator <= 0 {
		return
	}

	// Compute the buffered threshold with arbitrary-precision arithmetic
	// so an operator-tuned policy (large satPerVByteFloor or buffer
	// ratio) cannot overflow int64 in this helper. The leader-side
	// tbtcpg.applyWalletTxFeeFloor applies the same buffer formula but
	// with checked-arithmetic guards and returns an error on the same
	// implausible inputs; here the threshold simply ends up large enough
	// that no realistic proposed fee trips the warning.
	satPerVByte := big.NewInt(satPerVByteFloor)
	numerator := big.NewInt(WalletTxFeeBufferNumerator)
	denominator := big.NewInt(WalletTxFeeBufferDenominator)
	delta := new(big.Int).Sub(denominator, big.NewInt(1))

	// bufferedRate = ceil(satPerVByteFloor * Numerator / Denominator).
	bufferedRate := new(big.Int).Mul(satPerVByte, numerator)
	bufferedRate.Add(bufferedRate, delta)
	bufferedRate.Quo(bufferedRate, denominator)

	// minBufferedFee = bufferedRate * txVsize.
	minBufferedFee := new(big.Int).Mul(bufferedRate, big.NewInt(txVsize))

	switch {
	// This branch is defense-in-depth for test/mock chain implementations
	// and is not expected to be reachable on the real production path:
	// by the time control reaches the validator, on-chain
	// WalletProposalValidator has already ABI-packed the fee and panics
	// on a nil *big.Int before this code ever runs. Likewise, a proposal
	// decoded off the wire (Unmarshal in marshaling.go) always
	// constructs the fee via new(big.Int).SetBytes(...), which never
	// yields nil.
	case proposedFee == nil:
		logger.Warnf(
			"%s proposal has no tx fee set; expected at least the safe "+
				"buffered minimum [%v] ([%v] buffered sat/vByte = [%d] "+
				"bare floor * [%d]/[%d] buffer * [%d] vByte)",
			actionLabel,
			minBufferedFee,
			bufferedRate,
			satPerVByteFloor,
			WalletTxFeeBufferNumerator, WalletTxFeeBufferDenominator,
			txVsize,
		)
	case proposedFee.Cmp(minBufferedFee) < 0:
		logger.Warnf(
			"proposed %s tx fee [%v] is below the safe buffered minimum "+
				"[%v] ([%v] buffered sat/vByte = [%d] bare floor * [%d]/[%d] "+
				"buffer * [%d] vByte); the leader may be underpricing the "+
				"tx, which risks it getting stuck in the mempool and "+
				"jamming the wallet",
			actionLabel,
			proposedFee,
			minBufferedFee,
			bufferedRate,
			satPerVByteFloor,
			WalletTxFeeBufferNumerator, WalletTxFeeBufferDenominator,
			txVsize,
		)
	}
}
