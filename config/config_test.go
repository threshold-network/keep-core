package config

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/slices"

	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	ethereumBeacon "github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen"
	ethereumEcdsa "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen"
	ethereumTbtc "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen"
	ethereumThreshold "github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen"
)

func TestReadConfigFromFile(t *testing.T) {
	filePaths := []string{
		"../test/config.toml",
		"../test/config.json",
		"../test/config.yaml",
	}

	var configReadTests = map[string]struct {
		readValueFunc func(*Config) interface{}
		expectedValue interface{}
	}{
		"Ethereum.URL": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.URL },
			expectedValue: "ws://192.168.0.158:8546",
		},
		"Ethereum.KeyFile": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.Account.KeyFile },
			expectedValue: "/tmp/UTC--2018-03-11T01-37-33.202765887Z--c2a56884538778bacd91aa5bf343bf882c5fb18b",
		},
		"Ethereum.KeyFilePassword": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.Account.KeyFilePassword },
			expectedValue: "THIS IS TEST! Password should be defined in env variable or prompt",
		},
		"Ethereum.MiningCheckInterval": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.MiningCheckInterval },
			expectedValue: 33 * time.Second,
		},
		"Ethereum.RequestsPerSecondLimit": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.RequestsPerSecondLimit },
			expectedValue: 13,
		},
		"Ethereum.ConcurrencyLimit": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.ConcurrencyLimit },
			expectedValue: 56,
		},
		"Ethereum.MaxGasFeeCap": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.MaxGasFeeCap.Int },
			expectedValue: big.NewInt(148000000000),
		},
		"Ethereum.BalanceAlertThreshold": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.BalanceAlertThreshold.Int },
			expectedValue: big.NewInt(2300000000000000000),
		},
		"Ethereum.Developer - map": {
			readValueFunc: func(c *Config) interface{} { return c.Ethereum.ContractAddresses },
			expectedValue: map[string]string{
				"randombeacon":              "0xcf64c2a367341170cb4e09cf8c0ed137d8473ceb",
				"walletregistry":            "0x143ba24e66fce8bca22f7d739f9a932c519b1c76",
				"tokenstaking":              "0xa363a197f1bbb8877f50350234e3f15fb4175457",
				"bridge":                    "0x138D2a0c87BA9f6BE1DCc13D6224A6aCE9B6b6F0",
				"maintainerproxy":           "0xC6D21c2871586A2B098c0ad043fF0D47a3c7e7ae",
				"lightrelay":                "0x68e20afD773fDF1231B5cbFeA7040e73e79cAc36",
				"lightrelaymaintainerproxy": "0x30cd93828613D5945A2916a22E0f0e9bC561EAB5",
				"walletproposalvalidator":   "0xfdc315b0e608b7cDE9166D9D69a1506779e3E0CA",
			},
		},
		"Developer - RandomBeacon": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(ethereum.RandomBeaconContractName)
				return address.String()
			},
			expectedValue: "0xcf64C2A367341170cb4E09cf8C0ED137d8473CEB",
		},
		"Developer.WalletRegistryAddress": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(ethereum.WalletRegistryContractName)
				return address.String()
			},
			expectedValue: "0x143Ba24e66FCe8bCA22F7D739F9a932c519B1C76",
		},
		"Ethereum.Developer - TokenStaking": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(ethereum.TokenStakingContractName)
				return address.String()
			},
			expectedValue: "0xa363a197F1BbB8877F50350234e3f15fB4175457",
		},
		"Ethereum.Developer - Bridge": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(ethereum.BridgeContractName)
				return address.String()
			},
			expectedValue: "0x138D2a0c87BA9f6BE1DCc13D6224A6aCE9B6b6F0",
		},
		"Ethereum.Developer - MaintainerProxy": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(ethereum.MaintainerProxyContractName)
				return address.String()
			},
			expectedValue: "0xC6D21c2871586A2B098c0ad043fF0D47a3c7e7ae",
		},
		"Ethereum.Developer - LightRelay": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(
					ethereum.LightRelayContractName,
				)
				return address.String()
			},
			expectedValue: "0x68e20afD773fDF1231B5cbFeA7040e73e79cAc36",
		},
		"Ethereum.Developer - LightRelayMaintainerProxy": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(
					ethereum.LightRelayMaintainerProxyContractName,
				)
				return address.String()
			},
			expectedValue: "0x30cd93828613D5945A2916a22E0f0e9bC561EAB5",
		},
		"Ethereum.Developer - WalletProposalValidator": {
			readValueFunc: func(c *Config) interface{} {
				address, _ := c.Ethereum.ContractAddress(ethereum.WalletProposalValidatorContractName)
				return address.String()
			},
			expectedValue: "0xfdc315b0e608b7cDE9166D9D69a1506779e3E0CA",
		},
		"Bitcoin.Electrum.URL": {
			readValueFunc: func(c *Config) interface{} { return c.Bitcoin.Electrum.URL },
			expectedValue: "ssl://url.to.electrum:18332",
		},

		"Bitcoin.Electrum.ConnectTimeout": {
			readValueFunc: func(c *Config) interface{} { return c.Bitcoin.Electrum.ConnectTimeout },
			expectedValue: 54 * time.Second,
		},
		"Bitcoin.Electrum.ConnectRetryTimeout": {
			readValueFunc: func(c *Config) interface{} { return c.Bitcoin.Electrum.ConnectRetryTimeout },
			expectedValue: 192 * time.Second,
		},
		"Bitcoin.Electrum.RequestTimeout": {
			readValueFunc: func(c *Config) interface{} { return c.Bitcoin.Electrum.RequestTimeout },
			expectedValue: 94 * time.Second,
		},
		"Bitcoin.Electrum.RequestRetryTimeout": {
			readValueFunc: func(c *Config) interface{} { return c.Bitcoin.Electrum.RequestRetryTimeout },
			expectedValue: 300 * time.Second,
		},
		"Bitcoin.Electrum.KeepAliveInterval": {
			readValueFunc: func(c *Config) interface{} { return c.Bitcoin.Electrum.KeepAliveInterval },
			expectedValue: 720 * time.Second,
		},
		"Network.Port": {
			readValueFunc: func(c *Config) interface{} { return c.LibP2P.Port },
			expectedValue: 27001,
		},
		"Network.Peers": {
			readValueFunc: func(c *Config) interface{} { return c.LibP2P.Peers },
			expectedValue: []string{
				"/ip4/127.0.0.1/tcp/3820/ipfs/16Uiu2HAmVZGi9bgF3w6C4TFVo9HjbCDyuecsQbzQnXQJvP5wBjkd",
				"/ip4/127.0.0.1/tcp/3819/ipfs/16Uiu2HAmVNfJs6t7bB3hYPTxtuKXdTdqxKYn9wKKfUiGwpCDjySM",
			},
		},
		"Network.AnnouncedAddresses": {
			readValueFunc: func(c *Config) interface{} { return c.LibP2P.AnnouncedAddresses },
			expectedValue: []string{
				"/dns4/example.com/tcp/3919",
				"/ip4/80.70.60.50/tcp/3919",
			},
		},
		"Network.DisseminationTime": {
			readValueFunc: func(c *Config) interface{} { return c.LibP2P.DisseminationTime },
			expectedValue: 76,
		},
		"Storage.Dir": {
			readValueFunc: func(c *Config) interface{} { return c.Storage.Dir },
			expectedValue: "/my/secure/location",
		},
		"ClientInfo.Port": {
			readValueFunc: func(c *Config) interface{} { return c.ClientInfo.Port },
			expectedValue: 3498,
		},
		"ClientInfo.NetworkMetricsTick": {
			readValueFunc: func(c *Config) interface{} { return c.ClientInfo.NetworkMetricsTick },
			expectedValue: 43 * time.Second,
		},
		"ClientInfo.EthereumMetricsTick": {
			readValueFunc: func(c *Config) interface{} { return c.ClientInfo.EthereumMetricsTick },
			expectedValue: 87 * time.Second,
		},
		"Maintainer.BitcoinDifficulty.Enabled": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.BitcoinDifficulty.Enabled },
			expectedValue: true,
		},
		"Maintainer.BitcoinDifficulty.DisableProxy": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.BitcoinDifficulty.DisableProxy },
			expectedValue: true,
		},
		"Maintainer.BitcoinDifficulty.IdleOnPreflightFailure": {
			readValueFunc: func(c *Config) interface{} {
				return c.Maintainer.BitcoinDifficulty.IdleOnPreflightFailure
			},
			expectedValue: true,
		},
		"Maintainer.Spv.Enabled": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.Spv.Enabled },
			expectedValue: true,
		},
		"Maintainer.Spv.HistoryDepth": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.Spv.HistoryDepth },
			expectedValue: uint64(25000),
		},
		"Maintainer.Spv.TransactionLimit": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.Spv.TransactionLimit },
			expectedValue: 80,
		},
		"Maintainer.Spv.RestartBackoffTime": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.Spv.RestartBackoffTime },
			expectedValue: 2 * time.Hour,
		},
		"Maintainer.Spv.IdleBackoffTime": {
			readValueFunc: func(c *Config) interface{} { return c.Maintainer.Spv.IdleBackoffTime },
			expectedValue: 15 * time.Minute,
		},
	}

	for _, filePath := range filePaths {
		t.Run(strings.TrimPrefix(filepath.Ext(filePath), "."), func(t *testing.T) {
			cfg := &Config{}
			if err := cfg.ReadConfig(filePath, nil, AllCategories...); err != nil {
				t.Fatalf("failed to read test config: [%v]", err)
			}

			for testName, test := range configReadTests {
				t.Run(testName, func(t *testing.T) {
					expected := test.expectedValue
					actual := test.readValueFunc(cfg)
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("\nexpected: %s\nactual:   %s", expected, actual)
					}
				})
			}
		})
	}
}

