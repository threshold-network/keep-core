package tbtcpg

import (
	"errors"
	"fmt"
	"math"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ErrMaxFeeTooLow indicates that the Bridge maximum total fee is too low to
// build a wallet transaction at the safe minimum fee rate, so a non-underpriced
// transaction cannot be constructed. It mirrors the raw-estimate-too-high
// sentinels (ErrFeeTooHigh, ErrSweepTxFeeTooHigh): both signal an unserviceable
// fee configuration, one bounding from above and one from below.
var ErrMaxFeeTooLow = errors.New(
	"minimum safe transaction fee exceeds the maximum fee",
)

// maxWalletTxVsize and maxWalletTxEstimatedFee are sanity bounds on the
// applyWalletTxFeeFloor inputs. They are intentionally far above any
// realistic Bitcoin transaction (block weight caps vsize at ~4M weight
// units; a wallet tx fee over a few BTC is itself implausible) so
// legitimate callers never trip them. They are also defense-in-depth for
// the checked-arithmetic overflow guards below: a value within these
// bounds is guaranteed (modulo the explicit checks) to keep the internal
// int64 multiplications in range.
const (
	maxWalletTxVsize        int64 = 10_000_000    // 10M vbytes; ~2x Bitcoin block weight.
	maxWalletTxEstimatedFee int64 = 1_000_000_000 // 1e9 satoshis = 10 BTC.
)

// applyWalletTxFeeFloor raises a raw oracle fee estimate to a safe value for a
// non-RBF wallet transaction. It:
//   - applies a safety buffer (default 25%, controlled by
//     tbtc.WalletTxFeeBufferPercent) over the per-vByte fee rate so
//     there is margin during the estimate-to-broadcast delay and the
//     fee stays adaptive under congestion,
//   - enforces a floor of tbtc.MinWalletTxSatPerVByteFee sat/vByte, and
//   - bounds the result by maxTotalFee (the Bridge maximum total fee for
//     the transaction).
//
// It returns ErrMaxFeeTooLow if the minimum floor alone would exceed
// maxTotalFee - a safe transaction cannot be built, so the caller must
// not broadcast an underpriced one. estimatedFee is the raw oracle fee
// in satoshis and txVsize is the estimated transaction virtual size in
// vBytes. Both inputs are sanity-bounded against maxWalletTxEstimatedFee
// / maxWalletTxVsize to prevent int64 overflow in the internal
// multiplications when the oracle or size-estimator returns an
// implausible value; an input outside the bound is rejected with an
// error rather than silently overflowing. The buffer multiplication
// and the final totalFee multiplication additionally have
// checked-arithmetic overflow guards so an operator-tuned
// tbtc.WalletTxFeeBufferPercent / tbtc.MinWalletTxSatPerVByteFee
// cannot bypass the bound by exceeding the int64 limit on its own.
//
// The policy values (the floor and the buffer percentage) live in
// pkg/tbtc as exported vars so the leader-side floor application
// (here) and the follower-side soft check
// (pkg/tbtc.warnIfProposedWalletTxFeeBelowBufferedFloor, used by every
// wallet-tx proposal validator) consume a single source of truth.
// Operator tuning one side automatically tunes the other.
//
// The buffer is applied to the truncated per-vByte rate
// (estimatedFee / txVsize). This is lossless only because EstimateFee
// returns the fee as satPerVByteFee * txVsize (an exact multiple of the
// vsize), so the integer division recovers the exact rate. If that
// contract ever changes so estimatedFee is no longer an exact multiple
// of txVsize, apply the buffer to estimatedFee directly instead of to
// the truncated rate; otherwise up to txVsize-1 sat is silently
// dropped before buffering and the tx is underpriced.
//
// maxTotalFee bounds only the total transaction fee. Where the Bridge
// also enforces a per-request cap (e.g. the redemption TxMaxFee),
// satisfying that cap is the caller's or on-chain validation's
// responsibility; this helper is unaware of it. Callers are expected to
// reject a raw estimate already above maxTotalFee before calling (all
// current callers do); the result is in any case clamped down to
// maxTotalFee.
//
// The buffer and floor are applied to the estimated vsize; a
// transaction whose real on-wire vsize is larger than estimated (e.g.
// a deposit sweep containing legacy P2SH inputs) can land slightly
// below the intended rate, but still far above the relay floor this
// guards against.
func applyWalletTxFeeFloor(
	estimatedFee int64,
	txVsize int64,
	maxTotalFee uint64,
) (int64, error) {
	if txVsize <= 0 {
		return 0, fmt.Errorf("invalid transaction virtual size [%d]", txVsize)
	}
	if txVsize > maxWalletTxVsize {
		return 0, fmt.Errorf(
			"implausible transaction virtual size [%d]; expected at most [%d]",
			txVsize, maxWalletTxVsize,
		)
	}
	if estimatedFee < 0 {
		return 0, fmt.Errorf("invalid estimated fee [%d]", estimatedFee)
	}
	if estimatedFee > maxWalletTxEstimatedFee {
		return 0, fmt.Errorf(
			"implausible estimated fee [%d]; expected at most [%d]",
			estimatedFee, maxWalletTxEstimatedFee,
		)
	}
	if tbtc.MinWalletTxSatPerVByteFee <= 0 {
		return 0, fmt.Errorf(
			"implausible minimum fee rate [%d]; expected positive",
			tbtc.MinWalletTxSatPerVByteFee,
		)
	}
	if tbtc.WalletTxFeeBufferPercent < 0 {
		return 0, fmt.Errorf(
			"invalid wallet tx fee buffer percent [%d]; must be non-negative",
			tbtc.WalletTxFeeBufferPercent,
		)
	}
	bufferNumerator := 100 + tbtc.WalletTxFeeBufferPercent
	const bufferDenominator = 100

	// Checked-arithmetic guard: floor * txVsize must fit in int64 to
	// display correctly in the error message below and to keep the int64
	// product in range. Both operands are positive int64.
	if tbtc.MinWalletTxSatPerVByteFee > math.MaxInt64/txVsize {
		return 0, fmt.Errorf(
			"implausible minimum fee rate [%d] for vsize [%d]; "+
				"product would overflow",
			tbtc.MinWalletTxSatPerVByteFee, txVsize,
		)
	}
	floorProduct := tbtc.MinWalletTxSatPerVByteFee * txVsize
	if uint64(floorProduct) > maxTotalFee {
		return 0, fmt.Errorf(
			"%w: minimum fee [%d], maximum fee [%d]",
			ErrMaxFeeTooLow,
			floorProduct,
			maxTotalFee,
		)
	}

	rate := estimatedFee / txVsize
	// Checked-arithmetic guard for the buffer multiplication. Inputs
	// are also bounded (maxWalletTxVsize / maxWalletTxEstimatedFee), so
	// this is defense in depth: even if an operator tunes
	// tbtc.WalletTxFeeBufferPercent to a huge value, we reject rate
	// values whose product with bufferNumerator (plus
	// bufferDenominator-1 for the ceiling) cannot fit in int64. rate ==
	// 0 never overflows.
	if rate > 0 {
		maxRateForBuffer := (math.MaxInt64 - (bufferDenominator - 1)) /
			bufferNumerator
		if rate > maxRateForBuffer {
			return 0, fmt.Errorf(
				"implausible per-vByte rate [%d] would overflow when "+
					"applied with buffer percent [%d]; expected at most [%d]",
				rate,
				tbtc.WalletTxFeeBufferPercent,
				maxRateForBuffer,
			)
		}
	}
	// ceil(rate * (100+Percent) / 100). Both rate and bufferNumerator
	// are positive (or rate is zero), so the multiplication cannot
	// overflow; see the input-bounds check and the rate-vs-Numerator
	// check above.
	rate = (rate*bufferNumerator + bufferDenominator - 1) / bufferDenominator
	if rate < tbtc.MinWalletTxSatPerVByteFee {
		rate = tbtc.MinWalletTxSatPerVByteFee
	}

	// Checked-arithmetic guard for the total-fee multiplication: rate *
	// txVsize must fit in int64. rate == 0 never overflows. txVsize is
	// bounded above by maxWalletTxVsize, so this is defense in depth: an
	// operator-tuned tbtc.MinWalletTxSatPerVByteFee (e.g. set to
	// MaxInt64) would otherwise push rate past MaxInt64 / txVsize.
	if rate > 0 && rate > math.MaxInt64/txVsize {
		return 0, fmt.Errorf(
			"implausible buffered rate [%d] would overflow when multiplied "+
				"by txVsize [%d]; expected at most [%d]",
			rate, txVsize, math.MaxInt64/txVsize,
		)
	}

	// Clamp down to the Bridge maximum total fee. This can never drop
	// the fee below the floor: the floor-vs-cap guard above already
	// guaranteed maxTotalFee is at least the minimum floor total. The
	// product is now guaranteed to fit in int64 by the rate*txVsize
	// guard above.
	totalFee := rate * txVsize
	if uint64(totalFee) > maxTotalFee {
		totalFee = int64(maxTotalFee)
	}

	return totalFee, nil
}

// estimateReservationFixedSizeTxFee estimates the fee for a reservation
// transaction with a fixed virtual size. It mirrors the fee estimation
// logic in acceptance and re-anchor tasks, including fee flooring and
// max-fee clamping. exceedsMaxErrMsg is the caller-specific message used
// when the raw estimate already exceeds txMaxFee (acceptance and re-anchor
// use distinct action-named messages here, matching their pre-existing
// fixture expectations).
func estimateReservationFixedSizeTxFee(
	btcChain bitcoin.Chain,
	sizeEstimator *bitcoin.TransactionSizeEstimator,
	txMaxFee uint64,
	exceedsMaxErrMsg string,
) (int64, error) {
	transactionSize, err := sizeEstimator.VirtualSize()
	if err != nil {
		return 0, fmt.Errorf(
			"cannot estimate transaction virtual size: [%v]",
			err,
		)
	}

	feeEstimator := bitcoin.NewTransactionFeeEstimator(btcChain)
	totalFee, err := feeEstimator.EstimateFee(transactionSize)
	if err != nil {
		return 0, fmt.Errorf("cannot estimate transaction fee: [%v]", err)
	}

	if uint64(totalFee) > txMaxFee {
		return 0, fmt.Errorf("%s", exceedsMaxErrMsg)
	}

	totalFee, err = applyWalletTxFeeFloor(totalFee, transactionSize, txMaxFee)
	if err != nil {
		return 0, err
	}

	return totalFee, nil
}
