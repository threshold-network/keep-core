package persistence

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

var (
	dirCurrent  = "current"
	dirArchive  = "archive"
	dirSnapshot = "snapshot"

	dirName1   = "0x424242"
	fileName11 = "file11"
	fileName12 = "file12"

	dirName2   = "0x777777"
	fileName21 = "file21"

	fileContent = []byte{115, 111, 109, 101, 10}

	errExpectedRead  = fmt.Errorf("cannot read from the storage directory: ")
	errExpectedWrite = fmt.Errorf("cannot write to the storage directory: ")

	// 128 characters
	maxAllowedName = "0cc2abf49e067b3bede8426d9369c6952655d33629f44610536445c00c974c7e2740dfac967dbfceeeec0af88cd48a5d8c1c167df93cad1b8301a4a204c9f235"
	// 129 charactes
	notAllowedName = "0cc2abf49e067b3bede8426d9369c6952655d33629f44610536445c00c974c7e2740dfac967dbfceeeec0af88cd48a5d8c1c167df93cad1b8301a4a204c9f235a"

	errDirectoryNameLength = fmt.Errorf("the maximum directory name length of [128] exceeded for [%v]", notAllowedName)
	errFileNameLength      = fmt.Errorf("the maximum file name length of [128] exceeded for [%v]", notAllowedName)
)

func TestDiskPersistence_Save(t *testing.T) {
	var tests = map[string]struct {
		initDiskPersistenceFn func(t *testing.T) (RWHandle, string)
		expectedFilePath      string
	}{
		"basic disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initBasicDiskPersistence(t) },
			expectedFilePath:      filepath.Join(dirName1, fileName11),
		},
		"protected disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initProtectedDiskPersistence(t) },
			expectedFilePath:      filepath.Join(dirCurrent, dirName1, fileName11),
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			diskHandle, dataDir := test.initDiskPersistenceFn(t)

			if err := diskHandle.Save(fileContent, dirName1, fileName11); err != nil {
				t.Fatal(err)
			}

			assertExist(t, dataDir, test.expectedFilePath, "check file after save")
		})
	}
}

func TestDiskPersistence_SaveMaxAllowed(t *testing.T) {
	var tests = map[string]struct {
		initDiskPersistenceFn func(t *testing.T) (RWHandle, string)
		expectedFilePath      string
	}{
		"basic disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initBasicDiskPersistence(t) },
			expectedFilePath:      filepath.Join(maxAllowedName, maxAllowedName),
		},
		"protected disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initProtectedDiskPersistence(t) },
			expectedFilePath:      filepath.Join(dirCurrent, maxAllowedName, maxAllowedName),
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			diskHandle, dataDir := test.initDiskPersistenceFn(t)

			if err := diskHandle.Save(fileContent, maxAllowedName, maxAllowedName); err != nil {
				t.Fatal(err)
			}

			assertExist(t, dataDir, test.expectedFilePath, "check file after save")
		})
	}
}

func TestDiskPersistence_RefuseSave(t *testing.T) {
	var tests = map[string]struct {
		initDiskPersistenceFn func(t *testing.T) (RWHandle, string)
	}{
		"basic disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initBasicDiskPersistence(t) },
		},
		"protected disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initProtectedDiskPersistence(t) },
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			diskHandle, _ := test.initDiskPersistenceFn(t)

			err := diskHandle.Save(fileContent, notAllowedName, fileName11)
			if err == nil {
				t.Fatalf("expected error")
			}
			if errDirectoryNameLength.Error() != err.Error() {
				t.Fatalf(
					"unexpected error returned\nexpected: [%v]\nactual:   [%v]",
					errDirectoryNameLength.Error(),
					err.Error(),
				)
			}

			err = diskHandle.Save(fileContent, dirName1, notAllowedName)
			if err == nil {
				t.Fatalf("expected error")
			}
			if errFileNameLength.Error() != err.Error() {
				t.Fatalf(
					"unexpected error returned\nexpected: [%v]\nactual:   [%v]",
					errFileNameLength.Error(),
					err.Error(),
				)
			}
		})
	}
}

