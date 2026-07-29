package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestFrostNativeSignerAnchorStreamIDFrozenAndStable(t *testing.T) {
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:             testFrostNativeSignerAnchorBytes32(0x11),
		TrustDomainID:          "prod.example",
		SignerStoreFingerprint: testFrostNativeSignerAnchorBytes32(0x22),
	}
	expected, err := hex.DecodeString(
		"661f6dcc9958944992d82db22cf4986e70991ff4db3680e1014cbd6a90775661",
	)
	if err != nil {
		t.Fatal(err)
	}
	actual := ComputeFrostNativeSignerAnchorStreamID(identity)
	if !bytes.Equal(actual[:], expected) {
		t.Fatalf("unexpected frozen stream ID [%x]", actual)
	}

	rotatingMutations := []func(*FrostNativeSignerAnchorIdentity){
		func(value *FrostNativeSignerAnchorIdentity) {
			value.ActivationManifestHash = testFrostNativeSignerAnchorBytes32(1)
		},
		func(value *FrostNativeSignerAnchorIdentity) { value.ActivationManifestSequence = 9 },
		func(value *FrostNativeSignerAnchorIdentity) {
			value.EndpointLeafSPKIHash = testFrostNativeSignerAnchorBytes32(2)
		},
		func(value *FrostNativeSignerAnchorIdentity) {
			value.OnlineKeyHash = testFrostNativeSignerAnchorBytes32(3)
		},
		func(value *FrostNativeSignerAnchorIdentity) {
			value.OperatorFingerprint = testFrostNativeSignerAnchorBytes32(4)
		},
		func(value *FrostNativeSignerAnchorIdentity) { value.HistoryStoreID = "rotated" },
		func(value *FrostNativeSignerAnchorIdentity) {
			value.HistoryStoreFingerprint = testFrostNativeSignerAnchorBytes32(5)
		},
		func(value *FrostNativeSignerAnchorIdentity) {
			value.HistoryClusterFingerprint = testFrostNativeSignerAnchorBytes32(6)
		},
		func(value *FrostNativeSignerAnchorIdentity) {
			value.OfflineAuthorityHash = testFrostNativeSignerAnchorBytes32(7)
		},
		func(value *FrostNativeSignerAnchorIdentity) {
			value.ClientSPKIHash = testFrostNativeSignerAnchorBytes32(8)
		},
		func(value *FrostNativeSignerAnchorIdentity) {
			value.TransportBinding = testFrostNativeSignerAnchorBytes32(9)
		},
		func(value *FrostNativeSignerAnchorIdentity) { value.WitnessMaximumRecords = 100 },
		func(value *FrostNativeSignerAnchorIdentity) {
			value.WitnessRotationThresholdRecords = 50
		},
	}
	baseBinding := ComputeFrostNativeSignerAnchorBindingHash(identity)
	for index, mutate := range rotatingMutations {
		mutated := identity
		mutate(&mutated)
		if ComputeFrostNativeSignerAnchorStreamID(mutated) != actual {
			t.Fatalf("rotating field mutation [%d] changed the stable stream", index)
		}
		if ComputeFrostNativeSignerAnchorBindingHash(mutated) == baseBinding {
			t.Fatalf("rotating field mutation [%d] did not change the binding", index)
		}
	}
	stableMutations := []func(*FrostNativeSignerAnchorIdentity){
		func(value *FrostNativeSignerAnchorIdentity) {
			value.ProtocolID = testFrostNativeSignerAnchorBytes32(0x31)
		},
		func(value *FrostNativeSignerAnchorIdentity) { value.TrustDomainID = "other.example" },
		func(value *FrostNativeSignerAnchorIdentity) {
			value.SignerStoreFingerprint = testFrostNativeSignerAnchorBytes32(0x32)
		},
	}
	for index, mutate := range stableMutations {
		mutated := identity
		mutate(&mutated)
		if ComputeFrostNativeSignerAnchorStreamID(mutated) == actual {
			t.Fatalf("stable field mutation [%d] did not change the stream", index)
		}
	}
}

func TestFrostNativeSignerAnchorPost_ClassifiesTransportDelivery(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedEndpoint := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	client := &FrostNativeSignerAnchorClient{
		httpClient:     &http.Client{Transport: &http.Transport{}},
		requestTimeout: time.Second,
	}
	if _, sent, err := client.post(
		context.Background(),
		closedEndpoint,
		[]byte(`{"request":"test"}`),
	); err == nil || sent {
		t.Fatalf(
			"connection-refused request was not definitively unsent: sent [%v], error [%v]",
			sent,
			err,
		)
	}

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if _, err := io.ReadAll(request.Body); err != nil {
				t.Errorf("failed to read request body: %v", err)
			}
			hijacker, ok := writer.(http.Hijacker)
			if !ok {
				t.Error("response writer cannot be hijacked")
				return
			}
			connection, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("failed to hijack response: %v", err)
				return
			}
			_ = connection.Close()
		},
	))
	defer server.Close()
	if _, sent, err := client.post(
		context.Background(),
		server.URL,
		[]byte(`{"request":"test"}`),
	); err == nil || !sent {
		t.Fatalf(
			"written request was not classified ambiguous: sent [%v], error [%v]",
			sent,
			err,
		)
	}
}

func TestFrostNativeSignerAnchorRestartBoundsAlignAcrossGoLayers(
	t *testing.T,
) {
	if FrostNativeSignerAnchorMaximumHistoryEvents !=
		frostsigning.NativeTBTCSignerStateAnchorMaximumRevisionDistance {
		t.Fatalf(
			"anchor history bound [%d] differs from signer barrier bound [%d]",
			FrostNativeSignerAnchorMaximumHistoryEvents,
			frostsigning.NativeTBTCSignerStateAnchorMaximumRevisionDistance,
		)
	}
	if FrostNativeSignerAnchorMaximumHistoryProofEntries !=
		frostsigning.NativeTBTCSignerStateAnchorMaximumGenerationDistance {
		t.Fatalf(
			"signer proof bound [%d] differs from signer barrier generation bound [%d]",
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
			frostsigning.NativeTBTCSignerStateAnchorMaximumGenerationDistance,
		)
	}
	if frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall !=
		frostsigning.NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation {
		t.Fatalf(
			"admission per-call generation bound [%d] differs from output barrier bound [%d]",
			frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall,
			frostsigning.NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
		)
	}
}

