package covenantsigner

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-common/pkg/persistence"
)

type memoryDescriptor struct {
	name      string
	directory string
	content   []byte
}

func (md *memoryDescriptor) Name() string      { return md.name }
func (md *memoryDescriptor) Directory() string { return md.directory }
func (md *memoryDescriptor) Content() ([]byte, error) {
	return md.content, nil
}

type memoryHandle struct {
	items map[string]*memoryDescriptor
}

func newMemoryHandle() *memoryHandle {
	return &memoryHandle{items: make(map[string]*memoryDescriptor)}
}

func (mh *memoryHandle) key(directory, name string) string {
	return directory + "/" + name
}

func (mh *memoryHandle) Save(data []byte, directory string, name string) error {
	mh.items[mh.key(directory, name)] = &memoryDescriptor{
		name:      name,
		directory: directory,
		content:   append([]byte{}, data...),
	}
	return nil
}

func (mh *memoryHandle) Delete(directory string, name string) error {
	delete(mh.items, mh.key(directory, name))
	return nil
}

func (mh *memoryHandle) ReadAll() (<-chan persistence.DataDescriptor, <-chan error) {
	dataChan := make(chan persistence.DataDescriptor, len(mh.items))
	errorChan := make(chan error)
	for _, item := range mh.items {
		dataChan <- item
	}
	close(dataChan)
	close(errorChan)
	return dataChan, errorChan
}

type faultingMemoryHandle struct {
	*memoryHandle
	saveErrByName   map[string]error
	deleteErrByName map[string]error
}

func newFaultingMemoryHandle() *faultingMemoryHandle {
	return &faultingMemoryHandle{
		memoryHandle:    newMemoryHandle(),
		saveErrByName:   make(map[string]error),
		deleteErrByName: make(map[string]error),
	}
}

func (fmh *faultingMemoryHandle) Save(data []byte, directory string, name string) error {
	if err, ok := fmh.saveErrByName[name]; ok {
		return err
	}

	return fmh.memoryHandle.Save(data, directory, name)
}

func (fmh *faultingMemoryHandle) Delete(directory string, name string) error {
	if err, ok := fmh.deleteErrByName[name]; ok {
		return err
	}

	return fmh.memoryHandle.Delete(directory, name)
}

type scriptedEngine struct {
	submit func(*Job) (*Transition, error)
	poll   func(*Job) (*Transition, error)
}

func (se *scriptedEngine) OnSubmit(_ context.Context, job *Job) (*Transition, error) {
	if se.submit == nil {
		return nil, nil
	}
	return se.submit(job)
}

func (se *scriptedEngine) OnPoll(_ context.Context, job *Job) (*Transition, error) {
	if se.poll == nil {
		return nil, nil
	}
	return se.poll(job)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type approvalContractVector struct {
	CanonicalSubmitRequest json.RawMessage `json:"canonicalSubmitRequest"`
	ExpectedApprovalDigest string          `json:"expectedApprovalDigest"`
	ExpectedRequestDigest  string          `json:"expectedRequestDigest"`
}

type approvalContractVectorsFile struct {
	Version int                               `json:"version"`
	Scope   string                            `json:"scope"`
	Vectors map[string]approvalContractVector `json:"vectors"`
}

type migrationPlanQuoteSigningVector struct {
	UnsignedQuote     MigrationDestinationPlanQuote `json:"unsignedQuote"`
	ExpectedPayload   string                        `json:"expectedPayload"`
	ExpectedPreimage  string                        `json:"expectedPreimage"`
	ExpectedHash      string                        `json:"expectedHash"`
	ExpectedSignature string                        `json:"expectedSignature"`
}

type migrationPlanQuoteSigningVectorsFile struct {
	Version   int                                        `json:"version"`
	Scope     string                                     `json:"scope"`
	TrustRoot MigrationPlanQuoteTrustRoot                `json:"trustRoot"`
	Vectors   map[string]migrationPlanQuoteSigningVector `json:"vectors"`
}

func loadApprovalContractVector(
	t *testing.T,
	key string,
) (RouteSubmitRequest, string, string) {
	t.Helper()

	data, err := os.ReadFile("testdata/covenant_recovery_approval_vectors_v1.json")
	if err != nil {
		t.Fatal(err)
	}

	vectors := approvalContractVectorsFile{}
	if err := strictUnmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if vectors.Version != 1 {
		t.Fatalf("unexpected vector version: %d", vectors.Version)
	}
	if vectors.Scope != "covenant_recovery_approval_contract_v1" {
		t.Fatalf("unexpected vector scope: %s", vectors.Scope)
	}

	vector, ok := vectors.Vectors[key]
	if !ok {
		t.Fatalf("missing vector %s", key)
	}

	request := RouteSubmitRequest{}
	if err := strictUnmarshal(vector.CanonicalSubmitRequest, &request); err != nil {
		t.Fatal(err)
	}

	return request, vector.ExpectedApprovalDigest, vector.ExpectedRequestDigest
}

func loadMigrationPlanQuoteSigningVectors(
	t *testing.T,
) migrationPlanQuoteSigningVectorsFile {
	t.Helper()

	data, err := os.ReadFile("testdata/migration_plan_quote_signing_vectors_v1.json")
	if err != nil {
		t.Fatal(err)
	}

	vectors := migrationPlanQuoteSigningVectorsFile{}
	if err := strictUnmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}

	return vectors
}

const (
	testDepositorPrivateKeyHex = "0x1111111111111111111111111111111111111111111111111111111111111111"
	testSignerPrivateKeyHex    = "0x2222222222222222222222222222222222222222222222222222222222222222"
	testCustodianPrivateKeyHex = "0x3333333333333333333333333333333333333333333333333333333333333333"
)

var (
	testDepositorPrivateKey          = mustDeterministicTestPrivateKey(testDepositorPrivateKeyHex)
	testSignerPrivateKey             = mustDeterministicTestPrivateKey(testSignerPrivateKeyHex)
	testCustodianPrivateKey          = mustDeterministicTestPrivateKey(testCustodianPrivateKeyHex)
	testDepositorPublicKey           = mustCompressedPublicKeyHex(testDepositorPrivateKey)
	testSignerPublicKey              = mustCompressedPublicKeyHex(testSignerPrivateKey)
	testSignerUncompressedPublicKey  = mustUncompressedPublicKeyHex(testSignerPrivateKey)
	testCustodianPublicKey           = mustCompressedPublicKeyHex(testCustodianPrivateKey)
	testMigrationPlanQuoteSeed       = bytes.Repeat([]byte{0x44}, ed25519.SeedSize)
	testMigrationPlanQuotePrivateKey = ed25519.NewKeyFromSeed(testMigrationPlanQuoteSeed)
	testMigrationPlanQuoteTrustRoot  = MigrationPlanQuoteTrustRoot{
		KeyID:        "test-plan-quote-key",
		PublicKeyPEM: mustMigrationPlanQuoteTrustRootPEM(testMigrationPlanQuotePrivateKey.Public().(ed25519.PublicKey)),
	}
)

func testDepositorTrustRoot(route TemplateID) DepositorTrustRoot {
	migrationDestination := validMigrationDestination()

	return DepositorTrustRoot{
		Route:     route,
		Reserve:   migrationDestination.Reserve,
		Network:   migrationDestination.Network,
		PublicKey: testDepositorPublicKey,
	}
}

func testCustodianTrustRoot(route TemplateID) CustodianTrustRoot {
	migrationDestination := validMigrationDestination()

	return CustodianTrustRoot{
		Route:     route,
		Reserve:   migrationDestination.Reserve,
		Network:   migrationDestination.Network,
		PublicKey: testCustodianPublicKey,
	}
}

func mustDeterministicTestPrivateKey(encoded string) *btcec.PrivateKey {
	rawPrivateKey, err := hex.DecodeString(strings.TrimPrefix(encoded, "0x"))
	if err != nil {
		panic(err)
	}

	privateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), rawPrivateKey)
	return privateKey
}

func mustCompressedPublicKeyHex(privateKey *btcec.PrivateKey) string {
	return "0x" + hex.EncodeToString(privateKey.PubKey().SerializeCompressed())
}

func mustUncompressedPublicKeyHex(privateKey *btcec.PrivateKey) string {
	return "0x" + hex.EncodeToString(privateKey.PubKey().SerializeUncompressed())
}

func mustMigrationPlanQuoteTrustRootPEM(publicKey ed25519.PublicKey) string {
	encodedPublicKey, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: encodedPublicKey,
	}))
}

func mustArtifactApprovalSignature(
	privateKey *btcec.PrivateKey,
	payload ArtifactApprovalPayload,
) string {
	digest, err := artifactApprovalDigest(payload)
	if err != nil {
		panic(err)
	}

	signature, err := privateKey.Sign(digest)
	if err != nil {
		panic(err)
	}

	return "0x" + hex.EncodeToString(signature.Serialize())
}

func artifactApprovalSignatureByRole(
	artifactApprovals *ArtifactApprovalEnvelope,
	role ArtifactApprovalRole,
) string {
	for _, approval := range artifactApprovals.Approvals {
		if approval.Role == role {
			return approval.Signature
		}
	}

	panic(fmt.Sprintf("missing approval role %s", role))
}

func setArtifactApprovalSignature(
	artifactApprovals *ArtifactApprovalEnvelope,
	role ArtifactApprovalRole,
	signature string,
) {
	for i, approval := range artifactApprovals.Approvals {
		if approval.Role == role {
			artifactApprovals.Approvals[i].Signature = signature
			return
		}
	}

	panic(fmt.Sprintf("missing approval role %s", role))
}

func canonicalArtifactSignatures(
	route TemplateID,
	artifactApprovals *ArtifactApprovalEnvelope,
) []string {
	if artifactApprovals == nil {
		return nil
	}

	requiredRoles, err := requiredStructuredArtifactApprovalRoles(route)
	if err != nil {
		panic(err)
	}

	signatures := make([]string, len(requiredRoles))
	for i, role := range requiredRoles {
		signatures[i] = artifactApprovalSignatureByRole(artifactApprovals, role)
	}

	return signatures
}

func canonicalArtifactSignaturesWithSignerApproval(
	route TemplateID,
	artifactApprovals *ArtifactApprovalEnvelope,
	signerApproval *SignerApprovalCertificate,
) []string {
	requiredRoles, err := requiredStructuredArtifactApprovalRoles(route)
	if err != nil {
		panic(err)
	}

	signatures := make([]string, 0, len(requiredRoles)+1)
	for _, role := range requiredRoles {
		signatures = append(
			signatures,
			artifactApprovalSignatureByRole(artifactApprovals, role),
		)
	}
	if signerApproval == nil {
		return signatures
	}

	return append(signatures, signerApproval.Signature)
}

func validSelfTemplate() json.RawMessage {
	return mustTemplate(SelfV1Template{
		Template:           TemplateSelfV1,
		DepositorPublicKey: testDepositorPublicKey,
		SignerPublicKey:    testSignerPublicKey,
		Delta2:             4320,
	})
}

func validQcTemplate() json.RawMessage {
	return mustTemplate(QcV1Template{
		Template:           TemplateQcV1,
		DepositorPublicKey: testDepositorPublicKey,
		CustodianPublicKey: testCustodianPublicKey,
		SignerPublicKey:    testSignerPublicKey,
		Beta:               144,
		Delta2:             4320,
	})
}

