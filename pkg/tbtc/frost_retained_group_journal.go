package tbtc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"golang.org/x/sys/unix"
)

const (
	frostRetainedGroupJournalMetadataSchema = "tbtc-frost-retained-group-journal-metadata/v1"
	frostRetainedGroupJournalBatchSchema    = "tbtc-frost-retained-group-journal-batch/v1"
	frostRetainedGroupJournalStateSchema    = "tbtc-frost-retained-group-journal-state/v1"
	frostRetainedGroupJournalSnapshotSchema = "tbtc-frost-retained-group-journal-snapshot/v1"
	frostRetainedGroupJournalLockFile       = ".lock"
	frostRetainedGroupJournalMetadataFile   = "metadata.json"
	frostRetainedGroupJournalStateFile      = "state.json"
	frostRetainedGroupJournalBatchPrefix    = "batch-"
	frostRetainedGroupJournalFileSuffix     = ".json"
	frostRetainedGroupJournalTempSuffix     = ".tmp"
	frostRetainedGroupJournalMaximumFile    = 8 * 1024 * 1024
	frostRetainedGroupCanonicalDirectory    = "canonical"
	frostRetainedGroupQuarantineDirectory   = "quarantine"

	frostRetainedGroupQuarantineMetadataSchema = "tbtc-frost-retained-group-quarantine-metadata/v1"
	frostRetainedGroupQuarantineBatchSchema    = "tbtc-frost-retained-group-quarantine-batch/v1"
	frostRetainedGroupQuarantineStateSchema    = "tbtc-frost-retained-group-quarantine-state/v1"

	frostRetainedGroupInventoryEntriesDomain = "tbtc-p2tr-frost-wallet-group-inventory-entries-v1\x00"
	frostRetainedGroupInventoryLeafDomain    = "tbtc-p2tr-frost-wallet-group-inventory-leaf-v1\x00"
	frostRetainedGroupInventoryNodeDomain    = "tbtc-p2tr-frost-wallet-group-inventory-node-v1\x00"
	frostRetainedGroupInventoryRootDomain    = "tbtc-p2tr-frost-wallet-group-inventory-root-v1\x00"
	frostRetainedGroupBatchDomain            = "tbtc-frost-retained-group-journal-batch-v1\x00"
	frostRetainedGroupQuarantineBatchDomain  = "tbtc-frost-retained-group-quarantine-batch-v1\x00"
	frostRetainedGroupQuarantineDomain       = "tbtc-frost-retained-group-quarantine-v1\x00"
)

// FrostRetainedGroupEventPoint identifies one canonical Ethereum log. The
// transaction identity and ordering indexes prove Registry closure follows
// the matching Bridge terminal transition in the same transaction.
type FrostRetainedGroupEventPoint struct {
	BlockNumber      uint64   `json:"blockNumber"`
	BlockHash        [32]byte `json:"blockHash"`
	TransactionHash  [32]byte `json:"transactionHash"`
	TransactionIndex uint32   `json:"transactionIndex"`
	LogIndex         uint32   `json:"logIndex"`
}

func (frgep FrostRetainedGroupEventPoint) valid() bool {
	return frgep.BlockNumber > 0 && frgep.BlockHash != [32]byte{} &&
		frgep.TransactionHash != [32]byte{}
}

func compareFrostRetainedGroupEventPoints(
	left FrostRetainedGroupEventPoint,
	right FrostRetainedGroupEventPoint,
) int {
	if left.BlockNumber < right.BlockNumber {
		return -1
	}
	if left.BlockNumber > right.BlockNumber {
		return 1
	}
	if left.TransactionIndex < right.TransactionIndex {
		return -1
	}
	if left.TransactionIndex > right.TransactionIndex {
		return 1
	}
	if left.LogIndex < right.LogIndex {
		return -1
	}
	if left.LogIndex > right.LogIndex {
		return 1
	}
	return bytes.Compare(left.TransactionHash[:], right.TransactionHash[:])
}

type FrostRetainedGroupMutationKind string

const (
	FrostRetainedGroupAdmissionMutation        FrostRetainedGroupMutationKind = "admission"
	FrostRetainedGroupMovingFundsMutation      FrostRetainedGroupMutationKind = "moving-funds"
	FrostRetainedGroupClosingMutation          FrostRetainedGroupMutationKind = "closing"
	FrostRetainedGroupClosedMutation           FrostRetainedGroupMutationKind = "closed"
	FrostRetainedGroupTerminatedMutation       FrostRetainedGroupMutationKind = "terminated"
	FrostRetainedGroupRegistryClosureMutation  FrostRetainedGroupMutationKind = "registry-closure"
	FrostRetainedGroupQuarantineMutation       FrostRetainedGroupMutationKind = "quarantine"
	FrostRetainedGroupQuarantineLiftMutation   FrostRetainedGroupMutationKind = "quarantine-lift"
	FrostRetainedGroupRecoveryRequiredMutation FrostRetainedGroupMutationKind = "recovery-required"
)

type FrostRetainedGroupLifecycle string

const (
	FrostRetainedGroupLive        FrostRetainedGroupLifecycle = "Live"
	FrostRetainedGroupMovingFunds FrostRetainedGroupLifecycle = "MovingFunds"
	FrostRetainedGroupClosing     FrostRetainedGroupLifecycle = "Closing"
	FrostRetainedGroupClosed      FrostRetainedGroupLifecycle = "Closed"
	FrostRetainedGroupTerminated  FrostRetainedGroupLifecycle = "Terminated"
)

func (frgl FrostRetainedGroupLifecycle) terminal() bool {
	return frgl == FrostRetainedGroupClosed || frgl == FrostRetainedGroupTerminated
}

// FrostRetainedGroupMutation is one complete source-authenticated semantic
// history item. Admission carries exact ordered DKG operator IDs. A lift is
// accepted only with a nonzero authentication commitment for its exact ID.
type FrostRetainedGroupMutation struct {
	Point                   FrostRetainedGroupEventPoint   `json:"point"`
	Kind                    FrostRetainedGroupMutationKind `json:"kind"`
	WalletID                [32]byte                       `json:"walletID"`
	WalletPublicKeyHash     [20]byte                       `json:"walletPublicKeyHash"`
	OperatorIDs             []uint32                       `json:"operatorIDs,omitempty"`
	RetainedGroupHash       [32]byte                       `json:"retainedGroupHash,omitempty"`
	CreationPoint           FrostRetainedGroupEventPoint   `json:"creationPoint,omitempty"`
	BridgeRegistrationPoint FrostRetainedGroupEventPoint   `json:"bridgeRegistrationPoint,omitempty"`
	QuarantineID            [32]byte                       `json:"quarantineID,omitempty"`
	EvidenceHash            [32]byte                       `json:"evidenceHash,omitempty"`
	AuthenticationHash      [32]byte                       `json:"authenticationHash,omitempty"`
	Reason                  string                         `json:"reason,omitempty"`
}

type FrostRetainedGroupHistoryIdentity struct {
	TrustDomainID       string
	EndpointFingerprint [32]byte
	OperatorFingerprint [32]byte
}

type FrostRetainedGroupHistory struct {
	From              FrostPreSignFinality
	To                FrostPreSignFinality
	Mutations         []FrostRetainedGroupMutation
	Complete          bool
	EmptyAtFrom       bool
	DescriptorSetHash [32]byte
}

// FrostRetainedGroupHistorySource is independent of the primary deployment
// verifier. It authenticates canonicality, completeness, ordering, DKG
// admissions, and operator-ID resolution at the exact requested point.
type FrostRetainedGroupHistorySource interface {
	Identity(context.Context) (FrostRetainedGroupHistoryIdentity, error)
	FinalizedHead(context.Context) (FrostPreSignFinality, error)
	VerifyPoint(context.Context, FrostPreSignFinality) error
	ReadCompleteHistory(
		context.Context,
		FrostPreSignFinality,
		FrostPreSignFinality,
	) (*FrostRetainedGroupHistory, error)
	ResolveOperatorID(
		context.Context,
		chain.Address,
		FrostPreSignFinality,
	) (chain.OperatorID, error)
}

type FrostRetainedGroupCanonicalJournalManifest struct {
	StoreID            string
	StoreFingerprint   [32]byte
	ClusterFingerprint [32]byte
	// Checkpoint is the exclusive, canonical empty-inventory baseline. It must
	// precede every FROST creation or lifecycle event returned by the source.
	Checkpoint                FrostPreSignFinality
	DescriptorSetHash         [32]byte
	SourceTrustDomainID       string
	SourceEndpointFingerprint [32]byte
	SourceOperatorFingerprint [32]byte
	MinimumGeneration         uint64
}

type FrostRetainedGroupQuarantineJournalManifest struct {
	ProtocolID         [32]byte
	StoreID            string
	StoreFingerprint   [32]byte
	ClusterFingerprint [32]byte
	MinimumGeneration  uint64
}

type frostRetainedGroupJournalMetadata struct {
	Schema                    string               `json:"schema"`
	ManifestHash              [32]byte             `json:"manifestHash"`
	StoreID                   string               `json:"storeID"`
	StoreFingerprint          [32]byte             `json:"storeFingerprint"`
	ClusterFingerprint        [32]byte             `json:"clusterFingerprint"`
	Checkpoint                FrostPreSignFinality `json:"checkpoint"`
	DescriptorSetHash         [32]byte             `json:"descriptorSetHash"`
	SourceTrustDomainID       string               `json:"sourceTrustDomainID"`
	SourceEndpointFingerprint [32]byte             `json:"sourceEndpointFingerprint"`
	SourceOperatorFingerprint [32]byte             `json:"sourceOperatorFingerprint"`
}

type frostRetainedGroupWalletState struct {
	WalletID                [32]byte                     `json:"walletID"`
	WalletPublicKeyHash     [20]byte                     `json:"walletPublicKeyHash"`
	OperatorIDs             []uint32                     `json:"operatorIDs"`
	RetainedGroupHash       [32]byte                     `json:"retainedGroupHash"`
	Lifecycle               FrostRetainedGroupLifecycle  `json:"lifecycle"`
	CreationPoint           FrostRetainedGroupEventPoint `json:"creationPoint"`
	BridgeRegistrationPoint FrostRetainedGroupEventPoint `json:"bridgeRegistrationPoint"`
	LifecyclePoint          FrostRetainedGroupEventPoint `json:"lifecyclePoint"`
	LastBridgePoint         FrostRetainedGroupEventPoint `json:"lastBridgePoint"`
	RegistryClosurePoint    FrostRetainedGroupEventPoint `json:"registryClosurePoint"`
	RegistryClosed          bool                         `json:"registryClosed"`
}

