package ethereum

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Config is a struct that contains the configuration needed to connect to an
// Ethereum node. This information will give access to an Ethereum network.
type Config struct {
	Network

	Account `mapstructure:",squash"`

	// Example: "ws://192.168.0.157:8546".
	URL string

	// Example: "http://192.168.0.157:8545".
	URLRPC string

	// A map from contract names to contract addresses. The keys in the map are
	// expected to be lowercase contract names.
	ContractAddresses map[string]string

	// MiningCheckInterval is the interval in which transaction
	// mining status is checked. If the transaction is not mined within this
	// time, the gas price is increased and transaction is resubmitted.
	MiningCheckInterval time.Duration

	// RequestsPerSecondLimit sets the maximum average number of requests
	// per second which can be executed against the Ethereum node.
	// All types of chain requests are rate-limited,
	// including view function calls.
	RequestsPerSecondLimit int

	// ConcurrencyLimit sets the maximum number of concurrent requests which
	// can be executed against the Ethereum node at the same time.
	// This limit affects all types of chain requests,
	// including view function calls.
	ConcurrencyLimit int

	// MaxGasFeeCap specifies the maximum gas fee cap the client is
	// willing to pay for the transaction to be mined. The offered transaction
	// gas cost can not be higher than the max gas fee cap value. If the maximum
	// allowed gas fee cap is reached, no further resubmission attempts are
	// performed. This value is used for all types of Ethereum transactions.
	// For legacy transactions, this value works as a maximum gas price
	// and for EIP-1559 transactions, this value works as max gas fee cap.
	MaxGasFeeCap Wei

	// BalanceAlertThreshold defines a minimum value of the operator's
	// account balance below which an alert will be triggered.
	BalanceAlertThreshold Wei
}

// Account is a struct that contains the configuration for accessing an
// Ethereum network and a contract on the network.
type Account struct {
	// Keyfile is a full path to a key file.  Normally this file is one of the
	// imported keys in your local chain node.  It can normally be found in
	// a directory <some-path>/data/keystore/ and starts with its creation date
	// "UTC--.*".
	KeyFile string

	// KeyFilePassword is the password used to unlock the account specified in
	// KeyFile.
	KeyFilePassword string
}

// ErrAddressNotConfigured is an error that is returned when an address for the given
// contract name was not found in the Config.
var ErrAddressNotConfigured = errors.New("address not configured")

// ContractAddress finds a given contract's address configuration and returns it
// as Ethereum address.
func (c *Config) ContractAddress(contractName string) (common.Address, error) {
	addressString, exists := c.ContractAddresses[strings.ToLower(contractName)]
	if !exists || addressString == "" {
		return common.Address{}, ErrAddressNotConfigured
	}

	if !common.IsHexAddress(addressString) {
		return common.Address{}, fmt.Errorf(
			"configured address [%v] for contract [%v] "+
				"is not valid hex address",
			addressString,
			contractName,
		)
	}

	address := common.HexToAddress(addressString)
	return address, nil
}

// SetContractAddress sets address for a contract in the contracts addresses
// mapping.
func (c *Config) SetContractAddress(contractName string, address string) {
	c.ContractAddresses[strings.ToLower(contractName)] = address
}
