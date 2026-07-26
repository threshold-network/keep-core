package cmd

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

type tbtcSignerBootstrapTestFixture struct {
	plan      *tbtc.FrostNativeSignerAnchorBootstrapPlan
	facts     *frostsigning.NativeTBTCSignerStateAnchorBootstrapFacts
	authority ed25519.PrivateKey
	response  ed25519.PrivateKey
}

func tbtcSignerBootstrapTestBytes32(value byte) [32]byte {
	result := [32]byte{}
	for index := range result {
		result[index] = value
	}
	return result
}

func tbtcSignerBootstrapTestHex32(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

func tbtcSignerBootstrapTestPublicKey(
	privateKey ed25519.PrivateKey,
) [ed25519.PublicKeySize]byte {
	result := [ed25519.PublicKeySize]byte{}
	copy(result[:], privateKey.Public().(ed25519.PublicKey))
	return result
}

func newTBTCSignerBootstrapTestFixture() *tbtcSignerBootstrapTestFixture {
	authority := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x41}, ed25519.SeedSize),
	)
	response := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x42}, ed25519.SeedSize),
	)
	authorityPublic := tbtcSignerBootstrapTestPublicKey(authority)
	responsePublic := tbtcSignerBootstrapTestPublicKey(response)
	endpoint := "http://127.0.0.1:9788/anchor"
	store := tbtcSignerBootstrapTestBytes32(0x13)
	identity := tbtc.FrostNativeSignerAnchorIdentity{
		ProtocolID:                 tbtcSignerBootstrapTestBytes32(0x11),
		ActivationManifestHash:     tbtcSignerBootstrapTestBytes32(0x14),
		ActivationManifestSequence: 7,
		TrustDomainID:              "cli-bootstrap-trust-domain",
		OnlineKeyHash: tbtc.ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			responsePublic,
		),
		OperatorFingerprint:       tbtcSignerBootstrapTestBytes32(0x16),
		HistoryStoreID:            "cli-bootstrap-history-store",
		HistoryStoreFingerprint:   tbtcSignerBootstrapTestBytes32(0x17),
		HistoryClusterFingerprint: tbtcSignerBootstrapTestBytes32(0x18),
		OfflineAuthorityHash: tbtc.ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			authorityPublic,
		),
		ClientSPKIHash:         tbtcSignerBootstrapTestBytes32(0x19),
		SignerStoreFingerprint: store,
		TransportBinding: tbtc.ComputeFrostNativeSignerAnchorTransportBinding(
			endpoint,
		),
		WitnessMaximumRecords:           1000,
		WitnessRotationThresholdRecords: 900,
	}
	identity.StreamID = tbtc.ComputeFrostNativeSignerAnchorStreamID(identity)
	genesis := frostsigning.ComputeNativeTBTCSignerStateWitnessGenesis(store)
	image := tbtcSignerBootstrapTestBytes32(0x1a)
	return &tbtcSignerBootstrapTestFixture{
		plan: &tbtc.FrostNativeSignerAnchorBootstrapPlan{
			Schema:                    tbtc.FrostNativeSignerAnchorBootstrapPlanSchema,
			Endpoint:                  endpoint,
			Identity:                  identity,
			ResponsePublicKey:         responsePublic,
			OfflineAuthorityPublicKey: authorityPublic,
		},
		facts: &frostsigning.NativeTBTCSignerStateAnchorBootstrapFacts{
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
		},
		authority: authority,
		response:  response,
	}
}

type tbtcSignerBootstrapTestCheckpointWire struct {
	StoreFingerprint        string `json:"storeFingerprint"`
	Generation              string `json:"generation"`
	PreviousStateCommitment string `json:"previousStateCommitment"`
	StateImageDigest        string `json:"stateImageDigest"`
	StateCommitment         string `json:"stateCommitment"`
}

type tbtcSignerBootstrapTestAcknowledgementWire struct {
	Schema            string                                `json:"schema"`
	BindingHash       string                                `json:"bindingHash"`
	RequestDigest     string                                `json:"requestDigest"`
	Nonce             string                                `json:"nonce"`
	Status            string                                `json:"status"`
	ServiceEpoch      string                                `json:"serviceEpoch"`
	Revision          string                                `json:"revision"`
	PreviousEventRoot string                                `json:"previousEventRoot"`
	EventRoot         string                                `json:"eventRoot"`
	Checkpoint        tbtcSignerBootstrapTestCheckpointWire `json:"checkpoint"`
	OperationID       string                                `json:"operationID"`
	TransitionDigest  string                                `json:"transitionDigest"`
	CommittedAtUnixMs string                                `json:"committedAtUnixMs"`
	ExpiresAtUnixMs   string                                `json:"expiresAtUnixMs"`
	Signature         string                                `json:"signature"`
}

