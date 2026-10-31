package spv

import (
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationFuzzSeedsSanity drives each SPV reservation fuzz target
// once with a valid example taken from the existing proof-loop tests,
// proving the code paths under test are reachable and consistent with the
// production behavior those tests encode.
func TestReservationFuzzSeedsSanity(t *testing.T) {
	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyHash := [20]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
	}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	// A valid 1-input-1-output anchor transaction: the sole input spends
	// the deposit UTXO, the sole output pays the custody wallet with a
	// positive fee under the generation's snapshotted cap.
	matchingTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: walletScript,
		}},
	}

	spvChain := newLocalChain()
	reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)
	spvChain.setDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{Amount: 150000},
	)
	acceptanceEvent := &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		WalletPublicKeyHash: walletPublicKeyHash,
		DepositAmount:       150000,
		TxMaxFee:            60000,
	}
	if !isMatchingReservationAcceptanceTransaction(
		spvChain,
		acceptanceEvent,
		matchingTx,
	) {
		t.Fatal("the valid seed transaction is expected to match")
	}

	reanchorEvent := &tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		SourceWalletPublicKeyHash: walletPublicKeyHash,
		TargetWalletPublicKeyHash: [20]byte{0xf8, 0x7e, 0xb7},
		TxMaxFee:                  60000,
	}
	anchorScript, err := bitcoin.PayToWitnessPublicKeyHash(
		[20]byte{0xf8, 0x7e, 0xb7},
	)
	if err != nil {
		t.Fatal(err)
	}
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTxHash,
			OutputIndex:     0,
		},
		Value: 200000,
	}
	reanchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           150000,
			PublicKeyScript: anchorScript,
		}},
	}
	if !isMatchingReservationReanchorTransaction(
		reanchorEvent,
		anchorUtxo,
		reanchorTx,
	) {
		t.Fatal("the valid seed transaction is expected to match")
	}

	_ = matchingTx.Serialize()
}

// FuzzReservationAcceptanceTransactionMatcher drives the SPV reservation
// acceptance matcher with arbitrary Bitcoin transaction bytes: the bytes
// come from an untrusted Electrum backend, are deserialized through
// bitcoin.Transaction.Deserialize (whose recover() turns the underlying
// decoder's crafted-input panics into errors), and the resulting
// transaction is handed to the matcher that decides which wallet
// transactions settle a reservation acceptance generation.
//
// Invariants:
//   - no panics on arbitrary bytes, through the deserializer and the whole
//     matcher (outpoint dereferences, deposit key construction, script
//     parsing, and the fee arithmetic int64(depositAmount) - output value);
//   - when the matcher accepts a transaction, it really is a
//     1-input-1-output transaction spending the event's deposit outpoint,
//     paying the event's wallet, and carrying a positive fee within the
//     generation's snapshotted cap - recomputed here with the matcher's
//     own arithmetic so the check cannot disagree with it.
func FuzzReservationAcceptanceTransactionMatcher(
	f *testing.F,
) {
	// Seed: the valid matching transaction from TestFindReservationAcceptanceTransaction.
	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		panic(err)
	}
	walletPublicKeyHash := [20]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
	}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		panic(err)
	}
	seedTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: walletScript,
		}},
	}
	f.Add(seedTx.Serialize())
	// Seed: a deserialization error path (truncated, malformed bytes).
	f.Add([]byte{0x02, 0x00, 0x00, 0x00, 0x01})

	f.Fuzz(
		func(t *testing.T, data []byte) {
			transaction := &bitcoin.Transaction{}
			if err := transaction.Deserialize(data); err != nil {
				return
			}

			// Build the chain per call: the fuzz driver runs targets
			// concurrently, so no shared mutable state. No deposit
			// request is installed, so the matcher always takes its
			// event.DepositAmount fee branch; the on-chain deposit
			// request branch stays covered by the unit tests.
			spvChain := newLocalChain()

			// Derive the event from the transaction's own outpoint
			// whenever it carries one, so the deposit-key equality the
			// matcher requires holds and the match path is reachable.
			event := &tbtc.ReservationAcceptanceRequestedEvent{
				ReservationKey: big.NewInt(424242),
				RequestNonce:   1,
				WalletPublicKeyHash: [20]byte{
					1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
					17, 18, 19, 20,
				},
				DepositAmount: 150000,
				TxMaxFee:      60000,
			}

			if len(transaction.Inputs) == 1 &&
				transaction.Inputs[0].Outpoint != nil {
				input := transaction.Inputs[0]
				event.ReservationKey = spvChain.BuildDepositKey(
					input.Outpoint.TransactionHash,
					input.Outpoint.OutputIndex,
				)
			}

			if !isMatchingReservationAcceptanceTransaction(
				spvChain,
				event,
				transaction,
			) {
				return
			}

			// A matched transaction must satisfy the matcher's own
			// rules, recomputed with the matcher's arithmetic: it
			// spends the event's deposit outpoint exactly once, pays
			// the event's wallet, and leaves a positive fee under the
			// generation's snapshotted cap.
			input := transaction.Inputs[0]
			depositKey := spvChain.BuildDepositKey(
				input.Outpoint.TransactionHash,
				input.Outpoint.OutputIndex,
			)
			if depositKey.Cmp(event.ReservationKey) != 0 {
				t.Fatalf(
					"matched transaction spends deposit key [%s], "+
						"event key is [%s]",
					depositKey,
					event.ReservationKey,
				)
			}
			publicKeyHash, err := bitcoin.ExtractPublicKeyHash(
				transaction.Outputs[0].PublicKeyScript,
			)
			if err != nil || publicKeyHash != event.WalletPublicKeyHash {
				t.Fatalf(
					"matched transaction output does not pay the event "+
						"wallet: err=[%v] hash=[%x]",
					err,
					publicKeyHash,
				)
			}
			fee := int64(event.DepositAmount) - transaction.Outputs[0].Value
			if fee <= 0 || (event.TxMaxFee > 0 && uint64(fee) > event.TxMaxFee) {
				t.Fatalf(
					"matched transaction carries a fee [%d] that "+
						"violates the acceptance rules (cap [%d])",
					fee,
					event.TxMaxFee,
				)
			}
		},
	)
}

