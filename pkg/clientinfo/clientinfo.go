package clientinfo

import (
	"context"
	"net/http"
	"net/http/pprof"
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
		// Register the pprof handlers on http.DefaultServeMux, which is the
		// mux that keep-common's EnableServer hands to the http.Server.
		// Registering them explicitly here avoids the side-effecting blank
		// import of net/http/pprof, which would otherwise register
		// /debug/pprof/* unconditionally on DefaultServeMux regardless of
		// this flag.
		registerPprofHandlers()
		logger.Infof("pprof profiling endpoints enabled at /debug/pprof/")
	}

	registry.EnableServer(cfg.Port)

	return registry, true
}

// registerPprofHandlers registers the standard net/http/pprof handlers on
// http.DefaultServeMux. It is invoked explicitly from Initialize when
// EnablePprof is true, in place of the blank import of net/http/pprof that
// would otherwise register the endpoints at init time.
func registerPprofHandlers() {
	http.HandleFunc("/debug/pprof/", pprof.Index)
	http.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	http.HandleFunc("/debug/pprof/profile", pprof.Profile)
	http.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	http.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
