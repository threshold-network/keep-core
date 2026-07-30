package tbtc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"golang.org/x/sys/unix"
)

var (
	testOutboxWalletPublicKeyHash = [20]byte{0xa1}
	testOutboxWalletID            = [32]byte{0xb2}
	testOutboxActivationProfile   = [32]byte{0xc3}
)

func TestBitcoinBroadcastOutbox_PersistsAndRestoresFullRecord(t *testing.T) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)

	tx := testOutboxTransaction(1, 9000)
	authorization := testBitcoinBroadcastAuthorization(1, 1, 17)
	enqueueTestBitcoinTransaction(t, outbox, tx, authorization)

	record := outbox.records[tx.Hash()]
	if record == nil {
		t.Fatal("transaction record was not stored")
	}
	if record.TransactionHash != tx.Hash() ||
		record.WitnessTransactionHash != tx.WitnessHash() ||
		record.UnsignedTransactionHash != tx.Hash() {
		t.Fatal("txid/wtxid/unsigned hash evidence differs")
	}
	if record.WalletPublicKeyHash != testOutboxWalletPublicKeyHash ||
		record.WalletID != testOutboxWalletID ||
		record.Action != FrostPreSignActionDepositSweep {
		t.Fatal("wallet alias/ID/action evidence differs")
	}
	if len(record.OrderedOutpoints) != 1 ||
		record.OrderedOutpoints[0].TransactionHash != (bitcoin.Hash{1}) ||
		record.OrderedOutpoints[0].OutputIndex != 1 {
		t.Fatal("ordered outpoint evidence differs")
	}
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	restored := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer restored.close()
	if len(restored.records) != 1 {
		t.Fatalf("unexpected restored record count: [%d]", len(restored.records))
	}
	record = restored.records[tx.Hash()]
	if record == nil || record.Authorization != authorization {
		t.Fatalf("unexpected restored authorization: [%+v]", record)
	}
	if string(record.RawTransaction) != string(tx.Serialize(bitcoin.Witness)) {
		t.Fatal("restored raw transaction differs")
	}
}

func TestBitcoinBroadcastOutbox_RejectsFIFORecordWithoutBlocking(t *testing.T) {
	directory := t.TempDir()
	name := strings.Repeat("01", 32) + bitcoinBroadcastOutboxFileSuffix
	if err := unix.Mkfifo(filepath.Join(directory, name), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newTestBitcoinBroadcastOutbox(
		directory,
		newOutboxTestBitcoinChain(),
	); err == nil {
		t.Fatal("FIFO Bitcoin outbox record was accepted")
	}
}

func TestBitcoinBroadcastOutbox_StartupFailsOnCorruption(t *testing.T) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	tx := testOutboxTransaction(2, 8000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(2, 2, 1),
	)
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, bitcoinBroadcastOutboxRecordFileName(tx.Hash()))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 0x01
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newTestBitcoinBroadcastOutbox(directory, chain); err == nil {
		t.Fatal("expected corrupted outbox startup to fail")
	}
}

func TestBitcoinBroadcastOutbox_StrictJSONRejectsTrailingValues(t *testing.T) {
	record := testBitcoinBroadcastOutboxRecord(
		testOutboxTransaction(3, 8000),
		testBitcoinBroadcastAuthorization(3, 3, 1),
	)
	encoded, err := encodeBitcoinBroadcastOutboxRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBitcoinBroadcastOutboxRecord(append(encoded, []byte(` {}`)...)); err == nil {
		t.Fatal("expected trailing envelope value to fail")
	}

	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBitcoinBroadcastOutboxPayload(append(payload, []byte(` {}`)...)); err == nil {
		t.Fatal("expected trailing payload value to fail")
	}
}