// tbtcSignerBootstrapTestRecord is a minimal local reimplementation of the
// history-service acknowledgement transcripts. The tbtc package fixtures are
// package-private, so the CLI test reproduces the exact domain-separated
// hashes the anchor protocol pins.
func tbtcSignerBootstrapTestRecord(
	certificate *tbtc.FrostNativeSignerAnchorTrustCertificate,
	response ed25519.PrivateKey,
) (*tbtc.FrostNativeSignerStateWitnessAnchorRecord, error) {
	const (
		serviceEpoch = uint64(1)
		revision     = uint64(1)
		committedAt  = uint64(1_700_000_000_000)
		expiresAt    = uint64(1_700_000_020_000)
		statusByte   = byte(0x01)
	)
	bindingHash := certificate.To.BindingHash
	requestDigest := tbtcSignerBootstrapTestBytes32(0x21)
	nonce := tbtcSignerBootstrapTestBytes32(0x22)
	checkpoint := certificate.To.Reference.Checkpoint
	previousEventRoot := [32]byte{}

	writeCheckpoint := func(buffer *bytes.Buffer) {
		buffer.Write(checkpoint.StoreFingerprint[:])
		_ = binary.Write(buffer, binary.BigEndian, checkpoint.Generation)
		buffer.Write(checkpoint.PreviousStateCommitment[:])
		buffer.Write(checkpoint.StateImageDigest[:])
		buffer.Write(checkpoint.StateCommitment[:])
	}
	writeTail := func(buffer *bytes.Buffer) {
		buffer.Write(certificate.OperationID[:])
		buffer.Write(certificate.TransitionDigest[:])
		_ = binary.Write(buffer, binary.BigEndian, committedAt)
		_ = binary.Write(buffer, binary.BigEndian, expiresAt)
	}

	eventBuffer := bytes.NewBuffer(nil)
	eventBuffer.WriteString("tbtc-native-signer-state-anchor-event/v1\x00")
	eventBuffer.Write(bindingHash[:])
	_ = binary.Write(eventBuffer, binary.BigEndian, serviceEpoch)
	_ = binary.Write(eventBuffer, binary.BigEndian, revision)
	eventBuffer.Write(previousEventRoot[:])
	eventBuffer.Write(requestDigest[:])
	eventBuffer.Write(nonce[:])
	eventBuffer.WriteByte(statusByte)
	writeCheckpoint(eventBuffer)
	writeTail(eventBuffer)
	eventRoot := sha256.Sum256(eventBuffer.Bytes())

	signingBuffer := bytes.NewBuffer(nil)
	signingBuffer.WriteString(
		"tbtc-native-signer-state-anchor-service-response/v1\x00",
	)
	signingBuffer.Write(bindingHash[:])
	signingBuffer.Write(requestDigest[:])
	signingBuffer.Write(nonce[:])
	signingBuffer.WriteByte(statusByte)
	_ = binary.Write(signingBuffer, binary.BigEndian, serviceEpoch)
	_ = binary.Write(signingBuffer, binary.BigEndian, revision)
	signingBuffer.Write(previousEventRoot[:])
	signingBuffer.Write(eventRoot[:])
	writeCheckpoint(signingBuffer)
	writeTail(signingBuffer)
	signingDigest := sha256.Sum256(signingBuffer.Bytes())
	signature := ed25519.Sign(response, signingDigest[:])

	acknowledgementHasher := sha256.New()
	acknowledgementHasher.Write(
		[]byte("tbtc-signer-state-anchor-acknowledgement/v1\x00"),
	)
	acknowledgementHasher.Write(signingDigest[:])
	acknowledgementHasher.Write(signature)
	acknowledgementHasher.Write(certificate.To.ResponsePublicKeySPKISHA256[:])
	acknowledgementDigest := [32]byte{}
	copy(acknowledgementDigest[:], acknowledgementHasher.Sum(nil))

	raw, err := json.Marshal(tbtcSignerBootstrapTestAcknowledgementWire{
		Schema:            tbtc.FrostNativeSignerCheckpointAcknowledgementSchema,
		BindingHash:       tbtcSignerBootstrapTestHex32(bindingHash),
		RequestDigest:     tbtcSignerBootstrapTestHex32(requestDigest),
		Nonce:             tbtcSignerBootstrapTestHex32(nonce),
		Status:            "applied",
		ServiceEpoch:      "1",
		Revision:          "1",
		PreviousEventRoot: tbtcSignerBootstrapTestHex32(previousEventRoot),
		EventRoot:         tbtcSignerBootstrapTestHex32(eventRoot),
		Checkpoint: tbtcSignerBootstrapTestCheckpointWire{
			StoreFingerprint: tbtcSignerBootstrapTestHex32(
				checkpoint.StoreFingerprint,
			),
			Generation: "1",
			PreviousStateCommitment: tbtcSignerBootstrapTestHex32(
				checkpoint.PreviousStateCommitment,
			),
			StateImageDigest: tbtcSignerBootstrapTestHex32(
				checkpoint.StateImageDigest,
			),
			StateCommitment: tbtcSignerBootstrapTestHex32(
				checkpoint.StateCommitment,
			),
		},
		OperationID: tbtcSignerBootstrapTestHex32(certificate.OperationID),
		TransitionDigest: tbtcSignerBootstrapTestHex32(
			certificate.TransitionDigest,
		),
		CommittedAtUnixMs: "1700000000000",
		ExpiresAtUnixMs:   "1700000020000",
		Signature:         "0x" + hex.EncodeToString(signature),
	})
	if err != nil {
		return nil, err
	}
	return &tbtc.FrostNativeSignerStateWitnessAnchorRecord{
		Checkpoint:             checkpoint,
		BindingHash:            bindingHash,
		AcknowledgementDigest:  acknowledgementDigest,
		OperationID:            certificate.OperationID,
		TransitionDigest:       certificate.TransitionDigest,
		ServiceEpoch:           serviceEpoch,
		Revision:               revision,
		PreviousEventRoot:      previousEventRoot,
		EventRoot:              eventRoot,
		AcknowledgementJSON:    raw,
		AcknowledgementExpires: expiresAt,
		ReadRecoveryJSON:       []byte(`{"readRecovery":"fresh"}`),
		ReadRecoveryExpires:    expiresAt,
	}, nil
}

