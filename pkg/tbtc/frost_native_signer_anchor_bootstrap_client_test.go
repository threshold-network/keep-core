package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// bootstrapClientTestEnvironment runs a fake bootstrap history service that
// verifies client request signatures, keeps at most one stored genesis
// acknowledgement, and answers both bootstrap request kinds on the single
// initialize endpoint.
type bootstrapClientTestEnvironment struct {
	t        *testing.T
	server   *httptest.Server
	endpoint string
	nowFunc  func() time.Time

	authority  ed25519.PrivateKey
	response   ed25519.PrivateKey
	clientKey  ed25519.PrivateKey
	clientSPKI []byte

	identity      FrostNativeSignerAnchorIdentity
	plan          *FrostNativeSignerAnchorBootstrapPlan
	facts         *frostsigning.NativeTBTCSignerStateAnchorBootstrapFacts
	core          *FrostNativeSignerAnchorBootstrapCoreArtifact
	coreSignature *FrostNativeSignerAnchorBootstrapDetachedSignature
	client        *FrostNativeSignerAnchorBootstrapHTTPClient

	mutex                 sync.Mutex
	stored                *FrostNativeSignerCheckpointAcknowledgement
	storedJSON            []byte
	initializeHook        func(http.ResponseWriter, [32]byte, [32]byte) bool
	readHook              func(http.ResponseWriter, [32]byte, [32]byte) bool
	garbleFirstInitialize bool
	initializeCalls       int
	readCalls             int
	totalRequests         int
}

func newBootstrapClientTestEnvironment(
	t *testing.T,
) *bootstrapClientTestEnvironment {
	fixedNow := time.UnixMilli(1_700_000_000_000)
	return newBootstrapClientTestEnvironmentWithNow(
		t,
		func() time.Time { return fixedNow },
	)
}

func newBootstrapClientTestEnvironmentWithNow(
	t *testing.T,
	nowFunc func() time.Time,
) *bootstrapClientTestEnvironment {
	t.Helper()
	environment := &bootstrapClientTestEnvironment{
		t:       t,
		nowFunc: nowFunc,
		authority: ed25519.NewKeyFromSeed(
			bytes.Repeat([]byte{0x61}, ed25519.SeedSize),
		),
		response: ed25519.NewKeyFromSeed(
			bytes.Repeat([]byte{0x62}, ed25519.SeedSize),
		),
		clientKey: ed25519.NewKeyFromSeed(
			bytes.Repeat([]byte{0x63}, ed25519.SeedSize),
		),
	}
	clientSPKI, err := x509.MarshalPKIXPublicKey(environment.clientKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	environment.clientSPKI = clientSPKI
	environment.server = httptest.NewServer(http.HandlerFunc(environment.handle))
	t.Cleanup(environment.server.Close)
	environment.endpoint = environment.server.URL + "/anchor"

	authorityPublic := trustTestRawPublicKey(environment.authority)
	responsePublic := trustTestRawPublicKey(environment.response)
	store := trustTestBytes32(0x03)
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                 trustTestBytes32(0x01),
		ActivationManifestHash:     trustTestBytes32(0x04),
		ActivationManifestSequence: 9,
		TrustDomainID:              "bootstrap-client-trust-domain",
		OnlineKeyHash: ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			responsePublic,
		),
		OperatorFingerprint:       trustTestBytes32(0x06),
		HistoryStoreID:            "bootstrap-client-history-store",
		HistoryStoreFingerprint:   trustTestBytes32(0x07),
		HistoryClusterFingerprint: trustTestBytes32(0x08),
		OfflineAuthorityHash: ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			authorityPublic,
		),
		ClientSPKIHash:         sha256.Sum256(clientSPKI),
		SignerStoreFingerprint: store,
		TransportBinding: ComputeFrostNativeSignerAnchorTransportBinding(
			environment.endpoint,
		),
		WitnessMaximumRecords:           1000,
		WitnessRotationThresholdRecords: 900,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	environment.identity = identity
	genesis := frostsigning.ComputeNativeTBTCSignerStateWitnessGenesis(store)
	image := trustTestBytes32(0x0a)
	environment.plan = &FrostNativeSignerAnchorBootstrapPlan{
		Schema:                    FrostNativeSignerAnchorBootstrapPlanSchema,
		Endpoint:                  environment.endpoint,
		Identity:                  identity,
		ResponsePublicKey:         responsePublic,
		OfflineAuthorityPublicKey: authorityPublic,
	}
	environment.facts = &frostsigning.NativeTBTCSignerStateAnchorBootstrapFacts{
		Schema:           frostsigning.NativeTBTCSignerStateAnchorBootstrapFactsSchema,
		StoreFingerprint: store,
		CurrentCheckpoint: frostsigning.NativeTBTCSignerStateAnchorCheckpoint{
			StoreFingerprint:        store,
			Generation:              1,
			PreviousStateCommitment: genesis,
			StateImageDigest:        image,
			StateCommitment: frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
				store,
				1,
				genesis,
				image,
			),
		},
	}
	core, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		environment.facts,
		environment.plan,
	)
	if err != nil {
		t.Fatalf("bootstrap client test core preparation failed: %v", err)
	}
	environment.core = core
	environment.coreSignature = bootstrapProvisioningTestDetachedSignature(
		environment.authority,
		FrostNativeSignerAnchorBootstrapCoreSignatureStage,
		core.CoreDigest,
	)
	client, err := NewFrostNativeSignerAnchorBootstrapClient(
		FrostNativeSignerAnchorBootstrapClientConfig{
			Endpoint:          environment.endpoint,
			ResponsePublicKey: responsePublic,
			ResponsePublicKeySPKISHA256: ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
				responsePublic,
			),
			ClientPrivateKey: environment.clientKey,
			Now:              environment.nowFunc,
		},
	)
	if err != nil {
		t.Fatalf("bootstrap client construction failed: %v", err)
	}
	environment.client = client
	return environment
}

