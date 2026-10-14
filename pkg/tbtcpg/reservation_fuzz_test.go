package tbtcpg

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationFuzzSeedsSanity drives each reservation fee-decision fuzz
// target once with valid examples taken from the existing fee tests,
// proving the decision paths under test are reachable and consistent with
// the behavior those tests encode.
func TestReservationFuzzSeedsSanity(t *testing.T) {
	floorSeed, err := applyWalletTxFeeFloor(5000, 125, 100000)
	if err != nil {
		t.Fatal(err)
	}
	if floorSeed <= 0 || floorSeed > 100000 {
		t.Fatalf("unexpected seed fee [%d]", floorSeed)
	}

	btcChain := NewLocalBitcoinChain()
	btcChain.SetEstimateSatPerVByteFee(1, 1)
	acceptanceFee, err := estimateReservationAcceptanceFee(
		btcChain,
		100000,
	)
	if err != nil {
		t.Fatal(err)
	}
	if acceptanceFee <= 0 || acceptanceFee > 100000 {
		t.Fatalf("unexpected acceptance fee [%d]", acceptanceFee)
	}
}

// FuzzApplyWalletTxFeeFloor drives the reservation fee floor decision
// with arbitrary oracle estimates, transaction sizes, and caps.
//
// The data layout is a fixed 24-byte packing: estimated fee (8, big-
// endian int64), transaction virtual size (8, big-endian int64), maximum
// total fee (8, big-endian uint64).
//
// Invariants, justified from applyWalletTxFeeFloor in fee.go:
//   - never panics: every internal int64 multiplication is behind an
//     explicit checked-arithmetic guard, and out-of-domain inputs are
//     rejected with errors instead of overflowing;
//   - an error result always carries a zero fee: every error path in the
//     function returns 0 (the raw-estimate-too-high, sanity-bound,
//     overflow-guard, and floor-above-cap branches all do);
//   - a success result never exceeds the cap (the clamp to maxTotalFee)
//     and never drops below the floor total MinWalletTxSatPerVByteFee
//     times the vsize: the floor-vs-cap guard proves the cap dominates
//     the floor total on every success, and the buffered rate is always
//     at least the floor rate, so the clamped product stays above it.
//     (MinWalletTxSatPerVByteFee is 5 sat/vByte at its default.)
func FuzzApplyWalletTxFeeFloor(f *testing.F) {
	pack := func(estimatedFee int64, txVsize int64, maxTotalFee uint64) []byte {
		data := make([]byte, 24)
		binary.BigEndian.PutUint64(data[0:8], uint64(estimatedFee))
		binary.BigEndian.PutUint64(data[8:16], uint64(txVsize))
		binary.BigEndian.PutUint64(data[16:24], maxTotalFee)
		return data
	}

	// Seeds: a buffered-estimate success, a floor-raised success, and a
	// floor-above-cap error.
	f.Add(pack(5000, 125, 100000))
	f.Add(pack(1, 125, 100000))
	f.Add(pack(1, 125, 12))

	f.Fuzz(
		func(t *testing.T, data []byte) {
			if len(data) < 24 {
				return
			}
			estimatedFee := int64(binary.BigEndian.Uint64(data[0:8]))
			txVsize := int64(binary.BigEndian.Uint64(data[8:16]))
			maxTotalFee := binary.BigEndian.Uint64(data[16:24])

			fee, err := applyWalletTxFeeFloor(
				estimatedFee,
				txVsize,
				maxTotalFee,
			)
			if err != nil {
				if fee != 0 {
					t.Fatalf(
						"error result carries non-zero fee [%d]: %v",
						fee,
						err,
					)
				}
				return
			}

			if fee <= 0 {
				t.Fatalf("success result carries a non-positive fee [%d]", fee)
			}
			if uint64(fee) > maxTotalFee {
				t.Fatalf(
					"fee [%d] exceeds the maximum total fee [%d]",
					fee,
					maxTotalFee,
				)
			}
			// The floor total is only computable without overflow in
			// the domain the guard lets through (vsize is bounded by
			// maxWalletTxVsize on success, so this product cannot
			// overflow).
			floorTotal := tbtc.MinWalletTxSatPerVByteFee * txVsize
			if fee < floorTotal {
				t.Fatalf(
					"fee [%d] dropped below the floor total [%d] for "+
						"vsize [%d]",
					fee,
					floorTotal,
					txVsize,
				)
			}
		},
	)
}

