package tbtc

import (
	"encoding/hex"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"

	"github.com/keep-network/keep-core/internal/testutils"
)

func TestDeposit_Script(t *testing.T) {
	hexToSlice := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatalf("error while converting [%v]: [%v]", hexString, err)
		}
		return bytes
	}

	var tests = map[string]struct {
		depositor           string
		blindingFactor      string
		walletPublicKeyHash string
		refundPublicKeyHash string
		refundLocktime      string
		extraData           string
		expectedScript      string
	}{
		"no extra data": {
			depositor:           "934b98637ca318a4d6e7ca6ffd1690b8e77df637",
			blindingFactor:      "f9f0c90d00039523",
			walletPublicKeyHash: "8db50eb52063ea9d98b3eac91489a90f738986f6",
			refundPublicKeyHash: "28e081f285138ccbe389c1eb8985716230129f89",
			refundLocktime:      "60bcea61",
			extraData:           "",
			expectedScript: "14934b98637ca318a4d6e7ca6ffd1690b8e77df637750" +
				"8f9f0c90d000395237576a9148db50eb52063ea9d98b3eac91489a90f" +
				"738986f68763ac6776a91428e081f285138ccbe389c1eb89857162301" +
				"29f89880460bcea61b175ac68",
		},
		"with extra data": {
			depositor:           "934b98637ca318a4d6e7ca6ffd1690b8e77df637",
			blindingFactor:      "f9f0c90d00039523",
			walletPublicKeyHash: "8db50eb52063ea9d98b3eac91489a90f738986f6",
			refundPublicKeyHash: "28e081f285138ccbe389c1eb8985716230129f89",
			refundLocktime:      "60bcea61",
			extraData: "a9b38ea6435c8941d6eda6a46b68e3e2117196995bd154ab55" +
				"196396b03d9bda",
			expectedScript: "14934b98637ca318a4d6e7ca6ffd1690b8e77df637752" +
				"0a9b38ea6435c8941d6eda6a46b68e3e2117196995bd154ab55196396" +
				"b03d9bda7508f9f0c90d000395237576a9148db50eb52063ea9d98b3e" +
				"ac91489a90f738986f68763ac6776a91428e081f285138ccbe389c1eb" +
				"8985716230129f89880460bcea61b175ac68",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			// Fill only the fields relevant for script computation.
			d := new(Deposit)
			d.Depositor = chain.Address(test.depositor)
			copy(d.BlindingFactor[:], hexToSlice(test.blindingFactor))
			copy(d.WalletPublicKeyHash[:], hexToSlice(test.walletPublicKeyHash))
			copy(d.RefundPublicKeyHash[:], hexToSlice(test.refundPublicKeyHash))
			copy(d.RefundLocktime[:], hexToSlice(test.refundLocktime))

			if len(test.extraData) > 0 {
				var extraData [32]byte
				copy(extraData[:], hexToSlice(test.extraData))
				d.ExtraData = &extraData
			}

			script, err := d.Script()
			if err != nil {
				t.Fatal(err)
			}

			expectedScript := hexToSlice(test.expectedScript)

			testutils.AssertBytesEqual(t, expectedScript, script)
		})
	}
}

func TestDeposit_TaprootRefundScript(t *testing.T) {
	hexToSlice := func(hexString string) []byte {
		bytes, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatalf("error while converting [%v]: [%v]", hexString, err)
		}
		return bytes
	}

	var tests = map[string]struct {
		extraData            string
		expectedScript       string
		expectedMerkleRoot   string
		expectedTaprootKey   string
		expectedOutputScript string
	}{
		"no extra data": {
			extraData: "",
			expectedScript: "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377508" +
				"f9f0c90d00039523750460bcea61b1752011223344556677889900aabb" +
				"ccddeeff00112233445566778899aabbccddeeffac",
			expectedMerkleRoot: "3d6f9a2fea1de0a6c260d1fbc0343c9b2ed84307e6a7" +
				"231139b78438448ee8c0",
			expectedTaprootKey: "90e7ce2b6cd476b7a1c2c7f6585c3fd0eae4379a508e" +
				"981ed422b3e28b9ae8c2",
			expectedOutputScript: "512090e7ce2b6cd476b7a1c2c7f6585c3fd0eae4379" +
				"a508e981ed422b3e28b9ae8c2",
		},
		"with extra data": {
			extraData: "a9b38ea6435c8941d6eda6a46b68e3e2117196995bd154ab55" +
				"196396b03d9bda",
			expectedScript: "14934b98637ca318a4d6e7ca6ffd1690b8e77df6377520" +
				"a9b38ea6435c8941d6eda6a46b68e3e2117196995bd154ab55196396" +
				"b03d9bda7508f9f0c90d00039523750460bcea61b175201122334455" +
				"6677889900aabbccddeeff00112233445566778899aabbccddeeffac",
			expectedMerkleRoot: "6968648895261db4f667ff977b3bbd9b4684fe756050" +
				"894b092fd0e24e24f90f",
			expectedTaprootKey: "b57ad22351a7a074b6588836d08fbecae35b61ef9eeb" +
				"35376a1c5f3d6049376e",
			expectedOutputScript: "5120b57ad22351a7a074b6588836d08fbecae35b61ef" +
				"9eeb35376a1c5f3d6049376e",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			d := new(Deposit)
			d.Depositor = chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637")
			copy(d.BlindingFactor[:], hexToSlice("f9f0c90d00039523"))
			copy(d.WalletPublicKeyHash[:], hexToSlice("c92a772f11bc97d8938a16a9db435401f4e6a7bc"))
			copy(d.RefundPublicKeyHash[:], hexToSlice("c2a27a88d8d03e271e8edc556923e9398619f17c"))
			copy(d.RefundLocktime[:], hexToSlice("60bcea61"))

			var walletXOnlyPublicKey [32]byte
			copy(
				walletXOnlyPublicKey[:],
				hexToSlice("2336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008"),
			)
			d.WalletXOnlyPublicKey = &walletXOnlyPublicKey

			var refundXOnlyPublicKey [32]byte
			copy(
				refundXOnlyPublicKey[:],
				hexToSlice("11223344556677889900aabbccddeeff00112233445566778899aabbccddeeff"),
			)
			d.RefundXOnlyPublicKey = &refundXOnlyPublicKey

			if len(test.extraData) > 0 {
				var extraData [32]byte
				copy(extraData[:], hexToSlice(test.extraData))
				d.ExtraData = &extraData
			}

			refundScript, err := d.TaprootRefundScript()
			if err != nil {
				t.Fatal(err)
			}
			testutils.AssertBytesEqual(t, hexToSlice(test.expectedScript), refundScript)

			merkleRoot, err := d.TaprootMerkleRoot()
			if err != nil {
				t.Fatal(err)
			}
			testutils.AssertBytesEqual(t, hexToSlice(test.expectedMerkleRoot), merkleRoot[:])

			outputScript, err := bitcoin.PayToTaprootWithScriptTree(
				*d.WalletXOnlyPublicKey,
				merkleRoot,
			)
			if err != nil {
				t.Fatal(err)
			}
			testutils.AssertBytesEqual(
				t,
				hexToSlice(test.expectedOutputScript),
				outputScript,
			)

			outputKey, err := bitcoin.TaprootOutputKey(
				*d.WalletXOnlyPublicKey,
				&merkleRoot,
			)
			if err != nil {
				t.Fatal(err)
			}
			testutils.AssertBytesEqual(t, hexToSlice(test.expectedTaprootKey), outputKey[:])
		})
	}
}