type frostRetainedGroupQuarantineState struct {
	QuarantineID     [32]byte                     `json:"quarantineID"`
	WalletID         [32]byte                     `json:"walletID"`
	EvidenceHash     [32]byte                     `json:"evidenceHash"`
	Reason           string                       `json:"reason"`
	RecoveryRequired bool                         `json:"recoveryRequired"`
	RaisedAt         FrostRetainedGroupEventPoint `json:"raisedAt"`
}

type frostRetainedGroupJournalState struct {
	Schema             string                          `json:"schema"`
	BatchSequence      uint64                          `json:"batchSequence"`
	CurrentPoint       FrostPreSignFinality            `json:"currentPoint"`
	SnapshotGeneration uint64                          `json:"snapshotGeneration"`
	BatchRoot          [32]byte                        `json:"batchRoot"`
	InventoryRoot      [32]byte                        `json:"inventoryRoot"`
	Wallets            []frostRetainedGroupWalletState `json:"wallets"`
}

type frostRetainedGroupJournalBatch struct {
	Schema         string                       `json:"schema"`
	Sequence       uint64                       `json:"sequence"`
	From           FrostPreSignFinality         `json:"from"`
	To             FrostPreSignFinality         `json:"to"`
	PriorBatchRoot [32]byte                     `json:"priorBatchRoot"`
	Mutations      []FrostRetainedGroupMutation `json:"mutations"`
	Checksum       [32]byte                     `json:"checksum"`
}

type frostRetainedGroupQuarantineMetadata struct {
	Schema             string               `json:"schema"`
	ManifestHash       [32]byte             `json:"manifestHash"`
	ProtocolID         [32]byte             `json:"protocolID"`
	StoreID            string               `json:"storeID"`
	StoreFingerprint   [32]byte             `json:"storeFingerprint"`
	ClusterFingerprint [32]byte             `json:"clusterFingerprint"`
	Checkpoint         FrostPreSignFinality `json:"checkpoint"`
}

type frostRetainedGroupQuarantineJournalState struct {
	Schema        string                              `json:"schema"`
	BatchSequence uint64                              `json:"batchSequence"`
	CurrentPoint  FrostPreSignFinality                `json:"currentPoint"`
	Generation    uint64                              `json:"generation"`
	BatchRoot     [32]byte                            `json:"batchRoot"`
	Root          [32]byte                            `json:"root"`
	Quarantines   []frostRetainedGroupQuarantineState `json:"quarantines"`
}

type frostRetainedGroupQuarantineJournalBatch struct {
	Schema         string                       `json:"schema"`
	Sequence       uint64                       `json:"sequence"`
	From           FrostPreSignFinality         `json:"from"`
	To             FrostPreSignFinality         `json:"to"`
	PriorBatchRoot [32]byte                     `json:"priorBatchRoot"`
	Mutations      []FrostRetainedGroupMutation `json:"mutations"`
	Checksum       [32]byte                     `json:"checksum"`
}

type frostRetainedGroupEnvelope struct {
	Payload  json.RawMessage `json:"payload"`
	Checksum [32]byte        `json:"checksum"`
}

type frostRetainedGroupJournalSnapshot struct {
	Schema                       string
	StoreID                      string
	StoreFingerprint             [32]byte
	ClusterFingerprint           [32]byte
	CurrentPoint                 FrostPreSignFinality
	SnapshotGeneration           uint64
	BatchRoot                    [32]byte
	InventoryRoot                [32]byte
	WalletCount                  uint64
	MinimumActualGroupSize       uint64
	MaximumActualGroupSize       uint64
	QuarantineProtocolID         [32]byte
	QuarantineStoreID            string
	QuarantineStoreFingerprint   [32]byte
	QuarantineClusterFingerprint [32]byte
	QuarantineMinimumGeneration  uint64
	QuarantineGeneration         uint64
	QuarantineRoot               [32]byte
	QuarantineCount              uint64
	LocalSessionCount            uint64
	Complete                     bool
}

type frostRetainedGroupJournal struct {
	mutex                       sync.Mutex
	rootDirectory               string
	directory                   string
	quarantineDirectory         string
	metadata                    frostRetainedGroupJournalMetadata
	quarantineMetadata          frostRetainedGroupQuarantineMetadata
	minimumGeneration           uint64
	quarantineMinimumGeneration uint64
	source                      FrostRetainedGroupHistorySource
	walletRegistry              *walletRegistry
	operatorAddress             chain.Address
	lockFile                    *os.File
	quarantineLockFile          *os.File
	state                       frostRetainedGroupJournalState
	quarantineState             frostRetainedGroupQuarantineJournalState
	mutations                   []FrostRetainedGroupMutation
	quarantineMutations         []FrostRetainedGroupMutation
	persistFailureHook          func(string) error
	closed                      bool
}

func newFrostRetainedGroupJournal(
	directory string,
	manifestHash [32]byte,
	manifest FrostRetainedGroupCanonicalJournalManifest,
	quarantineManifest FrostRetainedGroupQuarantineJournalManifest,
	source FrostRetainedGroupHistorySource,
	walletRegistry *walletRegistry,
	operatorAddress chain.Address,
) (*frostRetainedGroupJournal, error) {
	if strings.TrimSpace(directory) == "" || manifestHash == [32]byte{} ||
		strings.TrimSpace(manifest.StoreID) == "" ||
		manifest.StoreFingerprint == [32]byte{} ||
		manifest.ClusterFingerprint == [32]byte{} ||
		manifest.Checkpoint.BlockNumber == 0 || manifest.Checkpoint.BlockHash == [32]byte{} ||
		manifest.DescriptorSetHash == [32]byte{} ||
		strings.TrimSpace(manifest.SourceTrustDomainID) == "" ||
		manifest.SourceEndpointFingerprint == [32]byte{} ||
		manifest.SourceOperatorFingerprint == [32]byte{} ||
		quarantineManifest.ProtocolID == [32]byte{} ||
		strings.TrimSpace(quarantineManifest.StoreID) == "" ||
		quarantineManifest.StoreFingerprint == [32]byte{} ||
		quarantineManifest.ClusterFingerprint == [32]byte{} ||
		manifest.StoreID == quarantineManifest.StoreID ||
		manifest.StoreFingerprint == quarantineManifest.StoreFingerprint ||
		manifest.ClusterFingerprint == quarantineManifest.ClusterFingerprint ||
		source == nil || walletRegistry == nil ||
		strings.TrimSpace(string(operatorAddress)) == "" {
		return nil, fmt.Errorf("FROST retained-group journal dependencies are incomplete")
	}
	cleanRootDirectory, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return nil, fmt.Errorf("cannot resolve FROST retained-group journal directory: [%w]", err)
	}
	if err := os.MkdirAll(cleanRootDirectory, 0700); err != nil {
		return nil, fmt.Errorf("cannot create FROST retained-group journal: [%w]", err)
	}
	if err := validateSecureBitcoinBroadcastDirectory(cleanRootDirectory); err != nil {
		return nil, fmt.Errorf("invalid FROST retained-group journal directory: [%w]", err)
	}
	if err := validateFrostRetainedGroupJournalRoot(cleanRootDirectory); err != nil {
		return nil, err
	}
	canonicalDirectory := filepath.Join(cleanRootDirectory, frostRetainedGroupCanonicalDirectory)
	quarantineDirectory := filepath.Join(cleanRootDirectory, frostRetainedGroupQuarantineDirectory)
	for _, child := range []string{canonicalDirectory, quarantineDirectory} {
		if err := os.MkdirAll(child, 0700); err != nil {
			return nil, fmt.Errorf("cannot create FROST retained-group journal store: [%w]", err)
		}
		if err := validateSecureBitcoinBroadcastDirectory(child); err != nil {
			return nil, fmt.Errorf("invalid FROST retained-group journal store: [%w]", err)
		}
	}
	if err := syncDirectory(cleanRootDirectory); err != nil {
		return nil, fmt.Errorf("cannot sync FROST retained-group journal directory: [%w]", err)
	}
	lockFile, err := acquireFrostRetainedGroupJournalLock(canonicalDirectory)
	if err != nil {
		return nil, err
	}
	quarantineLockFile, err := acquireFrostRetainedGroupJournalLock(quarantineDirectory)
	if err != nil {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		_ = lockFile.Close()
		return nil, err
	}
	journal := &frostRetainedGroupJournal{
		rootDirectory:       cleanRootDirectory,
		directory:           canonicalDirectory,
		quarantineDirectory: quarantineDirectory,
		metadata: frostRetainedGroupJournalMetadata{
			Schema:                    frostRetainedGroupJournalMetadataSchema,
			ManifestHash:              manifestHash,
			StoreID:                   manifest.StoreID,
			StoreFingerprint:          manifest.StoreFingerprint,
			ClusterFingerprint:        manifest.ClusterFingerprint,
			Checkpoint:                manifest.Checkpoint,
			DescriptorSetHash:         manifest.DescriptorSetHash,
			SourceTrustDomainID:       manifest.SourceTrustDomainID,
			SourceEndpointFingerprint: manifest.SourceEndpointFingerprint,
			SourceOperatorFingerprint: manifest.SourceOperatorFingerprint,
		},
		quarantineMetadata: frostRetainedGroupQuarantineMetadata{
			Schema:             frostRetainedGroupQuarantineMetadataSchema,
			ManifestHash:       manifestHash,
			ProtocolID:         quarantineManifest.ProtocolID,
			StoreID:            quarantineManifest.StoreID,
			StoreFingerprint:   quarantineManifest.StoreFingerprint,
			ClusterFingerprint: quarantineManifest.ClusterFingerprint,
			Checkpoint:         manifest.Checkpoint,
		},
		minimumGeneration:           manifest.MinimumGeneration,
		quarantineMinimumGeneration: quarantineManifest.MinimumGeneration,
		source:                      source,
		walletRegistry:              walletRegistry,
		operatorAddress:             operatorAddress,
		lockFile:                    lockFile,
		quarantineLockFile:          quarantineLockFile,
	}
	if err := journal.initialize(); err != nil {
		_ = journal.close()
		return nil, err
	}
	return journal, nil
}

func validateFrostRetainedGroupJournalRoot(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("cannot read FROST retained-group journal root: [%w]", err)
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 ||
			(entry.Name() != frostRetainedGroupCanonicalDirectory &&
				entry.Name() != frostRetainedGroupQuarantineDirectory) ||
			!entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST retained-group journal root: [%s]", entry.Name())
		}
	}
	return nil
}

