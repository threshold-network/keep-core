package encryption

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"
)

var accountPassword = []byte("passW0rd")

func TestEncryptDecrypt(t *testing.T) {
	msg := "Keep Calm and Carry On"

	box := NewBox(sha256.Sum256(accountPassword))

	encrypted, err := box.Encrypt([]byte(msg))
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := box.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	decryptedString := string(decrypted)
	if decryptedString != msg {
		t.Fatalf(
			"unexpected message\nexpected: %v\nactual: %v",
			msg,
			decryptedString,
		)
	}
}

func TestCiphertextRandomized(t *testing.T) {
	msg := `Why do we tell actors to 'break a leg?'
			 Because every play has a cast.`

	box := NewBox(sha256.Sum256(accountPassword))

	encrypted1, err := box.Encrypt([]byte(msg))
	if err != nil {
		t.Fatal(err)
	}

	encrypted2, err := box.Encrypt([]byte(msg))
	if err != nil {
		t.Fatal(err)
	}

	if len(encrypted1) != len(encrypted2) {
		t.Fatalf(
			"expected the same length of ciphertexts (%v vs %v)",
			len(encrypted1),
			len(encrypted2),
		)
	}

	if reflect.DeepEqual(encrypted1, encrypted2) {
		t.Fatalf("expected two different ciphertexts")
	}
}

func TestGracefullyHandleBrokenCipher(t *testing.T) {
	box := NewBox(sha256.Sum256(accountPassword))

	brokenCipher := []byte{0x01, 0x02, 0x03}

	_, err := box.Decrypt(brokenCipher)

	expectedError := fmt.Errorf("symmetric key decryption failed")
	if !reflect.DeepEqual(expectedError, err) {
		t.Fatalf(
			"unexpected error\nexpected: %v\nactual:   %v",
			expectedError,
			err,
		)
	}
}
