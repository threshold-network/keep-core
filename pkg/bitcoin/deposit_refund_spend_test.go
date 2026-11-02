package bitcoin

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// The constants below pin the reserved deposit's funding output to one
// concrete reveal, so the script built here is byte-for-byte the script
// tbtc-v2's Deposit._revealDeposit assembles for that reveal. The
// values are the ones the Solidity contract sees as
// `msg.sender` (the depositor), `reveal.blindingFactor`,
// `reveal.walletPubKeyHash`, `reveal.refundPubKeyHash` and
// `reveal.refundLocktime`.
const (
	// us12DepositorHex is the depositor's EVM address, the 20 bytes
	// the script pushes right after its `hex"14"` length prefix.
	us12DepositorHex = "934b98637ca318a4d6e7ca6ffd1690b8e77df637"

	// us12BlindingFactorHex is the 8-byte blinding factor.
	us12BlindingFactorHex = "f9f0c90d00039523"

	// us12WalletPKHHex is the 20-byte public key hash of the wallet
	// the deposit was revealed for.
	us12WalletPKHHex = "8db50eb52063ea9d98b3eac91489a90f738986f6"

	// us12RefundPKHHex is the 20-byte public key hash of the
	// depositor's refund key.
	us12RefundPKHHex = "28e081f285138ccbe389c1eb8985716230129f89"

	// us12RefundLocktimeHex is the 4-byte little-endian refund
	// locktime. The Bridge derives the deposit's refund deadline from
	// these very bytes with BTCUtils.reverseUint32 (a little-endian
	// read), so the numeric deadline and the Bitcoin nLockTime the
	// refund spend must carry are the same number.
	us12RefundLocktimeHex = "60bcea61"

	// us12ExpectedRedeemScriptHex is the redeem script the Solidity
	// template produces for the constants above, written out by hand
	// from Deposit.sol's `expectedScript` (the `extraData == bytes32(0)`
	// branch):
	//
	//	hex"14" msg.sender            hex"75"
	//	hex"08" reveal.blindingFactor hex"75"
	//	hex"76" hex"a9" hex"14" reveal.walletPubKeyHash
	//	hex"87" hex"63" hex"ac" hex"67"
	//	hex"76" hex"a9" hex"14" reveal.refundPubKeyHash
	//	hex"88" hex"04" reveal.refundLocktime
	//	hex"b1" hex"75" hex"ac" hex"68"
	us12ExpectedRedeemScriptHex = "14934b98637ca318a4d6e7ca6ffd1690b8e77df63775" +
		"08f9f0c90d00039523" +
		"7576a9148db50eb52063ea9d98b3eac91489a90f738986f6" +
		"8763ac" +
		"6776a91428e081f285138ccbe389c1eb8985716230129f89" +
		"8804" + us12RefundLocktimeHex + "b175ac68"
)

// us12FundingAmount is the value of the funding output the refund spend
// consumes. It only matters for the script engine's input amount.
const us12FundingAmount = 100_000

// us12ScriptFlags are the script flags the refund spend is executed
// under: the BIP16 P2SH rules plus the standard consensus checks
// (DER and strict encoding, minimal data pushes, low-S signatures, null
// fail) and BIP65's OP_CHECKLOCKTIMEVERIFY, which is what gates the
// refund branch on the locktime.
const us12ScriptFlags = txscript.ScriptBip16 |
	txscript.ScriptVerifyDERSignatures |
	txscript.ScriptVerifyStrictEncoding |
	txscript.ScriptVerifyMinimalData |
	txscript.ScriptVerifyCleanStack |
	txscript.ScriptVerifyNullFail |
	txscript.ScriptVerifyCheckLockTimeVerify |
	txscript.ScriptVerifyLowS |
	txscript.ScriptVerifyMinimalIf |
	txscript.ScriptVerifyWitnessPubKeyType |
	txscript.ScriptVerifyConstScriptCode

// us12Hex decodes a hex string, failing the test on malformed input.
func us12Hex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("cannot decode [%v]: [%v]", value, err)
	}

	return decoded
}

