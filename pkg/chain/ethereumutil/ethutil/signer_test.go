package ethutil

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestSignAndVerify(t *testing.T) {
	signer, err := newSigner()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("He that breaks a thing to find out what it is, has " +
		"left the path of wisdom.")

	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		message                 []byte
		signature               []byte
		validSignatureExpected  bool
		validationErrorExpected bool
	}{
		"valid signature for message": {
			message:                 message,
			signature:               signature,
			validSignatureExpected:  true,
			validationErrorExpected: false,
		},
		"invalid signature for message": {
			message:                 []byte("I am sorry"),
			signature:               signature,
			validSignatureExpected:  false,
			validationErrorExpected: false,
		},
		"corrupted signature": {
			message:                 message,
			signature:               []byte("I am so sorry"),
			validSignatureExpected:  false,
			validationErrorExpected: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ok, err := signer.Verify(test.message, test.signature)

			if !ok && test.validSignatureExpected {
				t.Errorf("expected valid signature but verification failed")
			}
			if ok && !test.validSignatureExpected {
				t.Errorf("expected invalid signature but verification succeeded")
			}

			if err == nil && test.validationErrorExpected {
				t.Errorf("expected signature validation error; none happened")
			}
			if err != nil && !test.validationErrorExpected {
				t.Errorf("unexpected signature validation error [%v]", err)
			}
		})
	}
}

func TestSignAndVerifyWithProvidedPublicKey(t *testing.T) {
	message := []byte("I am looking for someone to share in an adventure")

	signer1, err := newSigner()
	if err != nil {
		t.Fatal(err)
	}

	signer2, err := newSigner()
	if err != nil {
		t.Fatal(err)
	}

	publicKey := signer1.PublicKey()
	signature, err := signer1.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		message                 []byte
		signature               []byte
		publicKey               []byte
		validSignatureExpected  bool
		validationErrorExpected bool
	}{
		"valid signature for message": {
			message:                 message,
			signature:               signature,
			publicKey:               publicKey,
			validSignatureExpected:  true,
			validationErrorExpected: false,
		},
		"invalid signature for message": {
			message:                 []byte("And here..."),
			signature:               signature,
			publicKey:               publicKey,
			validSignatureExpected:  false,
			validationErrorExpected: false,
		},
		"corrupted signature": {
			message:                 message,
			signature:               []byte("we..."),
			publicKey:               publicKey,
			validSignatureExpected:  false,
			validationErrorExpected: true,
		},
		"invalid remote public key": {
			message:                 message,
			signature:               signature,
			publicKey:               signer2.PublicKey(),
			validSignatureExpected:  false,
			validationErrorExpected: false,
		},
		"corrupted remote public key": {
			message:                 message,
			signature:               signature,
			publicKey:               []byte("go..."),
			validSignatureExpected:  false,
			validationErrorExpected: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ok, err := signer2.VerifyWithPublicKey(
				test.message,
				test.signature,
				test.publicKey,
			)

			if !ok && test.validSignatureExpected {
				t.Errorf("expected valid signature but verification failed")
			}
			if ok && !test.validSignatureExpected {
				t.Errorf("expected invalid signature but verification succeeded")
			}

			if err == nil && test.validationErrorExpected {
				t.Errorf("expected signature validation error; none happened")
			}
			if err != nil && !test.validationErrorExpected {
				t.Errorf("unexpected signature validation error [%v]", err)
			}
		})
	}
}

func newSigner() (*EthereumSigner, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	return &EthereumSigner{key}, nil
}
