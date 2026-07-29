package tbtc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"golang.org/x/sync/semaphore"
	"golang.org/x/sys/unix"
)

const (
	bitcoinBroadcastOutboxRecordVersion = 5
	bitcoinBroadcastOutboxFileSuffix    = ".json"
	bitcoinBroadcastOutboxTempSuffix    = ".tmp"
	bitcoinBroadcastOutboxLockFile      = ".lock"

	defaultBitcoinBroadcastOutboxReplayInterval = time.Minute
	defaultBitcoinBroadcastArchiveConfirmations = 100
	defaultBitcoinBroadcastDeepReconcileBatch   = 4
)

var errBitcoinBroadcastQuarantined = errors.New(
	"canonical Bitcoin broadcast authorization does not permit broadcast",
)

type canonicalBitcoinBroadcastChain interface {
	bitcoin.Chain
	bitcoin.CanonicalTransactionStatusSource
}

// FrostBitcoinBroadcastOutpoint is one exact input committed by a durable
// authorization-status request.
type FrostBitcoinBroadcastOutpoint struct {
	TransactionHash bitcoin.Hash
	OutputIndex     uint32
}

// FrostBitcoinBroadcastAuthorizationStatusRequest is the ABI-neutral identity
// a concrete Ethereum adapter must revalidate against canonical finalized
// state before the outbox can broadcast. It deliberately carries both the
// record's pinned and the currently active activation profiles, plus the
// complete reservation and variant tuple.
type FrostBitcoinBroadcastAuthorizationStatusRequest struct {
	ActivationProfileHash       [32]byte
	ActiveActivationProfileHash [32]byte
	TransactionHash             bitcoin.Hash
	WalletPublicKeyHash         [20]byte
	WalletID                    [32]byte
	Action                      FrostPreSignAction
	OrderedOutpoints            []FrostBitcoinBroadcastOutpoint
	AuthorizationID             [32]byte
	ReservationID               [32]byte
	AuthorizationRoot           [32]byte
	SnapshotHash                [32]byte
	ResourceHash                [32]byte
	OrderedInputRoot            [32]byte
	LockedPlanHash              [32]byte
	VariantApplyPlanHash        [32]byte
	FeeLimitSnapshot            uint64
	FinalizedBlock              uint64
	FinalizedBlockHash          [32]byte
	FinalizedTransactionIndex   uint32
	FinalizedLogIndex           uint32
	VariantSequence             FrostPreSignVariantSequence
}

// ComputeHash returns a deterministic request commitment for exact response
// binding. This is an internal adapter protocol, not a guessed Solidity ABI.
func (fbasr *FrostBitcoinBroadcastAuthorizationStatusRequest) ComputeHash() [32]byte {
	if fbasr == nil {
		return [32]byte{}
	}
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-frost-bitcoin-broadcast-authorization-status-v2"))
	hasher.Write(fbasr.ActivationProfileHash[:])
	hasher.Write(fbasr.ActiveActivationProfileHash[:])
	hasher.Write(fbasr.TransactionHash[:])
	hasher.Write(fbasr.WalletPublicKeyHash[:])
	hasher.Write(fbasr.WalletID[:])
	hasher.Write([]byte{byte(fbasr.Action)})
	writeUint64 := func(value uint64) {
		var encoded [8]byte
		for i := 0; i < len(encoded); i++ {
			encoded[len(encoded)-1-i] = byte(value >> (8 * i))
		}
		hasher.Write(encoded[:])
	}
	writeUint32 := func(value uint32) {
		var encoded [4]byte
		for i := 0; i < len(encoded); i++ {
			encoded[len(encoded)-1-i] = byte(value >> (8 * i))
		}
		hasher.Write(encoded[:])
	}
	writeUint32(uint32(len(fbasr.OrderedOutpoints)))
	for _, outpoint := range fbasr.OrderedOutpoints {
		hasher.Write(outpoint.TransactionHash[:])
		writeUint32(outpoint.OutputIndex)
	}
	for _, value := range [][32]byte{
		fbasr.AuthorizationID,
		fbasr.ReservationID,
		fbasr.AuthorizationRoot,
		fbasr.SnapshotHash,
		fbasr.ResourceHash,
		fbasr.OrderedInputRoot,
		fbasr.LockedPlanHash,
		fbasr.VariantApplyPlanHash,
	} {
		hasher.Write(value[:])
	}
	writeUint64(fbasr.FeeLimitSnapshot)
	writeUint64(fbasr.FinalizedBlock)
	hasher.Write(fbasr.FinalizedBlockHash[:])
	writeUint32(fbasr.FinalizedTransactionIndex)
	writeUint32(fbasr.FinalizedLogIndex)
	hasher.Write(fbasr.VariantSequence.AuthorizationSequence[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

// FrostBitcoinBroadcastAuthorizationStatus is returned from a canonical,
// finalized Ethereum read. Canonical=false is never interpreted as permission.
// BroadcastAllowed may be false for a still-canonical, already-settled record;
// such a record can be reconciled but cannot be broadcast again.
type FrostBitcoinBroadcastAuthorizationStatus struct {
	RequestHash      [32]byte
	Canonical        bool
	BroadcastAllowed bool
}

// FrostBitcoinBroadcastAuthorizationStatusSource is intentionally only a hook
// until the reviewed COMPLETE ABI is stable. Activation has no default
// implementation and fails closed unless the configured chain supplies it.
type FrostBitcoinBroadcastAuthorizationStatusSource interface {
	GetCanonicalFrostBitcoinBroadcastAuthorizationStatus(
		context.Context,
		*FrostBitcoinBroadcastAuthorizationStatusRequest,
	) (*FrostBitcoinBroadcastAuthorizationStatus, error)
}

// bitcoinBroadcastAuthorization is the complete finalized authorization
// identity needed to audit and safely recover a signed transaction. Fields
// that define one reservation's semantic input/resource plan are immutable
// across every authorized RBF variant.
type bitcoinBroadcastAuthorization struct {
	ActivationProfileHash     [32]byte
	AuthorizationID           [32]byte
	ReservationID             [32]byte
	AuthorizationRoot         [32]byte
	SnapshotHash              [32]byte
	ResourceHash              [32]byte
	OrderedInputRoot          [32]byte
	LockedPlanHash            [32]byte
	VariantApplyPlanHash      [32]byte
	FeeLimitSnapshot          uint64
	FinalizedBlock            uint64
	FinalizedBlockHash        [32]byte
	FinalizedTransactionIndex uint32
	FinalizedLogIndex         uint32
	VariantSequence           FrostPreSignVariantSequence
}

type bitcoinBroadcastOutpoint struct {
	TransactionHash bitcoin.Hash `json:"transactionHash"`
	OutputIndex     uint32       `json:"outputIndex"`
}

type bitcoinBroadcastConfirmation struct {
	Confirmations  uint         `json:"confirmations"`
	BlockHeight    uint         `json:"blockHeight"`
	BlockHash      bitcoin.Hash `json:"blockHash"`
	Canonical      bool         `json:"canonical"`
	ObservedAtUnix int64        `json:"observedAtUnix"`
}

type bitcoinBroadcastQuarantine struct {
	ActiveActivationProfileHash [32]byte `json:"activeActivationProfileHash"`
	ObservedAtUnix              int64    `json:"observedAtUnix"`
}

type bitcoinBroadcastOutboxRecord struct {
	Version                 uint32                        `json:"version"`
	TransactionHash         bitcoin.Hash                  `json:"transactionHash"`
	WitnessTransactionHash  bitcoin.Hash                  `json:"witnessTransactionHash"`
	UnsignedTransactionHash bitcoin.Hash                  `json:"unsignedTransactionHash"`
	RawTransaction          []byte                        `json:"rawTransaction"`
	WalletPublicKeyHash     [20]byte                      `json:"walletPublicKeyHash"`
	WalletID                [32]byte                      `json:"walletID"`
	Action                  FrostPreSignAction            `json:"action"`
	OrderedOutpoints        []bitcoinBroadcastOutpoint    `json:"orderedOutpoints"`
	InputSetHash            [32]byte                      `json:"inputSetHash"`
	Authorization           bitcoinBroadcastAuthorization `json:"authorization"`
	CreatedAtUnix           int64                         `json:"createdAtUnix"`
	UpdatedAtUnix           int64                         `json:"updatedAtUnix"`
	FirstBroadcastAtUnix    int64                         `json:"firstBroadcastAtUnix"`
	LastAttemptUnix         int64                         `json:"lastAttemptUnix"`
	BroadcastAttempts       uint64                        `json:"broadcastAttempts"`
	Confirmation            *bitcoinBroadcastConfirmation `json:"confirmation,omitempty"`
	Quarantine              *bitcoinBroadcastQuarantine   `json:"quarantine,omitempty"`
}

type bitcoinBroadcastOutboxEnvelope struct {
	Payload  json.RawMessage `json:"payload"`
	Checksum [32]byte        `json:"checksum"`
}

// bitcoinBroadcastOutbox is an exclusively owned, crash-safe signed-
// transaction journal and rebroadcaster. Every mutation is committed with a
// temp-file write, file fsync, atomic rename, and directory fsync.
type bitcoinBroadcastOutbox struct {
	directory                 string
	btcChain                  canonicalBitcoinBroadcastChain
	authorizationStatusSource FrostBitcoinBroadcastAuthorizationStatusSource
	activationProfileHash     [32]byte
	replayInterval            time.Duration
	archiveConfirmations      uint
	deepReconcileBatch        int
	deepReconcileCursor       int
	now                       func() time.Time

	replaySemaphore *semaphore.Weighted
	mutex           sync.Mutex
	records         map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord
	lockFile        *os.File
	closed          bool
	recovered       bool

	persistFailureHook func(*bitcoinBroadcastOutboxRecord) error
}

func newBitcoinBroadcastOutbox(
	directory string,
	btcChain canonicalBitcoinBroadcastChain,
	authorizationStatusSource FrostBitcoinBroadcastAuthorizationStatusSource,
	activationProfileHash [32]byte,
) (*bitcoinBroadcastOutbox, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("Bitcoin broadcast outbox directory is empty")
	}
	if btcChain == nil {
		return nil, fmt.Errorf("Bitcoin broadcast outbox chain is nil")
	}
	if authorizationStatusSource == nil {
		return nil, fmt.Errorf("Bitcoin broadcast authorization status source is nil")
	}
	if activationProfileHash == [32]byte{} {
		return nil, fmt.Errorf("Bitcoin broadcast activation profile hash is zero")
	}

	cleanDirectory, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return nil, fmt.Errorf("cannot resolve Bitcoin broadcast outbox directory: [%w]", err)
	}
	if err := os.MkdirAll(cleanDirectory, 0700); err != nil {
		return nil, fmt.Errorf("cannot create Bitcoin broadcast outbox: [%w]", err)
	}
	if err := validateSecureBitcoinBroadcastDirectory(cleanDirectory); err != nil {
		return nil, err
	}
	if err := syncDirectory(cleanDirectory); err != nil {
		return nil, fmt.Errorf("cannot sync Bitcoin broadcast outbox directory: [%w]", err)
	}

	lockFile, err := acquireBitcoinBroadcastOutboxLock(cleanDirectory)
	if err != nil {
		return nil, err
	}
	outbox := &bitcoinBroadcastOutbox{
		directory:                 cleanDirectory,
		btcChain:                  btcChain,
		authorizationStatusSource: authorizationStatusSource,
		activationProfileHash:     activationProfileHash,
		replayInterval:            defaultBitcoinBroadcastOutboxReplayInterval,
		archiveConfirmations:      defaultBitcoinBroadcastArchiveConfirmations,
		deepReconcileBatch:        defaultBitcoinBroadcastDeepReconcileBatch,
		now:                       time.Now,
		replaySemaphore:           semaphore.NewWeighted(1),
		records:                   make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord),
		lockFile:                  lockFile,
	}
	if err := outbox.load(); err != nil {
		_ = outbox.close()
		return nil, err
	}

	return outbox, nil
}