func TestBitcoinBroadcastOutbox_CrashPointRecovery(t *testing.T) {
	tests := map[string]struct {
		writeInterrupted func(*testing.T, string, *bitcoinBroadcastOutboxRecord)
		expectSuccess    bool
	}{
		"before write fails closed": {
			writeInterrupted: func(t *testing.T, directory string, record *bitcoinBroadcastOutboxRecord) {
				name := bitcoinBroadcastOutboxRecordFileName(record.TransactionHash) + "-empty.tmp"
				if err := os.WriteFile(filepath.Join(directory, name), nil, 0600); err != nil {
					t.Fatal(err)
				}
			},
		},
		"partial write fails closed": {
			writeInterrupted: func(t *testing.T, directory string, record *bitcoinBroadcastOutboxRecord) {
				name := bitcoinBroadcastOutboxRecordFileName(record.TransactionHash) + "-partial.tmp"
				if err := os.WriteFile(filepath.Join(directory, name), []byte(`{"payload":`), 0600); err != nil {
					t.Fatal(err)
				}
			},
		},
		"fsynced temp before rename is promoted": {
			writeInterrupted: func(t *testing.T, directory string, record *bitcoinBroadcastOutboxRecord) {
				data, err := encodeBitcoinBroadcastOutboxRecord(record)
				if err != nil {
					t.Fatal(err)
				}
				name := bitcoinBroadcastOutboxRecordFileName(record.TransactionHash) + "-complete.tmp"
				file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write(data); err != nil {
					t.Fatal(err)
				}
				if err := file.Sync(); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			},
			expectSuccess: true,
		},
		"rename before directory fsync is retained": {
			writeInterrupted: func(t *testing.T, directory string, record *bitcoinBroadcastOutboxRecord) {
				data, err := encodeBitcoinBroadcastOutboxRecord(record)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(directory, bitcoinBroadcastOutboxRecordFileName(record.TransactionHash))
				file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write(data); err != nil {
					t.Fatal(err)
				}
				if err := file.Sync(); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			},
			expectSuccess: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			chain := newOutboxTestBitcoinChain()
			tx := testOutboxTransaction(3, 7000)
			record := testBitcoinBroadcastOutboxRecord(
				tx,
				testBitcoinBroadcastAuthorization(3, 3, 1),
			)
			test.writeInterrupted(t, directory, record)

			outbox, err := newTestBitcoinBroadcastOutbox(directory, chain)
			if !test.expectSuccess {
				if err == nil {
					outbox.close()
					t.Fatal("expected interrupted outbox startup to fail closed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer outbox.close()
			if outbox.records[tx.Hash()] == nil {
				t.Fatal("complete interrupted record was not recovered")
			}
			finalPath := filepath.Join(directory, bitcoinBroadcastOutboxRecordFileName(tx.Hash()))
			if _, err := os.Stat(finalPath); err != nil {
				t.Fatalf("recovered final record is absent: [%v]", err)
			}
		})
	}
}

func TestBitcoinBroadcastOutbox_ValidFinalAndTempUpdateRecovers(t *testing.T) {
	directory := t.TempDir()
	tx := testOutboxTransaction(4, 7000)
	current := testBitcoinBroadcastOutboxRecord(
		tx,
		testBitcoinBroadcastAuthorization(4, 4, 1),
	)
	data, err := encodeBitcoinBroadcastOutboxRecord(current)
	if err != nil {
		t.Fatal(err)
	}
	finalName := bitcoinBroadcastOutboxRecordFileName(tx.Hash())
	if err := os.WriteFile(filepath.Join(directory, finalName), data, 0600); err != nil {
		t.Fatal(err)
	}
	next := cloneBitcoinBroadcastOutboxRecord(current)
	next.BroadcastAttempts = 1
	next.FirstBroadcastAtUnix = current.UpdatedAtUnix + 1
	next.LastAttemptUnix = current.UpdatedAtUnix + 1
	next.UpdatedAtUnix = current.UpdatedAtUnix + 1
	data, err = encodeBitcoinBroadcastOutboxRecord(next)
	if err != nil {
		t.Fatal(err)
	}
	tempName := finalName + "-interrupted.tmp"
	if err := os.WriteFile(filepath.Join(directory, tempName), data, 0600); err != nil {
		t.Fatal(err)
	}
	outbox, err := newTestBitcoinBroadcastOutbox(directory, newOutboxTestBitcoinChain())
	if err != nil {
		t.Fatal(err)
	}
	defer outbox.close()
	if outbox.records[tx.Hash()].BroadcastAttempts != 1 {
		t.Fatal("valid fsynced update temp was not promoted over the old final")
	}
	if _, err := os.Lstat(filepath.Join(directory, tempName)); !os.IsNotExist(err) {
		t.Fatal("promoted update temp still exists")
	}
}

func TestBitcoinBroadcastOutbox_InvalidFinalAndTempUpdateFailsClosed(t *testing.T) {
	directory := t.TempDir()
	tx := testOutboxTransaction(4, 7000)
	current := testBitcoinBroadcastOutboxRecord(
		tx,
		testBitcoinBroadcastAuthorization(4, 4, 1),
	)
	data, err := encodeBitcoinBroadcastOutboxRecord(current)
	if err != nil {
		t.Fatal(err)
	}
	finalName := bitcoinBroadcastOutboxRecordFileName(tx.Hash())
	if err := os.WriteFile(filepath.Join(directory, finalName), data, 0600); err != nil {
		t.Fatal(err)
	}
	invalid := cloneBitcoinBroadcastOutboxRecord(current)
	invalid.Authorization.AuthorizationID = [32]byte{0xff}
	data, err = encodeBitcoinBroadcastOutboxRecord(invalid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, finalName+"-invalid.tmp"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newTestBitcoinBroadcastOutbox(directory, newOutboxTestBitcoinChain()); err == nil {
		t.Fatal("expected invalid final/temp transition to fail closed")
	}
}

func TestBitcoinBroadcastOutbox_ExclusiveOwnershipAndStaleLock(t *testing.T) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	first := openTestBitcoinBroadcastOutbox(t, directory, chain)
	if _, err := newTestBitcoinBroadcastOutbox(directory, chain); err == nil {
		t.Fatal("expected concurrent outbox open to fail")
	}
	if err := first.close(); err != nil {
		t.Fatal(err)
	}

	// The lock file remains after a clean close just as it does after process
	// death. The kernel lock, not file deletion, determines ownership.
	if _, err := os.Stat(filepath.Join(directory, bitcoinBroadcastOutboxLockFile)); err != nil {
		t.Fatal("expected stale lock file to remain")
	}
	second := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer second.close()
}

func TestBitcoinBroadcastOutbox_CloseRetainsLockUntilBroadcastFinishes(
	t *testing.T,
) {
	directory := t.TempDir()
	broadcastStarted := make(chan struct{})
	broadcastRelease := make(chan struct{})
	var releaseOnce sync.Once
	releaseBroadcast := func() {
		releaseOnce.Do(func() {
			close(broadcastRelease)
		})
	}
	defer releaseBroadcast()

	chain := &blockingBroadcastOutboxTestBitcoinChain{
		outboxTestBitcoinChain: newOutboxTestBitcoinChain(),
		broadcastStarted:       broadcastStarted,
		broadcastRelease:       broadcastRelease,
	}
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer outbox.close()
	tx := testOutboxTransaction(0x3a, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(0x3b, 0x3c, 1),
	)

	broadcastResult := make(chan error, 1)
	go func() {
		broadcastResult <- outbox.broadcastTransaction(
			context.Background(),
			tx.Hash(),
		)
	}()
	select {
	case <-broadcastStarted:
	case <-time.After(time.Second):
		t.Fatal("broadcast did not reach the blocking Bitcoin call")
	}

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- outbox.close()
	}()
	closingDeadline := time.After(time.Second)
	for {
		outbox.mutex.Lock()
		closing := outbox.closing
		outbox.mutex.Unlock()
		if closing {
			break
		}
		select {
		case <-closingDeadline:
			releaseBroadcast()
			<-broadcastResult
			t.Fatal("outbox close did not begin")
		case <-time.After(time.Millisecond):
		}
	}
	select {
	case err := <-closeResult:
		releaseBroadcast()
		<-broadcastResult
		t.Fatalf(
			"outbox close returned before the in-flight broadcast finished: [%v]",
			err,
		)
	default:
	}

	if replacement, err := newTestBitcoinBroadcastOutbox(
		directory,
		chain,
	); err == nil {
		_ = replacement.close()
		releaseBroadcast()
		<-broadcastResult
		<-closeResult
		t.Fatal("replacement outbox acquired the in-flight owner's lock")
	}

	releaseBroadcast()
	select {
	case err := <-broadcastResult:
		if err != nil {
			t.Fatalf("in-flight broadcast did not persist during shutdown: [%v]", err)
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight broadcast did not finish after release")
	}
	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("outbox close did not finish after the broadcast drained")
	}

	restarted := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer restarted.close()
	if restarted.records[tx.Hash()].BroadcastAttempts != 1 {
		t.Fatal("in-flight broadcast attempt was not durable before unlock")
	}
}

func TestBitcoinBroadcastOutbox_StorageHardening(t *testing.T) {
	newOutbox := func(directory string) (*bitcoinBroadcastOutbox, error) {
		return newBitcoinBroadcastOutbox(
			directory,
			newOutboxTestBitcoinChain(),
			newOutboxTestAuthorizationStatusSource(),
			testOutboxActivationProfile,
		)
	}

	t.Run("directory symlink", func(t *testing.T) {
		parent := t.TempDir()
		realDirectory := filepath.Join(parent, "real")
		if err := os.Mkdir(realDirectory, 0700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(parent, "link")
		if err := os.Symlink(realDirectory, link); err != nil {
			t.Fatal(err)
		}
		if _, err := newOutbox(link); err == nil {
			t.Fatal("expected symlink outbox directory to fail")
		}
	})

	t.Run("unsafe directory permissions", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.Chmod(directory, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := newOutbox(directory); err == nil {
			t.Fatal("expected group/world-readable outbox directory to fail")
		}
	})

	t.Run("lock symlink", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.Chmod(directory, 0700); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "target")
		if err := os.WriteFile(target, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(
			target,
			filepath.Join(directory, bitcoinBroadcastOutboxLockFile),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := newOutbox(directory); err == nil {
			t.Fatal("expected symlink lock file to fail")
		}
	})

	t.Run("record symlink", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.Chmod(directory, 0700); err != nil {
			t.Fatal(err)
		}
		tx := testOutboxTransaction(16, 7000)
		target := filepath.Join(t.TempDir(), "record")
		if err := os.WriteFile(target, []byte("not followed"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(
			target,
			filepath.Join(directory, bitcoinBroadcastOutboxRecordFileName(tx.Hash())),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := newOutbox(directory); err == nil {
			t.Fatal("expected symlink record file to fail")
		}
	})

	t.Run("unsafe record permissions", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.Chmod(directory, 0700); err != nil {
			t.Fatal(err)
		}
		tx := testOutboxTransaction(17, 7000)
		record := testBitcoinBroadcastOutboxRecord(
			tx,
			testBitcoinBroadcastAuthorization(18, 17, 1),
		)
		data, err := encodeBitcoinBroadcastOutboxRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(directory, bitcoinBroadcastOutboxRecordFileName(tx.Hash())),
			data,
			0644,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := newOutbox(directory); err == nil {
			t.Fatal("expected unsafe record permissions to fail")
		}
	})
}

func TestBitcoinBroadcastOutbox_IdempotenceAndBidirectionalReservationBinding(t *testing.T) {
	outbox := openTestBitcoinBroadcastOutbox(t, t.TempDir(), newOutboxTestBitcoinChain())
	defer outbox.close()

	first := testOutboxTransaction(5, 7000)
	firstAuthorization := testBitcoinBroadcastAuthorization(5, 5, 1)
	enqueueTestBitcoinTransaction(t, outbox, first, firstAuthorization)
	enqueueTestBitcoinTransaction(t, outbox, first, firstAuthorization)
	if len(outbox.records) != 1 {
		t.Fatalf("unexpected record count after idempotent enqueue: [%d]", len(outbox.records))
	}

	// Same ordered input set and reservation is an independently authorized RBF
	// variant when the immutable reservation semantics remain equal.
	replacement := testOutboxTransaction(5, 6500)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		replacement,
		testBitcoinBroadcastAuthorization(6, 5, 2),
	)

	conflictingReservation := testOutboxTransaction(5, 6000)
	err := enqueueTestBitcoinTransactionError(
		outbox,
		conflictingReservation,
		testBitcoinBroadcastAuthorization(7, 9, 3),
	)
	if err == nil {
		t.Fatal("expected the same input set under another reservation to fail")
	}

	differentInputs := testOutboxTransaction(8, 6000)
	err = enqueueTestBitcoinTransactionError(
		outbox,
		differentInputs,
		testBitcoinBroadcastAuthorization(8, 5, 3),
	)
	if err == nil {
		t.Fatal("expected one reservation with another input set to fail")
	}

	// The per-variant apply plan is expected to change for an RBF replacement.
	changedVariantApply := testBitcoinBroadcastAuthorization(9, 5, 3)
	changedVariantApply.VariantApplyPlanHash = [32]byte{0xff}
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		testOutboxTransaction(5, 5500),
		changedVariantApply,
	)
	duplicateSequence := testBitcoinBroadcastAuthorization(10, 5, 3)
	err = enqueueTestBitcoinTransactionError(
		outbox,
		testOutboxTransaction(5, 5250),
		duplicateSequence,
	)
	if err == nil {
		t.Fatal("expected duplicate authorization event position to fail")
	}
	retrogradeSequence := testBitcoinBroadcastAuthorization(11, 5, 2)
	err = enqueueTestBitcoinTransactionError(
		outbox,
		testOutboxTransaction(5, 5100),
		retrogradeSequence,
	)
	if err == nil {
		t.Fatal("expected retrograde authorization event position to fail")
	}

	conflictingPlan := testBitcoinBroadcastAuthorization(12, 5, 4)
	conflictingPlan.LockedPlanHash = [32]byte{0xff}
	err = enqueueTestBitcoinTransactionError(
		outbox,
		testOutboxTransaction(5, 5000),
		conflictingPlan,
	)
	if err == nil {
		t.Fatal("expected one reservation with another semantic plan to fail")
	}
}

func TestBitcoinBroadcastOutbox_StartupRejectsReservationWithDifferentInputs(t *testing.T) {
	directory := t.TempDir()
	first := testBitcoinBroadcastOutboxRecord(
		testOutboxTransaction(9, 7000),
		testBitcoinBroadcastAuthorization(10, 9, 1),
	)
	second := testBitcoinBroadcastOutboxRecord(
		testOutboxTransaction(10, 6500),
		testBitcoinBroadcastAuthorization(11, 9, 2),
	)
	for _, record := range []*bitcoinBroadcastOutboxRecord{first, second} {
		data, err := encodeBitcoinBroadcastOutboxRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(directory, bitcoinBroadcastOutboxRecordFileName(record.TransactionHash)),
			data,
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := newTestBitcoinBroadcastOutbox(directory, newOutboxTestBitcoinChain()); err == nil {
		t.Fatal("expected conflicting persisted reservation inputs to fail startup")
	}
}

func TestBitcoinBroadcastOutbox_ReplaysLatestVariantAndRecoversFromReorg(t *testing.T) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)

	oldVariant := testOutboxTransaction(11, 7000)
	newVariant := testOutboxTransaction(11, 6000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		oldVariant,
		testBitcoinBroadcastAuthorization(12, 11, 1),
	)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		newVariant,
		testBitcoinBroadcastAuthorization(13, 11, 2),
	)

	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if chain.broadcastCount(oldVariant.Hash()) != 0 ||
		chain.broadcastCount(newVariant.Hash()) != 1 {
		t.Fatal("outbox did not choose the latest authorized RBF variant")
	}

	chain.setCanonicalStatus(newVariant.Hash(), &bitcoin.CanonicalTransactionStatus{
		Found:         true,
		Confirmations: 2,
		BlockHeight:   800000,
		BlockHash:     bitcoin.Hash{0xcc},
	})
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[newVariant.Hash()].Confirmation == nil {
		t.Fatal("canonical confirmation was not persisted")
	}
	confirmedBroadcasts := chain.broadcastCount(newVariant.Hash())

	// An RPC error is not authenticated absence and must preserve evidence.
	chain.setCanonicalError(newVariant.Hash(), errors.New("index unavailable"))
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[newVariant.Hash()].Confirmation == nil ||
		chain.broadcastCount(newVariant.Hash()) != confirmedBroadcasts {
		t.Fatal("RPC error erased confirmation evidence or triggered rebroadcast")
	}

	// Authenticated canonical absence models a reorg. The prior evidence is
	// downgraded and the latest authorized variant resumes immediately.
	chain.setCanonicalError(newVariant.Hash(), nil)
	chain.setCanonicalStatus(newVariant.Hash(), &bitcoin.CanonicalTransactionStatus{Found: false})
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[newVariant.Hash()].Confirmation != nil {
		t.Fatal("reorg did not downgrade durable confirmation evidence")
	}
	if chain.broadcastCount(newVariant.Hash()) != confirmedBroadcasts+1 {
		t.Fatal("reorged transaction was not rebroadcast")
	}

	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer restarted.close()
	if err := restarted.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if chain.broadcastCount(newVariant.Hash()) != confirmedBroadcasts+2 {
		t.Fatal("reorg recovery did not remain active after restart")
	}
}

