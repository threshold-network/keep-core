package roast

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// coordinatorSeedVectorsPath is the cross-language conformance vector
// file for the normative coordinator-shuffle seed derivation (RFC-21
// Annex A). The Rust signer carries a byte-identical copy at
// pkg/tbtc/signer/testdata/coordinator_seed_vectors.json and pins the
// same expectations; this file is the canonical source, regenerated
// from the Go implementation via TestRegenerateCoordinatorSeedVectors.
const coordinatorSeedVectorsPath = "testdata/coordinator_seed_vectors.json"

type coordinatorSeedVectorFile struct {
	Description string                  `json:"description"`
	Vectors     []coordinatorSeedVector `json:"vectors"`
}

type coordinatorSeedVector struct {
	Name string `json:"name"`
	// KeyGroup is the canonical FROST key-group handle. Its UTF-8
	// bytes are the DkgGroupPublicKey input to DeriveAttemptSeed
	// (for FrostTBTCSignerV1 material this is the lowercase hex
	// encoding of the serialized group verifying key).
	KeyGroup         string `json:"keyGroup"`
	SessionID        string `json:"sessionID"`
	MessageDigestHex string `json:"messageDigestHex"`
	// IncludedMembers is the canonical ascending included set.
	IncludedMembers []uint16 `json:"includedMembers"`
	// AttemptNumber is the RFC-21 0-based attempt number used in the
	// shuffle-source composition.
	AttemptNumber uint32 `json:"attemptNumber"`
	// WireAttemptNumber is the 1-based attempt number carried by the
	// tbtc-signer FFI AttemptContext for the same logical attempt:
	// always AttemptNumber + 1.
	WireAttemptNumber uint32 `json:"wireAttemptNumber"`
	// ExpectedShuffleSeedInt64 is the folded legacy int64 shuffle
	// seed, encoded as a decimal string so JSON number precision
	// cannot corrupt it.
	ExpectedShuffleSeedInt64 string `json:"expectedShuffleSeedInt64"`
	ExpectedCoordinator      uint16 `json:"expectedCoordinator"`
}

func loadCoordinatorSeedVectors(t *testing.T) coordinatorSeedVectorFile {
	t.Helper()
	raw, err := os.ReadFile(coordinatorSeedVectorsPath)
	if err != nil {
		t.Fatalf(
			"cannot read %s (regenerate with ROAST_SEED_VECTORS_REGEN=1): %v",
			coordinatorSeedVectorsPath,
			err,
		)
	}
	var file coordinatorSeedVectorFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("cannot parse %s: %v", coordinatorSeedVectorsPath, err)
	}
	if len(file.Vectors) == 0 {
		t.Fatalf("%s contains no vectors", coordinatorSeedVectorsPath)
	}
	return file
}

func deriveCoordinatorSeedVectorOutputs(
	t *testing.T,
	vector coordinatorSeedVector,
) (int64, group.MemberIndex) {
	t.Helper()
	decoded, err := hex.DecodeString(vector.MessageDigestHex)
	if err != nil || len(decoded) != attempt.MessageDigestLength {
		t.Fatalf(
			"vector %q: messageDigestHex must decode to exactly %d bytes",
			vector.Name,
			attempt.MessageDigestLength,
		)
	}
	var digest [attempt.MessageDigestLength]byte
	copy(digest[:], decoded)

	seed := attempt.DeriveAttemptSeed(
		[]byte(vector.KeyGroup),
		vector.SessionID,
		digest,
	)
	folded := foldAttemptSeed(seed)

	members := make([]group.MemberIndex, 0, len(vector.IncludedMembers))
	for _, m := range vector.IncludedMembers {
		members = append(members, group.MemberIndex(m))
	}
	coordinator, err := SelectCoordinator(
		members,
		folded,
		uint(vector.AttemptNumber),
	)
	if err != nil {
		t.Fatalf("vector %q: SelectCoordinator: %v", vector.Name, err)
	}
	return folded, coordinator
}

