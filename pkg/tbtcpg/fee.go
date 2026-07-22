package tbtcpg

import "fmt"

// minWalletTxSatPerVByteFee is the minimum fee rate, in sat/vByte, applied to
// wallet Bitcoin transactions (deposit sweeps, redemptions, moving funds, moved
// funds sweeps). A fee oracle can return an unusably low estimate (down to the
// 1 sat/vByte relay floor enforced by the Electrum client) in an uncongested
// mempool. Because these transactions spend or consolidate significant wallet
// value and are not RBF-enabled, they cannot be replaced once broadcast, so a
// floor-rate transaction can get stuck in the mempool and jam the wallet: no
// new wallet transaction can be built while the previous one is unconfirmed.
// This minimum keeps the fee safely above the relay floor while remaining far
// below the Bridge's maximum fee. The value is intentionally conservative and
// could be made configurable; see threshold-network/keep-core#4171.
//
// NOTE: this static floor and the 25% buffer applied in applyWalletTxFeeFloor
// are a stopgap for the current fire-and-forget, non-RBF wallet transaction
// path: because a stuck transaction cannot be fee-bumped, the fee must be right
// on the first broadcast. Once RBF / fee-bumping lands (Part B, tracked in
// #4171) the safety net shifts to monitor-and-bump, and this policy should be
// revisited rather than carried forward unchanged: the defensive buffer can be
// dropped and the floor relaxed toward the live estimate, keeping only a small
// relay-propagation minimum.
const minWalletTxSatPerVByteFee = 5

// applyWalletTxFeeFloor raises a raw oracle fee estimate to a safe value for a
// non-RBF wallet transaction. It:
//   - adds a 25% buffer over the oracle estimate so there is margin during the
//     estimate-to-broadcast delay and the fee stays adaptive under congestion,
//   - enforces a floor of minWalletTxSatPerVByteFee sat/vByte, and
//   - bounds the result by maxTotalFee (the Bridge maximum total fee for the
//     transaction).
//
// It returns an error if the minimum floor alone would exceed maxTotalFee - a
// safe transaction cannot be built, so the caller must not broadcast an
// underpriced one. estimatedFee is the raw oracle fee and txVsize is the
// estimated transaction virtual size, both in the usual sat / vByte units.
//
// maxTotalFee bounds only the total transaction fee. Where the Bridge also
// enforces a per-request cap (e.g. the redemption TxMaxFee), satisfying that
// cap is the caller's or on-chain validation's responsibility; this helper is
// unaware of it. Callers are expected to reject a raw estimate already above
// maxTotalFee before calling (all current callers do); the result is in any
// case clamped down to maxTotalFee.
//
// The buffer and floor are applied to the estimated vsize; a transaction whose
// real on-wire vsize is larger than estimated (e.g. a deposit sweep containing
// legacy P2SH inputs) can land slightly below the intended rate, but still far
// above the relay floor this guards against.
func applyWalletTxFeeFloor(
	estimatedFee int64,
	txVsize int64,
	maxTotalFee uint64,
) (int64, error) {
	if txVsize <= 0 {
		return 0, fmt.Errorf("invalid transaction virtual size [%d]", txVsize)
	}

	// If even the minimum floor exceeds the Bridge maximum, a safe transaction
	// cannot be constructed; error rather than silently broadcast underpriced.
	if uint64(minWalletTxSatPerVByteFee*txVsize) > maxTotalFee {
		return 0, fmt.Errorf(
			"minimum safe transaction fee [%d] exceeds the maximum fee [%d]",
			minWalletTxSatPerVByteFee*txVsize,
			maxTotalFee,
		)
	}

	rate := estimatedFee / txVsize
	rate = (rate*5 + 3) / 4 // ceil(rate * 1.25)
	if rate < minWalletTxSatPerVByteFee {
		rate = minWalletTxSatPerVByteFee
	}

	// Clamp down to the Bridge maximum total fee. This can never drop the fee
	// below the floor: the floor-vs-cap guard above already guaranteed
	// maxTotalFee is at least the minimum floor total.
	totalFee := rate * txVsize
	if uint64(totalFee) > maxTotalFee {
		totalFee = int64(maxTotalFee)
	}

	return totalFee, nil
}