func TestProtectedDiskPersistence_Snapshot(t *testing.T) {
	diskHandle, dataDir := initProtectedDiskPersistence(t)

	counter := 0
	diskHandle.snapshotSuffixGenerator = func() string {
		counter++
		return fmt.Sprintf(".%d", counter)
	}

	for i := 0; i < 3; i++ {
		err := diskHandle.Snapshot(fileContent, dirName1, fileName11)
		if err != nil {
			t.Fatal(err)
		}
	}

	pathToFile := filepath.Join(dirSnapshot, dirName1, fileName11+".1")

	assertExist(t, dataDir, pathToFile, "check file 1 after snapshot")

	pathToFile = filepath.Join(dirSnapshot, dirName1, fileName11+".2")

	assertExist(t, dataDir, pathToFile, "check file 2 after snapshot")

	pathToFile = filepath.Join(dirSnapshot, dirName1, fileName11+".3")

	assertExist(t, dataDir, pathToFile, "check file 3 after snapshot")
}

func TestProtectedDiskPersistence_SnapshotMaxAllowed(t *testing.T) {
	diskHandle, dataDir := initProtectedDiskPersistence(t)

	snapshotSuffix := ".suffix"
	diskHandle.snapshotSuffixGenerator = func() string {
		return snapshotSuffix
	}

	maxAllowedSnapshotName := maxAllowedName[0 : len(maxAllowedName)-len(snapshotSuffix)]
	err := diskHandle.Snapshot(fileContent, maxAllowedName, maxAllowedSnapshotName)
	if err != nil {
		t.Fatal(err)
	}

	pathToFile := filepath.Join(
		dirSnapshot,
		maxAllowedName,
		maxAllowedSnapshotName+snapshotSuffix,
	)

	assertExist(t, dataDir, pathToFile, "check file after snapshot")
}

func TestProtectedDiskPersistence_RefuseSnapshot_MaxAllowedExceeded(t *testing.T) {
	diskHandle, _ := initProtectedDiskPersistence(t)

	snapshotSuffix := ".suffix"
	diskHandle.snapshotSuffixGenerator = func() string {
		return snapshotSuffix
	}

	err := diskHandle.Snapshot(fileContent, notAllowedName, fileName11)
	if err == nil {
		t.Fatalf("expected error")
	}

	if errDirectoryNameLength.Error() != err.Error() {
		t.Fatalf(
			"unexpected error returned\nexpected: [%v]\nactual:   [%v]",
			errDirectoryNameLength.Error(),
			err.Error(),
		)
	}

	err = diskHandle.Snapshot(fileContent, dirName1, notAllowedName)
	if err == nil {
		t.Fatalf("expected error")
	}

	errSnapshotFileNameLength := fmt.Errorf(
		"the maximum file name length of [%d] exceeded for [%v]",
		128-len(snapshotSuffix),
		notAllowedName,
	)

	if errSnapshotFileNameLength.Error() != err.Error() {
		t.Fatalf(
			"unexpected error returned\nexpected: [%v]\nactual:   [%v]",
			errFileNameLength.Error(),
			err.Error(),
		)
	}

}

func TestProtectedDiskPersistence_RefuseSnapshot_NameCollision(t *testing.T) {
	diskHandle, dataDir := initProtectedDiskPersistence(t)

	// snapshot suffix generator return always the same suffix in order to
	// cause name collision.
	snapshotSuffix := ".suffix"
	diskHandle.snapshotSuffixGenerator = func() string {
		return snapshotSuffix
	}

	err := diskHandle.Snapshot(fileContent, dirName1, fileName11)
	if err != nil {
		t.Fatal(err)
	}

	pathToFile := filepath.Join(
		dirSnapshot,
		dirName1,
		fileName11+snapshotSuffix,
	)

	assertExist(t, dataDir, pathToFile, "check file after snapshot")

	err = diskHandle.Snapshot(fileContent, dirName1, fileName11)

	expectedDuplicateError := fmt.Errorf(
		"could not create unique snapshot; " +
			"snapshot name collision has been detected",
	)
	if err == nil || (!reflect.DeepEqual(expectedDuplicateError, err)) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			expectedDuplicateError,
			err,
		)
	}
}

