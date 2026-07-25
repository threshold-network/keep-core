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
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

type testFrostActivationPointVerifier struct {
	mutex sync.Mutex
	err   error
	point FrostPreSignFinality
	calls uint64
}

func (tfapv *testFrostActivationPointVerifier) VerifyFrostPreSignActivationPoint(
	ctx context.Context,
	point FrostPreSignFinality,
) error {
	tfapv.mutex.Lock()
	defer tfapv.mutex.Unlock()
	tfapv.point = point
	tfapv.calls++
	return tfapv.err
}

func (tfapv *testFrostActivationPointVerifier) snapshot() (
	FrostPreSignFinality,
	uint64,
) {
	tfapv.mutex.Lock()
	defer tfapv.mutex.Unlock()
	return tfapv.point, tfapv.calls
}

func (tfapv *testFrostActivationPointVerifier) setError(err error) {
	tfapv.mutex.Lock()
	defer tfapv.mutex.Unlock()
	tfapv.err = err
}

type testFrostRetainedGroupHistorySource struct {
	mutex          sync.Mutex
	manifest       FrostRetainedGroupCanonicalJournalManifest
	bindingHash    [32]byte
	checkpointHead FrostRetainedGroupCheckpointCursor
	historyRoot    [32]byte
	target         FrostPreSignFinality
	readCalls      uint64
	readStarted    chan struct{}
	readRelease    <-chan struct{}
	readDeadline   time.Time
	hasDeadline    bool
	readOnce       sync.Once
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
	source.mutex.Lock()
	defer source.mutex.Unlock()
	return source.target, nil
}

func (source *testFrostRetainedGroupHistorySource) VerifyPoint(
	context.Context,
	FrostPreSignFinality,
) error {
	return nil
}

func (source *testFrostRetainedGroupHistorySource) ReadCompleteHistory(
	ctx context.Context,
	from FrostPreSignFinality,
	to FrostPreSignFinality,
	checkpointAfter FrostRetainedGroupCheckpointCursor,
) (*FrostRetainedGroupHistory, error) {
	source.mutex.Lock()
	source.readCalls++
	readStarted := source.readStarted
	readRelease := source.readRelease
	historyRoot := source.historyRoot
	checkpointHead := source.checkpointHead
	source.readDeadline, source.hasDeadline = ctx.Deadline()
	source.mutex.Unlock()
	if readStarted != nil {
		source.readOnce.Do(func() {
			close(readStarted)
		})
	}
	if readRelease != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-readRelease:
		}
	}
	return &FrostRetainedGroupHistory{
		From:            from,
		To:              to,
		HistoryRoot:     historyRoot,
		CheckpointAfter: checkpointAfter,
		Checkpoints:     []FrostRetainedGroupCheckpointCertificate{},
		CheckpointChainRoot: frostRetainedGroupCheckpointChainRoot(
			source.bindingHash,
			checkpointAfter,
			nil,
		),
		CheckpointTipHash:  checkpointHead.CertificateHash,
		CheckpointComplete: true,
		Complete:           true,
		EmptyAtFrom:        true,
		DescriptorSetHash:  source.manifest.DescriptorSetHash,
	}, nil
}

func (source *testFrostRetainedGroupHistorySource) readCallCount() uint64 {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	return source.readCalls
}

func (source *testFrostRetainedGroupHistorySource) reconciliationDeadline() (
	time.Time,
	bool,
) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	return source.readDeadline, source.hasDeadline
}

