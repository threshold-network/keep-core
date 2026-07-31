package covenantsigner

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// testDepositorEthPrivateKeyHex is a deterministic test-only Ethereum key whose
// address is pinned as a depositor ETH identity in the integration tests below.
const testDepositorEthPrivateKeyHex = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

// mustEthArtifactApprovalSignature signs the v2 domain-wrapped approval digest
// with an Ethereum key, returning the 65-byte r‖s‖v hex a wallet emits (v in
// {27,28}). It uses the zero test EIP-712 domain, matching a Service built
// without WithEIP712Domain.
func mustEthArtifactApprovalSignature(
	t *testing.T,
	privateKeyHex string,
	payload ArtifactApprovalPayload,
) string {
	t.Helper()

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := artifactApprovalDigest(payload, testEIP712ChainID, testEIP712Salt)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.Sign(digest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	return "0x" + hex.EncodeToString(signature)
}

// ethSignedSelfV1Request builds a valid self_v1 submit request whose depositor
// artifact approval is signed by the given Ethereum key instead of the
// secp256k1 script key, keeping the legacy artifactSignatures consistent.
func ethSignedSelfV1Request(t *testing.T, ethPrivateKeyHex string) RouteSubmitRequest {
	t.Helper()

	request := baseRequest(TemplateSelfV1)
	request.ArtifactApprovals.Approvals[0].Signature = mustEthArtifactApprovalSignature(
		t,
		ethPrivateKeyHex,
		request.ArtifactApprovals.Payload,
	)
	request.ArtifactSignatures = canonicalArtifactSignatures(
		request.Route,
		request.ArtifactApprovals,
	)
	return request
}

func TestServiceAcceptsSelfV1WithPinnedDepositorEthIdentity(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(testDepositorEthPrivateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	ethAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	trustRoot := testDepositorTrustRoot(TemplateSelfV1)
	trustRoot.EthAddress = ethAddress

	service, err := NewService(
		newMemoryHandle(),
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{trustRoot}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_eth_identity_match",
		Stage:          StageSignerCoordination,
		Request:        ethSignedSelfV1Request(t, testDepositorEthPrivateKeyHex),
	})
	if err != nil {
		t.Fatalf("expected ETH-signed approval to be accepted, got %v", err)
	}
}

// TestServiceAcceptsRedeemWithPinnedDepositorEthIdentity is the headline flow of
// vba-dashboard#172: a cooperative REDEEM whose depositor artifact approval is
// signed by a connected ETH wallet (eth_signTypedData_v4-shaped) and verified via
// the pinned depositor ETH identity.
func TestServiceAcceptsRedeemWithPinnedDepositorEthIdentity(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(testDepositorEthPrivateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	ethAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	trustRoot := testDepositorTrustRoot(TemplateSelfV1)
	trustRoot.EthAddress = ethAddress

	service, err := NewService(
		newMemoryHandle(),
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{trustRoot}),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := redeemSelfV1Request(t)
	request.ArtifactApprovals.Approvals[0].Signature = mustEthArtifactApprovalSignature(
		t,
		testDepositorEthPrivateKeyHex,
		request.ArtifactApprovals.Payload,
	)
	request.ArtifactSignatures = canonicalArtifactSignatures(
		request.Route,
		request.ArtifactApprovals,
	)

	if _, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_redeem_eth_identity",
		Stage:          StageSignerCoordination,
		Request:        request,
	}); err != nil {
		t.Fatalf("expected wallet-signed redeem to be accepted, got %v", err)
	}
}

func TestServiceRejectsSelfV1WithWrongDepositorEthIdentity(t *testing.T) {
	// Pin a different ETH address than the one that signed the approval.
	trustRoot := testDepositorTrustRoot(TemplateSelfV1)
	trustRoot.EthAddress = "0x000000000000000000000000000000000000dEaD"

	service, err := NewService(
		newMemoryHandle(),
		&scriptedEngine{},
		WithDepositorTrustRoots([]DepositorTrustRoot{trustRoot}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_eth_identity_mismatch",
		Stage:          StageSignerCoordination,
		Request:        ethSignedSelfV1Request(t, testDepositorEthPrivateKeyHex),
	})
	if err == nil || !strings.Contains(err.Error(), "depositor ETH address") {
		t.Fatalf("expected depositor ETH address mismatch error, got %v", err)
	}
}

// TestServicePollAcceptsEthSignedSelfV1Approval guards against a regression
// where a wallet-signed (ETH-identity) approval accepted at Submit is later
// rejected at every Poll: Poll's re-validation must reuse the depositor ETH
// address pinned at submit time rather than silently falling back to the
// secp256k1 script-key check, which cannot verify an ETH-style signature.
func TestServicePollAcceptsEthSignedSelfV1Approval(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(testDepositorEthPrivateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	ethAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	trustRoot := testDepositorTrustRoot(TemplateSelfV1)
	trustRoot.EthAddress = ethAddress

	service, err := NewService(
		newMemoryHandle(),
		&scriptedEngine{
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
		},
		WithDepositorTrustRoots([]DepositorTrustRoot{trustRoot}),
	)
	if err != nil {
		t.Fatal(err)
	}

	request := ethSignedSelfV1Request(t, testDepositorEthPrivateKeyHex)

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_self_eth_poll",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatalf("expected ETH-signed approval to be accepted at submit, got %v", err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_self_eth_poll",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatalf("expected ETH-signed approval to also be accepted at poll, got %v", err)
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected READY, got %s", pollResult.Status)
	}
}
