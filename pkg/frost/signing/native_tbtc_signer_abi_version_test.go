package signing

import (
	"errors"
	"testing"
)

func TestCheckABIContractCompatibility(t *testing.T) {
	// req = major 1, min minor 2, to exercise every branch (the too-old-minor branch is
	// kept parameterized so every compatibility branch remains explicit.
	const reqMajor, reqMinMinor = uint32(1), uint32(2)
	tests := []struct {
		name           string
		libMajor       uint32
		libMinor       uint32
		wantCompatible bool
	}{
		{"exact match", 1, 2, true},
		{"higher minor is additive-compatible", 1, 9, true},
		{"minor too old", 1, 1, false},
		{"major too high (lib broke something newer)", 2, 0, false},
		{"major too low (lib too old)", 0, 9, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkABIContractCompatibility(tt.libMajor, tt.libMinor, reqMajor, reqMinMinor)
			if tt.wantCompatible && err != nil {
				t.Fatalf("expected compatible, got error: %v", err)
			}
			if !tt.wantCompatible {
				if err == nil {
					t.Fatal("expected incompatibility error, got nil")
				}
				if !errors.Is(err, ErrTBTCSignerABIIncompatible) {
					t.Fatalf("error must wrap ErrTBTCSignerABIIncompatible: %v", err)
				}
			}
		})
	}
}

func TestParseTBTCSignerABIVersion(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		major, minor, err := parseTBTCSignerABIVersion([]byte(`{"abi_major":2,"abi_minor":3}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if major != 2 || minor != 3 {
			t.Fatalf("got (%d,%d), want (2,3)", major, minor)
		}
	})

	// Both fields are REQUIRED: a missing field must be rejected, not silently
	// zero-filled into a partial compatibility response.
	rejected := map[string]string{
		"missing abi_minor": `{"abi_major":1}`,
		"missing abi_major": `{"abi_minor":0}`,
		"empty object":      `{}`,
		"malformed json":    `{"abi_major":1,`,
		"not an object":     `42`,
	}
	for name, payload := range rejected {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseTBTCSignerABIVersion([]byte(payload))
			if err == nil {
				t.Fatalf("payload %q must be rejected", payload)
			}
			if !errors.Is(err, ErrTBTCSignerABIIncompatible) {
				t.Fatalf("rejection must wrap ErrTBTCSignerABIIncompatible: %v", err)
			}
		})
	}

	// A present zero minor with major present is VALID (abi 1.0 is a real version).
	t.Run("zero minor is valid when present", func(t *testing.T) {
		major, minor, err := parseTBTCSignerABIVersion([]byte(`{"abi_major":1,"abi_minor":0}`))
		if err != nil || major != 1 || minor != 0 {
			t.Fatalf("got (%d,%d,%v), want (1,0,nil)", major, minor, err)
		}
	})

	// Extra/unknown fields are tolerated (additive minor may add fields old bridges ignore).
	t.Run("extra fields tolerated", func(t *testing.T) {
		major, minor, err := parseTBTCSignerABIVersion([]byte(`{"abi_major":1,"abi_minor":0,"future":"x"}`))
		if err != nil || major != 1 || minor != 0 {
			t.Fatalf("got (%d,%d,%v), want (1,0,nil)", major, minor, err)
		}
	})
}

func TestCheckTBTCSignerABICompatibility_CurrentContract(t *testing.T) {
	// Pins the bridge's current required contract: major 4 moves the durable-store
	// identity schema and the state-witness transcript to v2, so that state
	// commitments bind only the stable `.store-id` and no longer break when a
	// benign filesystem change alters the lock file, directory inode, or device.
	// Minor 3 adds the trust transition/head and bootstrap-facts surface used
	// before production signing can start. Minor 4 adds durable distributed-DKG
	// retirement. The matching library version is compatible; ABI 4.3 and a
	// different major are not.
	if requiredTBTCSignerABIMajor != 4 || requiredTBTCSignerABIMinMinor != 4 {
		t.Fatalf(
			"unexpected required tbtc-signer ABI: [%d.%d]",
			requiredTBTCSignerABIMajor,
			requiredTBTCSignerABIMinMinor,
		)
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, requiredTBTCSignerABIMinMinor); err != nil {
		t.Fatalf("the required contract version must be self-compatible: %v", err)
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, 0); err == nil {
		t.Fatal("ABI 4.0 without readiness readbacks must be incompatible")
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, 1); err == nil {
		t.Fatal("ABI 4.1 without the output-barrier tip/ack symbols must be incompatible")
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, 2); err == nil {
		t.Fatal("ABI 4.2 without trust transition and bootstrap-facts symbols must be incompatible")
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, 3); err == nil {
		t.Fatal("ABI 4.3 without distributed-DKG retirement must be incompatible")
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor+1, requiredTBTCSignerABIMinMinor); err == nil {
		t.Fatal("a higher major must be incompatible")
	}
	if err := checkTBTCSignerABICompatibility(2, 0); err == nil {
		t.Fatal("an ABI-2 signer without the BIP-341 transaction artifact must be incompatible")
	}
	// Any minor >= the required minimum is accepted (additive).
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, requiredTBTCSignerABIMinMinor+3); err != nil {
		t.Fatalf("a higher minor must be accepted: %v", err)
	}
}
