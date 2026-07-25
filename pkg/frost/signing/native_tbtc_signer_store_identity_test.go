package signing

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestDecodeNativeTBTCSignerDurableStoreIdentity(t *testing.T) {
	identity := testNativeTBTCSignerDurableStoreIdentity(t)
	payload := testNativeTBTCSignerDurableStoreIdentityPayload(identity, true, true)

	decoded, err := DecodeNativeTBTCSignerDurableStoreIdentity(payload)
	if err != nil {
		t.Fatalf("cannot decode valid durable store identity: [%v]", err)
	}
	if *decoded != *identity {
		t.Fatalf("unexpected decoded identity\nexpected: [%+v]\nactual:   [%+v]", identity, decoded)
	}
}

func TestDecodeNativeTBTCSignerDurableStoreIdentityRejectsUnboundIdentity(
	t *testing.T,
) {
	tests := map[string]func(*NativeTBTCSignerDurableStoreIdentity){
		"wrong backend": func(identity *NativeTBTCSignerDurableStoreIdentity) {
			identity.Backend = "different-backend"
		},
		"wrong store ID": func(identity *NativeTBTCSignerDurableStoreIdentity) {
			identity.StoreID[0] ^= 0xff
		},
		"wrong fingerprint": func(identity *NativeTBTCSignerDurableStoreIdentity) {
			identity.Fingerprint[0] ^= 0xff
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			identity := testNativeTBTCSignerDurableStoreIdentity(t)
			mutate(identity)
			_, err := DecodeNativeTBTCSignerDurableStoreIdentity(
				testNativeTBTCSignerDurableStoreIdentityPayload(identity, true, true),
			)
			if err == nil || !strings.Contains(err.Error(), "does not bind") {
				t.Fatalf("expected identity-binding failure, got [%v]", err)
			}
		})
	}
}

func TestDecodeNativeTBTCSignerDurableStoreIdentityDoesNotBindVolatileDiagnostics(
	t *testing.T,
) {
	identity := testNativeTBTCSignerDurableStoreIdentity(t)
	identity.CanonicalPathFingerprint[0] ^= 0xff
	identity.FilesystemFingerprint[0] ^= 0xff
	identity.LockFingerprint[0] ^= 0xff

	decoded, err := DecodeNativeTBTCSignerDurableStoreIdentity(
		testNativeTBTCSignerDurableStoreIdentityPayload(identity, true, true),
	)
	if err != nil {
		t.Fatalf("v2 identity rejected changed diagnostic descriptors: [%v]", err)
	}
	if decoded.Fingerprint != identity.Fingerprint {
		t.Fatal("v2 stable store fingerprint changed with diagnostic descriptors")
	}
}

func TestComputeNativeTBTCSignerDurableStoreFingerprintMatchesRustV2Vectors(
	t *testing.T,
) {
	tests := []struct {
		storeID  byte
		expected string
	}{
		{
			0x11,
			"8bb8d21c69e78916e8f165b0c861c0d84c5d7af5393f75b0321fe048f772abba",
		},
		{
			0x24,
			"52fcbfc4b2c6a93645106a32c62113192cac30b934b905e1ad357792c4ce8628",
		},
	}

	for _, test := range tests {
		identity := &NativeTBTCSignerDurableStoreIdentity{
			Schema:                   "tbtc-signer-durable-session-store-identity/v2",
			Backend:                  "encrypted-file-v1",
			StoreID:                  repeatedNativeTBTCSignerBytes32(test.storeID),
			CanonicalPathFingerprint: [32]byte{0x01},
			FilesystemFingerprint:    [32]byte{0x02},
			LockFingerprint:          [32]byte{0x03},
		}
		actual, err := ComputeNativeTBTCSignerDurableStoreFingerprint(identity)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(actual[:]) != test.expected {
			t.Fatalf(
				"unexpected v2 store fingerprint for store ID 0x%02x: [%x]",
				test.storeID,
				actual,
			)
		}
	}
}

