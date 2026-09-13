// Package clientinfo provides tools for gathering and exposing system
// metrics and diagnostics for external monitoring tools.
//
// Currently, this package is intended to use with Prometheus but can be
// easily extended if needed. Also, not all Prometheus metric types are
// implemented.
//
// Following specifications were used as reference:
// - https://prometheus.io/docs/instrumenting/writing_clientlibs/
// - https://prometheus.io/docs/instrumenting/exposition_formats/
package clientinfo

import (
	"context"
	"io"
	"net/http"
	"net/http/pprof"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ipfs/go-log"
)

var logger = log.Logger("keep-clientinfo")

const readHeaderTimeout = 2 * time.Second

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

// Registry performs all management of metrics and diagnostics. Specifically,
// it allows registering and exposing them through the HTTP server, and
// exposes additional functions for registering client-custom metrics and
// diagnostics.
type Registry struct {
	ctx context.Context

	metrics      map[string]metric
	metricsMutex sync.RWMutex

	diagnosticsSources map[string]func() string
	diagnosticsMutex   sync.RWMutex
}

// newRegistry creates a new client info registry bound to ctx.
func newRegistry(ctx context.Context) *Registry {
	return &Registry{
		ctx:                ctx,
		metrics:            make(map[string]metric),
		diagnosticsSources: make(map[string]func() string),
	}
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

	registry := newRegistry(ctx)

	if cfg.EnablePprof {
		// Register the pprof handlers on http.DefaultServeMux, which is the
		// mux that EnableServer hands to the http.Server. Registering them
		// explicitly here avoids the side-effecting blank import of
		// net/http/pprof, which would otherwise register /debug/pprof/*
		// unconditionally on DefaultServeMux regardless of this flag.
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

// EnableServer enables the client info server on the given port. Data will
// be exposed on `/metrics` and `/diagnostics` paths.
func (r *Registry) EnableServer(port int) {
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	http.HandleFunc("/metrics", func(response http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(response, r.exposeMetrics()); err != nil {
			logger.Errorf("could not write response: [%v]", err)
		}
	})

	http.HandleFunc("/diagnostics", func(response http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(response, r.exposeDiagnostics()); err != nil {
			logger.Errorf("could not write response: [%v]", err)
		}
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Errorf("client info server error: [%v]", err)
		}
	}()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
