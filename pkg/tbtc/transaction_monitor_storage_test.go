package tbtc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
)

func transactionMonitorDisk(t *testing.T) (persistence.BasicHandle, string) {
	t.Helper()
	path := t.TempDir()
	handle, err := persistence.NewBasicDiskHandle(path)
	if err != nil {
		t.Fatal(err)
	}
	return handle, path
}

func transactionMonitorFixture(broadcastAt time.Time, alerted bool) []byte {
	return []byte(fmt.Sprintf(
		`{"version":1,"wallet_public_key_hash":"0102030000000000000000000000000000000000","broadcast_at":%q,"alerted":%t}`,
		broadcastAt.Format(time.RFC3339Nano), alerted,
	))
}

func saveTransactionMonitorFixture(
	t *testing.T,
	handle persistence.BasicHandle,
	hash bitcoin.Hash,
	broadcastAt time.Time,
	alerted bool,
) {
	t.Helper()
	if err := handle.Save(
		transactionMonitorFixture(broadcastAt, alerted),
		transactionMonitorDirectory,
		transactionMonitorRecordName(hash, alerted),
	); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionMonitor_PersistsRegistration(t *testing.T) {
	handle, _ := transactionMonitorDisk(t)
	chain := newLocalBitcoinChain()
	monitor := newTransactionMonitor(chain, handle)
	hash := bitcoin.Hash{1, 2, 3}
	wallet := [20]byte{4, 5, 6}
	monitor.track(hash, wallet)
	original := monitor.snapshotByAge()[0]

	// Recovery is complete when construction returns; no check pass is needed.
	restarted := newTransactionMonitor(chain, handle)
	restarted.track(hash, [20]byte{99})
	if got := restarted.snapshotByAge(); len(got) != 1 ||
		got[0].hash != hash || got[0].walletPublicKeyHash != wallet ||
		!got[0].broadcastAt.Equal(original.broadcastAt) || got[0].alerted {
		t.Fatalf("registration changed across restart or duplicate tracking: %+v", got)
	}
}

func TestTransactionMonitor_RestartLifecycle(t *testing.T) {
	for _, test := range []struct {
		name        string
		age         time.Duration
		confirmed   bool
		alerted     bool
		wantAlerts  float64
		wantTracked bool
	}{
		{"fresh", time.Hour, false, false, 0, true},
		{"stuck during downtime", 7 * time.Hour, false, false, 1, true},
		{"confirmed during downtime", 7 * time.Hour, true, false, 0, false},
		{"expired during downtime", 25 * time.Hour, false, false, 1, false},
		{"already alerted", 7 * time.Hour, false, true, 0, true},
		{"already alerted and expired", 25 * time.Hour, false, true, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			handle, path := transactionMonitorDisk(t)
			chain := newLocalBitcoinChain()
			tx := &bitcoin.Transaction{}
			hash := tx.Hash()
			broadcastAt := time.Now().Add(-test.age)
			saveTransactionMonitorFixture(t, handle, hash, broadcastAt, false)
			if test.alerted {
				saveTransactionMonitorFixture(t, handle, hash, broadcastAt, true)
			}
			if test.confirmed {
				if err := chain.BroadcastTransaction(tx); err != nil {
					t.Fatal(err)
				}
			}

			monitor := newTransactionMonitor(chain, handle)
			got := monitor.snapshotByAge()
			if len(got) != 1 || !got[0].broadcastAt.Equal(broadcastAt) ||
				got[0].walletPublicKeyHash != [20]byte{1, 2, 3} ||
				got[0].alerted != test.alerted {
				t.Fatalf("unexpected restored transaction: %+v", got)
			}
			recorder := newCountingMetricsRecorder()
			monitor.setMetricsRecorder(recorder)
			monitor.check(context.Background())
			if got := recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal); got != test.wantAlerts {
				t.Fatalf("expected %v stuck alerts; got %v", test.wantAlerts, got)
			}
			if isTracked(monitor, hash) != test.wantTracked {
				t.Fatalf("expected tracked=%v after check", test.wantTracked)
			}

			// Alert and removal decisions must survive another restart as well.
			restarted := newTransactionMonitor(chain, handle)
			restarted.setMetricsRecorder(recorder)
			restarted.check(context.Background())
			if isTracked(restarted, hash) != test.wantTracked ||
				recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal) != test.wantAlerts {
				t.Fatal("second restart lost an entry or repeated an alert")
			}
			if !test.wantTracked {
				files, err := os.ReadDir(filepath.Join(path, transactionMonitorDirectory))
				if err != nil || len(files) != 0 {
					t.Fatalf("expected durable records to be removed; files=%v, err=%v", files, err)
				}
			}
		})
	}
}

