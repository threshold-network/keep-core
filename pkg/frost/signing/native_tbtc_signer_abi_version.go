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
	// Major 2: the coarse-FROST signing path and its FFI symbols
	// (frost_tbtc_run_dkg, frost_tbtc_sign_share, frost_tbtc_aggregate,
	// frost_tbtc_generate_nonces_and_commitments, frost_tbtc_start_sign_round,
	// frost_tbtc_finalize_sign_round) were removed. Dropping exported symbols is a
	// breaking ABI change, so the lib's abi_major moves 1 -> 2 in lockstep and this
	// bridge requires exactly 2: a lib still reporting major 1 exposes the retired
	// coarse contract and must not be linked.
	requiredTBTCSignerABIMajor uint32 = 2
	// Minor 0: frost_tbtc_persist_distributed_dkg_key_package - which the
	// distributed-DKG path calls - is baseline in major 2 (it was added in 1.1 and
	// carried forward), so the minimum minor for major 2 is 0. Additive minor bumps
	// on major 2 remain backward compatible and their extras are ignored.
	requiredTBTCSignerABIMinMinor uint32 = 0
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
// legitimate zero, because Go's json.Unmarshal silently zero-fills a missing field - and
// a missing abi_minor would otherwise default to 0 and pass the (>= 0) rule, letting a
// partial/broken lib bypass the fail-closed guard. Extra/unknown fields are tolerated by
// design (an additive minor bump may add fields old bridges ignore). Pure (no cgo), so
// it is unit-tested in the default build.
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