func (environment *bootstrapClientTestEnvironment) authorization() FrostNativeSignerAnchorBootstrapAuthorization {
	certificate := frostNativeSignerAnchorBootstrapCoreCertificate(
		&environment.core.Plan,
		environment.core.Checkpoint,
	)
	certificate.CoreDigest = environment.core.CoreDigest
	certificate.CoreSignature = environment.coreSignature.Signature
	certificate.OperationID = environment.core.OperationID
	certificate.TransitionDigest = environment.core.TransitionDigest
	return FrostNativeSignerAnchorBootstrapAuthorization{
		Certificate: certificate,
	}
}

func (environment *bootstrapClientTestEnvironment) handle(
	writer http.ResponseWriter,
	request *http.Request,
) {
	environment.mutex.Lock()
	defer environment.mutex.Unlock()
	environment.totalRequests++
	if request.URL.Path != "/anchor/initialize" {
		http.NotFound(writer, request)
		return
	}
	payload, _ := io.ReadAll(request.Body)
	decoded := frostNativeSignerAnchorInitializeRequest{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(payload, &decoded); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if decoded.Schema != FrostNativeSignerAnchorInitializeRequestSchema {
		http.Error(writer, "unsupported schema", http.StatusBadRequest)
		return
	}
	nonce, err := frostNativeSignerAnchorParseHex32(decoded.Payload.Nonce)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	bindingHash, err := frostNativeSignerAnchorParseHex32(
		decoded.Payload.BindingHash,
	)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	operationID, err := frostNativeSignerAnchorParseHex32(
		decoded.Payload.OperationID,
	)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	transitionDigest, err := frostNativeSignerAnchorParseHex32(
		decoded.Payload.TransitionDigest,
	)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	checkpoint, err := frostNativeSignerAnchorCheckpointFromWire(
		decoded.Payload.Checkpoint,
	)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	transcript := frostNativeSignerAnchorInitializeRequestTranscript(
		decoded.Payload.Kind,
		bindingHash,
		nonce,
		operationID,
		transitionDigest,
		checkpoint,
		environment.clientSPKI,
	)
	signature, err := frostNativeSignerAnchorParseSignature(decoded.Signature)
	if err != nil || !ed25519.Verify(
		environment.clientKey.Public().(ed25519.PublicKey),
		transcript,
		signature[:],
	) {
		http.Error(writer, "invalid request signature", http.StatusUnauthorized)
		return
	}
	requestDigest := sha256.Sum256(transcript)
	writer.Header().Set("Content-Type", "application/json")
	switch decoded.Payload.Kind {
	case "initialize":
		environment.initializeCalls++
		if environment.initializeHook != nil &&
			environment.initializeHook(writer, requestDigest, nonce) {
			return
		}
		if environment.stored == nil {
			environment.stored, environment.storedJSON =
				testFrostNativeSignerAnchorAcknowledgement(
					environment.t,
					environment.identity,
					checkpoint,
					operationID,
					transitionDigest,
					requestDigest,
					nonce,
					"applied",
					1,
					1,
					[32]byte{},
					environment.nowFunc(),
					environment.response,
				)
			if environment.garbleFirstInitialize &&
				environment.initializeCalls == 1 {
				_, _ = writer.Write([]byte(`{"garbage"`))
				return
			}
			_, _ = writer.Write(environment.storedJSON)
			return
		}
		if environment.stored.OperationID != operationID {
			http.Error(
				writer,
				"conflicting native signer anchor stream",
				http.StatusConflict,
			)
			return
		}
		_, sentinelJSON := testFrostNativeSignerAnchorAcknowledgement(
			environment.t,
			environment.identity,
			environment.stored.Checkpoint,
			environment.stored.OperationID,
			environment.stored.TransitionDigest,
			requestDigest,
			nonce,
			"already-applied",
			1,
			1,
			[32]byte{},
			environment.nowFunc(),
			environment.response,
		)
		_, _ = writer.Write(sentinelJSON)
	case "read":
		environment.readCalls++
		if environment.readHook != nil &&
			environment.readHook(writer, requestDigest, nonce) {
			return
		}
		if environment.stored == nil {
			_, _ = writer.Write(testFrostNativeSignerAnchorAbsentReadResponse(
				environment.t,
				environment.identity,
				requestDigest,
				nonce,
				environment.response,
			))
			return
		}
		_, _ = writer.Write(bootstrapClientTestReadResponse(
			environment.t,
			environment.identity,
			requestDigest,
			nonce,
			environment.stored,
			environment.storedJSON,
			environment.nowFunc(),
			environment.response,
		))
	default:
		http.Error(writer, "unsupported bootstrap request kind", http.StatusBadRequest)
	}
}

func (environment *bootstrapClientTestEnvironment) setInitializeHook(
	hook func(http.ResponseWriter, [32]byte, [32]byte) bool,
) {
	environment.mutex.Lock()
	defer environment.mutex.Unlock()
	environment.initializeHook = hook
}

func (environment *bootstrapClientTestEnvironment) setReadHook(
	hook func(http.ResponseWriter, [32]byte, [32]byte) bool,
) {
	environment.mutex.Lock()
	defer environment.mutex.Unlock()
	environment.readHook = hook
}

func (environment *bootstrapClientTestEnvironment) setGarbleFirstInitialize() {
	environment.mutex.Lock()
	defer environment.mutex.Unlock()
	environment.garbleFirstInitialize = true
}

func (environment *bootstrapClientTestEnvironment) setStoredForeignRecord() {
	environment.mutex.Lock()
	defer environment.mutex.Unlock()
	environment.stored, environment.storedJSON =
		testFrostNativeSignerAnchorAcknowledgement(
			environment.t,
			environment.identity,
			environment.core.Checkpoint,
			trustTestBytes32(0xee),
			trustTestBytes32(0xef),
			trustTestBytes32(0xe1),
			trustTestBytes32(0xe2),
			"applied",
			1,
			1,
			[32]byte{},
			environment.nowFunc(),
			environment.response,
		)
}

func (environment *bootstrapClientTestEnvironment) counters() (int, int, int) {
	environment.mutex.Lock()
	defer environment.mutex.Unlock()
	return environment.initializeCalls, environment.readCalls, environment.totalRequests
}

func (environment *bootstrapClientTestEnvironment) boundAcknowledgementJSON(
	requestDigest [32]byte,
	nonce [32]byte,
	signingKey ed25519.PrivateKey,
) []byte {
	_, acknowledgementJSON := testFrostNativeSignerAnchorAcknowledgement(
		environment.t,
		environment.identity,
		environment.core.Checkpoint,
		environment.core.OperationID,
		environment.core.TransitionDigest,
		requestDigest,
		nonce,
		"applied",
		1,
		1,
		[32]byte{},
		environment.nowFunc(),
		signingKey,
	)
	return acknowledgementJSON
}

// bootstrapClientTestReadResponse mirrors testFrostNativeSignerAnchorReadResponse
// with caller-controlled wrapper times so real-clock loader tests stay fresh.
func bootstrapClientTestReadResponse(
	t *testing.T,
	identity FrostNativeSignerAnchorIdentity,
	requestDigest [32]byte,
	nonce [32]byte,
	acknowledgement *FrostNativeSignerCheckpointAcknowledgement,
	acknowledgementJSON []byte,
	now time.Time,
	onlinePrivate ed25519.PrivateKey,
) []byte {
	t.Helper()
	checkpointWire := frostNativeSignerAnchorCheckpointToWire(
		acknowledgement.Checkpoint,
	)
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
		CommittedAtUnixMs: fmt.Sprint(now.Add(-time.Second).UnixMilli()),
		ExpiresAtUnixMs:   fmt.Sprint(now.Add(29 * time.Second).UnixMilli()),
		CheckpointAck:     append([]byte{}, acknowledgementJSON...),
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

func TestFrostNativeSignerAnchorBootstrapClientAppliedEndToEnd(t *testing.T) {
	environment := newBootstrapClientTestEnvironment(t)
	final, err := InitializeFrostNativeSignerAnchorBootstrap(
		context.Background(),
		environment.core,
		environment.coreSignature,
		environment.client,
	)
	if err != nil {
		t.Fatalf("bootstrap initialize over the HTTP client failed: %v", err)
	}
	initializeCalls, readCalls, _ := environment.counters()
	if initializeCalls != 1 || readCalls != 1 {
		t.Fatalf(
			"expected one initialize and one reconciliation read, got [%d]/[%d]",
			initializeCalls,
			readCalls,
		)
	}
	if final.TargetReference.ServiceEpoch != 1 ||
		final.TargetReference.Revision != 1 ||
		final.TargetReference.PreviousEventRoot != [32]byte{} ||
		final.TargetReference.Checkpoint != environment.core.Checkpoint {
		t.Fatalf("unexpected bootstrap final target reference: %+v", final)
	}
	finalSignature := bootstrapProvisioningTestDetachedSignature(
		environment.authority,
		FrostNativeSignerAnchorBootstrapFinalSignatureStage,
		final.FinalDigest,
	)
	bundle, err := FinalizeFrostNativeSignerAnchorBootstrap(
		final,
		finalSignature,
		bootstrapProvisioningTestBaseConfig(),
	)
	if err != nil {
		t.Fatalf("bootstrap finalize after the HTTP client failed: %v", err)
	}
	if _, err := DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
		bundle,
	); err != nil {
		t.Fatalf("certified bundle from the HTTP client run is invalid: %v", err)
	}
}

func TestFrostNativeSignerAnchorBootstrapClientRecoversCommittedAmbiguousInitialize(
	t *testing.T,
) {
	environment := newBootstrapClientTestEnvironment(t)
	environment.setGarbleFirstInitialize()

	first, err := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	)
	if err != nil {
		t.Fatalf("committed-but-ambiguous initialize did not reconcile: %v", err)
	}
	initializeCalls, readCalls, _ := environment.counters()
	if initializeCalls != 1 || readCalls != 1 {
		t.Fatalf(
			"expected reconciliation through one read, got [%d]/[%d]",
			initializeCalls,
			readCalls,
		)
	}
	second, err := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	)
	if err != nil {
		t.Fatalf("idempotent already-applied retry failed: %v", err)
	}
	// The exact read wrapper is nonce-fresh by design, so the results must be
	// identical everywhere except the retained read-recovery JSON.
	firstRecord := *first.Record
	secondRecord := *second.Record
	if len(firstRecord.ReadRecoveryJSON) == 0 ||
		len(secondRecord.ReadRecoveryJSON) == 0 ||
		firstRecord.ReadRecoveryExpires == 0 ||
		secondRecord.ReadRecoveryExpires == 0 {
		t.Fatal("bootstrap results did not retain fresh exact-read recovery")
	}
	firstRecord.ReadRecoveryJSON = nil
	secondRecord.ReadRecoveryJSON = nil
	if !reflect.DeepEqual(firstRecord, secondRecord) {
		t.Fatalf(
			"recovered and already-applied results differ:\n%+v\n%+v",
			firstRecord,
			secondRecord,
		)
	}
	if !bytes.Equal(first.Record.AcknowledgementJSON, second.Record.AcknowledgementJSON) {
		t.Fatal("stored genesis acknowledgement bytes changed between calls")
	}
	// The reconciled result must still satisfy the full offline validation.
	if _, err := InitializeFrostNativeSignerAnchorBootstrap(
		context.Background(),
		environment.core,
		environment.coreSignature,
		environment.client,
	); err != nil {
		t.Fatalf("reconciled record failed the offline core validation: %v", err)
	}
}

