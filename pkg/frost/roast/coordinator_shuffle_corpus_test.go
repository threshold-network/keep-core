package roast

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// coordinatorShuffleCorpusPath is the cross-language differential
// corpus for the legacy Go math/rand coordinator shuffle. Where
// testdata/coordinator_seed_vectors.json pins the full RFC-21 Annex A
// derivation on a handful of hand-picked vectors, this corpus pins
// the shuffle itself -- SelectCoordinator semantics over explicit
// int64 seeds -- across hundreds of generated cases and the integer
// boundary values where a port is most likely to diverge (negative
// seeds, i64 extremes, wrapping seed+attempt composition, unsorted
// inputs, 1..255-member sets).
//
// The Rust signer carries a byte-identical copy at
// pkg/tbtc/signer/testdata/coordinator_shuffle_corpus.json consumed
// by go_math_rand.rs tests; either side drifting fails its own suite.
// Regenerate here with ROAST_SHUFFLE_CORPUS_REGEN=1 and re-copy.
const coordinatorShuffleCorpusPath = "testdata/coordinator_shuffle_corpus.json"

type coordinatorShuffleCorpusFile struct {
	Description string                   `json:"description"`
	Cases       []coordinatorShuffleCase `json:"cases"`
}

type coordinatorShuffleCase struct {
	Name string `json:"name"`
	// SeedInt64 is the legacy shuffle seed as a decimal string so
	// JSON number precision cannot corrupt it.
	SeedInt64 string `json:"seedInt64"`
	// AttemptNumber is the 0-based attempt composed into the
	// rand.Source seed as seed + int64(attemptNumber) (two's-
	// complement wrapping).
	AttemptNumber uint32 `json:"attemptNumber"`
	// Members is the included set in the order passed to the
	// shuffle. Deliberately not always sorted: both implementations
	// must sort internally, and unsorted cases pin that.
	Members             []uint16 `json:"members"`
	ExpectedCoordinator uint16   `json:"expectedCoordinator"`
}

func loadCoordinatorShuffleCorpus(t *testing.T) coordinatorShuffleCorpusFile {
	t.Helper()
	raw, err := os.ReadFile(coordinatorShuffleCorpusPath)
	if err != nil {
		t.Fatalf(
			"cannot read %s (regenerate with ROAST_SHUFFLE_CORPUS_REGEN=1): %v",
			coordinatorShuffleCorpusPath,
			err,
		)
	}
	var file coordinatorShuffleCorpusFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("cannot parse %s: %v", coordinatorShuffleCorpusPath, err)
	}
	if len(file.Cases) == 0 {
		t.Fatalf("%s contains no cases", coordinatorShuffleCorpusPath)
	}
	return file
}

func runCoordinatorShuffleCase(
	t *testing.T,
	shuffleCase coordinatorShuffleCase,
) group.MemberIndex {
	t.Helper()
	seed, err := strconv.ParseInt(shuffleCase.SeedInt64, 10, 64)
	if err != nil {
		t.Fatalf(
			"case %q: seedInt64 %q is not a valid int64: %v",
			shuffleCase.Name, shuffleCase.SeedInt64, err,
		)
	}
	members := make([]group.MemberIndex, 0, len(shuffleCase.Members))
	for _, m := range shuffleCase.Members {
		if m == 0 || m > math.MaxUint8 {
			t.Fatalf(
				"case %q: member %d outside group.MemberIndex range",
				shuffleCase.Name, m,
			)
		}
		members = append(members, group.MemberIndex(m))
	}
	coordinator, err := SelectCoordinator(
		members,
		seed,
		uint(shuffleCase.AttemptNumber),
	)
	if err != nil {
		t.Fatalf("case %q: SelectCoordinator: %v", shuffleCase.Name, err)
	}
	return coordinator
}

// TestCoordinatorShuffle_DifferentialCorpus replays the generated
// corpus against SelectCoordinator. The Rust go_math_rand port runs
// the identical corpus, so any semantic drift in either shuffle --
// source seeding, Fisher-Yates order, int31n bounds, sign handling,
// wrapping composition, internal sorting -- fails the drifting side's
// own unit suite instead of fracturing coordinator agreement in a
// mixed deployment.
func TestCoordinatorShuffle_DifferentialCorpus(t *testing.T) {
	file := loadCoordinatorShuffleCorpus(t)

	for _, shuffleCase := range file.Cases {
		coordinator := runCoordinatorShuffleCase(t, shuffleCase)
		if coordinator != group.MemberIndex(shuffleCase.ExpectedCoordinator) {
			t.Fatalf(
				"case %q: coordinator mismatch: derived %d, corpus pins %d",
				shuffleCase.Name, coordinator, shuffleCase.ExpectedCoordinator,
			)
		}
	}
}

