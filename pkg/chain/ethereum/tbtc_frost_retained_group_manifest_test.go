package ethereum

import (
	"fmt"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

func testManifestHex32(value byte) string {
	return fmt.Sprintf("0x%02x%s", value, strings.Repeat("00", 31))
}

func testFrostRetainedSourceIdentity() (
	tbtc.FrostRetainedGroupHistoryIdentity,
	frostPreSignManifestRetainedSourceIdentity,
) {
	protocolID := tbtc.FrostRetainedGroupTLSExporterProtocolID()
	exportIdentity := tbtc.FrostRetainedGroupEndpointIdentity{
		Schema:                    "tbtc-frost-retained-group-endpoint-identity/v1",
		Role:                      "retained-history-export",
		TrustDomainID:             "export.retained.example",
		CanonicalEndpoint:         "https://export.example:443/history",
		CanonicalDNSName:          "export.example",
		ResolvedDNSName:           "export-origin.example",
		ResolvedAddressSetHash:    [32]byte{0x60},
		TLSLeafSPKIHash:           [32]byte{0x61},
		ServiceIdentity:           "spiffe://export.retained.example/export",
		BackendServiceFingerprint: [32]byte{0x62},
		OperatorFingerprint:       [32]byte{0x63},
		AttestationKeyHash:        [32]byte{0x64},
		TLSExporterProtocolID:     protocolID,
	}
	exportIdentity.EndpointFingerprint =
		tbtc.ComputeFrostRetainedGroupEndpointIdentityFingerprint(exportIdentity)
	verifierIdentity := tbtc.FrostRetainedGroupEndpointIdentity{
		Schema:                    "tbtc-frost-retained-group-endpoint-identity/v1",
		Role:                      "retained-history-verifier",
		TrustDomainID:             "verifier.retained.example",
		CanonicalEndpoint:         "https://verifier.example:443/rpc",
		CanonicalDNSName:          "verifier.example",
		ResolvedDNSName:           "verifier-origin.example",
		ResolvedAddressSetHash:    [32]byte{0x65},
		TLSLeafSPKIHash:           [32]byte{0x66},
		ServiceIdentity:           "spiffe://verifier.retained.example/verifier",
		BackendServiceFingerprint: [32]byte{0x67},
		OperatorFingerprint:       [32]byte{0x68},
		AttestationKeyHash:        [32]byte{0x69},
		TLSExporterProtocolID:     protocolID,
	}
	verifierIdentity.EndpointFingerprint =
		tbtc.ComputeFrostRetainedGroupEndpointIdentityFingerprint(verifierIdentity)
	identity := tbtc.FrostRetainedGroupHistoryIdentity{
		Schema:               "tbtc-frost-retained-group-source-identity/v1",
		TrustDomainID:        "independent-journal-source",
		OperatorFingerprint:  exportIdentity.OperatorFingerprint,
		HistorySignerKeyHash: [32]byte{0x6a},
		Export:               exportIdentity,
		Verifier:             verifierIdentity,
	}
	identity.EndpointFingerprint =
		tbtc.ComputeFrostRetainedGroupSourceEndpointFingerprint(identity)
	toEndpoint := func(
		value tbtc.FrostRetainedGroupEndpointIdentity,
	) frostPreSignManifestRetainedEndpointIdentity {
		return frostPreSignManifestRetainedEndpointIdentity{
			Schema:                    value.Schema,
			Role:                      value.Role,
			TrustDomainID:             value.TrustDomainID,
			CanonicalEndpoint:         value.CanonicalEndpoint,
			CanonicalDNSName:          value.CanonicalDNSName,
			ResolvedDNSName:           value.ResolvedDNSName,
			ResolvedAddressSetHash:    fmt.Sprintf("0x%x", value.ResolvedAddressSetHash),
			TLSLeafSPKIHash:           fmt.Sprintf("0x%x", value.TLSLeafSPKIHash),
			ServiceIdentity:           value.ServiceIdentity,
			BackendServiceFingerprint: fmt.Sprintf("0x%x", value.BackendServiceFingerprint),
			OperatorFingerprint:       fmt.Sprintf("0x%x", value.OperatorFingerprint),
			AttestationKeyHash:        fmt.Sprintf("0x%x", value.AttestationKeyHash),
			TLSExporterProtocolID:     fmt.Sprintf("0x%x", value.TLSExporterProtocolID),
			EndpointFingerprint:       fmt.Sprintf("0x%x", value.EndpointFingerprint),
		}
	}
	return identity, frostPreSignManifestRetainedSourceIdentity{
		Schema:               identity.Schema,
		TrustDomainID:        identity.TrustDomainID,
		EndpointFingerprint:  fmt.Sprintf("0x%x", identity.EndpointFingerprint),
		OperatorFingerprint:  fmt.Sprintf("0x%x", identity.OperatorFingerprint),
		HistorySignerKeyHash: fmt.Sprintf("0x%x", identity.HistorySignerKeyHash),
		Export:               toEndpoint(identity.Export),
		Verifier:             toEndpoint(identity.Verifier),
	}
}

func testFrostJournalActivationManifest() *frostPreSignActivationManifest {
	checkpointHash := testManifestHex32(0x02)
	sourceIdentity, wireSourceIdentity := testFrostRetainedSourceIdentity()
	manifest := &frostPreSignActivationManifest{
		Schema:                     frostPreSignManifestVersion,
		ActivationSequence:         1,
		ActivationID:               testManifestHex32(0x01),
		Environment:                "test",
		manifestHash:               [32]byte{0x99},
		activationAuthorityKeyHash: [32]byte{0x40},
		Ethereum: frostPreSignManifestEthereum{
			ChainID:                         1,
			GenesisBlockHash:                testManifestHex32(0x30),
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
				SourceEndpointFingerprint: fmt.Sprintf("0x%x", sourceIdentity.EndpointFingerprint),
				SourceOperatorFingerprint: fmt.Sprintf("0x%x", sourceIdentity.OperatorFingerprint),
				SourceIdentity:            wireSourceIdentity,
				MinimumGeneration:         7,
			},
			QuarantineJournal: frostPreSignManifestQuarantineJournal{
				ProtocolID:                   testManifestHex32(0x25),
				LiftProtocolID:               testManifestHex32(0x29),
				TombstoneProtocolID:          testManifestHex32(0x2a),
				CheckpointAuthorityThreshold: 2,
				CheckpointAuthorities: []frostPreSignManifestLiftAuthority{
					{AuthorityID: "checkpoint-1", PublicKeySPKIHash: testManifestHex32(0x2b)},
					{AuthorityID: "checkpoint-2", PublicKeySPKIHash: testManifestHex32(0x2c)},
					{AuthorityID: "checkpoint-3", PublicKeySPKIHash: testManifestHex32(0x2d)},
				},
				CheckpointMinimumSequence: 1,
				CheckpointPredecessorHash: testManifestHex32(0x00),
				LiftAuthorityThreshold:    2,
				LiftAuthorities: []frostPreSignManifestLiftAuthority{
					{AuthorityID: "authority-1", PublicKeySPKIHash: testManifestHex32(0x36)},
					{AuthorityID: "authority-2", PublicKeySPKIHash: testManifestHex32(0x37)},
					{AuthorityID: "authority-3", PublicKeySPKIHash: testManifestHex32(0x38)},
				},
				StoreID:            "quarantine-journal-store",
				StoreFingerprint:   testManifestHex32(0x26),
				ClusterFingerprint: testManifestHex32(0x27),
				MinimumGeneration:  0,
			},
			NativeSignerAnchor: frostPreSignManifestNativeSignerAnchor{
				ProtocolID:                      testManifestHex32(0x40),
				StreamID:                        testManifestHex32(0x41),
				TrustDomainID:                   "independent-native-anchor",
				EndpointLeafSPKIHash:            testManifestHex32(0x42),
				OnlineKeyHash:                   testManifestHex32(0x43),
				OperatorFingerprint:             testManifestHex32(0x44),
				HistoryStoreID:                  "native-anchor-history",
				HistoryStoreFingerprint:         testManifestHex32(0x45),
				HistoryClusterFingerprint:       testManifestHex32(0x46),
				OfflineAuthorityHash:            testManifestHex32(0x47),
				ClientSPKIHash:                  testManifestHex32(0x48),
				SignerStoreFingerprint:          testManifestHex32(0x09),
				TransportBinding:                testManifestHex32(0x49),
				WitnessMaximumRecords:           100,
				WitnessRotationThresholdRecords: 8,
			},
		},
	}
	anchorIdentity, err := frostPreSignNativeSignerAnchorIdentity(manifest)
	if err != nil {
		panic(err)
	}
	streamID := tbtc.ComputeFrostNativeSignerAnchorStreamID(anchorIdentity)
	manifest.FrostSigner.NativeSignerAnchor.StreamID =
		fmt.Sprintf("0x%x", streamID[:])
	return manifest
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
			!strings.Contains(err.Error(), "differs from its aggregate") {
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
	t.Run("valid share-repair activation registry root", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.ShareRepairActivationRegistryRoot = testManifestHex32(0x18)
		if err := validateFrostPreSignActivationManifest(manifest); err != nil {
			t.Fatalf("expected optional share-repair registry root to validate: %v", err)
		}
	})
	t.Run("malformed share-repair activation registry root", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.ShareRepairActivationRegistryRoot = "operator-authored-label"
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "share-repair activation registry root") {
			t.Fatalf("expected share-repair registry-root failure, got [%v]", err)
		}
	})
	t.Run("native anchor stream mismatch", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.NativeSignerAnchor.StreamID = testManifestHex32(0xee)
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "stream ID mismatch") {
			t.Fatalf("expected native anchor stream failure, got [%v]", err)
		}
	})
	t.Run("native anchor store mismatch", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.NativeSignerAnchor.SignerStoreFingerprint =
			testManifestHex32(0xee)
		// The stream ID commits to the store fingerprint, so recompute it over
		// the mutated identity: otherwise the stream-ID pin fires first and
		// the store check under test is unreachable.
		anchorIdentity, err := frostPreSignNativeSignerAnchorIdentity(manifest)
		if err != nil {
			t.Fatalf("mutated anchor identity: %v", err)
		}
		streamID := tbtc.ComputeFrostNativeSignerAnchorStreamID(anchorIdentity)
		manifest.FrostSigner.NativeSignerAnchor.StreamID =
			fmt.Sprintf("0x%x", streamID[:])
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "differs from the durable signer store") {
			t.Fatalf("expected native anchor store failure, got [%v]", err)
		}
	})
	t.Run("native anchor witness geometry", func(t *testing.T) {
		for name, mutate := range map[string]func(*frostPreSignManifestNativeSignerAnchor){
			"maximum too large": func(anchor *frostPreSignManifestNativeSignerAnchor) {
				anchor.WitnessMaximumRecords = 1_000_001
			},
			"rotation leaves no crash margin": func(anchor *frostPreSignManifestNativeSignerAnchor) {
				anchor.WitnessMaximumRecords = 10
				anchor.WitnessRotationThresholdRecords = 9
			},
		} {
			t.Run(name, func(t *testing.T) {
				manifest := testFrostJournalActivationManifest()
				mutate(&manifest.FrostSigner.NativeSignerAnchor)
				if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
					!strings.Contains(err.Error(), "witness geometry") {
					t.Fatalf("expected native anchor geometry failure, got [%v]", err)
				}
			})
		}
	})
	t.Run("native anchor authority alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.NativeSignerAnchor.OnlineKeyHash =
			manifest.FrostSigner.NativeSignerAnchor.ClientSPKIHash
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "authority keys are not independent") {
			t.Fatalf("expected native anchor authority failure, got [%v]", err)
		}
	})
	t.Run("history signer aliases runtime attestation", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.AttestationSignerKeyHash =
			manifest.FrostSigner.CanonicalJournal.SourceIdentity.
				HistorySignerKeyHash
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "retained history signer") {
			t.Fatalf("expected history-signer role alias rejection, got [%v]", err)
		}
	})
	t.Run("retained TLS leaf aliases activation authority", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		leaf, err := frostPreSignParseBytes32(
			manifest.FrostSigner.CanonicalJournal.SourceIdentity.Export.
				TLSLeafSPKIHash,
		)
		if err != nil {
			t.Fatal(err)
		}
		manifest.activationAuthorityKeyHash = leaf
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "retained export TLS leaf") {
			t.Fatalf("expected retained-leaf role alias rejection, got [%v]", err)
		}
	})
	t.Run("retained backend aliases primary verifier", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.Ethereum.VerifierOperatorFingerprint =
			manifest.FrostSigner.CanonicalJournal.SourceIdentity.Export.
				BackendServiceFingerprint
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "retained export backend") {
			t.Fatalf("expected retained-backend role alias rejection, got [%v]", err)
		}
	})
	t.Run("outer operator roles alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.Ethereum.VerifierOperatorFingerprint =
			manifest.Ethereum.SourceOperatorFingerprint
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "aliases") {
			t.Fatalf("expected outer-role alias rejection, got [%v]", err)
		}
	})
	t.Run("activation authority aliases outer role", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		operator, err := frostPreSignParseBytes32(
			manifest.Ethereum.SourceOperatorFingerprint,
		)
		if err != nil {
			t.Fatal(err)
		}
		manifest.activationAuthorityKeyHash = operator
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "activation authority") {
			t.Fatalf("expected activation/outer alias rejection, got [%v]", err)
		}
	})
	t.Run("outer trust domains alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.Ethereum.VerifierTrustDomainID =
			manifest.Ethereum.SourceTrustDomainID
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "trust domain aliases") {
			t.Fatalf("expected outer trust-domain alias rejection, got [%v]", err)
		}
	})
	t.Run("retained nested trust domain aliases outer role", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.Ethereum.SourceTrustDomainID =
			manifest.FrostSigner.CanonicalJournal.SourceIdentity.Export.
				TrustDomainID
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "trust domain aliases") {
			t.Fatalf("expected nested/outer trust-domain alias rejection, got [%v]", err)
		}
	})
}