func acquireFrostRetainedGroupJournalLock(directory string) (*os.File, error) {
	lockPath := filepath.Join(directory, frostRetainedGroupJournalLockFile)
	file, err := openSecureBitcoinBroadcastFile(lockPath, unix.O_CREAT|unix.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot open FROST retained-group journal lock: [%w]", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("FROST retained-group journal is already owned by another process")
	}
	if err := file.Truncate(0); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	if err := syncDirectory(directory); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (frgj *frostRetainedGroupJournal) close() error {
	frgj.mutex.Lock()
	defer frgj.mutex.Unlock()
	if frgj.closed {
		return nil
	}
	frgj.closed = true
	var result error
	for _, lock := range []*os.File{frgj.quarantineLockFile, frgj.lockFile} {
		if lock == nil {
			continue
		}
		if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil && result == nil {
			result = err
		}
		if err := lock.Close(); err != nil && result == nil {
			result = err
		}
	}
	frgj.lockFile = nil
	frgj.quarantineLockFile = nil
	return result
}

func (frgj *frostRetainedGroupJournal) initialize() error {
	if err := frgj.verifyHistorySourceIdentity(context.Background()); err != nil {
		return err
	}

	entries, err := os.ReadDir(frgj.directory)
	if err != nil {
		return fmt.Errorf("cannot read FROST retained-group journal: [%w]", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	metadataExists := false
	stateExists := false
	batchNames := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == frostRetainedGroupJournalLockFile {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST retained-group journal: [%s]", name)
		}
		switch {
		case name == frostRetainedGroupJournalMetadataFile:
			metadataExists = true
		case name == frostRetainedGroupJournalStateFile:
			stateExists = true
		case strings.HasPrefix(name, frostRetainedGroupJournalBatchPrefix) &&
			strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix):
			batchNames = append(batchNames, name)
		case strings.HasSuffix(name, frostRetainedGroupJournalTempSuffix):
			return fmt.Errorf("interrupted FROST retained-group journal temp is present: [%s]", name)
		default:
			return fmt.Errorf("unexpected file in FROST retained-group journal: [%s]", name)
		}
	}

	if metadataExists {
		stored := frostRetainedGroupJournalMetadata{}
		if err := frgj.readEnvelope(frostRetainedGroupJournalMetadataFile, &stored); err != nil {
			return fmt.Errorf("cannot read FROST retained-group journal metadata: [%w]", err)
		}
		if stored != frgj.metadata {
			return fmt.Errorf("FROST retained-group journal metadata differs from signed manifest")
		}
	} else {
		if stateExists || len(batchNames) != 0 {
			return fmt.Errorf("FROST retained-group journal has state without immutable metadata")
		}
		if err := frgj.persistEnvelope(frostRetainedGroupJournalMetadataFile, &frgj.metadata, false); err != nil {
			return fmt.Errorf("cannot persist FROST retained-group journal metadata: [%w]", err)
		}
	}

	initial := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: frgj.metadata.Checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	initial.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(initial)
	if err != nil {
		return err
	}

	stored := initial
	if stateExists {
		if err := frgj.readEnvelope(frostRetainedGroupJournalStateFile, &stored); err != nil {
			return fmt.Errorf("cannot read FROST retained-group journal state: [%w]", err)
		}
		if stored.Schema != frostRetainedGroupJournalStateSchema {
			return fmt.Errorf("unsupported FROST retained-group journal state schema")
		}
	} else if len(batchNames) != 0 {
		return fmt.Errorf("FROST retained-group journal has batches without state checkpoint")
	}

	rebuilt := initial
	rebuiltMutations := make([]FrostRetainedGroupMutation, 0)
	matchedStored := stored.BatchSequence == 0
	if matchedStored {
		if err := equalFrostRetainedGroupStates(stored, rebuilt); err != nil {
			return err
		}
	}
	for index, name := range batchNames {
		expectedSequence := uint64(index + 1)
		if name != frostRetainedGroupBatchFileName(expectedSequence) {
			return fmt.Errorf("FROST retained-group journal batch sequence has a gap at [%d]", expectedSequence)
		}
		batch := frostRetainedGroupJournalBatch{}
		if err := frgj.readEnvelope(name, &batch); err != nil {
			return fmt.Errorf("cannot read FROST retained-group journal batch [%d]: [%w]", expectedSequence, err)
		}
		if err := validateFrostRetainedGroupBatch(batch, rebuilt); err != nil {
			return fmt.Errorf("invalid FROST retained-group journal batch [%d]: [%w]", expectedSequence, err)
		}
		if err := applyFrostRetainedGroupMutations(&rebuilt, batch.Mutations); err != nil {
			return fmt.Errorf("cannot replay FROST retained-group batch [%d]: [%w]", expectedSequence, err)
		}
		rebuilt.BatchSequence = batch.Sequence
		rebuilt.CurrentPoint = batch.To
		rebuilt.BatchRoot = frostRetainedGroupBatchRoot(batch.PriorBatchRoot, batch.Checksum)
		rebuilt.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(rebuilt)
		if err != nil {
			return err
		}
		rebuiltMutations = append(rebuiltMutations, cloneFrostRetainedGroupMutations(batch.Mutations)...)
		if rebuilt.BatchSequence == stored.BatchSequence {
			if err := equalFrostRetainedGroupStates(stored, rebuilt); err != nil {
				return err
			}
			matchedStored = true
		}
	}
	if stored.BatchSequence > uint64(len(batchNames)) || !matchedStored {
		return fmt.Errorf("FROST retained-group state checkpoint has no exact batch prefix")
	}
	frgj.state = rebuilt
	frgj.mutations = rebuiltMutations
	if !stateExists || stored.BatchSequence != rebuilt.BatchSequence {
		if err := frgj.persistEnvelope(frostRetainedGroupJournalStateFile, &rebuilt, true); err != nil {
			return fmt.Errorf("cannot integrate orphan FROST retained-group batch: [%w]", err)
		}
	}
	return frgj.initializeQuarantine()
}

func (frgj *frostRetainedGroupJournal) verifyHistorySourceIdentity(
	ctx context.Context,
) error {
	identity, err := frgj.source.Identity(ctx)
	if err != nil {
		return fmt.Errorf("cannot authenticate FROST retained-group history source: [%w]", err)
	}
	if identity.TrustDomainID != frgj.metadata.SourceTrustDomainID ||
		identity.EndpointFingerprint != frgj.metadata.SourceEndpointFingerprint ||
		identity.OperatorFingerprint != frgj.metadata.SourceOperatorFingerprint {
		return fmt.Errorf("FROST retained-group history source identity differs from signed manifest")
	}
	return nil
}

func equalFrostRetainedGroupStates(
	stored frostRetainedGroupJournalState,
	rebuilt frostRetainedGroupJournalState,
) error {
	storedBytes, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	rebuiltBytes, err := json.Marshal(rebuilt)
	if err != nil {
		return err
	}
	if !bytes.Equal(storedBytes, rebuiltBytes) {
		return fmt.Errorf("FROST retained-group state differs from exact journal prefix")
	}
	return nil
}

func (frgj *frostRetainedGroupJournal) initializeQuarantine() error {
	entries, err := os.ReadDir(frgj.quarantineDirectory)
	if err != nil {
		return fmt.Errorf("cannot read FROST retained-group quarantine journal: [%w]", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	metadataExists := false
	stateExists := false
	batchNames := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == frostRetainedGroupJournalLockFile {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST retained-group quarantine journal: [%s]", name)
		}
		switch {
		case name == frostRetainedGroupJournalMetadataFile:
			metadataExists = true
		case name == frostRetainedGroupJournalStateFile:
			stateExists = true
		case strings.HasPrefix(name, frostRetainedGroupJournalBatchPrefix) &&
			strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix):
			batchNames = append(batchNames, name)
		case strings.HasSuffix(name, frostRetainedGroupJournalTempSuffix):
			return fmt.Errorf("interrupted FROST retained-group quarantine temp is present: [%s]", name)
		default:
			return fmt.Errorf("unexpected file in FROST retained-group quarantine journal: [%s]", name)
		}
	}

	if metadataExists {
		stored := frostRetainedGroupQuarantineMetadata{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalMetadataFile,
			&stored,
		); err != nil {
			return fmt.Errorf("cannot read FROST retained-group quarantine metadata: [%w]", err)
		}
		if stored != frgj.quarantineMetadata {
			return fmt.Errorf("FROST retained-group quarantine metadata differs from signed manifest")
		}
	} else {
		if stateExists || len(batchNames) != 0 {
			return fmt.Errorf("FROST retained-group quarantine journal has state without immutable metadata")
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalMetadataFile,
			&frgj.quarantineMetadata,
			false,
		); err != nil {
			return fmt.Errorf("cannot persist FROST retained-group quarantine metadata: [%w]", err)
		}
	}

	initial := frostRetainedGroupQuarantineJournalState{
		Schema:       frostRetainedGroupQuarantineStateSchema,
		CurrentPoint: frgj.quarantineMetadata.Checkpoint,
		Root:         sha256.Sum256([]byte(frostRetainedGroupQuarantineDomain)),
		Quarantines:  []frostRetainedGroupQuarantineState{},
	}
	stored := initial
	if stateExists {
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&stored,
		); err != nil {
			return fmt.Errorf("cannot read FROST retained-group quarantine state: [%w]", err)
		}
		if stored.Schema != frostRetainedGroupQuarantineStateSchema {
			return fmt.Errorf("unsupported FROST retained-group quarantine state schema")
		}
	} else if len(batchNames) != 0 {
		return fmt.Errorf("FROST retained-group quarantine journal has batches without state checkpoint")
	}

	rebuilt := initial
	rebuiltMutations := make([]FrostRetainedGroupMutation, 0)
	matchedStored := stored.BatchSequence == 0
	if matchedStored {
		if err := equalFrostRetainedGroupQuarantineStates(stored, rebuilt); err != nil {
			return err
		}
	}
	for index, name := range batchNames {
		expectedSequence := uint64(index + 1)
		if name != frostRetainedGroupBatchFileName(expectedSequence) {
			return fmt.Errorf("FROST retained-group quarantine batch sequence has a gap at [%d]", expectedSequence)
		}
		batch := frostRetainedGroupQuarantineJournalBatch{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			name,
			&batch,
		); err != nil {
			return fmt.Errorf("cannot read FROST retained-group quarantine batch [%d]: [%w]", expectedSequence, err)
		}
		if err := validateFrostRetainedGroupQuarantineBatch(batch, rebuilt); err != nil {
			return fmt.Errorf("invalid FROST retained-group quarantine batch [%d]: [%w]", expectedSequence, err)
		}
		if err := applyFrostRetainedGroupQuarantineMutations(&rebuilt, batch.Mutations); err != nil {
			return fmt.Errorf("cannot replay FROST retained-group quarantine batch [%d]: [%w]", expectedSequence, err)
		}
		rebuilt.BatchSequence = batch.Sequence
		rebuilt.CurrentPoint = batch.To
		rebuilt.BatchRoot = frostRetainedGroupQuarantineBatchRoot(batch.PriorBatchRoot, batch.Checksum)
		rebuiltMutations = append(rebuiltMutations, cloneFrostRetainedGroupMutations(batch.Mutations)...)
		if rebuilt.BatchSequence == stored.BatchSequence {
			if err := equalFrostRetainedGroupQuarantineStates(stored, rebuilt); err != nil {
				return err
			}
			matchedStored = true
		}
	}
	if stored.BatchSequence > uint64(len(batchNames)) || !matchedStored {
		return fmt.Errorf("FROST retained-group quarantine state checkpoint has no exact batch prefix")
	}
	frgj.quarantineState = rebuilt
	frgj.quarantineMutations = rebuiltMutations
	if !stateExists || stored.BatchSequence != rebuilt.BatchSequence {
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&rebuilt,
			true,
		); err != nil {
			return fmt.Errorf("cannot integrate orphan FROST retained-group quarantine batch: [%w]", err)
		}
	}
	return nil
}

