package tbtc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestFrostDurableSessionStoreBindingAcceptsStableRestartIdentity(
	t *testing.T,
) {
	identity := testFrostDurableSessionStoreIdentity()
	reads := 0
	binding, err := newFrostDurableSessionStoreBinding(
		frostActivationHex32(identity.Fingerprint),
		func() (*signing.NativeTBTCSignerDurableStoreIdentity, error) {
			reads++
			restartedReadback := *identity
			return &restartedReadback, nil
		},
	)
	if err != nil {
		t.Fatalf("cannot bind stable signer store: [%v]", err)
	}
	if _, err := binding.verify(); err != nil {
		t.Fatalf("stable signer-store identity changed across restart: [%v]", err)
	}
	if reads != 2 {
		t.Fatalf("expected startup and restart readbacks, got [%d]", reads)
	}
}

func TestFrostDurableSessionStoreBindingRejectsRuntimeIdentityMismatch(
	t *testing.T,
) {
	original := testFrostDurableSessionStoreIdentity()
	tests := map[string]func(*signing.NativeTBTCSignerDurableStoreIdentity){
		"wrong path": func(identity *signing.NativeTBTCSignerDurableStoreIdentity) {
			identity.CanonicalPathFingerprint[0] ^= 0xff
		},
		"wrong backend": func(identity *signing.NativeTBTCSignerDurableStoreIdentity) {
			identity.Backend = "encrypted-database-v1"
		},
		"replacement": func(identity *signing.NativeTBTCSignerDurableStoreIdentity) {
			identity.FilesystemFingerprint[0] ^= 0xff
			identity.LockFingerprint[0] ^= 0xff
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			actual := *original
			mutate(&actual)
			fingerprint, err := signing.ComputeNativeTBTCSignerDurableStoreFingerprint(&actual)
			if err != nil {
				t.Fatal(err)
			}
			actual.Fingerprint = fingerprint

			_, err = newFrostDurableSessionStoreBinding(
				frostActivationHex32(original.Fingerprint),
				func() (*signing.NativeTBTCSignerDurableStoreIdentity, error) {
					return &actual, nil
				},
			)
			if err == nil || !strings.Contains(err.Error(), "signed activation manifest") {
				t.Fatalf("expected manifest binding failure, got [%v]", err)
			}
		})
	}
}

func TestFrostDurableSessionStoreBindingRejectsReadbackFailureAndWrongFingerprint(
	t *testing.T,
) {
	identity := testFrostDurableSessionStoreIdentity()
	_, err := newFrostDurableSessionStoreBinding(
		frostActivationHex32(identity.Fingerprint),
		func() (*signing.NativeTBTCSignerDurableStoreIdentity, error) {
			return nil, fmt.Errorf("native identity symbol unavailable")
		},
	)
	if err == nil || !strings.Contains(err.Error(), "native identity symbol unavailable") {
		t.Fatalf("expected native readback failure, got [%v]", err)
	}

	inconsistent := *identity
	inconsistent.Fingerprint[0] ^= 0xff
	_, err = newFrostDurableSessionStoreBinding(
		frostActivationHex32(identity.Fingerprint),
		func() (*signing.NativeTBTCSignerDurableStoreIdentity, error) {
			return &inconsistent, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "self-inconsistent") {
		t.Fatalf("expected self-inconsistent identity failure, got [%v]", err)
	}
}

func testFrostDurableSessionStoreIdentity() *signing.NativeTBTCSignerDurableStoreIdentity {
	identity := &signing.NativeTBTCSignerDurableStoreIdentity{
		Schema:                   signing.NativeTBTCSignerDurableStoreIdentitySchema,
		Backend:                  "encrypted-file-v1",
		StoreID:                  [32]byte{0x31},
		CanonicalPathFingerprint: [32]byte{0x32},
		FilesystemFingerprint:    [32]byte{0x33},
		LockFingerprint:          [32]byte{0x34},
	}
	fingerprint, err := signing.ComputeNativeTBTCSignerDurableStoreFingerprint(identity)
	if err != nil {
		panic(err)
	}
	identity.Fingerprint = fingerprint
	return identity
}

func testFrostDurableSessionStoreBinding(t *testing.T) *frostDurableSessionStoreBinding {
	t.Helper()
	identity := testFrostDurableSessionStoreIdentity()
	binding, err := newFrostDurableSessionStoreBinding(
		frostActivationHex32(identity.Fingerprint),
		func() (*signing.NativeTBTCSignerDurableStoreIdentity, error) {
			readback := *identity
			return &readback, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}
