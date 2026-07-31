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
	"errors"
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

func TestFrostActivationHandshakeExporter_CheckpointRecoveryReentry(
	t *testing.T,
) {
	point := FrostPreSignFinality{
		BlockNumber: 11,
		BlockHash:   [32]byte{0x11},
	}

	t.Run("requeues authenticated progress", func(t *testing.T) {
		job := &frostActivationReconciliationJob{
			sequence: 7,
			point:    point,
		}
		exporter := &frostActivationHandshakeExporter{
			reconciliationSequence: 7,
			reconciliationActive:   job,
		}
		hasDesired := exporter.completeReconciliationJob(
			job,
			nil,
			fmt.Errorf(
				"wrapped recovery result: %w",
				errFrostRetainedGroupCheckpointRecoveryProgress,
			),
		)
		if !hasDesired ||
			exporter.reconciliationActive != nil ||
			exporter.reconciliationCompleted != nil ||
			exporter.reconciliationSequence != 8 ||
			exporter.reconciliationDesired == nil ||
			exporter.reconciliationDesired.sequence != 8 ||
			exporter.reconciliationDesired.point != point {
			t.Fatalf(
				"authenticated checkpoint progress was not deterministically requeued: %+v",
				exporter,
			)
		}
	})

	t.Run("does not retry an ordinary failure", func(t *testing.T) {
		job := &frostActivationReconciliationJob{
			sequence: 7,
			point:    point,
		}
		exporter := &frostActivationHandshakeExporter{
			reconciliationSequence: 7,
			reconciliationActive:   job,
		}
		if exporter.completeReconciliationJob(
			job,
			nil,
			errors.New("ordinary failure"),
		) ||
			exporter.reconciliationDesired != nil ||
			exporter.reconciliationSequence != 7 {
			t.Fatalf(
				"ordinary reconciliation failure was automatically requeued: %+v",
				exporter,
			)
		}
	})

	t.Run("preserves a newer request", func(t *testing.T) {
		job := &frostActivationReconciliationJob{
			sequence: 7,
			point:    point,
		}
		newerPoint := FrostPreSignFinality{
			BlockNumber: 12,
			BlockHash:   [32]byte{0x12},
		}
		newerJob := &frostActivationReconciliationJob{
			sequence: 8,
			point:    newerPoint,
		}
		exporter := &frostActivationHandshakeExporter{
			reconciliationSequence: 8,
			reconciliationActive:   job,
			reconciliationDesired:  newerJob,
		}
		if !exporter.completeReconciliationJob(
			job,
			nil,
			errFrostRetainedGroupCheckpointRecoveryProgress,
		) ||
			exporter.reconciliationSequence != 8 ||
			exporter.reconciliationDesired != newerJob {
			t.Fatalf(
				"checkpoint progress overwrote the newer reconciliation request: %+v",
				exporter,
			)
		}
	})
}

func TestFrostActivationHandshakeExporter_CheckpointRecoveryWorkerLoopsUntilComplete(
	t *testing.T,
) {
	point := FrostPreSignFinality{
		BlockNumber: 11,
		BlockHash:   [32]byte{0x11},
	}
	job := &frostActivationReconciliationJob{
		sequence: 1,
		point:    point,
	}
	exporter := &frostActivationHandshakeExporter{
		reconciliationWake:     make(chan struct{}, 1),
		reconciliationSequence: 1,
		reconciliationDesired:  job,
	}
	thirdAttemptStarted := make(chan struct{})
	releaseThirdAttempt := make(chan struct{})
	attempts := 0
	reconcile := func(
		ctx context.Context,
		actualPoint FrostPreSignFinality,
	) (*frostActivationReconciliationCache, error) {
		attempts++
		if actualPoint != point {
			return nil, fmt.Errorf("worker reconciled an unexpected point")
		}
		if attempts <= 2 {
			return nil, fmt.Errorf(
				"bounded page [%d]: %w",
				attempts,
				errFrostRetainedGroupCheckpointRecoveryProgress,
			)
		}
		close(thirdAttemptStarted)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-releaseThirdAttempt:
		}
		return &frostActivationReconciliationCache{
			point: point,
		}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		exporter.runReconciliationWorker(ctx, reconcile)
		close(workerDone)
	}()
	exporter.reconciliationWake <- struct{}{}

	select {
	case <-thirdAttemptStarted:
	case <-time.After(3 * time.Second):
		cancel()
		<-workerDone
		t.Fatal("worker did not re-enter checkpoint recovery")
	}
	exporter.reconciliationMutex.Lock()
	if exporter.reconciliationCompleted != nil ||
		exporter.reconciliationActive == nil ||
		exporter.reconciliationActive.sequence != 3 ||
		exporter.reconciliationSequence != 3 {
		exporter.reconciliationMutex.Unlock()
		cancel()
		close(releaseThirdAttempt)
		<-workerDone
		t.Fatalf(
			"worker published health before checkpoint recovery completed: %+v",
			exporter,
		)
	}
	exporter.reconciliationMutex.Unlock()

	close(releaseThirdAttempt)
	deadline := time.Now().Add(3 * time.Second)
	for exporter.cachedReconciliation(point) == nil {
		if time.Now().After(deadline) {
			cancel()
			<-workerDone
			t.Fatal("worker did not publish completed reconciliation")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-workerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("checkpoint recovery worker did not stop")
	}
	if attempts != 3 {
		t.Fatalf(
			"worker made [%d] attempts, expected two progress pages and completion",
			attempts,
		)
	}
}

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
	mutex            sync.Mutex
	journal          *frostRetainedGroupJournal
	interactive      bool
	err              error
	calls            uint64
	inventory        frostNativeSignerInventorySnapshot
	unchangedStarted chan struct{}
	unchangedRelease <-chan struct{}
	unchangedOnce    sync.Once
}

