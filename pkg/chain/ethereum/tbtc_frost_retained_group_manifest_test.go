package ethereum

import (
	"fmt"
	"strings"
	"testing"
)

func testManifestHex32(value byte) string {
	return fmt.Sprintf("0x%02x%s", value, strings.Repeat("00", 31))
}

func testFrostJournalActivationManifest() *frostPreSignActivationManifest {
	checkpointHash := testManifestHex32(0x02)
	return &frostPreSignActivationManifest{
		Schema:             frostPreSignManifestVersion,
		ActivationSequence: 1,
		ActivationID:       testManifestHex32(0x01),
		Environment:        "test",
		manifestHash:       [32]byte{0x99},
		Ethereum: frostPreSignManifestEthereum{
			ChainID:                         1,
			Checkpoint:                      frostPreSignManifestPoint{BlockNumber: 10, BlockHash: checkpointHash},
			StoreID:                         "primary-store",
			SourceTrustDomainID:             "primary-source",
			SourceEndpointFingerprint:       testManifestHex32(0x03),
			SourceOperatorFingerprint:       testManifestHex32(0x04),
			SourceHistoryStoreID:            "primary-history",
			SourceHistoryStoreFingerprint:   testManifestHex32(0x05),
			VerifierTrustDomainID:           "primary-verifier",
			VerifierEndpointFingerprint:     testManifestHex32(0x06),
			VerifierOperatorFingerprint:     testManifestHex32(0x07),
			VerifierHistoryStoreID:          "verifier-history",
			VerifierHistoryStoreFingerprint: testManifestHex32(0x08),
		},
		FrostSigner: frostPreSignManifestFrostSigner{
			TrustDomainID:                       "runtime-signer",
			DurableSessionStoreFingerprint:      testManifestHex32(0x09),
			ProtocolID:                          testManifestHex32(0x10),
			ReservationProtocolID:               testManifestHex32(0x11),
			BitcoinOutboxProtocolID:             testManifestHex32(0x12),
			SigningPolicyHash:                   testManifestHex32(0x13),
			AttestationSignerKeyHash:            testManifestHex32(0x14),
			HandshakeEndpointFingerprint:        testManifestHex32(0x15),
			HandshakeOperatorFingerprint:        testManifestHex32(0x16),
			Threshold:                           51,
			MaximumGroupSize:                    100,
			RetainedGroupInventoryProtocolID:    testManifestHex32(0x17),
			ExactRetainedGroupInventoryRequired: true,
			FinalizedReservationReceiptRequired: true,
			ExactReservationIdentityRequired:    true,
			AuthorizationRootRequired:           true,
			DurableSessionPersistenceRequired:   true,
			DurableBitcoinOutboxRequired:        true,
			QuarantineFailClosed:                true,
			CanonicalJournal: frostPreSignManifestCanonicalJournal{
				StoreID:                   "canonical-journal-store",
				StoreFingerprint:          testManifestHex32(0x20),
				ClusterFingerprint:        testManifestHex32(0x21),
				Checkpoint:                frostPreSignManifestPoint{BlockNumber: 1, BlockHash: testManifestHex32(0x28)},
				DescriptorSetHash:         testManifestHex32(0x22),
				SourceTrustDomainID:       "independent-journal-source",
				SourceEndpointFingerprint: testManifestHex32(0x23),
				SourceOperatorFingerprint: testManifestHex32(0x24),
				MinimumGeneration:         7,
			},
			QuarantineJournal: frostPreSignManifestQuarantineJournal{
				ProtocolID:         testManifestHex32(0x25),
				StoreID:            "quarantine-journal-store",
				StoreFingerprint:   testManifestHex32(0x26),
				ClusterFingerprint: testManifestHex32(0x27),
				MinimumGeneration:  0,
			},
		},
	}
}

func TestValidateFrostPreSignActivationManifest_CanonicalJournal(t *testing.T) {
	manifest := testFrostJournalActivationManifest()
	if err := validateFrostPreSignActivationManifest(manifest); err != nil {
		t.Fatal(err)
	}
	t.Run("source endpoint alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.CanonicalJournal.SourceEndpointFingerprint =
			manifest.Ethereum.SourceEndpointFingerprint
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "not independent") {
			t.Fatalf("expected independent-source validation failure, got [%v]", err)
		}
	})
	t.Run("quarantine store alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.StoreFingerprint =
			manifest.FrostSigner.CanonicalJournal.StoreFingerprint
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "storage identities") {
			t.Fatalf("expected independent-store validation failure, got [%v]", err)
		}
	})
	t.Run("malformed durable session fingerprint", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.DurableSessionStoreFingerprint = "operator-authored-label"
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "durable session store fingerprint") {
			t.Fatalf("expected durable-session fingerprint failure, got [%v]", err)
		}
	})
}

func TestFrostPreSignDecodeStrictJSON_RejectsUnknownCanonicalJournalField(t *testing.T) {
	data := []byte(`{"storeID":"id","unknown":true}`)
	if err := frostPreSignDecodeStrictJSON(
		data,
		&frostPreSignManifestCanonicalJournal{},
	); err == nil {
		t.Fatal("expected unknown canonical-journal field to be rejected")
	}
}
