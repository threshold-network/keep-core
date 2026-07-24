package signing

import (
	"encoding/json"
	"errors"
	"fmt"
)

// The FFI CONTRACT version this bridge requires from libfrost_tbtc (the Rust
// frost_tbtc_abi_version export / api::FrostTbtcAbiVersionResult). The bridge requires
// the lib's abi_major to MATCH exactly - a higher major broke something this bridge
// does not know, a lower major is too old - and the lib's abi_minor to be AT LEAST the
// minor this bridge uses (additive features are backward-compatible; a higher minor is
// fine and its extras are ignored). Bump these in lockstep with the Rust constants, in
// the SAME PR that bumps ci/frost-signer-pin.env to the lib commit that provides them.
const (
	// Major 3: BuildTaprootTx now requires every input's spent-output
	// scriptPubKey and returns the ordered BIP-341 SIGHASH_DEFAULT key-spend
	// messages. The required request field and changed response semantics are an
	// incompatible JSON/crypto-contract change, so an ABI-2 library must not be
	// linked by this bridge.
	requiredTBTCSignerABIMajor uint32 = 3
	// Minor 1 adds the typed heartbeat signing intent required to authorize
	// non-transaction messages while the signing-policy firewall is on. Minor 2
	// adds the heartbeat rate-limit configuration and dedicated rejection metric.
	// Minor 3 adds the canary-evidence configuration. Minor 4 adds descriptor-
	// bound durable-store identity. Minor 5 adds exact retained key-package
	// inventory plus the append-only state-witness proof required to detect
	// rollback of key packages and durable replay markers.
	requiredTBTCSignerABIMinMinor uint32 = 5
)

// ErrTBTCSignerABIIncompatible marks a linked libfrost_tbtc whose FFI contract version
// is incompatible with this build. It is fatal: the engine fails closed rather than
// risk a silently misinterpreted struct/JSON contract.
var ErrTBTCSignerABIIncompatible = errors.New(
	"linked libfrost_tbtc FFI contract version is incompatible with this build",
)

// parseTBTCSignerABIVersion decodes the frozen {abi_major, abi_minor} root compatibility
// surface from the lib's frost_tbtc_abi_version response and rejects anything malformed
// as incompatible. BOTH fields are required: pointer fields distinguish "absent" from a
// legitimate zero, because Go's json.Unmarshal silently zero-fills a missing field.
// Both fields remain mandatory even when that zero would fail today's minimum: accepting
// a partial version response would weaken the frozen compatibility contract. Extra/unknown
// fields are tolerated by design (an additive minor bump may add fields old bridges ignore).
// Pure (no cgo), so it is unit-tested in the default build.
func parseTBTCSignerABIVersion(payload []byte) (major, minor uint32, err error) {
	var decoded struct {
		AbiMajor *uint32 `json:"abi_major"`
		AbiMinor *uint32 `json:"abi_minor"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return 0, 0, fmt.Errorf(
			"%w: malformed FFI contract version response: %v",
			ErrTBTCSignerABIIncompatible, err,
		)
	}
	if decoded.AbiMajor == nil || decoded.AbiMinor == nil {
		return 0, 0, fmt.Errorf(
			"%w: FFI contract version response is missing abi_major and/or abi_minor",
			ErrTBTCSignerABIIncompatible,
		)
	}
	return *decoded.AbiMajor, *decoded.AbiMinor, nil
}

// checkTBTCSignerABICompatibility applies the compatibility rule to a lib-reported FFI
// contract version against this build's required version. Pure (no cgo) so the rule is
// unit-tested in the default build.
func checkTBTCSignerABICompatibility(libMajor, libMinor uint32) error {
	return checkABIContractCompatibility(
		libMajor, libMinor, requiredTBTCSignerABIMajor, requiredTBTCSignerABIMinMinor,
	)
}

// checkABIContractCompatibility is the rule, parameterized over the required version so
// every branch (wrong major either direction, too-old minor, higher-minor-ok) is
// testable independent of the current constants: lib major must equal the required
// major, and lib minor must be >= the required minimum minor.
func checkABIContractCompatibility(libMajor, libMinor, reqMajor, reqMinMinor uint32) error {
	if libMajor != reqMajor {
		return fmt.Errorf(
			"%w: lib reports abi_major %d, this build requires exactly %d",
			ErrTBTCSignerABIIncompatible, libMajor, reqMajor,
		)
	}
	if libMinor < reqMinMinor {
		return fmt.Errorf(
			"%w: lib reports abi_minor %d, this build requires at least %d (major %d)",
			ErrTBTCSignerABIIncompatible, libMinor, reqMinMinor, reqMajor,
		)
	}
	return nil
}
