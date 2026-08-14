//go:build frost_native

package signing

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/net"
)

type captureRunnerBroadcastChannel struct {
	sent []*runnerTransportMessage
}

func (*captureRunnerBroadcastChannel) Name() string { return "runner-activation-test" }
func (channel *captureRunnerBroadcastChannel) Send(
	_ context.Context,
	message net.TaggedMarshaler,
	_ ...net.RetransmissionStrategy,
) error {
	wire, ok := message.(*runnerTransportMessage)
	if ok {
		copy := *wire
		copy.payload = append([]byte(nil), wire.payload...)
		copy.activationLease = append([]byte(nil), wire.activationLease...)
		channel.sent = append(channel.sent, &copy)
	}
	return nil
}
func (*captureRunnerBroadcastChannel) Recv(context.Context, func(net.Message))     {}
func (*captureRunnerBroadcastChannel) SetUnmarshaler(func() net.TaggedUnmarshaler) {}
func (*captureRunnerBroadcastChannel) SetFilter(net.BroadcastChannelFilter) error  { return nil }

func installTestShareRepairRegistry(
	t *testing.T,
	localStore string,
) (*ShareRepairAuthorization, []byte) {
	t.Helper()
	authorization, authority := testShareRepairAuthorization(t)
	lease := testShareRepairActivationLease(t, authorization, authority)
	registry := &ShareRepairActivationRegistry{
		Schema: ShareRepairActivationRegistrySchema,
		Leases: []ShareRepairActivationLease{lease},
	}
	publicKey := authority.Public().(ed25519.PublicKey)
	root, err := ShareRepairActivationRegistryRoot(registry, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	storeFingerprint, err := parseCanonicalShareRepairHex32(
		localStore,
		"local_store_fingerprint",
	)
	if err != nil {
		t.Fatal(err)
	}
	configureTestShareRepairActivationGuard(
		t,
		authorization,
		storeFingerprint,
		localStore == authorization.NewStoreFingerprint,
	)
	if err := InstallShareRepairActivationRegistry(
		payload,
		publicKey,
		root,
		storeFingerprint,
	); err != nil {
		t.Fatal(err)
	}
	wire, err := shareRepairActivationLeaseForBroadcast(
		authorization.KeyGroup,
		3,
	)
	if localStore == authorization.NewStoreFingerprint && (err != nil || len(wire) == 0) {
		t.Fatalf("expected active-store lease: %v", err)
	}
	return authorization, wire
}

func TestActivationBoundRunnerBusRequiresExactRecoveredSeatLease(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, leaseWire := installTestShareRepairRegistry(
		t,
		testShareRepairHex32(0x52),
	)
	fixture := newRunnerBusAuthFixture(t, 8)
	channel := &captureRunnerBroadcastChannel{}
	busInterface, err := NewActivationBoundBroadcastChannelRunnerBus(
		context.Background(),
		&testutils.MockLogger{},
		channel,
		fixture.validator,
		authorization.KeyGroup,
	)
	if err != nil {
		t.Fatal(err)
	}
	bus := busInterface.(*broadcastChannelRunnerBus)
	if !bus.activationBound {
		t.Fatal("non-zero signed registry root did not select the v2 transport")
	}
	subscriber := bus.Subscribe()
	message := RunnerMessage{
		Type:    RunnerMsgShareSubmission,
		Sender:  3,
		Attempt: [32]byte{0x44},
		Payload: []byte("recovered-share"),
	}
	bus.Broadcast(message)
	if len(channel.sent) != 1 || !channel.sent[0].activationBound ||
		channel.sent[0].Type() != "frost/roast_runner/v2/share_submission" ||
		string(channel.sent[0].activationLease) != string(leaseWire) {
		t.Fatalf("recovered-seat broadcast omitted the exact activation lease: %+v", channel.sent)
	}

	missingLease := fakeNetMessage{
		senderPublicKey: fixture.operatorA,
		payload: &runnerTransportMessage{
			messageType:     message.Type,
			sender:          message.Sender,
			attempt:         message.Attempt,
			payload:         message.Payload,
			activationBound: true,
		},
	}
	bus.handleMessage(missingLease)
	select {
	case <-subscriber.Shares():
		t.Fatal("recovered-seat message without a lease was delivered")
	default:
	}

	exactLease := missingLease
	wire := *missingLease.payload.(*runnerTransportMessage)
	wire.activationLease = append([]byte(nil), leaseWire...)
	exactLease.payload = &wire
	bus.handleMessage(exactLease)
	select {
	case received := <-subscriber.Shares():
		if received.Sender != 3 || string(received.Payload) != "recovered-share" {
			t.Fatalf("unexpected recovered-seat delivery: %+v", received)
		}
	default:
		t.Fatal("exact authority-signed active-store lease was not delivered")
	}

	encoded, err := channel.sent[0].Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := &runnerTransportMessage{
		messageType:     RunnerMsgShareSubmission,
		activationBound: true,
	}
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded.activationLease) != string(leaseWire) ||
		string(decoded.payload) != "recovered-share" {
		t.Fatal("v2 activation-bound frame did not round-trip")
	}
}

func TestActivationBoundRunnerBusRefusesPendingRecoveredSeatWithoutRegistry(t *testing.T) {
	resetShareRepairActivationRegistryForTest()
	t.Cleanup(resetShareRepairActivationRegistryForTest)
	authorization, _ := testShareRepairAuthorization(t)
	newStore, err := parseCanonicalShareRepairHex32(
		authorization.NewStoreFingerprint,
		"new_store_fingerprint",
	)
	if err != nil {
		t.Fatal(err)
	}
	configureTestShareRepairActivationGuard(t, authorization, newStore, true)
	fixture := newRunnerBusAuthFixture(t, 8)
	channel := &captureRunnerBroadcastChannel{}
	bus, err := NewActivationBoundBroadcastChannelRunnerBus(
		context.Background(),
		&testutils.MockLogger{},
		channel,
		fixture.validator,
		authorization.KeyGroup,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bus.(*broadcastChannelRunnerBus).activationBound {
		t.Fatal("pending recovered seat did not force the v2 transport")
	}
	bus.Broadcast(RunnerMessage{
		Type:    RunnerMsgCommitments,
		Sender:  3,
		Attempt: [32]byte{0x45},
		Payload: []byte("must-not-leave-old-store"),
	})
	if len(channel.sent) != 0 {
		t.Fatal("pending recovered seat broadcast before signed cutover")
	}
}
