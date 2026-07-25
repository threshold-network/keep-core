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

	"golang.org/x/sys/unix"
)

const (
	frostActivationHandshakeSchema          = "tbtc-p2tr-production-activation-handshake/v3"
	frostActivationInventorySchema          = "tbtc-p2tr-frost-wallet-group-inventory/v1"
	frostActivationHandshakeSignatureDomain = "tbtc-p2tr-production-activation-handshake-signature/v3\x00"
)

type frostActivationEthereumPoint struct {
	BlockNumber uint64 `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
}

type frostActivationChallenge struct {
	Nonce         string                       `json:"nonce"`
	ManifestHash  string                       `json:"manifestHash"`
	EthereumPoint frostActivationEthereumPoint `json:"ethereumPoint"`
}

type frostActivationHandshakeRequest struct {
	Schema    string                   `json:"schema"`
	Challenge frostActivationChallenge `json:"challenge"`
}

type frostActivationCanonicalJournalState struct {
	StoreID                   string                       `json:"storeID"`
	StoreFingerprint          string                       `json:"storeFingerprint"`
	ClusterFingerprint        string                       `json:"clusterFingerprint"`
	Checkpoint                frostActivationEthereumPoint `json:"checkpoint"`
	Current                   frostActivationEthereumPoint `json:"current"`
	DescriptorSetHash         string                       `json:"descriptorSetHash"`
	SourceTrustDomainID       string                       `json:"sourceTrustDomainID"`
	SourceEndpointFingerprint string                       `json:"sourceEndpointFingerprint"`
	SourceOperatorFingerprint string                       `json:"sourceOperatorFingerprint"`
	Generation                uint64                       `json:"generation"`
	Complete                  bool                         `json:"complete"`
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
	Generation             uint64 `json:"generation"`
	CurrentQuarantineCount uint64 `json:"currentQuarantineCount"`
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
	Complete                      bool   `json:"complete"`
}

type frostActivationHandshakeState struct {
	ProtocolID                                string                                `json:"protocolID"`
	ReservationProtocolID                     string                                `json:"reservationProtocolID"`
	BitcoinOutboxProtocolID                   string                                `json:"bitcoinOutboxProtocolID"`
	SigningPolicyHash                         string                                `json:"signingPolicyHash"`
	DurableSessionStoreFingerprint            string                                `json:"durableSessionStoreFingerprint"`
	CompleteRouterAddress                     string                                `json:"completeRouterAddress"`
	AuthorizationRegistryAddress              string                                `json:"authorizationRegistryAddress"`
	Threshold                                 uint64                                `json:"threshold"`
	MaximumGroupSize                          uint64                                `json:"maximumGroupSize"`
	RetainedGroupInventoryProtocolID          string                                `json:"retainedGroupInventoryProtocolID"`
	FrostWalletGroupInventory                 frostActivationWalletGroupInventory   `json:"frostWalletGroupInventory"`
	CanonicalJournal                          frostActivationCanonicalJournalState  `json:"canonicalJournal"`
	QuarantineJournal                         frostActivationQuarantineJournalState `json:"quarantineJournal"`
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
	pointVerifier FrostPreSignActivationPointVerifier
	storeBinding  *frostDurableSessionStoreBinding
	outbox        *bitcoinBroadcastOutbox
	readiness     frostProductionSignerReadinessVerifier

	mutex    sync.Mutex
	listener net.Listener
	server   *http.Server
	closed   bool
}

func newFrostActivationHandshakeExporter(
	endpoint string,
	privateKeyPath string,
	manifest FrostPreSignActivationRuntimeManifest,
	pointVerifier FrostPreSignActivationPointVerifier,
	storeBinding *frostDurableSessionStoreBinding,
	outbox *bitcoinBroadcastOutbox,
	readiness frostProductionSignerReadinessVerifier,
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
		pointVerifier == nil || storeBinding == nil || outbox == nil || readiness == nil {
		return nil, fmt.Errorf("FROST activation handshake dependencies are invalid")
	}
	boundStoreFingerprint, err := storeBinding.verify()
	if err != nil || boundStoreFingerprint != durableSessionStoreFingerprint {
		return nil, fmt.Errorf(
			"FROST activation handshake durable session store is not bound to the signed manifest",
		)
	}
	privateKey, publicKeyDER, err := loadFrostActivationAttestationKey(privateKeyPath)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(publicKeyDER) != manifest.AttestationSignerKeyHash {
		return nil, fmt.Errorf("FROST activation attestation key differs from signed manifest")
	}
	exporter := &frostActivationHandshakeExporter{
		endpoint:      parsedEndpoint,
		privateKey:    privateKey,
		publicKeySPKI: base64.StdEncoding.EncodeToString(publicKeyDER),
		manifest:      manifest,
		pointVerifier: pointVerifier,
		storeBinding:  storeBinding,
		outbox:        outbox,
		readiness:     readiness,
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
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    4096,
	}
	fahe.listener = listener
	fahe.server = server
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
	if fahe.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return fahe.server.Shutdown(ctx)
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
	handshake, err := fahe.attest(request.Context(), handshakeRequest)
	if err != nil {
		logger.Warnf("refusing FROST activation handshake: [%v]", err)
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
	blockHash, err := parseFrostActivationHex32(request.Challenge.EthereumPoint.BlockHash)
	if err != nil || request.Challenge.EthereumPoint.BlockNumber == 0 {
		return nil, fmt.Errorf("FROST activation challenge Ethereum point is invalid")
	}
	finality := FrostPreSignFinality{
		BlockNumber: request.Challenge.EthereumPoint.BlockNumber,
		BlockHash:   blockHash,
	}
	if err := fahe.pointVerifier.VerifyFrostPreSignActivationPoint(ctx, finality); err != nil {
		return nil, fmt.Errorf("cannot verify FROST activation point: [%w]", err)
	}
	readinessSnapshot, err := fahe.readiness.verifyFrostProductionSignerReadiness(ctx, finality)
	if err != nil {
		return nil, fmt.Errorf("cannot verify production FROST signer readiness: [%w]", err)
	}
	journalSnapshot := readinessSnapshot.Journal
	nativeSignerSnapshot := readinessSnapshot.Inventory
	if journalSnapshot == nil || nativeSignerSnapshot == nil {
		return nil, fmt.Errorf("production FROST signer readiness snapshot is incomplete")
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
			frostNativeSignerAnchorRotationWarning(
				minFrostNativeSignerAnchorHeadroom(
					nativeSignerSnapshot.RestartableRevisionHeadroom,
					nativeSignerSnapshot.RestartableGenerationHeadroom,
				),
			) {
		return nil, fmt.Errorf(
			"native signer state lacks an authenticated external rollback anchor trust certificate",
		)
	}
	journalManifest := fahe.manifest.CanonicalJournal
	quarantineManifest := fahe.manifest.QuarantineJournal
	if !journalSnapshot.Complete || journalSnapshot.CurrentPoint != finality ||
		journalSnapshot.StoreID != journalManifest.StoreID ||
		journalSnapshot.StoreFingerprint != journalManifest.StoreFingerprint ||
		journalSnapshot.ClusterFingerprint != journalManifest.ClusterFingerprint ||
		journalSnapshot.SnapshotGeneration < journalManifest.MinimumGeneration ||
		journalSnapshot.QuarantineProtocolID != quarantineManifest.ProtocolID ||
		journalSnapshot.QuarantineStoreID != quarantineManifest.StoreID ||
		journalSnapshot.QuarantineStoreFingerprint != quarantineManifest.StoreFingerprint ||
		journalSnapshot.QuarantineClusterFingerprint != quarantineManifest.ClusterFingerprint ||
		journalSnapshot.QuarantineGeneration < quarantineManifest.MinimumGeneration ||
		journalSnapshot.QuarantineRoot == [32]byte{} || journalSnapshot.QuarantineCount != 0 {
		return nil, fmt.Errorf("canonical FROST retained-group journal is not activation-ready")
	}
	outboxSnapshot, err := fahe.outbox.activationSnapshot()
	if err != nil {
		return nil, err
	}
	if !outboxSnapshot.Recovered || outboxSnapshot.AmbiguousReservationCount != 0 ||
		outboxSnapshot.QuarantineCount != 0 {
		return nil, fmt.Errorf("durable Bitcoin outbox is not activation-ready")
	}
	if err := fahe.pointVerifier.VerifyFrostPreSignActivationPoint(ctx, finality); err != nil {
		return nil, fmt.Errorf("FROST activation point changed during readiness reconciliation: [%w]", err)
	}
	durableSessionStoreFingerprint, err := fahe.storeBinding.verify()
	if err != nil {
		return nil, fmt.Errorf(
			"FROST durable session store changed during readiness reconciliation: [%w]",
			err,
		)
	}
	state := frostActivationHandshakeState{
		ProtocolID:                       frostActivationHex32(fahe.manifest.SignerProtocolID),
		ReservationProtocolID:            frostActivationHex32(fahe.manifest.ReservationProtocolID),
		BitcoinOutboxProtocolID:          frostActivationHex32(fahe.manifest.BitcoinOutboxProtocolID),
		SigningPolicyHash:                frostActivationHex32(fahe.manifest.SigningPolicyHash),
		DurableSessionStoreFingerprint:   frostActivationHex32(durableSessionStoreFingerprint),
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
			Checkpoint: frostActivationEthereumPoint{
				BlockNumber: journalManifest.Checkpoint.BlockNumber,
				BlockHash:   frostActivationHex32(journalManifest.Checkpoint.BlockHash),
			},
			Current:                   request.Challenge.EthereumPoint,
			DescriptorSetHash:         frostActivationHex32(journalManifest.DescriptorSetHash),
			SourceTrustDomainID:       journalManifest.SourceTrustDomainID,
			SourceEndpointFingerprint: frostActivationHex32(journalManifest.SourceEndpointFingerprint),
			SourceOperatorFingerprint: frostActivationHex32(journalManifest.SourceOperatorFingerprint),
			Generation:                journalSnapshot.SnapshotGeneration,
			Complete:                  true,
		},
		QuarantineJournal: frostActivationQuarantineJournalState{
			ProtocolID:             frostActivationHex32(journalSnapshot.QuarantineProtocolID),
			StoreID:                journalSnapshot.QuarantineStoreID,
			StoreFingerprint:       frostActivationHex32(journalSnapshot.QuarantineStoreFingerprint),
			ClusterFingerprint:     frostActivationHex32(journalSnapshot.QuarantineClusterFingerprint),
			Root:                   frostActivationHex32(journalSnapshot.QuarantineRoot),
			Generation:             journalSnapshot.QuarantineGeneration,
			CurrentQuarantineCount: journalSnapshot.QuarantineCount,
			Complete:               true,
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
			Complete:                      true,
		},
		InteractiveSigningReady:                   readinessSnapshot.InteractiveSigningReady,
		FinalizedReservationReadbackEnforced:      true,
		ExactTransactionAuthorizationRootEnforced: true,
		NonceShareGateEnforced: readinessSnapshot.InteractiveSigningReady &&
			nativeSignerSnapshot.StateGeneration > 0 &&
			nativeSignerSnapshot.StateCommitment != [32]byte{},
		DurableBitcoinOutboxRecovered: outboxSnapshot.Recovered,
		QuarantineFailClosed:          journalSnapshot.QuarantineCount == 0,
	}
	state.Healthy = frostActivationHandshakeHealthy(state)
	payload := frostActivationHandshakePayload{
		Schema:        frostActivationHandshakeSchema,
		Kind:          "frost-signer",
		Nonce:         request.Challenge.Nonce,
		ManifestHash:  request.Challenge.ManifestHash,
		EthereumPoint: request.Challenge.EthereumPoint,
		State:         state,
	}
	signatureTranscript, err := frostActivationHandshakeSignatureTranscript(
		payload,
	)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(fahe.privateKey, signatureTranscript)
	return &frostActivationSignedHandshake{
		Payload:             payload,
		SignerPublicKeySPKI: fahe.publicKeySPKI,
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}, nil
}

func frostActivationHandshakeHealthy(
	state frostActivationHandshakeState,
) bool {
	return state.InteractiveSigningReady &&
		state.NonceShareGateEnforced &&
		state.DurableBitcoinOutboxRecovered &&
		state.QuarantineFailClosed &&
		state.NativeSignerState.Complete &&
		state.NativeSignerState.ExternalRollbackAnchorBound &&
		!state.NativeSignerState.AnchorRotationWarning
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
