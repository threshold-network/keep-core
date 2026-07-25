package libp2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"golang.org/x/time/rate"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/firewall"
	keepNet "github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/gen/pb"
	"github.com/keep-network/keep-core/pkg/net/security/handshake"
	"github.com/keep-network/keep-core/pkg/operator"
)

func TestPinnedAndMessageKeyMismatch(t *testing.T) {
	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	err := firewall.updatePeer(initiator.networkPublicKey, true)
	if err != nil {
		t.Fatal(err)
	}

	err = firewall.updatePeer(responder.networkPublicKey, true)
	if err != nil {
		t.Fatal(err)
	}

	initiatorConn, responderConn := newConnPair()

	go func(
		initiatorConn net.Conn,
		initiatorPeerID peer.ID,
		initiatorStaticKey libp2pcrypto.PrivKey,
		responderPeerID peer.ID,
		responderStaticKey libp2pcrypto.PrivKey,
	) {
		ac := &authenticatedConnection{
			Conn:                initiatorConn,
			localPeerID:         initiatorPeerID,
			localPeerPrivateKey: initiatorStaticKey,
			remotePeerID:        responderPeerID,
			remotePeerPublicKey: responderStaticKey.GetPublic(),
		}

		ac.initializePipe()

		maliciousInitiatorHijacksHonestRun(t, ac)
	}(initiatorConn, initiator.peerID, initiator.networkPrivateKey, responder.peerID, responder.networkPrivateKey)

	_, err = newAuthenticatedInboundConnection(
		responderConn,
		libp2pnetwork.ConnectionState{},
		responder.peerID,
		responder.networkPrivateKey,
		firewall,
		authProtocolID,
		nil, // metricsRecorder
	)
	if err == nil {
		t.Fatal("should not have successfully completed handshake")
	}
}

// maliciousInitiatorHijacksHonestRun simulates an honest Acts 1 and 2 as an
// initiator, and then drops in a malicious peer for Act 3. Properly implemented
// peer-pinning should ensure that a malicious peer can't hijack a connection
// after the first act and sign subsequent messages.
func maliciousInitiatorHijacksHonestRun(t *testing.T, ac *authenticatedConnection) {
	initiatorAct1, err := handshake.InitiateHandshake(authProtocolID)
	if err != nil {
		t.Fatal(err)
	}

	act1WireMessage, err := initiatorAct1.Message().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := ac.initiatorSendAct1(act1WireMessage); err != nil {
		t.Fatal(err)
	}

	initiatorAct2 := initiatorAct1.Next()

	act2Message, err := ac.initiatorReceiveAct2()
	if err != nil {
		t.Fatal(err)
	}

	initiatorAct3, err := initiatorAct2.Next(act2Message)
	if err != nil {
		t.Fatal(err)
	}

	act3WireMessage, err := initiatorAct3.Message().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	maliciousInitiator := createTestConnectionConfig(t)
	signedAct3Message, err := maliciousInitiator.networkPrivateKey.Sign(act3WireMessage)
	if err != nil {
		t.Fatal(err)
	}

	act3Envelope := &pb.HandshakeEnvelope{
		Message:   act3WireMessage,
		PeerID:    []byte(maliciousInitiator.peerID),
		Signature: signedAct3Message,
	}

	if err := ac.pipe.send(act3Envelope); err != nil {
		t.Fatal(err)
	}
}

func TestHandshake(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	err := firewall.updatePeer(initiator.networkPublicKey, true)
	if err != nil {
		t.Fatal(err)
	}

	err = firewall.updatePeer(responder.networkPublicKey, true)
	if err != nil {
		t.Fatal(err)
	}

	authnInboundConn, authnOutboundConn, inboundError, outboundError :=
		connectInitiatorAndResponder(initiator, responder, firewall, t)
	if inboundError != nil {
		t.Fatal(inboundError)
	}
	if outboundError != nil {
		t.Fatal(outboundError)
	}

	// send a test message over the established connection
	msg := []byte("brown fox blue tail")
	go func(authnOutboundConn *authenticatedConnection, msg []byte) {
		if _, err := authnOutboundConn.Write(msg); err != nil {
			t.Error(err)
		}
	}(authnOutboundConn, msg)

	msgContainer := make([]byte, len(msg))
	if _, err := io.ReadFull(authnInboundConn.Conn, msgContainer); err != nil {
		t.Error(err)
	}

	if string(msgContainer) != string(msg) {
		t.Errorf("message mismatch got %v, want %v", string(msgContainer), string(msg))
	}
}

