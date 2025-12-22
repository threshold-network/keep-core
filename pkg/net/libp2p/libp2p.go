package libp2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/operator"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
	"github.com/keep-network/keep-core/pkg/net/watchtower"

	dstore "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"

	//lint:ignore SA1019 package deprecated, but we rely on its interface
	addrutil "github.com/libp2p/go-addr-util"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/net/upgrader"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"

	ma "github.com/multiformats/go-multiaddr"
)

var logger = log.Logger("keep-libp2p")

// Defaults from ipfs
const (
	// DefaultPort is the default value for the connection port.
	DefaultPort = 3919
	// DefaultConnMgrHighWater is the default value for the connection managers
	// 'high water' mark
	DefaultConnMgrHighWater = 900

	// DefaultConnMgrLowWater is the default value for the connection managers 'low
	// water' mark
	DefaultConnMgrLowWater = 600

	// DefaultConnMgrGracePeriod is the default value for the connection managers
	// grace period
	DefaultConnMgrGracePeriod = time.Second * 20
)

// watchtower constants
const (
	// FirewallCheckTick is the amount of time between periodic checks of all
	// firewall rules against all peers connected to this one.
	FirewallCheckTick = time.Minute * 10
	// ConnectedPeersCheckTick is the amount of time between periodic checks of
	// the number of connected peers.
	ConnectedPeersCheckTick = time.Minute * 1
)

// MaximumDisseminationTime is the maximum dissemination time of messages in
// topics we are not subscribed to. By default courteous dissemination is
// disabled and it should be enabled only on selected fast bootstrap nodes.
// This value should never be higher than the lifetime of libp2p cache (120 sec)
// to prevent uncontrolled message propagation.
const MaximumDisseminationTime = 90

// pingTimeout is the maximum duration of the ping test performed for
// freshly connected peers.
const pingTestTimeout = 60 * time.Second

// Config defines the configuration for the libp2p network provider.
type Config struct {
	Bootstrap          bool
	Peers              []string
	Port               int
	AnnouncedAddresses []string
	DisseminationTime  int // TODO: Convert to time.Duration
}

type provider struct {
	channelManagerMutex     sync.Mutex
	broadcastChannelManager *channelManager

	identity          *identity
	host              host.Host
	routing           *dht.IpfsDHT
	disseminationTime int

	connectionManager *connectionManager

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

func (p *provider) BroadcastChannelFor(name string) (net.BroadcastChannel, error) {
	p.channelManagerMutex.Lock()
	defer p.channelManagerMutex.Unlock()
	return p.broadcastChannelManager.getChannel(name)
}

func (p *provider) Type() string {
	return "libp2p"
}

func (p *provider) ID() net.TransportIdentifier {
	return networkIdentity(p.identity.id)
}

func (p *provider) ConnectionManager() net.ConnectionManager {
	return p.connectionManager
}

func (p *provider) CreateTransportIdentifier(operatorPublicKey *operator.PublicKey) (
	net.TransportIdentifier,
	error,
) {
	networkPublicKey, err := operatorPublicKeyToNetworkPublicKey(operatorPublicKey)
	if err != nil {
		return nil, err
	}

	return peer.IDFromPublicKey(networkPublicKey)
}

func (p *provider) BroadcastChannelForwarderFor(name string) {
	if p.disseminationTime == 0 {
		return
	}

	logger.Infof("starting message forwarder for channel [%v]", name)
	timeout := time.Duration(p.disseminationTime) * time.Second

	if err := p.broadcastChannelManager.newForwarder(name, timeout); err != nil {
		logger.Warnf(
			"could not create message forwarder for channel [%v]: [%v]",
			name,
			err,
		)
	}
}

type connectionManager struct {
	host.Host
}

func newConnectionManager(ctx context.Context, host host.Host) *connectionManager {
	connectionManager := &connectionManager{host}

	go connectionManager.monitorConnectedPeers(ctx)

	return connectionManager
}

func (cm *connectionManager) ConnectedPeers() []string {
	var peers []string
	for _, connectedPeer := range cm.Network().Peers() {
		peers = append(peers, connectedPeer.String())
	}
	return peers
}

func (cm *connectionManager) ConnectedPeersAddrInfo() map[string][]string {
	// map[peerID][]peerAddresses
	peersAddrInfo := make(map[string][]string)
	for _, connectedPeer := range cm.Network().Peers() {
		addrPeerInfo := cm.Peerstore().PeerInfo(connectedPeer)
		var addresses []string
		for _, addr := range addrPeerInfo.Addrs {
			addresses = append(addresses, addr.String())
		}
		peersAddrInfo[addrPeerInfo.ID.String()] = addresses
	}
	return peersAddrInfo
}

func (cm *connectionManager) GetPeerPublicKey(connectedPeer string) (*operator.PublicKey, error) {
	peerID, err := peer.Decode(connectedPeer)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode peer ID from [%s]: [%v]",
			connectedPeer,
			err,
		)
	}

	peerPublicKey, err := peerID.ExtractPublicKey()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to extract peer [%s] public key: [%v]",
			connectedPeer,
			err,
		)
	}

	return networkPublicKeyToOperatorPublicKey(peerPublicKey)
}

