package electrum

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Gate the socket write after the real HTTP/TLS upgrade. Gorilla's writer is
// active while Write waits, as it would be under network backpressure.
type gatedWriteConn struct {
	net.Conn
	gate    uint32
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
	start   sync.Once
}

func (c *gatedWriteConn) Write(p []byte) (int, error) {
	if atomic.LoadUint32(&c.gate) != 0 {
		c.start.Do(func() { close(c.started) })
		<-c.closed
		return 0, net.ErrClosed
	}
	return c.Conn.Write(p)
}

func (c *gatedWriteConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestShutdownInterruptsWebSocketWrite(t *testing.T) {
	for _, secure := range []bool{false, true} {
		for _, method := range []string{"verification", "keepalive"} {
			scheme := "ws"
			if secure {
				scheme = "wss"
			}
			t.Run(scheme+"/"+method, func(t *testing.T) {
				releaseServer := make(chan struct{})
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					upgrader := websocket.Upgrader{}
					peer, err := upgrader.Upgrade(w, r, nil)
					if err != nil {
						return
					}
					defer peer.Close()
					<-releaseServer
				}))
				if secure {
					server.StartTLS()
				} else {
					server.Start()
				}
				defer server.Close()
				defer close(releaseServer)
				var socket *gatedWriteConn
				dial := func(ctx context.Context, network, address string) (net.Conn, error) {
					var conn net.Conn
					var err error
					if secure {
						config := server.Client().Transport.(*http.Transport).TLSClientConfig
						dialer := tls.Dialer{Config: config}
						conn, err = dialer.DialContext(ctx, network, address)
					} else {
						var dialer net.Dialer
						conn, err = dialer.DialContext(ctx, network, address)
					}
					if err != nil {
						return nil, err
					}
					socket = &gatedWriteConn{Conn: conn, started: make(chan struct{}), closed: make(chan struct{})}
					return socket, nil
				}
				dialer := websocket.Dialer{NetDialContext: dial, NetDialTLSContext: dial}
				conn, response, err := dialer.Dial(strings.Replace(server.URL, "http", "ws", 1), nil)
				if response != nil {
					response.Body.Close()
				}
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				transport := &WebSocketTransport{conn: conn, done: make(chan struct{})}
				// No reader is needed for a write that fails on socket closure.
				client := &Client{transport: transport, quit: make(chan struct{}), handlers: make(map[uint64]chan *container)}
				atomic.StoreUint32(&socket.gate, 1)
				result := make(chan error, 1)
				go func() {
					if method == "verification" {
						_, _, err := client.ServerVersion(context.Background())
						result <- err
					} else {
						result <- client.Ping(context.Background())
					}
				}()
				select {
				case <-socket.started:
				case <-time.After(time.Second):
					t.Fatal("RPC did not enter the WebSocket write")
				}
				func() {
					defer func() {
						if p := recover(); p != nil {
							t.Errorf("shutdown panicked during an active WebSocket write: %v", p)
						}
					}()
					client.Shutdown()
				}()
				select {
				case <-socket.closed:
				default:
					t.Error("shutdown returned without aborting the socket")
				}
				// Release and join the writer even when the regression is present.
				socket.Close()
				select {
				case err := <-result:
					if err == nil {
						t.Error("expected the aborted write to fail")
					}
				case <-time.After(time.Second):
					t.Fatal("shutdown did not unblock the WebSocket writer")
				}
			})
		}
	}
}

// TestShutdownInterruptsTCPWrite is the TCPTransport parity case for
// TestShutdownInterruptsWebSocketWrite: a blocked write must be unblocked by
// Client.Shutdown aborting the transport, not left hanging on the socket.
func TestShutdownInterruptsTCPWrite(t *testing.T) {
	for _, method := range []string{"verification", "keepalive"} {
		t.Run(method, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer serverConn.Close()
			socket := &gatedWriteConn{Conn: clientConn, started: make(chan struct{}), closed: make(chan struct{})}
			transport := &TCPTransport{conn: socket, done: make(chan struct{})}
			// No reader is needed for a write that fails on socket closure.
			client := &Client{transport: transport, quit: make(chan struct{}), handlers: make(map[uint64]chan *container)}
			atomic.StoreUint32(&socket.gate, 1)
			result := make(chan error, 1)
			go func() {
				if method == "verification" {
					_, _, err := client.ServerVersion(context.Background())
					result <- err
				} else {
					result <- client.Ping(context.Background())
				}
			}()
			select {
			case <-socket.started:
			case <-time.After(time.Second):
				t.Fatal("RPC did not enter the TCP write")
			}
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("shutdown panicked during an active TCP write: %v", p)
					}
				}()
				client.Shutdown()
			}()
			select {
			case <-socket.closed:
			default:
				t.Error("shutdown returned without aborting the socket")
			}
			// Release and join the writer even when the regression is present.
			socket.Close()
			select {
			case err := <-result:
				if err == nil {
					t.Error("expected the aborted write to fail")
				}
			case <-time.After(time.Second):
				t.Fatal("shutdown did not unblock the TCP writer")
			}
		})
	}
}

func TestAbortReleasesPendingResponseAndReaders(t *testing.T) {
	conn, peer := net.Pipe()
	defer peer.Close()
	transport := &TCPTransport{
		conn: conn, done: make(chan struct{}),
		responses: make(chan []byte), errors: make(chan error),
	}
	client := &Client{
		transport: transport, quit: make(chan struct{}), Error: make(chan error, 1),
		handlers: make(map[uint64]chan *container),
	}
	transportDone := make(chan struct{})
	clientDone := make(chan struct{})
	rpcDone := make(chan struct{})
	peerDone := make(chan struct{})
	result := make(chan error, 1)
	go func() { defer close(transportDone); transport.listen() }()
	go func() { defer close(clientDone); client.listen() }()
	go func() {
		defer close(rpcDone)
		_, _, err := client.ServerVersion(context.Background())
		result <- err
	}()
	go func() {
		defer close(peerDone)
		_, _ = bufio.NewReader(peer).ReadString('\n')
	}()
	t.Cleanup(func() {
		client.Abort()
		peer.Close()
		for _, done := range []chan struct{}{transportDone, clientDone, rpcDone, peerDone} {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("abort left a client goroutine blocked")
			}
		}
	})
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("peer did not receive the version request")
	}
	// Abort may race with a caller, the reader, or failed-write cleanup.
	var group sync.WaitGroup
	for i := 0; i < 3; i++ {
		group.Add(1)
		go func() { defer group.Done(); client.Shutdown() }()
	}
	group.Wait()
	select {
	case err := <-result:
		if !errors.Is(err, ErrServerShutdown) {
			t.Errorf("pending response returned %v instead of shutdown", err)
		}
	case <-time.After(time.Second):
		t.Fatal("abort did not interrupt the pending response")
	}
}
