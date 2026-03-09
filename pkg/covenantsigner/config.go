package covenantsigner

const DefaultListenAddress = "127.0.0.1"

// Config configures the covenant signer HTTP service.
type Config struct {
	// Port enables the covenant signer provider HTTP surface when non-zero.
	Port int
	// ListenAddress controls which interface the covenant signer HTTP service
	// binds to. Empty defaults to loopback-only.
	ListenAddress string
	// AuthToken enables static Bearer authentication for signer endpoints.
	// Non-loopback binds must set this.
	AuthToken string
}