func TestNativeTBTCSignerStateWitnessChainMatchesRustV2Vector(t *testing.T) {
	identity := &NativeTBTCSignerDurableStoreIdentity{
		Schema:                   NativeTBTCSignerDurableStoreIdentitySchema,
		Backend:                  "encrypted-file-v1",
		StoreID:                  repeatedNativeTBTCSignerBytes32(0x11),
		CanonicalPathFingerprint: [32]byte{0x01},
		FilesystemFingerprint:    [32]byte{0x02},
		LockFingerprint:          [32]byte{0x03},
	}
	fingerprint, err := ComputeNativeTBTCSignerDurableStoreFingerprint(identity)
	if err != nil {
		t.Fatal(err)
	}
	genesis := ComputeNativeTBTCSignerStateWitnessGenesis(fingerprint)
	const expectedGenesis = "3179b8bc6614b0951b703f9c418b17cf7cd8b7f1bef1f86587385d4c150efab2"
	if hex.EncodeToString(genesis[:]) != expectedGenesis {
		t.Fatalf("unexpected derived state-witness genesis: [%x]", genesis)
	}

	commitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		fingerprint,
		1,
		genesis,
		repeatedNativeTBTCSignerBytes32(0x33),
	)
	const expectedCommitment = "5387626d5314b17b324f9a7df1ab16fcbf10917a137527bf33c71847e1b77da0"
	if hex.EncodeToString(commitment[:]) != expectedCommitment {
		t.Fatalf("unexpected derived state-witness commitment: [%x]", commitment)
	}
}

func TestDecodeNativeTBTCSignerDurableStoreIdentityRejectsUnsafePathState(
	t *testing.T,
) {
	identity := testNativeTBTCSignerDurableStoreIdentity(t)
	for name, payload := range map[string][]byte{
		"symlink": testNativeTBTCSignerDurableStoreIdentityPayload(
			identity,
			false,
			true,
		),
		"replacement": testNativeTBTCSignerDurableStoreIdentityPayload(
			identity,
			true,
			false,
		),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeNativeTBTCSignerDurableStoreIdentity(payload); err == nil {
				t.Fatal("expected unsafe store path state to be rejected")
			}
		})
	}
}

func repeatedNativeTBTCSignerBytes32(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value
	}
	return result
}

func testNativeTBTCSignerDurableStoreIdentity(
	t *testing.T,
) *NativeTBTCSignerDurableStoreIdentity {
	t.Helper()
	identity := &NativeTBTCSignerDurableStoreIdentity{
		Schema:                   NativeTBTCSignerDurableStoreIdentitySchema,
		Backend:                  "encrypted-file-v1",
		StoreID:                  [32]byte{0x01},
		CanonicalPathFingerprint: [32]byte{0x02},
		FilesystemFingerprint:    [32]byte{0x03},
		LockFingerprint:          [32]byte{0x04},
	}
	fingerprint, err := ComputeNativeTBTCSignerDurableStoreFingerprint(identity)
	if err != nil {
		t.Fatal(err)
	}
	identity.Fingerprint = fingerprint
	return identity
}

func testNativeTBTCSignerDurableStoreIdentityPayload(
	identity *NativeTBTCSignerDurableStoreIdentity,
	symlinkFree bool,
	replacementProtected bool,
) []byte {
	hex32 := func(value [32]byte) string {
		return "0x" + hex.EncodeToString(value[:])
	}
	return []byte(fmt.Sprintf(`{
        "schema":%q,
        "backend":%q,
        "store_id":%q,
        "canonical_path_fingerprint":%q,
        "filesystem_fingerprint":%q,
        "lock_fingerprint":%q,
        "fingerprint":%q,
        "durable":true,
        "exclusive_lock_held":true,
        "symlink_free":%t,
        "replacement_protected":%t
    }`,
		identity.Schema,
		identity.Backend,
		hex32(identity.StoreID),
		hex32(identity.CanonicalPathFingerprint),
		hex32(identity.FilesystemFingerprint),
		hex32(identity.LockFingerprint),
		hex32(identity.Fingerprint),
		symlinkFree,
		replacementProtected,
	))
}
