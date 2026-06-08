package libp2p

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/sec"

	keepNet "github.com/keep-network/keep-core/pkg/net"
)

// mockSecureConn wraps a net.Conn (e.g. one end of net.Pipe()) and satisfies
// libp2p's sec.SecureConn interface by embedding net.Conn for the byte-stream
// half and providing the four ConnSecurity methods explicitly. It is used by
// mockEncryptionLayer to bypass libp2p-TLS while still letting transport.go
// drive the Keep authentication handshake over a synthetic in-memory pipe.
type mockSecureConn struct {
	net.Conn
	localPeerID  peer.ID
	remotePeerID peer.ID
	remotePubKey libp2pcrypto.PubKey
}

func (m *mockSecureConn) LocalPeer() peer.ID                   { return m.localPeerID }
func (m *mockSecureConn) RemotePeer() peer.ID                  { return m.remotePeerID }
func (m *mockSecureConn) RemotePublicKey() libp2pcrypto.PubKey { return m.remotePubKey }
func (m *mockSecureConn) ConnState() libp2pnetwork.ConnectionState {
	return libp2pnetwork.ConnectionState{}
}

// Compile-time assertion that mockSecureConn satisfies sec.SecureConn.
var _ sec.SecureConn = (*mockSecureConn)(nil)

// mockEncryptionLayer satisfies sec.SecureTransport by returning the supplied
// net.Conn (already presumed connected) wrapped as a mockSecureConn. It does
// no TLS — the Keep authentication handshake test fixtures rely on this so
// they can stall, partial-byte, or run the handshake to completion without
// needing libp2p-TLS to complete over net.Pipe().
type mockEncryptionLayer struct {
	localPeerID  peer.ID
	remotePeerID peer.ID
	remotePubKey libp2pcrypto.PubKey
}

func (m *mockEncryptionLayer) SecureInbound(
	_ context.Context,
	insecure net.Conn,
	_ peer.ID,
) (sec.SecureConn, error) {
	return &mockSecureConn{
		Conn:         insecure,
		localPeerID:  m.localPeerID,
		remotePeerID: m.remotePeerID,
		remotePubKey: m.remotePubKey,
	}, nil
}

func (m *mockEncryptionLayer) SecureOutbound(
	_ context.Context,
	insecure net.Conn,
	_ peer.ID,
) (sec.SecureConn, error) {
	return &mockSecureConn{
		Conn:         insecure,
		localPeerID:  m.localPeerID,
		remotePeerID: m.remotePeerID,
		remotePubKey: m.remotePubKey,
	}, nil
}

func (m *mockEncryptionLayer) ID() protocol.ID { return securityProtocolID }

// Compile-time assertion that mockEncryptionLayer satisfies sec.SecureTransport.
var _ sec.SecureTransport = (*mockEncryptionLayer)(nil)

// newTestTransport constructs a *transport literal with a mockEncryptionLayer
// injected via the package-private encryptionLayer field. The metricsRecorderRef
// is intentionally nil — getMetricsRecorder() handles nil safely.
func newTestTransport(
	cfg *testConnectionConfig,
	remotePeerID peer.ID,
	remotePubKey libp2pcrypto.PubKey,
	firewall keepNet.Firewall,
) *transport {
	return &transport{
		protocolID:     securityProtocolID,
		authProtocolID: authProtocolID,
		localPeerID:    cfg.peerID,
		privateKey:     cfg.networkPrivateKey,
		encryptionLayer: &mockEncryptionLayer{
			localPeerID:  cfg.peerID,
			remotePeerID: remotePeerID,
			remotePubKey: remotePubKey,
		},
		firewall:           firewall,
		metricsRecorderRef: nil,
	}
}

// withShortenedHandshakeTimeout overrides keepHandshakeTimeout to d for the
// duration of the test, restoring the original via t.Cleanup. MUST be used
// in every test that asserts deadline-driven behavior. Do NOT call
// t.Parallel() in any test that uses this — all tests mutate the same
// package-level var.
func withShortenedHandshakeTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := keepHandshakeTimeout
	keepHandshakeTimeout = d
	t.Cleanup(func() { keepHandshakeTimeout = original })
}