func TestBitcoinBroadcastOutbox_ReplayIsolatesCandidateFailures(t *testing.T) {
	chain := newOutboxTestBitcoinChain()
	statusSource := newOutboxTestAuthorizationStatusSource()
	outbox := openTestBitcoinBroadcastOutboxWithStatusSource(
		t,
		t.TempDir(),
		chain,
		statusSource,
	)
	defer outbox.close()

	failing := testOutboxTransaction(31, 7000)
	healthy := testOutboxTransaction(32, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		failing,
		testBitcoinBroadcastAuthorization(31, 1, 1),
	)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		healthy,
		testBitcoinBroadcastAuthorization(32, 2, 1),
	)
	statusSource.setTransactionError(
		failing.Hash(),
		errors.New("historical state unavailable"),
	)

	if err := outbox.replayOnce(); err == nil ||
		!strings.Contains(err.Error(), "historical state unavailable") {
		t.Fatalf("unexpected replay result: [%v]", err)
	}
	if chain.broadcastCount(failing.Hash()) != 0 {
		t.Fatal("failing candidate reached broadcast")
	}
	if chain.broadcastCount(healthy.Hash()) != 1 {
		t.Fatal("candidate after a failing record was starved")
	}
}

func TestBitcoinBroadcastOutbox_ReportsRejectedBitcoinRebroadcast(t *testing.T) {
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, t.TempDir(), chain)
	defer outbox.close()

	rejected := testOutboxTransaction(41, 7000)
	healthy := testOutboxTransaction(42, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		rejected,
		testBitcoinBroadcastAuthorization(41, 11, 1),
	)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		healthy,
		testBitcoinBroadcastAuthorization(42, 12, 1),
	)
	chain.setBroadcastError(
		rejected.Hash(),
		errors.New("txn-mempool-conflict"),
	)

	err := outbox.replayOnce()
	if err == nil || !strings.Contains(err.Error(), "txn-mempool-conflict") ||
		!strings.Contains(err.Error(), "on attempt [1]") {
		t.Fatalf("Bitcoin rebroadcast rejection was not surfaced: [%v]", err)
	}
	var replayErrors *bitcoinBroadcastReplayErrors
	if !errors.As(err, &replayErrors) || replayErrors.hasFatalFailure() {
		t.Fatalf("Bitcoin rebroadcast rejection is not retryable: [%v]", err)
	}
	if chain.broadcastCount(healthy.Hash()) != 1 {
		t.Fatal("candidate after a rejected rebroadcast was starved")
	}

	// The attempt counter separates a persistently failing entry from a single
	// transient rejection, so a stuck reservation is visible in one log line.
	if err := outbox.replayOnce(); err == nil ||
		!strings.Contains(err.Error(), "on attempt [2]") {
		t.Fatalf("repeated Bitcoin rebroadcast rejection was not surfaced: [%v]", err)
	}

	// A rejected rebroadcast must not stop recovery: it is retried, not fatal.
	contextValue, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := outbox.start(contextValue); err != nil {
		t.Fatalf("rejected rebroadcast stopped outbox recovery: [%v]", err)
	}
	snapshot, err := outbox.activationSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Recovered || snapshot.PendingReservationCount != 2 {
		t.Fatalf("unexpected recovery snapshot: [%+v]", snapshot)
	}

	chain.setBroadcastError(rejected.Hash(), nil)
	if err := outbox.replayOnce(); err != nil {
		t.Fatalf("accepted rebroadcast still reported a failure: [%v]", err)
	}
}