func mustTemplate(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func baseRequest(route TemplateID) RouteSubmitRequest {
	migrationDestination := validMigrationDestination()
	request := RouteSubmitRequest{
		FacadeRequestID:           "rf_123",
		IdempotencyKey:            "idem_123",
		RequestType:               RequestTypeReconstruct,
		Route:                     route,
		Strategy:                  "0x1234",
		Reserve:                   migrationDestination.Reserve,
		Epoch:                     12,
		MaturityHeight:            912345,
		ActiveOutpoint:            CovenantOutpoint{TxID: "0x0102", Vout: 1, ScriptHash: "0x0304"},
		DestinationCommitmentHash: migrationDestination.DestinationCommitmentHash,
		MigrationDestination:      migrationDestination,
		MigrationTransactionPlan: &MigrationTransactionPlan{
			PlanVersion:          migrationTransactionPlanVersion,
			InputValueSats:       1000000,
			DestinationValueSats: 998000,
			AnchorValueSats:      canonicalAnchorValueSats,
			FeeSats:              1670,
			InputSequence:        canonicalCovenantInputSequence,
			LockTime:             912345,
		},
		Artifacts: map[RecoveryPathID]ArtifactRecord{},
	}

	switch route {
	case TemplateSelfV1:
		request.ScriptTemplate = validSelfTemplate()
		request.Signing = SigningRequirements{SignerRequired: true, CustodianRequired: false}
	case TemplateQcV1:
		request.ScriptTemplate = validQcTemplate()
		request.Signing = SigningRequirements{SignerRequired: true, CustodianRequired: true}
	}

	request.MigrationTransactionPlan.PlanCommitmentHash, _ =
		computeMigrationTransactionPlanCommitmentHash(
			request,
			request.MigrationTransactionPlan,
		)
	request.ArtifactApprovals = validArtifactApprovals(request)
	request.ArtifactSignatures = canonicalArtifactSignatures(
		request.Route,
		request.ArtifactApprovals,
	)

	return request
}

func validArtifactApprovals(request RouteSubmitRequest) *ArtifactApprovalEnvelope {
	payload := ArtifactApprovalPayload{
		ApprovalVersion:           artifactApprovalVersion,
		Route:                     request.Route,
		ScriptTemplateID:          request.Route,
		DestinationCommitmentHash: request.DestinationCommitmentHash,
		PlanCommitmentHash:        request.MigrationTransactionPlan.PlanCommitmentHash,
	}

	approvals := []ArtifactRoleApproval{
		{
			Role:      ArtifactApprovalRoleDepositor,
			Signature: mustArtifactApprovalSignature(testDepositorPrivateKey, payload),
		},
	}

	if request.Route == TemplateQcV1 {
		approvals = []ArtifactRoleApproval{
			{
				Role:      ArtifactApprovalRoleDepositor,
				Signature: mustArtifactApprovalSignature(testDepositorPrivateKey, payload),
			},
			{
				Role:      ArtifactApprovalRoleCustodian,
				Signature: mustArtifactApprovalSignature(testCustodianPrivateKey, payload),
			},
		}
	}

	return &ArtifactApprovalEnvelope{
		Payload:   payload,
		Approvals: approvals,
	}
}

func validSignerApproval(
	artifactApprovals *ArtifactApprovalEnvelope,
) *SignerApprovalCertificate {
	if artifactApprovals == nil {
		panic("artifact approvals are required")
	}

	digest, err := artifactApprovalDigest(artifactApprovals.Payload)
	if err != nil {
		panic(err)
	}

	endBlock := uint64(123456)
	return &SignerApprovalCertificate{
		CertificateVersion: signerApprovalCertificateVersion,
		SignatureAlgorithm: signerApprovalSignatureAlgorithm,
		ApprovalDigest:     "0x" + hex.EncodeToString(digest),
		WalletPublicKey:    testSignerUncompressedPublicKey,
		SignerSetHash:      "0x" + strings.Repeat("ab", 32),
		Signature:          "0x304402200102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2002202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f40",
		ActiveMembers:      []uint32{2, 1},
		InactiveMembers:    []uint32{4, 3},
		EndBlock:           &endBlock,
	}
}

func structuredSignerApprovalRequest(route TemplateID) RouteSubmitRequest {
	request := baseRequest(route)
	request.SignerApproval = validSignerApproval(request.ArtifactApprovals)
	request.ArtifactSignatures = canonicalArtifactSignaturesWithSignerApproval(
		request.Route,
		request.ArtifactApprovals,
		request.SignerApproval,
	)

	return request
}

func canonicalArtifactApprovalRequest(route TemplateID) RouteSubmitRequest {
	return baseRequest(route)
}

const (
	mixedCaseCoverageStrategy = "0xaabbccddeeff00112233445566778899aabbccdd"
	mixedCaseCoverageReserve  = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	mixedCaseCoverageRevealer = "0xdecafbaddecafbaddecafbaddecafbaddecafbad"
	mixedCaseCoverageVault    = "0xbeadfeedbeadfeedbeadfeedbeadfeedbeadfeed"
)

func canonicalMixedCaseCoverageArtifactApprovalRequest(
	t *testing.T,
	route TemplateID,
) RouteSubmitRequest {
	t.Helper()

	request := canonicalArtifactApprovalRequest(route)
	request.Strategy = mixedCaseCoverageStrategy
	request.Reserve = mixedCaseCoverageReserve
	request.MigrationDestination.Reserve = mixedCaseCoverageReserve
	request.MigrationDestination.Revealer = mixedCaseCoverageRevealer
	request.MigrationDestination.Vault = mixedCaseCoverageVault
	request.MigrationDestination.MigrationExtraData = computeMigrationExtraData(
		request.MigrationDestination.Revealer,
	)

	destinationCommitmentHash, err := computeDestinationCommitmentHash(
		request.MigrationDestination,
	)
	if err != nil {
		t.Fatal(err)
	}
	request.MigrationDestination.DestinationCommitmentHash = destinationCommitmentHash
	request.DestinationCommitmentHash = destinationCommitmentHash

	planCommitmentHash, err := computeMigrationTransactionPlanCommitmentHash(
		request,
		request.MigrationTransactionPlan,
	)
	if err != nil {
		t.Fatal(err)
	}
	request.MigrationTransactionPlan.PlanCommitmentHash = planCommitmentHash
	request.ArtifactApprovals.Payload.DestinationCommitmentHash = destinationCommitmentHash
	request.ArtifactApprovals.Payload.PlanCommitmentHash = planCommitmentHash

	return request
}

func cloneRouteSubmitRequest(
	t *testing.T,
	request RouteSubmitRequest,
) RouteSubmitRequest {
	t.Helper()

	cloned := RouteSubmitRequest{}
	if err := strictUnmarshal(mustJSON(t, request), &cloned); err != nil {
		t.Fatal(err)
	}

	return cloned
}

func artifactApprovalVariantFromRequest(
	t *testing.T,
	request RouteSubmitRequest,
	transformHex func(string) string,
) RouteSubmitRequest {
	t.Helper()

	variant := cloneRouteSubmitRequest(t, request)
	variant.Strategy = transformHex(variant.Strategy)
	variant.Reserve = transformHex(variant.Reserve)
	variant.ActiveOutpoint.TxID = transformHex(variant.ActiveOutpoint.TxID)
	if variant.ActiveOutpoint.ScriptHash != "" {
		variant.ActiveOutpoint.ScriptHash = transformHex(variant.ActiveOutpoint.ScriptHash)
	}
	variant.DestinationCommitmentHash = transformHex(variant.DestinationCommitmentHash)

	if variant.MigrationDestination != nil {
		variant.MigrationDestination.Reserve = transformHex(variant.MigrationDestination.Reserve)
		variant.MigrationDestination.Revealer = transformHex(variant.MigrationDestination.Revealer)
		variant.MigrationDestination.Vault = transformHex(variant.MigrationDestination.Vault)
		variant.MigrationDestination.DepositScript = transformHex(variant.MigrationDestination.DepositScript)
		variant.MigrationDestination.DepositScriptHash = transformHex(variant.MigrationDestination.DepositScriptHash)
		variant.MigrationDestination.MigrationExtraData = transformHex(variant.MigrationDestination.MigrationExtraData)
		variant.MigrationDestination.DestinationCommitmentHash = transformHex(
			variant.MigrationDestination.DestinationCommitmentHash,
		)
	}

	if variant.MigrationTransactionPlan != nil {
		variant.MigrationTransactionPlan.PlanCommitmentHash = transformHex(
			variant.MigrationTransactionPlan.PlanCommitmentHash,
		)
	}

	for i := range variant.ArtifactSignatures {
		variant.ArtifactSignatures[i] = transformHex(variant.ArtifactSignatures[i])
	}

	if variant.SignerApproval != nil {
		variant.SignerApproval.ApprovalDigest = transformHex(
			variant.SignerApproval.ApprovalDigest,
		)
		variant.SignerApproval.WalletPublicKey = transformHex(
			variant.SignerApproval.WalletPublicKey,
		)
		variant.SignerApproval.SignerSetHash = transformHex(
			variant.SignerApproval.SignerSetHash,
		)
		variant.SignerApproval.Signature = transformHex(
			variant.SignerApproval.Signature,
		)
		if len(variant.SignerApproval.ActiveMembers) > 1 {
			variant.SignerApproval.ActiveMembers = append(
				[]uint32{
					variant.SignerApproval.ActiveMembers[len(variant.SignerApproval.ActiveMembers)-1],
				},
				variant.SignerApproval.ActiveMembers[:len(variant.SignerApproval.ActiveMembers)-1]...,
			)
		}
		if len(variant.SignerApproval.InactiveMembers) > 1 {
			variant.SignerApproval.InactiveMembers = append(
				[]uint32{
					variant.SignerApproval.InactiveMembers[len(variant.SignerApproval.InactiveMembers)-1],
				},
				variant.SignerApproval.InactiveMembers[:len(variant.SignerApproval.InactiveMembers)-1]...,
			)
		}
	}

	for pathID, artifact := range variant.Artifacts {
		artifact.PSBTHash = transformHex(artifact.PSBTHash)
		artifact.DestinationCommitmentHash = transformHex(artifact.DestinationCommitmentHash)
		if artifact.TransactionHex != "" {
			artifact.TransactionHex = transformHex(artifact.TransactionHex)
		}
		if artifact.TransactionID != "" {
			artifact.TransactionID = transformHex(artifact.TransactionID)
		}
		variant.Artifacts[pathID] = artifact
	}

	if variant.ArtifactApprovals != nil {
		variant.ArtifactApprovals.Payload.DestinationCommitmentHash = transformHex(
			variant.ArtifactApprovals.Payload.DestinationCommitmentHash,
		)
		variant.ArtifactApprovals.Payload.PlanCommitmentHash = transformHex(
			variant.ArtifactApprovals.Payload.PlanCommitmentHash,
		)

		reorderedApprovals := make(
			[]ArtifactRoleApproval,
			len(variant.ArtifactApprovals.Approvals),
		)
		for i := range variant.ArtifactApprovals.Approvals {
			approval := variant.ArtifactApprovals.Approvals[len(variant.ArtifactApprovals.Approvals)-1-i]
			reorderedApprovals[i] = ArtifactRoleApproval{
				Role:      approval.Role,
				Signature: transformHex(approval.Signature),
			}
		}
		variant.ArtifactApprovals.Approvals = reorderedApprovals
	}

	switch variant.Route {
	case TemplateQcV1:
		template := &QcV1Template{}
		if err := strictUnmarshal(variant.ScriptTemplate, template); err != nil {
			t.Fatal(err)
		}
		template.DepositorPublicKey = transformHex(template.DepositorPublicKey)
		template.CustodianPublicKey = transformHex(template.CustodianPublicKey)
		template.SignerPublicKey = transformHex(template.SignerPublicKey)
		variant.ScriptTemplate = mustTemplate(template)
	case TemplateSelfV1:
		template := &SelfV1Template{}
		if err := strictUnmarshal(variant.ScriptTemplate, template); err != nil {
			t.Fatal(err)
		}
		template.DepositorPublicKey = transformHex(template.DepositorPublicKey)
		template.SignerPublicKey = transformHex(template.SignerPublicKey)
		variant.ScriptTemplate = mustTemplate(template)
	default:
		t.Fatalf("unsupported route %s", variant.Route)
	}

	return variant
}

func equivalentArtifactApprovalVariantFromRequest(
	t *testing.T,
	request RouteSubmitRequest,
) RouteSubmitRequest {
	t.Helper()
	return artifactApprovalVariantFromRequest(t, request, upperHexBody)
}

func upperHexBody(value string) string {
	if !strings.HasPrefix(value, "0x") {
		return strings.ToUpper(value)
	}

	return "0x" + strings.ToUpper(strings.TrimPrefix(value, "0x"))
}

func mixedCaseHexBody(value string) string {
	if !strings.HasPrefix(value, "0x") {
		return value
	}

	body := strings.ToLower(strings.TrimPrefix(value, "0x"))
	lettersSeen := 0
	variant := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'f' {
			if lettersSeen%2 == 0 {
				lettersSeen++
				return r - ('a' - 'A')
			}

			lettersSeen++
		}

		return r
	}, body)

	return "0x" + variant
}