func TestFrostNativeSignerAnchorBootstrapClientPoisonsDivergentGenesisRecord(
	t *testing.T,
) {
	environment := newBootstrapClientTestEnvironment(t)
	environment.setStoredForeignRecord()

	first, err := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	)
	if err == nil || first != nil ||
		!strings.Contains(err.Error(), "poisoned") ||
		!strings.Contains(err.Error(), "different genesis record") {
		t.Fatalf("expected divergence poisoning, got [%v]", err)
	}
	_, _, requestsAfterPoison := environment.counters()
	for attempt := 0; attempt < 2; attempt++ {
		repeat, repeatErr := environment.client.InitializeFrostNativeSignerAnchor(
			context.Background(),
			environment.authorization(),
		)
		if repeatErr == nil || repeat != nil ||
			repeatErr.Error() != err.Error() {
			t.Fatalf(
				"poisoned client returned a different result: [%v]",
				repeatErr,
			)
		}
	}
	if _, _, requests := environment.counters(); requests != requestsAfterPoison {
		t.Fatalf(
			"poisoned client touched the network: [%d] != [%d]",
			requests,
			requestsAfterPoison,
		)
	}
}

func TestFrostNativeSignerAnchorBootstrapClientPoisonsEquivocatingAbsentRead(
	t *testing.T,
) {
	environment := newBootstrapClientTestEnvironment(t)
	environment.setReadHook(func(
		writer http.ResponseWriter,
		requestDigest [32]byte,
		nonce [32]byte,
	) bool {
		_, _ = writer.Write(testFrostNativeSignerAnchorAbsentReadResponse(
			environment.t,
			environment.identity,
			requestDigest,
			nonce,
			environment.response,
		))
		return true
	})
	_, err := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	)
	if err == nil || !strings.Contains(err.Error(), "poisoned") ||
		!strings.Contains(err.Error(), "absent stream") {
		t.Fatalf("expected equivocation poisoning, got [%v]", err)
	}
	_, _, requests := environment.counters()
	if _, repeatErr := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	); repeatErr == nil || repeatErr.Error() != err.Error() {
		t.Fatalf("expected persistent poison, got [%v]", repeatErr)
	}
	if _, _, after := environment.counters(); after != requests {
		t.Fatal("poisoned client touched the network")
	}
}

