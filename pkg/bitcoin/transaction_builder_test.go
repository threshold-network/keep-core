package bitcoin

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/keep-network/keep-core/internal/testutils"
)

func TestNewTransactionBuilder(t *testing.T) {
	localChain := newLocalChain()
	builder := NewTransactionBuilder(localChain)

	if !reflect.DeepEqual(localChain, builder.chain) {
		t.Error("unexpected chain reference")
	}

	testutils.AssertIntsEqual(
		t,
		"internal version",
		1,
		int(builder.internal.Version),
	)
	testutils.AssertIntsEqual(
		t,
		"internal locktime",
		0,
		int(builder.internal.LockTime),
	)
}

func TestTransactionBuilder_AddPublicKeyHashInput(t *testing.T) {
	var tests = map[string]struct {
		inputTransactionHex string
		outputIndex         uint32
		value               int64
		witness             bool
	}{
		// https://live.blockcypher.com/btc-testnet/tx/c8a2c407309b9434cb73d4788ce4ac895084240eec7bb440e7f76b75be1296e1
		"non-witness public key hash input": {
			inputTransactionHex: "01000000012d4e0b1ef0bf21eed32f6e2f11353b78534dcf21852d506f6f53b64bb5c6b4c500000000c84730440220590e998a5c28965fd442e700445a60c494124fdbb8aa39cc20c04f2aedadb1a602206acb2f852cd7adea65fe9209024e18d2d6ccac0b1e45c61d80c9bcd62f3e5a12012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed880448f2b262b175ac68ffffffff0110400000000000001976a9148db50eb52063ea9d98b3eac91489a90f738986f688ac00000000",
			outputIndex:         0,
			value:               16400,
			witness:             false,
		},
		// https://live.blockcypher.com/btc-testnet/tx/f8eaf242a55ea15e602f9f990e33f67f99dfbe25d1802bbde63cc1caabf99668
		"witness public key hash input": {
			inputTransactionHex: "01000000000102bc187be612bc3db8cfcdec56b75e9bc0262ab6eacfe27cc1a699bacd53e3d07400000000c948304502210089a89aaf3fec97ac9ffa91cdff59829f0cb3ef852a468153e2c0e2b473466d2e022072902bb923ef016ac52e941ced78f816bf27991c2b73211e227db27ec200bc0a012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68ffffffffdc557e737b6688c5712649b86f7757a722dc3d42786f23b2fa826394dfec545c0000000000ffffffff01488a0000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6000347304402203747f5ee31334b11ebac6a2a156b1584605de8d91a654cd703f9c8438634997402202059d680211776f93c25636266b02e059ed9fcc6209f7d3d9926c49a0d8750ed012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d95c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac6800000000",
			outputIndex:         0,
			value:               35400,
			witness:             true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			localChain := newLocalChain()
			builder := NewTransactionBuilder(localChain)

			inputTransaction := transactionFrom(t, test.inputTransactionHex)

			err := localChain.addTransaction(inputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			inputTransactionUtxo := &UnspentTransactionOutput{
				Outpoint: &TransactionOutpoint{
					TransactionHash: inputTransaction.Hash(),
					OutputIndex:     test.outputIndex,
				},
				Value: test.value,
			}

			err = builder.AddPublicKeyHashInput(inputTransactionUtxo)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"sighash args count",
				1,
				len(builder.sigHashArgs),
			)
			assertSigHashArgs(
				t,
				&inputSigHashArgs{
					value:      test.value,
					scriptCode: inputTransaction.Outputs[test.outputIndex].PublicKeyScript,
					witness:    test.witness,
				},
				builder.sigHashArgs[0],
			)
			testutils.AssertIntsEqual(
				t,
				"internal inputs count",
				1,
				len(builder.internal.TxIn),
			)
			assertInternalInput(t, builder, 0, &TransactionInput{
				Outpoint:        inputTransactionUtxo.Outpoint,
				SignatureScript: nil,
				Witness:         nil,
				Sequence:        0xffffffff,
			})
		})
	}
}