func (cm *connectionManager) DisconnectPeer(peerHash string) {
	peerID, err := peer.Decode(peerHash)
	if err != nil {
		logger.Errorf("failed to decode peer hash [%v]: [%v]", peerHash, err)
		return
	}

	connections := cm.Network().ConnsToPeer(peerID)
	for _, connection := range connections {
		if err := connection.Close(); err != nil {
			logger.Errorf("failed to disconnect: [%v]", err)
		}
	}
}

func (cm *connectionManager) AddrStrings() []string {
	multiaddrStrings := make([]string, 0, len(cm.Addrs()))
	for _, multiaddr := range cm.Addrs() {
		multiaddrStrings = append(
			multiaddrStrings,
			multiaddressWithIdentity(multiaddr, cm.ID()),
		)
	}

	return multiaddrStrings
}

func (cm *connectionManager) IsConnected(address string) bool {
	peerInfos, err := extractMultiAddrFromPeers([]string{address})
	if err != nil {
		return false
	}

	return cm.Network().Connectedness(peerInfos[0].ID) == libp2pnet.Connected
}

func (cm *connectionManager) monitorConnectedPeers(ctx context.Context) {
	ticker := time.NewTicker(ConnectedPeersCheckTick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			connectedPeers := cm.ConnectedPeers()

			logger.Infof("number of connected peers: [%v]", len(connectedPeers))
			logger.Debugf("connected peers: [%v]", connectedPeers)
		case <-ctx.Done():
			return
		}
	}
}

// ConnectOptions allows to set various options used by libp2p.
type ConnectOptions struct {
	RoutingTableRefreshPeriod time.Duration
}

func defaultConnectOptions() *ConnectOptions {
	var options ConnectOptions

	// Half of the default value from libp2p.
	options.RoutingTableRefreshPeriod = 30 * time.Minute

	return &options
}

func (co *ConnectOptions) apply(options ...ConnectOption) {
	for _, option := range options {
		option(co)
	}
}

// ConnectOption allows to set an options used by libp2p.
type ConnectOption func(options *ConnectOptions)

// WithRoutingTableRefreshPeriod set a refresh period of the routing table.
func WithRoutingTableRefreshPeriod(period time.Duration) ConnectOption {
	return func(options *ConnectOptions) {
		options.RoutingTableRefreshPeriod = period
	}
}

// Connect connects to a libp2p network based on the provided config. The
// connection is managed in part by the passed context, and provides access to
// the functionality specified in the net.Provider interface.
//
// An error is returned if any part of the connection or bootstrap process
// fails.
func Connect(
	ctx context.Context,
	config Config,
	operatorPrivateKey *operator.PrivateKey,
	firewall net.Firewall,
	ticker *retransmission.Ticker,
	options ...ConnectOption,
) (net.Provider, error) {
	if config.DisseminationTime < 0 || config.DisseminationTime > MaximumDisseminationTime {
		return nil, fmt.Errorf(
			"dissemination time mut be in range [0, %v]",
			MaximumDisseminationTime,
		)
	}

	connectOptions := defaultConnectOptions()
	connectOptions.apply(options...)

	networkPrivateKey, _, err := operatorPrivateKeyToNetworkKeyPair(operatorPrivateKey)
	if err != nil {
		return nil, err
	}

	identity, err := createIdentity(networkPrivateKey)
	if err != nil {
		return nil, err
	}

	host, err := discoverAndListen(
		ctx,
		identity,
		config.Port,
		config.AnnouncedAddresses,
		firewall,
	)
	if err != nil {
		return nil, err
	}

	// Build notifiee with metrics recorder (will be set later if available)
	notifiee := buildNotifiee(host, nil)
	host.Network().Notify(notifiee)

	broadcastChannelManager, err := newChannelManager(ctx, identity, host, ticker)
	if err != nil {
		return nil, err
	}

	dhtDatastore := dssync.MutexWrap(dstore.NewMapDatastore())
	router, err := dht.New(
		ctx,
		host,
		dht.Datastore(dhtDatastore),
		dht.RoutingTableRefreshPeriod(
			connectOptions.RoutingTableRefreshPeriod,
		),
		dht.Mode(dht.ModeServer),
	)
	if err != nil {
		return nil, err
	}

	provider := &provider{
		broadcastChannelManager: broadcastChannelManager,
		identity:                identity,
		host:                    rhost.Wrap(host, router),
		routing:                 router,
		disseminationTime:       config.DisseminationTime,
	}

	if len(config.Peers) == 0 {
		logger.Infof("bootstrap peers list is empty")
	}

	if err := provider.bootstrap(ctx, config.Peers); err != nil {
		return nil, fmt.Errorf("bootstrap failed: [%v]", err)
	}

	provider.connectionManager = newConnectionManager(ctx, provider.host)

	// Instantiates and starts the connection management background process.
	watchtower.NewGuard(
		ctx,
		logger,
		FirewallCheckTick,
		firewall,
		provider.connectionManager,
	)

	return provider, nil
}