func TestBitcoinBroadcastOutbox_StartRetriesTransientCandidateFailure(
	t *testing.T,
) {
	chain := newOutboxTestBitcoinChain()
	statusSource := newOutboxTestAuthorizationStatusSource()
	outbox := openTestBitcoinBroadcastOutboxWithStatusSource(
		t,
		t.TempDir(),
		chain,
		statusSource,
	)
	defer outbox.close()
	transaction := testOutboxTransaction(33, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		transaction,
		testBitcoinBroadcastAuthorization(33, 3, 1),
	)
	statusSource.setTransactionError(
		transaction.Hash(),
		errors.New("provider temporarily unavailable"),
	)
	contextValue, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := outbox.start(contextValue); err != nil {
		t.Fatalf("transient first-pass failure stopped outbox: [%v]", err)
	}
	snapshot, err := outbox.activationSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Recovered || snapshot.PendingReservationCount != 1 {
		t.Fatalf("unexpected recovery snapshot: [%+v]", snapshot)
	}
}

func TestBitcoinBroadcastOutbox_ReconcilesPreviouslyBroadcastSupersededVariant(
	t *testing.T,
) {
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, t.TempDir(), chain)
	defer outbox.close()

	oldVariant := testOutboxTransaction(31, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		oldVariant,
		testBitcoinBroadcastAuthorization(32, 31, 1),
	)
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if chain.broadcastCount(oldVariant.Hash()) != 1 {
		t.Fatal("initial variant was not broadcast")
	}

	replacement := testOutboxTransaction(31, 6000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		replacement,
		testBitcoinBroadcastAuthorization(33, 31, 2),
	)
	chain.setCanonicalStatus(
		oldVariant.Hash(),
		&bitcoin.CanonicalTransactionStatus{
			Found:         true,
			Confirmations: 2,
			BlockHeight:   800031,
			BlockHash:     bitcoin.Hash{0x31, 0xcc},
		},
	)

	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[oldVariant.Hash()].Confirmation == nil {
		t.Fatal("canonical superseded RBF variant was not persisted")
	}
	if chain.broadcastCount(replacement.Hash()) != 0 {
		t.Fatal("replacement was broadcast after its predecessor confirmed")
	}
}

func TestBitcoinBroadcastOutbox_ReconcilesExternallyBroadcastSupersededVariant(
	t *testing.T,
) {
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, t.TempDir(), chain)
	defer outbox.close()

	oldVariant := testOutboxTransaction(34, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		oldVariant,
		testBitcoinBroadcastAuthorization(35, 34, 1),
	)

	replacement := testOutboxTransaction(34, 6000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		replacement,
		testBitcoinBroadcastAuthorization(36, 34, 2),
	)
	chain.setCanonicalStatus(
		oldVariant.Hash(),
		&bitcoin.CanonicalTransactionStatus{
			Found:         true,
			Confirmations: 2,
			BlockHeight:   800034,
			BlockHash:     bitcoin.Hash{0x34, 0xcc},
		},
	)

	if outbox.records[oldVariant.Hash()].BroadcastAttempts != 0 {
		t.Fatal("old variant unexpectedly has a local broadcast attempt")
	}
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[oldVariant.Hash()].Confirmation == nil {
		t.Fatal("externally broadcast superseded RBF variant was not persisted")
	}
	if chain.broadcastCount(replacement.Hash()) != 0 {
		t.Fatal("replacement was broadcast after its predecessor confirmed externally")
	}
}