type failingTransactionMonitorPersistence struct {
	persistence.BasicHandle
	failSave       bool
	failDeleteName string
}

func (p *failingTransactionMonitorPersistence) Save(data []byte, directory, name string) error {
	if p.failSave {
		// Emulate an interrupted in-place write, not just a failure to open.
		if err := p.BasicHandle.Save(data[:len(data)/2], directory, name); err != nil {
			return err
		}
		return errors.New("interrupted write")
	}
	return p.BasicHandle.Save(data, directory, name)
}

func (p *failingTransactionMonitorPersistence) Delete(directory, name string) error {
	if name == p.failDeleteName {
		return errors.New("delete failed")
	}
	return p.BasicHandle.Delete(directory, name)
}

func TestTransactionMonitor_RetriesFailedRegistration(t *testing.T) {
	handle, _ := transactionMonitorDisk(t)
	failing := &failingTransactionMonitorPersistence{BasicHandle: handle, failSave: true}
	chain := newLocalBitcoinChain()
	monitor := newTransactionMonitor(chain, failing)
	hash := bitcoin.Hash{1}
	monitor.track(hash, [20]byte{2})
	original := monitor.snapshotByAge()
	if len(original) != 1 || !original[0].dirty {
		t.Fatal("failed save must leave the transaction monitored and awaiting retry")
	}
	failing.failSave = false
	monitor.check(context.Background())
	restarted := newTransactionMonitor(chain, handle)
	got := restarted.snapshotByAge()
	if len(got) != 1 || !got[0].broadcastAt.Equal(original[0].broadcastAt) ||
		got[0].walletPublicKeyHash != original[0].walletPublicKeyHash {
		t.Fatalf("registration was not recovered after retry: %+v", got)
	}
}

func TestTransactionMonitor_PartialAlertWritePreservesRegistration(t *testing.T) {
	handle, _ := transactionMonitorDisk(t)
	chain := newLocalBitcoinChain()
	hash := bitcoin.Hash{1}
	broadcastAt := time.Now().Add(-7 * time.Hour)
	saveTransactionMonitorFixture(t, handle, hash, broadcastAt, false)
	failing := &failingTransactionMonitorPersistence{BasicHandle: handle, failSave: true}
	monitor := newTransactionMonitor(chain, failing)
	recorder := newCountingMetricsRecorder()
	monitor.setMetricsRecorder(recorder)
	monitor.check(context.Background())

	// A crash during the alert write must not destroy the initial record.
	restarted := newTransactionMonitor(chain, handle)
	got := restarted.snapshotByAge()
	if len(got) != 1 || !got[0].broadcastAt.Equal(broadcastAt) || got[0].alerted {
		t.Fatalf("partial alert write lost the original tracking state: %+v", got)
	}

	failing.failSave = false
	monitor.check(context.Background())
	if recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal) != 1 {
		t.Fatal("retrying persistence must not repeat the alert")
	}
	if got := newTransactionMonitor(chain, handle).snapshotByAge(); len(got) != 1 || !got[0].alerted {
		t.Fatalf("alert state was not persisted after retry: %+v", got)
	}
}

func TestTransactionMonitor_RetriesFailedDeletion(t *testing.T) {
	for _, failAlertedRecord := range []bool{false, true} {
		t.Run(fmt.Sprintf("alerted record=%v", failAlertedRecord), func(t *testing.T) {
			handle, path := transactionMonitorDisk(t)
			chain := newLocalBitcoinChain()
			hash := bitcoin.Hash{1}
			broadcastAt := time.Now().Add(-25 * time.Hour)
			saveTransactionMonitorFixture(t, handle, hash, broadcastAt, false)
			failing := &failingTransactionMonitorPersistence{
				BasicHandle:    handle,
				failDeleteName: transactionMonitorRecordName(hash, failAlertedRecord),
			}
			monitor := newTransactionMonitor(chain, failing)
			recorder := newCountingMetricsRecorder()
			monitor.setMetricsRecorder(recorder)
			monitor.check(context.Background())
			monitor.check(context.Background())
			got := monitor.snapshotByAge()
			if len(got) != 1 || !got[0].pendingRemoval ||
				recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal) != 1 {
				t.Fatal("failed deletion must be retained for retry without repeat alerts")
			}

			// Either deletion failure leaves a complete alerted record to recover.
			if got := newTransactionMonitor(chain, handle).snapshotByAge(); len(got) != 1 || !got[0].alerted {
				t.Fatalf("interrupted removal lost alert state: %+v", got)
			}
			failing.failDeleteName = ""
			monitor.check(context.Background())
			if trackedCount(monitor) != 0 || trackedCount(newTransactionMonitor(chain, handle)) != 0 {
				t.Fatal("deleted transaction was resurrected")
			}
			files, err := os.ReadDir(filepath.Join(path, transactionMonitorDirectory))
			if err != nil || len(files) != 0 {
				t.Fatalf("retry left durable records behind: files=%v, err=%v", files, err)
			}
		})
	}
}

