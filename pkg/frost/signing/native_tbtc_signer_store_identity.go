package signing

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"regexp"
	"strings"
)

const (
	// NativeTBTCSignerDurableStoreIdentitySchema is the versioned readback
	// contract implemented by libfrost_tbtc. The identity is about the store
	// the signer actually opened and locked. It is deliberately not the
	// fingerprint of the signer init configuration, which cannot prove where
	// session replay markers and key packages are being persisted.
	NativeTBTCSignerDurableStoreIdentitySchema = "tbtc-signer-durable-session-store-identity/v1"

	nativeTBTCSignerDurableStoreIdentityDomain = "tbtc-signer-durable-session-store-fingerprint-v1\x00"
)

var nativeTBTCSignerStoreBackendPattern = regexp.MustCompile(
	`^[a-z0-9][a-z0-9._-]{0,63}$`,
)

// NativeTBTCSignerDurableStoreIdentity is a validated runtime identity for
// the durable store libfrost_tbtc is actively using. StoreID must be created
// by the signer and persist across process restarts. The path, filesystem,
// and lock fingerprints must be derived from no-follow handles held by the
// signer, not copied from configuration text.
//
// The state file itself is atomically replaced on every write, so its inode is
// not a stable store identity. The signer-side contract instead binds the
// persistent store ID, canonical opened path, storage root, and exclusive lock
// object. It must fail readback if a symlink or replacement makes those opened
// identities differ from the live path lookup.
type NativeTBTCSignerDurableStoreIdentity struct {
	Schema                   string
	Backend                  string
	StoreID                  [32]byte
	CanonicalPathFingerprint [32]byte
	FilesystemFingerprint    [32]byte
	LockFingerprint          [32]byte
	Fingerprint              [32]byte
}

type nativeTBTCSignerDurableStoreIdentityWire struct {
	Schema                   string `json:"schema"`
	Backend                  string `json:"backend"`
	StoreID                  string `json:"store_id"`
	CanonicalPathFingerprint string `json:"canonical_path_fingerprint"`
	FilesystemFingerprint    string `json:"filesystem_fingerprint"`
	LockFingerprint          string `json:"lock_fingerprint"`
	Fingerprint              string `json:"fingerprint"`
	Durable                  *bool  `json:"durable"`
	ExclusiveLockHeld        *bool  `json:"exclusive_lock_held"`
	SymlinkFree              *bool  `json:"symlink_free"`
	ReplacementProtected     *bool  `json:"replacement_protected"`
}

// DecodeNativeTBTCSignerDurableStoreIdentity validates a libfrost_tbtc
// readback. The four affirmative safety claims are mandatory rather than
// advisory: missing and false are both rejected.
func DecodeNativeTBTCSignerDurableStoreIdentity(
	payload []byte,
) (*NativeTBTCSignerDurableStoreIdentity, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	wire := &nativeTBTCSignerDurableStoreIdentityWire{}
	if err := decoder.Decode(wire); err != nil {
		return nil, fmt.Errorf("cannot decode durable signer store identity: %w", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return nil, err
	}

	if wire.Schema != NativeTBTCSignerDurableStoreIdentitySchema {
		return nil, fmt.Errorf("unsupported durable signer store identity schema")
	}
	if !nativeTBTCSignerStoreBackendPattern.MatchString(wire.Backend) {
		return nil, fmt.Errorf("invalid durable signer store backend")
	}
	for label, value := range map[string]*bool{
		"durability":             wire.Durable,
		"exclusive lock":         wire.ExclusiveLockHeld,
		"symlink safety":         wire.SymlinkFree,
		"replacement protection": wire.ReplacementProtected,
	} {
		if value == nil || !*value {
			return nil, fmt.Errorf("durable signer store %s is not proven", label)
		}
	}

	identity := &NativeTBTCSignerDurableStoreIdentity{
		Schema:  wire.Schema,
		Backend: wire.Backend,
	}
	values := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"store ID", wire.StoreID, &identity.StoreID},
		{"canonical path fingerprint", wire.CanonicalPathFingerprint, &identity.CanonicalPathFingerprint},
		{"filesystem fingerprint", wire.FilesystemFingerprint, &identity.FilesystemFingerprint},
		{"lock fingerprint", wire.LockFingerprint, &identity.LockFingerprint},
		{"store fingerprint", wire.Fingerprint, &identity.Fingerprint},
	}
	for _, value := range values {
		decoded, err := decodeNativeTBTCSignerStoreBytes32(value.encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid durable signer %s: %w", value.label, err)
		}
		*value.destination = decoded
	}

	computed, err := ComputeNativeTBTCSignerDurableStoreFingerprint(identity)
	if err != nil {
		return nil, err
	}
	if computed != identity.Fingerprint {
		return nil, fmt.Errorf(
			"durable signer store fingerprint does not bind the reported runtime identity",
		)
	}

	return identity, nil
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot decode durable signer store identity: %w", err)
	}
	return fmt.Errorf("durable signer store identity contains trailing JSON")
}

func decodeNativeTBTCSignerStoreBytes32(value string) ([32]byte, error) {
	var result [32]byte
	if value != strings.ToLower(value) || !strings.HasPrefix(value, "0x") ||
		len(value) != 66 {
		return result, fmt.Errorf("expected canonical lowercase 0x-prefixed bytes32")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != len(result) {
		return result, fmt.Errorf("expected canonical lowercase 0x-prefixed bytes32")
	}
	copy(result[:], decoded)
	if result == [32]byte{} {
		return result, fmt.Errorf("value is zero")
	}
	return result, nil
}

// ComputeNativeTBTCSignerDurableStoreFingerprint derives the manifest-bound
// fingerprint from runtime store identity, not signer configuration content.
// All variable-width fields are length-prefixed to keep the transcript
// unambiguous.
func ComputeNativeTBTCSignerDurableStoreFingerprint(
	identity *NativeTBTCSignerDurableStoreIdentity,
) ([32]byte, error) {
	if identity == nil ||
		identity.Schema != NativeTBTCSignerDurableStoreIdentitySchema ||
		!nativeTBTCSignerStoreBackendPattern.MatchString(identity.Backend) ||
		identity.StoreID == [32]byte{} ||
		identity.CanonicalPathFingerprint == [32]byte{} ||
		identity.FilesystemFingerprint == [32]byte{} ||
		identity.LockFingerprint == [32]byte{} {
		return [32]byte{}, fmt.Errorf("durable signer store identity is incomplete")
	}

	digest := sha256.New()
	digest.Write([]byte(nativeTBTCSignerDurableStoreIdentityDomain))
	writeNativeTBTCSignerStoreFingerprintField(digest, []byte(identity.Schema))
	writeNativeTBTCSignerStoreFingerprintField(digest, []byte(identity.Backend))
	writeNativeTBTCSignerStoreFingerprintField(digest, identity.StoreID[:])
	writeNativeTBTCSignerStoreFingerprintField(
		digest,
		identity.CanonicalPathFingerprint[:],
	)
	writeNativeTBTCSignerStoreFingerprintField(
		digest,
		identity.FilesystemFingerprint[:],
	)
	writeNativeTBTCSignerStoreFingerprintField(digest, identity.LockFingerprint[:])

	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

func writeNativeTBTCSignerStoreFingerprintField(
	destination hash.Hash,
	value []byte,
) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	// hash.Hash.Write is documented to never return an error. Discard both
	// results explicitly so this infallible transcript operation cannot be
	// mistaken for an unchecked fallible write.
	_, _ = destination.Write(length[:])
	_, _ = destination.Write(value)
}