func acquireBitcoinBroadcastOutboxLock(directory string) (*os.File, error) {
	path := filepath.Join(directory, bitcoinBroadcastOutboxLockFile)
	file, err := openSecureBitcoinBroadcastFile(
		path,
		unix.O_CREAT|unix.O_RDWR,
		0600,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot open Bitcoin broadcast outbox lock: [%w]", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("Bitcoin broadcast outbox is already owned by another process")
	}
	if err := file.Truncate(0); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("cannot initialize Bitcoin outbox lock: [%w]", err)
	}
	if _, err := file.WriteAt(
		[]byte(strconv.Itoa(os.Getpid())+"\n"),
		0,
	); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("cannot write Bitcoin outbox lock: [%w]", err)
	}
	if err := file.Sync(); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("cannot sync Bitcoin outbox lock: [%w]", err)
	}
	if err := syncDirectory(directory); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("cannot sync Bitcoin outbox lock directory: [%w]", err)
	}

	return file, nil
}

func validateSecureBitcoinBroadcastDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("cannot inspect Bitcoin broadcast outbox directory: [%w]", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Bitcoin broadcast outbox path is not a real directory")
	}
	if info.Mode().Perm() != 0700 {
		return fmt.Errorf(
			"Bitcoin broadcast outbox directory permissions [%o] are not 0700",
			info.Mode().Perm(),
		)
	}
	if err := validateBitcoinBroadcastOwner(info); err != nil {
		return err
	}
	return nil
}

func validateBitcoinBroadcastOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine Bitcoin broadcast storage owner")
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf(
			"Bitcoin broadcast storage is owned by uid [%d], expected [%d]",
			stat.Uid,
			os.Geteuid(),
		)
	}
	return nil
}

