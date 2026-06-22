package signing

import (
	"errors"
	"testing"
)

func TestCheckABIContractCompatibility(t *testing.T) {
	// req = major 1, min minor 2, to exercise every branch (the too-old-minor branch is
	// unreachable against the real requiredTBTCSignerABIMinMinor of 0).
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

func TestCheckTBTCSignerABICompatibility_CurrentContract(t *testing.T) {
	// Pins the bridge's current required contract (major 1, min minor 0): the matching
	// lib version is compatible; a different major is not. A regression here means the
	// required constants drifted from what the bridge actually speaks.
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, requiredTBTCSignerABIMinMinor); err != nil {
		t.Fatalf("the required contract version must be self-compatible: %v", err)
	}
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor+1, requiredTBTCSignerABIMinMinor); err == nil {
		t.Fatal("a higher major must be incompatible")
	}
	// Any minor >= the required minimum is accepted (additive).
	if err := checkTBTCSignerABICompatibility(requiredTBTCSignerABIMajor, requiredTBTCSignerABIMinMinor+3); err != nil {
		t.Fatalf("a higher minor must be accepted: %v", err)
	}
}
