package covenantsigner

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/internal/canonicaljson"
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

// faultingDescriptor wraps a memoryDescriptor and returns an injected error
// from Content(), allowing tests to simulate unreadable job files.
type faultingDescriptor struct {
	name      string
	directory string
	err       error
}

func (fd *faultingDescriptor) Name() string             { return fd.name }
func (fd *faultingDescriptor) Directory() string        { return fd.directory }
func (fd *faultingDescriptor) Content() ([]byte, error) { return nil, fd.err }

// contentFaultingHandle extends memoryHandle by injecting faulting descriptors
// into the ReadAll channel alongside normal descriptors. This enables testing
// of load() behavior when individual file reads fail.
type contentFaultingHandle struct {
	*memoryHandle
	faultingDescriptors []*faultingDescriptor
}

func newContentFaultingHandle() *contentFaultingHandle {
	return &contentFaultingHandle{
		memoryHandle: newMemoryHandle(),
	}
}

func (cfh *contentFaultingHandle) AddFaultingDescriptor(name, directory string, err error) {
	cfh.faultingDescriptors = append(cfh.faultingDescriptors, &faultingDescriptor{
		name:      name,
		directory: directory,
		err:       err,
	})
}

func (cfh *contentFaultingHandle) ReadAll() (<-chan persistence.DataDescriptor, <-chan error) {
	dataChan := make(chan persistence.DataDescriptor, len(cfh.items)+len(cfh.faultingDescriptors))
	errorChan := make(chan error)
	for _, item := range cfh.items {
		dataChan <- item
	}
	for _, fd := range cfh.faultingDescriptors {
		dataChan <- fd
	}
	close(dataChan)
	close(errorChan)
	return dataChan, errorChan
}

type scriptedEngine struct {
	submit             func(*Job) (*Transition, error)
	poll               func(*Job) (*Transition, error)
	currentBlockHeight uint64
	currentBlockErr    error
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

func (se *scriptedEngine) CurrentBlockHeight(context.Context) (uint64, error) {
	return se.currentBlockHeight, se.currentBlockErr
}

// hookedHeightEngine is a minimal Engine used only by the expiry-recheck
// regression tests below. Unlike scriptedEngine's static currentBlockHeight
// field, blockHeight is an injectable function so a test can deterministically
// control exactly what each individual call observes and synchronize with a
// concurrent goroutine.
type hookedHeightEngine struct {
	onSubmit    func(*Job) (*Transition, error)
	blockHeight func(context.Context) (uint64, error)
}

func (hhe *hookedHeightEngine) OnSubmit(_ context.Context, job *Job) (*Transition, error) {
	return hhe.onSubmit(job)
}

func (hhe *hookedHeightEngine) OnPoll(context.Context, *Job) (*Transition, error) {
	return nil, nil
}

func (hhe *hookedHeightEngine) CurrentBlockHeight(ctx context.Context) (uint64, error) {
	return hhe.blockHeight(ctx)
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

	data, err := os.ReadFile("testdata/covenant_recovery_approval_vectors_v2.json")
	if err != nil {
		t.Fatal(err)
	}

	vectors := approvalContractVectorsFile{}
	if err := strictUnmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if vectors.Version != 2 {
		t.Fatalf("unexpected vector version: %d", vectors.Version)
	}
	if vectors.Scope != "covenant_recovery_approval_contract_v2" {
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

// testEIP712ChainID and testEIP712Salt are the EIP-712 domain params used across
// tests. They are the zero value so that every existing validationOptions{} and
// NewService (which default these to zero) stays consistent with signatures and
// pinned digests. The domain-wrap code path is still fully exercised; realistic
// wallet compatibility (chainId + salt) is proven separately by the real-wallet
// signature vector test.
var (
	testEIP712ChainID uint64
	testEIP712Salt    [32]byte
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
	digest, err := artifactApprovalDigest(payload, testEIP712ChainID, testEIP712Salt)
	if err != nil {
		panic(err)
	}

	signature, err := privateKey.Sign(digest)
	if err != nil {
		panic(err)
	}

	return "0x" + hex.EncodeToString(signature.Serialize())
}

func mustHighSCompactVariantSignature(signature string) string {
	rawSignature, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		panic(err)
	}

	parsedSignature, err := btcec.ParseDERSignature(rawSignature, btcec.S256())
	if err != nil {
		panic(err)
	}

	highS := new(big.Int).Sub(btcec.S256().N, parsedSignature.S)
	rBytes := parsedSignature.R.Bytes()
	sBytes := highS.Bytes()
	if len(rBytes) > 32 || len(sBytes) > 32 {
		panic("invalid compact signature component length")
	}

	compact := make([]byte, 64)
	copy(compact[32-len(rBytes):32], rBytes)
	copy(compact[64-len(sBytes):64], sBytes)

	return "0x" + hex.EncodeToString(compact)
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

	digest, err := artifactApprovalDigest(artifactApprovals.Payload, testEIP712ChainID, testEIP712Salt)
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
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(
		func(RouteSubmitRequest) error { return nil },
	)))
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_123",
		Stage:          StageSignerCoordination,
		Request:        structuredSignerApprovalRequest(TemplateSelfV1),
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

func TestServiceSubmitRejectsRouteRequestIDDigestMismatch(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(
		func(RouteSubmitRequest) error { return nil },
	)))
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_duplicate_digest_mismatch",
		Stage:          StageSignerCoordination,
		Request:        structuredSignerApprovalRequest(TemplateSelfV1),
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}

	input.Request.FacadeRequestID = "rf_different_payload"

	_, err = service.Submit(context.Background(), TemplateSelfV1, input)
	if err == nil || !strings.Contains(err.Error(), "routeRequestId already exists with a different request payload") {
		t.Fatalf("expected routeRequestId mismatch error, got %v", err)
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
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(
		func(RouteSubmitRequest) error { return nil },
	)))
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_inflight",
		Stage:          StageSignerCoordination,
		Request:        structuredSignerApprovalRequest(TemplateSelfV1),
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

