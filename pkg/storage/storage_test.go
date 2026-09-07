package storage

import (
	"os"
	"path/filepath"
	"testing"
)

const testPassword = "grzegorz-brzeczyszczykiewicz"

// assertIsDir fails the test unless path exists and is a directory.
func assertIsDir(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory [%s] to exist: [%v]", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected [%s] to be a directory", path)
	}
}

// assertDoesNotExist fails the test unless path is absent.
func assertDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected [%s] not to exist, stat error was [%v]", path, err)
	}
}

// Initialize must lay out the two sibling directories that give the package its
// reason to exist: `keystore` for sensitive data whose loss is "a serious
// protocol violation", and `work` for restart-recoverable data. If either is
// missing or misplaced the separation guarantee is broken.
func TestInitialize_CreatesKeystoreAndWorkDirectories(t *testing.T) {
	root := t.TempDir()

	if _, err := Initialize(Config{Dir: root}, testPassword); err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	assertIsDir(t, filepath.Join(root, keyStoreDirName))
	assertIsDir(t, filepath.Join(root, workDirName))
}

// A client restart re-runs Initialize against directories that already exist.
// EnsureDirectoryExists no-ops on an existing directory, so this must succeed
// rather than error on the second boot.
func TestInitialize_IsIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, err := Initialize(Config{Dir: root}, testPassword); err != nil {
		t.Fatalf("first Initialize failed: [%v]", err)
	}
	if _, err := Initialize(Config{Dir: root}, testPassword); err != nil {
		t.Fatalf("second Initialize on existing directories failed: [%v]", err)
	}
}

// EnsureDirectoryExists creates a single directory level (os.Mkdir, not
// MkdirAll), so a storage root whose parent does not exist cannot be created.
// Initialize must surface that as an error rather than silently persisting to
// the wrong place - persisting keys to an unexpected location is exactly the
// failure the package is meant to prevent.
func TestInitialize_ErrorsWhenRootParentMissing(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "does-not-exist", "root")

	if _, err := Initialize(Config{Dir: missingRoot}, testPassword); err == nil {
		t.Fatal("expected an error when the storage root parent is missing")
	}
}

// InitializeKeyStorePersistence must root the handle under the keystore
// directory and nowhere else; leaking key material into the work tree would
// defeat the sensitive-data separation.
func TestInitializeKeyStorePersistence_RootsUnderKeystore(t *testing.T) {
	root := t.TempDir()
	storage, err := Initialize(Config{Dir: root}, testPassword)
	if err != nil {
		t.Fatalf("Initialize failed: [%v]", err)
	}

	handle, err := storage.InitializeKeyStorePersistence("signer")
	if err != nil {
		t.Fatalf("InitializeKeyStorePersistence failed: [%v]", err)
	}
	if handle == nil {
		t.Fatal("expected a non-nil key store persistence handle")
	}

	assertIsDir(t, filepath.Join(root, keyStoreDirName, "signer"))
	assertDoesNotExist(t, filepath.Join(root, workDirName, "signer"))
}

// InitializeWorkPersistence is the mirror of the keystore case: the handle must
// be rooted under the work directory and must not appear under keystore.
func TestInitializeWorkPersistence_RootsUnderWork(t *testing.T) {
	root := t.TempDir()
	storage, err := Initialize(Config{Dir: root}, testPassword)
	if err != nil {
		t.Fatalf("Initialize failed: [%v]", err)
	}

	handle, err := storage.InitializeWorkPersistence("beacon")
	if err != nil {
		t.Fatalf("InitializeWorkPersistence failed: [%v]", err)
	}
	if handle == nil {
		t.Fatal("expected a non-nil work persistence handle")
	}

	assertIsDir(t, filepath.Join(root, workDirName, "beacon"))
	assertDoesNotExist(t, filepath.Join(root, keyStoreDirName, "beacon"))
}

// A nested target whose intermediate parent does not exist must error rather
// than partially succeed, again because EnsureDirectoryExists only creates one
// directory level.
func TestInitializePersistence_ErrorsWhenIntermediateDirMissing(t *testing.T) {
	root := t.TempDir()
	storage, err := Initialize(Config{Dir: root}, testPassword)
	if err != nil {
		t.Fatalf("Initialize failed: [%v]", err)
	}

	nested := filepath.Join("missing", "leaf")

	if _, err := storage.InitializeKeyStorePersistence(nested); err == nil {
		t.Error("expected an error for a key store dir with a missing intermediate parent")
	}
	if _, err := storage.InitializeWorkPersistence(nested); err == nil {
		t.Error("expected an error for a work dir with a missing intermediate parent")
	}
}
