package electrum

import (
	"context"
	"testing"
	"time"
)

func TestProvisionalVerificationDeadlineUnblocksWrite(t *testing.T) {
	for _, budget := range []string{"caller cancellation", "caller deadline", "RPC", "connection retry", "request retry"} {
		t.Run(budget, func(t *testing.T) {
			parent, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			config := failoverTestConfig()
			config.FallbackURLs = []string{"second", "third"}
			config.ConnectTimeout = time.Second
			config.RequestTimeout = time.Second
			config.ConnectRetryTimeout = 150 * time.Millisecond
			config.RequestRetryTimeout = 250 * time.Millisecond
			switch budget {
			case "RPC":
				config.RequestTimeout = 50 * time.Millisecond
			case "connection retry":
				config.ConnectRetryTimeout = 50 * time.Millisecond
			case "request retry":
				config.RequestRetryTimeout = 50 * time.Millisecond
			}
			first := new(failoverTestClient)
			provisional := new(failoverTestClient)
			write := newBlockedClientWrite(provisional)
			provisional.version = write.call
			healthy := new(failoverTestClient)
			var attempts []string
			connection, err := connect(parent, config, func(_ context.Context, url string) (electrumClient, error) {
				attempts = append(attempts, url)
				switch len(attempts) {
				case 1:
					return first, nil
				case 2:
					return provisional, nil
				default:
					return healthy, nil
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			first.Shutdown()
			caller, cancelCaller := context.WithCancel(parent)
			if budget == "caller deadline" {
				cancelCaller()
				caller, cancelCaller = context.WithTimeout(parent, 50*time.Millisecond)
			}
			defer cancelCaller()
			result := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				_, err := requestWithRetry(caller, connection, func(context.Context, electrumClient) (int, error) {
					return 42, nil
				}, "provisional verification")
				result <- err
			}()
			t.Cleanup(func() {
				cancelCaller()
				write.release()
				awaitClientResult(t, finished)
			})
			awaitClientResult(t, write.started)
			if budget == "caller cancellation" {
				cancelCaller()
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("verification deadline did not interrupt the provisional client's write")
			}
			if err := <-result; err == nil {
				t.Fatal("expected interrupted verification to fail")
			}
			if parent.Err() != nil {
				t.Fatal("connection cancellation must not be needed to unblock verification")
			}
			provisional.awaitShutdown(t)
			if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
				t.Fatalf("independent request remained blocked after verification: %d, %v", height, err)
			}
			wantURL := "third"
			if budget == "caller cancellation" || budget == "caller deadline" {
				wantURL = "second"
			}
			if len(attempts) != 3 || attempts[2] != wantURL {
				t.Fatalf("unexpected candidate after verification timeout: %v", attempts)
			}
		})
	}
}

func TestVerifiedClientOutlivesVerificationContext(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	var verification context.Context
	client := &failoverTestClient{version: func(ctx context.Context) error {
		verification = ctx
		return nil
	}}
	connection, err := connect(parent, config, func(context.Context, string) (electrumClient, error) {
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	awaitClientResult(t, verification.Done())
	// Pass the former verification deadline before using the retained client.
	deadline, _ := verification.Deadline()
	<-time.After(time.Until(deadline) + 20*time.Millisecond)
	if client.IsShutdown() {
		t.Fatal("verification cancellation aborted a retained client")
	}
	if height, err := connection.GetLatestBlockHeight(); err != nil || height != 42 {
		t.Fatalf("verified client did not remain usable: %d, %v", height, err)
	}
	if client.versions.Load() != 1 {
		t.Fatal("verification cancellation forced an unnecessary reconnect")
	}
}
