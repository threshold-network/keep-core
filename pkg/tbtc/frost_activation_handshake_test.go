package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

type testFrostActivationPointVerifier struct {
	err   error
	point FrostPreSignFinality
	calls uint64
}

func (tfapv *testFrostActivationPointVerifier) VerifyFrostPreSignActivationPoint(
	ctx context.Context,
	point FrostPreSignFinality,
) error {
	tfapv.point = point
	tfapv.calls++
	return tfapv.err
}

type testFrostRetainedGroupHistorySource struct {
	manifest FrostRetainedGroupCanonicalJournalManifest
	target   FrostPreSignFinality
}

type testFrostProductionSignerReadiness struct {
	journal     *frostRetainedGroupJournal
	interactive bool
	err         error
	calls       uint64
}

func (readiness *testFrostProductionSignerReadiness) verifyFrostProductionSignerReadiness(
	ctx context.Context,
	point FrostPreSignFinality,
) (*frostProductionSignerReadinessSnapshot, error) {
	readiness.calls++
	if readiness.err != nil {
		return nil, readiness.err
	}
	if !readiness.interactive {
		return nil, fmt.Errorf("interactive signer is not ready")
	}
	journalSnapshot, err := readiness.journal.reconcile(ctx, point)
	if err != nil {
		return nil, err
	}
	return &frostProductionSignerReadinessSnapshot{
		Journal: journalSnapshot,
		Inventory: &frostNativeSignerInventorySnapshot{
			Schema:                      "tbtc-signer-retained-key-package-inventory/v1",
			StoreFingerprint:            testFrostDurableSessionStoreIdentity().Fingerprint,
			StateGeneration:             7,
			StateCommitment:             [32]byte{0x31},
			PreviousStateCommitment:     [32]byte{0x30},
			StateImageDigest:            [32]byte{0x33},
			InventoryCommitment:         [32]byte{0x32},
			ExternalRollbackAnchorBound: true,
			TrustCertificateSequence:    3,
			TrustCertificateDigest:      [32]byte{0x34},
			AnchorServiceEpoch:          1,
			CertifiedFloorRevision:      1,
			CertifiedFloorGeneration:    1,
			CurrentAnchorRevision:       1,
			RestartableRevisionHeadroom: FrostNativeSignerAnchorMaximumHistoryEvents,
			RestartableGenerationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries -
				6,
			AnchorRotationWarning: false,
		},
		InteractiveSigningReady: true,
	}, nil
}

func (source *testFrostRetainedGroupHistorySource) BindFrostRetainedGroupActivationEvidence(
	_ FrostPreSignActivationProfile,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
) error {
	if runtimeManifest.CanonicalJournal.DescriptorSetHash !=
		source.manifest.DescriptorSetHash {
		return fmt.Errorf("descriptor set mismatch")
	}
	return nil
}

func (source *testFrostRetainedGroupHistorySource) FrostRetainedGroupProtocolBindingHash() (
	[32]byte,
	error,
) {
	return source.manifest.DescriptorSetHash, nil
}

func (source *testFrostRetainedGroupHistorySource) Identity(
	context.Context,
) (FrostRetainedGroupHistoryIdentity, error) {
	return FrostRetainedGroupHistoryIdentity{
		TrustDomainID:       source.manifest.SourceTrustDomainID,
		EndpointFingerprint: source.manifest.SourceEndpointFingerprint,
		OperatorFingerprint: source.manifest.SourceOperatorFingerprint,
	}, nil
}

func (source *testFrostRetainedGroupHistorySource) FinalizedHead(
	context.Context,
) (FrostPreSignFinality, error) {
	return source.target, nil
}

func (source *testFrostRetainedGroupHistorySource) VerifyPoint(
	context.Context,
	FrostPreSignFinality,
) error {
	return nil
}

func (source *testFrostRetainedGroupHistorySource) ReadCompleteHistory(
	_ context.Context,
	from FrostPreSignFinality,
	to FrostPreSignFinality,
) (*FrostRetainedGroupHistory, error) {
	return &FrostRetainedGroupHistory{
		From:              from,
		To:                to,
		Complete:          true,
		EmptyAtFrom:       true,
		DescriptorSetHash: source.manifest.DescriptorSetHash,
	}, nil
}

