package electrum

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
)

const (
	// ClientVersion identifies the client version/name to the remote server
	ClientVersion = "go-electrum1.1"

	// ProtocolVersion identifies the support protocol version to the remote server
	ProtocolVersion = "1.4"

	nl = byte('\n')
)

var (
	// DebugMode provides debug output on communications with the remote server if enabled.
	DebugMode bool

	// ErrServerConnected throws an error if remote server is already connected.
	ErrServerConnected = errors.New("server is already connected")

	// ErrServerShutdown throws an error if remote server has shutdown.
	ErrServerShutdown = errors.New("server has shutdown")

	// ErrTimeout throws an error if request has timed out
	ErrTimeout = errors.New("request timeout")

	// ErrNotImplemented throws an error if this RPC call has not been implemented yet.
	ErrNotImplemented = errors.New("RPC call is not implemented")

	// ErrDeprecated throws an error if this RPC call is deprecated.
	ErrDeprecated = errors.New("RPC call has been deprecated")
)

// Transport provides interface to server transport.
type Transport interface {
	SendMessage([]byte) error
	Responses() <-chan []byte
	Errors() <-chan error
	Close() error
}

type container struct {
	content []byte
	err     error
}

// Client stores information about the remote server.
type Client struct {
	transport Transport

	handlers     map[uint64]chan *container
	handlersLock sync.RWMutex

	pushHandlers     map[string][]chan *container
	pushHandlersLock sync.RWMutex

	Error        chan error
	quit         chan struct{}
	shutdownOnce sync.Once

	nextID uint64
}

// NewClient initializes a new client for remote server and connects to it using
// a transport protocol resolved from the URL's protocol scheme.
// A remote server URL should be provided in the `scheme://hostname:port` format
// (e.g. `tcp://electrum.io:50001`).
func NewClient(ctx context.Context, urlStr string, tlsConfig *tls.Config) (*Client, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url [%s]: [%w]", urlStr, err)
	}

	switch u.Scheme {
	case "tcp":
		return NewClientTCP(ctx, u.Host)
	case "ssl":
		return NewClientSSL(ctx, u.Host, tlsConfig)
	case "ws", "wss":
		return NewClientWebSocket(ctx, u.String(), tlsConfig)
	}

	return nil, fmt.Errorf("unsupported protocol scheme: [%s]", u.Scheme)
}

// NewClientTCP initialize a new client for remote server and connects to the remote server using TCP
func NewClientTCP(ctx context.Context, addr string) (*Client, error) {
	transport, err := NewTCPTransport(ctx, addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		handlers:     make(map[uint64]chan *container),
		pushHandlers: make(map[string][]chan *container),

		Error: make(chan error, 1),
		quit:  make(chan struct{}),
	}

	c.transport = transport
	go c.listen()

	return c, nil
}

// NewClientSSL initialize a new client for remote server and connects to the remote server using SSL
func NewClientSSL(ctx context.Context, addr string, config *tls.Config) (*Client, error) {
	transport, err := NewSSLTransport(ctx, addr, config)
	if err != nil {
		return nil, err
	}

	c := &Client{
		handlers:     make(map[uint64]chan *container),
		pushHandlers: make(map[string][]chan *container),

		Error: make(chan error, 1),
		quit:  make(chan struct{}),
	}

	c.transport = transport
	go c.listen()

	return c, nil
}

// NewClientWebSocket initialize a new client for remote server and connects to
// the remote server using WebSocket.
func NewClientWebSocket(ctx context.Context, url string, config *tls.Config) (*Client, error) {
	transport, err := NewWebSocketTransport(ctx, url, config)
	if err != nil {
		return nil, err
	}

	c := &Client{
		handlers:     make(map[uint64]chan *container),
		pushHandlers: make(map[string][]chan *container),

		Error: make(chan error, 1),
		quit:  make(chan struct{}),
	}

	c.transport = transport
	go c.listen()

	return c, nil
}

// JSON-RPC 2.0 Error Object
// See: https://www.jsonrpc.org/specificationJSON#error_object
type apiErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *apiErr) Error() string {
	return fmt.Sprintf("errNo: %d, errMsg: %s", e.Code, e.Message)
}