func TestTransactionBuilder_AddScriptHashInput(t *testing.T) {
	var tests = map[string]struct {
		inputTransactionHex string
		outputIndex         uint32
		value               int64
		redeemScriptHex     string
		witness             bool
	}{
		// https://live.blockcypher.com/btc-testnet/tx/74d0e353cdba99a6c17ce2cfeab62a26c09b5eb756eccdcfb83dbc12e67b18bc
		"non-witness script hash input": {
			inputTransactionHex: "01000000000101d9fdf44eb0874a31a462dc0aedce55c0b5be6d20956b4cdfbe1c16761f7c4aa60100000000ffffffff02a86100000000000017a9143ec459d0f3c29286ae5df5fcc421e2786024277e8716a1110000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed0247304402204e779706c5134032f6be73633a4d32de084154a7fd16c82810325584eea6406a022068bf855004476b8776f5a902a4d518a486ff7ebc6dc12fc31cd94e3e9b4220bb0121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000",
			outputIndex:         0,
			value:               25000,
			redeemScriptHex:     "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68",
			witness:             false,
		},
		// https://live.blockcypher.com/btc-testnet/tx/5c54ecdf946382fab2236f78423ddc22a757776fb8492671c588667b737e55dc
		"witness script hash input": {
			inputTransactionHex: "01000000000101a0367a0790e3dfc199df34ca9ce5c35591510b6525d2d5869166728a5ed554be0100000000ffffffff02e02e00000000000022002086a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca962c2c110000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed0247304402206dafd502aac9d4d542416664063533b1fed1d16877f0295740e1b09ec2abe05102200be28d9dd76863796addef4b9595aad23b2e9363ac2d64f75c21beb0e2ade5df0121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000",
			outputIndex:         0,
			value:               12000,
			redeemScriptHex:     "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68",
			witness:             true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			localChain := newLocalChain()
			builder := NewTransactionBuilder(localChain)

			inputTransaction := transactionFrom(t, test.inputTransactionHex)

			err := localChain.addTransaction(inputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			inputTransactionUtxo := &UnspentTransactionOutput{
				Outpoint: &TransactionOutpoint{
					TransactionHash: inputTransaction.Hash(),
					OutputIndex:     test.outputIndex,
				},
				Value: test.value,
			}

			redeemScript := hexToSlice(t, test.redeemScriptHex)

			err = builder.AddScriptHashInput(inputTransactionUtxo, redeemScript)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"sighash args count",
				1,
				len(builder.sigHashArgs),
			)
			assertSigHashArgs(
				t,
				&inputSigHashArgs{
					value:      test.value,
					scriptCode: redeemScript,
					witness:    test.witness,
				},
				builder.sigHashArgs[0],
			)
			testutils.AssertIntsEqual(
				t,
				"internal inputs count",
				1,
				len(builder.internal.TxIn),
			)

			var expectedSignatureScript []byte
			var expectedWitness [][]byte
			if test.witness {
				expectedWitness = append(expectedWitness, redeemScript)
			} else {
				expectedSignatureScript = redeemScript
			}
			assertInternalInput(t, builder, 0, &TransactionInput{
				Outpoint:        inputTransactionUtxo.Outpoint,
				SignatureScript: expectedSignatureScript,
				Witness:         expectedWitness,
				Sequence:        0xffffffff,
			})
		})
	}
}

func TestTransactionBuilder_AddOutput(t *testing.T) {
	builder := NewTransactionBuilder(nil) // chain is not relevant here

	output := &TransactionOutput{
		Value:           10000,
		PublicKeyScript: hexToSlice(t, "00148db50eb52063ea9d98b3eac91489a90f738986f6"),
	}

	builder.AddOutput(output)

	assertInternalOutput(t, builder, 0, output)
}

