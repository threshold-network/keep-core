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
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	frostActivationHandshakeSchema         = "tbtc-p2tr-production-activation-handshake/v1"
	frostActivationReadinessSnapshotSchema = "tbtc-p2tr-frost-signer-readiness-snapshot/v1"
	frostActivationInventorySchema         = "tbtc-p2tr-frost-wallet-group-inventory/v1"
	frostActivationMaximumJSONSize         = 64 * 1024
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

type frostActivationReadinessSnapshot struct {
	Schema                         string                              `json:"schema"`
	ManifestHash                   string                              `json:"manifestHash"`
	DurableSessionStoreFingerprint string                              `json:"durableSessionStoreFingerprint"`
	FrostWalletGroupInventory      frostActivationWalletGroupInventory `json:"frostWalletGroupInventory"`
}

type frostActivationHandshakeState struct {
	ProtocolID                                string                              `json:"protocolID"`
	ReservationProtocolID                     string                              `json:"reservationProtocolID"`
	BitcoinOutboxProtocolID                   string                              `json:"bitcoinOutboxProtocolID"`
	SigningPolicyHash                         string                              `json:"signingPolicyHash"`
	DurableSessionStoreFingerprint            string                              `json:"durableSessionStoreFingerprint"`
	CompleteRouterAddress                     string                              `json:"completeRouterAddress"`
	AuthorizationRegistryAddress              string                              `json:"authorizationRegistryAddress"`
	Threshold                                 uint64                              `json:"threshold"`
	MaximumGroupSize                          uint64                              `json:"maximumGroupSize"`
	RetainedGroupInventoryProtocolID          string                              `json:"retainedGroupInventoryProtocolID"`
	FrostWalletGroupInventory                 frostActivationWalletGroupInventory `json:"frostWalletGroupInventory"`
	FinalizedReservationReadbackEnforced      bool                                `json:"finalizedReservationReadbackEnforced"`
	ExactTransactionAuthorizationRootEnforced bool                                `json:"exactTransactionAuthorizationRootEnforced"`
	NonceShareGateEnforced                    bool                                `json:"nonceShareGateEnforced"`
	DurableBitcoinOutboxRecovered             bool                                `json:"durableBitcoinOutboxRecovered"`
	QuarantineFailClosed                      bool                                `json:"quarantineFailClosed"`
	Healthy                                   bool                                `json:"healthy"`
}

type frostActivationHandshakePayload struct {
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
	readinessPath string
	privateKey    ed25519.PrivateKey
	publicKeySPKI string
	manifest      FrostPreSignActivationRuntimeManifest
	pointVerifier FrostPreSignActivationPointVerifier
	outbox        *bitcoinBroadcastOutbox

	mutex    sync.Mutex
	listener net.Listener
	server   *http.Server
	closed   bool
}

