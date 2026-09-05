//go:build integration
// +build integration

package ethereum

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/internal/testutils"
	beaconabi "github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/abi"
	ecdsaabi "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	thresholdabi "github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen/abi"
)

// To run the tests execute:
// ETHEREUM_MAINNET_RPC_URL=<url> go test -v -tags=integration ./...
//
// The URL MUST point to Ethereum mainnet (not a testnet): the test asserts
// against well-known historical mainnet block numbers and will silently
// fail against any other network.
//
// The Infura URL previously hardcoded here was checked into the repository
// and must be treated as compromised; rotate the key on the provider side
// before reusing it.

func TestBaseChain_GetBlockNumberByTimestamp(t *testing.T) {
	ethereumURL := os.Getenv("ETHEREUM_MAINNET_RPC_URL")
	if ethereumURL == "" {
		t.Skip("ETHEREUM_MAINNET_RPC_URL not set; skipping integration test")
	}

	client, err := ethclient.Dial(ethereumURL)
	if err != nil {
		t.Fatal(err)
	}

	blockCounter, err := ethutil.NewBlockCounter(client)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize the baseChain with fields required by this scenario.
	bc := &baseChain{
		client:       client,
		blockCounter: blockCounter,
	}

	var tests = map[string]struct {
		timestamp           uint64
		expectedBlockNumber uint64
		expectedError       error
	}{
		"there is a block at the requested timestamp": {
			timestamp:           1681982135, // 20 April 2023 09:15:35
			expectedBlockNumber: 17086765,
		},
		"there is a block just after the requested timestamp": {
			timestamp:           1681982133, // 20 April 2023 09:15:33
			expectedBlockNumber: 17086765,
		},
		"there is a block just before the requested timestamp": {
			timestamp:           1681982125, // 20 April 2023 09:15:25
			expectedBlockNumber: 17086764,
		},
		"there are two blocks with equal distance to the requested timestamp": {
			timestamp:           1681982129, // 20 April 2023 09:15:29
			expectedBlockNumber: 17086765,
		},
		"the requested timestamp is in the future": {
			timestamp:     uint64(time.Now().Add(1 * time.Hour).Unix()),
			expectedError: fmt.Errorf("requested timestamp is in the future"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			blockNumber, err := bc.GetBlockNumberByTimestamp(test.timestamp)
			if shouldSkipEthereumIntegrationError(err) {
				t.Skipf("skipping due to transient Ethereum provider error: %v", err)
			}

			if !reflect.DeepEqual(err, test.expectedError) {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]\n",
					test.expectedError,
					err,
				)
			}

			testutils.AssertIntsEqual(
				t,
				"block number",
				int(test.expectedBlockNumber),
				int(blockNumber),
			)
		})
	}
}

func shouldSkipEthereumIntegrationError(err error) bool {
	if err == nil {
		return false
	}

	errorMessage := err.Error()

	return strings.Contains(errorMessage, "429 Too Many Requests") ||
		strings.Contains(errorMessage, "\"message\":\"Too Many Requests\"")
}

// The block the admission assertions below read at. Pinning it makes them
// describe one fixed chain state instead of whatever mainnet happens to hold
// when they run, so they need an archive endpoint rather than a pruned one.
const admissionPinnedBlock = 25908000

// Mainnet deployment addresses, as recorded under solidity/*/deployments/mainnet
// and solidity/random-beacon/external/mainnet.
const (
	mainnetWalletRegistryAddress = "0x46d52E41C2F300BC82217Ce22b920c34995204eb"
	mainnetRandomBeaconAddress   = "0x5499f54b4A1CB4816eefCf78962040461be3D80b"
	mainnetTokenStakingAddress   = "0x01B67b1194C75264d06F808A921228a95C765dd7"
)

// allowlistWeights mirrors the fields this file reads out of the ECDSA
// allowlist deploy data.
type allowlistWeights struct {
	Operators []struct {
		Identification  string `json:"identification"`
		StakingProvider string `json:"stakingProvider"`
		Operator        string `json:"operator"`
	} `json:"operators"`
	DeprecatedOperatorsNotAdded []struct {
		Identification  string `json:"identification"`
		StakingProvider string `json:"stakingProvider"`
		Operator        string `json:"operator"`
	} `json:"deprecatedOperatorsNotAdded"`
}

func readAllowlistWeights(t *testing.T) *allowlistWeights {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(
		"..", "..", "..",
		"solidity", "ecdsa", "deploy-data", "allowlist-weights-mainnet.json",
	))
	if err != nil {
		t.Fatal(err)
	}

	weights := &allowlistWeights{}
	if err := json.Unmarshal(content, weights); err != nil {
		t.Fatal(err)
	}

	return weights
}