// FuzzReservationReanchorTransactionMatcher drives the SPV reservation
// re-anchor matcher with arbitrary Bitcoin transaction bytes.
//
// Invariants:
//   - no panics on arbitrary bytes, through the deserializer and the whole
//     matcher (outpoint comparison, script parsing, and the fee arithmetic
//     int64(anchorValue) - output value, which wraps for negative or huge
//     output values instead of panicking);
//   - when the matcher accepts a transaction, it is a
//     1-input-1-output transaction spending exactly the anchor outpoint,
//     paying the event's target wallet, with a positive fee under the
//     generation's snapshotted cap - recomputed here with the matcher's own
//     arithmetic.
func FuzzReservationReanchorTransactionMatcher(
	f *testing.F,
) {
	// Seed: the valid matching re-anchor transaction.
	anchorWalletPublicKeyHash := [20]byte{0xf8, 0x7e, 0xb7}
	anchorScript, err := bitcoin.PayToWitnessPublicKeyHash(
		anchorWalletPublicKeyHash,
	)
	if err != nil {
		panic(err)
	}
	seedAnchorHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		panic(err)
	}
	seedTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: seedAnchorHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           150000,
			PublicKeyScript: anchorScript,
		}},
	}
	f.Add(seedTx.Serialize())
	// Seed: a deserialization error path (truncated, malformed bytes).
	f.Add([]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01})

	f.Fuzz(
		func(t *testing.T, data []byte) {
			transaction := &bitcoin.Transaction{}
			if err := transaction.Deserialize(data); err != nil {
				return
			}

			event := &tbtc.ReservationReanchorRequestedEvent{
				ReservationKey: big.NewInt(424242),
				RequestNonce:   1,
				SourceWalletPublicKeyHash: [20]byte{
					1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
					17, 18, 19, 20,
				},
				TargetWalletPublicKeyHash: [20]byte{0xf8, 0x7e, 0xb7},
				TxMaxFee:                  60000,
			}
			anchorUtxo := &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: seedAnchorHash,
					OutputIndex:     0,
				},
				Value: 200000,
			}

			if !isMatchingReservationReanchorTransaction(
				event,
				anchorUtxo,
				transaction,
			) {
				return
			}

			// A matched transaction must spend exactly the anchor
			// outpoint, pay the target wallet, and leave a positive
			// fee under the snapshotted cap, recomputed with the
			// matcher's own arithmetic (including its wrap-around for
			// values that overflow int64).
			input := transaction.Inputs[0]
			if input.Outpoint.TransactionHash != anchorUtxo.Outpoint.TransactionHash ||
				input.Outpoint.OutputIndex != anchorUtxo.Outpoint.OutputIndex {
				t.Fatal("matched transaction does not spend the anchor outpoint")
			}
			publicKeyHash, err := bitcoin.ExtractPublicKeyHash(
				transaction.Outputs[0].PublicKeyScript,
			)
			if err != nil || publicKeyHash != event.TargetWalletPublicKeyHash {
				t.Fatalf(
					"matched transaction output does not pay the target wallet: "+
						"err=[%v] hash=[%x]",
					err,
					publicKeyHash,
				)
			}
			fee := int64(anchorUtxo.Value) - transaction.Outputs[0].Value
			if fee <= 0 || (event.TxMaxFee > 0 && uint64(fee) > event.TxMaxFee) {
				t.Fatalf(
					"matched transaction carries a fee [%d] that "+
						"violates the re-anchor rules (cap [%d])",
					fee,
					event.TxMaxFee,
				)
			}
		},
	)
}

