package tbtc

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

func testFrostTransportPeer(exporter byte) frostTransportPeerIdentity {
	return frostTransportPeerIdentity{
		remoteIP:             netip.MustParseAddr("192.0.2.1"),
		leafCertificateHash:  [32]byte{0x01},
		leafSPKIHash:         [32]byte{0x02},
		spiffeAuthorities:    []string{"primary.example"},
		tlsExporterValueHash: [32]byte{exporter},
	}
}

func newTestFrostPrimaryEthereumTransport() *FrostPrimaryEthereumTransport {
	return &FrostPrimaryEthereumTransport{
		seenPeers: make(map[[32]byte]frostTransportPeerIdentity),
		livePeers: make(map[[32]byte]uint64),
	}
}

func TestFrostTransportPeerKeysSeparateHistoryFromLiveChannel(t *testing.T) {
	first := testFrostTransportPeer(0x11)
	second := testFrostTransportPeer(0x22)

	if frostTransportPeerIdentityKey(first) !=
		frostTransportPeerIdentityKey(second) {
		t.Fatal("TLS reconnect changed stable peer identity")
	}
	if frostTransportPeerChannelKey(first) ==
		frostTransportPeerChannelKey(second) {
		t.Fatal("distinct TLS exporters produced the same live channel key")
	}
}

func TestFrostPrimaryEthereumTransportReconnectsDoNotExhaustHistory(
	t *testing.T,
) {
	transport := newTestFrostPrimaryEthereumTransport()

	for index := 0; index <= frostPrimaryEthereumMaximumSeenPeers; index++ {
		peer := testFrostTransportPeer(byte(index))
		peer.tlsExporterValueHash = sha256.Sum256(
			[]byte(fmt.Sprintf("exporter-%d", index)),
		)
		if err := transport.recordPeer(
			frostTransportPeerIdentityKey(peer),
			peer,
		); err != nil {
			t.Fatalf("reconnect [%d] failed: [%v]", index, err)
		}
		transport.releaseLivePeer(frostTransportPeerChannelKey(peer))
	}

	seen, live := transport.peerCounts()
	if seen != 1 || live != 0 {
		t.Fatalf("unexpected peer counts [%d, %d]", seen, live)
	}
}

func TestFrostRetainedPeerReconnectsDoNotExhaustHistory(t *testing.T) {
	primaryIP := netip.MustParseAddr("192.0.2.1")
	retainedIP := netip.MustParseAddr("192.0.2.2")
	retainedPeer := testFrostTransportPeer(0x11)
	retainedPeer.remoteIP = retainedIP
	retainedPeer.leafSPKIHash = [32]byte{0x44}
	retainedPeer.spiffeAuthorities = []string{"retained.example"}

	policy := &frostPrimaryRetainedSeparationPolicy{
		primary: frostRetainedGroupResolvedEndpoint{
			addresses: []netip.Addr{primaryIP},
		},
		retained: map[string]frostPrimaryRetainedEndpointPolicy{
			"retained-history-export": {
				endpoint: frostRetainedGroupResolvedEndpoint{
					addresses: []netip.Addr{retainedIP},
				},
				identity: FrostRetainedGroupEndpointIdentity{
					TrustDomainID:   "retained.example",
					TLSLeafSPKIHash: retainedPeer.leafSPKIHash,
				},
			},
		},
		primaryPeers: map[[32]byte]frostTransportPeerIdentity{
			{0x01}: {
				remoteIP:            primaryIP,
				leafCertificateHash: [32]byte{0x55},
				leafSPKIHash:        [32]byte{0x66},
				spiffeAuthorities:   []string{"primary.example"},
			},
		},
		retainedPeers: map[string]map[[32]byte]frostTransportPeerIdentity{
			"retained-history-export": {},
		},
	}

	for index := 0; index <= frostPrimaryEthereumMaximumSeenPeers; index++ {
		retainedPeer.tlsExporterValueHash = sha256.Sum256(
			[]byte(fmt.Sprintf("retained-exporter-%d", index)),
		)
		if err := policy.registerRetainedPeer(
			"retained-history-export",
			retainedPeer,
		); err != nil {
			t.Fatalf("retained reconnect [%d] failed: [%v]", index, err)
		}
	}

	if actual := len(
		policy.retainedPeers["retained-history-export"],
	); actual != 1 {
		t.Fatalf("unexpected retained peer history size [%d]", actual)
	}
}

