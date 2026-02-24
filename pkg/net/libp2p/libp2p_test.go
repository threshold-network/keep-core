package libp2p

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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

func TestExtractPeersPublicKeys_EmptyList(t *testing.T) {
	peerAddresses := []string{}
	peerOperatorPublicKeys, err := ExtractPeersPublicKeys(peerAddresses)
	if err != nil {
		t.Fatal(err)
	}

	if len(peerOperatorPublicKeys) != len(peerAddresses) {
		t.Errorf(
			"unexpected peer operator public keys length\nexpected: %v\n"+
				"actual:   %v\n",
			len(peerAddresses),
			len(peerOperatorPublicKeys),
		)
	}
}

func TestExtractPeersPublicKeys_CorrectPeerAddresses(t *testing.T) {
	peerAddresses := []string{
		"/ip4/127.0.0.1/tcp/3919/ipfs/" +
			"16Uiu2HAmNpUbaz8UptSL1aWTNnR1GmcV6Pw1kSV5xkep3N44zi3m",
		"/ip4/127.0.0.1/tcp/3920/ipfs/" +
			"16Uiu2HAmQA19uJUtvMp7ZGCED7maXjQZCpdkLnEGCmxPRJRCvwJt",
		"/ip4/127.0.0.1/tcp/3921/ipfs/" +
			"16Uiu2HAm5N75v5gmMiSaR422q6RH2QfPxWVJkRjySonG3UbnmnnQ",
	}

	peerOperatorPublicKeys, err := ExtractPeersPublicKeys(peerAddresses)
	if err != nil {
		t.Fatal(err)
	}

	if len(peerOperatorPublicKeys) != len(peerAddresses) {
		t.Errorf(
			"unexpected peer operator public keys length\nexpected: %v\n"+
				"actual:   %v\n",
			len(peerAddresses),
			len(peerOperatorPublicKeys),
		)
	}

	// Convert to strings for easier testing
	actualPeerOperatorPublicKeys := make([]string, len(peerOperatorPublicKeys))
	for i, key := range peerOperatorPublicKeys {
		actualPeerOperatorPublicKeys[i] = key.String()
	}

	expectedPeerOperatorPublicKeys := []string{
		"03970308f34ba0397e4a54713c126e63b8e42effcce9766d30776f24571796c39c",
		"03aadf4ef0d4836404e5f06de50b05b9273e6b6b52b8c8726dae2735882d9354dd",
		"0293aaeed76b0636b1c464f1c20a2f73936c175bae01a469f89c66f0e963fdc24d",
	}

	if !reflect.DeepEqual(
		expectedPeerOperatorPublicKeys,
		actualPeerOperatorPublicKeys,
	) {
		t.Errorf(
			"unexpected peer operator public keys\nexpected: %v\nactual:   %v\n",
			expectedPeerOperatorPublicKeys,
			actualPeerOperatorPublicKeys,
		)
	}
}

func TestExtractPeersPublicKeys_IncorrectPeerAddresses(t *testing.T) {
	// Make the second address too short to cause an error
	peerAddresses := []string{
		"/ip4/127.0.0.1/tcp/3919/ipfs/" +
			"16Uiu2HAmNpUbaz8UptSL1aWTNnR1GmcV6Pw1kSV5xkep3N44zi3m",
		"/ip4/127.0.0.1/tcp/3920/ipfs/" +
			"16Uiu2HAmQA19uJUtvMp7ZGCED7maXjQZCpdkLnEGCmxPRJRCvwJ",
		"/ip4/127.0.0.1/tcp/3921/ipfs/" +
			"16Uiu2HAm5N75v5gmMiSaR422q6RH2QfPxWVJkRjySonG3UbnmnnQ",
	}

	_, err := ExtractPeersPublicKeys(peerAddresses)

	expectedError := fmt.Errorf(
		"failed to extract multiaddress from peer addresses: " +
			"[failed to parse multiaddr \"/ip4/127.0.0.1/tcp/3920/ipfs/" +
			"16Uiu2HAmQA19uJUtvMp7ZGCED7maXjQZCpdkLnEGCmxPRJRCvwJ\": " +
			"invalid value " +
			"\"16Uiu2HAmQA19uJUtvMp7ZGCED7maXjQZCpdkLnEGCmxPRJRCvwJ\" " +
			"for protocol p2p: failed to parse p2p addr: " +
			"16Uiu2HAmQA19uJUtvMp7ZGCED7maXjQZCpdkLnEGCmxPRJRCvwJ " +
			"length greater than remaining number of bytes in buffer]",
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
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
