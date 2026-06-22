//go:build frost_native && frost_tbtc_signer && cgo

package signing

import "errors"

// tbtcSignerABIIncompatibility reports a PRESENT-but-incompatible linked libfrost_tbtc
// (a wrong or malformed FFI contract version) so the coarse engine path can fail closed
// BEFORE its legacy fallback can mask it. It returns nil for a MISSING/absent lib
// (ErrNativeCryptographyUnavailable, not ErrTBTCSignerABIIncompatible): that case is the
// intended transitional "no engine -> legacy" fallback, not an incompatibility. The
// underlying preflight is cached (sync.Once), so this is cheap to call per signing.
func tbtcSignerABIIncompatibility() error {
	if err := ensureTBTCSignerABICompatible(); errors.Is(err, ErrTBTCSignerABIIncompatible) {
		return err
	}
	return nil
}
