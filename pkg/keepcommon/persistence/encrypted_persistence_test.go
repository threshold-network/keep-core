package persistence

import (
	"bytes"
	"sync"
	"testing"

	"crypto/sha256"

	"github.com/keep-network/keep-common/pkg/encryption"
)

const accountPassword = "grzeski"

var (
	dataToEncrypt1 = []byte{'b', 'o', 'l', 'e', 'k'}
	dataToEncrypt2 = []byte{'l', 'o', 'l', 'e', 'k'}
	dataToEncrypt  = [][]byte{dataToEncrypt1, dataToEncrypt2}
)

func TestSaveReadAndDecryptData(t *testing.T) {
	var tests = map[string]struct {
		initEncryptedPersistenceFn func() RWHandle
		expectedFilePath           string
	}{
		"basic encrypted persistence": {
			initEncryptedPersistenceFn: func() RWHandle {
				return NewEncryptedBasicPersistence(&delegatePersistenceMock{}, accountPassword)
			},
		},
		"protected encrypted persistence": {
			initEncryptedPersistenceFn: func() RWHandle {
				return NewEncryptedProtectedPersistence(&delegatePersistenceMock{}, accountPassword)
			},
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			encryptedPersistence := test.initEncryptedPersistenceFn()

			err := encryptedPersistence.Save(dataToEncrypt1, "dir1", "name1")
			if err != nil {
				t.Fatalf("Error occurred while saving data [%v]", err)
			}
			encryptedPersistence.Save(dataToEncrypt2, "dir2", "name2")
			if err != nil {
				t.Fatalf("Error occurred while saving data [%v]", err)
			}

			decryptedChan, errChan := encryptedPersistence.ReadAll()

			var decrypted [][]byte
			var errors []error

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				for err := range errChan {
					errors = append(errors, err)
				}
				wg.Done()
			}()

			go func() {
				for d := range decryptedChan {
					content, err := d.Content()
					if err != nil {
						errors = append(errors, err)
					}

					decrypted = append(decrypted, content)
				}
				wg.Done()
			}()

			wg.Wait()

			for err := range errors {
				t.Fatal(err)
			}

			if len(decrypted) != len(dataToEncrypt) {
				t.Fatalf(
					"Unexpected number of decrypted items\nExpected: [%v]\nActual:   [%v]",
					len(dataToEncrypt),
					len(decrypted),
				)
			}

			for i := 0; i < len(dataToEncrypt); i++ {
				if !bytes.Equal(dataToEncrypt[i], decrypted[i]) {
					t.Errorf(
						"unexpected decrypted item [%d]\nexpected: [%v]\nactual:   [%v]\n",
						i,
						dataToEncrypt[i],
						decrypted[i],
					)
				}
			}
		})
	}
}

type delegatePersistenceMock struct{}

func (dpm *delegatePersistenceMock) Save(data []byte, directory string, name string) error {
	// noop
	return nil
}

func (dpm *delegatePersistenceMock) Snapshot(data []byte, directory string, name string) error {
	// noop
	return nil
}

func (dpm *delegatePersistenceMock) ReadAll() (<-chan DataDescriptor, <-chan error) {
	encrypted := encryptData()

	outputData := make(chan DataDescriptor, 2)
	outputErrors := make(chan error)

	outputData <- &testDataDescriptor{"1", "dir", encrypted[0]}
	outputData <- &testDataDescriptor{"2", "dir", encrypted[1]}

	close(outputData)
	close(outputErrors)

	return outputData, outputErrors
}

func (dpm *delegatePersistenceMock) Archive(directory string) error {
	// noop
	return nil
}

func (dpm *delegatePersistenceMock) Delete(directory string, name string) error {
	// noop
	return nil
}

type testDataDescriptor struct {
	name      string
	directory string
	content   []byte
}

func (tdd *testDataDescriptor) Name() string {
	return tdd.name
}

func (tdd *testDataDescriptor) Directory() string {
	return tdd.directory
}

func (tdd *testDataDescriptor) Content() ([]byte, error) {
	return tdd.content, nil
}

func encryptData() [][]byte {
	passwordBytes := []byte(accountPassword)
	box := encryption.NewBox(sha256.Sum256(passwordBytes))

	encryptedData1, err := box.Encrypt(dataToEncrypt1)
	if err != nil {
		panic("Error occurred while encrypting data")
	}
	encryptedData2, err := box.Encrypt(dataToEncrypt2)
	if err != nil {
		panic("Error occurred while encrypting data")
	}

	return [][]byte{encryptedData1, encryptedData2}
}