type tbtcSignerBootstrapTestClient struct {
	response ed25519.PrivateKey
}

func (client *tbtcSignerBootstrapTestClient) InitializeFrostNativeSignerAnchor(
	_ context.Context,
	authorization tbtc.FrostNativeSignerAnchorBootstrapAuthorization,
) (*tbtc.FrostNativeSignerAnchorBootstrapClientResult, error) {
	record, err := tbtcSignerBootstrapTestRecord(
		&authorization.Certificate,
		client.response,
	)
	if err != nil {
		return nil, err
	}
	return &tbtc.FrostNativeSignerAnchorBootstrapClientResult{
		Record: record,
	}, nil
}

func runTBTCSignerBootstrapCommand(
	t *testing.T,
	clientFactory tbtcSignerAnchorBootstrapClientFactory,
	args ...string,
) error {
	t.Helper()
	command := newTBTCSignerCommand(clientFactory)
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs(args)
	return command.Execute()
}

func writeTBTCSignerBootstrapTestArtifact(
	t *testing.T,
	path string,
	data []byte,
) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFrostNativeSignerAnchorBootstrapCommandCeremony(t *testing.T) {
	fixture := newTBTCSignerBootstrapTestFixture()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}

	factsPath := filepath.Join(directory, "facts.json")
	factsJSON, err := frostsigning.EncodeNativeTBTCSignerStateAnchorBootstrapFacts(
		fixture.facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	writeTBTCSignerBootstrapTestArtifact(t, factsPath, factsJSON)

	planPath := filepath.Join(directory, "plan.json")
	planJSON, err := tbtc.EncodeFrostNativeSignerAnchorBootstrapPlan(
		fixture.plan,
	)
	if err != nil {
		t.Fatal(err)
	}
	writeTBTCSignerBootstrapTestArtifact(t, planPath, planJSON)

	corePath := filepath.Join(directory, "core.json")
	if err := runTBTCSignerBootstrapCommand(
		t,
		nil,
		"anchor", "bootstrap", "core",
		"--facts", factsPath,
		"--plan", planPath,
		"--output", corePath,
	); err != nil {
		t.Fatalf("bootstrap core command failed: %v", err)
	}
	coreJSON, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	core, err := tbtc.DecodeFrostNativeSignerAnchorBootstrapCoreArtifact(
		coreJSON,
	)
	if err != nil {
		t.Fatalf("bootstrap core command emitted an invalid artifact: %v", err)
	}

	coreSignaturePath := filepath.Join(directory, "core-signature.json")
	coreSignature := &tbtc.FrostNativeSignerAnchorBootstrapDetachedSignature{
		Schema: tbtc.FrostNativeSignerAnchorBootstrapDetachedSignatureSchema,
		Stage:  tbtc.FrostNativeSignerAnchorBootstrapCoreSignatureStage,
		Digest: core.CoreDigest,
	}
	copy(
		coreSignature.Signature[:],
		ed25519.Sign(fixture.authority, core.CoreDigest[:]),
	)
	coreSignatureJSON, err :=
		tbtc.EncodeFrostNativeSignerAnchorBootstrapDetachedSignature(
			coreSignature,
		)
	if err != nil {
		t.Fatal(err)
	}
	writeTBTCSignerBootstrapTestArtifact(
		t,
		coreSignaturePath,
		coreSignatureJSON,
	)

	clientConfigPath := filepath.Join(directory, "client-config.json")
	factory := func(
		_ context.Context,
		configPath string,
	) (tbtc.FrostNativeSignerAnchorBootstrapClient, error) {
		if configPath != clientConfigPath {
			t.Fatalf(
				"bootstrap client factory received path %q",
				configPath,
			)
		}
		return &tbtcSignerBootstrapTestClient{
			response: fixture.response,
		}, nil
	}
	finalPath := filepath.Join(directory, "final.json")
	if err := runTBTCSignerBootstrapCommand(
		t,
		factory,
		"anchor", "bootstrap", "initialize",
		"--core", corePath,
		"--core-signature", coreSignaturePath,
		"--client-config", clientConfigPath,
		"--output", finalPath,
	); err != nil {
		t.Fatalf("bootstrap initialize command failed: %v", err)
	}
	finalJSON, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	final, err := tbtc.DecodeFrostNativeSignerAnchorBootstrapFinalArtifact(
		finalJSON,
	)
	if err != nil {
		t.Fatalf("bootstrap initialize emitted an invalid artifact: %v", err)
	}
	if final.Core.CoreDigest != core.CoreDigest ||
		final.TargetReference.ServiceEpoch != 1 ||
		final.TargetReference.Revision != 1 {
		t.Fatalf("unexpected bootstrap final artifact: %+v", final)
	}

	finalSignaturePath := filepath.Join(directory, "final-signature.json")
	finalSignature := &tbtc.FrostNativeSignerAnchorBootstrapDetachedSignature{
		Schema: tbtc.FrostNativeSignerAnchorBootstrapDetachedSignatureSchema,
		Stage:  tbtc.FrostNativeSignerAnchorBootstrapFinalSignatureStage,
		Digest: final.FinalDigest,
	}
	copy(
		finalSignature.Signature[:],
		ed25519.Sign(fixture.authority, final.FinalDigest[:]),
	)
	finalSignatureJSON, err :=
		tbtc.EncodeFrostNativeSignerAnchorBootstrapDetachedSignature(
			finalSignature,
		)
	if err != nil {
		t.Fatal(err)
	}
	writeTBTCSignerBootstrapTestArtifact(
		t,
		finalSignaturePath,
		finalSignatureJSON,
	)

	baseConfigPath := filepath.Join(directory, "base-config.json")
	writeTBTCSignerBootstrapTestArtifact(
		t,
		baseConfigPath,
		[]byte(`{"profile":"production","state_path":"/var/lib/keep/tbtc-signer"}`),
	)

	bundlePath := filepath.Join(directory, "bundle.json")
	if err := runTBTCSignerBootstrapCommand(
		t,
		nil,
		"anchor", "bootstrap", "finalize",
		"--final", finalPath,
		"--final-signature", finalSignaturePath,
		"--base-config", baseConfigPath,
		"--output", bundlePath,
	); err != nil {
		t.Fatalf("bootstrap finalize command failed: %v", err)
	}
	bundleJSON, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := tbtc.DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
		bundleJSON,
	)
	if err != nil {
		t.Fatalf("bootstrap finalize emitted an invalid bundle: %v", err)
	}
	if len(bundle.CertificateChain) != 1 ||
		bundle.CertificateChain[0].Kind !=
			tbtc.FrostNativeSignerAnchorTrustCertificateBootstrap ||
		bundle.CertificateChain[0].To.Reference.Checkpoint !=
			core.Checkpoint {
		t.Fatalf("unexpected bootstrap output bundle: %+v", bundle)
	}
}

