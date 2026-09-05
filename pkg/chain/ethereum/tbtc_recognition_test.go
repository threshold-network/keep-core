package ethereum

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/operator"
)

// mockAdmissionReader stands in for the wallet registry reads the tBTC
// admission predicate performs.
type mockAdmissionReader struct {
	stakingProviders map[common.Address]common.Address
	eligibleStakes   map[common.Address]*big.Int

	stakingProviderErr error
	eligibleStakeErr   error

	stakingProviderCalls int
	eligibleStakeCalls   int
}

func (mar *mockAdmissionReader) OperatorToStakingProvider(
	operator common.Address,
) (common.Address, error) {
	mar.stakingProviderCalls++

	if mar.stakingProviderErr != nil {
		return common.Address{}, mar.stakingProviderErr
	}

	return mar.stakingProviders[operator], nil
}

func (mar *mockAdmissionReader) EligibleStake(
	stakingProvider common.Address,
) (*big.Int, error) {
	mar.eligibleStakeCalls++

	if mar.eligibleStakeErr != nil {
		return nil, mar.eligibleStakeErr
	}

	return mar.eligibleStakes[stakingProvider], nil
}

// legacyDelegationApplication mirrors the predicate BeaconChain.IsRecognized
// implements: recognition follows a legacy token staking delegation, which the
// tBTC branch no longer consults.
type legacyDelegationApplication struct {
	delegated map[string]bool
}

func (lda *legacyDelegationApplication) IsRecognized(
	operatorPublicKey *operator.PublicKey,
) (bool, error) {
	return lda.delegated[operatorPublicKey.String()], nil
}