func TestBitcoinBroadcastOutbox_BroadcastLockWaitHonorsContext(
	t *testing.T,
) {
	statusStarted := make(chan struct{})
	statusRelease := make(chan struct{})
	chain := &blockingOutboxTestBitcoinChain{
		outboxTestBitcoinChain: newOutboxTestBitcoinChain(),
		statusStarted:          statusStarted,
		statusRelease:          statusRelease,
	}
	outbox := openTestBitcoinBroadcastOutbox(
		t,
		t.TempDir(),
		chain,
	)
	defer outbox.close()
	tx := testOutboxTransaction(0x4a, 9000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(0x4b, 0x4c, 1),
	)

	replayResult := make(chan error, 1)
	go func() {
		replayResult <- outbox.replayOnceWithContext(context.Background())
	}()
	select {
	case <-statusStarted:
	case <-time.After(time.Second):
		close(statusRelease)
		t.Fatal("background replay did not enter the blocking Bitcoin read")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		50*time.Millisecond,
	)
	defer cancel()
	broadcastResult := make(chan error, 1)
	go func() {
		broadcastResult <- outbox.broadcastTransaction(ctx, tx.Hash())
	}()

	var broadcastErr error
	select {
	case broadcastErr = <-broadcastResult:
	case <-time.After(500 * time.Millisecond):
		close(statusRelease)
		<-replayResult
		broadcastErr = <-broadcastResult
		t.Fatalf(
			"foreground broadcast ignored its context while waiting for replay; eventual result: [%v]",
			broadcastErr,
		)
	}
	if !errors.Is(broadcastErr, context.DeadlineExceeded) {
		close(statusRelease)
		<-replayResult
		t.Fatalf("unexpected canceled broadcast result: [%v]", broadcastErr)
	}
	close(statusRelease)
	if err := <-replayResult; err != nil {
		t.Fatalf("background replay failed after release: [%v]", err)
	}
}

func TestBitcoinBroadcastOutbox_PersistenceFailureDoesNotPublishConfirmationMutation(
	t *testing.T,
) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer outbox.close()
	tx := testOutboxTransaction(12, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(14, 12, 1),
	)
	chain.setCanonicalStatus(tx.Hash(), &bitcoin.CanonicalTransactionStatus{
		Found:         true,
		Confirmations: 2,
		BlockHeight:   800001,
		BlockHash:     bitcoin.Hash{0xdd},
	})
	outbox.persistFailureHook = func(record *bitcoinBroadcastOutboxRecord) error {
		return errors.New("injected fsync failure")
	}
	if err := outbox.replayOnce(); err == nil {
		t.Fatal("expected injected confirmation persistence failure")
	}
	if outbox.records[tx.Hash()].Confirmation != nil {
		t.Fatal("non-durable confirmation leaked into live outbox state")
	}
	outbox.persistFailureHook = nil
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[tx.Hash()].Confirmation == nil {
		t.Fatal("confirmation was not retried after persistence recovered")
	}
	if chain.broadcastCount(tx.Hash()) != 0 {
		t.Fatal("confirmed transaction was broadcast before durable evidence")
	}
}

func TestBitcoinBroadcastOutbox_PersistenceFailureDoesNotPublishAttemptCounters(
	t *testing.T,
) {
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, t.TempDir(), chain)
	defer outbox.close()
	tx := testOutboxTransaction(13, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(15, 13, 1),
	)
	outbox.persistFailureHook = func(record *bitcoinBroadcastOutboxRecord) error {
		if record.BroadcastAttempts > 0 {
			return errors.New("injected attempt fsync failure")
		}
		return nil
	}
	if err := outbox.replayOnce(); err == nil {
		t.Fatal("expected injected attempt persistence failure")
	}
	if outbox.records[tx.Hash()].BroadcastAttempts != 0 {
		t.Fatal("non-durable attempt counters leaked into live outbox state")
	}
	if chain.broadcastCount(tx.Hash()) != 1 {
		t.Fatal("test did not reach the external Bitcoin broadcast boundary")
	}
	outbox.persistFailureHook = nil
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[tx.Hash()].BroadcastAttempts != 1 ||
		chain.broadcastCount(tx.Hash()) != 2 {
		t.Fatal("failed broadcast attempt was not safely retried")
	}
}

func TestBitcoinBroadcastOutbox_BroadcastAttemptTimestampsRemainMonotonicAcrossClockRollback(
	t *testing.T,
) {
	chain := newOutboxTestBitcoinChain()
	directory := t.TempDir()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	tx := testOutboxTransaction(34, 7000)
	currentTime := time.Unix(2_000, 0)
	outbox.now = func() time.Time {
		return currentTime
	}
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(35, 34, 1),
	)

	currentTime = time.Unix(2_100, 0)
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	currentTime = time.Unix(1_900, 0)
	if err := outbox.replayOnce(); err != nil {
		t.Fatalf("clock rollback broke durable replay: [%v]", err)
	}
	record := outbox.records[tx.Hash()]
	if record.BroadcastAttempts != 2 ||
		record.FirstBroadcastAtUnix != 2_100 ||
		record.LastAttemptUnix != 2_100 ||
		record.UpdatedAtUnix != 2_100 ||
		chain.broadcastCount(tx.Hash()) != 2 {
		t.Fatalf(
			"broadcast attempt timestamps regressed after clock rollback: %+v",
			record,
		)
	}
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestBitcoinBroadcastOutbox(t, directory, chain)
	defer restarted.close()
	if restarted.records[tx.Hash()].LastAttemptUnix != 2_100 {
		t.Fatal("monotonic broadcast attempt timestamp did not survive restart")
	}
}

func TestBitcoinBroadcastOutbox_PersistenceFailureDoesNotPublishQuarantine(
	t *testing.T,
) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	tx := testOutboxTransaction(14, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(16, 14, 1),
	)
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	statusSource := newOutboxTestAuthorizationStatusSource()
	statusSource.setBroadcastAllowed(false)
	rotatedProfile := [32]byte{0xed}
	restarted := openTestBitcoinBroadcastOutboxWithProfile(
		t,
		directory,
		chain,
		statusSource,
		rotatedProfile,
	)
	defer restarted.close()
	restarted.persistFailureHook = func(record *bitcoinBroadcastOutboxRecord) error {
		if record.Quarantine != nil {
			return errors.New("injected quarantine fsync failure")
		}
		return nil
	}
	if err := restarted.replayOnce(); err == nil {
		t.Fatal("expected injected quarantine persistence failure")
	}
	if restarted.records[tx.Hash()].Quarantine != nil {
		t.Fatal("non-durable quarantine leaked into live outbox state")
	}
	if chain.broadcastCount(tx.Hash()) != 0 {
		t.Fatal("failed quarantine mutation reached Bitcoin broadcast")
	}

	restarted.persistFailureHook = nil
	if err := restarted.replayOnce(); err != nil {
		t.Fatal(err)
	}
	quarantine := restarted.records[tx.Hash()].Quarantine
	if quarantine == nil ||
		quarantine.ActiveActivationProfileHash != rotatedProfile ||
		quarantine.ObservedAtUnix <= 0 {
		t.Fatal("quarantine was not retried after persistence recovered")
	}
}

func TestBitcoinBroadcastOutbox_CanonicalAuthorizationFailurePrecedesBroadcast(
	t *testing.T,
) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	statusSource := newOutboxTestAuthorizationStatusSource()
	outbox := openTestBitcoinBroadcastOutboxWithStatusSource(
		t,
		directory,
		chain,
		statusSource,
	)
	tx := testOutboxTransaction(14, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(16, 14, 1),
	)
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	statusSource.canonical = false
	restarted := openTestBitcoinBroadcastOutboxWithStatusSource(
		t,
		directory,
		chain,
		statusSource,
	)
	defer restarted.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := restarted.start(ctx); err == nil {
		t.Fatal("expected startup to fail on canonical authorization conflict")
	}
	if chain.broadcastCount(tx.Hash()) != 0 ||
		restarted.records[tx.Hash()].BroadcastAttempts != 0 {
		t.Fatal("authorization conflict reached Bitcoin broadcast")
	}
}

