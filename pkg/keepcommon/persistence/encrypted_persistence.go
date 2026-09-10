package persistence

import (
	"crypto/sha256"

	"github.com/keep-network/keep-common/pkg/encryption"
)

// KeyLength represents the byte size of the key.
const KeyLength = encryption.KeyLength

type encryptedPersistance[H RWHandle] struct {
	box      encryption.Box
	delegate H
}

type encryptedBasicPersistence struct {
	encryptedPersistance[BasicHandle]
}

type encryptedProtectedPersistence struct {
	encryptedPersistance[ProtectedHandle]
}

// NewEncryptedBasicPersistence creates an adapter for the disk persistence to store data
// in an encrypted format.
func NewEncryptedBasicPersistence(handle BasicHandle, password string) BasicHandle {
	return &encryptedBasicPersistence{
		encryptedPersistance: encryptedPersistance[BasicHandle]{
			box:      encryption.NewBox(sha256.Sum256([]byte(password))),
			delegate: handle,
		},
	}
}

// NewEncryptedProtectedPersistence creates an adapter for the disk persistence to store data
// in an encrypted format.
func NewEncryptedProtectedPersistence(handle ProtectedHandle, password string) ProtectedHandle {
	return &encryptedProtectedPersistence{
		encryptedPersistance[ProtectedHandle]{
			box:      encryption.NewBox(sha256.Sum256([]byte(password))),
			delegate: handle,
		},
	}
}

func (ep *encryptedPersistance[H]) Save(data []byte, directory string, name string) error {
	encrypted, err := ep.box.Encrypt(data)
	if err != nil {
		return err
	}

	return ep.delegate.Save(encrypted, directory, name)
}

func (ep *encryptedPersistance[H]) ReadAll() (<-chan DataDescriptor, <-chan error) {
	outputData := make(chan DataDescriptor)
	outputErrors := make(chan error)

	inputData, inputErrors := ep.delegate.ReadAll()

	// pass thru all errors from the input to the output channel without
	// changing anything
	go func() {
		defer close(outputErrors)
		for err := range inputErrors {
			outputErrors <- err
		}
	}()

	// pipe input data descriptor channel to the output data descriptor channel
	// decorading the descriptor passed so that the content is decrypted on read
	go func() {
		defer close(outputData)
		for descriptor := range inputData {
			// capture shared loop variable's value for the closure
			d := descriptor

			outputData <- &dataDescriptor{
				name:      d.Name(),
				directory: d.Directory(),
				readFunc: func() ([]byte, error) {
					content, err := d.Content()
					if err != nil {
						return nil, err
					}
					return ep.box.Decrypt(content)
				},
			}
		}
	}()

	return outputData, outputErrors
}

func (ep *encryptedBasicPersistence) Delete(directory string, name string) error {
	return ep.delegate.Delete(directory, name)
}

func (ep *encryptedProtectedPersistence) Archive(directory string) error {
	return ep.delegate.Archive(directory)
}

func (ep *encryptedProtectedPersistence) Snapshot(data []byte, directory string, name string) error {
	encrypted, err := ep.box.Encrypt(data)
	if err != nil {
		return err
	}

	return ep.delegate.Snapshot(encrypted, directory, name)
}
