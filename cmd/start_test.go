package cmd

import (
	"strings"
	"testing"

	"github.com/keep-network/keep-core/config"
	"github.com/spf13/cobra"
)

func TestNetworkBootstrapFlagDescription_ContainsDeprecationNotice(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.Config{}

	initNetworkFlags(cmd, cfg)

	flag := cmd.Flags().Lookup("network.bootstrap")
	if flag == nil {
		t.Fatal("expected network.bootstrap flag to be registered")
	}

	usageLower := strings.ToLower(flag.Usage)
	if !strings.Contains(usageLower, "deprecated") {
		t.Errorf(
			"expected flag description to contain deprecation notice, got: %q",
			flag.Usage,
		)
	}
}

func TestIsBootstrap(t *testing.T) {
	tests := map[string]struct {
		bootstrapValue bool
		expected       bool
	}{
		"returns true when bootstrap flag is set": {
			bootstrapValue: true,
			expected:       true,
		},
		"returns false when bootstrap flag is not set": {
			bootstrapValue: false,
			expected:       false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			originalBootstrap := clientConfig.LibP2P.Bootstrap
			defer func() { clientConfig.LibP2P.Bootstrap = originalBootstrap }()

			clientConfig.LibP2P.Bootstrap = tc.bootstrapValue

			got := isBootstrap()
			if got != tc.expected {
				t.Errorf("expected isBootstrap() to return %v, got %v", tc.expected, got)
			}
		})
	}
}
