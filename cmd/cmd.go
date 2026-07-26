package cmd

import (
	"fmt"
	"os"
	"path"

	"github.com/ipfs/go-log"
	"github.com/spf13/cobra"

	"github.com/keep-network/keep-core/config"
)

// Path to the configuration file. The value is set with `--config` command-line flag.
var configFilePath string

// The client configuration for command-line execution. The content is read from
// a config file and command-line flags.
var clientConfig = &config.Config{}

var logger = log.Logger("keep-cmd")

// RootCmd contains the definition of the root command-line command.
var RootCmd = &cobra.Command{
	Use:              path.Base(os.Args[0]),
	Short:            "CLI for The Keep Network",
	Long:             "Command line interface (CLI) for running a Keep provider",
	TraverseChildren: true,
}

func init() {
	initGlobalFlags(RootCmd, &configFilePath)

	RootCmd.AddCommand(
		StartCommand,
		PingCommand,
		EthereumCommand,
		MaintainerCommand,
		MaintainerCliCommand,
		TBTCSignerCommand,
	)
}

// Initialize initializes the root command and returns it.
func Initialize(version, revision string) *cobra.Command {
	RootCmd.Version = fmt.Sprintf("%s (revision %s)", version, revision)

	return RootCmd
}