// TestRegenerateCoordinatorShuffleCorpus rewrites the corpus from the
// deterministic case matrix below using the current Go
// implementation. Guarded behind an env flag so it never rewrites
// during normal CI:
//
//	ROAST_SHUFFLE_CORPUS_REGEN=1 go test ./pkg/frost/roast -run TestRegenerateCoordinatorShuffleCorpus
//
// After regenerating, copy the file byte-identically to
// pkg/tbtc/signer/testdata/coordinator_shuffle_corpus.json on the
// signer branch.
//
// Regenerating is a protocol-change event, not a refresh: the corpus
// pins the legacy math/rand shuffle semantics shared with the Rust
// port, so a run that produces different bytes means the shuffle
// changed and deployed engines would disagree on coordinator
// rotation. Both language suites passing after a dual regen is NOT
// evidence of compatibility with already-deployed engines.
func TestRegenerateCoordinatorShuffleCorpus(t *testing.T) {
	if os.Getenv("ROAST_SHUFFLE_CORPUS_REGEN") != "1" {
		t.Skip("set ROAST_SHUFFLE_CORPUS_REGEN=1 to regenerate the corpus file")
	}

	cases := make([]coordinatorShuffleCase, 0, 600)

	// Boundary block: integer extremes, sign boundaries, the
	// wrapping seed+attempt composition, and the historical
	// cross-language pin seed from PR #4026.
	boundarySeeds := []int64{
		0,
		1,
		-1,
		math.MaxInt64,
		math.MinInt64,
		math.MaxInt64 - 3,
		math.MinInt64 + 3,
		6879463052285329321,
		-6879463052285329321,
	}
	boundaryAttempts := []uint32{0, 1, 7, math.MaxUint32}
	boundarySets := [][]uint16{
		{1},
		{1, 2},
		{2, 1},          // unsorted: pins internal sorting
		{5, 4, 3, 2, 1}, // reverse order
		{1, 2, 3, 4, 5},
		{7, 2, 11, 9}, // sparse, unsorted
	}
	for _, seed := range boundarySeeds {
		for _, attemptNumber := range boundaryAttempts {
			for setIndex, members := range boundarySets {
				cases = append(cases, coordinatorShuffleCase{
					Name: fmt.Sprintf(
						"boundary-seed-%d-attempt-%d-set-%d",
						seed, attemptNumber, setIndex,
					),
					SeedInt64:     strconv.FormatInt(seed, 10),
					AttemptNumber: attemptNumber,
					Members:       members,
				})
			}
		}
	}

	// Generated block: deterministic pseudo-random sweep over set
	// sizes 1..255 (every member index value reachable), seeds across
	// the full int64 range, attempts across realistic and large
	// values. The generator RNG seed is fixed; it only chooses the
	// case inputs and has no parity significance itself.
	generatorRng := rand.New(rand.NewSource(0x5EED_C0DE))
	memberPool := make([]uint16, 255)
	for i := range memberPool {
		memberPool[i] = uint16(i + 1)
	}
	for caseIndex := 0; caseIndex < 384; caseIndex++ {
		// Bias towards small sets (production-realistic) while still
		// sweeping up to the full 255.
		var setSize int
		switch caseIndex % 4 {
		case 0:
			setSize = 1 + generatorRng.Intn(5)
		case 1:
			setSize = 6 + generatorRng.Intn(46)
		case 2:
			setSize = 51 + generatorRng.Intn(50)
		default:
			setSize = 101 + generatorRng.Intn(155)
		}

		generatorRng.Shuffle(len(memberPool), func(i, j int) {
			memberPool[i], memberPool[j] = memberPool[j], memberPool[i]
		})
		members := make([]uint16, setSize)
		copy(members, memberPool[:setSize])

		seed := int64(generatorRng.Uint64())
		var attemptNumber uint32
		switch caseIndex % 3 {
		case 0:
			attemptNumber = uint32(generatorRng.Intn(8))
		case 1:
			attemptNumber = uint32(generatorRng.Intn(1 << 16))
		default:
			attemptNumber = generatorRng.Uint32()
		}

		cases = append(cases, coordinatorShuffleCase{
			Name:          fmt.Sprintf("generated-%03d-size-%d", caseIndex, setSize),
			SeedInt64:     strconv.FormatInt(seed, 10),
			AttemptNumber: attemptNumber,
			Members:       members,
		})
	}

	for caseIndex := range cases {
		coordinator := runCoordinatorShuffleCase(t, cases[caseIndex])
		cases[caseIndex].ExpectedCoordinator = uint16(coordinator)
	}

	file := coordinatorShuffleCorpusFile{
		Description: "Cross-language differential corpus for the legacy Go math/rand " +
			"coordinator shuffle (SelectCoordinator / go_math_rand.rs " +
			"select_coordinator_identifier): source seed = seedInt64 + " +
			"int64(attemptNumber) with two's-complement wrapping; members sorted " +
			"ascending internally before the Fisher-Yates shuffle; first element " +
			"after shuffling is the coordinator. Canonical copy: " +
			"pkg/frost/roast/testdata/coordinator_shuffle_corpus.json (Go); " +
			"mirrored byte-identically to " +
			"pkg/tbtc/signer/testdata/coordinator_shuffle_corpus.json (Rust). " +
			"Regenerate with ROAST_SHUFFLE_CORPUS_REGEN=1.",
		Cases: cases,
	}

	encoded, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("encode corpus file: %v", err)
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("create testdata dir: %v", err)
	}
	if err := os.WriteFile(coordinatorShuffleCorpusPath, encoded, 0o644); err != nil {
		t.Fatalf("write corpus file: %v", err)
	}
	t.Logf("regenerated %s with %d cases", coordinatorShuffleCorpusPath, len(cases))
}
