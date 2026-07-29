package tbtc

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
)

const (
	frostPrimaryEthereumTLSExporterLabel         = "EXPORTER-tbtc-frost-primary-ethereum-v1"
	frostPrimaryEthereumTLSExporterContextDomain = "tbtc-frost-primary-ethereum-tls-exporter-context/v1\x00"
	frostPrimaryEthereumPeerIdentityDomain       = "tbtc-frost-primary-ethereum-peer-identity/v1\x00"
	frostPrimaryEthereumMaximumSeenPeers         = 4096
)

// FrostPrimaryEthereumTransportConfig configures the primary Ethereum
// transport used by a FROST-enabled node. The transport accepts only direct
// TLS 1.3 HTTP/1.1 connections and freezes the first complete DNS answer.
type FrostPrimaryEthereumTransportConfig struct {
	URL            string
	RequestTimeout time.Duration
	TLSRootCAs     *x509.CertPool
	Resolver       *net.Resolver
}

// FrostPrimaryEthereumTransport owns the exact RPC client used by the primary
// Ethereum chain handle. Every HTTP connection and every WebSocket reconnect
// is TLS-verified and recorded before it can reach go-ethereum.
type FrostPrimaryEthereumTransport struct {
	mutex         sync.RWMutex
	endpoint      frostRetainedGroupResolvedEndpoint
	resolver      frostRetainedGroupResolver
	timeout       time.Duration
	rootCAs       *x509.CertPool
	rpcClient     *rpc.Client
	client        FrostPrimaryEthereumClient
	httpTransport *http.Transport
	seenPeers     map[[32]byte]frostTransportPeerIdentity
	livePeers     map[[32]byte]uint64
	policy        *frostPrimaryRetainedSeparationPolicy
	closed        bool
}

type frostTransportPeerIdentity struct {
	remoteIP             netip.Addr
	leafCertificateHash  [32]byte
	leafSPKIHash         [32]byte
	spiffeAuthorities    []string
	tlsExporterValueHash [32]byte
}

type frostPrimaryTrackedRawConnection struct {
	net.Conn
	transport *FrostPrimaryEthereumTransport
	mutex     sync.Mutex
	peerKey   [32]byte
	active    bool
	closeOnce sync.Once
}

type frostPrimaryEthereumHTTPRoundTripper struct {
	base      *http.Transport
	transport *FrostPrimaryEthereumTransport
	endpoint  frostRetainedGroupResolvedEndpoint
}

type frostPrimaryRetainedEndpointPolicy struct {
	endpoint frostRetainedGroupResolvedEndpoint
	identity FrostRetainedGroupEndpointIdentity
}

type frostPrimaryRetainedSeparationPolicy struct {
	mutex         sync.RWMutex
	primary       frostRetainedGroupResolvedEndpoint
	retained      map[string]frostPrimaryRetainedEndpointPolicy
	primaryPeers  map[[32]byte]frostTransportPeerIdentity
	retainedPeers map[string]map[[32]byte]frostTransportPeerIdentity
	failure       error
}

// NewFrostPrimaryEthereumTransport creates and probes a primary Ethereum
// client whose actual network channels can be bound to the retained endpoint
// independence policy.
func NewFrostPrimaryEthereumTransport(
	ctx context.Context,
	config FrostPrimaryEthereumTransportConfig,
) (*FrostPrimaryEthereumTransport, error) {
	resolver := frostRetainedGroupResolver(config.Resolver)
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return newFrostPrimaryEthereumTransport(ctx, config, resolver)
}

