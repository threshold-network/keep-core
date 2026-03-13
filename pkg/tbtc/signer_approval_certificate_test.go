package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
)

func validStructuredSignerApprovalVerificationRequest(
	t *testing.T,
	node *node,
	walletPublicKey *ecdsa.PublicKey,
	route covenantsigner.TemplateID,
) covenantsigner.RouteSubmitRequest {
	t.Helper()

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	depositorPrivateKey, _ := btcec.PrivKeyFromBytes(
		btcec.S256(),
		bytes.Repeat([]byte{0xaa}, 32),
	)

	request := covenantsigner.RouteSubmitRequest{
		RequestType: covenantsigner.RequestTypeReconstruct,
		Route:       route,
		ArtifactApprovals: &covenantsigner.ArtifactApprovalEnvelope{
			Payload: covenantsigner.ArtifactApprovalPayload{
				ApprovalVersion:           1,
				Route:                     route,
				ScriptTemplateID:          route,
				DestinationCommitmentHash: "0x" + strings.Repeat("11", 32),
				PlanCommitmentHash:        "0x" + strings.Repeat("22", 32),
			},
		},
	}

	switch route {
	case covenantsigner.TemplateSelfV1:
		templateJSON, err := json.Marshal(&covenantsigner.SelfV1Template{
			Template:           covenantsigner.TemplateSelfV1,
			DepositorPublicKey: "0x" + hex.EncodeToString(depositorPrivateKey.PubKey().SerializeCompressed()),
			SignerPublicKey:    "0x" + hex.EncodeToString((*btcec.PublicKey)(walletPublicKey).SerializeCompressed()),
			Delta2:             4320,
		})
		if err != nil {
			t.Fatal(err)
		}
		request.ScriptTemplate = templateJSON
		request.ArtifactApprovals.Approvals = []covenantsigner.ArtifactRoleApproval{
			{
				Role: covenantsigner.ArtifactApprovalRoleDepositor,
				Signature: testSignArtifactApproval(
					t,
					depositorPrivateKey,
					request.ArtifactApprovals.Payload,
				),
			},
		}
	case covenantsigner.TemplateQcV1:
		custodianPrivateKey, _ := btcec.PrivKeyFromBytes(
			btcec.S256(),
			bytes.Repeat([]byte{0xbb}, 32),
		)
		templateJSON, err := json.Marshal(&covenantsigner.QcV1Template{
			Template:           covenantsigner.TemplateQcV1,
			DepositorPublicKey: "0x" + hex.EncodeToString(depositorPrivateKey.PubKey().SerializeCompressed()),
			CustodianPublicKey: "0x" + hex.EncodeToString(custodianPrivateKey.PubKey().SerializeCompressed()),
			SignerPublicKey:    "0x" + hex.EncodeToString((*btcec.PublicKey)(walletPublicKey).SerializeCompressed()),
			Beta:               144,
			Delta2:             4320,
		})
		if err != nil {
			t.Fatal(err)
		}
		request.ScriptTemplate = templateJSON
		request.ArtifactApprovals.Approvals = []covenantsigner.ArtifactRoleApproval{
			{
				Role: covenantsigner.ArtifactApprovalRoleDepositor,
				Signature: testSignArtifactApproval(
					t,
					depositorPrivateKey,
					request.ArtifactApprovals.Payload,
				),
			},
			{
				Role: covenantsigner.ArtifactApprovalRoleCustodian,
				Signature: testSignArtifactApproval(
					t,
					custodianPrivateKey,
					request.ArtifactApprovals.Payload,
				),
			},
		}
	default:
		t.Fatalf("unsupported route %s", route)
	}

	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		testArtifactApprovalDigest(t, request.ArtifactApprovals.Payload),
		startBlock,
	)
	if err != nil {
		t.Fatal(err)
	}
	request.SignerApproval = certificate

	return request
}

