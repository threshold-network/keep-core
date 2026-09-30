package maintainer

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
)

// Config contains maintainer configuration.
type Config struct {
	BitcoinDifficulty btcdiff.Config
	Spv               spv.Config
}

// Validate checks whether the maintainer configuration is valid for the
// modules that will be launched. Currently only Spv requires pre-launch
// validation; BitcoinDifficulty does not.
func (c Config) Validate() error {
	launchAll := !c.BitcoinDifficulty.Enabled &&
		!c.Spv.Enabled

	if c.Spv.Enabled || launchAll {
		if err := c.Spv.Validate(); err != nil {
			return fmt.Errorf("cannot validate spv maintainer config: [%w]", err)
		}
	}

	return nil
}
