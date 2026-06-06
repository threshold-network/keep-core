package bitcoin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	btcec2 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"github.com/keep-network/keep-core/internal/testutils"
)

func TestNewScriptFromVarLenData(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	var tests = map[string]struct {
		data           []byte
		expectedScript Script
		expectedErr    error
	}{
		"proper variable length data": {
			data:           fromHex("1600148db50eb52063ea9d98b3eac91489a90f738986f6"),
			expectedScript: fromHex("00148db50eb52063ea9d98b3eac91489a90f738986f6"),
		},
		"nil variable length data": {
			data:        nil,
			expectedErr: fmt.Errorf("cannot read compact size uint: [EOF]"),
		},
		"variable length data with missing script": {
			data:        fromHex("16"),
			expectedErr: fmt.Errorf("malformed var len data"),
		},
		"variable length data with missing compact size uint": {
			data:        fromHex("00148db50eb52063ea9d98b3eac91489a90f738986f6"),
			expectedErr: fmt.Errorf("malformed var len data"),
		},
		"variable length data with wrong compact size uint value": {
			data:        fromHex("1500148db50eb52063ea9d98b3eac91489a90f738986f6"),
			expectedErr: fmt.Errorf("malformed var len data"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			script, err := NewScriptFromVarLenData(test.data)

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\nexpected: %+v\nactual:   %+v\n",
					test.expectedErr,
					err,
				)
			}

			testutils.AssertBytesEqual(t, test.expectedScript, script)
		})
	}
}