func openSecureBitcoinBroadcastFile(
	path string,
	flags int,
	mode uint32,
) (*os.File, error) {
	fd, err := unix.Open(
		path,
		flags|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		mode,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("cannot wrap Bitcoin broadcast storage descriptor")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("Bitcoin broadcast storage file is not regular")
	}
	if info.Mode().Perm() != os.FileMode(mode).Perm() {
		_ = file.Close()
		return nil, fmt.Errorf(
			"Bitcoin broadcast storage file permissions [%o] are not [%o]",
			info.Mode().Perm(),
			os.FileMode(mode).Perm(),
		)
	}
	if err := validateBitcoinBroadcastOwner(info); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (bbo *bitcoinBroadcastOutbox) close() error {
	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return nil
	}
	bbo.closed = true
	if bbo.lockFile == nil {
		return nil
	}
	unlockErr := unix.Flock(int(bbo.lockFile.Fd()), unix.LOCK_UN)
	closeErr := bbo.lockFile.Close()
	bbo.lockFile = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

// enqueue durably records tx before its signature may be returned. Repeating
// the exact operation is idempotent. Both directions of reservation binding
// are enforced: one input set cannot move between reservations, and one
// reservation cannot acquire a different ordered input or semantic plan.
func (bbo *bitcoinBroadcastOutbox) enqueue(
	tx *bitcoin.Transaction,
	walletPublicKeyHash [20]byte,
	walletID [32]byte,
	action FrostPreSignAction,
	unsignedTransactionHash bitcoin.Hash,
	authorization bitcoinBroadcastAuthorization,
) error {
	if tx == nil {
		return fmt.Errorf("cannot enqueue nil Bitcoin transaction")
	}
	if walletPublicKeyHash == [20]byte{} || walletID == [32]byte{} {
		return fmt.Errorf("cannot enqueue transaction without wallet alias and ID")
	}
	if action < FrostPreSignActionDepositSweep || action > FrostPreSignActionMovedFundsSweep {
		return fmt.Errorf("cannot enqueue transaction with invalid action [%d]", action)
	}
	if err := validateBitcoinBroadcastAuthorization(authorization); err != nil {
		return err
	}
	if authorization.ActivationProfileHash != bbo.activationProfileHash {
		return fmt.Errorf("Bitcoin broadcast authorization activation profile mismatch")
	}

	rawTransaction := tx.Serialize(bitcoin.Witness)
	if len(rawTransaction) == 0 {
		return fmt.Errorf("cannot serialize signed Bitcoin transaction")
	}
	transactionHash := tx.Hash()
	if transactionHash != unsignedTransactionHash {
		return fmt.Errorf("signed transaction txid differs from authorized unsigned transaction")
	}
	orderedOutpoints, inputSetHash, err := bitcoinTransactionOutpoints(tx)
	if err != nil {
		return err
	}
	now := bbo.now().Unix()
	record := &bitcoinBroadcastOutboxRecord{
		Version:                 bitcoinBroadcastOutboxRecordVersion,
		TransactionHash:         transactionHash,
		WitnessTransactionHash:  tx.WitnessHash(),
		UnsignedTransactionHash: unsignedTransactionHash,
		RawTransaction:          append([]byte{}, rawTransaction...),
		WalletPublicKeyHash:     walletPublicKeyHash,
		WalletID:                walletID,
		Action:                  action,
		OrderedOutpoints:        orderedOutpoints,
		InputSetHash:            inputSetHash,
		Authorization:           authorization,
		CreatedAtUnix:           now,
		UpdatedAtUnix:           now,
	}
	if err := validateBitcoinBroadcastOutboxRecord(record); err != nil {
		return err
	}

	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	if existing, ok := bbo.records[transactionHash]; ok {
		if sameBitcoinBroadcastOperation(existing, record) {
			return nil
		}
		return fmt.Errorf(
			"Bitcoin transaction [%x] is already bound to another durable outbox record",
			transactionHash,
		)
	}
	if err := validateBitcoinBroadcastRecordBindings(record, bbo.records); err != nil {
		return err
	}
	if err := validateNewBitcoinBroadcastVariantSequence(record, bbo.records); err != nil {
		return err
	}
	if err := bbo.commitRecord(record); err != nil {
		return fmt.Errorf("cannot persist signed Bitcoin transaction: [%w]", err)
	}
	bbo.records[transactionHash] = cloneBitcoinBroadcastOutboxRecord(record)
	return nil
}

func validateBitcoinBroadcastAuthorization(
	authorization bitcoinBroadcastAuthorization,
) error {
	for name, value := range map[string][32]byte{
		"activation profile hash": authorization.ActivationProfileHash,
		"authorization ID":        authorization.AuthorizationID,
		"reservation ID":          authorization.ReservationID,
		"authorization root":      authorization.AuthorizationRoot,
		"snapshot hash":           authorization.SnapshotHash,
		"resource hash":           authorization.ResourceHash,
		"ordered input root":      authorization.OrderedInputRoot,
		"locked plan hash":        authorization.LockedPlanHash,
		"variant apply plan hash": authorization.VariantApplyPlanHash,
		"finalized block hash":    authorization.FinalizedBlockHash,
	} {
		if value == [32]byte{} {
			return fmt.Errorf("cannot enqueue transaction without %s", name)
		}
	}
	if authorization.FinalizedBlock == 0 ||
		authorization.VariantSequence.AuthorizationSequence == [32]byte{} {
		return fmt.Errorf("cannot enqueue transaction without finalized variant ordering")
	}
	return nil
}

func (bbo *bitcoinBroadcastOutbox) frostBitcoinBroadcastAuthorizationStatusRequest(
	record *bitcoinBroadcastOutboxRecord,
) *FrostBitcoinBroadcastAuthorizationStatusRequest {
	outpoints := make(
		[]FrostBitcoinBroadcastOutpoint,
		len(record.OrderedOutpoints),
	)
	for i, outpoint := range record.OrderedOutpoints {
		outpoints[i] = FrostBitcoinBroadcastOutpoint{
			TransactionHash: outpoint.TransactionHash,
			OutputIndex:     outpoint.OutputIndex,
		}
	}
	authorization := record.Authorization
	return &FrostBitcoinBroadcastAuthorizationStatusRequest{
		ActivationProfileHash:       authorization.ActivationProfileHash,
		ActiveActivationProfileHash: bbo.activationProfileHash,
		TransactionHash:             record.TransactionHash,
		WalletPublicKeyHash:         record.WalletPublicKeyHash,
		WalletID:                    record.WalletID,
		Action:                      record.Action,
		OrderedOutpoints:            outpoints,
		AuthorizationID:             authorization.AuthorizationID,
		ReservationID:               authorization.ReservationID,
		AuthorizationRoot:           authorization.AuthorizationRoot,
		SnapshotHash:                authorization.SnapshotHash,
		ResourceHash:                authorization.ResourceHash,
		OrderedInputRoot:            authorization.OrderedInputRoot,
		LockedPlanHash:              authorization.LockedPlanHash,
		VariantApplyPlanHash:        authorization.VariantApplyPlanHash,
		FeeLimitSnapshot:            authorization.FeeLimitSnapshot,
		FinalizedBlock:              authorization.FinalizedBlock,
		FinalizedBlockHash:          authorization.FinalizedBlockHash,
		FinalizedTransactionIndex:   authorization.FinalizedTransactionIndex,
		FinalizedLogIndex:           authorization.FinalizedLogIndex,
		VariantSequence:             authorization.VariantSequence,
	}
}

func (bbo *bitcoinBroadcastOutbox) canonicalAuthorizationStatus(
	ctx context.Context,
	record *bitcoinBroadcastOutboxRecord,
) (*FrostBitcoinBroadcastAuthorizationStatus, error) {
	request := bbo.frostBitcoinBroadcastAuthorizationStatusRequest(record)
	status, err := bbo.authorizationStatusSource.
		GetCanonicalFrostBitcoinBroadcastAuthorizationStatus(ctx, request)
	if err != nil {
		requestError := fmt.Errorf(
			"cannot revalidate canonical Bitcoin broadcast authorization: [%w]",
			err,
		)
		if ctx.Err() != nil {
			return nil, requestError
		}
		return nil, &bitcoinBroadcastTransientReplayError{requestError}
	}
	if status == nil ||
		status.RequestHash != request.ComputeHash() ||
		!status.Canonical {
		return nil, fmt.Errorf("canonical Bitcoin broadcast authorization identity is invalid")
	}
	return status, nil
}

func (bbo *bitcoinBroadcastOutbox) start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("Bitcoin broadcast outbox context is nil")
	}
	if err := bbo.replayOnceWithContext(ctx); err != nil {
		var replayErrors *bitcoinBroadcastReplayErrors
		if !errors.As(err, &replayErrors) || replayErrors.hasFatalFailure() {
			return err
		}
		logger.Warnf(
			"Bitcoin broadcast outbox initial replay has retryable failures: [%v]",
			err,
		)
	}
	bbo.mutex.Lock()
	if bbo.closed {
		bbo.mutex.Unlock()
		return fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	bbo.recovered = true
	bbo.mutex.Unlock()

	go func() {
		ticker := time.NewTicker(bbo.replayInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := bbo.replayOnceWithContext(ctx); err != nil {
					logger.Errorf("Bitcoin broadcast outbox replay failed: [%v]", err)
				}
			}
		}
	}()

	return nil
}

type bitcoinBroadcastOutboxActivationSnapshot struct {
	Recovered                 bool
	PendingReservationCount   uint64
	AmbiguousReservationCount uint64
	QuarantineCount           uint64
}

// activationSnapshot captures all activation-relevant outbox state under one
// mutex acquisition. It deliberately groups records by reservation so an RBF
// history counts as one pending operation, while multiple confirmed variants
// are surfaced as an ambiguity that blocks activation.
func (bbo *bitcoinBroadcastOutbox) activationSnapshot() (
	bitcoinBroadcastOutboxActivationSnapshot,
	error,
) {
	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return bitcoinBroadcastOutboxActivationSnapshot{},
			fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	return bbo.activationSnapshotLocked(), nil
}