// TestServiceSubmitDedupRejectsStaleResultWhenCertificateExpiresWhileOriginalSubmitIsInFlight
// proves createOrDedup's dedup hit is freshly rechecked rather than blindly
// echoed back. The first Submit call creates the job and then blocks inside
// OnSubmit; while that call is still in flight (the job is durable but only
// JobStateSubmitted, not yet terminal), the current-block height advances
// past EndBlock and a second, deduplicating Submit call for the same
// routeRequestId must observe that expiry -- proving the dedup return path
// itself now rechecks, not just the fresh-job pipeline the other expiry
// regression tests above exercise.
func TestServiceSubmitDedupRejectsStaleResultWhenCertificateExpiresWhileOriginalSubmitIsInFlight(t *testing.T) {
	handle := newMemoryHandle()
	engineStarted := make(chan struct{})
	releaseEngine := make(chan struct{})
	callCount := 0

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			close(engineStarted)
			<-releaseEngine
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount <= 3 {
			// Valid for the first Submit call's top-of-function validation
			// (1), its pre-OnSubmit recheck (2), and the second Submit
			// call's own top-of-function validation (3) -- the second call
			// must pass ordinary validation and reach createOrDedup on its
			// own merits.
			return 100, nil
		}
		// Expired from the second Submit call's dedup fresh-check (4)
		// onward, simulating the certificate lapsing while the first call's
		// OnSubmit is still in flight.
		return 999999999, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	input := SignerSubmitInput{
		RouteRequestID: "dedup_expires_in_flight",
		Stage:          StageSignerCoordination,
		Request:        request,
	}

	firstDone := make(chan struct{})
	var firstErr error
	go func() {
		defer close(firstDone)
		_, firstErr = service.Submit(context.Background(), TemplateSelfV1, input)
	}()

	<-engineStarted

	_, secondErr := service.Submit(context.Background(), TemplateSelfV1, input)
	if secondErr == nil || !strings.Contains(secondErr.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected the deduplicated submit to reject on fresh expiry, got %v", secondErr)
	}

	close(releaseEngine)
	<-firstDone
	if firstErr == nil || !strings.Contains(firstErr.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected the original submit to also reject once its own post-signing recheck runs, got %v", firstErr)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, "dedup_expires_in_flight")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the job to still be present in the store")
	}
	if job.State != JobStateSubmitted {
		t.Fatalf("expected the job to remain in its prior JobStateSubmitted state, got %v", job.State)
	}
}

// TestServiceSubmitDedupRejectsStaleReadyResultAfterCertificateExpires proves
// that a second Submit call for an already-completed (terminal, ready) job
// does not simply echo back the stored ready artifact once the certificate
// has since expired: createOrDedup's fresh recheck must reject it instead of
// returning the stale ready result, and must leave the durable job untouched.
//
// The height hook is deliberately structured so the second Submit call's own
// pre-existing top-of-function validation (on the freshly resubmitted,
// byte-identical certificate) still observes a VALID height and would let the
// request through on its own -- only createOrDedup's fresh recheck of the
// STORED job observes the expired height. This isolates the new dedup-path
// check: without it, this test fails closed for the wrong reason (or not at
// all), since a naive "advance a static height between two sequential Submit
// calls" design would trip the pre-existing top-of-function check instead and
// pass regardless of whether createOrDedup rechecks anything.
func TestServiceSubmitDedupRejectsStaleReadyResultAfterCertificateExpires(t *testing.T) {
	handle := newMemoryHandle()
	firstJobCreated := false
	dedupCallCount := 0
	onSubmitCallCount := 0

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			onSubmitCallCount++
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		if !firstJobCreated {
			// Unconditionally valid while the original job is created and
			// completed. This test does not depend on exactly how many
			// provider calls that pipeline makes internally.
			return 100, nil
		}
		dedupCallCount++
		if dedupCallCount == 1 {
			// The second Submit call's own top-of-function validation --
			// still valid.
			return 100, nil
		}
		// createOrDedup's fresh recheck of the stored job -- expired.
		return 999999999, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	input := SignerSubmitInput{
		RouteRequestID: "dedup_stale_ready",
		Stage:          StageSignerCoordination,
		Request:        request,
	}

	first, err := service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != StepStatusReady {
		t.Fatalf("expected the first submit to complete as ready, got %v", first.Status)
	}
	firstJobCreated = true

	_, err = service.Submit(context.Background(), TemplateSelfV1, input)
	if err == nil || !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected the deduplicated submit to reject the stale ready result, got %v", err)
	}
	if onSubmitCallCount != 1 {
		t.Fatalf("expected OnSubmit to be called exactly once, for the original job only, got %d calls", onSubmitCallCount)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, "dedup_stale_ready")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the job to still be present in the store")
	}
	if job.State != JobStateArtifactReady {
		t.Fatalf("expected the durable job to remain ArtifactReady, got %v", job.State)
	}
	if job.PSBTHash != "0xdeadbeef" {
		t.Fatalf("expected the original ready artifact to remain untouched, got %q", job.PSBTHash)
	}
}