func mixedCaseArtifactApprovalVariantFromRequest(
	t *testing.T,
	request RouteSubmitRequest,
) RouteSubmitRequest {
	t.Helper()
	return artifactApprovalVariantFromRequest(t, request, mixedCaseHexBody)
}

func validMigrationDestination() *MigrationDestinationReservation {
	reservation := &MigrationDestinationReservation{
		ReservationID: "cmdr_12345678",
		Reserve:       "0x1111111111111111111111111111111111111111",
		Epoch:         12,
		Route:         ReservationRouteMigration,
		Revealer:      "0x2222222222222222222222222222222222222222",
		Vault:         "0x3333333333333333333333333333333333333333",
		Network:       "regtest",
		Status:        ReservationStatusReserved,
		DepositScript: "0x0014aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	reservation.DepositScriptHash, _ = computeDepositScriptHash(reservation.DepositScript)
	reservation.MigrationExtraData = computeMigrationExtraData(reservation.Revealer)
	reservation.DestinationCommitmentHash, _ = computeDestinationCommitmentHash(reservation)

	return reservation
}

func validMigrationPlanQuote(
	request RouteSubmitRequest,
) *MigrationDestinationPlanQuote {
	quote := &MigrationDestinationPlanQuote{
		QuoteID:                   "cmdq_12345678",
		QuoteVersion:              migrationPlanQuoteVersion,
		ReservationID:             request.MigrationDestination.ReservationID,
		Reserve:                   request.Reserve,
		Epoch:                     request.Epoch,
		Route:                     ReservationRouteMigration,
		Revealer:                  request.MigrationDestination.Revealer,
		Vault:                     request.MigrationDestination.Vault,
		Network:                   request.MigrationDestination.Network,
		DestinationCommitmentHash: request.DestinationCommitmentHash,
		ActiveOutpointTxID:        request.ActiveOutpoint.TxID,
		ActiveOutpointVout:        request.ActiveOutpoint.Vout,
		PlanCommitmentHash:        request.MigrationTransactionPlan.PlanCommitmentHash,
		MigrationTransactionPlan:  normalizeMigrationTransactionPlan(request.MigrationTransactionPlan),
		IdempotencyKey:            "0x75a998ac6951c2776f3a85f6430fb41321c28c1113a71a52c754806c7a3de9c9",
		ExpiresInSeconds:          900,
		IssuedAt:                  "2099-03-09T00:00:00.000Z",
		ExpiresAt:                 "2099-03-09T00:15:00.000Z",
		Signature: MigrationDestinationPlanQuoteSignature{
			SignatureVersion: migrationPlanQuoteSignatureVersion,
			Algorithm:        migrationPlanQuoteSignatureAlgorithm,
			KeyID:            testMigrationPlanQuoteTrustRoot.KeyID,
		},
	}

	signingHash, err := migrationPlanQuoteSigningHash(quote)
	if err != nil {
		panic(err)
	}
	quote.Signature.Signature = "0x" + hex.EncodeToString(
		ed25519.Sign(testMigrationPlanQuotePrivateKey, signingHash),
	)

	return quote
}

func requestWithValidMigrationPlanQuote(route TemplateID) RouteSubmitRequest {
	request := baseRequest(route)
	request.ActiveOutpoint.TxID = "0x" + strings.Repeat("aa", 32)
	request.ActiveOutpoint.ScriptHash = "0x" + strings.Repeat("bb", 32)
	request.MigrationTransactionPlan.PlanCommitmentHash, _ =
		computeMigrationTransactionPlanCommitmentHash(
			request,
			request.MigrationTransactionPlan,
		)
	request.ArtifactApprovals = validArtifactApprovals(request)
	request.ArtifactSignatures = canonicalArtifactSignatures(
		request.Route,
		request.ArtifactApprovals,
	)
	request.MigrationPlanQuote = validMigrationPlanQuote(request)

	return request
}

func TestServiceSubmitDeduplicatesByRouteRequestID(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_123",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	first, err := service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}

	second, err := service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}

	if first.RequestID == "" {
		t.Fatal("expected durable request id")
	}
	if first.RequestID != second.RequestID {
		t.Fatalf("expected dedupe on routeRequestId, got %s vs %s", first.RequestID, second.RequestID)
	}
}

func TestServiceSubmitReturnsExistingJobWhileInitialEngineCallIsInFlight(t *testing.T) {
	handle := newMemoryHandle()
	engineStarted := make(chan struct{})
	releaseEngine := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-engineStarted:
			default:
				close(engineStarted)
			}

			<-releaseEngine
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_inflight",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	firstResultChan := make(chan StepResult, 1)
	firstErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			firstErrChan <- err
			return
		}

		firstResultChan <- result
	}()

	<-engineStarted

	secondResultChan := make(chan StepResult, 1)
	secondErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			secondErrChan <- err
			return
		}

		secondResultChan <- result
	}()

	var secondResult StepResult
	select {
	case err := <-secondErrChan:
		t.Fatal(err)
	case secondResult = <-secondResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected deduplicated submit to return while initial engine call is in flight")
	}

	close(releaseEngine)

	var firstResult StepResult
	select {
	case err := <-firstErrChan:
		t.Fatal(err)
	case firstResult = <-firstResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected initial submit to finish after engine release")
	}

	if firstResult.RequestID == "" {
		t.Fatal("expected durable request id on initial submit")
	}
	if firstResult.RequestID != secondResult.RequestID {
		t.Fatalf("expected in-flight dedupe to reuse request id, got %s vs %s", firstResult.RequestID, secondResult.RequestID)
	}
}

func TestServicePollReturnsNewerPersistedStateWhenItsTransitionBecomesStale(t *testing.T) {
	handle := newMemoryHandle()
	submitStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	pollStarted := make(chan struct{})
	releasePoll := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-submitStarted:
			default:
				close(submitStarted)
			}

			<-releaseSubmit
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
		poll: func(*Job) (*Transition, error) {
			select {
			case <-pollStarted:
			default:
				close(pollStarted)
			}

			<-releasePoll
			return &Transition{State: JobStatePending, Detail: "stale pending"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_poll_stale_pending",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	submitResultChan := make(chan StepResult, 1)
	submitErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			submitErrChan <- err
			return
		}

		submitResultChan <- result
	}()

	<-submitStarted

	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job to exist while submit engine is in flight")
	}

	pollResultChan := make(chan StepResult, 1)
	pollErrChan := make(chan error, 1)
	go func() {
		result, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
			RouteRequestID: input.RouteRequestID,
			RequestID:      storedJob.RequestID,
			Stage:          StageSignerCoordination,
			Request:        input.Request,
		})
		if err != nil {
			pollErrChan <- err
			return
		}

		pollResultChan <- result
	}()

	<-pollStarted
	close(releaseSubmit)

	var submitResult StepResult
	select {
	case err := <-submitErrChan:
		t.Fatal(err)
	case submitResult = <-submitResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected submit to finish after engine release")
	}

	close(releasePoll)

	var pollResult StepResult
	select {
	case err := <-pollErrChan:
		t.Fatal(err)
	case pollResult = <-pollResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected poll to finish after release")
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected stale poll to return latest READY state, got %#v", pollResult)
	}
	if pollResult.PSBTHash != submitResult.PSBTHash || pollResult.TransactionHex != submitResult.TransactionHex {
		t.Fatalf("expected poll to return persisted READY payload, got submit=%#v poll=%#v", submitResult, pollResult)
	}

	persistedJob, ok, err := service.store.GetByRequestID(storedJob.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected persisted job after stale poll")
	}
	if persistedJob.State != JobStateArtifactReady {
		t.Fatalf("expected persisted READY state, got %s", persistedJob.State)
	}
}

func TestServiceSubmitReturnsNewerPersistedStateWhenItsTransitionBecomesStale(t *testing.T) {
	handle := newMemoryHandle()
	submitStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	pollStarted := make(chan struct{})
	releasePoll := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-submitStarted:
			default:
				close(submitStarted)
			}

			<-releaseSubmit
			return &Transition{State: JobStatePending, Detail: "stale pending"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			select {
			case <-pollStarted:
			default:
				close(pollStarted)
			}

			<-releasePoll
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_submit_stale_pending",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	submitResultChan := make(chan StepResult, 1)
	submitErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			submitErrChan <- err
			return
		}

		submitResultChan <- result
	}()

	<-submitStarted

	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job to exist while submit engine is in flight")
	}

	pollResultChan := make(chan StepResult, 1)
	pollErrChan := make(chan error, 1)
	go func() {
		result, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
			RouteRequestID: input.RouteRequestID,
			RequestID:      storedJob.RequestID,
			Stage:          StageSignerCoordination,
			Request:        input.Request,
		})
		if err != nil {
			pollErrChan <- err
			return
		}

		pollResultChan <- result
	}()

	<-pollStarted
	close(releasePoll)

	var pollResult StepResult
	select {
	case err := <-pollErrChan:
		t.Fatal(err)
	case pollResult = <-pollResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected poll to finish after release")
	}

	close(releaseSubmit)

	var submitResult StepResult
	select {
	case err := <-submitErrChan:
		t.Fatal(err)
	case submitResult = <-submitResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected submit to finish after release")
	}

	if submitResult.Status != StepStatusReady {
		t.Fatalf("expected stale submit to return latest READY state, got %#v", submitResult)
	}
	if submitResult.PSBTHash != pollResult.PSBTHash || submitResult.TransactionHex != pollResult.TransactionHex {
		t.Fatalf("expected submit to return persisted READY payload, got submit=%#v poll=%#v", submitResult, pollResult)
	}

	persistedJob, ok, err := service.store.GetByRequestID(storedJob.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected persisted job after stale submit")
	}
	if persistedJob.State != JobStateArtifactReady {
		t.Fatalf("expected persisted READY state, got %s", persistedJob.State)
	}
}

