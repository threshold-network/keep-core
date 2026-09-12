//go:build integration
// +build integration

package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
			if isRateLimited(err) {
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

// isRateLimited reports whether an endpoint refused a read because it is
// throttling the caller, rather than because of anything the read asked for.
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}

	errorMessage := err.Error()

	return strings.Contains(errorMessage, "429 Too Many Requests") ||
		strings.Contains(errorMessage, "\"message\":\"Too Many Requests\"")
}

// An endpoint that rate-limits a read is asked again rather than taken as
// permission to stop asking: an assertion skipped part-way through reads the
// same as one that ran and found nothing wrong. The bound is small, so an
// endpoint that is genuinely throttling the run fails these tests instead of
// stalling them.
const (
	rateLimitedAttempts   = 6
	rateLimitedFirstDelay = 250 * time.Millisecond
)

// withRetries invokes fn, retrying only while the endpoint is rate-limiting
// it, and returns whatever the last attempt produced.
func withRetries[T any](fn func() (T, error)) (T, error) {
	delay := rateLimitedFirstDelay

	for attempt := 1; ; attempt++ {
		result, err := fn()
		if err == nil ||
			attempt == rateLimitedAttempts ||
			!isRateLimited(err) {
			return result, err
		}

		time.Sleep(delay)
		delay *= 2
	}
}