func newFrostPrimaryEthereumTransport(
	ctx context.Context,
	config FrostPrimaryEthereumTransportConfig,
	resolver frostRetainedGroupResolver,
) (*FrostPrimaryEthereumTransport, error) {
	if ctx == nil {
		return nil, fmt.Errorf("primary Ethereum transport context is nil")
	}
	timeout := config.RequestTimeout
	if timeout == 0 {
		timeout = frostRetainedGroupDefaultTimeout
	}
	if timeout < time.Second || timeout > time.Minute {
		return nil, fmt.Errorf(
			"primary Ethereum request timeout is outside supported bounds",
		)
	}
	endpointURL, err := validateFrostPrimaryEthereumTLSEndpoint(config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid primary Ethereum endpoint: [%w]", err)
	}
	if resolver == nil {
		return nil, fmt.Errorf("primary Ethereum resolver is nil")
	}
	resolveContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint, err := resolveFrostRetainedGroupEndpoint(
		resolveContext,
		endpointURL,
		resolver,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve primary Ethereum endpoint: [%w]", err)
	}
	var roots *x509.CertPool
	if config.TLSRootCAs != nil {
		roots = config.TLSRootCAs.Clone()
	}
	transport := &FrostPrimaryEthereumTransport{
		endpoint:  endpoint,
		resolver:  resolver,
		timeout:   timeout,
		rootCAs:   roots,
		seenPeers: make(map[[32]byte]frostTransportPeerIdentity),
		livePeers: make(map[[32]byte]uint64),
	}

	var rpcClient *rpc.Client
	switch endpoint.endpoint.Scheme {
	case "https":
		base := &http.Transport{
			Proxy:                  nil,
			DialTLSContext:         transport.dialTLSContext,
			DisableCompression:     true,
			ForceAttemptHTTP2:      false,
			MaxIdleConns:           16,
			MaxIdleConnsPerHost:    16,
			MaxConnsPerHost:        16,
			IdleConnTimeout:        90 * time.Second,
			ResponseHeaderTimeout:  timeout,
			ExpectContinueTimeout:  time.Second,
			MaxResponseHeaderBytes: 32 * 1024,
		}
		transport.httpTransport = base
		roundTripper := &frostPrimaryEthereumHTTPRoundTripper{
			base:      base,
			transport: transport,
			endpoint:  endpoint,
		}
		httpClient := &http.Client{
			Transport: roundTripper,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf("primary Ethereum redirects are forbidden")
			},
		}
		rpcClient, err = rpc.DialOptions(
			ctx,
			endpoint.canonical,
			rpc.WithHTTPClient(httpClient),
		)
	case "wss":
		dialer := websocket.Dialer{
			NetDialTLSContext: transport.dialTLSContext,
			Proxy:             nil,
			HandshakeTimeout:  timeout,
			EnableCompression: false,
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
		}
		rpcClient, err = rpc.DialOptions(
			ctx,
			endpoint.canonical,
			rpc.WithWebsocketDialer(dialer),
		)
	default:
		err = fmt.Errorf("primary Ethereum endpoint scheme is unsupported")
	}
	if err != nil {
		transport.closeConnections()
		return nil, fmt.Errorf("cannot dial guarded primary Ethereum endpoint: [%w]", err)
	}
	transport.rpcClient = rpcClient
	transport.client = &frostPrimaryEthereumTimeoutClient{
		client:         ethclient.NewClient(rpcClient),
		requestTimeout: timeout,
	}

	probeContext, probeCancel := context.WithTimeout(ctx, timeout)
	defer probeCancel()
	chainID, err := transport.client.ChainID(probeContext)
	if err != nil {
		transport.Close()
		return nil, fmt.Errorf(
			"cannot probe guarded primary Ethereum endpoint: [%w]",
			err,
		)
	}
	if chainID == nil || !chainID.IsUint64() || chainID.Sign() <= 0 {
		transport.Close()
		return nil, fmt.Errorf(
			"guarded primary Ethereum endpoint returned an invalid chain ID",
		)
	}
	transport.mutex.RLock()
	hasPeer := len(transport.seenPeers) > 0
	transport.mutex.RUnlock()
	if !hasPeer {
		transport.Close()
		return nil, fmt.Errorf(
			"guarded primary Ethereum probe has no authenticated peer",
		)
	}
	return transport, nil
}

// Client returns the exact client whose connections are guarded by this
// transport. It must be passed to ethereum.ConnectWithClient.
func (transport *FrostPrimaryEthereumTransport) Client() FrostPrimaryEthereumClient {
	if transport == nil {
		return nil
	}
	transport.mutex.RLock()
	defer transport.mutex.RUnlock()
	if transport.closed {
		return nil
	}
	return transport.client
}

// Close closes the primary RPC client and all idle HTTP connections.
func (transport *FrostPrimaryEthereumTransport) Close() {
	if transport == nil {
		return
	}
	transport.mutex.Lock()
	if transport.closed {
		transport.mutex.Unlock()
		return
	}
	transport.closed = true
	rpcClient := transport.rpcClient
	httpTransport := transport.httpTransport
	transport.mutex.Unlock()
	if rpcClient != nil {
		rpcClient.Close()
	}
	if httpTransport != nil {
		httpTransport.CloseIdleConnections()
	}
}

