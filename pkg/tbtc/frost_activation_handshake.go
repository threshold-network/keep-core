package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"golang.org/x/sys/unix"
)

const (
	frostActivationHandshakeSchema                = "tbtc-p2tr-production-activation-handshake/v6"
	frostActivationInventorySchema                = "tbtc-p2tr-frost-wallet-group-inventory/v1"
	frostActivationHandshakeSignatureDomain       = "tbtc-p2tr-production-activation-handshake-signature/v4\x00"
	frostActivationHandshakeReconciliationTimeout = frostRetainedGroupMaximumReconciliationDuration
	frostActivationHandshakeRequestTimeout        = 5 * time.Second
	frostActivationHandshakeQuickCheckTimeout     = 2 * time.Second
	frostActivationHandshakeRetryAfter            = "1"
)

var errFrostActivationReconciliationPending = errors.New(
	"FROST activation reconciliation is pending",
)

var errFrostActivationJournalBusy = errors.New(
	"FROST activation journal live-state check is busy",
)

// frostActivationNativeSignerStateAnchorPoisoned reports the latched terminal
// state-anchor failure that makes this node refuse every request-taking native
// signer call until the process is restarted, or nil while the barrier is
// healthy. It reads an atomic and never takes the barrier mutex, so calling it
// on the attestation path cannot block behind an in-flight signing operation.
//
// It is a variable only so that tests can drive the poisoned branch. The
// barrier it reads is a process-wide singleton in pkg/frost/signing whose
// poisoned state is deliberately one-way and clearable only by restart, so a
// test that latched it for real would leave every later test in this package
// unable to sign.
var frostActivationNativeSignerStateAnchorPoisoned = frostsigning.NativeTBTCSignerStateAnchorPoisoned

