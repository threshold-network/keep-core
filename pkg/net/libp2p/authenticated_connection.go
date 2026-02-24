package libp2p

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	keepNet "github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/gen/pb"
	"github.com/keep-network/keep-core/pkg/net/security/handshake"

	"google.golang.org/protobuf/proto"
	// TODO: Stop using `dev` version of `google.golang.org/protobuf` once v.1.28.2
	// is published.
	protodelim "google.golang.org/protobuf/dev/encoding/protodelim"
)

// Enough space for a proto-encoded envelope with a message, peer.ID, and sig.
const maxFrameSize = 1024

// authenticatedConnection turns inbound and outbound unauthenticated,
// plain-text connections into authenticated, plain-text connections. Noticeably,
// it does not guarantee confidentiality as it does not encrypt connections.
type authenticatedConnection struct {
	net.Conn

	connState libp2pnetwork.ConnectionState

	localPeerID         peer.ID
	localPeerPrivateKey libp2pcrypto.PrivKey

	remotePeerID        peer.ID
	remotePeerPublicKey libp2pcrypto.PubKey

	firewall keepNet.Firewall

	protocol string

	pipe pipe
}

// pipe is used to send and receive messages over the connection between peers.
type pipe struct {
	reader protodelim.Reader
	writer io.Writer

	delimWriter *protodelim.MarshalOptions
	delimReader *protodelim.UnmarshalOptions
}

func (ac *authenticatedConnection) initializePipe() {
	// The reader has to implement `io.ByteReader` interface so we need to wrap
	// the `net.Conn` with `bufio`.
	ac.pipe.reader = bufio.NewReader(ac.Conn)
	ac.pipe.writer = ac.Conn

	ac.pipe.delimReader = &protodelim.UnmarshalOptions{MaxSize: maxFrameSize}
	ac.pipe.delimWriter = &protodelim.MarshalOptions{}
}

// Sends a message through the pipe.
func (mc *pipe) send(msg proto.Message) (err error) {
	_, err = mc.delimWriter.MarshalTo(mc.writer, msg)
	return
}

// Receives a message from the pipe.
func (mc *pipe) receive(msg proto.Message) (err error) {
	err = mc.delimReader.UnmarshalFrom(mc.reader, msg)
	return
}

// newAuthenticatedInboundConnection is the connection that's formed by
// transport.SecureInbound. This function is executed by the receiver of a new
// connection, who acts as the responder. This side of the connection lacks
// knowledge of the remotePeerID (passed in as empty string). On success running
// the responder side of the handshake, it returns a fully-authenticated
// connection, which grants access to the network.
func newAuthenticatedInboundConnection(
	unauthenticatedConn net.Conn,
	connState libp2pnetwork.ConnectionState,
	localPeerID peer.ID,
	privateKey libp2pcrypto.PrivKey,
	firewall keepNet.Firewall,
	protocol string,
	metricsRecorder MetricsRecorder,
) (*authenticatedConnection, error) {
	startTime := time.Now()

	// Track inbound join request attempt
	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter(clientinfo.MetricNetworkJoinRequestsTotal, 1)
	}

	ac := &authenticatedConnection{
		Conn:                unauthenticatedConn,
		connState:           connState,
		localPeerID:         localPeerID,
		localPeerPrivateKey: privateKey,
		firewall:            firewall,
		protocol:            protocol,
	}

	ac.initializePipe()

	if err := ac.runHandshakeAsResponder(); err != nil {
		// Track failed join request (handshake failure)
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(clientinfo.MetricNetworkJoinRequestsFailedTotal, 1)
		}

		// close the conn before returning (if it hasn't already)
		// otherwise we leak.
		if closeErr := ac.Close(); closeErr != nil {
			logger.Debugf("could not close the connection: [%v]", closeErr)
		}

		return nil, fmt.Errorf("connection handshake failed: [%v]", err)
	}

	if err := ac.checkFirewallRules(); err != nil {
		// Track firewall rejection
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(clientinfo.MetricFirewallRejectionsTotal, 1)
			metricsRecorder.IncrementCounter(clientinfo.MetricNetworkJoinRequestsFailedTotal, 1)
		}

		if closeErr := ac.Close(); closeErr != nil {
			logger.Debugf("could not close the connection: [%v]", closeErr)
		}

		return nil, fmt.Errorf("connection handshake failed: [%v]", err)
	}

	// Track successful join request
	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter(clientinfo.MetricNetworkJoinRequestsSuccessTotal, 1)
		metricsRecorder.RecordDuration(clientinfo.MetricNetworkHandshakeDurationSeconds, time.Since(startTime))
	}

	return ac, nil
}

