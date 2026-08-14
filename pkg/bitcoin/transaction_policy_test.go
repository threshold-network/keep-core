package bitcoin

import "testing"

func TestIsDustOutput(t *testing.T) {
	p2wpkh, err := PayToWitnessPublicKeyHash([20]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	p2tr, err := PayToTaproot([32]byte{0x02})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		output *TransactionOutput
		dust   bool
	}{
		"nil": {
			output: nil,
			dust:   true,
		},
		"P2WPKH below threshold": {
			output: &TransactionOutput{Value: 293, PublicKeyScript: p2wpkh},
			dust:   true,
		},
		"P2WPKH at threshold": {
			output: &TransactionOutput{Value: 294, PublicKeyScript: p2wpkh},
			dust:   false,
		},
		"P2TR below threshold": {
			output: &TransactionOutput{Value: 329, PublicKeyScript: p2tr},
			dust:   true,
		},
		"P2TR at threshold": {
			output: &TransactionOutput{Value: 330, PublicKeyScript: p2tr},
			dust:   false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if actual := IsDustOutput(test.output); actual != test.dust {
				t.Fatalf("unexpected dust result [%v]", actual)
			}
		})
	}
}
