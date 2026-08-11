//go:build frost_native

package signing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const (
	ShareRepairAuthorizationSchema       = "tbtc-frost-share-repair-authorization/v1"
	ShareRepairTransportRosterSchema     = "tbtc-frost-share-repair-transport-roster/v1"
	ShareRepairRecoveryBundleSchema      = "tbtc-frost-share-repair-bundle/v1"
	ShareRepairTransportPreflightSchema  = "tbtc-frost-share-repair-transport-preflight/v1"
	ShareRepairInstallResultSchema       = "tbtc-frost-share-repair-install-result/v1"
	ShareRepairActivationLeaseSchema     = "tbtc-frost-share-repair-activation/v1"
	ShareRepairActivationRegistrySchema  = "tbtc-frost-share-repair-activation-registry/v1"
	shareRepairAuthorizationDomain       = "tbtc-frost-share-repair-authorization/v1\x00"
	shareRepairTransportRosterDomain     = "tbtc-frost-share-repair-transport-roster/v1\x00"
	shareRepairActivationLeaseDomain     = "tbtc-frost-share-repair-activation/v1\x00"
	shareRepairActivationRegistryDomain  = "tbtc-frost-share-repair-activation-registry/v1\x00"
	shareRepairMaximumAuthorizationAge   = 24 * time.Hour
	shareRepairMaximumActivationRegistry = 4096
	shareRepairMaximumSessionIDLength    = 128
)

// ShareRepairAuthorization is the frozen offline-authority certificate used by
// both the Go recovery protocol and the Rust signer. Every field except the
// signature is included in the domain-separated digest below.
type ShareRepairAuthorization struct {
	Schema                     string   `json:"schema"`
	SessionID                  string   `json:"session_id"`
	WalletID                   string   `json:"wallet_id"`
	KeyGroup                   string   `json:"key_group"`
	PublicKeyPackageCommitment string   `json:"public_key_package_commitment"`
	TargetIdentifier           uint16   `json:"target_identifier"`
	HelperIdentifiers          []uint16 `json:"helper_identifiers"`
	Threshold                  uint16   `json:"threshold"`
	ParticipantCount           uint16   `json:"participant_count"`
	OldStoreFingerprint        string   `json:"old_store_fingerprint"`
	NewStoreFingerprint        string   `json:"new_store_fingerprint"`
	RecoveryEpoch              uint64   `json:"recovery_epoch"`
	IssuedAtUnix               uint64   `json:"issued_at_unix"`
	NotBeforeUnix              uint64   `json:"not_before_unix"`
	ExpiresAtUnix              uint64   `json:"expires_at_unix"`
	Nonce                      string   `json:"nonce"`
	SignatureHex               string   `json:"signature_hex"`
}

// ShareRepairTransportPublicKey binds one authorized participant and native
// store to the public half of its Rust-derived, authorization-scoped repair
// transport key.
type ShareRepairTransportPublicKey struct {
	ParticipantIdentifier uint16 `json:"participant_identifier"`
	StoreFingerprint      string `json:"store_fingerprint"`
	PublicKeyHex          string `json:"public_key_hex"`
}

// ShareRepairTransportRoster is the offline-authority-signed rendezvous
// artifact. Its signature prevents the Go host from substituting a transport
// key it controls when asking Rust to encrypt a repair scalar.
type ShareRepairTransportRoster struct {
	Schema                string                          `json:"schema"`
	AuthorizationDigest   string                          `json:"authorization_digest"`
	ParticipantPublicKeys []ShareRepairTransportPublicKey `json:"participant_public_keys"`
	SignatureHex          string                          `json:"signature_hex"`
}

// ShareRepairRecoveryBundle is the single owner-only maintenance artifact.
// The authorization and transport roster carry independent signatures from
// the same offline authority; the outer schema only makes file decoding
// explicit and downgrade-safe.
type ShareRepairRecoveryBundle struct {
	Schema          string                     `json:"schema"`
	Authorization   ShareRepairAuthorization   `json:"authorization"`
	TransportRoster ShareRepairTransportRoster `json:"transport_roster"`
}

// ShareRepairTransportPreflight is an unsigned, public ceremony artifact
// emitted by one operator after the native signer proves local seat/store
// possession to the API. The offline authority authenticates its source and
// workload out of band, merges the exact participant set, and signs
// ShareRepairTransportRoster. This artifact is not hardware attestation.
type ShareRepairTransportPreflight struct {
	Schema                string                          `json:"schema"`
	AuthorizationDigest   string                          `json:"authorization_digest"`
	ParticipantPublicKeys []ShareRepairTransportPublicKey `json:"participant_public_keys"`
}

// NativeShareRepairSession exposes only the public half and store binding of an
// authorization-scoped repair transport key. The matching derived private key
// and every plaintext repair scalar remain behind the native API. Finish evicts
// the live cache; the key remains re-derivable from the protected state root
// until the signed authorization expires.
type NativeShareRepairSession struct {
	ContextDigest         string
	ParticipantIdentifier uint16
	StoreFingerprint      string
	TransportPublicKey    []byte
}

// NativeShareRepairEncryptedDelta and NativeShareRepairEncryptedSigma contain
// opaque authenticated ciphertexts. Protocol-conformant Go may route and
// retransmit Payload but receives no decryption capability; complete plaintext
// scalar sets never appear in FFI requests or responses. This API property is
// not process isolation from arbitrary same-address-space memory access.
type NativeShareRepairEncryptedDelta struct {
	ContextDigest       string
	SenderIdentifier    uint16
	RecipientIdentifier uint16
	Payload             []byte
}

type NativeShareRepairEncryptedSigma struct {
	ContextDigest    string
	HelperIdentifier uint16
	Payload          []byte
}

type NativeShareRepairPart1Result struct {
	ContextDigest    string
	HelperIdentifier uint16
	PublicKeyPackage *NativeFROSTPublicKeyPackage
	Deltas           []*NativeShareRepairEncryptedDelta
}

