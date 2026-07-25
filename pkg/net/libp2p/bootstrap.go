package libp2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	periodicproc "github.com/jbenet/goprocess/periodic"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
)

// ErrNotEnoughBootstrapPeers signals that we do not have enough bootstrap
// peers to bootstrap correctly.
var ErrNotEnoughBootstrapPeers = errors.New("not enough bootstrap peers to bootstrap")

// BootstrapConfig specifies parameters used in the network bootstrapping process.
type BootstrapConfig struct {
	// Period governs the periodic interval at which the node will
	// attempt to bootstrap. The bootstrap process is not very expensive, so
	// this threshold can afford to be small (<=30s).
	Period time.Duration

	// ConnectionTimeout determines how long to wait for a bootstrap
	// connection attempt before cancelling it.
	ConnectionTimeout time.Duration

	// BootstrapPeers is a function that returns a set of bootstrap peers
	// for the bootstrap process to use. This makes it possible for clients
	// to control the peers the process uses at any moment.
	BootstrapPeers func() []peer.AddrInfo
}

// DefaultBootstrapConfig specifies default sane parameters for bootstrapping.
var DefaultBootstrapConfig = BootstrapConfig{
	Period:            30 * time.Second,
	ConnectionTimeout: (30 * time.Second) / 3, // Perod / 3
}

func bootstrapConfigWithPeers(pis []peer.AddrInfo) BootstrapConfig {
	cfg := DefaultBootstrapConfig
	cfg.BootstrapPeers = func() []peer.AddrInfo {
		return pis
	}
	return cfg
}

// Bootstrap kicks off bootstrapping. This function will periodically
// check the number of open connections and -- if there are too few -- initiate
// connections to well-known bootstrap peers. It also kicks off subsystem
// bootstrapping (i.e. routing).
func Bootstrap(
	id peer.ID,
	host host.Host,
	rt routing.Routing,
	cfg BootstrapConfig,
) (io.Closer, error) {
	// make a signal to wait for one bootstrap round to complete.
	doneWithRound := make(chan struct{})

	// the periodic bootstrap function -- the connection supervisor
	periodic := func(worker goprocess.Process) {
		ctx := goprocessctx.OnClosingContext(worker)

		// bootstrapRound returns an error only when the node ends the
		// round connected to zero well-known peers (true isolation risk).
		// The round already logged that outcome at warning level with the
		// full unreachable/total ratio -- it is the single owner of the
		// round summary log -- so the error is only traced here at debug
		// level to avoid emitting a duplicate warning for the same round.
		if err := bootstrapRound(ctx, host, cfg); err != nil {
			logger.Debugf("well-known peers connection round error: [%v]", err)
		}

		<-doneWithRound
	}

	// kick off the node's periodic bootstrapping
	proc := periodicproc.Tick(cfg.Period, periodic)
	proc.Go(periodic) // run one right now.

	// kick off Routing.Bootstrap
	if rt != nil {
		ctx := goprocessctx.OnClosingContext(proc)
		if err := rt.Bootstrap(ctx); err != nil {
			if procErr := proc.Close(); procErr != nil {
				return nil, fmt.Errorf(
					"process closing error [%v] while handling bootstrap error [%v]",
					procErr,
					err,
				)
			}
			return nil, err
		}
	}

	doneWithRound <- struct{}{}
	close(doneWithRound) // it no longer blocks periodic
	return proc, nil
}

func bootstrapRound(
	ctx context.Context,
	host host.Host,
	cfg BootstrapConfig,
) error {
	ctx, cancel := context.WithTimeout(ctx, cfg.ConnectionTimeout)
	defer cancel()

	logger.Debugf("starting well-known peers connection round")

	// get well-known peers from config. retrieving them here makes
	// sure we remain observant of changes to client configuration.
	peers := cfg.BootstrapPeers()

	if len(peers) == 0 {
		logger.Debugf(
			"well-known peers connection round skipped; " +
				"no well-known peers in config",
		)
		return nil
	}

	// filter out well-known peers we are already connected to
	var notConnected []peer.AddrInfo
	for _, p := range peers {
		if host.Network().Connectedness(p.ID) != network.Connected {
			notConnected = append(notConnected, p)
		}
	}

	// if connected to all well-known peer candidates, exit
	if len(notConnected) < 1 {
		logger.Debugf(
			"well-known peers connection round skipped; " +
				"connected to all well-known peers from config",
		)
		return nil
	}

	logger.Debugf("connecting to well-known peers: [%v]", notConnected)

	return bootstrapConnect(ctx, host, notConnected, peers)
}

