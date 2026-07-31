package covenantsigner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/keep-network/keep-common/pkg/persistence"
)

func newDiskTestJob(suffix string) *Job {
	return &Job{
		RequestID:       "kcs_self_" + suffix,
		RouteRequestID:  "ors_" + suffix,
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_" + suffix,
		FacadeRequestID: "rf_" + suffix,
		RequestDigest:   "0xdeadbeef",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}
}

// TestStoreReloadPreservesJobsOnDisk exercises the store against a real
// disk-backed persistence handle (not the in-memory handle used elsewhere).
// The disk handle creates and enumerates only a single directory level, so a
// nested jobs directory name would make persisted jobs unrecoverable after a
// restart. This test persists a job, restarts the store over the same data
// directory, and asserts both the request ID and route request indexes are
// restored.
func TestStoreReloadPreservesJobsOnDisk(t *testing.T) {
	dataDir := t.TempDir()

	handle, err := persistence.NewBasicDiskHandle(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(handle, dataDir)
	if err != nil {
		t.Fatal(err)
	}

	job := newDiskTestJob("disk")
	if err := store.Put(job); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// Restart the store over the same data directory.
	reloadHandle, err := persistence.NewBasicDiskHandle(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(reloadHandle, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reloaded.Close() })

	// The route request index must be restored.
	byRoute, ok, err := reloaded.GetByRouteRequest(TemplateSelfV1, job.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the route request index to be restored after restart")
	}
	if byRoute.RequestID != job.RequestID {
		t.Fatalf("unexpected request ID from route index: %s", byRoute.RequestID)
	}

	// The request ID index must be restored.
	byID, ok, err := reloaded.GetByRequestID(job.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the request ID index to be restored after restart")
	}
	if byID.RouteRequestID != job.RouteRequestID {
		t.Fatalf("unexpected route request ID from request index: %s", byID.RouteRequestID)
	}
}

// TestStoreMigratesLegacyNestedJobs asserts that a job file left under the
// previously used nested directory is migrated to the flat jobs directory on
// startup and becomes reloadable.
func TestStoreMigratesLegacyNestedJobs(t *testing.T) {
	dataDir := t.TempDir()

	// Simulate a job persisted under the legacy nested directory.
	legacyDir := filepath.Join(dataDir, legacyJobsDirectory)
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatal(err)
	}
	job := newDiskTestJob("legacy")
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDir, job.RequestID+".json")
	if err := os.WriteFile(legacyFile, payload, 0600); err != nil {
		t.Fatal(err)
	}

	handle, err := persistence.NewBasicDiskHandle(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(handle, dataDir) // triggers migration and load
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	// The migrated job must be restored in both indexes.
	if _, ok, err := store.GetByRequestID(job.RequestID); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("expected the legacy job to be migrated and restored")
	}
	if _, ok, err := store.GetByRouteRequest(TemplateSelfV1, job.RouteRequestID); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("expected the legacy route request index to be restored")
	}

	// The file must now live under the flat jobs directory.
	newFile := filepath.Join(dataDir, jobsDirectory, job.RequestID+".json")
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("expected migrated file at %s: %v", newFile, err)
	}
	// The legacy file must no longer be present.
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Fatalf("expected legacy file at %s to be moved away", legacyFile)
	}
}
