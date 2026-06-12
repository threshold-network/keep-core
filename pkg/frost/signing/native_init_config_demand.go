package signing

import (
	"os"
	"strings"
)

// fatalNativeRegistrationExit terminates the process after logging the
// formatted message at fatal level. It is a package-level variable only so
// the demand-enforcement tests can observe the abort instead of dying with
// the test binary; production code must never override it.
var fatalNativeRegistrationExit = func(format string, args ...interface{}) {
	registrationLogger.Fatalf(format, args...)
}

// enforceNativeInitConfigDemand terminates the process when the operator
// demanded config-mode FROST operation (TBTC_SIGNER_INIT_CONFIG_PATH set)
// and this binary did not bring up the FROST-native engine. With the path
// unset this is a no-op and the registration layer keeps its safe-by-default
// posture: failures degrade to the legacy bridge with a warning.
//
// Decision (2026-06-12, recorded in the Phase 5 gates-doc Decision Log):
// setting the path is an explicit demand, so ANY state in which the demand
// is unmet is process-fatal, in every profile and environment, covering the
// whole failure family:
//
//   - the config-install leg failed (unreadable file, parse/validation
//     rejection, init-time policy or attestation-gate failure, or a loaded
//     signer library that predates frost_tbtc_init_signer_config),
//   - the engine-registration leg failed after a successful install,
//   - the binary cannot honor the demand at all (built without the
//     frost_native build tag, so no native registration ever runs).
//
// Fatality is deliberately NOT conditional on the configured profile: an
// unreadable config file cannot reveal its profile, and the signer treats a
// missing profile as production (production-by-omission), so the only
// non-circular rule is to enforce whenever the path is set. Uniform
// semantics also mean testnet rehearses exactly the behavior production
// will have.
//
// The checks are positive (registered state), not merely error-presence:
// LastNativeRegistrationError is reset by later registration legs, so the
// absence of a recorded error does not prove the engine came up.
func enforceNativeInitConfigDemand() {
	configPath := strings.TrimSpace(os.Getenv(TBTCSignerInitConfigPathEnv))
	if configPath == "" {
		return
	}

	if err := LastNativeRegistrationError(); err != nil {
		fatalNativeRegistrationExit(
			"%s is set [%s]: config-mode FROST operation is demanded but "+
				"native registration failed: [%v]; terminating instead of "+
				"continuing on the legacy bridge (unset %s to run in the "+
				"transitional env-fallback mode)",
			TBTCSignerInitConfigPathEnv,
			configPath,
			err,
			TBTCSignerInitConfigPathEnv,
		)
		return
	}

	executionBackendMutex.RLock()
	adapterRegistered := nativeExecutionAdapter != nil
	executorRegistered := nativeExecutionFFIExecutor != nil
	executionBackendMutex.RUnlock()

	if adapterRegistered && executorRegistered {
		return
	}

	if !buildHasNativeFROSTRegistration {
		fatalNativeRegistrationExit(
			"%s is set [%s]: config-mode FROST operation is demanded but "+
				"this binary was built without the frost_native build tag "+
				"and cannot honor it; terminating (deploy a frost_native "+
				"binary, or unset %s to run this one)",
			TBTCSignerInitConfigPathEnv,
			configPath,
			TBTCSignerInitConfigPathEnv,
		)
		return
	}

	// No recorded error here can mean an earlier leg's failure was
	// overwritten by a later leg's success, so point at the warnings
	// emitted at failure time instead of claiming a cause.
	fatalNativeRegistrationExit(
		"%s is set [%s]: config-mode FROST operation is demanded but "+
			"native registration did not complete (native adapter "+
			"registered [%v], native FFI executor registered [%v]); "+
			"terminating instead of continuing on the legacy bridge - "+
			"check the registration warnings logged above for the cause",
		TBTCSignerInitConfigPathEnv,
		configPath,
		adapterRegistered,
		executorRegistered,
	)
}