// withUnchangedActivationSnapshot serializes the final activation-state check
// and the operation it authorizes with every outbox mutation. Callers must
// keep operation short and must not call another method that acquires mutex.
func (bbo *bitcoinBroadcastOutbox) withUnchangedActivationSnapshot(
	expected bitcoinBroadcastOutboxActivationSnapshot,
	operation func() error,
) error {
	if operation == nil {
		return fmt.Errorf("Bitcoin broadcast outbox activation operation is nil")
	}
	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	if bbo.activationSnapshotLocked() != expected {
		return fmt.Errorf(
			"Bitcoin broadcast outbox activation state changed before signing",
		)
	}
	return operation()
}

func (bbo *bitcoinBroadcastOutbox) activationSnapshotLocked() bitcoinBroadcastOutboxActivationSnapshot {
	type reservationState struct {
		pending          bool
		confirmedVariant bitcoin.Hash
		ambiguous        bool
	}
	reservations := make(map[[32]byte]*reservationState)
	quarantineCount := uint64(0)
	for _, record := range bbo.records {
		reservationID := record.Authorization.ReservationID
		state := reservations[reservationID]
		if state == nil {
			state = &reservationState{}
			reservations[reservationID] = state
		}
		if record.Quarantine != nil {
			quarantineCount++
		}
		if record.Confirmation == nil || !record.Confirmation.Canonical {
			state.pending = true
			continue
		}
		if state.confirmedVariant != (bitcoin.Hash{}) &&
			state.confirmedVariant != record.TransactionHash {
			state.ambiguous = true
		}
		state.confirmedVariant = record.TransactionHash
	}
	snapshot := bitcoinBroadcastOutboxActivationSnapshot{
		Recovered:       bbo.recovered,
		QuarantineCount: quarantineCount,
	}
	for _, state := range reservations {
		if state.pending && state.confirmedVariant == (bitcoin.Hash{}) {
			snapshot.PendingReservationCount++
		}
		if state.ambiguous {
			snapshot.AmbiguousReservationCount++
		}
	}
	return snapshot
}

// replayOnce is the test/internal synchronous entry point.
func (bbo *bitcoinBroadcastOutbox) replayOnce() error {
	return bbo.replayOnceWithContext(context.Background())
}

type bitcoinBroadcastReplayCandidate struct {
	reservationID [32]byte
	primary       *bitcoinBroadcastOutboxRecord
	alternatives  []*bitcoinBroadcastOutboxRecord
}

type bitcoinBroadcastTransientReplayError struct {
	err error
}

func (errorValue *bitcoinBroadcastTransientReplayError) Error() string {
	return errorValue.err.Error()
}

func (errorValue *bitcoinBroadcastTransientReplayError) Unwrap() error {
	return errorValue.err
}

type bitcoinBroadcastReplayFailure struct {
	reservationID [32]byte
	err           error
}

type bitcoinBroadcastReplayErrors struct {
	failures []bitcoinBroadcastReplayFailure
}

func (errorValue *bitcoinBroadcastReplayErrors) Error() string {
	parts := make([]string, 0, len(errorValue.failures))
	for _, failure := range errorValue.failures {
		parts = append(parts, fmt.Sprintf(
			"reservation [%x]: %v",
			failure.reservationID,
			failure.err,
		))
	}
	return strings.Join(parts, "; ")
}

func (errorValue *bitcoinBroadcastReplayErrors) Unwrap() []error {
	result := make([]error, 0, len(errorValue.failures))
	for _, failure := range errorValue.failures {
		result = append(result, failure.err)
	}
	return result
}

func (errorValue *bitcoinBroadcastReplayErrors) hasFatalFailure() bool {
	for _, failure := range errorValue.failures {
		var transient *bitcoinBroadcastTransientReplayError
		if !errors.As(failure.err, &transient) {
			return true
		}
	}
	return false
}

// replayOnceWithContext checks the latest or previously confirmed record first.
// Before rebroadcasting, it also reconciles every superseded signed variant,
// because another wallet operator may have broadcast any one of those
// conflicting variants. Deeply confirmed history is reconciled in a fixed-size
// rotating reservation batch; healthy archived reservations still require only
// their primary confirmation read.
func (bbo *bitcoinBroadcastOutbox) replayOnceWithContext(ctx context.Context) error {
	if err := bbo.acquireReplaySemaphore(ctx); err != nil {
		return err
	}
	defer bbo.replaySemaphore.Release(1)

	active, archived, err := bbo.replayCandidates()
	if err != nil {
		return err
	}
	replayErrors := &bitcoinBroadcastReplayErrors{}
candidateLoop:
	for _, candidate := range append(active, archived...) {
		refreshed, err := bbo.refreshConfirmation(ctx, candidate.primary)
		if err != nil {
			replayErrors.failures = append(
				replayErrors.failures,
				bitcoinBroadcastReplayFailure{candidate.reservationID, err},
			)
			continue
		}
		if refreshed.Confirmation != nil {
			continue
		}
		canonicalVariantFound := false
		for _, alternative := range candidate.alternatives {
			refreshed, err := bbo.refreshConfirmation(ctx, alternative)
			if err != nil {
				replayErrors.failures = append(
					replayErrors.failures,
					bitcoinBroadcastReplayFailure{candidate.reservationID, err},
				)
				continue candidateLoop
			}
			if refreshed.Confirmation != nil {
				canonicalVariantFound = true
				break
			}
		}
		if canonicalVariantFound {
			continue
		}
		latest, err := bbo.latestBroadcastRecord(candidate.reservationID)
		if err != nil {
			replayErrors.failures = append(
				replayErrors.failures,
				bitcoinBroadcastReplayFailure{candidate.reservationID, err},
			)
			continue
		}
		broadcastErr, broadcastRecord, err := bbo.broadcastAuthorizedRecord(ctx, latest)
		if err != nil {
			if errors.Is(err, errBitcoinBroadcastQuarantined) {
				continue
			}
			replayErrors.failures = append(
				replayErrors.failures,
				bitcoinBroadcastReplayFailure{candidate.reservationID, err},
			)
			continue
		}
		if broadcastErr != nil {
			continue
		}
		if _, err := bbo.refreshConfirmation(ctx, broadcastRecord); err != nil {
			replayErrors.failures = append(
				replayErrors.failures,
				bitcoinBroadcastReplayFailure{candidate.reservationID, err},
			)
		}
	}
	if len(replayErrors.failures) > 0 {
		return replayErrors
	}
	return nil
}

func (bbo *bitcoinBroadcastOutbox) replayCandidates() (
	[]bitcoinBroadcastReplayCandidate,
	[]bitcoinBroadcastReplayCandidate,
	error,
) {
	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return nil, nil, fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	type reservationState struct {
		latest    *bitcoinBroadcastOutboxRecord
		confirmed *bitcoinBroadcastOutboxRecord
		records   []*bitcoinBroadcastOutboxRecord
	}
	states := make(map[[32]byte]*reservationState)
	for _, record := range bbo.records {
		reservationID := record.Authorization.ReservationID
		state := states[reservationID]
		if state == nil {
			state = &reservationState{}
			states[reservationID] = state
		}
		state.records = append(state.records, record)
		if state.latest == nil || laterBitcoinBroadcastVariant(record, state.latest) {
			state.latest = record
		}
		if record.Confirmation != nil {
			if state.confirmed != nil &&
				state.confirmed.TransactionHash != record.TransactionHash {
				return nil, nil, fmt.Errorf(
					"Bitcoin reservation [%x] has multiple confirmed variants",
					reservationID,
				)
			}
			state.confirmed = record
		}
	}
	active := make([]bitcoinBroadcastReplayCandidate, 0, len(states))
	archived := make([]bitcoinBroadcastReplayCandidate, 0, len(states))
	for reservationID, state := range states {
		primary := state.latest
		if state.confirmed != nil {
			primary = state.confirmed
		}
		alternatives := make(
			[]*bitcoinBroadcastOutboxRecord,
			0,
			len(state.records)-1,
		)
		for _, record := range state.records {
			if record.TransactionHash == primary.TransactionHash {
				continue
			}
			alternatives = append(
				alternatives,
				cloneBitcoinBroadcastOutboxRecord(record),
			)
		}
		sort.Slice(alternatives, func(i, j int) bool {
			return laterBitcoinBroadcastVariant(
				alternatives[j],
				alternatives[i],
			)
		})
		candidate := bitcoinBroadcastReplayCandidate{
			reservationID: reservationID,
			primary:       cloneBitcoinBroadcastOutboxRecord(primary),
			alternatives:  alternatives,
		}
		if primary.Confirmation != nil &&
			primary.Confirmation.Canonical &&
			primary.Confirmation.Confirmations >= bbo.archiveConfirmations {
			archived = append(archived, candidate)
		} else {
			active = append(active, candidate)
		}
	}
	sortCandidates := func(candidates []bitcoinBroadcastReplayCandidate) {
		sort.Slice(candidates, func(i, j int) bool {
			return bytes.Compare(
				candidates[i].reservationID[:],
				candidates[j].reservationID[:],
			) < 0
		})
	}
	sortCandidates(active)
	sortCandidates(archived)
	if len(archived) == 0 || bbo.deepReconcileBatch <= 0 {
		return active, nil, nil
	}
	batchSize := bbo.deepReconcileBatch
	if batchSize > len(archived) {
		batchSize = len(archived)
	}
	start := bbo.deepReconcileCursor % len(archived)
	batch := make([]bitcoinBroadcastReplayCandidate, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		batch = append(batch, archived[(start+i)%len(archived)])
	}
	bbo.deepReconcileCursor = (start + batchSize) % len(archived)
	return active, batch, nil
}