// TestServiceSubmitDedupRejectsStaleReadyResultWhenProviderStartsFailing
// proves createOrDedup's fresh recheck also fails closed -- rather than
// returning the stale ready artifact -- when the current block height
// provider itself starts erroring after the original job completed. As
// above, the second Submit call's own top-of-function validation still
// observes a healthy provider; only createOrDedup's fresh recheck of the
// stored job observes the failure, isolating the new dedup-path check.
func TestServiceSubmitDedupRejectsStaleReadyResultWhenProviderStartsFailing(t *testing.T) {
	handle := newMemoryHandle()
	wantErr := errors.New("blockchain is unavailable")
	firstJobCreated := false
	dedupCallCount := 0
	onSubmitCallCount := 0

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			onSubmitCallCount++
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		if !firstJobCreated {
			return 100, nil
		}
		dedupCallCount++
		if dedupCallCount == 1 {
			// The second Submit call's own top-of-function validation --
			// the provider is still healthy at this point.
			return 100, nil
		}
		// createOrDedup's fresh recheck of the stored job -- the provider
		// has now started failing.
		return 0, wantErr
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	input := SignerSubmitInput{
		RouteRequestID: "dedup_provider_fails",
		Stage:          StageSignerCoordination,
		Request:        request,
	}

	first, err := service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != StepStatusReady {
		t.Fatalf("expected the first submit to complete as ready, got %v", first.Status)
	}
	firstJobCreated = true

	_, err = service.Submit(context.Background(), TemplateSelfV1, input)
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected the deduplicated submit to propagate the provider error, got %v", err)
	}
	if onSubmitCallCount != 1 {
		t.Fatalf("expected OnSubmit to be called exactly once, for the original job only, got %d calls", onSubmitCallCount)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, "dedup_provider_fails")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the job to still be present in the store")
	}
	if job.State != JobStateArtifactReady {
		t.Fatalf("expected the durable job to remain ArtifactReady, got %v", job.State)
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
		{
			name:  "depositor signature must be low-S",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				setArtifactApprovalSignature(
					request.ArtifactApprovals,
					ArtifactApprovalRoleDepositor,
					mustHighSCompactVariantSignature(
						artifactApprovalSignatureByRole(
							request.ArtifactApprovals,
							ArtifactApprovalRoleDepositor,
						),
					),
				)
				request.ArtifactSignatures = canonicalArtifactSignatures(
					request.Route,
					request.ArtifactApprovals,
				)
			},
			expectErr: "request.artifactApprovals.approvals[0].signature must be a low-S secp256k1 signature",
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

	payload, err := canonicaljson.Marshal(normalizedRequest)
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

	payload, err := canonicaljson.Marshal(destinationCommitmentPayload{
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

func TestServicePollRemainsValidAfterMigrationQuoteTrustRootConfigDrift(t *testing.T) {
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
		RouteRequestID: "ors_quote_config_drift",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	service.migrationPlanQuoteTrustRoots = nil

	pollResult, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_quote_config_drift",
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

func TestNewServiceRejectsMixedEthAddressPresenceForSameReserve(t *testing.T) {
	handle := newMemoryHandle()

	// Same (route, reserve) but different networks: one pins an ethAddress, the
	// other does not. Allowing this mix would let a request steer verification
	// to the secp-only sibling scope via its network value, downgrading an
	// operator's intended wallet-signed enforcement.
	withEth := testDepositorTrustRoot(TemplateSelfV1)
	withEth.Network = "regtest"
	withEth.EthAddress = "0x000000000000000000000000000000000000dEaD"

	withoutEth := testDepositorTrustRoot(TemplateSelfV1)
	withoutEth.Network = "testnet"

	_, err := NewService(
		handle,
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{withEth, withoutEth}),
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"must set ethAddress on all network entries or on none",
	) {
		t.Fatalf("expected mixed ethAddress presence error, got %v", err)
	}
}

func TestNormalizeEthAddressRejectsZeroAddress(t *testing.T) {
	_, err := normalizeEthAddress(
		"depositorTrustRoots[0].ethAddress",
		"0x0000000000000000000000000000000000000000",
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"must not be the zero ETH address",
	) {
		t.Fatalf("expected zero ETH address rejection, got %v", err)
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
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "polling"}, nil
		},
		currentBlockHeight: 100,
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

func TestArtifactApprovalDigestMatchesV2Contract(t *testing.T) {
	expectedDigests := map[TemplateID]string{
		TemplateQcV1:   "0xc8246daca36f0116377210140056949b23c37b3de3a5c48ec8d125405a9f05fe",
		TemplateSelfV1: "0x063735af147351025209ba54a606b38598d67e60848d463bbd4bcfcbdf3506c7",
	}

	for _, route := range []TemplateID{TemplateQcV1, TemplateSelfV1} {
		t.Run(string(route), func(t *testing.T) {
			request := canonicalArtifactApprovalRequest(route)

			digest, err := artifactApprovalDigest(request.ArtifactApprovals.Payload, testEIP712ChainID, testEIP712Salt)
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

			digestBytes, err := artifactApprovalDigest(request.ArtifactApprovals.Payload, testEIP712ChainID, testEIP712Salt)
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

func TestRequestDigestUsesDomainSeparation(t *testing.T) {
	request := canonicalArtifactApprovalRequest(TemplateQcV1)

	normalizedRequest, err := normalizeRouteSubmitRequest(request, validationOptions{})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := canonicaljson.Marshal(normalizedRequest)
	if err != nil {
		t.Fatal(err)
	}

	// Manually compute the domain-prefixed digest using the expected domain
	// separator constant value. This verifies that the function prepends
	// the domain before hashing, preventing cross-context hash collisions.
	domainPrefix := "covenant-signer-request-v1:"
	prefixedInput := append([]byte(domainPrefix), payload...)
	expectedSum := sha256.Sum256(prefixedInput)
	expectedDigest := "0x" + hex.EncodeToString(expectedSum[:])

	// Compute the unprefixed digest to prove domain prefix has effect.
	unprefixedSum := sha256.Sum256(payload)
	unprefixedDigest := "0x" + hex.EncodeToString(unprefixedSum[:])

	if expectedDigest == unprefixedDigest {
		t.Fatal("domain-prefixed and unprefixed digests should differ")
	}

	actualDigest, err := requestDigestFromNormalized(normalizedRequest)
	if err != nil {
		t.Fatal(err)
	}

	if actualDigest != expectedDigest {
		t.Fatalf(
			"expected domain-prefixed digest %s, got %s",
			expectedDigest,
			actualDigest,
		)
	}

	if actualDigest == unprefixedDigest {
		t.Fatal("requestDigestFromNormalized should not produce unprefixed digest")
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

// Regression tests for deep audit findings.

// TestServicePollPropagatesCurrentBlockProviderError verifies P1-A: when the
// currentBlockProvider returns an error, Poll returns a wrapped error rather
// than panicking or silently proceeding. The provider only starts failing
// after Submit succeeds, since Submit now also fetches the current height for
// a certificate-bearing request; this isolates the failure to Poll.
func TestServicePollPropagatesCurrentBlockProviderError(t *testing.T) {
	handle := newMemoryHandle()
	wantErr := errors.New("blockchain is unavailable")
	engine := &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "polling"}, nil
		},
		currentBlockHeight: 100,
	}
	service, err := NewService(handle, engine, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(request RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	// Submit a job that has a SignerApproval with an EndBlock, so that
	// currentBlockProvider is called during Poll.
	request := structuredSignerApprovalRequest(TemplateSelfV1)
	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "poll_err",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	engine.currentBlockErr = wantErr

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "poll_err",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil {
		t.Fatal("expected error from currentBlockProvider, got nil")
	}
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected error containing %q, got %v", wantErr.Error(), err)
	}
}

// TestServiceLoadPollJobPropagatesCurrentBlockProviderError verifies P1-A: when
// loadPollJob calls the currentBlockProvider and it returns an error, the job
// load fails with a wrapped error. As above, the provider only starts failing
// after Submit succeeds.
func TestServiceLoadPollJobPropagatesCurrentBlockProviderError(t *testing.T) {
	handle := newMemoryHandle()
	wantErr := errors.New("blockchain is unavailable")
	engine := &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "polling"}, nil
		},
		currentBlockHeight: 100,
	}
	service, err := NewService(handle, engine, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(request RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "load_err",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	engine.currentBlockErr = wantErr

	// loadPollJob is called by Poll; the error should propagate through Poll.
	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "load_err",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil {
		t.Fatal("expected error from currentBlockProvider in loadPollJob, got nil")
	}
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected error containing %q, got %v", wantErr.Error(), err)
	}
}

// TestServicePollRejectsExpiredCertificate verifies P2-D: when the current
// block is past EndBlock, the certificate is considered expired and Poll
// returns an error. The height stays below EndBlock through Submit (which now
// also enforces expiry) and only advances past it before Poll, so the
// certificate genuinely expires between submission and polling.
func TestServicePollRejectsExpiredCertificate(t *testing.T) {
	handle := newMemoryHandle()
	engine := &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "polling"}, nil
		},
		currentBlockHeight: 100, // EndBlock is 123456; well below it at submit time
	}
	service, err := NewService(handle, engine, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(request RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	endBlock := *request.SignerApproval.EndBlock
	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "expired",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Advance the current block height one past EndBlock before polling.
	engine.currentBlockHeight = endBlock + 1

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "expired",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil {
		t.Fatal("expected expiration error, got nil")
	}
	if !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected expiration error, got %v", err)
	}
}

// TestServicePollAcceptsCertificateExactlyAtEndBlock verifies EndBlock is an
// inclusive upper bound: a certificate remains valid, not expired, when the
// current block equals EndBlock exactly.
func TestServicePollAcceptsCertificateExactlyAtEndBlock(t *testing.T) {
	handle := newMemoryHandle()
	engine := &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "polling"}, nil
		},
		currentBlockHeight: 100,
	}
	service, err := NewService(handle, engine, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(request RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	endBlock := *request.SignerApproval.EndBlock
	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "at_end_block",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The current block is now exactly EndBlock -- still valid (inclusive).
	engine.currentBlockHeight = endBlock

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "at_end_block",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatalf("expected no error at exactly EndBlock, got %v", err)
	}
}

// TestServicePollAcceptsValidCertificate verifies P2-D: when the current block
// is before EndBlock, the certificate is valid and Poll proceeds normally.
func TestServicePollAcceptsValidCertificate(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "polling"}, nil
		},
		currentBlockHeight: 100, // EndBlock is 123456, so currentBlock < EndBlock
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(request RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "valid",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "valid",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatalf("expected no error for valid certificate, got %v", err)
	}
}