func TestTransactionBuilder_ReplaceUnsignedTransaction(t *testing.T) {
	builder := NewTransactionBuilder(nil)

	var initialInputHash1 chainhash.Hash
	var initialInputHash2 chainhash.Hash
	initialInputHash1[0] = 0x11
	initialInputHash2[0] = 0x22

	builder.internal.AddTxIn(
		wire.NewTxIn(
			wire.NewOutPoint(&initialInputHash1, 1),
			[]byte{0xde, 0xad},
			nil,
		),
	)
	builder.internal.AddTxIn(
		wire.NewTxIn(
			wire.NewOutPoint(&initialInputHash2, 2),
			nil,
			[][]byte{{0xbe, 0xef}},
		),
	)
	builder.sigHashArgs = append(
		builder.sigHashArgs,
		&inputSigHashArgs{value: 111, scriptCode: []byte{0x51}, witness: false},
		&inputSigHashArgs{value: 222, scriptCode: []byte{0x52}, witness: true},
	)
	builder.sigHashes = []*big.Int{big.NewInt(1), big.NewInt(2)}

	var replacementInputHash1 chainhash.Hash
	var replacementInputHash2 chainhash.Hash
	replacementInputHash1[0] = 0x33
	replacementInputHash2[0] = 0x44

	err := builder.ReplaceUnsignedTransaction(
		&Transaction{
			Version: 2,
			Inputs: []*TransactionInput{
				{
					Outpoint: &TransactionOutpoint{
						TransactionHash: Hash(replacementInputHash1),
						OutputIndex:     7,
					},
					Sequence: 0xffffffff,
				},
				{
					Outpoint: &TransactionOutpoint{
						TransactionHash: Hash(replacementInputHash2),
						OutputIndex:     8,
					},
					Sequence: 0xffffffff,
				},
			},
			Outputs: []*TransactionOutput{
				{
					Value:           1000,
					PublicKeyScript: hexToSlice(t, "0014deadbeef"),
				},
			},
			Locktime: 0,
		},
	)
	if err != nil {
		t.Fatalf("unexpected replacement error: [%v]", err)
	}

	if len(builder.sigHashes) != 0 {
		t.Fatalf("expected sighashes reset after replacement: [%d]", len(builder.sigHashes))
	}

	// Preserve P2SH/P2WSH placeholder scripts needed for final signature
	// application while replacing tx skeleton.
	if !reflect.DeepEqual([]byte{0xde, 0xad}, builder.internal.TxIn[0].SignatureScript) {
		t.Fatalf(
			"unexpected preserved signature script\nexpected: [%x]\nactual:   [%x]",
			[]byte{0xde, 0xad},
			builder.internal.TxIn[0].SignatureScript,
		)
	}

	if len(builder.internal.TxIn[1].Witness) != 1 {
		t.Fatalf("unexpected preserved witness length: [%d]", len(builder.internal.TxIn[1].Witness))
	}

	if !reflect.DeepEqual([]byte{0xbe, 0xef}, builder.internal.TxIn[1].Witness[0]) {
		t.Fatalf(
			"unexpected preserved witness script\nexpected: [%x]\nactual:   [%x]",
			[]byte{0xbe, 0xef},
			builder.internal.TxIn[1].Witness[0],
		)
	}

	inputs, outputs, err := builder.UnsignedTransactionIO()
	if err != nil {
		t.Fatalf("unexpected extraction error after replacement: [%v]", err)
	}

	if len(inputs) != 2 {
		t.Fatalf("unexpected input count after replacement: [%d]", len(inputs))
	}

	if inputs[0].TxIDHex != replacementInputHash1.String() || inputs[0].Vout != 7 {
		t.Fatalf("unexpected first input after replacement: [%+v]", inputs[0])
	}

	if inputs[1].TxIDHex != replacementInputHash2.String() || inputs[1].Vout != 8 {
		t.Fatalf("unexpected second input after replacement: [%+v]", inputs[1])
	}

	if len(outputs) != 1 {
		t.Fatalf("unexpected output count after replacement: [%d]", len(outputs))
	}
}

