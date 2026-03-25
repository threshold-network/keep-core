package clientinfo

import (
	"context"
	"testing"
	"time"

	keepclientinfo "github.com/keep-network/keep-common/pkg/clientinfo"
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