// admissionCallers holds the three contracts the combined admission predicate
// reads, bound read-only at the pinned block.
type admissionCallers struct {
	walletRegistry *ecdsaabi.WalletRegistryCaller
	randomBeacon   *beaconabi.RandomBeaconCaller
	tokenStaking   *thresholdabi.TokenStakingCaller
	callOpts       *bind.CallOpts
}

func newAdmissionCallers(t *testing.T) *admissionCallers {
	t.Helper()

	ethereumURL := os.Getenv("ETHEREUM_MAINNET_RPC_URL")
	if ethereumURL == "" {
		t.Skip("ETHEREUM_MAINNET_RPC_URL not set; skipping integration test")
	}

	client, err := ethclient.Dial(ethereumURL)
	if err != nil {
		t.Fatal(err)
	}

	walletRegistry, err := ecdsaabi.NewWalletRegistryCaller(
		common.HexToAddress(mainnetWalletRegistryAddress),
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	randomBeacon, err := beaconabi.NewRandomBeaconCaller(
		common.HexToAddress(mainnetRandomBeaconAddress),
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	tokenStaking, err := thresholdabi.NewTokenStakingCaller(
		common.HexToAddress(mainnetTokenStakingAddress),
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	return &admissionCallers{
		walletRegistry: walletRegistry,
		randomBeacon:   randomBeacon,
		tokenStaking:   tokenStaking,
		callOpts: &bind.CallOpts{
			BlockNumber: big.NewInt(admissionPinnedBlock),
		},
	}
}

// TestTbtcChain_IsRecognized_DeprecatedOperatorsKeepBeaconAdmission reads the
// operators the ECDSA allowlist deliberately left out. The tBTC predicate
// rejects every one of them, and every one of them stays admitted through the
// beacon branch, which is why the beacon must keep its own predicate: the
// change de-admits nobody in production.
func TestTbtcChain_IsRecognized_DeprecatedOperatorsKeepBeaconAdmission(t *testing.T) {
	callers := newAdmissionCallers(t)
	weights := readAllowlistWeights(t)

	if len(weights.DeprecatedOperatorsNotAdded) == 0 {
		t.Fatal("no deprecated operators found in the allowlist deploy data")
	}

	for _, deprecated := range weights.DeprecatedOperatorsNotAdded {
		t.Run(deprecated.StakingProvider, func(t *testing.T) {
			stakingProvider := common.HexToAddress(deprecated.StakingProvider)
			operatorAddress := common.HexToAddress(deprecated.Operator)

			eligibleStake, err := callers.walletRegistry.EligibleStake(
				callers.callOpts,
				stakingProvider,
			)
			if err != nil {
				if shouldSkipEthereumIntegrationError(err) {
					t.Skip(err)
				}
				t.Fatal(err)
			}

			if eligibleStake.Sign() != 0 {
				t.Errorf(
					"expected the tbtc branch to reject, but eligible stake "+
						"is [%v]",
					eligibleStake,
				)
			}

			beaconStakingProvider, err := callers.randomBeacon.OperatorToStakingProvider(
				callers.callOpts,
				operatorAddress,
			)
			if err != nil {
				t.Fatal(err)
			}

			if (beaconStakingProvider == common.Address{}) {
				t.Error("expected the operator to be known to the beacon")
			}

			roles, err := callers.tokenStaking.RolesOf(
				callers.callOpts,
				beaconStakingProvider,
			)
			if err != nil {
				t.Fatal(err)
			}

			if (roles.Owner == common.Address{}) {
				t.Error("expected the beacon branch to keep admitting")
			}
		})
	}
}

// TestBeaconChain_EligibleStakeIsZeroForEveryKnownProvider is the reason the
// beacon keeps the legacy delegation predicate. Token staking reports no
// authorized stake for the beacon, so beacon eligible stake is zero for every
// provider in the deploy data. Were the beacon given the tBTC predicate it
// would recognize nobody and the watchtower would take the fleet apart within
// one round.
func TestBeaconChain_EligibleStakeIsZeroForEveryKnownProvider(t *testing.T) {
	callers := newAdmissionCallers(t)
	weights := readAllowlistWeights(t)

	stakingProviders := make([]string, 0)
	for _, operator := range weights.Operators {
		stakingProviders = append(stakingProviders, operator.StakingProvider)
	}
	for _, deprecated := range weights.DeprecatedOperatorsNotAdded {
		stakingProviders = append(stakingProviders, deprecated.StakingProvider)
	}

	if len(stakingProviders) == 0 {
		t.Fatal("no staking providers found in the allowlist deploy data")
	}

	for _, stakingProvider := range stakingProviders {
		t.Run(stakingProvider, func(t *testing.T) {
			eligibleStake, err := callers.randomBeacon.EligibleStake(
				callers.callOpts,
				common.HexToAddress(stakingProvider),
			)
			if err != nil {
				if shouldSkipEthereumIntegrationError(err) {
					t.Skip(err)
				}
				t.Fatal(err)
			}

			testutils.AssertBigIntsEqual(
				t,
				"beacon eligible stake",
				big.NewInt(0),
				eligibleStake,
			)
		})
	}
}

// TestTbtcChain_IsRecognized_PendingDecreaseAtTheFloor guards the mechanic that
// makes the predicate surprising: a requested weight decrease is applied to
// eligible stake the moment it is requested, and an authorization cannot be
// lowered past the minimum without going to zero outright. Providers sitting on
// that floor must stay recognized.
func TestTbtcChain_IsRecognized_PendingDecreaseAtTheFloor(t *testing.T) {
	callers := newAdmissionCallers(t)
	weights := readAllowlistWeights(t)

	minimumAuthorization, err := callers.walletRegistry.MinimumAuthorization(
		callers.callOpts,
	)
	if err != nil {
		if shouldSkipEthereumIntegrationError(err) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	providersAtTheFloor := 0

	for _, operator := range weights.Operators {
		eligibleStake, err := callers.walletRegistry.EligibleStake(
			callers.callOpts,
			common.HexToAddress(operator.StakingProvider),
		)
		if err != nil {
			t.Fatal(err)
		}

		if eligibleStake.Cmp(minimumAuthorization) != 0 {
			continue
		}

		providersAtTheFloor++
	}

	// The filter above leaves eligible stake equal to the minimum authorization,
	// so asserting it is positive per provider would be a tautology. The
	// invariant worth guarding is the floor itself: the predicate admits on
	// eligible stake being above zero, so a zero minimum authorization would
	// silently reject every provider sitting on it.
	if minimumAuthorization.Sign() <= 0 {
		t.Errorf(
			"minimum authorization is [%v] at block %d; providers sitting on "+
				"the floor would read zero eligible stake and lose admission",
			minimumAuthorization,
			admissionPinnedBlock,
		)
	}

	if providersAtTheFloor == 0 {
		t.Fatalf(
			"no provider sits on the minimum authorization at block %d; the "+
				"case this guards no longer exists at the pinned block",
			admissionPinnedBlock,
		)
	}
}

// TestTbtcChain_IsRecognized_ProviderAuthorizedAfterLegacyStakingFroze is the
// case the change exists for. Legacy token staking can no longer record a
// delegation for anyone, so a provider authorized after it froze reads a zero
// owner forever while holding full eligible stake: the delegation predicate
// rejects it permanently and the eligible stake predicate admits it. It has no
// beacon backstop either, which is the cost the change carries.
func TestTbtcChain_IsRecognized_ProviderAuthorizedAfterLegacyStakingFroze(t *testing.T) {
	callers := newAdmissionCallers(t)

	stakingProvider := common.HexToAddress(
		"0x3d921565ec837c6bfaf441579e07571646e0048a",
	)

	eligibleStake, err := callers.walletRegistry.EligibleStake(
		callers.callOpts,
		stakingProvider,
	)
	if err != nil {
		if shouldSkipEthereumIntegrationError(err) {
			t.Skip(err)
		}
		t.Fatal(err)
	}

	if eligibleStake.Sign() <= 0 {
		t.Fatalf(
			"expected the provider to hold eligible stake, got [%v]",
			eligibleStake,
		)
	}

	roles, err := callers.tokenStaking.RolesOf(callers.callOpts, stakingProvider)
	if err != nil {
		t.Fatal(err)
	}

	if (roles.Owner != common.Address{}) {
		t.Errorf(
			"expected no legacy delegation, got owner [%v]",
			roles.Owner,
		)
	}

	beaconStakingProvider, err := callers.randomBeacon.OperatorToStakingProvider(
		callers.callOpts,
		common.HexToAddress("0xc1E20a88C2130472B25B3C382773bA85944230d2"),
	)
	if err != nil {
		t.Fatal(err)
	}

	beaconRoles, err := callers.tokenStaking.RolesOf(
		callers.callOpts,
		beaconStakingProvider,
	)
	if err != nil {
		t.Fatal(err)
	}

	if (beaconRoles.Owner != common.Address{}) {
		t.Error("expected the provider to have no beacon backstop")
	}
}
