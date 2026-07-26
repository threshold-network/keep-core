package signing

import (
	"crypto/sha256"
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

const NativeTBTCSignerStateAnchorBootstrapProvisioningPurpose = "state_anchor_bootstrap_provisioning"

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
	ProtocolID                      [32]byte
	StreamID                        [32]byte
	ActivationManifestHash          [32]byte
	ActivationManifestSequence      uint64
	BindingHash                     [32]byte
	ResponsePublicKey               [32]byte
	ResponsePublicKeySPKISHA256     [32]byte
	OfflineAuthorityPublicKey       [32]byte
	OfflineAuthoritySPKISHA256      [32]byte
	TrustCertificateSequence        uint64
	TrustCertificateDigest          [32]byte
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
		Purpose                                *string `json:"purpose"`
		StateAnchorProtocolID                  *string `json:"state_anchor_protocol_id"`
		StateAnchorStreamID                    *string `json:"state_anchor_stream_id"`
		StateAnchorActivationManifestHash      *string `json:"state_anchor_activation_manifest_hash"`
		StateAnchorActivationManifestSequence  *uint64 `json:"state_anchor_activation_manifest_sequence"`
		StateWitnessMaximumRecords             *uint64 `json:"state_witness_max_records"`
		StateAnchorBindingHash                 *string `json:"state_anchor_binding_hash"`
		StateAnchorResponsePublicKey           *string `json:"state_anchor_response_public_key"`
		StateAnchorResponsePublicKeySPKISHA256 *string `json:"state_anchor_response_public_key_spki_sha256"`
		StateAnchorOfflineAuthorityPublicKey   *string `json:"state_anchor_offline_authority_public_key"`
		StateAnchorOfflineAuthoritySPKISHA256  *string `json:"state_anchor_offline_authority_public_key_spki_sha256"`
		StateAnchorTrustCertificateSequence    *uint64 `json:"state_anchor_trust_certificate_sequence"`
		StateAnchorTrustCertificateDigest      *string `json:"state_anchor_trust_certificate_digest"`
		StateWitnessRotationThresholdRecords   *uint64 `json:"state_witness_rotation_threshold_records"`
	}{}
	if err := json.Unmarshal(configJSON, &wire); err != nil {
		return fmt.Errorf("cannot decode installed signer anchor configuration: %w", err)
	}
	if wire.Purpose != nil &&
		*wire.Purpose ==
			NativeTBTCSignerStateAnchorBootstrapProvisioningPurpose {
		hasRuntimeAnchorField := wire.StateAnchorProtocolID != nil ||
			wire.StateAnchorStreamID != nil ||
			wire.StateAnchorActivationManifestHash != nil ||
			wire.StateAnchorActivationManifestSequence != nil ||
			wire.StateAnchorBindingHash != nil ||
			wire.StateAnchorResponsePublicKey != nil ||
			wire.StateAnchorResponsePublicKeySPKISHA256 != nil ||
			wire.StateAnchorOfflineAuthorityPublicKey != nil ||
			wire.StateAnchorOfflineAuthoritySPKISHA256 != nil ||
			wire.StateAnchorTrustCertificateSequence != nil ||
			wire.StateAnchorTrustCertificateDigest != nil ||
			wire.StateWitnessRotationThresholdRecords != nil
		if hasRuntimeAnchorField ||
			wire.StateWitnessMaximumRecords == nil ||
			*wire.StateWitnessMaximumRecords != 4 {
			return fmt.Errorf(
				"installed signer bootstrap-provisioning configuration contains runtime anchor fields or an invalid witness bound",
			)
		}

		// Bootstrap provisioning creates the store identity and emits facts
		// used to build the offline trust artifacts. It intentionally has no
		// runtime anchor authority to pin into this process.
		return nil
	}
	allAbsent := wire.StateAnchorProtocolID == nil &&
		wire.StateAnchorStreamID == nil &&
		wire.StateAnchorActivationManifestHash == nil &&
		wire.StateAnchorActivationManifestSequence == nil &&
		wire.StateWitnessMaximumRecords == nil &&
		wire.StateAnchorBindingHash == nil &&
		wire.StateAnchorResponsePublicKey == nil &&
		wire.StateAnchorResponsePublicKeySPKISHA256 == nil &&
		wire.StateAnchorOfflineAuthorityPublicKey == nil &&
		wire.StateAnchorOfflineAuthoritySPKISHA256 == nil &&
		wire.StateAnchorTrustCertificateSequence == nil &&
		wire.StateAnchorTrustCertificateDigest == nil &&
		wire.StateWitnessRotationThresholdRecords == nil
	if allAbsent {
		return nil
	}
	if wire.StateAnchorProtocolID == nil ||
		wire.StateAnchorStreamID == nil ||
		wire.StateAnchorActivationManifestHash == nil ||
		wire.StateAnchorActivationManifestSequence == nil ||
		wire.StateWitnessMaximumRecords == nil ||
		wire.StateAnchorBindingHash == nil ||
		wire.StateAnchorResponsePublicKey == nil ||
		wire.StateAnchorResponsePublicKeySPKISHA256 == nil ||
		wire.StateAnchorOfflineAuthorityPublicKey == nil ||
		wire.StateAnchorOfflineAuthoritySPKISHA256 == nil ||
		wire.StateAnchorTrustCertificateSequence == nil ||
		wire.StateAnchorTrustCertificateDigest == nil ||
		wire.StateWitnessRotationThresholdRecords == nil ||
		configFingerprint == "" {
		return fmt.Errorf("installed signer anchor configuration is incomplete")
	}
	protocolID, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorProtocolID,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor protocol ID is invalid: %w", err)
	}
	streamID, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorStreamID,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor stream ID is invalid: %w", err)
	}
	manifestHash, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorActivationManifestHash,
	)
	if err != nil {
		return fmt.Errorf("installed signer activation manifest hash is invalid: %w", err)
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
	offlineAuthorityPublicKey, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorOfflineAuthorityPublicKey,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor offline authority key is invalid: %w", err)
	}
	offlineAuthoritySPKIHash, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorOfflineAuthoritySPKISHA256,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor offline authority SPKI hash is invalid: %w", err)
	}
	trustCertificateDigest, err := decodeNativeTBTCSignerStoreBytes32(
		*wire.StateAnchorTrustCertificateDigest,
	)
	if err != nil {
		return fmt.Errorf("installed signer anchor trust certificate digest is invalid: %w", err)
	}
	maximum := *wire.StateWitnessMaximumRecords
	threshold := *wire.StateWitnessRotationThresholdRecords
	if protocolID == [32]byte{} || streamID == [32]byte{} ||
		manifestHash == [32]byte{} ||
		*wire.StateAnchorActivationManifestSequence == 0 ||
		nativeTBTCSignerEd25519SPKISHA256(responsePublicKey) != responseSPKIHash ||
		offlineAuthorityPublicKey == [32]byte{} ||
		offlineAuthoritySPKIHash == [32]byte{} ||
		nativeTBTCSignerEd25519SPKISHA256(offlineAuthorityPublicKey) !=
			offlineAuthoritySPKIHash ||
		*wire.StateAnchorTrustCertificateSequence == 0 ||
		trustCertificateDigest == [32]byte{} ||
		maximum < 2 || maximum > 1_000_000 || threshold < 2 ||
		threshold > maximum-2 {
		return fmt.Errorf("installed signer witness geometry is invalid")
	}
	value := &NativeTBTCSignerInstalledStateAnchorConfig{
		ProtocolID:                      protocolID,
		StreamID:                        streamID,
		ActivationManifestHash:          manifestHash,
		ActivationManifestSequence:      *wire.StateAnchorActivationManifestSequence,
		BindingHash:                     bindingHash,
		ResponsePublicKey:               responsePublicKey,
		ResponsePublicKeySPKISHA256:     responseSPKIHash,
		OfflineAuthorityPublicKey:       offlineAuthorityPublicKey,
		OfflineAuthoritySPKISHA256:      offlineAuthoritySPKIHash,
		TrustCertificateSequence:        *wire.StateAnchorTrustCertificateSequence,
		TrustCertificateDigest:          trustCertificateDigest,
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

func nativeTBTCSignerEd25519SPKISHA256(publicKey [32]byte) [32]byte {
	// RFC 8410 canonical DER SubjectPublicKeyInfo for Ed25519:
	// SEQUENCE { SEQUENCE { OID 1.3.101.112 }, BIT STRING <raw key> }.
	prefix := [...]byte{
		0x30, 0x2a, 0x30, 0x05, 0x06, 0x03,
		0x2b, 0x65, 0x70, 0x03, 0x21, 0x00,
	}
	input := make([]byte, 0, len(prefix)+len(publicKey))
	input = append(input, prefix[:]...)
	input = append(input, publicKey[:]...)
	return sha256.Sum256(input)
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
