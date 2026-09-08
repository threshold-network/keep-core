package electrum

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/checksum0/go-electrum/electrum"
)

// Like a transport write without a deadline, call ignores its context and only
// returns when the client is shut down. release also lets failing tests join
// their blocked goroutines without relying on the behavior under test.
type blockedClientWrite struct {
	started  chan struct{}
	finished chan struct{}
	unblock  chan struct{}
	release  func()
}

func newBlockedClientWrite(client *failoverTestClient) *blockedClientWrite {
	write := &blockedClientWrite{
		started:  make(chan struct{}),
		finished: make(chan struct{}),
		unblock:  make(chan struct{}),
	}
	write.release = sync.OnceFunc(func() { close(write.unblock) })
	client.shutdown = write.release
	return write
}

func (w *blockedClientWrite) call(context.Context) error {
	close(w.started)
	defer close(w.finished)
	<-w.unblock
	return io.ErrClosedPipe
}

func TestConnectionCancellationUnblocksTransportWrite(t *testing.T) {
	for _, operation := range []string{"request", "fee", "keepalive"} {
		for _, reconnect := range []bool{false, true} {
			name := operation + "/initial client"
			if reconnect {
				name = operation + "/replacement client"
			}
			t.Run(name, func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				defer cancel()
				first := new(failoverTestClient)
				client := new(failoverTestClient)
				write := newBlockedClientWrite(client)
				config := failoverTestConfig()
				switch operation {
				case "request":
					client.header = func(ctx context.Context) (*electrum.SubscribeHeadersResult, error) {
						return nil, write.call(ctx)
					}
				case "fee":
					client.fee = func(ctx context.Context) (float32, error) {
						return 0, write.call(ctx)
					}
				case "keepalive":
					config.KeepAliveInterval = time.Millisecond
					client.ping = write.call
				}
				connection, err := connect(parent, config, func(_ context.Context, url string) (electrumClient, error) {
					if reconnect && url == "first" {
						return first, nil
					}
					return client, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if reconnect {
					first.Shutdown()
				}
				finished := write.finished
				result := make(chan error, 1)
				if operation != "keepalive" {
					finished = make(chan struct{})
					go func() {
						defer close(finished)
						var err error
						if operation == "fee" {
							_, err = connection.getFeeBtcPerKbOnce(1)
						} else {
							_, err = connection.GetLatestBlockHeight()
						}
						result <- err
					}()
				}
				t.Cleanup(func() {
					cancel()
					write.release()
					awaitClientResult(t, finished)
				})
				awaitClientResult(t, write.started)
				cancel()
				select {
				case <-finished:
				case <-time.After(time.Second):
					t.Fatal("connection cancellation did not close the transport during a blocked write")
				}
				if operation != "keepalive" {
					if err := <-result; err == nil {
						t.Fatal("expected the interrupted RPC to fail")
					}
				}
				client.awaitShutdown(t)
				if reconnect {
					first.awaitShutdown(t)
				}
			})
		}
	}
}

func TestConnectionCancellationUnblocksVerificationWrite(t *testing.T) {
	for _, reconnect := range []bool{false, true} {
		name := "initial connection"
		if reconnect {
			name = "reconnection"
		}
		t.Run(name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := new(failoverTestClient)
			client := new(failoverTestClient)
			write := newBlockedClientWrite(client)
			client.version = write.call
			result := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				connection, err := connect(parent, failoverTestConfig(), func(_ context.Context, url string) (electrumClient, error) {
					if reconnect && url == "first" {
						return first, nil
					}
					return client, nil
				})
				if err == nil && reconnect {
					first.Shutdown()
					_, err = connection.GetLatestBlockHeight()
				}
				result <- err
			}()
			t.Cleanup(func() {
				cancel()
				write.release()
				awaitClientResult(t, finished)
			})
			awaitClientResult(t, write.started)
			cancel()
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("connection cancellation did not close the client being verified")
			}
			if err := <-result; err == nil {
				t.Fatal("expected interrupted verification to fail")
			}
			client.awaitShutdown(t)
		})
	}
}

func TestConnectionCancellationClosesReplacementDuringRetiredCleanup(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.RequestRetryTimeout = 50 * time.Millisecond
	first := &failoverTestClient{header: func(context.Context) (*electrum.SubscribeHeadersResult, error) {
		return nil, errors.New("server unavailable")
	}}
	closing := blockClientShutdown(t, first)
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
		t.Fatal("expected the first request to fail")
	}
	awaitClientResult(t, closing)
	if _, err := connection.GetLatestBlockHeight(); err != nil {
		t.Fatal(err)
	}
	cancel()
	second.awaitShutdown(t)
}
