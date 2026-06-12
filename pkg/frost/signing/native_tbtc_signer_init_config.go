package signing

// TBTCSignerInitConfigPathEnv optionally points at a JSON file holding the
// tbtc-signer init-time operational configuration. When set, the
// configuration is installed via frost_tbtc_init_signer_config during native
// FROST engine registration, BEFORE any other signer call; a read, parse,
// validation, or symbol-availability failure fails the registration closed.
// When unset, the signer falls back to reading TBTC_SIGNER_* from the
// process environment (the transitional path).
//
// The JSON schema is owned by the Rust signer (InitSignerConfigRequest in
// pkg/tbtc/signer/src/api.rs): field names are the lowercased TBTC_SIGNER_*
// suffixes, unknown fields are rejected, and once installed the process
// environment is ignored wholesale for covered knobs. Secrets never ride
// this channel: TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX stays on the dedicated
// env/command key-provider path. The file may carry the state_key_command
// execution spec, so restrict its permissions to the operator account
// (e.g. 0600), as with the signer state path.
const TBTCSignerInitConfigPathEnv = "TBTC_SIGNER_INIT_CONFIG_PATH"

// NativeTBTCSignerInitConfigResult captures the response of an init-time
// signer-config installation (frost_tbtc_init_signer_config).
type NativeTBTCSignerInitConfigResult struct {
	Installed          bool   `json:"installed"`
	Idempotent         bool   `json:"idempotent"`
	ConfigFingerprint  string `json:"config_fingerprint"`
	ConfiguredKeyCount uint32 `json:"configured_key_count"`
}