// assertHandshakeTimedOut fails the test if err is nil or elapsed reached the
// ceiling. label identifies the scenario in failure messages (e.g. "stalled
// inbound handshake"). Callers that also need a lower-bound sanity check on
// elapsed (to prove the deadline actually fired rather than the handshake
// returning instantly via some unrelated error path) keep that check inline
// so the floor value remains visible at the call site.
func assertHandshakeTimedOut(
	t *testing.T,
	elapsed time.Duration,
	ceiling time.Duration,
	label string,
	err error,
) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected non-nil error from %s", label)
	}
	if elapsed >= ceiling {
		t.Fatalf(
			"expected %s to fail within %v; got elapsed=%v",
			label, ceiling, elapsed,
		)
	}
}

// TestSecureInbound_StallsTimeOut verifies that when an attacker completes
// the (mocked) encryption layer but then sends zero bytes for the Keep
// handshake Act 1, SecureInbound returns within the shortened deadline.
func TestSecureInbound_StallsTimeOut(t *testing.T) {
	withShortenedHandshakeTimeout(t, 250*time.Millisecond)

	responder := createTestConnectionConfig(t)
	initiator := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	attackerConn, victimConn := newConnPair()
	defer attackerConn.Close()

	tr := newTestTransport(
		responder,
		initiator.peerID,
		initiator.networkPrivateKey.GetPublic(),
		firewall,
	)

	// No peer activity: attackerConn is held open by the test (via defer
	// Close) and writes nothing. This mimics the "completes TLS then silent"
	// attack profile. The victim's handshake read must hit the deadline.

	start := time.Now()
	_, err := tr.SecureInbound(context.Background(), victimConn, "")
	elapsed := time.Since(start)

	assertHandshakeTimedOut(t, elapsed, 500*time.Millisecond, "stalled inbound handshake", err)
	if elapsed < 200*time.Millisecond {
		t.Fatalf(
			"handshake returned too quickly (%v) — deadline likely did not fire; expected ≥200ms",
			elapsed,
		)
	}
}

// TestSecureInbound_PartialBytesTimeOut verifies that when an attacker sends
// a single byte of an unterminated varint then sleeps, the absolute deadline
// still fires (partial progress does not extend per-read timers).
func TestSecureInbound_PartialBytesTimeOut(t *testing.T) {
	withShortenedHandshakeTimeout(t, 250*time.Millisecond)

	responder := createTestConnectionConfig(t)
	initiator := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	attackerConn, victimConn := newConnPair()
	defer attackerConn.Close()

	tr := newTestTransport(
		responder,
		initiator.peerID,
		initiator.networkPrivateKey.GetPublic(),
		firewall,
	)

	// Attacker goroutine: writes 1 byte of a varint with the continuation
	// bit set (0x80) — protodelim will keep blocking waiting for the rest
	// of the length-prefix that never arrives.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = attackerConn.Write([]byte{0x80})
		// Sleep well past the handshake deadline so we test the deadline,
		// not the attacker's eventual close.
		time.Sleep(2 * time.Second)
	}()

	start := time.Now()
	_, err := tr.SecureInbound(context.Background(), victimConn, "")
	elapsed := time.Since(start)

	assertHandshakeTimedOut(t, elapsed, 500*time.Millisecond, "partial-byte inbound handshake", err)
	if elapsed < 200*time.Millisecond {
		t.Fatalf(
			"handshake returned too quickly (%v) — deadline likely did not fire; expected ≥200ms",
			elapsed,
		)
	}

	// Allow the attacker goroutine to terminate before test exit.
	attackerConn.Close()
	<-done
}