type NativeShareRepairPart2Result struct {
	ContextDigest string
	Sigma         *NativeShareRepairEncryptedSigma
}

type NativeShareRepairInstallResult struct {
	Schema                 string
	SessionID              string
	KeyGroup               string
	TargetIdentifier       uint16
	RecoveryEpoch          uint64
	AuthorizationDigest    string
	ActiveStoreFingerprint string
	Idempotent             bool
}

// NativeTBTCSignerShareRepairEngine is kept separate from the ordinary DKG
// capability so a stale ABI cannot accidentally be treated as DR-capable.
type NativeTBTCSignerShareRepairEngine interface {
	BeginShareRepairSession(
		authorization *ShareRepairAuthorization,
		participantIdentifier uint16,
	) (*NativeShareRepairSession, error)
	FinishShareRepairSession(
		authorization *ShareRepairAuthorization,
		participantIdentifier uint16,
	) error
	ShareRepairPart1(
		authorization *ShareRepairAuthorization,
		helperIdentifier uint16,
		transportRoster *ShareRepairTransportRoster,
	) (*NativeShareRepairPart1Result, error)
	ShareRepairPart2(
		authorization *ShareRepairAuthorization,
		helperIdentifier uint16,
		deltas []*NativeShareRepairEncryptedDelta,
		transportRoster *ShareRepairTransportRoster,
	) (*NativeShareRepairPart2Result, error)
	InstallRepairedShare(
		authorization *ShareRepairAuthorization,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
		sigmas []*NativeShareRepairEncryptedSigma,
		transportRoster *ShareRepairTransportRoster,
	) (*NativeShareRepairInstallResult, error)
}

// ShareRepairActivationLease is signed only after the repaired Rust state and
// its independent anchor acknowledgement are durable and the old anchor stream
// has been tombstoned. Nodes exchange this public certificate on every ROAST
// message from the recovered seat; a stale process has no current lease.
type ShareRepairActivationLease struct {
	Schema                  string                   `json:"schema"`
	Authorization           ShareRepairAuthorization `json:"authorization"`
	AuthorizationDigest     string                   `json:"authorization_digest"`
	OldStoreTombstoneDigest string                   `json:"old_store_tombstone_digest"`
	ActivatedAtUnix         uint64                   `json:"activated_at_unix"`
	SignatureHex            string                   `json:"signature_hex"`
}

type ShareRepairActivationRegistry struct {
	Schema string                       `json:"schema"`
	Leases []ShareRepairActivationLease `json:"leases"`
}

type validatedShareRepairAuthorization struct {
	digest              [32]byte
	walletID            [32]byte
	newStoreFingerprint [32]byte
}