func testFrostProductionSignerInventorySnapshot() frostNativeSignerInventorySnapshot {
	return frostNativeSignerInventorySnapshot{
		Schema:                      "tbtc-signer-retained-key-package-inventory/v1",
		StoreFingerprint:            testFrostDurableSessionStoreIdentity().Fingerprint,
		StateGeneration:             7,
		StateCommitment:             [32]byte{0x31},
		PreviousStateCommitment:     [32]byte{0x30},
		StateImageDigest:            [32]byte{0x33},
		InventoryCommitment:         [32]byte{0x32},
		LargestLocalSeatCount:       20,
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
	}
}

func (readiness *testFrostProductionSignerReadiness) snapshot() (
	bool,
	error,
	frostNativeSignerInventorySnapshot,
) {
	readiness.mutex.Lock()
	defer readiness.mutex.Unlock()
	readiness.calls++
	inventory := readiness.inventory
	if inventory.Schema == "" {
		inventory = testFrostProductionSignerInventorySnapshot()
	}
	return readiness.interactive, readiness.err, inventory
}

func (readiness *testFrostProductionSignerReadiness) setInteractive(
	interactive bool,
) {
	readiness.mutex.Lock()
	defer readiness.mutex.Unlock()
	readiness.interactive = interactive
}

func (readiness *testFrostProductionSignerReadiness) setInventory(
	inventory frostNativeSignerInventorySnapshot,
) {
	readiness.mutex.Lock()
	defer readiness.mutex.Unlock()
	readiness.inventory = inventory
}

func (readiness *testFrostProductionSignerReadiness) blockUnchangedVerification(
	started chan struct{},
	release <-chan struct{},
) {
	readiness.mutex.Lock()
	defer readiness.mutex.Unlock()
	readiness.unchangedStarted = started
	readiness.unchangedRelease = release
}

func (readiness *testFrostProductionSignerReadiness) verifyFrostProductionSignerReadiness(
	ctx context.Context,
	point FrostPreSignFinality,
) (*frostProductionSignerReadinessSnapshot, error) {
	interactive, readinessErr, inventory := readiness.snapshot()
	if readinessErr != nil {
		return nil, readinessErr
	}
	if !interactive {
		return nil, fmt.Errorf("interactive signer is not ready")
	}
	journalSnapshot, err := readiness.journal.reconcile(ctx, point)
	if err != nil {
		return nil, err
	}
	return &frostProductionSignerReadinessSnapshot{
		Journal:                 journalSnapshot,
		Inventory:               &inventory,
		InteractiveSigningReady: true,
	}, nil
}

func (readiness *testFrostProductionSignerReadiness) verifyFrostProductionSignerReadinessUnchanged(
	ctx context.Context,
	expected *frostProductionSignerReadinessSnapshot,
) error {
	_, err := readiness.revalidateFrostProductionSignerReadinessInventory(
		ctx,
		expected,
	)
	return err
}