// us12RedeemScript assembles the reserved deposit's funding output
// redeem script byte for byte as tbtc-v2's Deposit._revealDeposit does.
// Every piece is appended in the Solidity template's order, including the
// single-byte length prefixes (`hex"14"`, `hex"08"`, `hex"04"`), so the
// resulting script is the concatenation the contract compares the
// funding output against.
func us12RedeemScript(
	t *testing.T,
	walletPKH []byte,
	refundPKH []byte,
	refundLocktime []byte,
) Script {
	t.Helper()

	parts := [][]byte{
		// hex"14" <depositor> - the depositor's EVM address.
		{0x14},
		us12Hex(t, us12DepositorHex),
		// hex"75" - OP_DROP.
		{0x75},
		// hex"08" <blinding factor> - the 8-byte blinding factor.
		{0x08},
		us12Hex(t, us12BlindingFactorHex),
		// hex"75" - OP_DROP.
		{0x75},
		// hex"76" hex"a9" hex"14" <wallet pkh> hex"87" - the wallet
		// branch's key test.
		{0x76},
		{0xa9},
		{0x14},
		walletPKH,
		{0x87},
		// hex"63" hex"ac" - OP_IF and the wallet branch's OP_CHECKSIG.
		{0x63},
		{0xac},
		// hex"67" - OP_ELSE.
		{0x67},
		// hex"76" hex"a9" hex"14" <refund pkh> hex"88" - the refund
		// branch's key test.
		{0x76},
		{0xa9},
		{0x14},
		refundPKH,
		{0x88},
		// hex"04" <refund locktime> hex"b1" hex"75" hex"ac" - the
		// refund branch's locktime gate and its OP_CHECKSIG.
		{0x04},
		refundLocktime,
		{0xb1},
		{0x75},
		{0xac},
		// hex"68" - OP_ENDIF.
		{0x68},
	}

	var script []byte
	for _, part := range parts {
		script = append(script, part...)
	}

	return Script(script)
}

// us12Key returns a deterministic private key and the public key hash
// the script template compares against.
func us12Key(t *testing.T, seed byte) (*btcec.PrivateKey, [20]byte) {
	t.Helper()

	privateKey, publicKey := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{seed}, 32))

	return privateKey, PublicKeyHash(publicKey.ToECDSA())
}

// us12Spend builds the transaction that spends the funding output locked
// with the given redeem script. The spending key's public key and a
// signature over the whole redeem script (P2SH semantics: the redeem
// script is the sighash script code) are pushed on the stack, and the
// transaction's nLockTime is set to lockTime, which is what
// OP_CHECKLOCKTIMEVERIFY compares the script's locktime against.
//
// pushedKey, when non-nil, is the public key the scriptSig publishes
// while the signature is produced by signingKey. It lets a test state
// the "signed with the wrong key" cases explicitly: the scriptSig is
// free to publish any public key, and the branch's OP_CHECKSIG then
// decides whether the signature matches it.
func us12Spend(
	t *testing.T,
	redeemScript Script,
	signingKey *btcec.PrivateKey,
	pushedKey *btcec.PrivateKey,
	lockTime uint32,
) (*wire.MsgTx, Script) {
	t.Helper()

	lockingScript, err := PayToScriptHash(ScriptHash(redeemScript))
	if err != nil {
		t.Fatalf("cannot build the P2SH locking script: [%v]", err)
	}

	tx := wire.NewMsgTx(1)
	tx.LockTime = lockTime

	fundingTxHash := chainhash.Hash{0x01, 0x02, 0x03}
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&fundingTxHash, 0), nil, nil))
	// The spending input must not be finalized, otherwise
	// OP_CHECKLOCKTIMEVERIFY is disabled for it (see BIP65 and btcd's
	// opcodeCheckLockTimeVerify).
	tx.TxIn[0].Sequence = 0xfffffffd

	outputScript, err := txscript.NewScriptBuilder().
		AddData([]byte{0x51}).
		Script()
	if err != nil {
		t.Fatalf("cannot build the spending transaction's output script: [%v]", err)
	}
	tx.AddTxOut(wire.NewTxOut(us12FundingAmount-1_000, outputScript))

	signature, err := txscript.RawTxInSignature(
		tx,
		0,
		redeemScript,
		txscript.SigHashAll,
		signingKey,
	)
	if err != nil {
		t.Fatalf("cannot sign the refund spend: [%v]", err)
	}

	publishedKey := signingKey
	if pushedKey != nil {
		publishedKey = pushedKey
	}

	// P2SH scriptSig layout: the branch arguments first, then the
	// redeem script on top of the stack. The locking script pops the
	// redeem script (OP_HASH160 <script hash> OP_EQUAL) and the
	// remaining stack - the signature and the public key - is what the
	// redeem script's OP_CHECKSIG consumes.
	scriptSig, err := txscript.NewScriptBuilder().
		AddData(signature).
		AddData(publishedKey.PubKey().SerializeCompressed()).
		AddData(redeemScript).
		Script()
	if err != nil {
		t.Fatalf("cannot build the spending scriptSig: [%v]", err)
	}
	tx.TxIn[0].SignatureScript = scriptSig

	return tx, lockingScript
}