func equalFrostRetainedGroupQuarantineStates(
	stored frostRetainedGroupQuarantineJournalState,
	rebuilt frostRetainedGroupQuarantineJournalState,
) error {
	storedBytes, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	rebuiltBytes, err := json.Marshal(rebuilt)
	if err != nil {
		return err
	}
	if !bytes.Equal(storedBytes, rebuiltBytes) {
		return fmt.Errorf("FROST retained-group quarantine state differs from exact journal prefix")
	}
	return nil
}

func frostRetainedGroupBatchFileName(sequence uint64) string {
	return fmt.Sprintf("%s%020d%s", frostRetainedGroupJournalBatchPrefix, sequence, frostRetainedGroupJournalFileSuffix)
}

func validateFrostRetainedGroupBatch(
	batch frostRetainedGroupJournalBatch,
	prior frostRetainedGroupJournalState,
) error {
	if batch.Schema != frostRetainedGroupJournalBatchSchema ||
		batch.Sequence != prior.BatchSequence+1 || batch.From != prior.CurrentPoint ||
		batch.PriorBatchRoot != prior.BatchRoot || batch.To.BlockNumber < batch.From.BlockNumber ||
		batch.To.BlockHash == [32]byte{} || batch.Checksum == [32]byte{} {
		return fmt.Errorf("batch header is invalid")
	}
	declared := batch.Checksum
	batch.Checksum = [32]byte{}
	payload, err := frostRetainedGroupCanonicalValue(batch)
	if err != nil {
		return err
	}
	if sha256.Sum256(payload) != declared {
		return fmt.Errorf("batch checksum mismatch")
	}
	if err := validateFrostRetainedGroupBatchMutationBounds(batch.From, batch.To, batch.Mutations); err != nil {
		return err
	}
	for _, mutation := range batch.Mutations {
		if isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return fmt.Errorf("canonical batch contains a quarantine mutation")
		}
	}
	return nil
}

func validateFrostRetainedGroupQuarantineBatch(
	batch frostRetainedGroupQuarantineJournalBatch,
	prior frostRetainedGroupQuarantineJournalState,
) error {
	if batch.Schema != frostRetainedGroupQuarantineBatchSchema ||
		batch.Sequence != prior.BatchSequence+1 || batch.From != prior.CurrentPoint ||
		batch.PriorBatchRoot != prior.BatchRoot || batch.To.BlockNumber < batch.From.BlockNumber ||
		batch.To.BlockHash == [32]byte{} || batch.Checksum == [32]byte{} {
		return fmt.Errorf("quarantine batch header is invalid")
	}
	declared := batch.Checksum
	batch.Checksum = [32]byte{}
	payload, err := frostRetainedGroupCanonicalValue(batch)
	if err != nil {
		return err
	}
	if sha256.Sum256(payload) != declared {
		return fmt.Errorf("quarantine batch checksum mismatch")
	}
	if err := validateFrostRetainedGroupBatchMutationBounds(batch.From, batch.To, batch.Mutations); err != nil {
		return err
	}
	for _, mutation := range batch.Mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return fmt.Errorf("quarantine batch contains a canonical inventory mutation")
		}
	}
	return nil
}

func validateFrostRetainedGroupBatchMutationBounds(
	from FrostPreSignFinality,
	to FrostPreSignFinality,
	mutations []FrostRetainedGroupMutation,
) error {
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range mutations {
		if !mutation.Point.valid() || mutation.Point.BlockNumber <= from.BlockNumber ||
			mutation.Point.BlockNumber > to.BlockNumber ||
			(index > 0 && compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("batch mutations are outside cursor bounds or not strictly ordered")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("batch mutations disagree on canonical block hash")
		}
		previous = mutation.Point
	}
	return nil
}