func (transport *FrostPrimaryEthereumTransport) closeConnections() {
	if transport == nil || transport.httpTransport == nil {
		return
	}
	transport.httpTransport.CloseIdleConnections()
}

func validateFrostPrimaryEthereumTLSEndpoint(raw string) (*url.URL, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return nil, fmt.Errorf("URL is empty or has surrounding whitespace")
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Host == "" || endpoint.User != nil ||
		endpoint.Fragment != "" || endpoint.Opaque != "" ||
		endpoint.RawPath != "" || endpoint.ForceQuery ||
		strings.Contains(raw, "\\") {
		return nil, fmt.Errorf("URL is not an unambiguous TLS endpoint")
	}
	if endpoint.Scheme != "https" && endpoint.Scheme != "wss" {
		return nil, fmt.Errorf("URL must use HTTPS or WSS")
	}
	hostname := endpoint.Hostname()
	if hostname == "" || hostname != strings.ToLower(hostname) ||
		strings.HasSuffix(hostname, ".") ||
		!validFrostRetainedGroupEndpointHostname(hostname) {
		return nil, fmt.Errorf("URL hostname is not canonical")
	}
	port := endpoint.Port()
	if port == "" {
		port = "443"
	} else {
		parsedPort, parseErr := strconv.ParseUint(port, 10, 16)
		if parseErr != nil || parsedPort == 0 ||
			strconv.FormatUint(parsedPort, 10) != port {
			return nil, fmt.Errorf("URL port is not canonical")
		}
	}
	endpoint.Host = net.JoinHostPort(hostname, port)
	if endpoint.Path == "" {
		endpoint.Path = "/"
	}
	if !strings.HasPrefix(endpoint.Path, "/") ||
		path.Clean(endpoint.Path) != endpoint.Path ||
		(endpoint.Path != "/" && strings.HasSuffix(endpoint.Path, "/")) ||
		strings.Contains(endpoint.Path, "//") ||
		endpoint.EscapedPath() != endpoint.Path {
		return nil, fmt.Errorf("URL path is not canonical")
	}
	if endpoint.RawQuery != "" {
		query, parseErr := url.ParseQuery(endpoint.RawQuery)
		if parseErr != nil || query.Encode() != endpoint.RawQuery {
			return nil, fmt.Errorf("URL query is not canonical")
		}
	}
	return endpoint, nil
}

func (transport *FrostPrimaryEthereumTransport) tlsConfig() *tls.Config {
	var roots *x509.CertPool
	if transport.rootCAs != nil {
		roots = transport.rootCAs.Clone()
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		ServerName: transport.endpoint.endpoint.Hostname(),
		RootCAs:    roots,
		NextProtos: []string{"http/1.1"},
	}
}

func (transport *FrostPrimaryEthereumTransport) dialTLSContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	if transport == nil || ctx == nil ||
		!strings.HasPrefix(network, "tcp") {
		return nil, fmt.Errorf("primary Ethereum TLS dial is invalid")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || host != transport.endpoint.endpoint.Hostname() ||
		port != transport.endpoint.endpoint.Port() {
		return nil, fmt.Errorf(
			"primary Ethereum transport attempted an unpinned endpoint",
		)
	}
	dialer := &net.Dialer{
		Timeout:   transport.timeout,
		KeepAlive: 30 * time.Second,
	}
	var lastErr error
	for _, pinned := range transport.endpoint.addresses {
		raw, dialErr := dialer.DialContext(
			ctx,
			network,
			net.JoinHostPort(pinned.String(), port),
		)
		if dialErr != nil {
			lastErr = dialErr
			continue
		}
		tracked := &frostPrimaryTrackedRawConnection{
			Conn:      raw,
			transport: transport,
		}
		tlsConnection := tls.Client(tracked, transport.tlsConfig())
		if handshakeErr := tlsConnection.HandshakeContext(ctx); handshakeErr != nil {
			_ = tlsConnection.Close()
			lastErr = handshakeErr
			continue
		}
		state := tlsConnection.ConnectionState()
		if verifyErr := verifyFrostPrimaryEthereumTLSConnection(
			state,
			transport.endpoint,
		); verifyErr != nil {
			_ = tlsConnection.Close()
			return nil, verifyErr
		}
		peer, peerErr := frostTransportPeerIdentityFromTLS(
			transport.endpoint,
			tlsConnection.RemoteAddr(),
			state,
		)
		if peerErr != nil {
			_ = tlsConnection.Close()
			return nil, peerErr
		}
		peerKey := frostTransportPeerIdentityKey(peer)
		if recordErr := transport.recordPeer(peerKey, peer); recordErr != nil {
			_ = tlsConnection.Close()
			return nil, recordErr
		}
		tracked.activate(peerKey)
		return tlsConnection, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("primary Ethereum endpoint has no pinned addresses")
	}
	return nil, fmt.Errorf(
		"cannot connect to a pinned primary Ethereum address: [%w]",
		lastErr,
	)
}