func TestHandshakeInitiatorBlockedByFirewallRules(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	firewall := newMockFirewall()
	// only responder meets firewall rules
	firewall.updatePeer(responder.networkPublicKey, true)

	_, _, inboundError, outboundError :=
		connectInitiatorAndResponder(initiator, responder, firewall, t)

	if inboundError != nil {
		t.Fatal(inboundError)
	}

	expectedOutboundError := fmt.Errorf("connection handshake failed: [remote peer does not meet firewall criteria]")
	if !reflect.DeepEqual(expectedOutboundError, outboundError) {
		t.Fatalf(
			"unexpected outbound connection error\nexpected: %v\nactual: %v",
			expectedOutboundError,
			outboundError,
		)
	}
}

func TestHandshakeResponderBlockedByFirewallRules(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	firewall := newMockFirewall()

	// only initiator meets firewall rules
	err := firewall.updatePeer(initiator.networkPublicKey, true)
	if err != nil {
		t.Fatal(err)
	}

	_, _, inboundError, outboundError :=
		connectInitiatorAndResponder(initiator, responder, firewall, t)

	expectedInboundError := fmt.Errorf("connection handshake failed: [remote peer does not meet firewall criteria]")
	if !reflect.DeepEqual(expectedInboundError, inboundError) {
		t.Fatalf(
			"unexpected outbound connection error\nexpected: %v\nactual: %v",
			expectedInboundError,
			inboundError,
		)
	}

	if outboundError != nil {
		t.Fatal(outboundError)
	}
}

func connectInitiatorAndResponder(
	initiator *testConnectionConfig,
	responder *testConnectionConfig,
	firewall keepNet.Firewall,
	t *testing.T,
) (
	authnInboundConn *authenticatedConnection,
	authnOutboundConn *authenticatedConnection,
	outboundError error,
	inboundError error,
) {

	initiatorConn, responderConn := newConnPair()

	done := make(chan struct{})

	go func(
		initiatorConn net.Conn,
		initiatorPeerID peer.ID,
		initiatorPrivKey libp2pcrypto.PrivKey,
		responderPeerID peer.ID,
	) {
		authnOutboundConn, outboundError = newAuthenticatedOutboundConnection(
			initiatorConn,
			libp2pnetwork.ConnectionState{},
			initiatorPeerID,
			initiatorPrivKey,
			responderPeerID,
			firewall,
			authProtocolID,
			nil, // metricsRecorder
		)
		done <- struct{}{}
	}(initiatorConn, initiator.peerID, initiator.networkPrivateKey, responder.peerID)

	authnInboundConn, inboundError = newAuthenticatedInboundConnection(
		responderConn,
		libp2pnetwork.ConnectionState{},
		responder.peerID,
		responder.networkPrivateKey,
		firewall,
		authProtocolID,
		nil, // metricsRecorder
	)

	<-done // handshake is done

	return
}

type testConnectionConfig struct {
	networkPrivateKey *libp2pcrypto.Secp256k1PrivateKey
	networkPublicKey  *libp2pcrypto.Secp256k1PublicKey
	peerID            peer.ID
}

