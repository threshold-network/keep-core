package signing

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReadSecureNativeTBTCSignerInitConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signer-config.json")
	expected := []byte(`{"state_path":"/secure/state"}`)
	if err := os.WriteFile(path, expected, 0600); err != nil {
		t.Fatal(err)
	}
	actual, err := readSecureNativeTBTCSignerInitConfig(path)
	if err != nil {
		t.Fatalf("cannot read secure native signer config: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatal("secure native signer config bytes changed")
	}
}

func TestReadSecureNativeTBTCSignerInitConfigRejectsUnsafeFiles(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "target.json")
		link := filepath.Join(directory, "signer-config.json")
		if err := os.WriteFile(target, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := readSecureNativeTBTCSignerInitConfig(link); err == nil {
			t.Fatal("symlinked native signer init config was accepted")
		}
	})

	t.Run("group readable", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "signer-config.json")
		if err := os.WriteFile(path, []byte(`{}`), 0640); err != nil {
			t.Fatal(err)
		}
		if _, err := readSecureNativeTBTCSignerInitConfig(path); err == nil {
			t.Fatal("group-readable native signer init config was accepted")
		}
	})

	t.Run("directory", func(t *testing.T) {
		path := t.TempDir()
		if err := os.Chmod(path, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readSecureNativeTBTCSignerInitConfig(path); err == nil {
			t.Fatal("directory native signer init config was accepted")
		}
	})

	t.Run("fifo", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "signer-config.json")
		if err := unix.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readSecureNativeTBTCSignerInitConfig(path); err == nil {
			t.Fatal("FIFO native signer init config was accepted")
		}
	})

	t.Run("oversized", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "signer-config.json")
		payload := bytes.Repeat(
			[]byte{'x'},
			int(nativeTBTCSignerInitConfigMaximumBytes)+1,
		)
		if err := os.WriteFile(path, payload, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readSecureNativeTBTCSignerInitConfig(path); err == nil {
			t.Fatal("oversized native signer init config was accepted")
		}
	})
}