// TestServiceSubmitRejectsSignerApprovalWithMissingEndBlock verifies that a
// signer approval certificate without an EndBlock is rejected at Submit.
// EndBlock is a mandatory v2 field: there is no fail-open "never expires"
// certificate state.
func TestServiceSubmitRejectsSignerApprovalWithMissingEndBlock(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		currentBlockHeight: 100,
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	request.SignerApproval.EndBlock = nil

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "missing_end_block",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "endBlock is required") {
		t.Fatalf("expected missing endBlock error, got %v", err)
	}
}

// TestServiceAcceptsSignerApprovalCertificateWithEndBlockAboveUint32Range
// proves EndBlock is not artificially capped to uint32. The certificate v2
// signing digest binds EndBlock as a full 8-byte big-endian uint64 (see
// signerApprovalCertificateSigningDigest in pkg/tbtc), so request
// normalization -- and the full Submit/Poll round trip -- must accept and
// preserve a value above math.MaxUint32 rather than rejecting it.
func TestServiceAcceptsSignerApprovalCertificateWithEndBlockAboveUint32Range(t *testing.T) {
	handle := newMemoryHandle()
	endBlock := uint64(math.MaxUint32) + 100000
	if endBlock <= math.MaxUint32 {
		t.Fatalf("test vector must exceed math.MaxUint32, got %d", endBlock)
	}

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
		currentBlockHeight: 100,
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	request.SignerApproval.EndBlock = &endBlock

	normalized, err := normalizeSignerApprovalCertificate(request, testEIP712ChainID, testEIP712Salt)
	if err != nil {
		t.Fatalf("expected EndBlock above math.MaxUint32 to normalize, got %v", err)
	}
	if normalized.EndBlock == nil || *normalized.EndBlock != endBlock {
		t.Fatalf("expected normalized EndBlock to equal %d, got %v", endBlock, normalized.EndBlock)
	}

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "uint64_end_block",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatalf("expected submit to accept EndBlock above math.MaxUint32, got %v", err)
	}
	if submitResult.Status != StepStatusReady {
		t.Fatalf("expected ready status, got %v", submitResult.Status)
	}

	pollResult, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "uint64_end_block",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatalf("expected poll to accept EndBlock above math.MaxUint32, got %v", err)
	}
	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected ready status on poll, got %v", pollResult.Status)
	}
}

