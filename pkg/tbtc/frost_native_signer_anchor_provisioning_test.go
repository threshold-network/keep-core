package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type bootstrapProvisioningTestFixture struct {
	endpoint  string
	plan      *FrostNativeSignerAnchorBootstrapPlan
	facts     *frostsigning.NativeTBTCSignerStateAnchorBootstrapFacts
	authority ed25519.PrivateKey
	response  ed25519.PrivateKey
}

func newBootstrapProvisioningTestFixture() *bootstrapProvisioningTestFixture {
	authority := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x61}, ed25519.SeedSize),
	)
	response := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x62}, ed25519.SeedSize),
	)
	authorityPublic := trustTestRawPublicKey(authority)
	responsePublic := trustTestRawPublicKey(response)
	endpoint := "http://127.0.0.1:9799/anchor"
	store := trustTestBytes32(0x03)
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                 trustTestBytes32(0x01),
		ActivationManifestHash:     trustTestBytes32(0x04),
		ActivationManifestSequence: 9,
		TrustDomainID:              "bootstrap-trust-domain",
		OnlineKeyHash: ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			responsePublic,
		),
		OperatorFingerprint:       trustTestBytes32(0x06),
		HistoryStoreID:            "bootstrap-history-store",
		HistoryStoreFingerprint:   trustTestBytes32(0x07),
		HistoryClusterFingerprint: trustTestBytes32(0x08),
		OfflineAuthorityHash: ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			authorityPublic,
		),
		ClientSPKIHash:                  trustTestBytes32(0x09),
		SignerStoreFingerprint:          store,
		TransportBinding:                ComputeFrostNativeSignerAnchorTransportBinding(endpoint),
		WitnessMaximumRecords:           1000,
		WitnessRotationThresholdRecords: 900,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	genesis := frostsigning.ComputeNativeTBTCSignerStateWitnessGenesis(store)
	image := trustTestBytes32(0x0a)
	return &bootstrapProvisioningTestFixture{
		endpoint: endpoint,
		plan: &FrostNativeSignerAnchorBootstrapPlan{
			Schema:                    FrostNativeSignerAnchorBootstrapPlanSchema,
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

// bootstrapProvisioningTestRecord simulates the history service: it constructs
// the exact response-key-signed acknowledgement and reconciled record that a
// correct create-if-absent endpoint would return for the given authorization
// certificate.
func bootstrapProvisioningTestRecord(
	t *testing.T,
	certificate *FrostNativeSignerAnchorTrustCertificate,
	response ed25519.PrivateKey,
) *FrostNativeSignerStateWitnessAnchorRecord {
	t.Helper()
	acknowledgement := FrostNativeSignerCheckpointAcknowledgement{
		BindingHash:       certificate.To.BindingHash,
		RequestDigest:     trustTestBytes32(0x71),
		Nonce:             trustTestBytes32(0x72),
		Status:            "applied",
		ServiceEpoch:      1,
		Revision:          1,
		Checkpoint:        certificate.To.Reference.Checkpoint,
		OperationID:       certificate.OperationID,
		TransitionDigest:  certificate.TransitionDigest,
		CommittedAtUnixMs: 1_700_000_000_000,
		ExpiresAtUnixMs:   1_700_000_020_000,
	}
	acknowledgement.EventRoot =
		computeFrostNativeSignerAnchorEventRoot(acknowledgement)
	wire := frostNativeSignerAnchorAcknowledgementWire{
		Schema:            FrostNativeSignerCheckpointAcknowledgementSchema,
		BindingHash:       frostNativeSignerAnchorHex32(acknowledgement.BindingHash),
		RequestDigest:     frostNativeSignerAnchorHex32(acknowledgement.RequestDigest),
		Nonce:             frostNativeSignerAnchorHex32(acknowledgement.Nonce),
		Status:            acknowledgement.Status,
		ServiceEpoch:      fmt.Sprint(acknowledgement.ServiceEpoch),
		Revision:          fmt.Sprint(acknowledgement.Revision),
		PreviousEventRoot: frostNativeSignerAnchorHex32(acknowledgement.PreviousEventRoot),
		EventRoot:         frostNativeSignerAnchorHex32(acknowledgement.EventRoot),
		Checkpoint: frostNativeSignerAnchorCheckpointToWire(
			acknowledgement.Checkpoint,
		),
		OperationID:       frostNativeSignerAnchorHex32(acknowledgement.OperationID),
		TransitionDigest:  frostNativeSignerAnchorHex32(acknowledgement.TransitionDigest),
		CommittedAtUnixMs: fmt.Sprint(acknowledgement.CommittedAtUnixMs),
		ExpiresAtUnixMs:   fmt.Sprint(acknowledgement.ExpiresAtUnixMs),
	}
	signingDigest, err := frostNativeSignerAnchorAcknowledgementTranscript(wire)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(response, signingDigest)
	wire.Signature = frostNativeSignerAnchorSignatureHex(signature)
	var fixedSigningDigest [32]byte
	copy(fixedSigningDigest[:], signingDigest)
	var fixedSignature [ed25519.SignatureSize]byte
	copy(fixedSignature[:], signature)
	acknowledgementDigest :=
		computeFrostNativeSignerCheckpointAcknowledgementDigest(
			fixedSigningDigest,
			fixedSignature,
			certificate.To.ResponsePublicKeySPKISHA256,
		)
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	return &FrostNativeSignerStateWitnessAnchorRecord{
		Checkpoint:             acknowledgement.Checkpoint,
		BindingHash:            acknowledgement.BindingHash,
		AcknowledgementDigest:  acknowledgementDigest,
		OperationID:            acknowledgement.OperationID,
		TransitionDigest:       acknowledgement.TransitionDigest,
		ServiceEpoch:           acknowledgement.ServiceEpoch,
		Revision:               acknowledgement.Revision,
		PreviousEventRoot:      acknowledgement.PreviousEventRoot,
		EventRoot:              acknowledgement.EventRoot,
		AcknowledgementJSON:    raw,
		AcknowledgementExpires: acknowledgement.ExpiresAtUnixMs,
		ReadRecoveryJSON:       []byte(`{"readRecovery":"fresh"}`),
		ReadRecoveryExpires:    acknowledgement.ExpiresAtUnixMs,
	}
}

type bootstrapProvisioningTestClient struct {
	t        *testing.T
	response ed25519.PrivateKey
	err      error
	mutate   func(*FrostNativeSignerStateWitnessAnchorRecord)
	result   *FrostNativeSignerAnchorBootstrapClientResult
	seen     *FrostNativeSignerAnchorBootstrapAuthorization
}

func (client *bootstrapProvisioningTestClient) InitializeFrostNativeSignerAnchor(
	_ context.Context,
	authorization FrostNativeSignerAnchorBootstrapAuthorization,
) (*FrostNativeSignerAnchorBootstrapClientResult, error) {
	client.seen = &authorization
	if client.err != nil {
		return nil, client.err
	}
	if client.result != nil {
		return client.result, nil
	}
	record := bootstrapProvisioningTestRecord(
		client.t,
		&authorization.Certificate,
		client.response,
	)
	if client.mutate != nil {
		client.mutate(record)
	}
	return &FrostNativeSignerAnchorBootstrapClientResult{Record: record}, nil
}

func bootstrapProvisioningTestDetachedSignature(
	key ed25519.PrivateKey,
	stage FrostNativeSignerAnchorBootstrapSignatureStage,
	digest [32]byte,
) *FrostNativeSignerAnchorBootstrapDetachedSignature {
	result := &FrostNativeSignerAnchorBootstrapDetachedSignature{
		Schema: FrostNativeSignerAnchorBootstrapDetachedSignatureSchema,
		Stage:  stage,
		Digest: digest,
	}
	copy(result.Signature[:], ed25519.Sign(key, digest[:]))
	return result
}

func bootstrapProvisioningTestBaseConfig() []byte {
	return []byte(
		`{"profile":"production","state_path":"/var/lib/keep/tbtc-signer"}`,
	)
}

type bootstrapProvisioningTestCeremony struct {
	fixture        *bootstrapProvisioningTestFixture
	core           *FrostNativeSignerAnchorBootstrapCoreArtifact
	coreSignature  *FrostNativeSignerAnchorBootstrapDetachedSignature
	final          *FrostNativeSignerAnchorBootstrapFinalArtifact
	finalSignature *FrostNativeSignerAnchorBootstrapDetachedSignature
	baseConfig     []byte
	bundle         []byte
}

func runBootstrapProvisioningTestCeremony(
	t *testing.T,
) *bootstrapProvisioningTestCeremony {
	t.Helper()
	fixture := newBootstrapProvisioningTestFixture()
	core, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		fixture.facts,
		fixture.plan,
	)
	if err != nil {
		t.Fatalf("valid bootstrap core preparation failed: %v", err)
	}
	coreSignature := bootstrapProvisioningTestDetachedSignature(
		fixture.authority,
		FrostNativeSignerAnchorBootstrapCoreSignatureStage,
		core.CoreDigest,
	)
	final, err := InitializeFrostNativeSignerAnchorBootstrap(
		context.Background(),
		core,
		coreSignature,
		&bootstrapProvisioningTestClient{t: t, response: fixture.response},
	)
	if err != nil {
		t.Fatalf("valid bootstrap initialization failed: %v", err)
	}
	finalSignature := bootstrapProvisioningTestDetachedSignature(
		fixture.authority,
		FrostNativeSignerAnchorBootstrapFinalSignatureStage,
		final.FinalDigest,
	)
	baseConfig := bootstrapProvisioningTestBaseConfig()
	bundle, err := FinalizeFrostNativeSignerAnchorBootstrap(
		final,
		finalSignature,
		baseConfig,
	)
	if err != nil {
		t.Fatalf("valid bootstrap finalization failed: %v", err)
	}
	return &bootstrapProvisioningTestCeremony{
		fixture:        fixture,
		core:           core,
		coreSignature:  coreSignature,
		final:          final,
		finalSignature: finalSignature,
		baseConfig:     baseConfig,
		bundle:         bundle,
	}
}

func TestFrostNativeSignerAnchorBootstrapPlanCodec(t *testing.T) {
	fixture := newBootstrapProvisioningTestFixture()
	encoded, err := EncodeFrostNativeSignerAnchorBootstrapPlan(fixture.plan)
	if err != nil {
		t.Fatalf("valid bootstrap plan was rejected by the encoder: %v", err)
	}
	decoded, err := DecodeFrostNativeSignerAnchorBootstrapPlan(encoded)
	if err != nil {
		t.Fatalf("canonical bootstrap plan was rejected: %v", err)
	}
	if !reflect.DeepEqual(decoded, fixture.plan) {
		t.Fatalf("bootstrap plan round trip diverged: %+v", decoded)
	}

	tests := map[string]struct {
		mutate func(*frostNativeSignerAnchorBootstrapPlanWire)
	}{
		"wrong schema": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Schema = FrostNativeSignerAnchorBootstrapCoreArtifactSchema
			},
		},
		"endpoint transport-binding mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Endpoint = "http://127.0.0.1:9800/anchor"
			},
		},
		"non-canonical endpoint": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Endpoint = "http://127.0.0.1:9799/anchor/"
			},
		},
		"non-loopback HTTP endpoint": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Endpoint = "http://10.0.0.7:9799/anchor"
			},
		},
		"zero protocol pin": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Identity.ProtocolID =
					frostNativeSignerAnchorHex32([32]byte{})
			},
		},
		"zero transport binding pin": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Identity.TransportBinding =
					frostNativeSignerAnchorHex32([32]byte{})
			},
		},
		"loopback plan with a non-zero endpoint leaf pin": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Identity.EndpointLeafSPKIHash =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0d))
			},
		},
		"non-canonical response key": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.ResponsePublicKey =
					strings.ToUpper(wire.ResponsePublicKey)
			},
		},
		"response key differs from its activation pin": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.ResponsePublicKey = wire.OfflineAuthorityPublicKey
			},
		},
		"offline authority key differs from its activation pin": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.OfflineAuthorityPublicKey = wire.ResponsePublicKey
			},
		},
		"stream ID differs from its stable identity": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapPlanWire) {
				wire.Identity.StreamID =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0e))
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wire := frostNativeSignerAnchorBootstrapPlanToWire(fixture.plan)
			test.mutate(&wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeFrostNativeSignerAnchorBootstrapPlan(
				payload,
			); err == nil {
				t.Fatalf("bootstrap plan with %s was accepted", name)
			}
		})
	}

	// The identity role aliasing that collapses the response and authority
	// keys must stay rejected even when both pins are updated consistently.
	aliased := *fixture.plan
	aliased.OfflineAuthorityPublicKey = aliased.ResponsePublicKey
	aliased.Identity.OfflineAuthorityHash = aliased.Identity.OnlineKeyHash
	aliasedWire := frostNativeSignerAnchorBootstrapPlanToWire(&aliased)
	payload, err := json.Marshal(aliasedWire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeFrostNativeSignerAnchorBootstrapPlan(payload); err == nil {
		t.Fatal("bootstrap plan aliasing response and authority keys was accepted")
	}

	strictTests := map[string]string{
		"trailing data": string(encoded) + " {}",
		"duplicate member": strings.Replace(
			string(encoded),
			`"schema"`,
			`"schema":"x","schema"`,
			1,
		),
		"case-folded duplicate member": strings.Replace(
			string(encoded),
			`"schema"`,
			`"SCHEMA":"x","schema"`,
			1,
		),
		"unknown member": strings.Replace(
			string(encoded),
			`"schema"`,
			`"unknown":"x","schema"`,
			1,
		),
		"depth bomb":    strings.Repeat("[", 40) + strings.Repeat("]", 40),
		"empty payload": "",
	}
	for name, payload := range strictTests {
		t.Run("strict "+name, func(t *testing.T) {
			if _, err := DecodeFrostNativeSignerAnchorBootstrapPlan(
				[]byte(payload),
			); err == nil {
				t.Fatalf("bootstrap plan payload with %s was accepted", name)
			}
		})
	}
}