func (source *testFrostRetainedGroupHistorySource) setTarget(
	target FrostPreSignFinality,
) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	source.target = target
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
			CheckpointFloor: frostRetainedGroupWireCheckpointCursor{
				Sequence: journal.checkpointState.Sequence,
				CertificateHash: frostActivationHex32(
					journal.checkpointState.CertificateHash,
				),
			},
		},
	}
	response := postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf(
			"initial asynchronous response was [%d] Retry-After [%s]: %s",
			response.StatusCode,
			response.Header.Get("Retry-After"),
			body,
		)
	}
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	defer response.Body.Close()
	handshake := &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(handshake); err != nil {
		t.Fatal(err)
	}
	verifiedPoint, verifierCalls := verifier.snapshot()
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
		verifiedPoint.BlockNumber != point.BlockNumber ||
		frostActivationHex32(verifiedPoint.BlockHash) != point.BlockHash ||
		verifierCalls != 4 {
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
		"checkpointJournal", "completeRouterAddress", "durableBitcoinOutboxRecovered", "durableSessionStoreFingerprint",
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
		"activeRoot", "clusterFingerprint", "complete", "currentQuarantineCount",
		"generation", "protocolID", "root", "storeFingerprint", "storeID",
		"tombstoneCount", "tombstoneRoot",
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
	assertFrostActivationObjectKeys(t, handshake.Payload.State.CheckpointJournal, []string{
		"ancestry", "canonicalGeneration", "canonicalInventoryRoot", "challengeFloor",
		"complete", "durableHead", "historyRoot", "manifestMinimumSequence",
		"manifestPredecessorHash", "point", "quarantineActiveRoot",
		"quarantineEventRoot", "quarantineGeneration", "quarantineTombstoneRoot",
	})
	unknownFloor := request
	unknownFloor.Challenge.CheckpointFloor.CertificateHash =
		frostActivationHex32([32]byte{0xff})
	unknownResponse := postTestFrostActivationHandshake(
		t,
		endpoint,
		unknownFloor,
	)
	defer unknownResponse.Body.Close()
	if unknownResponse.StatusCode != http.StatusServiceUnavailable ||
		unknownResponse.Header.Get("Retry-After") != "" {
		t.Fatalf(
			"unknown external checkpoint floor returned [%d] with retry [%s]",
			unknownResponse.StatusCode,
			unknownResponse.Header.Get("Retry-After"),
		)
	}
}

func TestFrostActivationHandshakeExporter_PermitsAndAttestsTombstones(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	_, journal, source, _, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)
	raisedRecord := FrostRetainedGroupQuarantineRaisedRecord{
		QuarantineID:     [32]byte{0x51},
		WalletID:         [32]byte{0x52},
		EvidenceHash:     [32]byte{0x53},
		Reason:           "resolved quarantine",
		RecoveryRequired: true,
		RaisedAt: FrostRetainedGroupEventPoint{
			BlockNumber:      100,
			BlockHash:        [32]byte{0x64},
			TransactionHash:  [32]byte{0xa1},
			TransactionIndex: 1,
			LogIndex:         1,
		},
	}
	liftedAt := FrostRetainedGroupEventPoint{
		BlockNumber:      120,
		BlockHash:        [32]byte{0x78},
		TransactionHash:  [32]byte{0xa2},
		TransactionIndex: 1,
		LogIndex:         1,
	}
	certificateHash := [32]byte{0x54}
	quarantine := frostRetainedGroupQuarantineState{
		RaisedRecord:        raisedRecord,
		Status:              frostRetainedGroupQuarantineLifted,
		LiftCertificateHash: certificateHash,
		LiftedAt:            liftedAt,
	}
	tombstone := frostRetainedGroupQuarantineTombstone{
		QuarantineID:           raisedRecord.QuarantineID,
		WalletID:               raisedRecord.WalletID,
		LiftCertificateHash:    certificateHash,
		LiftedAt:               liftedAt,
		ResolutionEvidenceHash: [32]byte{0x55},
		ResolutionFinality: FrostPreSignFinality{
			BlockNumber: 110,
			BlockHash:   [32]byte{0x6e},
		},
	}
	journal.quarantineState.Generation = 2
	journal.quarantineState.Quarantines =
		[]frostRetainedGroupQuarantineState{quarantine}
	journal.quarantineState.Tombstones =
		[]frostRetainedGroupQuarantineTombstone{tombstone}
	var err error
	journal.quarantineState.ActiveRoot, err =
		frostRetainedGroupQuarantineActiveRoot(
			journal.metadata.BindingHash,
			map[[32]byte]frostRetainedGroupQuarantineState{
				raisedRecord.QuarantineID: quarantine,
			},
		)
	if err != nil {
		t.Fatal(err)
	}
	journal.quarantineState.TombstoneRoot, err =
		frostRetainedGroupQuarantineTombstoneRoot(
			journal.metadata.BindingHash,
			map[[32]byte]frostRetainedGroupQuarantineTombstone{
				raisedRecord.QuarantineID: tombstone,
			},
		)
	if err != nil {
		t.Fatal(err)
	}
	recertifyTestFrostActivationJournal(
		t,
		journal,
		source,
		&request,
		journal.checkpointPolicy.MinimumSequence,
	)

	response := postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") !=
			frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("initial tombstone reconciliation returned [%d]", response.StatusCode)
	}
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	defer response.Body.Close()
	handshake := &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(handshake); err != nil {
		t.Fatal(err)
	}
	attestation := handshake.Payload.State.QuarantineJournal
	if attestation.CurrentQuarantineCount != 0 ||
		attestation.TombstoneCount != 1 ||
		attestation.TombstoneRoot !=
			frostActivationHex32(journal.quarantineState.TombstoneRoot) {
		t.Fatalf("tombstoned ready state was not attested: %+v", attestation)
	}
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
			CheckpointFloor: frostRetainedGroupWireCheckpointCursor{
				Sequence: journal.checkpointState.Sequence,
				CertificateHash: frostActivationHex32(
					journal.checkpointState.CertificateHash,
				),
			},
		},
	}
	response := postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf(
			"initial reconciliation did not return retryable service unavailable",
		)
	}
	response.Body.Close()
	awaitTestFrostActivationReconciliation(
		t,
		exporter,
		FrostPreSignFinality{
			BlockNumber: point.BlockNumber,
			BlockHash:   [32]byte{0x11},
		},
	)
	response = postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("quarantined outbox returned status [%d]", response.StatusCode)
	}
	if response.Header.Get("Retry-After") != "" {
		t.Fatal("non-reconciliation readiness failure advertised a retry interval")
	}
	response.Body.Close()

	outbox.mutex.Lock()
	outbox.records = make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord)
	outbox.mutex.Unlock()
	readiness.interactive = false
	journal.mutex.Lock()
	journal.state.SnapshotGeneration++
	var rootErr error
	journal.state.InventoryRoot, _, _, _, rootErr =
		frostRetainedGroupInventoryRoot(journal.state)
	journal.mutex.Unlock()
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	response = postTestFrostActivationHandshake(t, endpoint, request)
	response.Body.Close()
	settleDeadline := time.Now().Add(2 * time.Second)
	for {
		exporter.reconciliationMutex.Lock()
		idle := exporter.reconciliationDesired == nil &&
			exporter.reconciliationActive == nil
		cached := exporter.reconciliationCompleted != nil
		exporter.reconciliationMutex.Unlock()
		if idle {
			if cached {
				t.Fatal("unready interactive signer cached reconciliation state")
			}
			break
		}
		if time.Now().After(settleDeadline) {
			t.Fatal("background reconciliation did not settle")
		}
		time.Sleep(5 * time.Millisecond)
	}
	response = postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unready interactive signer returned status [%d]", response.StatusCode)
	}
}

