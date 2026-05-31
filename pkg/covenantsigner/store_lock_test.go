package covenantsigner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keep-network/keep-common/pkg/persistence"
)

func TestNewStore_AcquiresFileLock(t *testing.T) {
	tempDir := t.TempDir()

	handle, err := persistence.NewBasicDiskHandle(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(handle, tempDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	lockPath := filepath.Join(tempDir, jobsDirectory, lockFileName)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatalf("expected lock file to exist at %s", lockPath)
	}
}

func TestNewStore_LockContention_ReturnsError(t *testing.T) {
	tempDir := t.TempDir()

	handle1, err := persistence.NewBasicDiskHandle(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	store1, err := NewStore(handle1, tempDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store1.Close() })

	handle2, err := persistence.NewBasicDiskHandle(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	store2, err := NewStore(handle2, tempDir)
	if store2 != nil {
		store2.Close()
		t.Fatal("expected second store to fail, but it succeeded")
	}
	if err == nil {
		t.Fatal("expected error when acquiring lock on already-locked directory")
	}
	if !strings.Contains(err.Error(), "lock") {
		t.Fatalf("expected error message to contain 'lock', got: %v", err)
	}
}

func TestStore_Close_ReleasesLock(t *testing.T) {
	tempDir := t.TempDir()

	handle1, err := persistence.NewBasicDiskHandle(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	store1, err := NewStore(handle1, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store1.Close(); err != nil {
		t.Fatalf("failed to close first store: %v", err)
	}

	handle2, err := persistence.NewBasicDiskHandle(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	store2, err := NewStore(handle2, tempDir)
	if err != nil {
		t.Fatalf("expected second store to succeed after first was closed, got: %v", err)
	}
	t.Cleanup(func() { store2.Close() })
}

func TestStore_Close_Idempotent(t *testing.T) {
	tempDir := t.TempDir()

	handle, err := persistence.NewBasicDiskHandle(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(handle, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}

	// Second close should not panic or return an error.
	if err := store.Close(); err != nil {
		t.Fatalf("second close should be a no-op, got: %v", err)
	}
}

func TestNewStore_InMemoryHandle_SkipsLock(t *testing.T) {
	handle := newMemoryHandle()

	store, err := NewStore(handle, "")
	if err != nil {
		t.Fatal(err)
	}

	// Close on a store without file locking should be a safe no-op.
	if err := store.Close(); err != nil {
		t.Fatalf("close on in-memory store should be a no-op, got: %v", err)
	}
}

func TestNewStore_SequentialOpenCloseOpen(t *testing.T) {
	tempDir := t.TempDir()

	for i := 0; i < 3; i++ {
		handle, err := persistence.NewBasicDiskHandle(tempDir)
		if err != nil {
			t.Fatalf("iteration %d: failed to create handle: %v", i, err)
		}

		store, err := NewStore(handle, tempDir)
		if err != nil {
			t.Fatalf("iteration %d: failed to open store: %v", i, err)
		}

		if err := store.Close(); err != nil {
			t.Fatalf("iteration %d: failed to close store: %v", i, err)
		}
	}
}