func TestTransactionBuilder_ReplaceUnsignedTransaction_RejectsInputMetadataMismatch(
	t *testing.T,
) {
	builder := NewTransactionBuilder(nil)

	var txHash chainhash.Hash
	builder.internal.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txHash, 0), nil, nil))
	builder.sigHashArgs = append(builder.sigHashArgs, &inputSigHashArgs{value: 1})

	err := builder.ReplaceUnsignedTransaction(
		&Transaction{
			Inputs: []*TransactionInput{
				{
					Outpoint: &TransactionOutpoint{
						TransactionHash: Hash(txHash),
						OutputIndex:     0,
					},
				},
				{
					Outpoint: &TransactionOutpoint{
						TransactionHash: Hash(txHash),
						OutputIndex:     1,
					},
				},
			},
		},
	)
	if err == nil {
		t.Fatal("expected input metadata mismatch error")
	}

	if !reflect.DeepEqual(
		fmt.Sprintf(
			"input metadata mismatch: [%d] tx inputs, [%d] sighash args",
			2,
			1,
		),
		err.Error(),
	) {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestTransactionBuilder_UnsignedTransactionIO(t *testing.T) {
	builder := NewTransactionBuilder(nil)

	var txHash chainhash.Hash
	for i := range txHash {
		txHash[i] = byte(i + 1)
	}
	const expectedTxIDHex = "201f1e1d1c1b1a191817161514131211100f0e0d0c0b0a090807060504030201"

	builder.internal.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txHash, 7), nil, nil))
	builder.sigHashArgs = append(builder.sigHashArgs, &inputSigHashArgs{value: 1234})
	builder.AddOutput(&TransactionOutput{
		Value:           1000,
		PublicKeyScript: hexToSlice(t, "0014deadbeef"),
	})

	inputs, outputs, err := builder.UnsignedTransactionIO()
	if err != nil {
		t.Fatalf("unexpected extraction error: [%v]", err)
	}

	if len(inputs) != 1 {
		t.Fatalf("unexpected input count: [%d]", len(inputs))
	}

	if inputs[0].TxIDHex != expectedTxIDHex {
		t.Fatalf(
			"unexpected input txid\nexpected: [%v]\nactual:   [%v]",
			expectedTxIDHex,
			inputs[0].TxIDHex,
		)
	}

	if inputs[0].Vout != 7 {
		t.Fatalf("unexpected input vout: [%d]", inputs[0].Vout)
	}

	if inputs[0].ValueSats != 1234 {
		t.Fatalf("unexpected input value: [%d]", inputs[0].ValueSats)
	}

	if len(outputs) != 1 {
		t.Fatalf("unexpected output count: [%d]", len(outputs))
	}

	if outputs[0].ScriptPubKeyHex != "0014deadbeef" {
		t.Fatalf(
			"unexpected output script\nexpected: [%v]\nactual:   [%v]",
			"0014deadbeef",
			outputs[0].ScriptPubKeyHex,
		)
	}

	if outputs[0].ValueSats != 1000 {
		t.Fatalf("unexpected output value: [%d]", outputs[0].ValueSats)
	}
}

func TestTransactionBuilder_UnsignedTransactionIO_RejectsNegativeInputValue(
	t *testing.T,
) {
	builder := NewTransactionBuilder(nil)

	var txHash chainhash.Hash
	builder.internal.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txHash, 0), nil, nil))
	builder.sigHashArgs = append(builder.sigHashArgs, &inputSigHashArgs{value: -1})
	builder.AddOutput(&TransactionOutput{
		Value:           1,
		PublicKeyScript: hexToSlice(t, "0014aa"),
	})

	_, _, err := builder.UnsignedTransactionIO()
	if err == nil {
		t.Fatal("expected extraction error")
	}
}

func TestTransactionBuilder_UnsignedTransactionIO_RejectsNegativeOutputValue(
	t *testing.T,
) {
	builder := NewTransactionBuilder(nil)

	var txHash chainhash.Hash
	builder.internal.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txHash, 0), nil, nil))
	builder.sigHashArgs = append(builder.sigHashArgs, &inputSigHashArgs{value: 1})
	builder.AddOutput(&TransactionOutput{
		Value:           -1,
		PublicKeyScript: hexToSlice(t, "0014aa"),
	})

	_, _, err := builder.UnsignedTransactionIO()
	if err == nil {
		t.Fatal("expected extraction error")
	}
}

