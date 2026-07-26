package signing

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const nativeTBTCSignerInitConfigMaximumBytes int64 = 1024 * 1024

const nativeTBTCSignerStateAnchorBootstrapProvisioningWitnessMaximum uint64 = 4

type nativeTBTCSignerStateAnchorBootstrapProvisioningConfig struct {
	Purpose                *string `json:"purpose"`
	Profile                *string `json:"profile"`
	StatePath              *string `json:"state_path"`
	StateWitnessMaxRecords *uint64 `json:"state_witness_max_records"`
}

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

// InstallNativeTBTCSignerStateAnchorBootstrapProvisioningConfigFile installs
// the deliberately minimal, production-only config accepted by the bootstrap
// facts FFI. It must run before any state-touching signer operation. Requiring
// an exact four-field object prevents an online ceremony process from
// accidentally acquiring runtime signing or anchor authority.
func InstallNativeTBTCSignerStateAnchorBootstrapProvisioningConfigFile(
	path string,
) (*NativeTBTCSignerInitConfigResult, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf(
			"native signer bootstrap provisioning config path is not canonical absolute",
		)
	}
	configJSON, err := readSecureNativeTBTCSignerInitConfig(path)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read native signer bootstrap provisioning config: %w",
			err,
		)
	}
	defer zeroNativeTBTCSignerConfigBytes(configJSON)

	wire := &nativeTBTCSignerStateAnchorBootstrapProvisioningConfig{}
	if err := decodeStrictNativeTBTCSignerJSON(
		configJSON,
		wire,
		"state-anchor bootstrap provisioning config",
	); err != nil {
		return nil, err
	}
	if wire.Purpose == nil ||
		*wire.Purpose !=
			NativeTBTCSignerStateAnchorBootstrapProvisioningPurpose ||
		wire.Profile == nil ||
		*wire.Profile != "production" ||
		wire.StatePath == nil ||
		strings.TrimSpace(*wire.StatePath) == "" ||
		!filepath.IsAbs(*wire.StatePath) ||
		filepath.Clean(*wire.StatePath) != *wire.StatePath ||
		wire.StateWitnessMaxRecords == nil ||
		*wire.StateWitnessMaxRecords !=
			nativeTBTCSignerStateAnchorBootstrapProvisioningWitnessMaximum {
		return nil, fmt.Errorf(
			"native signer bootstrap provisioning config must contain exactly purpose=%q, profile=production, a canonical absolute state_path, and state_witness_max_records=%d",
			NativeTBTCSignerStateAnchorBootstrapProvisioningPurpose,
			nativeTBTCSignerStateAnchorBootstrapProvisioningWitnessMaximum,
		)
	}

	result, err := InstallNativeTBTCSignerConfig(configJSON)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot install native signer bootstrap provisioning config: %w",
			err,
		)
	}
	if result == nil || !result.Installed ||
		strings.TrimSpace(result.ConfigFingerprint) == "" {
		return nil, fmt.Errorf(
			"native signer bootstrap provisioning config installation returned an incomplete result",
		)
	}
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		configJSON,
		result.ConfigFingerprint,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot bind native signer bootstrap provisioning config: %w",
			err,
		)
	}
	return result, nil
}

func zeroNativeTBTCSignerConfigBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