// TestCoordinatorSeedDerivation_ConformanceVectors pins the normative
// RFC-21 Annex A derivation end to end:
//
//	AttemptSeed32   = SHA256(KeyGroupBytes || SessionID || MessageDigest)
//	ShuffleSeed_i64 = int64_be(AttemptSeed32[0:8])
//	SourceSeed      = ShuffleSeed_i64 + int64(AttemptNumber)   // 0-based
//	Coordinator     = GoMathRandShuffle(sorted(IncludedSet), SourceSeed)[0]
//
// Any semantic change to DeriveAttemptSeed, foldAttemptSeed, or
// SelectCoordinator fails this suite, and the byte-identical vector
// copy in the Rust signer fails its mirror test -- either side
// drifting breaks its own CI rather than fracturing coordinator
// agreement in a mixed deployment.
func TestCoordinatorSeedDerivation_ConformanceVectors(t *testing.T) {
	file := loadCoordinatorSeedVectors(t)

	sawNegativeSeed := false
	for _, vector := range file.Vectors {
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			if vector.WireAttemptNumber != vector.AttemptNumber+1 {
				t.Fatalf(
					"wireAttemptNumber [%d] must equal attemptNumber+1 [%d]",
					vector.WireAttemptNumber,
					vector.AttemptNumber+1,
				)
			}

			folded, coordinator := deriveCoordinatorSeedVectorOutputs(t, vector)

			expectedSeed, err := strconv.ParseInt(
				vector.ExpectedShuffleSeedInt64, 10, 64,
			)
			if err != nil {
				t.Fatalf(
					"expectedShuffleSeedInt64 %q is not a valid int64: %v",
					vector.ExpectedShuffleSeedInt64, err,
				)
			}
			if folded != expectedSeed {
				t.Fatalf(
					"shuffle seed mismatch: derived %d, vector pins %d",
					folded, expectedSeed,
				)
			}
			if coordinator != group.MemberIndex(vector.ExpectedCoordinator) {
				t.Fatalf(
					"coordinator mismatch: derived %d, vector pins %d",
					coordinator, vector.ExpectedCoordinator,
				)
			}
		})
		if vector.ExpectedShuffleSeedInt64 != "" && vector.ExpectedShuffleSeedInt64[0] == '-' {
			sawNegativeSeed = true
		}
	}

	// The legacy seed is a reinterpreted uint64, so roughly half of
	// all derivations are negative. Keep at least one negative pin in
	// the file so a sign-handling regression (e.g. an unsigned port)
	// cannot pass.
	if !sawNegativeSeed {
		t.Fatal("vector file must pin at least one negative shuffle seed")
	}
}