// TestServiceSubmitRejectsSignerApprovalMissingCurrentBlockProvider verifies
// that a certificate-bearing Submit fails closed -- rather than treating the
// certificate as unexpired -- when no current block height provider is
// configured at all (the engine implements neither SignerApprovalVerifier nor
// CurrentBlockHeightProvider, so nothing auto-detects a provider).
func TestServiceSubmitRejectsSignerApprovalMissingCurrentBlockProvider(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &passiveEngineWithoutBlockHeight{}, WithSignerApprovalVerifier(
		SignerApprovalVerifierFunc(func(RouteSubmitRequest) error { return nil }),
	))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "missing_provider",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "no current block height is available") {
		t.Fatalf("expected missing provider error, got %v", err)
	}
}

// passiveEngineWithoutBlockHeight implements Engine and SignerApprovalVerifier
// but deliberately not CurrentBlockHeightProvider, to exercise the "verifier
// present, provider absent" fail-closed path at the Service level (Initialize
// rejects this combination at startup; Service itself must also fail closed
// per-request for callers that construct a Service directly).
type passiveEngineWithoutBlockHeight struct{}

func (passiveEngineWithoutBlockHeight) OnSubmit(context.Context, *Job) (*Transition, error) {
	return nil, nil
}

func (passiveEngineWithoutBlockHeight) OnPoll(context.Context, *Job) (*Transition, error) {
	return nil, nil
}

func (passiveEngineWithoutBlockHeight) VerifySignerApproval(RouteSubmitRequest) error {
	return nil
}

// TestServiceSubmitRejectsExpiryDiscoveredBeforeOnSubmit verifies the
// pre-OnSubmit recheck: if the certificate has expired by the time the
// in-flight slot is acquired (e.g. it expired while waiting on the signer
// approval verifier or for a slot), Submit must reject before ever invoking
// OnSubmit -- synchronous threshold signing must never start on an
// authorization that has already lapsed.
func TestServiceSubmitRejectsExpiryDiscoveredBeforeOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	callCount := 0
	onSubmitCalled := false

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			onSubmitCalled = true
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount == 1 {
			return 100, nil // valid at the initial top-of-Submit validation
		}
		return 999999999, nil // expired by the pre-OnSubmit recheck onward
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "expire_before_submit",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected an expiry rejection, got %v", err)
	}
	if onSubmitCalled {
		t.Fatal("expected OnSubmit to never be called once expiry was discovered pre-signing")
	}
}

