package tbtc

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

const FrostNativeSignerAnchorProvisioningArtifactMaximumBytes int64 = 16 * 1024 * 1024

// ReadFrostNativeSignerAnchorProvisioningArtifact opens an immutable ceremony
// artifact without following the final path component and validates the
// opened descriptor itself. Ceremony files are integrity-sensitive even when
// they contain only public material, so they use the same owner-only posture
// as signer configuration.
func ReadFrostNativeSignerAnchorProvisioningArtifact(
	path string,
	maximumBytes int64,
) ([]byte, error) {
	directory, name, err := frostNativeSignerAnchorProvisioningPath(path)
	if err != nil {
		return nil, err
	}
	if maximumBytes <= 0 ||
		maximumBytes > FrostNativeSignerAnchorProvisioningArtifactMaximumBytes {
		return nil, fmt.Errorf("provisioning artifact byte bound is invalid")
	}
	directoryFile, err := openFrostNativeSignerAnchorProvisioningDirectory(
		directory,
	)
	if err != nil {
		return nil, err
	}
	defer directoryFile.Close()
	fd, err := unix.Openat(
		int(directoryFile.Fd()),
		name,
		unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("cannot wrap provisioning artifact descriptor")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorProvisioningFileInfo(
		info,
	); err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > maximumBytes {
		return nil, fmt.Errorf("provisioning artifact size is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > maximumBytes {
		return nil, fmt.Errorf("provisioning artifact size is invalid")
	}
	return data, nil
}

// WriteFrostNativeSignerAnchorProvisioningArtifact publishes one immutable
// artifact with no-replace semantics. The file and containing directory are
// fsynced before success is returned, and a racing or pre-existing destination
// is never overwritten.
func WriteFrostNativeSignerAnchorProvisioningArtifact(
	path string,
	data []byte,
) error {
	directory, name, err := frostNativeSignerAnchorProvisioningPath(path)
	if err != nil {
		return err
	}
	if len(data) == 0 ||
		int64(len(data)) >
			FrostNativeSignerAnchorProvisioningArtifactMaximumBytes {
		return fmt.Errorf("provisioning artifact size is invalid")
	}
	directoryFile, err := openFrostNativeSignerAnchorProvisioningDirectory(
		directory,
	)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	directoryFD := int(directoryFile.Fd())

	var destination unix.Stat_t
	err = unix.Fstatat(
		directoryFD,
		name,
		&destination,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err == nil {
		return fmt.Errorf("provisioning artifact already exists: [%s]", path)
	}
	if !errors.Is(err, unix.ENOENT) {
		return err
	}

	temporary, temporaryName, err :=
		createFrostNativeSignerAnchorProvisioningTemporary(
			directoryFD,
			directory,
			name,
		)
	if err != nil {
		return err
	}
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = unix.Unlinkat(directoryFD, temporaryName, 0)
		}
	}()
	written, err := temporary.Write(data)
	if err != nil {
		_ = temporary.Close()
		return err
	}
	if written != len(data) {
		_ = temporary.Close()
		return fmt.Errorf(
			"provisioning artifact short write: [%d] of [%d] bytes",
			written,
			len(data),
		)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := unix.Linkat(
		directoryFD,
		temporaryName,
		directoryFD,
		name,
		0,
	); err != nil {
		return fmt.Errorf("cannot publish provisioning artifact: %w", err)
	}
	// Linkat has durably published the artifact. Flip the deferred
	// cleanup sentinel so a later error path does not retry the same
	// unlink, fsync the directory so the published link survives a
	// crash, and downgrade a temporary unlink failure to a logged
	// leftover: the canonical artifact is already in place and a
	// failed unlink must not abort the call or block re-provisioning.
	removeTemporary = false
	if err := directoryFile.Sync(); err != nil {
		return err
	}
	if err := unix.Unlinkat(directoryFD, temporaryName, 0); err != nil {
		logger.Warnf(
			"cannot remove provisioning artifact temporary [%s]: [%v]",
			temporaryName,
			err,
		)
	}
	return nil
}

func frostNativeSignerAnchorProvisioningPath(
	path string,
) (string, string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", "", fmt.Errorf(
			"provisioning artifact path is not canonical absolute",
		)
	}
	directory := filepath.Dir(path)
	name := filepath.Base(path)
	if !filepath.IsLocal(name) || name == "." || name == string(filepath.Separator) {
		return "", "", fmt.Errorf("provisioning artifact file name is invalid")
	}
	return directory, name, nil
}

func openFrostNativeSignerAnchorProvisioningDirectory(
	directory string,
) (*os.File, error) {
	fd, err := unix.Open(
		directory,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), directory)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("cannot wrap provisioning directory descriptor")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0700 {
		_ = file.Close()
		return nil, fmt.Errorf(
			"provisioning directory must be a non-symlink owner-only 0700 directory",
		)
	}
	if err := validateFrostNativeSignerAnchorProvisioningOwner(info); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func createFrostNativeSignerAnchorProvisioningTemporary(
	directoryFD int,
	directory string,
	finalName string,
) (*os.File, string, error) {
	const maximumAttempts = 128
	var entropy [16]byte
	for attempt := 0; attempt < maximumAttempts; attempt++ {
		if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return nil, "", err
		}
		name := finalName + "-" + hex.EncodeToString(entropy[:]) + ".tmp"
		fd, err := unix.Openat(
			directoryFD,
			name,
			unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW,
			0600,
		)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		file := os.NewFile(uintptr(fd), filepath.Join(directory, name))
		if file == nil {
			_ = unix.Close(fd)
			_ = unix.Unlinkat(directoryFD, name, 0)
			return nil, "", fmt.Errorf(
				"cannot wrap provisioning temporary descriptor",
			)
		}
		if err := file.Chmod(0600); err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryFD, name, 0)
			return nil, "", err
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryFD, name, 0)
			return nil, "", err
		}
		if err := validateFrostNativeSignerAnchorProvisioningFileInfo(
			info,
		); err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryFD, name, 0)
			return nil, "", err
		}
		return file, name, nil
	}
	return nil, "", fmt.Errorf(
		"cannot allocate a unique provisioning temporary file",
	)
}

func validateFrostNativeSignerAnchorProvisioningFileInfo(
	info os.FileInfo,
) error {
	if info == nil || !info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0600 {
		return fmt.Errorf(
			"provisioning artifact must be a non-symlink owner-only 0600 regular file",
		)
	}
	return validateFrostNativeSignerAnchorProvisioningOwner(info)
}

func validateFrostNativeSignerAnchorProvisioningOwner(
	info os.FileInfo,
) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine provisioning artifact owner")
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf(
			"provisioning artifact is owned by uid [%d], expected [%d]",
			stat.Uid,
			os.Geteuid(),
		)
	}
	return nil
}