func TestFrostNativeSignerAnchorBootstrapClientDoesNotPoisonUnreachableService(
	t *testing.T,
) {
	environment := newBootstrapClientTestEnvironment(t)
	environment.server.Close()
	_, err := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	)
	if err == nil || strings.Contains(err.Error(), "poisoned") ||
		!strings.Contains(err.Error(), "request failed") {
		t.Fatalf("expected retryable transport failure, got [%v]", err)
	}
	_, secondErr := environment.client.InitializeFrostNativeSignerAnchor(
		context.Background(),
		environment.authorization(),
	)
	if secondErr == nil || strings.Contains(secondErr.Error(), "poisoned") {
		t.Fatalf("transport failure poisoned the client: [%v]", secondErr)
	}
}

func TestFrostNativeSignerAnchorBootstrapClientInitializeResponseStrictness(
	t *testing.T,
) {
	oversized := bytes.Repeat(
		[]byte{'a'},
		frostNativeSignerAnchorMaximumResponseBytes+1,
	)
	tests := map[string]func(
		*bootstrapClientTestEnvironment,
		http.ResponseWriter,
		[32]byte,
		[32]byte,
	){
		"wrong acknowledgement schema": func(
			environment *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			requestDigest [32]byte,
			nonce [32]byte,
		) {
			acknowledgement := environment.boundAcknowledgementJSON(
				requestDigest,
				nonce,
				environment.response,
			)
			_, _ = writer.Write(bytes.Replace(
				acknowledgement,
				[]byte("tbtc-signer-state-witness-checkpoint-ack/v1"),
				[]byte("tbtc-signer-state-witness-checkpoint-ack/v2"),
				1,
			))
		},
		"wrong acknowledgement status": func(
			environment *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			requestDigest [32]byte,
			nonce [32]byte,
		) {
			acknowledgement := environment.boundAcknowledgementJSON(
				requestDigest,
				nonce,
				environment.response,
			)
			_, _ = writer.Write(bytes.Replace(
				acknowledgement,
				[]byte(`"status":"applied"`),
				[]byte(`"status":"rejected"`),
				1,
			))
		},
		"tampered acknowledgement signature": func(
			environment *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			requestDigest [32]byte,
			nonce [32]byte,
		) {
			_, _ = writer.Write(environment.boundAcknowledgementJSON(
				requestDigest,
				nonce,
				environment.authority,
			))
		},
		"oversized response": func(
			_ *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			_ [32]byte,
			_ [32]byte,
		) {
			_, _ = writer.Write(oversized)
		},
		"wrong content type": func(
			environment *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			requestDigest [32]byte,
			nonce [32]byte,
		) {
			writer.Header().Set("Content-Type", "text/plain")
			_, _ = writer.Write(environment.boundAcknowledgementJSON(
				requestDigest,
				nonce,
				environment.response,
			))
		},
		"non-200 status": func(
			_ *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			_ [32]byte,
			_ [32]byte,
		) {
			http.Error(writer, "service failure", http.StatusInternalServerError)
		},
		"trailing data": func(
			environment *bootstrapClientTestEnvironment,
			writer http.ResponseWriter,
			requestDigest [32]byte,
			nonce [32]byte,
		) {
			acknowledgement := environment.boundAcknowledgementJSON(
				requestDigest,
				nonce,
				environment.response,
			)
			_, _ = writer.Write(append(acknowledgement, []byte("{}")...))
		},
	}
	for name, respond := range tests {
		t.Run(name, func(t *testing.T) {
			environment := newBootstrapClientTestEnvironment(t)
			environment.setInitializeHook(func(
				writer http.ResponseWriter,
				requestDigest [32]byte,
				nonce [32]byte,
			) bool {
				respond(environment, writer, requestDigest, nonce)
				return true
			})
			_, err := environment.client.InitializeFrostNativeSignerAnchor(
				context.Background(),
				environment.authorization(),
			)
			if err == nil || strings.Contains(err.Error(), "poisoned") ||
				!strings.Contains(err.Error(), "did not commit") {
				t.Fatalf(
					"expected an ambiguous, retryable rejection, got [%v]",
					err,
				)
			}
			_, readCalls, _ := environment.counters()
			if readCalls != 1 {
				t.Fatalf(
					"expected one reconciliation read after the bad response, got [%d]",
					readCalls,
				)
			}
			environment.setInitializeHook(nil)
			if _, err := environment.client.InitializeFrostNativeSignerAnchor(
				context.Background(),
				environment.authorization(),
			); err != nil {
				t.Fatalf("retry after a rejected response failed: %v", err)
			}
		})
	}
}

