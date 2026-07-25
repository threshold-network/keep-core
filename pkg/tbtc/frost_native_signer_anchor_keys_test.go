package tbtc

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrostNativeSignerAnchorKeys(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	privatePath := filepath.Join(directory, "client-key.pem")
	publicPath := filepath.Join(directory, "online-key.der")
	if err := os.WriteFile(
		privatePath,
		pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateDER,
		}),
		0600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, publicDER, 0600); err != nil {
		t.Fatal(err)
	}

	loadedPrivate, err :=
		loadFrostNativeSignerAnchorClientPrivateKey(privatePath)
	if err != nil {
		t.Fatalf("cannot load valid client private key: %v", err)
	}
	if !loadedPrivate.Equal(privateKey) {
		t.Fatal("loaded client private key differs")
	}
	loadedSPKI, loadedPublic, err :=
		loadFrostNativeSignerAnchorOnlinePublicKeySPKI(publicPath)
	if err != nil {
		t.Fatalf("cannot load valid online public key: %v", err)
	}
	if string(loadedSPKI) != string(publicDER) ||
		!loadedPublic.Equal(publicKey) {
		t.Fatal("loaded online public key differs")
	}
}

func TestLoadFrostNativeSignerAnchorKeysRejectsUnsafeFiles(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateDER,
	})
	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("group-readable private key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "client-key.pem")
		if err := os.WriteFile(path, privatePEM, 0640); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFrostNativeSignerAnchorClientPrivateKey(path); err == nil {
			t.Fatal("group-readable client private key was accepted")
		}
	})

	t.Run("private key symlink", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "target.pem")
		link := filepath.Join(directory, "client-key.pem")
		if err := os.WriteFile(target, privatePEM, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFrostNativeSignerAnchorClientPrivateKey(link); err == nil {
			t.Fatal("symlinked client private key was accepted")
		}
	})

	t.Run("noncanonical public SPKI", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "online-key.der")
		noncanonical := append(append([]byte{}, publicDER...), 0)
		if err := os.WriteFile(path, noncanonical, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err :=
			loadFrostNativeSignerAnchorOnlinePublicKeySPKI(path); err == nil {
			t.Fatal("noncanonical online public-key SPKI was accepted")
		}
	})

	t.Run("public key symlink", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "target.der")
		link := filepath.Join(directory, "online-key.der")
		if err := os.WriteFile(target, publicDER, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, _, err :=
			loadFrostNativeSignerAnchorOnlinePublicKeySPKI(link); err == nil {
			t.Fatal("symlinked online public key was accepted")
		}
	})
}
