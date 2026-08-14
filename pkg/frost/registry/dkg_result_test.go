package registry

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-core/pkg/frost"
)

const (
	v4DigestFixtureTestPath = "testdata/v4_digest_fixture.json"
	v4DigestFixtureRepoPath = "pkg/frost/registry/testdata/v4_digest_fixture.json"
)

type v4DigestFixture struct {
	Name                      string   `json:"name"`
	Version                   string   `json:"version"`
	ChainID                   string   `json:"chainID"`
	Bridge                    string   `json:"bridge"`
	Registry                  string   `json:"registry"`
	Seed                      string   `json:"seed"`
	XOnlyOutputKey            string   `json:"xOnlyOutputKey"`
	Members                   []uint32 `json:"members"`
	MisbehavedMembersIndices  []uint8  `json:"misbehavedMembersIndices"`
	FullMembersHash           string   `json:"fullMembersHash"`
	ActiveMembersHash         string   `json:"activeMembersHash"`
	Digest                    string   `json:"digest"`
	EthereumSignedMessageHash string   `json:"ethereumSignedMessageHash"`
	Generator                 struct {
		Source  string `json:"source"`
		Command string `json:"command"`
	} `json:"generator"`
	DriftCheck struct {
		TbtcPath     string `json:"tbtc_path"`
		KeepCorePath string `json:"keep_core_path"`
		Rule         string `json:"rule"`
	} `json:"drift_check"`
}

func TestResultDigestMatchesCrossRepoFixture(t *testing.T) {
	fixture := loadV4DigestFixture(t)

	chainID := mustBigInt(t, fixture.ChainID)
	seed := mustBigInt(t, fixture.Seed)
	digest, err := ResultDigest(
		chainID,
		common.HexToAddress(fixture.Bridge),
		common.HexToAddress(fixture.Registry),
		seed,
		mustOutputKey(t, fixture.XOnlyOutputKey),
		FullMembers(fixture.Members),
		MisbehavedMemberIndices(fixture.MisbehavedMembersIndices),
	)
	if err != nil {
		t.Fatalf("unexpected digest error: [%v]", err)
	}

	// Fixture generated with the TS reference shape:
	//   keccak256(defaultAbiCoder.encode(
	//     ["string","uint256","address","address","uint256","bytes32","bytes32","bytes32"],
	//     ["tbtc-frost-dkg-result-v1", chainID, bridge, registry, seed,
	//      xOnlyOutputKey, keccak256(abi.encode(uint32[] members)),
	//      keccak256(abi.encode(uint8[] misbehavedMembersIndices))]))
	expectedDigest := mustBytes32(t, fixture.Digest)

	if digest != expectedDigest {
		t.Fatalf(
			"unexpected digest\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedDigest,
			digest,
		)
	}
}

func TestResultDigestRejectsZeroBindingAddresses(t *testing.T) {
	fixture := loadV4DigestFixture(t)
	chainID := mustBigInt(t, fixture.ChainID)
	seed := mustBigInt(t, fixture.Seed)
	outputKey := mustOutputKey(t, fixture.XOnlyOutputKey)

	_, err := ResultDigest(
		chainID,
		common.Address{},
		common.HexToAddress(fixture.Registry),
		seed,
		outputKey,
		FullMembers(fixture.Members),
		MisbehavedMemberIndices(fixture.MisbehavedMembersIndices),
	)
	if err == nil || !strings.Contains(err.Error(), "bridge address is zero") {
		t.Fatalf("expected zero bridge rejection, got [%v]", err)
	}

	_, err = ResultDigest(
		chainID,
		common.HexToAddress(fixture.Bridge),
		common.Address{},
		seed,
		outputKey,
		FullMembers(fixture.Members),
		MisbehavedMemberIndices(fixture.MisbehavedMembersIndices),
	)
	if err == nil || !strings.Contains(err.Error(), "registry address is zero") {
		t.Fatalf("expected zero registry rejection, got [%v]", err)
	}
}

func TestMembersHashesKeepFullAndActiveSetsDistinct(t *testing.T) {
	fixture := loadV4DigestFixture(t)
	fullMembers := FullMembers(fixture.Members)
	misbehaved := MisbehavedMemberIndices(fixture.MisbehavedMembersIndices)

	activeMembers, err := ActiveMembersFromMisbehaved(fullMembers, misbehaved)
	if err != nil {
		t.Fatalf("unexpected active members error: [%v]", err)
	}

	expectedActiveMembers := ActiveMembers{101, 303, 404}
	if len(activeMembers) != len(expectedActiveMembers) {
		t.Fatalf(
			"unexpected active members length\nexpected: [%d]\nactual:   [%d]",
			len(expectedActiveMembers),
			len(activeMembers),
		)
	}
	for i := range expectedActiveMembers {
		if activeMembers[i] != expectedActiveMembers[i] {
			t.Fatalf(
				"unexpected active member at index [%d]\nexpected: [%d]\nactual:   [%d]",
				i,
				expectedActiveMembers[i],
				activeMembers[i],
			)
		}
	}

	fullHash, err := FullMembersHash(fullMembers)
	if err != nil {
		t.Fatalf("unexpected full members hash error: [%v]", err)
	}

	activeHash, err := ActiveMembersHash(activeMembers)
	if err != nil {
		t.Fatalf("unexpected active members hash error: [%v]", err)
	}

	expectedFullHash := mustBytes32(t, fixture.FullMembersHash)
	expectedActiveHash := mustBytes32(t, fixture.ActiveMembersHash)

	if fullHash != expectedFullHash {
		t.Fatalf(
			"unexpected full members hash\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedFullHash,
			fullHash,
		)
	}
	if activeHash != expectedActiveHash {
		t.Fatalf(
			"unexpected active members hash\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedActiveHash,
			activeHash,
		)
	}
	if fullHash == activeHash {
		t.Fatal("expected full and active members hashes to differ")
	}
}