func TestDiskPersistence_StoragePermission(t *testing.T) {
	var tests = map[string]struct {
		newDiskPersistenceFn func(dataDir string) (RWHandle, error)
	}{
		"basic disk persistence": {
			newDiskPersistenceFn: func(dataDir string) (RWHandle, error) { return NewBasicDiskHandle(dataDir) },
		},
		"protected disk persistence": {
			newDiskPersistenceFn: func(dataDir string) (RWHandle, error) { return NewProtectedDiskHandle(dataDir) },
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tempDir := filepath.Join(t.TempDir(), "data_storage")

			err := os.Mkdir(tempDir, 0000) // d---------
			if _, err := os.Stat(tempDir); os.IsNotExist(err) {
				t.Fatalf("dir [%+v] was supposed to be created", tempDir)
			}

			_, err = test.newDiskPersistenceFn(tempDir)
			if err == nil || !strings.Contains(err.Error(), errExpectedRead.Error()) {
				t.Fatalf("error on read was supposed to be returned")
			}

			os.Chmod(tempDir, 0444) // dr--r--r

			_, err = NewProtectedDiskHandle(tempDir)
			if err == nil || !strings.Contains(err.Error(), errExpectedWrite.Error()) {
				t.Fatalf("error on write was supposed to be returned")
			}
		})
	}
}

func TestDiskPersistence_ReadAll(t *testing.T) {
	var tests = map[string]struct {
		initDiskPersistenceFn func(t *testing.T) (RWHandle, string)
		expectedFilePath      string
	}{
		"basic disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initBasicDiskPersistence(t) },
			expectedFilePath:      filepath.Join(maxAllowedName, maxAllowedName),
		},
		"protected disk persistence": {
			initDiskPersistenceFn: func(t *testing.T) (RWHandle, string) { return initProtectedDiskPersistence(t) },
			expectedFilePath:      filepath.Join(dirCurrent, maxAllowedName, maxAllowedName),
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			diskHandle, _ := test.initDiskPersistenceFn(t)

			expectedBytes := [][]byte{fileContent, fileContent, fileContent}

			diskHandle.Save(fileContent, dirName1, fileName11)
			diskHandle.Save(fileContent, dirName1, fileName12)
			diskHandle.Save(fileContent, dirName2, fileName21)

			dataChannel, errChannel := diskHandle.ReadAll()

			var descriptors []DataDescriptor
			var errors []error

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				for e := range errChannel {
					errors = append(errors, e)
				}
				wg.Done()
			}()

			go func() {
				for d := range dataChannel {
					descriptors = append(descriptors, d)
				}
				wg.Done()
			}()

			wg.Wait()

			for err := range errors {
				t.Fatal(err)
			}

			if len(descriptors) != 3 {
				t.Fatalf(
					"Number of descriptors does not match\nExpected: [%v]\nActual:   [%v]",
					3,
					len(descriptors),
				)
			}

			for i := 0; i < 3; i++ {
				fileContent, err := descriptors[i].Content()
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(expectedBytes[i], fileContent) {
					t.Errorf(
						"unexpected file content [%d]\nexpected: [%v]\nactual:   [%v]\n",
						i,
						expectedBytes[i],
						fileContent,
					)
				}
			}
		})
	}
}

func TestProtectedDiskPersistence_Archive(t *testing.T) {
	diskHandle, dataDir := initProtectedDiskPersistence(t)

	pathMoveFrom := filepath.Join(dirCurrent, dirName1)
	pathMoveTo := filepath.Join(dirArchive, dirName1)

	diskHandle.Save(fileContent, dirName1, fileName11)

	assertExist(t, dataDir, pathMoveFrom, "check path from before archive")
	assertNotExist(t, dataDir, pathMoveTo, "check path to before archive")

	diskHandle.Archive(dirName1)

	assertNotExist(t, dataDir, pathMoveFrom, "check path from after archive")
	assertExist(t, dataDir, pathMoveTo, "check path to after archive")

}

func TestProtectedDiskPersistence_ArchiveMaxAllowed(t *testing.T) {
	diskHandle, dataDir := initProtectedDiskPersistence(t)

	pathMoveFrom := filepath.Join(dirCurrent, maxAllowedName)
	pathMoveTo := filepath.Join(dirArchive, maxAllowedName)

	diskHandle.Save(fileContent, maxAllowedName, maxAllowedName)

	assertExist(t, dataDir, pathMoveFrom, "check path from before archive")
	assertNotExist(t, dataDir, pathMoveTo, "check path to before archive")

	err := diskHandle.Archive(maxAllowedName)
	if err != nil {
		t.Fatal(err)
	}

	assertNotExist(t, dataDir, pathMoveFrom, "check path from after archive")
	assertExist(t, dataDir, pathMoveTo, "check path to after archive")

}