func bootstrapClientTestConfigJSON(
	endpoint string,
	responseKeyHex string,
	responsePinHex string,
	leafHex string,
	keyPath string,
	timeout string,
) string {
	return `{"schema":"` + FrostNativeSignerAnchorBootstrapClientConfigSchema +
		`","endpoint":"` + endpoint +
		`","responsePublicKey":"` + responseKeyHex +
		`","responsePublicKeySpkiSha256":"` + responsePinHex +
		`","endpointLeafSpkiHash":"` + leafHex +
		`","clientPrivateKeyPath":"` + keyPath +
		`","requestTimeoutMilliseconds":"` + timeout + `"}`
}

func TestFrostNativeSignerAnchorBootstrapClientConfigDecodeStrictness(
	t *testing.T,
) {
	response := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x62}, ed25519.SeedSize),
	)
	responsePublic := trustTestRawPublicKey(response)
	responseHex := frostNativeSignerAnchorHex32(responsePublic)
	pinHex := frostNativeSignerAnchorHex32(
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(responsePublic),
	)
	zeroHex := frostNativeSignerAnchorHex32([32]byte{})
	leafHex := frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
	keyPath := "/var/lib/keep/bootstrap-client-key.pem"
	valid := bootstrapClientTestConfigJSON(
		"http://127.0.0.1:9799/anchor",
		responseHex,
		pinHex,
		zeroHex,
		keyPath,
		"3000",
	)
	config, err := DecodeFrostNativeSignerAnchorBootstrapClientConfig(
		[]byte(valid),
	)
	if err != nil {
		t.Fatalf("canonical bootstrap client config was rejected: %v", err)
	}
	if config.Endpoint != "http://127.0.0.1:9799/anchor" ||
		config.ResponsePublicKey != responsePublic ||
		config.ClientPrivateKeyPath != keyPath ||
		config.RequestTimeout != 3*time.Second {
		t.Fatalf("canonical bootstrap client config decoded incorrectly: %+v", config)
	}
	httpsValid := bootstrapClientTestConfigJSON(
		"https://anchor.example/anchor",
		responseHex,
		pinHex,
		leafHex,
		keyPath,
		"3000",
	)
	if _, err := DecodeFrostNativeSignerAnchorBootstrapClientConfig(
		[]byte(httpsValid),
	); err != nil {
		t.Fatalf("canonical HTTPS bootstrap client config was rejected: %v", err)
	}

	invalid := map[string]string{
		"wrong schema": strings.Replace(
			valid,
			FrostNativeSignerAnchorBootstrapClientConfigSchema,
			"tbtc-frost-native-signer-anchor-bootstrap-client-config/v2",
			1,
		),
		"non-numeric loopback endpoint": bootstrapClientTestConfigJSON(
			"http://localhost:9799/anchor", responseHex, pinHex, zeroHex, keyPath, "3000",
		),
		"plaintext non-loopback endpoint": bootstrapClientTestConfigJSON(
			"http://10.0.0.1:9799/anchor", responseHex, pinHex, zeroHex, keyPath, "3000",
		),
		"loopback endpoint without a fixed port": bootstrapClientTestConfigJSON(
			"http://127.0.0.1/anchor", responseHex, pinHex, zeroHex, keyPath, "3000",
		),
		"uppercase endpoint host": bootstrapClientTestConfigJSON(
			"https://ANCHOR.example/anchor", responseHex, pinHex, leafHex, keyPath, "3000",
		),
		"endpoint trailing slash": bootstrapClientTestConfigJSON(
			"https://anchor.example/anchor/", responseHex, pinHex, leafHex, keyPath, "3000",
		),
		"endpoint query": bootstrapClientTestConfigJSON(
			"https://anchor.example/anchor?x=1", responseHex, pinHex, leafHex, keyPath, "3000",
		),
		"HTTPS without a leaf SPKI pin": bootstrapClientTestConfigJSON(
			"https://anchor.example/anchor", responseHex, pinHex, zeroHex, keyPath, "3000",
		),
		"loopback HTTP with a leaf SPKI pin": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex, pinHex, leafHex, keyPath, "3000",
		),
		"zero response key": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", zeroHex, pinHex, zeroHex, keyPath, "3000",
		),
		"response key SPKI pin mismatch": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex,
			frostNativeSignerAnchorHex32(trustTestBytes32(0x0d)),
			zeroHex, keyPath, "3000",
		),
		"relative client key path": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex, pinHex, zeroHex,
			"relative/key.pem", "3000",
		),
		"empty client key path": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex, pinHex, zeroHex, "", "3000",
		),
		"zero request timeout": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex, pinHex, zeroHex, keyPath, "0",
		),
		"non-canonical request timeout": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex, pinHex, zeroHex, keyPath, "0300",
		),
		"request timeout above the bound": bootstrapClientTestConfigJSON(
			"http://127.0.0.1:9799/anchor", responseHex, pinHex, zeroHex, keyPath, "30001",
		),
		"bare JSON number timeout": strings.Replace(
			valid, `"requestTimeoutMilliseconds":"3000"`,
			`"requestTimeoutMilliseconds":3000`, 1,
		),
		"duplicate member": strings.Replace(
			valid, `"endpoint":`,
			`"endpoint":"http://127.0.0.1:1/anchor","endpoint":`, 1,
		),
		"case-folded duplicate member": strings.Replace(
			valid, `"endpoint":`, `"Endpoint":"x","endpoint":`, 1,
		),
		"unknown member": strings.Replace(
			valid, `{"schema"`, `{"extra":"x","schema"`, 1,
		),
		"trailing data": valid + "{}",
		"empty config":  "",
	}
	for name, payload := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFrostNativeSignerAnchorBootstrapClientConfig(
				[]byte(payload),
			); err == nil {
				t.Fatal("expected canonical bootstrap client config rejection")
			}
		})
	}
}

