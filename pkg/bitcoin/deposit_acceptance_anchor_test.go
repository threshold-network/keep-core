package bitcoin

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// us12AnchorFee is the fee the acceptance anchor pays, kept well below
// the funding amount.
const us12AnchorFee = 1_500

// us12AnchorHeight is the block height the acceptance anchor is mined
// at. It only matters for the UTXO-set checks, which use it as the
// height the spending transaction is evaluated at.
const us12AnchorHeight = int32(100)

// us12AnchorSpend builds the acceptance anchor transaction: a
// 1-input-1-output spend of the reserved deposit's funding output into
// a fresh output controlled by the accepting wallet.
//
// The anchor spends the deposit through the WALLET branch of the
// deposit's redeem script - it is the wallet that takes custody of the
// deposit - so the wallet's key signs the anchor and the wallet's
// public key hash is the one the script's wallet branch names.
//
// The anchor is the transaction the Bridge's
// `submitReservationAcceptanceProof` settles against: the contract
// checks that the anchor's single input spends the reserved deposit
// (see consumeAcceptedDeposit in ReservationProofs.sol) and that the
// anchor's output pays the authorized wallet.
func us12AnchorSpend(
	t *testing.T,
	redeemScript Script,
	lockingScript Script,
	walletKey *btcec.PrivateKey,
	fundingTxHash chainhash.Hash,
) *wire.MsgTx {
	t.Helper()

	tx := wire.NewMsgTx(1)

	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&fundingTxHash, 0), nil, nil))

	walletPKH := PublicKeyHash(walletKey.PubKey().ToECDSA())
	anchorOutputScript, err := PayToWitnessPublicKeyHash(walletPKH)
	if err != nil {
		t.Fatalf("cannot build the anchor's output script: [%v]", err)
	}
	tx.AddTxOut(
		wire.NewTxOut(
			us12FundingAmount-us12AnchorFee,
			anchorOutputScript,
		),
	)

	signature, err := txscript.RawTxInSignature(
		tx,
		0,
		redeemScript,
		txscript.SigHashAll,
		walletKey,
	)
	if err != nil {
		t.Fatalf("cannot sign the acceptance anchor: [%v]", err)
	}

	scriptSig, err := txscript.NewScriptBuilder().
		AddData(signature).
		AddData(walletKey.PubKey().SerializeCompressed()).
		AddData(redeemScript).
		Script()
	if err != nil {
		t.Fatalf("cannot build the anchor's scriptSig: [%v]", err)
	}
	tx.TxIn[0].SignatureScript = scriptSig

	return tx
}