func TestFrostNativeSignerAnchorBootstrapPrepareCore(t *testing.T) {
	fixture := newBootstrapProvisioningTestFixture()
	core, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		fixture.facts,
		fixture.plan,
	)
	if err != nil {
		t.Fatalf("valid bootstrap core preparation failed: %v", err)
	}
	factsJSON, err := frostsigning.EncodeNativeTBTCSignerStateAnchorBootstrapFacts(
		fixture.facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	if core.Schema != FrostNativeSignerAnchorBootstrapCoreArtifactSchema ||
		core.FactsSHA256 != sha256.Sum256(factsJSON) ||
		core.CoreDigest == [32]byte{} ||
		core.OperationID !=
			ComputeFrostNativeSignerAnchorTrustOperationID(core.CoreDigest) ||
		core.TransitionDigest !=
			ComputeFrostNativeSignerAnchorTrustTransitionDigest(
				core.CoreDigest,
				core.OperationID,
			) {
		t.Fatalf("unexpected bootstrap core artifact: %+v", core)
	}

	if _, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		nil,
		fixture.plan,
	); err == nil {
		t.Fatal("nil bootstrap facts were accepted")
	}
	if _, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		fixture.facts,
		nil,
	); err == nil {
		t.Fatal("nil bootstrap plan was accepted")
	}

	crossStore := newBootstrapProvisioningTestFixture()
	otherStore := trustTestBytes32(0x0b)
	otherGenesis :=
		frostsigning.ComputeNativeTBTCSignerStateWitnessGenesis(otherStore)
	crossStore.facts.StoreFingerprint = otherStore
	crossStore.facts.CurrentCheckpoint.StoreFingerprint = otherStore
	crossStore.facts.CurrentCheckpoint.PreviousStateCommitment = otherGenesis
	crossStore.facts.CurrentCheckpoint.StateCommitment =
		frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			otherStore,
			1,
			otherGenesis,
			crossStore.facts.CurrentCheckpoint.StateImageDigest,
		)
	if _, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		crossStore.facts,
		crossStore.plan,
	); err == nil {
		t.Fatal("bootstrap facts from another store were accepted")
	}
}

