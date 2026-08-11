//go:build frost_native

package signing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func testShareRepairHex32(value byte) string {
	return "0x" + hex.EncodeToString(bytes.Repeat([]byte{value}, 32))
}

func testShareRepairAuthorization(
	t *testing.T,
) (*ShareRepairAuthorization, ed25519.PrivateKey) {
	t.Helper()
	_, keyGroup := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x09}, 32))
	walletID := make([]byte, 32)
	keyGroup.X().FillBytes(walletID)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	authorization := &ShareRepairAuthorization{
		Schema:                     ShareRepairAuthorizationSchema,
		SessionID:                  "repair-wallet-a-seat-3-epoch-1",
		WalletID:                   "0x" + hex.EncodeToString(walletID),
		KeyGroup:                   hex.EncodeToString(keyGroup.SerializeCompressed()),
		PublicKeyPackageCommitment: testShareRepairHex32(0x31),
		TargetIdentifier:           3,
		HelperIdentifiers:          []uint16{1, 2},
		Threshold:                  2,
		ParticipantCount:           3,
		OldStoreFingerprint:        testShareRepairHex32(0x51),
		NewStoreFingerprint:        testShareRepairHex32(0x52),
		RecoveryEpoch:              1,
		IssuedAtUnix:               1_700_000_000,
		NotBeforeUnix:              1_700_000_000,
		ExpiresAtUnix:              1_700_003_600,
		Nonce:                      testShareRepairHex32(0x61),
	}
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatalf("compute authorization digest: %v", err)
	}
	authorization.SignatureHex = "0x" + hex.EncodeToString(
		ed25519.Sign(authority, digest[:]),
	)
	return authorization, authority
}

func testShareRepairTransportRoster(
	t *testing.T,
	authorization *ShareRepairAuthorization,
	authority ed25519.PrivateKey,
) *ShareRepairTransportRoster {
	t.Helper()
	authorizationDigest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatalf("compute authorization digest: %v", err)
	}
	participantIdentifiers := append(
		append([]uint16(nil), authorization.HelperIdentifiers...),
		authorization.TargetIdentifier,
	)
	participantPublicKeys := make(
		[]ShareRepairTransportPublicKey,
		len(participantIdentifiers),
	)
	for index, participantIdentifier := range participantIdentifiers {
		_, publicKey := btcec.PrivKeyFromBytes(
			bytes.Repeat([]byte{byte(0x11 * (index + 1))}, 32),
		)
		participantPublicKeys[index] = ShareRepairTransportPublicKey{
			ParticipantIdentifier: participantIdentifier,
			StoreFingerprint:      authorization.NewStoreFingerprint,
			PublicKeyHex:          hex.EncodeToString(publicKey.SerializeCompressed()),
		}
	}
	roster := &ShareRepairTransportRoster{
		Schema:                ShareRepairTransportRosterSchema,
		AuthorizationDigest:   "0x" + hex.EncodeToString(authorizationDigest[:]),
		ParticipantPublicKeys: participantPublicKeys,
	}
	digest, err := ComputeShareRepairTransportRosterDigest(roster, authorization)
	if err != nil {
		t.Fatalf("compute transport roster digest: %v", err)
	}
	roster.SignatureHex = "0x" + hex.EncodeToString(ed25519.Sign(authority, digest[:]))
	return roster
}

func cloneShareRepairTransportRoster(
	roster *ShareRepairTransportRoster,
) *ShareRepairTransportRoster {
	cloned := *roster
	cloned.ParticipantPublicKeys = append(
		[]ShareRepairTransportPublicKey(nil),
		roster.ParticipantPublicKeys...,
	)
	return &cloned
}

func testShareRepairActivationLease(
	t *testing.T,
	authorization *ShareRepairAuthorization,
	authority ed25519.PrivateKey,
) ShareRepairActivationLease {
	t.Helper()
	authorizationDigest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	lease := ShareRepairActivationLease{
		Schema:                  ShareRepairActivationLeaseSchema,
		Authorization:           *authorization,
		AuthorizationDigest:     "0x" + hex.EncodeToString(authorizationDigest[:]),
		OldStoreTombstoneDigest: testShareRepairHex32(0x71),
		ActivatedAtUnix:         authorization.ExpiresAtUnix + 1,
	}
	digest, err := ComputeShareRepairActivationLeaseDigest(
		&lease,
		authority.Public().(ed25519.PublicKey),
	)
	if err != nil {
		t.Fatalf("compute activation digest: %v", err)
	}
	lease.SignatureHex = "0x" + hex.EncodeToString(ed25519.Sign(authority, digest[:]))
	return lease
}