func frostRetainedGroupBatchRoot(prior [32]byte, checksum [32]byte) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupBatchDomain))
	hasher.Write(prior[:])
	hasher.Write(checksum[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

func frostRetainedGroupQuarantineBatchRoot(
	prior [32]byte,
	checksum [32]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupQuarantineBatchDomain))
	hasher.Write(prior[:])
	hasher.Write(checksum[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

func (frgj *frostRetainedGroupJournal) readEnvelope(name string, target interface{}) error {
	return readFrostRetainedGroupEnvelopeAt(frgj.directory, name, target)
}

func readFrostRetainedGroupEnvelopeAt(
	directory string,
	name string,
	target interface{},
) error {
	if err := validateFrostRetainedGroupJournalFileName(name); err != nil {
		return err
	}
	file, err := openSecureBitcoinBroadcastFile(filepath.Join(directory, name), unix.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, frostRetainedGroupJournalMaximumFile+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > frostRetainedGroupJournalMaximumFile {
		return fmt.Errorf("journal file size is invalid")
	}
	envelope := frostRetainedGroupEnvelope{}
	if err := decodeStrictFrostActivationJSON(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Payload) == 0 || sha256.Sum256(envelope.Payload) != envelope.Checksum {
		return fmt.Errorf("journal file checksum mismatch")
	}
	return decodeStrictFrostActivationJSON(envelope.Payload, target)
}

func (frgj *frostRetainedGroupJournal) persistEnvelope(
	name string,
	payload interface{},
	replace bool,
) error {
	return persistFrostRetainedGroupEnvelopeAt(frgj.directory, name, payload, replace)
}

func persistFrostRetainedGroupEnvelopeAt(
	directory string,
	name string,
	payload interface{},
	replace bool,
) error {
	if err := validateFrostRetainedGroupJournalFileName(name); err != nil {
		return err
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	envelopeBytes, err := json.Marshal(frostRetainedGroupEnvelope{
		Payload:  payloadBytes,
		Checksum: sha256.Sum256(payloadBytes),
	})
	if err != nil {
		return err
	}

	directoryFile, err := openFrostRetainedGroupJournalDirectory(directory)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	directoryDescriptor := int(directoryFile.Fd())

	if exists, err := validateFrostRetainedGroupJournalFileAt(
		directoryDescriptor,
		name,
	); err != nil {
		return err
	} else if exists {
		if !replace {
			return fmt.Errorf("immutable journal file already exists: [%s]", name)
		}
	}

	temporary, temporaryName, err := createFrostRetainedGroupJournalTemporaryFileAt(
		directoryDescriptor,
		directory,
		name,
	)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		if remove {
			_ = unix.Unlinkat(directoryDescriptor, temporaryName, 0)
		}
	}()
	if _, err := temporary.Write(envelopeBytes); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if !replace {
		// Link is an atomic no-replace publication: unlike Rename it cannot
		// overwrite an immutable batch or metadata file that appeared after
		// the destination check above.
		if err := unix.Linkat(
			directoryDescriptor,
			temporaryName,
			directoryDescriptor,
			name,
			0,
		); err != nil {
			return fmt.Errorf("cannot publish immutable journal file [%s]: [%w]", name, err)
		}
		if err := unix.Unlinkat(directoryDescriptor, temporaryName, 0); err != nil {
			return err
		}
		remove = false
		return directoryFile.Sync()
	}
	if err := unix.Renameat(
		directoryDescriptor,
		temporaryName,
		directoryDescriptor,
		name,
	); err != nil {
		return err
	}
	remove = false
	return directoryFile.Sync()
}

func validateFrostRetainedGroupJournalFileName(name string) error {
	if !filepath.IsLocal(name) || filepath.Base(name) != name {
		return fmt.Errorf("FROST retained-group journal file name is not local: [%s]", name)
	}
	if name == frostRetainedGroupJournalMetadataFile ||
		name == frostRetainedGroupJournalStateFile {
		return nil
	}
	if !strings.HasPrefix(name, frostRetainedGroupJournalBatchPrefix) ||
		!strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix) {
		return fmt.Errorf("unsupported FROST retained-group journal file name: [%s]", name)
	}
	sequenceText := strings.TrimSuffix(
		strings.TrimPrefix(name, frostRetainedGroupJournalBatchPrefix),
		frostRetainedGroupJournalFileSuffix,
	)
	if len(sequenceText) != 20 {
		return fmt.Errorf("noncanonical FROST retained-group journal batch name: [%s]", name)
	}
	sequence, err := strconv.ParseUint(sequenceText, 10, 64)
	if err != nil || sequence == 0 ||
		frostRetainedGroupBatchFileName(sequence) != name {
		return fmt.Errorf("noncanonical FROST retained-group journal batch name: [%s]", name)
	}
	return nil
}

func openFrostRetainedGroupJournalDirectory(directory string) (*os.File, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, fmt.Errorf("FROST retained-group journal directory is not canonical")
	}
	fd, err := unix.Open(
		directory,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), directory)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("cannot wrap FROST retained-group journal directory descriptor")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0700 {
		_ = file.Close()
		return nil, fmt.Errorf("FROST retained-group journal directory is unsafe")
	}
	if err := validateBitcoinBroadcastOwner(info); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func validateFrostRetainedGroupJournalFileAt(
	directoryDescriptor int,
	name string,
) (bool, error) {
	var info unix.Stat_t
	err := unix.Fstatat(
		directoryDescriptor,
		name,
		&info,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if uint32(info.Mode)&unix.S_IFMT != unix.S_IFREG ||
		uint32(info.Mode)&0777 != 0600 {
		return false, fmt.Errorf("journal destination is unsafe: [%s]", name)
	}
	if info.Uid != uint32(os.Geteuid()) {
		return false, fmt.Errorf(
			"Bitcoin broadcast storage is owned by uid [%d], expected [%d]",
			info.Uid,
			os.Geteuid(),
		)
	}
	return true, nil
}

func createFrostRetainedGroupJournalTemporaryFileAt(
	directoryDescriptor int,
	directory string,
	finalName string,
) (*os.File, string, error) {
	const maximumAttempts = 128
	var entropy [16]byte
	for attempt := 0; attempt < maximumAttempts; attempt++ {
		if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return nil, "", fmt.Errorf("cannot generate journal temporary file name: [%w]", err)
		}
		name := finalName + "-" + hex.EncodeToString(entropy[:]) +
			frostRetainedGroupJournalTempSuffix
		fd, err := unix.Openat(
			directoryDescriptor,
			name,
			unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW,
			0600,
		)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		file := os.NewFile(uintptr(fd), filepath.Join(directory, name))
		if file == nil {
			_ = unix.Close(fd)
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", fmt.Errorf("cannot wrap journal temporary file descriptor")
		}
		if err := file.Chmod(0600); err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", err
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0600 {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", fmt.Errorf("journal temporary file is unsafe")
		}
		if err := validateBitcoinBroadcastOwner(info); err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", err
		}
		return file, name, nil
	}
	return nil, "", fmt.Errorf("cannot allocate a unique journal temporary file")
}

func applyFrostRetainedGroupMutations(
	state *frostRetainedGroupJournalState,
	mutations []FrostRetainedGroupMutation,
) error {
	if state == nil {
		return fmt.Errorf("FROST retained-group state is nil")
	}
	wallets := make(map[[32]byte]frostRetainedGroupWalletState, len(state.Wallets))
	for _, wallet := range state.Wallets {
		if wallet.WalletID == [32]byte{} || wallets[wallet.WalletID].WalletID != [32]byte{} {
			return fmt.Errorf("FROST retained-group wallet state is duplicate or invalid")
		}
		wallet.OperatorIDs = append([]uint32{}, wallet.OperatorIDs...)
		wallets[wallet.WalletID] = wallet
	}
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range mutations {
		if !mutation.Point.valid() || (index > 0 &&
			compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("FROST retained-group mutations are not strictly ordered")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("FROST retained-group mutations disagree on canonical block hash")
		}
		previous = mutation.Point
		wallet := wallets[mutation.WalletID]
		switch mutation.Kind {
		case FrostRetainedGroupAdmissionMutation:
			if mutation.WalletID == [32]byte{} || mutation.WalletPublicKeyHash == [20]byte{} ||
				len(mutation.OperatorIDs) < 51 || len(mutation.OperatorIDs) > 100 ||
				mutation.RetainedGroupHash == [32]byte{} ||
				!mutation.CreationPoint.valid() || !mutation.BridgeRegistrationPoint.valid() ||
				mutation.Point != mutation.BridgeRegistrationPoint ||
				!sameFrostRetainedGroupTransaction(mutation.CreationPoint, mutation.BridgeRegistrationPoint) ||
				compareFrostRetainedGroupEventPoints(mutation.CreationPoint, mutation.BridgeRegistrationPoint) >= 0 ||
				wallet.WalletID != [32]byte{} || mutation.QuarantineID != [32]byte{} {
				return fmt.Errorf("invalid or duplicate FROST retained-group admission")
			}
			for _, operatorID := range mutation.OperatorIDs {
				if operatorID == 0 {
					return fmt.Errorf("FROST retained-group admission contains zero operator ID")
				}
			}
			wallets[mutation.WalletID] = frostRetainedGroupWalletState{
				WalletID:                mutation.WalletID,
				WalletPublicKeyHash:     mutation.WalletPublicKeyHash,
				OperatorIDs:             append([]uint32{}, mutation.OperatorIDs...),
				RetainedGroupHash:       mutation.RetainedGroupHash,
				Lifecycle:               FrostRetainedGroupLive,
				CreationPoint:           mutation.CreationPoint,
				BridgeRegistrationPoint: mutation.BridgeRegistrationPoint,
				LifecyclePoint:          mutation.BridgeRegistrationPoint,
				LastBridgePoint:         mutation.Point,
			}
			state.SnapshotGeneration++
		case FrostRetainedGroupMovingFundsMutation,
			FrostRetainedGroupClosingMutation,
			FrostRetainedGroupClosedMutation,
			FrostRetainedGroupTerminatedMutation:
			if wallet.WalletID == [32]byte{} ||
				mutation.WalletPublicKeyHash != wallet.WalletPublicKeyHash ||
				len(mutation.OperatorIDs) != 0 || wallet.Lifecycle.terminal() {
				return fmt.Errorf("invalid FROST retained-group lifecycle mutation")
			}
			next := FrostRetainedGroupLifecycle("")
			switch mutation.Kind {
			case FrostRetainedGroupMovingFundsMutation:
				next = FrostRetainedGroupMovingFunds
			case FrostRetainedGroupClosingMutation:
				next = FrostRetainedGroupClosing
			case FrostRetainedGroupClosedMutation:
				next = FrostRetainedGroupClosed
			case FrostRetainedGroupTerminatedMutation:
				next = FrostRetainedGroupTerminated
			}
			if !validFrostRetainedGroupTransition(wallet.Lifecycle, next) {
				return fmt.Errorf("invalid FROST retained-group transition [%s -> %s]", wallet.Lifecycle, next)
			}
			wallet.Lifecycle = next
			wallet.LifecyclePoint = mutation.Point
			wallet.LastBridgePoint = mutation.Point
			wallets[mutation.WalletID] = wallet
			state.SnapshotGeneration++
		case FrostRetainedGroupRegistryClosureMutation:
			if wallet.WalletID == [32]byte{} || !wallet.Lifecycle.terminal() || wallet.RegistryClosed ||
				mutation.WalletPublicKeyHash != wallet.WalletPublicKeyHash ||
				mutation.Point.BlockNumber != wallet.LastBridgePoint.BlockNumber ||
				mutation.Point.BlockHash != wallet.LastBridgePoint.BlockHash ||
				mutation.Point.TransactionHash != wallet.LastBridgePoint.TransactionHash ||
				mutation.Point.TransactionIndex != wallet.LastBridgePoint.TransactionIndex ||
				mutation.Point.LogIndex <= wallet.LastBridgePoint.LogIndex {
				return fmt.Errorf("FROST Registry closure does not follow its Bridge terminal event")
			}
			wallet.RegistryClosed = true
			wallet.RegistryClosurePoint = mutation.Point
			wallets[mutation.WalletID] = wallet
			state.SnapshotGeneration++
		case FrostRetainedGroupQuarantineMutation,
			FrostRetainedGroupRecoveryRequiredMutation,
			FrostRetainedGroupQuarantineLiftMutation:
			return fmt.Errorf("quarantine mutation cannot enter canonical retained-group journal")
		default:
			return fmt.Errorf("unknown FROST retained-group mutation kind [%s]", mutation.Kind)
		}
	}
	state.Wallets = state.Wallets[:0]
	for _, wallet := range wallets {
		state.Wallets = append(state.Wallets, wallet)
	}
	sort.Slice(state.Wallets, func(i, j int) bool {
		return bytes.Compare(state.Wallets[i].WalletID[:], state.Wallets[j].WalletID[:]) < 0
	})
	return nil
}

func isFrostRetainedGroupQuarantineMutation(
	kind FrostRetainedGroupMutationKind,
) bool {
	return kind == FrostRetainedGroupQuarantineMutation ||
		kind == FrostRetainedGroupRecoveryRequiredMutation ||
		kind == FrostRetainedGroupQuarantineLiftMutation
}

func frostRetainedGroupQuarantineMutations(
	mutations []FrostRetainedGroupMutation,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, 0)
	for _, mutation := range mutations {
		if isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			result = append(result, mutation)
		}
	}
	return cloneFrostRetainedGroupMutations(result)
}

func frostRetainedGroupCanonicalMutations(
	mutations []FrostRetainedGroupMutation,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, 0)
	for _, mutation := range mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			result = append(result, mutation)
		}
	}
	return cloneFrostRetainedGroupMutations(result)
}

func applyFrostRetainedGroupQuarantineMutations(
	state *frostRetainedGroupQuarantineJournalState,
	mutations []FrostRetainedGroupMutation,
) error {
	if state == nil {
		return fmt.Errorf("FROST retained-group quarantine state is nil")
	}
	quarantines := make(
		map[[32]byte]frostRetainedGroupQuarantineState,
		len(state.Quarantines),
	)
	for _, quarantine := range state.Quarantines {
		if quarantine.QuarantineID == [32]byte{} ||
			quarantines[quarantine.QuarantineID].QuarantineID != [32]byte{} {
			return fmt.Errorf("FROST retained-group quarantine state is duplicate or invalid")
		}
		quarantines[quarantine.QuarantineID] = quarantine
	}
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return fmt.Errorf("canonical inventory mutation cannot enter quarantine journal")
		}
		if !mutation.Point.valid() || (index > 0 &&
			compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("FROST retained-group quarantine mutations are not strictly ordered")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("FROST retained-group quarantine mutations disagree on canonical block hash")
		}
		previous = mutation.Point
		switch mutation.Kind {
		case FrostRetainedGroupQuarantineMutation, FrostRetainedGroupRecoveryRequiredMutation:
			if mutation.QuarantineID == [32]byte{} || mutation.EvidenceHash == [32]byte{} ||
				strings.TrimSpace(mutation.Reason) == "" ||
				quarantines[mutation.QuarantineID].QuarantineID != [32]byte{} {
				return fmt.Errorf("invalid or duplicate FROST retained-group quarantine")
			}
			quarantines[mutation.QuarantineID] = frostRetainedGroupQuarantineState{
				QuarantineID:     mutation.QuarantineID,
				WalletID:         mutation.WalletID,
				EvidenceHash:     mutation.EvidenceHash,
				Reason:           mutation.Reason,
				RecoveryRequired: mutation.Kind == FrostRetainedGroupRecoveryRequiredMutation,
				RaisedAt:         mutation.Point,
			}
		case FrostRetainedGroupQuarantineLiftMutation:
			quarantine := quarantines[mutation.QuarantineID]
			if quarantine.QuarantineID == [32]byte{} || mutation.AuthenticationHash == [32]byte{} ||
				mutation.WalletID != quarantine.WalletID {
				return fmt.Errorf("unauthenticated or unknown FROST retained-group quarantine lift")
			}
			delete(quarantines, mutation.QuarantineID)
		}
		appendFrostRetainedGroupQuarantineRoot(state, mutation)
	}
	state.Quarantines = state.Quarantines[:0]
	for _, quarantine := range quarantines {
		state.Quarantines = append(state.Quarantines, quarantine)
	}
	sort.Slice(state.Quarantines, func(i, j int) bool {
		return bytes.Compare(state.Quarantines[i].QuarantineID[:], state.Quarantines[j].QuarantineID[:]) < 0
	})
	return nil
}