func TestFrostNativeSignerAnchorBootstrapCoreArtifactCodec(t *testing.T) {
	ceremony := runBootstrapProvisioningTestCeremony(t)
	encoded, err := EncodeFrostNativeSignerAnchorBootstrapCoreArtifact(
		ceremony.core,
	)
	if err != nil {
		t.Fatalf("valid bootstrap core artifact was rejected: %v", err)
	}
	decoded, err := DecodeFrostNativeSignerAnchorBootstrapCoreArtifact(encoded)
	if err != nil {
		t.Fatalf("canonical bootstrap core artifact was rejected: %v", err)
	}
	if !reflect.DeepEqual(decoded, ceremony.core) {
		t.Fatalf("bootstrap core artifact round trip diverged: %+v", decoded)
	}

	genesisCheckpoint := ceremony.core.Checkpoint
	tests := map[string]struct {
		mutate func(*frostNativeSignerAnchorBootstrapCoreArtifactWire)
	}{
		"wrong schema": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				wire.Schema = FrostNativeSignerAnchorBootstrapPlanSchema
			},
		},
		"core digest mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				wire.CoreDigest =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"operation ID mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				wire.OperationID =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"transition digest mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				wire.TransitionDigest =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"zero facts digest": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				wire.FactsSHA256 = frostNativeSignerAnchorHex32([32]byte{})
			},
		},
		"non-genesis checkpoint generation": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				checkpoint := genesisCheckpoint
				checkpoint.Generation = 2
				checkpoint.PreviousStateCommitment = checkpoint.StateCommitment
				checkpoint.StateCommitment =
					frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
						checkpoint.StoreFingerprint,
						checkpoint.Generation,
						checkpoint.PreviousStateCommitment,
						checkpoint.StateImageDigest,
					)
				wire.Checkpoint =
					frostNativeSignerAnchorCheckpointToWire(checkpoint)
			},
		},
		"checkpoint commitment mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapCoreArtifactWire) {
				wire.Checkpoint.StateCommitment =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wire := frostNativeSignerAnchorBootstrapCoreToWire(ceremony.core)
			test.mutate(&wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeFrostNativeSignerAnchorBootstrapCoreArtifact(
				payload,
			); err == nil {
				t.Fatalf("bootstrap core artifact with %s was accepted", name)
			}
		})
	}
}

