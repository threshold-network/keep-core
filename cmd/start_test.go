package cmd

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/internal/ethtest"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
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

// TestStart_AdmissionHandoff runs the client's own start path and reads back
// the firewall it hands the network layer. The chain handles guarding the
// network are the ones start connected to and the policy is the one
// initializeNetwork built, so the composition asserted here is the one a
// running node admits peers with rather than one the test assembled for
// itself. Opening the provider is where the path is stopped, which is the last
// point at which the handoff is still observable.
//
// The chain behind the handles is a deterministic endpoint; only the endpoint
// is a fixture.
func TestStart_AdmissionHandoff(t *testing.T) {
	backend := ethtest.New(t, ethtest.AdmissionState(t))

	configured := clientConfig.Ethereum
	t.Cleanup(func() { clientConfig.Ethereum = configured })
	clientConfig.Ethereum = backend.ChainConfig(t)

	policy := captureAdmissionPolicy(t, func() error {
		return start(&cobra.Command{})
	})

	// What the policy admits comes first: the assertions below give up on a
	// policy whose shape they cannot read, and stopping there would leave the
	// verdicts unexamined.
	assertAdmissionTable(t, backend, policy)
	assertNoStaticBypass(t, policy)

	// start builds its own chain handles, so there is no instance here to
	// compare them against; what it handed over is pinned by type and order.
	assertAdmissionApplicationTypes(
		t,
		policy,
		reflect.TypeOf((*ethereum.BeaconChain)(nil)),
		reflect.TypeOf((*ethereum.TbtcChain)(nil)),
	)
}

// TestInitializeNetwork_AdmissionComposition covers the assembly the client
// hands to the network provider on start: the chain handles it connects to,
// in the order they are evaluated, behind the static allow list production
// runs with. The policy is read back from the network handoff rather than
// rebuilt here, so what is asserted is what initializeNetwork passed on.
//
// The chain handles are real, built through the public connection path against
// a deterministic endpoint; only the endpoint is a fixture.
func TestInitializeNetwork_AdmissionComposition(t *testing.T) {
	backend := ethtest.New(t, ethtest.AdmissionState(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beaconChain, tbtcChain, blockCounter, _, operatorPrivateKey, err :=
		ethereum.Connect(ctx, backend.ChainConfig(t))
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

	policy := captureAdmissionPolicy(t, func() error {
		_, err := initializeNetwork(
			ctx,
			applications,
			operatorPrivateKey,
			blockCounter,
		)
		return err
	})

	assertAdmissionTable(t, backend, policy)
	assertNoStaticBypass(t, policy)

	// The policy guards with the very handles it was given, in that order,
	// rather than with a set assembled somewhere between here and the network.
	assertAdmissionApplications(t, policy, beaconChain, tbtcChain)

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

// TestStaticBypassKeys_ReportsAnAllowedKey is the negative control for
// assertNoStaticBypass: a policy carrying one unrelated key has to be reported
// as bypassing the chain. Without it, an assertion that only ever sees empty
// allow lists passes whether or not it can tell a populated one apart.
func TestStaticBypassKeys_ReportsAnAllowedKey(t *testing.T) {
	allowed := caseOperatorKey(
		t,
		ethtest.AdmissionCaseNamed(t, "unregistered"),
	)

	bypassed := staticBypassKeys(t, firewall.AnyApplicationPolicy(
		nil,
		firewall.NewAllowList([]*operator.PublicKey{allowed}),
	))

	testutils.AssertIntsEqual(t, "statically bypassed keys", 1, len(bypassed))
	if len(bypassed) == 1 && bypassed[0] != allowed.String() {
		t.Errorf(
			"the reported bypass is %q, not the allow-listed key %q",
			bypassed[0],
			allowed.String(),
		)
	}

	// The same reading of the list production builds reports nothing, so the
	// difference above is the list's contents and not the way it is read.
	testutils.AssertIntsEqual(
		t,
		"statically bypassed keys of the production list",
		0,
		len(staticBypassKeys(t, admissionPolicy(nil))),
	)
}

// errNetworkNotOpened stops the start path at the network handoff. It is the
// stub constructor's own failure, so a path that got there for any other
// reason does not look like a successful capture.
var errNetworkNotOpened = errors.New("the network provider is not opened here")

// captureAdmissionPolicy runs a piece of the client's start path with the
// network provider constructor stubbed out, and returns the firewall that path
// handed it. The stub fails, so the path unwinds at the handoff and nothing
// behind it is started.
func captureAdmissionPolicy(t *testing.T, run func() error) net.Firewall {
	t.Helper()

	var captured net.Firewall
	handoffs := 0

	opened := connectNetwork
	t.Cleanup(func() { connectNetwork = opened })

	connectNetwork = func(
		_ context.Context,
		_ libp2p.Config,
		_ *operator.PrivateKey,
		policy net.Firewall,
		_ *retransmission.Ticker,
		_ ...libp2p.ConnectOption,
	) (net.Provider, error) {
		handoffs++
		captured = policy
		return nil, errNetworkNotOpened
	}

	// The failure is wrapped on the way out with %v rather than %w, so the
	// stub is recognized by its message.
	err := run()
	if err == nil ||
		!strings.Contains(err.Error(), errNetworkNotOpened.Error()) {
		t.Fatalf("the start path did not reach the network handoff: %v", err)
	}

	testutils.AssertIntsEqual(t, "network handoffs", 1, handoffs)

	if captured == nil {
		t.Fatal("the network layer was opened with no firewall")
	}

	return captured
}

// assertAdmissionTable drives every identity of the fixed admission table
// through the given policy and fails unless both the verdict and the reads
// that verdict cost are the ones the table pins. The reads are asserted
// alongside the verdict because a verdict alone cannot tell an on-chain
// decision apart from a bypass, nor beacon-first evaluation apart from the
// reverse.
func assertAdmissionTable(
	t *testing.T,
	backend *ethtest.Backend,
	policy net.Firewall,
) {
	t.Helper()

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

			// The beacon branch opens every one of these reads, so this
			// identity was judged by the chain rather than short-circuited
			// ahead of it, and the tBTC branch is read only where the beacon
			// declined.
			backend.AssertTrace(
				t,
				"admission reads",
				admissionCase.AdmissionReads(t)...,
			)
			backend.AssertNoUnexpectedCalls(t)
		})
	}
}