// TestDepositAcceptanceAnchorSpend_RemovesTheFundingRefundPath is the
// third arm of the US-12 row: "a confirmed acceptance spend removes
// this funding refund path".
//
// The test builds the reserved deposit's funding output and TWO
// competing spends of it:
//
//   - the acceptance anchor, signed with the wallet key and spending
//     the wallet branch of the deposit's redeem script;
//   - the depositor's refund, signed with the refund key and spending
//     the refund branch.
//
// It then asserts:
//
//  1. the acceptance anchor validates against the deposit's script;
//  2. the refund spend validates against the deposit's script too, so
//     the two are genuinely competing spends of the SAME output - the
//     anchor does not "win" by being the only valid script path;
//  3. while the funding output is unspent, both spends pass btcd's
//     consensus input check;
//  4. once the anchor is confirmed - that is, once the funding output
//     is marked spent in the UTXO set - the refund spend is REJECTED by
//     the very same consensus check, with the double-spend error.
//
// The on-chain half of this clause - the contract's
// `consumeAcceptedDeposit`, which marks the deposit swept and
// decrements `pendingReservedDeposits` when the acceptance proof
// settles - is contract-side behaviour and is covered by the Solidity
// suite (Bridge.ReservationStranding.test.ts and
// ReservationProofs.test.ts). This test proves the Bitcoin-level fact
// the clause rests on: the acceptance anchor and the refund spend the
// same output, and a confirmed anchor makes the refund spend
// impossible.
func TestDepositAcceptanceAnchorSpend_RemovesTheFundingRefundPath(t *testing.T) {
	walletKey, walletPKH := us12Key(t, 0x01)
	refundKey, refundPKH := us12Key(t, 0x02)

	redeemScript := us12RedeemScript(
		t,
		walletPKH[:],
		refundPKH[:],
		us12Hex(t, us12RefundLocktimeHex),
	)

	lockingScript, err := PayToScriptHash(ScriptHash(redeemScript))
	if err != nil {
		t.Fatalf("cannot build the P2SH locking script: [%v]", err)
	}

	// The funding transaction holding the reserved deposit's output.
	fundingTx := wire.NewMsgTx(1)
	fundingTx.AddTxOut(wire.NewTxOut(us12FundingAmount, lockingScript))
	fundingTxHash := fundingTx.TxHash()

	// The acceptance anchor: the wallet branch's spend.
	anchorTx := us12AnchorSpend(
		t,
		redeemScript,
		lockingScript,
		walletKey,
		fundingTxHash,
	)

	if err := us12Execute(t, lockingScript, anchorTx); err != nil {
		t.Fatalf(
			"the acceptance anchor must validate against the deposit's script, got [%v]",
			err,
		)
	}

	// The depositor's refund: the refund branch's spend, of the very
	// same funding output. It is built independently of the anchor so
	// that the two spends are genuinely competing - neither is derived
	// from the other.
	refundTx := wire.NewMsgTx(1)
	refundTx.LockTime = uint32(1642773600)
	refundTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&fundingTxHash, 0), nil, nil))
	refundTx.TxIn[0].Sequence = 0xfffffffd
	refundTx.AddTxOut(wire.NewTxOut(us12FundingAmount-1_000, []byte{0x51}))

	refundSignature, err := txscript.RawTxInSignature(
		refundTx,
		0,
		redeemScript,
		txscript.SigHashAll,
		refundKey,
	)
	if err != nil {
		t.Fatalf("cannot sign the refund spend: [%v]", err)
	}

	refundScriptSig, err := txscript.NewScriptBuilder().
		AddData(refundSignature).
		AddData(refundKey.PubKey().SerializeCompressed()).
		AddData(redeemScript).
		Script()
	if err != nil {
		t.Fatalf("cannot build the refund spend's scriptSig: [%v]", err)
	}
	refundTx.TxIn[0].SignatureScript = refundScriptSig

	if err := us12Execute(t, lockingScript, refundTx); err != nil {
		t.Fatalf(
			"the refund spend must validate against the deposit's script, got [%v]",
			err,
		)
	}

	// The UTXO set: the funding output is present and unspent, so both
	// spends pass btcd's consensus input check.
	view := blockchain.NewUtxoViewpoint()
	view.AddTxOuts(btcutil.NewTx(fundingTx), us12AnchorHeight-1)

	outpoint := wire.OutPoint{Hash: fundingTxHash, Index: 0}
	entry := view.LookupEntry(outpoint)
	if entry == nil {
		t.Fatal("expected the funding output in the UTXO set")
	}
	if entry.IsSpent() {
		t.Fatal("expected the funding output to be unspent")
	}

	if _, err := blockchain.CheckTransactionInputs(
		btcutil.NewTx(anchorTx),
		us12AnchorHeight,
		view,
		&chaincfg.MainNetParams,
	); err != nil {
		t.Fatalf(
			"the acceptance anchor must pass the consensus input check while the funding output is unspent, got [%v]",
			err,
		)
	}

	if _, err := blockchain.CheckTransactionInputs(
		btcutil.NewTx(refundTx),
		us12AnchorHeight,
		view,
		&chaincfg.MainNetParams,
	); err != nil {
		t.Fatalf(
			"the refund spend must pass the consensus input check while the funding output is unspent, got [%v]",
			err,
		)
	}

	// The acceptance anchor confirms: the funding output is spent.
	entry.Spend()

	// The refund spend is now a double spend and the consensus input
	// check rejects it.
	_, err = blockchain.CheckTransactionInputs(
		btcutil.NewTx(refundTx),
		us12AnchorHeight,
		view,
		&chaincfg.MainNetParams,
	)
	if err == nil {
		t.Fatal(
			"expected the refund spend to be rejected once the acceptance " +
				"anchor confirmed, got no error",
		)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("already been spent")) {
		t.Fatalf(
			"expected the refund spend to be rejected as a double spend, got [%v]",
			err,
		)
	}

	// And the acceptance anchor's own input is now spent too - the
	// output cannot be spent twice under any branch.
	_, err = blockchain.CheckTransactionInputs(
		btcutil.NewTx(anchorTx),
		us12AnchorHeight,
		view,
		&chaincfg.MainNetParams,
	)
	if err == nil {
		t.Fatal(
			"expected the funding output to be unspendable after the " +
				"acceptance anchor confirmed, got no error",
		)
	}
}