func TestBitcoinBroadcastOutbox_FirstBroadcastRequiresCurrentPermission(t *testing.T) {
	chain := newOutboxTestBitcoinChain()
	statusSource := newOutboxTestAuthorizationStatusSource()
	outbox := openTestBitcoinBroadcastOutboxWithStatusSource(
		t,
		t.TempDir(),
		chain,
		statusSource,
	)
	defer outbox.close()
	tx := testOutboxTransaction(18, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(19, 18, 1),
	)
	statusSource.broadcastAllowed = false
	if err := outbox.broadcastTransaction(context.Background(), tx.Hash()); err == nil {
		t.Fatal("expected current authorization to deny first broadcast")
	}
	if chain.broadcastCount(tx.Hash()) != 0 ||
		outbox.records[tx.Hash()].BroadcastAttempts != 0 {
		t.Fatal("denied first broadcast crossed the Bitcoin boundary")
	}
	statusSource.broadcastAllowed = true
	if err := outbox.broadcastTransaction(context.Background(), tx.Hash()); err != nil {
		t.Fatal(err)
	}
	if chain.broadcastCount(tx.Hash()) != 1 ||
		outbox.records[tx.Hash()].BroadcastAttempts != 1 {
		t.Fatal("authorized first broadcast did not durably record its attempt")
	}
}

func TestBitcoinBroadcastOutbox_ProfileRotationQuarantinesOldUnconfirmedRecord(
	t *testing.T,
) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	tx := testOutboxTransaction(15, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(17, 15, 1),
	)
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	rotatedProfile := [32]byte{0xee}
	statusSource := newOutboxTestAuthorizationStatusSource()
	statusSource.setBroadcastAllowed(false)
	restarted := openTestBitcoinBroadcastOutboxWithProfile(
		t,
		directory,
		chain,
		statusSource,
		rotatedProfile,
	)
	defer restarted.close()
	if err := restarted.replayOnce(); err != nil {
		t.Fatal(err)
	}
	record := restarted.records[tx.Hash()]
	if record == nil ||
		record.Authorization.ActivationProfileHash != testOutboxActivationProfile {
		t.Fatal("old-profile record was not preserved across rotation")
	}
	if record.Quarantine == nil ||
		record.Quarantine.ActiveActivationProfileHash != rotatedProfile {
		t.Fatal("old-profile record was not durably quarantined")
	}
	request := statusSource.lastStatusRequest()
	if request == nil ||
		request.ActivationProfileHash != testOutboxActivationProfile ||
		request.ActiveActivationProfileHash != rotatedProfile ||
		request.TransactionHash != tx.Hash() {
		t.Fatal("rotation status request did not bind old and active profiles")
	}
	if chain.canonicalStatusCallCount() == 0 {
		t.Fatal("rotation quarantine skipped canonical Bitcoin reconciliation")
	}
	if chain.broadcastCount(tx.Hash()) != 0 || record.BroadcastAttempts != 0 {
		t.Fatal("quarantined old-profile record crossed the Bitcoin boundary")
	}

	statusSource.setBroadcastAllowed(true)
	if err := restarted.replayOnce(); err != nil {
		t.Fatal(err)
	}
	record = restarted.records[tx.Hash()]
	if record.Quarantine != nil || record.BroadcastAttempts != 1 ||
		chain.broadcastCount(tx.Hash()) != 1 {
		t.Fatal("exact active-generation reauthorization did not release quarantine")
	}
}

func TestBitcoinBroadcastOutbox_ProfileRotationReconcilesOldConfirmation(
	t *testing.T,
) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	tx := testOutboxTransaction(16, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(18, 16, 1),
	)
	chain.setCanonicalStatus(tx.Hash(), &bitcoin.CanonicalTransactionStatus{
		Found:         true,
		Confirmations: defaultBitcoinBroadcastArchiveConfirmations,
		BlockHeight:   800016,
		BlockHash:     bitcoin.Hash{0x16, 0xcc},
	})
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if outbox.records[tx.Hash()].Confirmation == nil {
		t.Fatal("old-profile confirmation was not prepared")
	}
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	chain.setCanonicalStatus(tx.Hash(), &bitcoin.CanonicalTransactionStatus{
		Found:         true,
		Confirmations: defaultBitcoinBroadcastArchiveConfirmations + 1,
		BlockHeight:   800016,
		BlockHash:     bitcoin.Hash{0x16, 0xcc},
	})
	rotatedProfile := [32]byte{0xef}
	statusSource := newOutboxTestAuthorizationStatusSource()
	statusSource.setBroadcastAllowed(false)
	restarted := openTestBitcoinBroadcastOutboxWithProfile(
		t,
		directory,
		chain,
		statusSource,
		rotatedProfile,
	)
	defer restarted.close()
	if err := restarted.replayOnce(); err != nil {
		t.Fatal(err)
	}
	record := restarted.records[tx.Hash()]
	if record.Confirmation == nil ||
		record.Confirmation.Confirmations != defaultBitcoinBroadcastArchiveConfirmations+1 {
		t.Fatal("old-profile confirmation stopped reconciling after rotation")
	}
	if record.Quarantine == nil ||
		record.Quarantine.ActiveActivationProfileHash != rotatedProfile {
		t.Fatal("old confirmed record was not quarantined under the active profile")
	}
	if chain.broadcastCount(tx.Hash()) != 0 {
		t.Fatal("confirmed old-profile record was broadcast during rotation")
	}
}

func TestBitcoinBroadcastOutbox_ProfileRotationReconcilesOldReorg(
	t *testing.T,
) {
	directory := t.TempDir()
	chain := newOutboxTestBitcoinChain()
	outbox := openTestBitcoinBroadcastOutbox(t, directory, chain)
	tx := testOutboxTransaction(17, 7000)
	enqueueTestBitcoinTransaction(
		t,
		outbox,
		tx,
		testBitcoinBroadcastAuthorization(19, 17, 1),
	)
	chain.setCanonicalStatus(tx.Hash(), &bitcoin.CanonicalTransactionStatus{
		Found:         true,
		Confirmations: defaultBitcoinBroadcastArchiveConfirmations,
		BlockHeight:   800017,
		BlockHash:     bitcoin.Hash{0x17, 0xcc},
	})
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if err := outbox.close(); err != nil {
		t.Fatal(err)
	}

	chain.setCanonicalStatus(
		tx.Hash(),
		&bitcoin.CanonicalTransactionStatus{Found: false},
	)
	rotatedProfile := [32]byte{0xf0}
	statusSource := newOutboxTestAuthorizationStatusSource()
	statusSource.setBroadcastAllowed(false)
	restarted := openTestBitcoinBroadcastOutboxWithProfile(
		t,
		directory,
		chain,
		statusSource,
		rotatedProfile,
	)
	if err := restarted.replayOnce(); err != nil {
		restarted.close()
		t.Fatal(err)
	}
	record := restarted.records[tx.Hash()]
	if record.Confirmation != nil || record.Quarantine == nil {
		restarted.close()
		t.Fatal("old-profile reorg was not durably reconciled and quarantined")
	}
	if chain.broadcastCount(tx.Hash()) != 0 || record.BroadcastAttempts != 0 {
		restarted.close()
		t.Fatal("reorged old-profile record was broadcast without reauthorization")
	}
	if err := restarted.close(); err != nil {
		t.Fatal(err)
	}

	statusSource = newOutboxTestAuthorizationStatusSource()
	statusSource.setBroadcastAllowed(false)
	restarted = openTestBitcoinBroadcastOutboxWithProfile(
		t,
		directory,
		chain,
		statusSource,
		rotatedProfile,
	)
	defer restarted.close()
	if restarted.records[tx.Hash()].Confirmation != nil ||
		restarted.records[tx.Hash()].Quarantine == nil {
		t.Fatal("reorg quarantine was not durable across restart")
	}
	if err := restarted.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if chain.broadcastCount(tx.Hash()) != 0 {
		t.Fatal("persisted reorg quarantine was silently replayed")
	}
}

