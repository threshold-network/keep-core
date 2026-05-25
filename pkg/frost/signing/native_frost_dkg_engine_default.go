//go:build !frost_native

package signing

import "fmt"

// NativeFROSTDKGEngine is unavailable in non-frost_native builds.
type NativeFROSTDKGEngine interface{}

// RegisterNativeFROSTDKGEngine fails closed when native FROST DKG is not linked
// into the current build.
func RegisterNativeFROSTDKGEngine(engine NativeFROSTDKGEngine) error {
	return fmt.Errorf("native FROST DKG engine is unavailable in this build")
}

// UnregisterNativeFROSTDKGEngine is a no-op in non-frost_native builds.
func UnregisterNativeFROSTDKGEngine() {}