func TestFrostNativeSignerAnchorClientRejectsAliasedCryptographicRoles(
	t *testing.T,
) {
	privateKey := func(seedByte byte) ed25519.PrivateKey {
		seed := make([]byte, ed25519.SeedSize)
		for index := range seed {
			seed[index] = seedByte
		}
		return ed25519.NewKeyFromSeed(seed)
	}
	spki := func(t *testing.T, publicKey ed25519.PublicKey) []byte {
		t.Helper()
		result, err := x509.MarshalPKIXPublicKey(publicKey)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	clientKey := privateKey(0x11)
	onlineKey := privateKey(0x22)
	offlineKey := privateKey(0x33)
	clientSPKI := spki(t, clientKey.Public().(ed25519.PublicKey))
	onlineSPKI := spki(t, onlineKey.Public().(ed25519.PublicKey))
	offlineSPKI := spki(t, offlineKey.Public().(ed25519.PublicKey))
	endpoint := "http://127.0.0.1:19487/v1/anchor"
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                      testFrostNativeSignerAnchorBytes32(0x41),
		ActivationManifestHash:          testFrostNativeSignerAnchorBytes32(0x42),
		ActivationManifestSequence:      1,
		TrustDomainID:                   "role-separation.example",
		OnlineKeyHash:                   sha256.Sum256(onlineSPKI),
		OperatorFingerprint:             testFrostNativeSignerAnchorBytes32(0x43),
		HistoryStoreID:                  "role-separation-history",
		HistoryStoreFingerprint:         testFrostNativeSignerAnchorBytes32(0x44),
		HistoryClusterFingerprint:       testFrostNativeSignerAnchorBytes32(0x45),
		OfflineAuthorityHash:            sha256.Sum256(offlineSPKI),
		ClientSPKIHash:                  sha256.Sum256(clientSPKI),
		SignerStoreFingerprint:          testFrostNativeSignerAnchorBytes32(0x46),
		TransportBinding:                ComputeFrostNativeSignerAnchorTransportBinding(endpoint),
		WitnessMaximumRecords:           100,
		WitnessRotationThresholdRecords: 50,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	config := FrostNativeSignerAnchorClientConfig{
		Endpoint:            endpoint,
		ClientPrivateKey:    clientKey,
		OnlinePublicKeySPKI: onlineSPKI,
		Identity:            identity,
	}
	if _, err := NewFrostNativeSignerAnchorClient(config); err != nil {
		t.Fatalf("distinct cryptographic roles were rejected: %v", err)
	}

	tests := map[string]func(
		*FrostNativeSignerAnchorClientConfig,
	){
		"client and online": func(
			config *FrostNativeSignerAnchorClientConfig,
		) {
			config.OnlinePublicKeySPKI = clientSPKI
			config.Identity.OnlineKeyHash = sha256.Sum256(clientSPKI)
		},
		"client and offline": func(
			config *FrostNativeSignerAnchorClientConfig,
		) {
			config.Identity.OfflineAuthorityHash = sha256.Sum256(clientSPKI)
		},
		"online and offline": func(
			config *FrostNativeSignerAnchorClientConfig,
		) {
			config.Identity.OfflineAuthorityHash = sha256.Sum256(onlineSPKI)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := config
			mutate(&candidate)
			if _, err := NewFrostNativeSignerAnchorClient(candidate); err == nil ||
				!strings.Contains(err.Error(), "pairwise distinct") {
				t.Fatalf("aliased cryptographic roles were accepted: %v", err)
			}
		})
	}
}

func TestFrostNativeSignerCheckpointAcknowledgementFrozenVector(t *testing.T) {
	wire := frostNativeSignerAnchorAcknowledgementWire{
		Schema:            FrostNativeSignerCheckpointAcknowledgementSchema,
		BindingHash:       frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x11)),
		RequestDigest:     frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x22)),
		Nonce:             frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x33)),
		Status:            "applied",
		ServiceEpoch:      "2",
		Revision:          "3",
		PreviousEventRoot: frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x44)),
		EventRoot:         frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x55)),
		Checkpoint: frostNativeSignerAnchorCheckpointWire{
			StoreFingerprint:        frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x66)),
			Generation:              "7",
			PreviousStateCommitment: frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x77)),
			StateImageDigest:        frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x88)),
			StateCommitment:         frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x99)),
		},
		OperationID:       frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0xaa)),
		TransitionDigest:  frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0xbb)),
		CommittedAtUnixMs: "1700000000000",
		ExpiresAtUnixMs:   "1700000030000",
	}
	digest, err := frostNativeSignerAnchorAcknowledgementTranscript(wire)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(digest) !=
		"55f88c32a0b168003cedfb88cf47a467b607dbd1f2ab6f20ddc7976bd396b239" {
		t.Fatalf("unexpected Rust service-response digest [%x]", digest)
	}

	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x01}, ed25519.SeedSize))
	signature := ed25519.Sign(privateKey, digest)
	if hex.EncodeToString(signature) !=
		"0a60e68808285197c4ddb4b68dc10439aad6cbde085fd93b7cf863b7abf8197131d73f35304862ea80dc5cfd88d0cac80f9fa42b54efa036b0a82956c62f0608" {
		t.Fatalf("unexpected frozen Ed25519 signature [%x]", signature)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(publicKeyDER) !=
		"302a300506032b65700321008a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c" {
		t.Fatalf("unexpected frozen public key [%x]", publicKeyDER)
	}
	var signatureArray [ed25519.SignatureSize]byte
	copy(signatureArray[:], signature)
	var signingDigest [32]byte
	copy(signingDigest[:], digest)
	acknowledgementDigest := computeFrostNativeSignerCheckpointAcknowledgementDigest(
		signingDigest,
		signatureArray,
		sha256.Sum256(publicKeyDER),
	)
	if hex.EncodeToString(acknowledgementDigest[:]) !=
		"4c30e2aa6a048993fede1a754a0567a6faef8180544398ff284567f722c6ad01" {
		t.Fatalf("unexpected frozen acknowledgement digest [%x]", acknowledgementDigest)
	}
}

func TestFrostNativeSignerAnchorEventRootFrozenVector(t *testing.T) {
	acknowledgement := FrostNativeSignerCheckpointAcknowledgement{
		BindingHash:       testFrostNativeSignerAnchorBytes32(0x11),
		RequestDigest:     testFrostNativeSignerAnchorBytes32(0x22),
		Nonce:             testFrostNativeSignerAnchorBytes32(0x33),
		Status:            "applied",
		ServiceEpoch:      2,
		Revision:          3,
		PreviousEventRoot: testFrostNativeSignerAnchorBytes32(0x44),
		Checkpoint: FrostNativeSignerStateWitnessCheckpoint{
			StoreFingerprint:        testFrostNativeSignerAnchorBytes32(0x66),
			Generation:              7,
			PreviousStateCommitment: testFrostNativeSignerAnchorBytes32(0x77),
			StateImageDigest:        testFrostNativeSignerAnchorBytes32(0x88),
			StateCommitment:         testFrostNativeSignerAnchorBytes32(0x99),
		},
		OperationID:       testFrostNativeSignerAnchorBytes32(0xaa),
		TransitionDigest:  testFrostNativeSignerAnchorBytes32(0xbb),
		CommittedAtUnixMs: 1700000000000,
		ExpiresAtUnixMs:   1700000030000,
	}
	actual := computeFrostNativeSignerAnchorEventRoot(acknowledgement)
	if hex.EncodeToString(actual[:]) !=
		"251cf2f635ea82533f55d323104232ecfd47a748a45fbbe16e8ed212c8c69a90" {
		t.Fatalf("unexpected frozen event root [%x]", actual)
	}
}