// tTokens returns the given whole number of T in the 18-decimal base unit
// authorized weights are expressed in.
func tTokens(amount int64) *big.Int {
	return new(big.Int).Mul(
		big.NewInt(amount),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
}

// newTestOperator returns an operator public key together with the chain
// address the admission predicate derives from it.
func newTestOperator(t *testing.T) (*operator.PublicKey, common.Address) {
	t.Helper()

	chainPrivateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	_, operatorPublicKey, err := ChainPrivateKeyToOperatorKeyPair(chainPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	return operatorPublicKey, crypto.PubkeyToAddress(chainPrivateKey.PublicKey)
}

func TestTbtcChain_IsRecognized(t *testing.T) {
	stakingProvider := common.HexToAddress("0x1")

	// The floor an authorization decrease cannot be lowered past without going
	// to zero outright.
	minimumAuthorization := tTokens(40_000)

	var tests = map[string]struct {
		eligibleStake      *big.Int
		expectedRecognized bool
	}{
		"authorized staking provider that never held a legacy delegation": {
			eligibleStake:      tTokens(40_000_000),
			expectedRecognized: true,
		},
		"pending decrease sitting exactly on the minimum authorization": {
			eligibleStake:      minimumAuthorization,
			expectedRecognized: true,
		},
		"eligible stake of a single base unit": {
			eligibleStake:      big.NewInt(1),
			expectedRecognized: true,
		},
		"authorization decrease approved down to zero": {
			eligibleStake:      big.NewInt(0),
			expectedRecognized: false,
		},
		"operator registered by an address holding no authorization": {
			eligibleStake:      big.NewInt(0),
			expectedRecognized: false,
		},
		"deprecated operator deliberately left unauthorized": {
			eligibleStake:      big.NewInt(0),
			expectedRecognized: false,
		},
		"eligible stake the registry reported as no value at all": {
			eligibleStake:      nil,
			expectedRecognized: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			operatorPublicKey, operatorAddress := newTestOperator(t)

			chain := &TbtcChain{
				admission: &mockAdmissionReader{
					stakingProviders: map[common.Address]common.Address{
						operatorAddress: stakingProvider,
					},
					eligibleStakes: map[common.Address]*big.Int{
						stakingProvider: test.eligibleStake,
					},
				},
			}

			isRecognized, err := chain.IsRecognized(operatorPublicKey)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBoolsEqual(
				t,
				"recognition",
				test.expectedRecognized,
				isRecognized,
			)
		})
	}
}

// TestTbtcChain_IsRecognized_UnregisteredOperator covers an operator that has
// never been mapped to a staking provider. The predicate must short-circuit
// before reading eligible stake, so that recognition costs exactly one chain
// call for an unknown peer.
func TestTbtcChain_IsRecognized_UnregisteredOperator(t *testing.T) {
	operatorPublicKey, _ := newTestOperator(t)

	admission := &mockAdmissionReader{
		stakingProviders: map[common.Address]common.Address{},
		eligibleStakes:   map[common.Address]*big.Int{},
	}

	chain := &TbtcChain{admission: admission}

	isRecognized, err := chain.IsRecognized(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBoolsEqual(t, "recognition", false, isRecognized)
	testutils.AssertIntsEqual(
		t,
		"eligible stake reads",
		0,
		admission.eligibleStakeCalls,
	)
}

// TestTbtcChain_IsRecognized_StakingProviderLookupFails asserts the predicate
// fails closed: a chain error propagates instead of being reported as a
// negative recognition.
func TestTbtcChain_IsRecognized_StakingProviderLookupFails(t *testing.T) {
	operatorPublicKey, _ := newTestOperator(t)

	chain := &TbtcChain{
		admission: &mockAdmissionReader{
			stakingProviderErr: errors.New("connection refused"),
		},
	}

	isRecognized, err := chain.IsRecognized(operatorPublicKey)

	testutils.AssertBoolsEqual(t, "recognition", false, isRecognized)
	if err == nil {
		t.Fatal("expected the chain error to be returned to the caller")
	}
}

// TestTbtcChain_IsRecognized_EligibleStakeLookupFails asserts the same
// fail-closed behaviour for the read the predicate actually decides on.
func TestTbtcChain_IsRecognized_EligibleStakeLookupFails(t *testing.T) {
	operatorPublicKey, operatorAddress := newTestOperator(t)

	stakingProvider := common.HexToAddress("0x1")
	lookupErr := errors.New("connection refused")

	chain := &TbtcChain{
		admission: &mockAdmissionReader{
			stakingProviders: map[common.Address]common.Address{
				operatorAddress: stakingProvider,
			},
			eligibleStakeErr: lookupErr,
		},
	}

	isRecognized, err := chain.IsRecognized(operatorPublicKey)

	testutils.AssertBoolsEqual(t, "recognition", false, isRecognized)
	testutils.AssertAnyErrorInChainMatchesTarget(t, lookupErr, err)
}

// TestTbtcChain_IsRecognized_ChainErrorIsNotCached pins the reason the
// predicate must not fold an error into a negative result. The firewall caches
// negative recognition for an hour; were an error to arrive as "not
// recognized", a peer would stay locked out for that hour after a momentary
// chain fault. The error must instead surface to the caller, leaving the cache
// untouched, so that the very next attempt re-reads the chain.
func TestTbtcChain_IsRecognized_ChainErrorIsNotCached(t *testing.T) {
	operatorPublicKey, operatorAddress := newTestOperator(t)

	stakingProvider := common.HexToAddress("0x1")

	admission := &mockAdmissionReader{
		stakingProviders: map[common.Address]common.Address{
			operatorAddress: stakingProvider,
		},
		eligibleStakes: map[common.Address]*big.Int{
			stakingProvider: tTokens(40_000),
		},
		eligibleStakeErr: errors.New("connection refused"),
	}

	policy := firewall.AnyApplicationPolicy(
		[]firewall.Application{&TbtcChain{admission: admission}},
		firewall.EmptyAllowList(),
	)

	err := policy.Validate(operatorPublicKey)
	if err == nil {
		t.Fatal("expected the chain error to surface from validation")
	}
	if errors.Is(err, firewall.ErrNotRecognized) {
		t.Fatal("chain error was reported as a non-recognition")
	}

	// The fault clears. Nothing was cached, so the peer is admitted straight
	// away rather than after the caching period.
	admission.eligibleStakeErr = nil

	if err := policy.Validate(operatorPublicKey); err != nil {
		t.Fatalf("peer was not admitted once the chain recovered: [%v]", err)
	}
}

// TestTbtcChain_IsRecognized_LegacyDelegationAdmittedByBeaconBranch documents a
// gap this predicate does not close, and asserts the behaviour as it stands
// rather than as it should be. Admission is a disjunction over applications, so
// an identity holding nothing but a legacy token staking delegation - including
// one whose authorization has since been revoked - keeps its admission through
// the beacon branch even though the tBTC branch now rejects it. The beacon
// branch is represented here by a stand-in mirroring the predicate beacon.go
// implements, because the property under test belongs to the composition.
func TestTbtcChain_IsRecognized_LegacyDelegationAdmittedByBeaconBranch(t *testing.T) {
	operatorPublicKey, operatorAddress := newTestOperator(t)

	stakingProvider := common.HexToAddress("0x1")

	tbtcChain := &TbtcChain{
		admission: &mockAdmissionReader{
			stakingProviders: map[common.Address]common.Address{
				operatorAddress: stakingProvider,
			},
			eligibleStakes: map[common.Address]*big.Int{
				stakingProvider: big.NewInt(0),
			},
		},
	}

	beaconChain := &legacyDelegationApplication{
		delegated: map[string]bool{operatorPublicKey.String(): true},
	}

	isRecognized, err := tbtcChain.IsRecognized(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBoolsEqual(t, "tbtc branch recognition", false, isRecognized)

	// Applications are supplied beacon first, matching the production wiring.
	policy := firewall.AnyApplicationPolicy(
		[]firewall.Application{beaconChain, tbtcChain},
		firewall.EmptyAllowList(),
	)

	if err := policy.Validate(operatorPublicKey); err != nil {
		t.Fatalf(
			"a legacy delegation is expected to remain admitted: [%v]",
			err,
		)
	}
}

// TestTbtcChain_IsRecognized_UnauthorizedOperatorRejectedByBothBranches is the
// counterpart to the case above: registering an operator is permissionless on
// both registries, so an address that only registered itself must be rejected
// whichever branch evaluates it.
func TestTbtcChain_IsRecognized_UnauthorizedOperatorRejectedByBothBranches(t *testing.T) {
	operatorPublicKey, operatorAddress := newTestOperator(t)

	stakingProvider := common.HexToAddress("0x1")

	tbtcChain := &TbtcChain{
		admission: &mockAdmissionReader{
			stakingProviders: map[common.Address]common.Address{
				operatorAddress: stakingProvider,
			},
			eligibleStakes: map[common.Address]*big.Int{
				stakingProvider: big.NewInt(0),
			},
		},
	}

	policy := firewall.AnyApplicationPolicy(
		[]firewall.Application{
			&legacyDelegationApplication{delegated: map[string]bool{}},
			tbtcChain,
		},
		firewall.EmptyAllowList(),
	)

	testutils.AssertErrorsSame(
		t,
		firewall.ErrNotRecognized,
		policy.Validate(operatorPublicKey),
	)
}