// wellknownPeerDialFailure pairs a well-known peer with its dial error so
// failure logs can be emitted after the round completes, once the node's
// actual post-round connectivity is known.
type wellknownPeerDialFailure struct {
	peerID peer.ID
	err    error
}

// connectedWellknownPeersCount returns the number of the given well-known
// peers the host is currently connected to.
func connectedWellknownPeersCount(ph host.Host, peers []peer.AddrInfo) int {
	connectedCount := 0
	for _, p := range peers {
		if ph.Network().Connectedness(p.ID) == network.Connected {
			connectedCount++
		}
	}
	return connectedCount
}

// bootstrapConnect dials the given not-yet-connected well-known peers.
// allPeers is the complete set of well-known peers from the config. Once
// every dial attempt has finished -- even when none of them failed -- the
// node's connectivity to the complete well-known peer set is queried live
// and is the sole source of the round summary and its severity: the summary
// reports how many of the configured well-known peers the node actually
// ends the round not connected to, with dial failures logged only as
// per-peer diagnostic details. WARN severity and a returned error are
// reserved for a node that ends the round connected to zero well-known
// peers (true isolation risk), while partial unreachability on an
// otherwise-connected node is logged at info level. This function owns the
// round summary log -- callers must not log the returned error above debug
// level, or an isolated round would emit duplicate warnings.
func bootstrapConnect(
	ctx context.Context,
	ph host.Host,
	notConnected []peer.AddrInfo,
	allPeers []peer.AddrInfo,
) error {
	if len(notConnected) < 1 {
		return ErrNotEnoughBootstrapPeers
	}

	failures := make(chan wellknownPeerDialFailure, len(notConnected))
	var wg sync.WaitGroup
	for _, p := range notConnected {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.
		// Also, performed asynchronously for dial speed.

		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()

			logger.Debugf("trying to establish connection with well-known peer [%v]", p.ID)

			ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)

			if err := ph.Connect(ctx, p); err != nil {
				failures <- wellknownPeerDialFailure{peerID: p.ID, err: err}
				return
			}

			logger.Debugf("established connection with well-known peer [%v]", p.ID)
		}(p)
	}
	wg.Wait()

	// drain the failures channel, collecting the failed connection attempts.
	close(failures)
	var dialFailures []wellknownPeerDialFailure
	for failure := range failures {
		dialFailures = append(dialFailures, failure)
	}

	// Connectivity is queried live over the complete well-known peer set
	// after every dial batch -- including one with zero dial failures --
	// instead of being derived from dial results and the pre-round snapshot:
	// peers may have connected or dropped concurrently while dials were in
	// flight, and only the node's actual post-round connectivity determines
	// whether unreachable peers are routine churn or an isolation risk.
	connectedCount := connectedWellknownPeersCount(ph, allPeers)
	unreachableCount := len(allPeers) - connectedCount
	isolated := connectedCount == 0

	for _, failure := range dialFailures {
		if isolated {
			logger.Warnf(
				"could not establish connection with well-known peer [%v]: [%v]",
				failure.peerID,
				failure.err,
			)
		} else {
			logger.Debugf(
				"could not establish connection with well-known peer [%v]: [%v]",
				failure.peerID,
				failure.err,
			)
		}
	}

	if unreachableCount == 0 {
		logger.Debugf(
			"connected to all %d well-known peers from config",
			len(allPeers),
		)
		return nil
	}

	if isolated {
		summary := fmt.Sprintf(
			"%d of %d well-known peers unreachable; "+
				"node is not connected to any well-known peer",
			unreachableCount,
			len(allPeers),
		)
		logger.Warn(summary)
		return errors.New(summary)
	}

	logger.Infof(
		"%d of %d well-known peers unreachable; "+
			"connected to %d well-known peer(s)",
		unreachableCount,
		len(allPeers),
		connectedCount,
	)

	return nil
}

type peers []peer.AddrInfo

func (p peers) toPeerInfos() []peer.AddrInfo {
	pinfos := make(map[peer.ID]*peer.AddrInfo)
	for _, bootstrap := range p {
		pinfo, ok := pinfos[bootstrap.ID]
		if !ok {
			pinfo = new(peer.AddrInfo)
			pinfos[bootstrap.ID] = pinfo
			pinfo.ID = bootstrap.ID
		}

		pinfo.Addrs = append(pinfo.Addrs, bootstrap.Addrs...)
	}

	var peers []peer.AddrInfo
	for _, pinfo := range pinfos {
		peers = append(peers, *pinfo)
	}

	return peers
}