func TestFrostActivationHandshakeExporter_RejectsUnboundOrLegacyChallenge(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	_, _, source, _, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)

	testCases := map[string]func(*frostActivationHandshakeRequest){
		"legacy v1 schema": func(request *frostActivationHandshakeRequest) {
			request.Schema = "tbtc-p2tr-production-activation-handshake/v1"
		},
		"legacy v2 schema": func(request *frostActivationHandshakeRequest) {
			request.Schema = "tbtc-p2tr-production-activation-handshake/v2"
		},
		"different binding": func(request *frostActivationHandshakeRequest) {
			request.Challenge.BindingHash = frostActivationHex32([32]byte{0xff})
		},
		"missing checkpoint floor": func(request *frostActivationHandshakeRequest) {
			request.Challenge.CheckpointFloor =
				frostRetainedGroupWireCheckpointCursor{}
		},
		"uncertified manifest predecessor": func(
			request *frostActivationHandshakeRequest,
		) {
			request.Challenge.CheckpointFloor =
				frostRetainedGroupWireCheckpointCursor{
					Sequence:        0,
					CertificateHash: frostActivationHex32([32]byte{}),
				}
		},
	}
	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			candidate := request
			mutate(&candidate)
			response := postTestFrostActivationHandshake(t, endpoint, candidate)
			defer response.Body.Close()
			if response.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("unbound challenge returned [%d]", response.StatusCode)
			}
			if response.Header.Get("Retry-After") != "" {
				t.Fatal("invalid transcript was treated as pending reconciliation")
			}
		})
	}
	if source.readCallCount() != 0 {
		t.Fatal("invalid transcript started retained-history reconciliation")
	}
}