func createTestConnectionConfig(t *testing.T) *testConnectionConfig {
	operatorPrivateKey, _, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	networkPrivateKey, networkPublicKey, err := operatorPrivateKeyToNetworkKeyPair(operatorPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	peerID, err := peer.IDFromPrivateKey(networkPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	return &testConnectionConfig{
		networkPrivateKey,
		networkPublicKey,
		peerID,
	}
}

// Connect an initiator and responder via a full duplex network connection (reads
// on one end should be matched with writes on the other).
func newConnPair() (net.Conn, net.Conn) {
	return net.Pipe()
}

// --- join failure reason classification tests ---

func TestClassifyHandshakeFailure(t *testing.T) {
	tests := map[string]struct {
		err            error
		expectedReason string
	}{
		"deadline exceeded": {
			err:            fmt.Errorf("read failed: %w", os.ErrDeadlineExceeded),
			expectedReason: clientinfo.JoinFailureReasonTimeout,
		},
		"net op error with deadline exceeded": {
			err:            &net.OpError{Op: "read", Err: os.ErrDeadlineExceeded},
			expectedReason: clientinfo.JoinFailureReasonTimeout,
		},
		"eof": {
			err:            io.EOF,
			expectedReason: clientinfo.JoinFailureReasonEOFReset,
		},
		"unexpected eof wrapped": {
			err:            fmt.Errorf("receive failed: %w", io.ErrUnexpectedEOF),
			expectedReason: clientinfo.JoinFailureReasonEOFReset,
		},
		"closed pipe": {
			err:            fmt.Errorf("send failed: %w", io.ErrClosedPipe),
			expectedReason: clientinfo.JoinFailureReasonEOFReset,
		},
		"closed network connection": {
			err:            fmt.Errorf("read failed: %w", net.ErrClosed),
			expectedReason: clientinfo.JoinFailureReasonEOFReset,
		},
		"connection reset by peer": {
			err:            &net.OpError{Op: "read", Err: syscall.ECONNRESET},
			expectedReason: clientinfo.JoinFailureReasonEOFReset,
		},
		"broken pipe": {
			err:            &net.OpError{Op: "write", Err: syscall.EPIPE},
			expectedReason: clientinfo.JoinFailureReasonEOFReset,
		},
		"signature verification failure": {
			err:            fmt.Errorf("invalid signature [0xff] on message from sender"),
			expectedReason: clientinfo.JoinFailureReasonProtocolCrypto,
		},
		"pinned identity mismatch": {
			err:            fmt.Errorf("pinned identity does not match sender identity"),
			expectedReason: clientinfo.JoinFailureReasonProtocolCrypto,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualReason := classifyHandshakeFailure(test.err)
			if actualReason != test.expectedReason {
				t.Errorf(
					"unexpected failure reason\nexpected: %s\nactual: %s",
					test.expectedReason,
					actualReason,
				)
			}
		})
	}
}

// --- join request metrics and failure log tests ---

type fakeMetricsRecorder struct {
	mutex    sync.Mutex
	counters map[string]float64
}

func newFakeMetricsRecorder() *fakeMetricsRecorder {
	return &fakeMetricsRecorder{
		counters: make(map[string]float64),
	}
}

func (f *fakeMetricsRecorder) IncrementCounter(name string, value float64) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.counters[name] += value
}

func (f *fakeMetricsRecorder) RecordDuration(name string, duration time.Duration) {}

func (f *fakeMetricsRecorder) counterValue(name string) float64 {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.counters[name]
}

func (f *fakeMetricsRecorder) assertCounters(
	t *testing.T,
	expected map[string]float64,
) {
	t.Helper()
	for name, expectedValue := range expected {
		if actualValue := f.counterValue(name); actualValue != expectedValue {
			t.Errorf(
				"unexpected value of counter [%s]\nexpected: %v\nactual: %v",
				name,
				expectedValue,
				actualValue,
			)
		}
	}
}

// notRecognizedFirewall rejects every peer with the firewall's sentinel
// non-recognition error.
type notRecognizedFirewall struct{}

func (f *notRecognizedFirewall) Validate(
	remotePeerOperatorPublicKey *operator.PublicKey,
) error {
	return firewall.ErrNotRecognized
}

// erroringFirewall fails every validation with an application/RPC error.
type erroringFirewall struct{}

func (f *erroringFirewall) Validate(
	remotePeerOperatorPublicKey *operator.PublicKey,
) error {
	return fmt.Errorf(
		"could not validate if remote peer is recognized by application: " +
			"[rpc timeout]",
	)
}

func connectInitiatorAndResponderWithRecorder(
	initiator *testConnectionConfig,
	responder *testConnectionConfig,
	firewall keepNet.Firewall,
	recorder MetricsRecorder,
) (inboundError error, outboundError error) {
	initiatorConn, responderConn := newConnPair()

	done := make(chan struct{})

	go func() {
		_, outboundError = newAuthenticatedOutboundConnection(
			initiatorConn,
			libp2pnetwork.ConnectionState{},
			initiator.peerID,
			initiator.networkPrivateKey,
			responder.peerID,
			firewall,
			authProtocolID,
			recorder,
		)
		done <- struct{}{}
	}()

	_, inboundError = newAuthenticatedInboundConnection(
		responderConn,
		libp2pnetwork.ConnectionState{},
		responder.peerID,
		responder.networkPrivateKey,
		firewall,
		authProtocolID,
		recorder,
	)

	<-done // handshake is done

	return
}