// refreshConfirmation performs external reads against a clone, then commits
// and swaps only after the new envelope is durable. A Bitcoin RPC failure
// preserves confirmation evidence but may still durably record a canonical
// authorization quarantine; any persistence failure leaves shared state
// byte-for-byte unchanged.
func (bbo *bitcoinBroadcastOutbox) refreshConfirmation(
	ctx context.Context,
	record *bitcoinBroadcastOutboxRecord,
) (*bitcoinBroadcastOutboxRecord, error) {
	authorizationStatus, err := bbo.canonicalAuthorizationStatus(ctx, record)
	if err != nil {
		return nil, err
	}
	next, authorizationChanged := bbo.recordWithAuthorizationStatus(
		record,
		authorizationStatus,
	)

	status, err := bbo.btcChain.GetCanonicalTransactionStatus(record.TransactionHash)
	if err != nil {
		if !authorizationChanged {
			return cloneBitcoinBroadcastOutboxRecord(record), nil
		}
		if err := bbo.persistAndSwapRecord(record, next); err != nil {
			return nil, fmt.Errorf(
				"cannot persist Bitcoin broadcast quarantine observation: [%w]",
				err,
			)
		}
		return cloneBitcoinBroadcastOutboxRecord(next), nil
	}
	if status == nil {
		return nil, fmt.Errorf("canonical Bitcoin transaction status is nil")
	}
	confirmationChanged := false
	if !status.Found || status.Confirmations == 0 {
		if next.Confirmation != nil {
			next.Confirmation = nil
			confirmationChanged = true
		}
	} else {
		if status.BlockHeight == 0 || status.BlockHash == (bitcoin.Hash{}) {
			return nil, fmt.Errorf("canonical Bitcoin confirmation lacks block identity")
		}
		confirmation := &bitcoinBroadcastConfirmation{
			Confirmations:  status.Confirmations,
			BlockHeight:    status.BlockHeight,
			BlockHash:      status.BlockHash,
			Canonical:      true,
			ObservedAtUnix: bbo.now().Unix(),
		}
		if !sameBitcoinBroadcastConfirmation(next.Confirmation, confirmation) {
			next.Confirmation = confirmation
			confirmationChanged = true
		}
	}
	if !authorizationChanged && !confirmationChanged {
		return cloneBitcoinBroadcastOutboxRecord(record), nil
	}
	next.UpdatedAtUnix = maxBitcoinBroadcastTimestamp(
		next.UpdatedAtUnix,
		bbo.now().Unix(),
	)
	if next.Confirmation != nil && confirmationChanged {
		next.Confirmation.ObservedAtUnix = next.UpdatedAtUnix
	}
	if err := bbo.persistAndSwapRecord(record, next); err != nil {
		return nil, fmt.Errorf("cannot persist Bitcoin reconciliation observation: [%w]", err)
	}
	return cloneBitcoinBroadcastOutboxRecord(next), nil
}

func (bbo *bitcoinBroadcastOutbox) recordWithAuthorizationStatus(
	record *bitcoinBroadcastOutboxRecord,
	status *FrostBitcoinBroadcastAuthorizationStatus,
) (*bitcoinBroadcastOutboxRecord, bool) {
	next := cloneBitcoinBroadcastOutboxRecord(record)
	if status.BroadcastAllowed {
		if next.Quarantine == nil {
			return next, false
		}
		next.Quarantine = nil
		next.UpdatedAtUnix = maxBitcoinBroadcastTimestamp(
			next.UpdatedAtUnix,
			bbo.now().Unix(),
		)
		return next, true
	}
	if next.Quarantine != nil &&
		next.Quarantine.ActiveActivationProfileHash == bbo.activationProfileHash {
		return next, false
	}
	observedAt := maxBitcoinBroadcastTimestamp(
		next.UpdatedAtUnix,
		bbo.now().Unix(),
	)
	next.Quarantine = &bitcoinBroadcastQuarantine{
		ActiveActivationProfileHash: bbo.activationProfileHash,
		ObservedAtUnix:              observedAt,
	}
	next.UpdatedAtUnix = observedAt
	return next, true
}

