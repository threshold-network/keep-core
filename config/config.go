package config

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/keep-network/keep-core/config/network"
	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-log"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/term"

	commonEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
	"github.com/keep-network/keep-core/pkg/maintainer"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/storage"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

var logger = log.Logger("keep-config")

const (
	// #nosec G101 (look for hardcoded credentials)
	// This line doesn't contain any credentials.
	// It's just the name of the environment variable.
	EthereumPasswordEnvVariable = "KEEP_ETHEREUM_PASSWORD"

	// LogLevelEnvVariable can be used to define logging configuration.
	LogLevelEnvVariable = "LOG_LEVEL"

	// PubsubLogLevelEnvVariable can be used to define logging configuration
	// for the pubsub implementation.
	PubsubLogLevelEnvVariable = "PUBSUB_LOG_LEVEL"
)

// Config is the top level config structure.
type Config struct {
	Ethereum       commonEthereum.Config
	Bitcoin        BitcoinConfig
	LibP2P         libp2p.Config `mapstructure:"network"`
	Storage        storage.Config
	ClientInfo     clientinfo.Config
	CovenantSigner covenantsigner.Config
	Maintainer     maintainer.Config
	Tbtc           tbtc.Config
}

// BitcoinConfig defines the configuration for Bitcoin.
type BitcoinConfig struct {
	bitcoin.Network
	// Electrum defines the configuration for the Electrum client.
	Electrum electrum.Config
}

// Bind the flags to the viper configuration. Viper reads configuration from
// command-line flags, environment variables and config file.
func bindFlags(flagSet *pflag.FlagSet) error {
	if err := viper.BindPFlags(flagSet); err != nil {
		return err
	}
	return nil
}

// Resolve Ethereum and Bitcoin networks based on the set boolean flags.
func (c *Config) resolveNetworks(flagSet *pflag.FlagSet) (network.Type, error) {
	clientNetwork, err := func() (
		network.Type,
		error,
	) {
		isTestnet, err := flagSet.GetBool(network.Testnet.String())
		if err != nil {
			return network.Unknown, err
		}
		if isTestnet {
			return network.Testnet, nil
		}

		isDeveloper, err := flagSet.GetBool(network.Developer.String())
		if err != nil {
			return network.Unknown, err
		}
		if isDeveloper {
			return network.Developer, nil
		}

		return network.Mainnet, nil
	}()

	c.Ethereum.Network = clientNetwork.Ethereum()
	c.Bitcoin.Network = clientNetwork.Bitcoin()

	logger.Infof(
		"using [%v] Ethereum network and [%v] Bitcoin network",
		c.Ethereum.Network,
		c.Bitcoin.Network,
	)

	return clientNetwork, err
}

// ReadConfig reads in the configuration file at `configFilePath` and flags defined in
// the `flagSet`.
func (c *Config) ReadConfig(configFilePath string, flagSet *pflag.FlagSet, categories ...Category) error {
	initializeContractAddressesAliases()

	clientNetwork := network.Mainnet
	if flagSet != nil {
		if err := bindFlags(flagSet); err != nil {
			return fmt.Errorf("unable to bind the flags: [%w]", err)
		}

		var err error
		clientNetwork, err = c.resolveNetworks(flagSet)
		if err != nil {
			return fmt.Errorf("unable to resolve networks: [%w]", err)
		}
	}

	// Read configuration from a file if the config file path is set.
	if configFilePath != "" {
		if err := readConfigFile(configFilePath); err != nil {
			return fmt.Errorf(
				"unable to load config (file: [%s]): [%w]",
				configFilePath,
				err,
			)
		}
	}

	// Unmarshal config based on loaded config file and command-line flags.
	if err := unmarshalConfig(c); err != nil {
		return fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Resolve contracts addresses.
	c.resolveContractsAddresses()

	// Resolve network peers.
	err := c.resolvePeers(clientNetwork)
	if err != nil {
		return fmt.Errorf("failed to resolve peers: %w", err)
	}

	// Resolve Electrum server.
	// #nosec G404 (insecure random number source (rand))
	// Picking up an Electrum server does not require secure randomness.
	err = c.resolveElectrum(rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return fmt.Errorf("failed to resolve Electrum: %w", err)
	}

	// Validate configuration.
	if err := validateConfig(c, categories...); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Don't use viper.BindEnv for password reading as it's too sensitive value
	// to read it with an external library.
	if c.Ethereum.Account.KeyFilePassword == "" {
		c.Ethereum.Account.KeyFilePassword = os.Getenv(EthereumPasswordEnvVariable)
	}

	if strings.TrimSpace(c.Ethereum.Account.KeyFilePassword) == "" {
		var (
			password string
			err      error
		)
		fmt.Printf(
			"Ethereum Account Password has to be set for the configured Ethereum Key File.\n"+
				"Please set %s environment variable, or set it in the config file, or provide it in the prompt below.\n",
			EthereumPasswordEnvVariable,
		)

		for strings.TrimSpace(password) == "" {
			if password, err = readPassword("Enter Ethereum Account Password: "); err != nil {
				return err
			}
		}

		c.Ethereum.Account.KeyFilePassword = password
	}

	return nil
}

func validateConfig(config *Config, categories ...Category) error {
	var result *multierror.Error

	for _, category := range categories {
		switch category {
		case Ethereum:
			if config.Ethereum.URL == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for ethereum.url; see ethereum section in configuration",
				))
			}

			if config.Ethereum.Account.KeyFile == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for ethereum.keyFile; see ethereum section in configuration",
				))
			}
		case BitcoinElectrum:
			if config.Bitcoin.Electrum.URL == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for bitcoin.electrum.url; see bitcoin electrum section in configuration",
				))
			}
		case Network:
			if config.LibP2P.Port == 0 {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for network.port; see network section in configuration",
				))
			}
		case Storage:
			if config.Storage.Dir == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for storage.dir; see storage section in configuration",
				))
			}
		}
	}

	return result.ErrorOrNil()
}

// readConfigFile uses viper to read configuration from a config file. The config file
// is not mandatory, if the path is
func readConfigFile(configFilePath string) error {
	// Read configuration from a file, located in `configFilePath`.
	viper.SetConfigFile(configFilePath)

	// Read configuration.
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf(
			"failed to read configuration from file [%s]: %w",
			configFilePath,
			err,
		)
	}

	return nil
}

// unmarshalConfig unmarshals config with viper from config file and command-line
// flags into a struct.
func unmarshalConfig(config *Config) error {
	if err := viper.Unmarshal(
		config,
		viper.DecodeHook(
			mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.StringToSliceHookFunc(","),
				mapstructure.TextUnmarshallerHookFunc(),
			),
		),
	); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return nil
}

// readPassword prompts a user to enter a password. The read password uses
// the system password reading call that helps to prevent key loggers from
// capturing the password.
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	if err != nil {
		return "", fmt.Errorf("unable to read password, error [%s]", err)
	}

	return strings.TrimSpace(string(bytePassword)), nil
}