func TestFrostActivationHandshakeExporter_ReconciliationIsAsynchronousAndGenerationBound(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	_, journal, source, verifier, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	source.mutex.Lock()
	source.readStarted = make(chan struct{})
	readStarted := source.readStarted
	source.readRelease = release
	source.mutex.Unlock()

	startedAt := time.Now()
	response := postTestFrostActivationHandshake(t, endpoint, request)
	elapsed := time.Since(startedAt)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("initial reconciliation response was not retryable: [%d]", response.StatusCode)
	}
	response.Body.Close()
	if elapsed >= time.Second {
		t.Fatalf("reconciliation held the HTTP request for [%v]", elapsed)
	}
	select {
	case <-readStarted:
	case <-time.After(time.Second):
		t.Fatal("background reconciliation did not start")
	}
	reconciliationDeadline, hasDeadline := source.reconciliationDeadline()
	remaining := time.Until(reconciliationDeadline)
	if !hasDeadline || remaining <= 0 ||
		remaining > frostActivationHandshakeReconciliationTimeout {
		t.Fatalf(
			"background reconciliation deadline is not bounded: [%v] [%v]",
			hasDeadline,
			remaining,
		)
	}

	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("in-flight reconciliation response was not retryable: [%d]", response.StatusCode)
	}
	response.Body.Close()
	if source.readCallCount() != 1 {
		t.Fatalf(
			"same-point requests started [%d] reconciliations",
			source.readCallCount(),
		)
	}

	close(release)
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	response.Body.Close()
	if source.readCallCount() != 1 {
		t.Fatalf("completed exact-point cache was not reused")
	}

	journal.mutex.Lock()
	response = postTestFrostActivationHandshake(t, endpoint, request)
	journal.mutex.Unlock()
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("busy live-state check was not retryable: [%d]", response.StatusCode)
	}
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	response.Body.Close()
	if source.readCallCount() != 1 {
		t.Fatal("a transient live-state lock invalidated the completed cache")
	}

	journal.mutex.Lock()
	journal.state.SnapshotGeneration++
	canonicalGeneration := journal.state.SnapshotGeneration
	canonicalPoint := FrostPreSignFinality{
		BlockNumber: point.BlockNumber + 1,
		BlockHash:   [32]byte{0x45},
	}
	journal.state.CurrentPoint = canonicalPoint
	journal.quarantineState.CurrentPoint = canonicalPoint
	var rootErr error
	journal.state.InventoryRoot, _, _, _, rootErr =
		frostRetainedGroupInventoryRoot(journal.state)
	journal.mutex.Unlock()
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	source.setTarget(canonicalPoint)
	request.Challenge.EthereumPoint = frostActivationEthereumPoint{
		BlockNumber: canonicalPoint.BlockNumber,
		BlockHash:   frostActivationHex32(canonicalPoint.BlockHash),
	}
	recertifyTestFrostActivationJournal(
		t,
		journal,
		source,
		&request,
		journal.checkpointState.Sequence+1,
	)
	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("canonical generation drift reused stale cache: [%d]", response.StatusCode)
	}
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	handshake := &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(handshake); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if handshake.Payload.State.CanonicalJournal.Generation != canonicalGeneration {
		t.Fatalf(
			"reconciled generation is [%d], expected [%d]",
			handshake.Payload.State.CanonicalJournal.Generation,
			canonicalGeneration,
		)
	}

	journal.mutex.Lock()
	journal.quarantineState.Generation++
	quarantineGeneration := journal.quarantineState.Generation
	quarantinePoint := FrostPreSignFinality{
		BlockNumber: canonicalPoint.BlockNumber + 1,
		BlockHash:   [32]byte{0x46},
	}
	journal.state.CurrentPoint = quarantinePoint
	journal.quarantineState.CurrentPoint = quarantinePoint
	journal.state.InventoryRoot, _, _, _, rootErr =
		frostRetainedGroupInventoryRoot(journal.state)
	journal.mutex.Unlock()
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	source.setTarget(quarantinePoint)
	request.Challenge.EthereumPoint = frostActivationEthereumPoint{
		BlockNumber: quarantinePoint.BlockNumber,
		BlockHash:   frostActivationHex32(quarantinePoint.BlockHash),
	}
	recertifyTestFrostActivationJournal(
		t,
		journal,
		source,
		&request,
		journal.checkpointState.Sequence+1,
	)
	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("quarantine generation drift reused stale cache: [%d]", response.StatusCode)
	}
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	handshake = &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(handshake); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if handshake.Payload.State.QuarantineJournal.Generation != quarantineGeneration {
		t.Fatalf(
			"reconciled quarantine generation is [%d], expected [%d]",
			handshake.Payload.State.QuarantineJournal.Generation,
			quarantineGeneration,
		)
	}

	nextPoint := FrostPreSignFinality{
		BlockNumber: quarantinePoint.BlockNumber + 1,
		BlockHash:   [32]byte{0x47},
	}
	source.setTarget(nextPoint)
	journal.mutex.Lock()
	journal.state.CurrentPoint = nextPoint
	journal.state.InventoryRoot, _, _, _, rootErr =
		frostRetainedGroupInventoryRoot(journal.state)
	journal.quarantineState.CurrentPoint = nextPoint
	journal.mutex.Unlock()
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	request.Challenge.EthereumPoint = frostActivationEthereumPoint{
		BlockNumber: nextPoint.BlockNumber,
		BlockHash:   frostActivationHex32(nextPoint.BlockHash),
	}
	recertifyTestFrostActivationJournal(
		t,
		journal,
		source,
		&request,
		journal.checkpointState.Sequence+1,
	)
	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf("different finality point reused stale cache: [%d]", response.StatusCode)
	}
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	response.Body.Close()
	if source.readCallCount() != 4 {
		t.Fatalf(
			"expected one reconciliation per cache invalidation, got [%d]",
			source.readCallCount(),
		)
	}

	verifier.setError(fmt.Errorf("exact point no longer canonical"))
	response = postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		t.Fatalf("failed quick point check signed cached state: [%d]", response.StatusCode)
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
		ActivationAuthorityKeyHash:       [32]byte{0x30},
		VerifierOperatorFingerprint:      [32]byte{0x31},
		HandshakeOperatorFingerprint:     [32]byte{0x37},
		DomainChainID:                    [32]byte{31: 0x01},
		GenesisBlockHash:                 [32]byte{0x32},
		ProfileHash:                      [32]byte{0x33},
		ImplementationSetHash:            [32]byte{0x34},
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
			ProtocolID:                   [32]byte{0x25},
			LiftProtocolID:               [32]byte{0x35},
			TombstoneProtocolID:          [32]byte{0x36},
			CheckpointAuthorityThreshold: 2,
			CheckpointAuthorities: []FrostRetainedGroupAuthority{
				{AuthorityID: "checkpoint-1", PublicKeySPKIHash: [32]byte{0x40}},
				{AuthorityID: "checkpoint-2", PublicKeySPKIHash: [32]byte{0x41}},
				{AuthorityID: "checkpoint-3", PublicKeySPKIHash: [32]byte{0x42}},
			},
			CheckpointMinimumSequence: 1,
			CheckpointPredecessorHash: [32]byte{},
			LiftAuthorityThreshold:    2,
			LiftAuthorities: []FrostRetainedGroupAuthority{
				{AuthorityID: "lift-1", PublicKeySPKIHash: [32]byte{0x43}},
				{AuthorityID: "lift-2", PublicKeySPKIHash: [32]byte{0x44}},
				{AuthorityID: "lift-3", PublicKeySPKIHash: [32]byte{0x45}},
			},
			StoreID:            "quarantine-store-id",
			StoreFingerprint:   [32]byte{0x26},
			ClusterFingerprint: [32]byte{0x27},
			MinimumGeneration:  0,
		},
	}
}