// TestFrostRetainedPeerLeafRotationDoesNotLatchSeparationFailure rotates the
// retained leaf certificate past the bounded history cap while its key, address
// and SPIFFE authorities stay frozen, which is what routine certificate
// rotation looks like to this policy. The rotation must not consume history,
// and the separation property must still hold afterwards.
func TestFrostRetainedPeerLeafRotationDoesNotLatchSeparationFailure(
	t *testing.T,
) {
	primaryIP := netip.MustParseAddr("192.0.2.1")
	retainedIP := netip.MustParseAddr("192.0.2.2")
	retainedPeer := testFrostTransportPeer(0x11)
	retainedPeer.remoteIP = retainedIP
	retainedPeer.leafSPKIHash = [32]byte{0x44}
	retainedPeer.spiffeAuthorities = []string{
		"retained.example",
		"shared.example",
	}

	policy := &frostPrimaryRetainedSeparationPolicy{
		primary: frostRetainedGroupResolvedEndpoint{
			addresses: []netip.Addr{primaryIP},
		},
		retained: map[string]frostPrimaryRetainedEndpointPolicy{
			"retained-history-export": {
				endpoint: frostRetainedGroupResolvedEndpoint{
					addresses: []netip.Addr{retainedIP},
				},
				identity: FrostRetainedGroupEndpointIdentity{
					TrustDomainID:   "retained.example",
					TLSLeafSPKIHash: retainedPeer.leafSPKIHash,
				},
			},
		},
		primaryPeers: map[[32]byte]frostTransportPeerIdentity{},
		retainedPeers: map[string]map[[32]byte]frostTransportPeerIdentity{
			"retained-history-export": {},
		},
	}
	primaryPeer := frostTransportPeerIdentity{
		remoteIP:            primaryIP,
		leafCertificateHash: [32]byte{0x55},
		leafSPKIHash:        [32]byte{0x66},
		spiffeAuthorities:   []string{"primary.example"},
	}
	if err := policy.registerPrimaryPeer(
		frostTransportPeerIdentityKey(primaryPeer),
		primaryPeer,
	); err != nil {
		t.Fatal(err)
	}

	for index := 0; index <= frostPrimaryEthereumMaximumSeenPeers; index++ {
		retainedPeer.leafCertificateHash = sha256.Sum256(
			[]byte(fmt.Sprintf("retained-certificate-%d", index)),
		)
		retainedPeer.tlsExporterValueHash = sha256.Sum256(
			[]byte(fmt.Sprintf("retained-exporter-%d", index)),
		)
		if err := policy.registerRetainedPeer(
			"retained-history-export",
			retainedPeer,
		); err != nil {
			t.Fatalf("retained leaf rotation [%d] failed: [%v]", index, err)
		}
	}

	if actual := len(
		policy.retainedPeers["retained-history-export"],
	); actual != 1 {
		t.Fatalf("unexpected retained peer history size [%d]", actual)
	}
	if err := policy.verify(); err != nil {
		t.Fatalf("routine leaf rotation latched a separation failure: [%v]", err)
	}

	// The surviving entry still decides the comparisons the frozen identities
	// cannot: this primary peer satisfies the frozen retained identity, and only
	// the retained history reveals that it shares a SPIFFE authority with the
	// retained endpoint's actual peer.
	aliasingPrimaryPeer := frostTransportPeerIdentity{
		remoteIP:            primaryIP,
		leafCertificateHash: [32]byte{0x77},
		leafSPKIHash:        [32]byte{0x78},
		spiffeAuthorities:   []string{"primary.example", "shared.example"},
	}
	if err := policy.registerPrimaryPeer(
		frostTransportPeerIdentityKey(aliasingPrimaryPeer),
		aliasingPrimaryPeer,
	); err == nil {
		t.Fatal("primary peer aliasing the retained peer was accepted")
	}
	if err := policy.verify(); err == nil {
		t.Fatal("separation policy stayed healthy after an aliasing peer")
	}
}

