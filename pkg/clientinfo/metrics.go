package clientinfo

import (
	"fmt"
	"strings"
	"time"

	"github.com/keep-network/keep-common/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/net"
)

type Source func() float64

// Names under which metrics are exposed.
const (
	ConnectedPeersCountMetricName          = "connected_peers_count"
	ConnectedWellknownPeersCountMetricName = "connected_wellknown_peers_count"
	EthConnectivityMetricName              = "eth_connectivity"
	BtcConnectivityMetricName              = "btc_connectivity"
	ClientInfoMetricName                   = "client_info"
)

const (
	// DefaultNetworkMetricsTick is the default duration of the
	// observation tick for network metrics.
	DefaultNetworkMetricsTick = 1 * time.Minute
	// DefaultEthereumMetricsTick is the default duration of the
	// observation tick for Ethereum metrics.
	DefaultEthereumMetricsTick = 10 * time.Minute
	// DefaultBitcoinMetricsTick is the default duration of the
	// observation tick for Bitcoin metrics.
	DefaultBitcoinMetricsTick = 10 * time.Minute
	// The duration of the observation tick for all application-specific
	// metrics.
	ApplicationMetricsTick = 1 * time.Minute
)

// ObserveConnectedPeersCount triggers an observation process of the
// connected_peers_count metric.
func (r *Registry) ObserveConnectedPeersCount(
	netProvider net.Provider,
	tick time.Duration,
) {
	input := func() float64 {
		connectedPeers := netProvider.ConnectionManager().ConnectedPeers()
		return float64(len(connectedPeers))
	}

	r.observe(
		ConnectedPeersCountMetricName,
		input,
		validateTick(tick, DefaultNetworkMetricsTick),
	)
}

// ObserveConnectedWellknownPeersCount triggers an observation process of the
// connected_wellknown_peers_count metric.
func (r *Registry) ObserveConnectedWellknownPeersCount(
	netProvider net.Provider,
	wellknownPeers []string,
	tick time.Duration,
) {
	input := func() float64 {
		currentCount := 0

		for _, address := range wellknownPeers {
			if netProvider.ConnectionManager().IsConnected(address) {
				currentCount++
			}
		}

		return float64(currentCount)
	}

	r.observe(
		ConnectedWellknownPeersCountMetricName,
		input,
		validateTick(tick, DefaultNetworkMetricsTick),
	)
}

// ObserveEthConnectivity triggers an observation process of the
// eth_connectivity metric.
func (r *Registry) ObserveEthConnectivity(
	blockCounter chain.BlockCounter,
	tick time.Duration,
) {
	input := func() float64 {
		_, err := blockCounter.CurrentBlock()
		if err != nil {
			return 0
		}

		return 1
	}

	r.observe(
		EthConnectivityMetricName,
		input,
		validateTick(tick, DefaultEthereumMetricsTick),
	)
}

// ObserveBtcConnectivity triggers an observation process of the
// btc_connectivity metric.
func (r *Registry) ObserveBtcConnectivity(
	btcChain bitcoin.Chain,
	tick time.Duration,
) {
	input := func() float64 {
		_, err := btcChain.GetLatestBlockHeight()
		if err != nil {
			return 0
		}

		return 1
	}

	r.observe(
		BtcConnectivityMetricName,
		input,
		validateTick(tick, DefaultBitcoinMetricsTick),
	)
}

// ObserveApplicationSource triggers an observation process of
// application-specific metrics.
func (r *Registry) ObserveApplicationSource(
	application string,
	inputs map[string]Source,
) {
	for k, v := range inputs {
		r.observe(
			fmt.Sprintf("%s_%s", application, k),
			v,
			ApplicationMetricsTick,
		)
	}
}

// RegisterMetricClientInfo registers static client information labels for metrics.
func (r *Registry) RegisterMetricClientInfo(version string) {
	_, err := r.NewMetricInfo(
		ClientInfoMetricName,
		[]clientinfo.Label{
			clientinfo.NewLabel("version", version),
		},
	)
	if err != nil {
		logger.Warnf("could not register metric client info: [%v]", err)
	}
}

func (r *Registry) observe(
	name string,
	input Source,
	tick time.Duration,
) {
	observer, err := r.NewMetricGaugeObserver(name, clientinfo.MetricObserverInput(input))
	if err != nil {
		// Check if the error is due to metric already existing (expected in some cases)
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") {
			// Metric already registered, this is expected if registerAllMetrics is called multiple times
			// or if the same metric is registered in multiple places. Log at debug level.
			logger.Debugf("metric [%v] already registered, skipping duplicate registration: %v", name, err)
			return
		}
		// For other errors, log as warning
		logger.Warnf("could not create gauge observer [%v]: %v", name, err)
		return
	}

	observer.Observe(r.ctx, tick)

	logger.Infof("observing %s with [%s] tick", name, tick)
}

func validateTick(tick time.Duration, defaultTick time.Duration) time.Duration {
	if tick > 0 {
		return tick
	}

	return defaultTick
}