// FuzzReservationParamsInfeasibleReasons drives the reservation parameter
// infeasibility rules with arbitrary on-chain parameter values.
//
// The data layout is a fixed 40-byte packing of the inputs the rules
// compare: dust threshold (8), reveal-ahead period (4), reservation term
// (4), per-wallet cap (8), single-amount cap (8), max-total cap (8).
//
// Invariants:
//   - never panics on arbitrary packed inputs;
//   - the reason list has exactly one entry per numerically violated
//     documented rule - a non-zero reveal-ahead period that exceeds the
//     term plus the refund safety margin, and each non-zero amount cap
//     that sits below the dust threshold - recomputed here from the
//     rule definitions in the function's doc comment, so a drift between
//     the documented rules and the implementation is caught;
//   - monotonicity: with the dust threshold raised to MaxUint64, every
//     non-zero amount cap is below it, so the cap-based reasons equal
//     exactly the count of non-zero caps - an independently stated
//     property of the "cap below the dust threshold" rule that holds
//     regardless of the implementation's concrete comparison.
func FuzzReservationParamsInfeasibleReasons(f *testing.F) {
	// Seed: one value set that violates both rule families.
	seed := make([]byte, 40)
	binary.BigEndian.PutUint64(seed[0:8], 330)     // dust threshold
	binary.BigEndian.PutUint32(seed[8:12], 100000) // reveal-ahead
	binary.BigEndian.PutUint32(seed[12:16], 86400) // term
	binary.BigEndian.PutUint64(seed[16:24], 100)   // per-wallet cap
	binary.BigEndian.PutUint64(seed[24:32], 50)    // single-amount cap
	binary.BigEndian.PutUint64(seed[32:40], 0)     // max-total cap
	f.Add(seed)
	// Seed: a fully feasible set.
	feasibleSeed := make([]byte, 40)
	binary.BigEndian.PutUint64(feasibleSeed[0:8], 546)
	binary.BigEndian.PutUint32(feasibleSeed[8:12], 3600)
	binary.BigEndian.PutUint32(feasibleSeed[12:16], 7776000)
	f.Add(feasibleSeed)

	f.Fuzz(
		func(t *testing.T, data []byte) {
			if len(data) < 40 {
				return
			}
			dustThreshold := binary.BigEndian.Uint64(data[0:8])
			revealAhead := binary.BigEndian.Uint32(data[8:12])
			termSeconds := binary.BigEndian.Uint32(data[12:16])
			perWalletCap := binary.BigEndian.Uint64(data[16:24])
			singleAmountCap := binary.BigEndian.Uint64(data[24:32])
			maxTotalAmount := binary.BigEndian.Uint64(data[32:40])

			caps := []uint64{perWalletCap, singleAmountCap, maxTotalAmount}

			// Documented-rules cross-check: recompute the expected
			// count from the rule definitions.
			reasons := reservationParamsInfeasibleReasons(
				tbtc.DepositParameters{
					DustThreshold:     dustThreshold,
					RevealAheadPeriod: revealAhead,
				},
				termSeconds,
				maxTotalAmount,
				perWalletCap,
				singleAmountCap,
			)

			expected := 0
			if uint64(revealAhead) > 0 &&
				uint64(revealAhead) > uint64(termSeconds)+
					uint64(reservationDepositRefundSafetyMarginSeconds) {
				expected++
			}
			for _, cap := range caps {
				if cap > 0 && cap < dustThreshold {
					expected++
				}
			}
			if len(reasons) != expected {
				t.Fatalf(
					"expected [%d] infeasibility reason(s) under dust "+
						"threshold [%d], got [%d]: %v",
					expected,
					dustThreshold,
					len(reasons),
					reasons,
				)
			}

			// Monotonicity cross-check: at the maximum dust
			// threshold every cap that is positive and not itself
			// MaxUint64 is below it.
			maxDust := reservationParamsInfeasibleReasons(
				tbtc.DepositParameters{
					DustThreshold:     math.MaxUint64,
					RevealAheadPeriod: revealAhead,
				},
				termSeconds,
				maxTotalAmount,
				perWalletCap,
				singleAmountCap,
			)
			expectedMax := 0
			if uint64(revealAhead) > 0 &&
				uint64(revealAhead) > uint64(termSeconds)+
					uint64(reservationDepositRefundSafetyMarginSeconds) {
				expectedMax++
			}
			for _, cap := range caps {
				if cap > 0 && cap != math.MaxUint64 {
					expectedMax++
				}
			}
			if len(maxDust) != expectedMax {
				t.Fatalf(
					"expected [%d] reason(s) under MaxUint64 dust, "+
						"got [%d]: %v",
					expectedMax,
					len(maxDust),
					maxDust,
				)
			}
		},
	)
}
