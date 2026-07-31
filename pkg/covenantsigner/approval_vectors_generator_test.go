package covenantsigner

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const approvalContractVectorsPath = "testdata/covenant_recovery_approval_vectors_v2.json"

// TestGenerateCovenantRecoveryApprovalVectorsV2 regenerates the canonical
// approval-contract vectors from the same production code the runtime uses:
// artifactApprovalDigest for the v2 domain-wrapped approval digest and
// requestDigest (which normalizes first) for the request digest. Every protocol
// change that moves either digest — a new approval or certificate version, a
// changed EIP-712 domain, a new normalization rule — therefore has a
// reproducible way to refresh the fixture instead of hand-edited hashes.
//
// Writes are guarded so the fixture cannot drift silently:
//
//	UPDATE_GOLDEN=1 go test ./pkg/covenantsigner \
//	  -run '^TestGenerateCovenantRecoveryApprovalVectorsV2$' -count=1
//
// Without UPDATE_GOLDEN the test verifies the checked-in derived values instead
// of rewriting them, so an unregenerated fixture fails here as well as in the
// vector assertion tests.
func TestGenerateCovenantRecoveryApprovalVectorsV2(t *testing.T) {
	data, err := os.ReadFile(approvalContractVectorsPath)
	if err != nil {
		t.Fatal(err)
	}

	current := approvalContractVectorsFile{}
	if err := strictUnmarshal(data, &current); err != nil {
		t.Fatal(err)
	}

	regenerated := approvalContractVectorsFile{
		Version: current.Version,
		Scope:   current.Scope,
		Vectors: make(map[string]approvalContractVector, len(current.Vectors)),
	}

	for key, vector := range current.Vectors {
		request := RouteSubmitRequest{}
		if err := strictUnmarshal(vector.CanonicalSubmitRequest, &request); err != nil {
			t.Fatalf("vector %s: %v", key, err)
		}
		if request.ArtifactApprovals == nil {
			t.Fatalf("vector %s: artifact approvals are required", key)
		}

		digestBytes, err := artifactApprovalDigest(
			request.ArtifactApprovals.Payload,
			testEIP712ChainID,
			testEIP712Salt,
		)
		if err != nil {
			t.Fatalf("vector %s: %v", key, err)
		}
		approvalDigest := "0x" + hex.EncodeToString(digestBytes)

		// The certificate commits to the artifact approval it was issued over, so
		// the certificate's approvalDigest is the same v2 domain-wrapped digest.
		// Rebinding it here keeps the two in lockstep across a domain change.
		if request.SignerApproval != nil {
			request.SignerApproval.ApprovalDigest = approvalDigest
		}

		canonicalSubmitRequest, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("vector %s: %v", key, err)
		}

		expectedRequestDigest, err := requestDigest(request, validationOptions{})
		if err != nil {
			t.Fatalf("vector %s: %v", key, err)
		}

		regenerated.Vectors[key] = approvalContractVector{
			CanonicalSubmitRequest: canonicalSubmitRequest,
			ExpectedApprovalDigest: approvalDigest,
			ExpectedRequestDigest:  expectedRequestDigest,
		}
	}

	if os.Getenv("UPDATE_GOLDEN") != "1" {
		for key, vector := range regenerated.Vectors {
			existing, ok := current.Vectors[key]
			if !ok {
				t.Fatalf("vector %s is missing from %s", key, approvalContractVectorsPath)
			}
			if !strings.EqualFold(
				existing.ExpectedApprovalDigest,
				vector.ExpectedApprovalDigest,
			) {
				t.Errorf(
					"vector %s approval digest is stale\nchecked in: %s\nregenerated: %s\nrerun with UPDATE_GOLDEN=1",
					key,
					existing.ExpectedApprovalDigest,
					vector.ExpectedApprovalDigest,
				)
			}
			if !strings.EqualFold(
				existing.ExpectedRequestDigest,
				vector.ExpectedRequestDigest,
			) {
				t.Errorf(
					"vector %s request digest is stale\nchecked in: %s\nregenerated: %s\nrerun with UPDATE_GOLDEN=1",
					key,
					existing.ExpectedRequestDigest,
					vector.ExpectedRequestDigest,
				)
			}
		}

		return
	}

	encoded, err := json.MarshalIndent(regenerated, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		approvalContractVectorsPath,
		append(encoded, '\n'),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	t.Logf("regenerated %s", approvalContractVectorsPath)
}
