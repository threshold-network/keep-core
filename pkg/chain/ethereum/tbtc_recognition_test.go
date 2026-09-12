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
	stakingProviders              map[common.Address]common.Address
	eligibleStakes                map[common.Address]*big.Int
	pendingAuthorizationDecreases map[common.Address]*big.Int

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

// PendingAuthorizationDecrease answers the pending decreases seeded into the
// reader. A pending decrease is not an admission credential, so the predicate
// under test has no way to reach this read: tbtcAdmissionReader does not
// declare it.
func (mar *mockAdmissionReader) PendingAuthorizationDecrease(
	stakingProvider common.Address,
) (*big.Int, error) {
	return mar.pendingAuthorizationDecreases[stakingProvider], nil
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

	// The floor an authorization cannot be lowered past without going to zero
	// outright.
	minimumAuthorization := tTokens(40_000)

	var tests = map[string]struct {
		eligibleStake      *big.Int
		expectedRecognized bool
	}{
		"authorized staking provider that never held a legacy delegation": {
			eligibleStake:      tTokens(40_000_000),
			expectedRecognized: true,
		},
		"authorization sitting exactly on the minimum": {
			eligibleStake:      minimumAuthorization,
			expectedRecognized: true,
		},
		"authorization decreased to zero with nothing pending": {
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

// TestTbtcChain_IsRecognized_PendingOnlyRejected covers the two shapes a
// staking provider can carry a positive pending authorization decrease in
// while holding no eligible stake. A requested decrease is subtracted from
// eligible stake when it is requested rather than when it is approved, so both
// shapes are ordinary chain states rather than corner cases. Neither of them
// is an admission credential: the provider is not currently authorized for the
// wallet registry, and only the authorizer can change that.
func TestTbtcChain_IsRecognized_PendingOnlyRejected(t *testing.T) {
	stakingProvider := common.HexToAddress("0x1")

	var tests = map[string]struct {
		pendingAuthorizationDecrease *big.Int
	}{
		// The whole authorization was requested for decrease, taking eligible
		// stake to zero while the request waits.
		"full_decrease": {
			pendingAuthorizationDecrease: tTokens(40_000),
		},
		// A decrease record outliving the authorization it was requested
		// against, so nothing remains for it to be subtracted from.
		"orphan": {
			pendingAuthorizationDecrease: tTokens(30_000),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			operatorPublicKey, operatorAddress := newTestOperator(t)

			admission := &mockAdmissionReader{
				stakingProviders: map[common.Address]common.Address{
					operatorAddress: stakingProvider,
				},
				eligibleStakes: map[common.Address]*big.Int{
					stakingProvider: big.NewInt(0),
				},
				pendingAuthorizationDecreases: map[common.Address]*big.Int{
					stakingProvider: test.pendingAuthorizationDecrease,
				},
			}

			// The fixture is only meaningful if the pending amount it seeds is
			// actually positive; a reader answering nil would make the case
			// indistinguishable from having no pending record at all.
			testutils.AssertBigIntNonZero(
				t,
				"seeded pending authorization decrease",
				admission.pendingAuthorizationDecreases[stakingProvider],
			)

			chain := &TbtcChain{admission: admission}

			isRecognized, err := chain.IsRecognized(operatorPublicKey)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBoolsEqual(t, "recognition", false, isRecognized)
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
		"staking provider reads",
		1,
		admission.stakingProviderCalls,
	)
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

	lookupErr := errors.New("connection refused")

	chain := &TbtcChain{
		admission: &mockAdmissionReader{
			stakingProviderErr: lookupErr,
		},
	}

	isRecognized, err := chain.IsRecognized(operatorPublicKey)

	testutils.AssertBoolsEqual(t, "recognition", false, isRecognized)
	if err == nil {
		t.Fatal("expected the chain error to be returned to the caller")
	}
	if errors.Is(err, firewall.ErrNotRecognized) {
		t.Fatal("chain error was reported as a non-recognition")
	}

	// Recognition wraps rather than formats its chain errors, so the cause
	// stays reachable through errors.Is for any caller that needs to classify
	// it. The firewall's own non-recognition check does not rely on this - it
	// keys on its ErrNotRecognized sentinel - so nothing else would notice the
	// wrapping being dropped.
	testutils.AssertAnyErrorInChainMatchesTarget(t, lookupErr, err)
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
	if err == nil {
		t.Fatal("expected the chain error to be returned to the caller")
	}
	if errors.Is(err, firewall.ErrNotRecognized) {
		t.Fatal("chain error was reported as a non-recognition")
	}

	testutils.AssertAnyErrorInChainMatchesTarget(t, lookupErr, err)
}