func TestValidateFrostPreSignActivationManifest_QuarantineAuthoritySets(
	t *testing.T,
) {
	t.Run("2-of-3 lift authority set", func(t *testing.T) {
		if err := validateFrostPreSignActivationManifest(
			testFrostJournalActivationManifest(),
		); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("3-of-4 lift authority set", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.LiftAuthorityThreshold = 3
		manifest.FrostSigner.QuarantineJournal.LiftAuthorities = append(
			manifest.FrostSigner.QuarantineJournal.LiftAuthorities,
			frostPreSignManifestLiftAuthority{
				AuthorityID:       "authority-4",
				PublicKeySPKIHash: testManifestHex32(0x39),
			},
		)
		if err := validateFrostPreSignActivationManifest(manifest); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("2-of-4 is not a strict majority", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.LiftAuthorities = append(
			manifest.FrostSigner.QuarantineJournal.LiftAuthorities,
			frostPreSignManifestLiftAuthority{
				AuthorityID:       "authority-4",
				PublicKeySPKIHash: testManifestHex32(0x39),
			},
		)
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "strict majority") {
			t.Fatalf("expected 2-of-4 rejection, got [%v]", err)
		}
	})
	t.Run("unsorted authority IDs", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		authorities := manifest.FrostSigner.QuarantineJournal.LiftAuthorities
		authorities[0], authorities[1] = authorities[1], authorities[0]
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "strictly sorted") {
			t.Fatalf("expected unsorted authority rejection, got [%v]", err)
		}
	})
	t.Run("duplicate authority key", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		authorities := manifest.FrostSigner.QuarantineJournal.LiftAuthorities
		authorities[1].PublicKeySPKIHash = authorities[0].PublicKeySPKIHash
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("expected duplicate authority-key rejection, got [%v]", err)
		}
	})
	t.Run("activation role alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.LiftAuthorities[0].
			PublicKeySPKIHash = testManifestHex32(0x40)
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "aliases the activation role") {
			t.Fatalf("expected activation-role alias rejection, got [%v]", err)
		}
	})
	t.Run("checkpoint role alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.LiftAuthorities[0].
			PublicKeySPKIHash = manifest.FrostSigner.QuarantineJournal.
			CheckpointAuthorities[0].PublicKeySPKIHash
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "checkpoint authority") {
			t.Fatalf("expected checkpoint-role alias rejection, got [%v]", err)
		}
	})
	t.Run("retained backend role alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.LiftAuthorities[0].
			PublicKeySPKIHash = manifest.FrostSigner.CanonicalJournal.
			SourceIdentity.Verifier.BackendServiceFingerprint
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "retained verifier backend") {
			t.Fatalf("expected retained-backend authority alias rejection, got [%v]", err)
		}
	})
	t.Run("protocol identity alias", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.LiftProtocolID =
			manifest.FrostSigner.QuarantineJournal.ProtocolID
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "not distinct") {
			t.Fatalf("expected protocol-identity alias rejection, got [%v]", err)
		}
	})
	t.Run("zero checkpoint floor", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.CheckpointMinimumSequence = 0
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "transparency floor") {
			t.Fatalf("expected zero checkpoint-floor rejection, got [%v]", err)
		}
	})
	t.Run("missing non-genesis predecessor", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.CheckpointMinimumSequence = 2
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "transparency floor") {
			t.Fatalf("expected missing predecessor rejection, got [%v]", err)
		}
	})
	t.Run("nonzero genesis predecessor", func(t *testing.T) {
		manifest := testFrostJournalActivationManifest()
		manifest.FrostSigner.QuarantineJournal.CheckpointPredecessorHash =
			testManifestHex32(0x7f)
		if err := validateFrostPreSignActivationManifest(manifest); err == nil ||
			!strings.Contains(err.Error(), "transparency floor") {
			t.Fatalf("expected genesis predecessor rejection, got [%v]", err)
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