// DecodeShareRepairAuthorization strictly decodes the detached, public
// recovery certificate. Signature and wall-clock validation remain in
// RunShareRepair, where the manifest-pinned authority key is available.
func DecodeShareRepairAuthorization(payload []byte) (*ShareRepairAuthorization, error) {
	if len(payload) == 0 || len(payload) > 256*1024 {
		return nil, fmt.Errorf("share-repair authorization size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	authorization := &ShareRepairAuthorization{}
	if err := decoder.Decode(authorization); err != nil {
		return nil, fmt.Errorf("cannot decode share-repair authorization: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("share-repair authorization has trailing JSON")
	}
	if _, err := ComputeShareRepairAuthorizationDigest(authorization); err != nil {
		return nil, err
	}
	if _, err := parseCanonicalShareRepairSignature(authorization.SignatureHex); err != nil {
		return nil, err
	}
	return authorization, nil
}

// DecodeShareRepairRecoveryBundle strictly decodes the one-shot maintenance
// artifact. Authority and time validation remain in RunShareRepair, after the
// manifest-pinned authority key is available.
func DecodeShareRepairRecoveryBundle(
	payload []byte,
) (*ShareRepairRecoveryBundle, error) {
	if len(payload) == 0 || len(payload) > 256*1024 {
		return nil, fmt.Errorf("share-repair recovery bundle size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	bundle := &ShareRepairRecoveryBundle{}
	if err := decoder.Decode(bundle); err != nil {
		return nil, fmt.Errorf("cannot decode share-repair recovery bundle: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("share-repair recovery bundle has trailing JSON")
	}
	if bundle.Schema != ShareRepairRecoveryBundleSchema {
		return nil, fmt.Errorf("unsupported share-repair recovery bundle schema")
	}
	if _, err := ComputeShareRepairAuthorizationDigest(&bundle.Authorization); err != nil {
		return nil, err
	}
	if _, err := parseCanonicalShareRepairSignature(
		bundle.Authorization.SignatureHex,
	); err != nil {
		return nil, err
	}
	if _, err := ComputeShareRepairTransportRosterDigest(
		&bundle.TransportRoster,
		&bundle.Authorization,
	); err != nil {
		return nil, err
	}
	if _, err := parseCanonicalShareRepairSignature(
		bundle.TransportRoster.SignatureHex,
	); err != nil {
		return nil, err
	}
	return bundle, nil
}

func parseCanonicalShareRepairHex32(value string, label string) ([32]byte, error) {
	result := [32]byte{}
	if len(value) != 66 || !strings.HasPrefix(value, "0x") || value != strings.ToLower(value) {
		return result, fmt.Errorf("%s must be canonical lowercase 0x-prefixed bytes32", label)
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != 32 {
		return result, fmt.Errorf("%s must be canonical lowercase 0x-prefixed bytes32", label)
	}
	copy(result[:], decoded)
	if result == [32]byte{} {
		return result, fmt.Errorf("%s must not be zero", label)
	}
	return result, nil
}

func parseCanonicalShareRepairSignature(value string) ([]byte, error) {
	if len(value) != 130 || !strings.HasPrefix(value, "0x") || value != strings.ToLower(value) {
		return nil, fmt.Errorf("signature must be canonical lowercase 0x-prefixed 64-byte hex")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != ed25519.SignatureSize {
		return nil, fmt.Errorf("signature must be canonical lowercase 0x-prefixed 64-byte hex")
	}
	return decoded, nil
}

func writeShareRepairLengthPrefixed(buffer *bytes.Buffer, value []byte) error {
	if uint64(len(value)) > uint64(^uint32(0)) {
		return fmt.Errorf("share-repair transcript field exceeds uint32")
	}
	length := [4]byte{}
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	buffer.Write(length[:])
	buffer.Write(value)
	return nil
}

func isValidShareRepairSessionID(sessionID string) bool {
	if len(sessionID) == 0 ||
		len(sessionID) > shareRepairMaximumSessionIDLength ||
		!utf8.ValidString(sessionID) {
		return false
	}

	// Keep this byte-level grammar synchronized with Rust's
	// validate_session_id. The signed value is forwarded unchanged across the
	// FFI boundary, so Go must never authorize a value the native signer rejects.
	for i := 0; i < len(sessionID); i++ {
		value := sessionID[i]
		if value <= 0x1f || value == 0x7f {
			return false
		}
		switch value {
		case ' ', '=', '"', '\\':
			return false
		}
	}

	return true
}

// ComputeShareRepairAuthorizationDigest implements the same frozen transcript
// as Rust. It performs all structural validation but intentionally does not
// check wall-clock validity or the signature.
func ComputeShareRepairAuthorizationDigest(
	authorization *ShareRepairAuthorization,
) ([32]byte, error) {
	result := [32]byte{}
	if authorization == nil {
		return result, fmt.Errorf("share-repair authorization is nil")
	}
	if authorization.Schema != ShareRepairAuthorizationSchema {
		return result, fmt.Errorf("unsupported share-repair authorization schema")
	}
	if !isValidShareRepairSessionID(authorization.SessionID) {
		return result, fmt.Errorf("share-repair session id is invalid")
	}
	if authorization.Threshold < 2 || authorization.ParticipantCount < authorization.Threshold ||
		authorization.ParticipantCount > 100 ||
		authorization.ParticipantCount > uint16(group.MaxMemberIndex) {
		return result, fmt.Errorf("share-repair threshold or participant count is invalid")
	}
	if len(authorization.HelperIdentifiers) != int(authorization.Threshold) {
		return result, fmt.Errorf("helper set must contain exactly threshold members")
	}
	if authorization.TargetIdentifier == 0 ||
		authorization.TargetIdentifier > authorization.ParticipantCount {
		return result, fmt.Errorf("target identifier is outside the participant set")
	}
	previous := uint16(0)
	for _, helper := range authorization.HelperIdentifiers {
		if helper == 0 || helper > authorization.ParticipantCount || helper <= previous ||
			helper == authorization.TargetIdentifier {
			return result, fmt.Errorf("helper set must be sorted, distinct, in-range, and exclude the target")
		}
		previous = helper
	}
	if authorization.RecoveryEpoch == 0 ||
		authorization.IssuedAtUnix > authorization.NotBeforeUnix ||
		authorization.NotBeforeUnix >= authorization.ExpiresAtUnix ||
		authorization.ExpiresAtUnix-authorization.IssuedAtUnix >
			uint64(shareRepairMaximumAuthorizationAge/time.Second) {
		return result, fmt.Errorf("share-repair epoch or authorization lifetime is invalid")
	}

	walletID, err := parseCanonicalShareRepairHex32(authorization.WalletID, "wallet_id")
	if err != nil {
		return result, err
	}
	publicCommitment, err := parseCanonicalShareRepairHex32(
		authorization.PublicKeyPackageCommitment,
		"public_key_package_commitment",
	)
	if err != nil {
		return result, err
	}
	oldStore, err := parseCanonicalShareRepairHex32(
		authorization.OldStoreFingerprint,
		"old_store_fingerprint",
	)
	if err != nil {
		return result, err
	}
	newStore, err := parseCanonicalShareRepairHex32(
		authorization.NewStoreFingerprint,
		"new_store_fingerprint",
	)
	if err != nil {
		return result, err
	}
	if oldStore == newStore {
		return result, fmt.Errorf("old and new share-repair stores must differ")
	}
	nonce, err := parseCanonicalShareRepairHex32(authorization.Nonce, "nonce")
	if err != nil {
		return result, err
	}
	if len(authorization.KeyGroup) != 66 ||
		authorization.KeyGroup != strings.ToLower(authorization.KeyGroup) {
		return result, fmt.Errorf("key_group must be canonical lowercase compressed SEC1 hex")
	}
	compressedKeyGroup, err := hex.DecodeString(authorization.KeyGroup)
	if err != nil || len(compressedKeyGroup) != 33 {
		return result, fmt.Errorf("key_group must be canonical lowercase compressed SEC1 hex")
	}
	publicKey, err := btcec.ParsePubKey(compressedKeyGroup)
	if err != nil || !bytes.Equal(publicKey.SerializeCompressed(), compressedKeyGroup) {
		return result, fmt.Errorf("key_group must be canonical lowercase compressed SEC1 hex")
	}
	derivedWalletID := publicKey.X().FillBytes(make([]byte, 32))
	if !bytes.Equal(derivedWalletID, walletID[:]) {
		return result, fmt.Errorf("wallet_id does not match key_group")
	}

	transcript := bytes.NewBuffer(nil)
	transcript.WriteString(shareRepairAuthorizationDomain)
	if err := writeShareRepairLengthPrefixed(transcript, []byte(authorization.SessionID)); err != nil {
		return result, err
	}
	transcript.Write(walletID[:])
	transcript.Write(compressedKeyGroup)
	transcript.Write(publicCommitment[:])
	_ = binary.Write(transcript, binary.BigEndian, authorization.TargetIdentifier)
	_ = binary.Write(transcript, binary.BigEndian, uint16(len(authorization.HelperIdentifiers)))
	for _, helper := range authorization.HelperIdentifiers {
		_ = binary.Write(transcript, binary.BigEndian, helper)
	}
	_ = binary.Write(transcript, binary.BigEndian, authorization.Threshold)
	_ = binary.Write(transcript, binary.BigEndian, authorization.ParticipantCount)
	transcript.Write(oldStore[:])
	transcript.Write(newStore[:])
	_ = binary.Write(transcript, binary.BigEndian, authorization.RecoveryEpoch)
	_ = binary.Write(transcript, binary.BigEndian, authorization.IssuedAtUnix)
	_ = binary.Write(transcript, binary.BigEndian, authorization.NotBeforeUnix)
	_ = binary.Write(transcript, binary.BigEndian, authorization.ExpiresAtUnix)
	transcript.Write(nonce[:])
	return sha256.Sum256(transcript.Bytes()), nil
}

func parseCanonicalShareRepairTransportPublicKey(
	value string,
	label string,
) ([33]byte, error) {
	result := [33]byte{}
	if len(value) != 66 || value != strings.ToLower(value) {
		return result, fmt.Errorf(
			"%s must be canonical lowercase unprefixed 33-byte compressed SEC1 hex",
			label,
		)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(result) {
		return result, fmt.Errorf(
			"%s must be canonical lowercase unprefixed 33-byte compressed SEC1 hex",
			label,
		)
	}
	publicKey, err := btcec.ParsePubKey(decoded)
	if err != nil || !bytes.Equal(publicKey.SerializeCompressed(), decoded) {
		return result, fmt.Errorf(
			"%s must be canonical lowercase unprefixed 33-byte compressed SEC1 hex",
			label,
		)
	}
	copy(result[:], decoded)
	return result, nil
}

// ComputeShareRepairTransportRosterDigest implements the frozen transcript
// shared with Rust. It validates the exact helper-then-target participant
// ordering and public-key encoding, but intentionally does not verify either
// offline-authority signature.
func ComputeShareRepairTransportRosterDigest(
	transportRoster *ShareRepairTransportRoster,
	authorization *ShareRepairAuthorization,
) ([32]byte, error) {
	result := [32]byte{}
	authorizationDigest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return result, err
	}
	if transportRoster == nil {
		return result, fmt.Errorf("share-repair transport roster is nil")
	}
	if transportRoster.Schema != ShareRepairTransportRosterSchema {
		return result, fmt.Errorf("unsupported share-repair transport roster schema")
	}
	wireAuthorizationDigest, err := parseCanonicalShareRepairHex32(
		transportRoster.AuthorizationDigest,
		"transport roster authorization_digest",
	)
	if err != nil {
		return result, err
	}
	if wireAuthorizationDigest != authorizationDigest {
		return result, fmt.Errorf("share-repair transport roster authorization digest mismatch")
	}
	targetStoreFingerprint, err := parseCanonicalShareRepairHex32(
		authorization.NewStoreFingerprint,
		"new_store_fingerprint",
	)
	if err != nil {
		return result, err
	}

	expectedCount := len(authorization.HelperIdentifiers) + 1
	if len(transportRoster.ParticipantPublicKeys) != expectedCount {
		return result, fmt.Errorf(
			"share-repair transport roster must contain the exact helper and target set",
		)
	}

	transcript := bytes.NewBuffer(nil)
	transcript.WriteString(shareRepairTransportRosterDomain)
	transcript.Write(authorizationDigest[:])
	_ = binary.Write(transcript, binary.BigEndian, uint16(expectedCount))
	seenPublicKeys := make(map[[33]byte]struct{}, expectedCount)
	for index, participantPublicKey := range transportRoster.ParticipantPublicKeys {
		expectedIdentifier := authorization.TargetIdentifier
		if index < len(authorization.HelperIdentifiers) {
			expectedIdentifier = authorization.HelperIdentifiers[index]
		}
		if participantPublicKey.ParticipantIdentifier != expectedIdentifier {
			return result, fmt.Errorf(
				"share-repair transport roster participant [%d] is invalid or out of order",
				index,
			)
		}
		storeFingerprint, err := parseCanonicalShareRepairHex32(
			participantPublicKey.StoreFingerprint,
			fmt.Sprintf("transport roster store fingerprint [%d]", index),
		)
		if err != nil {
			return result, err
		}
		if participantPublicKey.ParticipantIdentifier == authorization.TargetIdentifier &&
			storeFingerprint != targetStoreFingerprint {
			return result, fmt.Errorf(
				"share-repair target transport roster entry does not name new_store_fingerprint",
			)
		}
		publicKey, err := parseCanonicalShareRepairTransportPublicKey(
			participantPublicKey.PublicKeyHex,
			fmt.Sprintf("transport roster public key [%d]", index),
		)
		if err != nil {
			return result, err
		}
		if _, duplicate := seenPublicKeys[publicKey]; duplicate {
			return result, fmt.Errorf("share-repair transport roster public keys must be unique")
		}
		seenPublicKeys[publicKey] = struct{}{}
		_ = binary.Write(
			transcript,
			binary.BigEndian,
			participantPublicKey.ParticipantIdentifier,
		)
		transcript.Write(storeFingerprint[:])
		transcript.Write(publicKey[:])
	}

	return sha256.Sum256(transcript.Bytes()), nil
}

func validateShareRepairAuthorization(
	authorization *ShareRepairAuthorization,
	authorityPublicKey ed25519.PublicKey,
	enforceTime bool,
) (*validatedShareRepairAuthorization, error) {
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return nil, err
	}
	if len(authorityPublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("share-repair authority public key is invalid")
	}
	signature, err := parseCanonicalShareRepairSignature(authorization.SignatureHex)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(authorityPublicKey, digest[:], signature) {
		return nil, fmt.Errorf("share-repair authorization signature is invalid")
	}
	if enforceTime {
		now := uint64(time.Now().Unix())
		if now < authorization.NotBeforeUnix || now >= authorization.ExpiresAtUnix {
			return nil, fmt.Errorf("share-repair authorization is not currently valid")
		}
	}
	walletID, _ := parseCanonicalShareRepairHex32(authorization.WalletID, "wallet_id")
	newStore, _ := parseCanonicalShareRepairHex32(
		authorization.NewStoreFingerprint,
		"new_store_fingerprint",
	)
	return &validatedShareRepairAuthorization{
		digest:              digest,
		walletID:            walletID,
		newStoreFingerprint: newStore,
	}, nil
}

// ValidateShareRepairTransportRoster verifies that the authorization and its
// exact transport roster were signed by the same manifest-pinned authority.
// The caller remains responsible for enforcing the authorization time window.
func ValidateShareRepairTransportRoster(
	transportRoster *ShareRepairTransportRoster,
	authorization *ShareRepairAuthorization,
	authorityPublicKey ed25519.PublicKey,
) error {
	if _, err := validateShareRepairAuthorization(
		authorization,
		authorityPublicKey,
		false,
	); err != nil {
		return fmt.Errorf("invalid share-repair transport roster authorization: %w", err)
	}
	digest, err := ComputeShareRepairTransportRosterDigest(
		transportRoster,
		authorization,
	)
	if err != nil {
		return err
	}
	signature, err := parseCanonicalShareRepairSignature(transportRoster.SignatureHex)
	if err != nil {
		return fmt.Errorf("invalid share-repair transport roster signature encoding: %w", err)
	}
	if !ed25519.Verify(authorityPublicKey, digest[:], signature) {
		return fmt.Errorf("share-repair transport roster signature is invalid")
	}
	return nil
}

func computeShareRepairActivationLeaseDigest(
	lease *ShareRepairActivationLease,
	authorizationDigest [32]byte,
	tombstoneDigest [32]byte,
) [32]byte {
	transcript := bytes.NewBuffer(nil)
	transcript.WriteString(shareRepairActivationLeaseDomain)
	transcript.Write(authorizationDigest[:])
	transcript.Write(tombstoneDigest[:])
	_ = binary.Write(transcript, binary.BigEndian, lease.ActivatedAtUnix)
	return sha256.Sum256(transcript.Bytes())
}

func validatedShareRepairActivationLeaseDigest(
	lease *ShareRepairActivationLease,
	authorityPublicKey ed25519.PublicKey,
) ([32]byte, *validatedShareRepairAuthorization, error) {
	result := [32]byte{}
	if lease == nil || lease.Schema != ShareRepairActivationLeaseSchema {
		return result, nil, fmt.Errorf("unsupported share-repair activation lease schema")
	}
	validatedAuthorization, err := validateShareRepairAuthorization(
		&lease.Authorization,
		authorityPublicKey,
		false,
	)
	if err != nil {
		return result, nil, fmt.Errorf("invalid share-repair activation authorization: %w", err)
	}
	wireAuthorizationDigest, err := parseCanonicalShareRepairHex32(
		lease.AuthorizationDigest,
		"authorization_digest",
	)
	if err != nil || wireAuthorizationDigest != validatedAuthorization.digest {
		return result, nil, fmt.Errorf("activation lease authorization digest mismatch")
	}
	tombstoneDigest, err := parseCanonicalShareRepairHex32(
		lease.OldStoreTombstoneDigest,
		"old_store_tombstone_digest",
	)
	if err != nil {
		return result, nil, err
	}
	if lease.ActivatedAtUnix == 0 || lease.ActivatedAtUnix < lease.Authorization.IssuedAtUnix {
		return result, nil, fmt.Errorf("share-repair activation time is invalid")
	}
	return computeShareRepairActivationLeaseDigest(
		lease,
		validatedAuthorization.digest,
		tombstoneDigest,
	), validatedAuthorization, nil
}

// ComputeShareRepairActivationLeaseDigest returns the exact digest an offline
// authority signs after independently confirming both the new-store anchor ACK
// and the old-store stream tombstone. The embedded recovery authorization must
// already carry a valid signature from the same authority.
func ComputeShareRepairActivationLeaseDigest(
	lease *ShareRepairActivationLease,
	authorityPublicKey ed25519.PublicKey,
) ([32]byte, error) {
	digest, _, err := validatedShareRepairActivationLeaseDigest(
		lease,
		authorityPublicKey,
	)
	return digest, err
}

type validatedShareRepairActivationLease struct {
	lease               ShareRepairActivationLease
	authorizationDigest [32]byte
	activationDigest    [32]byte
	newStoreFingerprint [32]byte
	wire                []byte
}

func validateShareRepairActivationLease(
	lease *ShareRepairActivationLease,
	authorityPublicKey ed25519.PublicKey,
) (*validatedShareRepairActivationLease, error) {
	activationDigest, validatedAuthorization, err :=
		validatedShareRepairActivationLeaseDigest(
			lease,
			authorityPublicKey,
		)
	if err != nil {
		return nil, err
	}
	signature, err := parseCanonicalShareRepairSignature(lease.SignatureHex)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(authorityPublicKey, activationDigest[:], signature) {
		return nil, fmt.Errorf("share-repair activation lease signature is invalid")
	}
	wire, err := json.Marshal(lease)
	if err != nil {
		return nil, fmt.Errorf("cannot encode share-repair activation lease: %w", err)
	}
	return &validatedShareRepairActivationLease{
		lease:               *lease,
		authorizationDigest: validatedAuthorization.digest,
		activationDigest:    activationDigest,
		newStoreFingerprint: validatedAuthorization.newStoreFingerprint,
		wire:                wire,
	}, nil
}

func validateShareRepairActivationRegistry(
	registry *ShareRepairActivationRegistry,
	authorityPublicKey ed25519.PublicKey,
) (map[shareRepairActivationKey]*validatedShareRepairActivationLease, [32]byte, error) {
	result := [32]byte{}
	if registry == nil || registry.Schema != ShareRepairActivationRegistrySchema ||
		len(registry.Leases) == 0 ||
		len(registry.Leases) > shareRepairMaximumActivationRegistry {
		return nil, result, fmt.Errorf("share-repair activation registry shape is invalid")
	}

	validated := make(map[shareRepairActivationKey]*validatedShareRepairActivationLease)
	orderedDigests := make([][32]byte, 0, len(registry.Leases))
	previous := shareRepairActivationKey{}
	for index := range registry.Leases {
		lease, err := validateShareRepairActivationLease(
			&registry.Leases[index],
			authorityPublicKey,
		)
		if err != nil {
			return nil, result, fmt.Errorf("invalid share-repair activation lease [%d]: %w", index, err)
		}
		key := shareRepairActivationKey{
			keyGroup: lease.lease.Authorization.KeyGroup,
			seat:     lease.lease.Authorization.TargetIdentifier,
		}
		if index > 0 && (key.keyGroup < previous.keyGroup ||
			(key.keyGroup == previous.keyGroup && key.seat <= previous.seat)) {
			return nil, result, fmt.Errorf("share-repair activation leases must be unique and canonically sorted")
		}
		previous = key
		validated[key] = lease
		orderedDigests = append(orderedDigests, lease.activationDigest)
	}
	rootInput := bytes.NewBuffer(nil)
	rootInput.WriteString(shareRepairActivationRegistryDomain)
	_ = binary.Write(rootInput, binary.BigEndian, uint32(len(orderedDigests)))
	for _, digest := range orderedDigests {
		rootInput.Write(digest[:])
	}
	return validated, sha256.Sum256(rootInput.Bytes()), nil
}

type shareRepairActivationKey struct {
	keyGroup string
	seat     uint16
}

type shareRepairRecoveredSeatBinding struct {
	recoveryEpoch          uint64
	authorizationDigest    [32]byte
	activeStoreFingerprint [32]byte
}

type installedShareRepairActivationRegistry struct {
	root   [32]byte
	leases map[shareRepairActivationKey]*validatedShareRepairActivationLease
}

var shareRepairActivationRegistryState struct {
	sync.RWMutex
	configured            bool
	localStoreFingerprint [32]byte
	localSeats            map[shareRepairActivationKey]struct{}
	recoveredSeats        map[shareRepairActivationKey]shareRepairRecoveredSeatBinding
	installed             *installedShareRepairActivationRegistry
}

// ConfigureShareRepairActivationGuard installs the descriptor-bound recovery
// facts read directly from the Rust inventory. This call must precede signing
// readiness and registry installation. In particular, a v2 inventory with no
// matching activation lease remains unable to sign across process restarts.
func ConfigureShareRepairActivationGuard(
	inventory *NativeTBTCSignerRetainedKeyPackageInventory,
) error {
	if inventory == nil || inventory.StoreFingerprint == [32]byte{} {
		return fmt.Errorf("share-repair activation inventory is missing its store binding")
	}
	if inventory.Schema != NativeTBTCSignerRetainedKeyPackageInventorySchema &&
		inventory.Schema != NativeTBTCSignerRetainedKeyPackageInventoryRecoverySchema {
		return fmt.Errorf("share-repair activation inventory schema is unsupported")
	}

	localSeats := make(map[shareRepairActivationKey]struct{})
	for _, entry := range inventory.Entries {
		if entry.KeyGroup == "" {
			return fmt.Errorf("share-repair activation inventory contains an empty key group")
		}
		for _, keyPackage := range entry.KeyPackages {
			key := shareRepairActivationKey{
				keyGroup: entry.KeyGroup,
				seat:     keyPackage.ParticipantSeat,
			}
			if key.seat == 0 {
				return fmt.Errorf("share-repair activation inventory contains a zero seat")
			}
			if _, exists := localSeats[key]; exists {
				return fmt.Errorf("share-repair activation inventory contains a duplicate local seat")
			}
			localSeats[key] = struct{}{}
		}
	}

	recoveredSeats := make(
		map[shareRepairActivationKey]shareRepairRecoveredSeatBinding,
		len(inventory.RecoveredSeats),
	)
	for _, recovered := range inventory.RecoveredSeats {
		key := shareRepairActivationKey{
			keyGroup: recovered.KeyGroup,
			seat:     recovered.ParticipantSeat,
		}
		if _, retained := localSeats[key]; !retained || recovered.RecoveryEpoch == 0 ||
			recovered.AuthorizationDigest == [32]byte{} ||
			recovered.ActiveStoreFingerprint != inventory.StoreFingerprint {
			return fmt.Errorf("share-repair recovered seat is not bound to the retained inventory")
		}
		if _, exists := recoveredSeats[key]; exists {
			return fmt.Errorf("share-repair activation inventory contains a duplicate recovered seat")
		}
		recoveredSeats[key] = shareRepairRecoveredSeatBinding{
			recoveryEpoch:          recovered.RecoveryEpoch,
			authorizationDigest:    recovered.AuthorizationDigest,
			activeStoreFingerprint: recovered.ActiveStoreFingerprint,
		}
	}
	if inventory.Schema == NativeTBTCSignerRetainedKeyPackageInventorySchema {
		if len(recoveredSeats) != 0 || inventory.RecoveryActivationCommitment != [32]byte{} {
			return fmt.Errorf("v1 share-repair activation inventory contains recovery facts")
		}
	} else if len(recoveredSeats) == 0 ||
		inventory.RecoveryActivationCommitment == [32]byte{} ||
		ComputeNativeTBTCSignerRecoveredSeatActivationCommitment(inventory.RecoveredSeats) !=
			inventory.RecoveryActivationCommitment {
		return fmt.Errorf("v2 share-repair activation inventory commitment is invalid")
	}

	shareRepairActivationRegistryState.Lock()
	defer shareRepairActivationRegistryState.Unlock()
	if shareRepairActivationRegistryState.configured {
		return fmt.Errorf("share-repair activation guard is already configured")
	}
	if shareRepairActivationRegistryState.installed != nil {
		return fmt.Errorf("share-repair activation registry preceded its inventory guard")
	}
	shareRepairActivationRegistryState.configured = true
	shareRepairActivationRegistryState.localStoreFingerprint = inventory.StoreFingerprint
	shareRepairActivationRegistryState.localSeats = localSeats
	shareRepairActivationRegistryState.recoveredSeats = recoveredSeats
	return nil
}

func shareRepairActivationLeaseMatchesBinding(
	lease *validatedShareRepairActivationLease,
	binding shareRepairRecoveredSeatBinding,
) bool {
	return lease != nil &&
		lease.authorizationDigest == binding.authorizationDigest &&
		lease.lease.Authorization.RecoveryEpoch == binding.recoveryEpoch &&
		lease.newStoreFingerprint == binding.activeStoreFingerprint
}

func validateLocalShareRepairActivationRegistryLocked(
	registry *installedShareRepairActivationRegistry,
) error {
	for key, binding := range shareRepairActivationRegistryState.recoveredSeats {
		if !shareRepairActivationLeaseMatchesBinding(registry.leases[key], binding) {
			return fmt.Errorf(
				"recovered seat [%d] for key group [%s] has no exact activation lease",
				key.seat,
				key.keyGroup,
			)
		}
	}
	for key := range shareRepairActivationRegistryState.localSeats {
		lease := registry.leases[key]
		if lease == nil {
			continue
		}
		binding, recovered := shareRepairActivationRegistryState.recoveredSeats[key]
		if !recovered || !shareRepairActivationLeaseMatchesBinding(lease, binding) ||
			lease.newStoreFingerprint != shareRepairActivationRegistryState.localStoreFingerprint {
			return fmt.Errorf(
				"activation lease for local seat [%d] does not match the recovered durable store",
				key.seat,
			)
		}
	}
	return nil
}

func recordInstalledShareRepair(result *NativeShareRepairInstallResult) error {
	if result == nil || result.KeyGroup == "" || result.TargetIdentifier == 0 ||
		result.RecoveryEpoch == 0 {
		return fmt.Errorf("installed share-repair result is incomplete")
	}
	authorizationDigest, err := parseCanonicalShareRepairHex32(
		result.AuthorizationDigest,
		"authorization_digest",
	)
	if err != nil {
		return err
	}
	activeStoreFingerprint, err := parseCanonicalShareRepairHex32(
		result.ActiveStoreFingerprint,
		"active_store_fingerprint",
	)
	if err != nil {
		return err
	}
	key := shareRepairActivationKey{
		keyGroup: result.KeyGroup,
		seat:     result.TargetIdentifier,
	}
	binding := shareRepairRecoveredSeatBinding{
		recoveryEpoch:          result.RecoveryEpoch,
		authorizationDigest:    authorizationDigest,
		activeStoreFingerprint: activeStoreFingerprint,
	}

	shareRepairActivationRegistryState.Lock()
	defer shareRepairActivationRegistryState.Unlock()
	if !shareRepairActivationRegistryState.configured {
		shareRepairActivationRegistryState.configured = true
		shareRepairActivationRegistryState.localStoreFingerprint = activeStoreFingerprint
		shareRepairActivationRegistryState.localSeats = make(map[shareRepairActivationKey]struct{})
		shareRepairActivationRegistryState.recoveredSeats =
			make(map[shareRepairActivationKey]shareRepairRecoveredSeatBinding)
	}
	if shareRepairActivationRegistryState.localStoreFingerprint != activeStoreFingerprint {
		return fmt.Errorf("installed repaired share belongs to another durable store")
	}
	shareRepairActivationRegistryState.localSeats[key] = struct{}{}
	if existing, exists := shareRepairActivationRegistryState.recoveredSeats[key]; exists {
		if existing.recoveryEpoch > binding.recoveryEpoch ||
			(existing.recoveryEpoch == binding.recoveryEpoch && existing != binding) {
			return fmt.Errorf("installed repaired share conflicts with the activation guard")
		}
	}
	shareRepairActivationRegistryState.recoveredSeats[key] = binding
	return nil
}

// InstallShareRepairActivationRegistry verifies the complete authority-signed
// cutover artifact and installs it immutably for this process. expectedRoot is
// taken from the signed activation manifest; localStoreFingerprint is the Rust
// descriptor-bound readback.
func InstallShareRepairActivationRegistry(
	payload []byte,
	authorityPublicKey ed25519.PublicKey,
	expectedRoot [32]byte,
	localStoreFingerprint [32]byte,
) error {
	if len(payload) == 0 || len(payload) > 8*1024*1024 || expectedRoot == [32]byte{} ||
		localStoreFingerprint == [32]byte{} {
		return fmt.Errorf("share-repair activation registry dependencies are incomplete")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	registry := &ShareRepairActivationRegistry{}
	if err := decoder.Decode(registry); err != nil {
		return fmt.Errorf("cannot decode share-repair activation registry: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("share-repair activation registry has trailing JSON")
	}
	validated, root, err := validateShareRepairActivationRegistry(
		registry,
		authorityPublicKey,
	)
	if err != nil {
		return err
	}
	if root != expectedRoot {
		return fmt.Errorf("share-repair activation registry root differs from signed manifest")
	}

	shareRepairActivationRegistryState.Lock()
	defer shareRepairActivationRegistryState.Unlock()
	if !shareRepairActivationRegistryState.configured ||
		shareRepairActivationRegistryState.localStoreFingerprint != localStoreFingerprint {
		return fmt.Errorf("share-repair activation registry is not bound to the native inventory")
	}
	if shareRepairActivationRegistryState.installed != nil {
		return fmt.Errorf("share-repair activation registry is already installed")
	}
	installed := &installedShareRepairActivationRegistry{
		root:   root,
		leases: validated,
	}
	if err := validateLocalShareRepairActivationRegistryLocked(installed); err != nil {
		return err
	}
	shareRepairActivationRegistryState.installed = installed
	return nil
}

// CurrentShareRepairActivationRegistryRoot is included in readiness and the
// activation handshake. Zero means the signed manifest declared no recovered
// seats and no registry was installed.
func CurrentShareRepairActivationRegistryRoot() [32]byte {
	shareRepairActivationRegistryState.RLock()
	defer shareRepairActivationRegistryState.RUnlock()
	if shareRepairActivationRegistryState.installed == nil {
		return [32]byte{}
	}
	return shareRepairActivationRegistryState.installed.root
}

func shareRepairActivationLeaseForBroadcast(
	keyGroup string,
	seat group.MemberIndex,
) ([]byte, error) {
	shareRepairActivationRegistryState.RLock()
	defer shareRepairActivationRegistryState.RUnlock()
	key := shareRepairActivationKey{keyGroup: keyGroup, seat: uint16(seat)}
	registry := shareRepairActivationRegistryState.installed
	binding, recovered := shareRepairActivationRegistryState.recoveredSeats[key]
	if registry == nil {
		if recovered {
			return nil, fmt.Errorf(
				"recovered seat [%d] is pending authority-signed activation",
				seat,
			)
		}
		return nil, nil
	}
	lease := registry.leases[key]
	if lease == nil {
		if recovered {
			return nil, fmt.Errorf("recovered seat [%d] has no activation lease", seat)
		}
		return nil, nil
	}
	if lease.newStoreFingerprint != shareRepairActivationRegistryState.localStoreFingerprint {
		return nil, fmt.Errorf(
			"recovered seat [%d] is bound to another durable signer store",
			seat,
		)
	}
	if _, local := shareRepairActivationRegistryState.localSeats[key]; local &&
		(!recovered || !shareRepairActivationLeaseMatchesBinding(lease, binding)) {
		return nil, fmt.Errorf(
			"recovered seat [%d] is not proven by the native signer inventory",
			seat,
		)
	}
	return append([]byte(nil), lease.wire...), nil
}

func validateShareRepairActivationLeaseForMessage(
	keyGroup string,
	seat group.MemberIndex,
	wire []byte,
) error {
	shareRepairActivationRegistryState.RLock()
	defer shareRepairActivationRegistryState.RUnlock()
	registry := shareRepairActivationRegistryState.installed
	if registry == nil {
		if len(wire) != 0 {
			return fmt.Errorf("unexpected share-repair activation lease")
		}
		return nil
	}
	expected := registry.leases[shareRepairActivationKey{keyGroup: keyGroup, seat: uint16(seat)}]
	if expected == nil {
		if len(wire) != 0 {
			return fmt.Errorf("unexpected share-repair activation lease")
		}
		return nil
	}
	if !bytes.Equal(wire, expected.wire) {
		return fmt.Errorf("missing or stale share-repair activation lease")
	}
	return nil
}

func ValidateLocalShareRepairSeatActivation(
	keyGroup string,
	seat group.MemberIndex,
) error {
	_, err := shareRepairActivationLeaseForBroadcast(keyGroup, seat)
	return err
}

// ShareRepairActivationReady is the fail-closed readiness predicate used by
// startup and the activation handshake. A zero manifest root is ready only
// when the native inventory has no recovered seats awaiting cutover.
func ShareRepairActivationReady(expectedRoot [32]byte) bool {
	shareRepairActivationRegistryState.RLock()
	defer shareRepairActivationRegistryState.RUnlock()
	if !shareRepairActivationRegistryState.configured {
		return false
	}
	registry := shareRepairActivationRegistryState.installed
	if registry == nil {
		return expectedRoot == [32]byte{} &&
			len(shareRepairActivationRegistryState.recoveredSeats) == 0
	}
	if registry.root != expectedRoot {
		return false
	}
	return validateLocalShareRepairActivationRegistryLocked(registry) == nil
}

// ShareRepairActivationTransportRequired reports whether ROAST must use its
// lease-carrying v2 transport. Pending recovered seats force v2 even before a
// registry exists, preventing a same-process fallback to legacy frames.
func ShareRepairActivationTransportRequired() bool {
	shareRepairActivationRegistryState.RLock()
	defer shareRepairActivationRegistryState.RUnlock()
	return shareRepairActivationRegistryState.installed != nil ||
		len(shareRepairActivationRegistryState.recoveredSeats) != 0
}

// ShareRepairActivationRegistryRoot computes the manifest pin for an artifact
// without installing process-global state. It is used by offline tooling/tests.
func ShareRepairActivationRegistryRoot(
	registry *ShareRepairActivationRegistry,
	authorityPublicKey ed25519.PublicKey,
) ([32]byte, error) {
	result := [32]byte{}
	_, root, err := validateShareRepairActivationRegistry(
		registry,
		authorityPublicKey,
	)
	if err != nil {
		return result, err
	}
	return root, nil
}

func resetShareRepairActivationRegistryForTest() {
	shareRepairActivationRegistryState.Lock()
	defer shareRepairActivationRegistryState.Unlock()
	shareRepairActivationRegistryState.configured = false
	shareRepairActivationRegistryState.localStoreFingerprint = [32]byte{}
	shareRepairActivationRegistryState.localSeats = nil
	shareRepairActivationRegistryState.recoveredSeats = nil
	shareRepairActivationRegistryState.installed = nil
}

// ResetShareRepairActivationStateForTest clears the process-global guard for
// cross-package tests. Production code must never call this function.
func ResetShareRepairActivationStateForTest() {
	resetShareRepairActivationRegistryForTest()
}
