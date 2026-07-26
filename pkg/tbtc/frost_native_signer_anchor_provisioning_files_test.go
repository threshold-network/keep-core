package tbtc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bootstrapProvisioningTestDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	return directory
}

func bootstrapProvisioningTestNoTemporaryResidue(
	t *testing.T,
	directory string,
) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("provisioning temporary file was left behind: %s", entry.Name())
		}
	}
}

func TestFrostNativeSignerAnchorBootstrapProvisioningArtifactWriteRead(
	t *testing.T,
) {
	directory := bootstrapProvisioningTestDirectory(t)
	path := filepath.Join(directory, "artifact.json")
	data := []byte(`{"schema":"test-artifact/v1"}`)
	if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
		path,
		data,
	); err != nil {
		t.Fatalf("valid provisioning artifact write failed: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		t.Fatalf("provisioning artifact mode is %v, expected 0600 regular", info.Mode())
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, data) {
		t.Fatalf("provisioning artifact bytes diverged: %q", stored)
	}
	bootstrapProvisioningTestNoTemporaryResidue(t, directory)

	read, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
		path,
		int64(len(data)),
	)
	if err != nil {
		t.Fatalf("valid provisioning artifact read failed: %v", err)
	}
	if !bytes.Equal(read, data) {
		t.Fatalf("provisioning artifact read diverged: %q", read)
	}

	if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
		path,
		[]byte("overwrite"),
	); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing provisioning artifact was overwritten: %v", err)
	}
	stored, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(stored, data) {
		t.Fatalf("refused overwrite still changed the artifact: %q, %v", stored, err)
	}
	bootstrapProvisioningTestNoTemporaryResidue(t, directory)
}

func TestFrostNativeSignerAnchorBootstrapProvisioningArtifactWriteRejections(
	t *testing.T,
) {
	directory := bootstrapProvisioningTestDirectory(t)
	data := []byte(`{"schema":"test-artifact/v1"}`)

	tests := map[string]struct {
		path string
		data []byte
	}{
		"relative path": {
			path: "artifact.json",
			data: data,
		},
		"non-canonical path": {
			path: filepath.Join(directory, "sub", "..", "artifact.json") + string(filepath.Separator),
			data: data,
		},
		"empty data": {
			path: filepath.Join(directory, "empty.json"),
			data: nil,
		},
		"oversize data": {
			path: filepath.Join(directory, "oversize.json"),
			data: make(
				[]byte,
				FrostNativeSignerAnchorProvisioningArtifactMaximumBytes+1,
			),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
				test.path,
				test.data,
			); err == nil {
				t.Fatalf("provisioning artifact write with %s succeeded", name)
			}
		})
	}
	bootstrapProvisioningTestNoTemporaryResidue(t, directory)

	t.Run("symlinked target", func(t *testing.T) {
		symlinked := filepath.Join(directory, "symlinked.json")
		if err := os.Symlink("/nonexistent-target", symlinked); err != nil {
			t.Fatal(err)
		}
		if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
			symlinked,
			data,
		); err == nil {
			t.Fatal("provisioning artifact write through a symlink succeeded")
		}
		bootstrapProvisioningTestNoTemporaryResidue(t, directory)
	})

	t.Run("group-accessible directory", func(t *testing.T) {
		loose := bootstrapProvisioningTestDirectory(t)
		if err := os.Chmod(loose, 0750); err != nil {
			t.Fatal(err)
		}
		if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
			filepath.Join(loose, "artifact.json"),
			data,
		); err == nil ||
			!strings.Contains(err.Error(), "0700") {
			t.Fatalf("write into a non-0700 directory succeeded: %v", err)
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
			filepath.Join(directory, "missing", "artifact.json"),
			data,
		); err == nil {
			t.Fatal("write into a missing directory succeeded")
		}
	})
}

func TestFrostNativeSignerAnchorBootstrapProvisioningArtifactReadRejections(
	t *testing.T,
) {
	directory := bootstrapProvisioningTestDirectory(t)
	path := filepath.Join(directory, "artifact.json")
	data := []byte(`{"schema":"test-artifact/v1"}`)
	if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
		path,
		data,
	); err != nil {
		t.Fatal(err)
	}

	t.Run("relative path", func(t *testing.T) {
		if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
			"artifact.json",
			1024,
		); err == nil {
			t.Fatal("relative provisioning artifact path was read")
		}
	})
	t.Run("invalid byte bound", func(t *testing.T) {
		for _, bound := range []int64{
			0,
			-1,
			FrostNativeSignerAnchorProvisioningArtifactMaximumBytes + 1,
		} {
			if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
				path,
				bound,
			); err == nil {
				t.Fatalf("provisioning artifact byte bound %d was accepted", bound)
			}
		}
	})
	t.Run("oversize artifact", func(t *testing.T) {
		if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
			path,
			int64(len(data))-1,
		); err == nil {
			t.Fatal("provisioning artifact above the byte bound was read")
		}
	})
	t.Run("empty artifact", func(t *testing.T) {
		empty := filepath.Join(directory, "empty.json")
		if err := os.WriteFile(empty, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
			empty,
			1024,
		); err == nil {
			t.Fatal("empty provisioning artifact was read")
		}
	})
	t.Run("wrong mode", func(t *testing.T) {
		loose := filepath.Join(directory, "loose.json")
		if err := os.WriteFile(loose, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(loose, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
			loose,
			1024,
		); err == nil ||
			!strings.Contains(err.Error(), "0600") {
			t.Fatalf("group-readable provisioning artifact was read: %v", err)
		}
	})
	t.Run("symlinked artifact", func(t *testing.T) {
		symlinked := filepath.Join(directory, "symlinked.json")
		if err := os.Symlink(path, symlinked); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
			symlinked,
			1024,
		); err == nil {
			t.Fatal("symlinked provisioning artifact was read")
		}
	})
	t.Run("group-accessible directory", func(t *testing.T) {
		loose := bootstrapProvisioningTestDirectory(t)
		loosePath := filepath.Join(loose, "artifact.json")
		if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
			loosePath,
			data,
		); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(loose, 0750); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
			loosePath,
			1024,
		); err == nil {
			t.Fatal("provisioning artifact in a non-0700 directory was read")
		}
	})
}