func verifyFrostPrimaryEthereumTLSConnection(
	state tls.ConnectionState,
	endpoint frostRetainedGroupResolvedEndpoint,
) error {
	if endpoint.endpoint == nil || len(state.VerifiedChains) == 0 ||
		len(state.PeerCertificates) == 0 {
		return fmt.Errorf("primary Ethereum TLS peer is not PKIX-verified")
	}
	if state.Version != tls.VersionTLS13 ||
		state.NegotiatedProtocol != "http/1.1" ||
		!state.NegotiatedProtocolIsMutual {
		return fmt.Errorf("primary Ethereum TLS protocol profile mismatch")
	}
	if err := state.PeerCertificates[0].VerifyHostname(
		endpoint.endpoint.Hostname(),
	); err != nil {
		return fmt.Errorf("primary Ethereum TLS hostname mismatch: [%w]", err)
	}
	return nil
}

func frostTransportPeerIdentityFromTLS(
	endpoint frostRetainedGroupResolvedEndpoint,
	remote net.Addr,
	state tls.ConnectionState,
) (frostTransportPeerIdentity, error) {
	if endpoint.endpoint == nil || remote == nil ||
		len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 ||
		!state.HandshakeComplete || state.Version != tls.VersionTLS13 ||
		state.NegotiatedProtocol != "http/1.1" {
		return frostTransportPeerIdentity{},
			fmt.Errorf("TLS peer observation is incomplete")
	}
	remoteHost, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		return frostTransportPeerIdentity{},
			fmt.Errorf("TLS peer address is invalid")
	}
	remoteIP, err := netip.ParseAddr(remoteHost)
	if err != nil || !remoteIP.IsValid() || remoteIP.Zone() != "" {
		return frostTransportPeerIdentity{},
			fmt.Errorf("TLS peer IP is invalid")
	}
	remoteIP = remoteIP.Unmap()
	found := false
	for _, pinned := range endpoint.addresses {
		found = found || pinned == remoteIP
	}
	if !found {
		return frostTransportPeerIdentity{},
			fmt.Errorf("TLS peer IP is outside the frozen address set")
	}
	leaf := state.PeerCertificates[0]
	authorities := make([]string, 0)
	seenAuthorities := make(map[string]bool)
	for _, serviceIdentity := range leaf.URIs {
		if serviceIdentity == nil || serviceIdentity.Scheme != "spiffe" {
			continue
		}
		if err := validateFrostRetainedGroupServiceIdentity(
			serviceIdentity.String(),
		); err != nil {
			return frostTransportPeerIdentity{},
				fmt.Errorf("TLS peer has an invalid SPIFFE identity: [%w]", err)
		}
		authority := serviceIdentity.Hostname()
		if !seenAuthorities[authority] {
			seenAuthorities[authority] = true
			authorities = append(authorities, authority)
		}
	}
	sort.Strings(authorities)
	leafSPKIHash := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	contextTranscript := frostRetainedGroupIdentityTranscript(
		frostPrimaryEthereumTLSExporterContextDomain,
	)
	contextTranscript.text("endpoint", endpoint.canonical)
	contextTranscript.text("remoteIP", remoteIP.String())
	contextTranscript.bytes32("leafSpkiHash", leafSPKIHash)
	exporterContext := contextTranscript.sum()
	exporterValue, err := state.ExportKeyingMaterial(
		frostPrimaryEthereumTLSExporterLabel,
		exporterContext[:],
		32,
	)
	if err != nil {
		return frostTransportPeerIdentity{},
			fmt.Errorf("cannot derive primary Ethereum TLS exporter: [%w]", err)
	}
	return frostTransportPeerIdentity{
		remoteIP:             remoteIP,
		leafCertificateHash:  sha256.Sum256(leaf.Raw),
		leafSPKIHash:         leafSPKIHash,
		spiffeAuthorities:    authorities,
		tlsExporterValueHash: sha256.Sum256(exporterValue),
	}, nil
}

