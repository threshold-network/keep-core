package ethereum

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/keep-network/keep-core/internal/ethtest"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/operator"
)

// connectAdmissionFixture builds both production chain handles through the
// public Connect path against a deterministic endpoint serving the fixed
// admission table. Nothing between the JSON-RPC boundary and the predicates is
// substituted: the generated bindings, the chain handle constructors and the
// production admission adapters are the code under test.
func connectAdmissionFixture(t *testing.T) (
	*ethtest.Backend,
	*BeaconChain,
	*TbtcChain,
) {
	t.Helper()

	backend := ethtest.New(t, ethtest.AdmissionState(t))

	beaconChain, tbtcChain, _, _, _, err := Connect(
		context.Background(),
		backend.ChainConfig(t),
	)
	if err != nil {
		t.Fatalf("failed to connect to the fixture: %v", err)
	}

	// What connecting costs, before anything is forgotten. Construction
	// resolves the addresses it needs and nothing else; in particular it reads
	// no admission state, so no later assertion is looking at a read that was
	// already served.
	backend.AssertTrace(
		t,
		"construction reads",
		ethtest.Read(ethtest.RandomBeaconContract, "sortitionPool"),
		ethtest.Read(ethtest.BridgeContract, "contractReferences"),
		ethtest.Read(ethtest.WalletRegistryContract, "sortitionPool"),
		ethtest.Read(ethtest.BridgeContract, "getRedemptionWatchtower"),
		ethtest.Read(ethtest.RandomBeaconContract, "staking"),
		ethtest.Read(ethtest.WalletRegistryContract, "staking"),
	)
	backend.AssertNoUnexpectedCalls(t)
	backend.ResetCalls()

	return backend, beaconChain, tbtcChain
}