func TestFrostNativeSignerAnchorReadResponseFrozenVector(t *testing.T) {
	checkpoint := frostNativeSignerAnchorCheckpointWire{
		StoreFingerprint:        frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x55)),
		Generation:              "6",
		PreviousStateCommitment: frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x66)),
		StateImageDigest:        frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x77)),
		StateCommitment:         frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x88)),
	}
	response := frostNativeSignerAnchorReadResponse{
		Schema:              FrostNativeSignerAnchorReadResponseSchema,
		BindingHash:         frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x11)),
		RequestDigest:       frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x22)),
		Nonce:               frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x33)),
		Status:              "present",
		ServiceEpoch:        "2",
		Revision:            "3",
		EventRoot:           frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x44)),
		Checkpoint:          &checkpoint,
		OperationID:         frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0x99)),
		TransitionDigest:    frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0xaa)),
		CommittedAtUnixMs:   "1700000000000",
		ExpiresAtUnixMs:     "1700000030000",
		CheckpointAck:       json.RawMessage(`{"x":1}`),
		CheckpointAckDigest: frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(0xbb)),
	}
	digest, err := frostNativeSignerAnchorReadResponseTranscript(response)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(digest) !=
		"bc595335e39a91bdaf49fc749f6df910be31385ad394089ba633bec359f47a20" {
		t.Fatalf("unexpected frozen read-response digest [%x]", digest)
	}
}

func TestFrostNativeSignerAnchorHistoryFrozenVectors(t *testing.T) {
	proof := []frostsigning.NativeTBTCSignerStateWitnessProofEntry{
		{
			Generation:              4,
			PreviousStateCommitment: testFrostNativeSignerAnchorBytes32(0x22),
			StateImageDigest:        testFrostNativeSignerAnchorBytes32(0x33),
			StateCommitment:         testFrostNativeSignerAnchorBytes32(0x44),
		},
		{
			Generation:              5,
			PreviousStateCommitment: testFrostNativeSignerAnchorBytes32(0x44),
			StateImageDigest:        testFrostNativeSignerAnchorBytes32(0x55),
			StateCommitment:         testFrostNativeSignerAnchorBytes32(0x66),
		},
	}
	eventDigest := computeFrostNativeSignerAnchorHistoryEventDigest(
		3,
		testFrostNativeSignerAnchorBytes32(0x11),
		[]byte(`{"x":1}`),
		proof,
	)
	if hex.EncodeToString(eventDigest[:]) !=
		"2c97251a24d2444e4e6b1e6fa28e1956847d6a652e7cbd7dd18b4b0ce2aba167" {
		t.Fatalf("unexpected frozen history event digest [%x]", eventDigest)
	}
	floor := FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          4,
		Revision:              1,
		EventRoot:             testFrostNativeSignerAnchorBytes32(0x04),
		AcknowledgementDigest: testFrostNativeSignerAnchorBytes32(0x05),
		Checkpoint: FrostNativeSignerStateWitnessCheckpoint{
			StoreFingerprint:        testFrostNativeSignerAnchorBytes32(0x06),
			Generation:              7,
			PreviousStateCommitment: testFrostNativeSignerAnchorBytes32(0x07),
			StateImageDigest:        testFrostNativeSignerAnchorBytes32(0x08),
			StateCommitment:         testFrostNativeSignerAnchorBytes32(0x09),
		},
	}
	target := FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          4,
		Revision:              3,
		EventRoot:             testFrostNativeSignerAnchorBytes32(0x0a),
		AcknowledgementDigest: testFrostNativeSignerAnchorBytes32(0x0b),
		Checkpoint: FrostNativeSignerStateWitnessCheckpoint{
			StoreFingerprint:        testFrostNativeSignerAnchorBytes32(0x06),
			Generation:              9,
			PreviousStateCommitment: testFrostNativeSignerAnchorBytes32(0x0c),
			StateImageDigest:        testFrostNativeSignerAnchorBytes32(0x0d),
			StateCommitment:         testFrostNativeSignerAnchorBytes32(0x0e),
		},
	}
	events := []frostNativeSignerAnchorHistoryEventWire{{
		CheckpointAck: json.RawMessage(`{"x":1}`),
		WitnessProof:  frostNativeSignerAnchorProofToWire(proof),
	}}
	response := frostNativeSignerAnchorHistoryResponse{
		Schema:            FrostNativeSignerAnchorHistoryResponseSchema,
		BindingHash:       frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(1)),
		RequestDigest:     frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(2)),
		Nonce:             frostNativeSignerAnchorHex32(testFrostNativeSignerAnchorBytes32(3)),
		Status:            "partial",
		ServiceEpoch:      "4",
		FloorRef:          frostNativeSignerAnchorHistoryReferenceToWire(floor),
		TargetRef:         frostNativeSignerAnchorHistoryReferenceToWire(target),
		StartRevision:     "2",
		NextRevision:      "3",
		EventCount:        "1",
		ProofEntryCount:   "2",
		Events:            &events,
		CommittedAtUnixMs: "1700000000000",
		ExpiresAtUnixMs:   "1700000030000",
	}
	responseDigest, err := frostNativeSignerAnchorHistoryResponseTranscript(
		response,
		[][32]byte{eventDigest},
	)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(responseDigest) !=
		"3832295275cf6ef5dc3f0653191215dd141c197ec4c1c856787f4b64381c792e" {
		t.Fatalf("unexpected frozen history response digest [%x]", responseDigest)
	}
}