func TestShareRepairAuthorizationDigestFrozenVector(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	encoded, err := json.Marshal(authorization)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeShareRepairAuthorization(encoded); err != nil {
		t.Fatalf("strict authorization decoder rejected valid input: %v", err)
	}
	if _, err := DecodeShareRepairAuthorization(append(encoded, []byte(`{}`)...)); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("strict authorization decoder accepted trailing JSON: %v", err)
	}
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatal(err)
	}
	const expected = "aa8e36cbf287d988c6ed34bf0c38fd64c177500c768fbd3ea7c184b031d7511b"
	if actual := hex.EncodeToString(digest[:]); actual != expected {
		t.Fatalf("authorization digest changed: got [%s], want [%s]", actual, expected)
	}
	if _, err := validateShareRepairAuthorization(
		authorization,
		authority.Public().(ed25519.PublicKey),
		false,
	); err != nil {
		t.Fatalf("validate authorization: %v", err)
	}

	malformed := *authorization
	malformed.HelperIdentifiers = []uint16{2, 1}
	if _, err := ComputeShareRepairAuthorizationDigest(&malformed); err == nil {
		t.Fatal("expected an unsorted helper set to be rejected")
	}
	malformed = *authorization
	malformed.WalletID = testShareRepairHex32(0x99)
	if _, err := ComputeShareRepairAuthorizationDigest(&malformed); err == nil {
		t.Fatal("expected a key-group/wallet mismatch to be rejected")
	}
	malformed = *authorization
	malformed.ParticipantCount = 101
	if _, err := ComputeShareRepairAuthorizationDigest(&malformed); err == nil {
		t.Fatal("expected a participant count above the production group bound to be rejected")
	}
}

func TestShareRepairTransportRosterDigestFrozenVector(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	roster := testShareRepairTransportRoster(t, authorization, authority)
	digest, err := ComputeShareRepairTransportRosterDigest(roster, authorization)
	if err != nil {
		t.Fatal(err)
	}
	const expected = "1a46b993431f075de1adef58a668e8133cca8ca7070eb5d6ffbedee92d224364"
	if actual := hex.EncodeToString(digest[:]); actual != expected {
		t.Fatalf("transport roster digest changed: got [%s], want [%s]", actual, expected)
	}
	if err := ValidateShareRepairTransportRoster(
		roster,
		authorization,
		authority.Public().(ed25519.PublicKey),
	); err != nil {
		t.Fatalf("validate transport roster: %v", err)
	}
}

func TestShareRepairTransportRosterRejectsMalformedAndSubstitutedKeys(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	roster := testShareRepairTransportRoster(t, authorization, authority)

	tests := map[string]func(*ShareRepairTransportRoster){
		"wrong schema": func(candidate *ShareRepairTransportRoster) {
			candidate.Schema = "tbtc-frost-share-repair-transport-roster/v2"
		},
		"wrong authorization digest": func(candidate *ShareRepairTransportRoster) {
			candidate.AuthorizationDigest = testShareRepairHex32(0x99)
		},
		"wrong participant order": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[0], candidate.ParticipantPublicKeys[1] =
				candidate.ParticipantPublicKeys[1], candidate.ParticipantPublicKeys[0]
		},
		"missing target": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys = candidate.ParticipantPublicKeys[:2]
		},
		"duplicate public key": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[1].PublicKeyHex =
				candidate.ParticipantPublicKeys[0].PublicKeyHex
		},
		"uppercase public key": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[0].PublicKeyHex =
				strings.ToUpper(candidate.ParticipantPublicKeys[0].PublicKeyHex)
		},
		"prefixed public key": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[0].PublicKeyHex =
				"0x" + candidate.ParticipantPublicKeys[0].PublicKeyHex
		},
		"invalid SEC1 public key": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[0].PublicKeyHex = strings.Repeat("00", 33)
		},
		"invalid store fingerprint": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[0].StoreFingerprint = "0x00"
		},
		"target wrong store": func(candidate *ShareRepairTransportRoster) {
			candidate.ParticipantPublicKeys[len(candidate.ParticipantPublicKeys)-1].StoreFingerprint =
				testShareRepairHex32(0x99)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := cloneShareRepairTransportRoster(roster)
			mutate(candidate)
			if _, err := ComputeShareRepairTransportRosterDigest(
				candidate,
				authorization,
			); err == nil {
				t.Fatal("malformed transport roster was accepted")
			}
		})
	}

	t.Run("caller key substitution", func(t *testing.T) {
		candidate := cloneShareRepairTransportRoster(roster)
		_, substitutedPublicKey := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x44}, 32))
		candidate.ParticipantPublicKeys[0].PublicKeyHex =
			hex.EncodeToString(substitutedPublicKey.SerializeCompressed())
		if _, err := ComputeShareRepairTransportRosterDigest(
			candidate,
			authorization,
		); err != nil {
			t.Fatalf("structurally valid substituted roster was rejected too early: %v", err)
		}
		if err := ValidateShareRepairTransportRoster(
			candidate,
			authorization,
			authority.Public().(ed25519.PublicKey),
		); err == nil || !strings.Contains(err.Error(), "signature is invalid") {
			t.Fatalf("authority-unapproved key substitution was accepted: %v", err)
		}
	})

	t.Run("noncanonical signature", func(t *testing.T) {
		candidate := cloneShareRepairTransportRoster(roster)
		candidate.SignatureHex = strings.ToUpper(candidate.SignatureHex)
		if err := ValidateShareRepairTransportRoster(
			candidate,
			authorization,
			authority.Public().(ed25519.PublicKey),
		); err == nil {
			t.Fatal("noncanonical transport roster signature was accepted")
		}
	})

	t.Run("different authority", func(t *testing.T) {
		differentAuthority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x24}, 32))
		if err := ValidateShareRepairTransportRoster(
			roster,
			authorization,
			differentAuthority.Public().(ed25519.PublicKey),
		); err == nil {
			t.Fatal("transport roster from a different authority was accepted")
		}
	})
}