func frostTransportPeerIdentityKey(
	peer frostTransportPeerIdentity,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostPrimaryEthereumPeerIdentityDomain,
	)
	transcript.text("remoteIP", peer.remoteIP.String())
	transcript.bytes32("leafCertificateHash", peer.leafCertificateHash)
	transcript.bytes32("leafSpkiHash", peer.leafSPKIHash)
	transcript.uint64(
		"spiffeAuthorityCount",
		uint64(len(peer.spiffeAuthorities)),
	)
	for index, authority := range peer.spiffeAuthorities {
		transcript.text(
			fmt.Sprintf("spiffeAuthority[%d]", index),
			authority,
		)
	}
	transcript.bytes32(
		"tlsExporterValueHash",
		peer.tlsExporterValueHash,
	)
	return transcript.sum()
}

func (transport *FrostPrimaryEthereumTransport) recordPeer(
	key [32]byte,
	peer frostTransportPeerIdentity,
) error {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	if transport.closed {
		return fmt.Errorf("primary Ethereum transport is closed")
	}
	if _, exists := transport.seenPeers[key]; !exists &&
		len(transport.seenPeers) >= frostPrimaryEthereumMaximumSeenPeers {
		return fmt.Errorf("primary Ethereum peer history limit exceeded")
	}
	transport.seenPeers[key] = peer
	if transport.policy != nil {
		if err := transport.policy.registerPrimaryPeer(key, peer); err != nil {
			return err
		}
	}
	transport.livePeers[key]++
	return nil
}

func (connection *frostPrimaryTrackedRawConnection) activate(key [32]byte) {
	connection.mutex.Lock()
	connection.peerKey = key
	connection.active = true
	connection.mutex.Unlock()
}

func (connection *frostPrimaryTrackedRawConnection) Close() error {
	err := connection.Conn.Close()
	connection.closeOnce.Do(func() {
		connection.mutex.Lock()
		key := connection.peerKey
		active := connection.active
		connection.mutex.Unlock()
		if active && connection.transport != nil {
			connection.transport.releaseLivePeer(key)
		}
	})
	return err
}

func (transport *FrostPrimaryEthereumTransport) releaseLivePeer(
	key [32]byte,
) {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	count := transport.livePeers[key]
	if count <= 1 {
		delete(transport.livePeers, key)
		return
	}
	transport.livePeers[key] = count - 1
}

func (roundTripper *frostPrimaryEthereumHTTPRoundTripper) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	if roundTripper == nil || roundTripper.base == nil ||
		roundTripper.transport == nil || request == nil ||
		request.URL == nil || request.Method != http.MethodPost ||
		request.URL.String() != roundTripper.endpoint.canonical ||
		(request.Host != "" && request.Host != roundTripper.endpoint.endpoint.Host) ||
		request.Header.Get("Accept-Encoding") != "" {
		return nil, fmt.Errorf("primary Ethereum HTTP request escaped its pinned target")
	}
	connection := make(chan net.Conn, 1)
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Conn != nil {
				select {
				case connection <- info.Conn:
				default:
				}
			}
		},
	}
	request = request.WithContext(
		httptrace.WithClientTrace(request.Context(), trace),
	)
	response, err := roundTripper.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	var actual net.Conn
	select {
	case actual = <-connection:
	default:
		_ = response.Body.Close()
		return nil, fmt.Errorf(
			"primary Ethereum response has no exact connection observation",
		)
	}
	tlsConnection, ok := actual.(*tls.Conn)
	if !ok || response.TLS == nil {
		_ = response.Body.Close()
		return nil, fmt.Errorf(
			"primary Ethereum response did not use the guarded TLS connection",
		)
	}
	if err := verifyFrostPrimaryEthereumTLSConnection(
		*response.TLS,
		roundTripper.endpoint,
	); err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	peer, err := frostTransportPeerIdentityFromTLS(
		roundTripper.endpoint,
		tlsConnection.RemoteAddr(),
		*response.TLS,
	)
	if err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	if err := roundTripper.transport.verifySeenPeer(
		frostTransportPeerIdentityKey(peer),
	); err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	if response.Uncompressed ||
		(response.Header.Get("Content-Encoding") != "" &&
			response.Header.Get("Content-Encoding") != "identity") {
		_ = response.Body.Close()
		return nil, fmt.Errorf(
			"primary Ethereum response transformation is forbidden",
		)
	}
	return response, nil
}