func TestFrostNativeSignerAnchorBootstrapCommandInitializeWithoutTransport(
	t *testing.T,
) {
	directory := t.TempDir()
	err := runTBTCSignerBootstrapCommand(
		t,
		nil,
		"anchor", "bootstrap", "initialize",
		"--core", filepath.Join(directory, "core.json"),
		"--core-signature", filepath.Join(directory, "core-signature.json"),
		"--client-config", filepath.Join(directory, "client-config.json"),
		"--output", filepath.Join(directory, "final.json"),
	)
	if err == nil ||
		!strings.Contains(err.Error(), "transport is not available") {
		t.Fatalf("initialize without a transport factory returned: %v", err)
	}
}

func TestFrostNativeSignerAnchorBootstrapCommandRejectsNonCanonicalClientConfig(
	t *testing.T,
) {
	directory := t.TempDir()
	factory := func(
		_ context.Context,
		_ string,
	) (tbtc.FrostNativeSignerAnchorBootstrapClient, error) {
		t.Fatal("client factory was invoked for a non-canonical path")
		return nil, nil
	}
	for _, path := range []string{
		"relative/client-config.json",
		filepath.Join(directory, "sub", "..", "client-config.json") + "/",
	} {
		err := runTBTCSignerBootstrapCommand(
			t,
			factory,
			"anchor", "bootstrap", "initialize",
			"--core", filepath.Join(directory, "core.json"),
			"--core-signature", filepath.Join(directory, "core-signature.json"),
			"--client-config", path,
			"--output", filepath.Join(directory, "final.json"),
		)
		if err == nil ||
			!strings.Contains(err.Error(), "not canonical absolute") {
			t.Fatalf(
				"initialize with client config path %q returned: %v",
				path,
				err,
			)
		}
	}
}