// The goal of this test is making sure that the TransactionBuilder can
// produce proper signature hashes and apply signatures for all input types,
// i.e. P2PKH, P2WPKH, P2SH, and P2WSH. This test uses transactions that
// contain those inputs.
func TestTransactionBuilder_Signing(t *testing.T) {
	var tests = map[string]struct {
		inputs []struct {
			transactionHex  string
			outputIndex     uint32
			value           int64
			redeemScriptHex string
		}
		outputs []struct {
			publicKeyScriptHex string
			value              int64
		}
		signatures                   []*SignatureContainer
		expectedSigHashesHexes       []string
		expectedSignedTransactionHex string
	}{
		// https://live.blockcypher.com/btc-testnet/tx/435d4aff6d4bc34134877bd3213c17970142fdd04d4113d534120033b9eecb2e
		"P2WPKH, P2SH and P2WSH inputs with one P2WPKH output": {
			inputs: []struct {
				transactionHex  string
				outputIndex     uint32
				value           int64
				redeemScriptHex string
			}{
				{
					transactionHex:  "01000000000102bc187be612bc3db8cfcdec56b75e9bc0262ab6eacfe27cc1a699bacd53e3d07400000000c948304502210089a89aaf3fec97ac9ffa91cdff59829f0cb3ef852a468153e2c0e2b473466d2e022072902bb923ef016ac52e941ced78f816bf27991c2b73211e227db27ec200bc0a012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68ffffffffdc557e737b6688c5712649b86f7757a722dc3d42786f23b2fa826394dfec545c0000000000ffffffff01488a0000000000001600148db50eb52063ea9d98b3eac91489a90f738986f6000347304402203747f5ee31334b11ebac6a2a156b1584605de8d91a654cd703f9c8438634997402202059d680211776f93c25636266b02e059ed9fcc6209f7d3d9926c49a0d8750ed012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d95c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac6800000000",
					outputIndex:     0,
					value:           35400,
					redeemScriptHex: "",
				},
				{
					transactionHex:  "01000000000101e37f552fc23fa0032bfd00c8eef5f5c22bf85fe4c6e735857719ff8a4ff66eb80100000000ffffffff02684200000000000017a9143ec459d0f3c29286ae5df5fcc421e2786024277e8742b7100000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed0248304502210084eb60347b9aa48d9a53c6ab0fc2c2357a0df430d193507facfb2238e46f034502202a29d11e128dba3ff3a8ad9a1e820a3b58e89e37fa90d1cc2b3f05207599fef00121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000",
					outputIndex:     0,
					value:           17000,
					redeemScriptHex: "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68",
				},
				{
					transactionHex:  "01000000000101dc557e737b6688c5712649b86f7757a722dc3d42786f23b2fa826394dfec545c0100000000ffffffff02102700000000000022002086a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca962cff100000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed02473044022050759dde2c84bccf3c1502b0e33a6acb570117fd27a982c0c2991c9f9737508e02201fcba5d6f6c0ab780042138a9110418b3f589d8d09a900f20ee28cfcdb14d2970121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000",
					outputIndex:     0,
					value:           10000,
					redeemScriptHex: "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68",
				},
			},
			outputs: []struct {
				publicKeyScriptHex string
				value              int64
			}{
				{
					publicKeyScriptHex: "00148db50eb52063ea9d98b3eac91489a90f738986f6",
					value:              60800,
				},
			},
			signatures: []*SignatureContainer{
				{
					R:         new(big.Int).SetBytes(hexToSlice(t, "baf754252d0d6a49aceba7eb0ec40b4cc568e8c659e168b96598a11cf56dc078")),
					S:         new(big.Int).SetBytes(hexToSlice(t, "51117466ee998a3fc72221006817e8cfe9c2e71ad622ff811a0bf100d888d49c")),
					PublicKey: hexToPublicKet(t, "04989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9d218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af"),
				},
				{
					R:         new(big.Int).SetBytes(hexToSlice(t, "92327ddff69a2b8c7ae787c5d590a2f14586089e6339e942d56e82aa42052cd9")),
					S:         new(big.Int).SetBytes(hexToSlice(t, "4c0d1700ba1ac617da27fee032a57937c9607f0187199ed3c46954df845643d7")),
					PublicKey: hexToPublicKet(t, "04989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9d218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af"),
				},
				{
					R:         new(big.Int).SetBytes(hexToSlice(t, "14a535eb334656665ac69a678dbf7c019c4f13262e9ea4d195c61a00cd5f698d")),
					S:         new(big.Int).SetBytes(hexToSlice(t, "23c0062913c4614bdff07f94475ceb4c585df53f71611776c3521ed8f8785913")),
					PublicKey: hexToPublicKet(t, "04989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9d218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af"),
				},
			},
			expectedSigHashesHexes: []string{
				"db0e8c898d3a59a23a70b3d910db720b5942445a24bce2dd96e0488a9de660a9",
				"0730c379a7c60686255d4730afdf7ce321e83f5e4956346c19956b764a237831",
				"126b2edd1b3c28dbff6cd48a9eb666558cb59d1008db60bb5f7bbf1a0d45e588",
			},
			expectedSignedTransactionHex: "010000000001036896f9abcac13ce6bd2b80d125bedf997ff6330e999f2f605ea15ea542f2eaf80000000000ffffffffed0ae94da996c6f3b89dfe967675d4808251db93e81022ae9e038d06f92efed400000000c948304502210092327ddff69a2b8c7ae787c5d590a2f14586089e6339e942d56e82aa42052cd902204c0d1700ba1ac617da27fee032a57937c9607f0187199ed3c46954df845643d7012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68ffffffffe37f552fc23fa0032bfd00c8eef5f5c22bf85fe4c6e735857719ff8a4ff66eb80000000000ffffffff0180ed0000000000001600148db50eb52063ea9d98b3eac91489a90f738986f602483045022100baf754252d0d6a49aceba7eb0ec40b4cc568e8c659e168b96598a11cf56dc078022051117466ee998a3fc72221006817e8cfe9c2e71ad622ff811a0bf100d888d49c012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d90003473044022014a535eb334656665ac69a678dbf7c019c4f13262e9ea4d195c61a00cd5f698d022023c0062913c4614bdff07f94475ceb4c585df53f71611776c3521ed8f8785913012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d95c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac6800000000",
		},
		// https://live.blockcypher.com/btc-testnet/tx/7831d0dfde7e160f3b9bb66c433710f0d3110d73ea78b9db65e81c091a6718a0
		"P2WSH and P2PKH inputs with one P2WPKH output": {
			inputs: []struct {
				transactionHex  string
				outputIndex     uint32
				value           int64
				redeemScriptHex string
			}{
				{
					transactionHex:  "010000000001012d4e0b1ef0bf21eed32f6e2f11353b78534dcf21852d506f6f53b64bb5c6b4c50100000000ffffffff02384a000000000000220020b1f83e226979dc9fe74e87f6d303dbb08a27a1c7ce91664033f34c7f2d214cd76c45110000000000160014e257eccafbc07c381642ce6e7e55120fb077fbed02473044022072109558ed0ad905e3853df8a987bb1353c0b3935b30c568763820c711600657022051ebcb9f03897f9c508d66d1c587cd81d888994e3b0bf819a9ef3b2df934328c0121039d61d62dcd048d3f8550d22eb90b4af908db60231d117aeede04e7bc11907bfa00000000",
					outputIndex:     0,
					value:           19000,
					redeemScriptHex: "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed880448f2b262b175ac68",
				},
				{
					transactionHex:  "01000000012d4e0b1ef0bf21eed32f6e2f11353b78534dcf21852d506f6f53b64bb5c6b4c500000000c84730440220590e998a5c28965fd442e700445a60c494124fdbb8aa39cc20c04f2aedadb1a602206acb2f852cd7adea65fe9209024e18d2d6ccac0b1e45c61d80c9bcd62f3e5a12012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d94c5c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed880448f2b262b175ac68ffffffff0110400000000000001976a9148db50eb52063ea9d98b3eac91489a90f738986f688ac00000000",
					outputIndex:     0,
					value:           16400,
					redeemScriptHex: "",
				},
			},
			outputs: []struct {
				publicKeyScriptHex string
				value              int64
			}{
				{
					publicKeyScriptHex: "00148db50eb52063ea9d98b3eac91489a90f738986f6",
					value:              33800,
				},
			},
			signatures: []*SignatureContainer{
				{
					R:         new(big.Int).SetBytes(hexToSlice(t, "c52bc876cdee80a3061ace3ffbce5e860942d444cd38e00e5f63fd8e818d7e7c")),
					S:         new(big.Int).SetBytes(hexToSlice(t, "40a7017bb8213991697705e7092c481526c788a4731d06e582dc1c57bed7243b")),
					PublicKey: hexToPublicKet(t, "04989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9d218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af"),
				},
				{
					R:         new(big.Int).SetBytes(hexToSlice(t, "4382deb051f9f3e2b539e4bac2d1a50faf8d66bc7a3a3f3d286dabd96d92b58b")),
					S:         new(big.Int).SetBytes(hexToSlice(t, "7c74c6aaf48e25d07e02bb4039606d77ecfd80c492c050ab2486af6027fc2d5a")),
					PublicKey: hexToPublicKet(t, "04989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9d218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af"),
				},
			},
			expectedSigHashesHexes: []string{
				"5c83f28b996fedb35ffb1e02e885599d6a1fe9ed7671e849e81ecc50a3020ea5",
				"f75ee5a069404db9a8684159589c59b01c913135a47d36828b433019e46733f1",
			},
			expectedSignedTransactionHex: "01000000000102173a201f597a2c8ccd7842303a6653bb87437fb08dae671731a075403b32a2fd0000000000ffffffffe19612be756bf7e740b47bec0e24845089ace48c78d473cb34949b3007c4a2c8000000006a47304402204382deb051f9f3e2b539e4bac2d1a50faf8d66bc7a3a3f3d286dabd96d92b58b02207c74c6aaf48e25d07e02bb4039606d77ecfd80c492c050ab2486af6027fc2d5a012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9ffffffff0108840000000000001600148db50eb52063ea9d98b3eac91489a90f738986f603483045022100c52bc876cdee80a3061ace3ffbce5e860942d444cd38e00e5f63fd8e818d7e7c022040a7017bb8213991697705e7092c481526c788a4731d06e582dc1c57bed7243b012103989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d95c14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e257eccafbc07c381642ce6e7e55120fb077fbed880448f2b262b175ac680000000000",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			localChain := newLocalChain()
			builder := NewTransactionBuilder(localChain)

			for _, input := range test.inputs {
				inputTransaction := transactionFrom(t, input.transactionHex)

				err := localChain.addTransaction(inputTransaction)
				if err != nil {
					t.Fatal(err)
				}

				inputTransactionUtxo := &UnspentTransactionOutput{
					Outpoint: &TransactionOutpoint{
						TransactionHash: inputTransaction.Hash(),
						OutputIndex:     input.outputIndex,
					},
					Value: input.value,
				}

				if len(input.redeemScriptHex) > 0 {
					redeemScript := hexToSlice(t, input.redeemScriptHex)
					err := builder.AddScriptHashInput(inputTransactionUtxo, redeemScript)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					err := builder.AddPublicKeyHashInput(inputTransactionUtxo)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			for _, output := range test.outputs {
				builder.AddOutput(&TransactionOutput{
					Value:           output.value,
					PublicKeyScript: hexToSlice(t, output.publicKeyScriptHex),
				})
			}

			sigHashes, err := builder.ComputeSignatureHashes()
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"sighashes count",
				len(test.expectedSigHashesHexes),
				len(sigHashes),
			)

			for i, sigHashHex := range test.expectedSigHashesHexes {
				testutils.AssertBigIntsEqual(
					t,
					fmt.Sprintf("sighash for input [%v]", i),
					new(big.Int).SetBytes(hexToSlice(t, sigHashHex)),
					sigHashes[i],
				)
			}

			testutils.AssertIntsEqual(
				t,
				"stored sighashes count",
				len(test.expectedSigHashesHexes),
				len(builder.sigHashes),
			)

			transaction, err := builder.AddSignatures(test.signatures)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				transaction.Serialize(),
				hexToSlice(t, test.expectedSignedTransactionHex),
			)
		})
	}
}