func TestReadConfig_ReadPassword(t *testing.T) {
	expectToPrompt := "expect-to-prompt"

	var configReadTests = map[string]struct {
		configFilePath      string
		envVariablePassword string
		expectedPassword    string
	}{
		"no password in file; password in environment variable": {
			configFilePath:      "../test/config_no_password.toml",
			envVariablePassword: "some-SECRET-phrase-as-ENVIRONMENT-VARIABLE",
			expectedPassword:    "some-SECRET-phrase-as-ENVIRONMENT-VARIABLE",
		},
		"password in file; password in environment variable": {
			configFilePath:      "../test/config.toml",
			envVariablePassword: "some-SECRET-phrase-as-ENVIRONMENT-VARIABLE",
			expectedPassword:    "THIS IS TEST! Password should be defined in env variable or prompt",
		},
		"password in file; no password in environment variable": {
			configFilePath:      "../test/config.toml",
			envVariablePassword: "",
			expectedPassword:    "THIS IS TEST! Password should be defined in env variable or prompt",
		},
		"no password in file; no password in environment variable": {
			configFilePath:      "../test/config_no_password.toml",
			envVariablePassword: "",
			expectedPassword:    expectToPrompt,
		},
	}

	for testName, test := range configReadTests {
		t.Run(testName, func(t *testing.T) {
			if err := os.Setenv(EthereumPasswordEnvVariable, test.envVariablePassword); err != nil {
				t.Fatal(err)
			}

			cfg := &Config{}
			err := cfg.ReadConfig(test.configFilePath, nil, AllCategories...)

			if test.expectedPassword != expectToPrompt {
				if err != nil {
					t.Fatalf("failed to read test config: [%v]", err)
				}

				if cfg.Ethereum.Account.KeyFilePassword != test.expectedPassword {
					t.Errorf(
						"\nexpected: %s\nactual:   %s",
						test.expectedPassword,
						cfg.Ethereum.Account.KeyFilePassword,
					)
				}
			} else {
				// When password is not set neither in config file nor environment
				// variable we expect to prompt user to provide the password.
				// The prompt execution fail in unit tests so we assume that
				// if a specific error is thrown the prompt was issued.
				expectedErrors := []string{
					"unable to read password, error [operation not supported by device]",
					"unable to read password, error [inappropriate ioctl for device]",
				}
				if !slices.Contains(expectedErrors, err.Error()) {
					t.Errorf(
						"unexpected error\nexpected: %v\nactual:   %v\n",
						fmt.Sprintf(
							"\n  one of:\n  * %s",
							strings.Join(expectedErrors, "\n  * "),
						),
						err,
					)
				}
			}
		})
	}
}

