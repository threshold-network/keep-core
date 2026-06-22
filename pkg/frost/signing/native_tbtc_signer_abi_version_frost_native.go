//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"errors"
	"fmt"
	"sync"
)

// assertTBTCSignerABICompatible fetches the linked lib's FFI contract version and
// applies the compatibility rule, failing closed. It distinguishes two cases:
//
//   - MISSING frost_tbtc_abi_version symbol (lib predates ABI versioning, or absent):
//     keeps ErrNativeCryptographyUnavailable in the chain - explicitly worded, but still
//     "unavailable", so the existing skip/require-cgo handling applies (a stale/absent
//     lib SKIPS in dev and is FATAL under the require-cgo gate, like any missing newer
//     symbol).
//   - PRESENT but wrong (malformed response, wrong major, too-old minor): a real,
//     responding-but-incompatible lib -> ErrTBTCSignerABIIncompatible, which fails loudly
//     ALWAYS (not skippable). A node must not sign through a lib whose contract diverges.
func assertTBTCSignerABICompatible() error {
	payload, err := callBuildTaggedTBTCSignerABIVersion()
	if err != nil {
		if errors.Is(err, ErrNativeCryptographyUnavailable) {
			return fmt.Errorf(
				"linked libfrost_tbtc is missing frost_tbtc_abi_version; it predates FFI "+
					"contract versioning or is absent (this build requires major %d, minor >= %d): %w",
				requiredTBTCSignerABIMajor, requiredTBTCSignerABIMinMinor, err,
			)
		}
		return fmt.Errorf(
			"%w: fetching the lib FFI contract version: %v",
			ErrTBTCSignerABIIncompatible, err,
		)
	}

	major, minor, err := parseTBTCSignerABIVersion(payload)
	if err != nil {
		return err
	}

	return checkTBTCSignerABICompatibility(major, minor)
}

var (
	tbtcSignerABIOnce sync.Once
	tbtcSignerABIErr  error
)

// ensureTBTCSignerABICompatible runs the ABI preflight ONCE per process and caches the
// verdict; every subsequent engine operation sees the same result. The library is
// process-global and not hot-swapped, so a single check before the first contract-
// sensitive call is sufficient and deterministic (a per-call check would add no safety).
func ensureTBTCSignerABICompatible() error {
	tbtcSignerABIOnce.Do(func() {
		tbtcSignerABIErr = assertTBTCSignerABICompatible()
	})
	return tbtcSignerABIErr
}

// resetTBTCSignerABIOnceForTest clears the cached preflight verdict so a test can
// re-run it. Test-only.
func resetTBTCSignerABIOnceForTest() {
	tbtcSignerABIOnce = sync.Once{}
	tbtcSignerABIErr = nil
}
