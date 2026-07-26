package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/keep-network/keep-core/pkg/chain"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	frostregistry "github.com/keep-network/keep-core/pkg/frost/registry"
)

type frostRetainedGroupHistoryTestVerifier struct {
	mutex          sync.Mutex
	chainID        *big.Int
	finalized      *types.Header
	headers        map[uint64]*types.Header
	reads          map[uint64]int
	reorgBlock     uint64
	reorgAfterRead int
	reorgHeader    *types.Header
	receipts       map[common.Hash]*types.Receipt
	code           map[common.Address][]byte
	storage        map[common.Address]map[common.Hash][]byte
	sortitionPool  common.Address
	operator       common.Address
	operatorAt     uint64
	operatorID     uint32
	closed         bool
}

type frostRetainedGroupCanonicalRPCTestAPI struct {
	points []rpc.BlockNumberOrHash
}

func (api *frostRetainedGroupCanonicalRPCTestAPI) GetCode(
	_ context.Context,
	_ common.Address,
	point rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	api.points = append(api.points, point)
	return hexutil.Bytes{0x01}, nil
}

func (api *frostRetainedGroupCanonicalRPCTestAPI) GetStorageAt(
	_ context.Context,
	_ common.Address,
	_ common.Hash,
	point rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	api.points = append(api.points, point)
	return make(hexutil.Bytes, 32), nil
}

func (api *frostRetainedGroupCanonicalRPCTestAPI) Call(
	_ context.Context,
	_ map[string]interface{},
	point rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	api.points = append(api.points, point)
	return make(hexutil.Bytes, 32), nil
}

func TestCanonicalFrostRetainedGroupEthereumVerifier_RequiresCanonicalHashState(
	t *testing.T,
) {
	server := rpc.NewServer()
	api := &frostRetainedGroupCanonicalRPCTestAPI{}
	if err := server.RegisterName("eth", api); err != nil {
		t.Fatal(err)
	}
	client := rpc.DialInProc(server)
	defer client.Close()
	verifier := &canonicalFrostRetainedGroupEthereumVerifier{
		rpcClient: client,
	}
	blockHash := common.HexToHash("0x1234")
	address := common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	)
	if _, err := verifier.CodeAtHash(
		context.Background(),
		address,
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.StorageAtHash(
		context.Background(),
		address,
		common.Hash{},
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.CallContractAtHash(
		context.Background(),
		ethereum.CallMsg{To: &address, Data: []byte{0x01}},
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if len(api.points) != 3 {
		t.Fatalf("unexpected exact-hash call count [%d]", len(api.points))
	}
	for _, point := range api.points {
		if point.BlockHash == nil || *point.BlockHash != blockHash ||
			!point.RequireCanonical || point.BlockNumber != nil {
			t.Fatalf("state read did not require canonical hash [%+v]", point)
		}
	}
}

func (verifier *frostRetainedGroupHistoryTestVerifier) ChainID(
	context.Context,
) (*big.Int, error) {
	return new(big.Int).Set(verifier.chainID), nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) HeaderByNumber(
	_ context.Context,
	number *big.Int,
) (*types.Header, error) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if number.Sign() < 0 {
		return verifier.finalized, nil
	}
	blockNumber := number.Uint64()
	verifier.reads[blockNumber]++
	if blockNumber == verifier.reorgBlock && verifier.reorgHeader != nil &&
		verifier.reads[blockNumber] > verifier.reorgAfterRead {
		return verifier.reorgHeader, nil
	}
	return verifier.headers[blockNumber], nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) HeaderByHash(
	_ context.Context,
	hash common.Hash,
) (*types.Header, error) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	for blockNumber, header := range verifier.headers {
		if header.Hash() != hash {
			continue
		}
		verifier.reads[blockNumber]++
		if blockNumber == verifier.reorgBlock && verifier.reorgHeader != nil &&
			verifier.reads[blockNumber] > verifier.reorgAfterRead {
			if verifier.reorgHeader.Hash() == hash {
				return verifier.reorgHeader, nil
			}
			return nil, nil
		}
		return header, nil
	}
	return nil, nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) Close() {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	verifier.closed = true
}

func (verifier *frostRetainedGroupHistoryTestVerifier) TransactionReceipt(
	_ context.Context,
	hash common.Hash,
) (*types.Receipt, error) {
	receipt := verifier.receipts[hash]
	if receipt == nil {
		return nil, fmt.Errorf("missing receipt")
	}
	return receipt, nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) CodeAtHash(
	_ context.Context,
	address common.Address,
	hash common.Hash,
) ([]byte, error) {
	if _, err := verifier.HeaderByHash(context.Background(), hash); err != nil {
		return nil, err
	}
	code := verifier.code[address]
	if len(code) == 0 {
		return nil, fmt.Errorf("missing code")
	}
	return append([]byte{}, code...), nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) StorageAtHash(
	_ context.Context,
	address common.Address,
	slot common.Hash,
	blockHash common.Hash,
) ([]byte, error) {
	if _, err := verifier.HeaderByHash(context.Background(), blockHash); err != nil {
		return nil, err
	}
	if slots := verifier.storage[address]; slots != nil {
		if value := slots[slot]; value != nil {
			return append([]byte{}, value...), nil
		}
	}
	return make([]byte, 32), nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) CallContractAtHash(
	_ context.Context,
	call ethereum.CallMsg,
	blockHash common.Hash,
) ([]byte, error) {
	if call.To == nil || *call.To != verifier.sortitionPool ||
		blockHash != verifier.headers[verifier.operatorAt].Hash() || len(call.Data) != 36 ||
		!bytes.Equal(call.Data[:4], []byte{0x5a, 0x48, 0xb4, 0x6b}) ||
		!bytes.Equal(call.Data[4:16], make([]byte, 12)) ||
		!bytes.Equal(call.Data[16:], verifier.operator[:]) {
		return nil, fmt.Errorf("unexpected operator call")
	}
	result := make([]byte, 32)
	binary.BigEndian.PutUint32(result[28:], verifier.operatorID)
	return result, nil
}

type frostRetainedGroupHistoryTestExport struct {
	t                             *testing.T
	privateKey                    ed25519.PrivateKey
	publicKeyDER                  []byte
	transportPrivateKey           ed25519.PrivateKey
	transportPublicKeyDER         []byte
	backendPrivateKey             ed25519.PrivateKey
	backendPublicKeyDER           []byte
	operatorPrivateKey            ed25519.PrivateKey
	operatorPublicKeyDER          []byte
	historyResponder              func(frostRetainedGroupHistoryPageRequest) interface{}
	operatorResponder             func(frostRetainedGroupOperatorQuery) interface{}
	corruptSignature              bool
	bindingHash                   [32]byte
	identity                      FrostRetainedGroupEndpointIdentity
	now                           func() time.Time
	transportAttestationMutator   func(*frostRetainedGroupTransportAttestation)
	omitTransportAttestation      bool
	duplicateTransportAttestation bool
	replayTransportAttestation    bool
	transportAttestationMutex     sync.Mutex
	savedTransportAttestation     string
}

func (export *frostRetainedGroupHistoryTestExport) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(responseWriter, "method", http.StatusMethodNotAllowed)
		return
	}
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(responseWriter, "request", http.StatusBadRequest)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(requestBody))
	var payload interface{}
	switch request.URL.Path {
	case "/history":
		historyRequest := frostRetainedGroupHistoryPageRequest{}
		if err := json.NewDecoder(bytes.NewReader(requestBody)).Decode(&historyRequest); err != nil {
			http.Error(responseWriter, "request", http.StatusBadRequest)
			return
		}
		payload = export.historyResponder(historyRequest)
	case "/operator-id":
		operatorRequest := frostRetainedGroupOperatorQuery{}
		if err := json.NewDecoder(bytes.NewReader(requestBody)).Decode(&operatorRequest); err != nil {
			http.Error(responseWriter, "request", http.StatusBadRequest)
			return
		}
		payload = export.operatorResponder(operatorRequest)
	default:
		http.NotFound(responseWriter, request)
		return
	}
	if payload == nil {
		request.Body = io.NopCloser(bytes.NewReader(requestBody))
		export.writeAttestedResponse(
			responseWriter,
			request,
			http.StatusServiceUnavailable,
			"text/plain",
			[]byte("missing\n"),
		)
		return
	}
	canonical, err := canonicalFrostActivationValue(payload)
	if err != nil {
		export.t.Fatal(err)
	}
	payloadHash := sha256.Sum256(canonical)
	signed := append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...)
	signature := ed25519.Sign(export.privateKey, signed)
	if export.corruptSignature {
		signature[0] ^= 0xff
	}
	envelope := frostRetainedGroupSignedEnvelope{
		Schema:              "tbtc-frost-retained-group-signed-envelope/v3",
		BindingHash:         frostActivationHex32(export.bindingHash),
		Payload:             json.RawMessage(canonical),
		PayloadSHA256:       frostActivationHex32(payloadHash),
		SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(export.publicKeyDER),
		SignatureAlgorithm:  "ed25519",
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}
	responseBody, err := json.Marshal(envelope)
	if err != nil {
		export.t.Fatal(err)
	}
	request.Body = io.NopCloser(bytes.NewReader(requestBody))
	export.writeAttestedResponse(
		responseWriter,
		request,
		http.StatusOK,
		"application/json",
		responseBody,
	)
}

