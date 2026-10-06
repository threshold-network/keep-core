package electrum

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/checksum0/go-electrum/electrum"
)

func TestRequestRetryBudgetExpiryFailsOver(t *testing.T) {
	for _, requestTimeout := range []time.Duration{50 * time.Millisecond, time.Second} {
		t.Run(requestTimeout.String(), func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			config := failoverTestConfig()
			config.RequestRetryTimeout = 50 * time.Millisecond
			config.RequestTimeout = requestTimeout
			var firstRequests atomic.Int32
			first := &failoverTestClient{header: func(ctx context.Context) (*electrum.SubscribeHeadersResult, error) {
				firstRequests.Add(1)
				<-ctx.Done()
				return nil, ctx.Err()
			}}
			second := new(failoverTestClient)
			connection, err := connect(parent, config, func(_ context.Context, url string) (electrumClient, error) {
				if url == "first" {
					return first, nil
				}
				return second, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := connection.GetLatestBlockHeight(); err == nil {
				t.Fatal("expected the first request to exhaust its retry budget")
			}
			if parent.Err() != nil {
				t.Fatal("the caller must remain active after the internal timeout")
			}
			if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
				t.Fatalf("request after budget expiry did not reach the fallback: %d, %v", height, err)
			}
			if firstRequests.Load() != 1 || second.versions.Load() != 1 {
				t.Fatal("a later request reused the unresponsive server")
			}
			first.awaitShutdown(t)
		})
	}
}

func TestCallerDeadlineDoesNotFailover(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	first := new(failoverTestClient)
	config := failoverTestConfig()
	config.RequestTimeout = time.Second
	connection, err := connect(parent, config, func(context.Context, string) (electrumClient, error) {
		return first, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	caller, cancelCaller := context.WithTimeout(parent, 100*time.Millisecond)
	defer cancelCaller()
	_, err = requestWithRetry(caller, connection, func(ctx context.Context, _ electrumClient) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}, "caller deadline")
	if err == nil || !errors.Is(caller.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected caller deadline expiry, got %v", err)
	}
	// The request's own abort watcher (electrum.go requestWithRetry) may
	// still retire the blocked transport once requestCtx inherits the
	// caller's expired deadline - that is a transport-level cleanup, not a
	// failover decision. What must not happen is the connection treating
	// this as a server failure: no failover, same client, same index.
	if connection.client != first || connection.serverIndex != 0 {
		t.Fatal("caller deadline expiry changed server health")
	}
}

// A transport close handshake can outlive any of the request contexts. Keep
// cleanup blocked until the test has checked the caller's result and, where
// applicable, a subsequent request. Always release and join cleanup afterward.
func blockClientShutdown(t *testing.T, client *failoverTestClient) <-chan struct{} {
	t.Helper()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	client.shutdown = func() {
		close(started)
		<-release
		close(done)
	}
	t.Cleanup(func() {
		close(release)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("retired client cleanup did not finish")
		}
		if client.shutdowns.Load() != 1 {
			t.Error("retired client must be closed exactly once")
		}
	})
	return started
}

func awaitClientResult[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(time.Second):
		t.Fatal("operation waited for retired-client cleanup")
		var zero T
		return zero
	}
}

func TestVerificationCleanupHonorsDeadline(t *testing.T) {
	for _, callerDeadline := range []bool{false, true} {
		name := "connection retry budget"
		if callerDeadline {
			name = "caller deadline"
		}
		t.Run(name, func(t *testing.T) {
			config := failoverTestConfig()
			config.ConnectTimeout = time.Second
			config.RequestTimeout = time.Second
			config.ConnectRetryTimeout = 100 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if callerDeadline {
				var cancelDeadline context.CancelFunc
				ctx, cancelDeadline = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancelDeadline()
				config.ConnectRetryTimeout = time.Second
			}
			client := &failoverTestClient{version: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			}}
			closing := blockClientShutdown(t, client)
			result := make(chan error, 1)
			go func() {
				_, err := connect(ctx, config, func(context.Context, string) (electrumClient, error) {
					return client, nil
				})
				result <- err
			}()
			awaitClientResult(t, closing)
			if err := awaitClientResult(t, result); err == nil {
				t.Fatal("expected verification to exhaust its deadline")
			}
		})
	}
}