// revalidateFrostProductionSignerReadinessInventory mirrors production: it
// compares the live inventory against the reconciled one with exactly the
// comparison the real verifier uses - strict on identity, key material, trust
// head and the rotation warning, monotone on the state checkpoint, the anchor
// revision and both headrooms - and returns the live snapshot. Comparing the
// whole struct by value here would model a production behaviour that no longer
// exists and would assert a 503 the exporter no longer returns.
func (readiness *testFrostProductionSignerReadiness) revalidateFrostProductionSignerReadinessInventory(
	ctx context.Context,
	expected *frostProductionSignerReadinessSnapshot,
) (*frostNativeSignerInventorySnapshot, error) {
	if ctx == nil || expected == nil || expected.Inventory == nil ||
		!expected.InteractiveSigningReady {
		return nil, fmt.Errorf("cached signer readiness is incomplete")
	}
	readiness.mutex.Lock()
	unchangedStarted := readiness.unchangedStarted
	unchangedRelease := readiness.unchangedRelease
	readiness.mutex.Unlock()
	if unchangedStarted != nil {
		readiness.unchangedOnce.Do(func() {
			close(unchangedStarted)
		})
	}
	if unchangedRelease != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-unchangedRelease:
		}
	}
	interactive, readinessErr, inventory := readiness.snapshot()
	if readinessErr != nil {
		return nil, readinessErr
	}
	if !interactive {
		return nil, fmt.Errorf("interactive signer is not ready")
	}
	if err := verifyFrostNativeSignerInventoryUnchanged(
		expected.Inventory,
		&inventory,
	); err != nil {
		return nil, err
	}
	return &inventory, nil
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
	return source.manifest.SourceIdentity, nil
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
	if len(handshake.Payload.State.CheckpointJournal.Ancestry) != 1 {
		t.Fatalf(
			"exact-head checkpoint proof contains [%d] certificates",
			len(handshake.Payload.State.CheckpointJournal.Ancestry),
		)
	}
	if err := verifyFrostActivationCheckpointHandshakeState(
		manifest,
		journal.metadata.BindingHash,
		request.Challenge,
		handshake.Payload.State,
	); err != nil {
		t.Fatalf(
			"independent exact-head checkpoint proof verification failed: [%v]",
			err,
		)
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
		"sourceIdentity", "sourceTrustDomainID", "storeFingerprint", "storeID",
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
		"schema", "stateAnchorPoisoned", "stateCommitment", "stateGeneration",
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

func TestFrostActivationHandshakeExporter_AttestsInclusiveCheckpointAncestry(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	exporter, journal, source, _, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)
	externalFloor := request.Challenge.CheckpointFloor
	nextPoint := FrostPreSignFinality{
		BlockNumber: point.BlockNumber + 1,
		BlockHash:   [32]byte{0x45},
	}
	journal.mutex.Lock()
	journal.state.CurrentPoint = nextPoint
	journal.quarantineState.CurrentPoint = nextPoint
	var err error
	journal.state.InventoryRoot, _, _, _, err =
		frostRetainedGroupInventoryRoot(journal.state)
	journal.mutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	source.setTarget(nextPoint)
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
	request.Challenge.CheckpointFloor = externalFloor

	response := postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable ||
		response.Header.Get("Retry-After") !=
			frostActivationHandshakeRetryAfter {
		response.Body.Close()
		t.Fatalf(
			"initial ancestry reconciliation returned [%d]",
			response.StatusCode,
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
	if len(handshake.Payload.State.CheckpointJournal.Ancestry) != 2 {
		t.Fatalf(
			"checkpoint proof contains [%d] certificates, expected floor and head",
			len(handshake.Payload.State.CheckpointJournal.Ancestry),
		)
	}
	if err := verifyFrostActivationCheckpointHandshakeState(
		exporter.manifest,
		journal.metadata.BindingHash,
		request.Challenge,
		handshake.Payload.State,
	); err != nil {
		t.Fatalf(
			"independent descendant checkpoint proof verification failed: [%v]",
			err,
		)
	}

	missingFloor := handshake.Payload.State
	missingFloor.CheckpointJournal.Ancestry =
		append(
			[]frostRetainedGroupWireCheckpointCertificate{},
			missingFloor.CheckpointJournal.Ancestry[1:]...,
		)
	if err := verifyFrostActivationCheckpointHandshakeState(
		exporter.manifest,
		journal.metadata.BindingHash,
		request.Challenge,
		missingFloor,
	); err == nil {
		t.Fatal("checkpoint proof without its external floor was accepted")
	}
	wrongRoot := handshake.Payload.State
	wrongRoot.CheckpointJournal.HistoryRoot =
		frostActivationHex32([32]byte{0xff})
	if err := verifyFrostActivationCheckpointHandshakeState(
		exporter.manifest,
		journal.metadata.BindingHash,
		request.Challenge,
		wrongRoot,
	); err == nil {
		t.Fatal("checkpoint proof with a different tail root was accepted")
	}
}

func TestFrostActivationHandshakeExporter_RevalidatesNativeSignerStateBeforeSigning(
	t *testing.T,
) {
	// Only changes the revalidation fails closed on belong here. A pure
	// monotone advance of the state checkpoint, the anchor revision or the
	// headrooms is the authorized signing window's own progress and is covered
	// by TestFrostActivationHandshakeExporter_SignsLiveNativeSignerState.
	testCases := map[string]func(*testFrostProductionSignerReadiness){
		"retained key material changes": func(
			readiness *testFrostProductionSignerReadiness,
		) {
			inventory := testFrostProductionSignerInventorySnapshot()
			inventory.StateGeneration++
			inventory.PreviousStateCommitment = inventory.StateCommitment
			inventory.StateCommitment = [32]byte{0x41}
			inventory.StateImageDigest = [32]byte{0x42}
			inventory.InventoryCommitment = [32]byte{0x43}
			inventory.RestartableGenerationHeadroom--
			readiness.setInventory(inventory)
		},
		"native state rolls back": func(
			readiness *testFrostProductionSignerReadiness,
		) {
			inventory := testFrostProductionSignerInventorySnapshot()
			inventory.StateGeneration--
			inventory.StateCommitment = inventory.PreviousStateCommitment
			inventory.PreviousStateCommitment = [32]byte{0x2f}
			inventory.StateImageDigest = [32]byte{0x35}
			inventory.RestartableGenerationHeadroom++
			readiness.setInventory(inventory)
		},
		"rotated anchor trust certificate": func(
			readiness *testFrostProductionSignerReadiness,
		) {
			inventory := testFrostProductionSignerInventorySnapshot()
			inventory.TrustCertificateSequence++
			inventory.TrustCertificateDigest = [32]byte{0x3a}
			readiness.setInventory(inventory)
		},
		"interactive readiness changes": func(
			readiness *testFrostProductionSignerReadiness,
		) {
			readiness.setInteractive(false)
		},
	}

	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			point := frostActivationEthereumPoint{
				BlockNumber: 123,
				BlockHash:   frostActivationHex32([32]byte{0x44}),
			}
			exporter, _, _, _, endpoint, request :=
				startTestFrostActivationHandshakeExporter(t, point)
			readiness, ok :=
				exporter.readiness.(*testFrostProductionSignerReadiness)
			if !ok {
				t.Fatal("unexpected production signer readiness verifier")
			}

			response := postTestFrostActivationHandshake(t, endpoint, request)
			response.Body.Close()
			response = awaitTestFrostActivationHandshake(
				t,
				endpoint,
				request,
				http.StatusOK,
			)
			response.Body.Close()

			mutate(readiness)
			response = postTestFrostActivationHandshake(t, endpoint, request)
			if response.StatusCode != http.StatusServiceUnavailable ||
				response.Header.Get("Retry-After") !=
					frostActivationHandshakeRetryAfter {
				response.Body.Close()
				t.Fatalf(
					"obsolete cached signer state returned status [%d] with retry [%s]",
					response.StatusCode,
					response.Header.Get("Retry-After"),
				)
			}
			response.Body.Close()

			readiness.setInteractive(true)
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
			if !handshake.Payload.State.Healthy {
				t.Fatal("fresh stable signer state did not recover healthy attestation")
			}
			if name == "retained key material changes" &&
				(handshake.Payload.State.NativeSignerState.StateGeneration != 8 ||
					handshake.Payload.State.NativeSignerState.StateCommitment !=
						frostActivationHex32([32]byte{0x41})) {
				t.Fatalf(
					"reconciled attestation did not use the current native state: %+v",
					handshake.Payload.State.NativeSignerState,
				)
			}
			if name == "native state rolls back" &&
				handshake.Payload.State.NativeSignerState.StateGeneration != 6 {
				t.Fatalf(
					"reconciled attestation did not use the current native state: %+v",
					handshake.Payload.State.NativeSignerState,
				)
			}
		})
	}
}

