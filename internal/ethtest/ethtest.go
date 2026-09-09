// Package ethtest serves a deterministic Ethereum JSON-RPC endpoint so tests
// can drive the production contract bindings, chain handle constructors and
// admission predicates without a live chain behind them.
//
// Every read is routed by contract address, matched against the generated
// contract ABI by method selector, decoded and recorded. A test can therefore
// assert what was actually asked of the chain - which contract, which method,
// with which arguments and how many times - rather than what a stand-in was
// told to answer. Anything the fixture cannot attribute to a registered
// contract method is recorded as unexpected and answered with an error.
package ethtest

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	chainethereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
)

// Contract names the fixture serves reads for. They double as the keys the
// recorded calls are attributed to. The names are spelled out here rather than
// taken from the chain handle package because that package's own tests use
// this fixture, and importing it back would close an import cycle.
const (
	BridgeContract                  = "Bridge"
	MaintainerProxyContract         = "MaintainerProxy"
	WalletProposalValidatorContract = "WalletProposalValidator"
	WalletRegistryContract          = "WalletRegistry"
	RandomBeaconContract            = "RandomBeacon"
	TokenStakingContract            = "TokenStaking"
	AllowlistContract               = "Allowlist"
)

// Addresses the fixture deploys its contracts at. They are all distinct so
// that every read can be attributed to exactly one contract. TokenStaking and
// Allowlist are deliberately separate: rolesOf has to be read from token
// staking, and a read landing on the allowlist instead must be observable
// rather than silently answered.
var (
	BridgeAddress                  = common.HexToAddress("0x0000000000000000000000000000000000000101")
	MaintainerProxyAddress         = common.HexToAddress("0x0000000000000000000000000000000000000102")
	WalletProposalValidatorAddress = common.HexToAddress("0x0000000000000000000000000000000000000103")
	WalletRegistryAddress          = common.HexToAddress("0x0000000000000000000000000000000000000104")
	EcdsaSortitionPoolAddress      = common.HexToAddress("0x0000000000000000000000000000000000000105")
	RandomBeaconAddress            = common.HexToAddress("0x0000000000000000000000000000000000000106")
	BeaconSortitionPoolAddress     = common.HexToAddress("0x0000000000000000000000000000000000000107")
	TokenStakingAddress            = common.HexToAddress("0x0000000000000000000000000000000000000108")
	AllowlistAddress               = common.HexToAddress("0x0000000000000000000000000000000000000109")
	BankAddress                    = common.HexToAddress("0x000000000000000000000000000000000000010a")
	RelayAddress                   = common.HexToAddress("0x000000000000000000000000000000000000010b")
	ReimbursementPoolAddress       = common.HexToAddress("0x000000000000000000000000000000000000010c")
)

// keyFileUnlockPhrase unlocks the generated operator key file. The key it
// protects is derived from a fixed scalar and holds nothing.
const keyFileUnlockPhrase = "deterministic-fixture-phrase"

// defaultBlockHash is what a served block reports as its own hash unless a
// test seeds another. It is an arbitrary constant and is deliberately not the
// hash of anything the fixture serves.
const defaultBlockHash = "0x8f8b1c9a6ad3fa2c0d5e4b7a19836c2e5d0f47b8a1c93e6d24f70b5a8c31e9d6"

// Roles mirrors the tuple TokenStaking.rolesOf returns.
type Roles struct {
	Owner       common.Address
	Beneficiary common.Address
	Authorizer  common.Address
}

// State is the chain state the fixture answers reads from. Absent map entries
// read as the zero value the corresponding contract would return, so a test
// only has to seed what it wants to differ from an unknown address.
type State struct {
	// ChainID is reported by eth_chainId and must match the configured
	// network for the chain handle constructors to accept the endpoint.
	ChainID *big.Int
	// BlockNumber is the height every served block reports.
	BlockNumber uint64
	// BlockHash is the hash the served block reports as its own. It is a
	// standalone value rather than a digest of the served fields, so a reader
	// that recomputes the hash instead of reading it back cannot agree with
	// it.
	BlockHash common.Hash
	// BlockFields overrides or adds raw JSON members of the served block, so a
	// test can serve fields the pinned header type does not model, or replace
	// one the fixture would otherwise report.
	BlockFields map[string]json.RawMessage
	// BlockMissing makes the endpoint answer a null block, the way a node
	// answers for a height it does not have.
	BlockMissing bool

	// WalletRegistry state.
	TbtcOperators        map[common.Address]common.Address
	EligibleStakes       map[common.Address]*big.Int
	PendingDecreases     map[common.Address]*big.Int
	MinimumAuthorization *big.Int

	// RandomBeacon state.
	BeaconOperators map[common.Address]common.Address

	// TokenStaking state.
	Roles map[common.Address]Roles
}