func (source *testFrostRetainedGroupHistorySource) ResolveOperatorID(
	context.Context,
	chain.Address,
	FrostPreSignFinality,
) (chain.OperatorID, error) {
	return 1, nil
}

func TestFrostActivationHandshakeExporter_AttestsExactReadyState(t *testing.T) {
	directory := t.TempDir()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	privateKeyPath := filepath.Join(directory, "attestation-key.pem")
	if err := os.WriteFile(privateKeyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testFrostActivationRuntimeManifest(sha256.Sum256(publicKeyDER))
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	journal := testFrostRetainedGroupJournal(t, manifest, point)
	endpoint := testLoopbackEndpoint(t)
	verifier := &testFrostActivationPointVerifier{}
	outbox := &bitcoinBroadcastOutbox{
		records:   make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord),
		recovered: true,
	}
	readiness := &testFrostProductionSignerReadiness{
		journal:     journal,
		interactive: true,
	}
	exporter, err := newFrostActivationHandshakeExporter(
		endpoint,
		privateKeyPath,
		manifest,
		verifier,
		testFrostDurableSessionStoreBinding(t),
		outbox,
		journal,
		readiness,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := exporter.start(ctx); err != nil {
		t.Fatal(err)
	}
	defer exporter.close()

	nonce := [32]byte{0x77}
	request := frostActivationHandshakeRequest{
		Schema: frostActivationHandshakeSchema,
		Challenge: frostActivationChallenge{
			Nonce:         frostActivationHex32(nonce),
			ManifestHash:  frostActivationHex32(manifest.ManifestHash),
			BindingHash:   frostActivationHex32(journal.metadata.BindingHash),
			EthereumPoint: point,
		},
	}
	response := postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected status [%d]: %s", response.StatusCode, body)
	}
	handshake := &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(handshake); err != nil {
		t.Fatal(err)
	}
	if handshake.Payload.Kind != "frost-signer" ||
		handshake.Payload.Nonce != request.Challenge.Nonce ||
		handshake.Payload.ManifestHash != request.Challenge.ManifestHash ||
		handshake.Payload.BindingHash != request.Challenge.BindingHash ||
		handshake.Payload.State.CanonicalJournal.BindingHash !=
			request.Challenge.BindingHash ||
		!handshake.Payload.State.Healthy ||
		!handshake.Payload.State.InteractiveSigningReady ||
		!handshake.Payload.State.NonceShareGateEnforced ||
		!handshake.Payload.State.DurableBitcoinOutboxRecovered ||
		verifier.point.BlockNumber != point.BlockNumber ||
		frostActivationHex32(verifier.point.BlockHash) != point.BlockHash ||
		verifier.calls != 2 {
		t.Fatalf("unexpected handshake: %+v", handshake)
	}
	canonicalPayload, err := canonicalFrostActivationValue(handshake.Payload)
	if err != nil {
		t.Fatal(err)
	}
	signatureTranscript := append(
		[]byte(frostActivationHandshakeSignatureDomain),
		canonicalPayload...,
	)
	signature, err := base64.StdEncoding.Strict().DecodeString(handshake.Signature)
	if err != nil || !ed25519.Verify(publicKey, signatureTranscript, signature) {
		t.Fatal("handshake signature did not verify over canonical payload")
	}
	assertFrostActivationObjectKeys(t, handshake.Payload.State, []string{
		"authorizationRegistryAddress", "bitcoinOutboxProtocolID", "canonicalJournal",
		"completeRouterAddress", "durableBitcoinOutboxRecovered", "durableSessionStoreFingerprint",
		"exactTransactionAuthorizationRootEnforced", "finalizedReservationReadbackEnforced",
		"frostWalletGroupInventory", "healthy", "maximumGroupSize", "nonceShareGateEnforced",
		"interactiveSigningReady", "nativeSignerState",
		"protocolID", "quarantineFailClosed", "quarantineJournal", "reservationProtocolID",
		"retainedGroupInventoryProtocolID", "signingPolicyHash", "threshold",
	})
	assertFrostActivationObjectKeys(t, handshake.Payload.State.FrostWalletGroupInventory, []string{
		"complete", "groupSizeViolationCount", "inventoryRoot", "maximumActualGroupSize",
		"membershipAmbiguityCount", "minimumActualGroupSize", "point", "schema",
		"snapshotGeneration", "walletCount",
	})
	assertFrostActivationObjectKeys(t, handshake.Payload.State.CanonicalJournal, []string{
		"bindingHash", "checkpoint", "clusterFingerprint", "complete", "current",
		"descriptorSetHash", "generation", "sourceEndpointFingerprint", "sourceOperatorFingerprint",
		"sourceTrustDomainID", "storeFingerprint", "storeID",
	})
	assertFrostActivationObjectKeys(t, handshake.Payload.State.QuarantineJournal, []string{
		"clusterFingerprint", "complete", "currentQuarantineCount", "generation",
		"protocolID", "root", "storeFingerprint", "storeID",
	})
	assertFrostActivationObjectKeys(t, handshake.Payload.State.NativeSignerState, []string{
		"anchorRotationWarning", "anchorServiceEpoch", "certifiedFloorGeneration",
		"certifiedFloorRevision",
		"complete", "currentAnchorRevision", "externalRollbackAnchorBound", "inventoryCommitment",
		"previousStateCommitment", "retainedKeyPackageCount", "retainedWalletCount",
		"restartableGenerationHeadroom", "restartableRevisionHeadroom",
		"schema", "stateCommitment", "stateGeneration",
		"stateImageDigest", "storeFingerprint",
		"trustCertificateDigest", "trustCertificateSequence",
	})
}