func TestSigningExecutorCanIssueSignerApprovalCertificateForArbitraryDigest(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256(
		[]byte("psbt-covenant-signer-approval-certificate-spike"),
	)

	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
	)
	if err != nil {
		t.Fatal(err)
	}

	walletChainData, err := executor.chain.GetWallet(
		bitcoin.PublicKeyHash(executor.wallet().publicKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		executor.wallet().publicKey,
		walletChainData,
		executor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySignerApprovalCertificate(certificate, expectedSignerSetHash); err != nil {
		t.Fatalf("expected certificate verification to succeed: %v", err)
	}

	expectedDigest := "0x" + hex.EncodeToString(approvalDigest[:])
	if certificate.ApprovalDigest != expectedDigest {
		t.Fatalf(
			"unexpected approval digest\nexpected: %s\nactual:   %s",
			expectedDigest,
			certificate.ApprovalDigest,
		)
	}
	if certificate.SignerSetHash != expectedSignerSetHash {
		t.Fatalf(
			"unexpected signer set hash\nexpected: %s\nactual:   %s",
			expectedSignerSetHash,
			certificate.SignerSetHash,
		)
	}
	if len(certificate.ActiveMembers) < executor.groupParameters.HonestThreshold {
		t.Fatalf(
			"expected at least honest threshold active members, got %v",
			certificate.ActiveMembers,
		)
	}
	if certificate.EndBlock == nil || *certificate.EndBlock < startBlock {
		t.Fatalf(
			"expected end block [%v] to be >= start block [%v]",
			certificate.EndBlock,
			startBlock,
		)
	}
}

func TestSignerApprovalCertificateVerificationRejectsTamperedDigest(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256([]byte("psbt-covenant-signer-approval-certificate"))
	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
	)
	if err != nil {
		t.Fatal(err)
	}

	walletChainData, err := executor.chain.GetWallet(
		bitcoin.PublicKeyHash(executor.wallet().publicKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		executor.wallet().publicKey,
		walletChainData,
		executor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	tampered := *certificate
	tamperedDigest := sha256.Sum256([]byte("tampered"))
	tampered.ApprovalDigest = "0x" + hex.EncodeToString(tamperedDigest[:])
	if err := verifySignerApprovalCertificate(&tampered, expectedSignerSetHash); err == nil {
		t.Fatal("expected tampered approval digest to fail verification")
	}
}

func TestSignerApprovalCertificateSignerSetHashBindsOnChainWalletIdentityAndThreshold(t *testing.T) {
	_, _, walletPublicKey := setupCovenantSignerTestNode(t)

	baseWalletChainData := &WalletChainData{
		EcdsaWalletID:  sha256.Sum256([]byte("wallet-id-base")),
		MembersIDsHash: sha256.Sum256([]byte("members-hash-base")),
	}
	baseGroupParameters := &GroupParameters{
		GroupSize:       3,
		GroupQuorum:     2,
		HonestThreshold: 2,
	}

	baseHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		baseWalletChainData,
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	changedMembersHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		&WalletChainData{
			EcdsaWalletID:  baseWalletChainData.EcdsaWalletID,
			MembersIDsHash: sha256.Sum256([]byte("members-hash-changed")),
		},
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changedMembersHash == baseHash {
		t.Fatal("expected signer set hash to change when members IDs hash changes")
	}

	changedWalletIDHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		&WalletChainData{
			EcdsaWalletID:  sha256.Sum256([]byte("wallet-id-changed")),
			MembersIDsHash: baseWalletChainData.MembersIDsHash,
		},
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changedWalletIDHash == baseHash {
		t.Fatal("expected signer set hash to change when wallet ID changes")
	}

	thresholdChangedHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		baseWalletChainData,
		&GroupParameters{
			GroupSize:       3,
			GroupQuorum:     2,
			HonestThreshold: 3,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if thresholdChangedHash == baseHash {
		t.Fatal("expected signer set hash to change when honest threshold changes")
	}
}

func TestCovenantSignerEngineVerifySignerApprovalAcceptsValidCertificate(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	if err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request); err != nil {
		t.Fatalf("expected signer approval verification to succeed: %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsWalletPublicKeyMismatch(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)
	request.SignerApproval.WalletPublicKey = "0x04" + strings.Repeat("55", 64)

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval.walletPublicKey must match request.scriptTemplate.signerPublicKey",
	) {
		t.Fatalf("expected wallet public key mismatch error, got %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsMissingCertificate(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)
	request.SignerApproval = nil

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval is required for signer approval verification",
	) {
		t.Fatalf("expected missing signer approval error, got %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsApprovalDigestMismatch(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)
	request.SignerApproval.ApprovalDigest = "0x" + strings.Repeat("11", 32)

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval.approvalDigest must match request.artifactApprovals.payload",
	) {
		t.Fatalf("expected signer approval digest mismatch error, got %v", err)
	}
}