// TestFrostActivationHandshakeExporter_SignsLiveNativeSignerState pins that the
// signed attestation carries native signer facts read at the signing boundary,
// not the ones the finality-keyed reconciliation cache recorded.
//
// The cache is refreshed only when finality advances, which is minutes apart,
// while an authorized signing batch advances the state generation, the anchor
// revision and both restartable headrooms continuously. Those fields are
// exactly the ones the readiness revalidation permits to move, so exporting
// them from the cache would sign a headroom that is stale by orders of
// magnitude and mislead a consumer scheduling offline anchor rotation.
func TestFrostActivationHandshakeExporter_SignsLiveNativeSignerState(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	exporter, _, _, _, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)
	readiness, ok := exporter.readiness.(*testFrostProductionSignerReadiness)
	if !ok {
		t.Fatal("unexpected production signer readiness verifier")
	}

	response := postTestFrostActivationHandshake(t, endpoint, request)
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	response.Body.Close()

	cached := testFrostProductionSignerInventorySnapshot()

	// One authorized batch: the engine persists a consumption marker and the
	// output barrier advances the anchor revision once, so the checkpoint
	// chain, the revision and both headrooms move while identity, key material
	// and the trust head stay put.
	advanced := cached
	advanced.StateGeneration++
	advanced.PreviousStateCommitment = cached.StateCommitment
	advanced.StateCommitment = [32]byte{0x41}
	advanced.StateImageDigest = [32]byte{0x42}
	advanced.RestartableGenerationHeadroom--
	advanced.CurrentAnchorRevision++
	advanced.RestartableRevisionHeadroom--
	readiness.setInventory(advanced)

	// The advance is not a readiness change, so the very next challenge is
	// answered from the same cache without a reconciliation round trip.
	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf(
			"authorized durable advance was refused with status [%d]: %s",
			response.StatusCode,
			body,
		)
	}
	defer response.Body.Close()
	handshake := &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(handshake); err != nil {
		t.Fatal(err)
	}

	exporter.reconciliationMutex.Lock()
	stillCached := exporter.reconciliationCompleted != nil &&
		exporter.reconciliationCompleted.inventory == cached
	exporter.reconciliationMutex.Unlock()
	if !stillCached {
		t.Fatal("reconciliation cache was refreshed; the export is not proven live")
	}

	signed := handshake.Payload.State.NativeSignerState
	if signed.StateGeneration != advanced.StateGeneration ||
		signed.StateCommitment != frostActivationHex32(advanced.StateCommitment) ||
		signed.PreviousStateCommitment !=
			frostActivationHex32(advanced.PreviousStateCommitment) ||
		signed.StateImageDigest !=
			frostActivationHex32(advanced.StateImageDigest) ||
		signed.CurrentAnchorRevision != advanced.CurrentAnchorRevision ||
		signed.RestartableRevisionHeadroom !=
			advanced.RestartableRevisionHeadroom ||
		signed.RestartableGenerationHeadroom !=
			advanced.RestartableGenerationHeadroom {
		t.Fatalf(
			"attestation signed the cached native signer state instead of the live one: %+v",
			signed,
		)
	}
	if !handshake.Payload.State.Healthy {
		t.Fatal("live native signer state did not produce a healthy attestation")
	}
}

