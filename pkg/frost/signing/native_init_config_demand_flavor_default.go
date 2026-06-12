//go:build !frost_native

package signing

// buildHasNativeFROSTRegistration reports whether this build flavor runs
// native FROST registration at init time. Used only to pick the precise
// fatal message when an init-config demand cannot be honored.
const buildHasNativeFROSTRegistration = false