func TestServicePollDoesNotOverwriteNewerPersistedStateWithJobNotFound(t *testing.T) {
	handle := newMemoryHandle()
	submitStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	pollStarted := make(chan struct{})
	releasePoll := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-submitStarted:
			default:
				close(submitStarted)
			}

			<-releaseSubmit
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x0d0e",
				TransactionHex: "0x0f10",
			}, nil
		},
		poll: func(*Job) (*Transition, error) {
			select {
			case <-pollStarted:
			default:
				close(pollStarted)
			}

			<-releasePoll
			return nil, errJobNotFound
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_poll_stale_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	submitResultChan := make(chan StepResult, 1)
	submitErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			submitErrChan <- err
			return
		}

		submitResultChan <- result
	}()

	<-submitStarted

	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job to exist while submit engine is in flight")
	}

	pollResultChan := make(chan StepResult, 1)
	pollErrChan := make(chan error, 1)
	go func() {
		result, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
			RouteRequestID: input.RouteRequestID,
			RequestID:      storedJob.RequestID,
			Stage:          StageSignerCoordination,
			Request:        input.Request,
		})
		if err != nil {
			pollErrChan <- err
			return
		}

		pollResultChan <- result
	}()

	<-pollStarted
	close(releaseSubmit)

	var submitResult StepResult
	select {
	case err := <-submitErrChan:
		t.Fatal(err)
	case submitResult = <-submitResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected submit to finish after engine release")
	}

	close(releasePoll)

	var pollResult StepResult
	select {
	case err := <-pollErrChan:
		t.Fatal(err)
	case pollResult = <-pollResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected poll to finish after release")
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected stale job-not-found poll to return latest READY state, got %#v", pollResult)
	}
	if pollResult.PSBTHash != submitResult.PSBTHash || pollResult.TransactionHex != submitResult.TransactionHex {
		t.Fatalf("expected poll to return persisted READY payload, got submit=%#v poll=%#v", submitResult, pollResult)
	}
}

func TestServicePollCanTransitionToReady(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_ready",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_ready",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected READY, got %s", pollResult.Status)
	}
	if pollResult.PSBTHash != "0x090a" || pollResult.TransactionHex != "0x0b0c" {
		t.Fatalf("unexpected ready payload: %#v", pollResult)
	}
}

func TestServiceTimestampsAdvanceAcrossTransitions(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_timestamps",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	submittedJob, ok, err := service.store.GetByRequestID(submitResult.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job")
	}

	time.Sleep(5 * time.Millisecond)

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_timestamps",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	polledJob, ok, err := service.store.GetByRequestID(submitResult.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected polled job")
	}

	if submittedJob.CreatedAt == polledJob.UpdatedAt {
		t.Fatalf("expected updated timestamp to advance, got created=%s updated=%s", submittedJob.CreatedAt, polledJob.UpdatedAt)
	}
	if polledJob.CompletedAt == "" {
		t.Fatal("expected completed timestamp to be populated")
	}
}

func TestServicePollMapsJobNotFoundToFailed(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return nil, errJobNotFound
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitResult, err := service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateQcV1, SignerPollInput{
		RouteRequestID: "orq_missing",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if pollResult.Status != StepStatusFailed || pollResult.Reason != ReasonJobNotFound {
		t.Fatalf("unexpected failed result: %#v", pollResult)
	}
}

func TestMigrationDestinationMatchesKnownVector(t *testing.T) {
	reservation := validMigrationDestination()

	if reservation.DepositScriptHash != "0x8532ec6785e391b2af968b5728d574e271c7f46658f5ed10845d9ad5b23ac6d3" {
		t.Fatalf("unexpected depositScriptHash: %s", reservation.DepositScriptHash)
	}
	if reservation.MigrationExtraData != "0x41435f4d49475241544556312222222222222222222222222222222222222222" {
		t.Fatalf("unexpected migrationExtraData: %s", reservation.MigrationExtraData)
	}
	if reservation.DestinationCommitmentHash != "0x3efc50372759413e0f1900a2340fbb947648c524e5ec3cb4cf8887ea2d7df474" {
		t.Fatalf("unexpected destinationCommitmentHash: %s", reservation.DestinationCommitmentHash)
	}
}

func TestServiceRejectsMismatchedMigrationDestinationArtifact(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateSelfV1)
	request.MigrationDestination.DepositScriptHash = "0xdeadbeef"

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_bad_reservation",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "depositScriptHash does not match depositScript") {
		t.Fatalf("expected depositScriptHash mismatch, got %v", err)
	}
}

func TestServiceRejectsInvalidMigrationDestinationVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		mutate    func(request *RouteSubmitRequest)
		expectErr string
	}{
		{
			name: "missing reservation artifact",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination = nil
			},
			expectErr: "request.migrationDestination is required",
		},
		{
			name: "wrong reservation route",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Route = "COOPERATIVE"
			},
			expectErr: "request.migrationDestination.route must be MIGRATION",
		},
		{
			name: "retired reservation status",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Status = ReservationStatusRetired
			},
			expectErr: "request.migrationDestination.status must be RESERVED or COMMITTED_TO_EPOCH",
		},
		{
			name: "epoch mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Epoch = 13
			},
			expectErr: "request.migrationDestination.epoch does not match request.epoch",
		},
		{
			name: "reserve mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Reserve = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			expectErr: "request.migrationDestination.reserve does not match request.reserve",
		},
		{
			name: "request commitment mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.DestinationCommitmentHash = "0xdeadbeef"
			},
			expectErr: "request.migrationDestination.destinationCommitmentHash does not match request.destinationCommitmentHash",
		},
		{
			name: "migration extraData mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.MigrationExtraData = "0xdeadbeef"
			},
			expectErr: "request.migrationDestination.migrationExtraData does not match migration revealer encoding",
		},
		{
			name: "canonical commitment mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.DestinationCommitmentHash = "0xdeadbeef"
				request.DestinationCommitmentHash = "0xdeadbeef"
			},
			expectErr: "request.migrationDestination.destinationCommitmentHash does not match canonical reservation artifact",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(TemplateSelfV1)
			testCase.mutate(&request)

			_, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
				RouteRequestID: "ors_invalid_variant_" + strings.ReplaceAll(testCase.name, " ", "_"),
				Stage:          StageSignerCoordination,
				Request:        request,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected %q, got %v", testCase.expectErr, err)
			}
		})
	}
}

func TestServiceRejectsInvalidMigrationTransactionPlanVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		mutate    func(request *RouteSubmitRequest)
		expectErr string
	}{
		{
			name: "missing transaction plan",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan = nil
			},
			expectErr: "request.migrationTransactionPlan is required",
		},
		{
			name: "missing plan version",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.PlanVersion = 0
			},
			expectErr: "request.migrationTransactionPlan.planVersion must equal 1",
		},
		{
			name: "wrong commitment hash",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.PlanCommitmentHash = ""
			},
			expectErr: "request.migrationTransactionPlan.planCommitmentHash must be a 0x-prefixed even-length hex string",
		},
		{
			name: "zero input value",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputValueSats = 0
			},
			expectErr: "request.migrationTransactionPlan.inputValueSats must be greater than zero",
		},
		{
			name: "zero destination value",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.DestinationValueSats = 0
			},
			expectErr: "request.migrationTransactionPlan.destinationValueSats must be greater than zero",
		},
		{
			name: "zero fee",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.FeeSats = 0
				request.MigrationTransactionPlan.DestinationValueSats =
					request.MigrationTransactionPlan.InputValueSats -
						request.MigrationTransactionPlan.AnchorValueSats
			},
			expectErr: "request.migrationTransactionPlan.feeSats must be greater than zero",
		},
		{
			name: "wrong anchor value",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.AnchorValueSats = 331
			},
			expectErr: "request.migrationTransactionPlan.anchorValueSats must equal the canonical 330 sat anchor",
		},
		{
			name: "wrong input sequence",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputSequence = 0xFFFFFFFF
			},
			expectErr: "request.migrationTransactionPlan.inputSequence must equal 0xFFFFFFFD",
		},
		{
			name: "locktime mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.LockTime = uint32(request.MaturityHeight + 1)
			},
			expectErr: "request.migrationTransactionPlan.lockTime must match request.maturityHeight",
		},
		{
			name: "maturity height exceeds uint32",
			mutate: func(request *RouteSubmitRequest) {
				request.MaturityHeight = 0x1_0000_0000
			},
			expectErr: "request.maturityHeight must fit in uint32",
		},
		{
			name: "insufficient input for destination",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputValueSats = request.MigrationTransactionPlan.DestinationValueSats - 1
			},
			expectErr: "request.migrationTransactionPlan.inputValueSats must cover destinationValueSats",
		},
		{
			name: "insufficient input for anchor",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputValueSats = request.MigrationTransactionPlan.DestinationValueSats + canonicalAnchorValueSats - 1
			},
			expectErr: "request.migrationTransactionPlan.inputValueSats must cover anchorValueSats",
		},
		{
			name: "tampered commitment hash",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.PlanCommitmentHash = "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			},
			expectErr: "request.migrationTransactionPlan.planCommitmentHash does not match canonical migration transaction plan",
		},
		{
			name: "accounting mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.FeeSats++
			},
			expectErr: "request.migrationTransactionPlan values must satisfy inputValueSats = destinationValueSats + anchorValueSats + feeSats",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(TemplateSelfV1)
			testCase.mutate(&request)

			_, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
				RouteRequestID: "ors_invalid_plan_" + strings.ReplaceAll(testCase.name, " ", "_"),
				Stage:          StageSignerCoordination,
				Request:        request,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected %q, got %v", testCase.expectErr, err)
			}
		})
	}
}

func TestServiceRejectsMigrationTransactionPlanBoundToDifferentDestinationCommitment(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateSelfV1)

	mutatedDestination := validMigrationDestination()
	mutatedDestination.Revealer = "0x4444444444444444444444444444444444444444"
	mutatedDestination.MigrationExtraData = computeMigrationExtraData(mutatedDestination.Revealer)
	mutatedDestination.DestinationCommitmentHash, err = computeDestinationCommitmentHash(mutatedDestination)
	if err != nil {
		t.Fatal(err)
	}

	request.DestinationCommitmentHash = mutatedDestination.DestinationCommitmentHash
	request.MigrationDestination = mutatedDestination

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_invalid_plan_destination_binding",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "request.migrationTransactionPlan.planCommitmentHash does not match canonical migration transaction plan") {
		t.Fatalf("expected plan binding error, got %v", err)
	}
}

func TestServiceAcceptsStructuredSignerApprovalWithCanonicalLegacySignatures(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateQcV1)
	request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
		request.ArtifactApprovals.Approvals[0],
		request.ArtifactApprovals.Approvals[1],
	}
	request.ArtifactSignatures = canonicalArtifactSignaturesWithSignerApproval(
		request.Route,
		request.ArtifactApprovals,
		request.SignerApproval,
	)

	result, err := service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_artifact_approvals",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != StepStatusPending {
		t.Fatalf("expected PENDING, got %#v", result)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateQcV1, "orq_artifact_approvals")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored job")
	}
	if job.Request.ArtifactApprovals == nil {
		t.Fatal("expected stored artifact approvals")
	}
}