func TestFrostActivationHandshakeExporter_RejectsFlatFloorOnlyAnchorWarning(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	exporter, _, _, _, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)
	readiness, ok := exporter.readiness.(*testFrostProductionSignerReadiness)
	if !ok {
		t.Fatal("unexpected production signer readiness verifier")
	}

	response := postTestFrostActivationHandshake(t, endpoint, request)
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	response.Body.Close()

	cost, err := frostPreSignAnchoredInputCost(20, signingAttemptsLimit)
	if err != nil {
		t.Fatal(err)
	}
	inventory := testFrostProductionSignerInventorySnapshot()
	inventory.StateGeneration = inventory.CertifiedFloorGeneration +
		FrostNativeSignerAnchorMaximumHistoryProofEntries -
		(cost.Generations - 1)
	inventory.PreviousStateCommitment = inventory.StateCommitment
	inventory.StateCommitment = [32]byte{0x51}
	inventory.StateImageDigest = [32]byte{0x52}
	inventory.RestartableGenerationHeadroom = cost.Generations - 1
	inventory.CurrentAnchorRevision = inventory.CertifiedFloorRevision +
		FrostNativeSignerAnchorMaximumHistoryEvents -
		(cost.Revisions - 1)
	inventory.RestartableRevisionHeadroom = cost.Revisions - 1
	// This is the production bug under test: both windows remain above the flat
	// floor, but neither can reserve this node's next twenty-seat input.
	inventory.AnchorRotationWarning = false
	readiness.setInventory(inventory)

	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf(
			"flat-floor-only warning produced status [%d]: %s",
			response.StatusCode,
			body,
		)
	}
	response.Body.Close()

	inventory.AnchorRotationWarning = true
	readiness.setInventory(inventory)
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
	if handshake.Payload.State.Healthy ||
		!handshake.Payload.State.NativeSignerState.AnchorRotationWarning {
		t.Fatalf(
			"workload-exhausted signer attested healthy: %+v",
			handshake.Payload.State.NativeSignerState,
		)
	}
}

// TestFrostActivationHandshakeHealthy_RequiresAnUnpoisonedStateAnchor pins the
// health term itself, independently of how the payload is assembled: an
// otherwise perfect state is unhealthy once the native signer state anchor is
// terminally poisoned.
func TestFrostActivationHandshakeHealthy_RequiresAnUnpoisonedStateAnchor(
	t *testing.T,
) {
	state := frostActivationHandshakeState{
		InteractiveSigningReady:       true,
		NonceShareGateEnforced:        true,
		DurableBitcoinOutboxRecovered: true,
		QuarantineFailClosed:          true,
		NativeSignerState: frostActivationNativeSignerState{
			ExternalRollbackAnchorBound: true,
			Complete:                    true,
		},
	}
	if !frostActivationHandshakeHealthy(state) {
		t.Fatal("a ready state did not attest healthy")
	}
	state.NativeSignerState.StateAnchorPoisoned = true
	if frostActivationHandshakeHealthy(state) {
		t.Fatal(
			"a terminally poisoned native signer state anchor still attested " +
				"healthy, while the node refuses every request-taking signer " +
				"call it is asked for",
		)
	}
}