// TestRegenerateCoordinatorSeedVectors rewrites the conformance
// vector file from the deterministic input matrix below using the
// current Go implementation. Guarded behind an env flag so it never
// rewrites during normal CI:
//
//	ROAST_SEED_VECTORS_REGEN=1 go test ./pkg/frost/roast -run TestRegenerateCoordinatorSeedVectors
//
// After regenerating, copy the file byte-identically to
// pkg/tbtc/signer/testdata/coordinator_seed_vectors.json on the
// signer branch.
func TestRegenerateCoordinatorSeedVectors(t *testing.T) {
	if os.Getenv("ROAST_SEED_VECTORS_REGEN") != "1" {
		t.Skip("set ROAST_SEED_VECTORS_REGEN=1 to regenerate the vector file")
	}

	productionKeyGroup := "024d79b696a25e478a1c747fcaad380a" +
		"ddbd8b2ef7c333126ab2e2c3b2533b7df2"
	opaqueKeyGroup := "roast-vector-opaque-key-group-handle"

	digestA := make([]byte, attempt.MessageDigestLength)
	for i := range digestA {
		digestA[i] = byte(i)
	}
	digestB := make([]byte, attempt.MessageDigestLength)
	for i := range digestB {
		digestB[i] = byte(0xf0 - i)
	}

	wideSet := make([]uint16, 0, 100)
	for m := uint16(1); m <= 100; m++ {
		wideSet = append(wideSet, m)
	}

	type vectorInput struct {
		name      string
		keyGroup  string
		sessionID string
		digest    []byte
		members   []uint16
		attempt   uint32
	}
	inputs := []vectorInput{
		{
			name:      "five-members-attempt-0",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestA,
			members:   []uint16{1, 2, 3, 4, 5},
			attempt:   0,
		},
		{
			name:      "five-members-attempt-1",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestA,
			members:   []uint16{1, 2, 3, 4, 5},
			attempt:   1,
		},
		{
			name:      "five-members-attempt-5",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestA,
			members:   []uint16{1, 2, 3, 4, 5},
			attempt:   5,
		},
		{
			name:      "sparse-members-attempt-0",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestA,
			members:   []uint16{2, 7, 9, 11},
			attempt:   0,
		},
		{
			name:      "different-session-changes-seed",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-2",
			digest:    digestA,
			members:   []uint16{1, 2, 3, 4, 5},
			attempt:   0,
		},
		{
			name:      "different-digest-changes-seed",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestB,
			members:   []uint16{1, 2, 3, 4, 5},
			attempt:   0,
		},
		{
			name:      "opaque-key-group-handle",
			keyGroup:  opaqueKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestA,
			members:   []uint16{1, 2, 3, 4, 5},
			attempt:   0,
		},
		{
			name:      "production-group-size-attempt-0",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestA,
			members:   wideSet,
			attempt:   0,
		},
		{
			name:      "production-group-size-attempt-3",
			keyGroup:  productionKeyGroup,
			sessionID: "session-roast-seed-vector-1",
			digest:    digestB,
			members:   wideSet,
			attempt:   3,
		},
		{
			name:      "opaque-key-group-wide-set-attempt-7",
			keyGroup:  opaqueKeyGroup,
			sessionID: "session-roast-seed-vector-2",
			digest:    digestB,
			members:   wideSet,
			attempt:   7,
		},
	}

	file := coordinatorSeedVectorFile{
		Description: "Cross-language conformance vectors for the RFC-21 Annex A " +
			"coordinator-shuffle seed derivation: ShuffleSeed_i64 = " +
			"int64_be(SHA256(KeyGroupBytes || SessionID || MessageDigest)[0:8]); " +
			"coordinator = GoMathRandShuffle(sorted(IncludedMembers), " +
			"ShuffleSeed_i64 + int64(AttemptNumber))[0] with the 0-based RFC-21 " +
			"AttemptNumber. wireAttemptNumber is the 1-based tbtc-signer FFI " +
			"encoding of the same attempt. Canonical copy: " +
			"pkg/frost/roast/testdata/coordinator_seed_vectors.json (Go); " +
			"mirrored byte-identically to " +
			"pkg/tbtc/signer/testdata/coordinator_seed_vectors.json (Rust).",
	}

	for _, input := range inputs {
		var digest [attempt.MessageDigestLength]byte
		copy(digest[:], input.digest)
		vector := coordinatorSeedVector{
			Name:              input.name,
			KeyGroup:          input.keyGroup,
			SessionID:         input.sessionID,
			MessageDigestHex:  hex.EncodeToString(input.digest),
			IncludedMembers:   input.members,
			AttemptNumber:     input.attempt,
			WireAttemptNumber: input.attempt + 1,
		}
		folded, coordinator := deriveCoordinatorSeedVectorOutputs(t, vector)
		vector.ExpectedShuffleSeedInt64 = fmt.Sprintf("%d", folded)
		vector.ExpectedCoordinator = uint16(coordinator)
		file.Vectors = append(file.Vectors, vector)
	}

	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("encode vector file: %v", err)
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("create testdata dir: %v", err)
	}
	if err := os.WriteFile(coordinatorSeedVectorsPath, encoded, 0o644); err != nil {
		t.Fatalf("write vector file: %v", err)
	}
	t.Logf("regenerated %s with %d vectors", coordinatorSeedVectorsPath, len(file.Vectors))
}
