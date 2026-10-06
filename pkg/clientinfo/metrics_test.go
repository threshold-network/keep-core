package clientinfo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
)

// mockTransportIdentifier implements net.TransportIdentifier for testing.
type mockTransportIdentifier struct{}

func (m *mockTransportIdentifier) String() string { return "mock-id" }

// mockConnectionManager implements net.ConnectionManager for testing.
type mockConnectionManager struct {
	connectedAddresses map[string]bool
}

func (m *mockConnectionManager) ConnectedPeers() []string { return nil }
func (m *mockConnectionManager) ConnectedPeersAddrInfo() map[string][]string {
	return nil
}
func (m *mockConnectionManager) GetPeerPublicKey(string) (*operator.PublicKey, error) {
	return nil, nil
}
func (m *mockConnectionManager) DisconnectPeer(string) {}
func (m *mockConnectionManager) AddrStrings() []string { return nil }
func (m *mockConnectionManager) IsConnected(address string) bool {
	if m.connectedAddresses == nil {
		return false
	}
	return m.connectedAddresses[address]
}

// mockProvider implements net.Provider for testing.
type mockProvider struct {
	connectionManager net.ConnectionManager
}

func (m *mockProvider) ID() net.TransportIdentifier { return &mockTransportIdentifier{} }
func (m *mockProvider) Type() string                { return "mock" }
func (m *mockProvider) BroadcastChannelFor(string) (net.BroadcastChannel, error) {
	return nil, nil
}
func (m *mockProvider) ConnectionManager() net.ConnectionManager {
	return m.connectionManager
}
func (m *mockProvider) CreateTransportIdentifier(
	*operator.PublicKey,
) (net.TransportIdentifier, error) {
	return nil, nil
}
func (m *mockProvider) BroadcastChannelForwarderFor(string) {}

// TestObserveConnectedWellknownPeersCount verifies that the
// connected_wellknown_peers_count gauge reflects only the wellknown peers that
// are actually connected. Asserting merely that the call does not panic left
// the counting loop - the one piece of logic here - unverified.
func TestObserveConnectedWellknownPeersCount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := newRegistry(ctx)

	provider := &mockProvider{
		connectionManager: &mockConnectionManager{
			connectedAddresses: map[string]bool{
				"/ip4/127.0.0.1/tcp/3919": true,
			},
		},
	}

	registry.ObserveConnectedWellknownPeersCount(
		provider,
		[]string{
			"/ip4/127.0.0.1/tcp/3919", // connected
			"/ip4/10.0.0.1/tcp/1",     // not connected
		},
		1*time.Minute,
	)

	registry.metricsMutex.RLock()
	gauge, ok := registry.metrics[ConnectedWellknownPeersCountMetricName].(*Gauge)
	registry.metricsMutex.RUnlock()
	if !ok {
		t.Fatal("connected_wellknown_peers_count gauge was not registered")
	}

	// Observe sets the gauge from the input source on a background goroutine
	// as soon as it starts; poll briefly for that first tick.
	deadline := time.Now().Add(time.Second)
	for {
		if strings.Contains(
			gauge.expose(),
			ConnectedWellknownPeersCountMetricName+" 1 ",
		) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"expected exactly one of two wellknown peers to be counted, "+
					"got: %s",
				gauge.expose(),
			)
		}
		time.Sleep(time.Millisecond)
	}
}