func (export *frostRetainedGroupHistoryTestExport) writeAttestedResponse(
	responseWriter http.ResponseWriter,
	request *http.Request,
	status int,
	contentType string,
	responseBody []byte,
) {
	export.t.Helper()
	localAddress, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		export.t.Fatal("missing test server local address")
	}
	localIP, err := frostRetainedGroupRemoteIP(localAddress)
	if err != nil {
		export.t.Fatal(err)
	}
	now := time.Now()
	if export.now != nil {
		now = export.now()
	}
	attestation, err := marshalFrostRetainedGroupTransportAttestation(
		request,
		status,
		responseBody,
		export.identity,
		export.transportPrivateKey,
		export.transportPublicKeyDER,
		export.backendPrivateKey,
		export.backendPublicKeyDER,
		export.operatorPrivateKey,
		export.operatorPublicKeyDER,
		now,
		localIP,
	)
	if err != nil {
		export.t.Fatal(err)
	}
	if export.transportAttestationMutator != nil {
		raw, err := base64.StdEncoding.Strict().DecodeString(attestation)
		if err != nil {
			export.t.Fatal(err)
		}
		mutated := frostRetainedGroupTransportAttestation{}
		if err := decodeStrictFrostActivationJSON(raw, &mutated); err != nil {
			export.t.Fatal(err)
		}
		export.transportAttestationMutator(&mutated)
		digest, err := frostRetainedGroupAttestationTranscript(mutated)
		if err != nil {
			export.t.Fatal(err)
		}
		backendDigest := frostRetainedGroupBackendAttestationDigest(digest)
		mutated.BackendSignature = base64.StdEncoding.EncodeToString(
			ed25519.Sign(export.backendPrivateKey, backendDigest[:]),
		)
		operatorDigest := frostRetainedGroupOperatorAttestationDigest(digest)
		mutated.OperatorSignature = base64.StdEncoding.EncodeToString(
			ed25519.Sign(export.operatorPrivateKey, operatorDigest[:]),
		)
		mutated.Signature = base64.StdEncoding.EncodeToString(
			ed25519.Sign(export.transportPrivateKey, digest[:]),
		)
		encoded, err := json.Marshal(mutated)
		if err != nil {
			export.t.Fatal(err)
		}
		attestation = base64.StdEncoding.EncodeToString(encoded)
	}
	if export.replayTransportAttestation {
		export.transportAttestationMutex.Lock()
		if export.savedTransportAttestation == "" {
			export.savedTransportAttestation = attestation
		} else {
			attestation = export.savedTransportAttestation
		}
		export.transportAttestationMutex.Unlock()
	}
	responseWriter.Header().Set("Content-Type", contentType)
	if !export.omitTransportAttestation {
		responseWriter.Header().Add(
			frostRetainedGroupTransportAttestationHeader,
			attestation,
		)
		if export.duplicateTransportAttestation {
			responseWriter.Header().Add(
				frostRetainedGroupTransportAttestationHeader,
				attestation,
			)
		}
	}
	responseWriter.WriteHeader(status)
	if _, err := responseWriter.Write(responseBody); err != nil {
		export.t.Fatal(err)
	}
}

type frostRetainedGroupHistorySourceFixture struct {
	t                 *testing.T
	verifier          *frostRetainedGroupHistoryTestVerifier
	export            *frostRetainedGroupHistoryTestExport
	server            *httptest.Server
	source            *signedFrostRetainedGroupHistorySource
	identity          FrostRetainedGroupHistoryIdentity
	profile           FrostPreSignActivationProfile
	runtimeManifest   FrostPreSignActivationRuntimeManifest
	from              FrostPreSignFinality
	to                FrostPreSignFinality
	descriptorSetHash [32]byte
	snapshotID        [32]byte
	mutations         []FrostRetainedGroupMutation
	dkgFullMembers    []uint32
	dkgMisbehaved     []uint8
	checkpointIssuer  func(
		FrostRetainedGroupCheckpointCursor,
		FrostPreSignFinality,
		[]FrostRetainedGroupMutation,
	) ([]FrostRetainedGroupCheckpointCertificate, error)
	pageMutator     func(*frostRetainedGroupHistoryPagePayload)
	operatorMutator func(*frostRetainedGroupOperatorReceiptPayload)
}

func (fixture *frostRetainedGroupHistorySourceFixture) checkpointAfter() FrostRetainedGroupCheckpointCursor {
	return FrostRetainedGroupCheckpointCursor{
		Sequence: fixture.runtimeManifest.QuarantineJournal.
			CheckpointMinimumSequence - 1,
		CertificateHash: fixture.runtimeManifest.QuarantineJournal.
			CheckpointPredecessorHash,
	}
}

func newFrostRetainedGroupHistoryTLSTestServer(
	t *testing.T,
	handler http.Handler,
	serviceIdentity string,
) (*httptest.Server, *x509.Certificate, *x509.CertPool) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serviceURI, err := url.Parse(serviceIdentity)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "retained-history-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		URIs:                  []*url.URL{serviceURI},
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		NextProtos: []string{"http/1.1"},
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certificateDER},
			PrivateKey:  privateKey,
			Leaf:        leaf,
		}},
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	return server, leaf, roots
}