func TestReadConfig_ReadContracts(t *testing.T) {
	if err := os.Setenv(EthereumPasswordEnvVariable, "password from env var"); err != nil {
		t.Fatal(err)
	}

	ethereumBeacon.RandomBeaconAddress = "0xd1640b381327c2d5425d6d3d605539a3db72f857"
	ethereumEcdsa.WalletRegistryAddress = "0xdb3dd6d4f43d39c996d0afeb6fbabc284f9ffb1a"
	ethereumThreshold.TokenStakingAddress = "0xaa7b41039ea8f9ec2d89bbe96e19f97b6c267a27"
	ethereumTbtc.BridgeAddress = "0x9490165195503fcf6a0fd20ac113223fefb66ed5"
	ethereumTbtc.WalletProposalValidatorAddress = "0xE7d33d8AA55B73a93059a24b900366894684a497"

	var configReadTests = map[string]struct {
		configFilePath string

		expectedRandomBeaconAddress            string
		expectedWalletRegistryAddress          string
		expectedTokenStakingAddress            string
		expectedBridgeAddress                  string
		expectedWalletProposalValidatorAddress string
	}{
		"no developer contracts addresses configured": {
			configFilePath:                         "../test/config_no_contracts.toml",
			expectedRandomBeaconAddress:            "0xd1640b381327c2d5425d6d3d605539a3db72f857",
			expectedWalletRegistryAddress:          "0xdb3dd6d4f43d39c996d0afeb6fbabc284f9ffb1a",
			expectedTokenStakingAddress:            "0xaa7b41039ea8f9ec2d89bbe96e19f97b6c267a27",
			expectedBridgeAddress:                  "0x9490165195503fcf6a0fd20ac113223fefb66ed5",
			expectedWalletProposalValidatorAddress: "0xE7d33d8AA55B73a93059a24b900366894684a497",
		},
		"developer contracts addresses configured": {
			configFilePath:                         "../test/config.toml",
			expectedRandomBeaconAddress:            "0xcf64c2a367341170cb4e09cf8c0ed137d8473ceb",
			expectedWalletRegistryAddress:          "0x143ba24e66fce8bca22f7d739f9a932c519b1c76",
			expectedTokenStakingAddress:            "0xa363a197f1bbb8877f50350234e3f15fb4175457",
			expectedBridgeAddress:                  "0x138D2a0c87BA9f6BE1DCc13D6224A6aCE9B6b6F0",
			expectedWalletProposalValidatorAddress: "0xfdc315b0e608b7cDE9166D9D69a1506779e3E0CA",
		},
		"mxied contracts addresses configured": {
			configFilePath:                         "../test/config_mixed_contracts.toml",
			expectedRandomBeaconAddress:            "0xd1640b381327c2d5425d6d3d605539a3db72f857",
			expectedWalletRegistryAddress:          "0x143ba24e66fce8bca22f7d739f9a932c519b1c76",
			expectedTokenStakingAddress:            "0xaa7b41039ea8f9ec2d89bbe96e19f97b6c267a27",
			expectedBridgeAddress:                  "0x9490165195503fcf6a0fd20ac113223fefb66ed5",
			expectedWalletProposalValidatorAddress: "0xE7d33d8AA55B73a93059a24b900366894684a497",
		},
	}

	for testName, test := range configReadTests {

		t.Run(testName, func(t *testing.T) {
			cfg := &Config{}

			validate := func(contractName string, expectedAddress string) {
				actualAddress, err := cfg.Ethereum.ContractAddress(contractName)
				if err != nil {
					t.Fatalf("failed to get %s address: [%v]", contractName, err)
				}

				if actualAddress != common.HexToAddress(expectedAddress) {
					t.Errorf(
						"invalid %s address\nexpected: %s\nactual:   %s",
						contractName,
						expectedAddress,
						actualAddress,
					)
				}

			}

			if err := cfg.ReadConfig(test.configFilePath, nil, AllCategories...); err != nil {
				t.Fatalf("failed to read test config: [%v]", err)
			}

			validate(ethereum.RandomBeaconContractName, test.expectedRandomBeaconAddress)
			validate(ethereum.WalletRegistryContractName, test.expectedWalletRegistryAddress)
			validate(ethereum.TokenStakingContractName, test.expectedTokenStakingAddress)
			validate(ethereum.BridgeContractName, test.expectedBridgeAddress)
			validate(ethereum.WalletProposalValidatorContractName, test.expectedWalletProposalValidatorAddress)
		})
	}
}
