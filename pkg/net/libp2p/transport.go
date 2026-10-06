package libp2p

import (
	"context"
	"fmt"
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

// keepHandshakeTimeout bounds the post-TLS Keep authentication handshake
// (Act 1 + Act 2 + Act 3). Chosen to match go-libp2p's upgrader default
// accept timeout (15s, defined as `defaultAcceptTimeout` in
// github.com/libp2p/go-libp2p/p2p/net/upgrader, configurable via
// upgrader.WithAcceptTimeout), comfortably above realistic single-RTT
// budgets across global peers while preventing indefinite stalls that
// would otherwise saturate the resource-manager transient-inbound slot
// pool. Declared as `var` (not `const`) solely to allow test code to
// shorten it under `t.Cleanup`.
var keepHandshakeTimeout = 15 * time.Second

// Compile time assertions of custom types
var _ sec.SecureTransport = (*transport)(nil)
var _ sec.SecureConn = (*authenticatedConnection)(nil)

// MetricsRecorder is an interface for recording network metrics.
type MetricsRecorder interface {
	IncrementCounter(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}

// fullMetricsRecorder is a MetricsRecorder that also supports gauge metrics.
// It is the contract for components that record counters, durations, and
// gauges (e.g. message queue sizes), unlike the transport which records only
// counters and durations.
type fullMetricsRecorder interface {
	MetricsRecorder
	SetGauge(name string, value float64)
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

// applyHandshakeDeadline sets an absolute read+write deadline on conn that is
// the minimum of (now+keepHandshakeTimeout) and any deadline already attached
// to ctx. It returns a cleanup function that restores the connection to a
// no-deadline state — callers MUST invoke the cleanup on success so that
// subsequent application I/O is not subject to the handshake budget.
//
// The deadline is absolute (per net.Conn semantics): once set, it does not
// extend with successful intermediate reads, which is exactly the property we
// want — a partial-byte trickle attack cannot keep the connection alive past
// the cap.
//
// Returns an error only if SetDeadline fails on the underlying connection,
// which would itself indicate a closed or non-functional connection. Callers
// should propagate this error.
func applyHandshakeDeadline(ctx context.Context, conn net.Conn) (clear func(), err error) {
	deadline := time.Now().Add(keepHandshakeTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set handshake deadline: %w", err)
	}
	return func() {
		// Best-effort clear; the only failure mode is a closed connection,
		// in which case the deadline is irrelevant anyway.
		_ = conn.SetDeadline(time.Time{})
	}, nil
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

	clear, err := applyHandshakeDeadline(ctx, encryptedConnection)
	if err != nil {
		_ = encryptedConnection.Close()
		return nil, fmt.Errorf("inbound handshake setup: %w", err)
	}
	defer clear()

	ac, err := newAuthenticatedInboundConnection(
		encryptedConnection,
		encryptedConnection.ConnState(),
		t.localPeerID,
		t.privateKey,
		t.firewall,
		t.authProtocolID,
		t.getMetricsRecorder(),
	)
	if err != nil {
		// newAuthenticatedInboundConnection already closes the conn on its
		// internal failure paths; the deferred clear() above is a no-op on
		// a closed conn (SetDeadline returns an error which we swallow).
		return nil, err
	}
	return ac, nil
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

	clear, err := applyHandshakeDeadline(ctx, encryptedConnection)
	if err != nil {
		_ = encryptedConnection.Close()
		return nil, fmt.Errorf("outbound handshake setup: %w", err)
	}
	defer clear()

	ac, err := newAuthenticatedOutboundConnection(
		encryptedConnection,
		encryptedConnection.ConnState(),
		t.localPeerID,
		t.privateKey,
		remotePeerID,
		t.firewall,
		t.authProtocolID,
		t.getMetricsRecorder(),
	)
	if err != nil {
		return nil, err
	}
	return ac, nil
}

// ID is the protocol ID of the security protocol.
func (t *transport) ID() protocol.ID {
	return t.protocolID
}