func TestFrostNativeSignerAnchorBootstrapDetachedSignatureCodec(t *testing.T) {
	ceremony := runBootstrapProvisioningTestCeremony(t)
	for _, signature := range []*FrostNativeSignerAnchorBootstrapDetachedSignature{
		ceremony.coreSignature,
		ceremony.finalSignature,
	} {
		encoded, err := EncodeFrostNativeSignerAnchorBootstrapDetachedSignature(
			signature,
		)
		if err != nil {
			t.Fatalf("valid detached signature was rejected: %v", err)
		}
		decoded, err := DecodeFrostNativeSignerAnchorBootstrapDetachedSignature(
			encoded,
		)
		if err != nil {
			t.Fatalf("canonical detached signature was rejected: %v", err)
		}
		if !reflect.DeepEqual(decoded, signature) {
			t.Fatalf("detached signature round trip diverged: %+v", decoded)
		}
	}

	if _, err := EncodeFrostNativeSignerAnchorBootstrapDetachedSignature(
		&FrostNativeSignerAnchorBootstrapDetachedSignature{
			Schema: FrostNativeSignerAnchorBootstrapDetachedSignatureSchema,
			Stage:  "attest",
			Digest: trustTestBytes32(0x0c),
		},
	); err == nil {
		t.Fatal("detached signature with an unknown stage was encoded")
	}

	canonical, err := EncodeFrostNativeSignerAnchorBootstrapDetachedSignature(
		ceremony.coreSignature,
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		mutate func(*frostNativeSignerAnchorBootstrapDetachedSignatureWire)
	}{
		"wrong schema": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapDetachedSignatureWire) {
				wire.Schema = FrostNativeSignerAnchorBootstrapPlanSchema
			},
		},
		"unknown stage": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapDetachedSignatureWire) {
				wire.Stage = "attest"
			},
		},
		"zero digest": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapDetachedSignatureWire) {
				wire.Digest = frostNativeSignerAnchorHex32([32]byte{})
			},
		},
		"non-canonical digest": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapDetachedSignatureWire) {
				wire.Digest = strings.ToUpper(wire.Digest)
			},
		},
		"invalid signature base64": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapDetachedSignatureWire) {
				wire.Signature = "!" + wire.Signature[1:]
			},
		},
		"short signature": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapDetachedSignatureWire) {
				wire.Signature = "c2hvcnQ="
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wire := frostNativeSignerAnchorBootstrapDetachedSignatureWire{}
			if err := json.Unmarshal(canonical, &wire); err != nil {
				t.Fatal(err)
			}
			test.mutate(&wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeFrostNativeSignerAnchorBootstrapDetachedSignature(
				payload,
			); err == nil {
				t.Fatalf("detached signature with %s was accepted", name)
			}
		})
	}
}