func TestFrostPrimaryEthereumTransportRejectsNewStablePeerPastLimit(
	t *testing.T,
) {
	transport := newTestFrostPrimaryEthereumTransport()

	for index := 0; index < frostPrimaryEthereumMaximumSeenPeers; index++ {
		peer := testFrostTransportPeer(0x11)
		peer.leafCertificateHash = sha256.Sum256(
			[]byte(fmt.Sprintf("certificate-%d", index)),
		)
		if err := transport.recordPeer(
			frostTransportPeerIdentityKey(peer),
			peer,
		); err != nil {
			t.Fatalf("stable peer [%d] failed: [%v]", index, err)
		}
	}

	peer := testFrostTransportPeer(0x11)
	peer.leafCertificateHash = sha256.Sum256([]byte("one-too-many"))
	if err := transport.recordPeer(
		frostTransportPeerIdentityKey(peer),
		peer,
	); err == nil {
		t.Fatal("new stable peer beyond history limit was accepted")
	}
}

func TestFrostPrimaryEthereumTransportRequiresExactLiveChannel(t *testing.T) {
	transport := newTestFrostPrimaryEthereumTransport()
	peer := testFrostTransportPeer(0x11)
	if err := transport.recordPeer(
		frostTransportPeerIdentityKey(peer),
		peer,
	); err != nil {
		t.Fatal(err)
	}
	channelKey := frostTransportPeerChannelKey(peer)

	if err := transport.verifySeenPeer(peer); err != nil {
		t.Fatalf("live channel rejected: [%v]", err)
	}

	otherChannel := peer
	otherChannel.tlsExporterValueHash = [32]byte{0x22}
	if err := transport.verifySeenPeer(otherChannel); err == nil {
		t.Fatal("unrecorded TLS channel accepted")
	}

	transport.releaseLivePeer(channelKey)
	if err := transport.verifySeenPeer(peer); err == nil {
		t.Fatal("closed TLS channel accepted")
	}
}

