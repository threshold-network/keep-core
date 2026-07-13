package electrum

import (
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

const (
	// DefaultConnectTimeout is a default timeout used for a single attempt of
	// Electrum connection establishment.
	DefaultConnectTimeout = 10 * time.Second
	// DefaultConnectRetryTimeout is a default timeout used for Electrum
	// connection establishment retries.
	DefaultConnectRetryTimeout = 1 * time.Minute
	// DefaultRequestTimeout is a default timeout used for a single attempt of
	// Electrum protocol request.
	DefaultRequestTimeout = 30 * time.Second
	// DefaultRequestRetryTimeout is a default timeout used for Electrum protocol
	// request retries.
	DefaultRequestRetryTimeout = 2 * time.Minute
	// DefaultKeepAliveInterval is a default interval used for Electrum server
	// connection keep alive requests.
	DefaultKeepAliveInterval = 5 * time.Minute
)

// Config holds configurable properties.
type Config struct {
	// URL to the Electrum server in format: `scheme://hostname:port`.
	URL string
	// Timeout for a single attempt of Electrum connection establishment.
	ConnectTimeout time.Duration
	// Timeout for Electrum connection establishment retries.
	ConnectRetryTimeout time.Duration
	// Timeout for a single attempt of Electrum protocol request.
	RequestTimeout time.Duration
	// Timeout for Electrum protocol request retries.
	RequestRetryTimeout time.Duration
	// Interval for connection keep alive requests.
	// An Electrum server may disconnect clients that have not sent any requests
	// for roughly 10 minutes.
	KeepAliveInterval time.Duration
	// Network is the Bitcoin network this connection operates on. It is set
	// internally from the resolved client configuration, not decoded from the
	// config file (`mapstructure:"-"`), so it cannot be used to alter
	// network-sensitive client behavior such as the low-fee estimate fallback.
	// A zero value (bitcoin.Unknown) disables that fallback (fail-safe).
	Network bitcoin.Network `mapstructure:"-"`
}