func maxBitcoinBroadcastTimestamp(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func (bbo *bitcoinBroadcastOutbox) latestBroadcastRecord(
	reservationID [32]byte,
) (*bitcoinBroadcastOutboxRecord, error) {
	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return nil, fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	var latest *bitcoinBroadcastOutboxRecord
	for _, record := range bbo.records {
		if record.Authorization.ReservationID != reservationID {
			continue
		}
		if record.Confirmation != nil {
			return nil, fmt.Errorf(
				"Bitcoin reservation [%x] already has a confirmed variant",
				reservationID,
			)
		}
		if latest == nil || laterBitcoinBroadcastVariant(record, latest) {
			latest = record
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("Bitcoin reservation [%x] is absent", reservationID)
	}
	return cloneBitcoinBroadcastOutboxRecord(latest), nil
}

func (bbo *bitcoinBroadcastOutbox) broadcastAuthorizedRecord(
	ctx context.Context,
	record *bitcoinBroadcastOutboxRecord,
) (error, *bitcoinBroadcastOutboxRecord, error) {
	authorizationStatus, err := bbo.canonicalAuthorizationStatus(ctx, record)
	if err != nil {
		return nil, nil, err
	}
	next, authorizationChanged := bbo.recordWithAuthorizationStatus(
		record,
		authorizationStatus,
	)
	if !authorizationStatus.BroadcastAllowed {
		if authorizationChanged {
			if err := bbo.persistAndSwapRecord(record, next); err != nil {
				return nil, nil, fmt.Errorf(
					"cannot persist Bitcoin broadcast quarantine observation: [%w]",
					err,
				)
			}
		}
		return nil, cloneBitcoinBroadcastOutboxRecord(next),
			errBitcoinBroadcastQuarantined
	}
	tx := &bitcoin.Transaction{}
	if err := tx.Deserialize(record.RawTransaction); err != nil {
		return nil, nil, fmt.Errorf(
			"cannot deserialize durable Bitcoin transaction [%x]: [%w]",
			record.TransactionHash,
			err,
		)
	}
	broadcastErr := bbo.btcChain.BroadcastTransaction(tx)
	now := bbo.now().Unix()
	attemptedAt := maxBitcoinBroadcastTimestamp(
		next.UpdatedAtUnix,
		now,
	)
	attemptedAt = maxBitcoinBroadcastTimestamp(
		attemptedAt,
		next.FirstBroadcastAtUnix,
	)
	attemptedAt = maxBitcoinBroadcastTimestamp(
		attemptedAt,
		next.LastAttemptUnix,
	)
	if next.FirstBroadcastAtUnix == 0 {
		next.FirstBroadcastAtUnix = attemptedAt
	}
	next.BroadcastAttempts++
	next.LastAttemptUnix = attemptedAt
	next.UpdatedAtUnix = attemptedAt
	if err := bbo.persistAndSwapRecord(record, next); err != nil {
		return broadcastErr, nil, fmt.Errorf("cannot persist Bitcoin broadcast attempt: [%w]", err)
	}
	return broadcastErr, cloneBitcoinBroadcastOutboxRecord(next), nil
}

func (bbo *bitcoinBroadcastOutbox) broadcastTransaction(
	ctx context.Context,
	transactionHash bitcoin.Hash,
) error {
	if err := bbo.acquireReplaySemaphore(ctx); err != nil {
		return err
	}
	defer bbo.replaySemaphore.Release(1)

	bbo.mutex.Lock()
	record := bbo.records[transactionHash]
	if record != nil {
		record = cloneBitcoinBroadcastOutboxRecord(record)
	}
	bbo.mutex.Unlock()
	if record == nil {
		return fmt.Errorf(
			"Bitcoin transaction [%x] is not in the durable authorized outbox",
			transactionHash,
		)
	}
	latest, err := bbo.latestBroadcastRecord(record.Authorization.ReservationID)
	if err != nil {
		return err
	}
	if latest.TransactionHash != transactionHash {
		return fmt.Errorf(
			"Bitcoin transaction [%x] is not the latest authorized reservation variant",
			transactionHash,
		)
	}
	broadcastErr, _, err := bbo.broadcastAuthorizedRecord(ctx, latest)
	if err != nil {
		return err
	}
	return broadcastErr
}

func (bbo *bitcoinBroadcastOutbox) acquireReplaySemaphore(
	ctx context.Context,
) error {
	if ctx == nil {
		return fmt.Errorf("Bitcoin broadcast replay context is nil")
	}
	if bbo == nil || bbo.replaySemaphore == nil {
		return fmt.Errorf("Bitcoin broadcast replay semaphore is unavailable")
	}
	if err := bbo.replaySemaphore.Acquire(ctx, 1); err != nil {
		return fmt.Errorf(
			"cannot acquire Bitcoin broadcast replay semaphore: [%w]",
			err,
		)
	}
	return nil
}

func (bbo *bitcoinBroadcastOutbox) persistAndSwapRecord(
	expected *bitcoinBroadcastOutboxRecord,
	next *bitcoinBroadcastOutboxRecord,
) error {
	bbo.mutex.Lock()
	defer bbo.mutex.Unlock()
	if bbo.closed {
		return fmt.Errorf("Bitcoin broadcast outbox is closed")
	}
	current := bbo.records[expected.TransactionHash]
	if current == nil || !reflect.DeepEqual(current, expected) {
		return fmt.Errorf("Bitcoin broadcast outbox record changed concurrently")
	}
	if err := validateBitcoinBroadcastRecordTransition(current, next); err != nil {
		return err
	}
	if err := bbo.commitRecord(next); err != nil {
		return err
	}
	bbo.records[next.TransactionHash] = cloneBitcoinBroadcastOutboxRecord(next)
	return nil
}

func sameBitcoinBroadcastConfirmation(
	left *bitcoinBroadcastConfirmation,
	right *bitcoinBroadcastConfirmation,
) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Confirmations == right.Confirmations &&
		left.BlockHeight == right.BlockHeight &&
		left.BlockHash == right.BlockHash &&
		left.Canonical == right.Canonical
}

func laterBitcoinBroadcastVariant(
	left *bitcoinBroadcastOutboxRecord,
	right *bitcoinBroadcastOutboxRecord,
) bool {
	comparison := compareFrostPreSignVariantSequence(
		left.Authorization.VariantSequence,
		right.Authorization.VariantSequence,
	)
	if comparison != 0 {
		return comparison > 0
	}
	return bytes.Compare(left.TransactionHash[:], right.TransactionHash[:]) > 0
}

func compareFrostPreSignVariantSequence(
	left FrostPreSignVariantSequence,
	right FrostPreSignVariantSequence,
) int {
	return bytes.Compare(
		left.AuthorizationSequence[:],
		right.AuthorizationSequence[:],
	)
}

func (bbo *bitcoinBroadcastOutbox) load() error {
	entries, err := os.ReadDir(bbo.directory)
	if err != nil {
		return fmt.Errorf("cannot read Bitcoin broadcast outbox: [%w]", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	finalEntries := make([]os.DirEntry, 0)
	temporaryEntries := make([]os.DirEntry, 0)
	for _, entry := range entries {
		if entry.Name() == bitcoinBroadcastOutboxLockFile {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf(
				"symbolic link in Bitcoin broadcast outbox: [%s]",
				entry.Name(),
			)
		}
		if entry.IsDir() {
			return fmt.Errorf("unexpected directory in Bitcoin broadcast outbox: [%s]", entry.Name())
		}
		switch {
		case strings.HasSuffix(entry.Name(), bitcoinBroadcastOutboxTempSuffix):
			temporaryEntries = append(temporaryEntries, entry)
		case strings.HasSuffix(entry.Name(), bitcoinBroadcastOutboxFileSuffix):
			finalEntries = append(finalEntries, entry)
		default:
			return fmt.Errorf("unexpected file in Bitcoin broadcast outbox: [%s]", entry.Name())
		}
	}

	for _, entry := range finalEntries {
		record, err := bbo.readRecord(entry.Name())
		if err != nil {
			return err
		}
		if entry.Name() != bitcoinBroadcastOutboxRecordFileName(record.TransactionHash) {
			return fmt.Errorf("Bitcoin outbox record filename/hash mismatch: [%s]", entry.Name())
		}
		if err := bbo.addLoadedRecord(record); err != nil {
			return err
		}
	}

	type interruptedRecord struct {
		name   string
		record *bitcoinBroadcastOutboxRecord
	}
	interrupted := make([]interruptedRecord, 0, len(temporaryEntries))
	seenTemporaryHashes := make(map[bitcoin.Hash]struct{})
	prospective := make(map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord, len(bbo.records))
	for hash, record := range bbo.records {
		prospective[hash] = record
	}
	for _, entry := range temporaryEntries {
		record, err := bbo.readRecord(entry.Name())
		if err != nil {
			return fmt.Errorf(
				"interrupted Bitcoin outbox temp [%s] is partial or corrupt; refusing startup: [%w]",
				entry.Name(),
				err,
			)
		}
		finalName := bitcoinBroadcastOutboxRecordFileName(record.TransactionHash)
		if !strings.HasPrefix(entry.Name(), finalName+"-") {
			return fmt.Errorf("Bitcoin outbox temp filename/hash mismatch: [%s]", entry.Name())
		}
		if _, exists := seenTemporaryHashes[record.TransactionHash]; exists {
			return fmt.Errorf(
				"ambiguous Bitcoin outbox state: multiple temporary records exist for [%x]",
				record.TransactionHash,
			)
		}
		seenTemporaryHashes[record.TransactionHash] = struct{}{}
		if current := prospective[record.TransactionHash]; current != nil {
			if err := validateBitcoinBroadcastRecordTransition(current, record); err != nil {
				return fmt.Errorf(
					"interrupted Bitcoin outbox update [%s] is invalid: [%w]",
					entry.Name(),
					err,
				)
			}
		} else {
			if err := validateBitcoinBroadcastRecordBindings(record, prospective); err != nil {
				return err
			}
			if err := validateNewBitcoinBroadcastVariantSequence(record, prospective); err != nil {
				return err
			}
		}
		prospective[record.TransactionHash] = record
		interrupted = append(interrupted, interruptedRecord{entry.Name(), record})
	}

	promoted := false
	for _, item := range interrupted {
		finalName := bitcoinBroadcastOutboxRecordFileName(item.record.TransactionHash)
		if err := os.Rename(
			filepath.Join(bbo.directory, item.name),
			filepath.Join(bbo.directory, finalName),
		); err != nil {
			return fmt.Errorf("cannot promote durable Bitcoin outbox temp: [%w]", err)
		}
		promoted = true
		bbo.records[item.record.TransactionHash] = item.record
	}
	if promoted {
		if err := syncDirectory(bbo.directory); err != nil {
			return fmt.Errorf("cannot sync promoted Bitcoin outbox records: [%w]", err)
		}
	}

	return nil
}

func (bbo *bitcoinBroadcastOutbox) readRecord(
	name string,
) (*bitcoinBroadcastOutboxRecord, error) {
	file, err := openSecureBitcoinBroadcastFile(
		filepath.Join(bbo.directory, name),
		unix.O_RDONLY,
		0600,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot open Bitcoin outbox record [%s]: [%w]", name, err)
	}
	data, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read Bitcoin outbox record [%s]: [%w]", name, err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("cannot close Bitcoin outbox record [%s]: [%w]", name, closeErr)
	}
	record, err := decodeBitcoinBroadcastOutboxRecord(data)
	if err != nil {
		return nil, fmt.Errorf("corrupted Bitcoin outbox record [%s]: [%w]", name, err)
	}
	if err := validateBitcoinBroadcastOutboxRecord(record); err != nil {
		return nil, fmt.Errorf("invalid Bitcoin outbox record [%s]: [%w]", name, err)
	}
	return record, nil
}

func (bbo *bitcoinBroadcastOutbox) addLoadedRecord(
	record *bitcoinBroadcastOutboxRecord,
) error {
	if _, exists := bbo.records[record.TransactionHash]; exists {
		return fmt.Errorf("duplicate Bitcoin outbox transaction [%x]", record.TransactionHash)
	}
	if err := validateBitcoinBroadcastRecordBindings(record, bbo.records); err != nil {
		return err
	}
	bbo.records[record.TransactionHash] = record
	return nil
}

func validateBitcoinBroadcastRecordBindings(
	record *bitcoinBroadcastOutboxRecord,
	records map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord,
) error {
	for _, existing := range records {
		sameInputs := existing.InputSetHash == record.InputSetHash &&
			equalBitcoinBroadcastOutpoints(existing.OrderedOutpoints, record.OrderedOutpoints)
		sameReservation := existing.Authorization.ReservationID == record.Authorization.ReservationID
		if sameInputs && !sameReservation {
			return fmt.Errorf(
				"Bitcoin input set is already bound to reservation [%x]",
				existing.Authorization.ReservationID,
			)
		}
		if sameReservation && !sameInputs {
			return fmt.Errorf(
				"Bitcoin reservation [%x] is bound to another ordered input set",
				record.Authorization.ReservationID,
			)
		}
		if sameReservation && !sameBitcoinBroadcastReservationSemantics(existing, record) {
			return fmt.Errorf(
				"Bitcoin reservation [%x] has conflicting wallet/action/resource semantics",
				record.Authorization.ReservationID,
			)
		}
		if sameReservation &&
			existing.Authorization.VariantSequence == record.Authorization.VariantSequence &&
			existing.TransactionHash != record.TransactionHash {
			return fmt.Errorf(
				"Bitcoin reservation [%x] has duplicate variant sequence",
				record.Authorization.ReservationID,
			)
		}
	}
	return nil
}

func validateNewBitcoinBroadcastVariantSequence(
	record *bitcoinBroadcastOutboxRecord,
	records map[bitcoin.Hash]*bitcoinBroadcastOutboxRecord,
) error {
	for _, existing := range records {
		if existing.Authorization.ReservationID != record.Authorization.ReservationID {
			continue
		}
		if compareFrostPreSignVariantSequence(
			record.Authorization.VariantSequence,
			existing.Authorization.VariantSequence,
		) <= 0 {
			return fmt.Errorf(
				"Bitcoin reservation [%x] variant sequence is duplicate or retrograde",
				record.Authorization.ReservationID,
			)
		}
	}
	return nil
}

func sameBitcoinBroadcastReservationSemantics(
	left *bitcoinBroadcastOutboxRecord,
	right *bitcoinBroadcastOutboxRecord,
) bool {
	return left.WalletPublicKeyHash == right.WalletPublicKeyHash &&
		left.WalletID == right.WalletID &&
		left.Action == right.Action &&
		left.Authorization.SnapshotHash == right.Authorization.SnapshotHash &&
		left.Authorization.ResourceHash == right.Authorization.ResourceHash &&
		left.Authorization.OrderedInputRoot == right.Authorization.OrderedInputRoot &&
		left.Authorization.LockedPlanHash == right.Authorization.LockedPlanHash &&
		left.Authorization.FeeLimitSnapshot == right.Authorization.FeeLimitSnapshot
}

func sameBitcoinBroadcastOperation(
	left *bitcoinBroadcastOutboxRecord,
	right *bitcoinBroadcastOutboxRecord,
) bool {
	return left.Version == right.Version &&
		left.TransactionHash == right.TransactionHash &&
		left.WitnessTransactionHash == right.WitnessTransactionHash &&
		left.UnsignedTransactionHash == right.UnsignedTransactionHash &&
		bytes.Equal(left.RawTransaction, right.RawTransaction) &&
		left.WalletPublicKeyHash == right.WalletPublicKeyHash &&
		left.WalletID == right.WalletID &&
		left.Action == right.Action &&
		equalBitcoinBroadcastOutpoints(left.OrderedOutpoints, right.OrderedOutpoints) &&
		left.InputSetHash == right.InputSetHash &&
		left.Authorization == right.Authorization
}

func validateBitcoinBroadcastRecordTransition(
	current *bitcoinBroadcastOutboxRecord,
	next *bitcoinBroadcastOutboxRecord,
) error {
	if current == nil || next == nil {
		return fmt.Errorf("Bitcoin broadcast record transition contains nil state")
	}
	if err := validateBitcoinBroadcastOutboxRecord(next); err != nil {
		return err
	}
	if !sameBitcoinBroadcastOperation(current, next) ||
		current.CreatedAtUnix != next.CreatedAtUnix {
		return fmt.Errorf("Bitcoin broadcast record transition changed immutable identity")
	}
	if next.UpdatedAtUnix < current.UpdatedAtUnix ||
		next.BroadcastAttempts < current.BroadcastAttempts ||
		next.BroadcastAttempts > current.BroadcastAttempts+1 {
		return fmt.Errorf("Bitcoin broadcast record transition is retrograde or skips state")
	}
	if next.BroadcastAttempts == current.BroadcastAttempts {
		if next.FirstBroadcastAtUnix != current.FirstBroadcastAtUnix ||
			next.LastAttemptUnix != current.LastAttemptUnix {
			return fmt.Errorf("Bitcoin confirmation transition changed broadcast counters")
		}
	} else {
		if !reflect.DeepEqual(next.Confirmation, current.Confirmation) ||
			next.LastAttemptUnix < current.LastAttemptUnix ||
			next.LastAttemptUnix <= 0 {
			return fmt.Errorf("Bitcoin broadcast-attempt transition changed confirmation evidence")
		}
		if current.FirstBroadcastAtUnix == 0 {
			if next.FirstBroadcastAtUnix <= 0 {
				return fmt.Errorf("first Bitcoin broadcast attempt lacks timestamp")
			}
		} else if next.FirstBroadcastAtUnix != current.FirstBroadcastAtUnix {
			return fmt.Errorf("Bitcoin broadcast transition changed first-attempt timestamp")
		}
	}
	if current.Confirmation != nil && next.Confirmation != nil &&
		next.Confirmation.ObservedAtUnix < current.Confirmation.ObservedAtUnix {
		return fmt.Errorf("Bitcoin confirmation observation is retrograde")
	}
	if current.Quarantine != nil && next.Quarantine != nil &&
		next.Quarantine.ObservedAtUnix < current.Quarantine.ObservedAtUnix {
		return fmt.Errorf("Bitcoin quarantine observation is retrograde")
	}
	return nil
}

func (bbo *bitcoinBroadcastOutbox) commitRecord(
	record *bitcoinBroadcastOutboxRecord,
) error {
	if bbo.persistFailureHook != nil {
		if err := bbo.persistFailureHook(record); err != nil {
			return err
		}
	}
	return bbo.persistRecord(record)
}

func (bbo *bitcoinBroadcastOutbox) persistRecord(
	record *bitcoinBroadcastOutboxRecord,
) error {
	data, err := encodeBitcoinBroadcastOutboxRecord(record)
	if err != nil {
		return err
	}
	finalName := bitcoinBroadcastOutboxRecordFileName(record.TransactionHash)
	finalPath := filepath.Join(bbo.directory, finalName)
	temporaryFile, err := os.CreateTemp(
		bbo.directory,
		finalName+"-*"+bitcoinBroadcastOutboxTempSuffix,
	)
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporaryFile.Chmod(0600); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if _, err := temporaryFile.Write(data); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	if info, err := os.Lstat(finalPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("Bitcoin outbox final path is not a regular file")
		}
		if info.Mode().Perm() != 0600 {
			return fmt.Errorf("Bitcoin outbox final file permissions are unsafe")
		}
		if err := validateBitcoinBroadcastOwner(info); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return err
	}
	removeTemporary = false
	return syncDirectory(bbo.directory)
}

func encodeBitcoinBroadcastOutboxRecord(
	record *bitcoinBroadcastOutboxRecord,
) ([]byte, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	envelope := bitcoinBroadcastOutboxEnvelope{
		Payload:  payload,
		Checksum: sha256.Sum256(payload),
	}
	return json.Marshal(&envelope)
}

func decodeBitcoinBroadcastOutboxRecord(
	data []byte,
) (*bitcoinBroadcastOutboxRecord, error) {
	var envelope bitcoinBroadcastOutboxEnvelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("trailing outbox envelope data: [%w]", err)
	}
	if len(envelope.Payload) == 0 {
		return nil, fmt.Errorf("record payload is empty")
	}
	if sha256.Sum256(envelope.Payload) != envelope.Checksum {
		return nil, fmt.Errorf("record checksum mismatch")
	}

	return decodeBitcoinBroadcastOutboxPayload(envelope.Payload)
}