type transactionMonitorReadFailure struct {
	persistence.BasicHandle
	descriptors []persistence.DataDescriptor
}

func (p *transactionMonitorReadFailure) ReadAll() (<-chan persistence.DataDescriptor, <-chan error) {
	descriptors := make(chan persistence.DataDescriptor)
	failures := make(chan error)
	go func() {
		defer close(descriptors)
		defer close(failures)
		failures <- errors.New("directory read failed")
		for _, descriptor := range p.descriptors {
			descriptors <- descriptor
		}
		failures <- errors.New("another directory read failed")
	}()
	return descriptors, failures
}

type unreadableTransactionMonitorDescriptor struct {
	mockDescriptor
	read bool
}

func (d *unreadableTransactionMonitorDescriptor) Content() ([]byte, error) {
	d.read = true
	return nil, errors.New("file read failed")
}

func TestTransactionMonitor_RestoresDespiteReadFailures(t *testing.T) {
	hash := bitcoin.Hash{1}
	name := transactionMonitorRecordName(hash, false)
	unrelated := &unreadableTransactionMonitorDescriptor{
		mockDescriptor: mockDescriptor{directory: "preparams", name: "pp_1"},
	}
	unreadable := &unreadableTransactionMonitorDescriptor{
		mockDescriptor: mockDescriptor{directory: transactionMonitorDirectory, name: name},
	}
	handle := &transactionMonitorReadFailure{
		descriptors: []persistence.DataDescriptor{unrelated, unreadable},
	}
	for _, content := range []string{
		`{`,
		`{"version":2}`,
		`{"version":1,"wallet_public_key_hash":"01","broadcast_at":"2026-01-01T00:00:00Z"}`,
		`{"version":1,"wallet_public_key_hash":"0102030000000000000000000000000000000000"}`,
		`{"version":1,"wallet_public_key_hash":"0102030000000000000000000000000000000000","broadcast_at":"2026-01-01T00:00:00Z","alerted":true}`,
	} {
		handle.descriptors = append(handle.descriptors, &mockDescriptor{
			directory: transactionMonitorDirectory, name: name, content: []byte(content),
		})
	}
	handle.descriptors = append(handle.descriptors,
		&mockDescriptor{directory: transactionMonitorDirectory, name: "invalid.json"},
		&mockDescriptor{
			directory: transactionMonitorDirectory, name: name,
			content: transactionMonitorFixture(time.Now(), false),
		},
	)
	monitor := newTransactionMonitor(newLocalBitcoinChain(), handle)
	if !isTracked(monitor, hash) || trackedCount(monitor) != 1 || unrelated.read || !unreadable.read {
		t.Fatal("valid monitor records were lost or unrelated work storage was read")
	}
}

func TestTransactionMonitor_RestoredCapacityBound(t *testing.T) {
	handle := &mockPersistenceHandle{}
	for i := 0; i <= transactionMonitorMaxTracked; i++ {
		hash := bitcoin.Hash{byte(i), byte(i >> 8)}
		saveTransactionMonitorFixture(t, handle, hash, time.Now(), false)
	}
	// An alert record encountered after reaching capacity must still update an
	// existing entry. The disk lifecycle test covers the opposite read order.
	saveTransactionMonitorFixture(t, handle, bitcoin.Hash{}, time.Now(), true)
	monitor := newTransactionMonitor(newLocalBitcoinChain(), handle)
	if trackedCount(monitor) != transactionMonitorMaxTracked || !monitor.tracked[bitcoin.Hash{}].alerted {
		t.Fatal("restoration exceeded capacity or lost a duplicate's alert state")
	}
}

func TestTransactionMonitor_PersistsConcurrentTracking(t *testing.T) {
	handle, _ := transactionMonitorDisk(t)
	blockedHash := bitcoin.Hash{99}
	chain := newBlockingTransactionConfirmationsChain(blockedHash)
	monitor := newTransactionMonitor(chain, handle)
	monitor.track(blockedHash, [20]byte{})
	defer close(chain.lookupRelease)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checked := make(chan struct{})
	go func() {
		monitor.check(ctx)
		close(checked)
	}()
	select {
	case <-chain.lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("check did not start")
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hash := bitcoin.Hash{byte(i % 8)}
			monitor.track(hash, [20]byte{1})
		}(i)
	}
	wg.Wait()
	cancel()
	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("check did not stop")
	}
	if got := trackedCount(newTransactionMonitor(chain, handle)); got != 9 {
		t.Fatalf("concurrent registrations were lost or duplicated: got %d", got)
	}
}