// FuzzEstimateReservationAcceptanceFee drives the acceptance fee
// estimation with an arbitrary oracle fee rate and snapshotted cap.
//
// The data layout is a fixed 16-byte packing: the oracle sat/vByte fee
// rate (8, big-endian int64) and the generation's snapshotted maximum
// fee (8, big-endian uint64). The rate is installed on a fresh
// LocalBitcoinChain per call, so the target is safe to run
// concurrently.
//
// Invariants, justified from estimateReservationFixedSizeTxFee /
// estimateReservationAcceptanceFee:
//   - never panics: the rate-to-fee multiplication wraps for
//     implausible rates instead of overflowing, and the wrapped or
//     negative result is rejected by the "estimated fee is less than
//     or equal zero" guard before any further arithmetic;
//   - an error result always carries a zero fee;
//   - a success result is positive and never exceeds the snapshotted
//     cap: the raw estimate is rejected above the cap before the floor
//     runs, and the floor clamps the buffered result back down to the
//     cap.
func FuzzEstimateReservationAcceptanceFee(f *testing.F) {
	pack := func(satPerVByte int64, txMaxFee uint64) []byte {
		data := make([]byte, 16)
		binary.BigEndian.PutUint64(data[0:8], uint64(satPerVByte))
		binary.BigEndian.PutUint64(data[8:16], txMaxFee)
		return data
	}

	// Seeds: a comfortable estimate under the cap, a zero estimate (the
	// raw-estimate error), and a huge estimate that overflows the raw
	// multiplication.
	f.Add(pack(1, 100000))
	f.Add(pack(0, 100000))
	f.Add(pack(1<<63-1, 100000))

	f.Fuzz(
		func(t *testing.T, data []byte) {
			if len(data) < 16 {
				return
			}
			satPerVByte := int64(binary.BigEndian.Uint64(data[0:8]))
			txMaxFee := binary.BigEndian.Uint64(data[8:16])

			btcChain := NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, satPerVByte)

			fee, err := estimateReservationAcceptanceFee(
				btcChain,
				txMaxFee,
			)
			if err != nil {
				if fee != 0 {
					t.Fatalf(
						"error result carries non-zero fee [%d]: %v",
						fee,
						err,
					)
				}
				return
			}

			if fee <= 0 {
				t.Fatalf("success result carries a non-positive fee [%d]", fee)
			}
			if uint64(fee) > txMaxFee {
				t.Fatalf(
					"fee [%d] exceeds the snapshotted cap [%d]",
					fee,
					txMaxFee,
				)
			}
		},
	)
}

// FuzzEstimateReservationReanchorFee is the re-anchor analog of
// FuzzEstimateReservationAcceptanceFee: same invariants, with the
// re-anchor transaction's own fixed virtual size (one P2WPKH input and
// one P2WPKH output).
func FuzzEstimateReservationReanchorFee(f *testing.F) {
	pack := func(satPerVByte int64, txMaxFee uint64) []byte {
		data := make([]byte, 16)
		binary.BigEndian.PutUint64(data[0:8], uint64(satPerVByte))
		binary.BigEndian.PutUint64(data[8:16], txMaxFee)
		return data
	}

	// Seeds: a comfortable estimate under the cap and a zero estimate.
	f.Add(pack(1, 100000))
	f.Add(pack(0, 100000))

	f.Fuzz(
		func(t *testing.T, data []byte) {
			if len(data) < 16 {
				return
			}
			satPerVByte := int64(binary.BigEndian.Uint64(data[0:8]))
			txMaxFee := binary.BigEndian.Uint64(data[8:16])

			btcChain := NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, satPerVByte)

			fee, err := estimateReservationReanchorFee(
				btcChain,
				txMaxFee,
			)
			if err != nil {
				if fee != 0 {
					t.Fatalf(
						"error result carries non-zero fee [%d]: %v",
						fee,
						err,
					)
				}
				return
			}

			if fee <= 0 {
				t.Fatalf("success result carries a non-positive fee [%d]", fee)
			}
			if uint64(fee) > txMaxFee {
				t.Fatalf(
					"fee [%d] exceeds the snapshotted cap [%d]",
					fee,
					txMaxFee,
				)
			}
		},
	)
}