func TestFrostNativeSignerAnchorBootstrapCommandRequiresFlags(t *testing.T) {
	for _, subcommand := range []string{
		"facts",
		"core",
		"initialize",
		"finalize",
	} {
		err := runTBTCSignerBootstrapCommand(
			t,
			nil,
			"anchor", "bootstrap", subcommand,
		)
		if err == nil ||
			!strings.Contains(err.Error(), "required flag(s)") {
			t.Fatalf(
				"bootstrap %s without flags returned: %v",
				subcommand,
				err,
			)
		}
	}
}

func TestFrostNativeSignerAnchorBootstrapCommandFactsFailsClosed(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	provisioningConfigPath := filepath.Join(
		directory,
		"provisioning-config.json",
	)
	writeTBTCSignerBootstrapTestArtifact(
		t,
		provisioningConfigPath,
		[]byte(`{"purpose":"state_anchor_bootstrap_provisioning",`+
			`"profile":"production",`+
			`"state_path":"/var/lib/keep/tbtc-signer",`+
			`"state_witness_max_records":4}`),
	)
	outputPath := filepath.Join(directory, "facts.json")
	err := runTBTCSignerBootstrapCommand(
		t,
		nil,
		"anchor", "bootstrap", "facts",
		"--provisioning-config", provisioningConfigPath,
		"--output", outputPath,
	)
	if err == nil {
		t.Fatal("facts command succeeded without the native tbtc-signer bridge")
	}
	if !errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
		t.Skipf(
			"facts command failed for a non-default-build reason: %v",
			err,
		)
	}
	if _, statErr := os.Lstat(outputPath); !os.IsNotExist(statErr) {
		t.Fatal("failed facts command still produced an output artifact")
	}
}