func (s *State) normalize() {
	if s.ChainID == nil {
		s.ChainID = big.NewInt(chainethereum.Mainnet.ChainID())
	}
	if s.BlockNumber == 0 {
		s.BlockNumber = 1
	}
	if s.BlockHash == (common.Hash{}) {
		s.BlockHash = common.HexToHash(defaultBlockHash)
	}
	if s.MinimumAuthorization == nil {
		s.MinimumAuthorization = TTokens(40_000)
	}
	if s.TbtcOperators == nil {
		s.TbtcOperators = map[common.Address]common.Address{}
	}
	if s.EligibleStakes == nil {
		s.EligibleStakes = map[common.Address]*big.Int{}
	}
	if s.PendingDecreases == nil {
		s.PendingDecreases = map[common.Address]*big.Int{}
	}
	if s.BeaconOperators == nil {
		s.BeaconOperators = map[common.Address]common.Address{}
	}
	if s.Roles == nil {
		s.Roles = map[common.Address]Roles{}
	}
}

// TTokens returns the given whole number of T in the 18-decimal base unit
// authorization amounts are expressed in.
func TTokens(amount int64) *big.Int {
	return new(big.Int).Mul(
		big.NewInt(amount),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
}

// Key returns the chain private key derived from the given index. Deriving
// keys from a fixed scalar keeps every address in a test run reproducible;
// these keys are synthetic and correspond to no address holding anything.
func Key(t *testing.T, index int64) *ecdsa.PrivateKey {
	t.Helper()

	if index <= 0 {
		t.Fatalf("key index [%d] must be positive", index)
	}

	privateKey, err := crypto.ToECDSA(
		common.LeftPadBytes(big.NewInt(index).Bytes(), 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	return privateKey
}

// Address returns the chain address of the key at the given index.
func Address(t *testing.T, index int64) common.Address {
	t.Helper()

	return crypto.PubkeyToAddress(Key(t, index).PublicKey)
}

// ChainConfig returns the configuration pointing the production chain handle
// constructors at this fixture. The EcdsaDkgValidator address is deliberately
// left unconfigured so construction takes its optional-contract path.
func (b *Backend) ChainConfig(t *testing.T) chainethereum.Config {
	t.Helper()

	config := chainethereum.Config{
		Network: chainethereum.Mainnet,
		URL:     b.URL(),
		Account: chainethereum.Account{
			KeyFile:         b.keyFile,
			KeyFilePassword: keyFileUnlockPhrase,
		},
		ContractAddresses: map[string]string{},
	}

	config.SetContractAddress(TokenStakingContract, TokenStakingAddress.Hex())
	config.SetContractAddress(RandomBeaconContract, RandomBeaconAddress.Hex())
	config.SetContractAddress(BridgeContract, BridgeAddress.Hex())
	config.SetContractAddress(
		MaintainerProxyContract,
		MaintainerProxyAddress.Hex(),
	)
	config.SetContractAddress(
		WalletProposalValidatorContract,
		WalletProposalValidatorAddress.Hex(),
	)

	return config
}

// writeKeyFile encrypts the fixture's operator key into a keystore file under
// the given directory and returns its path.
func writeKeyFile(t *testing.T, directory string) string {
	t.Helper()

	store := keystore.NewKeyStore(
		directory,
		keystore.LightScryptN,
		keystore.LightScryptP,
	)

	account, err := store.ImportECDSA(Key(t, operatorKeyIndex), keyFileUnlockPhrase)
	if err != nil {
		t.Fatalf("failed to write the operator key file: %v", err)
	}

	return account.URL.Path
}