// TestServiceSubmitRejectsExpiryDiscoveredAfterRealInFlightSemaphoreWait
// proves the pre-OnSubmit recheck catches expiry that occurs while Submit is
// genuinely blocked waiting for a WithMaxInFlight slot -- an actual blocking
// send on the production s.inFlightSlots channel, not a mocked or simulated
// wait. The test occupies the sole slot itself (rather than via a second,
// concurrent Submit call), which makes the blocking point unambiguous and
// race-free by construction: Submit cannot possibly acquire the slot, and
// therefore cannot reach the pre-OnSubmit recheck or OnSubmit itself, until
// the test explicitly releases it.
func TestServiceSubmitRejectsExpiryDiscoveredAfterRealInFlightSemaphoreWait(t *testing.T) {
	handle := newMemoryHandle()
	callCount := 0
	onSubmitCalled := false

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			onSubmitCalled = true
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount == 1 {
			return 100, nil // valid at the initial top-of-Submit validation
		}
		// Expired by the pre-OnSubmit recheck, which is only reachable after
		// the real semaphore wait below is released.
		return 999999999, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithMaxInFlight(1),
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Occupy the sole in-flight slot before Submit ever runs, using the real
	// production channel directly (this test is in-package). Submit's own
	// semaphore acquire is then guaranteed to block for real, rather than
	// possibly racing ahead of a second goroutine that would otherwise need
	// to be observed blocking.
	service.inFlightSlots <- struct{}{}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	submitDone := make(chan struct{})
	var submitErr error
	go func() {
		defer close(submitDone)
		_, submitErr = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
			RouteRequestID: "real_semaphore_wait",
			Stage:          StageSignerCoordination,
			Request:        request,
		})
	}()

	select {
	case <-submitDone:
		t.Fatal("expected Submit to block on the in-flight slot semaphore before completing")
	case <-time.After(100 * time.Millisecond):
	}

	// Release the slot: Submit's semaphore acquire can now proceed, and the
	// pre-OnSubmit recheck that immediately follows observes the expired
	// height.
	<-service.inFlightSlots

	select {
	case <-submitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected Submit to finish after the in-flight slot was released")
	}

	if submitErr == nil || !strings.Contains(submitErr.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected an expiry rejection after the real semaphore wait, got %v", submitErr)
	}
	if onSubmitCalled {
		t.Fatal("expected OnSubmit to never be called once expiry was discovered after the semaphore wait")
	}
}

// TestServiceSubmitRejectsExpiryDiscoveredAfterOnSubmit verifies the
// post-OnSubmit, pre-lock fast recheck: if synchronous threshold signing
// itself takes long enough for the certificate to expire, Submit must reject
// the now-stale result rather than persisting or returning it.
func TestServiceSubmitRejectsExpiryDiscoveredAfterOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	callCount := 0

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount <= 2 {
			// Valid for both the top-of-Submit validation and the
			// pre-OnSubmit recheck.
			return 100, nil
		}
		// Expired for the post-OnSubmit fast recheck onward, simulating
		// signing itself having taken long enough to cross EndBlock.
		return 999999999, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "expire_after_submit",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected an expiry rejection, got %v", err)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, "expire_after_submit")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the job to still be present in the store")
	}
	if job.State != JobStateSubmitted {
		t.Fatalf(
			"expected the job to remain in its prior JobStateSubmitted state, got %v",
			job.State,
		)
	}
	if job.PSBTHash != "" {
		t.Fatal("expected the ready artifact to not be persisted after post-signing expiry")
	}
}

// TestServiceSubmitRejectsExpiryDiscoveredWhileWaitingForMutex is the
// deterministic concurrency regression test for the authoritative,
// mutex-held expiry recheck. The pre-lock fast check observes a still-valid
// certificate; while that check is resolving, another goroutine acquires
// s.mutex, advances the current height past EndBlock, and releases it before
// Submit's own s.mutex.Lock() call. Submit must reject once it acquires the
// mutex, and the ready artifact must never be persisted -- this is the
// TOCTOU window between the fast check and the store write that the
// authoritative check closes.
func TestServiceSubmitRejectsExpiryDiscoveredWhileWaitingForMutex(t *testing.T) {
	handle := newMemoryHandle()

	height := uint64(100)
	callCount := 0
	resumeRequests := make(chan chan struct{})

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStateArtifactReady, Detail: "ready", PSBTHash: "0xdeadbeef"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount == 3 {
			// This is the post-OnSubmit, pre-lock fast check (step 6 of
			// Submit's sequence). Block until another goroutine has acquired
			// s.mutex, advanced the height past EndBlock, and released it --
			// simulating a concurrent Submit/Poll racing in right as this
			// call is about to return a still-valid answer.
			resume := make(chan struct{})
			resumeRequests <- resume
			<-resume
			return 100, nil // still valid for THIS (pre-lock) check
		}
		return height, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	endBlock := *request.SignerApproval.EndBlock

	submitDone := make(chan struct{})
	var submitErr error
	go func() {
		defer close(submitDone)
		_, submitErr = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
			RouteRequestID: "mutex_race",
			Stage:          StageSignerCoordination,
			Request:        request,
		})
	}()

	resume := <-resumeRequests
	service.mutex.Lock()
	height = endBlock + 1
	service.mutex.Unlock()
	close(resume)

	<-submitDone
	if submitErr == nil || !strings.Contains(submitErr.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected an expiry rejection after acquiring the mutex, got %v", submitErr)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, "mutex_race")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the job to still be present in the store")
	}
	if job.State != JobStateSubmitted {
		t.Fatalf(
			"expected the job to remain in its prior JobStateSubmitted state, got %v",
			job.State,
		)
	}
	if job.PSBTHash != "" {
		t.Fatal("expected the ready artifact to not be persisted after expiry was discovered under the mutex")
	}
}