// newAuthenticatedOutboundConnection is the connection that's formed by
// transport.SecureOutbound. This function is executed by the initiator of a new
// connection. This side of the connection has knowledge of the remote peer
// identity. On success running the initiator side of the handshake, it returns a
// fully-authenticated connection, which grants access to the network.
func newAuthenticatedOutboundConnection(
	unauthenticatedConn net.Conn,
	connState libp2pnetwork.ConnectionState,
	localPeerID peer.ID,
	privateKey libp2pcrypto.PrivKey,
	remotePeerID peer.ID,
	firewall keepNet.Firewall,
	protocol string,
	metricsRecorder MetricsRecorder,
) (*authenticatedConnection, error) {
	startTime := time.Now()

	remotePublicKey, err := remotePeerID.ExtractPublicKey()
	if err != nil {
		return nil, fmt.Errorf(
			"could not create new authenticated outbound connection: [%v]",
			err,
		)
	}

	ac := &authenticatedConnection{
		Conn:                unauthenticatedConn,
		connState:           connState,
		localPeerID:         localPeerID,
		localPeerPrivateKey: privateKey,
		remotePeerID:        remotePeerID,
		remotePeerPublicKey: remotePublicKey,
		firewall:            firewall,
		protocol:            protocol,
	}

	ac.initializePipe()

	if err := ac.runHandshakeAsInitiator(); err != nil {
		if closeErr := ac.Close(); closeErr != nil {
			logger.Debugf("could not close the connection: [%v]", closeErr)
		}

		return nil, fmt.Errorf("connection handshake failed: [%v]", err)
	}

	if err := ac.checkFirewallRules(); err != nil {
		if closeErr := ac.Close(); closeErr != nil {
			logger.Debugf("could not close the connection: [%v]", closeErr)
		}

		return nil, fmt.Errorf("connection handshake failed: [%v]", err)
	}

	// Record handshake duration for outbound connections
	if metricsRecorder != nil {
		metricsRecorder.RecordDuration(clientinfo.MetricNetworkHandshakeDurationSeconds, time.Since(startTime))
	}

	return ac, nil
}

func (ac *authenticatedConnection) checkFirewallRules() error {
	operatorPublicKey, err := networkPublicKeyToOperatorPublicKey(ac.remotePeerPublicKey)
	if err != nil {
		return fmt.Errorf(
			"cannot convert libp2p public key to operator public key: [%v]",
			err,
		)
	}

	return ac.firewall.Validate(operatorPublicKey)
}

func (ac *authenticatedConnection) runHandshakeAsInitiator() error {
	// initiator station

	//
	// Act 1
	//

	initiatorAct1, err := handshake.InitiateHandshake(ac.protocol)
	if err != nil {
		return err
	}

	act1WireMessage, err := initiatorAct1.Message().Marshal()
	if err != nil {
		return err
	}

	if err := ac.initiatorSendAct1(act1WireMessage); err != nil {
		return err
	}

	initiatorAct2 := initiatorAct1.Next()

	//
	// Act 2
	//

	act2Message, err := ac.initiatorReceiveAct2()
	if err != nil {
		return err
	}

	initiatorAct3, err := initiatorAct2.Next(act2Message)
	if err != nil {
		return err
	}

	//
	// Act 3
	//

	act3WireMessage, err := initiatorAct3.Message().Marshal()
	if err != nil {
		return err
	}

	if err := ac.initiatorSendAct3(act3WireMessage); err != nil {
		return err
	}

	return nil
}

