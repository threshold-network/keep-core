package electrum

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// TCPTransport store information about the TCP transport.
type TCPTransport struct {
	conn      net.Conn
	responses chan []byte
	errors    chan error
	done      chan struct{}
	closeOnce sync.Once
}

// NewTCPTransport opens a new TCP connection to the remote server.
func NewTCPTransport(ctx context.Context, addr string) (*TCPTransport, error) {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tcp := &TCPTransport{
		conn:      conn,
		responses: make(chan []byte),
		errors:    make(chan error),
		done:      make(chan struct{}),
	}

	go tcp.listen()

	return tcp, nil
}

// NewSSLTransport opens a new SSL connection to the remote server.
func NewSSLTransport(ctx context.Context, addr string, config *tls.Config) (*TCPTransport, error) {
	dialer := tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    config,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tcp := &TCPTransport{
		conn:      conn,
		responses: make(chan []byte),
		errors:    make(chan error),
		done:      make(chan struct{}),
	}

	go tcp.listen()

	return tcp, nil
}

// ErrMessageTooLarge indicates a transport read was abandoned because it
// exceeded the configured per-message size limit (maxLineSize here,
// SetReadLimit for the WebSocket transport), distinguishing an oversized
// response from a generic transport failure.
var ErrMessageTooLarge = errors.New("line exceeds maximum size")

// maxLineSize bounds a single line read from the transport. Without a limit,
// a server withholding the trailing newline could force unbounded buffering.
// Electrum protocol 1.4 has no response pagination, so long script
// histories, large UTXO lists, and raw tx hex can legitimately approach
// several MiB; the limit stays well above realistic response sizes.
const maxLineSize = 32 << 20

// readBoundedLine reads up to and including the next nl-terminated line,
// accumulating buffer-sized chunks across bufio.ErrBufferFull. It returns an
// error once the accumulated line exceeds limit, instead of buffering
// without bound.
func readBoundedLine(reader *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice(nl)
		line = append(line, chunk...)
		if len(line) > limit {
			return nil, fmt.Errorf("%w: %d bytes", ErrMessageTooLarge, limit)
		}
		if err == nil {
			return line, nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
}

func (t *TCPTransport) listen() {
	defer t.Close()
	reader := bufio.NewReader(t.conn)

	for {
		line, err := readBoundedLine(reader, maxLineSize)
		if err != nil {
			select {
			case t.errors <- err:
			case <-t.done:
			}
			break
		}
		if DebugMode {
			log.Printf("%s [debug] %s -> %s", time.Now().Format("2006-01-02 15:04:05"), t.conn.RemoteAddr(), line)
		}

		select {
		case t.responses <- line:
		case <-t.done:
			return
		}
	}
}

// SendMessage sends a message to the remote server through the TCP transport.
func (t *TCPTransport) SendMessage(body []byte) error {
	if DebugMode {
		log.Printf("%s [debug] %s <- %s", time.Now().Format("2006-01-02 15:04:05"), t.conn.RemoteAddr(), body)
	}

	_, err := t.conn.Write(body)
	return err
}

// Responses returns chan to TCP transport responses.
func (t *TCPTransport) Responses() <-chan []byte {
	return t.responses
}

// Errors returns chan to TCP transport errors.
func (t *TCPTransport) Errors() <-chan error {
	return t.errors
}

func (t *TCPTransport) Close() (err error) {
	t.closeOnce.Do(func() {
		close(t.done)
		err = t.conn.Close()
	})
	return err
}