func assertFrostActivationObjectKeys(
	t *testing.T,
	value interface{},
	expected []string,
) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	object := make(map[string]json.RawMessage)
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	actual := make([]string, 0, len(object))
	for key := range object {
		actual = append(actual, key)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected handshake object keys\nexpected: %v\nactual:   %v", expected, actual)
	}
}

func TestFrostActivationHandshakeExporter_FailsClosed(t *testing.T) {
	directory := t.TempDir()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateKeyDER, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	privateKeyPath := filepath.Join(directory, "attestation-key.pem")
	if err := os.WriteFile(privateKeyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}
	publicKeyDER, _ := x509.MarshalPKIXPublicKey(publicKey)
	manifest := testFrostActivationRuntimeManifest(sha256.Sum256(publicKeyDER))
	point := frostActivationEthereumPoint{
		BlockNumber: 7,
		BlockHash:   frostActivationHex32([32]byte{0x11}),
	}
	journal := testFrostRetainedGroupJournal(t, manifest, point)
	endpoint := testLoopbackEndpoint(t)
	outbox := &bitcoinBroadcastOutbox{
		records: map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord{
			{0x01}: {
				TransactionHash: bitcoin.Hash{0x01},
				Authorization: bitcoinBroadcastAuthorization{
					ReservationID: [32]byte{0x02},
				},
				Quarantine: &bitcoinBroadcastQuarantine{
					ActiveActivationProfileHash: [32]byte{0x03},
					ObservedAtUnix:              1,
				},
			},
		},
		recovered: true,
	}
	readiness := &testFrostProductionSignerReadiness{journal: journal, interactive: true}
	exporter, err := newFrostActivationHandshakeExporter(
		endpoint,
		privateKeyPath,
		manifest,
		&testFrostActivationPointVerifier{},
		testFrostDurableSessionStoreBinding(t),
		outbox,
		journal,
		readiness,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := exporter.start(ctx); err != nil {
		t.Fatal(err)
	}
	defer exporter.close()

	request := frostActivationHandshakeRequest{
		Schema: frostActivationHandshakeSchema,
		Challenge: frostActivationChallenge{
			Nonce:         frostActivationHex32([32]byte{0x22}),
			ManifestHash:  frostActivationHex32(manifest.ManifestHash),
			BindingHash:   frostActivationHex32(journal.metadata.BindingHash),
			EthereumPoint: point,
		},
	}
	response := postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("quarantined outbox returned status [%d]", response.StatusCode)
	}
	response.Body.Close()

	outbox.mutex.Lock()
	outbox.records = make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord)
	outbox.mutex.Unlock()
	readiness.interactive = false
	response = postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unready interactive signer returned status [%d]", response.StatusCode)
	}
}