// FuzzReservationReanchorTargetHeadroom drives the reservation re-anchor
// target's count and amount headroom check with arbitrary on-chain
// capacity values: the number of reservations a candidate wallet
// custodies, its reserved amount, the anchor value being re-anchored,
// and the two governance caps.
//
// The data layout is a fixed 44-byte packing: reservation count (4,
// big-endian uint32), reserved amount (8, big-endian uint64), anchor
// value (8, big-endian uint64), per-wallet reservation cap (4, big-
// endian uint32), per-wallet amount cap (8, big-endian uint64), and 12
// padding bytes.
//
// Invariants, justified from reanchorTargetSearch.pick:
//   - never panics on any input;
//   - in the overflow-free domain (count below 2^24, and every amount
//     value and cap below 2^52 so no sum or comparison wraps), the
//     candidate is picked if and only if it has headroom: the count cap
//     allows one more reservation (count + 1 <= cap) and, when the
//     amount cap is enabled (non-zero), the reserved amount plus the
//     anchor value fits under it. A zero amount cap disables the amount
//     check, mirroring the on-chain convention. The age gate is always
//     passed: the candidate's CreatedAt is the zero time and the search
//     head time is the host clock, so IsReanchorTargetAged reports the
//     target as aged out.
func FuzzReservationReanchorTargetHeadroom(f *testing.F) {
	const maxFuzzedAmount = uint64(1) << 52 // overflow-free domain
	const maxFuzzedCount = uint32(1 << 24)  // overflow-free domain

	pack := func(
		count uint32,
		amount uint64,
		anchorValue uint64,
		maxPerWallet uint32,
		maxAmount uint64,
	) []byte {
		data := make([]byte, 44)
		binary.BigEndian.PutUint32(data[0:4], count)
		binary.BigEndian.PutUint64(data[4:12], amount)
		binary.BigEndian.PutUint64(data[12:20], anchorValue)
		binary.BigEndian.PutUint32(data[20:24], maxPerWallet)
		binary.BigEndian.PutUint64(data[24:32], maxAmount)
		return data
	}

	// Seeds: a wallet with headroom under both caps, and a wallet at
	// its count cap.
	f.Add(pack(0, 0, 100000, 5, 200000))
	f.Add(pack(5, 0, 100000, 5, 200000))

	f.Fuzz(
		func(t *testing.T, data []byte) {
			if len(data) < 44 {
				return
			}
			count := binary.BigEndian.Uint32(data[0:4])
			amount := binary.BigEndian.Uint64(data[4:12])
			anchorValue := binary.BigEndian.Uint64(data[12:20])
			maxPerWallet := binary.BigEndian.Uint32(data[20:24])
			maxAmount := binary.BigEndian.Uint64(data[24:32])

			// Constrain to the overflow-free domain: the headroom
			// invariants below are stated for sums that cannot
			// wrap. Wilder values still must not panic.
			found := runPickOnce(
				t,
				count,
				amount,
				anchorValue,
				maxPerWallet,
				maxAmount,
			)
			if count > maxFuzzedCount ||
				amount > maxFuzzedAmount ||
				anchorValue > maxFuzzedAmount ||
				maxAmount > maxFuzzedAmount {
				return
			}

			countHeadroom := uint64(count)+1 <= uint64(maxPerWallet)
			amountHeadroom := maxAmount == 0 ||
				amount+anchorValue <= maxAmount
			expectedPick := countHeadroom && amountHeadroom

			if found != expectedPick {
				t.Fatalf(
					"pick found=[%v] but headroom rules expect "+
						"[%v]: count [%d] cap [%d], amount [%d] "+
						"anchor [%d] cap [%d]",
					found,
					expectedPick,
					count,
					maxPerWallet,
					amount,
					anchorValue,
					maxAmount,
				)
			}
		},
	)
}

// runPickOnce wires one candidate wallet with the given capacity state
// into a fresh target search and reports whether pick selects it.
func runPickOnce(
	t *testing.T,
	count uint32,
	amount uint64,
	anchorValue uint64,
	maxPerWallet uint32,
	maxAmount uint64,
) bool {
	t.Helper()

	target := [20]byte{0xfb, 0x2a}
	source := [20]byte{0x01}

	lc := NewLocalChain()
	lc.reanchorReservedCapacity[target] = reservationCapacity{
		count:  count,
		amount: amount,
	}
	lc.SetWallet(target, &tbtc.WalletChainData{State: tbtc.StateLive})

	btcChain := NewLocalBitcoinChain()
	task := NewReservationReanchorTask(lc, btcChain)

	search := task.newReanchorTargetSearch(
		logger,
		source,
		maxPerWallet,
		maxAmount,
		time.Now(),
		0,
		false,
	)
	// Pre-seed the search with the single candidate, bypassing the
	// registration scan: pick evaluates the capacity and age gates
	// directly.
	search.candidates = append(search.candidates, target)
	search.createdAt[target] = time.Time{} // aged out of any creation period

	picked, found, err := search.pick(anchorValue, 0)
	if err != nil {
		t.Fatalf("unexpected pick error: [%v]", err)
	}
	if found {
		if picked != target {
			t.Fatalf("pick returned wallet [%x], want [%x]", picked, target)
		}
	}
	return found
}