// callOrFail invokes fn through withRetries and fails the test if it still
// cannot be answered; every chain read behind an admission assertion goes
// through this helper.
func callOrFail[T any](t *testing.T, fn func() (T, error)) T {
	t.Helper()

	result, err := withRetries(fn)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

const (
	// The block every admission read below is taken at. Pinning it makes these
	// assertions describe one fixed chain state instead of whatever mainnet
	// happens to hold when they run, so they need an archive endpoint rather
	// than a pruned one. Both the state reads and the upper bound of the
	// registration histories use it.
	admissionPinnedBlock = 25937431

	// The hash mainnet records for admissionPinnedBlock. Reading it back
	// proves the endpoint served the chain, and the block, the numbers below
	// were taken from.
	admissionPinnedBlockHash = "0xcffd80c8013f3a65c29eb2355ce58fe01cc98e7eb5123352e81907adc77fed36"

	mainnetChainID = 1

	// The blocks the two registries were deployed at, taken from their mainnet
	// deployment records. Their registration histories start here.
	walletRegistryDeploymentBlock = 15639521
	randomBeaconDeploymentBlock   = 15638933
)

// Mainnet deployment addresses, as recorded under solidity/*/deployments/mainnet
// and solidity/random-beacon/external/mainnet.
const (
	mainnetWalletRegistryAddress = "0x46d52E41C2F300BC82217Ce22b920c34995204eb"
	mainnetRandomBeaconAddress   = "0x5499f54b4A1CB4816eefCf78962040461be3D80b"
	mainnetTokenStakingAddress   = "0x01B67b1194C75264d06F808A921228a95C765dd7"
)

// zeroAddress is what a registry reports for an operator it holds no mapping
// for, and what token staking reports for a provider that never delegated.
var zeroAddress = common.Address{}

// allowlistOperatorEntry mirrors one entry of the ECDSA allowlist deploy
// data's operator lists.
type allowlistOperatorEntry struct {
	Identification  string `json:"identification"`
	StakingProvider string `json:"stakingProvider"`
	Operator        string `json:"operator"`
}

// allowlistWeights mirrors the fields this file reads out of the ECDSA
// allowlist deploy data. Only the deliberately excluded operators are read
// from it; the registered population every other assertion runs over is built
// from the registration histories on chain rather than from deploy data.
type allowlistWeights struct {
	DeprecatedOperatorsNotAdded []allowlistOperatorEntry `json:"deprecatedOperatorsNotAdded"`
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

// admissionCallers holds the contracts the combined admission predicate reads,
// bound read-only at the pinned block, together with the log readers the
// registered population is built from.
type admissionCallers struct {
	walletRegistry     *ecdsaabi.WalletRegistryCaller
	randomBeacon       *beaconabi.RandomBeaconCaller
	tokenStaking       *thresholdabi.TokenStakingCaller
	walletRegistryLogs *ecdsaabi.WalletRegistryFilterer
	randomBeaconLogs   *beaconabi.RandomBeaconFilterer
	callOpts           *bind.CallOpts
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

	walletRegistryAddress := common.HexToAddress(mainnetWalletRegistryAddress)
	randomBeaconAddress := common.HexToAddress(mainnetRandomBeaconAddress)

	walletRegistry, err := ecdsaabi.NewWalletRegistryCaller(
		walletRegistryAddress,
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	randomBeacon, err := beaconabi.NewRandomBeaconCaller(
		randomBeaconAddress,
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

	walletRegistryLogs, err := ecdsaabi.NewWalletRegistryFilterer(
		walletRegistryAddress,
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	randomBeaconLogs, err := beaconabi.NewRandomBeaconFilterer(
		randomBeaconAddress,
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Anchor the endpoint before reading anything off it. A pinned block
	// number means nothing on its own: another chain has a block at that
	// height too, and it would answer every read below with different values.
	chainID := callOrFail(t, func() (*big.Int, error) {
		return client.ChainID(context.Background())
	})
	testutils.AssertBigIntsEqual(
		t,
		"chain id",
		big.NewInt(mainnetChainID),
		chainID,
	)

	anchor := callOrFail(t, func() (anchorBlock, error) {
		return readAnchorBlock(
			context.Background(),
			client.Client(),
			admissionPinnedBlock,
		)
	})
	testutils.AssertStringsEqual(
		t,
		fmt.Sprintf("hash of block %d", admissionPinnedBlock),
		admissionPinnedBlockHash,
		anchor.hash,
	)
	testutils.AssertUintsEqual(
		t,
		"anchored block number",
		admissionPinnedBlock,
		anchor.number,
	)

	return &admissionCallers{
		walletRegistry:     walletRegistry,
		randomBeacon:       randomBeacon,
		tokenStaking:       tokenStaking,
		walletRegistryLogs: walletRegistryLogs,
		randomBeaconLogs:   randomBeaconLogs,
		callOpts: &bind.CallOpts{
			BlockNumber: big.NewInt(admissionPinnedBlock),
		},
	}
}

func (ac *admissionCallers) rolesOwner(
	t *testing.T,
	stakingProvider common.Address,
) common.Address {
	t.Helper()

	if stakingProvider == zeroAddress {
		return zeroAddress
	}

	roles := callOrFail(t, func() (struct {
		Owner       common.Address
		Beneficiary common.Address
		Authorizer  common.Address
	}, error) {
		return ac.tokenStaking.RolesOf(ac.callOpts, stakingProvider)
	})

	return roles.Owner
}

// registration is one OperatorRegistered event: an operator address and the
// staking provider that registered it.
type registration struct {
	stakingProvider common.Address
	operator        common.Address
}

// registrationLogSpan is the block range one OperatorRegistered query covers.
// The histories run over roughly ten million blocks and endpoints cap either
// the span or the result count, so they are walked in pieces; a piece an
// endpoint refuses is retried in halves down to registrationLogSpanFloor.
const (
	registrationLogSpan      = 500_000
	registrationLogSpanFloor = 10_000
)

// collectRegistrations walks one registry's OperatorRegistered history from
// its deployment up to the pinned block and returns every event in it.
func collectRegistrations(
	t *testing.T,
	registry string,
	deploymentBlock uint64,
	query func(*bind.FilterOpts) ([]registration, error),
) []registration {
	t.Helper()

	registrations := make([]registration, 0)

	for start := deploymentBlock; start <= admissionPinnedBlock; {
		span := uint64(registrationLogSpan)

		for {
			end := start + span - 1
			if end > admissionPinnedBlock {
				end = admissionPinnedBlock
			}

			found, err := withRetries(func() ([]registration, error) {
				return query(&bind.FilterOpts{Start: start, End: &end})
			})
			if err == nil {
				registrations = append(registrations, found...)
				start = end + 1
				break
			}

			if span <= registrationLogSpanFloor {
				t.Fatalf(
					"failed to read the %s registration history over blocks "+
						"%d-%d: %v",
					registry,
					start,
					end,
					err,
				)
			}

			span /= 2
		}
	}

	return registrations
}

func walletRegistryRegistrations(
	t *testing.T,
	callers *admissionCallers,
) []registration {
	t.Helper()

	return collectRegistrations(
		t,
		"wallet registry",
		walletRegistryDeploymentBlock,
		func(opts *bind.FilterOpts) ([]registration, error) {
			iterator, err := callers.walletRegistryLogs.FilterOperatorRegistered(
				opts,
				nil,
				nil,
			)
			if err != nil {
				return nil, err
			}
			defer iterator.Close()

			found := make([]registration, 0)
			for iterator.Next() {
				found = append(found, registration{
					stakingProvider: iterator.Event.StakingProvider,
					operator:        iterator.Event.Operator,
				})
			}

			return found, iterator.Error()
		},
	)
}

func randomBeaconRegistrations(
	t *testing.T,
	callers *admissionCallers,
) []registration {
	t.Helper()

	return collectRegistrations(
		t,
		"random beacon",
		randomBeaconDeploymentBlock,
		func(opts *bind.FilterOpts) ([]registration, error) {
			iterator, err := callers.randomBeaconLogs.FilterOperatorRegistered(
				opts,
				nil,
				nil,
			)
			if err != nil {
				return nil, err
			}
			defer iterator.Close()

			found := make([]registration, 0)
			for iterator.Next() {
				found = append(found, registration{
					stakingProvider: iterator.Event.StakingProvider,
					operator:        iterator.Event.Operator,
				})
			}

			return found, iterator.Error()
		},
	)
}

// operatorUnion returns the distinct operator addresses appearing in the given
// registration histories, sorted so the population is walked in a stable order.
func operatorUnion(histories ...[]registration) []common.Address {
	seen := make(map[common.Address]bool)
	union := make([]common.Address, 0)

	for _, history := range histories {
		for _, entry := range history {
			if seen[entry.operator] {
				continue
			}
			seen[entry.operator] = true
			union = append(union, entry.operator)
		}
	}

	sort.Slice(union, func(i, j int) bool {
		return strings.ToLower(union[i].Hex()) < strings.ToLower(union[j].Hex())
	})

	return union
}

// stakingProviderSet returns the distinct staking providers in a registration
// history, sorted for a stable walk.
func stakingProviderSet(history []registration) []common.Address {
	seen := make(map[common.Address]bool)
	providers := make([]common.Address, 0)

	for _, entry := range history {
		if seen[entry.stakingProvider] {
			continue
		}
		seen[entry.stakingProvider] = true
		providers = append(providers, entry.stakingProvider)
	}

	sort.Slice(providers, func(i, j int) bool {
		return strings.ToLower(providers[i].Hex()) <
			strings.ToLower(providers[j].Hex())
	})

	return providers
}

// admissionFacts is the anchored chain state one registered operator address
// is judged on. Every field is read at admissionPinnedBlock.
type admissionFacts struct {
	operator common.Address

	tbtcStakingProvider   common.Address
	beaconStakingProvider common.Address

	eligibleStake   *big.Int
	pendingDecrease *big.Int

	tbtcOwner   common.Address
	beaconOwner common.Address
}

// beaconRecognized is the beacon branch, unchanged by this work: a registered
// operator whose staking provider has, or ever had, a stake delegation.
func (f admissionFacts) beaconRecognized() bool {
	return f.beaconStakingProvider != zeroAddress && f.beaconOwner != zeroAddress
}

// baselineTbtcRecognized is the tBTC branch as it stands on the merge base:
// the same stake-delegation predicate the beacon uses, read through the wallet
// registry's own operator mapping.
func (f admissionFacts) baselineTbtcRecognized() bool {
	return f.tbtcStakingProvider != zeroAddress && f.tbtcOwner != zeroAddress
}

// originalHeadTbtcRecognized is the tBTC branch as first proposed: eligible
// stake, or a pending authorization decrease standing in for it.
func (f admissionFacts) originalHeadTbtcRecognized() bool {
	return f.tbtcStakingProvider != zeroAddress &&
		(f.eligibleStake.Sign() > 0 || f.pendingDecrease.Sign() > 0)
}

// proposedTbtcRecognized is the tBTC branch this change settles on: eligible
// stake alone.
func (f admissionFacts) proposedTbtcRecognized() bool {
	return f.tbtcStakingProvider != zeroAddress && f.eligibleStake.Sign() > 0
}

// readAdmissionFacts reads the anchored state of every operator in the
// population.
func readAdmissionFacts(
	t *testing.T,
	callers *admissionCallers,
	population []common.Address,
) []admissionFacts {
	t.Helper()

	facts := make([]admissionFacts, 0, len(population))

	for _, operator := range population {
		entry := admissionFacts{
			operator:        operator,
			eligibleStake:   big.NewInt(0),
			pendingDecrease: big.NewInt(0),
		}

		entry.tbtcStakingProvider = callOrFail(t, func() (common.Address, error) {
			return callers.walletRegistry.OperatorToStakingProvider(
				callers.callOpts,
				operator,
			)
		})

		entry.beaconStakingProvider = callOrFail(t, func() (common.Address, error) {
			return callers.randomBeacon.OperatorToStakingProvider(
				callers.callOpts,
				operator,
			)
		})

		if entry.tbtcStakingProvider != zeroAddress {
			entry.eligibleStake = callOrFail(t, func() (*big.Int, error) {
				return callers.walletRegistry.EligibleStake(
					callers.callOpts,
					entry.tbtcStakingProvider,
				)
			})
			entry.pendingDecrease = callOrFail(t, func() (*big.Int, error) {
				return callers.walletRegistry.PendingAuthorizationDecrease(
					callers.callOpts,
					entry.tbtcStakingProvider,
				)
			})
			entry.tbtcOwner = callers.rolesOwner(t, entry.tbtcStakingProvider)
		}

		if entry.beaconStakingProvider == entry.tbtcStakingProvider {
			entry.beaconOwner = entry.tbtcOwner
		} else {
			entry.beaconOwner = callers.rolesOwner(
				t,
				entry.beaconStakingProvider,
			)
		}

		facts = append(facts, entry)
	}

	return facts
}

// admissionSplit counts a population by which branch admits it.
type admissionSplit struct {
	beaconOnly int
	both       int
	tbtcOnly   int
	admitted   []common.Address
}

func splitPopulation(
	facts []admissionFacts,
	tbtcRecognized func(admissionFacts) bool,
) admissionSplit {
	split := admissionSplit{admitted: make([]common.Address, 0)}

	for _, entry := range facts {
		beacon := entry.beaconRecognized()
		tbtc := tbtcRecognized(entry)

		switch {
		case beacon && tbtc:
			split.both++
		case beacon:
			split.beaconOnly++
		case tbtc:
			split.tbtcOnly++
		default:
			continue
		}

		split.admitted = append(split.admitted, entry.operator)
	}

	return split
}

func (s admissionSplit) combined() int {
	return s.beaconOnly + s.both + s.tbtcOnly
}

func assertSplit(
	t *testing.T,
	policy string,
	expected admissionSplit,
	actual admissionSplit,
) {
	t.Helper()

	testutils.AssertIntsEqual(
		t,
		policy+" beacon-only",
		expected.beaconOnly,
		actual.beaconOnly,
	)
	testutils.AssertIntsEqual(t, policy+" both", expected.both, actual.both)
	testutils.AssertIntsEqual(
		t,
		policy+" tbtc-only",
		expected.tbtcOnly,
		actual.tbtcOnly,
	)
	testutils.AssertIntsEqual(
		t,
		policy+" combined",
		expected.combined(),
		actual.combined(),
	)
}

// difference returns the addresses in minuend that subtrahend does not hold,
// rendered lowercase and sorted so it can be compared literally.
func difference(minuend, subtrahend []common.Address) []string {
	held := make(map[common.Address]bool, len(subtrahend))
	for _, address := range subtrahend {
		held[address] = true
	}

	only := make([]string, 0)
	for _, address := range minuend {
		if !held[address] {
			only = append(only, strings.ToLower(address.Hex()))
		}
	}

	sort.Strings(only)

	return only
}

func assertAddressSet(
	t *testing.T,
	description string,
	expected []string,
	actual []string,
) {
	t.Helper()

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf(
			"unexpected %s\nexpected: %v\nactual:   %v",
			description,
			expected,
			actual,
		)
	}
}

// TestMainnetChainState_AdmissionCensus evaluates the three admission policies
// this change moves between - the merge base, the originally proposed head and
// the policy settled on - over every operator address either registry has ever
// recorded a registration for, using the chain state at the anchor.
//
// These are predicate results over registered addresses, not counts of peers
// that were connected at the anchor. An address that registered once and has
// been offline since still appears here, and a peer that never registered
// never does.
func TestMainnetChainState_AdmissionCensus(t *testing.T) {
	callers := newAdmissionCallers(t)

	walletRegistryHistory := walletRegistryRegistrations(t, callers)
	randomBeaconHistory := randomBeaconRegistrations(t, callers)

	testutils.AssertIntsEqual(
		t,
		"wallet registry registration events",
		288,
		len(walletRegistryHistory),
	)
	testutils.AssertIntsEqual(
		t,
		"random beacon registration events",
		289,
		len(randomBeaconHistory),
	)

	population := operatorUnion(walletRegistryHistory, randomBeaconHistory)
	testutils.AssertIntsEqual(
		t,
		"unique registered operator addresses",
		291,
		len(population),
	)

	facts := readAdmissionFacts(t, callers, population)

	baseline := splitPopulation(facts, admissionFacts.baselineTbtcRecognized)
	originalHead := splitPopulation(
		facts,
		admissionFacts.originalHeadTbtcRecognized,
	)
	proposed := splitPopulation(facts, admissionFacts.proposedTbtcRecognized)

	assertSplit(
		t,
		"original head",
		admissionSplit{beaconOnly: 243, both: 38, tbtcOnly: 1},
		originalHead,
	)
	assertSplit(
		t,
		"proposed",
		admissionSplit{beaconOnly: 262, both: 19, tbtcOnly: 1},
		proposed,
	)

	// Dropping the pending-decrease credential moves nineteen providers from
	// being recognized by both branches to being carried by the beacon alone,
	// which leaves the two policies admitting the same addresses as each
	// other. Neither admits the same set as the merge base: both admit one
	// address it does not, and both stop admitting one address it does. The
	// combined totals match only because those two happen to cancel out.
	gained := []string{"0xc1e20a88c2130472b25b3c382773ba85944230d2"}
	lost := []string{"0xc19f2434236254fcbd2d329bbe048184bba21975"}

	assertAddressSet(
		t,
		"addresses the original head admits over the merge base",
		gained,
		difference(originalHead.admitted, baseline.admitted),
	)
	assertAddressSet(
		t,
		"addresses the original head stops admitting",
		lost,
		difference(baseline.admitted, originalHead.admitted),
	)
	assertAddressSet(
		t,
		"addresses the proposed policy admits over the merge base",
		gained,
		difference(proposed.admitted, baseline.admitted),
	)
	assertAddressSet(
		t,
		"addresses the proposed policy stops admitting",
		lost,
		difference(baseline.admitted, proposed.admitted),
	)
}

// TestMainnetChainState_DeprecatedOperatorsKeepBeaconAdmission reads the
// operators the ECDSA allowlist deliberately left out. The tBTC predicate
// rejects every one of them, and every one of them stays admitted through the
// beacon branch. That is why the beacon must keep its own predicate; who the
// change admits and stops admitting overall is settled by the census above.
func TestMainnetChainState_DeprecatedOperatorsKeepBeaconAdmission(t *testing.T) {
	callers := newAdmissionCallers(t)
	weights := readAllowlistWeights(t)

	if len(weights.DeprecatedOperatorsNotAdded) == 0 {
		t.Fatal("no deprecated operators found in the allowlist deploy data")
	}

	for _, deprecated := range weights.DeprecatedOperatorsNotAdded {
		t.Run(deprecated.StakingProvider, func(t *testing.T) {
			stakingProvider := common.HexToAddress(deprecated.StakingProvider)
			operatorAddress := common.HexToAddress(deprecated.Operator)

			eligibleStake := callOrFail(t, func() (*big.Int, error) {
				return callers.walletRegistry.EligibleStake(
					callers.callOpts,
					stakingProvider,
				)
			})

			if eligibleStake.Sign() != 0 {
				t.Errorf(
					"expected the tbtc branch to reject, but eligible stake "+
						"is [%v]",
					eligibleStake,
				)
			}

			beaconStakingProvider := callOrFail(t, func() (common.Address, error) {
				return callers.randomBeacon.OperatorToStakingProvider(
					callers.callOpts,
					operatorAddress,
				)
			})

			if beaconStakingProvider == zeroAddress {
				t.Error("expected the operator to be known to the beacon")
			}

			if callers.rolesOwner(t, beaconStakingProvider) == zeroAddress {
				t.Error("expected the beacon branch to keep admitting")
			}
		})
	}
}

// TestBeaconChain_EligibleStakeIsZeroForEveryRegisteredProvider is the reason
// the beacon keeps the legacy delegation predicate. Token staking authorizes
// no stake for the beacon, so beacon eligible stake is zero for every staking
// provider that ever registered a beacon operator - the whole registered
// population at the anchor, not a subset of it. Were the beacon given the
// tBTC predicate, this branch would recognize nobody.
func TestBeaconChain_EligibleStakeIsZeroForEveryRegisteredProvider(t *testing.T) {
	callers := newAdmissionCallers(t)

	stakingProviders := stakingProviderSet(randomBeaconRegistrations(t, callers))
	if len(stakingProviders) == 0 {
		t.Fatal("the beacon has no registered staking providers at the anchor")
	}

	for _, stakingProvider := range stakingProviders {
		t.Run(stakingProvider.Hex(), func(t *testing.T) {
			eligibleStake := callOrFail(t, func() (*big.Int, error) {
				return callers.randomBeacon.EligibleStake(
					callers.callOpts,
					stakingProvider,
				)
			})

			testutils.AssertBigIntsEqual(
				t,
				"beacon eligible stake",
				big.NewInt(0),
				eligibleStake,
			)
		})
	}
}

// TestMainnetChainState_EligibleStakeAtMinimumAuthorization pins the floor the
// tBTC predicate decides against. An authorization cannot be lowered past the
// minimum without going to zero outright, so the smallest positive eligible
// stake a provider can hold is exactly that minimum. The test confirms the
// minimum the wallet registry enforces at the anchor is the 40,000 T the
// predicate was reasoned about, and that the registered population actually
// contains providers sitting on it - a floor nobody occupies would make the
// case moot.
//
// Whether the predicate recognizes such a provider is covered by the
// deterministic tests over the production read path; the lifecycle a decrease
// request goes through belongs to the maintainer-owned fork test.
func TestMainnetChainState_EligibleStakeAtMinimumAuthorization(t *testing.T) {
	callers := newAdmissionCallers(t)

	minimumAuthorization := callOrFail(t, func() (*big.Int, error) {
		return callers.walletRegistry.MinimumAuthorization(callers.callOpts)
	})

	testutils.AssertBigIntsEqual(
		t,
		"minimum authorization",
		tTokens(40_000),
		minimumAuthorization,
	)

	stakingProviders := stakingProviderSet(
		walletRegistryRegistrations(t, callers),
	)
	if len(stakingProviders) == 0 {
		t.Fatal("the wallet registry has no registered providers at the anchor")
	}

	providersAtTheFloor := 0
	for _, stakingProvider := range stakingProviders {
		eligibleStake := callOrFail(t, func() (*big.Int, error) {
			return callers.walletRegistry.EligibleStake(
				callers.callOpts,
				stakingProvider,
			)
		})

		if eligibleStake.Cmp(minimumAuthorization) == 0 {
			providersAtTheFloor++
		}
	}

	if providersAtTheFloor == 0 {
		t.Errorf(
			"no registered provider sits on the minimum authorization at "+
				"block %d; the case this guards no longer exists at the anchor",
			admissionPinnedBlock,
		)
	}
}

// TestMainnetChainState_ProviderAuthorizedAfterLegacyStakingFroze is the
// case the change exists for. Legacy token staking can no longer record a
// delegation for anyone, so a provider authorized after it froze reads a zero
// owner forever while holding full eligible stake: the delegation predicate
// rejects it permanently and the eligible stake predicate admits it. It has no
// beacon backstop either, which is the cost the change carries.
func TestMainnetChainState_ProviderAuthorizedAfterLegacyStakingFroze(t *testing.T) {
	callers := newAdmissionCallers(t)

	stakingProvider := common.HexToAddress(
		"0x3d921565ec837c6bfaf441579e07571646e0048a",
	)

	eligibleStake := callOrFail(t, func() (*big.Int, error) {
		return callers.walletRegistry.EligibleStake(
			callers.callOpts,
			stakingProvider,
		)
	})

	if eligibleStake.Sign() <= 0 {
		t.Fatalf(
			"expected the provider to hold eligible stake, got [%v]",
			eligibleStake,
		)
	}

	if owner := callers.rolesOwner(t, stakingProvider); owner != zeroAddress {
		t.Errorf("expected no legacy delegation, got owner [%v]", owner)
	}

	// Before using the hardcoded operator address for the beacon-side check,
	// prove it actually belongs to the staking provider under test on the
	// tBTC side: the wallet registry and the beacon are queried with two
	// independently hardcoded addresses, and nothing else ties them together.
	operatorAddress := common.HexToAddress(
		"0xc1E20a88C2130472B25B3C382773bA85944230d2",
	)

	walletRegistryStakingProvider := callOrFail(t, func() (common.Address, error) {
		return callers.walletRegistry.OperatorToStakingProvider(
			callers.callOpts,
			operatorAddress,
		)
	})
	if walletRegistryStakingProvider != stakingProvider {
		t.Fatalf(
			"operator does not map to the staking provider under test: "+
				"got [%v], want [%v]",
			walletRegistryStakingProvider,
			stakingProvider,
		)
	}

	beaconStakingProvider := callOrFail(t, func() (common.Address, error) {
		return callers.randomBeacon.OperatorToStakingProvider(
			callers.callOpts,
			operatorAddress,
		)
	})

	if beaconStakingProvider == zeroAddress {
		t.Fatal("expected the operator to be known to the beacon")
	}

	if callers.rolesOwner(t, beaconStakingProvider) != zeroAddress {
		t.Error("expected the provider to have no beacon backstop")
	}
}