// SetMetricsRecorder sets the metrics recorder for the provider and wires it
// into network components.
func (p *provider) SetMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	p.metricsRecorder = recorder
	if p.broadcastChannelManager != nil {
		p.broadcastChannelManager.setMetricsRecorder(recorder)
	}
	// Update notifiee with metrics recorder
	p.host.Network().Notify(buildNotifiee(p.host, recorder))
}

func discoverAndListen(
	ctx context.Context,
	identity *identity,
	port int,
	announcedAddresses []string,
	firewall net.Firewall,
) (host.Host, error) {
	var err error

	// Get available network ifaces, for a specific port, as multiaddrs
	addrs, err := getListenAddrs(port)
	if err != nil {
		return nil, err
	}

	connectionManager, err := connmgr.NewConnManager(
		DefaultConnMgrLowWater,
		DefaultConnMgrHighWater,
		connmgr.WithGracePeriod(DefaultConnMgrGracePeriod),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create connection manager: [%v]",
			err,
		)
	}

	options := []libp2p.Option{
		libp2p.ListenAddrs(addrs...),
		libp2p.Identity(identity.privKey),
		libp2p.Security(
			securityProtocolID,
			func(
				protocolID protocol.ID,
				privateKey libp2pcrypto.PrivKey,
				muxers []upgrader.StreamMuxer,
			) (*transport, error) {
				encAuthTransport, err := newEncryptedAuthenticatedTransport(
					protocolID,
					authProtocolID,
					privateKey,
					muxers,
					firewall,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"could not create encrypted authenticated transport: [%v]",
						err,
					)
				}

				return encAuthTransport, nil
			},
		),
		libp2p.ConnectionManager(connectionManager),
	}

	if addresses := parseMultiaddresses(announcedAddresses); len(addresses) > 0 {
		addressFactory := func(addrs []ma.Multiaddr) []ma.Multiaddr {
			logger.Debugf(
				"replacing default announced addresses [%v] with [%v]",
				addrs,
				addresses,
			)
			return addresses
		}
		options = append(options, libp2p.AddrsFactory(addressFactory))
	}

	return libp2p.New(options...)
}

func getListenAddrs(port int) ([]ma.Multiaddr, error) {
	ia, err := addrutil.InterfaceAddresses()
	if err != nil {
		return nil, err
	}
	addrs := make([]ma.Multiaddr, 0)
	for _, addr := range ia {
		portAddr, err := ma.NewMultiaddr(fmt.Sprintf("/tcp/%d", port))
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr.Encapsulate(portAddr))
	}
	return addrs, nil
}

func parseMultiaddresses(addresses []string) []ma.Multiaddr {
	multiaddresses := make([]ma.Multiaddr, 0)
	for _, address := range addresses {
		multiaddress, err := ma.NewMultiaddr(address)
		if err != nil {
			logger.Warnf(
				"could not parse address string [%v]: [%v]",
				address,
				err,
			)
			continue
		}
		multiaddresses = append(multiaddresses, multiaddress)
	}

	return multiaddresses
}

func (p *provider) bootstrap(
	ctx context.Context,
	bootstrapPeers []string,
) error {
	peerInfos, err := extractMultiAddrFromPeers(bootstrapPeers)
	if err != nil {
		return err
	}

	ownID := p.identity.id
	filteredPeerInfos := make([]peer.AddrInfo, 0)

	// If the client's own address is on the list of bootstrap peers, filter it
	// out to prevent self-dialing.
	for _, peerInfo := range peerInfos {
		if peerInfo.ID != ownID {
			filteredPeerInfos = append(filteredPeerInfos, peerInfo)
		}
	}

	bootstrapConfig := bootstrapConfigWithPeers(filteredPeerInfos)

	// TODO: use the io.Closer to shutdown the bootstrapper when we build out
	// a shutdown process.
	_, err = Bootstrap(
		p.identity.id,
		p.host,
		p.routing,
		bootstrapConfig,
	)
	return err
}

