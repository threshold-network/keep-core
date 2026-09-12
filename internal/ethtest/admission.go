package ethtest

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// AdmissionCase is one synthetic operator identity together with the results
// the admission predicate and policy are pinned to produce for it. The result
// fields are stated here rather than computed from the state below, so a test
// asserting them cannot derive its expectation from the behaviour under test.
type AdmissionCase struct {
	Name string

	// OperatorKey and StakingProviderKey index the synthetic keys the
	// identity is built from.
	OperatorKey        int64
	StakingProviderKey int64

	// The chain state the identity stands for.
	RegisteredWithBeacon bool
	RegisteredWithTbtc   bool
	HasLegacyDelegation  bool
	EligibleStake        *big.Int
	PendingDecrease      *big.Int

	// The pinned outcome.
	TbtcRecognizes bool
	Admitted       bool
}

// StakingProvider is the address both registries map the identity's operator
// to, whether or not the identity is registered with them.
func (c AdmissionCase) StakingProvider(t *testing.T) common.Address {
	t.Helper()

	return Address(t, c.StakingProviderKey)
}

// Operator is the identity's operator address.
func (c AdmissionCase) Operator(t *testing.T) common.Address {
	t.Helper()

	return Address(t, c.OperatorKey)
}

// TbtcReads is the trace for the tBTC predicate: the registry mapping, and
// eligible stake only when the registry holds a mapping. A pending authorization
// decrease is seeded for some identities and is not an admission credential, so
// it never appears in a trace.
func (c AdmissionCase) TbtcReads(t *testing.T) []ExpectedCall {
	t.Helper()

	reads := []ExpectedCall{
		Read(
			WalletRegistryContract,
			"operatorToStakingProvider",
			c.Operator(t),
		),
	}

	if c.RegisteredWithTbtc {
		reads = append(
			reads,
			Read(WalletRegistryContract, "eligibleStake", c.StakingProvider(t)),
		)
	}

	return reads
}

// AdmissionReads is what validating this identity through the production
// policy must cost.
func (c AdmissionCase) AdmissionReads(t *testing.T) []ExpectedCall {
	t.Helper()

	return c.TbtcReads(t)
}

// AdmissionCases returns the fixed admission table. Admission follows tBTC
// registration and positive eligible stake; beacon registration and legacy
// token staking roles are present only to catch accidental fallback to them.
func AdmissionCases() []AdmissionCase {
	return []AdmissionCase{
		{
			// Authorized for the wallet registry after legacy token staking
			// stopped recording delegations.
			Name:                 "post_legacy_authorized",
			OperatorKey:          11,
			StakingProviderKey:   21,
			RegisteredWithBeacon: true,
			RegisteredWithTbtc:   true,
			EligibleStake:        TTokens(40_000_000),
			TbtcRecognizes:       true,
			Admitted:             true,
		},
		{
			// Holds a pending decrease with no authorization left behind it.
			// The decrease alone is not an admission credential.
			Name:                 "orphan_pending_only",
			OperatorKey:          12,
			StakingProviderKey:   22,
			RegisteredWithBeacon: true,
			RegisteredWithTbtc:   true,
			HasLegacyDelegation:  true,
			EligibleStake:        big.NewInt(0),
			PendingDecrease:      TTokens(30_000),
			TbtcRecognizes:       false,
			Admitted:             false,
		},
		{
			// Authorization is gone and nothing is pending, but a legacy
			// delegation remains recorded.
			Name:                 "legacy_revoked",
			OperatorKey:          13,
			StakingProviderKey:   23,
			RegisteredWithBeacon: true,
			RegisteredWithTbtc:   true,
			HasLegacyDelegation:  true,
			EligibleStake:        big.NewInt(0),
			TbtcRecognizes:       false,
			Admitted:             false,
		},
		{
			// Known to the wallet registry alone, by a provider that holds a
			// legacy delegation and no authorization. A roles-based fallback
			// would admit it, while eligible stake does not.
			Name:                "registry_only_legacy",
			OperatorKey:         14,
			StakingProviderKey:  24,
			RegisteredWithTbtc:  true,
			HasLegacyDelegation: true,
			EligibleStake:       big.NewInt(0),
			TbtcRecognizes:      false,
			Admitted:            false,
		},
		{
			// Positive wallet-registry eligibility is sufficient even when the
			// operator was never registered with the beacon.
			Name:               "tbtc_only_authorized",
			OperatorKey:        18,
			StakingProviderKey: 28,
			RegisteredWithTbtc: true,
			EligibleStake:      TTokens(40_000),
			TbtcRecognizes:     true,
			Admitted:           true,
		},
		{
			// Registered only with the beacon and backed by a legacy
			// delegation.
			Name:                 "beacon_only",
			OperatorKey:          15,
			StakingProviderKey:   25,
			RegisteredWithBeacon: true,
			HasLegacyDelegation:  true,
			TbtcRecognizes:       false,
			Admitted:             false,
		},
		{
			// Registered with both registries and authorized by neither.
			Name:                 "registered_unauthorized",
			OperatorKey:          16,
			StakingProviderKey:   26,
			RegisteredWithBeacon: true,
			RegisteredWithTbtc:   true,
			EligibleStake:        big.NewInt(0),
			TbtcRecognizes:       false,
			Admitted:             false,
		},
		{
			// Known to neither registry.
			Name:               "unregistered",
			OperatorKey:        17,
			StakingProviderKey: 27,
			TbtcRecognizes:     false,
			Admitted:           false,
		},
	}
}

// AdmissionCase returns the table entry with the given name.
func AdmissionCaseNamed(t *testing.T, name string) AdmissionCase {
	t.Helper()

	for _, admissionCase := range AdmissionCases() {
		if admissionCase.Name == name {
			return admissionCase
		}
	}

	t.Fatalf("the admission table holds no case named [%s]", name)

	return AdmissionCase{}
}

// AdmissionState builds the chain state the admission table describes.
func AdmissionState(t *testing.T) State {
	t.Helper()

	state := State{
		TbtcOperators:    map[common.Address]common.Address{},
		EligibleStakes:   map[common.Address]*big.Int{},
		PendingDecreases: map[common.Address]*big.Int{},
		BeaconOperators:  map[common.Address]common.Address{},
		Roles:            map[common.Address]Roles{},
	}

	for _, admissionCase := range AdmissionCases() {
		operator := admissionCase.Operator(t)
		stakingProvider := admissionCase.StakingProvider(t)

		if admissionCase.RegisteredWithBeacon {
			state.BeaconOperators[operator] = stakingProvider
		}
		if admissionCase.RegisteredWithTbtc {
			state.TbtcOperators[operator] = stakingProvider
		}
		if admissionCase.HasLegacyDelegation {
			state.Roles[stakingProvider] = Roles{
				Owner:       Address(t, admissionCase.StakingProviderKey+100),
				Beneficiary: Address(t, admissionCase.StakingProviderKey+200),
				Authorizer:  Address(t, admissionCase.StakingProviderKey+300),
			}
		}
		if admissionCase.EligibleStake != nil {
			state.EligibleStakes[stakingProvider] = admissionCase.EligibleStake
		}
		if admissionCase.PendingDecrease != nil {
			state.PendingDecreases[stakingProvider] =
				admissionCase.PendingDecrease
		}
	}

	return state
}
