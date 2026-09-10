// Package encryption adds a layer to store data on disk encrypted in case 
// a filesystem is compromised.
// Under the hood we use "golang.org/x/crypto/nacl/secretbox" for encryption.
// Secretbox uses XSalsa20 and Poly1305 to encrypt an array of bytes with
// secret-key cryptography.
package encryption

// Box is a general interface to encrypt and decrypt an array of bytes.
type Box interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}
