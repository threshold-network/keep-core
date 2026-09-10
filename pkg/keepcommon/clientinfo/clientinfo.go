// Package clientinfo provides some tools useful for gathering and
// exposing system metrics and diagnostics for external monitoring tools.
//
// Currently, this package is intended to use with Prometheus but
// can be easily extended if needed. Also, not all Prometheus metric
// types are implemented.
//
// Following specifications were used as reference:
// - https://prometheus.io/docs/instrumenting/writing_clientlibs/
// - https://prometheus.io/docs/instrumenting/exposition_formats/
package clientinfo

import (
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ipfs/go-log"
)

var logger = log.Logger("keep-clientinfo")

const readHeaderTimeout = 2 * time.Second

// Registry performs all management of metrics and diagnostics. Specifically, it
// allows registering and exposing them through the HTTP server.
type Registry struct {
	metrics      map[string]metric
	metricsMutex sync.RWMutex

	diagnosticsSources map[string]func() string
	diagnosticsMutex   sync.RWMutex
}

// NewRegistry creates a new client info registry.
func NewRegistry() *Registry {
	return &Registry{
		metrics:            make(map[string]metric),
		diagnosticsSources: make(map[string]func() string),
	}
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