// TestReconnectVerificationCleanupDoesNotBlockRequests verifies that a
// verification abandoned due to retry budget exhaustion (not candidate
// health, see electrumConnect) does not block a later reconnection from
// retrying and using that same preserved candidate, even while its earlier
// abandoned client is still stuck mid-shutdown.
func TestReconnectVerificationCleanupDoesNotBlockRequests(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.FallbackURLs = []string{"second"}
	config.RequestRetryTimeout = 50 * time.Millisecond
	config.ConnectTimeout = time.Second
	config.RequestTimeout = time.Second
	first := new(failoverTestClient)
	var verifyCalls atomic.Int32
	second := &failoverTestClient{version: func(ctx context.Context) error {
		if verifyCalls.Add(1) == 1 {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	}}
	closing := blockClientShutdown(t, second)
	connection, err := connect(parent, config, func(_ context.Context, url string) (electrumClient, error) {
		if url == "first" {
			return first, nil
		}
		return second, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first.Shutdown()
	result := make(chan error, 1)
	go func() {
		_, err := connection.GetLatestBlockHeight()
		result <- err
	}()
	awaitClientResult(t, closing)
	if err := awaitClientResult(t, result); err == nil {
		t.Fatal("expected the retry budget to be exhausted mid-verification")
	}
	if connection.serverIndex != 1 || connection.client != nil {
		t.Fatal("budget exhaustion incorrectly discarded the preserved candidate")
	}
	// second's Abort/Shutdown from the abandoned attempt remains blocked
	// (released only in t.Cleanup); a later reconnection with a workable
	// budget must still be able to retry and use the preserved candidate,
	// proving the blocked cleanup did not wedge the connection.
	connection.config.RequestRetryTimeout = 2 * time.Second
	if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
		t.Fatalf("blocked verification cleanup prevented retrying the preserved candidate: %d, %v", height, err)
	}
	if verifyCalls.Load() != 2 {
		t.Fatal("preserved candidate was not retried")
	}
}

func TestFailoverCleanupDoesNotBlockRequests(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.RequestRetryTimeout = 50 * time.Millisecond
	first := &failoverTestClient{header: func(context.Context) (*electrum.SubscribeHeadersResult, error) {
		return nil, electrum.ErrServerShutdown
	}}
	second := new(failoverTestClient)
	closing := blockClientShutdown(t, first)
	connection, err := connect(parent, config, func(_ context.Context, url string) (electrumClient, error) {
		if url == "first" {
			return first, nil
		}
		return second, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	failedRequest := make(chan error, 1)
	go func() {
		_, err := connection.GetLatestBlockHeight()
		failedRequest <- err
	}()
	awaitClientResult(t, closing)
	nextRequest := make(chan error, 1)
	go func() {
		_, err := connection.GetLatestBlockHeight()
		nextRequest <- err
	}()
	if err := awaitClientResult(t, nextRequest); err != nil {
		t.Fatalf("concurrent request could not use the fallback during cleanup: %v", err)
	}
	if err := awaitClientResult(t, failedRequest); err == nil {
		t.Fatal("expected the failed request to exhaust its retry budget")
	}
}

func TestConnectionShutdownDoesNotBlockRequests(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := new(failoverTestClient)
	closing := blockClientShutdown(t, client)
	connection, err := connect(parent, failoverTestConfig(), func(context.Context, string) (electrumClient, error) {
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	awaitClientResult(t, closing)
	caller, cancelCaller := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelCaller()
	result := make(chan error, 1)
	go func() {
		_, err := requestWithRetry(caller, connection, func(context.Context, electrumClient) (int, error) {
			t.Error("request used a closed connection")
			return 0, nil
		}, "shutdown")
		result <- err
	}()
	if err := awaitClientResult(t, result); err == nil {
		t.Fatal("expected connection cancellation")
	}
}