// TestFrostActivationHandshakeExporter_AttestsPoisonedStateAnchorAsUnhealthy
// pins that a terminally poisoned native tBTC signer state anchor reaches the
// signed attestation, both as the health verdict and as the named reason for
// it.
//
// The barrier guards every request-taking native signer call, so a node that
// latches it keeps running and silently stops signing. Nothing else in this
// payload moves when that happens: interactive readiness is configuration, and
// every native signer fact is read through no-arg entry points that never enter
// the barrier's request-taking path. Without the poisoning term, a permissioned
// FROST set could therefore lose threshold while every member attests health,
// with no node reporting a cause.
func TestFrostActivationHandshakeExporter_AttestsPoisonedStateAnchorAsUnhealthy(
	t *testing.T,
) {
	point := frostActivationEthereumPoint{
		BlockNumber: 123,
		BlockHash:   frostActivationHex32([32]byte{0x44}),
	}
	signal := installTestFrostActivationStateAnchorPoisonSignal(t)
	exporter, _, _, _, endpoint, request :=
		startTestFrostActivationHandshakeExporter(t, point)

	response := postTestFrostActivationHandshake(t, endpoint, request)
	response.Body.Close()
	response = awaitTestFrostActivationHandshake(
		t,
		endpoint,
		request,
		http.StatusOK,
	)
	healthy := &frostActivationSignedHandshake{}
	healthyErr := json.NewDecoder(response.Body).Decode(healthy)
	response.Body.Close()
	if healthyErr != nil {
		t.Fatal(healthyErr)
	}
	if !healthy.Payload.State.Healthy ||
		healthy.Payload.State.NativeSignerState.StateAnchorPoisoned {
		t.Fatalf(
			"an unpoisoned state anchor attested healthy [%t] with "+
				"stateAnchorPoisoned [%t]",
			healthy.Payload.State.Healthy,
			healthy.Payload.State.NativeSignerState.StateAnchorPoisoned,
		)
	}
	exporter.reconciliationMutex.Lock()
	cached := exporter.reconciliationCompleted
	exporter.reconciliationMutex.Unlock()
	if cached == nil {
		t.Fatal("the healthy attestation left no reconciliation cache")
	}

	// One more healthy read, so the next attestation builds its payload before
	// the barrier latches and meets the latch only under the signing lock. That
	// attestation must be refused rather than signed, and must be refused as
	// retryable: the latch is one-way, so the immediate retry answers with the
	// poisoned verdict instead of flapping.
	poisoning := fmt.Errorf(
		"native tBTC signer state anchor is terminally poisoned: " +
			"anchor compare-and-swap lost after the native mutation",
	)
	signal.latch(poisoning, 1)
	refused := postTestFrostActivationHandshake(t, endpoint, request)
	refusedBody, _ := io.ReadAll(refused.Body)
	refused.Body.Close()
	if refused.StatusCode != http.StatusServiceUnavailable ||
		refused.Header.Get("Retry-After") != frostActivationHandshakeRetryAfter {
		t.Fatalf(
			"an anchor poisoned during signing was attested with status [%d] "+
				"and Retry-After [%s]: %s",
			refused.StatusCode,
			refused.Header.Get("Retry-After"),
			refusedBody,
		)
	}

	response = postTestFrostActivationHandshake(t, endpoint, request)
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf(
			"a poisoned node refused to attest at all with status [%d]: %s; "+
				"the attestation is the only place the reason is reported",
			response.StatusCode,
			body,
		)
	}
	defer response.Body.Close()
	poisoned := &frostActivationSignedHandshake{}
	if err := json.NewDecoder(response.Body).Decode(poisoned); err != nil {
		t.Fatal(err)
	}
	if poisoned.Payload.State.Healthy ||
		!poisoned.Payload.State.NativeSignerState.StateAnchorPoisoned {
		t.Fatalf(
			"a terminally poisoned node attested healthy [%t] with "+
				"stateAnchorPoisoned [%t], while it refuses every "+
				"request-taking native signer call it is asked for",
			poisoned.Payload.State.Healthy,
			poisoned.Payload.State.NativeSignerState.StateAnchorPoisoned,
		)
	}
	// Exactly two bits differ from the healthy attestation: the verdict and the
	// reason for it. An operator holding both must be able to tell that this
	// node stopped signing because of the anchor and not because some other
	// activation invariant also broke.
	expected := healthy.Payload.State
	expected.NativeSignerState.StateAnchorPoisoned = true
	expected.Healthy = false
	if !reflect.DeepEqual(poisoned.Payload.State, expected) {
		t.Fatal(
			"the poisoned attestation moved more than the health verdict and " +
				"its reason, so it does not name the anchor as the cause",
		)
	}

	// The reason must be inside what the node signed, not decoration added on
	// the way out: a consumer recomputes the verdict from the transcript it
	// verified.
	publicKeyDER, err := base64.StdEncoding.Strict().DecodeString(
		poisoned.SignerPublicKeySPKI,
	)
	if err != nil {
		t.Fatal(err)
	}
	parsedPublicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
	if !ok {
		t.Fatal("attestation public key is not Ed25519")
	}
	canonicalPayload, err := canonicalFrostActivationValue(poisoned.Payload)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(poisoned.Signature)
	if err != nil || !ed25519.Verify(
		publicKey,
		append(
			[]byte(frostActivationHandshakeSignatureDomain),
			canonicalPayload...,
		),
		signature,
	) {
		t.Fatal(
			"poisoned attestation signature did not verify over the canonical payload",
		)
	}
	if frostActivationHandshakeHealthy(poisoned.Payload.State) {
		t.Fatal(
			"the health verdict recomputed from the signed payload disagrees " +
				"with the attested one",
		)
	}

	// The unhealthy attestation was served from the same reconciliation cache
	// that produced the healthy one, which is what proves the verdict is read
	// live at the signing boundary instead of being cached with the rest of the
	// activation state and going stale across the transition.
	exporter.reconciliationMutex.Lock()
	sameCache := exporter.reconciliationCompleted == cached
	exporter.reconciliationMutex.Unlock()
	if !sameCache {
		t.Fatal(
			"the unhealthy attestation came from a refreshed reconciliation " +
				"cache, so it does not prove the barrier verdict is read live",
		)
	}
}

// testFrostActivationStateAnchorPoisonSignal stands in for the process-wide
// state anchor barrier of pkg/frost/signing. The real barrier latches its
// poisoning until the process restarts, so a test that poisoned it for real
// would leave every later test in this package unable to sign.
type testFrostActivationStateAnchorPoisonSignal struct {
	mutex sync.Mutex
	// healthyReads is the number of leading reads still answered healthy after
	// the poisoning was latched, which is how a test places the latch inside
	// the window between building an attestation and signing it.
	healthyReads int
	poisoning    error
}

func (signal *testFrostActivationStateAnchorPoisonSignal) read() error {
	signal.mutex.Lock()
	defer signal.mutex.Unlock()
	if signal.poisoning == nil {
		return nil
	}
	if signal.healthyReads > 0 {
		signal.healthyReads--
		return nil
	}
	return signal.poisoning
}

func (signal *testFrostActivationStateAnchorPoisonSignal) latch(
	poisoning error,
	healthyReads int,
) {
	signal.mutex.Lock()
	defer signal.mutex.Unlock()
	signal.poisoning = poisoning
	signal.healthyReads = healthyReads
}

func installTestFrostActivationStateAnchorPoisonSignal(
	t *testing.T,
) *testFrostActivationStateAnchorPoisonSignal {
	t.Helper()
	signal := &testFrostActivationStateAnchorPoisonSignal{}
	previous := frostActivationNativeSignerStateAnchorPoisoned
	frostActivationNativeSignerStateAnchorPoisoned = signal.read
	t.Cleanup(func() {
		frostActivationNativeSignerStateAnchorPoisoned = previous
	})
	return signal
}