func newFrostRetainedGroupHistorySourceFixture(
	t *testing.T,
) *frostRetainedGroupHistorySourceFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	transportPublicKey, transportPrivateKey, err :=
		ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	transportPublicKeyDER, err := x509.MarshalPKIXPublicKey(
		transportPublicKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	backendPublicKey, backendPrivateKey, err :=
		ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	backendPublicKeyDER, err := x509.MarshalPKIXPublicKey(backendPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	operatorPublicKey, operatorPrivateKey, err :=
		ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	operatorPublicKeyDER, err := x509.MarshalPKIXPublicKey(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	headers := make(map[uint64]*types.Header)
	for blockNumber := uint64(1); blockNumber <= 10; blockNumber++ {
		headers[blockNumber] = &types.Header{
			Number: new(big.Int).SetUint64(blockNumber),
			Time:   blockNumber,
			Extra:  []byte{byte(blockNumber), 0x9a},
		}
	}
	bridgeCode := []byte{0x60, 0x01, 0x60, 0x02}
	registryCode := []byte{0x60, 0x03, 0x60, 0x04}
	sortitionPoolCode := []byte{0x60, 0x05, 0x60, 0x06}
	profile, runtimeManifest := frostRetainedGroupHistoryTestProfile(
		t,
		bridgeCode,
		registryCode,
		sortitionPoolCode,
		headers[1].Hash(),
	)
	checkpointPrivateKeys := make([]ed25519.PrivateKey, 3)
	checkpointPublicKeySPKIs := make([]string, 3)
	checkpointAuthorities := make([]FrostRetainedGroupAuthority, 3)
	for index := range checkpointAuthorities {
		authority, privateKey, publicKeySPKI := journalTestAuthority(
			t,
			fmt.Sprintf("checkpoint-%d", index+1),
			byte(0x51+index),
		)
		checkpointAuthorities[index] = authority
		checkpointPrivateKeys[index] = privateKey
		checkpointPublicKeySPKIs[index] = publicKeySPKI
	}
	runtimeManifest.QuarantineJournal.CheckpointAuthorities =
		checkpointAuthorities
	operatorAddress := common.HexToAddress("0x1111111111111111111111111111111111111111")
	verifier := &frostRetainedGroupHistoryTestVerifier{
		chainID:       big.NewInt(1),
		finalized:     headers[10],
		headers:       headers,
		reads:         make(map[uint64]int),
		receipts:      make(map[common.Hash]*types.Receipt),
		code:          make(map[common.Address][]byte),
		storage:       make(map[common.Address]map[common.Hash][]byte),
		sortitionPool: common.Address(profile.SortitionPool),
		operator:      operatorAddress,
		operatorAt:    6,
		operatorID:    17,
	}
	verifier.code[common.Address(profile.BridgeAddress)] = bridgeCode
	verifier.code[common.Address(profile.FrostRegistry)] = registryCode
	verifier.code[common.Address(profile.SortitionPool)] = sortitionPoolCode
	export := &frostRetainedGroupHistoryTestExport{
		t:                     t,
		privateKey:            privateKey,
		publicKeyDER:          publicKeyDER,
		transportPrivateKey:   transportPrivateKey,
		transportPublicKeyDER: transportPublicKeyDER,
		backendPrivateKey:     backendPrivateKey,
		backendPublicKeyDER:   backendPublicKeyDER,
		operatorPrivateKey:    operatorPrivateKey,
		operatorPublicKeyDER:  operatorPublicKeyDER,
	}
	exportServiceIdentity := "spiffe://export.retained.test/export"
	server, leaf, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		export,
		exportServiceIdentity,
	)
	endpoint, canonicalEndpoint, err := validateFrostRetainedGroupTLSEndpoint(
		server.URL + "/",
	)
	if err != nil {
		t.Fatal(err)
	}
	resolvedEndpoint, err := resolveFrostRetainedGroupEndpoint(
		context.Background(),
		endpoint,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	exportIdentity := FrostRetainedGroupEndpointIdentity{
		Schema:                    frostRetainedGroupEndpointIdentitySchema,
		Role:                      "retained-history-export",
		TrustDomainID:             "export.retained.test",
		CanonicalEndpoint:         canonicalEndpoint,
		CanonicalDNSName:          resolvedEndpoint.canonicalDNSName,
		ResolvedDNSName:           resolvedEndpoint.resolvedDNSName,
		ResolvedAddressSetHash:    resolvedEndpoint.addressSetHash,
		TLSLeafSPKIHash:           sha256.Sum256(leaf.RawSubjectPublicKeyInfo),
		ServiceIdentity:           exportServiceIdentity,
		BackendServiceFingerprint: sha256.Sum256(backendPublicKeyDER),
		OperatorFingerprint:       sha256.Sum256(operatorPublicKeyDER),
		AttestationKeyHash:        sha256.Sum256(transportPublicKeyDER),
		TLSExporterProtocolID:     frostRetainedGroupTLSExporterProtocolID(),
	}
	exportIdentity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(exportIdentity)
	verifierAddress := netip.MustParseAddr("127.0.0.2")
	verifierIdentity := FrostRetainedGroupEndpointIdentity{
		Schema:                    frostRetainedGroupEndpointIdentitySchema,
		Role:                      "retained-history-verifier",
		TrustDomainID:             "verifier.retained.test",
		CanonicalEndpoint:         "https://127.0.0.2:9443/rpc",
		CanonicalDNSName:          "127.0.0.2",
		ResolvedDNSName:           "127.0.0.2",
		ResolvedAddressSetHash:    frostRetainedGroupResolvedAddressSetHash([]netip.Addr{verifierAddress}),
		TLSLeafSPKIHash:           [32]byte{0x63},
		ServiceIdentity:           "spiffe://verifier.retained.test/verifier",
		BackendServiceFingerprint: [32]byte{0x64},
		OperatorFingerprint:       [32]byte{0x65},
		AttestationKeyHash:        [32]byte{0x66},
		TLSExporterProtocolID:     frostRetainedGroupTLSExporterProtocolID(),
	}
	verifierIdentity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(verifierIdentity)
	identity := FrostRetainedGroupHistoryIdentity{
		Schema:               frostRetainedGroupSourceIdentitySchema,
		TrustDomainID:        "independent-retained-history-test",
		OperatorFingerprint:  exportIdentity.OperatorFingerprint,
		HistorySignerKeyHash: sha256.Sum256(publicKeyDER),
		Export:               exportIdentity,
		Verifier:             verifierIdentity,
	}
	identity.EndpointFingerprint =
		computeFrostRetainedGroupSourceEndpointFingerprint(identity)
	export.identity = exportIdentity
	runtimeManifest.CanonicalJournal.SourceTrustDomainID =
		identity.TrustDomainID
	runtimeManifest.CanonicalJournal.SourceEndpointFingerprint =
		identity.EndpointFingerprint
	runtimeManifest.CanonicalJournal.SourceOperatorFingerprint =
		identity.OperatorFingerprint
	runtimeManifest.CanonicalJournal.SourceIdentity = identity
	exportHTTPClient, exportTransport, err :=
		newFrostRetainedGroupAttestedHTTPClient(
			resolvedEndpoint,
			exportIdentity,
			roots,
			frostRetainedGroupDefaultTimeout,
		)
	if err != nil {
		t.Fatal(err)
	}
	verifierURL, _, err := validateFrostRetainedGroupTLSEndpoint(
		verifierIdentity.CanonicalEndpoint,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolvedVerifier, err := resolveFrostRetainedGroupEndpoint(
		context.Background(),
		verifierURL,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	primaryURL, _, err := validateFrostRetainedGroupTLSEndpoint(
		"https://127.0.0.3:9444/rpc",
	)
	if err != nil {
		t.Fatal(err)
	}
	resolvedPrimary, err := resolveFrostRetainedGroupEndpoint(
		context.Background(),
		primaryURL,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := newSignedFrostRetainedGroupHistorySource(
		context.Background(),
		endpoint,
		verifier,
		exportHTTPClient,
		1,
		identity,
		identity.HistorySignerKeyHash,
		frostRetainedGroupDefaultTimeout,
		&frostRetainedGroupIndependenceMonitor{
			exportEndpoint:   resolvedEndpoint,
			verifierEndpoint: resolvedVerifier,
			primaryEndpoint:  resolvedPrimary,
			timeout:          frostRetainedGroupDefaultTimeout,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	source.httpTransports = []*http.Transport{exportTransport}
	descriptorSetHash := [32]byte{0x42}
	if err := source.BindFrostRetainedGroupActivationEvidence(
		profile,
		runtimeManifest,
	); err != nil {
		t.Fatal(err)
	}
	export.bindingHash = source.evidence.bindingHash
	t.Cleanup(source.Close)
	fixture := &frostRetainedGroupHistorySourceFixture{
		t:                 t,
		verifier:          verifier,
		export:            export,
		server:            server,
		source:            source,
		identity:          identity,
		profile:           profile,
		runtimeManifest:   runtimeManifest,
		from:              FrostPreSignFinality{BlockNumber: 1, BlockHash: headers[1].Hash()},
		to:                FrostPreSignFinality{BlockNumber: 6, BlockHash: headers[6].Hash()},
		descriptorSetHash: descriptorSetHash,
		snapshotID:        [32]byte{0x53},
	}
	fixture.mutations = frostRetainedGroupHistoryTestMutations(t, headers, source.evidence)
	fixture.dkgFullMembers = append(
		[]uint32{},
		fixture.mutations[0].OperatorIDs...,
	)
	fixture.installReceipts()
	fixture.checkpointIssuer = newFrostRetainedGroupTestCheckpointIssuer(
		t,
		source.evidence.checkpointPolicy,
		fixture.from,
		checkpointPrivateKeys,
		checkpointPublicKeySPKIs,
	)
	export.historyResponder = fixture.historyResponse
	export.operatorResponder = fixture.operatorResponse
	return fixture
}

func frostRetainedGroupHistoryTestProfile(
	t *testing.T,
	bridgeCode []byte,
	registryCode []byte,
	sortitionPoolCode []byte,
	deploymentBlockHash common.Hash,
) (FrostPreSignActivationProfile, FrostPreSignActivationRuntimeManifest) {
	t.Helper()
	emptyLinkedLibraryDescriptorHash, err :=
		frostRetainedGroupLinkedLibraryInventoryHash(
			[]FrostPreSignLinkedLibraryEvidence{},
		)
	if err != nil {
		t.Fatal(err)
	}
	profile := FrostPreSignActivationProfile{
		DomainChainID:             [32]byte{31: 0x01},
		ActivationManifestHash:    [32]byte{0x02},
		BridgeAddress:             [20]byte{0x11},
		RegistryAddress:           [20]byte{0x12},
		CompleteRouter:            [20]byte{0x13},
		FrostRegistry:             [20]byte{0x14},
		ProposalValidator:         [20]byte{0x15},
		SortitionPool:             [20]byte{0x16},
		BridgeCodeHash:            [32]byte(crypto.Keccak256Hash(bridgeCode)),
		RegistryCodeHash:          [32]byte{0x22},
		CompleteRouterCodeHash:    [32]byte{0x23},
		FrostRegistryCodeHash:     [32]byte(crypto.Keccak256Hash(registryCode)),
		ProposalValidatorCodeHash: [32]byte{0x25},
		SortitionPoolCodeHash:     [32]byte(crypto.Keccak256Hash(sortitionPoolCode)),
		ReservationProtocolID:     frostPreSignReservationProtocolID(),
		EvidenceProtocolID:        frostCompleteEvidenceProtocolID(),
		SigningPolicyHash:         frostPreSignSigningPolicyHash(),
	}
	inputs := []struct {
		role     string
		name     string
		address  [20]byte
		codeHash [32]byte
	}{
		{"bridge", "Bridge", profile.BridgeAddress, profile.BridgeCodeHash},
		{"completeRouter", "COMPLETE router", profile.CompleteRouter, profile.CompleteRouterCodeHash},
		{"authorizationRegistry", "authorization registry", profile.RegistryAddress, profile.RegistryCodeHash},
		{"frostWalletRegistry", "FROST wallet registry", profile.FrostRegistry, profile.FrostRegistryCodeHash},
		{"frostProposalValidator", "proposal validator", profile.ProposalValidator, profile.ProposalValidatorCodeHash},
		{"frostSortitionPool", "sortition pool", profile.SortitionPool, profile.SortitionPoolCodeHash},
		{"ecdsaFraudRouter", "ECDSA fraud router", [20]byte{0x17}, [32]byte{0x27}},
		{"ecdsaCutoverCoordinator", "ECDSA cutover coordinator", [20]byte{0x18}, [32]byte{0x28}},
	}
	deployments := make([]FrostPreSignDeploymentEvidence, 0, len(inputs))
	for _, input := range inputs {
		descriptor := FrostPreSignDeploymentDescriptorEvidence{
			Address:                     input.address,
			RuntimeCodeHash:             input.codeHash,
			Upgradeability:              "immutable",
			LinkedLibraryDescriptorHash: emptyLinkedLibraryDescriptorHash,
		}
		descriptor.DescriptorHash = descriptor.ComputeHash()
		deployments = append(deployments, FrostPreSignDeploymentEvidence{
			Role:                    input.role,
			Name:                    input.name,
			DeploymentBlock:         1,
			RelevantEventStartBlock: 1,
			Current:                 descriptor,
			HistoricalEpochs: []FrostPreSignDeploymentEpochEvidence{{
				Start: FrostPreSignFinality{
					BlockNumber: 1,
					BlockHash:   [32]byte(deploymentBlockHash),
				},
				Descriptor: descriptor,
			}},
		})
	}
	profile.ImplementationSetHash =
		ComputeFrostPreSignDeploymentEvidenceHash(deployments)
	profile.ProfileHash = profile.ComputeHash()
	linkedLibraryDescriptorSetHash, err :=
		frostRetainedGroupLinkedLibraryDescriptorSetHash(deployments)
	if err != nil {
		t.Fatal(err)
	}
	checkpointAuthorities := []FrostRetainedGroupAuthority{
		{AuthorityID: "checkpoint-1", PublicKeySPKIHash: [32]byte{0x51}},
		{AuthorityID: "checkpoint-2", PublicKeySPKIHash: [32]byte{0x52}},
		{AuthorityID: "checkpoint-3", PublicKeySPKIHash: [32]byte{0x53}},
	}
	liftAuthorities := []FrostRetainedGroupAuthority{
		{AuthorityID: "lift-1", PublicKeySPKIHash: [32]byte{0x54}},
		{AuthorityID: "lift-2", PublicKeySPKIHash: [32]byte{0x55}},
		{AuthorityID: "lift-3", PublicKeySPKIHash: [32]byte{0x56}},
	}
	return profile, FrostPreSignActivationRuntimeManifest{
		ManifestHash:                     profile.ActivationManifestHash,
		ActivationAuthorityKeyHash:       [32]byte{0x35},
		VerifierOperatorFingerprint:      [32]byte{0x36},
		HandshakeOperatorFingerprint:     [32]byte{0x38},
		DomainChainID:                    profile.DomainChainID,
		GenesisBlockHash:                 [32]byte{0x39},
		ProfileHash:                      profile.ProfileHash,
		ImplementationSetHash:            profile.ImplementationSetHash,
		LinkedLibraryDescriptorSetHash:   linkedLibraryDescriptorSetHash,
		EndpointIdentitySetHash:          [32]byte{0x3a},
		Deployments:                      deployments,
		SignerProtocolID:                 [32]byte{0x45},
		ReservationProtocolID:            profile.ReservationProtocolID,
		BitcoinOutboxProtocolID:          [32]byte{0x46},
		SigningPolicyHash:                profile.SigningPolicyHash,
		AttestationSignerKeyHash:         [32]byte{0x37},
		RetainedGroupInventoryProtocolID: [32]byte{0x43},
		CanonicalJournal: FrostRetainedGroupCanonicalJournalManifest{
			StoreID:                   "canonical-test-store",
			StoreFingerprint:          [32]byte{0x47},
			ClusterFingerprint:        [32]byte{0x48},
			Checkpoint:                FrostPreSignFinality{BlockNumber: 1, BlockHash: [32]byte(deploymentBlockHash)},
			DescriptorSetHash:         [32]byte{0x42},
			SourceTrustDomainID:       "independent-retained-history-test",
			SourceEndpointFingerprint: [32]byte{0x31},
		},
		QuarantineJournal: FrostRetainedGroupQuarantineJournalManifest{
			ProtocolID:                   [32]byte{0x44},
			LiftProtocolID:               [32]byte{0x4b},
			TombstoneProtocolID:          [32]byte{0x4c},
			CheckpointAuthorityThreshold: 2,
			CheckpointAuthorities:        checkpointAuthorities,
			CheckpointMinimumSequence:    1,
			CheckpointPredecessorHash:    [32]byte{},
			LiftAuthorityThreshold:       2,
			LiftAuthorities:              liftAuthorities,
			StoreID:                      "quarantine-test-store",
			StoreFingerprint:             [32]byte{0x49},
			ClusterFingerprint:           [32]byte{0x4a},
		},
	}
}

func frostRetainedGroupHistoryTestMutations(
	t *testing.T,
	headers map[uint64]*types.Header,
	evidence *frostRetainedGroupEvidenceProfile,
) []FrostRetainedGroupMutation {
	t.Helper()
	walletID := [32]byte{0x71}
	walletPublicKeyHash := [20]byte{0x72}
	operatorIDs := make([]uint32, 51)
	for index := range operatorIDs {
		operatorIDs[index] = uint32(index + 1)
	}
	dkgSubmission := FrostRetainedGroupEventPoint{
		BlockNumber:      2,
		BlockHash:        headers[2].Hash(),
		TransactionHash:  [32]byte{0xa2},
		TransactionIndex: 0,
		LogIndex:         2,
	}
	admissionTransaction := [32]byte{0xa3}
	dkgApproval := FrostRetainedGroupEventPoint{
		BlockNumber:      3,
		BlockHash:        headers[3].Hash(),
		TransactionHash:  admissionTransaction,
		TransactionIndex: 3,
		LogIndex:         10,
	}
	creation := FrostRetainedGroupEventPoint{
		BlockNumber:      3,
		BlockHash:        headers[3].Hash(),
		TransactionHash:  admissionTransaction,
		TransactionIndex: 3,
		LogIndex:         11,
	}
	registration := creation
	registration.LogIndex = 12
	closing := FrostRetainedGroupEventPoint{
		BlockNumber:      4,
		BlockHash:        headers[4].Hash(),
		TransactionHash:  [32]byte{0xa4},
		TransactionIndex: 1,
		LogIndex:         20,
	}
	closureTransaction := [32]byte{0xa5}
	closed := FrostRetainedGroupEventPoint{
		BlockNumber:      5,
		BlockHash:        headers[5].Hash(),
		TransactionHash:  closureTransaction,
		TransactionIndex: 2,
		LogIndex:         30,
	}
	registryClosure := closed
	registryClosure.LogIndex = 31
	dkgResult, _, dkgResultHash := frostRetainedGroupHistoryTestDkgResult(
		t,
		evidence,
		walletID,
		operatorIDs,
	)
	return []FrostRetainedGroupMutation{
		{
			Point:                   registration,
			Kind:                    FrostRetainedGroupAdmissionMutation,
			WalletID:                walletID,
			WalletPublicKeyHash:     walletPublicKeyHash,
			OperatorIDs:             operatorIDs,
			RetainedGroupHash:       dkgResult.MembersHash,
			DkgResultHash:           dkgResultHash,
			DkgSubmissionPoint:      dkgSubmission,
			DkgApprovalPoint:        dkgApproval,
			CreationPoint:           creation,
			BridgeRegistrationPoint: registration,
		},
		{
			Point:               closing,
			Kind:                FrostRetainedGroupClosingMutation,
			WalletID:            walletID,
			WalletPublicKeyHash: walletPublicKeyHash,
		},
		{
			Point:               closed,
			Kind:                FrostRetainedGroupClosedMutation,
			WalletID:            walletID,
			WalletPublicKeyHash: walletPublicKeyHash,
		},
		{
			Point:               registryClosure,
			Kind:                FrostRetainedGroupRegistryClosureMutation,
			WalletID:            walletID,
			WalletPublicKeyHash: walletPublicKeyHash,
		},
	}
}

func frostRetainedGroupHistoryTestDkgResult(
	t *testing.T,
	evidence *frostRetainedGroupEvidenceProfile,
	walletID [32]byte,
	operatorIDs []uint32,
) (frostabi.FrostDkgResult, []byte, [32]byte) {
	return frostRetainedGroupHistoryTestDkgResultWithMisbehaved(
		t,
		evidence,
		walletID,
		operatorIDs,
		nil,
	)
}

func frostRetainedGroupHistoryTestDkgResultWithMisbehaved(
	t *testing.T,
	evidence *frostRetainedGroupEvidenceProfile,
	walletID [32]byte,
	fullMembers []uint32,
	misbehaved []uint8,
) (frostabi.FrostDkgResult, []byte, [32]byte) {
	t.Helper()
	activeMembers, err := frostregistry.ActiveMembersFromMisbehaved(
		frostregistry.FullMembers(fullMembers),
		frostregistry.MisbehavedMemberIndices(misbehaved),
	)
	if err != nil {
		t.Fatal(err)
	}
	activeMembersHash, err := frostregistry.ActiveMembersHash(activeMembers)
	if err != nil {
		t.Fatal(err)
	}
	result := frostabi.FrostDkgResult{
		SubmitterMemberIndex:     big.NewInt(1),
		XOnlyOutputKey:           walletID,
		MisbehavedMembersIndices: append([]uint8{}, misbehaved...),
		Signatures:               []byte{0x01, 0x02},
		SigningMembersIndices:    []*big.Int{big.NewInt(1)},
		Members:                  append([]uint32{}, fullMembers...),
		MembersHash:              activeMembersHash,
	}
	data, err := evidence.registryABI.Events["DkgResultSubmitted"].Inputs.NonIndexed().Pack(result)
	if err != nil {
		t.Fatal(err)
	}
	return result, data, [32]byte(crypto.Keccak256Hash(data))
}

func (fixture *frostRetainedGroupHistorySourceFixture) installReceipts() {
	fixture.t.Helper()
	evidence, err := fixture.source.activationEvidence()
	if err != nil {
		fixture.t.Fatal(err)
	}
	registryAddress := common.Address(
		evidence.deployments["frostWalletRegistry"].Current.Address,
	)
	bridgeAddress := common.Address(
		evidence.deployments["bridge"].Current.Address,
	)
	admission := fixture.mutations[0]
	dkgResult, dkgData, dkgResultHash := frostRetainedGroupHistoryTestDkgResultWithMisbehaved(
		fixture.t,
		evidence,
		admission.WalletID,
		fixture.dkgFullMembers,
		fixture.dkgMisbehaved,
	)
	admission.OperatorIDs = append(
		[]uint32{},
		mustFrostRetainedGroupActiveMembers(
			fixture.t,
			fixture.dkgFullMembers,
			fixture.dkgMisbehaved,
		)...,
	)
	admission.RetainedGroupHash = dkgResult.MembersHash
	admission.DkgResultHash = dkgResultHash
	fixture.mutations[0] = admission
	submissionLog := frostRetainedGroupHistoryTestLog(
		admission.DkgSubmissionPoint,
		registryAddress,
		[]common.Hash{
			evidence.registryABI.Events["DkgResultSubmitted"].ID,
			common.Hash(admission.DkgResultHash),
			common.BigToHash(big.NewInt(123)),
		},
		dkgData,
	)
	fixture.verifier.receipts[common.Hash(admission.DkgSubmissionPoint.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(admission.DkgSubmissionPoint, submissionLog)

	approvalLog := frostRetainedGroupHistoryTestLog(
		admission.DkgApprovalPoint,
		registryAddress,
		[]common.Hash{
			evidence.registryABI.Events["DkgResultApproved"].ID,
			common.Hash(admission.DkgResultHash),
			common.BytesToHash(common.HexToAddress("0x2222222222222222222222222222222222222222").Bytes()),
		},
		nil,
	)
	creationLog := frostRetainedGroupHistoryTestLog(
		admission.CreationPoint,
		registryAddress,
		[]common.Hash{
			evidence.registryABI.Events["WalletCreated"].ID,
			common.Hash(admission.WalletID),
			common.Hash(admission.DkgResultHash),
		},
		nil,
	)
	registrationLog := frostRetainedGroupHistoryTestLog(
		admission.BridgeRegistrationPoint,
		bridgeAddress,
		[]common.Hash{
			evidence.bridgeABI.Events["NewWalletRegisteredV2"].ID,
			common.Hash(admission.WalletID),
			{},
			frostRetainedGroupBytes20Topic(admission.WalletPublicKeyHash),
		},
		nil,
	)
	fixture.verifier.receipts[common.Hash(admission.DkgApprovalPoint.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(
			admission.DkgApprovalPoint,
			approvalLog,
			creationLog,
			registrationLog,
		)

	closing := fixture.mutations[1]
	closingLog := frostRetainedGroupHistoryTestLog(
		closing.Point,
		bridgeAddress,
		[]common.Hash{
			evidence.bridgeABI.Events["WalletClosing"].ID,
			{},
			frostRetainedGroupBytes20Topic(closing.WalletPublicKeyHash),
		},
		nil,
	)
	fixture.verifier.receipts[common.Hash(closing.Point.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(closing.Point, closingLog)

	closed := fixture.mutations[2]
	registryClosure := fixture.mutations[3]
	closedLog := frostRetainedGroupHistoryTestLog(
		closed.Point,
		bridgeAddress,
		[]common.Hash{
			evidence.bridgeABI.Events["WalletClosed"].ID,
			{},
			frostRetainedGroupBytes20Topic(closed.WalletPublicKeyHash),
		},
		nil,
	)
	registryClosureLog := frostRetainedGroupHistoryTestLog(
		registryClosure.Point,
		registryAddress,
		[]common.Hash{
			evidence.registryABI.Events["WalletClosed"].ID,
			common.Hash(registryClosure.WalletID),
		},
		nil,
	)
	fixture.verifier.receipts[common.Hash(closed.Point.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(closed.Point, closedLog, registryClosureLog)
}

func mustFrostRetainedGroupActiveMembers(
	t *testing.T,
	fullMembers []uint32,
	misbehaved []uint8,
) []uint32 {
	t.Helper()
	activeMembers, err := frostregistry.ActiveMembersFromMisbehaved(
		frostregistry.FullMembers(fullMembers),
		frostregistry.MisbehavedMemberIndices(misbehaved),
	)
	if err != nil {
		t.Fatal(err)
	}
	return append([]uint32{}, activeMembers...)
}

func frostRetainedGroupHistoryTestLog(
	point FrostRetainedGroupEventPoint,
	address common.Address,
	topics []common.Hash,
	data []byte,
) *types.Log {
	return &types.Log{
		Address:     address,
		Topics:      append([]common.Hash{}, topics...),
		Data:        append([]byte{}, data...),
		BlockNumber: point.BlockNumber,
		TxHash:      common.Hash(point.TransactionHash),
		TxIndex:     uint(point.TransactionIndex),
		BlockHash:   common.Hash(point.BlockHash),
		Index:       uint(point.LogIndex),
	}
}

func frostRetainedGroupHistoryTestReceipt(
	point FrostRetainedGroupEventPoint,
	logs ...*types.Log,
) *types.Receipt {
	return &types.Receipt{
		Status:           types.ReceiptStatusSuccessful,
		TxHash:           common.Hash(point.TransactionHash),
		BlockHash:        common.Hash(point.BlockHash),
		BlockNumber:      new(big.Int).SetUint64(point.BlockNumber),
		TransactionIndex: uint(point.TransactionIndex),
		Logs:             logs,
	}
}

func (fixture *frostRetainedGroupHistorySourceFixture) historyPages(
	query frostRetainedGroupHistoryQuery,
	checkpointAfterWire frostRetainedGroupWireCheckpointCursor,
) []*frostRetainedGroupHistoryPagePayload {
	fixture.t.Helper()
	queryHash, err := frostRetainedGroupDomainHash(frostRetainedGroupHistoryQueryDomain, query)
	if err != nil {
		fixture.t.Fatal(err)
	}
	wireMutations := make([]frostRetainedGroupWireMutation, len(fixture.mutations))
	for index, mutation := range fixture.mutations {
		wireMutations[index] = frostRetainedGroupMutationToWire(mutation)
	}
	checkpointAfterHash, err := parseFrostActivationHex32(
		checkpointAfterWire.CertificateHash,
	)
	if err != nil {
		fixture.t.Fatal(err)
	}
	checkpointAfter := FrostRetainedGroupCheckpointCursor{
		Sequence:        checkpointAfterWire.Sequence,
		CertificateHash: checkpointAfterHash,
	}
	checkpointTarget, err := frostRetainedGroupFinalityFromWire(query.To)
	if err != nil {
		fixture.t.Fatal(err)
	}
	checkpoints, err := fixture.checkpointIssuer(
		checkpointAfter,
		checkpointTarget,
		fixture.mutations,
	)
	if err != nil {
		fixture.t.Fatal(err)
	}
	checkpointComplete := true
	if len(checkpoints) > frostRetainedGroupMaximumCheckpointsPerPage {
		checkpoints = checkpoints[:frostRetainedGroupMaximumCheckpointsPerPage]
		checkpointComplete = false
	}
	checkpointHashes := make([][32]byte, len(checkpoints))
	wireCheckpoints := make(
		[]frostRetainedGroupWireCheckpointCertificate,
		len(checkpoints),
	)
	for index, checkpoint := range checkpoints {
		checkpointHashes[index], err =
			frostRetainedGroupCheckpointCertificateHash(checkpoint)
		if err != nil {
			fixture.t.Fatal(err)
		}
		wireCheckpoints[index] =
			frostRetainedGroupCheckpointCertificateToWire(checkpoint)
	}
	checkpointTipHash := checkpointAfter.CertificateHash
	if len(checkpointHashes) > 0 {
		checkpointTipHash = checkpointHashes[len(checkpointHashes)-1]
	}
	pageCount := (len(wireMutations) + 1) / 2
	if pageCount == 0 {
		pageCount = 1
	}
	pages := make([]*frostRetainedGroupHistoryPagePayload, pageCount)
	previousPageHash := [32]byte{}
	identity := frostRetainedGroupIdentityToWire(fixture.identity)
	for index := 0; index < pageCount; index++ {
		start := index * 2
		end := start + 2
		if end > len(wireMutations) {
			end = len(wireMutations)
		}
		cursor := ""
		if index > 0 {
			cursor = "page_" + string(rune('0'+index))
		}
		nextCursor := ""
		if index+1 < pageCount {
			nextCursor = "page_" + string(rune('0'+index+1))
		}
		page := &frostRetainedGroupHistoryPagePayload{
			Schema:            frostRetainedGroupHistoryPageSchema,
			BindingHash:       frostActivationHex32(fixture.source.evidence.bindingHash),
			Identity:          identity,
			ChainID:           1,
			QueryHash:         frostActivationHex32(queryHash),
			SnapshotID:        frostActivationHex32(fixture.snapshotID),
			PageIndex:         uint64(index),
			Cursor:            cursor,
			PreviousPageHash:  frostActivationHex32(previousPageHash),
			From:              query.From,
			To:                query.To,
			EmptyAtFrom:       true,
			DescriptorSetHash: frostActivationHex32(fixture.descriptorSetHash),
			CheckpointAfter:   checkpointAfterWire,
			Mutations:         append([]frostRetainedGroupWireMutation{}, wireMutations[start:end]...),
			NextCursor:        nextCursor,
			Complete:          index+1 == pageCount,
		}
		pages[index] = page
		canonical, err := canonicalFrostActivationValue(page)
		if err != nil {
			fixture.t.Fatal(err)
		}
		previousPageHash = sha256.Sum256(canonical)
	}
	historyRoot, err := frostRetainedGroupHistoryRoot(
		fixture.source.evidence.bindingHash,
		queryHash,
		wireMutations,
	)
	if err != nil {
		fixture.t.Fatal(err)
	}
	pages[len(pages)-1].Receipt = &frostRetainedGroupHistoryReceipt{
		PageCount:              uint64(len(pages)),
		MutationCount:          uint64(len(wireMutations)),
		BindingHash:            frostActivationHex32(fixture.source.evidence.bindingHash),
		HistoryRoot:            frostActivationHex32(historyRoot),
		CheckpointAfter:        checkpointAfterWire,
		CheckpointCertificates: wireCheckpoints,
		CheckpointChainRoot: frostActivationHex32(
			frostRetainedGroupCheckpointChainRoot(
				fixture.source.evidence.bindingHash,
				checkpointAfter,
				checkpointHashes,
			),
		),
		CheckpointTipHash:  frostActivationHex32(checkpointTipHash),
		CheckpointComplete: checkpointComplete,
	}
	return pages
}

func (fixture *frostRetainedGroupHistorySourceFixture) historyResponse(
	request frostRetainedGroupHistoryPageRequest,
) interface{} {
	if request.BindingHash !=
		frostActivationHex32(fixture.source.evidence.bindingHash) {
		return nil
	}
	pages := fixture.historyPages(request.Query, request.CheckpointAfter)
	for _, page := range pages {
		if page.Cursor == request.Cursor {
			copy := *page
			copy.Mutations = append([]frostRetainedGroupWireMutation{}, page.Mutations...)
			if page.Receipt != nil {
				receipt := *page.Receipt
				copy.Receipt = &receipt
			}
			if fixture.pageMutator != nil {
				fixture.pageMutator(&copy)
			}
			return &copy
		}
	}
	return nil
}

func (fixture *frostRetainedGroupHistorySourceFixture) operatorResponse(
	query frostRetainedGroupOperatorQuery,
) interface{} {
	queryHash, err := frostRetainedGroupDomainHash(frostRetainedGroupOperatorQueryDomain, query)
	if err != nil {
		fixture.t.Fatal(err)
	}
	payload := &frostRetainedGroupOperatorReceiptPayload{
		Schema: frostRetainedGroupOperatorReceiptSchema,
		BindingHash: frostActivationHex32(
			fixture.source.evidence.bindingHash,
		),
		Identity:        frostRetainedGroupIdentityToWire(fixture.identity),
		ChainID:         1,
		QueryHash:       frostActivationHex32(queryHash),
		OperatorAddress: query.OperatorAddress,
		At:              query.At,
		OperatorID:      17,
		Found:           true,
	}
	if fixture.operatorMutator != nil {
		fixture.operatorMutator(payload)
	}
	return payload
}

func TestSignedFrostRetainedGroupHistorySource_ReadsCompletePaginatedHistory(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	identity, err := fixture.source.Identity(context.Background())
	if err != nil || identity != fixture.identity {
		t.Fatalf("unexpected identity: [%v] [%v]", identity, err)
	}
	history, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !history.Complete || !history.EmptyAtFrom || len(history.Mutations) != 4 ||
		history.DescriptorSetHash != fixture.descriptorSetHash {
		t.Fatalf("unexpected complete history: [%+v]", history)
	}
	operatorID, err := fixture.source.ResolveOperatorID(
		context.Background(),
		chain.Address("0x1111111111111111111111111111111111111111"),
		fixture.to,
	)
	if err != nil || operatorID != 17 {
		t.Fatalf("unexpected operator ID: [%d] [%v]", operatorID, err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_PaginatesLongCheckpointChain(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	checkpointCount := frostRetainedGroupMaximumCheckpointsPerPage + 2
	targetBlock := uint64(checkpointCount + 2)
	finalizedBlock := targetBlock + 10
	for blockNumber := uint64(11); blockNumber <= finalizedBlock; blockNumber++ {
		fixture.verifier.headers[blockNumber] = &types.Header{
			Number: new(big.Int).SetUint64(blockNumber),
			Time:   blockNumber,
			Extra:  []byte{byte(blockNumber), 0x9a},
		}
	}
	fixture.to = FrostPreSignFinality{
		BlockNumber: targetBlock,
		BlockHash:   fixture.verifier.headers[targetBlock].Hash(),
	}
	fixture.verifier.finalized = fixture.verifier.headers[finalizedBlock]

	cursor := fixture.checkpointAfter()
	for blockNumber := uint64(3); blockNumber <= targetBlock; blockNumber++ {
		point := FrostPreSignFinality{
			BlockNumber: blockNumber,
			BlockHash:   fixture.verifier.headers[blockNumber].Hash(),
		}
		prefix := make([]FrostRetainedGroupMutation, 0, len(fixture.mutations))
		for _, mutation := range fixture.mutations {
			if mutation.Point.BlockNumber <= blockNumber {
				prefix = append(prefix, mutation)
			}
		}
		certificates, err := fixture.checkpointIssuer(
			cursor,
			point,
			prefix,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(certificates) != 1 {
			t.Fatalf(
				"expected one newly issued checkpoint, got [%d]",
				len(certificates),
			)
		}
		certificateHash, err :=
			frostRetainedGroupCheckpointCertificateHash(certificates[0])
		if err != nil {
			t.Fatal(err)
		}
		cursor = FrostRetainedGroupCheckpointCursor{
			Sequence:        certificates[0].Body.Sequence,
			CertificateHash: certificateHash,
		}
	}

	first, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.CheckpointComplete ||
		len(first.Checkpoints) !=
			frostRetainedGroupMaximumCheckpointsPerPage {
		t.Fatalf(
			"unexpected first checkpoint page: complete [%t], count [%d]",
			first.CheckpointComplete,
			len(first.Checkpoints),
		)
	}
	firstTail := FrostRetainedGroupCheckpointCursor{
		Sequence:        first.Checkpoints[len(first.Checkpoints)-1].Body.Sequence,
		CertificateHash: first.CheckpointTipHash,
	}
	second, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		firstTail,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !second.CheckpointComplete ||
		len(second.Checkpoints) != 2 ||
		second.CheckpointTipHash != cursor.CertificateHash ||
		second.HistoryRoot != first.HistoryRoot {
		t.Fatalf(
			"unexpected final checkpoint page: complete [%t], count [%d]",
			second.CheckpointComplete,
			len(second.Checkpoints),
		)
	}
}

func TestSignedFrostRetainedGroupHistorySource_ProtocolBindingCommitsRuntime(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	baseline := fixture.source.evidence.bindingHash
	testCases := map[string]func(
		*FrostPreSignActivationProfile,
		*FrostPreSignActivationRuntimeManifest,
	){
		"domain chain": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.DomainChainID[31] ^= 0xff
		},
		"genesis": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.GenesisBlockHash[0] ^= 0xff
		},
		"checkpoint": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.CanonicalJournal.Checkpoint.BlockHash[0] ^= 0xff
		},
		"manifest": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.ManifestHash[0] ^= 0xff
		},
		"profile": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.ProfileHash[0] ^= 0xff
		},
		"implementation": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.ImplementationSetHash[0] ^= 0xff
		},
		"descriptor": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.CanonicalJournal.DescriptorSetHash[0] ^= 0xff
		},
		"linked libraries": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.LinkedLibraryDescriptorSetHash[0] ^= 0xff
		},
		"endpoint identities": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.EndpointIdentitySetHash[0] ^= 0xff
		},
		"protocol": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.SignerProtocolID[0] ^= 0xff
		},
		"evidence protocol": func(
			profile *FrostPreSignActivationProfile,
			_ *FrostPreSignActivationRuntimeManifest,
		) {
			profile.EvidenceProtocolID[0] ^= 0xff
		},
		"canonical store": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.CanonicalJournal.StoreID += "-other"
		},
		"canonical cluster": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.CanonicalJournal.ClusterFingerprint[0] ^= 0xff
		},
		"quarantine store": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.StoreFingerprint[0] ^= 0xff
		},
		"quarantine protocol": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.ProtocolID[0] ^= 0xff
		},
		"quarantine lift protocol": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.LiftProtocolID[0] ^= 0xff
		},
		"quarantine tombstone protocol": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.TombstoneProtocolID[0] ^= 0xff
		},
		"checkpoint authority set": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.CheckpointAuthorities[0].
				PublicKeySPKIHash[0] ^= 0xff
		},
		"checkpoint minimum sequence": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.CheckpointMinimumSequence++
			runtime.QuarantineJournal.CheckpointPredecessorHash =
				[32]byte{0x7f}
		},
		"checkpoint predecessor": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.CheckpointMinimumSequence = 2
			runtime.QuarantineJournal.CheckpointPredecessorHash =
				[32]byte{0x7e}
		},
		"lift authority set": func(
			_ *FrostPreSignActivationProfile,
			runtime *FrostPreSignActivationRuntimeManifest,
		) {
			runtime.QuarantineJournal.LiftAuthorities[0].
				PublicKeySPKIHash[0] ^= 0xff
		},
	}
	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			profile := fixture.profile
			runtime := fixture.runtimeManifest
			runtime.QuarantineJournal.CheckpointAuthorities = append(
				[]FrostRetainedGroupAuthority{},
				fixture.runtimeManifest.QuarantineJournal.
					CheckpointAuthorities...,
			)
			runtime.QuarantineJournal.LiftAuthorities = append(
				[]FrostRetainedGroupAuthority{},
				fixture.runtimeManifest.QuarantineJournal.LiftAuthorities...,
			)
			mutate(&profile, &runtime)
			binding, err := fixture.source.computeProtocolBinding(profile, runtime)
			if err != nil {
				t.Fatal(err)
			}
			if binding == baseline {
				t.Fatalf("%s was omitted from the protocol binding", name)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsInconsistentRuntimeEvidence(
	t *testing.T,
) {
	testCases := map[string]struct {
		mutate   func(*FrostPreSignActivationProfile, *FrostPreSignActivationRuntimeManifest)
		expected string
	}{
		"profile role": {
			mutate: func(
				profile *FrostPreSignActivationProfile,
				runtime *FrostPreSignActivationRuntimeManifest,
			) {
				for index := range runtime.Deployments {
					deployment := &runtime.Deployments[index]
					if deployment.Role != "bridge" {
						continue
					}
					deployment.Current.Address[0] ^= 0xff
					deployment.Current.DescriptorHash =
						deployment.Current.ComputeHash()
					last := len(deployment.HistoricalEpochs) - 1
					deployment.HistoricalEpochs[last].Descriptor =
						cloneFrostRetainedGroupDeploymentDescriptor(
							deployment.Current,
						)
				}
				profile.ImplementationSetHash =
					ComputeFrostPreSignDeploymentEvidenceHash(
						runtime.Deployments,
					)
				profile.ProfileHash = profile.ComputeHash()
				runtime.ImplementationSetHash =
					profile.ImplementationSetHash
				runtime.ProfileHash = profile.ProfileHash
			},
			expected: "differs from the activation profile",
		},
		"recursive descriptor": {
			mutate: func(
				profile *FrostPreSignActivationProfile,
				runtime *FrostPreSignActivationRuntimeManifest,
			) {
				for index := range runtime.Deployments {
					deployment := &runtime.Deployments[index]
					if deployment.Role != "bridge" {
						continue
					}
					deployment.Current.LinkedLibraryDescriptorHash[0] ^= 0xff
					deployment.Current.DescriptorHash =
						deployment.Current.ComputeHash()
					last := len(deployment.HistoricalEpochs) - 1
					deployment.HistoricalEpochs[last].Descriptor =
						cloneFrostRetainedGroupDeploymentDescriptor(
							deployment.Current,
						)
				}
				profile.ImplementationSetHash =
					ComputeFrostPreSignDeploymentEvidenceHash(
						runtime.Deployments,
					)
				profile.ProfileHash = profile.ComputeHash()
				runtime.ImplementationSetHash =
					profile.ImplementationSetHash
				runtime.ProfileHash = profile.ProfileHash
			},
			expected: "linked-library descriptor hash mismatch",
		},
		"global descriptor set": {
			mutate: func(
				_ *FrostPreSignActivationProfile,
				runtime *FrostPreSignActivationRuntimeManifest,
			) {
				runtime.LinkedLibraryDescriptorSetHash[0] ^= 0xff
			},
			expected: "descriptor set differs",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			profile := fixture.profile
			runtime := fixture.runtimeManifest
			runtime.Deployments = make(
				[]FrostPreSignDeploymentEvidence,
				len(fixture.runtimeManifest.Deployments),
			)
			for index, deployment := range fixture.runtimeManifest.Deployments {
				runtime.Deployments[index] =
					cloneFrostRetainedGroupDeploymentEvidence(deployment)
			}
			testCase.mutate(&profile, &runtime)
			unbound := &signedFrostRetainedGroupHistorySource{
				chainID:  fixture.source.chainID,
				identity: fixture.source.identity,
			}
			err := unbound.BindFrostRetainedGroupActivationEvidence(
				profile,
				runtime,
			)
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf(
					"expected %s inconsistency rejection, got [%v]",
					name,
					err,
				)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_DecodesCanonicalVerifiedBytes(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	raw := json.RawMessage("{\n  \"z\": 1,\n  \"a\": 2\n}")
	canonical, err := canonicalFrostActivationValue(raw)
	if err != nil {
		t.Fatal(err)
	}
	payloadHash := sha256.Sum256(canonical)
	signature := ed25519.Sign(
		fixture.export.privateKey,
		append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...),
	)
	envelope := &frostRetainedGroupSignedEnvelope{
		Schema:              "tbtc-frost-retained-group-signed-envelope/v3",
		BindingHash:         frostActivationHex32(fixture.source.evidence.bindingHash),
		Payload:             raw,
		PayloadSHA256:       frostActivationHex32(payloadHash),
		SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(fixture.export.publicKeyDER),
		SignatureAlgorithm:  "ed25519",
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}
	capture := json.RawMessage{}
	if err := fixture.source.verifySignedEnvelope(envelope, &capture); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(capture, canonical) {
		t.Fatalf(
			"signed payload decoder did not consume exact verified bytes\nexpected: %s\nactual:   %s",
			canonical,
			capture,
		)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsCrossBindingEnvelopeAndRoot(
	t *testing.T,
) {
	t.Run("signed envelope", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		raw := json.RawMessage(`{"bindingHash":"0x01"}`)
		canonical, err := canonicalFrostActivationValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		payloadHash := sha256.Sum256(canonical)
		signature := ed25519.Sign(
			fixture.export.privateKey,
			append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...),
		)
		envelope := &frostRetainedGroupSignedEnvelope{
			Schema:              "tbtc-frost-retained-group-signed-envelope/v3",
			BindingHash:         frostActivationHex32([32]byte{0xff}),
			Payload:             raw,
			PayloadSHA256:       frostActivationHex32(payloadHash),
			SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(fixture.export.publicKeyDER),
			SignatureAlgorithm:  "ed25519",
			Signature:           base64.StdEncoding.EncodeToString(signature),
		}
		capture := json.RawMessage{}
		err = fixture.source.verifySignedEnvelope(envelope, &capture)
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("expected cross-binding envelope rejection, got [%v]", err)
		}
	})

	t.Run("history root", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.pageMutator = func(page *frostRetainedGroupHistoryPagePayload) {
			if !page.Complete {
				return
			}
			queryHash, err := parseFrostActivationHex32(page.QueryHash)
			if err != nil {
				t.Fatal(err)
			}
			mutationHashes := make([][32]byte, 0, len(fixture.mutations))
			for _, mutation := range fixture.mutations {
				canonical, err := canonicalFrostActivationValue(
					frostRetainedGroupMutationToWire(mutation),
				)
				if err != nil {
					t.Fatal(err)
				}
				mutationHashes = append(mutationHashes, sha256.Sum256(canonical))
			}
			page.Receipt.HistoryRoot = frostActivationHex32(
				frostRetainedGroupHistoryRootFromHashes(
					[32]byte{0xfe},
					queryHash,
					mutationHashes,
				),
			)
		}
		_, err := fixture.source.ReadCompleteHistory(
			context.Background(),
			fixture.from,
			fixture.to,
			fixture.checkpointAfter(),
		)
		if err == nil ||
			!strings.Contains(err.Error(), "does not cover the exact mutation sequence") {
			t.Fatalf("expected cross-binding history-root rejection, got [%v]", err)
		}
	})
}

func TestSignedFrostRetainedGroupHistorySource_RejectsDuplicateSignedPayloadKey(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	envelope := &frostRetainedGroupSignedEnvelope{
		Schema:              "tbtc-frost-retained-group-signed-envelope/v3",
		BindingHash:         frostActivationHex32(fixture.source.evidence.bindingHash),
		Payload:             json.RawMessage(`{"schema":"v1","schema":"v2"}`),
		PayloadSHA256:       frostActivationHex32([32]byte{0x01}),
		SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(fixture.export.publicKeyDER),
		SignatureAlgorithm:  "ed25519",
		Signature:           base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize)),
	}
	target := frostRetainedGroupHistoryPagePayload{}
	err := fixture.source.verifySignedEnvelope(envelope, &target)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate signed payload key rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsNonExactSignedPayloadKey(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	payload := json.RawMessage(`{"Schema":"tbtc-frost-retained-group-history-page/v2"}`)
	canonical, err := canonicalFrostActivationValue(payload)
	if err != nil {
		t.Fatal(err)
	}
	payloadHash := sha256.Sum256(canonical)
	signature := ed25519.Sign(
		fixture.export.privateKey,
		append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...),
	)
	envelope := &frostRetainedGroupSignedEnvelope{
		Schema:              "tbtc-frost-retained-group-signed-envelope/v3",
		BindingHash:         frostActivationHex32(fixture.source.evidence.bindingHash),
		Payload:             payload,
		PayloadSHA256:       frostActivationHex32(payloadHash),
		SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(fixture.export.publicKeyDER),
		SignatureAlgorithm:  "ed25519",
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}
	target := frostRetainedGroupHistoryPagePayload{}
	err = fixture.source.verifySignedEnvelope(envelope, &target)
	if err == nil || !strings.Contains(err.Error(), "non-exact or unknown key") {
		t.Fatalf("expected non-exact signed payload key rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsOmittedPage(t *testing.T) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.export.historyResponder = func(request frostRetainedGroupHistoryPageRequest) interface{} {
		pages := fixture.historyPages(
			request.Query,
			request.CheckpointAfter,
		)
		return pages[len(pages)-1]
	}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err == nil || !strings.Contains(err.Error(), "wrong identity or position") {
		t.Fatalf("expected omitted-page rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsTruncationAndForgery(
	t *testing.T,
) {
	t.Run("truncated pagination", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		defaultResponder := fixture.export.historyResponder
		fixture.export.historyResponder = func(
			request frostRetainedGroupHistoryPageRequest,
		) interface{} {
			if request.Cursor != "" {
				return nil
			}
			return defaultResponder(request)
		}
		_, err := fixture.source.ReadCompleteHistory(
			context.Background(),
			fixture.from,
			fixture.to,
			fixture.checkpointAfter(),
		)
		if err == nil || !strings.Contains(err.Error(), "HTTP status [503]") {
			t.Fatalf("expected truncated pagination rejection, got [%v]", err)
		}
	})

	t.Run("forged envelope", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.export.corruptSignature = true
		_, err := fixture.source.ReadCompleteHistory(
			context.Background(),
			fixture.from,
			fixture.to,
			fixture.checkpointAfter(),
		)
		if err == nil || !strings.Contains(err.Error(), "signature is invalid") {
			t.Fatalf("expected forged envelope rejection, got [%v]", err)
		}
	})
}

func TestSignedFrostRetainedGroupHistorySource_EnforcesAggregateResourceLimits(
	t *testing.T,
) {
	testCases := map[string]struct {
		configure func(*signedFrostRetainedGroupHistorySource)
		expected  string
	}{
		"aggregate response bytes": {
			configure: func(source *signedFrostRetainedGroupHistorySource) {
				source.maximumResponseBytes = 1
			},
			expected: "aggregate response-byte limit",
		},
		"page count": {
			configure: func(source *signedFrostRetainedGroupHistorySource) {
				source.maximumPages = 1
			},
			expected: "exceeded the page limit",
		},
		"mutation count": {
			configure: func(source *signedFrostRetainedGroupHistorySource) {
				source.maximumMutations = 3
			},
			expected: "exceeds the mutation limit",
		},
		"unique block count": {
			configure: func(source *signedFrostRetainedGroupHistorySource) {
				source.maximumUniqueBlocks = 2
			},
			expected: "exceeds the unique-block limit",
		},
		"end-to-end duration": {
			configure: func(source *signedFrostRetainedGroupHistorySource) {
				source.maximumReadDuration = time.Nanosecond
			},
			expected: "context deadline exceeded",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			testCase.configure(fixture.source)
			_, err := fixture.source.ReadCompleteHistory(
				context.Background(),
				fixture.from,
				fixture.to,
				fixture.checkpointAfter(),
			)
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf("expected %s rejection, got [%v]", name, err)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsOversizedReceiptLogSet(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	submissionHash := common.Hash(
		fixture.mutations[0].DkgSubmissionPoint.TransactionHash,
	)
	receipt := fixture.verifier.receipts[submissionHash]
	receipt.Logs = make([]*types.Log, frostRetainedGroupMaximumReceiptLogs+1)
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err == nil || !strings.Contains(err.Error(), "log limit") {
		t.Fatalf("expected oversized receipt-log rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsDuplicateAndReorderedHistory(
	t *testing.T,
) {
	tests := map[string]func(*frostRetainedGroupHistorySourceFixture){
		"duplicate event": func(fixture *frostRetainedGroupHistorySourceFixture) {
			fixture.mutations = append(fixture.mutations, fixture.mutations[3])
		},
		"reordered event": func(fixture *frostRetainedGroupHistorySourceFixture) {
			fixture.mutations[0], fixture.mutations[1] = fixture.mutations[1], fixture.mutations[0]
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			mutate(fixture)
			_, err := fixture.source.ReadCompleteHistory(
				context.Background(),
				fixture.from,
				fixture.to,
				fixture.checkpointAfter(),
			)
			if err == nil {
				t.Fatal("expected malformed exact history to be rejected")
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_AcceptsFilteredStakeWeightedSeats(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fullMembers := make([]uint32, 52)
	for index := range fullMembers {
		fullMembers[index] = uint32(index + 1)
	}
	// Repeated nonzero IDs are distinct stake-weighted sortition seats. The
	// second seat is excluded using the contract's strict 1-based index.
	fullMembers[51] = fullMembers[0]
	fixture.dkgFullMembers = fullMembers
	fixture.dkgMisbehaved = []uint8{2}
	fixture.installReceipts()

	history, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err != nil {
		t.Fatal(err)
	}
	active := history.Mutations[0].OperatorIDs
	if len(active) != 51 || active[0] != 1 || active[50] != 1 {
		t.Fatalf("unexpected ordered active DKG members: [%v]", active)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsInvalidMisbehavedIndices(
	t *testing.T,
) {
	testCases := map[string][]uint8{
		"zero":       {0},
		"duplicate":  {2, 2},
		"not sorted": {3, 2},
		"out of range": {
			52,
		},
	}
	for name, misbehaved := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			evidence, err := fixture.source.activationEvidence()
			if err != nil {
				t.Fatal(err)
			}
			admission := &fixture.mutations[0]
			result, _, _ := frostRetainedGroupHistoryTestDkgResult(
				t,
				evidence,
				admission.WalletID,
				fixture.dkgFullMembers,
			)
			result.MisbehavedMembersIndices = append([]uint8{}, misbehaved...)
			data, err := evidence.registryABI.Events["DkgResultSubmitted"].Inputs.NonIndexed().Pack(result)
			if err != nil {
				t.Fatal(err)
			}
			resultHash := [32]byte(crypto.Keccak256Hash(data))
			admission.DkgResultHash = resultHash

			submissionTransactionHash := common.Hash(
				admission.DkgSubmissionPoint.TransactionHash,
			)
			submissionReceipt := fixture.verifier.receipts[submissionTransactionHash]
			submissionReceipt.Logs[0].Topics[1] = common.Hash(resultHash)
			submissionReceipt.Logs[0].Data = data
			approvalTransactionHash := common.Hash(
				admission.DkgApprovalPoint.TransactionHash,
			)
			approvalReceipt := fixture.verifier.receipts[approvalTransactionHash]
			approvalReceipt.Logs[0].Topics[1] = common.Hash(resultHash)
			approvalReceipt.Logs[1].Topics[2] = common.Hash(resultHash)

			_, err = fixture.source.ReadCompleteHistory(
				context.Background(),
				fixture.from,
				fixture.to,
				fixture.checkpointAfter(),
			)
			if err == nil || !strings.Contains(
				err.Error(),
				"invalid misbehaved member indices",
			) {
				t.Fatalf("expected strict 1-based index rejection, got [%v]", err)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsIdentityCheckpointAndReceiptDrift(
	t *testing.T,
) {
	tests := map[string]func(*frostRetainedGroupHistoryPagePayload){
		"identity": func(page *frostRetainedGroupHistoryPagePayload) {
			page.Identity.TrustDomainID = "wrong-domain"
		},
		"checkpoint": func(page *frostRetainedGroupHistoryPagePayload) {
			page.From.BlockHash = frostActivationHex32([32]byte{0xff})
		},
		"manifest descriptor": func(page *frostRetainedGroupHistoryPagePayload) {
			page.DescriptorSetHash = frostActivationHex32([32]byte{0xdd})
		},
		"protocol binding": func(page *frostRetainedGroupHistoryPagePayload) {
			page.BindingHash = frostActivationHex32([32]byte{0xdc})
		},
		"receipt binding": func(page *frostRetainedGroupHistoryPagePayload) {
			if page.Complete {
				page.Receipt.BindingHash = frostActivationHex32([32]byte{0xdb})
			}
		},
		"receipt": func(page *frostRetainedGroupHistoryPagePayload) {
			if page.Complete {
				page.Receipt.HistoryRoot = frostActivationHex32([32]byte{0xee})
			}
		},
		"checkpoint chain root": func(page *frostRetainedGroupHistoryPagePayload) {
			if page.Complete {
				page.Receipt.CheckpointChainRoot =
					frostActivationHex32([32]byte{0xed})
			}
		},
		"checkpoint tip": func(page *frostRetainedGroupHistoryPagePayload) {
			if page.Complete {
				page.Receipt.CheckpointTipHash =
					frostActivationHex32([32]byte{0xec})
			}
		},
		"checkpoint completion": func(page *frostRetainedGroupHistoryPagePayload) {
			if page.Complete {
				page.Receipt.CheckpointComplete = false
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			fixture.pageMutator = mutate
			_, err := fixture.source.ReadCompleteHistory(
				context.Background(),
				fixture.from,
				fixture.to,
				fixture.checkpointAfter(),
			)
			if err == nil {
				t.Fatalf("expected %s drift to be rejected", name)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsSemanticForgeryInCanonicalBlock(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	for index := range fixture.mutations {
		fixture.mutations[index].WalletPublicKeyHash[0] ^= 0xff
	}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err == nil || !strings.Contains(err.Error(), "Bridge registration log") {
		t.Fatalf("expected canonical-block semantic forgery rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsWrongReceiptLog(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	admission := fixture.mutations[0]
	receipt := fixture.verifier.receipts[common.Hash(admission.BridgeRegistrationPoint.TransactionHash)]
	receipt.Logs[2].Topics[3] = common.Hash{0xff}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err == nil || !strings.Contains(err.Error(), "Bridge registration log") {
		t.Fatalf("expected wrong receipt-log rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsManifestCodeDrift(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.verifier.code[common.Address(fixture.profile.BridgeAddress)] = []byte{0xff}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err == nil || !strings.Contains(err.Error(), "signed activation manifest") {
		t.Fatalf("expected manifest code-drift rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsDeploymentTransitionEvent(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	deployment := cloneFrostRetainedGroupDeploymentEvidence(
		fixture.source.evidence.deployments["bridge"],
	)
	firstEnd := FrostPreSignFinality{
		BlockNumber: 2,
		BlockHash:   fixture.verifier.headers[2].Hash(),
	}
	deployment.HistoricalEpochs = []FrostPreSignDeploymentEpochEvidence{
		{
			Start:      deployment.HistoricalEpochs[0].Start,
			End:        &firstEnd,
			Descriptor: deployment.HistoricalEpochs[0].Descriptor,
		},
		{
			Start: FrostPreSignFinality{
				BlockNumber: 3,
				BlockHash:   fixture.verifier.headers[3].Hash(),
			},
			Descriptor: deployment.Current,
		},
	}

	if _, err := frostRetainedGroupDeploymentDescriptorAt(
		deployment,
		3,
		fixture.verifier.headers[3].Hash(),
		true,
	); err == nil || !strings.Contains(err.Error(), "implementation-transition block") {
		t.Fatalf("expected implementation-transition event rejection, got [%v]", err)
	}
	if _, err := frostRetainedGroupDeploymentDescriptorAt(
		deployment,
		3,
		fixture.verifier.headers[3].Hash(),
		false,
	); err != nil {
		t.Fatalf("expected exact transition state read to select the new epoch: [%v]", err)
	}
	if _, err := frostRetainedGroupDeploymentDescriptorAt(
		deployment,
		2,
		[32]byte{0xff},
		false,
	); err == nil || !strings.Contains(err.Error(), "signed deployment boundary") {
		t.Fatalf("expected exact epoch-boundary hash rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsEIP1967SlotDrift(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	descriptor := cloneFrostRetainedGroupDeploymentDescriptor(
		fixture.source.evidence.deployments["bridge"].Current,
	)
	proxyAddress := common.Address(descriptor.Address)
	implementationAddress := common.HexToAddress(
		"0x2222222222222222222222222222222222222222",
	)
	adminAddress := common.HexToAddress(
		"0x3333333333333333333333333333333333333333",
	)
	implementationCode := []byte{0x60, 0x51}
	adminCode := []byte{0x60, 0x52}
	implementationSlotValue := [32]byte{}
	copy(implementationSlotValue[12:], implementationAddress[:])
	adminSlotValue := [32]byte{}
	copy(adminSlotValue[12:], adminAddress[:])

	descriptor.Upgradeability = "eip1967"
	descriptor.ImplementationAddress = [20]byte(implementationAddress)
	descriptor.ImplementationCodeHash = [32]byte(
		crypto.Keccak256Hash(implementationCode),
	)
	descriptor.AdminAddress = [20]byte(adminAddress)
	descriptor.AdminCodeHash = [32]byte(crypto.Keccak256Hash(adminCode))
	descriptor.ImplementationSlotValue = implementationSlotValue
	descriptor.AdminSlotValue = adminSlotValue
	descriptor.DescriptorHash = descriptor.ComputeHash()
	fixture.verifier.code[implementationAddress] = implementationCode
	fixture.verifier.code[adminAddress] = adminCode
	fixture.verifier.storage[proxyAddress] = map[common.Hash][]byte{
		frostRetainedGroupEIP1967Slot("eip1967.proxy.implementation"): make(
			[]byte,
			32,
		),
		frostRetainedGroupEIP1967Slot("eip1967.proxy.admin"): append(
			[]byte{},
			adminSlotValue[:]...,
		),
	}

	err := fixture.source.authenticateContractDeployment(
		context.Background(),
		descriptor,
		2,
		fixture.verifier.headers[2].Hash(),
		make(frostRetainedGroupCodeCache),
	)
	if err == nil || !strings.Contains(err.Error(), "slot value mismatch") {
		t.Fatalf("expected EIP-1967 slot-drift rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsLinkedLibraryDrift(
	t *testing.T,
) {
	testCases := map[string]struct {
		copyReference bool
		actualCode    []byte
		expected      string
	}{
		"reference": {
			actualCode: []byte{0x60, 0x61},
			expected:   "reference",
		},
		"runtime code": {
			copyReference: true,
			actualCode:    []byte{0xff},
			expected:      "signed activation manifest",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			descriptor := cloneFrostRetainedGroupDeploymentDescriptor(
				fixture.source.evidence.deployments["bridge"].Current,
			)
			ownerAddress := common.Address(descriptor.Address)
			libraryAddress := common.HexToAddress(
				"0x4444444444444444444444444444444444444444",
			)
			expectedLibraryCode := []byte{0x60, 0x61}
			ownerCode := make([]byte, 24)
			if testCase.copyReference {
				copy(ownerCode[2:], libraryAddress[:])
			}
			descriptor.RuntimeCodeHash = [32]byte(crypto.Keccak256Hash(ownerCode))
			descriptor.LinkedLibraries = []FrostPreSignLinkedLibraryEvidence{{
				ProtocolRole:                "bridge-library",
				Address:                     [20]byte(libraryAddress),
				RuntimeCodeHash:             [32]byte(crypto.Keccak256Hash(expectedLibraryCode)),
				LinkedLibraryDescriptorHash: [32]byte{0x67},
				References: []FrostPreSignLinkedLibraryReference{{
					Start:  2,
					Length: 20,
				}},
			}}
			descriptor.DescriptorHash = descriptor.ComputeHash()
			fixture.verifier.code[ownerAddress] = ownerCode
			fixture.verifier.code[libraryAddress] = testCase.actualCode

			err := fixture.source.authenticateContractDeployment(
				context.Background(),
				descriptor,
				2,
				fixture.verifier.headers[2].Hash(),
				make(frostRetainedGroupCodeCache),
			)
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf("expected linked-library %s drift rejection, got [%v]", name, err)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsWrongEndpointAndReorg(
	t *testing.T,
) {
	resolve := func(raw string) frostRetainedGroupResolvedEndpoint {
		endpoint, _, err := validateFrostRetainedGroupTLSEndpoint(raw)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := resolveFrostRetainedGroupEndpoint(
			context.Background(),
			endpoint,
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	exportEndpoint := resolve("https://127.0.0.1:9443/export")
	verifierAlias := resolve("https://127.0.0.1:9444/rpc")
	primaryAlias := resolve("https://127.0.0.1:9445/rpc")
	if !frostRetainedGroupEndpointSetsOverlap(
		exportEndpoint,
		verifierAlias,
	) || !frostRetainedGroupEndpointSetsOverlap(
		exportEndpoint,
		primaryAlias,
	) {
		t.Fatal("expected resolved shared-backend aliases to be rejected")
	}

	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.verifier.reorgBlock = fixture.to.BlockNumber
	fixture.verifier.reorgAfterRead = 2
	fixture.verifier.reorgHeader = &types.Header{
		Number: new(big.Int).SetUint64(fixture.to.BlockNumber),
		Time:   999,
		Extra:  []byte{0xff},
	}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
		fixture.checkpointAfter(),
	)
	if err == nil || !strings.Contains(err.Error(), "canonical chain") {
		t.Fatalf("expected finalized-chain reorg rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsWrongOperatorReceipt(
	t *testing.T,
) {
	t.Run("wrong binding", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.operatorMutator = func(payload *frostRetainedGroupOperatorReceiptPayload) {
			payload.BindingHash = frostActivationHex32([32]byte{0x98})
		}
		_, err := fixture.source.ResolveOperatorID(
			context.Background(),
			chain.Address("0x1111111111111111111111111111111111111111"),
			fixture.to,
		)
		if err == nil || !strings.Contains(err.Error(), "differently bound") {
			t.Fatalf("expected operator binding rejection, got [%v]", err)
		}
	})

	t.Run("wrong point", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.operatorMutator = func(payload *frostRetainedGroupOperatorReceiptPayload) {
			payload.At.BlockHash = frostActivationHex32([32]byte{0x99})
		}
		_, err := fixture.source.ResolveOperatorID(
			context.Background(),
			chain.Address("0x1111111111111111111111111111111111111111"),
			fixture.to,
		)
		if err == nil || !strings.Contains(err.Error(), "differently bound") {
			t.Fatalf("expected operator receipt rejection, got [%v]", err)
		}
	})

	t.Run("wrong ID", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.operatorMutator = func(payload *frostRetainedGroupOperatorReceiptPayload) {
			payload.OperatorID++
		}
		_, err := fixture.source.ResolveOperatorID(
			context.Background(),
			chain.Address("0x1111111111111111111111111111111111111111"),
			fixture.to,
		)
		if err == nil || !strings.Contains(err.Error(), "disagrees with exact finalized") {
			t.Fatalf("expected independent operator-ID rejection, got [%v]", err)
		}
	})
}