func TestProtectedDiskPersistence_RefuseArchive(t *testing.T) {
	diskHandle, _ := initProtectedDiskPersistence(t)

	err := diskHandle.Archive(notAllowedName)
	if err == nil {
		t.Fatalf("expected error")
	}
	if errDirectoryNameLength.Error() != err.Error() {
		t.Fatalf(
			"unexpected error returned\nexpected: [%v]\nactual:   [%v]",
			errDirectoryNameLength.Error(),
			err.Error(),
		)
	}
}

func TestProtectedDiskPersistence_AppendToArchive(t *testing.T) {
	diskHandle, dataDir := initProtectedDiskPersistence(t)

	pathMoveFrom := filepath.Join(dataDir, dirCurrent, dirName1)
	pathMoveTo := filepath.Join(dataDir, dirArchive, dirName1)

	diskHandle.Save(fileContent, dirName1, fileName11)
	diskHandle.Save(fileContent, dirName1, fileName12)
	diskHandle.Archive(dirName1)

	diskHandle.Save(fileContent, dirName1, "/file13")
	diskHandle.Save(fileContent, dirName1, "/file14")
	diskHandle.Archive(dirName1)

	assertNotExist(t, dataDir, pathMoveFrom, "check path from after archive")

	files, _ := ioutil.ReadDir(pathMoveTo)
	if len(files) != 4 {
		t.Fatalf(
			"unexpected number of files\nexpected: [%v]\nactual:   [%v]",
			4,
			len(files),
		)
	}
}

func TestBasicDiskPersistence_Delete(t *testing.T) {
	diskHandle, dataDir := initBasicDiskPersistence(t)

	pathToDir := filepath.Join(dirName1)
	pathToFile := filepath.Join(pathToDir, fileName11)

	diskHandle.Save(fileContent, dirName1, fileName11)

	assertExist(t, dataDir, pathToDir, "check directory before delete")
	assertExist(t, dataDir, pathToFile, "check file before delete")

	if err := diskHandle.Delete(dirName1, fileName11); err != nil {
		t.Fatalf("unexpected error for Delete call: %v", err)
	}

	assertExist(t, dataDir, pathToDir, "check directory after delete")
	assertNotExist(t, dataDir, pathToFile, "check file after delete")

}

func TestBasicDiskPersistence_RefuseDelete(t *testing.T) {
	diskHandle, dataDir := initBasicDiskPersistence(t)

	expectedError := fmt.Errorf(
		"remove %s: no such file or directory",
		filepath.Join(dataDir, dirName1, fileName11),
	)

	err := diskHandle.Delete(dirName1, fileName11)
	if err == nil {
		t.Fatalf("expected error")
	}
	if expectedError.Error() != err.Error() {
		t.Fatalf(
			"unexpected error returned\nexpected: [%v]\nactual:   [%v]",
			expectedError.Error(),
			err.Error(),
		)
	}
}

func initBasicDiskPersistence(t *testing.T) (*basicDiskPersistence, string) {
	dataDir := t.TempDir()
	handle, err := NewBasicDiskHandle(dataDir)
	if err != nil {
		t.Fatalf("failed to initialize disk handle: %v", err)
	}
	return handle.(*basicDiskPersistence), dataDir
}
func initProtectedDiskPersistence(t *testing.T) (*protectedDiskPersistence, string) {
	dataDir := t.TempDir()
	handle, err := NewProtectedDiskHandle(dataDir)
	if err != nil {
		t.Fatalf("failed to initialize disk handle: %v", err)
	}
	return handle.(*protectedDiskPersistence), dataDir
}

func assertExist(t *testing.T, dataDir, path, message string) {
	_, err := os.Stat(filepath.Join(dataDir, path))
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("%s: path [%s] does not exist, but is expected to exist", message, path)
		}
		t.Fatalf("%s: unexpected error for path [%s]: %v", message, path, err)
	}
}

func assertNotExist(t *testing.T, dataDir, path, message string) {
	_, err := os.Stat(filepath.Join(dataDir, path))
	if err == nil {
		t.Fatalf("%s: path [%s] exist, but is expected to does not exist", message, path)
	}

	if os.IsNotExist(err) {
		return
	}
	t.Fatalf("%s: unexpected error for path [%s]: %v", message, path, err)
}