func TestAssembleResultUsesFilteredMembersHash(t *testing.T) {
	fixture := loadV4DigestFixture(t)

	result, err := AssembleResult(
		1,
		mustOutputKey(t, fixture.XOnlyOutputKey),
		FullMembers(fixture.Members),
		MisbehavedMemberIndices(fixture.MisbehavedMembersIndices),
		[]byte{0x01, 0x02},
		[]uint64{1, 3, 4},
	)
	if err != nil {
		t.Fatalf("unexpected assembly error: [%v]", err)
	}

	expectedMembersHash := mustBytes32(t, fixture.ActiveMembersHash)

	if result.MembersHash != expectedMembersHash {
		t.Fatalf(
			"unexpected result membersHash\nexpected: [0x%x]\nactual:   [0x%x]",
			expectedMembersHash,
			result.MembersHash,
		)
	}
}

func TestEthereumSignedMessageHash(t *testing.T) {
	fixture := loadV4DigestFixture(t)

	hash := EthereumSignedMessageHash(mustBytes32(t, fixture.Digest))
	expected := mustBytes32(t, fixture.EthereumSignedMessageHash)

	if hash != expected {
		t.Fatalf(
			"unexpected EIP-191 hash\nexpected: [0x%x]\nactual:   [0x%x]",
			expected,
			hash,
		)
	}
}

func TestV4DigestFixtureMetadata(t *testing.T) {
	fixture := loadV4DigestFixture(t)

	if fixture.Name != "frost-dkg-result-digest-vectors" {
		t.Errorf("unexpected fixture name: [%s]", fixture.Name)
	}
	if fixture.Version != "v1" {
		t.Errorf("unexpected fixture version: [%s]", fixture.Version)
	}
	if fixture.Generator.Source != "tlabs-xyz/tbtc/contracts/tbtc-v2/test/integration/utils/frost-wallet-registry.ts:computeFrostResultDigest" {
		t.Errorf("unexpected generator source: [%s]", fixture.Generator.Source)
	}
	if fixture.Generator.Command == "" {
		t.Error("generator command must be documented")
	}
	if fixture.DriftCheck.TbtcPath != "docs/test-vectors/frost-dkg-result-digest-v1.json" {
		t.Errorf("unexpected tbtc drift-check path: [%s]", fixture.DriftCheck.TbtcPath)
	}
	if fixture.DriftCheck.KeepCorePath != v4DigestFixtureRepoPath {
		t.Errorf(
			"unexpected keep-core drift-check path\nexpected: [%s]\nactual:   [%s]",
			v4DigestFixtureRepoPath,
			fixture.DriftCheck.KeepCorePath,
		)
	}
	if fixture.DriftCheck.Rule == "" {
		t.Error("drift-check rule must be documented")
	}
}

func TestV4DigestFixtureFileShouldExistAtMirrorPath(t *testing.T) {
	fixture := loadV4DigestFixture(t)

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: cannot locate test source file")
	}

	repoRoot := filepath.Clean(
		filepath.Join(filepath.Dir(thisFile), "..", "..", ".."),
	)
	abs := filepath.Join(repoRoot, fixture.DriftCheck.KeepCorePath)
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf(
			"fixture self-declares it lives at [%s] resolved to [%s] but the file is not there: [%v]",
			fixture.DriftCheck.KeepCorePath,
			abs,
			err,
		)
	}
}

func TestActiveMembersFromMisbehavedRejectsInvalidIndices(t *testing.T) {
	testCases := map[string]MisbehavedMemberIndices{
		"zero":      {0},
		"too large": {4},
		"duplicate": {2, 2},
		"unsorted":  {3, 1},
	}

	for name, misbehaved := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := ActiveMembersFromMisbehaved(
				FullMembers{101, 202, 303},
				misbehaved,
			)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func loadV4DigestFixture(t *testing.T) *v4DigestFixture {
	t.Helper()

	data, err := os.ReadFile(v4DigestFixtureTestPath)
	if err != nil {
		t.Fatalf("cannot read fixture: [%v]", err)
	}

	var fixture v4DigestFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("cannot unmarshal fixture: [%v]", err)
	}

	return &fixture
}

func mustBigInt(t *testing.T, value string) *big.Int {
	t.Helper()

	result, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("cannot parse big int: [%s]", value)
	}

	return result
}

func mustOutputKey(t *testing.T, hexString string) frost.OutputKey {
	t.Helper()

	var outputKey frost.OutputKey
	copy(outputKey[:], mustBytes(t, hexString, frost.OutputKeySize))
	return outputKey
}

func mustBytes32(t *testing.T, hexString string) [32]byte {
	t.Helper()

	var result [32]byte
	copy(result[:], mustBytes(t, hexString, 32))
	return result
}

func mustBytes(t *testing.T, hexString string, expectedLength int) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(strings.TrimPrefix(hexString, "0x"))
	if err != nil {
		t.Fatalf("cannot decode hex string: [%v]", err)
	}

	if len(decoded) != expectedLength {
		t.Fatalf(
			"unexpected decoded length\nexpected: [%d]\nactual:   [%d]",
			expectedLength,
			len(decoded),
		)
	}

	return decoded
}