func (transport *FrostPrimaryEthereumTransport) verifySeenPeer(
	key [32]byte,
) error {
	transport.mutex.RLock()
	defer transport.mutex.RUnlock()
	if transport.closed {
		return fmt.Errorf("primary Ethereum transport is closed")
	}
	if _, exists := transport.seenPeers[key]; !exists {
		return fmt.Errorf(
			"primary Ethereum request used an unauthenticated TLS channel",
		)
	}
	return nil
}

func (transport *FrostPrimaryEthereumTransport) bindRetainedEndpoints(
	exportEndpoint frostRetainedGroupResolvedEndpoint,
	exportIdentity FrostRetainedGroupEndpointIdentity,
	verifierEndpoint frostRetainedGroupResolvedEndpoint,
	verifierIdentity FrostRetainedGroupEndpointIdentity,
) (*frostPrimaryRetainedSeparationPolicy, error) {
	if transport == nil {
		return nil, fmt.Errorf("primary Ethereum transport is nil")
	}
	policy, err := newFrostPrimaryRetainedSeparationPolicy(
		transport.endpoint,
		exportEndpoint,
		exportIdentity,
		verifierEndpoint,
		verifierIdentity,
	)
	if err != nil {
		return nil, err
	}
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	if transport.closed || transport.policy != nil ||
		len(transport.seenPeers) == 0 {
		return nil, fmt.Errorf(
			"primary Ethereum transport cannot bind retained endpoints",
		)
	}
	for key, peer := range transport.seenPeers {
		if err := policy.registerPrimaryPeer(key, peer); err != nil {
			return nil, err
		}
	}
	transport.policy = policy
	return policy, nil
}

func newFrostPrimaryRetainedSeparationPolicy(
	primary frostRetainedGroupResolvedEndpoint,
	exportEndpoint frostRetainedGroupResolvedEndpoint,
	exportIdentity FrostRetainedGroupEndpointIdentity,
	verifierEndpoint frostRetainedGroupResolvedEndpoint,
	verifierIdentity FrostRetainedGroupEndpointIdentity,
) (*frostPrimaryRetainedSeparationPolicy, error) {
	for role, value := range map[string]frostPrimaryRetainedEndpointPolicy{
		"retained-history-export": {
			endpoint: exportEndpoint,
			identity: exportIdentity,
		},
		"retained-history-verifier": {
			endpoint: verifierEndpoint,
			identity: verifierIdentity,
		},
	} {
		if value.identity.Role != role ||
			!frostResolvedEndpointMatchesIdentity(
				value.endpoint,
				value.identity,
			) {
			return nil, fmt.Errorf(
				"retained endpoint policy differs from its identity",
			)
		}
		if frostRetainedGroupEndpointSetsOverlap(primary, value.endpoint) {
			return nil, fmt.Errorf(
				"primary Ethereum frozen endpoint aliases %s",
				role,
			)
		}
	}
	return &frostPrimaryRetainedSeparationPolicy{
		primary: primary,
		retained: map[string]frostPrimaryRetainedEndpointPolicy{
			"retained-history-export": {
				endpoint: exportEndpoint,
				identity: exportIdentity,
			},
			"retained-history-verifier": {
				endpoint: verifierEndpoint,
				identity: verifierIdentity,
			},
		},
		primaryPeers: make(map[[32]byte]frostTransportPeerIdentity),
		retainedPeers: map[string]map[[32]byte]frostTransportPeerIdentity{
			"retained-history-export":   {},
			"retained-history-verifier": {},
		},
	}, nil
}

func frostResolvedEndpointMatchesIdentity(
	endpoint frostRetainedGroupResolvedEndpoint,
	identity FrostRetainedGroupEndpointIdentity,
) bool {
	return endpoint.endpoint != nil &&
		endpoint.canonical == identity.CanonicalEndpoint &&
		endpoint.canonicalDNSName == identity.CanonicalDNSName &&
		endpoint.resolvedDNSName == identity.ResolvedDNSName &&
		endpoint.addressSetHash == identity.ResolvedAddressSetHash
}