func TestServiceAcceptsStructuredSignerApprovalWhenVerifierConfigured(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(request RouteSubmitRequest) error {
			if request.SignerApproval == nil {
				t.Fatal("expected signer approval")
			}
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateQcV1)

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_structured_signer_approval",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	job, ok, err := service.store.GetByRouteRequest(
		TemplateQcV1,
		"orq_structured_signer_approval",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored job")
	}
	if job.Request.SignerApproval == nil {
		t.Fatal("expected stored signer approval")
	}
	if !reflect.DeepEqual(
		job.Request.SignerApproval.ActiveMembers,
		[]uint32{1, 2},
	) {
		t.Fatalf(
			"unexpected active members: %#v",
			job.Request.SignerApproval.ActiveMembers,
		)
	}
	if !reflect.DeepEqual(
		job.Request.SignerApproval.InactiveMembers,
		[]uint32{3, 4},
	) {
		t.Fatalf(
			"unexpected inactive members: %#v",
			job.Request.SignerApproval.InactiveMembers,
		)
	}
}

func TestServiceRejectsStructuredSignerApprovalWithoutVerifier(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_structured_signer_approval_unsupported",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval cannot be verified by this signer deployment",
	) {
		t.Fatalf("expected unsupported signer approval error, got %v", err)
	}
}

func TestServiceRejectsMissingSignerApprovalWhenVerifierConfigured(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_legacy_signer_approval_path",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval is required when the signer approval verifier is configured",
	) {
		t.Fatalf("expected missing signer approval error, got %v", err)
	}
}

func TestServiceRejectsStructuredSignerApprovalWithMismatchedApprovalDigest(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	request.SignerApproval.ApprovalDigest = "0x" + strings.Repeat("11", 32)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_structured_signer_approval_bad_digest",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval.approvalDigest must match the canonical artifactApprovals payload digest",
	) {
		t.Fatalf("expected signer approval digest mismatch error, got %v", err)
	}
}
func TestServiceRejectsStructuredSignerApprovalWithLegacySignerRole(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	request.ArtifactApprovals.Approvals = append(
		request.ArtifactApprovals.Approvals,
		ArtifactRoleApproval{
			Role:      ArtifactApprovalRole("S"),
			Signature: "0x5151",
		},
	)
	request.ArtifactSignatures = append(
		request.ArtifactSignatures[:len(request.ArtifactSignatures)-1],
		"0x5151",
		request.ArtifactSignatures[len(request.ArtifactSignatures)-1],
	)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_structured_signer_approval_legacy_role",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.artifactApprovals.approvals[1].role is not allowed for self_v1",
	) {
		t.Fatalf("expected structured signer-role rejection, got %v", err)
	}
}

func TestServiceRejectsInvalidArtifactApprovalVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		route     TemplateID
		mutate    func(request *RouteSubmitRequest)
		expectErr string
	}{
		{
			name:  "missing qc custodian approval",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
					request.ArtifactApprovals.Approvals[0],
				}
				request.ArtifactSignatures = []string{
					request.ArtifactSignatures[0],
				}
			},
			expectErr: "request.artifactApprovals.approvals must include role C for qc_v1",
		},
		{
			name:  "self route rejects custodian approval role",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
					request.ArtifactApprovals.Approvals[0],
					{
						Role: ArtifactApprovalRoleCustodian,
						Signature: mustArtifactApprovalSignature(
							testCustodianPrivateKey,
							request.ArtifactApprovals.Payload,
						),
					},
				}
			},
			expectErr: "request.artifactApprovals.approvals[1].role is not allowed for self_v1",
		},
		{
			name:  "plan commitment mismatch",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals.Payload.PlanCommitmentHash = "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			},
			expectErr: "request.artifactApprovals.payload.planCommitmentHash must match request.migrationTransactionPlan.planCommitmentHash",
		},
		{
			name:  "artifact approvals required",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals = nil
			},
			expectErr: "request.artifactApprovals is required",
		},
		{
			name:  "legacy signature mismatch",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactSignatures = []string{
					request.ArtifactSignatures[1],
					request.ArtifactSignatures[0],
				}
			},
			expectErr: "request.artifactSignatures must match canonical approval role order derived from request.artifactApprovals",
		},
		{
			name:  "legacy signatures remain required when approvals are present",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactSignatures = nil
			},
			expectErr: "request.artifactSignatures must not be empty",
		},
		{
			name:  "depositor signature does not verify",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				setArtifactApprovalSignature(
					request.ArtifactApprovals,
					ArtifactApprovalRoleDepositor,
					mustArtifactApprovalSignature(
						testCustodianPrivateKey,
						request.ArtifactApprovals.Payload,
					),
				)
				request.ArtifactSignatures = canonicalArtifactSignatures(
					request.Route,
					request.ArtifactApprovals,
				)
			},
			expectErr: "request.artifactApprovals.approvals[0].signature does not verify against the required public key",
		},
		{
			name:  "custodian signature does not verify",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				setArtifactApprovalSignature(
					request.ArtifactApprovals,
					ArtifactApprovalRoleCustodian,
					artifactApprovalSignatureByRole(
						request.ArtifactApprovals,
						ArtifactApprovalRoleDepositor,
					),
				)
				request.ArtifactSignatures = canonicalArtifactSignatures(
					request.Route,
					request.ArtifactApprovals,
				)
			},
			expectErr: "request.artifactApprovals.approvals[1].signature does not verify against the required public key",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(testCase.route)
			testCase.mutate(&request)

			_, err := service.Submit(context.Background(), testCase.route, SignerSubmitInput{
				RouteRequestID: "ors_invalid_artifact_approval_" + strings.ReplaceAll(testCase.name, " ", "_"),
				Stage:          StageSignerCoordination,
				Request:        request,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected %q, got %v", testCase.expectErr, err)
			}
		})
	}
}