func TestScript_ToVarLenData(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	bytes20HashBytes, err := hex.DecodeString(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	if err != nil {
		t.Fatal(err)
	}
	var bytes20Hash [20]byte
	copy(bytes20Hash[:], bytes20HashBytes)

	bytes32HashBytes, err := hex.DecodeString(
		"86a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca96",
	)
	if err != nil {
		t.Fatal(err)
	}
	var bytes32Hash [32]byte
	copy(bytes32Hash[:], bytes32HashBytes)

	var tests = map[string]struct {
		script        Script
		expectedBytes []byte
	}{
		"p2pkh script": {
			script: func() Script {
				result, err := PayToPublicKeyHash(bytes20Hash)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}(),
			expectedBytes: fromHex("1976a9148db50eb52063ea9d98b3eac91489a90f738986f688ac"),
		},
		"p2wpkh script": {
			script: func() Script {
				result, err := PayToWitnessPublicKeyHash(bytes20Hash)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}(),
			expectedBytes: fromHex("1600148db50eb52063ea9d98b3eac91489a90f738986f6"),
		},
		"p2sh script": {
			script: func() Script {
				result, err := PayToScriptHash(bytes20Hash)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}(),
			expectedBytes: fromHex("17a9148db50eb52063ea9d98b3eac91489a90f738986f687"),
		},
		"p2wsh script": {
			script: func() Script {
				result, err := PayToWitnessScriptHash(bytes32Hash)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}(),
			expectedBytes: fromHex("22002086a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca96"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			bytes, err := test.script.ToVarLenData()
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(t, test.expectedBytes, bytes)
		})
	}
}

func TestPublicKeyHash(t *testing.T) {
	// An arbitrary uncompressed public key.
	publicKeyBytes, err := hex.DecodeString(
		"04989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9" +
			"d218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af",
	)
	if err != nil {
		t.Fatal(err)
	}

	x, y := elliptic.Unmarshal(btcec.S256(), publicKeyBytes)
	publicKey := &ecdsa.PublicKey{
		Curve: btcec.S256(),
		X:     x,
		Y:     y,
	}

	result := PublicKeyHash(publicKey)

	expectedResult, err := hex.DecodeString(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestPayToWitnessPublicKeyHash(t *testing.T) {
	// The 20-byte public key hash, same as the output of TestPublicKeyHash.
	publicKeyHashBytes, err := hex.DecodeString(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	if err != nil {
		t.Fatal(err)
	}

	var publicKeyHash [20]byte
	copy(publicKeyHash[:], publicKeyHashBytes)

	result, err := PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult, err := hex.DecodeString(
		"00148db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestPayToPublicKeyHash(t *testing.T) {
	// The 20-byte public key hash, same as the output of TestPublicKeyHash.
	publicKeyHashBytes, err := hex.DecodeString(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	if err != nil {
		t.Fatal(err)
	}

	var publicKeyHash [20]byte
	copy(publicKeyHash[:], publicKeyHashBytes)

	result, err := PayToPublicKeyHash(publicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult, err := hex.DecodeString(
		"76a9148db50eb52063ea9d98b3eac91489a90f738986f688ac",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestWitnessScriptHash(t *testing.T) {
	// An arbitrary redeem script.
	script, err := hex.DecodeString(
		"14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d0003952375" +
			"76a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e25" +
			"7eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68",
	)
	if err != nil {
		t.Fatal(err)
	}

	result := WitnessScriptHash(script)

	expectedResult, err := hex.DecodeString(
		"86a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca96",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestScriptHash(t *testing.T) {
	// An arbitrary redeem script.
	script, err := hex.DecodeString(
		"14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d0003952375" +
			"76a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a914e25" +
			"7eccafbc07c381642ce6e7e55120fb077fbed8804e0250162b175ac68",
	)
	if err != nil {
		t.Fatal(err)
	}

	result := ScriptHash(script)

	expectedResult, err := hex.DecodeString(
		"3ec459d0f3c29286ae5df5fcc421e2786024277e",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestPayToWitnessScriptHash(t *testing.T) {
	// The 32-byte script hash, same as the output of TestWitnessScriptHash.
	scriptHashBytes, err := hex.DecodeString(
		"86a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca96",
	)
	if err != nil {
		t.Fatal(err)
	}

	var scriptHash [32]byte
	copy(scriptHash[:], scriptHashBytes)

	result, err := PayToWitnessScriptHash(scriptHash)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult, err := hex.DecodeString(
		"002086a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca96",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestPayToScriptHash(t *testing.T) {
	// The 20-byte script hash, same as the output of TestScriptHash.
	scriptHashBytes, err := hex.DecodeString(
		"3ec459d0f3c29286ae5df5fcc421e2786024277e",
	)
	if err != nil {
		t.Fatal(err)
	}

	var scriptHash [20]byte
	copy(scriptHash[:], scriptHashBytes)

	result, err := PayToScriptHash(scriptHash)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult, err := hex.DecodeString(
		"a9143ec459d0f3c29286ae5df5fcc421e2786024277e87",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestPayToTaproot(t *testing.T) {
	outputKeyBytes, err := hex.DecodeString(
		"1b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f",
	)
	if err != nil {
		t.Fatal(err)
	}

	var outputKey [32]byte
	copy(outputKey[:], outputKeyBytes)

	result, err := PayToTaproot(outputKey)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult, err := hex.DecodeString(
		"51201b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f",
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestTaprootLeafHash(t *testing.T) {
	script := Script(hexToSlice(
		t,
		"76a9140102030405060708090a0b0c0d0e0f101112131488ac",
	))

	result, err := TaprootLeafHash(script)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult := hexToSlice(
		t,
		"37a57b86de2819d2b72a173df46238a7ad295ea1485d3b40e9415daa82b4fdcb",
	)

	testutils.AssertBytesEqual(t, expectedResult, result[:])
}

func TestTaprootTweakAndOutputKey(t *testing.T) {
	privateKey, _ := btcec2.PrivKeyFromBytes(
		hexToSlice(
			t,
			"0101010101010101010101010101010101010101010101010101010101010101",
		),
	)

	var internalKey [32]byte
	copy(internalKey[:], schnorr.SerializePubKey(privateKey.PubKey()))

	refundLeaf := Script(hexToSlice(
		t,
		"76a9140102030405060708090a0b0c0d0e0f101112131488ac",
	))

	merkleRoot, err := TaprootLeafHash(refundLeaf)
	if err != nil {
		t.Fatal(err)
	}

	tweak, err := TaprootTweak(internalKey, &merkleRoot)
	if err != nil {
		t.Fatal(err)
	}

	expectedTweak := hexToSlice(
		t,
		"6ca66b4600554f36d490d227669ba78c2d4778a8ecc07565ae2f9e87c28f124a",
	)

	testutils.AssertBytesEqual(t, expectedTweak, tweak[:])

	outputKey, err := TaprootOutputKey(internalKey, &merkleRoot)
	if err != nil {
		t.Fatal(err)
	}

	expectedOutputKey := hexToSlice(
		t,
		"b31d6b4f10bcea1dfcace63ce7defda9e718a4340b4b5befef6194488780ef17",
	)

	testutils.AssertBytesEqual(t, expectedOutputKey, outputKey[:])
}

func TestPayToTaprootWithScriptTree(t *testing.T) {
	privateKey, _ := btcec2.PrivKeyFromBytes(
		hexToSlice(
			t,
			"0202020202020202020202020202020202020202020202020202020202020202",
		),
	)

	var internalKey [32]byte
	copy(internalKey[:], schnorr.SerializePubKey(privateKey.PubKey()))

	merkleRootBytes := hexToSlice(
		t,
		"b2c459126150e0d47063ea7b6d0474a24c39e25908aae5740dd4787b67c6e19a",
	)
	var merkleRoot [32]byte
	copy(merkleRoot[:], merkleRootBytes)

	result, err := PayToTaprootWithScriptTree(internalKey, merkleRoot)
	if err != nil {
		t.Fatal(err)
	}

	expectedOutputKey := hexToSlice(
		t,
		"e339710a2348c113ade4a4e7d52bd1c12bc69818f1af7f41e161142701b93c96",
	)

	// Rebuild the expected P2TR script directly to avoid reusing
	// PayToTaprootWithScriptTree.
	var expectedKey [32]byte
	copy(expectedKey[:], expectedOutputKey)
	expectedResult, err := PayToTaproot(expectedKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBytesEqual(t, expectedResult, result)
}

func TestGetScriptType(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	var tests = map[string]struct {
		script       Script
		expectedType ScriptType
	}{
		"p2pkh script": {
			script:       fromHex("76a9148db50eb52063ea9d98b3eac91489a90f738986f688ac"),
			expectedType: P2PKHScript,
		},
		"p2wpkh script": {
			script:       fromHex("00148db50eb52063ea9d98b3eac91489a90f738986f6"),
			expectedType: P2WPKHScript,
		},
		"p2sh script": {
			script:       fromHex("a9143ec459d0f3c29286ae5df5fcc421e2786024277e87"),
			expectedType: P2SHScript,
		},
		"p2wsh script": {
			script:       fromHex("002086a303cdd2e2eab1d1679f1a813835dc5a1b65321077cdccaf08f98cbf04ca96"),
			expectedType: P2WSHScript,
		},
		"p2tr script": {
			script:       fromHex("51201b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f"),
			expectedType: P2TRScript,
		},
		"non-standard script": {
			script: fromHex(
				"14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508f9f0c90d0003" +
					"95237576a9148db50eb52063ea9d98b3eac91489a90f738986f68763ac6776a" +
					"91428e081f285138ccbe389c1eb8985716230129f89880460bcea61b175ac68",
			),
			expectedType: NonStandardScript,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualType := GetScriptType(test.script)

			testutils.AssertIntsEqual(
				t,
				"script type",
				int(test.expectedType),
				int(actualType),
			)
		})
	}
}

func TestExtractTaprootKey(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	var outputKey [32]byte
	copy(
		outputKey[:],
		fromHex("1b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f"),
	)

	var tests = map[string]struct {
		script            Script
		expectedOutputKey [32]byte
		expectedErr       error
	}{
		"P2TR script": {
			script:            fromHex("51201b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f"),
			expectedOutputKey: outputKey,
		},
		"other script": {
			script:      fromHex("00148db50eb52063ea9d98b3eac91489a90f738986f6"),
			expectedErr: fmt.Errorf("not a P2TR script"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualOutputKey, err := ExtractTaprootKey(test.script)

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\nexpected: %+v\nactual:   %+v\n",
					test.expectedErr,
					err,
				)
			}

			if test.expectedOutputKey != actualOutputKey {
				t.Errorf(
					"unexpected taproot output key\nexpected: 0x%x\nactual:   0x%x\n",
					test.expectedOutputKey,
					actualOutputKey,
				)
			}
		})
	}
}

func TestExtractPublicKeyHash(t *testing.T) {
	fromHex := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	var publicKeyHash [20]byte
	copy(publicKeyHash[:], fromHex("8db50eb52063ea9d98b3eac91489a90f738986f6"))

	var tests = map[string]struct {
		script                Script
		expectedPublicKeyHash [20]byte
		expectedErr           error
	}{
		"P2WPKH script": {
			script:                fromHex("00148db50eb52063ea9d98b3eac91489a90f738986f6"),
			expectedPublicKeyHash: publicKeyHash,
		},
		"P2PKH script": {
			script:                fromHex("76a9148db50eb52063ea9d98b3eac91489a90f738986f688ac"),
			expectedPublicKeyHash: publicKeyHash,
		},
		"other script": {
			script:      fromHex("a9143ec459d0f3c29286ae5df5fcc421e2786024277e87"),
			expectedErr: fmt.Errorf("not a P2WPKH or P2PKH script"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualPublicKeyHash, err := ExtractPublicKeyHash(test.script)

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\nexpected: %+v\nactual:   %+v\n",
					test.expectedErr,
					err,
				)
			}

			if test.expectedPublicKeyHash != actualPublicKeyHash {
				t.Errorf(
					"unexpected public key hash\nexpected: 0x%x\nactual:   0x%x\n",
					test.expectedPublicKeyHash,
					actualPublicKeyHash,
				)
			}
		})
	}
}
