package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/keep-network/keep-core/config"
)

// TestMaintainerConfig_RejectsExplicitZeroMaxProofHeaders asserts that an
// explicit `--spv.maxProofHeaders 0` is rejected by the same validation the
// maintainer command runs at startup.
//
// The flag is registered with cobra's UintVar, which accepts 0 as a valid
// unsigned value, so the 144 default only protects the case where the flag is
// omitted entirely. Without validation a zero bound reaches the proof
// assembly loop, where getProofInfo skips every transaction, disabling SPV
// proving. A test exercising only the omitted-flag default would pass
// regardless and mask this.
//
// The test drives validateMaintainerConfig, which delegates to
// maintainer.Config.Validate - the same rule the config-load pass applies.
// A fresh flag-initialized config has neither maintainer enabled, so the
// launch-all branch validates the SPV settings and reports the offending
// field.
func TestMaintainerConfig_RejectsExplicitZeroMaxProofHeaders(t *testing.T) {
	command := &cobra.Command{Use: "maintainer-test"}
	cfg := &config.Config{}
	initMaintainerFlags(command, cfg)

	if err := command.Flags().Parse(
		[]string{"--spv.maxProofHeaders", "0"},
	); err != nil {
		t.Fatalf("failed to parse flags: [%v]", err)
	}

	// Establishes that zero is genuinely reachable through the CLI rather than
	// being rejected by flag parsing itself.
	if got := cfg.Maintainer.Spv.MaxProofHeaders; got != 0 {
		t.Fatalf("expected the flag to parse zero into config, got [%d]", got)
	}

	err := validateMaintainerConfig(cfg)
	if err == nil {
		t.Fatal(
			"expected startup validation to reject --spv.maxProofHeaders 0, got no error",
		)
	}
	if !strings.Contains(err.Error(), "maxProofHeaders") {
		t.Errorf(
			"expected the error to name the offending setting, got: [%v]",
			err,
		)
	}
}

// TestMaintainerConfig_AcceptsDefaultMaxProofHeaders asserts the validation
// added above does not reject a normally-configured maintainer.
func TestMaintainerConfig_AcceptsDefaultMaxProofHeaders(t *testing.T) {
	command := &cobra.Command{Use: "maintainer-test"}
	cfg := &config.Config{}
	initMaintainerFlags(command, cfg)

	if err := command.Flags().Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: [%v]", err)
	}

	if err := validateMaintainerConfig(cfg); err != nil {
		t.Errorf(
			"expected the default configuration to be accepted, got error: [%v]",
			err,
		)
	}
}

// TestMaintainerConfig_AcceptsCustomMaxProofHeaders asserts that explicitly
// provided positive values flow through the flag -> config -> validation path,
// not just the omitted-flag default. Without these cases a regression that
// breaks non-default values (e.g. a type conversion or off-by-one in
// validation) would only surface when an operator actually deviates from the
// 144 default.
func TestMaintainerConfig_AcceptsCustomMaxProofHeaders(t *testing.T) {
	values := []string{"1", "288"}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			command := &cobra.Command{Use: "maintainer-test"}
			cfg := &config.Config{}
			initMaintainerFlags(command, cfg)

			if err := command.Flags().Parse(
				[]string{"--spv.maxProofHeaders", value},
			); err != nil {
				t.Fatalf("failed to parse flags: [%v]", err)
			}

			if err := validateMaintainerConfig(cfg); err != nil {
				t.Errorf(
					"expected --spv.maxProofHeaders %s to be accepted, got error: [%v]",
					value,
					err,
				)
			}
		})
	}
}