func TestFrostNativeSignerAnchorBootstrapInitialize(t *testing.T) {
	fixture := newBootstrapProvisioningTestFixture()
	core, err := PrepareFrostNativeSignerAnchorBootstrapCore(
		fixture.facts,
		fixture.plan,
	)
	if err != nil {
		t.Fatal(err)
	}
	coreSignature := bootstrapProvisioningTestDetachedSignature(
		fixture.authority,
		FrostNativeSignerAnchorBootstrapCoreSignatureStage,
		core.CoreDigest,
	)
	client := &bootstrapProvisioningTestClient{
		t:        t,
		response: fixture.response,
	}
	final, err := InitializeFrostNativeSignerAnchorBootstrap(
		context.Background(),
		core,
		coreSignature,
		client,
	)
	if err != nil {
		t.Fatalf("valid bootstrap initialization failed: %v", err)
	}
	if client.seen == nil ||
		client.seen.Certificate.Kind !=
			FrostNativeSignerAnchorTrustCertificateBootstrap ||
		client.seen.Certificate.CoreSignature != coreSignature.Signature {
		t.Fatal("bootstrap client did not receive the authorized certificate")
	}
	if final.Schema != FrostNativeSignerAnchorBootstrapFinalArtifactSchema ||
		final.Core.CoreDigest != core.CoreDigest ||
		final.CoreSignature != coreSignature.Signature ||
		final.TargetReference.ServiceEpoch != 1 ||
		final.TargetReference.Revision != 1 ||
		final.TargetReference.PreviousEventRoot != [32]byte{} ||
		final.TargetReference.Checkpoint != core.Checkpoint ||
		final.TargetAcknowledgementSHA256 !=
			sha256.Sum256(final.TargetAcknowledgement) ||
		final.FinalDigest == [32]byte{} {
		t.Fatalf("unexpected bootstrap final artifact: %+v", final)
	}

	newClient := func(
		mutate func(*FrostNativeSignerStateWitnessAnchorRecord),
	) *bootstrapProvisioningTestClient {
		return &bootstrapProvisioningTestClient{
			t:        t,
			response: fixture.response,
			mutate:   mutate,
		}
	}
	divergences := map[string]func(*FrostNativeSignerStateWitnessAnchorRecord){
		"binding hash": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.BindingHash = trustTestBytes32(0x0c)
		},
		"operation ID": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.OperationID = trustTestBytes32(0x0c)
		},
		"transition digest": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.TransitionDigest = trustTestBytes32(0x0c)
		},
		"checkpoint": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.Checkpoint.Generation = 2
		},
		"service epoch": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.ServiceEpoch = 2
		},
		"revision": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.Revision = 2
		},
		"previous event root": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.PreviousEventRoot = trustTestBytes32(0x0c)
		},
		"tampered acknowledgement": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.AcknowledgementJSON[len(record.AcknowledgementJSON)-2] ^= 0x01
		},
		"missing acknowledgement": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.AcknowledgementJSON = nil
		},
		"missing read recovery": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.ReadRecoveryJSON = nil
		},
		"expired read recovery": func(record *FrostNativeSignerStateWitnessAnchorRecord) {
			record.ReadRecoveryExpires = 0
		},
	}
	for name, mutate := range divergences {
		t.Run("diverging "+name, func(t *testing.T) {
			if _, err := InitializeFrostNativeSignerAnchorBootstrap(
				context.Background(),
				core,
				coreSignature,
				newClient(mutate),
			); err == nil {
				t.Fatalf("client result with diverging %s was accepted", name)
			}
		})
	}

	t.Run("client error propagates", func(t *testing.T) {
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			coreSignature,
			&bootstrapProvisioningTestClient{
				t:   t,
				err: fmt.Errorf("transport failed"),
			},
		); err == nil || !strings.Contains(err.Error(), "transport failed") {
			t.Fatalf("client error was not propagated: %v", err)
		}
	})
	t.Run("nil client result", func(t *testing.T) {
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			coreSignature,
			&bootstrapProvisioningTestClient{
				t: t,
				result: &FrostNativeSignerAnchorBootstrapClientResult{
					Record: nil,
				},
			},
		); err == nil {
			t.Fatal("client result without a record was accepted")
		}
	})
	t.Run("nil client", func(t *testing.T) {
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			coreSignature,
			nil,
		); err == nil {
			t.Fatal("nil bootstrap client was accepted")
		}
	})
	t.Run("final-stage signature rejected", func(t *testing.T) {
		wrongStage := bootstrapProvisioningTestDetachedSignature(
			fixture.authority,
			FrostNativeSignerAnchorBootstrapFinalSignatureStage,
			core.CoreDigest,
		)
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			wrongStage,
			client,
		); err == nil {
			t.Fatal("final-stage signature authorized the core stage")
		}
	})
	t.Run("signature digest mismatch rejected", func(t *testing.T) {
		wrongDigest := bootstrapProvisioningTestDetachedSignature(
			fixture.authority,
			FrostNativeSignerAnchorBootstrapCoreSignatureStage,
			trustTestBytes32(0x0c),
		)
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			wrongDigest,
			client,
		); err == nil {
			t.Fatal("signature over another digest was accepted")
		}
	})
	t.Run("tampered signature rejected", func(t *testing.T) {
		tampered := *coreSignature
		tampered.Signature[0] ^= 0x01
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			&tampered,
			client,
		); err == nil {
			t.Fatal("tampered core signature was accepted")
		}
	})
	t.Run("non-authority signature rejected", func(t *testing.T) {
		foreign := bootstrapProvisioningTestDetachedSignature(
			fixture.response,
			FrostNativeSignerAnchorBootstrapCoreSignatureStage,
			core.CoreDigest,
		)
		if _, err := InitializeFrostNativeSignerAnchorBootstrap(
			context.Background(),
			core,
			foreign,
			client,
		); err == nil {
			t.Fatal("non-authority core signature was accepted")
		}
	})
}