type frostActivationEthereumPoint struct {
	BlockNumber uint64 `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
}

type frostActivationChallenge struct {
	Nonce           string                                 `json:"nonce"`
	ManifestHash    string                                 `json:"manifestHash"`
	BindingHash     string                                 `json:"bindingHash"`
	EthereumPoint   frostActivationEthereumPoint           `json:"ethereumPoint"`
	CheckpointFloor frostRetainedGroupWireCheckpointCursor `json:"checkpointFloor"`
}

type frostActivationHandshakeRequest struct {
	Schema    string                   `json:"schema"`
	Challenge frostActivationChallenge `json:"challenge"`
}

type frostActivationCanonicalJournalState struct {
	StoreID                   string                         `json:"storeID"`
	StoreFingerprint          string                         `json:"storeFingerprint"`
	ClusterFingerprint        string                         `json:"clusterFingerprint"`
	BindingHash               string                         `json:"bindingHash"`
	Checkpoint                frostActivationEthereumPoint   `json:"checkpoint"`
	Current                   frostActivationEthereumPoint   `json:"current"`
	DescriptorSetHash         string                         `json:"descriptorSetHash"`
	SourceTrustDomainID       string                         `json:"sourceTrustDomainID"`
	SourceEndpointFingerprint string                         `json:"sourceEndpointFingerprint"`
	SourceOperatorFingerprint string                         `json:"sourceOperatorFingerprint"`
	SourceIdentity            frostRetainedGroupWireIdentity `json:"sourceIdentity"`
	Generation                uint64                         `json:"generation"`
	Complete                  bool                           `json:"complete"`
}

type frostActivationWalletGroupInventory struct {
	Schema                   string                       `json:"schema"`
	Point                    frostActivationEthereumPoint `json:"point"`
	SnapshotGeneration       uint64                       `json:"snapshotGeneration"`
	InventoryRoot            string                       `json:"inventoryRoot"`
	WalletCount              uint64                       `json:"walletCount"`
	MinimumActualGroupSize   uint64                       `json:"minimumActualGroupSize"`
	MaximumActualGroupSize   uint64                       `json:"maximumActualGroupSize"`
	MembershipAmbiguityCount uint64                       `json:"membershipAmbiguityCount"`
	GroupSizeViolationCount  uint64                       `json:"groupSizeViolationCount"`
	Complete                 bool                         `json:"complete"`
}

type frostActivationQuarantineJournalState struct {
	ProtocolID             string `json:"protocolID"`
	StoreID                string `json:"storeID"`
	StoreFingerprint       string `json:"storeFingerprint"`
	ClusterFingerprint     string `json:"clusterFingerprint"`
	Root                   string `json:"root"`
	ActiveRoot             string `json:"activeRoot"`
	TombstoneRoot          string `json:"tombstoneRoot"`
	Generation             uint64 `json:"generation"`
	CurrentQuarantineCount uint64 `json:"currentQuarantineCount"`
	TombstoneCount         uint64 `json:"tombstoneCount"`
	Complete               bool   `json:"complete"`
}

type frostActivationNativeSignerState struct {
	Schema                        string `json:"schema"`
	StoreFingerprint              string `json:"storeFingerprint"`
	StateGeneration               uint64 `json:"stateGeneration"`
	StateCommitment               string `json:"stateCommitment"`
	PreviousStateCommitment       string `json:"previousStateCommitment"`
	StateImageDigest              string `json:"stateImageDigest"`
	InventoryCommitment           string `json:"inventoryCommitment"`
	RetainedWalletCount           uint64 `json:"retainedWalletCount"`
	RetainedKeyPackageCount       uint64 `json:"retainedKeyPackageCount"`
	ExternalRollbackAnchorBound   bool   `json:"externalRollbackAnchorBound"`
	TrustCertificateSequence      uint64 `json:"trustCertificateSequence"`
	TrustCertificateDigest        string `json:"trustCertificateDigest"`
	AnchorServiceEpoch            uint64 `json:"anchorServiceEpoch"`
	CertifiedFloorRevision        uint64 `json:"certifiedFloorRevision"`
	CertifiedFloorGeneration      uint64 `json:"certifiedFloorGeneration"`
	CurrentAnchorRevision         uint64 `json:"currentAnchorRevision"`
	RestartableRevisionHeadroom   uint64 `json:"restartableRevisionHeadroom"`
	RestartableGenerationHeadroom uint64 `json:"restartableGenerationHeadroom"`
	AnchorRotationWarning         bool   `json:"anchorRotationWarning"`
	StateAnchorPoisoned           bool   `json:"stateAnchorPoisoned"`
	Complete                      bool   `json:"complete"`
}

type frostActivationCheckpointJournalState struct {
	ManifestMinimumSequence uint64                                        `json:"manifestMinimumSequence"`
	ManifestPredecessorHash string                                        `json:"manifestPredecessorHash"`
	ChallengeFloor          frostRetainedGroupWireCheckpointCursor        `json:"challengeFloor"`
	DurableHead             frostRetainedGroupWireCheckpointCursor        `json:"durableHead"`
	Point                   frostActivationEthereumPoint                  `json:"point"`
	HistoryRoot             string                                        `json:"historyRoot"`
	CanonicalGeneration     uint64                                        `json:"canonicalGeneration"`
	CanonicalInventoryRoot  string                                        `json:"canonicalInventoryRoot"`
	QuarantineGeneration    uint64                                        `json:"quarantineGeneration"`
	QuarantineEventRoot     string                                        `json:"quarantineEventRoot"`
	QuarantineActiveRoot    string                                        `json:"quarantineActiveRoot"`
	QuarantineTombstoneRoot string                                        `json:"quarantineTombstoneRoot"`
	Ancestry                []frostRetainedGroupWireCheckpointCertificate `json:"ancestry"`
	Complete                bool                                          `json:"complete"`
}

type frostActivationHandshakeState struct {
	ProtocolID                                string                                `json:"protocolID"`
	ReservationProtocolID                     string                                `json:"reservationProtocolID"`
	BitcoinOutboxProtocolID                   string                                `json:"bitcoinOutboxProtocolID"`
	SigningPolicyHash                         string                                `json:"signingPolicyHash"`
	DurableSessionStoreFingerprint            string                                `json:"durableSessionStoreFingerprint"`
	ShareRepairActivationRegistryRoot         string                                `json:"shareRepairActivationRegistryRoot"`
	CompleteRouterAddress                     string                                `json:"completeRouterAddress"`
	AuthorizationRegistryAddress              string                                `json:"authorizationRegistryAddress"`
	Threshold                                 uint64                                `json:"threshold"`
	MaximumGroupSize                          uint64                                `json:"maximumGroupSize"`
	RetainedGroupInventoryProtocolID          string                                `json:"retainedGroupInventoryProtocolID"`
	FrostWalletGroupInventory                 frostActivationWalletGroupInventory   `json:"frostWalletGroupInventory"`
	CanonicalJournal                          frostActivationCanonicalJournalState  `json:"canonicalJournal"`
	QuarantineJournal                         frostActivationQuarantineJournalState `json:"quarantineJournal"`
	CheckpointJournal                         frostActivationCheckpointJournalState `json:"checkpointJournal"`
	NativeSignerState                         frostActivationNativeSignerState      `json:"nativeSignerState"`
	InteractiveSigningReady                   bool                                  `json:"interactiveSigningReady"`
	FinalizedReservationReadbackEnforced      bool                                  `json:"finalizedReservationReadbackEnforced"`
	ExactTransactionAuthorizationRootEnforced bool                                  `json:"exactTransactionAuthorizationRootEnforced"`
	NonceShareGateEnforced                    bool                                  `json:"nonceShareGateEnforced"`
	DurableBitcoinOutboxRecovered             bool                                  `json:"durableBitcoinOutboxRecovered"`
	QuarantineFailClosed                      bool                                  `json:"quarantineFailClosed"`
	Healthy                                   bool                                  `json:"healthy"`
}

type frostActivationHandshakePayload struct {
	Schema        string                        `json:"schema"`
	Kind          string                        `json:"kind"`
	Nonce         string                        `json:"nonce"`
	ManifestHash  string                        `json:"manifestHash"`
	BindingHash   string                        `json:"bindingHash"`
	EthereumPoint frostActivationEthereumPoint  `json:"ethereumPoint"`
	State         frostActivationHandshakeState `json:"state"`
}

type frostActivationSignedHandshake struct {
	Payload             frostActivationHandshakePayload `json:"payload"`
	SignerPublicKeySPKI string                          `json:"signerPublicKeySpki"`
	Signature           string                          `json:"signature"`
}

type frostActivationHandshakeExporter struct {
	endpoint      *url.URL
	privateKey    ed25519.PrivateKey
	publicKeySPKI string
	manifest      FrostPreSignActivationRuntimeManifest
	bindingHash   [32]byte
	pointVerifier FrostPreSignActivationPointVerifier
	storeBinding  *frostDurableSessionStoreBinding
	outbox        *bitcoinBroadcastOutbox
	journal       *frostRetainedGroupJournal
	readiness     frostActivationHandshakeReadinessVerifier

	mutex                sync.Mutex
	listener             net.Listener
	server               *http.Server
	reconciliationCancel context.CancelFunc
	closed               bool

	reconciliationMutex        sync.Mutex
	reconciliationWake         chan struct{}
	reconciliationSequence     uint64
	reconciliationDesired      *frostActivationReconciliationJob
	reconciliationActive       *frostActivationReconciliationJob
	reconciliationActiveCancel context.CancelFunc
	reconciliationCompleted    *frostActivationReconciliationCache
}

type frostActivationReconciliationJob struct {
	sequence uint64
	point    FrostPreSignFinality
}

type frostActivationJournalStamp struct {
	bindingHash             [32]byte
	canonicalPoint          FrostPreSignFinality
	canonicalGeneration     uint64
	canonicalBatchRoot      [32]byte
	canonicalInventory      [32]byte
	quarantinePoint         FrostPreSignFinality
	quarantineGeneration    uint64
	quarantineBatchRoot     [32]byte
	quarantineRoot          [32]byte
	quarantineActiveRoot    [32]byte
	quarantineTombstoneRoot [32]byte
	checkpointSequence      uint64
	checkpointHash          [32]byte
	checkpointHistoryRoot   [32]byte
}

type frostActivationReconciliationCache struct {
	point                   FrostPreSignFinality
	journal                 frostRetainedGroupJournalSnapshot
	inventory               frostNativeSignerInventorySnapshot
	interactiveSigningReady bool
	readiness               frostProductionSignerReadinessSnapshot
	stamp                   frostActivationJournalStamp
}

type frostActivationHandshakeReadinessVerifier interface {
	frostProductionSignerReadinessVerifier
	// revalidateFrostProductionSignerReadinessInventory revalidates cached
	// readiness and hands back the live native signer inventory it read, so a
	// signed attestation can export what the signer reports now rather than
	// what the finality-keyed reconciliation cache recorded minutes ago.
	revalidateFrostProductionSignerReadinessInventory(
		context.Context,
		*frostProductionSignerReadinessSnapshot,
	) (*frostNativeSignerInventorySnapshot, error)
}

func newFrostActivationHandshakeExporter(
	endpoint string,
	privateKeyPath string,
	manifest FrostPreSignActivationRuntimeManifest,
	pointVerifier FrostPreSignActivationPointVerifier,
	storeBinding *frostDurableSessionStoreBinding,
	outbox *bitcoinBroadcastOutbox,
	journal *frostRetainedGroupJournal,
	readiness frostActivationHandshakeReadinessVerifier,
) (*frostActivationHandshakeExporter, error) {
	parsedEndpoint, err := validateFrostActivationHandshakeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	durableSessionStoreFingerprint, durableSessionStoreFingerprintErr :=
		parseFrostActivationHex32(manifest.DurableSessionStoreFingerprint)
	if manifest.ManifestHash == [32]byte{} ||
		manifest.AttestationSignerKeyHash == [32]byte{} ||
		manifest.Threshold != 51 || manifest.MaximumGroupSize != 100 ||
		manifest.CanonicalJournal.StoreFingerprint == [32]byte{} ||
		manifest.QuarantineJournal.ProtocolID == [32]byte{} ||
		manifest.CanonicalJournal.StoreID == manifest.QuarantineJournal.StoreID ||
		manifest.CanonicalJournal.StoreFingerprint == manifest.QuarantineJournal.StoreFingerprint ||
		manifest.CanonicalJournal.ClusterFingerprint == manifest.QuarantineJournal.ClusterFingerprint ||
		durableSessionStoreFingerprintErr != nil || durableSessionStoreFingerprint == [32]byte{} ||
		durableSessionStoreFingerprint == manifest.CanonicalJournal.StoreFingerprint ||
		durableSessionStoreFingerprint == manifest.QuarantineJournal.StoreFingerprint ||
		pointVerifier == nil || storeBinding == nil || outbox == nil ||
		journal == nil || readiness == nil {
		return nil, fmt.Errorf("FROST activation handshake dependencies are invalid")
	}
	boundStoreFingerprint, err := storeBinding.verify()
	if err != nil || boundStoreFingerprint != durableSessionStoreFingerprint {
		return nil, fmt.Errorf(
			"FROST activation handshake durable session store is not bound to the signed manifest",
		)
	}
	if !frostShareRepairActivationReady(
		manifest.ShareRepairActivationRegistryRoot,
	) {
		return nil, fmt.Errorf(
			"FROST share-repair activation state is not ready for the signed manifest",
		)
	}
	privateKey, publicKeyDER, err := loadFrostActivationAttestationKey(privateKeyPath)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(publicKeyDER) != manifest.AttestationSignerKeyHash {
		return nil, fmt.Errorf("FROST activation attestation key differs from signed manifest")
	}
	journal.mutex.Lock()
	bindingHash := journal.metadata.BindingHash
	journalBindingValid := bindingHash != [32]byte{} &&
		journal.metadata.ManifestHash == manifest.ManifestHash &&
		journal.quarantineMetadata.ManifestHash == manifest.ManifestHash &&
		journal.quarantineMetadata.BindingHash == bindingHash &&
		journal.state.BindingHash == bindingHash &&
		journal.quarantineState.BindingHash == bindingHash &&
		journal.checkpointState.BindingHash == bindingHash
	journal.mutex.Unlock()
	if !journalBindingValid {
		return nil, fmt.Errorf(
			"FROST activation handshake journal binding differs from signed runtime state",
		)
	}
	exporter := &frostActivationHandshakeExporter{
		endpoint:           parsedEndpoint,
		privateKey:         privateKey,
		publicKeySPKI:      base64.StdEncoding.EncodeToString(publicKeyDER),
		manifest:           manifest,
		bindingHash:        bindingHash,
		pointVerifier:      pointVerifier,
		storeBinding:       storeBinding,
		outbox:             outbox,
		journal:            journal,
		readiness:          readiness,
		reconciliationWake: make(chan struct{}, 1),
	}
	return exporter, nil
}

func validateFrostActivationHandshakeEndpoint(value string) (*url.URL, error) {
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Scheme != "http" || endpoint.User != nil ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.RawPath != "" ||
		endpoint.Path == "" || path.Clean(endpoint.Path) != endpoint.Path {
		return nil, fmt.Errorf("FROST activation handshake endpoint is invalid")
	}
	host := net.ParseIP(endpoint.Hostname())
	if host == nil || !host.IsLoopback() || endpoint.Port() == "" || endpoint.Port() == "0" {
		return nil, fmt.Errorf("FROST activation handshake endpoint must be numeric loopback with a fixed port")
	}
	if _, err := net.LookupPort("tcp", endpoint.Port()); err != nil {
		return nil, fmt.Errorf("FROST activation handshake endpoint port is invalid: [%w]", err)
	}
	return endpoint, nil
}

func loadFrostActivationAttestationKey(
	keyPath string,
) (ed25519.PrivateKey, []byte, error) {
	data, err := readSecureFrostActivationFile(keyPath, 16*1024)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read FROST activation attestation key: [%w]", err)
	}
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, nil, fmt.Errorf("FROST activation attestation key must be one PKCS#8 PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse FROST activation attestation key: [%w]", err)
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("FROST activation attestation key is not Ed25519")
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, fmt.Errorf("cannot encode FROST activation attestation public key: [%w]", err)
	}
	return append(ed25519.PrivateKey{}, privateKey...), publicKeyDER, nil
}

func readSecureFrostActivationFile(filePath string, limit int64) ([]byte, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("path is empty")
	}
	file, err := openSecureBitcoinBroadcastFile(filePath, unix.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, fmt.Errorf("secure activation file size is invalid")
	}
	return data, nil
}

func (fahe *frostActivationHandshakeExporter) start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("FROST activation handshake context is nil")
	}
	fahe.mutex.Lock()
	defer fahe.mutex.Unlock()
	if fahe.listener != nil || fahe.closed {
		return fmt.Errorf("FROST activation handshake exporter is already started or closed")
	}
	listener, err := net.Listen("tcp", fahe.endpoint.Host)
	if err != nil {
		return fmt.Errorf("cannot listen for FROST activation handshake: [%w]", err)
	}
	server := &http.Server{
		Handler:           http.HandlerFunc(fahe.serveHTTP),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      frostActivationHandshakeRequestTimeout + time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    4096,
	}
	reconciliationContext, reconciliationCancel := context.WithCancel(ctx)
	fahe.listener = listener
	fahe.server = server
	fahe.reconciliationCancel = reconciliationCancel
	go fahe.reconciliationWorker(reconciliationContext)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Errorf("FROST activation handshake exporter failed: [%v]", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Errorf("cannot stop FROST activation handshake exporter: [%v]", err)
		}
	}()
	return nil
}

func (fahe *frostActivationHandshakeExporter) close() error {
	fahe.mutex.Lock()
	defer fahe.mutex.Unlock()
	if fahe.closed {
		return nil
	}
	fahe.closed = true
	if fahe.reconciliationCancel != nil {
		fahe.reconciliationCancel()
	}
	if fahe.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return fahe.server.Shutdown(ctx)
}

func (fahe *frostActivationHandshakeExporter) reconciliationWorker(
	ctx context.Context,
) {
	fahe.runReconciliationWorker(ctx, fahe.reconcileActivationState)
}

func (fahe *frostActivationHandshakeExporter) runReconciliationWorker(
	ctx context.Context,
	reconcile func(
		context.Context,
		FrostPreSignFinality,
	) (*frostActivationReconciliationCache, error),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-fahe.reconciliationWake:
		}
		for {
			job, reconciliationContext, cancel := fahe.takeReconciliationJob(ctx)
			if job == nil {
				break
			}
			cache, err := reconcile(
				reconciliationContext,
				job.point,
			)
			cancel()
			if !fahe.completeReconciliationJob(job, cache, err) {
				break
			}
		}
	}
}

func (fahe *frostActivationHandshakeExporter) takeReconciliationJob(
	ctx context.Context,
) (
	*frostActivationReconciliationJob,
	context.Context,
	context.CancelFunc,
) {
	fahe.reconciliationMutex.Lock()
	defer fahe.reconciliationMutex.Unlock()
	if fahe.reconciliationDesired == nil {
		return nil, nil, nil
	}
	job := fahe.reconciliationDesired
	fahe.reconciliationDesired = nil
	reconciliationContext, cancel := context.WithTimeout(
		ctx,
		frostActivationHandshakeReconciliationTimeout,
	)
	fahe.reconciliationActive = job
	fahe.reconciliationActiveCancel = cancel
	return job, reconciliationContext, cancel
}

func (fahe *frostActivationHandshakeExporter) completeReconciliationJob(
	job *frostActivationReconciliationJob,
	cache *frostActivationReconciliationCache,
	reconciliationErr error,
) bool {
	fahe.reconciliationMutex.Lock()
	if fahe.reconciliationActive != nil &&
		fahe.reconciliationActive.sequence == job.sequence {
		fahe.reconciliationActive = nil
		fahe.reconciliationActiveCancel = nil
	}
	if reconciliationErr == nil && cache != nil &&
		fahe.reconciliationSequence == job.sequence &&
		fahe.reconciliationDesired == nil {
		fahe.reconciliationCompleted = cache
	}
	checkpointRecoveryProgress := errors.Is(
		reconciliationErr,
		errFrostRetainedGroupCheckpointRecoveryProgress,
	)
	if checkpointRecoveryProgress &&
		fahe.reconciliationSequence == job.sequence &&
		fahe.reconciliationDesired == nil {
		fahe.reconciliationSequence++
		fahe.reconciliationDesired = &frostActivationReconciliationJob{
			sequence: fahe.reconciliationSequence,
			point:    job.point,
		}
	}
	hasDesired := fahe.reconciliationDesired != nil
	fahe.reconciliationMutex.Unlock()
	if checkpointRecoveryProgress {
		logger.Infof(
			"background FROST activation reconciliation advanced the checkpoint recovery cursor for block [%d]",
			job.point.BlockNumber,
		)
	} else if reconciliationErr != nil {
		logger.Warnf(
			"background FROST activation reconciliation failed for block [%d]: [%v]",
			job.point.BlockNumber,
			reconciliationErr,
		)
	}
	return hasDesired
}

func (fahe *frostActivationHandshakeExporter) queueReconciliation(
	point FrostPreSignFinality,
	force bool,
) {
	fahe.reconciliationMutex.Lock()
	if force || (fahe.reconciliationCompleted != nil &&
		fahe.reconciliationCompleted.point != point) {
		fahe.reconciliationCompleted = nil
	}
	if fahe.reconciliationDesired != nil &&
		fahe.reconciliationDesired.point == point {
		fahe.reconciliationMutex.Unlock()
		return
	}
	if !force && fahe.reconciliationDesired == nil &&
		fahe.reconciliationActive != nil &&
		fahe.reconciliationActive.point == point {
		fahe.reconciliationMutex.Unlock()
		return
	}
	fahe.reconciliationSequence++
	job := &frostActivationReconciliationJob{
		sequence: fahe.reconciliationSequence,
		point:    point,
	}
	fahe.reconciliationDesired = job
	if fahe.reconciliationActiveCancel != nil {
		fahe.reconciliationActiveCancel()
	}
	fahe.reconciliationMutex.Unlock()
	select {
	case fahe.reconciliationWake <- struct{}{}:
	default:
	}
}

func (fahe *frostActivationHandshakeExporter) cachedReconciliation(
	point FrostPreSignFinality,
) *frostActivationReconciliationCache {
	fahe.reconciliationMutex.Lock()
	defer fahe.reconciliationMutex.Unlock()
	if fahe.reconciliationCompleted == nil ||
		fahe.reconciliationCompleted.point != point {
		return nil
	}
	cache := *fahe.reconciliationCompleted
	cache.readiness.Journal = &cache.journal
	cache.readiness.Inventory = &cache.inventory
	return &cache
}

func (fahe *frostActivationHandshakeExporter) reconcileActivationState(
	ctx context.Context,
	finality FrostPreSignFinality,
) (*frostActivationReconciliationCache, error) {
	if err := fahe.pointVerifier.VerifyFrostPreSignActivationPoint(
		ctx,
		finality,
	); err != nil {
		return nil, fmt.Errorf("cannot verify FROST activation point: [%w]", err)
	}
	readinessSnapshot, err := fahe.readiness.verifyFrostProductionSignerReadiness(
		ctx,
		finality,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot verify production FROST signer readiness: [%w]",
			err,
		)
	}
	journalSnapshot := readinessSnapshot.Journal
	inventorySnapshot := readinessSnapshot.Inventory
	if journalSnapshot == nil || inventorySnapshot == nil {
		return nil, fmt.Errorf(
			"production FROST signer readiness snapshot is incomplete",
		)
	}
	if err := fahe.validateActivationJournalSnapshot(
		journalSnapshot,
		finality,
	); err != nil {
		return nil, err
	}
	if err := fahe.pointVerifier.VerifyFrostPreSignActivationPoint(
		ctx,
		finality,
	); err != nil {
		return nil, fmt.Errorf(
			"FROST activation point changed during readiness reconciliation: [%w]",
			err,
		)
	}
	stamp, err := fahe.tryJournalStamp()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read canonical FROST retained-group journal after reconciliation: [%w]",
			err,
		)
	}
	if !frostActivationStampMatchesSnapshot(
		stamp,
		journalSnapshot,
		finality,
	) {
		return nil, fmt.Errorf(
			"canonical FROST retained-group journal changed after reconciliation",
		)
	}
	cache := &frostActivationReconciliationCache{
		point:                   finality,
		journal:                 *journalSnapshot,
		inventory:               *inventorySnapshot,
		interactiveSigningReady: readinessSnapshot.InteractiveSigningReady,
		readiness:               *readinessSnapshot,
		stamp:                   stamp,
	}
	cache.readiness.Journal = &cache.journal
	cache.readiness.Inventory = &cache.inventory
	return cache, nil
}

func (fahe *frostActivationHandshakeExporter) validateActivationJournalSnapshot(
	journalSnapshot *frostRetainedGroupJournalSnapshot,
	finality FrostPreSignFinality,
) error {
	if journalSnapshot == nil {
		return fmt.Errorf("canonical FROST retained-group journal snapshot is nil")
	}
	journalManifest := fahe.manifest.CanonicalJournal
	quarantineManifest := fahe.manifest.QuarantineJournal
	if journalSnapshot.Schema != frostRetainedGroupJournalSnapshotSchema ||
		journalSnapshot.BindingHash != fahe.bindingHash ||
		!journalSnapshot.Complete || journalSnapshot.CurrentPoint != finality ||
		journalSnapshot.StoreID != journalManifest.StoreID ||
		journalSnapshot.StoreFingerprint != journalManifest.StoreFingerprint ||
		journalSnapshot.ClusterFingerprint != journalManifest.ClusterFingerprint ||
		journalSnapshot.SnapshotGeneration < journalManifest.MinimumGeneration ||
		journalSnapshot.QuarantineProtocolID != quarantineManifest.ProtocolID ||
		journalSnapshot.QuarantineStoreID != quarantineManifest.StoreID ||
		journalSnapshot.QuarantineStoreFingerprint != quarantineManifest.StoreFingerprint ||
		journalSnapshot.QuarantineClusterFingerprint != quarantineManifest.ClusterFingerprint ||
		journalSnapshot.QuarantineGeneration < quarantineManifest.MinimumGeneration ||
		journalSnapshot.QuarantineRoot == [32]byte{} ||
		journalSnapshot.QuarantineActiveRoot == [32]byte{} ||
		journalSnapshot.QuarantineTombstoneRoot == [32]byte{} ||
		journalSnapshot.CheckpointMinimumSequence !=
			quarantineManifest.CheckpointMinimumSequence ||
		journalSnapshot.CheckpointPredecessorHash !=
			quarantineManifest.CheckpointPredecessorHash ||
		journalSnapshot.CheckpointSequence <
			quarantineManifest.CheckpointMinimumSequence ||
		journalSnapshot.CheckpointCertificateHash == [32]byte{} ||
		journalSnapshot.CheckpointHistoryRoot == [32]byte{} ||
		journalSnapshot.QuarantineCount != 0 {
		return fmt.Errorf(
			"canonical FROST retained-group journal is not activation-ready",
		)
	}
	return nil
}

func (fahe *frostActivationHandshakeExporter) tryJournalStamp() (
	frostActivationJournalStamp,
	error,
) {
	if !fahe.journal.mutex.TryLock() {
		return frostActivationJournalStamp{}, errFrostActivationJournalBusy
	}
	defer fahe.journal.mutex.Unlock()
	return fahe.journalStampLocked()
}

func (fahe *frostActivationHandshakeExporter) journalStampLocked() (
	frostActivationJournalStamp,
	error,
) {
	journal := fahe.journal
	if journal.closed ||
		journal.metadata.BindingHash != fahe.bindingHash ||
		journal.quarantineMetadata.BindingHash != fahe.bindingHash ||
		journal.state.BindingHash != fahe.bindingHash ||
		journal.quarantineState.BindingHash != fahe.bindingHash ||
		journal.checkpointState.BindingHash != fahe.bindingHash {
		return frostActivationJournalStamp{}, fmt.Errorf(
			"canonical FROST retained-group journal binding is not live",
		)
	}
	return frostActivationJournalStamp{
		bindingHash:             fahe.bindingHash,
		canonicalPoint:          journal.state.CurrentPoint,
		canonicalGeneration:     journal.state.SnapshotGeneration,
		canonicalBatchRoot:      journal.state.BatchRoot,
		canonicalInventory:      journal.state.InventoryRoot,
		quarantinePoint:         journal.quarantineState.CurrentPoint,
		quarantineGeneration:    journal.quarantineState.Generation,
		quarantineBatchRoot:     journal.quarantineState.BatchRoot,
		quarantineRoot:          journal.quarantineState.Root,
		quarantineActiveRoot:    journal.quarantineState.ActiveRoot,
		quarantineTombstoneRoot: journal.quarantineState.TombstoneRoot,
		checkpointSequence:      journal.checkpointState.Sequence,
		checkpointHash:          journal.checkpointState.CertificateHash,
		checkpointHistoryRoot:   journal.checkpointState.HistoryRoot,
	}, nil
}

func frostActivationStampMatchesSnapshot(
	stamp frostActivationJournalStamp,
	snapshot *frostRetainedGroupJournalSnapshot,
	point FrostPreSignFinality,
) bool {
	return snapshot != nil &&
		stamp.bindingHash == snapshot.BindingHash &&
		stamp.canonicalPoint == point &&
		stamp.quarantinePoint == point &&
		stamp.canonicalGeneration == snapshot.SnapshotGeneration &&
		stamp.canonicalBatchRoot == snapshot.BatchRoot &&
		stamp.canonicalInventory == snapshot.InventoryRoot &&
		stamp.quarantineGeneration == snapshot.QuarantineGeneration &&
		stamp.quarantineRoot == snapshot.QuarantineRoot &&
		stamp.quarantineActiveRoot == snapshot.QuarantineActiveRoot &&
		stamp.quarantineTombstoneRoot == snapshot.QuarantineTombstoneRoot &&
		stamp.checkpointSequence == snapshot.CheckpointSequence &&
		stamp.checkpointHash == snapshot.CheckpointCertificateHash &&
		stamp.checkpointHistoryRoot == snapshot.CheckpointHistoryRoot
}

func (fahe *frostActivationHandshakeExporter) verifyActivationPointQuick(
	ctx context.Context,
	point FrostPreSignFinality,
) error {
	quickContext, cancel := context.WithTimeout(
		ctx,
		frostActivationHandshakeQuickCheckTimeout,
	)
	defer cancel()
	return fahe.pointVerifier.VerifyFrostPreSignActivationPoint(
		quickContext,
		point,
	)
}

func (fahe *frostActivationHandshakeExporter) serveHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	responseWriter.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodPost || request.URL.Path != fahe.endpoint.Path {
		http.Error(responseWriter, "not found", http.StatusNotFound)
		return
	}
	remoteHost, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil || net.ParseIP(remoteHost) == nil || !net.ParseIP(remoteHost).IsLoopback() {
		http.Error(responseWriter, "forbidden", http.StatusForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(responseWriter, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	request.Body = http.MaxBytesReader(responseWriter, request.Body, 4096)
	defer request.Body.Close()
	handshakeRequest := &frostActivationHandshakeRequest{}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(handshakeRequest); err != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	attestationContext, cancel := context.WithTimeout(
		request.Context(),
		frostActivationHandshakeRequestTimeout,
	)
	defer cancel()
	handshake, err := fahe.attest(attestationContext, handshakeRequest)
	if err != nil {
		logger.Warnf("refusing FROST activation handshake: [%v]", err)
		if errors.Is(err, errFrostActivationReconciliationPending) {
			responseWriter.Header().Set(
				"Retry-After",
				frostActivationHandshakeRetryAfter,
			)
		}
		http.Error(responseWriter, "activation state is not ready", http.StatusServiceUnavailable)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(responseWriter)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(handshake); err != nil {
		logger.Errorf("cannot encode FROST activation handshake: [%v]", err)
	}
}

func (fahe *frostActivationHandshakeExporter) attest(
	ctx context.Context,
	request *frostActivationHandshakeRequest,
) (*frostActivationSignedHandshake, error) {
	if request == nil || request.Schema != frostActivationHandshakeSchema {
		return nil, fmt.Errorf("unsupported FROST activation handshake schema")
	}
	nonce, err := parseFrostActivationHex32(request.Challenge.Nonce)
	if err != nil || nonce == [32]byte{} {
		return nil, fmt.Errorf("FROST activation challenge nonce is invalid")
	}
	manifestHash, err := parseFrostActivationHex32(request.Challenge.ManifestHash)
	if err != nil || manifestHash != fahe.manifest.ManifestHash {
		return nil, fmt.Errorf("FROST activation challenge manifest hash mismatch")
	}
	bindingHash, err := parseFrostActivationHex32(request.Challenge.BindingHash)
	if err != nil || bindingHash != fahe.bindingHash {
		return nil, fmt.Errorf("FROST activation challenge binding hash mismatch")
	}
	checkpointFloorHash, err := parseFrostActivationHex32(
		request.Challenge.CheckpointFloor.CertificateHash,
	)
	checkpointFloor := FrostRetainedGroupCheckpointCursor{
		Sequence:        request.Challenge.CheckpointFloor.Sequence,
		CertificateHash: checkpointFloorHash,
	}
	if err != nil ||
		request.Challenge.CheckpointFloor.CertificateHash !=
			frostActivationHex32(checkpointFloorHash) ||
		checkpointFloor.Sequence <
			fahe.manifest.QuarantineJournal.CheckpointMinimumSequence ||
		checkpointFloor.Sequence >
			frostRetainedGroupMaximumCanonicalJSONInteger ||
		checkpointFloorHash == [32]byte{} {
		return nil, fmt.Errorf(
			"FROST activation challenge checkpoint floor is invalid",
		)
	}
	blockHash, err := parseFrostActivationHex32(request.Challenge.EthereumPoint.BlockHash)
	if err != nil || request.Challenge.EthereumPoint.BlockNumber == 0 {
		return nil, fmt.Errorf("FROST activation challenge Ethereum point is invalid")
	}
	finality := FrostPreSignFinality{
		BlockNumber: request.Challenge.EthereumPoint.BlockNumber,
		BlockHash:   blockHash,
	}
	reconciliation := fahe.cachedReconciliation(finality)
	if reconciliation == nil {
		fahe.queueReconciliation(finality, false)
		return nil, errFrostActivationReconciliationPending
	}
	liveStamp, stampErr := fahe.tryJournalStamp()
	if errors.Is(stampErr, errFrostActivationJournalBusy) {
		return nil, errFrostActivationReconciliationPending
	}
	if stampErr != nil || liveStamp != reconciliation.stamp {
		fahe.queueReconciliation(finality, true)
		return nil, fmt.Errorf(
			"%w: canonical or quarantine journal generation changed",
			errFrostActivationReconciliationPending,
		)
	}
	if err := fahe.verifyActivationPointQuick(ctx, finality); err != nil {
		fahe.queueReconciliation(finality, true)
		return nil, fmt.Errorf(
			"%w: cannot verify cached FROST activation point: [%v]",
			errFrostActivationReconciliationPending,
			err,
		)
	}
	// The journal snapshot is served from the reconciliation cache, and the
	// stamp compared above pins exactly it: canonical and quarantine
	// generations and roots move the stamp, so a cached journal state that no
	// longer matches the live journal cannot reach the payload. The stamp says
	// nothing about the native signer inventory, which is why that snapshot is
	// re-read live further down instead of being exported from the cache.
	journalSnapshot := &reconciliation.journal
	journalManifest := fahe.manifest.CanonicalJournal
	if !fahe.journal.mutex.TryLock() {
		return nil, fmt.Errorf(
			"%w: %v",
			errFrostActivationReconciliationPending,
			errFrostActivationJournalBusy,
		)
	}
	ancestryStamp, ancestryStampErr := fahe.journalStampLocked()
	if ancestryStampErr != nil || ancestryStamp != reconciliation.stamp {
		fahe.journal.mutex.Unlock()
		fahe.queueReconciliation(finality, true)
		return nil, fmt.Errorf(
			"%w: canonical or quarantine journal generation changed before checkpoint ancestry",
			errFrostActivationReconciliationPending,
		)
	}
	checkpointAncestry, ancestryErr :=
		fahe.journal.checkpointAncestryFrom(checkpointFloor)
	fahe.journal.mutex.Unlock()
	if ancestryErr != nil {
		return nil, fmt.Errorf(
			"checkpoint ancestry rejected the external transparency floor: [%w]",
			ancestryErr,
		)
	}
	wireCheckpointAncestry := make(
		[]frostRetainedGroupWireCheckpointCertificate,
		len(checkpointAncestry),
	)
	for index, certificate := range checkpointAncestry {
		wireCheckpointAncestry[index] =
			frostRetainedGroupCheckpointCertificateToWire(certificate)
	}
	outboxSnapshot, err := fahe.outbox.activationSnapshot()
	if err != nil {
		return nil, err
	}
	if !outboxSnapshot.Recovered || outboxSnapshot.AmbiguousReservationCount != 0 ||
		outboxSnapshot.QuarantineCount != 0 {
		return nil, fmt.Errorf("durable Bitcoin outbox is not activation-ready")
	}
	durableSessionStoreFingerprint, err := fahe.storeBinding.verify()
	if err != nil {
		return nil, fmt.Errorf(
			"FROST durable session store changed during readiness reconciliation: [%w]",
			err,
		)
	}
	// The reconciliation cache is keyed on the finalized Ethereum point and is
	// refreshed only when finality advances, so its native signer inventory can
	// be many minutes old by the time a challenge arrives. This revalidation
	// re-reads live native signer state, and the payload below is built from
	// that live read rather than from the cache, because the two are pinned to
	// each other only in part:
	//
	//   - the strict facts - store identity and fingerprint, retained key
	//     material, trust head, anchor service epoch and certified floor - must
	//     still equal the reconciled values or signing fails closed here. The
	//     AnchorRotationWarning flag may turn on, but not off, as an admitted
	//     input consumes its reserved capacity. An attestation can therefore
	//     never claim a healthier anchor, or different key material, than the
	//     signer actually has;
	//   - the volatile facts that an authorized signing window advances by
	//     design - state generation, the state commitment chain, the state
	//     image digest, the anchor revision and both restartable headrooms -
	//     are only held to a monotone advance. A concurrent authorized batch
	//     can consume thousands of generations inside one finality window, so
	//     exporting the cached values would sign a headroom that is stale by
	//     orders of magnitude. They are exported as read here instead.
	//
	// The residual a consumer of a signed handshake must tolerate is the window
	// between this read and the signature - the checkpoint self-verification,
	// the transcript, one activation-point check and the journal stamp taken
	// under the signing lock - never the age of the cache. A concurrent batch
	// can still advance within that window, so the six volatile values are
	// bounds rather than instants: attested generation and anchor revision are
	// lower bounds on live state, attested headrooms are upper bounds.
	nativeSignerSnapshot, err :=
		fahe.readiness.revalidateFrostProductionSignerReadinessInventory(
			ctx,
			&reconciliation.readiness,
		)
	if err != nil {
		fahe.queueReconciliation(finality, true)
		return nil, fmt.Errorf(
			"%w: cached FROST signer readiness changed before signing: [%v]",
			errFrostActivationReconciliationPending,
			err,
		)
	}
	if !nativeSignerSnapshot.ExternalRollbackAnchorBound ||
		nativeSignerSnapshot.TrustCertificateSequence == 0 ||
		nativeSignerSnapshot.TrustCertificateDigest == [32]byte{} ||
		nativeSignerSnapshot.AnchorServiceEpoch == 0 ||
		nativeSignerSnapshot.CertifiedFloorRevision == 0 ||
		nativeSignerSnapshot.CertifiedFloorGeneration == 0 ||
		nativeSignerSnapshot.CurrentAnchorRevision <
			nativeSignerSnapshot.CertifiedFloorRevision ||
		nativeSignerSnapshot.RestartableRevisionHeadroom == 0 ||
		nativeSignerSnapshot.StateGeneration <
			nativeSignerSnapshot.CertifiedFloorGeneration ||
		nativeSignerSnapshot.RestartableGenerationHeadroom == 0 ||
		nativeSignerSnapshot.CurrentAnchorRevision-
			nativeSignerSnapshot.CertifiedFloorRevision+
			nativeSignerSnapshot.RestartableRevisionHeadroom !=
			FrostNativeSignerAnchorMaximumHistoryEvents ||
		nativeSignerSnapshot.StateGeneration-
			nativeSignerSnapshot.CertifiedFloorGeneration+
			nativeSignerSnapshot.RestartableGenerationHeadroom !=
			FrostNativeSignerAnchorMaximumHistoryProofEntries ||
		nativeSignerSnapshot.AnchorRotationWarning !=
			frostNativeSignerAnchorWorkloadRotationWarning(
				nativeSignerSnapshot.RestartableRevisionHeadroom,
				nativeSignerSnapshot.RestartableGenerationHeadroom,
				nativeSignerSnapshot.LargestLocalSeatCount,
			) {
		return nil, fmt.Errorf(
			"native signer state lacks an authenticated external rollback anchor trust certificate",
		)
	}
	// Every fact revalidated above is read through no-arg native entry points -
	// the state witness tip, the retained key package inventory and the anchor
	// trust head - and none of them takes the state-anchor barrier's
	// request-taking path. A node whose barrier has latched its terminal
	// failure therefore still reports a complete, anchor-bound, rotation-quiet
	// native signer state while it refuses every signing request it is asked
	// for, which is precisely the silent failure this attestation exists to
	// make visible. The barrier verdict is read live here rather than taken
	// from the finality-keyed reconciliation cache, because the cache is
	// refreshed only when finality advances and a poisoning that happened after
	// the last refresh must not be attested away as health.
	stateAnchorPoisoning := frostActivationNativeSignerStateAnchorPoisoned()
	if stateAnchorPoisoning != nil {
		logger.Errorf(
			"FROST activation handshake is attesting an unhealthy node: the "+
				"native tBTC signer state anchor is terminally poisoned: [%v]; "+
				"this node refuses every request-taking native signer call and "+
				"cannot contribute a signature share until it is restarted",
			stateAnchorPoisoning,
		)
	}
	state := frostActivationHandshakeState{
		ProtocolID:                     frostActivationHex32(fahe.manifest.SignerProtocolID),
		ReservationProtocolID:          frostActivationHex32(fahe.manifest.ReservationProtocolID),
		BitcoinOutboxProtocolID:        frostActivationHex32(fahe.manifest.BitcoinOutboxProtocolID),
		SigningPolicyHash:              frostActivationHex32(fahe.manifest.SigningPolicyHash),
		DurableSessionStoreFingerprint: frostActivationHex32(durableSessionStoreFingerprint),
		ShareRepairActivationRegistryRoot: frostActivationHex32(
			currentFrostShareRepairActivationRegistryRoot(),
		),
		CompleteRouterAddress:            frostActivationHex20(fahe.manifest.CompleteRouterAddress),
		AuthorizationRegistryAddress:     frostActivationHex20(fahe.manifest.AuthorizationRegistryAddress),
		Threshold:                        fahe.manifest.Threshold,
		MaximumGroupSize:                 fahe.manifest.MaximumGroupSize,
		RetainedGroupInventoryProtocolID: frostActivationHex32(fahe.manifest.RetainedGroupInventoryProtocolID),
		FrostWalletGroupInventory: frostActivationWalletGroupInventory{
			Schema:                   frostActivationInventorySchema,
			Point:                    request.Challenge.EthereumPoint,
			SnapshotGeneration:       journalSnapshot.SnapshotGeneration,
			InventoryRoot:            frostActivationHex32(journalSnapshot.InventoryRoot),
			WalletCount:              journalSnapshot.WalletCount,
			MinimumActualGroupSize:   journalSnapshot.MinimumActualGroupSize,
			MaximumActualGroupSize:   journalSnapshot.MaximumActualGroupSize,
			MembershipAmbiguityCount: 0,
			GroupSizeViolationCount:  0,
			Complete:                 true,
		},
		CanonicalJournal: frostActivationCanonicalJournalState{
			StoreID:            journalSnapshot.StoreID,
			StoreFingerprint:   frostActivationHex32(journalSnapshot.StoreFingerprint),
			ClusterFingerprint: frostActivationHex32(journalSnapshot.ClusterFingerprint),
			BindingHash:        frostActivationHex32(journalSnapshot.BindingHash),
			Checkpoint: frostActivationEthereumPoint{
				BlockNumber: journalManifest.Checkpoint.BlockNumber,
				BlockHash:   frostActivationHex32(journalManifest.Checkpoint.BlockHash),
			},
			Current:                   request.Challenge.EthereumPoint,
			DescriptorSetHash:         frostActivationHex32(journalManifest.DescriptorSetHash),
			SourceTrustDomainID:       journalManifest.SourceTrustDomainID,
			SourceEndpointFingerprint: frostActivationHex32(journalManifest.SourceEndpointFingerprint),
			SourceOperatorFingerprint: frostActivationHex32(journalManifest.SourceOperatorFingerprint),
			SourceIdentity:            frostRetainedGroupIdentityToWire(journalManifest.SourceIdentity),
			Generation:                journalSnapshot.SnapshotGeneration,
			Complete:                  true,
		},
		QuarantineJournal: frostActivationQuarantineJournalState{
			ProtocolID:             frostActivationHex32(journalSnapshot.QuarantineProtocolID),
			StoreID:                journalSnapshot.QuarantineStoreID,
			StoreFingerprint:       frostActivationHex32(journalSnapshot.QuarantineStoreFingerprint),
			ClusterFingerprint:     frostActivationHex32(journalSnapshot.QuarantineClusterFingerprint),
			Root:                   frostActivationHex32(journalSnapshot.QuarantineRoot),
			ActiveRoot:             frostActivationHex32(journalSnapshot.QuarantineActiveRoot),
			TombstoneRoot:          frostActivationHex32(journalSnapshot.QuarantineTombstoneRoot),
			Generation:             journalSnapshot.QuarantineGeneration,
			CurrentQuarantineCount: journalSnapshot.QuarantineCount,
			TombstoneCount:         journalSnapshot.QuarantineTombstoneCount,
			Complete:               true,
		},
		CheckpointJournal: frostActivationCheckpointJournalState{
			ManifestMinimumSequence: journalSnapshot.CheckpointMinimumSequence,
			ManifestPredecessorHash: frostActivationHex32(
				journalSnapshot.CheckpointPredecessorHash,
			),
			ChallengeFloor: request.Challenge.CheckpointFloor,
			DurableHead: frostRetainedGroupWireCheckpointCursor{
				Sequence: journalSnapshot.CheckpointSequence,
				CertificateHash: frostActivationHex32(
					journalSnapshot.CheckpointCertificateHash,
				),
			},
			Point:                   request.Challenge.EthereumPoint,
			HistoryRoot:             frostActivationHex32(journalSnapshot.CheckpointHistoryRoot),
			CanonicalGeneration:     journalSnapshot.SnapshotGeneration,
			CanonicalInventoryRoot:  frostActivationHex32(journalSnapshot.InventoryRoot),
			QuarantineGeneration:    journalSnapshot.QuarantineGeneration,
			QuarantineEventRoot:     frostActivationHex32(journalSnapshot.QuarantineRoot),
			QuarantineActiveRoot:    frostActivationHex32(journalSnapshot.QuarantineActiveRoot),
			QuarantineTombstoneRoot: frostActivationHex32(journalSnapshot.QuarantineTombstoneRoot),
			Ancestry:                wireCheckpointAncestry,
			Complete:                true,
		},
		NativeSignerState: frostActivationNativeSignerState{
			Schema:                        nativeSignerSnapshot.Schema,
			StoreFingerprint:              frostActivationHex32(nativeSignerSnapshot.StoreFingerprint),
			StateGeneration:               nativeSignerSnapshot.StateGeneration,
			StateCommitment:               frostActivationHex32(nativeSignerSnapshot.StateCommitment),
			PreviousStateCommitment:       frostActivationHex32(nativeSignerSnapshot.PreviousStateCommitment),
			StateImageDigest:              frostActivationHex32(nativeSignerSnapshot.StateImageDigest),
			InventoryCommitment:           frostActivationHex32(nativeSignerSnapshot.InventoryCommitment),
			RetainedWalletCount:           nativeSignerSnapshot.WalletCount,
			RetainedKeyPackageCount:       nativeSignerSnapshot.KeyPackageCount,
			ExternalRollbackAnchorBound:   nativeSignerSnapshot.ExternalRollbackAnchorBound,
			TrustCertificateSequence:      nativeSignerSnapshot.TrustCertificateSequence,
			TrustCertificateDigest:        frostActivationHex32(nativeSignerSnapshot.TrustCertificateDigest),
			AnchorServiceEpoch:            nativeSignerSnapshot.AnchorServiceEpoch,
			CertifiedFloorRevision:        nativeSignerSnapshot.CertifiedFloorRevision,
			CertifiedFloorGeneration:      nativeSignerSnapshot.CertifiedFloorGeneration,
			CurrentAnchorRevision:         nativeSignerSnapshot.CurrentAnchorRevision,
			RestartableRevisionHeadroom:   nativeSignerSnapshot.RestartableRevisionHeadroom,
			RestartableGenerationHeadroom: nativeSignerSnapshot.RestartableGenerationHeadroom,
			AnchorRotationWarning:         nativeSignerSnapshot.AnchorRotationWarning,
			StateAnchorPoisoned:           stateAnchorPoisoning != nil,
			Complete:                      true,
		},
		InteractiveSigningReady:                   reconciliation.interactiveSigningReady,
		FinalizedReservationReadbackEnforced:      true,
		ExactTransactionAuthorizationRootEnforced: true,
		NonceShareGateEnforced: reconciliation.interactiveSigningReady &&
			nativeSignerSnapshot.StateGeneration > 0 &&
			nativeSignerSnapshot.StateCommitment != [32]byte{},
		DurableBitcoinOutboxRecovered: outboxSnapshot.Recovered,
		QuarantineFailClosed:          journalSnapshot.QuarantineCount == 0,
	}
	state.Healthy = frostActivationHandshakeHealthy(state)
	if err := verifyFrostActivationCheckpointHandshakeState(
		fahe.manifest,
		fahe.bindingHash,
		request.Challenge,
		state,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot self-verify FROST activation checkpoint proof: [%w]",
			err,
		)
	}
	payload := frostActivationHandshakePayload{
		Schema:        frostActivationHandshakeSchema,
		Kind:          "frost-signer",
		Nonce:         request.Challenge.Nonce,
		ManifestHash:  request.Challenge.ManifestHash,
		BindingHash:   request.Challenge.BindingHash,
		EthereumPoint: request.Challenge.EthereumPoint,
		State:         state,
	}
	signatureTranscript, err := frostActivationHandshakeSignatureTranscript(
		payload,
	)
	if err != nil {
		return nil, err
	}
	if err := fahe.verifyActivationPointQuick(ctx, finality); err != nil {
		fahe.queueReconciliation(finality, true)
		return nil, fmt.Errorf(
			"%w: cached FROST activation point changed before signing: [%v]",
			errFrostActivationReconciliationPending,
			err,
		)
	}
	var signature []byte
	journalChanged := false
	err = fahe.outbox.withUnchangedActivationSnapshot(
		outboxSnapshot,
		func() error {
			if !fahe.journal.mutex.TryLock() {
				return fmt.Errorf(
					"%w: %v",
					errFrostActivationReconciliationPending,
					errFrostActivationJournalBusy,
				)
			}
			defer fahe.journal.mutex.Unlock()
			// The barrier can latch at any moment, including after the payload
			// above recorded its verdict. Everything the volatile native signer
			// values tolerate in that window is a bound - an attested
			// generation is a lower bound on the live one - but health is a
			// verdict, not a bound: signing "healthy" for a node that has
			// already stopped being able to sign is exactly the failure this
			// attestation must not produce, and the window is not short, since
			// it spans the checkpoint self-verification and one activation
			// point check over the network. Refuse instead, as a retryable
			// pending state. The latch is one-way and sticky until restart, so
			// the retry cannot flap back: it rebuilds the payload with the
			// poisoned verdict and signs that.
			if poisoning := frostActivationNativeSignerStateAnchorPoisoned(); (poisoning != nil) !=
				state.NativeSignerState.StateAnchorPoisoned {
				return fmt.Errorf(
					"%w: native tBTC signer state anchor poisoning changed before signing: [%v]",
					errFrostActivationReconciliationPending,
					poisoning,
				)
			}
			signingStamp, stampErr := fahe.journalStampLocked()
			if stampErr != nil || signingStamp != reconciliation.stamp ||
				!fahe.journal.checkpointDescendsFrom(checkpointFloor) {
				journalChanged = true
				return fmt.Errorf(
					"%w: canonical or quarantine journal generation changed before signing",
					errFrostActivationReconciliationPending,
				)
			}
			signature = ed25519.Sign(fahe.privateKey, signatureTranscript)
			return nil
		},
	)
	if journalChanged {
		fahe.queueReconciliation(finality, true)
	}
	if err != nil {
		return nil, err
	}
	return &frostActivationSignedHandshake{
		Payload:             payload,
		SignerPublicKeySPKI: fahe.publicKeySPKI,
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}, nil
}

// frostActivationHandshakeHealthy derives the attested health verdict from the
// signed payload alone, so that a consumer that keeps the handshake can
// recompute the verdict from the bytes it verified rather than trusting the
// flag. Every term is therefore a field of the payload, including
// StateAnchorPoisoned.
//
// StateAnchorPoisoned is the only term that no other term implies. The
// remaining ones are configuration and no-arg native reads:
// InteractiveSigningReady is opt-in configuration plus a registered engine, and
// the whole native signer snapshot is read through entry points that never
// enter the state-anchor barrier's request-taking path. A node whose barrier is
// poisoned keeps every one of them true while it cannot produce a single
// signature share, so without this term a permissioned set could have several
// members silently unable to sign while all of them attest health.
//
// The term is one-way. The barrier's poisoning is latched until the process
// restarts, so an attestation that reports it unhealthy is never followed by
// one that reports it healthy again short of a restart, and nothing here caches
// the verdict across that transition: it is read live on every attestation.
func frostActivationHandshakeHealthy(
	state frostActivationHandshakeState,
) bool {
	return state.InteractiveSigningReady &&
		state.NonceShareGateEnforced &&
		state.DurableBitcoinOutboxRecovered &&
		state.QuarantineFailClosed &&
		state.NativeSignerState.Complete &&
		state.NativeSignerState.ExternalRollbackAnchorBound &&
		!state.NativeSignerState.AnchorRotationWarning &&
		!state.NativeSignerState.StateAnchorPoisoned
}

func frostActivationHandshakeSignatureTranscript(
	payload frostActivationHandshakePayload,
) ([]byte, error) {
	if payload.Schema != frostActivationHandshakeSchema {
		return nil, fmt.Errorf(
			"unsupported FROST activation handshake payload schema",
		)
	}
	canonicalPayload, err := canonicalFrostActivationValue(payload)
	if err != nil {
		return nil, err
	}
	result := make(
		[]byte,
		0,
		len(frostActivationHandshakeSignatureDomain)+len(canonicalPayload),
	)
	result = append(result, frostActivationHandshakeSignatureDomain...)
	result = append(result, canonicalPayload...)
	return result, nil
}

func verifyFrostActivationCheckpointHandshakeState(
	manifest FrostPreSignActivationRuntimeManifest,
	bindingHash [32]byte,
	challenge frostActivationChallenge,
	state frostActivationHandshakeState,
) error {
	checkpoint := state.CheckpointJournal
	if !checkpoint.Complete ||
		checkpoint.ManifestMinimumSequence !=
			manifest.QuarantineJournal.CheckpointMinimumSequence ||
		checkpoint.ManifestPredecessorHash !=
			frostActivationHex32(
				manifest.QuarantineJournal.CheckpointPredecessorHash,
			) ||
		checkpoint.ChallengeFloor != challenge.CheckpointFloor ||
		checkpoint.Point != challenge.EthereumPoint ||
		checkpoint.Point != state.CanonicalJournal.Current ||
		checkpoint.Point != state.FrostWalletGroupInventory.Point ||
		checkpoint.CanonicalGeneration !=
			state.CanonicalJournal.Generation ||
		checkpoint.CanonicalGeneration !=
			state.FrostWalletGroupInventory.SnapshotGeneration ||
		checkpoint.CanonicalInventoryRoot !=
			state.FrostWalletGroupInventory.InventoryRoot ||
		checkpoint.QuarantineGeneration !=
			state.QuarantineJournal.Generation ||
		checkpoint.QuarantineEventRoot !=
			state.QuarantineJournal.Root ||
		checkpoint.QuarantineActiveRoot !=
			state.QuarantineJournal.ActiveRoot ||
		checkpoint.QuarantineTombstoneRoot !=
			state.QuarantineJournal.TombstoneRoot {
		return fmt.Errorf(
			"FROST activation checkpoint proof differs from the surrounding handshake state",
		)
	}
	parseCursor := func(
		name string,
		wire frostRetainedGroupWireCheckpointCursor,
	) (FrostRetainedGroupCheckpointCursor, error) {
		certificateHash, err := parseFrostActivationHex32(
			wire.CertificateHash,
		)
		if err != nil ||
			wire.CertificateHash !=
				frostActivationHex32(certificateHash) {
			return FrostRetainedGroupCheckpointCursor{}, fmt.Errorf(
				"invalid %s checkpoint cursor",
				name,
			)
		}
		return FrostRetainedGroupCheckpointCursor{
			Sequence:        wire.Sequence,
			CertificateHash: certificateHash,
		}, nil
	}
	floor, err := parseCursor("floor", checkpoint.ChallengeFloor)
	if err != nil {
		return err
	}
	durableHead, err := parseCursor("durable head", checkpoint.DurableHead)
	if err != nil {
		return err
	}
	pointHash, err := parseFrostActivationHex32(checkpoint.Point.BlockHash)
	if err != nil ||
		checkpoint.Point.BlockHash != frostActivationHex32(pointHash) {
		return fmt.Errorf("invalid FROST checkpoint proof point")
	}
	parseRoot := func(name string, value string) ([32]byte, error) {
		root, err := parseFrostActivationHex32(value)
		if err != nil || value != frostActivationHex32(root) {
			return [32]byte{}, fmt.Errorf(
				"invalid FROST checkpoint proof %s",
				name,
			)
		}
		return root, nil
	}
	historyRoot, err := parseRoot("history root", checkpoint.HistoryRoot)
	if err != nil {
		return err
	}
	canonicalInventoryRoot, err := parseRoot(
		"canonical inventory root",
		checkpoint.CanonicalInventoryRoot,
	)
	if err != nil {
		return err
	}
	quarantineEventRoot, err := parseRoot(
		"quarantine event root",
		checkpoint.QuarantineEventRoot,
	)
	if err != nil {
		return err
	}
	quarantineActiveRoot, err := parseRoot(
		"quarantine active root",
		checkpoint.QuarantineActiveRoot,
	)
	if err != nil {
		return err
	}
	quarantineTombstoneRoot, err := parseRoot(
		"quarantine tombstone root",
		checkpoint.QuarantineTombstoneRoot,
	)
	if err != nil {
		return err
	}
	certificates := make(
		[]FrostRetainedGroupCheckpointCertificate,
		len(checkpoint.Ancestry),
	)
	for index, wireCertificate := range checkpoint.Ancestry {
		certificate, err :=
			frostRetainedGroupCheckpointCertificateFromWire(
				wireCertificate,
			)
		if err != nil {
			return fmt.Errorf(
				"invalid FROST checkpoint proof certificate [%d]: [%w]",
				index,
				err,
			)
		}
		certificates[index] = certificate
	}
	return VerifyFrostRetainedGroupCheckpointProof(
		bindingHash,
		manifest,
		floor,
		FrostRetainedGroupCheckpointCommitment{
			DurableHead: durableHead,
			Point: FrostPreSignFinality{
				BlockNumber: checkpoint.Point.BlockNumber,
				BlockHash:   pointHash,
			},
			HistoryRoot:             historyRoot,
			CanonicalGeneration:     checkpoint.CanonicalGeneration,
			CanonicalInventoryRoot:  canonicalInventoryRoot,
			QuarantineGeneration:    checkpoint.QuarantineGeneration,
			QuarantineEventRoot:     quarantineEventRoot,
			QuarantineActiveRoot:    quarantineActiveRoot,
			QuarantineTombstoneRoot: quarantineTombstoneRoot,
		},
		certificates,
	)
}

func decodeStrictFrostActivationJSON(data []byte, target interface{}) error {
	if err := validateUniqueFrostActivationJSONKeys(data); err != nil {
		return err
	}
	var decoded interface{}
	shapeDecoder := json.NewDecoder(bytes.NewReader(data))
	shapeDecoder.UseNumber()
	if err := shapeDecoder.Decode(&decoded); err != nil {
		return err
	}
	if err := validateExactFrostActivationJSONShape(
		decoded,
		reflect.TypeOf(target),
	); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON contains trailing data")
	}
	return nil
}

func validateUniqueFrostActivationJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var readValue func() error
	readValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			return nil
		}
		switch delimiter {
		case '{':
			keys := make(map[string]struct{})
			foldedKeys := make(map[string]string)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("JSON object key is not a string")
				}
				for _, character := range key {
					if character < 0x20 || character > 0x7e {
						return fmt.Errorf(
							"JSON object key [%s] is not printable ASCII",
							key,
						)
					}
				}
				if _, exists := keys[key]; exists {
					return fmt.Errorf("JSON object contains duplicate key [%s]", key)
				}
				folded := strings.ToLower(key)
				if existing, exists := foldedKeys[folded]; exists {
					return fmt.Errorf(
						"JSON object contains case-fold-equivalent keys [%s] and [%s]",
						existing,
						key,
					)
				}
				keys[key] = struct{}{}
				foldedKeys[folded] = key
				if err := readValue(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return fmt.Errorf("JSON object is not closed")
			}
		case '[':
			for decoder.More() {
				if err := readValue(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return fmt.Errorf("JSON array is not closed")
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter [%s]", delimiter)
		}
		return nil
	}
	if err := readValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON contains trailing data")
		}
		return err
	}
	return nil
}

func validateExactFrostActivationJSONShape(
	value interface{},
	targetType reflect.Type,
) error {
	if targetType == nil {
		return fmt.Errorf("JSON target type is nil")
	}
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	rawMessageType := reflect.TypeOf(json.RawMessage{})
	jsonUnmarshalerType := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	if targetType == rawMessageType || targetType.Kind() == reflect.Interface {
		return nil
	}
	if targetType.Implements(jsonUnmarshalerType) ||
		reflect.PointerTo(targetType).Implements(jsonUnmarshalerType) {
		return fmt.Errorf("custom JSON unmarshal targets are not supported")
	}
	if value == nil {
		return nil
	}
	if targetType.Kind() == reflect.Struct {
		object, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		fields := make(map[string]reflect.Type)
		for index := 0; index < targetType.NumField(); index++ {
			field := targetType.Field(index)
			if field.PkgPath != "" {
				continue
			}
			tag := field.Tag.Get("json")
			name := strings.Split(tag, ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			fields[name] = field.Type
		}
		for key, item := range object {
			fieldType, ok := fields[key]
			if !ok {
				return fmt.Errorf("JSON object contains non-exact or unknown key [%s]", key)
			}
			if err := validateExactFrostActivationJSONShape(item, fieldType); err != nil {
				return fmt.Errorf("JSON key [%s]: [%w]", key, err)
			}
		}
		return nil
	}
	if targetType.Kind() == reflect.Slice || targetType.Kind() == reflect.Array {
		if targetType.Kind() == reflect.Slice &&
			targetType.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		items, ok := value.([]interface{})
		if !ok {
			return nil
		}
		for index, item := range items {
			if err := validateExactFrostActivationJSONShape(
				item,
				targetType.Elem(),
			); err != nil {
				return fmt.Errorf("JSON array item [%d]: [%w]", index, err)
			}
		}
		return nil
	}
	if targetType.Kind() == reflect.Map {
		object, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		for key, item := range object {
			if err := validateExactFrostActivationJSONShape(
				item,
				targetType.Elem(),
			); err != nil {
				return fmt.Errorf("JSON map key [%s]: [%w]", key, err)
			}
		}
	}
	return nil
}

func canonicalFrostActivationValue(value interface{}) ([]byte, error) {
	if raw, ok := value.(json.RawMessage); ok {
		return canonicalFrostActivationJSON(raw)
	}
	if raw, ok := value.(*json.RawMessage); ok {
		if raw == nil {
			return nil, fmt.Errorf("canonical JSON raw message is nil")
		}
		return canonicalFrostActivationJSON(*raw)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return canonicalFrostActivationJSON(encoded)
}

func canonicalFrostActivationJSON(encoded []byte) ([]byte, error) {
	if err := validateUniqueFrostActivationJSONKeys(encoded); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded interface{}
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("JSON contains trailing data")
	}
	buffer := bytes.NewBuffer(nil)
	if err := writeCanonicalFrostActivationJSON(buffer, decoded); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeCanonicalFrostActivationJSON(buffer *bytes.Buffer, value interface{}) error {
	switch typed := value.(type) {
	case nil:
		buffer.WriteString("null")
	case bool:
		if typed {
			buffer.WriteString("true")
		} else {
			buffer.WriteString("false")
		}
	case string:
		encoded, _ := json.Marshal(typed)
		buffer.Write(encoded)
	case json.Number:
		raw := typed.String()
		if strings.ContainsAny(raw, ".eE") {
			return fmt.Errorf("canonical JSON number is not an integer")
		}
		integer, ok := new(big.Int).SetString(raw, 10)
		limit := big.NewInt(9007199254740991)
		if !ok || integer.Cmp(new(big.Int).Neg(limit)) < 0 || integer.Cmp(limit) > 0 {
			return fmt.Errorf("canonical JSON number is unsafe")
		}
		buffer.WriteString(integer.String())
	case []interface{}:
		buffer.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				buffer.WriteByte(',')
			}
			if err := writeCanonicalFrostActivationJSON(buffer, item); err != nil {
				return err
			}
		}
		buffer.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buffer.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.WriteByte(',')
			}
			encodedKey, _ := json.Marshal(key)
			buffer.Write(encodedKey)
			buffer.WriteByte(':')
			if err := writeCanonicalFrostActivationJSON(buffer, typed[key]); err != nil {
				return err
			}
		}
		buffer.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON value [%T]", value)
	}
	return nil
}

func parseFrostActivationHex32(value string) ([32]byte, error) {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") ||
		value != strings.ToLower(value) {
		return [32]byte{}, fmt.Errorf("value is not canonical bytes32")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, fmt.Errorf("value is not bytes32")
	}
	result := [32]byte{}
	copy(result[:], decoded)
	return result, nil
}

func frostActivationHex32(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

func frostActivationHex20(value [20]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}
