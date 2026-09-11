package electrum

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/checksum0/go-electrum/electrum"
)

// Unused RPC methods remain embedded so a test fails if it unexpectedly invokes
// a different operation. Lifecycle state is synchronized with the keepalive loop.
type failoverTestClient struct {
	electrumClient
	stopped        atomic.Bool
	versions       atomic.Int32
	shutdowns      atomic.Int32
	abortOnce      sync.Once
	versionErr     error
	version        func(context.Context) error
	shutdown       func()
	header         func(context.Context) (*electrum.SubscribeHeadersResult, error)
	ping           func(context.Context) error
	fee            func(context.Context) (float32, error)
	serverFeatures func(context.Context) (*electrum.ServerFeaturesResult, error)
}

func (c *failoverTestClient) ServerVersion(ctx context.Context) (string, string, error) {
	c.versions.Add(1)
	if c.version != nil {
		return "test", "1.4", c.version(ctx)
	}
	return "test", "1.4", c.versionErr
}
func (c *failoverTestClient) Shutdown() {
	c.shutdowns.Add(1)
	c.stopped.Store(true)
	if c.shutdown != nil {
		c.shutdown()
	}
}
func (c *failoverTestClient) IsShutdown() bool { return c.stopped.Load() }

func (c *failoverTestClient) Abort() {
	c.abortOnce.Do(func() {
		if !c.IsShutdown() {
			c.Shutdown()
		}
	})
}

func (c *failoverTestClient) awaitShutdown(t *testing.T) {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for c.shutdowns.Load() == 0 {
		select {
		case <-tick.C:
		case <-timeout.C:
			t.Fatal("retired client was not closed")
		}
	}
	if c.shutdowns.Load() != 1 {
		t.Fatal("retired client was closed more than once")
	}
}
func (c *failoverTestClient) SubscribeHeadersSingle(ctx context.Context) (*electrum.SubscribeHeadersResult, error) {
	if c.header != nil {
		return c.header(ctx)
	}
	return &electrum.SubscribeHeadersResult{Height: 42}, nil
}
func (c *failoverTestClient) Ping(ctx context.Context) error {
	if c.ping != nil {
		return c.ping(ctx)
	}
	return nil
}
func (c *failoverTestClient) GetFee(ctx context.Context, _ uint32) (float32, error) {
	if c.fee != nil {
		return c.fee(ctx)
	}
	return 0.001, nil
}
func (c *failoverTestClient) ServerFeatures(ctx context.Context) (*electrum.ServerFeaturesResult, error) {
	if c.serverFeatures != nil {
		return c.serverFeatures(ctx)
	}
	return &electrum.ServerFeaturesResult{}, nil
}

func failoverTestConfig() Config {
	return Config{
		URL: "first", FallbackURLs: []string{"second"},
		ConnectTimeout: 100 * time.Millisecond, ConnectRetryTimeout: 5 * time.Second,
		RequestTimeout: 20 * time.Millisecond, RequestRetryTimeout: 5 * time.Second,
		KeepAliveInterval: time.Hour,
	}
}

func TestConnectFailover(t *testing.T) {
	for _, handshakeFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "dial failure", true: "handshake failure"}[handshakeFailure], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := &failoverTestClient{versionErr: errors.New("server unavailable")}
			second := new(failoverTestClient)
			config := failoverTestConfig()
			config.FallbackURLs = []string{"first", "", "second", "second"}
			connection, err := connect(ctx, config, func(
				_ context.Context,
				url string,
			) (electrumClient, error) {
				if url == "first" {
					if handshakeFailure {
						return first, nil
					}
					return nil, errors.New("dial failed")
				}
				return second, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(connection.serverURLs) != 2 || connection.client != second {
				t.Fatal("did not select the fallback server")
			}
			if second.versions.Load() != 1 {
				t.Fatal("fallback server was not verified")
			}
			if handshakeFailure {
				first.awaitShutdown(t)
			}
		})
	}
}

