package persistence

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	currentDir  = "current"
	archiveDir  = "archive"
	snapshotDir = "snapshot"

	maxFileNameLength = 128
)

type basicDiskPersistence struct {
	dataDir string
}

type protectedDiskPersistence struct {
	dataDir string

	snapshotMutex           sync.Mutex
	snapshotSuffixGenerator func() string
}

// NewBasicDiskHandle creates on-disk data persistence handle
func NewBasicDiskHandle(path string) (BasicHandle, error) {
	if err := CheckStoragePermission(path); err != nil {
		return nil, err
	}

	return &basicDiskPersistence{path}, nil
}

// NewProtectedDiskHandle creates on-disk data persistence handle
func NewProtectedDiskHandle(path string) (ProtectedHandle, error) {
	if err := CheckStoragePermission(path); err != nil {
		return nil, err
	}

	if err := EnsureDirectoryExists(path, currentDir); err != nil {
		return nil, err
	}

	if err := EnsureDirectoryExists(path, archiveDir); err != nil {
		return nil, err
	}

	if err := EnsureDirectoryExists(path, snapshotDir); err != nil {
		return nil, err
	}

	snapshotSuffixGenerator := func() string {
		timestamp := time.Now().UnixMilli()
		return fmt.Sprintf(".%d", timestamp)
	}

	return &protectedDiskPersistence{
		path,
		sync.Mutex{},
		snapshotSuffixGenerator,
	}, nil
}

func (ds *basicDiskPersistence) currentDirPath() string {
	return filepath.Clean(ds.dataDir)
}

func (ds *protectedDiskPersistence) currentDirPath() string {
	return filepath.Join(ds.dataDir, currentDir)
}

func (ds *basicDiskPersistence) Save(data []byte, dirName, fileName string) error {
	return save(ds.currentDirPath(), data, dirName, fileName)
}

func (ds *protectedDiskPersistence) Save(data []byte, dirName, fileName string) error {
	return save(ds.currentDirPath(), data, dirName, fileName)
}

func save(directoryPath string, data []byte, dirName, fileName string) error {
	if len(dirName) > maxFileNameLength {
		return fmt.Errorf(
			"the maximum directory name length of [%v] exceeded for [%v]",
			maxFileNameLength,
			dirName,
		)
	}

	if len(fileName) > maxFileNameLength {
		return fmt.Errorf(
			"the maximum file name length of [%v] exceeded for [%v]",
			maxFileNameLength,
			fileName,
		)
	}

	err := EnsureDirectoryExists(directoryPath, dirName)
	if err != nil {
		return err
	}

	return Write(filepath.Join(directoryPath, dirName, fileName), data)
}

func (ds *basicDiskPersistence) ReadAll() (<-chan DataDescriptor, <-chan error) {
	return readAll(ds.currentDirPath())
}

func (ds *protectedDiskPersistence) ReadAll() (<-chan DataDescriptor, <-chan error) {
	return readAll(ds.currentDirPath())
}

func (ds *basicDiskPersistence) Delete(dirName string, fileName string) error {
	dirPath := ds.currentDirPath()
	filePath := filepath.Join(dirPath, dirName, fileName)

	return remove(filePath)
}

func (ds *protectedDiskPersistence) Snapshot(data []byte, dirName, fileName string) error {
	if len(dirName) > maxFileNameLength {
		return fmt.Errorf(
			"the maximum directory name length of [%v] exceeded for [%v]",
			maxFileNameLength,
			dirName,
		)
	}

	snapshotSuffix := ds.snapshotSuffixGenerator()

	maxSnapshotFileNameLength := maxFileNameLength - len(snapshotSuffix)
	if len(fileName) > maxSnapshotFileNameLength {
		return fmt.Errorf(
			"the maximum file name length of [%v] exceeded for [%v]",
			maxSnapshotFileNameLength,
			fileName,
		)
	}

	ds.snapshotMutex.Lock()
	defer ds.snapshotMutex.Unlock()

	dirPath := filepath.Join(ds.dataDir, snapshotDir)
	err := EnsureDirectoryExists(dirPath, dirName)
	if err != nil {
		return err
	}

	filePath := filepath.Join(dirPath, dirName, fileName+snapshotSuffix)

	// very unlikely but better fail than overwrite an existing file
	if !isNonExistingFile(filePath) {
		return fmt.Errorf(
			"could not create unique snapshot; " +
				"snapshot name collision has been detected",
		)
	}

	return Write(filePath, data)
}

func isNonExistingFile(filePath string) bool {
	_, err := os.Stat(filePath)
	return os.IsNotExist(err)
}

func (ds *protectedDiskPersistence) Archive(directory string) error {
	if len(directory) > maxFileNameLength {
		return fmt.Errorf(
			"the maximum directory name length of [%v] exceeded for [%v]",
			maxFileNameLength,
			directory,
		)
	}

	from := filepath.Join(ds.dataDir, currentDir, directory)
	to := filepath.Join(ds.dataDir, archiveDir, directory)

	return moveAll(from, to)
}

