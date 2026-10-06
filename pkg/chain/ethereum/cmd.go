package ethereum

import (
	"github.com/spf13/cobra"

	commonEthereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
)

// Command if a wrapper for cobra.Command that holds Ethereum config used by the
// generated sub-commands to initialize the Ethereum chain connection.
type Command struct {
	cobra.Command
	config *commonEthereum.Config
}

// SetConfig is used to set the ethereum configuration that is used by the
// generated sub-commands to initialize the Ethereum chain connection.
func (c *Command) SetConfig(config *commonEthereum.Config) {
	c.config = config
}

// GetConfig returns the Ethereum config.
func (c *Command) GetConfig() *commonEthereum.Config {
	return c.config
}