func TestRequestDigestNormalizesEquivalentArtifactApprovalVariants(t *testing.T) {
	canonicalDigest, err := requestDigest(canonicalArtifactApprovalRequest(TemplateQcV1), validationOptions{})
	if err != nil {
		t.Fatal(err)
	}

	variantDigest, err := requestDigest(
		equivalentArtifactApprovalVariantFromRequest(
			t,
			canonicalArtifactApprovalRequest(TemplateQcV1),
		),
		validationOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	if canonicalDigest != variantDigest {
		t.Fatalf("expected matching request digest, got %s vs %s", canonicalDigest, variantDigest)
	}
}

func TestRequestDigestNormalizesEquivalentStructuredSignerApprovalVariants(t *testing.T) {
	canonicalDigest, err := requestDigest(structuredSignerApprovalRequest(TemplateQcV1), validationOptions{})
	if err != nil {
		t.Fatal(err)
	}

	variantDigest, err := requestDigest(
		equivalentArtifactApprovalVariantFromRequest(
			t,
			structuredSignerApprovalRequest(TemplateQcV1),
		),
		validationOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	if canonicalDigest != variantDigest {
		t.Fatalf(
			"expected matching structured request digest, got %s vs %s",
			canonicalDigest,
			variantDigest,
		)
	}
}

func TestRequestDigestDoesNotEscapeHTMLSensitiveCharacters(t *testing.T) {
	request := canonicalArtifactApprovalRequest(TemplateSelfV1)
	request.FacadeRequestID = "rf_<tag>&sink"
	request.IdempotencyKey = "idem_>bridge"

	normalizedRequest, err := normalizeRouteSubmitRequest(request, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := marshalCanonicalJSON(normalizedRequest)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(payload, []byte(`"facadeRequestId":"rf_<tag>&sink"`)) {
		t.Fatalf("expected raw HTML-sensitive characters in payload, got %s", payload)
	}
	if bytes.Contains(payload, []byte(`\u003c`)) ||
		bytes.Contains(payload, []byte(`\u003e`)) ||
		bytes.Contains(payload, []byte(`\u0026`)) {
		t.Fatalf("expected unescaped HTML-sensitive characters in payload, got %s", payload)
	}

	digestFromRawRequest, err := requestDigest(request, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	digestFromNormalizedRequest, err := requestDigestFromNormalized(normalizedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if digestFromRawRequest != digestFromNormalizedRequest {
		t.Fatalf(
			"expected matching digests, got %s vs %s",
			digestFromRawRequest,
			digestFromNormalizedRequest,
		)
	}
}

func TestDestinationCommitmentHashDoesNotEscapeHTMLSensitiveCharacters(t *testing.T) {
	destination := validMigrationDestination()
	destination.Network = "regtest<v2>&sink"

	payload, err := marshalCanonicalJSON(destinationCommitmentPayload{
		Reserve:            normalizeLowerHex(destination.Reserve),
		Epoch:              destination.Epoch,
		Route:              string(destination.Route),
		Revealer:           normalizeLowerHex(destination.Revealer),
		Vault:              normalizeLowerHex(destination.Vault),
		Network:            strings.TrimSpace(destination.Network),
		DepositScriptHash:  normalizeLowerHex(destination.DepositScriptHash),
		MigrationExtraData: normalizeLowerHex(destination.MigrationExtraData),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(payload, []byte(`"network":"regtest<v2>&sink"`)) {
		t.Fatalf("expected raw HTML-sensitive characters in payload, got %s", payload)
	}
	if bytes.Contains(payload, []byte(`\u003c`)) ||
		bytes.Contains(payload, []byte(`\u003e`)) ||
		bytes.Contains(payload, []byte(`\u0026`)) {
		t.Fatalf("expected unescaped HTML-sensitive characters in payload, got %s", payload)
	}

	hash, err := computeDestinationCommitmentHash(destination)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("expected destination commitment hash")
	}
}

func TestMigrationPlanQuoteSigningVectorsMatchFixture(t *testing.T) {
	vectors := loadMigrationPlanQuoteSigningVectors(t)
	if vectors.Version != 1 {
		t.Fatalf("unexpected vector version: %d", vectors.Version)
	}
	if vectors.Scope != "migration_plan_quote_signing_contract_v1" {
		t.Fatalf("unexpected vector scope: %s", vectors.Scope)
	}

	block, _ := pem.Decode([]byte(vectors.TrustRoot.PublicKeyPEM))
	if block == nil {
		t.Fatal("expected migration plan quote fixture to contain a PEM public key")
	}
	parsedPublicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
	if !ok {
		t.Fatalf("expected Ed25519 public key, got %T", parsedPublicKey)
	}

	for name, vector := range vectors.Vectors {
		t.Run(name, func(t *testing.T) {
			payload, err := migrationPlanQuoteSigningPayloadBytes(&vector.UnsignedQuote)
			if err != nil {
				t.Fatal(err)
			}
			if string(payload) != vector.ExpectedPayload {
				t.Fatalf("unexpected signing payload: %s", payload)
			}

			preimage, err := migrationPlanQuoteSigningPreimage(&vector.UnsignedQuote)
			if err != nil {
				t.Fatal(err)
			}
			if string(preimage) != vector.ExpectedPreimage {
				t.Fatalf("unexpected signing preimage: %s", preimage)
			}

			signingHash, err := migrationPlanQuoteSigningHash(&vector.UnsignedQuote)
			if err != nil {
				t.Fatal(err)
			}
			if "0x"+hex.EncodeToString(signingHash) != vector.ExpectedHash {
				t.Fatalf("unexpected signing hash: 0x%s", hex.EncodeToString(signingHash))
			}

			rawSignature, err := hex.DecodeString(strings.TrimPrefix(vector.ExpectedSignature, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			if !ed25519.Verify(publicKey, signingHash, rawSignature) {
				t.Fatal("expected fixture signature to verify against the fixture trust root")
			}
		})
	}
}

func TestServiceRequiresMigrationPlanQuoteWhenTrustRootsConfigured(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithMigrationPlanQuoteTrustRoots([]MigrationPlanQuoteTrustRoot{
			testMigrationPlanQuoteTrustRoot,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_quote_required",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.migrationPlanQuote is required when migrationPlanQuoteTrustRoots are configured",
	) {
		t.Fatalf("expected missing quote error, got %v", err)
	}
}

func TestServiceAcceptsValidMigrationPlanQuoteWhenTrustRootsConfigured(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithMigrationPlanQuoteTrustRoots([]MigrationPlanQuoteTrustRoot{
			testMigrationPlanQuoteTrustRoot,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := requestWithValidMigrationPlanQuote(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_quote_valid",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceRejectsExpiredMigrationPlanQuoteOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithMigrationPlanQuoteTrustRoots([]MigrationPlanQuoteTrustRoot{
			testMigrationPlanQuoteTrustRoot,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time {
		return time.Date(2099, time.March, 9, 0, 16, 0, 0, time.UTC)
	}

	request := requestWithValidMigrationPlanQuote(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_quote_expired",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "request.migrationPlanQuote is expired") {
		t.Fatalf("expected expired quote error, got %v", err)
	}
}

func TestServicePollAcceptsStoredMigrationPlanQuoteAfterQuoteExpiry(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{
			submit: func(*Job) (*Transition, error) {
				return &Transition{State: JobStatePending, Detail: "queued"}, nil
			},
		},
		WithMigrationPlanQuoteTrustRoots([]MigrationPlanQuoteTrustRoot{
			testMigrationPlanQuoteTrustRoot,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time {
		return time.Date(2099, time.March, 9, 0, 10, 0, 0, time.UTC)
	}

	request := requestWithValidMigrationPlanQuote(TemplateSelfV1)

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_quote_poll",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	service.now = func() time.Time {
		return time.Date(2099, time.March, 9, 0, 16, 0, 0, time.UTC)
	}

	pollResult, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_quote_poll",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pollResult.Status != StepStatusPending {
		t.Fatalf("expected pending poll result, got %#v", pollResult)
	}
}

func TestParseMigrationPlanQuoteTrustRootRejectsInvalidPEM(t *testing.T) {
	_, err := parseMigrationPlanQuoteTrustRoot("trustRoot", MigrationPlanQuoteTrustRoot{
		PublicKeyPEM: "not a PEM value",
	})
	if err == nil || !strings.Contains(err.Error(), "trustRoot.publicKeyPem must be a PEM-encoded public key") {
		t.Fatalf("expected invalid PEM error, got: %v", err)
	}
}

func TestParseMigrationPlanQuoteTrustRootRejectsNonEd25519Key(t *testing.T) {
	secpKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&secpKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = parseMigrationPlanQuoteTrustRoot("trustRoot", MigrationPlanQuoteTrustRoot{
		PublicKeyPEM: string(
			pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: publicKeyDER,
			}),
		),
	})
	if err == nil || !strings.Contains(err.Error(), "trustRoot.publicKeyPem must be a PEM-encoded Ed25519 public key") {
		t.Fatalf("expected non-ed25519 key error, got: %v", err)
	}
}

func TestServiceAcceptsSelfV1WithMatchingDepositorTrustRoot(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			testDepositorTrustRoot(TemplateSelfV1),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_trust_root_match",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceRejectsSelfV1WithoutMatchingDepositorTrustRoot(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			testDepositorTrustRoot(TemplateSelfV1),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateSelfV1)
	request.ScriptTemplate = mustTemplate(SelfV1Template{
		Template:           TemplateSelfV1,
		DepositorPublicKey: testSignerPublicKey,
		SignerPublicKey:    testSignerPublicKey,
		Delta2:             4320,
	})

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_trust_root_mismatch",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.scriptTemplate.depositorPublicKey must match the configured depositorTrustRoots publicKey for self_v1",
	) {
		t.Fatalf("expected self_v1 depositor trust-root mismatch, got %v", err)
	}
}

func TestServiceRejectsSelfV1WithoutConfiguredDepositorTrustRootMatch(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			{
				Route:     TemplateSelfV1,
				Reserve:   "0x9999999999999999999999999999999999999999",
				Network:   "regtest",
				PublicKey: testDepositorPublicKey,
			},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_trust_root_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.scriptTemplate.depositorPublicKey requires a matching configured depositorTrustRoots entry for self_v1",
	) {
		t.Fatalf("expected missing self_v1 depositor trust-root error, got %v", err)
	}
}

func TestServiceAcceptsQcV1WithMatchingCustodianTrustRoot(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithCustodianTrustRoots([]CustodianTrustRoot{
			testCustodianTrustRoot(TemplateQcV1),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_qc_trust_root_match",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceAcceptsQcV1WithMatchingDepositorAndCustodianTrustRoots(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			testDepositorTrustRoot(TemplateQcV1),
		}),
		WithCustodianTrustRoots([]CustodianTrustRoot{
			testCustodianTrustRoot(TemplateQcV1),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_qc_depositor_and_custodian_trust_root_match",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceRejectsQcV1WithoutMatchingCustodianTrustRoot(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithCustodianTrustRoots([]CustodianTrustRoot{
			testCustodianTrustRoot(TemplateQcV1),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateQcV1)
	request.ScriptTemplate = mustTemplate(QcV1Template{
		Template:           TemplateQcV1,
		DepositorPublicKey: testDepositorPublicKey,
		CustodianPublicKey: testSignerPublicKey,
		SignerPublicKey:    testSignerPublicKey,
		Beta:               144,
		Delta2:             4320,
	})

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_qc_trust_root_mismatch",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.scriptTemplate.custodianPublicKey must match the configured custodianTrustRoots publicKey for qc_v1",
	) {
		t.Fatalf("expected qc_v1 custodian trust-root mismatch, got %v", err)
	}
}

func TestServiceRejectsQcV1WithoutMatchingDepositorTrustRoot(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			testDepositorTrustRoot(TemplateQcV1),
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateQcV1)
	request.ScriptTemplate = mustTemplate(QcV1Template{
		Template:           TemplateQcV1,
		DepositorPublicKey: testSignerPublicKey,
		CustodianPublicKey: testCustodianPublicKey,
		SignerPublicKey:    testSignerPublicKey,
		Beta:               144,
		Delta2:             4320,
	})

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_qc_depositor_trust_root_mismatch",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.scriptTemplate.depositorPublicKey must match the configured depositorTrustRoots publicKey for qc_v1",
	) {
		t.Fatalf("expected qc_v1 depositor trust-root mismatch, got %v", err)
	}
}

func TestServiceRejectsQcV1WithoutConfiguredDepositorTrustRootMatch(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			{
				Route:     TemplateQcV1,
				Reserve:   "0x9999999999999999999999999999999999999999",
				Network:   "regtest",
				PublicKey: testDepositorPublicKey,
			},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_qc_depositor_trust_root_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.scriptTemplate.depositorPublicKey requires a matching configured depositorTrustRoots entry for qc_v1",
	) {
		t.Fatalf("expected missing qc_v1 depositor trust-root error, got %v", err)
	}
}

func TestServiceRejectsQcV1WithoutConfiguredCustodianTrustRootMatch(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithCustodianTrustRoots([]CustodianTrustRoot{
			{
				Route:     TemplateQcV1,
				Reserve:   "0x9999999999999999999999999999999999999999",
				Network:   "regtest",
				PublicKey: testCustodianPublicKey,
			},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_qc_trust_root_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.scriptTemplate.custodianPublicKey requires a matching configured custodianTrustRoots entry for qc_v1",
	) {
		t.Fatalf("expected missing qc_v1 custodian trust-root error, got %v", err)
	}
}

func TestNewServiceRejectsDuplicateDepositorTrustRootScope(t *testing.T) {
	handle := newMemoryHandle()

	_, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			testDepositorTrustRoot(TemplateSelfV1),
			testDepositorTrustRoot(TemplateSelfV1),
		}),
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"duplicates depositorTrustRoots[0]",
	) {
		t.Fatalf("expected duplicate depositor trust-root error, got %v", err)
	}
}

func TestNewServiceRejectsInvalidCustodianTrustRootPublicKey(t *testing.T) {
	handle := newMemoryHandle()

	_, err := NewService(
		handle,
		&scriptedEngine{},
		WithCustodianTrustRoots([]CustodianTrustRoot{
			{
				Route:     TemplateQcV1,
				Reserve:   validMigrationDestination().Reserve,
				Network:   validMigrationDestination().Network,
				PublicKey: "0x1234",
			},
		}),
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"custodianTrustRoots[0].publicKey must be a compressed secp256k1 public key",
	) {
		t.Fatalf("expected invalid custodian trust-root public key error, got %v", err)
	}
}

func TestServiceAcceptsMixedCaseDepositorTrustRootConfig(t *testing.T) {
	handle := newMemoryHandle()
	migrationDestination := validMigrationDestination()

	service, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{
			{
				Route:     TemplateSelfV1,
				Reserve:   mixedCaseHexBody(migrationDestination.Reserve),
				Network:   strings.ToUpper(migrationDestination.Network),
				PublicKey: mixedCaseHexBody(testDepositorPublicKey),
			},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_trust_root_mixed_case_config",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServicePollAcceptsEquivalentArtifactApprovalRequestVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitRequest := equivalentArtifactApprovalVariantFromRequest(
		t,
		canonicalArtifactApprovalRequest(TemplateQcV1),
	)
	submitResult, err := service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_equivalent_digest",
		Stage:          StageSignerCoordination,
		Request:        submitRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateQcV1, SignerPollInput{
		RouteRequestID: "orq_equivalent_digest",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        canonicalArtifactApprovalRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if pollResult.Status != StepStatusPending {
		t.Fatalf("expected PENDING, got %#v", pollResult)
	}
}

func TestServiceStoresNormalizedArtifactApprovalRequest(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := equivalentArtifactApprovalVariantFromRequest(
		t,
		canonicalArtifactApprovalRequest(TemplateQcV1),
	)
	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_normalized_store",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateQcV1, "orq_normalized_store")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored job")
	}

	expected := canonicalArtifactApprovalRequest(TemplateQcV1)
	if !reflect.DeepEqual(job.Request, expected) {
		t.Fatalf("expected normalized request %#v, got %#v", expected, job.Request)
	}
}

func TestRequestDigestRejectsArtifactApprovalsWithoutMigrationTransactionPlan(t *testing.T) {
	request := canonicalArtifactApprovalRequest(TemplateSelfV1)
	request.MigrationTransactionPlan = nil

	_, err := requestDigest(request, validationOptions{})
	if err == nil || !strings.Contains(
		err.Error(),
		"request.migrationTransactionPlan is required when request.artifactApprovals is present",
	) {
		t.Fatalf("expected missing plan error, got %v", err)
	}
}

func TestArtifactApprovalDigestMatchesPhase1Contract(t *testing.T) {
	expectedDigests := map[TemplateID]string{
		TemplateQcV1:   "0x4e1c72624e85c41d8d8a050d75704dc881ec6cd2dcfe1d240052887feef87ad8",
		TemplateSelfV1: "0x960d7082d6eac550d7647d8fbeb90781e6cbd001b4d433e6635aa447dd937e79",
	}

	for _, route := range []TemplateID{TemplateQcV1, TemplateSelfV1} {
		t.Run(string(route), func(t *testing.T) {
			request := canonicalArtifactApprovalRequest(route)

			digest, err := artifactApprovalDigest(request.ArtifactApprovals.Payload)
			if err != nil {
				t.Fatal(err)
			}

			actualDigest := "0x" + hex.EncodeToString(digest)
			if actualDigest != expectedDigests[route] {
				t.Fatalf("expected digest %s, got %s", expectedDigests[route], actualDigest)
			}
		})
	}
}

func TestApprovalContractVectorsMatchExpectedRequestDigests(t *testing.T) {
	for _, vectorKey := range []string{"qc_v1", "self_v1", "self_v1_presign"} {
		t.Run(vectorKey, func(t *testing.T) {
			request, expectedApprovalDigest, expectedDigest := loadApprovalContractVector(t, vectorKey)

			digestBytes, err := artifactApprovalDigest(request.ArtifactApprovals.Payload)
			if err != nil {
				t.Fatal(err)
			}
			if actualApprovalDigest := "0x" + hex.EncodeToString(digestBytes); actualApprovalDigest != expectedApprovalDigest {
				t.Fatalf(
					"expected approval digest %s, got %s",
					expectedApprovalDigest,
					actualApprovalDigest,
				)
			}

			digest, err := requestDigest(request, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if digest != expectedDigest {
				t.Fatalf("expected digest %s, got %s", expectedDigest, digest)
			}
		})
	}
}

func TestApprovalContractVectorsNormalizeEquivalentVariants(t *testing.T) {
	for _, vectorKey := range []string{"qc_v1", "self_v1", "self_v1_presign"} {
		t.Run(vectorKey, func(t *testing.T) {
			canonicalRequest, _, expectedDigest := loadApprovalContractVector(t, vectorKey)

			normalizedCanonical, err := normalizeRouteSubmitRequest(canonicalRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}

			variantRequest := equivalentArtifactApprovalVariantFromRequest(
				t,
				canonicalRequest,
			)
			normalizedVariant, err := normalizeRouteSubmitRequest(variantRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(normalizedVariant, normalizedCanonical) {
				t.Fatalf(
					"expected normalized variant %#v, got %#v",
					normalizedCanonical,
					normalizedVariant,
				)
			}

			digest, err := requestDigest(variantRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if digest != expectedDigest {
				t.Fatalf("expected digest %s, got %s", expectedDigest, digest)
			}
		})
	}
}

func TestRequestDigestDistinguishesSelfV1PresignFromReconstruct(t *testing.T) {
	reconstructRequest := structuredSignerApprovalRequest(TemplateSelfV1)
	reconstructRequest.RequestType = RequestTypeReconstruct

	presignRequest := cloneRouteSubmitRequest(t, reconstructRequest)
	presignRequest.RequestType = RequestTypePresignSelfV1

	reconstructDigest, err := requestDigest(reconstructRequest, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	presignDigest, err := requestDigest(presignRequest, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if reconstructDigest == presignDigest {
		t.Fatalf("expected distinct self_v1 digests, got %s", reconstructDigest)
	}

	normalizedReconstruct, err := normalizeRouteSubmitRequest(reconstructRequest, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	normalizedPresign, err := normalizeRouteSubmitRequest(presignRequest, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if normalizedReconstruct.RequestType != RequestTypeReconstruct {
		t.Fatalf("expected reconstruct requestType, got %s", normalizedReconstruct.RequestType)
	}
	if normalizedPresign.RequestType != RequestTypePresignSelfV1 {
		t.Fatalf("expected presign requestType, got %s", normalizedPresign.RequestType)
	}
}

func TestServiceRejectsQcV1PresignRequestType(t *testing.T) {
	service, err := NewService(newMemoryHandle(), &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateQcV1)
	request.RequestType = RequestTypePresignSelfV1

	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "route_qc_invalid_presign",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil {
		t.Fatal("expected requestType validation error")
	}
	if !strings.Contains(err.Error(), "request.requestType must be reconstruct for qc_v1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestDigestNormalizesMixedCaseArtifactApprovalVariants(t *testing.T) {
	for _, route := range []TemplateID{TemplateQcV1, TemplateSelfV1} {
		t.Run(string(route), func(t *testing.T) {
			canonicalRequest := canonicalMixedCaseCoverageArtifactApprovalRequest(t, route)
			mixedCaseRequest := mixedCaseArtifactApprovalVariantFromRequest(
				t,
				canonicalRequest,
			)

			if mixedCaseRequest.Reserve == canonicalRequest.Reserve {
				t.Fatalf(
					"expected mixed-case reserve variant, got %s",
					mixedCaseRequest.Reserve,
				)
			}

			normalizedCanonical, err := normalizeRouteSubmitRequest(canonicalRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			normalizedMixedCase, err := normalizeRouteSubmitRequest(mixedCaseRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(normalizedMixedCase, normalizedCanonical) {
				t.Fatalf(
					"expected normalized mixed-case request %#v, got %#v",
					normalizedCanonical,
					normalizedMixedCase,
				)
			}

			canonicalDigest, err := requestDigest(canonicalRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			mixedCaseDigest, err := requestDigest(mixedCaseRequest, validationOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if mixedCaseDigest != canonicalDigest {
				t.Fatalf(
					"expected matching digest %s, got %s",
					canonicalDigest,
					mixedCaseDigest,
				)
			}
		})
	}
}

func TestMigrationTransactionPlanCommitmentHashMatchesCanonicalVectors(t *testing.T) {
	testCases := []struct {
		name     string
		request  RouteSubmitRequest
		plan     *MigrationTransactionPlan
		expected string
	}{
		{
			name: "canonical cross-stack vector",
			request: RouteSubmitRequest{
				Reserve:                   "0x2000000000000000000000000000000000000002",
				Epoch:                     12,
				ActiveOutpoint:            CovenantOutpoint{TxID: "0x1111111111111111111111111111111111111111111111111111111111111111", Vout: 1},
				DestinationCommitmentHash: "0xf1b1739d99ea890ea6d419d6db28f4d5fe0871c32619a0984c1bfdbe4025f768",
			},
			plan: &MigrationTransactionPlan{
				PlanVersion:          migrationTransactionPlanVersion,
				PlanCommitmentHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
				InputValueSats:       1_000_000,
				DestinationValueSats: 998_000,
				AnchorValueSats:      canonicalAnchorValueSats,
				FeeSats:              1_670,
				InputSequence:        canonicalCovenantInputSequence,
				LockTime:             950000,
			},
			expected: "0x8dcafe57b888040d644e80dfd1b8b089dfd5016205d78316549ef71d032070f2",
		},
		{
			name: "mixed-case hex inputs normalize before hashing",
			request: RouteSubmitRequest{
				Reserve:                   "0xAbCd00000000000000000000000000000000Ef01",
				Epoch:                     0,
				ActiveOutpoint:            CovenantOutpoint{TxID: "0xAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDd", Vout: 0},
				DestinationCommitmentHash: "0xFfEeDdCcBbAa00998877665544332211FfEeDdCcBbAa00998877665544332211",
			},
			plan: &MigrationTransactionPlan{
				PlanVersion:          migrationTransactionPlanVersion,
				PlanCommitmentHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
				InputValueSats:       100_000,
				DestinationValueSats: 99_300,
				AnchorValueSats:      canonicalAnchorValueSats,
				FeeSats:              370,
				InputSequence:        canonicalCovenantInputSequence,
				LockTime:             1,
			},
			expected: "0x626ce76714e04a41a5ec06a96082cac2ebd4d8f687fdc77766ffd9c0d11dac14",
		},
		{
			name: "max uint32 fields and large safe integer amounts remain stable",
			request: RouteSubmitRequest{
				Reserve:                   "0x9999999999999999999999999999999999999999",
				Epoch:                     4294967295,
				ActiveOutpoint:            CovenantOutpoint{TxID: "0x0000000000000000000000000000000000000000000000000000000000000001", Vout: 4294967295},
				DestinationCommitmentHash: "0x00000000000000000000000000000000000000000000000000000000000000aa",
			},
			plan: &MigrationTransactionPlan{
				PlanVersion:          migrationTransactionPlanVersion,
				PlanCommitmentHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
				InputValueSats:       9_007_199_254_740_000,
				DestinationValueSats: 9_007_199_254_737_000,
				AnchorValueSats:      canonicalAnchorValueSats,
				FeeSats:              2670,
				InputSequence:        canonicalCovenantInputSequence,
				LockTime:             0xffffffff,
			},
			expected: "0x42983bef3abb9680093ca0254c780c6ed4e6178405649bf1846ebb381ca89e02",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := computeMigrationTransactionPlanCommitmentHash(testCase.request, testCase.plan)
			if err != nil {
				t.Fatal(err)
			}

			if actual != testCase.expected {
				t.Fatalf("unexpected plan commitment hash: %s", actual)
			}
		})
	}
}

func TestStoreReloadPreservesJobs(t *testing.T) {
	handle := newMemoryHandle()
	store, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	job := &Job{
		RequestID:       "kcs_self_1234",
		RouteRequestID:  "ors_reload",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_reload",
		FacadeRequestID: "rf_reload",
		RequestDigest:   "0xdeadbeef",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	if err := store.Put(job); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	loadedJob, ok, err := reloaded.GetByRouteRequest(TemplateSelfV1, "ors_reload")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected persisted job")
	}
	if !reflect.DeepEqual(job.Request, loadedJob.Request) {
		t.Fatalf("unexpected reloaded request: %#v", loadedJob.Request)
	}
}

func TestStorePutReturnsErrorWhenSaveFails(t *testing.T) {
	handle := newFaultingMemoryHandle()
	handle.saveErrByName["kcs_self_fail_save.json"] = errors.New("injected save failure")

	store, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Put(&Job{
		RequestID:       "kcs_self_fail_save",
		RouteRequestID:  "ors_fail_save",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_fail_save",
		FacadeRequestID: "rf_fail_save",
		RequestDigest:   "0xdeadbeef",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	})
	if err == nil || !strings.Contains(err.Error(), "injected save failure") {
		t.Fatalf("expected injected save failure, got: %v", err)
	}

	_, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_fail_save")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no route mapping after failed save")
	}
}

func TestStorePutKeepsNewRouteMappingWhenOldDeleteFails(t *testing.T) {
	handle := newFaultingMemoryHandle()
	store, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	initial := &Job{
		RequestID:       "kcs_self_old",
		RouteRequestID:  "ors_replace",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_old",
		FacadeRequestID: "rf_old",
		RequestDigest:   "0x1111",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	if err := store.Put(initial); err != nil {
		t.Fatal(err)
	}

	handle.deleteErrByName["kcs_self_old.json"] = errors.New("injected delete failure")

	replacement := &Job{
		RequestID:       "kcs_self_new",
		RouteRequestID:  "ors_replace",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_new",
		FacadeRequestID: "rf_new",
		RequestDigest:   "0x2222",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-10T00:00:00Z",
		UpdatedAt:       "2026-03-10T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	if err := store.Put(replacement); err != nil {
		t.Fatalf("expected replacement put to succeed, got: %v", err)
	}

	loaded, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_replace")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected route mapping to exist")
	}
	if loaded.RequestID != "kcs_self_new" {
		t.Fatalf("expected route key to map to replacement job, got: %s", loaded.RequestID)
	}
}

func TestStoreLoadSelectsNewestJobForDuplicateRouteKeys(t *testing.T) {
	handle := newMemoryHandle()

	oldJob := &Job{
		RequestID:       "kcs_self_old_load",
		RouteRequestID:  "ors_load_dupe",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_old_load",
		FacadeRequestID: "rf_old_load",
		RequestDigest:   "0xaaa",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}
	newJob := &Job{
		RequestID:       "kcs_self_new_load",
		RouteRequestID:  "ors_load_dupe",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_new_load",
		FacadeRequestID: "rf_new_load",
		RequestDigest:   "0xbbb",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-10T00:00:00Z",
		UpdatedAt:       "2026-03-10T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	oldPayload, err := json.Marshal(oldJob)
	if err != nil {
		t.Fatal(err)
	}
	newPayload, err := json.Marshal(newJob)
	if err != nil {
		t.Fatal(err)
	}

	if err := handle.Save(oldPayload, jobsDirectory, oldJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}
	if err := handle.Save(newPayload, jobsDirectory, newJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	loaded, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_load_dupe")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected loaded route mapping")
	}
	if loaded.RequestID != newJob.RequestID {
		t.Fatalf("expected newest request ID %s, got %s", newJob.RequestID, loaded.RequestID)
	}
}

func TestServerHandlesSubmitAndPathPoll(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, "", true))
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	response, err := http.Post(server.URL+"/v1/self_v1/signer/requests", "application/json", bytes.NewReader(submitPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}

	submitResult := StepResult{}
	if err := json.NewDecoder(response.Body).Decode(&submitResult); err != nil {
		t.Fatal(err)
	}

	pollPayload := mustJSON(t, SignerPollInput{
		RouteRequestID: "ors_http",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	pollResponse, err := http.Post(server.URL+"/v1/self_v1/signer/requests/"+submitResult.RequestID+":poll", "application/json", bytes.NewReader(pollPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer pollResponse.Body.Close()

	if pollResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pollResponse.Body)
		t.Fatalf("unexpected poll status: %d %s", pollResponse.StatusCode, string(body))
	}
}

func TestServerRejectsUnknownFieldsOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, "", true))
	defer server.Close()

	base := baseRequest(TemplateSelfV1)
	template := &SelfV1Template{}
	if err := strictUnmarshal(base.ScriptTemplate, template); err != nil {
		t.Fatal(err)
	}
	payload := bytes.NewBufferString(fmt.Sprintf(`{
		"routeRequestId":"ors_http_unknown",
		"stage":"SIGNER_COORDINATION",
		"request":{
			"facadeRequestId":"rf_123",
			"idempotencyKey":"idem_123",
			"route":"self_v1",
			"requestType":"reconstruct",
			"strategy":"0x1234",
			"reserve":"0x1111111111111111111111111111111111111111",
			"epoch":12,
			"maturityHeight":912345,
			"activeOutpoint":{"txid":"0x0102","vout":1,"scriptHash":"0x0304"},
			"destinationCommitmentHash":"%s",
			"migrationDestination":{
				"reservationId":"cmdr_12345678",
				"reserve":"0x1111111111111111111111111111111111111111",
				"epoch":12,
				"route":"MIGRATION",
				"revealer":"0x2222222222222222222222222222222222222222",
				"vault":"0x3333333333333333333333333333333333333333",
				"network":"regtest",
				"status":"RESERVED",
				"depositScript":"0x0014aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"depositScriptHash":"0x8532ec6785e391b2af968b5728d574e271c7f46658f5ed10845d9ad5b23ac6d3",
				"migrationExtraData":"0x41435f4d49475241544556312222222222222222222222222222222222222222",
				"destinationCommitmentHash":"%s"
			},
			"migrationTransactionPlan":{
				"planVersion":1,
				"planCommitmentHash":"%s",
				"inputValueSats":1000000,
				"destinationValueSats":998000,
				"anchorValueSats":330,
				"feeSats":1670,
				"inputSequence":4294967293,
				"lockTime":912345
			},
			"artifactApprovals":{
				"payload":{
					"approvalVersion":1,
					"route":"self_v1",
					"scriptTemplateId":"self_v1",
					"destinationCommitmentHash":"%s",
					"planCommitmentHash":"%s"
				},
				"approvals":[
					{"role":"D","signature":"%s"}
				]
			},
			"artifactSignatures":["%s"],
			"artifacts":{},
			"scriptTemplate":{"template":"self_v1","depositorPublicKey":"%s","signerPublicKey":"%s","delta2":4320},
			"signing":{"signerRequired":true,"custodianRequired":false},
			"futureField":"ignored"
		},
		"futureTopLevel":"ignored"
	}`,
		base.DestinationCommitmentHash,
		base.DestinationCommitmentHash,
		base.MigrationTransactionPlan.PlanCommitmentHash,
		base.ArtifactApprovals.Payload.DestinationCommitmentHash,
		base.ArtifactApprovals.Payload.PlanCommitmentHash,
		base.ArtifactApprovals.Approvals[0].Signature,
		base.ArtifactSignatures[0],
		template.DepositorPublicKey,
		template.SignerPublicKey,
	))

	response, err := http.Post(server.URL+"/v1/self_v1/signer/requests", "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}

	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "malformed request body") {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestInitializeRejectsInvalidOrUnavailablePort(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, enabled, err := Initialize(ctx, Config{Port: -1}, handle, nil); err == nil || enabled {
		t.Fatalf("expected invalid negative port to fail, got enabled=%v err=%v", enabled, err)
	}
	if _, enabled, err := Initialize(
		ctx,
		Config{Port: 9711, ListenAddress: "0.0.0.0"},
		handle,
		nil,
	); err == nil || enabled {
		t.Fatalf("expected non-loopback bind without auth token to fail, got enabled=%v err=%v", enabled, err)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(DefaultListenAddress, "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	if _, enabled, err := Initialize(
		ctx,
		Config{Port: port, ListenAddress: DefaultListenAddress},
		handle,
		nil,
	); err == nil || enabled {
		t.Fatalf("expected occupied port to fail, got enabled=%v err=%v", enabled, err)
	}
}

func availableLoopbackPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", net.JoinHostPort(DefaultListenAddress, "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func TestInitializeRequiresQcV1DepositorTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			RequireApprovalTrustRoots: true,
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected missing qc_v1 depositor trust roots to fail, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"covenant signer qc_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeRequiresQcV1CustodianTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected missing qc_v1 custodian trust roots to fail, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"covenant signer qc_v1 routes require custodianTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeRequiresSelfV1DepositorTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			EnableSelfV1:              true,
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
			},
			CustodianTrustRoots: []CustodianTrustRoot{
				testCustodianTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected missing self_v1 depositor trust roots to fail, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"covenant signer self_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeAcceptsRequiredApprovalTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			EnableSelfV1:              true,
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
				testDepositorTrustRoot(TemplateSelfV1),
			},
			CustodianTrustRoots: []CustodianTrustRoot{
				testCustodianTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedEngine{},
	)
	if err != nil || !enabled || server == nil {
		t.Fatalf("expected startup to succeed with required trust roots, got enabled=%v server=%v err=%v", enabled, server != nil, err)
	}
}

func TestIsLoopbackListenAddressAcceptsBracketedIPv6Loopback(t *testing.T) {
	if !isLoopbackListenAddress("[::1]") {
		t.Fatal("expected bracketed IPv6 loopback address to be recognized")
	}
}

func TestServerRequiresBearerTokenWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, "test-token", true))
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http_auth",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	response, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected healthz status: %d", response.StatusCode)
	}

	request, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/v1/self_v1/signer/requests",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected unauthorized submit without bearer token, got %d %s", response.StatusCode, string(body))
	}

	authorizedRequest, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/v1/self_v1/signer/requests",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	authorizedRequest.Header.Set("Content-Type", "application/json")
	authorizedRequest.Header.Set("Authorization", "Bearer test-token")

	authorizedResponse, err := http.DefaultClient.Do(authorizedRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer authorizedResponse.Body.Close()

	if authorizedResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authorizedResponse.Body)
		t.Fatalf("unexpected authorized submit status: %d %s", authorizedResponse.StatusCode, string(body))
	}
}

func TestServerCanKeepSelfV1RoutesDark(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, "", false))
	defer server.Close()

	response, err := http.Post(
		server.URL+"/v1/self_v1/signer/requests",
		"application/json",
		bytes.NewReader(mustJSON(t, SignerSubmitInput{
			RouteRequestID: "ors_http_self_dark",
			Stage:          StageSignerCoordination,
			Request:        baseRequest(TemplateSelfV1),
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected disabled self_v1 route to return 404, got %d %s", response.StatusCode, string(body))
	}

	qcResponse, err := http.Post(
		server.URL+"/v1/qc_v1/signer/requests",
		"application/json",
		bytes.NewReader(mustJSON(t, SignerSubmitInput{
			RouteRequestID: "orq_http_qc",
			Stage:          StageSignerCoordination,
			Request:        baseRequest(TemplateQcV1),
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer qcResponse.Body.Close()

	if qcResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(qcResponse.Body)
		t.Fatalf("expected qc_v1 route to remain available, got %d %s", qcResponse.StatusCode, string(body))
	}
}

func TestServerBoundaryErrorMatrix(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, "test-token", true))
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http_matrix",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	mismatchedPollPayload := mustJSON(t, SignerPollInput{
		RequestID:      "different_id",
		RouteRequestID: "ors_http_matrix",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	oversizedBody := []byte(
		`{"routeRequestId":"ors_big","stage":"SIGNER_COORDINATION","request":{"facadeRequestId":"` +
			strings.Repeat("a", maxRequestBodyBytes+1) + `"}}`,
	)

	testCases := []struct {
		name             string
		method           string
		path             string
		body             []byte
		authHeader       string
		wantStatus       int
		wantBodyContains string
		wantAllow        string
	}{
		{
			name:             "invalid bearer token",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests",
			body:             submitPayload,
			authHeader:       "Bearer wrong-token",
			wantStatus:       http.StatusUnauthorized,
			wantBodyContains: "invalid bearer token",
		},
		{
			name:             "method mismatch on poll path returns 405",
			method:           http.MethodGet,
			path:             "/v1/self_v1/signer/requests/request_1:poll",
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusMethodNotAllowed,
			wantBodyContains: "method not allowed",
			wantAllow:        http.MethodPost,
		},
		{
			name:             "unknown fields in envelope rejected",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests",
			body:             []byte(`{"routeRequestId":"ors_http_unknown","stage":"SIGNER_COORDINATION","request":{},"futureTopLevel":"ignored"}`),
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "malformed request body",
		},
		{
			name:             "oversized body rejected",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests",
			body:             oversizedBody,
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "malformed request body",
		},
		{
			name:             "poll path and body request id mismatch rejected",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests/request_from_path:poll",
			body:             mismatchedPollPayload,
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "requestId in body does not match path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(
				tc.method,
				server.URL+tc.path,
				bytes.NewReader(tc.body),
			)
			if err != nil {
				t.Fatal(err)
			}

			if tc.body != nil {
				request.Header.Set("Content-Type", "application/json")
			}
			if tc.authHeader != "" {
				request.Header.Set("Authorization", tc.authHeader)
			}

			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			if response.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(response.Body)
				t.Fatalf("unexpected status: %d body: %s", response.StatusCode, string(body))
			}

			if tc.wantAllow != "" && response.Header.Get("Allow") != tc.wantAllow {
				t.Fatalf("unexpected Allow header: %q", response.Header.Get("Allow"))
			}

			if tc.wantBodyContains != "" {
				body, _ := io.ReadAll(response.Body)
				if !strings.Contains(string(body), tc.wantBodyContains) {
					t.Fatalf("expected body to contain %q, got %q", tc.wantBodyContains, string(body))
				}
			}
		})
	}
}