// UnmarshalJSON defines a workaround for servers that respond with error
// that doesn't follow the JSON-RPC 2.0 Error Object format, i.e. electrs/esplora.
// See: https://github.com/Blockstream/esplora/issues/453
func (e *apiErr) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal error [%s]: %v", data, err)
	}

	switch v := v.(type) {
	case string:
		e.Message = v
	case map[string]interface{}:
		if code, ok := v["code"].(float64); ok {
			e.Code = int(code)
		}

		if _, ok := v["message"]; ok {
			e.Message = fmt.Sprint(v["message"])
		}
	default:
		return fmt.Errorf("unsupported type: %v", v)
	}

	return nil
}

// JSON-RPC 2.0 Response Object
// See: https://www.jsonrpc.org/specification#response_object
type response struct {
	ID     uint64  `json:"id"`
	Method string  `json:"method"`
	Error  *apiErr `json:"error"`
}

func (s *Client) listen() {
	for {
		if s.IsShutdown() {
			break
		}
		if s.transport == nil {
			break
		}

		exit := func() (exit bool) {
			// A hostile or malformed response must never propagate a panic
			// out of the read loop goroutine; recover, log, and keep the
			// loop running unless the client is shutting down anyway.
			defer func() {
				if r := recover(); r != nil {
					if DebugMode {
						log.Printf("recovered from panic while processing message: %v", r)
					}
					exit = s.IsShutdown()
				}
			}()

			select {
			case <-s.quit:
				return true
			case err := <-s.transport.Errors():
				select {
				case s.Error <- err:
				default:
				}
				s.Shutdown()
				return true
			case bytes := <-s.transport.Responses():
				result := &container{
					content: bytes,
				}

				msg := &response{}
				err := json.Unmarshal(bytes, msg)
				if err != nil {
					if DebugMode {
						log.Printf("unmarshal received message [%s] failed: [%v]", bytes, err)
					}
					result.err = fmt.Errorf("Unmarshal received message failed: %v", err)
				} else if msg.Error != nil {
					result.err = msg.Error
				}

				if len(msg.Method) > 0 {
					s.pushHandlersLock.RLock()
					handlers := s.pushHandlers[msg.Method]
					s.pushHandlersLock.RUnlock()

					for _, handler := range handlers {
						select {
						case handler <- result:
						default:
						}
					}
				}

				s.handlersLock.RLock()
				c, ok := s.handlers[msg.ID]
				s.handlersLock.RUnlock()

				if ok {
					select {
					case c <- result:
					case <-s.quit:
						return true
					}
				}
			}
			return false
		}()

		if exit {
			return
		}
	}
}

func (s *Client) listenPush(method string) <-chan *container {
	c := make(chan *container, 1)
	s.pushHandlersLock.Lock()
	s.pushHandlers[method] = append(s.pushHandlers[method], c)
	s.pushHandlersLock.Unlock()

	return c
}

type request struct {
	ID     uint64        `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func (s *Client) request(ctx context.Context, method string, params []interface{}, v interface{}) error {
	select {
	case <-s.quit:
		return ErrServerShutdown
	default:
	}

	msg := request{
		ID:     atomic.AddUint64(&s.nextID, 1),
		Method: method,
		Params: params,
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	bytes = append(bytes, nl)

	c := make(chan *container, 1)

	s.handlersLock.Lock()
	s.handlers[msg.ID] = c
	s.handlersLock.Unlock()

	defer func() {
		s.handlersLock.Lock()
		delete(s.handlers, msg.ID)
		s.handlersLock.Unlock()
	}()

	// Register the response handler before sending: a local or fast server can
	// reply before SendMessage returns.
	err = s.transport.SendMessage(bytes)
	if err != nil {
		s.Shutdown()
		return err
	}

	var resp *container
	select {
	case resp = <-c:
	case <-ctx.Done():
		return ErrTimeout
	case <-s.quit:
		return ErrServerShutdown
	}

	if resp.err != nil {
		return resp.err
	}

	if v != nil {
		err = json.Unmarshal(resp.content, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// Abort interrupts outstanding reads and writes without a protocol close
// handshake. It is safe to call concurrently with an RPC or another shutdown.
func (s *Client) Abort() {
	s.shutdownOnce.Do(func() {
		close(s.quit)
		_ = s.transport.Close()
	})
}

func (s *Client) Shutdown() {
	s.Abort()
}

func (s *Client) IsShutdown() bool {
	select {
	case <-s.quit:
		return true
	default:
	}
	return false
}
