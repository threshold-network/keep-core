package clientinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClientInfoServerDoesNotExposePprofWhenDisabled asserts that the Go
// profiling endpoints are NOT reachable on the client info port when profiling
// was not enabled.
//
// This is a security regression guard, not a coverage exercise. Importing
// net/http/pprof registers /debug/pprof/* on http.DefaultServeMux from that
// package's init function, whether the import is blank or named. If the client
// info server serves DefaultServeMux, those endpoints are exposed on the client
// info port regardless of Config.EnablePprof, silently defeating an operator
// who explicitly disabled profiling and handing any caller that can reach the
// port goroutine/runtime introspection plus the expensive profile and trace
// endpoints.
//
// The server under test is started by TestMain with profiling disabled.
func TestClientInfoServerDoesNotExposePprofWhenDisabled(t *testing.T) {
	for _, path := range []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile?seconds=1",
		"/debug/pprof/symbol",
		"/debug/pprof/trace?seconds=1",
	} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(
				fmt.Sprintf("http://localhost:%d%s", port, path),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusNotFound {
				t.Errorf(
					"profiling endpoint [%s] is reachable while profiling is disabled\n"+
						"expected status: %d\nactual status:   %d",
					path,
					http.StatusNotFound,
					response.StatusCode,
				)
			}
		})
	}
}

// TestClientInfoServerServesMetricsWhenPprofDisabled asserts the endpoints the
// server exists to serve stay reachable, so the pprof fix cannot regress them.
func TestClientInfoServerServesMetricsWhenPprofDisabled(t *testing.T) {
	for _, path := range []string{"/metrics", "/diagnostics"} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(
				fmt.Sprintf("http://localhost:%d%s", port, path),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusOK {
				t.Errorf(
					"expected [%s] to be reachable\nexpected status: %d\nactual status:   %d",
					path,
					http.StatusOK,
					response.StatusCode,
				)
			}
		})
	}
}

// TestNewServeMux_PprofEnabled asserts the flag remains functional in the
// other direction: opting into profiling must actually serve the endpoints.
// Without this, the disabled-path test above could be satisfied by never
// registering the handlers at all.
func TestNewServeMux_PprofEnabled(t *testing.T) {
	handler := newRegistry(context.Background()).newServeMux(true)

	for _, path := range []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile?seconds=1",
		"/debug/pprof/symbol",
		"/debug/pprof/trace?seconds=1",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(
				recorder,
				httptest.NewRequest(http.MethodGet, path, nil),
			)

			if recorder.Code != http.StatusOK {
				t.Errorf(
					"expected [%s] to be served when profiling is enabled\n"+
						"expected status: %d\nactual status:   %d",
					path,
					http.StatusOK,
					recorder.Code,
				)
			}
		})
	}
}

// TestNewServeMux_PprofDisabledIsIsolatedFromDefaultServeMux asserts the
// mux is isolated from http.DefaultServeMux, which is where importing
// net/http/pprof installs its handlers. This is the root cause the live-server
// test above observes, asserted directly against the constructed mux.
func TestNewServeMux_PprofDisabledIsIsolatedFromDefaultServeMux(t *testing.T) {
	handler := newRegistry(context.Background()).newServeMux(false)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil),
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf(
			"handler is serving http.DefaultServeMux, exposing profiling "+
				"endpoints while disabled\nexpected status: %d\nactual status:   %d",
			http.StatusNotFound,
			recorder.Code,
		)
	}
}
