package frost

import (
	"encoding/hex"
	"testing"
)

func TestWalletPublicKeyHashCompatibilityAlias(t *testing.T) {
	outputKeyHex := "11223344556677889900aabbccddeeff00112233445566778899aabbccddeeff"
	expectedAliasHex := "c2a27a88d8d03e271e8edc556923e9398619f17c"

	outputKeyBytes, err := hex.DecodeString(outputKeyHex)
	if err != nil {
		t.Fatalf("failed to decode output key: [%v]", err)
	}

	var outputKey OutputKey
	copy(outputKey[:], outputKeyBytes)

	actualAlias := WalletPublicKeyHashCompatibilityAlias(outputKey)
	actualAliasHex := hex.EncodeToString(actualAlias[:])

	if actualAliasHex != expectedAliasHex {
		t.Fatalf(
			"unexpected alias\nactual:   [%s]\nexpected: [%s]",
			actualAliasHex,
			expectedAliasHex,
		)
	}
}

func TestSignatureSerialize(t *testing.T) {
	signature := &Signature{}
	signature.R = [SignatureComponentSize]byte{0x01, 0x02, 0x03}
	signature.S = [SignatureComponentSize]byte{0xaa, 0xbb, 0xcc}

	serialized := signature.Serialize()

	if serialized[0] != 0x01 || serialized[1] != 0x02 || serialized[2] != 0x03 {
		t.Fatalf("unexpected R serialization")
	}

	if serialized[SignatureComponentSize] != 0xaa ||
		serialized[SignatureComponentSize+1] != 0xbb ||
		serialized[SignatureComponentSize+2] != 0xcc {
		t.Fatalf("unexpected S serialization")
	}
}

func TestSignatureString(t *testing.T) {
	signature := &Signature{
		R: [SignatureComponentSize]byte{0x01, 0x02},
		S: [SignatureComponentSize]byte{0x0a, 0x0b},
	}

	expected := "R: 0x0102000000000000000000000000000000000000000000000000000000000000, S: 0x0a0b000000000000000000000000000000000000000000000000000000000000"

	if signature.String() != expected {
		t.Fatalf(
			"unexpected signature string\nactual:   [%s]\nexpected: [%s]",
			signature.String(),
			expected,
		)
	}
}