// us12Execute runs the spending transaction through btcd's script engine
// with the given locking script and returns the engine's error, if any.
func us12Execute(t *testing.T, lockingScript Script, tx *wire.MsgTx) error {
	t.Helper()

	previousOutputFetcher := txscript.NewCannedPrevOutputFetcher(
		lockingScript,
		us12FundingAmount,
	)

	engine, err := txscript.NewEngine(
		lockingScript,
		tx,
		0,
		us12ScriptFlags,
		txscript.NewSigCache(100),
		txscript.NewTxSigHashes(tx, previousOutputFetcher),
		us12FundingAmount,
		previousOutputFetcher,
	)
	if err != nil {
		t.Fatalf("cannot create the script engine: [%v]", err)
	}

	return engine.Execute()
}

// us12ErrorCode extracts btcd's script error code from an engine error,
// so a test can assert on the exact rule that rejected the spend.
func us12ErrorCode(t *testing.T, err error) txscript.ErrorCode {
	t.Helper()

	if err == nil {
		t.Fatal("expected the script engine to fail, got no error")
	}

	scriptError, ok := err.(txscript.Error)
	if !ok {
		t.Fatalf("expected a script error, got [%v]", err)
	}

	return scriptError.ErrorCode
}

// TestDepositRefundScript_MatchesSolidityRevealTemplate pins the
// reserved deposit's funding script against the Solidity template,
// byte for byte, and pins the P2SH locking script derived from it. The
// refund spend tests below are only meaningful if this script is the
// contract's script: the contract refuses a reveal whose funding output
// does not hash to it.
func TestDepositRefundScript_MatchesSolidityRevealTemplate(t *testing.T) {
	redeemScript := us12RedeemScript(
		t,
		us12Hex(t, us12WalletPKHHex),
		us12Hex(t, us12RefundPKHHex),
		us12Hex(t, us12RefundLocktimeHex),
	)

	expected := us12Hex(t, us12ExpectedRedeemScriptHex)

	if !bytes.Equal(redeemScript, expected) {
		t.Fatalf(
			"the redeem script does not match the Solidity template:\n"+
				"got      %x\n"+
				"expected %x",
			redeemScript,
			expected,
		)
	}

	// The template's own structure: the wallet branch's OP_EQUAL
	// (0x87) is followed by OP_IF (0x63) and the refund branch's
	// OP_EQUALVERIFY (0x88) by the 4-byte locktime push (0x04) and
	// OP_CHECKLOCKTIMEVERIFY (0xb1).
	for _, marker := range []struct {
		name  string
		bytes []byte
	}{
		{"wallet branch OP_EQUAL", []byte{0x87, 0x63, 0xac}},
		{"refund branch OP_EQUALVERIFY", []byte{0x88, 0x04}},
		{"OP_CHECKLOCKTIMEVERIFY", []byte{0xb1, 0x75, 0xac}},
	} {
		if !bytes.Contains(redeemScript, marker.bytes) {
			t.Fatalf(
				"the redeem script does not contain the %s opcode sequence %x",
				marker.name,
				marker.bytes,
			)
		}
	}

	scriptHash := ScriptHash(redeemScript)

	lockingScript, err := PayToScriptHash(scriptHash)
	if err != nil {
		t.Fatalf("cannot build the P2SH locking script: [%v]", err)
	}

	expectedLockingScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_HASH160).
		AddData(scriptHash[:]).
		AddOp(txscript.OP_EQUAL).
		Script()
	if err != nil {
		t.Fatalf("cannot build the expected P2SH locking script: [%v]", err)
	}

	if !bytes.Equal(lockingScript, expectedLockingScript) {
		t.Fatalf(
			"the locking script is not the P2SH of the redeem script:\n"+
				"got      %x\n"+
				"expected %x",
			lockingScript,
			expectedLockingScript,
		)
	}
}

