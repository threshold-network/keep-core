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

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const (
	ShareRepairAuthorizationSchema       = "tbtc-frost-share-repair-authorization/v1"
	ShareRepairInstallResultSchema       = "tbtc-frost-share-repair-install-result/v1"
	ShareRepairActivationLeaseSchema     = "tbtc-frost-share-repair-activation/v1"
	ShareRepairActivationRegistrySchema  = "tbtc-frost-share-repair-activation-registry/v1"
	shareRepairAuthorizationDomain       = "tbtc-frost-share-repair-authorization/v1\x00"
	shareRepairActivationLeaseDomain     = "tbtc-frost-share-repair-activation/v1\x00"
	shareRepairActivationRegistryDomain  = "tbtc-frost-share-repair-activation-registry/v1\x00"
	shareRepairMaximumAuthorizationAge   = 24 * time.Hour
	shareRepairMaximumActivationRegistry = 4096
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

// NativeShareRepairDelta and NativeShareRepairSigma contain secret scalars.
// Callers must zero Data as soon as the next native phase has copied it.
type NativeShareRepairDelta struct {
	ContextDigest       string
	SenderIdentifier    uint16
	RecipientIdentifier uint16
	Data                []byte
}

type NativeShareRepairSigma struct {
	ContextDigest    string
	HelperIdentifier uint16
	Data             []byte
}

type NativeShareRepairPart1Result struct {
	ContextDigest    string
	HelperIdentifier uint16
	PublicKeyPackage *NativeFROSTPublicKeyPackage
	Deltas           []*NativeShareRepairDelta
}

type NativeShareRepairPart2Result struct {
	ContextDigest string
	Sigma         *NativeShareRepairSigma
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
	ShareRepairPart1(
		authorization *ShareRepairAuthorization,
		helperIdentifier uint16,
	) (*NativeShareRepairPart1Result, error)
	ShareRepairPart2(
		authorization *ShareRepairAuthorization,
		helperIdentifier uint16,
		deltas []*NativeShareRepairDelta,
	) (*NativeShareRepairPart2Result, error)
	InstallRepairedShare(
		authorization *ShareRepairAuthorization,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
		sigmas []*NativeShareRepairSigma,
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

// encodeShareRepairSecretHexJSON and decodeShareRepairSecretHexJSON keep repair
// scalars in mutable byte slices. Using ordinary Go strings here would leave
// immutable plaintext hex copies behind after the FFI or ECIES operation.
func encodeShareRepairSecretHexJSON(data []byte) (json.RawMessage, error) {
	if len(data) != 32 {
		return nil, fmt.Errorf("share-repair secret scalar must be 32 bytes")
	}
	result := make([]byte, 66)
	result[0] = '"'
	hex.Encode(result[1:65], data)
	result[65] = '"'
	return json.RawMessage(result), nil
}

func decodeShareRepairSecretHexJSON(data json.RawMessage) ([]byte, error) {
	if len(data) != 66 || data[0] != '"' || data[65] != '"' {
		return nil, fmt.Errorf("share-repair secret scalar is not canonical JSON hex")
	}
	for _, value := range data[1:65] {
		if value >= 'A' && value <= 'F' {
			return nil, fmt.Errorf("share-repair secret scalar is not lowercase hex")
		}
	}
	result := make([]byte, 32)
	decoded, err := hex.Decode(result, data[1:65])
	if err != nil || decoded != len(result) {
		zeroBytes(result)
		return nil, fmt.Errorf("share-repair secret scalar is invalid")
	}
	return result, nil
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
	if strings.TrimSpace(authorization.SessionID) == "" ||
		authorization.SessionID != strings.TrimSpace(authorization.SessionID) ||
		len(authorization.SessionID) > 256 {
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
