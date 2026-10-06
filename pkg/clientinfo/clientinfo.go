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
//
// pprof handlers are intentionally served from a private ServeMux built
// per-Registry so the EnablePprof flag fully controls their reachability.
// Importing net/http/pprof also registers those handlers on
// http.DefaultServeMux at init time as a side effect; nothing in this
// package serves DefaultServeMux, so the registration is currently
// dormant. Do not add http.ListenAndServe(":port", nil) (or any other
// nil-handler Listen call) to this package or its consumers: that would
// expose pprof on the new listener regardless of the EnablePprof flag.
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
		if cfg.EnablePprof {
			// Enabling pprof without a port would be silently ignored because
			// no server is started; surface the misconfiguration instead.
			logger.Warnf(
				"EnablePprof is set but Port is 0; no server is started and " +
					"profiling endpoints will not be exposed",
			)
		}
		return nil, false
	}

	registry := newRegistry(ctx)

	registry.enableServer(cfg.Port, cfg.EnablePprof)

	return registry, true
}

// registerPprofHandlers registers the standard net/http/pprof handlers on
// the provided ServeMux when EnablePprof is true. See the package doc for
// the dormant DefaultServeMux side-effect warning.
func registerPprofHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// newServeMux builds the HTTP handler served on the client info port.
//
// The mux is created per call and never shared with http.DefaultServeMux.
// That isolation is load-bearing for two reasons: importing net/http/pprof
// registers /debug/pprof/* on DefaultServeMux from that package's init
// regardless of how it is imported, so serving DefaultServeMux would expose
// profiling endpoints even when disabled; and registering this registry's own
// routes on a process-global mux makes a second registry panic on duplicate
// patterns.
func (r *Registry) newServeMux(enablePprof bool) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(response http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(response, r.exposeMetrics()); err != nil {
			logger.Errorf("could not write response: [%v]", err)
		}
	})

	mux.HandleFunc("/diagnostics", func(response http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(response, r.exposeDiagnostics()); err != nil {
			logger.Errorf("could not write response: [%v]", err)
		}
	})

	if enablePprof {
		registerPprofHandlers(mux)
		logger.Infof("pprof profiling endpoints enabled at /debug/pprof/")
	}

	return mux
}

func (r *Registry) newServer(port int, enablePprof bool) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           r.newServeMux(enablePprof),
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

// enableServer starts the client info HTTP server, exposing the profiling
// endpoints only when enablePprof is true.
func (r *Registry) enableServer(port int, enablePprof bool) {
	server := r.newServer(port, enablePprof)

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Errorf("client info server error: [%v]", err)
		}
	}()
}

// EnableServer enables the client info server on the given port. Data will
// be exposed on `/metrics` and `/diagnostics` paths. pprof profiling
// endpoints are NOT enabled by this method; prefer Initialize, which
// threads the EnablePprof flag from the caller-supplied Config and never
// exposes pprof unless explicitly requested.
//
// Deprecated: prefer Initialize; this method exists for backwards
// compatibility but always disables pprof. The previous behavior of
// implicitly exposing pprof via http.DefaultServeMux is no longer relied on.
func (r *Registry) EnableServer(port int) {
	r.enableServer(port, false)
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
