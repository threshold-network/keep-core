package clientinfo

import (
	"context"
	_ "net/http/pprof" // #nosec G108 -- opt-in profiling; registered on DefaultServeMux intentionally
	"time"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/clientinfo"
)

var logger = log.Logger("keep-clientinfo")

// Config stores configuration for the client info.
type Config struct {
	Port                   int
	NetworkMetricsTick     time.Duration
	EthereumMetricsTick    time.Duration
	BitcoinMetricsTick     time.Duration
	RPCHealthCheckInterval time.Duration
	// EnablePprof exposes Go runtime profiling endpoints at /debug/pprof/ on
	// the clientinfo port. Requires Port != 0. Never expose to untrusted
	// networks; bind behind a firewall or restrict with an SSH tunnel.
	EnablePprof bool
}

// Registry wraps keep-common clientinfo registry and exposes additional
// functions for registering client-custom metrics and diagnostics
type Registry struct {
	*clientinfo.Registry

	ctx context.Context
}

// Initialize set up the client info registry and enables metrics and
// diagnostics server.
func Initialize(
	ctx context.Context,
	cfg Config,
) (*Registry, bool) {
	if cfg.Port == 0 {
		return nil, false
	}

	registry := &Registry{clientinfo.NewRegistry(), ctx}

	if cfg.EnablePprof {
		logger.Infof("pprof profiling endpoints enabled at /debug/pprof/")
	}

	registry.EnableServer(cfg.Port)

	return registry, true
}