// caseOperatorKey returns the operator public key of one admission table
// identity, derived with the same conversion the client applies to its own
// key. See ethtest.Key for what those keys are and why deriving identities
// from them costs nothing here.
func caseOperatorKey(
	t *testing.T,
	admissionCase ethtest.AdmissionCase,
) *operator.PublicKey {
	t.Helper()

	_, operatorPublicKey, err := ChainPrivateKeyToOperatorKeyPair(
		ethtest.Key(t, admissionCase.OperatorKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	return operatorPublicKey
}

// TestBeaconChain_IsRecognized_ProductionAdapter drives the beacon branch of
// the fixed admission table through beaconAdmissionReaderChain, the adapter
// newBeaconChain installs, and through the token staking binding newBaseChain
// configures. Every read is answered by the deterministic endpoint, so the
// result is produced by the production code path rather than by a stand-in
// predicate.
func TestBeaconChain_IsRecognized_ProductionAdapter(t *testing.T) {
	backend, beaconChain, _ := connectAdmissionFixture(t)

	adapter, ok := beaconChain.admission.(*beaconAdmissionReaderChain)
	if !ok {
		t.Fatalf(
			"beacon admission is served by [%T], not the production adapter",
			beaconChain.admission,
		)
	}
	if adapter.randomBeacon != beaconChain.randomBeacon {
		t.Error("the beacon adapter reads a different RandomBeacon binding")
	}
	if adapter.baseChain != beaconChain.baseChain {
		t.Error("the beacon adapter reads a different base chain")
	}

	for _, admissionCase := range ethtest.AdmissionCases() {
		t.Run(admissionCase.Name, func(t *testing.T) {
			backend.ResetCalls()

			isRecognized, err := beaconChain.IsRecognized(
				caseOperatorKey(t, admissionCase),
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBoolsEqual(
				t,
				"beacon recognition",
				admissionCase.BeaconRecognizes,
				isRecognized,
			)

			// An operator the registry does not know costs exactly one read:
			// the predicate must not go on to ask token staking about the
			// zero address.
			backend.AssertTrace(
				t,
				"beacon recognition reads",
				admissionCase.BeaconReads(t)...,
			)
			backend.AssertNoUnexpectedCalls(t)
		})
	}
}

// TestTbtcChain_IsRecognized_ProductionAdapter drives the tBTC branch of the
// same table through the WalletRegistry binding newTbtcChain installs as the
// admission reader.
func TestTbtcChain_IsRecognized_ProductionAdapter(t *testing.T) {
	backend, _, tbtcChain := connectAdmissionFixture(t)

	walletRegistry, ok := tbtcChain.admission.(*ecdsacontract.WalletRegistry)
	if !ok {
		t.Fatalf(
			"tBTC admission is served by [%T], not the wallet registry",
			tbtcChain.admission,
		)
	}
	if walletRegistry != tbtcChain.walletRegistry {
		t.Error("tBTC admission reads a different WalletRegistry binding")
	}

	for _, admissionCase := range ethtest.AdmissionCases() {
		t.Run(admissionCase.Name, func(t *testing.T) {
			backend.ResetCalls()

			isRecognized, err := tbtcChain.IsRecognized(
				caseOperatorKey(t, admissionCase),
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBoolsEqual(
				t,
				"tbtc recognition",
				admissionCase.TbtcRecognizes,
				isRecognized,
			)

			backend.AssertTrace(
				t,
				"tbtc recognition reads",
				admissionCase.TbtcReads(t)...,
			)
			backend.AssertNoUnexpectedCalls(t)
		})
	}
}

// TestTbtcChain_IsRecognized_AtMinimumAuthorization is the floor case, driven
// through the production read path. An authorization cannot be lowered past
// the registry's minimum without going to zero outright, so the smallest
// positive eligible stake a provider can hold is exactly that minimum, and a
// provider sitting on it has to stay recognized.
//
// The floor is read from the registry rather than written into the test, and
// the state it is seeded into is read back through the same binding the
// predicate decides on, so neither the input nor the boundary is a copy.
func TestTbtcChain_IsRecognized_AtMinimumAuthorization(t *testing.T) {
	backend, _, tbtcChain := connectAdmissionFixture(t)

	minimumAuthorization, err := tbtcChain.walletRegistry.MinimumAuthorization()
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBigIntNonZero(
		t,
		"minimum authorization",
		minimumAuthorization,
	)
	backend.AssertTrace(
		t,
		"minimum authorization read",
		ethtest.Read(ethtest.WalletRegistryContract, "minimumAuthorization"),
	)

	// An identity the table leaves unauthorized, brought up to the floor and
	// no further.
	atTheFloor := ethtest.AdmissionCaseNamed(t, "registered_unauthorized")
	stakingProvider := atTheFloor.StakingProvider(t)
	backend.SetEligibleStake(stakingProvider, minimumAuthorization)

	eligibleStake, err := tbtcChain.EligibleStake(
		chainAddress(stakingProvider),
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBigIntsEqual(
		t,
		"eligible stake",
		minimumAuthorization,
		eligibleStake,
	)

	backend.ResetCalls()

	isRecognized, err := tbtcChain.IsRecognized(caseOperatorKey(t, atTheFloor))
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBoolsEqual(
		t,
		"recognition at the minimum authorization",
		true,
		isRecognized,
	)

	backend.AssertTrace(
		t,
		"recognition reads at the floor",
		atTheFloor.TbtcReads(t)...,
	)
	backend.AssertNoUnexpectedCalls(t)
}

// TestAdmission_ProductionConstruction asserts what the public construction
// path assembles and how the assembled handles behave together. Recognition is
// evaluated beacon first over an empty static allow list, which is the
// composition the client starts with.
func TestAdmission_ProductionConstruction(t *testing.T) {
	backend, beaconChain, tbtcChain := connectAdmissionFixture(t)

	if beaconChain.baseChain != tbtcChain.baseChain {
		t.Error("the two chain handles were given different base chains")
	}
	if _, ok := beaconChain.admission.(*beaconAdmissionReaderChain); !ok {
		t.Errorf(
			"beacon admission is served by [%T], not the production adapter",
			beaconChain.admission,
		)
	}
	if tbtcChain.admission != tbtcAdmissionReader(tbtcChain.walletRegistry) {
		t.Error("tBTC admission is not the constructed WalletRegistry binding")
	}

	allowList := firewall.EmptyAllowList()
	policy := firewall.AnyApplicationPolicy(
		[]firewall.Application{beaconChain, tbtcChain},
		allowList,
	)

	for _, admissionCase := range ethtest.AdmissionCases() {
		t.Run(admissionCase.Name, func(t *testing.T) {
			operatorPublicKey := caseOperatorKey(t, admissionCase)

			// The static allow list production builds is empty, so it decides
			// nothing and every identity below is settled by a chain read.
			if allowList.Contains(operatorPublicKey) {
				t.Fatal("the production allow list is not empty")
			}

			backend.ResetCalls()

			err := policy.Validate(operatorPublicKey)

			if admissionCase.Admitted {
				if err != nil {
					t.Fatalf("expected the peer to be admitted: %v", err)
				}
			} else {
				testutils.AssertErrorsSame(t, firewall.ErrNotRecognized, err)
			}

			// Applications are evaluated beacon first: once the beacon
			// recognizes a peer, the tBTC branch is never consulted. The
			// beacon branch is always read, so nothing settled this identity
			// ahead of the chain.
			backend.AssertTrace(
				t,
				"admission reads",
				admissionCase.AdmissionReads(t)...,
			)
			backend.AssertNoUnexpectedCalls(t)
		})
	}
}

// TestBaseChain_RolesOf_ProductionBinding pins the contract the beacon branch
// reads stake delegations from. The allowlist is a separate deployment holding
// its own role mapping, and reading roles from it instead of from token
// staking would silently answer a different question.
func TestBaseChain_RolesOf_ProductionBinding(t *testing.T) {
	backend, beaconChain, _ := connectAdmissionFixture(t)

	delegated := ethtest.AdmissionCaseNamed(t, "beacon_only")
	stakingProvider := delegated.StakingProvider(t)

	backend.ResetCalls()

	owner, beneficiary, authorizer, hasStake, err := beaconChain.RolesOf(
		chainAddress(stakingProvider),
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBoolsEqual(t, "stake delegation", true, hasStake)
	testutils.AssertStringsEqual(
		t,
		"owner",
		ethtest.Address(t, delegated.StakingProviderKey+100).Hex(),
		owner.String(),
	)
	testutils.AssertStringsEqual(
		t,
		"beneficiary",
		ethtest.Address(t, delegated.StakingProviderKey+200).Hex(),
		beneficiary.String(),
	)
	testutils.AssertStringsEqual(
		t,
		"authorizer",
		ethtest.Address(t, delegated.StakingProviderKey+300).Hex(),
		authorizer.String(),
	)

	backend.AssertTrace(
		t,
		"roles lookup reads",
		ethtest.Read(ethtest.TokenStakingContract, "rolesOf", stakingProvider),
	)
	backend.AssertNoUnexpectedCalls(t)

	// The address discipline above is only evidence if the fixture would
	// actually notice a read landing on the allowlist. Point a base chain at
	// the allowlist and confirm it does.
	misroutedConfig := backend.ChainConfig(t)
	misroutedConfig.SetContractAddress(
		TokenStakingContractName,
		ethtest.AllowlistAddress.Hex(),
	)

	client, err := ethclient.Dial(backend.URL())
	if err != nil {
		t.Fatal(err)
	}

	misrouted, err := newBaseChain(context.Background(), misroutedConfig, client)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, _, _, err := misrouted.RolesOf(
		chainAddress(stakingProvider),
	); err == nil {
		t.Fatal("a rolesOf read sent to the allowlist was answered")
	}

	misrouteReported := false
	for _, unexpected := range backend.UnexpectedCalls() {
		if strings.Contains(unexpected, ethtest.AllowlistAddress.Hex()) {
			misrouteReported = true
		}
	}
	if !misrouteReported {
		t.Error("the fixture did not report the misrouted rolesOf read")
	}
}

// chainAddress renders a chain address the way the roles lookup expects it.
func chainAddress(address common.Address) chain.Address {
	return chain.Address(address.Hex())
}