func newFrostActivationHandshakeExporter(
	endpoint string,
	privateKeyPath string,
	readinessPath string,
	manifest FrostPreSignActivationRuntimeManifest,
	pointVerifier FrostPreSignActivationPointVerifier,
	outbox *bitcoinBroadcastOutbox,
) (*frostActivationHandshakeExporter, error) {
	parsedEndpoint, err := validateFrostActivationHandshakeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(readinessPath) == "" {
		return nil, fmt.Errorf("FROST activation readiness snapshot path is empty")
	}
	if manifest.ManifestHash == [32]byte{} ||
		manifest.AttestationSignerKeyHash == [32]byte{} ||
		manifest.Threshold != 51 || manifest.MaximumGroupSize != 100 ||
		pointVerifier == nil || outbox == nil {
		return nil, fmt.Errorf("FROST activation handshake dependencies are invalid")
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
		readinessPath: readinessPath,
		privateKey:    privateKey,
		publicKeySPKI: base64.StdEncoding.EncodeToString(publicKeyDER),
		manifest:      manifest,
		pointVerifier: pointVerifier,
		outbox:        outbox,
	}
	if _, err := exporter.readReadinessSnapshot(); err != nil {
		return nil, err
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

func (fahe *frostActivationHandshakeExporter) readReadinessSnapshot() (
	*frostActivationReadinessSnapshot,
	error,
) {
	data, err := readSecureFrostActivationFile(
		fahe.readinessPath,
		frostActivationMaximumJSONSize,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot read FROST readiness snapshot: [%w]", err)
	}
	snapshot := &frostActivationReadinessSnapshot{}
	if err := decodeStrictFrostActivationJSON(data, snapshot); err != nil {
		return nil, fmt.Errorf("cannot decode FROST readiness snapshot: [%w]", err)
	}
	if snapshot.Schema != frostActivationReadinessSnapshotSchema ||
		snapshot.ManifestHash != frostActivationHex32(fahe.manifest.ManifestHash) ||
		snapshot.DurableSessionStoreFingerprint != fahe.manifest.DurableSessionStoreFingerprint {
		return nil, fmt.Errorf("FROST readiness snapshot differs from signed activation manifest")
	}
	if err := validateFrostActivationInventory(snapshot.FrostWalletGroupInventory); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func validateFrostActivationInventory(
	inventory frostActivationWalletGroupInventory,
) error {
	if inventory.Schema != frostActivationInventorySchema || !inventory.Complete ||
		inventory.MembershipAmbiguityCount != 0 || inventory.GroupSizeViolationCount != 0 ||
		inventory.Point.BlockNumber == 0 {
		return fmt.Errorf("FROST retained-group inventory is incomplete or ambiguous")
	}
	if _, err := parseFrostActivationHex32(inventory.Point.BlockHash); err != nil {
		return fmt.Errorf("invalid FROST inventory block hash: [%w]", err)
	}
	root, err := parseFrostActivationHex32(inventory.InventoryRoot)
	if err != nil || root == [32]byte{} {
		return fmt.Errorf("invalid FROST inventory root")
	}
	if inventory.WalletCount == 0 {
		if inventory.MinimumActualGroupSize != 0 || inventory.MaximumActualGroupSize != 0 {
			return fmt.Errorf("empty FROST inventory has nonzero group bounds")
		}
	} else if inventory.MinimumActualGroupSize < 51 ||
		inventory.MinimumActualGroupSize > inventory.MaximumActualGroupSize ||
		inventory.MaximumActualGroupSize > 100 {
		return fmt.Errorf("FROST inventory group bounds are invalid")
	}
	return nil
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
	snapshot, err := fahe.readReadinessSnapshot()
	if err != nil {
		return nil, err
	}
	if snapshot.FrostWalletGroupInventory.Point != request.Challenge.EthereumPoint {
		return nil, fmt.Errorf("FROST readiness inventory is not at the challenged point")
	}
	finality := FrostPreSignFinality{
		BlockNumber: request.Challenge.EthereumPoint.BlockNumber,
		BlockHash:   blockHash,
	}
	if err := fahe.pointVerifier.VerifyFrostPreSignActivationPoint(ctx, finality); err != nil {
		return nil, fmt.Errorf("cannot verify FROST activation point: [%w]", err)
	}
	outboxSnapshot, err := fahe.outbox.activationSnapshot()
	if err != nil {
		return nil, err
	}
	if !outboxSnapshot.Recovered || outboxSnapshot.AmbiguousReservationCount != 0 ||
		outboxSnapshot.QuarantineCount != 0 {
		return nil, fmt.Errorf("durable Bitcoin outbox is not activation-ready")
	}
	state := frostActivationHandshakeState{
		ProtocolID:                                frostActivationHex32(fahe.manifest.SignerProtocolID),
		ReservationProtocolID:                     frostActivationHex32(fahe.manifest.ReservationProtocolID),
		BitcoinOutboxProtocolID:                   frostActivationHex32(fahe.manifest.BitcoinOutboxProtocolID),
		SigningPolicyHash:                         frostActivationHex32(fahe.manifest.SigningPolicyHash),
		DurableSessionStoreFingerprint:            snapshot.DurableSessionStoreFingerprint,
		CompleteRouterAddress:                     frostActivationHex20(fahe.manifest.CompleteRouterAddress),
		AuthorizationRegistryAddress:              frostActivationHex20(fahe.manifest.AuthorizationRegistryAddress),
		Threshold:                                 fahe.manifest.Threshold,
		MaximumGroupSize:                          fahe.manifest.MaximumGroupSize,
		RetainedGroupInventoryProtocolID:          frostActivationHex32(fahe.manifest.RetainedGroupInventoryProtocolID),
		FrostWalletGroupInventory:                 snapshot.FrostWalletGroupInventory,
		FinalizedReservationReadbackEnforced:      true,
		ExactTransactionAuthorizationRootEnforced: true,
		NonceShareGateEnforced:                    true,
		DurableBitcoinOutboxRecovered:             true,
		QuarantineFailClosed:                      true,
		Healthy:                                   true,
	}
	payload := frostActivationHandshakePayload{
		Kind:          "frost-signer",
		Nonce:         request.Challenge.Nonce,
		ManifestHash:  request.Challenge.ManifestHash,
		EthereumPoint: request.Challenge.EthereumPoint,
		State:         state,
	}
	canonicalPayload, err := canonicalFrostActivationValue(payload)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(fahe.privateKey, canonicalPayload)
	return &frostActivationSignedHandshake{
		Payload:             payload,
		SignerPublicKeySPKI: fahe.publicKeySPKI,
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}, nil
}

func decodeStrictFrostActivationJSON(data []byte, target interface{}) error {
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

func canonicalFrostActivationValue(value interface{}) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded interface{}
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
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