// assertAdmissionApplications fails unless the given policy evaluates exactly
// the given applications, in order, each of them the same instance.
func assertAdmissionApplications(
	t *testing.T,
	policy net.Firewall,
	expected ...firewall.Application,
) {
	t.Helper()

	evaluated, ok := policyApplications(t, policy, len(expected))
	if !ok {
		return
	}

	for index, want := range expected {
		if evaluated[index].Pointer() != reflect.ValueOf(want).Pointer() {
			t.Errorf(
				"application %d is not the %T the policy was built from",
				index,
				want,
			)
		}
	}
}

// assertAdmissionApplicationTypes is the same assertion for a caller that
// cannot name the instances, because the path under test built them itself.
func assertAdmissionApplicationTypes(
	t *testing.T,
	policy net.Firewall,
	expected ...reflect.Type,
) {
	t.Helper()

	evaluated, ok := policyApplications(t, policy, len(expected))
	if !ok {
		return
	}

	for index, want := range expected {
		if evaluated[index].Type() != want {
			t.Errorf(
				"application %d is a %v, not a %v",
				index,
				evaluated[index].Type(),
				want,
			)
		}
	}
}

// policyApplications reads the applications the given policy evaluates, in the
// order it evaluates them, and reports whether there are exactly count of them
// - a caller comparing them one by one has nothing to say once the counts
// disagree. Neither the policy type nor the field is exported, hence the
// reflection, and the applications come back as reflect.Values because a value
// read out of an unexported field cannot be handed back as an interface; its
// identity and its dynamic type still can be compared.
func policyApplications(
	t *testing.T,
	policy net.Firewall,
	count int,
) ([]reflect.Value, bool) {
	t.Helper()

	value := reflect.ValueOf(policy)
	if value.Kind() != reflect.Ptr || value.IsNil() ||
		value.Elem().Kind() != reflect.Struct {
		t.Fatalf("cannot read the applications of a policy of type %T", policy)
	}

	field := value.Elem().FieldByName("applications")
	if !field.IsValid() || field.Kind() != reflect.Slice {
		t.Fatalf("the policy of type %T evaluates no applications", policy)
	}

	applications := make([]reflect.Value, 0, field.Len())
	for index := 0; index < field.Len(); index++ {
		applications = append(applications, field.Index(index).Elem())
	}

	testutils.AssertIntsEqual(
		t,
		"applications the policy evaluates",
		count,
		len(applications),
	)

	return applications, len(applications) == count
}

// assertNoStaticBypass fails unless the given policy admits nobody without
// asking the chain.
func assertNoStaticBypass(t *testing.T, policy net.Firewall) {
	t.Helper()

	if bypassed := staticBypassKeys(t, policy); len(bypassed) != 0 {
		t.Errorf("the policy bypasses the chain for %v", bypassed)
	}
}

// staticBypassKeys returns the operator keys the given policy admits without
// asking the chain, that is, the contents of its static allow list. The list
// has to be read off the policy rather than probed through Validate, because
// an allow list only changes the verdict for a key it holds: no finite set of
// validated identities tells an empty list apart from one holding a key the
// test never tried. Neither the policy type nor the field is exported, hence
// the reflection.
func staticBypassKeys(t *testing.T, policy net.Firewall) []string {
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

	allowed := allowList.Elem().FieldByName("allowedPublicKeys")
	if !allowed.IsValid() || allowed.Kind() != reflect.Map {
		t.Fatal("the allow list holds no set of allowed keys")
	}

	keys := make([]string, 0, allowed.Len())
	for _, key := range allowed.MapKeys() {
		keys = append(keys, key.String())
	}

	return keys
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