func TestBitcoinBroadcastOutbox_DeepReconciliationWorkIsBounded(t *testing.T) {
	chain := newOutboxTestBitcoinChain()
	statusSource := newOutboxTestAuthorizationStatusSource()
	outbox := openTestBitcoinBroadcastOutboxWithStatusSource(
		t,
		t.TempDir(),
		chain,
		statusSource,
	)
	defer outbox.close()
	outbox.deepReconcileBatch = 3

	const historySize = 24
	for i := 1; i <= historySize; i++ {
		tx := testOutboxTransaction(byte(i), 7000)
		enqueueTestBitcoinTransaction(
			t,
			outbox,
			tx,
			testBitcoinBroadcastAuthorization(byte(i), byte(i), 1),
		)
		chain.setCanonicalStatus(tx.Hash(), &bitcoin.CanonicalTransactionStatus{
			Found:         true,
			Confirmations: defaultBitcoinBroadcastArchiveConfirmations,
			BlockHeight:   uint(800100 + i),
			BlockHash:     bitcoin.Hash{byte(i), 0xaa},
		})
	}
	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	chain.resetStatusCallCount()
	statusSource.resetCallCount()

	if err := outbox.replayOnce(); err != nil {
		t.Fatal(err)
	}
	if chain.canonicalStatusCallCount() > uint(outbox.deepReconcileBatch) ||
		statusSource.callCount() > outbox.deepReconcileBatch {
		t.Fatalf(
			"archived replay work grew with history: bitcoin=[%d] ethereum=[%d] batch=[%d]",
			chain.canonicalStatusCallCount(),
			statusSource.callCount(),
			outbox.deepReconcileBatch,
		)
	}
}

func openTestBitcoinBroadcastOutbox(
	t *testing.T,
	directory string,
	chain canonicalBitcoinBroadcastChain,
) *bitcoinBroadcastOutbox {
	t.Helper()
	outbox, err := newTestBitcoinBroadcastOutbox(directory, chain)
	if err != nil {
		t.Fatal(err)
	}
	return outbox
}

func openTestBitcoinBroadcastOutboxWithStatusSource(
	t *testing.T,
	directory string,
	chain canonicalBitcoinBroadcastChain,
	statusSource FrostBitcoinBroadcastAuthorizationStatusSource,
) *bitcoinBroadcastOutbox {
	return openTestBitcoinBroadcastOutboxWithProfile(
		t,
		directory,
		chain,
		statusSource,
		testOutboxActivationProfile,
	)
}

func openTestBitcoinBroadcastOutboxWithProfile(
	t *testing.T,
	directory string,
	chain canonicalBitcoinBroadcastChain,
	statusSource FrostBitcoinBroadcastAuthorizationStatusSource,
	activationProfileHash [32]byte,
) *bitcoinBroadcastOutbox {
	t.Helper()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	outbox, err := newBitcoinBroadcastOutbox(
		directory,
		chain,
		statusSource,
		activationProfileHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	return outbox
}

func newTestBitcoinBroadcastOutbox(
	directory string,
	chain canonicalBitcoinBroadcastChain,
) (*bitcoinBroadcastOutbox, error) {
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	return newBitcoinBroadcastOutbox(
		directory,
		chain,
		newOutboxTestAuthorizationStatusSource(),
		testOutboxActivationProfile,
	)
}

func enqueueTestBitcoinTransaction(
	t *testing.T,
	outbox *bitcoinBroadcastOutbox,
	tx *bitcoin.Transaction,
	authorization bitcoinBroadcastAuthorization,
) {
	t.Helper()
	if err := enqueueTestBitcoinTransactionError(outbox, tx, authorization); err != nil {
		t.Fatal(err)
	}
}

func enqueueTestBitcoinTransactionError(
	outbox *bitcoinBroadcastOutbox,
	tx *bitcoin.Transaction,
	authorization bitcoinBroadcastAuthorization,
) error {
	return outbox.enqueue(
		tx,
		testOutboxWalletPublicKeyHash,
		testOutboxWalletID,
		FrostPreSignActionDepositSweep,
		tx.Hash(),
		authorization,
	)
}

func testBitcoinBroadcastAuthorization(
	authorizationByte byte,
	reservationByte byte,
	sequence uint64,
) bitcoinBroadcastAuthorization {
	sequenceWord := [32]byte{}
	binary.BigEndian.PutUint64(sequenceWord[24:], sequence)
	return bitcoinBroadcastAuthorization{
		ActivationProfileHash:     testOutboxActivationProfile,
		AuthorizationID:           [32]byte{authorizationByte},
		ReservationID:             [32]byte{reservationByte},
		AuthorizationRoot:         [32]byte{authorizationByte, reservationByte},
		SnapshotHash:              [32]byte{reservationByte, 0x01},
		ResourceHash:              [32]byte{reservationByte, 0x02},
		OrderedInputRoot:          [32]byte{reservationByte, 0x03},
		LockedPlanHash:            [32]byte{reservationByte, 0x04},
		VariantApplyPlanHash:      [32]byte{authorizationByte, 0x05},
		FeeLimitSnapshot:          10000,
		FinalizedBlock:            100 + sequence,
		FinalizedBlockHash:        [32]byte{reservationByte, byte(sequence)},
		FinalizedTransactionIndex: uint32(sequence),
		FinalizedLogIndex:         0,
		VariantSequence: FrostPreSignVariantSequence{
			AuthorizationSequence: sequenceWord,
		},
	}
}

func testBitcoinBroadcastOutboxRecord(
	tx *bitcoin.Transaction,
	authorization bitcoinBroadcastAuthorization,
) *bitcoinBroadcastOutboxRecord {
	outpoints, inputSetHash, err := bitcoinTransactionOutpoints(tx)
	if err != nil {
		panic(err)
	}
	now := time.Unix(1700000000, 0).Unix()
	return &bitcoinBroadcastOutboxRecord{
		Version:                 bitcoinBroadcastOutboxRecordVersion,
		TransactionHash:         tx.Hash(),
		WitnessTransactionHash:  tx.WitnessHash(),
		UnsignedTransactionHash: tx.Hash(),
		RawTransaction:          tx.Serialize(bitcoin.Witness),
		WalletPublicKeyHash:     testOutboxWalletPublicKeyHash,
		WalletID:                testOutboxWalletID,
		Action:                  FrostPreSignActionDepositSweep,
		OrderedOutpoints:        outpoints,
		InputSetHash:            inputSetHash,
		Authorization:           authorization,
		CreatedAtUnix:           now,
		UpdatedAtUnix:           now,
	}
}

func testOutboxTransaction(inputByte byte, outputValue int64) *bitcoin.Transaction {
	return &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{inputByte},
					OutputIndex:     1,
				},
				Witness:  [][]byte{{0x01, 0x02}},
				Sequence: 0xfffffffd,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           outputValue,
				PublicKeyScript: bitcoin.Script{0x51},
			},
		},
	}
}

