package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/internal/ethtest"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/spf13/cobra"
)

func TestNetworkBootstrapFlagDescription_ContainsDeprecationNotice(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.Config{}

	initNetworkFlags(cmd, cfg)

	flag := cmd.Flags().Lookup("network.bootstrap")
	if flag == nil {
		t.Fatal("expected network.bootstrap flag to be registered")
	}

	usageLower := strings.ToLower(flag.Usage)
	if !strings.Contains(usageLower, "deprecated") {
		t.Errorf(
			"expected flag description to contain deprecation notice, got: %q",
			flag.Usage,
		)
	}
}

func TestIsBootstrap(t *testing.T) {
	tests := map[string]struct {
		bootstrapValue bool
		expected       bool
	}{
		"returns true when bootstrap flag is set": {
			bootstrapValue: true,
			expected:       true,
		},
		"returns false when bootstrap flag is not set": {
			bootstrapValue: false,
			expected:       false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			originalBootstrap := clientConfig.LibP2P.Bootstrap
			defer func() { clientConfig.LibP2P.Bootstrap = originalBootstrap }()

			clientConfig.LibP2P.Bootstrap = tc.bootstrapValue

			got := isBootstrap()
			if got != tc.expected {
				t.Errorf("expected isBootstrap() to return %v, got %v", tc.expected, got)
			}
		})
	}
}

// TestInitializeNetwork_AdmissionComposition covers the assembly the client
// hands to the network provider on start: the chain handles it connects to,
// in the order they are evaluated, behind the static allow list production
// runs with. Both the application list and the policy are built by the same
// functions start and initializeNetwork call, so the composition asserted here
// is the one a running node uses.
//
// The chain handles are real, built through the public connection path against
// a deterministic endpoint; only the endpoint is a fixture. Every identity is
// judged by the reads it costs as well as by its verdict, because a verdict
// alone cannot tell an on-chain decision apart from a bypass, nor beacon-first
// evaluation apart from the reverse.
func TestInitializeNetwork_AdmissionComposition(t *testing.T) {
	backend := ethtest.New(t, ethtest.AdmissionState(t))

	beaconChain, tbtcChain, _, _, _, err := ethereum.Connect(
		context.Background(),
		backend.ChainConfig(t),
	)
	if err != nil {
		t.Fatalf("failed to connect to the fixture: %v", err)
	}

	applications := admissionApplications(beaconChain, tbtcChain)

	testutils.AssertIntsEqual(t, "applications", 2, len(applications))
	if applications[0] != firewall.Application(beaconChain) {
		t.Error("the beacon chain is not the first application evaluated")
	}
	if applications[1] != firewall.Application(tbtcChain) {
		t.Error("the tBTC chain is not the second application evaluated")
	}

	policy := admissionPolicy(applications)

	assertNoStaticBypass(t, policy)

	for _, admissionCase := range ethtest.AdmissionCases() {
		t.Run(admissionCase.Name, func(t *testing.T) {
			operatorPublicKey := caseOperatorKey(t, admissionCase)

			backend.ResetCalls()

			err := policy.Validate(operatorPublicKey)

			if admissionCase.Admitted {
				if err != nil {
					t.Fatalf("expected the peer to be admitted: %v", err)
				}
			} else {
				testutils.AssertErrorsSame(t, firewall.ErrNotRecognized, err)
			}

			// The reads the verdict cost. The beacon branch opens every one of
			// them, so this identity was judged by the chain rather than
			// short-circuited ahead of it, and the tBTC branch is read only
			// where the beacon declined.
			backend.AssertTrace(
				t,
				"admission reads",
				admissionCase.AdmissionReads(t)...,
			)
			backend.AssertNoUnexpectedCalls(t)
		})
	}

	t.Run("what a static bypass looks like", func(t *testing.T) {
		// The counterexample the assertions above are read against, built as a
		// separate policy: a peer the chain rejects, allow-listed, is admitted
		// with the chain never asked. That is what an allow list does, not
		// evidence about the one production builds - the list this test's
		// policy actually carries is inspected by assertNoStaticBypass.
		rejected := ethtest.AdmissionCaseNamed(t, "registered_unauthorized")
		operatorPublicKey := caseOperatorKey(t, rejected)

		bypassed := firewall.AnyApplicationPolicy(
			applications,
			firewall.NewAllowList(
				[]*operator.PublicKey{operatorPublicKey},
			),
		)

		backend.ResetCalls()

		if err := bypassed.Validate(operatorPublicKey); err != nil {
			t.Fatalf("an allow-listed peer was not admitted: %v", err)
		}

		backend.AssertTrace(t, "reads behind an allow list")
	})
}

// assertNoStaticBypass fails unless the given policy carries the production
// empty allow list. The list has to be read off the policy rather than probed
// through Validate, because an allow list only changes the verdict for a key
// it holds: no finite set of validated identities tells an empty list apart
// from one holding a key the test never tried. Neither the policy type nor the
// field is exported, hence the reflection.
func assertNoStaticBypass(t *testing.T, policy net.Firewall) {
	t.Helper()

	value := reflect.ValueOf(policy)
	if value.Kind() != reflect.Ptr || value.IsNil() ||
		value.Elem().Kind() != reflect.Struct {
		t.Fatalf("cannot read the allow list of a policy of type %T", policy)
	}

	allowList := value.Elem().FieldByName("allowList")
	if !allowList.IsValid() || allowList.Kind() != reflect.Ptr ||
		allowList.IsNil() {
		t.Fatalf("the policy of type %T holds no allow list", policy)
	}

	if allowList.Pointer() !=
		reflect.ValueOf(firewall.EmptyAllowList()).Pointer() {
		t.Error("the policy does not carry the production empty allow list")
	}

	allowed := allowList.Elem().FieldByName("allowedPublicKeys")
	if !allowed.IsValid() || allowed.Kind() != reflect.Map {
		t.Fatal("the allow list holds no set of allowed keys")
	}

	if allowed.Len() != 0 {
		t.Errorf(
			"the allow list bypasses the chain for %v",
			allowed.MapKeys(),
		)
	}
}

// caseOperatorKey returns the operator public key of one admission table
// identity, derived with the conversion the client applies to its own key.
func caseOperatorKey(
	t *testing.T,
	admissionCase ethtest.AdmissionCase,
) *operator.PublicKey {
	t.Helper()

	_, operatorPublicKey, err := ethereum.ChainPrivateKeyToOperatorKeyPair(
		ethtest.Key(t, admissionCase.OperatorKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	return operatorPublicKey
}
