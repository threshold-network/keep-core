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
		"wrong canonical path": func(identity *NativeTBTCSignerDurableStoreIdentity) {
			identity.CanonicalPathFingerprint[0] ^= 0xff
		},
		"wrong filesystem": func(identity *NativeTBTCSignerDurableStoreIdentity) {
			identity.FilesystemFingerprint[0] ^= 0xff
		},
		"replaced lock": func(identity *NativeTBTCSignerDurableStoreIdentity) {
			identity.LockFingerprint[0] ^= 0xff
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