func TestRequestFailover(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mkErr        func(ctx context.Context) error
		preShutdown  bool
		wantFailover bool
	}{
		{
			name: "transport timeout",
			mkErr: func(ctx context.Context) error {
				<-ctx.Done()
				return electrum.ErrTimeout
			},
			wantFailover: true,
		},
		{
			name:         "server shutdown",
			preShutdown:  true,
			wantFailover: true,
		},
		{
			name: "benign JSON-RPC application error",
			mkErr: func(context.Context) error {
				return errors.New("errNo: -32600, errMsg: tx not found")
			},
			wantFailover: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := &failoverTestClient{header: func(ctx context.Context) (*electrum.SubscribeHeadersResult, error) {
				if tc.mkErr == nil {
					t.Error("shutdown case must not issue a request against the retired client")
					return nil, errors.New("unreachable")
				}
				return nil, tc.mkErr(ctx)
			}}
			second := new(failoverTestClient)
			config := failoverTestConfig()
			if !tc.wantFailover {
				// A benign application error never fails over, so the
				// retry loop keeps hammering the same (still healthy)
				// server; keep the budget short so the test stays fast.
				config.RequestRetryTimeout = 60 * time.Millisecond
			}
			connection, err := connect(ctx, config, func(
				_ context.Context,
				url string,
			) (electrumClient, error) {
				if url == "first" {
					return first, nil
				}
				return second, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.preShutdown {
				first.Shutdown()
			}

			if !tc.wantFailover {
				if _, err := connection.GetLatestBlockHeight(); err == nil {
					t.Fatal("expected the benign application error to surface")
				}
				if connection.client != first || first.IsShutdown() || second.versions.Load() != 0 {
					t.Fatal("a benign JSON-RPC application error incorrectly triggered failover")
				}
				return
			}

			height, err := connection.GetLatestBlockHeight()
			if err != nil || height != 42 {
				t.Fatalf("fallback request: %d, %v", height, err)
			}
			first.awaitShutdown(t)
			if second.versions.Load() != 1 {
				t.Fatal("connection lifecycle was not preserved")
			}
		})
	}
}