func (policy *frostPrimaryRetainedSeparationPolicy) registerPrimaryPeer(
	key [32]byte,
	peer frostTransportPeerIdentity,
) error {
	policy.mutex.Lock()
	defer policy.mutex.Unlock()
	if policy.failure != nil {
		return policy.failure
	}
	if _, exists := policy.primaryPeers[key]; !exists &&
		len(policy.primaryPeers) >= frostPrimaryEthereumMaximumSeenPeers {
		policy.failure = fmt.Errorf(
			"primary Ethereum policy peer history limit exceeded",
		)
		return policy.failure
	}
	policy.primaryPeers[key] = peer
	if !frostAddressSetContains(policy.primary.addresses, peer.remoteIP) {
		policy.failure = fmt.Errorf(
			"primary Ethereum actual peer is outside its frozen address set",
		)
		return policy.failure
	}
	for role, retained := range policy.retained {
		if err := frostPeerIndependentOfFrozenEndpoint(
			"primary Ethereum",
			peer,
			role,
			retained,
		); err != nil {
			policy.failure = err
			return err
		}
		for _, retainedPeer := range policy.retainedPeers[role] {
			if err := frostTransportPeersIndependent(
				"primary Ethereum",
				peer,
				role,
				retainedPeer,
			); err != nil {
				policy.failure = err
				return err
			}
		}
	}
	return nil
}

func (policy *frostPrimaryRetainedSeparationPolicy) registerRetainedPeer(
	role string,
	peer frostTransportPeerIdentity,
) error {
	policy.mutex.Lock()
	defer policy.mutex.Unlock()
	if policy.failure != nil {
		return policy.failure
	}
	retained, exists := policy.retained[role]
	if !exists {
		return fmt.Errorf("retained peer role is outside the separation policy")
	}
	peers := policy.retainedPeers[role]
	key := frostTransportPeerIdentityKey(peer)
	if _, exists := peers[key]; !exists &&
		len(peers) >= frostPrimaryEthereumMaximumSeenPeers {
		policy.failure = fmt.Errorf(
			"%s peer history limit exceeded",
			role,
		)
		return policy.failure
	}
	peers[key] = peer
	if !frostAddressSetContains(retained.endpoint.addresses, peer.remoteIP) ||
		peer.leafSPKIHash != retained.identity.TLSLeafSPKIHash ||
		!frostStringSetContains(
			peer.spiffeAuthorities,
			retained.identity.TrustDomainID,
		) {
		policy.failure = fmt.Errorf(
			"%s actual TLS peer differs from its frozen identity",
			role,
		)
		return policy.failure
	}
	if frostAddressSetContains(policy.primary.addresses, peer.remoteIP) {
		policy.failure = fmt.Errorf(
			"%s actual peer aliases the primary Ethereum frozen address set",
			role,
		)
		return policy.failure
	}
	for _, primaryPeer := range policy.primaryPeers {
		if err := frostTransportPeersIndependent(
			role,
			peer,
			"primary Ethereum",
			primaryPeer,
		); err != nil {
			policy.failure = err
			return err
		}
	}
	return nil
}

func frostPeerIndependentOfFrozenEndpoint(
	peerRole string,
	peer frostTransportPeerIdentity,
	frozenRole string,
	frozen frostPrimaryRetainedEndpointPolicy,
) error {
	if frostAddressSetContains(frozen.endpoint.addresses, peer.remoteIP) {
		return fmt.Errorf(
			"%s actual peer aliases %s frozen address set",
			peerRole,
			frozenRole,
		)
	}
	if peer.leafSPKIHash == frozen.identity.TLSLeafSPKIHash {
		return fmt.Errorf(
			"%s TLS leaf SPKI aliases %s",
			peerRole,
			frozenRole,
		)
	}
	if frostStringSetContains(
		peer.spiffeAuthorities,
		frozen.identity.TrustDomainID,
	) {
		return fmt.Errorf(
			"%s SPIFFE authority aliases %s",
			peerRole,
			frozenRole,
		)
	}
	return nil
}

