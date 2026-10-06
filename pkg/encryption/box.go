package encryption

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	// KeyLength represents the byte size of the key.
	KeyLength = 32

	// NonceSize represents the byte size of nonce for XSalsa20 cipher used for
	// encryption.
	NonceSize = 24
)

// box is used to encrypt and decrypt a plaintext.
type box struct {
	key [KeyLength]byte
}

// NewBox uses XSalsa20 and Poly1305 to encrypt and decrypt the plaintext
// with the key.
func NewBox(key [KeyLength]byte) Box {
	return &box{
		key: key,
	}
}

// Encrypt takes the input plaintext and uses XSalsa20 and Poly1305 to encrypt
// the plaintext with the key.
func (b *box) Encrypt(plaintext []byte) ([]byte, error) {
	// The nonce needs to be unique, but not secure. Therefore we include it
	// at the beginning of the ciphertext.
	var nonce [NonceSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, fmt.Errorf("key encryption failed [%v]", err)
	}

	return secretbox.Seal(nonce[:], plaintext, &nonce, &b.key), nil
}

// Decrypt takes the input ciphertext and decrypts it.
func (b *box) Decrypt(ciphertext []byte) (plaintext []byte, err error) {
	defer func() {
		// secretbox Open panics for invalid input
		if recover() != nil {
			err = errors.New("symmetric key decryption failed")
		}
	}()

	var nonce [NonceSize]byte
	copy(nonce[:], ciphertext[:NonceSize])

	plaintext, ok := secretbox.Open(nil, ciphertext[NonceSize:], &nonce, &b.key)
	if !ok {
		err = fmt.Errorf("symmetric key decryption failed")
	}

	return
}