func TestFrostPrimaryEthereumTransportTriesEveryPinnedAddressWithinDeadline(
	t *testing.T,
) {
	server, _, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		"spiffe://primary.example/rpc",
	)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}

	stalledListener, err := net.Listen(
		"tcp6",
		net.JoinHostPort("::1", port),
	)
	if err != nil {
		t.Fatal(err)
	}
	releaseStalledConnection := make(chan struct{})
	stalledConnectionDone := make(chan struct{})
	go func() {
		defer close(stalledConnectionDone)
		connection, acceptErr := stalledListener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		<-releaseStalledConnection
	}()
	t.Cleanup(func() {
		close(releaseStalledConnection)
		_ = stalledListener.Close()
		<-stalledConnectionDone
	})

	transport := &FrostPrimaryEthereumTransport{
		endpoint: frostRetainedGroupResolvedEndpoint{
			endpoint:  endpoint,
			canonical: endpoint.String(),
			addresses: []netip.Addr{
				netip.MustParseAddr("::1"),
				netip.MustParseAddr("127.0.0.1"),
			},
		},
		timeout:   time.Second,
		rootCAs:   roots,
		seenPeers: make(map[[32]byte]frostTransportPeerIdentity),
		livePeers: make(map[[32]byte]uint64),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, err := transport.dialTLSContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		t.Fatalf(
			"healthy pinned address was starved by a stalled predecessor: [%v]",
			err,
		)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFrostPrimaryEthereumTransportTriesNextAddressAfterTLSProfileRejection(
	t *testing.T,
) {
	server, _, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		"spiffe://primary.example/rpc",
	)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}

	profileMismatchListener, err := net.Listen(
		"tcp6",
		net.JoinHostPort("::1", port),
	)
	if err != nil {
		t.Fatal(err)
	}
	profileMismatchDone := make(chan struct{})
	go func() {
		defer close(profileMismatchDone)
		raw, acceptErr := profileMismatchListener.Accept()
		if acceptErr != nil {
			return
		}
		serverTLSConfig := server.TLS.Clone()
		serverTLSConfig.NextProtos = nil
		connection := tls.Server(raw, serverTLSConfig)
		defer connection.Close()
		_ = connection.Handshake()
	}()
	t.Cleanup(func() {
		_ = profileMismatchListener.Close()
		<-profileMismatchDone
	})

	transport := &FrostPrimaryEthereumTransport{
		endpoint: frostRetainedGroupResolvedEndpoint{
			endpoint:  endpoint,
			canonical: endpoint.String(),
			addresses: []netip.Addr{
				netip.MustParseAddr("::1"),
				netip.MustParseAddr("127.0.0.1"),
			},
		},
		timeout:   time.Second,
		rootCAs:   roots,
		seenPeers: make(map[[32]byte]frostTransportPeerIdentity),
		livePeers: make(map[[32]byte]uint64),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, err := transport.dialTLSContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		t.Fatalf(
			"healthy pinned address was ignored after TLS profile rejection: [%v]",
			err,
		)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFrostPrimaryEthereumTransportTriesNextAddressAfterPeerIdentityRejection(
	t *testing.T,
) {
	server, _, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		"spiffe://primary.example/rpc",
	)
	identityMismatchServer, identityMismatchLeaf, _ :=
		newFrostRetainedGroupHistoryTLSTestServer(
			t,
			http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			"spiffe://Primary.example/rpc",
		)
	roots.AddCert(identityMismatchLeaf)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}

	identityMismatchListener, err := net.Listen(
		"tcp6",
		net.JoinHostPort("::1", port),
	)
	if err != nil {
		t.Fatal(err)
	}
	identityMismatchDone := make(chan struct{})
	go func() {
		defer close(identityMismatchDone)
		raw, acceptErr := identityMismatchListener.Accept()
		if acceptErr != nil {
			return
		}
		connection := tls.Server(raw, identityMismatchServer.TLS.Clone())
		defer connection.Close()
		_ = connection.Handshake()
	}()
	t.Cleanup(func() {
		_ = identityMismatchListener.Close()
		<-identityMismatchDone
	})

	transport := &FrostPrimaryEthereumTransport{
		endpoint: frostRetainedGroupResolvedEndpoint{
			endpoint:  endpoint,
			canonical: endpoint.String(),
			addresses: []netip.Addr{
				netip.MustParseAddr("::1"),
				netip.MustParseAddr("127.0.0.1"),
			},
		},
		timeout:   time.Second,
		rootCAs:   roots,
		seenPeers: make(map[[32]byte]frostTransportPeerIdentity),
		livePeers: make(map[[32]byte]uint64),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, err := transport.dialTLSContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		t.Fatalf(
			"healthy pinned address was ignored after peer identity rejection: [%v]",
			err,
		)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestFrostPrimaryEthereumTransportHTTPSRoundTripFreezesPeerIdentity drives a
// real JSON-RPC exchange through the guarded HTTPS round tripper against a live
// TLS server, then replays the exact same request over a second TLS server that
// serves the same SPIFFE identity from a rotated key at the same pinned address.
// Both directions matter: the frozen peer must round-trip, and a peer that never
// passed the guarded dialer must be refused even though it satisfies PKIX, the
// TLS profile, the hostname, and the frozen address set.
func TestFrostPrimaryEthereumTransportHTTPSRoundTripFreezesPeerIdentity(
	t *testing.T,
) {
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName(
		"eth",
		&testFrostPrimaryEthereumChainIDRPC{},
	); err != nil {
		t.Fatal(err)
	}
	server, _, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		rpcServer,
		"spiffe://primary.example/rpc",
	)
	transport, err := NewFrostPrimaryEthereumTransport(
		context.Background(),
		FrostPrimaryEthereumTransportConfig{
			URL:            server.URL,
			RequestTimeout: time.Second,
			TLSRootCAs:     roots,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()

	requestContext, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()
	chainID, err := transport.Client().ChainID(requestContext)
	if err != nil {
		t.Fatalf("guarded HTTPS round trip rejected the frozen peer: [%v]", err)
	}
	if chainID == nil || chainID.Uint64() != 1 {
		t.Fatalf("guarded HTTPS round trip returned chain ID [%v]", chainID)
	}
	if seen, live := transport.peerCounts(); seen != 1 || live == 0 {
		t.Fatalf(
			"guarded HTTPS round trip left peer counts [%d, %d]",
			seen,
			live,
		)
	}

	rotatedServer, rotatedLeaf, _ := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		rpcServer,
		"spiffe://primary.example/rpc",
	)
	rotatedEndpoint, err := url.Parse(rotatedServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	rotatedRoots := roots.Clone()
	rotatedRoots.AddCert(rotatedLeaf)
	rotatedTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		ServerName: transport.endpoint.endpoint.Hostname(),
		RootCAs:    rotatedRoots,
		NextProtos: []string{"http/1.1"},
	}
	rotated := &frostPrimaryEthereumHTTPRoundTripper{
		base: &http.Transport{
			Proxy:              nil,
			DisableCompression: true,
			ForceAttemptHTTP2:  false,
			DialTLSContext: func(
				ctx context.Context,
				network string,
				_ string,
			) (net.Conn, error) {
				dialer := &net.Dialer{}
				raw, dialErr := dialer.DialContext(
					ctx,
					network,
					rotatedEndpoint.Host,
				)
				if dialErr != nil {
					return nil, dialErr
				}
				connection := tls.Client(raw, rotatedTLSConfig)
				if handshakeErr := connection.HandshakeContext(
					ctx,
				); handshakeErr != nil {
					_ = connection.Close()
					return nil, handshakeErr
				}
				return connection, nil
			},
		},
		transport: transport,
		endpoint:  transport.endpoint,
	}
	defer rotated.base.CloseIdleConnections()

	rotatedRequest, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		transport.endpoint.canonical,
		strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	rotatedRequest.Header.Set("Content-Type", "application/json")
	response, err := rotated.RoundTrip(rotatedRequest)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("rotated TLS identity round-tripped through the frozen transport")
	}
	if response != nil {
		t.Fatal("rejected round trip returned a response")
	}
	if !strings.Contains(err.Error(), "unauthenticated TLS channel") {
		t.Fatalf("rotated TLS identity was rejected for [%v]", err)
	}
	if seen, _ := transport.peerCounts(); seen != 1 {
		t.Fatalf(
			"rejected round trip recorded the rotated identity; seen [%d]",
			seen,
		)
	}
}

