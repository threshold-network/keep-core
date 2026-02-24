package libp2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

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