func decodeBitcoinBroadcastOutboxPayload(
	payload []byte,
) (*bitcoinBroadcastOutboxRecord, error) {
	var record bitcoinBroadcastOutboxRecord
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("trailing outbox payload data: [%w]", err)
	}
	return &record, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("additional JSON value")
}

func validateBitcoinBroadcastOutboxRecord(
	record *bitcoinBroadcastOutboxRecord,
) error {
	if record == nil {
		return fmt.Errorf("record is nil")
	}
	if record.Version != bitcoinBroadcastOutboxRecordVersion {
		return fmt.Errorf("unsupported record version [%d]", record.Version)
	}
	if err := validateBitcoinBroadcastAuthorization(record.Authorization); err != nil {
		return err
	}
	if record.WalletPublicKeyHash == [20]byte{} || record.WalletID == [32]byte{} {
		return fmt.Errorf("record wallet alias or ID is empty")
	}
	if record.Action < FrostPreSignActionDepositSweep ||
		record.Action > FrostPreSignActionMovedFundsSweep {
		return fmt.Errorf("record action [%d] is invalid", record.Action)
	}
	if len(record.RawTransaction) == 0 {
		return fmt.Errorf("raw transaction is empty")
	}
	if record.CreatedAtUnix <= 0 || record.UpdatedAtUnix < record.CreatedAtUnix {
		return fmt.Errorf("record state timestamps are invalid")
	}
	if record.BroadcastAttempts == 0 {
		if record.FirstBroadcastAtUnix != 0 || record.LastAttemptUnix != 0 {
			return fmt.Errorf("record has broadcast timestamps without attempts")
		}
	} else if record.FirstBroadcastAtUnix <= 0 ||
		record.LastAttemptUnix < record.FirstBroadcastAtUnix {
		return fmt.Errorf("record broadcast timestamps are invalid")
	}
	if record.Confirmation != nil {
		if record.Confirmation.Confirmations == 0 || record.Confirmation.ObservedAtUnix <= 0 {
			return fmt.Errorf("record confirmation evidence is invalid")
		}
		if record.Confirmation.Canonical &&
			(record.Confirmation.BlockHeight == 0 ||
				record.Confirmation.BlockHash == (bitcoin.Hash{})) {
			return fmt.Errorf("canonical confirmation evidence lacks block identity")
		}
	}
	if record.Quarantine != nil {
		if record.Quarantine.ActiveActivationProfileHash == [32]byte{} ||
			record.Quarantine.ObservedAtUnix <= 0 ||
			record.Quarantine.ObservedAtUnix > record.UpdatedAtUnix {
			return fmt.Errorf("record broadcast quarantine evidence is invalid")
		}
	}

	tx := &bitcoin.Transaction{}
	if err := tx.Deserialize(record.RawTransaction); err != nil {
		return fmt.Errorf("cannot deserialize raw transaction: [%w]", err)
	}
	if tx.Hash() != record.TransactionHash ||
		tx.Hash() != record.UnsignedTransactionHash {
		return fmt.Errorf("raw transaction txid/unsigned hash mismatch")
	}
	if tx.WitnessHash() != record.WitnessTransactionHash {
		return fmt.Errorf("raw transaction wtxid mismatch")
	}
	if !bytes.Equal(tx.Serialize(bitcoin.Witness), record.RawTransaction) {
		return fmt.Errorf("raw transaction is not canonically encoded")
	}
	orderedOutpoints, inputSetHash, err := bitcoinTransactionOutpoints(tx)
	if err != nil {
		return err
	}
	if inputSetHash != record.InputSetHash ||
		!equalBitcoinBroadcastOutpoints(orderedOutpoints, record.OrderedOutpoints) {
		return fmt.Errorf("transaction ordered outpoints mismatch")
	}

	return nil
}