func TestFrostActivationHandshakeExporter_RevalidatesOutboxStateBeforeSigning(
	t *testing.T,
) {
	testCases := map[string]func(*bitcoinBroadcastOutbox){
		"quarantine": func(outbox *bitcoinBroadcastOutbox) {
			transactionHash := bitcoin.Hash{0x91}
			outbox.records[transactionHash] = &bitcoinBroadcastOutboxRecord{
				TransactionHash: transactionHash,
				Authorization: bitcoinBroadcastAuthorization{
					ReservationID: [32]byte{0x92},
				},
				Quarantine: &bitcoinBroadcastQuarantine{},
			}
		},
		"ambiguous reservation": func(outbox *bitcoinBroadcastOutbox) {
			reservationID := [32]byte{0xa1}
			for _, transactionHash := range []bitcoin.Hash{
				{0xa2},
				{0xa3},
			} {
				outbox.records[transactionHash] = &bitcoinBroadcastOutboxRecord{
					TransactionHash: transactionHash,
					Authorization: bitcoinBroadcastAuthorization{
						ReservationID: reservationID,
					},
					Confirmation: &bitcoinBroadcastConfirmation{
						Canonical: true,
					},
				}
			}
		},
	}

	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			point := frostActivationEthereumPoint{
				BlockNumber: 123,
				BlockHash:   frostActivationHex32([32]byte{0x44}),
			}
			exporter, _, _, _, endpoint, request :=
				startTestFrostActivationHandshakeExporter(t, point)
			readiness, ok :=
				exporter.readiness.(*testFrostProductionSignerReadiness)
			if !ok {
				t.Fatal("unexpected production signer readiness verifier")
			}

			response := postTestFrostActivationHandshake(t, endpoint, request)
			response.Body.Close()
			response = awaitTestFrostActivationHandshake(
				t,
				endpoint,
				request,
				http.StatusOK,
			)
			response.Body.Close()

			unchangedStarted := make(chan struct{})
			unchangedRelease := make(chan struct{})
			readiness.blockUnchangedVerification(
				unchangedStarted,
				unchangedRelease,
			)
			type attestationResult struct {
				handshake *frostActivationSignedHandshake
				err       error
			}
			result := make(chan attestationResult, 1)
			go func() {
				handshake, err := exporter.attest(
					context.Background(),
					&request,
				)
				result <- attestationResult{handshake: handshake, err: err}
			}()

			select {
			case <-unchangedStarted:
			case <-time.After(time.Second):
				t.Fatal("signing-boundary readiness verification did not start")
			}
			exporter.outbox.mutex.Lock()
			mutate(exporter.outbox)
			exporter.outbox.mutex.Unlock()
			close(unchangedRelease)

			var obsolete attestationResult
			select {
			case obsolete = <-result:
			case <-time.After(time.Second):
				t.Fatal("activation attestation did not complete")
			}
			if obsolete.err == nil || obsolete.handshake != nil {
				t.Fatal("activation signed state from an obsolete outbox snapshot")
			}
			if !strings.Contains(
				obsolete.err.Error(),
				"outbox activation state changed before signing",
			) {
				t.Fatalf("unexpected obsolete outbox error: [%v]", obsolete.err)
			}

			exporter.outbox.mutex.Lock()
			exporter.outbox.records =
				make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord)
			exporter.outbox.mutex.Unlock()
			handshake, err := exporter.attest(context.Background(), &request)
			if err != nil {
				t.Fatalf("healthy outbox did not recover signing: [%v]", err)
			}
			if handshake == nil || !handshake.Payload.State.Healthy {
				t.Fatal("healthy stable outbox did not produce a healthy attestation")
			}
		})
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
	readiness.setInteractive(false)
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
		// v4 is the schema that did not carry the state-anchor poisoning
		// verdict, so an auditor still pinned to it would read a payload whose
		// health verdict it cannot recompute. It is refused like any other
		// superseded schema.
		"legacy v4 schema": func(request *frostActivationHandshakeRequest) {
			request.Schema = "tbtc-p2tr-production-activation-handshake/v4"
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
	checkpointAuthorities, _, _ :=
		testFrostActivationCheckpointCredentials()
	sourceIdentity := testFrostRetainedGroupCompleteIdentity()
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
			SourceTrustDomainID:       sourceIdentity.TrustDomainID,
			SourceEndpointFingerprint: sourceIdentity.EndpointFingerprint,
			SourceOperatorFingerprint: sourceIdentity.OperatorFingerprint,
			SourceIdentity:            sourceIdentity,
			MinimumGeneration:         1,
		},
		QuarantineJournal: FrostRetainedGroupQuarantineJournalManifest{
			ProtocolID:                   [32]byte{0x25},
			LiftProtocolID:               [32]byte{0x35},
			TombstoneProtocolID:          [32]byte{0x36},
			CheckpointAuthorityThreshold: 2,
			CheckpointAuthorities:        checkpointAuthorities,
			CheckpointMinimumSequence:    1,
			CheckpointPredecessorHash:    [32]byte{},
			LiftAuthorityThreshold:       2,
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

func testFrostActivationCheckpointCredentials() (
	[]FrostRetainedGroupAuthority,
	[]ed25519.PrivateKey,
	[]string,
) {
	authorities := make([]FrostRetainedGroupAuthority, 3)
	privateKeys := make([]ed25519.PrivateKey, 3)
	publicKeySPKIs := make([]string, 3)
	for index := range authorities {
		seed := make([]byte, ed25519.SeedSize)
		seed[0] = byte(0x70 + index)
		privateKey := ed25519.NewKeyFromSeed(seed)
		publicKeyDER, err := x509.MarshalPKIXPublicKey(
			privateKey.Public(),
		)
		if err != nil {
			panic(err)
		}
		authorities[index] = FrostRetainedGroupAuthority{
			AuthorityID: fmt.Sprintf(
				"checkpoint-%d",
				index+1,
			),
			PublicKeySPKIHash: sha256.Sum256(publicKeyDER),
		}
		privateKeys[index] = privateKey
		publicKeySPKIs[index] =
			base64.StdEncoding.EncodeToString(publicKeyDER)
	}
	return authorities, privateKeys, publicKeySPKIs
}

func testFrostActivationCheckpointCertificate(
	t *testing.T,
	policy frostRetainedGroupCheckpointPolicy,
	sequence uint64,
	previousHash [32]byte,
	commitment FrostRetainedGroupCheckpointCommitment,
) (FrostRetainedGroupCheckpointCertificate, [32]byte) {
	t.Helper()
	body := FrostRetainedGroupCheckpointBody{
		Schema:                  frostRetainedGroupCheckpointBodySchema,
		ProtocolBindingHash:     policy.ProtocolBindingHash,
		ManifestHash:            policy.ManifestHash,
		ProfileHash:             policy.ProfileHash,
		ImplementationSetHash:   policy.ImplementationSetHash,
		ChainID:                 policy.ChainID,
		DomainChainID:           policy.DomainChainID,
		GenesisBlockHash:        policy.GenesisBlockHash,
		AuthoritySetHash:        policy.AuthoritySetHash,
		Sequence:                sequence,
		PreviousCertificateHash: previousHash,
		Point:                   commitment.Point,
		HistoryRoot:             commitment.HistoryRoot,
		CanonicalGeneration:     commitment.CanonicalGeneration,
		CanonicalInventoryRoot:  commitment.CanonicalInventoryRoot,
		QuarantineGeneration:    commitment.QuarantineGeneration,
		QuarantineEventRoot:     commitment.QuarantineEventRoot,
		QuarantineActiveRoot:    commitment.QuarantineActiveRoot,
		QuarantineTombstoneRoot: commitment.QuarantineTombstoneRoot,
	}
	bodyHash, err := frostRetainedGroupCheckpointBodyHash(body)
	if err != nil {
		t.Fatal(err)
	}
	authorities, privateKeys, publicKeySPKIs :=
		testFrostActivationCheckpointCredentials()
	if len(authorities) != len(policy.Authorities) {
		t.Fatal("test checkpoint authority count differs from policy")
	}
	signatureHash := frostRetainedGroupCheckpointSignatureHash(bodyHash)
	signatures := make(
		[]FrostRetainedGroupCheckpointSignature,
		policy.AuthorityThreshold,
	)
	for index := range signatures {
		if authorities[index] != policy.Authorities[index] {
			t.Fatal("test checkpoint authority differs from policy")
		}
		signatures[index] = FrostRetainedGroupCheckpointSignature{
			AuthorityID:         authorities[index].AuthorityID,
			SignerPublicKeySPKI: publicKeySPKIs[index],
			Signature: base64.StdEncoding.EncodeToString(
				ed25519.Sign(
					privateKeys[index],
					signatureHash[:],
				),
			),
		}
	}
	certificate := FrostRetainedGroupCheckpointCertificate{
		Schema:     frostRetainedGroupCheckpointCertificateSchema,
		Body:       body,
		BodyHash:   bodyHash,
		Signatures: signatures,
	}
	certificateHash, err :=
		validateFrostRetainedGroupCheckpointCertificateShape(
			policy,
			certificate,
		)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, certificateHash
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
	checkpointCertificate, checkpointHash :=
		testFrostActivationCheckpointCertificate(
			t,
			checkpointPolicy,
			manifest.QuarantineJournal.CheckpointMinimumSequence,
			manifest.QuarantineJournal.CheckpointPredecessorHash,
			FrostRetainedGroupCheckpointCommitment{
				Point:                   target,
				HistoryRoot:             historyRoot,
				CanonicalGeneration:     state.SnapshotGeneration,
				CanonicalInventoryRoot:  state.InventoryRoot,
				QuarantineGeneration:    0,
				QuarantineEventRoot:     quarantineRoot,
				QuarantineActiveRoot:    activeRoot,
				QuarantineTombstoneRoot: tombstoneRoot,
			},
		)
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
			SourceIdentity:            manifest.CanonicalJournal.SourceIdentity,
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
		checkpointCertificates: map[uint64]FrostRetainedGroupCheckpointCertificate{
			checkpointHead.Sequence: checkpointCertificate,
		},
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
	commitment := FrostRetainedGroupCheckpointCommitment{
		Point:                   journal.state.CurrentPoint,
		HistoryRoot:             historyRoot,
		CanonicalGeneration:     journal.state.SnapshotGeneration,
		CanonicalInventoryRoot:  journal.state.InventoryRoot,
		QuarantineGeneration:    journal.quarantineState.Generation,
		QuarantineEventRoot:     journal.quarantineState.Root,
		QuarantineActiveRoot:    journal.quarantineState.ActiveRoot,
		QuarantineTombstoneRoot: journal.quarantineState.TombstoneRoot,
	}
	previousHash := journal.checkpointPolicy.PredecessorHash
	if sequence > journal.checkpointPolicy.MinimumSequence {
		var exists bool
		previousHash, exists =
			journal.checkpointHashes[sequence-1]
		if !exists {
			journal.mutex.Unlock()
			t.Fatalf(
				"test checkpoint predecessor [%d] is missing",
				sequence-1,
			)
		}
	}
	certificate, certificateHash :=
		testFrostActivationCheckpointCertificate(
			t,
			journal.checkpointPolicy,
			sequence,
			previousHash,
			commitment,
		)
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
	journal.checkpointCertificates[sequence] = certificate
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
