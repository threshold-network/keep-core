package clientinfo

import (
	"context"
	"testing"
	"time"

	keepclientinfo "github.com/keep-network/keep-common/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
)

// TestConnectedWellknownPeersCountMetricName verifies that the metric constant
// for well-known peers connectivity has the correct string value used by
// Prometheus for metric registration.
func TestConnectedWellknownPeersCountMetricName(t *testing.T) {
	expected := "connected_wellknown_peers_count"
	actual := ConnectedWellknownPeersCountMetricName

	if actual != expected {
		t.Errorf(
			"expected metric name %q, got %q",
			expected,
			actual,
		)
	}
}

// TestMetricConstants verifies that all metric name constants are defined with
// the expected non-empty string values. This ensures no accidental changes to
// metric names that would break Prometheus queries and Grafana dashboards.
func TestMetricConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "connected peers count",
			constant: ConnectedPeersCountMetricName,
			expected: "connected_peers_count",
		},
		{
			name:     "connected wellknown peers count",
			constant: ConnectedWellknownPeersCountMetricName,
			expected: "connected_wellknown_peers_count",
		},
		{
			name:     "eth connectivity",
			constant: EthConnectivityMetricName,
			expected: "eth_connectivity",
		},
		{
			name:     "btc connectivity",
			constant: BtcConnectivityMetricName,
			expected: "btc_connectivity",
		},
		{
			name:     "client info",
			constant: ClientInfoMetricName,
			expected: "client_info",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.constant != tc.expected {
				t.Errorf(
					"expected metric name %q, got %q",
					tc.expected,
					tc.constant,
				)
			}
			if tc.constant == "" {
				t.Error("metric name constant must not be empty")
			}
		})
	}
}

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

// TestObserveConnectedWellknownPeersCount_Callable verifies that the renamed
// function exists on the Registry type and can be called without panicking.
func TestObserveConnectedWellknownPeersCount_Callable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}

	provider := &mockProvider{
		connectionManager: &mockConnectionManager{
			connectedAddresses: map[string]bool{
				"/ip4/127.0.0.1/tcp/3919": true,
			},
		},
	}

	// The function should execute without panic. We use a recovered call
	// to detect if the method does not exist or panics.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf(
				"ObserveConnectedWellknownPeersCount panicked: %v",
				r,
			)
		}
	}()

	registry.ObserveConnectedWellknownPeersCount(
		provider,
		[]string{"/ip4/127.0.0.1/tcp/3919"},
		1*time.Minute,
	)
}