func bitcoinTransactionOutpoints(
	tx *bitcoin.Transaction,
) ([]bitcoinBroadcastOutpoint, [32]byte, error) {
	if tx == nil || len(tx.Inputs) == 0 {
		return nil, [32]byte{}, fmt.Errorf("Bitcoin transaction has no inputs")
	}
	outpoints := make([]bitcoinBroadcastOutpoint, len(tx.Inputs))
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-bitcoin-broadcast-outbox-ordered-inputs-v2"))
	for i, input := range tx.Inputs {
		if input == nil || input.Outpoint == nil {
			return nil, [32]byte{}, fmt.Errorf("Bitcoin transaction input [%d] has no outpoint", i)
		}
		outpoints[i] = bitcoinBroadcastOutpoint{
			TransactionHash: input.Outpoint.TransactionHash,
			OutputIndex:     input.Outpoint.OutputIndex,
		}
		hasher.Write(input.Outpoint.TransactionHash[:])
		var index [4]byte
		index[0] = byte(input.Outpoint.OutputIndex)
		index[1] = byte(input.Outpoint.OutputIndex >> 8)
		index[2] = byte(input.Outpoint.OutputIndex >> 16)
		index[3] = byte(input.Outpoint.OutputIndex >> 24)
		hasher.Write(index[:])
	}
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return outpoints, result, nil
}

func equalBitcoinBroadcastOutpoints(
	left []bitcoinBroadcastOutpoint,
	right []bitcoinBroadcastOutpoint,
) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func bitcoinBroadcastOutboxRecordFileName(hash bitcoin.Hash) string {
	return hex.EncodeToString(hash[:]) + bitcoinBroadcastOutboxFileSuffix
}

func cloneBitcoinBroadcastOutboxRecord(
	record *bitcoinBroadcastOutboxRecord,
) *bitcoinBroadcastOutboxRecord {
	clone := *record
	clone.RawTransaction = append([]byte{}, record.RawTransaction...)
	clone.OrderedOutpoints = append([]bitcoinBroadcastOutpoint{}, record.OrderedOutpoints...)
	if record.Confirmation != nil {
		confirmation := *record.Confirmation
		clone.Confirmation = &confirmation
	}
	if record.Quarantine != nil {
		quarantine := *record.Quarantine
		clone.Quarantine = &quarantine
	}
	return &clone
}

func syncDirectory(directory string) error {
	fd, err := unix.Open(
		filepath.Clean(directory),
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), directory)
	if file == nil {
		_ = unix.Close(fd)
		return fmt.Errorf("cannot wrap Bitcoin outbox directory descriptor")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
