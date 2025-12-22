package storage

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/keep-network/keep-common/pkg/persistence"
)

// Config stores meta-info about keeping data on disk
type Config struct {
	// Path to the persistent storage directory on disk.
	Dir string
}

const (
	// The key store persistent storage directory is a place where the sensitive
	// data are stored. The greatest care should be taken to backup the directory
	// and never loose any data stored there.
	// Losing data stored in this directory is a serious protocol violation.
	keyStoreDirName = "keystore"
	// The work persistent storage directory keeps data that should persist
	// the client restart. Losing data stored in this directory may
	// lead to losing rewards as a result of inactivity but is not
	// a protocol violation.
	workDirName = "work"
)

// Storage is a disk persistent storage for the client.
type Storage struct {
	keystoreDir        string
	workDir            string
	encryptionPassword string
}

// Initialize initializes a disk storage with `keystore` and `work` directories.
// The provided `encryptionPassword` will be used to encrypt the work persisted
// to the storage.
func Initialize(config Config, encryptionPassword string) (Storage, error) {
	storage := Storage{}

	storageRootDir := filepath.Clean(config.Dir)

	// Ensure the parent directory exists before creating subdirectories
	if err := os.MkdirAll(storageRootDir, 0755); err != nil {
		return storage, fmt.Errorf(
			"cannot create storage root directory [%s]: [%w]",
		storageRootDir,
			err,
		)
	}

	// Create keystore directory
	keystoreDir := filepath.Join(storageRootDir, keyStoreDirName)
	if err := os.MkdirAll(keystoreDir, 0755); err != nil {
		return storage, fmt.Errorf(
			"cannot create storage directory for keystore: [%w]",
			err,
		)
	}
	storage.keystoreDir = keystoreDir

	// Create work directory
	workDir := filepath.Join(storageRootDir, workDirName)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return storage, fmt.Errorf(
			"cannot create storage directory for work: [%w]",
			err,
		)
	}
	storage.workDir = workDir

	storage.encryptionPassword = encryptionPassword

	return storage, nil
}

// InitializeKeyStorePersistence initializes a disk persistence under keystore parent.
func (s *Storage) InitializeKeyStorePersistence(dir string) (
	persistence.ProtectedHandle,
	error,
) {
	return s.initializeKeyStorePersistence(s.keystoreDir, dir)
}

// InitializeWorkPersistence initializes a disk persistence under work parent.
func (s *Storage) InitializeWorkPersistence(dir string) (
	persistence.BasicHandle,
	error,
) {
	return s.initializeWorkPersistence(s.workDir, dir)
}

// initializeKeyStorePersistence creates a persistent directory under a parent directory.
// It returns an error is the parent directory doesn't exist.
func (s *Storage) initializeKeyStorePersistence(parentDir string, dir string) (
	persistence.ProtectedHandle,
	error,
) {
	dirPath := filepath.Join(parentDir, dir)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf(
			"cannot create storage directory [%s] in [%s]: [%w]",
			dir,
			parentDir,
			err,
		)
	}

	path := path.Join(parentDir, dir)

	diskHandle, err := persistence.NewProtectedDiskHandle(path)
	if err != nil {
		return nil, fmt.Errorf("cannot create [%s] disk handle: [%w]", path, err)
	}

	return persistence.NewEncryptedProtectedPersistence(
		diskHandle,
		s.encryptionPassword,
	), nil
}

// initializeWorkPersistence creates a persistent directory under a parent directory.
// It returns an error is the parent directory doesn't exist.
func (s *Storage) initializeWorkPersistence(parentDir string, dir string) (
	persistence.BasicHandle,
	error,
) {
	dirPath := filepath.Join(parentDir, dir)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf(
			"cannot create storage directory [%s] in [%s]: [%w]",
			dir,
			parentDir,
			err,
		)
	}

	path := path.Join(parentDir, dir)

	diskHandle, err := persistence.NewBasicDiskHandle(path)
	if err != nil {
		return nil, fmt.Errorf("cannot create [%s] disk handle: [%w]", path, err)
	}

	return persistence.NewEncryptedBasicPersistence(
		diskHandle,
		s.encryptionPassword,
	), nil
}