func TestFrostNativeSignerAnchorBootstrapFinalArtifactCodec(t *testing.T) {
	ceremony := runBootstrapProvisioningTestCeremony(t)
	encoded, err := EncodeFrostNativeSignerAnchorBootstrapFinalArtifact(
		ceremony.final,
	)
	if err != nil {
		t.Fatalf("valid bootstrap final artifact was rejected: %v", err)
	}
	decoded, err := DecodeFrostNativeSignerAnchorBootstrapFinalArtifact(encoded)
	if err != nil {
		t.Fatalf("canonical bootstrap final artifact was rejected: %v", err)
	}
	if !reflect.DeepEqual(decoded, ceremony.final) {
		t.Fatalf("bootstrap final artifact round trip diverged: %+v", decoded)
	}

	tests := map[string]struct {
		mutate func(*frostNativeSignerAnchorBootstrapFinalArtifactWire)
	}{
		"wrong schema": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapFinalArtifactWire) {
				wire.Schema = FrostNativeSignerAnchorBootstrapCoreArtifactSchema
			},
		},
		"tampered core signature": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapFinalArtifactWire) {
				tampered := ceremony.final.CoreSignature
				tampered[0] ^= 0x01
				wire.CoreSignature =
					base64StdEncoding(tampered[:])
			},
		},
		"acknowledgement digest mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapFinalArtifactWire) {
				wire.TargetAcknowledgementSHA256 =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"tampered acknowledgement with recomputed digest": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapFinalArtifactWire) {
				tampered := append(
					[]byte{},
					ceremony.final.TargetAcknowledgement...,
				)
				tampered[len(tampered)-2] ^= 0x01
				wire.TargetAcknowledgementBase64 = base64StdEncoding(tampered)
				wire.TargetAcknowledgementSHA256 =
					frostNativeSignerAnchorHex32(sha256.Sum256(tampered))
			},
		},
		"final digest mismatch": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapFinalArtifactWire) {
				wire.FinalDigest =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"target reference revision divergence": {
			mutate: func(wire *frostNativeSignerAnchorBootstrapFinalArtifactWire) {
				wire.TargetReference.Revision = "2"
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wire := frostNativeSignerAnchorBootstrapFinalArtifactWire{}
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			test.mutate(&wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeFrostNativeSignerAnchorBootstrapFinalArtifact(
				payload,
			); err == nil {
				t.Fatalf("bootstrap final artifact with %s was accepted", name)
			}
		})
	}
}