// initiatorSendAct1 signs a marshaled *handshake.Act1Message, prepares
// the message in a pb.HandshakeEnvelope, and sends the message to the responder
// (over the open connection) from the initiator.
func (ac *authenticatedConnection) initiatorSendAct1(act1WireMessage []byte) error {
	signedAct1Message, err := ac.localPeerPrivateKey.Sign(act1WireMessage)
	if err != nil {
		return err
	}

	act1Envelope := &pb.HandshakeEnvelope{
		Message:   act1WireMessage,
		PeerID:    []byte(ac.localPeerID),
		Signature: signedAct1Message,
	}

	return ac.pipe.send(act1Envelope)
}

// initiatorReceiveAct2 unmarshals a pb.HandshakeEnvelope from a responder,
// verifies that the signed messages matches the expected peer.ID, and returns
// the handshake.Act2Message for processing by the initiator.
func (ac *authenticatedConnection) initiatorReceiveAct2() (*handshake.Act2Message, error) {
	var (
		act2Envelope pb.HandshakeEnvelope
		act2Message  = &handshake.Act2Message{}
	)

	if err := ac.pipe.receive(&act2Envelope); err != nil {
		return nil, err
	}

	if err := ac.verify(
		ac.remotePeerID,
		peer.ID(act2Envelope.GetPeerID()),
		act2Envelope.GetMessage(),
		act2Envelope.GetSignature(),
	); err != nil {
		return nil, err
	}

	if err := act2Message.Unmarshal(act2Envelope.Message); err != nil {
		return nil, err
	}

	return act2Message, nil
}

// initiatorSendAct3 signs a marshaled *handshake.Act3Message, prepares the
// message in a pb.HandshakeEnvelope, and sends the message to the responder
// (over the open connection) from the initiator.
func (ac *authenticatedConnection) initiatorSendAct3(act3WireMessage []byte) error {
	signedAct3Message, err := ac.localPeerPrivateKey.Sign(act3WireMessage)
	if err != nil {
		return err
	}

	act3Envelope := &pb.HandshakeEnvelope{
		Message:   act3WireMessage,
		PeerID:    []byte(ac.localPeerID),
		Signature: signedAct3Message,
	}

	return ac.pipe.send(act3Envelope)
}

func (ac *authenticatedConnection) runHandshakeAsResponder() error {
	// responder station

	//
	// Act 1
	//

	act1Message, err := ac.responderReceiveAct1()
	if err != nil {
		return err
	}

	responderAct2, err := handshake.AnswerHandshake(act1Message, ac.protocol)
	if err != nil {
		return err
	}

	//
	// Act 2
	//

	act2WireMessage, err := responderAct2.Message().Marshal()
	if err != nil {
		return err
	}
	if err := ac.responderSendAct2(act2WireMessage); err != nil {
		return err
	}

	responderAct3 := responderAct2.Next()

	//
	// Act 3
	//

	act3Message, err := ac.responderReceiveAct3()
	if err != nil {
		return err
	}

	if err := responderAct3.FinalizeHandshake(act3Message); err != nil {
		return err
	}

	return nil
}

// responderReceiveAct1 unmarshals a pb.HandshakeEnvelope from an initiator,
// verifies that the signed messages matches the expected peer.ID, and returns
// the handshake.Act1Message for processing by the responder.
func (ac *authenticatedConnection) responderReceiveAct1() (*handshake.Act1Message, error) {
	var (
		act1Envelope pb.HandshakeEnvelope
		act1Message  = &handshake.Act1Message{}
	)

	if err := ac.pipe.receive(&act1Envelope); err != nil {
		return nil, err
	}

	// In libp2p, the responder doesn't know the identity of the initiator
	// during the handshake. We overcome this limitation by sending the identity
	// and public key in the envelope. In the first act of the handshake, the
	// responder extracts this information.
	ac.remotePeerID = peer.ID(act1Envelope.GetPeerID())
	remotePublicKey, err := ac.remotePeerID.ExtractPublicKey()
	if err != nil {
		return nil, err
	}
	ac.remotePeerPublicKey = remotePublicKey

	if err := ac.verify(
		ac.remotePeerID,
		peer.ID(act1Envelope.GetPeerID()),
		act1Envelope.GetMessage(),
		act1Envelope.GetSignature(),
	); err != nil {
		return nil, err
	}

	if err := act1Message.Unmarshal(act1Envelope.Message); err != nil {
		return nil, err
	}

	return act1Message, nil
}