func TestDecodeStrictFrostNativeSignerAnchorJSONRejectsParserDifferentials(t *testing.T) {
	tests := map[string]string{
		"duplicate top level":              `{"schema":"a","schema":"b"}`,
		"duplicate payload":                `{"payload":{"kind":"read","kind":"advance"}}`,
		"duplicate identity":               `{"identity":{"protocolID":"a","protocolID":"b"}}`,
		"duplicate checkpoint":             `{"checkpoint":{"generation":"1","generation":"2"}}`,
		"duplicate nested acknowledgement": `{"checkpointAck":{"status":"applied","status":"already-applied"}}`,
		"case folded alias":                `{"schema":"a","Schema":"b"}`,
		"non ASCII member":                 `{"sch\u0065ma":"a","státus":"b"}`,
		"trailing value":                   `{"schema":"a"}{"schema":"b"}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			target := map[string]interface{}{}
			if err := decodeStrictFrostNativeSignerAnchorJSON(
				[]byte(payload),
				&target,
			); err == nil {
				t.Fatal("expected hardened JSON rejection")
			}
		})
	}
}

func TestFrostNativeSignerAnchorJSONDepthBound(t *testing.T) {
	atLimit := strings.Repeat("[", frostNativeSignerAnchorMaximumJSONDepth) +
		"0" +
		strings.Repeat("]", frostNativeSignerAnchorMaximumJSONDepth)
	if err := preflightFrostNativeSignerAnchorJSON([]byte(atLimit)); err != nil {
		t.Fatalf("expected JSON at the depth bound: %v", err)
	}
	overLimit := "[" + atLimit + "]"
	if err := preflightFrostNativeSignerAnchorJSON(
		[]byte(overLimit),
	); err == nil || !strings.Contains(err.Error(), "depth bound") {
		t.Fatalf("expected JSON depth rejection, got [%v]", err)
	}
}

func TestFrostNativeSignerAnchorCanonicalUint64(t *testing.T) {
	for _, value := range []string{"0", "1", "18446744073709551615"} {
		if _, err := frostNativeSignerAnchorParseUint64(value); err != nil {
			t.Fatalf("expected canonical uint64 [%s]: %v", value, err)
		}
	}
	for _, value := range []string{"", "00", "01", "+1", "-1", "1.0", "18446744073709551616"} {
		if _, err := frostNativeSignerAnchorParseUint64(value); err == nil {
			t.Fatalf("expected non-canonical uint64 rejection [%s]", value)
		}
	}
}

func TestValidateFrostNativeSignerAnchorEndpoint(t *testing.T) {
	valid := []string{
		"http://127.0.0.1:8080/anchor",
		"http://[::1]:8080/anchor",
		"https://anchor.example/anchor",
		"https://anchor.example:8443/anchor",
	}
	for _, value := range valid {
		if _, _, err := validateFrostNativeSignerAnchorEndpoint(value); err != nil {
			t.Fatalf("expected valid endpoint [%s]: %v", value, err)
		}
	}
	invalid := []string{
		"http://localhost:8080/anchor",
		"http://127.0.0.1/anchor",
		"http://127.0.0.1:08080/anchor",
		"https://user@anchor.example/anchor",
		"https://anchor.example/anchor?query=1",
		"https://anchor.example/anchor#fragment",
		"https://ANCHOR.example/anchor",
		"https://anchor.example/a/../anchor",
		"https://anchor.example/anchor/",
		"https://anchor.example/%61nchor",
	}
	for _, value := range invalid {
		if _, _, err := validateFrostNativeSignerAnchorEndpoint(value); err == nil {
			t.Fatalf("expected invalid endpoint rejection [%s]", value)
		}
	}
}

func TestFrostNativeSignerAnchorClientRequiresPKIXAndLeafSPKIPin(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config: &http.Server{
			Handler: http.HandlerFunc(
				func(writer http.ResponseWriter, _ *http.Request) {
					http.Error(
						writer,
						"expected test response",
						http.StatusInternalServerError,
					)
				},
			),
		},
	}
	server.StartTLS()
	defer server.Close()
	endpoint := server.URL + "/anchor"
	clientPrivate := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x31}, ed25519.SeedSize),
	)
	onlinePrivate := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x32}, ed25519.SeedSize),
	)
	clientSPKI, _ := x509.MarshalPKIXPublicKey(clientPrivate.Public())
	onlineSPKI, _ := x509.MarshalPKIXPublicKey(onlinePrivate.Public())
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                 testFrostNativeSignerAnchorBytes32(1),
		ActivationManifestHash:     testFrostNativeSignerAnchorBytes32(2),
		ActivationManifestSequence: 1,
		TrustDomainID:              "tls-test.example",
		EndpointLeafSPKIHash: sha256.Sum256(
			server.Certificate().RawSubjectPublicKeyInfo,
		),
		OnlineKeyHash:                   sha256.Sum256(onlineSPKI),
		OperatorFingerprint:             testFrostNativeSignerAnchorBytes32(3),
		HistoryStoreID:                  "tls-history",
		HistoryStoreFingerprint:         testFrostNativeSignerAnchorBytes32(4),
		HistoryClusterFingerprint:       testFrostNativeSignerAnchorBytes32(5),
		OfflineAuthorityHash:            testFrostNativeSignerAnchorBytes32(6),
		ClientSPKIHash:                  sha256.Sum256(clientSPKI),
		SignerStoreFingerprint:          testFrostNativeSignerAnchorBytes32(7),
		TransportBinding:                ComputeFrostNativeSignerAnchorTransportBinding(endpoint),
		WitnessMaximumRecords:           100,
		WitnessRotationThresholdRecords: 50,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	newClient := func(
		identity FrostNativeSignerAnchorIdentity,
		roots *x509.CertPool,
	) *FrostNativeSignerAnchorClient {
		client, err := NewFrostNativeSignerAnchorClient(
			FrostNativeSignerAnchorClientConfig{
				Endpoint:            endpoint,
				TLSRootCAs:          roots,
				ClientPrivateKey:    clientPrivate,
				OnlinePublicKeySPKI: onlineSPKI,
				Identity:            identity,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	validClient := newClient(identity, roots)
	if _, err := validClient.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err == nil || !strings.Contains(err.Error(), "HTTP status [500]") {
		t.Fatalf("expected request to pass TLS and reach the handler, got [%v]", err)
	}

	wrongPinIdentity := identity
	wrongPinIdentity.EndpointLeafSPKIHash = testFrostNativeSignerAnchorBytes32(0xee)
	wrongPinClient := newClient(wrongPinIdentity, roots)
	if _, err := wrongPinClient.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err == nil || !strings.Contains(err.Error(), "TLS leaf SPKI mismatch") {
		t.Fatalf("expected TLS leaf SPKI rejection, got [%v]", err)
	}

	untrustedClient := newClient(identity, nil)
	if _, err := untrustedClient.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err == nil {
		t.Fatal("expected normal PKIX verification to reject the untrusted test root")
	}
}

func TestFrostNativeSignerAnchorAcknowledgementTimeBounds(t *testing.T) {
	environment := newTestFrostNativeSignerAnchorEnvironment(t, "normal")
	defer environment.server.Close()

	environment.client.maximumAckLife = 29 * time.Second
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err == nil {
		t.Fatal("expected acknowledgement lifetime rejection below the 30s boundary")
	}
	environment.client.maximumAckLife = 30 * time.Second
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err != nil {
		t.Fatalf("expected exact 30s acknowledgement lifetime: %v", err)
	}
}

func TestFrostNativeSignerAnchorClientRepeatedReadAndCASRecovery(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"apply then lose response", "apply-ambiguous"},
		{"retain expected then retry", "expected-ambiguous"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := newTestFrostNativeSignerAnchorEnvironment(t, test.mode)
			defer environment.server.Close()

			first, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
				context.Background(),
			)
			if err != nil {
				t.Fatal(err)
			}
			second, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
				context.Background(),
			)
			if err != nil {
				t.Fatalf("repeated fresh read failed: %v", err)
			}
			if first.AcknowledgementDigest != second.AcknowledgementDigest ||
				!bytes.Equal(first.AcknowledgementJSON, second.AcknowledgementJSON) {
				t.Fatal("repeated read changed the exact stored acknowledgement")
			}

			result, err := environment.client.CompareAndSwapFrostNativeSignerStateWitnessAnchor(
				context.Background(),
				environment.expected,
				environment.candidate,
				environment.proof,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Recovered ||
				result.Acknowledgement.Checkpoint != environment.candidate ||
				len(result.Acknowledgement.ExactAcknowledgement) == 0 {
				t.Fatal("ambiguous CAS did not return the exact recovered candidate receipt")
			}
		})
	}
}

func TestFrostNativeSignerAnchorClientCASRequiresAuthenticatedRead(t *testing.T) {
	environment := newTestFrostNativeSignerAnchorEnvironment(t, "normal")
	defer environment.server.Close()
	if _, err := environment.client.CompareAndSwapFrostNativeSignerStateWitnessAnchor(
		context.Background(),
		environment.expected,
		environment.candidate,
		environment.proof,
	); err == nil || !strings.Contains(err.Error(), "requires a fresh authenticated") {
		t.Fatalf("expected missing-read rejection, got [%v]", err)
	}
}

func TestFrostNativeSignerAnchorClientHistoryPublishesTargetAtomically(t *testing.T) {
	for _, publicReadFirst := range []bool{false, true} {
		name := "internal target reads"
		if publicReadFirst {
			name = "legacy public target read first"
		}
		t.Run(name, func(t *testing.T) {
			environment := newTestFrostNativeSignerAnchorEnvironment(t, "history")
			defer environment.server.Close()
			if publicReadFirst {
				record, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
					context.Background(),
				)
				if err != nil {
					t.Fatal(err)
				}
				if record.Revision != environment.historyTarget.Revision {
					t.Fatal("public target read did not return the history target")
				}
			}
			history, err :=
				environment.client.ReadFrostNativeSignerStateWitnessAnchorHistory(
					context.Background(),
					environment.historyFloor,
				)
			if err != nil {
				t.Fatal(err)
			}
			if history.Floor != environment.historyFloor ||
				history.Target != environment.historyTarget ||
				len(history.Events) != 3 ||
				history.FinalRead == nil ||
				history.FinalRead.Revision != environment.historyTarget.Revision ||
				environment.client.last == nil ||
				frostNativeSignerAnchorReferenceFromAcknowledgement(
					environment.client.last,
				) != environment.historyTarget {
				t.Fatal("history did not atomically publish the exact validated target")
			}
		})
	}
}

func TestFrostNativeSignerAnchorClientHistoryRejectsChangedEqualRevisionAck(t *testing.T) {
	environment := newTestFrostNativeSignerAnchorEnvironment(t, "history")
	defer environment.server.Close()
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err != nil {
		t.Fatal(err)
	}
	environment.client.last.ExactAcknowledgement = append(
		environment.client.last.ExactAcknowledgement,
		' ',
	)
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchorHistory(
		context.Background(),
		environment.historyFloor,
	); err == nil || !strings.Contains(err.Error(), "differs at an equal revision") {
		t.Fatalf("expected equal-revision acknowledgement rejection, got [%v]", err)
	}
}

func TestFrostNativeSignerAnchorClientAcceptsEmptyCompleteHistory(t *testing.T) {
	environment := newTestFrostNativeSignerAnchorEnvironment(t, "history-empty")
	defer environment.server.Close()
	history, err := environment.client.ReadFrostNativeSignerStateWitnessAnchorHistory(
		context.Background(),
		environment.historyFloor,
	)
	if err != nil {
		t.Fatal(err)
	}
	if history.Floor != environment.historyFloor ||
		history.Target != environment.historyTarget ||
		len(history.Events) != 0 ||
		history.FinalRead == nil ||
		history.FinalRead.Revision != environment.historyTarget.Revision {
		t.Fatal("empty complete history did not publish its exact target")
	}
}

func TestFrostNativeSignerAnchorClientPoisonsDivergentAmbiguousCAS(t *testing.T) {
	environment := newTestFrostNativeSignerAnchorEnvironment(t, "divergent-ambiguous")
	defer environment.server.Close()
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.client.CompareAndSwapFrostNativeSignerStateWitnessAnchor(
		context.Background(),
		environment.expected,
		environment.candidate,
		environment.proof,
	); err == nil || !strings.Contains(err.Error(), "neither the exact expected nor exact candidate") {
		t.Fatalf("expected divergent CAS poison, got [%v]", err)
	}
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err == nil || !strings.Contains(err.Error(), "poisoned") {
		t.Fatalf("expected persistent poison, got [%v]", err)
	}
}

func TestFrostNativeSignerAnchorClientRejectsSignedAbsentStream(t *testing.T) {
	environment := newTestFrostNativeSignerAnchorEnvironment(t, "absent")
	defer environment.server.Close()
	if _, err := environment.client.ReadFrostNativeSignerStateWitnessAnchor(
		context.Background(),
	); err == nil || !strings.Contains(err.Error(), "stream is absent") {
		t.Fatalf("expected signed absent stream rejection, got [%v]", err)
	}
}

type testFrostNativeSignerAnchorEnvironment struct {
	server        *httptest.Server
	client        *FrostNativeSignerAnchorClient
	expected      FrostNativeSignerStateWitnessCheckpoint
	candidate     FrostNativeSignerStateWitnessCheckpoint
	proof         []frostsigning.NativeTBTCSignerStateWitnessProofEntry
	historyFloor  FrostNativeSignerStateWitnessAnchorReference
	historyTarget FrostNativeSignerStateWitnessAnchorReference
}

func newTestFrostNativeSignerAnchorEnvironment(
	t *testing.T,
	mode string,
) *testFrostNativeSignerAnchorEnvironment {
	t.Helper()
	now := time.UnixMilli(1700000000000)
	clientPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	onlinePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	clientSPKI, err := x509.MarshalPKIXPublicKey(clientPrivate.Public())
	if err != nil {
		t.Fatal(err)
	}
	onlineSPKI, err := x509.MarshalPKIXPublicKey(onlinePrivate.Public())
	if err != nil {
		t.Fatal(err)
	}

	storeFingerprint := testFrostNativeSignerAnchorBytes32(0x51)
	expected := testFrostNativeSignerAnchorCheckpoint(
		storeFingerprint,
		1,
		testFrostNativeSignerAnchorBytes32(0x52),
		0x53,
	)
	candidate := testFrostNativeSignerAnchorCheckpoint(
		storeFingerprint,
		2,
		expected.StateCommitment,
		0x54,
	)
	divergent := testFrostNativeSignerAnchorCheckpoint(
		storeFingerprint,
		2,
		expected.StateCommitment,
		0x55,
	)
	proof := []frostsigning.NativeTBTCSignerStateWitnessProofEntry{{
		Generation:              candidate.Generation,
		PreviousStateCommitment: candidate.PreviousStateCommitment,
		StateImageDigest:        candidate.StateImageDigest,
		StateCommitment:         candidate.StateCommitment,
	}}

	var identity FrostNativeSignerAnchorIdentity
	var currentJSON []byte
	var current *FrostNativeSignerCheckpointAcknowledgement
	var historyFloor FrostNativeSignerStateWitnessAnchorReference
	var historyTarget FrostNativeSignerStateWitnessAnchorReference
	var historyEvents []FrostNativeSignerStateWitnessAnchorHistoryEvent
	advanceCalls := 0
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/anchor/read":
			payload, _ := io.ReadAll(request.Body)
			readRequest := frostNativeSignerAnchorReadRequest{}
			if err := decodeStrictFrostNativeSignerAnchorJSON(payload, &readRequest); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			nonce, err := frostNativeSignerAnchorParseHex32(readRequest.Payload.Nonce)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			transcript := frostNativeSignerAnchorReadRequestTranscript(
				identity,
				nonce,
				clientSPKI,
			)
			requestSignature, err := frostNativeSignerAnchorParseSignature(readRequest.Signature)
			if err != nil ||
				!ed25519.Verify(clientPrivate.Public().(ed25519.PublicKey), transcript, requestSignature[:]) {
				http.Error(writer, "invalid signature", http.StatusUnauthorized)
				return
			}
			requestDigest := sha256.Sum256(transcript)
			if mode == "absent" {
				response := testFrostNativeSignerAnchorAbsentReadResponse(
					t,
					identity,
					requestDigest,
					nonce,
					onlinePrivate,
				)
				_, _ = writer.Write(response)
				return
			}
			response := testFrostNativeSignerAnchorReadResponse(
				t,
				identity,
				requestDigest,
				nonce,
				current,
				currentJSON,
				onlinePrivate,
			)
			_, _ = writer.Write(response)
		case "/anchor/advance":
			payload, _ := io.ReadAll(request.Body)
			advanceRequest := frostNativeSignerAnchorCASRequest{}
			if err := decodeStrictFrostNativeSignerAnchorJSON(payload, &advanceRequest); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			nonce, _ := frostNativeSignerAnchorParseHex32(advanceRequest.Payload.Nonce)
			operationID, _ := frostNativeSignerAnchorParseHex32(
				advanceRequest.Payload.OperationID,
			)
			transitionDigest, _ := frostNativeSignerAnchorParseHex32(
				advanceRequest.Payload.TransitionDigest,
			)
			transcript := frostNativeSignerAnchorCASRequestTranscript(
				identity,
				nonce,
				operationID,
				transitionDigest,
				expected,
				candidate,
				proof,
				clientSPKI,
			)
			requestDigest := sha256.Sum256(transcript)
			advanceCalls++
			shouldApply := mode != "expected-ambiguous" || advanceCalls > 1
			target := candidate
			if mode == "divergent-ambiguous" {
				target = divergent
				shouldApply = true
			}
			if shouldApply {
				current, currentJSON = testFrostNativeSignerAnchorAcknowledgement(
					t,
					identity,
					target,
					operationID,
					transitionDigest,
					requestDigest,
					nonce,
					"applied",
					current.ServiceEpoch,
					current.Revision+1,
					current.EventRoot,
					now,
					onlinePrivate,
				)
			}
			if advanceCalls == 1 && mode != "normal" {
				http.Error(writer, "response lost", http.StatusInternalServerError)
				return
			}
			_, _ = writer.Write(currentJSON)
		case "/anchor/history":
			payload, _ := io.ReadAll(request.Body)
			historyRequest := frostNativeSignerAnchorHistoryRequest{}
			if err := decodeStrictFrostNativeSignerAnchorJSON(payload, &historyRequest); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			nonce, _ := frostNativeSignerAnchorParseHex32(historyRequest.Payload.Nonce)
			startRevision, _ := frostNativeSignerAnchorParseUint64(
				historyRequest.Payload.StartRevision,
			)
			maximumEvents, _ := frostNativeSignerAnchorParseUint64(
				historyRequest.Payload.MaximumEvents,
			)
			maximumProofEntries, _ := frostNativeSignerAnchorParseUint64(
				historyRequest.Payload.MaximumProofEntries,
			)
			transcript := frostNativeSignerAnchorHistoryRequestTranscript(
				identity,
				nonce,
				historyFloor,
				historyTarget,
				startRevision,
				maximumEvents,
				maximumProofEntries,
				clientSPKI,
			)
			requestSignature, err := frostNativeSignerAnchorParseSignature(
				historyRequest.Signature,
			)
			if err != nil || !ed25519.Verify(
				clientPrivate.Public().(ed25519.PublicKey),
				transcript,
				requestSignature[:],
			) {
				http.Error(writer, "invalid signature", http.StatusUnauthorized)
				return
			}
			if historyFloor == historyTarget {
				response := testFrostNativeSignerAnchorHistoryResponse(
					t,
					identity,
					sha256.Sum256(transcript),
					nonce,
					historyFloor,
					historyTarget,
					startRevision,
					nil,
					now,
					onlinePrivate,
				)
				_, _ = writer.Write(response)
				return
			}
			startIndex := int(startRevision - historyFloor.Revision - 1)
			if startIndex < 0 || startIndex >= len(historyEvents) {
				http.Error(writer, "invalid history cursor", http.StatusBadRequest)
				return
			}
			endIndex := len(historyEvents)
			if startIndex == 0 {
				// Exercise the boundary where the next partial-page cursor is
				// exactly the target revision.
				endIndex = 2
			}
			pageEvents := historyEvents[startIndex:endIndex]
			response := testFrostNativeSignerAnchorHistoryResponse(
				t,
				identity,
				sha256.Sum256(transcript),
				nonce,
				historyFloor,
				historyTarget,
				startRevision,
				pageEvents,
				now,
				onlinePrivate,
			)
			_, _ = writer.Write(response)
		default:
			http.NotFound(writer, request)
		}
	})
	server := httptest.NewServer(handler)
	endpoint := server.URL + "/anchor"
	identity = FrostNativeSignerAnchorIdentity{
		ProtocolID:                      testFrostNativeSignerAnchorBytes32(0x61),
		ActivationManifestHash:          testFrostNativeSignerAnchorBytes32(0x62),
		ActivationManifestSequence:      1,
		TrustDomainID:                   "test.example",
		OnlineKeyHash:                   sha256.Sum256(onlineSPKI),
		OperatorFingerprint:             testFrostNativeSignerAnchorBytes32(0x63),
		HistoryStoreID:                  "history-store-1",
		HistoryStoreFingerprint:         testFrostNativeSignerAnchorBytes32(0x64),
		HistoryClusterFingerprint:       testFrostNativeSignerAnchorBytes32(0x65),
		OfflineAuthorityHash:            testFrostNativeSignerAnchorBytes32(0x66),
		ClientSPKIHash:                  sha256.Sum256(clientSPKI),
		SignerStoreFingerprint:          storeFingerprint,
		TransportBinding:                ComputeFrostNativeSignerAnchorTransportBinding(endpoint),
		WitnessMaximumRecords:           1000,
		WitnessRotationThresholdRecords: 900,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	current, currentJSON = testFrostNativeSignerAnchorAcknowledgement(
		t,
		identity,
		expected,
		testFrostNativeSignerAnchorBytes32(0x71),
		testFrostNativeSignerAnchorBytes32(0x72),
		testFrostNativeSignerAnchorBytes32(0x73),
		testFrostNativeSignerAnchorBytes32(0x74),
		"applied",
		1,
		1,
		[32]byte{},
		now,
		onlinePrivate,
	)
	if mode == "history-empty" {
		historyFloor = frostNativeSignerAnchorReferenceFromAcknowledgement(current)
		historyTarget = historyFloor
	}
	if mode == "history" {
		historyFloor = frostNativeSignerAnchorReferenceFromAcknowledgement(current)
		third := testFrostNativeSignerAnchorCheckpoint(
			storeFingerprint,
			3,
			candidate.StateCommitment,
			0x56,
		)
		fourth := testFrostNativeSignerAnchorCheckpoint(
			storeFingerprint,
			4,
			third.StateCommitment,
			0x57,
		)
		checkpoints := []FrostNativeSignerStateWitnessCheckpoint{
			candidate,
			third,
			fourth,
		}
		priorCheckpoint := expected
		for index, checkpoint := range checkpoints {
			eventProof := []frostsigning.NativeTBTCSignerStateWitnessProofEntry{{
				Generation:              checkpoint.Generation,
				PreviousStateCommitment: checkpoint.PreviousStateCommitment,
				StateImageDigest:        checkpoint.StateImageDigest,
				StateCommitment:         checkpoint.StateCommitment,
			}}
			operationID := testFrostNativeSignerAnchorBytes32(byte(0x81 + index))
			transitionDigest := computeFrostNativeSignerAnchorTransitionDigest(
				identity,
				operationID,
				priorCheckpoint,
				checkpoint,
				eventProof,
			)
			current, currentJSON = testFrostNativeSignerAnchorAcknowledgement(
				t,
				identity,
				checkpoint,
				operationID,
				transitionDigest,
				testFrostNativeSignerAnchorBytes32(byte(0x91+index)),
				testFrostNativeSignerAnchorBytes32(byte(0xa1+index)),
				"applied",
				1,
				uint64(index+2),
				current.EventRoot,
				now,
				onlinePrivate,
			)
			historyEvents = append(
				historyEvents,
				FrostNativeSignerStateWitnessAnchorHistoryEvent{
					Acknowledgement: *current,
					WitnessProof:    eventProof,
				},
			)
			priorCheckpoint = checkpoint
		}
		historyTarget = frostNativeSignerAnchorReferenceFromAcknowledgement(current)
	}
	client, err := NewFrostNativeSignerAnchorClient(FrostNativeSignerAnchorClientConfig{
		Endpoint:            endpoint,
		ClientPrivateKey:    clientPrivate,
		OnlinePublicKeySPKI: onlineSPKI,
		Identity:            identity,
		Now:                 func() time.Time { return now },
	})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return &testFrostNativeSignerAnchorEnvironment{
		server:        server,
		client:        client,
		expected:      expected,
		candidate:     candidate,
		proof:         proof,
		historyFloor:  historyFloor,
		historyTarget: historyTarget,
	}
}

func testFrostNativeSignerAnchorAbsentReadResponse(
	t *testing.T,
	identity FrostNativeSignerAnchorIdentity,
	requestDigest [32]byte,
	nonce [32]byte,
	onlinePrivate ed25519.PrivateKey,
) []byte {
	t.Helper()
	response := frostNativeSignerAnchorReadResponse{
		Schema: FrostNativeSignerAnchorReadResponseSchema,
		BindingHash: frostNativeSignerAnchorHex32(
			ComputeFrostNativeSignerAnchorBindingHash(identity),
		),
		RequestDigest:       frostNativeSignerAnchorHex32(requestDigest),
		Nonce:               frostNativeSignerAnchorHex32(nonce),
		Status:              "absent",
		ServiceEpoch:        "0",
		Revision:            "0",
		EventRoot:           frostNativeSignerAnchorHex32([32]byte{}),
		Checkpoint:          nil,
		OperationID:         frostNativeSignerAnchorHex32([32]byte{}),
		TransitionDigest:    frostNativeSignerAnchorHex32([32]byte{}),
		CommittedAtUnixMs:   "0",
		ExpiresAtUnixMs:     "0",
		CheckpointAck:       json.RawMessage("null"),
		CheckpointAckDigest: frostNativeSignerAnchorHex32([32]byte{}),
	}
	digest, err := frostNativeSignerAnchorReadResponseTranscript(response)
	if err != nil {
		t.Fatal(err)
	}
	response.Signature = frostNativeSignerAnchorSignatureHex(
		ed25519.Sign(onlinePrivate, digest),
	)
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testFrostNativeSignerAnchorAcknowledgement(
	t *testing.T,
	identity FrostNativeSignerAnchorIdentity,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
	operationID [32]byte,
	transitionDigest [32]byte,
	requestDigest [32]byte,
	nonce [32]byte,
	status string,
	serviceEpoch uint64,
	revision uint64,
	previousEventRoot [32]byte,
	now time.Time,
	onlinePrivate ed25519.PrivateKey,
) (*FrostNativeSignerCheckpointAcknowledgement, []byte) {
	t.Helper()
	acknowledgement := FrostNativeSignerCheckpointAcknowledgement{
		BindingHash:       ComputeFrostNativeSignerAnchorBindingHash(identity),
		RequestDigest:     requestDigest,
		Nonce:             nonce,
		Status:            status,
		ServiceEpoch:      serviceEpoch,
		Revision:          revision,
		PreviousEventRoot: previousEventRoot,
		Checkpoint:        checkpoint,
		OperationID:       operationID,
		TransitionDigest:  transitionDigest,
		CommittedAtUnixMs: uint64(now.Add(-time.Second).UnixMilli()),
		ExpiresAtUnixMs:   uint64(now.Add(29 * time.Second).UnixMilli()),
	}
	acknowledgement.EventRoot = computeFrostNativeSignerAnchorEventRoot(acknowledgement)
	wire := frostNativeSignerAnchorAcknowledgementWire{
		Schema:            FrostNativeSignerCheckpointAcknowledgementSchema,
		BindingHash:       frostNativeSignerAnchorHex32(acknowledgement.BindingHash),
		RequestDigest:     frostNativeSignerAnchorHex32(requestDigest),
		Nonce:             frostNativeSignerAnchorHex32(nonce),
		Status:            status,
		ServiceEpoch:      fmt.Sprint(serviceEpoch),
		Revision:          fmt.Sprint(revision),
		PreviousEventRoot: frostNativeSignerAnchorHex32(previousEventRoot),
		EventRoot:         frostNativeSignerAnchorHex32(acknowledgement.EventRoot),
		Checkpoint:        frostNativeSignerAnchorCheckpointToWire(checkpoint),
		OperationID:       frostNativeSignerAnchorHex32(operationID),
		TransitionDigest:  frostNativeSignerAnchorHex32(transitionDigest),
		CommittedAtUnixMs: fmt.Sprint(acknowledgement.CommittedAtUnixMs),
		ExpiresAtUnixMs:   fmt.Sprint(acknowledgement.ExpiresAtUnixMs),
	}
	signingDigest, err := frostNativeSignerAnchorAcknowledgementTranscript(wire)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(onlinePrivate, signingDigest)
	wire.Signature = frostNativeSignerAnchorSignatureHex(signature)
	copy(acknowledgement.SigningDigest[:], signingDigest)
	copy(acknowledgement.Signature[:], signature)
	onlineSPKI, _ := x509.MarshalPKIXPublicKey(onlinePrivate.Public())
	acknowledgement.AcknowledgementDigest =
		computeFrostNativeSignerCheckpointAcknowledgementDigest(
			acknowledgement.SigningDigest,
			acknowledgement.Signature,
			sha256.Sum256(onlineSPKI),
		)
	payload, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	acknowledgement.ExactAcknowledgement = append([]byte{}, payload...)
	return &acknowledgement, payload
}

func testFrostNativeSignerAnchorReadResponse(
	t *testing.T,
	identity FrostNativeSignerAnchorIdentity,
	requestDigest [32]byte,
	nonce [32]byte,
	acknowledgement *FrostNativeSignerCheckpointAcknowledgement,
	acknowledgementJSON []byte,
	onlinePrivate ed25519.PrivateKey,
) []byte {
	t.Helper()
	checkpointWire := frostNativeSignerAnchorCheckpointToWire(acknowledgement.Checkpoint)
	response := frostNativeSignerAnchorReadResponse{
		Schema: FrostNativeSignerAnchorReadResponseSchema,
		BindingHash: frostNativeSignerAnchorHex32(
			ComputeFrostNativeSignerAnchorBindingHash(identity),
		),
		RequestDigest:     frostNativeSignerAnchorHex32(requestDigest),
		Nonce:             frostNativeSignerAnchorHex32(nonce),
		Status:            "present",
		ServiceEpoch:      fmt.Sprint(acknowledgement.ServiceEpoch),
		Revision:          fmt.Sprint(acknowledgement.Revision),
		EventRoot:         frostNativeSignerAnchorHex32(acknowledgement.EventRoot),
		Checkpoint:        &checkpointWire,
		OperationID:       frostNativeSignerAnchorHex32(acknowledgement.OperationID),
		TransitionDigest:  frostNativeSignerAnchorHex32(acknowledgement.TransitionDigest),
		CommittedAtUnixMs: "1700000000000",
		ExpiresAtUnixMs:   "1700000030000",
		CheckpointAck:     json.RawMessage(append([]byte{}, acknowledgementJSON...)),
		CheckpointAckDigest: frostNativeSignerAnchorHex32(
			acknowledgement.AcknowledgementDigest,
		),
	}
	digest, err := frostNativeSignerAnchorReadResponseTranscript(response)
	if err != nil {
		t.Fatal(err)
	}
	response.Signature = frostNativeSignerAnchorSignatureHex(
		ed25519.Sign(onlinePrivate, digest),
	)
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testFrostNativeSignerAnchorHistoryResponse(
	t *testing.T,
	identity FrostNativeSignerAnchorIdentity,
	requestDigest [32]byte,
	nonce [32]byte,
	floor FrostNativeSignerStateWitnessAnchorReference,
	target FrostNativeSignerStateWitnessAnchorReference,
	startRevision uint64,
	events []FrostNativeSignerStateWitnessAnchorHistoryEvent,
	now time.Time,
	onlinePrivate ed25519.PrivateKey,
) []byte {
	t.Helper()
	wireEvents := make([]frostNativeSignerAnchorHistoryEventWire, len(events))
	eventDigests := make([][32]byte, len(events))
	proofEntryCount := 0
	for index, event := range events {
		rawAcknowledgement := append(
			[]byte{},
			event.Acknowledgement.ExactAcknowledgement...,
		)
		wireEvents[index] = frostNativeSignerAnchorHistoryEventWire{
			CheckpointAck: json.RawMessage(rawAcknowledgement),
			WitnessProof:  frostNativeSignerAnchorProofToWire(event.WitnessProof),
		}
		proofEntryCount += len(event.WitnessProof)
		eventDigests[index] = computeFrostNativeSignerAnchorHistoryEventDigest(
			event.Acknowledgement.Revision,
			event.Acknowledgement.AcknowledgementDigest,
			rawAcknowledgement,
			event.WitnessProof,
		)
	}
	status := "complete"
	nextRevision := uint64(0)
	if len(events) == 0 {
		if floor != target {
			t.Fatal("empty history response requires equal floor and target")
		}
	} else if frostNativeSignerAnchorReferenceFromAcknowledgement(
		&events[len(events)-1].Acknowledgement,
	) != target {
		status = "partial"
		nextRevision = events[len(events)-1].Acknowledgement.Revision + 1
	}
	response := frostNativeSignerAnchorHistoryResponse{
		Schema: FrostNativeSignerAnchorHistoryResponseSchema,
		BindingHash: frostNativeSignerAnchorHex32(
			ComputeFrostNativeSignerAnchorBindingHash(identity),
		),
		RequestDigest:     frostNativeSignerAnchorHex32(requestDigest),
		Nonce:             frostNativeSignerAnchorHex32(nonce),
		Status:            status,
		ServiceEpoch:      fmt.Sprint(target.ServiceEpoch),
		FloorRef:          frostNativeSignerAnchorHistoryReferenceToWire(floor),
		TargetRef:         frostNativeSignerAnchorHistoryReferenceToWire(target),
		StartRevision:     fmt.Sprint(startRevision),
		NextRevision:      fmt.Sprint(nextRevision),
		EventCount:        fmt.Sprint(len(events)),
		ProofEntryCount:   fmt.Sprint(proofEntryCount),
		Events:            &wireEvents,
		CommittedAtUnixMs: fmt.Sprint(now.UnixMilli()),
		ExpiresAtUnixMs:   fmt.Sprint(now.Add(30 * time.Second).UnixMilli()),
	}
	digest, err := frostNativeSignerAnchorHistoryResponseTranscript(
		response,
		eventDigests,
	)
	if err != nil {
		t.Fatal(err)
	}
	response.Signature = frostNativeSignerAnchorSignatureHex(
		ed25519.Sign(onlinePrivate, digest),
	)
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testFrostNativeSignerAnchorCheckpoint(
	storeFingerprint [32]byte,
	generation uint64,
	previousCommitment [32]byte,
	imageByte byte,
) FrostNativeSignerStateWitnessCheckpoint {
	imageDigest := testFrostNativeSignerAnchorBytes32(imageByte)
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              generation,
		PreviousStateCommitment: previousCommitment,
		StateImageDigest:        imageDigest,
		StateCommitment: frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			storeFingerprint,
			generation,
			previousCommitment,
			imageDigest,
		),
	}
}

func testFrostNativeSignerAnchorBytes32(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value
	}
	return result
}
