package signing

import (
	"encoding/json"
	"fmt"
	"sync"
)

// TBTCSignerInitConfigPathEnv optionally points at a JSON file holding the
// tbtc-signer init-time operational configuration. When set, the
// configuration is installed via frost_tbtc_init_signer_config during native
// FROST engine registration, BEFORE any other signer call; a read, parse,
// validation, or symbol-availability failure fails the registration closed
// and TERMINATES THE PROCESS at the end of registration, in every profile
// and build flavor (see enforceNativeInitConfigDemand for the decision
// record and the full failure family). When unset, the signer falls back to
// reading TBTC_SIGNER_* from the process environment (the transitional
// path), where registration failures degrade to the legacy bridge instead.
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

// NativeTBTCSignerInstalledStateAnchorConfig is the exact anchor-sensitive
// subset of the JSON successfully installed into Rust. Production activation
// compares it with the signed manifest instead of re-reading mutable
// environment variables or trusting a file that may have changed after init.
type NativeTBTCSignerInstalledStateAnchorConfig struct {
	BindingHash                     [32]byte
	ResponsePublicKey               [32]byte
	ResponsePublicKeySPKISHA256     [32]byte
	WitnessMaximumRecords           uint64
	WitnessRotationThresholdRecords uint64
	ConfigFingerprint               string
}

var nativeTBTCSignerInstalledStateAnchorConfig struct {
	sync.RWMutex
	value *NativeTBTCSignerInstalledStateAnchorConfig
}

func recordNativeTBTCSignerInstalledStateAnchorConfig(
	configJSON []byte,
	configFingerprint string,
) error {
	wire := struct {
		StateWitnessMaximumRecords             *uint64 `json:"state_witness_max_records"`
		StateAnchorBindingHash                 *string `json:"state_anchor_binding_hash"`
		StateAnchorResponsePublicKey           *string `json:"state_anchor_response_public_key"`
		StateAnchorResponsePublicKeySPKISHA256 *string `json:"state_anchor_response_public_key_spki_sha256"`
		StateWitnessRotationThresholdRecords   *uint64 `json:"state_witness_rotation_threshold_records"`
	}{}
	if err := json.Unmarshal(configJSON, &wire); err != nil {
		return fmt.Errorf("cannot decode installed signer anchor configuration: %w", err)
	}
	allAbsent := wire.StateWitnessMaximumRecords == nil &&
		wire.StateAnchorBindingHash == nil &&
		wire.StateAnchorResponsePublicKey == nil &&
		wire.StateAnchorResponsePublicKeySPKISHA256 == nil &&
		wire.StateWitnessRotationThresholdRecords == nil
	if allAbsent {
		return nil
	}
	if wire.StateWitnessMaximumRecords == nil ||
		wire.StateAnchorBindingHash == nil ||
		wire.StateAnchorResponsePublicKey == nil ||
		wire.StateAnchorResponsePublicKeySPKISHA256 == nil ||
		wire.StateWitnessRotationThresholdRecords == nil ||
		configFingerprint == "" {
		return fmt.Errorf("installed signer anchor configuration is incomplete")
	}
	bindingHash, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorBindingHash,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor binding hash is invalid: %w", err)
	}
	responsePublicKey, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorResponsePublicKey,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor response key is invalid: %w", err)
	}
	responseSPKIHash, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorResponsePublicKeySPKISHA256,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor response SPKI hash is invalid: %w", err)
	}
	maximum := *wire.StateWitnessMaximumRecords
	threshold := *wire.StateWitnessRotationThresholdRecords
	if maximum < 2 || maximum > 1_000_000 || threshold < 2 ||
		threshold > maximum-2 {
		return fmt.Errorf("installed signer witness geometry is invalid")
	}
	value := &NativeTBTCSignerInstalledStateAnchorConfig{
		BindingHash:                     bindingHash,
		ResponsePublicKey:               responsePublicKey,
		ResponsePublicKeySPKISHA256:     responseSPKIHash,
		WitnessMaximumRecords:           maximum,
		WitnessRotationThresholdRecords: threshold,
		ConfigFingerprint:               configFingerprint,
	}
	nativeTBTCSignerInstalledStateAnchorConfig.Lock()
	defer nativeTBTCSignerInstalledStateAnchorConfig.Unlock()
	if nativeTBTCSignerInstalledStateAnchorConfig.value != nil &&
		*nativeTBTCSignerInstalledStateAnchorConfig.value != *value {
		return fmt.Errorf("installed signer anchor configuration changed")
	}
	nativeTBTCSignerInstalledStateAnchorConfig.value = value
	return nil
}

// ReadInstalledNativeTBTCSignerStateAnchorConfig returns only material from
// the exact config bytes accepted by Rust. A nil result means the signer used
// transitional environment fallback and production anchor activation must
// fail closed.
func ReadInstalledNativeTBTCSignerStateAnchorConfig() (
	*NativeTBTCSignerInstalledStateAnchorConfig,
	error,
) {
	nativeTBTCSignerInstalledStateAnchorConfig.RLock()
	defer nativeTBTCSignerInstalledStateAnchorConfig.RUnlock()
	if nativeTBTCSignerInstalledStateAnchorConfig.value == nil {
		return nil, fmt.Errorf("native signer anchor configuration was not installed")
	}
	copy := *nativeTBTCSignerInstalledStateAnchorConfig.value
	return &copy, nil
}