func validFrostRetainedGroupTransition(
	current FrostRetainedGroupLifecycle,
	next FrostRetainedGroupLifecycle,
) bool {
	switch current {
	case FrostRetainedGroupLive:
		return next == FrostRetainedGroupMovingFunds || next == FrostRetainedGroupClosing ||
			next == FrostRetainedGroupTerminated
	case FrostRetainedGroupMovingFunds:
		return next == FrostRetainedGroupClosing || next == FrostRetainedGroupTerminated
	case FrostRetainedGroupClosing:
		return next == FrostRetainedGroupClosed || next == FrostRetainedGroupTerminated
	default:
		return false
	}
}

func sameFrostRetainedGroupTransaction(
	left FrostRetainedGroupEventPoint,
	right FrostRetainedGroupEventPoint,
) bool {
	return left.BlockNumber == right.BlockNumber &&
		left.BlockHash == right.BlockHash &&
		left.TransactionHash == right.TransactionHash &&
		left.TransactionIndex == right.TransactionIndex
}

func appendFrostRetainedGroupQuarantineRoot(
	state *frostRetainedGroupQuarantineJournalState,
	mutation FrostRetainedGroupMutation,
) {
	payload, _ := frostRetainedGroupCanonicalValue(mutation)
	leaf := sha256.Sum256(payload)
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupQuarantineDomain))
	hasher.Write(state.Root[:])
	hasher.Write(leaf[:])
	copy(state.Root[:], hasher.Sum(nil))
	state.Generation++
}

func frostRetainedGroupInventoryRoot(
	state frostRetainedGroupJournalState,
) ([32]byte, uint64, uint64, uint64, error) {
	type inventoryEventPoint struct {
		BlockNumber      uint64 `json:"blockNumber"`
		BlockHash        string `json:"blockHash"`
		TransactionHash  string `json:"transactionHash"`
		TransactionIndex uint32 `json:"transactionIndex"`
		LogIndex         uint32 `json:"logIndex"`
	}
	type inventoryEntry struct {
		WalletID                string               `json:"walletID"`
		RetainedGroupHash       string               `json:"retainedGroupHash"`
		ActualGroupSize         uint64               `json:"actualGroupSize"`
		Lifecycle               string               `json:"lifecycle"`
		CreationPoint           inventoryEventPoint  `json:"creationPoint"`
		BridgeRegistrationPoint inventoryEventPoint  `json:"bridgeRegistrationPoint"`
		LifecyclePoint          inventoryEventPoint  `json:"lifecyclePoint"`
		RegistryClosurePoint    *inventoryEventPoint `json:"registryClosurePoint,omitempty"`
	}
	eventPoint := func(point FrostRetainedGroupEventPoint) inventoryEventPoint {
		return inventoryEventPoint{
			BlockNumber:      point.BlockNumber,
			BlockHash:        "0x" + hex.EncodeToString(point.BlockHash[:]),
			TransactionHash:  "0x" + hex.EncodeToString(point.TransactionHash[:]),
			TransactionIndex: point.TransactionIndex,
			LogIndex:         point.LogIndex,
		}
	}
	entries := make([]inventoryEntry, 0)
	minimumSize := uint64(0)
	maximumSize := uint64(0)
	for _, wallet := range state.Wallets {
		size := uint64(len(wallet.OperatorIDs))
		if wallet.WalletID == [32]byte{} || size < 51 || size > 100 ||
			wallet.RetainedGroupHash == [32]byte{} ||
			!wallet.CreationPoint.valid() || !wallet.BridgeRegistrationPoint.valid() ||
			!wallet.LifecyclePoint.valid() ||
			!sameFrostRetainedGroupTransaction(wallet.CreationPoint, wallet.BridgeRegistrationPoint) ||
			compareFrostRetainedGroupEventPoints(wallet.CreationPoint, wallet.BridgeRegistrationPoint) >= 0 ||
			compareFrostRetainedGroupEventPoints(wallet.BridgeRegistrationPoint, wallet.LifecyclePoint) > 0 ||
			(wallet.BridgeRegistrationPoint.BlockNumber == wallet.LifecyclePoint.BlockNumber &&
				wallet.BridgeRegistrationPoint.BlockHash != wallet.LifecyclePoint.BlockHash) ||
			wallet.LifecyclePoint.BlockNumber > state.CurrentPoint.BlockNumber ||
			(wallet.LifecyclePoint.BlockNumber == state.CurrentPoint.BlockNumber &&
				wallet.LifecyclePoint.BlockHash != state.CurrentPoint.BlockHash) ||
			wallet.Lifecycle.terminal() != wallet.RegistryClosed ||
			(wallet.RegistryClosed &&
				(!sameFrostRetainedGroupTransaction(wallet.LifecyclePoint, wallet.RegistryClosurePoint) ||
					compareFrostRetainedGroupEventPoints(wallet.LifecyclePoint, wallet.RegistryClosurePoint) >= 0)) {
			return [32]byte{}, 0, 0, 0, fmt.Errorf("FROST retained-group inventory has invalid group size")
		}
		if minimumSize == 0 || size < minimumSize {
			minimumSize = size
		}
		if size > maximumSize {
			maximumSize = size
		}
		lifecycle := ""
		switch wallet.Lifecycle {
		case FrostRetainedGroupLive:
			lifecycle = "live"
		case FrostRetainedGroupMovingFunds:
			lifecycle = "moving-funds"
		case FrostRetainedGroupClosing:
			lifecycle = "closing"
		case FrostRetainedGroupClosed:
			lifecycle = "closed"
		case FrostRetainedGroupTerminated:
			lifecycle = "terminated"
		default:
			return [32]byte{}, 0, 0, 0, fmt.Errorf("FROST retained-group inventory has unknown lifecycle")
		}
		entry := inventoryEntry{
			WalletID:                "0x" + hex.EncodeToString(wallet.WalletID[:]),
			RetainedGroupHash:       "0x" + hex.EncodeToString(wallet.RetainedGroupHash[:]),
			ActualGroupSize:         size,
			Lifecycle:               lifecycle,
			CreationPoint:           eventPoint(wallet.CreationPoint),
			BridgeRegistrationPoint: eventPoint(wallet.BridgeRegistrationPoint),
			LifecyclePoint:          eventPoint(wallet.LifecyclePoint),
		}
		if wallet.RegistryClosed {
			registryPoint := eventPoint(wallet.RegistryClosurePoint)
			entry.RegistryClosurePoint = &registryPoint
		}
		entries = append(entries, entry)
	}
	// ASCII ordering of lowercase fixed-width wallet IDs is identical to byte
	// ordering and is stated explicitly here to make the wire commitment clear.
	sort.Slice(entries, func(i, j int) bool { return entries[i].WalletID < entries[j].WalletID })
	for index := 1; index < len(entries); index++ {
		if entries[index-1].WalletID == entries[index].WalletID {
			return [32]byte{}, 0, 0, 0, fmt.Errorf("FROST retained-group inventory has ambiguous wallet membership")
		}
	}
	accumulator := sha256.Sum256([]byte(frostRetainedGroupInventoryEntriesDomain))
	for _, entry := range entries {
		canonical, err := frostRetainedGroupCanonicalValue(entry)
		if err != nil {
			return [32]byte{}, 0, 0, 0, err
		}
		leafHasher := sha256.New()
		leafHasher.Write([]byte(frostRetainedGroupInventoryLeafDomain))
		leafHasher.Write(canonical)
		leaf := leafHasher.Sum(nil)
		nodeHasher := sha256.New()
		nodeHasher.Write([]byte(frostRetainedGroupInventoryNodeDomain))
		nodeHasher.Write(accumulator[:])
		nodeHasher.Write(leaf)
		copy(accumulator[:], nodeHasher.Sum(nil))
	}
	rootMetadata := struct {
		Point struct {
			BlockHash   string `json:"blockHash"`
			BlockNumber uint64 `json:"blockNumber"`
		} `json:"point"`
		SnapshotGeneration uint64 `json:"snapshotGeneration"`
		WalletCount        uint64 `json:"walletCount"`
	}{SnapshotGeneration: state.SnapshotGeneration, WalletCount: uint64(len(entries))}
	rootMetadata.Point.BlockNumber = state.CurrentPoint.BlockNumber
	rootMetadata.Point.BlockHash = "0x" + hex.EncodeToString(state.CurrentPoint.BlockHash[:])
	canonicalMetadata, err := frostRetainedGroupCanonicalValue(rootMetadata)
	if err != nil {
		return [32]byte{}, 0, 0, 0, err
	}
	rootHasher := sha256.New()
	rootHasher.Write([]byte(frostRetainedGroupInventoryRootDomain))
	rootHasher.Write(canonicalMetadata)
	rootHasher.Write(accumulator[:])
	var root [32]byte
	copy(root[:], rootHasher.Sum(nil))
	return root, uint64(len(entries)), minimumSize, maximumSize, nil
}

func frostRetainedGroupCanonicalValue(value interface{}) ([]byte, error) {
	return canonicalFrostActivationValue(value)
}

func cloneFrostRetainedGroupMutations(
	mutations []FrostRetainedGroupMutation,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, len(mutations))
	copy(result, mutations)
	for index := range result {
		result[index].OperatorIDs = append([]uint32{}, mutations[index].OperatorIDs...)
	}
	return result
}

