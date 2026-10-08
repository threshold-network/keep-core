package ethereum

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/ethtest"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/firewall"
)

// TestAdmission_TransientChainFaultIsNotADenial walks every chain read the
// admission predicate performs and, for each of them in turn, makes the
// endpoint fail. A read failing says nothing about the peer, so validation has
// to surface the failure rather than report a non-recognition: the firewall
// caches a non-recognition for an hour and would lock a legitimate peer out
// for that hour over a momentary fault. Once the fault clears, the very next
// validation must read the chain again and admit.
//
// The identity used throughout is admitted by positive eligible stake.
func TestAdmission_TransientChainFaultIsNotADenial(t *testing.T) {
	var tests = map[string]struct {
		contract string
		method   string
	}{
		"tbtc_operator_lookup": {
			contract: ethtest.WalletRegistryContract,
			method:   "operatorToStakingProvider",
		},
		"tbtc_eligible_stake_lookup": {
			contract: ethtest.WalletRegistryContract,
			method:   "eligibleStake",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			backend, _, tbtcChain := connectAdmissionFixture(t)

			admitted := ethtest.AdmissionCaseNamed(t, "post_legacy_authorized")
			operatorPublicKey := caseOperatorKey(t, admitted)

			policy := firewall.AnyApplicationPolicy(
				[]firewall.Application{tbtcChain},
				firewall.EmptyAllowList(),
			)

			fault := "the endpoint dropped the connection"
			backend.Fail(test.contract, test.method, fault)
			backend.ResetCalls()

			err := policy.Validate(operatorPublicKey)
			if err == nil {
				t.Fatal("expected the chain fault to surface from validation")
			}
			if errors.Is(err, firewall.ErrNotRecognized) {
				t.Fatal("chain fault was reported as a non-recognition")
			}
			if !strings.Contains(err.Error(), fault) {
				t.Errorf(
					"the underlying chain fault is not discoverable in [%v]",
					err,
				)
			}

			// Validation stopped at the read that failed and asked for
			// nothing after it.
			backend.AssertTrace(
				t,
				"reads up to the fault",
				readsThroughFault(
					admitted.AdmissionReads(t),
					test.contract,
					test.method,
				)...,
			)

			backend.Recover(test.contract, test.method)
			backend.ResetCalls()

			if err := policy.Validate(operatorPublicKey); err != nil {
				t.Fatalf(
					"the peer was not admitted once the chain recovered: %v",
					err,
				)
			}

			// Nothing was cached, so every read is performed again rather than
			// answered from the previous attempt - including the one that had
			// failed.
			backend.AssertTrace(
				t,
				"reads after the fault cleared",
				admitted.AdmissionReads(t)...,
			)
			backend.AssertNoUnexpectedCalls(t)
		})
	}

	t.Run("beacon_faults_are_not_admission_dependencies", func(t *testing.T) {
		backend, _, tbtcChain := connectAdmissionFixture(t)
		admitted := ethtest.AdmissionCaseNamed(t, "post_legacy_authorized")
		policy := firewall.AnyApplicationPolicy(
			[]firewall.Application{tbtcChain},
			firewall.EmptyAllowList(),
		)

		backend.Fail(
			ethtest.RandomBeaconContract,
			"operatorToStakingProvider",
			"beacon mapping unavailable",
		)
		backend.Fail(
			ethtest.TokenStakingContract,
			"rolesOf",
			"legacy roles unavailable",
		)
		backend.ResetCalls()

		if err := policy.Validate(caseOperatorKey(t, admitted)); err != nil {
			t.Fatalf("beacon faults changed the admission verdict: %v", err)
		}

		backend.AssertTrace(
			t,
			"admission reads with beacon faults",
			admitted.AdmissionReads(t)...,
		)
		backend.AssertNoUnexpectedCalls(t)
	})
}

// TestAdmission_GenuineDenialIsCached characterizes what a real denial costs,
// as opposed to the chain fault above. A peer no application recognizes is
// remembered as unrecognized for NegativeIsRecognizedCachePeriod, and nothing
// happening on chain within that period is observed: authorizing the provider
// does not readmit the peer, and no chain read is even attempted. Only a
// policy that has not already denied it sees the new state.
func TestAdmission_GenuineDenialIsCached(t *testing.T) {
	backend, _, tbtcChain := connectAdmissionFixture(t)

	if firewall.NegativeIsRecognizedCachePeriod != time.Hour {
		t.Fatalf(
			"denials are cached for [%v], not the hour this test describes",
			firewall.NegativeIsRecognizedCachePeriod,
		)
	}

	denied := ethtest.AdmissionCaseNamed(t, "registered_unauthorized")
	operatorPublicKey := caseOperatorKey(t, denied)

	applications := []firewall.Application{tbtcChain}
	policy := firewall.AnyApplicationPolicy(
		applications,
		firewall.EmptyAllowList(),
	)

	testutils.AssertErrorsSame(
		t,
		firewall.ErrNotRecognized,
		policy.Validate(operatorPublicKey),
	)

	// The provider is authorized for the wallet registry. On chain the peer is
	// now recognizable; the policy that already denied it is not looking.
	backend.SetEligibleStake(denied.StakingProvider(t), ethtest.TTokens(40_000))
	backend.ResetCalls()

	testutils.AssertErrorsSame(
		t,
		firewall.ErrNotRecognized,
		policy.Validate(operatorPublicKey),
	)
	testutils.AssertIntsEqual(
		t,
		"chain reads while the denial is cached",
		0,
		len(backend.Calls()),
	)

	// A policy instance that has not denied this peer before holds no cache
	// entry for it, so it reads the chain and finds the authorization. This is
	// a fresh policy, not the cached denial above expiring.
	freshPolicy := firewall.AnyApplicationPolicy(
		applications,
		firewall.EmptyAllowList(),
	)

	if err := freshPolicy.Validate(operatorPublicKey); err != nil {
		t.Fatalf(
			"a policy holding no denial for the peer did not admit it: %v",
			err,
		)
	}

	backend.AssertNoUnexpectedCalls(t)
}

// readsThroughFault is the trace a validation produces when the given read is
// the one that fails: everything up to that read, then that read twice. The
// repeat is keep-common's error resolver, not anything go-ethereum does: the
// generated contract wrapper hands a failed call to ErrorResolver.ResolveError,
// which re-invokes the same method on the same contract to decode a revert
// reason out of the answer. A failing read therefore costs two calls where a
// successful one costs a single call.
func readsThroughFault(
	reads []ethtest.ExpectedCall,
	contractName string,
	method string,
) []ethtest.ExpectedCall {
	for index, read := range reads {
		if read.Contract != contractName || read.Method != method {
			continue
		}

		return append(append([]ethtest.ExpectedCall{}, reads[:index+1]...), read)
	}

	return reads
}