// TestServiceSubmitRejectsStaleTerminalResultWhenCertificateExpiresWhileOnSubmitIsInFlight
// proves that Submit's mutex-held early-return path -- the one that hands
// back a concurrently-advanced or terminal currentJob instead of applying
// this call's own transition -- rechecks certificate freshness before
// returning that job, not after (or never, if the early return fires first).
//
// OnSubmit blocks. While it is in flight, another operation (simulating a
// concurrent Poll racing this Submit call) advances the same durable job
// straight to a terminal ArtifactReady state via the store. Submit's
// post-OnSubmit, pre-lock recheck runs against the ORIGINAL pre-OnSubmit job
// snapshot held in a local variable, not the store, so it does not observe
// this mutation and still passes on a valid height -- the race is only
// observable by whatever reloads the job from the store under the mutex.
//
// The height provider is call-counted: calls 1-3 cover Submit's own
// top-of-function validation, pre-OnSubmit recheck, and post-OnSubmit
// pre-lock recheck, and must all observe a valid height for the race to be
// genuine. A 4th call is the mutex-held recheck of the freshly reloaded
// currentJob; it must happen at all (proving the early return did not skip
// it) and it must observe the expired height (proving the check runs before,
// not after, the early return). Without the fix, the early return fires as
// soon as currentJob.State is seen to be terminal, the 4th call never
// happens, and the stale ArtifactReady result leaks out with an expired
// certificate.
func TestServiceSubmitRejectsStaleTerminalResultWhenCertificateExpiresWhileOnSubmitIsInFlight(t *testing.T) {
	handle := newMemoryHandle()
	onSubmitStarted := make(chan struct{})
	releaseOnSubmit := make(chan struct{})
	callCount := 0

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			close(onSubmitStarted)
			<-releaseOnSubmit
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount <= 3 {
			// Valid through Submit's top-of-function validation (1),
			// pre-OnSubmit recheck (2), and post-OnSubmit pre-lock recheck
			// (3) -- none of these reload the concurrently-mutated store, so
			// they must all pass on their own merits for the race below to
			// be the only thing standing between the stale result and the
			// caller.
			return 100, nil
		}
		// The 4th call, if it happens, is the mutex-held recheck of the
		// freshly reloaded currentJob -- it must observe the certificate as
		// expired.
		return 999999999, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	input := SignerSubmitInput{
		RouteRequestID: "submit_terminal_race",
		Stage:          StageSignerCoordination,
		Request:        request,
	}

	submitDone := make(chan struct{})
	var submitResult StepResult
	var submitErr error
	go func() {
		defer close(submitDone)
		submitResult, submitErr = service.Submit(context.Background(), TemplateSelfV1, input)
	}()

	<-onSubmitStarted

	// Simulate a concurrent Poll advancing the same durable job straight to a
	// terminal ArtifactReady state while this Submit call's OnSubmit is still
	// in flight -- exactly the race the "another poll already advanced the
	// stored job" early return exists to handle.
	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the submitted job to be durable while OnSubmit is in flight")
	}
	storedJob.State = JobStateArtifactReady
	storedJob.Detail = "ready"
	storedJob.PSBTHash = "0xleaked"
	storedJob.CompletedAt = "2024-01-01T00:00:00Z"
	storedJob.UpdatedAt = storedJob.CompletedAt
	if err := service.store.Put(storedJob); err != nil {
		t.Fatal(err)
	}

	close(releaseOnSubmit)
	<-submitDone

	if submitErr == nil || !strings.Contains(submitErr.Error(), "signer approval certificate has expired") {
		t.Fatalf(
			"expected Submit to reject the stale terminal result on fresh expiry, got result=%#v err=%v",
			submitResult, submitErr,
		)
	}
	if submitResult.PSBTHash == "0xleaked" {
		t.Fatal("expected the concurrently-persisted terminal result to not be returned")
	}
	if callCount != 4 {
		t.Fatalf(
			"expected the mutex-held recheck to run as a 4th provider call before the early return, got %d calls",
			callCount,
		)
	}

	persisted, ok, err := service.store.GetByRequestID(storedJob.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the job to still be present in the store")
	}
	if persisted.State != JobStateArtifactReady || persisted.PSBTHash != "0xleaked" {
		t.Fatalf(
			"expected the concurrently-persisted terminal job to remain untouched, got %#v",
			persisted,
		)
	}
}

// TestServiceSubmitRejectsAlreadyExpiredCertificate verifies that Submit
// fails closed on the very first, top-of-function expiry check when the
// certificate is already expired before any validation or signing work
// begins.
func TestServiceSubmitRejectsAlreadyExpiredCertificate(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		currentBlockHeight: 999999999,
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "already_expired",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected an expiry rejection, got %v", err)
	}
}