// TestSecureInbound_SuccessClearsDeadline verifies that a real successful
// Keep handshake completes and that, after SecureInbound returns, post-handshake
// reads on the returned connection are no longer subject to the handshake
// deadline.
func TestSecureInbound_SuccessClearsDeadline(t *testing.T) {
	withShortenedHandshakeTimeout(t, 250*time.Millisecond)

	responder := createTestConnectionConfig(t)
	initiator := createTestConnectionConfig(t)

	firewall := newMockFirewall()
	if err := firewall.updatePeer(initiator.networkPublicKey, true); err != nil {
		t.Fatal(err)
	}
	if err := firewall.updatePeer(responder.networkPublicKey, true); err != nil {
		t.Fatal(err)
	}

	initiatorConn, responderConn := newConnPair()

	tr := newTestTransport(
		responder,
		initiator.peerID,
		initiator.networkPrivateKey.GetPublic(),
		firewall,
	)

	// Honest initiator runs the 3-act handshake directly via
	// newAuthenticatedOutboundConnection (same-package, no need to wrap
	// in transport for the peer side). After the handshake completes the
	// initiator writes a probe payload, sleeps past the handshake deadline,
	// then closes its end so the responder's later read returns io.EOF.
	probe := []byte("clear-deadline-probe")
	peerDone := make(chan error, 1)
	go func() {
		outConn, err := newAuthenticatedOutboundConnection(
			initiatorConn,
			libp2pnetwork.ConnectionState{},
			initiator.peerID,
			initiator.networkPrivateKey,
			responder.peerID,
			firewall,
			authProtocolID,
			nil,
		)
		if err != nil {
			peerDone <- err
			return
		}
		if _, err := outConn.Write(probe); err != nil {
			peerDone <- err
			return
		}
		// Hold the conn open past the original deadline so the responder
		// can verify that the deadline has been cleared. Then close it.
		time.Sleep(2 * keepHandshakeTimeout)
		_ = initiatorConn.Close()
		peerDone <- nil
	}()

	secureConn, err := tr.SecureInbound(context.Background(), responderConn, "")
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}
	if secureConn == nil {
		t.Fatal("expected non-nil secure connection on successful handshake")
	}

	// Sleep past the original handshake deadline before reading. If the
	// deadline had not been cleared, any subsequent Read would surface
	// os.ErrDeadlineExceeded (or a net.Error timeout) at this point.
	time.Sleep(2 * keepHandshakeTimeout)

	buf := make([]byte, len(probe))
	_, readErr := io.ReadFull(secureConn, buf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		// Tolerate EOF variants (peer closed) but never a timeout.
		if errors.Is(readErr, os.ErrDeadlineExceeded) {
			t.Fatalf("post-handshake read hit deadline: %v", readErr)
		}
		var netErr net.Error
		if errors.As(readErr, &netErr) && netErr.Timeout() {
			t.Fatalf("post-handshake read hit net.Error timeout: %v", readErr)
		}
	}

	if err := <-peerDone; err != nil {
		t.Fatalf("peer goroutine error: %v", err)
	}
}

// TestSecureInbound_HonorsCtxDeadline verifies that the helper takes the
// tighter of (now + keepHandshakeTimeout, ctx.Deadline()). With the package
// var at its default 15s and a 100ms ctx deadline, the timeout must fire
// well under the package default.
func TestSecureInbound_HonorsCtxDeadline(t *testing.T) {
	// Intentionally do NOT override keepHandshakeTimeout; the test must
	// observe the ctx-side bound dominating.
	responder := createTestConnectionConfig(t)
	initiator := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	attackerConn, victimConn := newConnPair()
	defer attackerConn.Close()

	tr := newTestTransport(
		responder,
		initiator.peerID,
		initiator.networkPrivateKey.GetPublic(),
		firewall,
	)

	// No peer activity: attackerConn is held open by the test (via defer
	// Close) and writes nothing. The ctx-side deadline must dominate over
	// the default 15s package-level timeout.

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := tr.SecureInbound(ctx, victimConn, "")
	elapsed := time.Since(start)

	assertHandshakeTimedOut(t, elapsed, 300*time.Millisecond, "ctx-bounded inbound handshake", err)
}

