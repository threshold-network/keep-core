package covenantsigner

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// realWalletVector pins a real wallet-produced signature over a v2 domain-wrapped
// artifact approval digest, proving eth_signTypedData_v4 compatibility and
// locking the v-byte (27/28) and low-S handling of the ecrecover path.
//
// digestHex is NOT self-derived: it was independently computed by the eth_account
// (0.13.7) EIP-712 encoder — the same encoding a wallet's eth_signTypedData_v4
// uses — for the domain/types/message below, and asserted equal to keep-core's
// artifactApprovalDigest. This cross-implementation equality is what proves the
// domain wrap is wallet-correct; the self-signed signature below only locks the
// v-byte/low-S handling. Reproduce with (chainId Sepolia 11155111, default salt):
//
//	from eth_account.messages import encode_typed_data
//	from eth_utils import keccak
//	salt  = keccak(text="tBTC Covenant Artifact Approval Domain v2")
//	route = keccak(text="self_v1")
//	typed = {"types": {
//	    "EIP712Domain":[{"name":"name","type":"string"},{"name":"version","type":"string"},
//	                    {"name":"chainId","type":"uint256"},{"name":"salt","type":"bytes32"}],
//	    "ArtifactApproval":[{"name":"approvalVersion","type":"uint8"},{"name":"route","type":"bytes32"},
//	                    {"name":"scriptTemplateId","type":"bytes32"},{"name":"destinationCommitmentHash","type":"bytes32"},
//	                    {"name":"planCommitmentHash","type":"bytes32"}]},
//	  "primaryType":"ArtifactApproval",
//	  "domain":{"name":"tBTC Covenant Artifact Approval","version":"2","chainId":11155111,"salt":salt},
//	  "message":{"approvalVersion":2,"route":route,"scriptTemplateId":route,
//	    "destinationCommitmentHash":bytes.fromhex("913b...cdf9"),"planCommitmentHash":bytes.fromhex("c14b...c969")}}
//	m = encode_typed_data(full_message=typed); digest = keccak(b"\x19\x01"+m.header+m.body)
//
// Note the dApp must pre-hash the route/scriptTemplateId identifiers to bytes32
// (keccak of the string) to reproduce this digest, matching the bytes32 fields.
// If the digest formula, domain, or version changes, this pinned value no longer
// matches keep-core AND must be re-derived externally, not simply copied.
var realWalletVector = struct {
	privateKeyHex string
	address       string
	chainID       uint64
	payload       ArtifactApprovalPayload
	digestHex     string
	signatureHex  string
}{
	privateKeyHex: "0x4646464646464646464646464646464646464646464646464646464646464646",
	address:       "0x9d8A62f656a8d1615C1294fd71e9CFb3E4855A4F",
	chainID:       11155111,
	payload: ArtifactApprovalPayload{
		ApprovalVersion:           artifactApprovalVersion,
		Route:                     TemplateSelfV1,
		ScriptTemplateID:          TemplateSelfV1,
		DestinationCommitmentHash: "0x913b832b3736a29966fd53f8a733a7587b150d3dfacb1c2d54994c1d3e56cdf9",
		PlanCommitmentHash:        "0xc14b6b7c58211ceaee8f57a39c07481d9835ef959dbd6a02908312db4cf3c969",
	},
	digestHex:    "0x8561df74bc316a8838316fcbb477b693fc17a8f7ba9c3f40520d461c43cb2705",
	signatureHex: "0xccfd9522f1e5c6db9add5808b9338d9bed3badb4d790204ded545250fdadd48525ec39ec4eb46433c2681b8cca583e56cf3ad91082270364d74cf4afef79da7e1b",
}

func TestArtifactApprovalRealWalletSignatureVector(t *testing.T) {
	// The digest keep-core computes must match the pinned wallet-signed digest.
	digest, err := artifactApprovalDigest(
		realWalletVector.payload,
		realWalletVector.chainID,
		defaultArtifactApprovalDomainSalt,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := "0x" + hex.EncodeToString(digest); got != realWalletVector.digestHex {
		t.Fatalf("digest drift: expected %s, got %s", realWalletVector.digestHex, got)
	}

	// The pinned wallet signature (v in {27,28}) must verify against the address.
	if err := verifyEthSignature(
		"depositor",
		realWalletVector.address,
		digest,
		realWalletVector.signatureHex,
	); err != nil {
		t.Fatalf("pinned wallet signature must verify, got %v", err)
	}

	// The equivalent {0,1} recovery-id form must also verify.
	rawSignature, err := hex.DecodeString(realWalletVector.signatureHex[2:])
	if err != nil {
		t.Fatal(err)
	}
	legacyForm := make([]byte, 65)
	copy(legacyForm, rawSignature)
	legacyForm[64] -= 27
	if err := verifyEthSignature(
		"depositor",
		realWalletVector.address,
		digest,
		"0x"+hex.EncodeToString(legacyForm),
	); err != nil {
		t.Fatalf("legacy recovery-id form must verify, got %v", err)
	}

	// A different pinned address must be rejected (ecrecover-and-compare).
	if err := verifyEthSignature(
		"depositor",
		"0x000000000000000000000000000000000000dead",
		digest,
		realWalletVector.signatureHex,
	); err == nil {
		t.Fatal("expected verification against a wrong address to fail")
	}

	// A high-S signature (non-canonical) must be rejected per EIP-2.
	highS := make([]byte, 65)
	copy(highS, rawSignature)
	order := crypto.S256().Params().N
	s := new(big.Int).SetBytes(rawSignature[32:64])
	highSValue := new(big.Int).Sub(order, s)
	var sBytes [32]byte
	highSValue.FillBytes(sBytes[:])
	copy(highS[32:64], sBytes[:])
	if err := verifyEthSignature(
		"depositor",
		realWalletVector.address,
		digest,
		"0x"+hex.EncodeToString(highS),
	); err == nil {
		t.Fatal("expected high-S signature to be rejected")
	}
}