// TestDepositRefundBranchSpend_ValidatesOnlyAfterRefundLocktime is the
// heart of the US-12 row: the reserved deposit's refund branch is really
// spent, and it is really gated on the refund locktime.
//
// The test executes the refund spend through btcd's consensus-accurate
// script engine and asserts every arm of the template:
//
//  1. the refund branch (signed with the depositor's refund key)
//     validates at and after the refund locktime;
//  2. it does NOT validate before the refund locktime, because
//     OP_CHECKLOCKTIMEVERIFY rejects the spend;
//  3. the refund branch is reachable only by the refund key: a key that
//     is neither the wallet key nor the refund key is rejected by the
//     refund branch's OP_EQUALVERIFY, and the wallet key's signature
//     does not satisfy the refund branch's OP_CHECKSIG.
//
// The script is built from the same template as the byte-for-byte test
// above, with the two public key hashes derived from the keys the test
// actually spends with, so the branch selection is the real one.
func TestDepositRefundBranchSpend_ValidatesOnlyAfterRefundLocktime(t *testing.T) {
	walletKey, _ := us12Key(t, 0x01)
	refundKey, refundPKH := us12Key(t, 0x02)
	foreignKey, _ := us12Key(t, 0x03)

	// The script names the reveal's wallet public key hash - the
	// constant the byte-for-byte template test above pins - and the
	// refund key hash of the key that actually performs the refund
	// spend. The two hashes therefore differ, so publishing the wallet
	// key or a foreign key takes the refund branch and is rejected
	// there, while publishing the refund key takes the refund branch
	// and is accepted once the locktime has passed.
	redeemScript := us12RedeemScript(
		t,
		us12Hex(t, us12WalletPKHHex),
		refundPKH[:],
		us12Hex(t, us12RefundLocktimeHex),
	)

	// The refund deadline the Bridge snapshots at reveal: the very
	// locktime bytes the script carries, read little-endian, exactly
	// as BTCUtils.reverseUint32 does.
	deadline := uint32(1642773600)

	t.Run("refund branch validates at the refund locktime", func(t *testing.T) {
		tx, lockingScript := us12Spend(t, redeemScript, refundKey, nil, deadline)

		if err := us12Execute(t, lockingScript, tx); err != nil {
			t.Fatalf(
				"the refund branch must validate at the refund locktime, got [%v]",
				err,
			)
		}
	})

	t.Run("refund branch validates after the refund locktime", func(t *testing.T) {
		tx, lockingScript := us12Spend(
			t,
			redeemScript,
			refundKey,
			nil,
			deadline+3_600,
		)

		if err := us12Execute(t, lockingScript, tx); err != nil {
			t.Fatalf(
				"the refund branch must validate after the refund locktime, got [%v]",
				err,
			)
		}
	})

	t.Run("refund branch does not validate before the refund locktime", func(t *testing.T) {
		tx, lockingScript := us12Spend(t, redeemScript, refundKey, nil, deadline-1)

		code := us12ErrorCode(t, us12Execute(t, lockingScript, tx))
		if code != txscript.ErrUnsatisfiedLockTime {
			t.Fatalf(
				"expected the spend to be rejected with %v, got %v",
				txscript.ErrUnsatisfiedLockTime,
				code,
			)
		}
	})

	t.Run("a foreign key cannot spend the refund branch", func(t *testing.T) {
		// The spending key is neither the wallet key nor the refund
		// key, so the script takes the refund branch (the wallet
		// branch's OP_EQUAL is false) and the refund branch's
		// OP_EQUALVERIFY against the refund public key hash fails.
		tx, lockingScript := us12Spend(t, redeemScript, foreignKey, nil, deadline)

		code := us12ErrorCode(t, us12Execute(t, lockingScript, tx))
		if code != txscript.ErrEqualVerify {
			t.Fatalf(
				"expected the refund branch's OP_EQUALVERIFY to reject the spend with %v, got %v",
				txscript.ErrEqualVerify,
				code,
			)
		}
	})

	t.Run("the wallet key cannot spend the refund branch", func(t *testing.T) {
		// The wallet key's public key is published, so the script's
		// wallet-branch OP_EQUAL is false (the script names the
		// reveal's wallet public key hash, not this key's) and the
		// refund branch's OP_EQUALVERIFY against the refund public key
		// hash fails.
		tx, lockingScript := us12Spend(t, redeemScript, walletKey, nil, deadline)

		code := us12ErrorCode(t, us12Execute(t, lockingScript, tx))
		if code != txscript.ErrEqualVerify {
			t.Fatalf(
				"expected the refund branch's OP_EQUALVERIFY to reject the spend with %v, got %v",
				txscript.ErrEqualVerify,
				code,
			)
		}
	})

	t.Run("the wallet key's signature cannot satisfy the refund branch", func(t *testing.T) {
		// The refund key's public key is published (so the refund
		// branch's OP_EQUALVERIFY passes), but the signature was made
		// by the wallet key, so the refund branch's OP_CHECKSIG fails.
		tx, lockingScript := us12Spend(t, redeemScript, walletKey, refundKey, deadline)

		code := us12ErrorCode(t, us12Execute(t, lockingScript, tx))
		if code != txscript.ErrNullFail {
			t.Fatalf(
				"expected the refund branch's OP_CHECKSIG to reject the spend with %v, got %v",
				txscript.ErrNullFail,
				code,
			)
		}
	})
}

