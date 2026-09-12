package ethereum

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/firewall"
)

// mockBeaconAdmissionReader stands in for the chain reads the beacon
// admission predicate performs.
type mockBeaconAdmissionReader struct {
	stakingProviders    map[common.Address]common.Address
	hasStakeDelegations map[common.Address]bool

	stakingProviderErr    error
	hasStakeDelegationErr error

	stakingProviderCalls    int
	hasStakeDelegationCalls int
}

func (mbar *mockBeaconAdmissionReader) OperatorToStakingProvider(
	operatorAddress common.Address,
) (common.Address, error) {
	mbar.stakingProviderCalls++

	if mbar.stakingProviderErr != nil {
		return common.Address{}, mbar.stakingProviderErr
	}

	return mbar.stakingProviders[operatorAddress], nil
}

func (mbar *mockBeaconAdmissionReader) HasStakeDelegation(
	stakingProvider common.Address,
) (bool, error) {
	mbar.hasStakeDelegationCalls++

	if mbar.hasStakeDelegationErr != nil {
		return false, mbar.hasStakeDelegationErr
	}

	return mbar.hasStakeDelegations[stakingProvider], nil
}

// TestBeaconChain_IsRecognized_HasStakeDelegation pins the beacon/tBTC
// predicate asymmetry: the beacon branch recognizes an operator purely on
// whether its staking provider currently has, or ever had, a stake
// delegation, regardless of eligible stake.
func TestBeaconChain_IsRecognized_HasStakeDelegation(t *testing.T) {
	stakingProvider := common.HexToAddress("0x1")

	var tests = map[string]struct {
		hasStakeDelegation bool
		expectedRecognized bool
	}{
		"staking provider with a stake delegation": {
			hasStakeDelegation: true,
			expectedRecognized: true,
		},
		"staking provider with no stake delegation": {
			hasStakeDelegation: false,
			expectedRecognized: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			operatorPublicKey, operatorAddress := newTestOperator(t)

			chain := &BeaconChain{
				admission: &mockBeaconAdmissionReader{
					stakingProviders: map[common.Address]common.Address{
						operatorAddress: stakingProvider,
					},
					hasStakeDelegations: map[common.Address]bool{
						stakingProvider: test.hasStakeDelegation,
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

// TestBeaconChain_IsRecognized_UnregisteredOperator covers an operator that
// has never been mapped to a staking provider. The predicate must
// short-circuit before reading the stake delegation, so that recognition
// costs exactly one chain call for an unknown peer.
func TestBeaconChain_IsRecognized_UnregisteredOperator(t *testing.T) {
	operatorPublicKey, _ := newTestOperator(t)

	admission := &mockBeaconAdmissionReader{
		stakingProviders:    map[common.Address]common.Address{},
		hasStakeDelegations: map[common.Address]bool{},
	}

	chain := &BeaconChain{admission: admission}

	isRecognized, err := chain.IsRecognized(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBoolsEqual(t, "recognition", false, isRecognized)
	testutils.AssertIntsEqual(
		t,
		"stake delegation reads",
		0,
		admission.hasStakeDelegationCalls,
	)
}

// TestBeaconChain_IsRecognized_StakingProviderLookupFails asserts the
// predicate fails closed: a chain error propagates instead of being reported
// as a negative recognition.
func TestBeaconChain_IsRecognized_StakingProviderLookupFails(t *testing.T) {
	operatorPublicKey, _ := newTestOperator(t)

	lookupErr := errors.New("connection refused")

	chain := &BeaconChain{
		admission: &mockBeaconAdmissionReader{
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

	testutils.AssertAnyErrorInChainMatchesTarget(t, lookupErr, err)
}

// TestBeaconChain_IsRecognized_HasStakeDelegationLookupFails asserts the same
// fail-closed behaviour for the read the predicate actually decides on.
func TestBeaconChain_IsRecognized_HasStakeDelegationLookupFails(t *testing.T) {
	operatorPublicKey, operatorAddress := newTestOperator(t)

	stakingProvider := common.HexToAddress("0x1")
	lookupErr := errors.New("connection refused")

	chain := &BeaconChain{
		admission: &mockBeaconAdmissionReader{
			stakingProviders: map[common.Address]common.Address{
				operatorAddress: stakingProvider,
			},
			hasStakeDelegationErr: lookupErr,
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
