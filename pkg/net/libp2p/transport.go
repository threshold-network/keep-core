package libp2p

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"

	keepNet "github.com/keep-network/keep-core/pkg/net"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/libp2p/go-libp2p/p2p/net/upgrader"
)

// Keep Network protocol identifiers
const (
	// securityProtocolID is the ID of the secured transport protocol.
	securityProtocolID = "/keep/handshake/1.0.0"
	// authProtocolID is the ID of the authentication protocol.
	authProtocolID = "keep"
)

// Compile time assertions of custom types
var _ sec.SecureTransport = (*transport)(nil)
var _ sec.SecureConn = (*authenticatedConnection)(nil)

// MetricsRecorder is an interface for recording network metrics.
type MetricsRecorder interface {
	IncrementCounter(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}

// transport constructs an encrypted and authenticated connection for a peer.
type transport struct {
	protocolID     protocol.ID
	authProtocolID string

	localPeerID peer.ID
	privateKey  libp2pcrypto.PrivKey

	encryptionLayer sec.SecureTransport

	firewall keepNet.Firewall

	// metricsRecorderRef is a pointer to an atomic.Value that holds the metrics recorder.
	// This allows late binding of the metrics recorder after the transport is created.
	metricsRecorderRef *atomic.Value
}

func newEncryptedAuthenticatedTransport(
	protocolID protocol.ID,
	authProtocolID string,
	privateKey libp2pcrypto.PrivKey,
	muxers []upgrader.StreamMuxer,
	firewall keepNet.Firewall,
	metricsRecorderRef *atomic.Value,
) (*transport, error) {
	id, err := peer.IDFromPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	encryptionLayer, err := libp2ptls.New(protocolID, privateKey, muxers)
	if err != nil {
		return nil, err
	}

	return &transport{
		protocolID:         protocolID,
		authProtocolID:     authProtocolID,
		localPeerID:        id,
		privateKey:         privateKey,
		encryptionLayer:    encryptionLayer,
		firewall:           firewall,
		metricsRecorderRef: metricsRecorderRef,
	}, nil
}

// getMetricsRecorder returns the current metrics recorder from the atomic reference,
// or nil if none is set.
func (t *transport) getMetricsRecorder() MetricsRecorder {
	if t.metricsRecorderRef == nil {
		return nil
	}
	if val := t.metricsRecorderRef.Load(); val != nil {
		if recorder, ok := val.(MetricsRecorder); ok {
			return recorder
		}
	}
	return nil
}

// SecureInbound secures an inbound connection.
func (t *transport) SecureInbound(
	ctx context.Context,
	connection net.Conn,
	remotePeerID peer.ID,
) (sec.SecureConn, error) {
	encryptedConnection, err := t.encryptionLayer.SecureInbound(ctx, connection, remotePeerID)
	if err != nil {
		return nil, err
	}

	return newAuthenticatedInboundConnection(
		encryptedConnection,
		encryptedConnection.ConnState(),
		t.localPeerID,
		t.privateKey,
		t.firewall,
		t.authProtocolID,
		t.getMetricsRecorder(),
	)
}

// SecureOutbound secures an outbound connection.
func (t *transport) SecureOutbound(
	ctx context.Context,
	connection net.Conn,
	remotePeerID peer.ID,
) (sec.SecureConn, error) {
	encryptedConnection, err := t.encryptionLayer.SecureOutbound(
		ctx,
		connection,
		remotePeerID,
	)
	if err != nil {
		return nil, err
	}

	return newAuthenticatedOutboundConnection(
		encryptedConnection,
		encryptedConnection.ConnState(),
		t.localPeerID,
		t.privateKey,
		remotePeerID,
		t.firewall,
		t.authProtocolID,
		t.getMetricsRecorder(),
	)
}

// ID is the protocol ID of the security protocol.
func (t *transport) ID() protocol.ID {
	return t.protocolID
}