func assertSigHashArgs(t *testing.T, expected, actual *inputSigHashArgs) {
	testutils.AssertIntsEqual(
		t,
		"sighash args value",
		int(expected.value),
		int(actual.value),
	)

	testutils.AssertBytesEqual(
		t,
		expected.scriptCode,
		actual.scriptCode,
	)

	testutils.AssertBoolsEqual(
		t,
		"sighash args witness flag",
		expected.witness,
		actual.witness,
	)
}

func assertInternalInput(
	t *testing.T,
	builder *TransactionBuilder,
	index int,
	expected *TransactionInput,
) {
	internalInput := builder.internal.TxIn[index]

	testutils.AssertStringsEqual(
		t,
		"outpoint's transaction hash",
		expected.Outpoint.TransactionHash.Hex(ReversedByteOrder),
		internalInput.PreviousOutPoint.Hash.String(),
	)

	testutils.AssertIntsEqual(
		t,
		"outpoint's output index",
		int(expected.Outpoint.OutputIndex),
		int(internalInput.PreviousOutPoint.Index),
	)

	testutils.AssertBytesEqual(t, expected.SignatureScript, internalInput.SignatureScript)

	if !reflect.DeepEqual(expected.Witness, [][]byte(internalInput.Witness)) {
		t.Errorf("unexpected witness")
	}

	testutils.AssertIntsEqual(
		t,
		"sequence",
		int(expected.Sequence),
		int(internalInput.Sequence),
	)
}

func assertInternalOutput(
	t *testing.T,
	builder *TransactionBuilder,
	index int,
	expected *TransactionOutput,
) {
	internalOutput := builder.internal.TxOut[index]

	testutils.AssertIntsEqual(
		t,
		"value",
		int(expected.Value),
		int(internalOutput.Value),
	)

	testutils.AssertBytesEqual(t, expected.PublicKeyScript, internalOutput.PkScript)
}