func TestFrostNativeSignerAnchorBootstrapClientConstructorRejectsInvalidMaterial(
	t *testing.T,
) {
	response := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x62}, ed25519.SeedSize),
	)
	client := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x63}, ed25519.SeedSize),
	)
	responsePublic := trustTestRawPublicKey(response)
	responsePin := ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		responsePublic,
	)
	valid := FrostNativeSignerAnchorBootstrapClientConfig{
		Endpoint:                    "http://127.0.0.1:9799/anchor",
		ResponsePublicKey:           responsePublic,
		ResponsePublicKeySPKISHA256: responsePin,
		ClientPrivateKey:            client,
	}
	if _, err := NewFrostNativeSignerAnchorBootstrapClient(valid); err != nil {
		t.Fatalf("valid bootstrap client config was rejected: %v", err)
	}

	tests := map[string]func(*FrostNativeSignerAnchorBootstrapClientConfig){
		"zero client key": func(config *FrostNativeSignerAnchorBootstrapClientConfig) {
			config.ClientPrivateKey = make(ed25519.PrivateKey, ed25519.PrivateKeySize)
		},
		"client key aliases the response key": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.ClientPrivateKey = response
		},
		"loopback HTTP with TLS roots": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.TLSRootCAs = x509.NewCertPool()
		},
		"loopback HTTP with a leaf pin": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.EndpointLeafSPKIHash = trustTestBytes32(0x0c)
		},
		"HTTPS without a leaf pin": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.Endpoint = "https://anchor.example/anchor"
		},
		"request timeout above the bound": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.RequestTimeout = 31 * time.Second
		},
		"response key SPKI pin mismatch": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.ResponsePublicKeySPKISHA256 = trustTestBytes32(0x0d)
		},
		"missing client key and path": func(
			config *FrostNativeSignerAnchorBootstrapClientConfig,
		) {
			config.ClientPrivateKey = nil
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if _, err := NewFrostNativeSignerAnchorBootstrapClient(
				candidate,
			); err == nil {
				t.Fatal("expected bootstrap client constructor rejection")
			}
		})
	}
}