func TestFrostPrimaryEthereumTransportWSSAppliesRequestTimeout(
	t *testing.T,
) {
	service := &testFrostPrimaryEthereumStalledRPC{
		stalled: make(chan struct{}),
		release: make(chan struct{}),
	}
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName("eth", service); err != nil {
		t.Fatal(err)
	}
	server, _, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		rpcServer.WebsocketHandler([]string{"*"}),
		"spiffe://primary.example/rpc",
	)
	transport, err := NewFrostPrimaryEthereumTransport(
		context.Background(),
		FrostPrimaryEthereumTransportConfig{
			URL: strings.Replace(
				server.URL,
				"https://",
				"wss://",
				1,
			),
			RequestTimeout: time.Second,
			TLSRootCAs:     roots,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	if transport.ChainID() != 1 {
		t.Fatalf(
			"guarded transport did not retain probed chain ID: [%d]",
			transport.ChainID(),
		)
	}

	result := make(chan error, 1)
	go func() {
		_, err := transport.Client().ChainID(context.Background())
		result <- err
	}()
	select {
	case <-service.stalled:
	case <-time.After(time.Second):
		close(service.release)
		t.Fatal("post-probe WSS request did not reach the stalled provider")
	}

	select {
	case err := <-result:
		close(service.release)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled WSS request returned unexpected error: [%v]", err)
		}
	case <-time.After(3 * time.Second):
		close(service.release)
		err := <-result
		t.Fatalf(
			"stalled WSS request exceeded configured timeout; eventual result: [%v]",
			err,
		)
	}
}

type testFrostPrimaryEthereumChainIDRPC struct{}

func (service *testFrostPrimaryEthereumChainIDRPC) ChainId(
	context.Context,
) (*hexutil.Big, error) {
	value := hexutil.Big(*big.NewInt(1))
	return &value, nil
}

type testFrostPrimaryEthereumStalledRPC struct {
	mutex   sync.Mutex
	calls   int
	stalled chan struct{}
	release chan struct{}
	once    sync.Once
}

func (service *testFrostPrimaryEthereumStalledRPC) ChainId(
	ctx context.Context,
) (*hexutil.Big, error) {
	service.mutex.Lock()
	service.calls++
	call := service.calls
	service.mutex.Unlock()
	if call == 1 {
		value := hexutil.Big(*big.NewInt(1))
		return &value, nil
	}
	service.once.Do(func() {
		close(service.stalled)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-service.release:
		value := hexutil.Big(*big.NewInt(1))
		return &value, nil
	}
}