// CheckStoragePermission returns an error if we don't have both read and write access to a directory.
func CheckStoragePermission(dirBasePath string) error {
	_, err := ioutil.ReadDir(dirBasePath)
	if err != nil {
		return fmt.Errorf("cannot read from the storage directory: [%v]", err)
	}

	tempFile, err := ioutil.TempFile(dirBasePath, "write-test.*.tmp")
	if err != nil {
		return fmt.Errorf("cannot write to the storage directory: [%v]", err)
	}

	defer os.RemoveAll(tempFile.Name())

	return nil
}

// EnsureDirectoryExists creates a new directory in a base path if it doesn't
// exist, returns nil if it does, and errors out if the base path doesn't exist.
func EnsureDirectoryExists(dirBasePath, newDirName string) error {
	dirPath := filepath.Join(dirBasePath, newDirName)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err = os.Mkdir(dirPath, 0o750)
		if err != nil {
			return fmt.Errorf(
				"error occurred while creating a dir: [%w]; "+
					"please make sure the parent directory [%s] exists",
				err,
				dirBasePath,
			)
		}
	}

	return nil
}

// Write creates and writes data to a file
func Write(filePath string, data []byte) error {
	writeFile, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return err
	}

	defer closeFile(writeFile)

	_, err = writeFile.Write(data)
	if err != nil {
		return err
	}

	err = writeFile.Sync()
	if err != nil {
		return err
	}

	return nil
}

// Read a file from a file system
func Read(filePath string) ([]byte, error) {
	// #nosec G304 (file path provided as taint input)
	// This line opens a file from the predefined storage.
	// There is no user input.
	readFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer closeFile(readFile)

	data, err := ioutil.ReadAll(readFile)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// remove a file from a file system
func remove(filePath string) error {
	return os.Remove(filePath)
}

func closeFile(file *os.File) {
	err := file.Close()
	if err != nil {
		logger.Errorf("could not close file [%v]: [%v]", file.Name(), err)
	}
}

// readAll reads all files from the provided directoryPath and outputs them
// as DataDescriptors into the first returned output channel. All errors
// occurred during file system reading are sent to the second output channel
// returned from this function. The output can be later processed using
// pipeline pattern. This function is non-blocking and returned channels are
// not buffered. Channels are closed when there is no more to be read.
func readAll(directoryPath string) (<-chan DataDescriptor, <-chan error) {
	dataChannel := make(chan DataDescriptor)
	errorChannel := make(chan error)

	go func() {
		defer close(dataChannel)
		defer close(errorChannel)

		files, err := ioutil.ReadDir(directoryPath)
		if err != nil {
			errorChannel <- fmt.Errorf(
				"could not read the directory [%v]: [%v]",
				directoryPath,
				err,
			)
		}

		for _, file := range files {
			if file.IsDir() {
				dir, err := ioutil.ReadDir(filepath.Join(directoryPath, file.Name()))
				if err != nil {
					errorChannel <- fmt.Errorf(
						"could not read the directory [%s/%s]: [%v]",
						directoryPath,
						file.Name(),
						err,
					)
				}

				for _, dirFile := range dir {
					// capture shared loop variables for the closure
					dirName := file.Name()
					fileName := dirFile.Name()

					readFunc := func() ([]byte, error) {
						return Read(filepath.Join(
							directoryPath,
							dirName,
							fileName,
						))
					}
					dataChannel <- &dataDescriptor{fileName, dirName, readFunc}
				}
			}
		}
	}()

	return dataChannel, errorChannel
}

func moveAll(directoryFromPath, directoryToPath string) error {
	_, err := os.Stat(directoryToPath)

	// target directory does not exist, we can move everything
	if os.IsNotExist(err) {
		err := os.Rename(directoryFromPath, directoryToPath)
		if err != nil {
			return fmt.Errorf("error occurred while moving a dir: [%v]", err)
		}

		return nil
	}

	// unexpected error occurred while checking target directory existence,
	// returning
	if err != nil {
		return fmt.Errorf("could not stat target directory: [%v]", err)
	}

	// target directory does exit, we need to append files
	files, err := ioutil.ReadDir(directoryFromPath)
	if err != nil {
		return fmt.Errorf("could not read directory [%v]: [%v]", directoryFromPath, err)
	}
	for _, file := range files {
		from := filepath.Join(directoryFromPath, file.Name())
		to := filepath.Join(directoryToPath, file.Name())
		err := os.Rename(from, to)
		if err != nil {
			return err
		}
	}
	err = os.RemoveAll(directoryFromPath)
	if err != nil {
		return fmt.Errorf("error occurred while removing archived dir: [%v]", err)
	}

	return nil
}