func startTestFrostActivationHandshakeExporter(
	t *testing.T,
	point frostActivationEthereumPoint,
) (
	*frostActivationHandshakeExporter,
	*frostRetainedGroupJournal,
	*testFrostRetainedGroupHistorySource,
	*testFrostActivationPointVerifier,
	string,
	frostActivationHandshakeRequest,
) {
	t.Helper()
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
	journal := testFrostRetainedGroupJournal(t, manifest, point)
	source, ok := journal.source.(*testFrostRetainedGroupHistorySource)
	if !ok {
		t.Fatal("unexpected retained-group history source")
	}
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
	if err := exporter.start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = exporter.close()
	})
	return exporter, journal, source, verifier, endpoint, frostActivationHandshakeRequest{
		Schema: frostActivationHandshakeSchema,
		Challenge: frostActivationChallenge{
			Nonce:         frostActivationHex32([32]byte{0x77}),
			ManifestHash:  frostActivationHex32(manifest.ManifestHash),
			BindingHash:   frostActivationHex32(journal.metadata.BindingHash),
			EthereumPoint: point,
			CheckpointFloor: frostRetainedGroupWireCheckpointCursor{
				Sequence: journal.checkpointState.Sequence,
				CertificateHash: frostActivationHex32(
					journal.checkpointState.CertificateHash,
				),
			},
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
	liftPolicy, err := frostRetainedGroupLiftPolicyFromRuntimeManifest(
		bindingHash,
		manifest,
	)
	if err != nil {
		t.Fatal(err)
	}
	activeRoot, err := frostRetainedGroupQuarantineActiveRoot(
		bindingHash,
		map[[32]byte]frostRetainedGroupQuarantineState{},
	)
	if err != nil {
		t.Fatal(err)
	}
	tombstoneRoot, err := frostRetainedGroupQuarantineTombstoneRoot(
		bindingHash,
		map[[32]byte]frostRetainedGroupQuarantineTombstone{},
	)
	if err != nil {
		t.Fatal(err)
	}
	state.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	checkpointPolicy, err :=
		frostRetainedGroupCheckpointPolicyFromRuntimeManifest(
			bindingHash,
			manifest,
		)
	if err != nil {
		t.Fatal(err)
	}
	historyRoot, err := frostRetainedGroupTestHistoryRoot(
		bindingHash,
		manifest.CanonicalJournal.Checkpoint,
		target,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	checkpointHash := [32]byte{0x55}
	checkpointHead := FrostRetainedGroupCheckpointCursor{
		Sequence:        manifest.QuarantineJournal.CheckpointMinimumSequence,
		CertificateHash: checkpointHash,
	}
	source := &testFrostRetainedGroupHistorySource{
		manifest:       manifest.CanonicalJournal,
		bindingHash:    bindingHash,
		checkpointHead: checkpointHead,
		historyRoot:    historyRoot,
		target:         target,
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
			Schema:                 frostRetainedGroupQuarantineMetadataSchema,
			ManifestHash:           manifest.ManifestHash,
			BindingHash:            bindingHash,
			ProtocolID:             manifest.QuarantineJournal.ProtocolID,
			LiftProtocolID:         manifest.QuarantineJournal.LiftProtocolID,
			TombstoneProtocolID:    manifest.QuarantineJournal.TombstoneProtocolID,
			LiftAuthoritySetHash:   liftPolicy.AuthoritySetHash,
			LiftAuthorityThreshold: liftPolicy.AuthorityThreshold,
			LiftAuthorities: append(
				[]FrostRetainedGroupAuthority{},
				liftPolicy.Authorities...,
			),
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
		liftPolicy:                  liftPolicy,
		liftCertificates:            make(map[[32]byte]FrostRetainedGroupQuarantineLiftCertificate),
		checkpointPolicy:            checkpointPolicy,
		checkpointState: frostRetainedGroupCheckpointJournalState{
			Schema:                  frostRetainedGroupCheckpointStateSchema,
			BindingHash:             bindingHash,
			Sequence:                checkpointHead.Sequence,
			CertificateHash:         checkpointHead.CertificateHash,
			Point:                   target,
			HistoryRoot:             historyRoot,
			CanonicalGeneration:     state.SnapshotGeneration,
			CanonicalInventoryRoot:  state.InventoryRoot,
			QuarantineGeneration:    0,
			QuarantineEventRoot:     quarantineRoot,
			QuarantineActiveRoot:    activeRoot,
			QuarantineTombstoneRoot: tombstoneRoot,
		},
		checkpointCertificates: make(map[uint64]FrostRetainedGroupCheckpointCertificate),
		checkpointHashes: map[uint64][32]byte{
			checkpointHead.Sequence: checkpointHead.CertificateHash,
		},
		quarantineState: frostRetainedGroupQuarantineJournalState{
			Schema:        frostRetainedGroupQuarantineStateSchema,
			BindingHash:   bindingHash,
			CurrentPoint:  target,
			Root:          quarantineRoot,
			ActiveRoot:    activeRoot,
			TombstoneRoot: tombstoneRoot,
			Quarantines:   []frostRetainedGroupQuarantineState{},
			Tombstones:    []frostRetainedGroupQuarantineTombstone{},
		},
	}
}

func recertifyTestFrostActivationJournal(
	t *testing.T,
	journal *frostRetainedGroupJournal,
	source *testFrostRetainedGroupHistorySource,
	request *frostActivationHandshakeRequest,
	sequence uint64,
) {
	t.Helper()
	journal.mutex.Lock()
	historyRoot, err := frostRetainedGroupTestHistoryRoot(
		journal.metadata.BindingHash,
		journal.metadata.Checkpoint,
		journal.state.CurrentPoint,
		nil,
	)
	if err != nil {
		journal.mutex.Unlock()
		t.Fatal(err)
	}
	commitment := struct {
		Sequence                uint64
		Point                   FrostPreSignFinality
		HistoryRoot             [32]byte
		CanonicalGeneration     uint64
		CanonicalInventoryRoot  [32]byte
		QuarantineGeneration    uint64
		QuarantineEventRoot     [32]byte
		QuarantineActiveRoot    [32]byte
		QuarantineTombstoneRoot [32]byte
	}{
		Sequence:                sequence,
		Point:                   journal.state.CurrentPoint,
		HistoryRoot:             historyRoot,
		CanonicalGeneration:     journal.state.SnapshotGeneration,
		CanonicalInventoryRoot:  journal.state.InventoryRoot,
		QuarantineGeneration:    journal.quarantineState.Generation,
		QuarantineEventRoot:     journal.quarantineState.Root,
		QuarantineActiveRoot:    journal.quarantineState.ActiveRoot,
		QuarantineTombstoneRoot: journal.quarantineState.TombstoneRoot,
	}
	certificateHash, err := frostRetainedGroupDomainHash(
		"test-frost-checkpoint-certificate-v1\x00",
		commitment,
	)
	if err != nil {
		journal.mutex.Unlock()
		t.Fatal(err)
	}
	journal.checkpointState = frostRetainedGroupCheckpointJournalState{
		Schema:                  frostRetainedGroupCheckpointStateSchema,
		BindingHash:             journal.metadata.BindingHash,
		Sequence:                sequence,
		CertificateHash:         certificateHash,
		Point:                   journal.state.CurrentPoint,
		HistoryRoot:             historyRoot,
		CanonicalGeneration:     journal.state.SnapshotGeneration,
		CanonicalInventoryRoot:  journal.state.InventoryRoot,
		QuarantineGeneration:    journal.quarantineState.Generation,
		QuarantineEventRoot:     journal.quarantineState.Root,
		QuarantineActiveRoot:    journal.quarantineState.ActiveRoot,
		QuarantineTombstoneRoot: journal.quarantineState.TombstoneRoot,
	}
	journal.checkpointHashes[sequence] = certificateHash
	journal.mutex.Unlock()

	source.mutex.Lock()
	source.historyRoot = historyRoot
	source.checkpointHead = FrostRetainedGroupCheckpointCursor{
		Sequence:        sequence,
		CertificateHash: certificateHash,
	}
	source.mutex.Unlock()
	request.Challenge.CheckpointFloor =
		frostRetainedGroupWireCheckpointCursor{
			Sequence:        sequence,
			CertificateHash: frostActivationHex32(certificateHash),
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

func awaitTestFrostActivationHandshake(
	t *testing.T,
	endpoint string,
	request frostActivationHandshakeRequest,
	status int,
) *http.Response {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		response := postTestFrostActivationHandshake(t, endpoint, request)
		if response.StatusCode == status {
			return response
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusServiceUnavailable ||
			time.Now().After(deadline) {
			t.Fatalf(
				"handshake did not reach status [%d]; last status [%d]: %s",
				status,
				response.StatusCode,
				body,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func awaitTestFrostActivationReconciliation(
	t *testing.T,
	exporter *frostActivationHandshakeExporter,
	point FrostPreSignFinality,
) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for exporter.cachedReconciliation(point) == nil {
		if time.Now().After(deadline) {
			t.Fatal("activation reconciliation did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