func TestFrostNativeSignerAnchorBootstrapFinalize(t *testing.T) {
	ceremony := runBootstrapProvisioningTestCeremony(t)
	bundle, err := DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
		ceremony.bundle,
	)
	if err != nil {
		t.Fatalf("canonical bootstrap output bundle was rejected: %v", err)
	}
	if bundle.Schema != FrostNativeSignerAnchorBootstrapOutputBundleSchema ||
		len(bundle.CertificateChain) != 1 ||
		bundle.CertificateChain[0].CertificateDigest !=
			bundle.CertificateDigest ||
		bundle.CertificateChain[0].Kind !=
			FrostNativeSignerAnchorTrustCertificateBootstrap {
		t.Fatalf("unexpected bootstrap output bundle: %+v", bundle)
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificate(
		&bundle.CertificateChain[0],
		ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement,
	); err != nil {
		t.Fatalf("bundled certificate failed full validation: %v", err)
	}
	signerConfig := map[string]interface{}{}
	if err := json.Unmarshal(bundle.SignerConfigJSON, &signerConfig); err != nil {
		t.Fatal(err)
	}
	if signerConfig["purpose"] != "normal_signer" ||
		signerConfig["profile"] != "production" ||
		signerConfig["state_anchor_trust_certificate_digest"] !=
			frostNativeSignerAnchorHex32(bundle.CertificateDigest) {
		t.Fatalf("unexpected bundled signer config: %v", signerConfig)
	}

	t.Run("core-stage signature rejected", func(t *testing.T) {
		wrongStage := bootstrapProvisioningTestDetachedSignature(
			ceremony.fixture.authority,
			FrostNativeSignerAnchorBootstrapCoreSignatureStage,
			ceremony.final.FinalDigest,
		)
		if _, err := FinalizeFrostNativeSignerAnchorBootstrap(
			ceremony.final,
			wrongStage,
			ceremony.baseConfig,
		); err == nil {
			t.Fatal("core-stage signature authorized the final stage")
		}
	})
	t.Run("tampered final signature rejected", func(t *testing.T) {
		tampered := *ceremony.finalSignature
		tampered.Signature[0] ^= 0x01
		if _, err := FinalizeFrostNativeSignerAnchorBootstrap(
			ceremony.final,
			&tampered,
			ceremony.baseConfig,
		); err == nil {
			t.Fatal("tampered final signature was accepted")
		}
	})
	t.Run("tampered certificate rejected", func(t *testing.T) {
		tampered := *ceremony.final
		tampered.TargetReference.EventRoot = trustTestBytes32(0x0c)
		if _, err := FinalizeFrostNativeSignerAnchorBootstrap(
			&tampered,
			ceremony.finalSignature,
			ceremony.baseConfig,
		); err == nil {
			t.Fatal("final artifact with a tampered target reference was accepted")
		}
	})
	t.Run("nil final artifact rejected", func(t *testing.T) {
		if _, err := FinalizeFrostNativeSignerAnchorBootstrap(
			nil,
			ceremony.finalSignature,
			ceremony.baseConfig,
		); err == nil {
			t.Fatal("nil final artifact was accepted")
		}
	})

	baseConfigTests := map[string]string{
		"missing profile": `{"state_path":"/var/lib/keep/tbtc-signer"}`,
		"non-production profile": `{"profile":"development",` +
			`"state_path":"/var/lib/keep/tbtc-signer"}`,
		"missing state path": `{"profile":"production"}`,
		"relative state path": `{"profile":"production",` +
			`"state_path":"var/lib/keep/tbtc-signer"}`,
		"non-canonical state path": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/../keep/tbtc-signer"}`,
		"conflicting purpose": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer",` +
			`"purpose":"state_anchor_bootstrap_provisioning"}`,
		"conflicting certified field": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer",` +
			`"state_anchor_protocol_id":"0x00"}`,
		"non-canonical number": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer","retries":1.5}`,
		"negative number": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer","retries":-1}`,
		"leading-zero number": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer","retries":01}`,
		"duplicate member": `{"profile":"production","profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer"}`,
		"trailing data": `{"profile":"production",` +
			`"state_path":"/var/lib/keep/tbtc-signer"} {}`,
		"non-object": `["profile"]`,
	}
	for name, baseConfig := range baseConfigTests {
		t.Run("base config "+name, func(t *testing.T) {
			if _, err := FinalizeFrostNativeSignerAnchorBootstrap(
				ceremony.final,
				ceremony.finalSignature,
				[]byte(baseConfig),
			); err == nil {
				t.Fatalf("base config with %s was accepted", name)
			}
		})
	}
}

