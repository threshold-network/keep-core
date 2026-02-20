package signing

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestFromTECDSASignature(t *testing.T) {
	signature := &tecdsa.Signature{
		R: big.NewInt(0x1234),
		S: big.NewInt(0xabcd),
	}

	result, err := FromTECDSASignature(signature)
	if err != nil {
		t.Fatalf("conversion failed: [%v]", err)
	}

	if result.R[30] != 0x12 || result.R[31] != 0x34 {
		t.Fatalf("unexpected R component bytes")
	}

	if result.S[30] != 0xab || result.S[31] != 0xcd {
		t.Fatalf("unexpected S component bytes")
	}
}

func TestFromTECDSASignature_ValidationErrors(t *testing.T) {
	testData := []struct {
		name      string
		signature *tecdsa.Signature
	}{
		{
			name:      "nil signature",
			signature: nil,
		},
		{
			name: "nil R",
			signature: &tecdsa.Signature{
				R: nil,
				S: big.NewInt(1),
			},
		},
		{
			name: "nil S",
			signature: &tecdsa.Signature{
				R: big.NewInt(1),
				S: nil,
			},
		},
		{
			name: "negative R",
			signature: &tecdsa.Signature{
				R: big.NewInt(-1),
				S: big.NewInt(1),
			},
		},
		{
			name: "negative S",
			signature: &tecdsa.Signature{
				R: big.NewInt(1),
				S: big.NewInt(-1),
			},
		},
	}

	for _, tc := range testData {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromTECDSASignature(tc.signature)
			if err == nil {
				t.Fatal("expected conversion error")
			}
		})
	}
}