// TestSecureOutbound_StallsTimeOut verifies that when the responder side
// stalls after the (mocked) encryption layer completes, SecureOutbound
// returns within the shortened deadline.
func TestSecureOutbound_StallsTimeOut(t *testing.T) {
	withShortenedHandshakeTimeout(t, 250*time.Millisecond)

	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	initiatorConn, attackerConn := newConnPair()
	defer attackerConn.Close()

	tr := newTestTransport(
		initiator,
		responder.peerID,
		responder.networkPrivateKey.GetPublic(),
		firewall,
	)

	// Attacker goroutine: drains anything the initiator writes (so the
	// synchronous net.Pipe Act 1 send does not block) but never replies.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, attackerConn)
	}()

	start := time.Now()
	_, err := tr.SecureOutbound(context.Background(), initiatorConn, responder.peerID)
	elapsed := time.Since(start)

	assertHandshakeTimedOut(t, elapsed, 500*time.Millisecond, "stalled outbound handshake", err)
	if elapsed < 200*time.Millisecond {
		t.Fatalf(
			"handshake returned too quickly (%v) — deadline likely did not fire; expected ≥200ms",
			elapsed,
		)
	}

	// Closing attackerConn unblocks the io.Copy goroutine.
	attackerConn.Close()
	<-done
}

// TestSecureInbound_FastHandshakeUnaffected verifies that a real handshake
// which completes well under the default 15s timeout does not spuriously
// fail with a timeout error.
func TestSecureInbound_FastHandshakeUnaffected(t *testing.T) {
	// Intentionally do NOT override keepHandshakeTimeout; we want to prove
	// the default 15s does not cause spurious timeouts on a fast handshake.
	responder := createTestConnectionConfig(t)
	initiator := createTestConnectionConfig(t)

	firewall := newMockFirewall()
	if err := firewall.updatePeer(initiator.networkPublicKey, true); err != nil {
		t.Fatal(err)
	}
	if err := firewall.updatePeer(responder.networkPublicKey, true); err != nil {
		t.Fatal(err)
	}

	initiatorConn, responderConn := newConnPair()

	tr := newTestTransport(
		responder,
		initiator.peerID,
		initiator.networkPrivateKey.GetPublic(),
		firewall,
	)

	peerDone := make(chan error, 1)
	go func() {
		_, err := newAuthenticatedOutboundConnection(
			initiatorConn,
			libp2pnetwork.ConnectionState{},
			initiator.peerID,
			initiator.networkPrivateKey,
			responder.peerID,
			firewall,
			authProtocolID,
			nil,
		)
		peerDone <- err
	}()

	start := time.Now()
	secureConn, err := tr.SecureInbound(context.Background(), responderConn, "")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}
	if secureConn == nil {
		t.Fatal("expected non-nil secure connection on successful handshake")
	}
	if elapsed >= 1*time.Second {
		t.Fatalf(
			"fast handshake took too long (%v); expected well under 1s",
			elapsed,
		)
	}

	if peerErr := <-peerDone; peerErr != nil {
		t.Fatalf("peer goroutine error: %v", peerErr)
	}
}

// TestApplyHandshakeDeadline_FailsOnClosedConn verifies that calling the
// helper directly on a closed net.Pipe connection returns an error chain
// that satisfies errors.Is(err, io.ErrClosedPipe). The helper wraps its
// underlying SetDeadline failure with %w so the chain is preserved.
func TestApplyHandshakeDeadline_FailsOnClosedConn(t *testing.T) {
	a, b := net.Pipe()
	_ = a.Close()
	_ = b.Close()

	clear, err := applyHandshakeDeadline(context.Background(), a)
	if clear != nil {
		t.Fatal("expected nil clear closure on SetDeadline failure")
	}
	if err == nil {
		t.Fatal("expected non-nil error from applyHandshakeDeadline on closed conn")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected error chain to contain io.ErrClosedPipe; got %v", err)
	}
}
