package electrum

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketTransport struct {
	conn      *websocket.Conn
	responses chan []byte
	errors    chan error
	done      chan struct{}
	closeOnce sync.Once
}

// NewWebSocketTransport initializes new WebSocket transport.
func NewWebSocketTransport(
	ctx context.Context,
	url string,
	tlsConfig *tls.Config,
) (*WebSocketTransport, error) {
	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	conn, response, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		if DebugMode {
			log.Printf(
				"%s [debug] connect -> status: %v, error: %v",
				time.Now().Format("2006-01-02 15:04:05"),
				response.Status,
				err,
			)
		}
		return nil, err
	}

	// Bound per-message allocation: without a limit, a hostile or buggy
	// server can force unbounded reads via ReadMessage.
	conn.SetReadLimit(32 << 20)

	ws := &WebSocketTransport{
		conn:      conn,
		responses: make(chan []byte),
		errors:    make(chan error),
		done:      make(chan struct{}),
	}

	go ws.listen()

	return ws, nil
}

func (t *WebSocketTransport) listen() {
	defer t.Close()

	for {
		_, msg, err := t.conn.ReadMessage()
		if DebugMode {
			log.Printf(
				"%s [debug] %s -> msg: %s, err: %v",
				time.Now().Format("2006-01-02 15:04:05"),
				t.conn.RemoteAddr(),
				msg,
				err,
			)
		}
		if err != nil {
			if errors.Is(err, websocket.ErrReadLimit) {
				err = fmt.Errorf("%w: %v", ErrMessageTooLarge, err)
			}
			select {
			case t.errors <- err:
			case <-t.done:
			}

			break
		}

		select {
		case t.responses <- msg:
		case <-t.done:
			return
		}
	}
}

// SendMessage sends a message to the remote server through the WebSocket transport.
func (t *WebSocketTransport) SendMessage(body []byte) error {
	if DebugMode {
		log.Printf("%s [debug] %s <- %s", time.Now().Format("2006-01-02 15:04:05"), t.conn.RemoteAddr(), body)
	}

	return t.conn.WriteMessage(websocket.TextMessage, body)
}

// Responses returns chan to WebSocket transport responses.
func (t *WebSocketTransport) Responses() <-chan []byte {
	return t.responses
}

// Errors returns chan to WebSocket transport errors.
func (t *WebSocketTransport) Errors() <-chan error {
	return t.errors
}

// Close aborts the connection. Gorilla permits Conn.Close concurrently with
// reads and writes; WriteMessage cannot be used here while a writer is active.
func (t *WebSocketTransport) Close() (err error) {
	t.closeOnce.Do(func() {
		close(t.done)
		err = t.conn.Close()
	})
	return err
}
