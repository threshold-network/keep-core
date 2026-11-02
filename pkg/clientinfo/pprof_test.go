package clientinfo

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// pprofPaths are the /debug/pprof/* endpoints a remote caller could request
// on the metrics port. profile and trace use a short duration.
var pprofPaths = []string{
	"/debug/pprof/",
	"/debug/pprof/cmdline",
	"/debug/pprof/symbol?lookup=main",
	"/debug/pprof/goroutine?debug=1",
	"/debug/pprof/goroutine?debug=2",
	"/debug/pprof/heap",
	"/debug/pprof/heap?debug=1",
	"/debug/pprof/allocs",
	"/debug/pprof/block",
	"/debug/pprof/mutex",
	"/debug/pprof/profile?seconds=1",
	"/debug/pprof/trace?seconds=1",
}

// TestMetricsServer_DoesNotServePprof verifies that the metrics server started
// by Initialize serves no /debug/pprof/* endpoint. Importing net/http/pprof
// registers them on http.DefaultServeMux, which the metrics server serves,
// in its init(), so this fails if that import comes back anywhere in the
// package's dependencies.
//
// keep-common's EnableServer registers /metrics on http.DefaultServeMux once
// per process, so Initialize must be called only once in this package's
// tests.
func TestMetricsServer_DoesNotServePprof(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := freePort(t)

	if _, ok := Initialize(ctx, Config{Port: port}); !ok {
		t.Fatal("Initialize did not report the clientinfo server enabled")
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForServer(t, base+"/metrics")

	if status := getStatus(t, base+"/metrics"); status != http.StatusOK {
		t.Errorf(
			"GET /metrics: got status %d, want %d",
			status,
			http.StatusOK,
		)
	}

	for _, path := range pprofPaths {
		if status := getStatus(t, base+path); status != http.StatusNotFound {
			t.Errorf(
				"GET %s: got status %d, want %d",
				path,
				status,
				http.StatusNotFound,
			)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func waitForServer(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			response.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("server did not come up at %s", url)
}

func getStatus(t *testing.T, url string) int {
	t.Helper()

	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()

	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatalf("reading response of GET %s: %v", url, err)
	}

	return response.StatusCode
}