// TestServiceSubmitPropagatesCurrentBlockProviderError verifies that Submit
// (not just Poll) fails closed with a wrapped error when the current block
// height provider itself errors for a certificate-bearing request.
func TestServiceSubmitPropagatesCurrentBlockProviderError(t *testing.T) {
	handle := newMemoryHandle()
	wantErr := errors.New("blockchain is unavailable")
	service, err := NewService(handle, &scriptedEngine{
		currentBlockErr: wantErr,
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "submit_provider_err",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected error containing %q, got %v", wantErr.Error(), err)
	}
}

// TestServicePollRejectsMissingCurrentBlockProvider verifies Poll fails
// closed -- rather than treating the certificate as unexpired -- when no
// current block height provider is configured, even though the job was
// submitted successfully through a different, provider-backed service
// instance sharing the same underlying storage. This models a deployment
// misconfiguration where the poll path loses access to the provider that the
// submit path had.
func TestServicePollRejectsMissingCurrentBlockProvider(t *testing.T) {
	handle := newMemoryHandle()

	submitService, err := NewService(handle, &scriptedEngine{
		submit:             func(*Job) (*Transition, error) { return &Transition{State: JobStatePending, Detail: "queued"}, nil },
		currentBlockHeight: 100,
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	submitResult, err := submitService.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "poll_missing_provider",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	pollService, err := NewService(handle, &passiveEngineWithoutBlockHeight{}, WithSignerApprovalVerifier(
		SignerApprovalVerifierFunc(func(RouteSubmitRequest) error { return nil }),
	))
	if err != nil {
		t.Fatal(err)
	}

	_, err = pollService.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "poll_missing_provider",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	// The exact rejection point (top-of-Poll validation of the resubmitted
	// certificate, or the stored-job recheck) is an implementation detail;
	// both independently fail closed on a missing provider. What matters is
	// that Poll rejects rather than treating the certificate as unexpired.
	if err == nil || !strings.Contains(err.Error(), "no current block height") {
		t.Fatalf("expected a missing-provider rejection, got %v", err)
	}
}

// TestServicePollRejectsLegacyStoredJobWithMissingEndBlock models a job
// persisted before v2 EndBlock enforcement rolled out: its SignerApproval has
// no EndBlock at all, because that was legal under v1. Such a job cannot have
// passed through the current (v2-enforcing) Submit path, so it is
// constructed directly against the store, the way an on-disk legacy file
// would load. Polling it must fail closed rather than treat the missing
// EndBlock as "never expires".
//
// The poll is resubmitted with a fresh, structurally valid v2 request (not
// the stored legacy one) so that top-of-Poll input validation passes and the
// rejection is specifically attributable to loadPollJob's stored-job recheck,
// not to the resubmitted certificate itself being malformed. A real client
// replaying its only (legacy) certificate would be rejected even earlier, at
// input validation -- also intentionally failing closed, just at a different
// layer.
func TestServicePollRejectsLegacyStoredJobWithMissingEndBlock(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		poll:               func(*Job) (*Transition, error) { return nil, nil },
		currentBlockHeight: 100,
	}, WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	legacyRequest := structuredSignerApprovalRequest(TemplateSelfV1)
	legacyRequest.SignerApproval.CertificateVersion = 1
	legacyRequest.SignerApproval.EndBlock = nil

	// The stored request digest is irrelevant to this test: loadPollJob's
	// ensureStoredCertificateTimely check runs, and must reject, before the
	// digest is ever compared. It is also no longer computable through the
	// normal requestDigest path, since normalization now rejects a nil
	// EndBlock unconditionally -- exactly like a real legacy v1 job file
	// loaded from disk, whose digest predates that requirement.
	legacyJob := &Job{
		RequestID:      "kcs_self_legacy",
		RouteRequestID: "legacy_no_end_block",
		Route:          TemplateSelfV1,
		RequestDigest:  "0xdeadbeef",
		State:          JobStatePending,
		Detail:         "queued",
		CreatedAt:      "2026-01-01T00:00:00Z",
		UpdatedAt:      "2026-01-01T00:00:00Z",
		Request:        legacyRequest,
	}
	if err := service.store.Put(legacyJob); err != nil {
		t.Fatal(err)
	}

	freshPollBody := structuredSignerApprovalRequest(TemplateSelfV1)

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "legacy_no_end_block",
		RequestID:      "kcs_self_legacy",
		Stage:          StageSignerCoordination,
		Request:        freshPollBody,
	})
	if err == nil || !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected a legacy job with a missing end block to fail closed, got %v", err)
	}
}

// TestServicePollRejectsExpiryDiscoveredDuringOnPoll verifies Poll's
// equivalent of Submit's post-signing recheck: loadPollJob is called both
// before and after OnPoll, so a certificate that was still valid when Poll
// started but expires while OnPoll is running (e.g. OnPoll itself triggers
// slow work) is still caught by the second, mutex-held loadPollJob call
// before any transition is persisted.
func TestServicePollRejectsExpiryDiscoveredDuringOnPoll(t *testing.T) {
	handle := newMemoryHandle()
	callCount := 0

	engine := &hookedHeightEngine{
		onSubmit: func(*Job) (*Transition, error) {
			// Left non-terminal so the job is still pollable afterward.
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}
	engine.blockHeight = func(context.Context) (uint64, error) {
		callCount++
		if callCount <= 6 {
			// Valid through Submit's four checks, Poll's top-of-function
			// validation, and the first (pre-OnPoll) loadPollJob call.
			return 100, nil
		}
		// Expired for the second (post-OnPoll, mutex-held) loadPollJob call
		// onward, simulating OnPoll itself having taken long enough to cross
		// EndBlock.
		return 999999999, nil
	}

	service, err := NewService(
		handle,
		engine,
		WithSignerApprovalVerifier(SignerApprovalVerifierFunc(func(RouteSubmitRequest) error {
			return nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := structuredSignerApprovalRequest(TemplateSelfV1)
	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "expire_during_poll",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "expire_during_poll",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "signer approval certificate has expired") {
		t.Fatalf("expected an expiry rejection, got %v", err)
	}
}

// TestNormalizeSignerApprovalCertificateRejectsV1CertificateVersion verifies
// that a v1 signerApproval certificate is rejected outright rather than
// accepted through a compatibility path.
func TestNormalizeSignerApprovalCertificateRejectsV1CertificateVersion(t *testing.T) {
	request := structuredSignerApprovalRequest(TemplateSelfV1)
	request.SignerApproval.CertificateVersion = 1

	_, err := normalizeSignerApprovalCertificate(request, testEIP712ChainID, testEIP712Salt)
	if err == nil || !strings.Contains(err.Error(), "request.signerApproval.certificateVersion must equal 2") {
		t.Fatalf("expected v1 certificate version rejection, got %v", err)
	}
}