func frostTransportPeersIndependent(
	leftRole string,
	left frostTransportPeerIdentity,
	rightRole string,
	right frostTransportPeerIdentity,
) error {
	if left.remoteIP == right.remoteIP {
		return fmt.Errorf(
			"%s actual peer IP aliases %s",
			leftRole,
			rightRole,
		)
	}
	if left.leafCertificateHash == right.leafCertificateHash {
		return fmt.Errorf(
			"%s TLS leaf certificate aliases %s",
			leftRole,
			rightRole,
		)
	}
	if left.leafSPKIHash == right.leafSPKIHash {
		return fmt.Errorf(
			"%s TLS leaf SPKI aliases %s",
			leftRole,
			rightRole,
		)
	}
	for _, authority := range left.spiffeAuthorities {
		if frostStringSetContains(right.spiffeAuthorities, authority) {
			return fmt.Errorf(
				"%s SPIFFE authority aliases %s",
				leftRole,
				rightRole,
			)
		}
	}
	return nil
}

func frostAddressSetContains(addresses []netip.Addr, target netip.Addr) bool {
	for _, address := range addresses {
		if address == target {
			return true
		}
	}
	return false
}

func frostStringSetContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (policy *frostPrimaryRetainedSeparationPolicy) verify() error {
	if policy == nil {
		return fmt.Errorf("primary/retained separation policy is nil")
	}
	policy.mutex.RLock()
	defer policy.mutex.RUnlock()
	if policy.failure != nil {
		return policy.failure
	}
	if len(policy.primaryPeers) == 0 {
		return fmt.Errorf(
			"primary/retained separation policy has no primary TLS peer",
		)
	}
	return nil
}

func (transport *FrostPrimaryEthereumTransport) verifyIndependence(
	ctx context.Context,
	exportEndpoint frostRetainedGroupResolvedEndpoint,
	verifierEndpoint frostRetainedGroupResolvedEndpoint,
) error {
	if transport == nil || ctx == nil {
		return fmt.Errorf("primary Ethereum transport verification is incomplete")
	}
	transport.mutex.RLock()
	if transport.closed || transport.policy == nil {
		transport.mutex.RUnlock()
		return fmt.Errorf("primary Ethereum transport is not bound")
	}
	endpoint := transport.endpoint
	resolver := transport.resolver
	timeout := transport.timeout
	policy := transport.policy
	transport.mutex.RUnlock()
	if frostRetainedGroupEndpointSetsOverlap(endpoint, exportEndpoint) ||
		frostRetainedGroupEndpointSetsOverlap(endpoint, verifierEndpoint) {
		return fmt.Errorf(
			"primary Ethereum frozen endpoint aliases a retained endpoint",
		)
	}
	resolveContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	current, err := resolveFrostRetainedGroupEndpoint(
		resolveContext,
		endpoint.endpoint,
		resolver,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot re-resolve primary Ethereum endpoint: [%w]",
			err,
		)
	}
	if current.canonical != endpoint.canonical ||
		current.canonicalDNSName != endpoint.canonicalDNSName ||
		current.resolvedDNSName != endpoint.resolvedDNSName ||
		current.addressSetHash != endpoint.addressSetHash {
		return fmt.Errorf("primary Ethereum DNS identity drifted")
	}
	if frostRetainedGroupEndpointSetsOverlap(current, exportEndpoint) ||
		frostRetainedGroupEndpointSetsOverlap(current, verifierEndpoint) {
		return fmt.Errorf(
			"primary Ethereum endpoint now aliases a retained endpoint",
		)
	}
	return policy.verify()
}

func (transport *FrostPrimaryEthereumTransport) frozenEndpoint() (
	frostRetainedGroupResolvedEndpoint,
	frostRetainedGroupResolver,
	error,
) {
	if transport == nil {
		return frostRetainedGroupResolvedEndpoint{}, nil,
			fmt.Errorf("primary Ethereum transport is nil")
	}
	transport.mutex.RLock()
	defer transport.mutex.RUnlock()
	if transport.closed || transport.endpoint.endpoint == nil ||
		transport.resolver == nil {
		return frostRetainedGroupResolvedEndpoint{}, nil,
			fmt.Errorf("primary Ethereum transport is incomplete")
	}
	endpoint := transport.endpoint
	endpoint.addresses = append([]netip.Addr{}, endpoint.addresses...)
	return endpoint, transport.resolver, nil
}

func (transport *FrostPrimaryEthereumTransport) peerCounts() (
	seen int,
	live uint64,
) {
	if transport == nil {
		return 0, 0
	}
	transport.mutex.RLock()
	defer transport.mutex.RUnlock()
	for _, count := range transport.livePeers {
		live += count
	}
	return len(transport.seenPeers), live
}