func (frgj *frostRetainedGroupJournal) reconcile(
	ctx context.Context,
	target FrostPreSignFinality,
) (*frostRetainedGroupJournalSnapshot, error) {
	if ctx == nil {
		return nil, fmt.Errorf("FROST retained-group reconciliation context is nil")
	}
	frgj.mutex.Lock()
	defer frgj.mutex.Unlock()
	if frgj.closed {
		return nil, fmt.Errorf("FROST retained-group journal is closed")
	}
	if target.BlockNumber == 0 || target.BlockHash == [32]byte{} ||
		target.BlockNumber < frgj.metadata.Checkpoint.BlockNumber ||
		target.BlockNumber < frgj.state.CurrentPoint.BlockNumber ||
		target.BlockNumber < frgj.quarantineState.CurrentPoint.BlockNumber {
		return nil, fmt.Errorf("FROST retained-group target is invalid or retrograde")
	}
	if err := frgj.verifyHistorySourceIdentity(ctx); err != nil {
		return nil, err
	}
	beforeHead, err := frgj.source.FinalizedHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read independent finalized head before journal replay: [%w]", err)
	}
	if beforeHead.BlockNumber < target.BlockNumber || beforeHead.BlockHash == [32]byte{} {
		return nil, fmt.Errorf("FROST retained-group target is above independent finalized head")
	}
	if beforeHead.BlockNumber == target.BlockNumber && beforeHead.BlockHash != target.BlockHash {
		return nil, fmt.Errorf("independent finalized head disagrees with challenged target")
	}
	if err := frgj.source.VerifyPoint(ctx, beforeHead); err != nil {
		return nil, fmt.Errorf("cannot verify independent finalized head before journal replay: [%w]", err)
	}
	for name, point := range map[string]FrostPreSignFinality{
		"signed checkpoint":         frgj.metadata.Checkpoint,
		"durable canonical cursor":  frgj.state.CurrentPoint,
		"durable quarantine cursor": frgj.quarantineState.CurrentPoint,
		"challenged target":         target,
	} {
		if err := frgj.source.VerifyPoint(ctx, point); err != nil {
			return nil, fmt.Errorf("cannot verify FROST retained-group %s: [%w]", name, err)
		}
	}
	history, err := frgj.source.ReadCompleteHistory(ctx, frgj.metadata.Checkpoint, target)
	if err != nil {
		return nil, fmt.Errorf("cannot reconstruct complete FROST retained-group history: [%w]", err)
	}
	if history == nil || !history.Complete || !history.EmptyAtFrom ||
		history.From != frgj.metadata.Checkpoint ||
		history.To != target || history.DescriptorSetHash != frgj.metadata.DescriptorSetHash {
		return nil, fmt.Errorf("FROST retained-group history receipt is incomplete or differently bound")
	}
	if err := validateCompleteFrostRetainedGroupHistory(history); err != nil {
		return nil, err
	}
	canonicalInventoryMutations := frostRetainedGroupCanonicalMutations(history.Mutations)
	if len(frgj.mutations) > len(canonicalInventoryMutations) {
		return nil, fmt.Errorf("durable FROST retained-group journal has events absent from canonical history")
	}
	for index := range frgj.mutations {
		durable, err := frostRetainedGroupCanonicalValue(frgj.mutations[index])
		if err != nil {
			return nil, err
		}
		canonical, err := frostRetainedGroupCanonicalValue(canonicalInventoryMutations[index])
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(durable, canonical) {
			return nil, fmt.Errorf("canonical FROST retained-group history rewrote, omitted, or reordered event [%d]", index)
		}
	}
	canonicalQuarantineMutations := frostRetainedGroupQuarantineMutations(history.Mutations)
	if len(frgj.quarantineMutations) > len(canonicalQuarantineMutations) {
		return nil, fmt.Errorf("durable FROST quarantine journal has events absent from canonical history")
	}
	for index := range frgj.quarantineMutations {
		durable, err := frostRetainedGroupCanonicalValue(frgj.quarantineMutations[index])
		if err != nil {
			return nil, err
		}
		canonical, err := frostRetainedGroupCanonicalValue(canonicalQuarantineMutations[index])
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(durable, canonical) {
			return nil, fmt.Errorf("canonical FROST retained-group history rewrote, omitted, or reordered quarantine event [%d]", index)
		}
	}
	suffix := cloneFrostRetainedGroupMutations(
		canonicalInventoryMutations[len(frgj.mutations):],
	)
	for _, mutation := range suffix {
		if mutation.Point.BlockNumber <= frgj.state.CurrentPoint.BlockNumber ||
			mutation.Point.BlockNumber > target.BlockNumber {
			return nil, fmt.Errorf("canonical FROST retained-group suffix is outside durable cursor bounds")
		}
	}
	if target != frgj.state.CurrentPoint || len(suffix) != 0 {
		candidate := cloneFrostRetainedGroupState(frgj.state)
		if err := applyFrostRetainedGroupMutations(&candidate, suffix); err != nil {
			return nil, err
		}
		batch := frostRetainedGroupJournalBatch{
			Schema:         frostRetainedGroupJournalBatchSchema,
			Sequence:       frgj.state.BatchSequence + 1,
			From:           frgj.state.CurrentPoint,
			To:             target,
			PriorBatchRoot: frgj.state.BatchRoot,
			Mutations:      suffix,
		}
		checksumPayload, err := frostRetainedGroupCanonicalValue(batch)
		if err != nil {
			return nil, err
		}
		batch.Checksum = sha256.Sum256(checksumPayload)
		candidate.BatchSequence = batch.Sequence
		candidate.CurrentPoint = target
		candidate.BatchRoot = frostRetainedGroupBatchRoot(batch.PriorBatchRoot, batch.Checksum)
		candidate.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(candidate)
		if err != nil {
			return nil, err
		}
		if err := frgj.persistEnvelope(frostRetainedGroupBatchFileName(batch.Sequence), &batch, false); err != nil {
			return nil, fmt.Errorf("cannot append FROST retained-group journal batch: [%w]", err)
		}
		if frgj.persistFailureHook != nil {
			if err := frgj.persistFailureHook("after-batch-before-state"); err != nil {
				return nil, err
			}
		}
		if err := frgj.persistEnvelope(frostRetainedGroupJournalStateFile, &candidate, true); err != nil {
			return nil, fmt.Errorf("cannot checkpoint FROST retained-group journal state: [%w]", err)
		}
		frgj.state = candidate
		frgj.mutations = append(frgj.mutations, suffix...)
	}
	quarantineSuffix := cloneFrostRetainedGroupMutations(
		canonicalQuarantineMutations[len(frgj.quarantineMutations):],
	)
	for _, mutation := range quarantineSuffix {
		if mutation.Point.BlockNumber <= frgj.quarantineState.CurrentPoint.BlockNumber ||
			mutation.Point.BlockNumber > target.BlockNumber {
			return nil, fmt.Errorf("canonical FROST quarantine suffix is outside durable cursor bounds")
		}
	}
	if target != frgj.quarantineState.CurrentPoint || len(quarantineSuffix) != 0 {
		candidate := cloneFrostRetainedGroupQuarantineState(frgj.quarantineState)
		if err := applyFrostRetainedGroupQuarantineMutations(&candidate, quarantineSuffix); err != nil {
			return nil, err
		}
		batch := frostRetainedGroupQuarantineJournalBatch{
			Schema:         frostRetainedGroupQuarantineBatchSchema,
			Sequence:       frgj.quarantineState.BatchSequence + 1,
			From:           frgj.quarantineState.CurrentPoint,
			To:             target,
			PriorBatchRoot: frgj.quarantineState.BatchRoot,
			Mutations:      quarantineSuffix,
		}
		checksumPayload, err := frostRetainedGroupCanonicalValue(batch)
		if err != nil {
			return nil, err
		}
		batch.Checksum = sha256.Sum256(checksumPayload)
		candidate.BatchSequence = batch.Sequence
		candidate.CurrentPoint = target
		candidate.BatchRoot = frostRetainedGroupQuarantineBatchRoot(
			batch.PriorBatchRoot,
			batch.Checksum,
		)
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupBatchFileName(batch.Sequence),
			&batch,
			false,
		); err != nil {
			return nil, fmt.Errorf("cannot append FROST retained-group quarantine batch: [%w]", err)
		}
		if frgj.persistFailureHook != nil {
			if err := frgj.persistFailureHook("after-quarantine-batch-before-state"); err != nil {
				return nil, err
			}
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&candidate,
			true,
		); err != nil {
			return nil, fmt.Errorf("cannot checkpoint FROST retained-group quarantine state: [%w]", err)
		}
		frgj.quarantineState = candidate
		frgj.quarantineMutations = append(frgj.quarantineMutations, quarantineSuffix...)
	}
	afterHead, err := frgj.source.FinalizedHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read independent finalized head after journal replay: [%w]", err)
	}
	if afterHead.BlockNumber < beforeHead.BlockNumber ||
		afterHead.BlockNumber < target.BlockNumber || afterHead.BlockHash == [32]byte{} ||
		(beforeHead.BlockNumber == afterHead.BlockNumber && beforeHead.BlockHash != afterHead.BlockHash) {
		return nil, fmt.Errorf("independent finalized head changed inconsistently during journal replay")
	}
	if afterHead.BlockNumber == target.BlockNumber && afterHead.BlockHash != target.BlockHash {
		return nil, fmt.Errorf("independent finalized head disagrees with challenged target after replay")
	}
	if err := frgj.source.VerifyPoint(ctx, afterHead); err != nil {
		return nil, fmt.Errorf("cannot verify independent finalized head after journal replay: [%w]", err)
	}
	if err := frgj.source.VerifyPoint(ctx, target); err != nil {
		return nil, fmt.Errorf("challenged FROST retained-group point changed during replay: [%w]", err)
	}
	localSessionCount, err := frgj.reconcileLocalSessions(ctx, target)
	if err != nil {
		return nil, err
	}
	root, walletCount, minimumSize, maximumSize, err := frostRetainedGroupInventoryRoot(frgj.state)
	if err != nil {
		return nil, err
	}
	if root != frgj.state.InventoryRoot || frgj.state.SnapshotGeneration < frgj.minimumGeneration {
		return nil, fmt.Errorf("FROST retained-group journal generation or inventory root is not activation-ready")
	}
	if frgj.quarantineState.CurrentPoint != target ||
		frgj.quarantineState.Generation < frgj.quarantineMinimumGeneration ||
		frgj.quarantineState.Root == [32]byte{} {
		return nil, fmt.Errorf("independent FROST quarantine journal is not activation-ready")
	}
	return &frostRetainedGroupJournalSnapshot{
		Schema:                       frostRetainedGroupJournalSnapshotSchema,
		StoreID:                      frgj.metadata.StoreID,
		StoreFingerprint:             frgj.metadata.StoreFingerprint,
		ClusterFingerprint:           frgj.metadata.ClusterFingerprint,
		CurrentPoint:                 frgj.state.CurrentPoint,
		SnapshotGeneration:           frgj.state.SnapshotGeneration,
		BatchRoot:                    frgj.state.BatchRoot,
		InventoryRoot:                root,
		WalletCount:                  walletCount,
		MinimumActualGroupSize:       minimumSize,
		MaximumActualGroupSize:       maximumSize,
		QuarantineProtocolID:         frgj.quarantineMetadata.ProtocolID,
		QuarantineStoreID:            frgj.quarantineMetadata.StoreID,
		QuarantineStoreFingerprint:   frgj.quarantineMetadata.StoreFingerprint,
		QuarantineClusterFingerprint: frgj.quarantineMetadata.ClusterFingerprint,
		QuarantineMinimumGeneration:  frgj.quarantineMinimumGeneration,
		QuarantineGeneration:         frgj.quarantineState.Generation,
		QuarantineRoot:               frgj.quarantineState.Root,
		QuarantineCount:              uint64(len(frgj.quarantineState.Quarantines)),
		LocalSessionCount:            localSessionCount,
		Complete:                     true,
	}, nil
}