func TestInboundConnectionFirewallUnrecognizedReason(t *testing.T) {
	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	recorder := newFakeMetricsRecorder()

	// Reset the shared package-level limiter so the throttled failure log
	// lines below are emitted deterministically, independent of other tests
	// that share the limiter.
	connectionFailureLogLimiter = newConnectionFailureLogLimiter()

	var inboundError error
	entries := captureLogs(t, "keep-libp2p", func() {
		inboundError, _ = connectInitiatorAndResponderWithRecorder(
			initiator,
			responder,
			&notRecognizedFirewall{},
			recorder,
		)
	})

	if inboundError == nil {
		t.Fatal("expected inbound connection to be rejected by the firewall")
	}

	recorder.assertCounters(t, map[string]float64{
		clientinfo.MetricNetworkJoinRequestsTotal:        1,
		clientinfo.MetricNetworkJoinRequestsFailedTotal:  1,
		clientinfo.MetricNetworkJoinRequestsSuccessTotal: 0,
		clientinfo.MetricFirewallRejectionsTotal:         1,
		clientinfo.NetworkJoinFailureMetricName(
			clientinfo.JoinFailureReasonFirewallUnrecognized,
		): 1,
		clientinfo.NetworkJoinFailureMetricName(
			clientinfo.JoinFailureReasonFirewallRPCError,
		): 0,
	})

	inboundLogs := countLogEntries(
		entries,
		"info",
		"inbound connection rejected by firewall with reason "+
			"[firewall_unrecognized]",
	)
	if inboundLogs != 1 {
		t.Errorf(
			"expected 1 inbound firewall rejection log entry, got %d",
			inboundLogs,
		)
	}

	outboundLogs := countLogEntries(
		entries,
		"info",
		"outbound connection rejected by firewall with reason "+
			"[firewall_unrecognized]",
	)
	if outboundLogs != 1 {
		t.Errorf(
			"expected 1 outbound firewall rejection log entry, got %d",
			outboundLogs,
		)
	}
}

func TestInboundConnectionFirewallRPCErrorReason(t *testing.T) {
	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	recorder := newFakeMetricsRecorder()

	inboundError, _ := connectInitiatorAndResponderWithRecorder(
		initiator,
		responder,
		&erroringFirewall{},
		recorder,
	)

	if inboundError == nil {
		t.Fatal("expected inbound connection to be rejected by the firewall")
	}

	recorder.assertCounters(t, map[string]float64{
		clientinfo.MetricNetworkJoinRequestsTotal:        1,
		clientinfo.MetricNetworkJoinRequestsFailedTotal:  1,
		clientinfo.MetricNetworkJoinRequestsSuccessTotal: 0,
		clientinfo.MetricFirewallRejectionsTotal:         1,
		clientinfo.NetworkJoinFailureMetricName(
			clientinfo.JoinFailureReasonFirewallRPCError,
		): 1,
		clientinfo.NetworkJoinFailureMetricName(
			clientinfo.JoinFailureReasonFirewallUnrecognized,
		): 0,
	})
}

func TestInboundConnectionHandshakeEOFReason(t *testing.T) {
	responder := createTestConnectionConfig(t)

	recorder := newFakeMetricsRecorder()

	// Reset the shared package-level limiter so the throttled failure log
	// line below is emitted deterministically, independent of other tests
	// that share the limiter.
	connectionFailureLogLimiter = newConnectionFailureLogLimiter()

	initiatorConn, responderConn := newConnPair()

	// The initiator drops the connection without sending anything, so the
	// responder's handshake fails with a closed-connection error.
	go func() {
		_ = initiatorConn.Close()
	}()

	var inboundError error
	entries := captureLogs(t, "keep-libp2p", func() {
		_, inboundError = newAuthenticatedInboundConnection(
			responderConn,
			libp2pnetwork.ConnectionState{},
			responder.peerID,
			responder.networkPrivateKey,
			newMockFirewall(),
			authProtocolID,
			recorder,
		)
	})

	if inboundError == nil {
		t.Fatal("expected inbound connection handshake to fail")
	}

	recorder.assertCounters(t, map[string]float64{
		clientinfo.MetricNetworkJoinRequestsTotal:        1,
		clientinfo.MetricNetworkJoinRequestsFailedTotal:  1,
		clientinfo.MetricNetworkJoinRequestsSuccessTotal: 0,
		clientinfo.MetricFirewallRejectionsTotal:         0,
		clientinfo.NetworkJoinFailureMetricName(
			clientinfo.JoinFailureReasonEOFReset,
		): 1,
	})

	handshakeLogs := countLogEntries(
		entries,
		"info",
		"inbound connection handshake failed with reason [eof_reset]",
	)
	if handshakeLogs != 1 {
		t.Errorf(
			"expected 1 inbound handshake failure log entry, got %d",
			handshakeLogs,
		)
	}
}