// TestFrostNativeSignerAnchorBootstrapInitializeTranscriptFrozenVector pins
// the exact signed request bytes for fixed inputs so the wire transcript
// cannot drift silently, and proves the create and reconciliation kinds are
// signature-disjoint.
func TestFrostNativeSignerAnchorBootstrapInitializeTranscriptFrozenVector(
	t *testing.T,
) {
	privateKey := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x01}, ed25519.SeedSize),
	)
	clientSPKI, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        testFrostNativeSignerAnchorBytes32(0x55),
		Generation:              7,
		PreviousStateCommitment: testFrostNativeSignerAnchorBytes32(0x66),
		StateImageDigest:        testFrostNativeSignerAnchorBytes32(0x77),
		StateCommitment:         testFrostNativeSignerAnchorBytes32(0x88),
	}
	transcript := frostNativeSignerAnchorInitializeRequestTranscript(
		"initialize",
		testFrostNativeSignerAnchorBytes32(0x11),
		testFrostNativeSignerAnchorBytes32(0x22),
		testFrostNativeSignerAnchorBytes32(0x33),
		testFrostNativeSignerAnchorBytes32(0x44),
		checkpoint,
		clientSPKI,
	)
	if len(transcript) != 741 {
		t.Fatalf("unexpected frozen transcript length [%d]", len(transcript))
	}
	transcriptDigest := sha256.Sum256(transcript)
	if hex.EncodeToString(transcriptDigest[:]) !=
		"753e5bad0920848776af636ea4a53d3a221621c7ae59b17430ed53602f42a694" {
		t.Fatalf("unexpected frozen initialize transcript digest [%x]", transcriptDigest)
	}
	signature := ed25519.Sign(privateKey, transcript)
	if hex.EncodeToString(signature) !=
		"1e0a673a2bd5871d6817b47a977d5e6d73fc00d0ec856d997c98171240310e7b"+
			"7c9c485e0df38083173d6ac69d23d07cee5dca582cdc8bd211d16223ec751a08" {
		t.Fatalf("unexpected frozen initialize signature [%x]", signature)
	}
	readTranscript := frostNativeSignerAnchorInitializeRequestTranscript(
		"read",
		testFrostNativeSignerAnchorBytes32(0x11),
		testFrostNativeSignerAnchorBytes32(0x22),
		testFrostNativeSignerAnchorBytes32(0x33),
		testFrostNativeSignerAnchorBytes32(0x44),
		checkpoint,
		clientSPKI,
	)
	readDigest := sha256.Sum256(readTranscript)
	if len(readTranscript) != 735 ||
		hex.EncodeToString(readDigest[:]) !=
			"e2107032e2e397a615f2a1889f2c46c66585e965e11fa42345baed3c2dd36e2b" {
		t.Fatalf("unexpected frozen read transcript digest [%x]", readDigest)
	}
	if bytes.Equal(transcript, readTranscript) {
		t.Fatal("initialize and read transcripts must be signature-disjoint")
	}
}

func TestFrostNativeSignerAnchorBootstrapClientLoadsCanonicalConfig(
	t *testing.T,
) {
	environment := newBootstrapClientTestEnvironmentWithNow(t, time.Now)
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(environment.clientKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(directory, "client-key.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyDER,
	})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	responsePublic := trustTestRawPublicKey(environment.response)
	configPath := filepath.Join(directory, "client-config.json")
	configJSON := bootstrapClientTestConfigJSON(
		environment.endpoint,
		frostNativeSignerAnchorHex32(responsePublic),
		frostNativeSignerAnchorHex32(
			ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(responsePublic),
		),
		frostNativeSignerAnchorHex32([32]byte{}),
		keyPath,
		"3000",
	)
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := LoadFrostNativeSignerAnchorBootstrapClient(configPath)
	if err != nil {
		t.Fatalf("canonical bootstrap client config failed to load: %v", err)
	}
	if _, err := InitializeFrostNativeSignerAnchorBootstrap(
		context.Background(),
		environment.core,
		environment.coreSignature,
		client,
	); err != nil {
		t.Fatalf("loaded bootstrap client failed end-to-end: %v", err)
	}
}
