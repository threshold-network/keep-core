package electrum

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/checksum0/go-electrum/electrum"

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
				config.ConnectTimeout = 40 * time.Millisecond
				config.RequestTimeout = 300 * time.Millisecond
				config.ConnectRetryTimeout = 500 * time.Millisecond
				config.RequestRetryTimeout = 500 * time.Millisecond
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
					response := time.NewTimer(time.Until(dialDeadline.Add(40 * time.Millisecond)))
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
					caller, cancelCaller = context.WithTimeout(parent, 100*time.Millisecond)
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

// TestReconnectPerCandidateTimeoutAdvancesCandidate covers a genuine
// per-candidate timeout (the candidate's own ConnectTimeout/RequestTimeout
// elapses while the enclosing connect/request retry budget still has ample
// room) - the only case that should advance past the candidate as unhealthy.
func TestReconnectPerCandidateTimeoutAdvancesCandidate(t *testing.T) {
	for _, test := range []struct {
		phase  string
		budget string
	}{
		{"dial", "dial"},
		{"verification", "RPC"},
	} {
		t.Run(test.phase+"/"+test.budget, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			config := failoverTestConfig()
			config.FallbackURLs = []string{"second", "third"}
			config.ConnectTimeout = time.Second
			config.RequestTimeout = time.Second
			config.ConnectRetryTimeout = 300 * time.Millisecond
			config.RequestRetryTimeout = 500 * time.Millisecond
			switch test.budget {
			case "dial":
				config.ConnectTimeout = 40 * time.Millisecond
			case "RPC":
				config.RequestTimeout = 40 * time.Millisecond
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
				t.Fatal("expected reconnection to exhaust a per-candidate timeout")
			}
			if parent.Err() != nil || connection.serverIndex != 2 || connection.client != nil {
				t.Fatal("per-candidate timeout did not advance the reconnect candidate")
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

// TestReconnectRetryBudgetExhaustionPreservesCandidate covers the enclosing
// connect/request retry budget itself expiring mid-attempt (rather than the
// candidate's own per-request timeout). electrumConnect must not attribute
// that to the candidate's health: the candidate is preserved (retried on
// the next attempt), never advanced past as unhealthy.
func TestReconnectRetryBudgetExhaustionPreservesCandidate(t *testing.T) {
	for _, test := range []struct {
		phase  string
		budget string
	}{
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
			config.ConnectRetryTimeout = 300 * time.Millisecond
			config.RequestRetryTimeout = 500 * time.Millisecond
			if test.budget == "request retry" {
				config.ConnectRetryTimeout = time.Second
				config.RequestRetryTimeout = 100 * time.Millisecond
			}
			first := new(failoverTestClient)
			var verifyCalls atomic.Int32
			second := &failoverTestClient{version: func(ctx context.Context) error {
				verifyCalls.Add(1)
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
				t.Fatal("expected the retry budget to be exhausted")
			}
			if parent.Err() != nil {
				t.Fatal("the caller must remain active after the internal timeout")
			}
			if connection.serverIndex != 1 || connection.client != nil {
				t.Fatal("budget exhaustion incorrectly advanced past the preserved candidate")
			}
			if third.versions.Load() != 0 {
				t.Fatal("budget exhaustion incorrectly reached an untried server")
			}
		})
	}
}

// TestElectrumConnectRejectsGenesisMismatch verifies the D4 genesis identity
// behavior: the first successfully verified server's genesis hash becomes
// this connection's chain reference, and a later candidate reporting a
// different genesis hash is rejected outright rather than retained.
func TestElectrumConnectRejectsGenesisMismatch(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.FallbackURLs = []string{"second"}
	config.RequestRetryTimeout = 200 * time.Millisecond
	first := &failoverTestClient{serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
		return &electrum.ServerFeaturesResult{GenesisHash: "aaaa"}, nil
	}}
	wrongChain := &failoverTestClient{serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
		return &electrum.ServerFeaturesResult{GenesisHash: "bbbb"}, nil
	}}
	connection, err := connect(parent, config, func(_ context.Context, url string) (electrumClient, error) {
		if url == "first" {
			return first, nil
		}
		return wrongChain, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.verifiedGenesisHash != "aaaa" {
		t.Fatalf("expected the first server's genesis hash to be adopted, got %q", connection.verifiedGenesisHash)
	}
	first.Shutdown()
	if _, err := connection.GetLatestBlockHeight(); err == nil {
		t.Fatal("expected the wrong-chain candidate to be rejected")
	}
	if connection.client == wrongChain || !wrongChain.IsShutdown() {
		t.Fatal("a candidate reporting a mismatched genesis hash must not be retained")
	}
}