type outboxTestBitcoinChain struct {
	*localBitcoinChain

	mutex         sync.Mutex
	statuses      map[bitcoin.Hash]*bitcoin.CanonicalTransactionStatus
	statusErrs    map[bitcoin.Hash]error
	broadcastErrs map[bitcoin.Hash]error
	broadcasts    map[bitcoin.Hash]uint
	statusCalls   uint
}

type blockingOutboxTestBitcoinChain struct {
	*outboxTestBitcoinChain
	statusStarted chan struct{}
	statusRelease <-chan struct{}
	statusOnce    sync.Once
}

type blockingBroadcastOutboxTestBitcoinChain struct {
	*outboxTestBitcoinChain
	broadcastStarted chan struct{}
	broadcastRelease <-chan struct{}
	broadcastOnce    sync.Once
}

func (bbotbc *blockingBroadcastOutboxTestBitcoinChain) BroadcastTransaction(
	tx *bitcoin.Transaction,
) error {
	bbotbc.broadcastOnce.Do(func() {
		close(bbotbc.broadcastStarted)
	})
	<-bbotbc.broadcastRelease
	return bbotbc.outboxTestBitcoinChain.BroadcastTransaction(tx)
}

func (botbc *blockingOutboxTestBitcoinChain) GetCanonicalTransactionStatus(
	hash bitcoin.Hash,
) (*bitcoin.CanonicalTransactionStatus, error) {
	botbc.statusOnce.Do(func() {
		close(botbc.statusStarted)
	})
	<-botbc.statusRelease
	return botbc.outboxTestBitcoinChain.GetCanonicalTransactionStatus(hash)
}

type outboxTestAuthorizationStatusSource struct {
	mutex            sync.Mutex
	err              error
	transactionErrs  map[bitcoin.Hash]error
	canonical        bool
	broadcastAllowed bool
	calls            int
	lastRequest      *FrostBitcoinBroadcastAuthorizationStatusRequest
}

func newOutboxTestAuthorizationStatusSource() *outboxTestAuthorizationStatusSource {
	return &outboxTestAuthorizationStatusSource{
		canonical:        true,
		broadcastAllowed: true,
		transactionErrs:  make(map[bitcoin.Hash]error),
	}
}

func (otass *outboxTestAuthorizationStatusSource) GetCanonicalFrostBitcoinBroadcastAuthorizationStatus(
	ctx context.Context,
	request *FrostBitcoinBroadcastAuthorizationStatusRequest,
) (*FrostBitcoinBroadcastAuthorizationStatus, error) {
	otass.mutex.Lock()
	defer otass.mutex.Unlock()
	otass.calls++
	requestClone := *request
	requestClone.OrderedOutpoints = append(
		[]FrostBitcoinBroadcastOutpoint{},
		request.OrderedOutpoints...,
	)
	otass.lastRequest = &requestClone
	if otass.err != nil {
		return nil, otass.err
	}
	if err := otass.transactionErrs[request.TransactionHash]; err != nil {
		return nil, err
	}
	return &FrostBitcoinBroadcastAuthorizationStatus{
		RequestHash:      request.ComputeHash(),
		Canonical:        otass.canonical,
		BroadcastAllowed: otass.broadcastAllowed,
	}, nil
}

func (otass *outboxTestAuthorizationStatusSource) setTransactionError(
	hash bitcoin.Hash,
	err error,
) {
	otass.mutex.Lock()
	defer otass.mutex.Unlock()
	otass.transactionErrs[hash] = err
}

func (otass *outboxTestAuthorizationStatusSource) setBroadcastAllowed(
	allowed bool,
) {
	otass.mutex.Lock()
	defer otass.mutex.Unlock()
	otass.broadcastAllowed = allowed
}

func (otass *outboxTestAuthorizationStatusSource) lastStatusRequest() *FrostBitcoinBroadcastAuthorizationStatusRequest {
	otass.mutex.Lock()
	defer otass.mutex.Unlock()
	if otass.lastRequest == nil {
		return nil
	}
	clone := *otass.lastRequest
	clone.OrderedOutpoints = append(
		[]FrostBitcoinBroadcastOutpoint{},
		otass.lastRequest.OrderedOutpoints...,
	)
	return &clone
}

func (otass *outboxTestAuthorizationStatusSource) callCount() int {
	otass.mutex.Lock()
	defer otass.mutex.Unlock()
	return otass.calls
}

func (otass *outboxTestAuthorizationStatusSource) resetCallCount() {
	otass.mutex.Lock()
	defer otass.mutex.Unlock()
	otass.calls = 0
}

func newOutboxTestBitcoinChain() *outboxTestBitcoinChain {
	return &outboxTestBitcoinChain{
		localBitcoinChain: newLocalBitcoinChain(),
		statuses:          make(map[bitcoin.Hash]*bitcoin.CanonicalTransactionStatus),
		statusErrs:        make(map[bitcoin.Hash]error),
		broadcastErrs:     make(map[bitcoin.Hash]error),
		broadcasts:        make(map[bitcoin.Hash]uint),
	}
}

func (otbc *outboxTestBitcoinChain) GetCanonicalTransactionStatus(
	hash bitcoin.Hash,
) (*bitcoin.CanonicalTransactionStatus, error) {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	otbc.statusCalls++
	if err := otbc.statusErrs[hash]; err != nil {
		return nil, err
	}
	status, ok := otbc.statuses[hash]
	if !ok {
		return &bitcoin.CanonicalTransactionStatus{Found: false}, nil
	}
	clone := *status
	return &clone, nil
}

func (otbc *outboxTestBitcoinChain) BroadcastTransaction(
	tx *bitcoin.Transaction,
) error {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	otbc.broadcasts[tx.Hash()]++
	return otbc.broadcastErrs[tx.Hash()]
}

func (otbc *outboxTestBitcoinChain) setCanonicalStatus(
	hash bitcoin.Hash,
	status *bitcoin.CanonicalTransactionStatus,
) {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	clone := *status
	otbc.statuses[hash] = &clone
}

func (otbc *outboxTestBitcoinChain) setBroadcastError(
	hash bitcoin.Hash,
	err error,
) {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	otbc.broadcastErrs[hash] = err
}

func (otbc *outboxTestBitcoinChain) setCanonicalError(
	hash bitcoin.Hash,
	err error,
) {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	otbc.statusErrs[hash] = err
}

func (otbc *outboxTestBitcoinChain) broadcastCount(hash bitcoin.Hash) uint {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	return otbc.broadcasts[hash]
}

func (otbc *outboxTestBitcoinChain) resetStatusCallCount() {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	otbc.statusCalls = 0
}

func (otbc *outboxTestBitcoinChain) canonicalStatusCallCount() uint {
	otbc.mutex.Lock()
	defer otbc.mutex.Unlock()
	return otbc.statusCalls
}

func (otbc *outboxTestBitcoinChain) GetTransactionConfirmations(
	hash bitcoin.Hash,
) (uint, error) {
	status, err := otbc.GetCanonicalTransactionStatus(hash)
	if err != nil {
		return 0, err
	}
	if !status.Found {
		return 0, fmt.Errorf("transaction not found")
	}
	return status.Confirmations, nil
}