// TestDepositWalletBranchSpend_DoesNotConsumeTheRefundPath documents the
// other half of the template: the wallet branch carries no
// OP_CHECKLOCKTIMEVERIFY, so the wallet may spend the deposit at any
// time, and a wallet-branch spend is not a refund spend. The refund
// path stays available to the depositor's refund key until the deposit
// is swept or accepted.
func TestDepositWalletBranchSpend_DoesNotConsumeTheRefundPath(t *testing.T) {
	walletKey, walletPKH := us12Key(t, 0x01)
	refundKey, refundPKH := us12Key(t, 0x02)

	// This reveal names the wallet key we are going to spend with, so
	// the script's wallet branch is the one that gets taken.
	redeemScript := us12RedeemScript(
		t,
		walletPKH[:],
		refundPKH[:],
		us12Hex(t, us12RefundLocktimeHex),
	)

	deadline := uint32(1642773600)

	// Well before the refund locktime: the wallet branch has no locktime
	// gate, so the spend validates.
	tx, lockingScript := us12Spend(t, redeemScript, walletKey, nil, deadline-1)

	if err := us12Execute(t, lockingScript, tx); err != nil {
		t.Fatalf(
			"the wallet branch must validate before the refund locktime, got [%v]",
			err,
		)
	}

	// The refund key's own branch is unaffected by that: the refund
	// spend still validates at the locktime.
	tx, lockingScript = us12Spend(t, redeemScript, refundKey, nil, deadline)

	if err := us12Execute(t, lockingScript, tx); err != nil {
		t.Fatalf(
			"the refund branch must still validate at the refund locktime, got [%v]",
			err,
		)
	}
}
