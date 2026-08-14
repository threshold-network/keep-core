//go:build frost_native

package signing

import "encoding/json"

func unsupportedUniFFIV2Payload(t testFataler, verifyingKey string) []byte {
	t.Helper()

	payload, err := json.Marshal(&struct {
		KeyPackage       *NativeFROSTKeyPackage       `json:"keyPackage"`
		PublicKeyPackage *NativeFROSTPublicKeyPackage `json:"publicKeyPackage"`
	}{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: verifyingKey,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return payload
}

type testFataler interface {
	Helper()
	Fatalf(format string, args ...any)
}
