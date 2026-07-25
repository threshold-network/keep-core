package signing

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const nativeTBTCSignerInitConfigMaximumBytes int64 = 1024 * 1024

// readSecureNativeTBTCSignerInitConfig opens the operator-selected config
// without following the final path component and validates the opened
// descriptor itself. The config can carry state_key_command, so treating it as
// ordinary mutable configuration would permit local command substitution
// between launcher checks and signer initialization.
func readSecureNativeTBTCSignerInitConfig(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("native signer init config path is empty")
	}
	fd, err := unix.Open(
		path,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf(
			"cannot wrap native signer init config descriptor",
		)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("native signer init config is not a regular file")
	}
	if info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf(
			"native signer init config permissions [%o] are not 0600",
			info.Mode().Perm(),
		)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot determine native signer init config owner")
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return nil, fmt.Errorf(
			"native signer init config is owned by uid [%d], expected [%d]",
			stat.Uid,
			os.Geteuid(),
		)
	}
	if info.Size() <= 0 ||
		info.Size() > nativeTBTCSignerInitConfigMaximumBytes {
		return nil, fmt.Errorf("native signer init config size is invalid")
	}
	result, err := io.ReadAll(io.LimitReader(
		file,
		nativeTBTCSignerInitConfigMaximumBytes+1,
	))
	if err != nil {
		zeroNativeTBTCSignerConfigBytes(result)
		return nil, err
	}
	if len(result) == 0 ||
		int64(len(result)) > nativeTBTCSignerInitConfigMaximumBytes {
		zeroNativeTBTCSignerConfigBytes(result)
		return nil, fmt.Errorf("native signer init config size is invalid")
	}
	return result, nil
}

func zeroNativeTBTCSignerConfigBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