func extractMultiAddrFromPeers(peers []string) ([]peer.AddrInfo, error) {
	var peerInfos []peer.AddrInfo
	for _, peerInstance := range peers {
		ipfsaddr, err := ma.NewMultiaddr(peerInstance)
		if err != nil {
			return nil, err
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(ipfsaddr)
		if err != nil {
			return nil, err
		}

		peerInfos = append(peerInfos, *peerInfo)
	}
	return peerInfos, nil
}

func buildNotifiee(libp2pHost host.Host, metricsRecorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) libp2pnet.Notifiee {
	notifyBundle := &libp2pnet.NotifyBundle{}

	notifyBundle.ConnectedF = func(_ libp2pnet.Network, connection libp2pnet.Conn) {
		peerID := connection.RemotePeer()

		peerMultiaddress := multiaddressWithIdentity(
			connection.RemoteMultiaddr(),
			peerID,
		)

		logger.Infof("established connection to [%v]", peerMultiaddress)

		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("peer_connections_total", 1)
		}

		go executePingTest(libp2pHost, peerID, peerMultiaddress, metricsRecorder)
	}
	notifyBundle.DisconnectedF = func(_ libp2pnet.Network, connection libp2pnet.Conn) {
		logger.Infof(
			"disconnected from [%v]",
			multiaddressWithIdentity(
				connection.RemoteMultiaddr(),
				connection.RemotePeer(),
			),
		)

		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("peer_disconnections_total", 1)
		}
	}

	return notifyBundle
}

func executePingTest(
	libp2pHost host.Host,
	peerID peer.ID,
	peerMultiaddress string,
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	},
) {
	logger.Infof("starting ping test for [%v]", peerMultiaddress)

	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter("ping_test_total", 1)
	}

	ctx, cancelCtx := context.WithTimeout(
		context.Background(),
		pingTestTimeout,
	)
	defer cancelCtx()

	startTime := time.Now()
	resultChan := ping.Ping(ctx, libp2pHost, peerID)

	select {
	case result := <-resultChan:
		if metricsRecorder != nil {
			metricsRecorder.RecordDuration("ping_test_duration_seconds", time.Since(startTime))
		}
		if result.Error != nil {
			logger.Warnf(
				"ping test for [%v] failed: [%v]",
				peerMultiaddress,
				result.Error,
			)
			if metricsRecorder != nil {
				metricsRecorder.IncrementCounter("ping_test_failed_total", 1)
			}
		} else if result.Error == nil && result.RTT == 0 {
			logger.Warnf(
				"peer test for [%v] failed without clear reason",
				peerMultiaddress,
			)
			if metricsRecorder != nil {
				metricsRecorder.IncrementCounter("ping_test_failed_total", 1)
			}
		} else {
			logger.Infof(
				"ping test for [%v] completed with success (RTT [%v])",
				peerMultiaddress,
				result.RTT,
			)
			if metricsRecorder != nil {
				metricsRecorder.IncrementCounter("ping_test_success_total", 1)
			}
		}
	case <-ctx.Done():
		logger.Warnf("ping test for [%v] timed out", peerMultiaddress)
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("ping_test_failed_total", 1)
		}
	}
}

func multiaddressWithIdentity(
	multiaddress ma.Multiaddr,
	peerID peer.ID,
) string {
	return fmt.Sprintf("%s/ipfs/%s", multiaddress.String(), peerID.String())
}

// ExtractPeersPublicKeys returns a list of operator public keys based on the
// provided list of peer addresses. Peer addresses must be in the format:
// <endpoint>/ipfs/<cid>
func ExtractPeersPublicKeys(peerAddresses []string) ([]*operator.PublicKey, error) {
	peerInfos, err := extractMultiAddrFromPeers(peerAddresses)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to extract multiaddress from peer addresses: [%v]",
			err,
		)
	}

	peersPublicKeys := make([]*operator.PublicKey, 0, len(peerInfos))

	for _, peerInfo := range peerInfos {
		peerNetworkPublicKey, err := peerInfo.ID.ExtractPublicKey()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to extract network public key for peer [%s]: [%v]",
				peerInfo.ID.String(),
				err,
			)
		}

		peerOperatorPublicKey, err := networkPublicKeyToOperatorPublicKey(
			peerNetworkPublicKey,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to convert to operator public key for peer [%s]: [%v]",
				peerInfo.ID.String(),
				err,
			)
		}

		peersPublicKeys = append(peersPublicKeys, peerOperatorPublicKey)
	}

	return peersPublicKeys, nil
}