func TestShareRepairRecoveryBundleStrictDecode(t *testing.T) {
	authorization, authority := testShareRepairAuthorization(t)
	bundle := ShareRepairRecoveryBundle{
		Schema:          ShareRepairRecoveryBundleSchema,
		Authorization:   *authorization,
		TransportRoster: *testShareRepairTransportRoster(t, authorization, authority),
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeShareRepairRecoveryBundle(payload)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != bundle.Schema ||
		decoded.TransportRoster.SignatureHex != bundle.TransportRoster.SignatureHex {
		t.Fatalf("recovery bundle changed across strict decode: %+v", decoded)
	}
	if _, err := DecodeShareRepairRecoveryBundle(
		append(payload, []byte(`{}`)...),
	); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("recovery bundle accepted trailing JSON: %v", err)
	}

	wrongSchema := bundle
	wrongSchema.Schema = "tbtc-frost-share-repair-bundle/v2"
	payload, err = json.Marshal(wrongSchema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeShareRepairRecoveryBundle(payload); err == nil {
		t.Fatal("recovery bundle accepted an unsupported schema")
	}
}

func configureTestShareRepairActivationGuard(
	t *testing.T,
	authorization *ShareRepairAuthorization,
	storeFingerprint [32]byte,
	recovered bool,
) {
	t.Helper()
	walletID, err := parseCanonicalShareRepairHex32(authorization.WalletID, "wallet_id")
	if err != nil {
		t.Fatal(err)
	}
	inventory := &NativeTBTCSignerRetainedKeyPackageInventory{
		Schema:           NativeTBTCSignerRetainedKeyPackageInventorySchema,
		StoreFingerprint: storeFingerprint,
		Entries: []NativeTBTCSignerRetainedKeyGroup{
			{
				WalletID: walletID,
				KeyGroup: authorization.KeyGroup,
				KeyPackages: []NativeTBTCSignerRetainedKeyPackage{
					{ParticipantSeat: authorization.TargetIdentifier},
				},
			},
		},
	}
	if recovered {
		digest, err := ComputeShareRepairAuthorizationDigest(authorization)
		if err != nil {
			t.Fatal(err)
		}
		inventory.Schema = NativeTBTCSignerRetainedKeyPackageInventoryRecoverySchema
		inventory.RecoveredSeats = []NativeTBTCSignerRecoveredSeat{
			{
				WalletID:               walletID,
				KeyGroup:               authorization.KeyGroup,
				ParticipantSeat:        authorization.TargetIdentifier,
				RecoveryEpoch:          authorization.RecoveryEpoch,
				AuthorizationDigest:    digest,
				ActiveStoreFingerprint: storeFingerprint,
			},
		}
		inventory.RecoveryActivationCommitment =
			ComputeNativeTBTCSignerRecoveredSeatActivationCommitment(
				inventory.RecoveredSeats,
			)
	}
	if err := ConfigureShareRepairActivationGuard(inventory); err != nil {
		t.Fatalf("configure share-repair activation guard: %v", err)
	}
}

func TestShareRepairActivationRegistryEnforcesStoreCutover(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, authority := testShareRepairAuthorization(t)
	lease := testShareRepairActivationLease(t, authorization, authority)
	registry := &ShareRepairActivationRegistry{
		Schema: ShareRepairActivationRegistrySchema,
		Leases: []ShareRepairActivationLease{lease},
	}
	publicKey := authority.Public().(ed25519.PublicKey)
	root, err := ShareRepairActivationRegistryRoot(registry, publicKey)
	if err != nil {
		t.Fatalf("compute registry root: %v", err)
	}
	payload, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	newStore, err := parseCanonicalShareRepairHex32(
		authorization.NewStoreFingerprint,
		"new_store_fingerprint",
	)
	if err != nil {
		t.Fatal(err)
	}
	oldStore, err := parseCanonicalShareRepairHex32(
		authorization.OldStoreFingerprint,
		"old_store_fingerprint",
	)
	if err != nil {
		t.Fatal(err)
	}
	configureTestShareRepairActivationGuard(t, authorization, newStore, true)

	if err := InstallShareRepairActivationRegistry(
		append(append([]byte(nil), payload...), []byte(`{}`)...),
		publicKey,
		root,
		newStore,
	); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("expected trailing registry JSON to fail, got [%v]", err)
	}
	if err := InstallShareRepairActivationRegistry(payload, publicKey, root, newStore); err != nil {
		t.Fatalf("install registry: %v", err)
	}
	if CurrentShareRepairActivationRegistryRoot() != root {
		t.Fatal("installed registry root mismatch")
	}
	if err := ValidateLocalShareRepairSeatActivation(
		authorization.KeyGroup,
		group.MemberIndex(authorization.TargetIdentifier),
	); err != nil {
		t.Fatalf("new store rejected recovered seat: %v", err)
	}
	wire, err := shareRepairActivationLeaseForBroadcast(
		authorization.KeyGroup,
		group.MemberIndex(authorization.TargetIdentifier),
	)
	if err != nil || len(wire) == 0 {
		t.Fatalf("missing activation lease for recovered seat: [%v]", err)
	}
	if err := validateShareRepairActivationLeaseForMessage(
		authorization.KeyGroup,
		group.MemberIndex(authorization.TargetIdentifier),
		wire,
	); err != nil {
		t.Fatalf("exact activation lease rejected: %v", err)
	}
	if err := validateShareRepairActivationLeaseForMessage(
		authorization.KeyGroup,
		group.MemberIndex(authorization.TargetIdentifier),
		nil,
	); err == nil {
		t.Fatal("expected a recovered seat without its exact lease to be rejected")
	}
	if err := validateShareRepairActivationLeaseForMessage(
		authorization.KeyGroup,
		1,
		nil,
	); err != nil {
		t.Fatalf("unrecovered seat unexpectedly required a lease: %v", err)
	}

	resetShareRepairActivationRegistryForTest()
	configureTestShareRepairActivationGuard(t, authorization, oldStore, false)
	if err := InstallShareRepairActivationRegistry(
		payload,
		publicKey,
		root,
		oldStore,
	); err == nil {
		t.Fatal("expected the old store process to reject the recovered-seat registry")
	}
}

func TestShareRepairActivationRegistryRejectsNonCanonicalAndWrongRoot(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, authority := testShareRepairAuthorization(t)
	lease := testShareRepairActivationLease(t, authorization, authority)
	publicKey := authority.Public().(ed25519.PublicKey)
	duplicate := &ShareRepairActivationRegistry{
		Schema: ShareRepairActivationRegistrySchema,
		Leases: []ShareRepairActivationLease{lease, lease},
	}
	if _, err := ShareRepairActivationRegistryRoot(duplicate, publicKey); err == nil {
		t.Fatal("expected duplicate activation leases to be rejected")
	}
	registry := &ShareRepairActivationRegistry{
		Schema: ShareRepairActivationRegistrySchema,
		Leases: []ShareRepairActivationLease{lease},
	}
	payload, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	newStore, _ := parseCanonicalShareRepairHex32(
		authorization.NewStoreFingerprint,
		"new_store_fingerprint",
	)
	if err := InstallShareRepairActivationRegistry(
		payload,
		publicKey,
		[32]byte{0xff},
		newStore,
	); err == nil {
		t.Fatal("expected a registry root different from the signed manifest to fail")
	}
}