// responderSendAct2 signs a marshaled *handshake.Act2Message, prepares the
// message in a pb.HandshakeEnvelope, and sends the message to the initiator
// (over the open connection) from the responder.
func (ac *authenticatedConnection) responderSendAct2(act2WireMessage []byte) error {
	signedAct2Message, err := ac.localPeerPrivateKey.Sign(act2WireMessage)
	if err != nil {
		return err
	}

	act2Envelope := &pb.HandshakeEnvelope{
		Message:   act2WireMessage,
		PeerID:    []byte(ac.localPeerID),
		Signature: signedAct2Message,
	}

	return ac.pipe.send(act2Envelope)
}

// responderReceiveAct3 unmarshals a pb.HandshakeEnvelope from an initiator,
// verifies that the signed messages matches the expected peer.ID, and returns
// the handshake.Act3Message for processing by the responder.
func (ac *authenticatedConnection) responderReceiveAct3() (*handshake.Act3Message, error) {
	var (
		act3Envelope pb.HandshakeEnvelope
		act3Message  = &handshake.Act3Message{}
	)
	if err := ac.pipe.receive(&act3Envelope); err != nil {
		return nil, err
	}

	if err := ac.verify(
		ac.remotePeerID,
		peer.ID(act3Envelope.GetPeerID()),
		act3Envelope.GetMessage(),
		act3Envelope.GetSignature(),
	); err != nil {
		return nil, err
	}

	if err := act3Message.Unmarshal(act3Envelope.Message); err != nil {
		return nil, err
	}

	return act3Message, nil
}

// verify checks to see if the pinned (expected) identity matches the message
// sender's identity before running through the signature verification check.
func (ac *authenticatedConnection) verify(
	expectedSender, actualSender peer.ID,
	messageBytes, signatureBytes []byte,
) error {
	if expectedSender != actualSender {
		return fmt.Errorf(
			"pinned identity [%v] does not match sender identity [%v]",
			expectedSender,
			actualSender,
		)
	}

	pubKey, err := actualSender.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf(
			"failed to extract public key from peer [%v]",
			actualSender,
		)
	}

	ok, err := pubKey.Verify(messageBytes, signatureBytes)
	if err != nil {
		return fmt.Errorf(
			"failed to verify signature [0x%v] for sender [%v]: [%v]",
			hex.EncodeToString(signatureBytes),
			actualSender.String(),
			err,
		)
	}

	if !ok {
		return fmt.Errorf(
			"invalid signature [0x%v] on message from sender [%v]",
			hex.EncodeToString(signatureBytes),
			actualSender.String(),
		)
	}

	return nil
}

// LocalPeer retrieves the local peer.
func (ac *authenticatedConnection) LocalPeer() peer.ID {
	return ac.localPeerID
}

// LocalPrivateKey retrieves the local peer's privateKey
func (ac *authenticatedConnection) LocalPrivateKey() libp2pcrypto.PrivKey {
	return ac.localPeerPrivateKey
}

// RemotePeer returns the remote peer ID if we initiated the dial. Otherwise, it
// returns "" (because this connection isn't actually secure).
func (ac *authenticatedConnection) RemotePeer() peer.ID {
	return ac.remotePeerID
}

// RemotePublicKey retrieves the remote public key.
func (ac *authenticatedConnection) RemotePublicKey() libp2pcrypto.PubKey {
	return ac.remotePeerPublicKey
}

// ConnState returns information about the connection state.
func (ac *authenticatedConnection) ConnState() libp2pnetwork.ConnectionState {
	return ac.connState
}