func TestCanonicalFrostActivationValue_MatchesRuntimeOrdering(t *testing.T) {
	value := map[string]interface{}{
		"state": map[string]interface{}{"healthy": true, "count": 3},
		"nonce": "0x01",
		"kind":  "frost-signer",
	}
	encoded, err := canonicalFrostActivationValue(value)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"kind":"frost-signer","nonce":"0x01","state":{"count":3,"healthy":true}}`
	if string(encoded) != expected {
		t.Fatalf("unexpected canonical JSON\nexpected: %s\nactual:   %s", expected, encoded)
	}
}

func TestDecodeStrictFrostActivationJSON_RejectsAmbiguousObjectKeys(
	t *testing.T,
) {
	type nested struct {
		Count uint64 `json:"count"`
	}
	type payload struct {
		Schema string `json:"schema"`
		Nested nested `json:"nested"`
	}
	testCases := map[string]string{
		"duplicate exact key":          `{"schema":"v1","schema":"v2","nested":{"count":1}}`,
		"case-insensitive field alias": `{"Schema":"v1","nested":{"count":1}}`,
		"case-fold-equivalent keys":    `{"schema":"v1","SCHEMA":"v2","nested":{"count":1}}`,
		"nested duplicate key":         `{"schema":"v1","nested":{"count":1,"count":2}}`,
		"nested field alias":           `{"schema":"v1","nested":{"Count":1}}`,
		"non-ASCII field alias":        `{"ſchema":"v1","nested":{"count":1}}`,
	}
	for name, encoded := range testCases {
		t.Run(name, func(t *testing.T) {
			target := payload{}
			if err := decodeStrictFrostActivationJSON(
				[]byte(encoded),
				&target,
			); err == nil {
				t.Fatal("expected ambiguous JSON to be rejected")
			}
		})
	}

	target := payload{}
	if err := decodeStrictFrostActivationJSON(
		[]byte(`{"schema":"v1","nested":{"count":1}}`),
		&target,
	); err != nil {
		t.Fatalf("expected exact JSON keys to remain valid: [%v]", err)
	}
	if target.Schema != "v1" || target.Nested.Count != 1 {
		t.Fatalf("unexpected exact JSON decode: [%+v]", target)
	}
}

func TestCanonicalFrostActivationValue_RejectsDuplicateRawMessageKeys(
	t *testing.T,
) {
	_, err := canonicalFrostActivationValue(
		json.RawMessage(`{"schema":"v1","schema":"v2"}`),
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate raw-message key rejection, got [%v]", err)
	}
}

func testFrostActivationRuntimeManifest(
	keyHash [32]byte,
) FrostPreSignActivationRuntimeManifest {
	return FrostPreSignActivationRuntimeManifest{
		ManifestHash:                     [32]byte{0x10},
		SignerProtocolID:                 [32]byte{0x11},
		ReservationProtocolID:            [32]byte{0x12},
		BitcoinOutboxProtocolID:          [32]byte{0x13},
		SigningPolicyHash:                [32]byte{0x14},
		DurableSessionStoreFingerprint:   frostActivationHex32(testFrostDurableSessionStoreIdentity().Fingerprint),
		CompleteRouterAddress:            [20]byte{0x15},
		AuthorizationRegistryAddress:     [20]byte{0x16},
		AttestationSignerKeyHash:         keyHash,
		Threshold:                        51,
		MaximumGroupSize:                 100,
		RetainedGroupInventoryProtocolID: [32]byte{0x17},
		CanonicalJournal: FrostRetainedGroupCanonicalJournalManifest{
			StoreID:            "journal-store-id",
			StoreFingerprint:   [32]byte{0x18},
			ClusterFingerprint: [32]byte{0x19},
			Checkpoint: FrostPreSignFinality{
				BlockNumber: 1,
				BlockHash:   [32]byte{0x20},
			},
			DescriptorSetHash:         [32]byte{0x21},
			SourceTrustDomainID:       "independent-journal-source",
			SourceEndpointFingerprint: [32]byte{0x22},
			SourceOperatorFingerprint: [32]byte{0x23},
			MinimumGeneration:         1,
		},
		QuarantineJournal: FrostRetainedGroupQuarantineJournalManifest{
			ProtocolID:         [32]byte{0x25},
			StoreID:            "quarantine-store-id",
			StoreFingerprint:   [32]byte{0x26},
			ClusterFingerprint: [32]byte{0x27},
			MinimumGeneration:  0,
		},
	}
}

func testFrostRetainedGroupJournal(
	t *testing.T,
	manifest FrostPreSignActivationRuntimeManifest,
	point frostActivationEthereumPoint,
) *frostRetainedGroupJournal {
	t.Helper()
	blockHash, err := parseFrostActivationHex32(point.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	target := FrostPreSignFinality{BlockNumber: point.BlockNumber, BlockHash: blockHash}
	bindingHash := [32]byte{0x28}
	state := frostRetainedGroupJournalState{
		Schema:             frostRetainedGroupJournalStateSchema,
		BindingHash:        bindingHash,
		CurrentPoint:       target,
		SnapshotGeneration: 9,
		Wallets:            []frostRetainedGroupWalletState{},
	}
	quarantineRoot := sha256.Sum256([]byte(frostRetainedGroupQuarantineDomain))
	state.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	source := &testFrostRetainedGroupHistorySource{
		manifest: manifest.CanonicalJournal,
		target:   target,
	}
	return &frostRetainedGroupJournal{
		metadata: frostRetainedGroupJournalMetadata{
			Schema:                    frostRetainedGroupJournalMetadataSchema,
			ManifestHash:              manifest.ManifestHash,
			BindingHash:               bindingHash,
			StoreID:                   manifest.CanonicalJournal.StoreID,
			StoreFingerprint:          manifest.CanonicalJournal.StoreFingerprint,
			ClusterFingerprint:        manifest.CanonicalJournal.ClusterFingerprint,
			Checkpoint:                manifest.CanonicalJournal.Checkpoint,
			DescriptorSetHash:         manifest.CanonicalJournal.DescriptorSetHash,
			SourceTrustDomainID:       manifest.CanonicalJournal.SourceTrustDomainID,
			SourceEndpointFingerprint: manifest.CanonicalJournal.SourceEndpointFingerprint,
			SourceOperatorFingerprint: manifest.CanonicalJournal.SourceOperatorFingerprint,
		},
		quarantineMetadata: frostRetainedGroupQuarantineMetadata{
			Schema:             frostRetainedGroupQuarantineMetadataSchema,
			ManifestHash:       manifest.ManifestHash,
			BindingHash:        bindingHash,
			ProtocolID:         manifest.QuarantineJournal.ProtocolID,
			StoreID:            manifest.QuarantineJournal.StoreID,
			StoreFingerprint:   manifest.QuarantineJournal.StoreFingerprint,
			ClusterFingerprint: manifest.QuarantineJournal.ClusterFingerprint,
			Checkpoint:         manifest.CanonicalJournal.Checkpoint,
		},
		minimumGeneration:           manifest.CanonicalJournal.MinimumGeneration,
		quarantineMinimumGeneration: manifest.QuarantineJournal.MinimumGeneration,
		source:                      source,
		walletRegistry:              &walletRegistry{walletCache: make(map[string]*walletCacheValue)},
		operatorAddress:             chain.Address("0x01"),
		state:                       state,
		quarantineState: frostRetainedGroupQuarantineJournalState{
			Schema:       frostRetainedGroupQuarantineStateSchema,
			BindingHash:  bindingHash,
			CurrentPoint: target,
			Root:         quarantineRoot,
			Quarantines:  []frostRetainedGroupQuarantineState{},
		},
	}
}

func testLoopbackEndpoint(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("http://%s/frost-activation", address)
}

func postTestFrostActivationHandshake(
	t *testing.T,
	endpoint string,
	request frostActivationHandshakeRequest,
) *http.Response {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	httpRequest, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(httpRequest)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