func TestFrostNativeSignerAnchorBootstrapOutputBundleDecode(t *testing.T) {
	ceremony := runBootstrapProvisioningTestCeremony(t)
	rehash := func(wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
		wire.CertificateChainSHA256 = frostNativeSignerAnchorHex32(
			sha256.Sum256(wire.CertificateChain),
		)
		wire.SignerConfigSHA256 = frostNativeSignerAnchorHex32(
			sha256.Sum256(wire.SignerConfig),
		)
	}
	tests := map[string]struct {
		mutate func(*testing.T, *frostNativeSignerAnchorBootstrapOutputBundleWire)
	}{
		"wrong schema": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				wire.Schema = FrostNativeSignerAnchorBootstrapFinalArtifactSchema
			},
		},
		"certificate chain digest mismatch": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				wire.CertificateChainSHA256 =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"signer config digest mismatch": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				wire.SignerConfigSHA256 =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"zero certificate digest": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				wire.CertificateDigest =
					frostNativeSignerAnchorHex32([32]byte{})
			},
		},
		"certificate digest differs from chain head": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				wire.CertificateDigest =
					frostNativeSignerAnchorHex32(trustTestBytes32(0x0c))
			},
		},
		"tampered certificate with recomputed digest": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				// Flip one nibble of the embedded core signature while
				// keeping canonical hex, then recompute the chain hash so
				// only certificate validation can reject the bundle.
				chain := []json.RawMessage{}
				if err := json.Unmarshal(
					wire.CertificateChain,
					&chain,
				); err != nil || len(chain) != 1 {
					t.Fatal("cannot parse bundled certificate chain")
				}
				certificate := map[string]json.RawMessage{}
				if err := json.Unmarshal(chain[0], &certificate); err != nil {
					t.Fatal(err)
				}
				var coreSignature string
				if err := json.Unmarshal(
					certificate["coreSignature"],
					&coreSignature,
				); err != nil {
					t.Fatal(err)
				}
				flipped := []byte(coreSignature)
				if flipped[2] == 'f' {
					flipped[2] = 'e'
				} else {
					flipped[2] = 'f'
				}
				encoded, err := json.Marshal(string(flipped))
				if err != nil {
					t.Fatal(err)
				}
				certificate["coreSignature"] = encoded
				mutated, err := json.Marshal(certificate)
				if err != nil {
					t.Fatal(err)
				}
				wire.CertificateChain, err = json.Marshal(
					[]json.RawMessage{mutated},
				)
				if err != nil {
					t.Fatal(err)
				}
				rehash(wire)
			},
		},
		"duplicated chain with recomputed digest": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				chain := []json.RawMessage{}
				if err := json.Unmarshal(
					wire.CertificateChain,
					&chain,
				); err != nil || len(chain) != 1 {
					t.Fatal("cannot parse bundled certificate chain")
				}
				duplicated, err := json.Marshal(
					[]json.RawMessage{chain[0], chain[0]},
				)
				if err != nil {
					t.Fatal(err)
				}
				wire.CertificateChain = duplicated
				rehash(wire)
			},
		},
		"non-canonical signer config with recomputed digest": {
			mutate: func(t *testing.T, wire *frostNativeSignerAnchorBootstrapOutputBundleWire) {
				wire.SignerConfig = append(
					[]byte(" "),
					wire.SignerConfig...,
				)
				rehash(wire)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wire := frostNativeSignerAnchorBootstrapOutputBundleWire{}
			if err := json.Unmarshal(ceremony.bundle, &wire); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, &wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
				payload,
			); err == nil {
				t.Fatalf("bootstrap output bundle with %s was accepted", name)
			}
		})
	}

	strictTests := map[string]string{
		"trailing data": string(ceremony.bundle) + " {}",
		"duplicate member": strings.Replace(
			string(ceremony.bundle),
			`"schema"`,
			`"schema":"x","schema"`,
			1,
		),
		"empty payload": "",
	}
	for name, payload := range strictTests {
		t.Run("strict "+name, func(t *testing.T) {
			if _, err := DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
				[]byte(payload),
			); err == nil {
				t.Fatalf(
					"bootstrap output bundle payload with %s was accepted",
					name,
				)
			}
		})
	}
}

// TestFrostNativeSignerAnchorRejectsRustInvalidWitnessGeometry pins every Go
// intake that pre-verifies a witness geometry against the exact bound the
// native signer enforces. Go mints the offline-authority-signed trust
// certificate from the operator plan, so a geometry only Go accepts survives
// the entire offline ceremony and is first rejected by the signer at node
// startup, which can only be undone by re-running the ceremony. The listed
// geometries are the ones the retired two-record reserve accepted and the
// signer's six-record terminal reserve does not.
func TestFrostNativeSignerAnchorRejectsRustInvalidWitnessGeometry(t *testing.T) {
	var geometries = map[string]struct {
		maximumRecords           uint64
		rotationThresholdRecords uint64
	}{
		"threshold inside the terminal reserve": {
			maximumRecords:           64,
			rotationThresholdRecords: 60,
		},
		"threshold one record inside the terminal reserve": {
			maximumRecords:           1000,
			rotationThresholdRecords: 995,
		},
		"maximum below the terminal reserve": {
			maximumRecords:           4,
			rotationThresholdRecords: 2,
		},
	}

	for geometryName, geometry := range geometries {
		t.Run(geometryName, func(t *testing.T) {
			fixture := newBootstrapProvisioningTestFixture()
			identity := fixture.plan.Identity
			identity.WitnessMaximumRecords = geometry.maximumRecords
			identity.WitnessRotationThresholdRecords =
				geometry.rotationThresholdRecords
			identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)

			// The operator plan is the ceremony input; rejecting it here is what
			// keeps an unusable geometry from ever reaching the offline
			// authority's signature.
			plan := *fixture.plan
			plan.Identity = identity
			if err := validateFrostNativeSignerAnchorBootstrapPlan(
				&plan,
			); err == nil || !strings.Contains(err.Error(), "witness geometry") {
				t.Fatalf(
					"bootstrap plan with a signer-invalid geometry was accepted: [%v]",
					err,
				)
			}

			// The certificate endpoint is verified again on every trust-chain
			// intake, including the one that installs an already-signed
			// certificate, so it must reject the same geometry.
			certificate, _ := trustTestBootstrapCertificate(t)
			certificate.To.WitnessMaximumRecords = geometry.maximumRecords
			certificate.To.WitnessRotationThresholdRecords =
				geometry.rotationThresholdRecords
			if err := frostNativeSignerAnchorTrustValidateEndpoint(
				&certificate.To,
				certificate.SignerStoreFingerprint,
				"target",
			); err == nil || !strings.Contains(err.Error(), "witness geometry") {
				t.Fatalf(
					"trust certificate endpoint with a signer-invalid geometry was accepted: [%v]",
					err,
				)
			}

			if err := frostsigning.ValidateNativeTBTCSignerStateWitnessGeometry(
				geometry.maximumRecords,
				geometry.rotationThresholdRecords,
			); err == nil {
				t.Fatal("shared witness geometry bound accepted a signer-invalid geometry")
			}
		})
	}
}
