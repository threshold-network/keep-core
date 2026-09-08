package electrum

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestVerificationUsesRequestTimeout(t *testing.T) {
	for _, explicitURL := range []bool{false, true} {
		for _, reconnect := range []bool{false, true} {
			name := "server pool"
			if explicitURL {
				name = "explicit URL"
			}
			if reconnect {
				name += "/reconnect"
			} else {
				name += "/initial connection"
			}
			t.Run(name, func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				defer cancel()
				config := failoverTestConfig()
				config.ConnectTimeout = 20 * time.Millisecond
				config.RequestTimeout = 200 * time.Millisecond
				config.ConnectRetryTimeout = 300 * time.Millisecond
				config.RequestRetryTimeout = 300 * time.Millisecond
				if explicitURL {
					config.FallbackURLs = nil
				}
				first := new(failoverTestClient)
				verified := new(failoverTestClient)
				var dialDeadline time.Time
				verified.version = func(ctx context.Context) error {
					deadline, ok := ctx.Deadline()
					if !ok || !deadline.After(dialDeadline) {
						t.Error("verification inherited the dial deadline")
					}
					// Reply after the dial deadline but within the RPC timeout.
					response := time.NewTimer(time.Until(dialDeadline.Add(20 * time.Millisecond)))
					defer response.Stop()
					select {
					case <-response.C:
						return ctx.Err()
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				attempts := 0
				connection, err := connect(parent, config, func(ctx context.Context, _ string) (electrumClient, error) {
					attempts++
					if reconnect && attempts == 1 {
						return first, nil
					}
					dialDeadline, _ = ctx.Deadline()
					return verified, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if reconnect {
					first.Shutdown()
					if _, err := connection.GetLatestBlockHeight(); err != nil {
						t.Fatal(err)
					}
				}
				if connection.client != verified || verified.versions.Load() != 1 || verified.IsShutdown() {
					t.Fatal("verification within the RPC timeout did not retain the client")
				}
			})
		}
	}
}

func TestReconnectCallerCancellationPreservesCandidate(t *testing.T) {
	for _, phase := range []string{"dial", "verification"} {
		for _, deadline := range []bool{false, true} {
			name := phase + "/cancellation"
			if deadline {
				name = phase + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				parent, cancelParent := context.WithCancel(context.Background())
				defer cancelParent()
				config := failoverTestConfig()
				config.ConnectTimeout = time.Second
				config.RequestTimeout = time.Second
				first := new(failoverTestClient)
				interrupted := new(failoverTestClient)
				healthy := new(failoverTestClient)
				var cancelCaller context.CancelFunc
				interrupt := func(ctx context.Context) error {
					if !deadline {
						cancelCaller()
					}
					<-ctx.Done()
					return ctx.Err()
				}
				interrupted.version = interrupt
				var attempts []string
				connection, err := connect(parent, config, func(ctx context.Context, url string) (electrumClient, error) {
					attempts = append(attempts, url)
					if len(attempts) == 1 {
						return first, nil
					}
					if len(attempts) == 2 {
						if phase == "dial" {
							return nil, interrupt(ctx)
						}
						return interrupted, nil
					}
					return healthy, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				first.Shutdown()
				var caller context.Context
				wantErr := context.Canceled
				if deadline {
					caller, cancelCaller = context.WithTimeout(parent, 50*time.Millisecond)
					wantErr = context.DeadlineExceeded
				} else {
					caller, cancelCaller = context.WithCancel(parent)
				}
				defer cancelCaller()
				if _, err := connection.GetTransactionConfirmations(caller, bitcoin.Hash{}); !errors.Is(err, wantErr) {
					t.Fatalf("expected caller interruption, got %v", err)
				}
				if connection.serverIndex != 1 || connection.client != nil {
					t.Error("caller interruption discarded the reconnect candidate")
				}
				if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
					t.Fatalf("independent request could not reconnect: %d, %v", height, err)
				}
				if len(attempts) != 3 || attempts[0] != "first" || attempts[1] != "second" || attempts[2] != "second" {
					t.Fatalf("independent request did not retry the preserved candidate: %v", attempts)
				}
				if phase == "verification" {
					interrupted.awaitShutdown(t)
				}
			})
		}
	}
}

func TestReconnectInternalTimeoutAdvancesCandidate(t *testing.T) {
	for _, test := range []struct {
		phase  string
		budget string
	}{
		{"dial", "dial"},
		{"verification", "RPC"},
		{"dial", "connection retry"},
		{"verification", "connection retry"},
		{"dial", "request retry"},
		{"verification", "request retry"},
	} {
		t.Run(test.phase+"/"+test.budget, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			config := failoverTestConfig()
			config.FallbackURLs = []string{"second", "third"}
			config.ConnectTimeout = time.Second
			config.RequestTimeout = time.Second
			config.ConnectRetryTimeout = 150 * time.Millisecond
			config.RequestRetryTimeout = 250 * time.Millisecond
			switch test.budget {
			case "dial":
				config.ConnectTimeout = 20 * time.Millisecond
			case "RPC":
				config.RequestTimeout = 20 * time.Millisecond
			case "request retry":
				config.ConnectRetryTimeout = time.Second
				config.RequestRetryTimeout = 50 * time.Millisecond
			}
			first := new(failoverTestClient)
			second := &failoverTestClient{version: func(ctx context.Context) error {
				if deadline, ok := ctx.Deadline(); !ok || deadline.After(time.Now().Add(config.RequestTimeout)) {
					t.Error("verification exceeded the configured RPC timeout")
				}
				<-ctx.Done()
				return ctx.Err()
			}}
			third := new(failoverTestClient)
			connection, err := connect(parent, config, func(ctx context.Context, url string) (electrumClient, error) {
				switch url {
				case "first":
					return first, nil
				case "second":
					if test.phase == "dial" {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return second, nil
				default:
					return third, nil
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			first.Shutdown()
			if _, err := connection.GetLatestBlockHeight(); err == nil {
				t.Fatal("expected reconnection to exhaust an internal timeout")
			}
			if parent.Err() != nil || connection.serverIndex != 2 || connection.client != nil {
				t.Fatal("internal timeout did not advance the reconnect candidate")
			}
			if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
				t.Fatalf("independent request did not reach the third server: %d, %v", height, err)
			}
			if third.versions.Load() != 1 {
				t.Fatal("replacement client was not verified")
			}
			if test.phase == "verification" {
				second.awaitShutdown(t)
			}
		})
	}
}