func validateCompleteFrostRetainedGroupHistory(
	history *FrostRetainedGroupHistory,
) error {
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range history.Mutations {
		if !mutation.Point.valid() || mutation.Point.BlockNumber <= history.From.BlockNumber ||
			mutation.Point.BlockNumber > history.To.BlockNumber ||
			(index > 0 && compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("complete FROST retained-group history has invalid event bounds or ordering")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("complete FROST retained-group history has conflicting block hashes")
		}
		previous = mutation.Point
	}
	probe := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: history.From,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	if err := applyFrostRetainedGroupMutations(
		&probe,
		frostRetainedGroupCanonicalMutations(history.Mutations),
	); err != nil {
		return fmt.Errorf("complete FROST retained-group history is semantically invalid: [%w]", err)
	}
	quarantineProbe := frostRetainedGroupQuarantineJournalState{
		Schema:       frostRetainedGroupQuarantineStateSchema,
		CurrentPoint: history.From,
		Root:         sha256.Sum256([]byte(frostRetainedGroupQuarantineDomain)),
		Quarantines:  []frostRetainedGroupQuarantineState{},
	}
	if err := applyFrostRetainedGroupQuarantineMutations(
		&quarantineProbe,
		frostRetainedGroupQuarantineMutations(history.Mutations),
	); err != nil {
		return fmt.Errorf("complete FROST quarantine history is semantically invalid: [%w]", err)
	}
	return nil
}

func cloneFrostRetainedGroupState(
	state frostRetainedGroupJournalState,
) frostRetainedGroupJournalState {
	result := state
	result.Wallets = append([]frostRetainedGroupWalletState{}, state.Wallets...)
	for index := range result.Wallets {
		result.Wallets[index].OperatorIDs = append([]uint32{}, state.Wallets[index].OperatorIDs...)
	}
	return result
}

func cloneFrostRetainedGroupQuarantineState(
	state frostRetainedGroupQuarantineJournalState,
) frostRetainedGroupQuarantineJournalState {
	result := state
	result.Quarantines = append(
		[]frostRetainedGroupQuarantineState{},
		state.Quarantines...,
	)
	return result
}

type frostLocalSessionSnapshot struct {
	WalletID            [32]byte
	WalletPublicKeyHash [20]byte
	KeyGroup            string
	OperatorAddresses   []chain.Address
	ControlledSeats     []group.MemberIndex
}

func (wr *walletRegistry) frostLocalSessionSnapshot() (
	[]frostLocalSessionSnapshot,
	error,
) {
	if wr == nil {
		return nil, fmt.Errorf("wallet registry is nil")
	}
	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	return wr.frostLocalSessionSnapshotLocked()
}

func (wr *walletRegistry) frostLocalSessionSnapshotLocked() (
	[]frostLocalSessionSnapshot,
	error,
) {
	result := make([]frostLocalSessionSnapshot, 0)
	for _, value := range wr.walletCache {
		if value == nil || len(value.signers) == 0 {
			return nil, fmt.Errorf("wallet registry contains an empty session")
		}
		_, isFrostWallet, err := frostKeyGroupFromWalletCacheValue(value)
		if err != nil {
			return nil, fmt.Errorf(
				"wallet registry cannot classify local session material: [%w]",
				err,
			)
		}
		if !isFrostWallet {
			continue
		}
		nativeSigners := make([]*signer, 0)
		for _, signer := range value.signers {
			if signer == nil {
				return nil, fmt.Errorf("wallet registry contains a nil signer")
			}
			switch signer.signingMaterial().(type) {
			case *frostsigning.NativeSignerMaterial, frostsigning.NativeSignerMaterial:
				nativeSigners = append(nativeSigners, signer)
			}
		}
		if len(nativeSigners) == 0 {
			continue
		}
		if len(nativeSigners) != len(value.signers) {
			return nil, fmt.Errorf("wallet registry mixes FROST and legacy signer material")
		}
		wallet := nativeSigners[0].wallet
		session := frostLocalSessionSnapshot{
			WalletID:            value.walletID,
			WalletPublicKeyHash: value.walletPublicKeyHash,
			OperatorAddresses:   append([]chain.Address{}, wallet.signingGroupOperators...),
			ControlledSeats:     make([]group.MemberIndex, 0, len(nativeSigners)),
		}
		seenSeats := make(map[group.MemberIndex]struct{})
		for _, signer := range nativeSigners {
			if signer.wallet.publicKey == nil ||
				len(signer.wallet.signingGroupOperators) != len(session.OperatorAddresses) ||
				signer.signingGroupMemberIndex == 0 ||
				int(signer.signingGroupMemberIndex) > len(session.OperatorAddresses) {
				return nil, fmt.Errorf("FROST local session signer is malformed")
			}
			for index := range session.OperatorAddresses {
				if signer.wallet.signingGroupOperators[index] != session.OperatorAddresses[index] {
					return nil, fmt.Errorf("FROST local session operators disagree")
				}
			}
			var material *frostsigning.NativeSignerMaterial
			switch value := signer.signingMaterial().(type) {
			case *frostsigning.NativeSignerMaterial:
				material = value
			case frostsigning.NativeSignerMaterial:
				valueCopy := value
				material = &valueCopy
			default:
				return nil, fmt.Errorf("FROST local session signer material is unavailable")
			}
			keyGroup, err := frostKeyGroupFromSignerMaterial(
				material,
				session.WalletID,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"FROST local session key-group handle does not identify its wallet: [%w]",
					err,
				)
			}
			if session.KeyGroup == "" {
				session.KeyGroup = keyGroup
			} else if session.KeyGroup != keyGroup {
				return nil, fmt.Errorf("FROST local session key-group handles disagree")
			}
			if _, exists := seenSeats[signer.signingGroupMemberIndex]; exists {
				return nil, fmt.Errorf("FROST local session repeats a controlled seat")
			}
			seenSeats[signer.signingGroupMemberIndex] = struct{}{}
			session.ControlledSeats = append(session.ControlledSeats, signer.signingGroupMemberIndex)
		}
		sort.Slice(session.ControlledSeats, func(i, j int) bool {
			return session.ControlledSeats[i] < session.ControlledSeats[j]
		})
		result = append(result, session)
	}
	sort.Slice(result, func(i, j int) bool {
		return bytes.Compare(result[i].WalletID[:], result[j].WalletID[:]) < 0
	})
	return result, nil
}

func (frgj *frostRetainedGroupJournal) reconcileLocalSessions(
	ctx context.Context,
	point FrostPreSignFinality,
) (uint64, error) {
	localOperatorID, err := frgj.source.ResolveOperatorID(ctx, frgj.operatorAddress, point)
	if err != nil || localOperatorID == 0 {
		return 0, fmt.Errorf("cannot resolve local FROST operator ID at challenged point: [%w]", err)
	}
	sessions, err := frgj.walletRegistry.frostLocalSessionSnapshot()
	if err != nil {
		return 0, err
	}
	sessionsByWallet := make(map[[32]byte]frostLocalSessionSnapshot, len(sessions))
	for _, session := range sessions {
		if _, exists := sessionsByWallet[session.WalletID]; exists {
			return 0, fmt.Errorf("duplicate FROST local session wallet ID")
		}
		sessionsByWallet[session.WalletID] = session
	}
	operatorIDCache := make(map[chain.Address]chain.OperatorID)
	for _, wallet := range frgj.state.Wallets {
		session, hasSession := sessionsByWallet[wallet.WalletID]
		containsLocalOperator := false
		for _, operatorID := range wallet.OperatorIDs {
			if chain.OperatorID(operatorID) == localOperatorID {
				containsLocalOperator = true
				break
			}
		}
		if wallet.Lifecycle.terminal() {
			if hasSession {
				return 0, fmt.Errorf("terminal FROST retained group still has a local session")
			}
			continue
		}
		if containsLocalOperator != hasSession {
			return 0, fmt.Errorf("FROST local-session presence differs from retained-group membership")
		}
		if !hasSession {
			continue
		}
		if session.WalletPublicKeyHash != wallet.WalletPublicKeyHash ||
			len(session.OperatorAddresses) != len(wallet.OperatorIDs) {
			return 0, fmt.Errorf("FROST local session identity or group size differs from canonical DKG")
		}
		resolvedIDs := make([]chain.OperatorID, len(session.OperatorAddresses))
		for index, address := range session.OperatorAddresses {
			operatorID, exists := operatorIDCache[address]
			if !exists {
				operatorID, err = frgj.source.ResolveOperatorID(ctx, address, point)
				if err != nil || operatorID == 0 {
					return 0, fmt.Errorf("cannot resolve FROST DKG operator at challenged point: [%w]", err)
				}
				operatorIDCache[address] = operatorID
			}
			resolvedIDs[index] = operatorID
			if uint32(operatorID) != wallet.OperatorIDs[index] {
				return 0, fmt.Errorf("FROST local session operator ordering differs from canonical DKG")
			}
		}
		expectedSeats := make([]group.MemberIndex, 0)
		for index, operatorID := range resolvedIDs {
			if operatorID == localOperatorID {
				expectedSeats = append(expectedSeats, group.MemberIndex(index+1))
			}
		}
		if len(expectedSeats) != len(session.ControlledSeats) {
			return 0, fmt.Errorf("FROST local controlled-seat count differs from canonical DKG")
		}
		for index := range expectedSeats {
			if expectedSeats[index] != session.ControlledSeats[index] {
				return 0, fmt.Errorf("FROST local controlled seats differ from canonical DKG")
			}
		}
		delete(sessionsByWallet, wallet.WalletID)
	}
	if len(sessionsByWallet) != 0 {
		return 0, fmt.Errorf("FROST local session has no canonical retained group")
	}
	return uint64(len(sessions)), nil
}
