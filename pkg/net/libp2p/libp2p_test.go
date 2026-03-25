package libp2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/operator"

	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
)

func TestProviderReturnsType(t *testing.T) {
	ctx, cancel := newTestContext()
	defer cancel()

	operatorPrivateKey, _, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	expectedType := "libp2p"
	provider, err := Connect(
		ctx,
		generateDeterministicNetworkConfig(),
		operatorPrivateKey,
		firewall.Disabled,
		idleTicker(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if provider.Type() != expectedType {
		t.Fatalf("expected: provider type [%s]\nactual:   provider type [%s]",
			provider.Type(),
			expectedType,
		)
	}
}

func TestProviderReturnsChannel(t *testing.T) {
	ctx, cancel := newTestContext()
	defer cancel()

	testName := "testname"

	operatorPrivateKey, _, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	provider, err := Connect(
		ctx,
		generateDeterministicNetworkConfig(),
		operatorPrivateKey,
		firewall.Disabled,
		idleTicker(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = provider.BroadcastChannelFor(testName); err != nil {
		t.Fatalf("expected: test to fail with [%v]\nactual:   failed with [%v]",
			nil,
			err,
		)
	}
}

func TestSendReceive(t *testing.T) {
	ctx, cancel := newTestContext()
	defer cancel()

	var (
		config          = generateDeterministicNetworkConfig()
		name            = "testchannel"
		expectedPayload = "some text"
	)

	operatorPrivateKey, _, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	networkPrivateKey, _, err := operatorPrivateKeyToNetworkKeyPair(operatorPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	identity, err := createIdentity(networkPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	provider, err := Connect(
		ctx,
		config,
		operatorPrivateKey,
		firewall.Disabled,
		idleTicker(),
	)
	if err != nil {
		t.Fatal(err)
	}
	broadcastChannel, err := provider.BroadcastChannelFor(name)
	if err != nil {
		t.Fatal(err)
	}

	broadcastChannel.SetUnmarshaler(
		func() net.TaggedUnmarshaler { return &testMessage{} },
	)

	if err := broadcastChannel.Send(
		ctx,
		&testMessage{Sender: identity, Payload: expectedPayload},
	); err != nil {
		t.Fatal(err)
	}

	recvChan := make(chan net.Message)
	broadcastChannel.Recv(ctx, func(msg net.Message) {
		recvChan <- msg
	})

	for {
		select {
		case msg := <-recvChan:
			testPayload, ok := msg.Payload().(*testMessage)
			if !ok {
				t.Fatalf(
					"expected: payload type string\nactual:   payload type [%v]",
					testPayload,
				)
			}

			if expectedPayload != testPayload.Payload {
				t.Fatalf(
					"expected: message payload [%s]\ngot:   payload [%s]",
					expectedPayload,
					testPayload.Payload,
				)
			}
			return
		case <-ctx.Done():
			t.Fatal(err)
		}
	}
}

func TestProviderSetAnnouncedAddresses(t *testing.T) {
	ctx, cancel := newTestContext()
	defer cancel()

	operatorPrivateKey, _, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	config := generateDeterministicNetworkConfig()
	config.AnnouncedAddresses = []string{
		"/bad/address",
		"/dns4/address.com/tcp/3919",
		"totallyBadAddress",
		"/ip4/100.20.50.30/tcp/3919",
	}

	provider, err := Connect(
		ctx,
		config,
		operatorPrivateKey,
		firewall.Disabled,
		idleTicker(),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedAddresses := []string{
		fmt.Sprintf("/dns4/address.com/tcp/3919/ipfs/%v", provider.ID()),
		fmt.Sprintf("/ip4/100.20.50.30/tcp/3919/ipfs/%v", provider.ID()),
	}
	providerAddresses := provider.ConnectionManager().AddrStrings()
	// Compare as sets; libp2p address order may vary by version
	sort.Strings(expectedAddresses)
	got := make([]string, len(providerAddresses))
	copy(got, providerAddresses)
	sort.Strings(got)
	if strings.Join(expectedAddresses, " ") != strings.Join(got, " ") {
		t.Fatalf(
			"expected: provider addresses [%v]\nactual: provider addresses [%v]",
			expectedAddresses,
			providerAddresses,
		)
	}
}

type testMessage struct {
	Sender    *identity
	Recipient *identity
	Payload   string
}

func (m *testMessage) Type() string {
	return "test/unmarshaler"
}

func (m *testMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *testMessage) Unmarshal(bytes []byte) error {
	var message testMessage
	if err := json.Unmarshal(bytes, &message); err != nil {
		return err
	}
	m.Sender = message.Sender
	m.Recipient = message.Recipient
	m.Payload = message.Payload

	return nil
}

func newTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func idleTicker() *retransmission.Ticker {
	ticks := make(chan uint64)
	close(ticks)
	return retransmission.NewTicker(ticks)
}

func generateDeterministicNetworkConfig() Config {
	return Config{Port: 8080}
}