// TestConnectionFailureLogThrottling verifies that the INFO-level
// connection-failure log line is rate limited so a burst of failures from the
// same source cannot flood the logs, while the per-reason failure metrics are
// still incremented for every single failure.
func TestConnectionFailureLogThrottling(t *testing.T) {
	responder := createTestConnectionConfig(t)

	// Install a strict limiter that admits only a small burst of log lines and
	// does not refill within the test window, then restore the production
	// limiter afterwards.
	const allowedLogLines = 2
	originalLimiter := connectionFailureLogLimiter
	connectionFailureLogLimiter = rate.NewLimiter(rate.Every(time.Hour), allowedLogLines)
	defer func() { connectionFailureLogLimiter = originalLimiter }()

	recorder := newFakeMetricsRecorder()

	const failureCount = 10

	entries := captureLogs(t, "keep-libp2p", func() {
		for i := 0; i < failureCount; i++ {
			initiatorConn, responderConn := newConnPair()

			// The initiator drops the connection without sending anything, so
			// the responder's handshake fails with a closed-connection error.
			go func() { _ = initiatorConn.Close() }()

			_, inboundError := newAuthenticatedInboundConnection(
				responderConn,
				libp2pnetwork.ConnectionState{},
				responder.peerID,
				responder.networkPrivateKey,
				newMockFirewall(),
				authProtocolID,
				recorder,
			)
			if inboundError == nil {
				t.Fatalf(
					"expected inbound handshake failure on attempt %d", i,
				)
			}
		}
	})

	// The failure metrics are incremented once per failure regardless of
	// whether the log line was emitted - throttling the log must not hide the
	// observability signal.
	recorder.assertCounters(t, map[string]float64{
		clientinfo.MetricNetworkJoinRequestsTotal:       failureCount,
		clientinfo.MetricNetworkJoinRequestsFailedTotal: failureCount,
		clientinfo.NetworkJoinFailureMetricName(
			clientinfo.JoinFailureReasonEOFReset,
		): failureCount,
	})

	// The log line is throttled to the limiter's burst even though every one of
	// the failures above was recorded in the metrics.
	loggedLines := countLogEntries(
		entries,
		"info",
		"inbound connection handshake failed with reason [eof_reset]",
	)
	if loggedLines != allowedLogLines {
		t.Errorf(
			"expected connection-failure log throttled to %d lines, got %d",
			allowedLogLines,
			loggedLines,
		)
	}
}

func TestInboundConnectionSuccessfulJoinCounters(t *testing.T) {
	initiator := createTestConnectionConfig(t)
	responder := createTestConnectionConfig(t)

	firewall := newMockFirewall()
	if err := firewall.updatePeer(initiator.networkPublicKey, true); err != nil {
		t.Fatal(err)
	}
	if err := firewall.updatePeer(responder.networkPublicKey, true); err != nil {
		t.Fatal(err)
	}

	recorder := newFakeMetricsRecorder()

	inboundError, outboundError := connectInitiatorAndResponderWithRecorder(
		initiator,
		responder,
		firewall,
		recorder,
	)

	if inboundError != nil {
		t.Fatal(inboundError)
	}
	if outboundError != nil {
		t.Fatal(outboundError)
	}

	expected := map[string]float64{
		clientinfo.MetricNetworkJoinRequestsTotal:        1,
		clientinfo.MetricNetworkJoinRequestsSuccessTotal: 1,
		clientinfo.MetricNetworkJoinRequestsFailedTotal:  0,
		clientinfo.MetricFirewallRejectionsTotal:         0,
	}
	for _, reason := range clientinfo.GetAllNetworkJoinFailureReasons() {
		expected[clientinfo.NetworkJoinFailureMetricName(reason)] = 0
	}
	recorder.assertCounters(t, expected)
}

func newMockFirewall() *mockFirewall {
	return &mockFirewall{
		meetsCriteria: make(map[uint64]bool),
	}
}

type mockFirewall struct {
	meetsCriteria map[uint64]bool
}

func (mf *mockFirewall) Validate(remotePeerOperatorPublicKey *operator.PublicKey) error {
	if !mf.meetsCriteria[remotePeerOperatorPublicKey.X.Uint64()] {
		return fmt.Errorf("remote peer does not meet firewall criteria")
	}
	return nil
}

func (mf *mockFirewall) updatePeer(
	remotePeerNetworkPublicKey *libp2pcrypto.Secp256k1PublicKey,
	meetsCriteria bool,
) error {
	operatorPublicKey, err := networkPublicKeyToOperatorPublicKey(
		remotePeerNetworkPublicKey,
	)
	if err != nil {
		return err
	}

	mf.meetsCriteria[operatorPublicKey.X.Uint64()] = meetsCriteria

	return nil
}
