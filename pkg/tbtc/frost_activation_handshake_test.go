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
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

type testFrostActivationPointVerifier struct {
	err   error
	point FrostPreSignFinality
}

func (tfapv *testFrostActivationPointVerifier) VerifyFrostPreSignActivationPoint(
	ctx context.Context,
	point FrostPreSignFinality,
) error {
	tfapv.point = point
	return tfapv.err
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
	readinessPath := filepath.Join(directory, "readiness.json")
	writeTestFrostActivationReadiness(t, readinessPath, manifest, point)
	endpoint := testLoopbackEndpoint(t)
	verifier := &testFrostActivationPointVerifier{}
	outbox := &bitcoinBroadcastOutbox{
		records:   make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord),
		recovered: true,
	}
	exporter, err := newFrostActivationHandshakeExporter(
		endpoint,
		privateKeyPath,
		readinessPath,
		manifest,
		verifier,
		outbox,
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
		!handshake.Payload.State.Healthy ||
		!handshake.Payload.State.DurableBitcoinOutboxRecovered ||
		verifier.point.BlockNumber != point.BlockNumber ||
		frostActivationHex32(verifier.point.BlockHash) != point.BlockHash {
		t.Fatalf("unexpected handshake: %+v", handshake)
	}
	canonicalPayload, err := canonicalFrostActivationValue(handshake.Payload)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(handshake.Signature)
	if err != nil || !ed25519.Verify(publicKey, canonicalPayload, signature) {
		t.Fatal("handshake signature did not verify over canonical payload")
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
	readinessPath := filepath.Join(directory, "readiness.json")
	writeTestFrostActivationReadiness(t, readinessPath, manifest, point)
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
					ObservedAtUnix:               1,
				},
			},
		},
		recovered: true,
	}
	exporter, err := newFrostActivationHandshakeExporter(
		endpoint,
		privateKeyPath,
		readinessPath,
		manifest,
		&testFrostActivationPointVerifier{},
		outbox,
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
			EthereumPoint: point,
		},
	}
	response := postTestFrostActivationHandshake(t, endpoint, request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("quarantined outbox returned status [%d]", response.StatusCode)
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

func testFrostActivationRuntimeManifest(
	keyHash [32]byte,
) FrostPreSignActivationRuntimeManifest {
	return FrostPreSignActivationRuntimeManifest{
		ManifestHash:                     [32]byte{0x10},
		SignerProtocolID:                 [32]byte{0x11},
		ReservationProtocolID:            [32]byte{0x12},
		BitcoinOutboxProtocolID:          [32]byte{0x13},
		SigningPolicyHash:                [32]byte{0x14},
		DurableSessionStoreFingerprint:   "store-fingerprint",
		CompleteRouterAddress:            [20]byte{0x15},
		AuthorizationRegistryAddress:     [20]byte{0x16},
		AttestationSignerKeyHash:         keyHash,
		Threshold:                        51,
		MaximumGroupSize:                 100,
		RetainedGroupInventoryProtocolID: [32]byte{0x17},
	}
}

func writeTestFrostActivationReadiness(
	t *testing.T,
	filePath string,
	manifest FrostPreSignActivationRuntimeManifest,
	point frostActivationEthereumPoint,
) {
	t.Helper()
	snapshot := frostActivationReadinessSnapshot{
		Schema:                         frostActivationReadinessSnapshotSchema,
		ManifestHash:                   frostActivationHex32(manifest.ManifestHash),
		DurableSessionStoreFingerprint: manifest.DurableSessionStoreFingerprint,
		FrostWalletGroupInventory: frostActivationWalletGroupInventory{
			Schema:                   frostActivationInventorySchema,
			Point:                    point,
			SnapshotGeneration:       9,
			InventoryRoot:            frostActivationHex32([32]byte{0x18}),
			WalletCount:              1,
			MinimumActualGroupSize:   51,
			MaximumActualGroupSize:   100,
			MembershipAmbiguityCount: 0,
			GroupSizeViolationCount:  0,
			Complete:                 true,
		},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatal(err)
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