func TestExplicitElectrumURLRemainsPinned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &failoverTestClient{header: func(context.Context) (*electrum.SubscribeHeadersResult, error) {
		return nil, errors.New("temporary RPC error")
	}}
	config := failoverTestConfig()
	config.FallbackURLs = nil
	config.RequestRetryTimeout = 30 * time.Millisecond
	var connections atomic.Int32
	connection, err := connect(ctx, config, func(
		_ context.Context,
		url string,
	) (electrumClient, error) {
		if url != config.URL {
			t.Errorf("unexpected URL %q", url)
		}
		connections.Add(1)
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.GetLatestBlockHeight(); err == nil {
		t.Fatal("expected request failure")
	}
	if connection.client != client || client.IsShutdown() || connections.Load() != 1 {
		t.Fatal("RPC failure changed the explicit connection")
	}
}

func TestCallerCancellationDoesNotFailover(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	caller, cancelCaller := context.WithCancel(parent)
	defer cancelCaller()
	first := new(failoverTestClient)
	var connections atomic.Int32
	connection, err := connect(parent, failoverTestConfig(), func(context.Context, string) (electrumClient, error) {
		connections.Add(1)
		return first, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = requestWithRetry(caller, connection, func(context.Context, electrumClient) (int, error) {
		cancelCaller()
		return 0, context.Canceled
	}, "cancelled")
	if err == nil {
		t.Fatal("expected cancellation")
	}
	if connection.client != first || connection.serverIndex != 0 ||
		first.IsShutdown() || connections.Load() != 1 {
		t.Fatal("caller cancellation changed server health")
	}
}

func TestFailoverHonorsRequestBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.RequestRetryTimeout = 60 * time.Millisecond
	config.ConnectTimeout = time.Second
	first := new(failoverTestClient)
	connection, err := connect(ctx, config, func(ctx context.Context, url string) (electrumClient, error) {
		if url == "first" {
			return first, nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	first.Shutdown()
	start := time.Now()
	if _, err := connection.GetLatestBlockHeight(); err == nil {
		t.Fatal("expected unavailable server error")
	}
	if elapsed := time.Since(start); elapsed > 1000*time.Millisecond {
		t.Fatalf("reconnection exceeded request budget: %v", elapsed)
	}
}

func TestKeepAliveFailover(t *testing.T) {
	t.Run("multiple servers", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		pong := make(chan struct{}, 1)
		first := &failoverTestClient{ping: func(context.Context) error { return electrum.ErrTimeout }}
		second := &failoverTestClient{ping: func(context.Context) error {
			select {
			case pong <- struct{}{}:
			default:
			}
			return nil
		}}
		config := failoverTestConfig()
		config.KeepAliveInterval = time.Millisecond
		_, err := connect(ctx, config, func(
			_ context.Context,
			url string,
		) (electrumClient, error) {
			if url == "first" {
				return first, nil
			}
			return second, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-pong:
		case <-time.After(5 * time.Second):
			t.Fatal("keepalive did not reach the fallback server")
		}
		first.awaitShutdown(t)
	})

	t.Run("pinned single server recovers via retire and redial", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		config := failoverTestConfig()
		config.FallbackURLs = nil
		config.KeepAliveInterval = time.Millisecond
		var dials atomic.Int32
		var healthy atomic.Bool
		pong := make(chan struct{}, 1)
		var clientsMu sync.Mutex
		var clients []*failoverTestClient
		connection, err := connect(ctx, config, func(_ context.Context, url string) (electrumClient, error) {
			dials.Add(1)
			client := &failoverTestClient{ping: func(context.Context) error {
				if !healthy.Load() {
					return electrum.ErrTimeout
				}
				select {
				case pong <- struct{}{}:
				default:
				}
				return nil
			}}
			clientsMu.Lock()
			clients = append(clients, client)
			clientsMu.Unlock()
			return client, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		clientsMu.Lock()
		initial := clients[0]
		clientsMu.Unlock()
		// A pinned single-URL connection must still retire and re-dial the
		// same server on a failed ping, instead of wedging on the dead
		// client forever.
		initial.awaitShutdown(t)
		healthy.Store(true)
		select {
		case <-pong:
		case <-time.After(5 * time.Second):
			t.Fatal("keepalive did not recover the pinned server after a failed ping")
		}
		if dials.Load() < 2 {
			t.Fatal("pinned server was not re-dialed after the failed ping")
		}
		if connection.serverIndex != 0 {
			t.Fatal("pinned single-URL connection must stay at index 0")
		}
	})
}

// TestKeepAliveHealsPrimaryAfterConsecutivePings verifies the D3 healing
// behavior: after healAfterConsecutivePings consecutive successful keepalive
// pings on a non-primary index, the connection actively re-probes the
// primary and swaps back to it once verified healthy.
func TestKeepAliveHealsPrimaryAfterConsecutivePings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.KeepAliveInterval = time.Millisecond
	firstAttemptFailed := &failoverTestClient{versionErr: errors.New("primary temporarily unavailable")}
	second := new(failoverTestClient)
	primaryHealthy := new(failoverTestClient)
	var primaryDials atomic.Int32
	connection, err := connect(ctx, config, func(_ context.Context, url string) (electrumClient, error) {
		if url == "first" {
			if primaryDials.Add(1) == 1 {
				return firstAttemptFailed, nil
			}
			return primaryHealthy, nil
		}
		return second, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.serverIndex != 1 || connection.client != second {
		t.Fatal("setup did not start on the fallback server")
	}
	firstAttemptFailed.awaitShutdown(t)

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	healed := false
	for !healed {
		select {
		case <-tick.C:
			connection.clientMutex.Lock()
			healed = connection.serverIndex == 0 && connection.client == primaryHealthy
			connection.clientMutex.Unlock()
		case <-timeout.C:
			t.Fatal("keepalive did not re-canonicalize the primary server")
		}
	}
	if primaryHealthy.versions.Load() != 1 {
		t.Fatal("re-canonicalized primary was not verified")
	}
	second.awaitShutdown(t)
}

func TestConcurrentRequestsShareFailover(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := &failoverTestClient{header: func(context.Context) (*electrum.SubscribeHeadersResult, error) {
		return nil, electrum.ErrTimeout
	}}
	second := new(failoverTestClient)
	var fallbackConnections atomic.Int32
	connection, err := connect(ctx, failoverTestConfig(), func(
		_ context.Context,
		url string,
	) (electrumClient, error) {
		if url == "first" {
			return first, nil
		}
		fallbackConnections.Add(1)
		return second, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var requests sync.WaitGroup
	for i := 0; i < 8; i++ {
		requests.Add(1)
		go func() {
			defer requests.Done()
			if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
				t.Errorf("concurrent request: %d, %v", height, err)
			}
		}()
	}
	requests.Wait()
	if fallbackConnections.Load() != 1 {
		t.Fatal("concurrent requests replaced a healthy fallback")
	}
}

func TestFeeApplicationErrorDoesNotFailover(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "no fee data text", err: errors.New("cannot estimate fee")},
		{name: "-32603 internal error", err: errors.New("errNo: -32603, errMsg: internal error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &failoverTestClient{fee: func(context.Context) (float32, error) {
				return 0, tc.err
			}}
			connection, err := connect(ctx, failoverTestConfig(), func(context.Context, string) (electrumClient, error) {
				return client, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := connection.getFeeBtcPerKbOnce(ctx, 1); err == nil {
				t.Fatal("expected unavailable fee estimate")
			}
			if connection.client != client || connection.serverIndex != 0 || client.IsShutdown() {
				t.Fatal("a JSON-RPC application error incorrectly marked the server unhealthy")
			}
		})
	}
}

func TestConnectFailureReturnsNilChain(t *testing.T) {
	chain, err := Connect(context.Background(), Config{
		URL:                 "unsupported://server",
		ConnectRetryTimeout: time.Millisecond,
	})
	if err == nil || chain != nil {
		t.Fatalf("expected nil chain and error, got %v, %v", chain, err)
	}
}

func TestFeeRequestFailover(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := &failoverTestClient{fee: func(context.Context) (float32, error) {
		return 0, electrum.ErrServerShutdown
	}}
	second := new(failoverTestClient)
	connection, err := connect(ctx, failoverTestConfig(), func(
		_ context.Context,
		url string,
	) (electrumClient, error) {
		if url == "first" {
			return first, nil
		}
		return second, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.getFeeBtcPerKbOnce(ctx, 1); err == nil {
		t.Fatal("expected transport error")
	}
	if fee, err := connection.getFeeBtcPerKbOnce(ctx, 6); err != nil || fee != 0.001 {
		t.Fatalf("fee request on fallback: %v, %v", fee, err)
	}
	first.awaitShutdown(t)
	if second.versions.Load() != 1 {
		t.Fatal("fee request did not switch servers")
	}
}

func TestAllElectrumServersUnavailable(t *testing.T) {
	config := failoverTestConfig()
	config.ConnectRetryTimeout = 1500 * time.Millisecond
	var attempts atomic.Int32
	connection, err := connect(context.Background(), config, func(context.Context, string) (electrumClient, error) {
		attempts.Add(1)
		return nil, errors.New("unavailable")
	})
	if err == nil || connection != nil || attempts.Load() != 2 {
		t.Fatalf("expected bounded retries across both servers, got %v, %v, %d attempts",
			connection, err, attempts.Load())
	}
}

func TestConnectFailsWithNoConfiguredServers(t *testing.T) {
	connection, err := connect(context.Background(), Config{}, func(context.Context, string) (electrumClient, error) {
		t.Fatal("dial must not be attempted with no configured server URLs")
		return nil, nil
	})
	if err == nil || connection != nil {
		t.Fatalf("expected an error and a nil connection, got %v, %v", connection, err)
	}
}
